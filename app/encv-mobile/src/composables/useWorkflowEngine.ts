/**
 * DAG 工作流引擎 — 主 composable
 *
 * 整合调度器、执行器、存储层、WS 回调，提供完整的
 * WorkflowDefinition CRUD + 运行 + 状态跟踪能力。
 *
 * 架构：
 *   useWorkflowEngine (composable)
 *   ├── Scheduler（resolveDAG / getNextReadyJobs）
 *   ├── Executor（executeStep / handleMatrix）
 *   ├── Store（useWorkflowStore — localStorage）
 *   └── Observer（Vue ref 驱动 UI）
 */

import { ref, computed } from 'vue'
import {
  createTask,
  type EncvTask,
} from '@/api/encv'
import { recordTriggeredBy, setTaskMetadata, type TriggeredBy } from '@/composables/useTaskTrigger'
import { analyzeError } from '@/composables/useErrorAnalyzer'
import { eventBus } from '@/composables/useEventBus'
import { useWorkflowStore } from './useWorkflowStore'
import type {
  WorkflowRun,
  JobRun,
  StepRun,
  JobDefinition,
  StepDefinition,
  MatrixBinding,
  StepStatus,
  ActionSpec,
  EncvTaskActionSpec,
} from '@/lib/workflow/types'
import { isTerminalStep } from '@/lib/workflow/types'
import { computeJobConclusion, inferWorkflowStatus } from '@/lib/workflow/stateMachine'
import { resolveExecutionOrder, getNextReadyJobs } from '@/lib/workflow/scheduler'
import { expandMatrix, isMatrixStrategy } from '@/lib/workflow/matrixExpander'
import { evaluateCondition } from '@/lib/workflow/conditionEvaluator'

function genId(): string {
  return 'run-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 8)
}

export function useWorkflowEngine() {
  const store = useWorkflowStore()

  // ==================== 响应式状态 ====================

  const isRunning = ref(false)
  const currentRun = ref<WorkflowRun | null>(null)

  /** 当前运行中所有 step 的总数 */
  const totalSteps = computed(() => {
    if (!currentRun.value) return 0
    return currentRun.value.jobs.reduce((sum, j) => sum + j.steps.length, 0)
  })

  /** 当前已完成（终态）的 step 数 */
  const completedSteps = computed(() => {
    if (!currentRun.value) return 0
    return currentRun.value.jobs.reduce(
      (sum, j) => sum + j.steps.filter((s) => isTerminalStep(s.status)).length,
      0,
    )
  })

  /** 当前成功的 step 数 */
  const successSteps = computed(() => {
    if (!currentRun.value) return 0
    return currentRun.value.jobs.reduce(
      (sum, j) => sum + j.steps.filter((s) => s.status === 'success').length,
      0,
    )
  })

  /** 当前失败的 step 数 */
  const failedSteps = computed(() => {
    if (!currentRun.value) return 0
    return currentRun.value.jobs.reduce(
      (sum, j) => sum + j.steps.filter((s) => s.status === 'failure' || s.status === 'timed_out').length,
      0,
    )
  })

  // ==================== WS 回调监听 ====================
  // 🆕 2026-06-10 修复：补齐 task:update / task:progress / task:created 监听
  // 历史 bug：startListening 只监听 task:completed → 测试报告 "running" 期间
  //   step.status 永远是 'pending'，completedSteps = 0 / totalSteps = N
  //   progress 一直 0%、徽章一直 ◌（用户截图的"测试报告状态更新异常"）
  // 修复：完整订阅 4 个 task 事件，currentRun.jobs[].steps[].status 实时同步

  function findStepByTaskId(taskId: string): { step: StepRun; job: JobRun } | null {
    if (!currentRun.value) return null
    for (const job of currentRun.value.jobs) {
      for (const step of job.steps) {
        if (step.taskId === taskId) return { step, job }
      }
    }
    return null
  }

  function onTaskCreated(data: { id: string; type: string; sourcePath: string }) {
    if (!currentRun.value) return
    // 任务刚被后端创建 → step 从 'pending' 升级到 'queued'（如果后端已绑 taskId）
    const found = findStepByTaskId(data.id)
    if (found && found.step.status === 'pending') {
      found.step.status = 'queued'
    }
  }

  function onTaskUpdate(data: { id: string; type: string; status: string; progress: number }) {
    if (!currentRun.value) return
    const found = findStepByTaskId(data.id)
    if (!found) return
    const { step } = found

    // 🆕 2026-06-10 修复 #3：终态保护 — 已完成的 step 不接受 task:update 降级
    if (isTerminalStep(step.status)) {
      if (typeof data.progress === 'number') step.progress = data.progress
      return
    }

    // 状态机升级（pending → queued → running）
    if (data.status === 'running' && (step.status === 'pending' || step.status === 'queued')) {
      step.status = 'running'
      if (!step.startedAt) step.startedAt = new Date().toISOString()
    } else if (data.status === 'cancelling' && step.status === 'running') {
      step.status = 'cancelling'
    } else if (data.status === 'cancelled' && !isTerminalStep(step.status)) {
      step.status = 'cancelled'
      step.completedAt = new Date().toISOString()
      if (step.startedAt) {
        step.durationMs = Date.now() - new Date(step.startedAt).getTime()
      }
    }
    if (typeof data.progress === 'number') {
      step.progress = data.progress
    }
  }

  function onTaskProgress(data: { id: string; progress: number; phase: string; speed: string; eta: string }) {
    if (!currentRun.value) return
    const found = findStepByTaskId(data.id)
    if (!found) return
    const { step } = found
    if (typeof data.progress === 'number') step.progress = data.progress
    if (data.phase) step.phase = data.phase
    if (data.speed) step.speed = data.speed
    if (data.eta) step.eta = data.eta
  }

  function onTaskCompleted(data: { id: string; error?: string }) {
    if (!currentRun.value) return

    // 在所有 jobs/steps 中查找匹配的 taskId
    let found = false
    for (const job of currentRun.value.jobs) {
      for (const step of job.steps) {
        if (step.taskId !== data.id) continue

        // 🆕 2026-06-10 修复 #3：放宽校验 — 接受任何非终态 step
        // 历史 bug：`step.status !== 'running'` 强校验 → 后端没发 task:update 时
        //   step 永远停留在 'pending'，task:completed 找不到匹配 → 一直显示运行中
        if (isTerminalStep(step.status)) continue
        found = true

        // 更新 step 状态
        const newStatus: StepStatus = data.error ? 'failure' : 'success'
        step.status = newStatus
        step.completedAt = new Date().toISOString()
        step.durationMs = step.startedAt
          ? Date.now() - new Date(step.startedAt).getTime()
          : 0
        if (data.error) {
          step.error = data.error
          step.errorAnalysis = analyzeError(data.error, { phase: 'backend' })
        }

        // 检查所属 Job 是否全部完成
        checkJobCompletion(job)
        break
      }
      if (found) break
    }

    if (found) {
      // 检查整个 workflow 是否完成
      checkWorkflowCompletion()
      // 持久化运行状态
      store.updateRun(currentRun.value.id, { ...currentRun.value })
    } else {
      // 🆕 调试：找不到匹配 step 时输出（防止"已完成的 task 不在 run 里"被静默吞）
      console.debug('[useWorkflowEngine] task:completed but no matching step:', {
        taskId: data.id,
        runId: currentRun.value.id,
        totalSteps: currentRun.value.jobs.reduce((sum, j) => sum + j.steps.length, 0),
      })
    }
  }

  function startListening() {
    eventBus.on('task:completed', onTaskCompleted)
    eventBus.on('task:update', onTaskUpdate)
    eventBus.on('task:progress', onTaskProgress)
    eventBus.on('task:created', onTaskCreated)
  }

  function stopListening() {
    eventBus.off('task:completed', onTaskCompleted)
    eventBus.off('task:update', onTaskUpdate)
    eventBus.off('task:progress', onTaskProgress)
    eventBus.off('task:created', onTaskCreated)
  }

  // ==================== 执行核心 ====================

  /**
   * 触发一次工作流运行。
   *
   * 🆕 2026-06-10 修复 #3 + #4：测试报告不显示 job / 所有任务平铺并行
   *   历史 bug：
   *     - runWorkflow for layerJobIds await executeJob 串行 → jobs 推入延后
   *     - executeJob for-await 串行 submitAction → 200 个 case 全排队
   *   修复：
   *     - layer 内 Promise.all（启动后立即返回所有 Promise）
   *     - jobRun 创建后立即 push 到 run.jobs（UI 立刻可见）
   *     - executeJob 内部用 stepRunner 队列 + max 并发（按 jobDef.strategy.max）
   */
  async function runWorkflow(defId: string, triggeredBy: TriggeredBy): Promise<WorkflowRun> {
    const def = store.getDefinition(defId)
    if (!def) throw new Error(`Workflow definition "${defId}" not found`)

    if (isRunning.value) throw new Error('A workflow is already running')

    isRunning.value = true

    // 1. 创建运行实例
    const now = new Date().toISOString()
    const run: WorkflowRun = {
      id: genId(),
      workflowDefId: defId,
      status: 'running',
      triggeredBy,
      createdAt: now,
      startedAt: now,
      jobs: [],
    }
    currentRun.value = run
    store.addRun(run)

    try {
      // 2. 解析 DAG
      const layers = resolveExecutionOrder(def.jobs)
      if (layers.length === 0) {
        run.status = 'success'
        run.completedAt = new Date().toISOString()
        run.durationMs = Date.now() - new Date(run.startedAt!).getTime()
        store.updateRun(run.id, run)
        return run
      }

      // 3. 按 layer 顺序执行（layer 内真正并行，job 立即 push）
      // 第一层所有 job 都是入度 0（无 needs）→ 全部并行启动
      for (const layerJobIds of layers) {
        const jobPromises = layerJobIds.map((jobId) => {
          const jobDef = def.jobs.find((j) => j.id === jobId)!
          // 🆕 修复 #3：jobRun 创建后立即 push 到 run.jobs（UI 立刻可见）
          const jobRun: JobRun = {
            id: genId(),
            jobDefId: jobDef.id,
            status: 'running',
            startedAt: new Date().toISOString(),
            steps: [],
          }
          run.jobs.push(jobRun)
          // 🆕 修复 #2：传 run.id（不是 jobRun.id），让所有 job 共享同一 runId
          return executeJob(jobDef, def.env ?? {}, jobRun, run.id).then(() => jobRun)
        })
        // 等待当前层所有 job 提交完毕（submit 完成，不等 WS 回调）
        await Promise.all(jobPromises)
      }

      // 如果没有 job 有 steps（空 workflow），直接标记完成
      if (run.jobs.every((j) => j.steps.length === 0)) {
        run.status = 'success'
        run.completedAt = new Date().toISOString()
        run.durationMs = Date.now() - new Date(run.startedAt!).getTime()
      }

      store.updateRun(run.id, run)
      return run
    } catch (e) {
      run.status = 'failure'
      run.completedAt = new Date().toISOString()
      store.updateRun(run.id, run)
      throw e
    } finally {
      isRunning.value = false
    }
  }

  /**
   * 执行单个 Job：展开 Steps 并按 strategy 限流提交。
   *
   * 🆕 2026-06-10 修复 #4：内部 stepRunner 队列 + max concurrency
   *   - 接受外部传入的 jobRun（runWorkflow 已 push 到 run.jobs）
   *   - 构造所有 stepRun 立即 push 到 jobRun.steps（UI 立刻可见）
   *   - 用并发限流执行（parallel.max / sequential 串行 / matrix 全展开）
   */
  async function executeJob(
    jobDef: JobDefinition,
    env: Record<string, string>,
    jobRun: JobRun,
    _runIdOverride?: string,  // 🆕 2026-06-10 修复 #2：workflow run 共享 runId（用于 group 分组）
  ): Promise<JobRun> {
    // 构建 continueOnError 映射（用于结论计算）
    const continueOnErrorMap = new Map<string, boolean>()
    for (const step of jobDef.steps) {
      continueOnErrorMap.set(step.id, step.continueOnError ?? false)
    }

    // 展开 matrix 或直接使用步骤列表
    let stepExecutions: Array<{ stepDef: StepDefinition; binding?: MatrixBinding }> = []

    if (isMatrixStrategy(jobDef.strategy)) {
      // 笛卡尔积展开
      const bindings = expandMatrix(jobDef.strategy)
      for (const binding of bindings) {
        for (const step of jobDef.steps) {
          stepExecutions.push({ stepDef: step, binding })
        }
      }
    } else {
      for (const step of jobDef.steps) {
        stepExecutions.push({ stepDef: step })
      }
    }

    // 评估条件 + 构造 stepRun 列表（同步 push 到 jobRun.steps，UI 立刻可见）
    type ExecutableStep = { stepDef: StepDefinition; binding?: MatrixBinding; stepRun: StepRun }
    const executableSteps: ExecutableStep[] = []
    let prevStatus: StepStatus | undefined

    for (const exec of stepExecutions) {
      const { stepDef, binding } = exec

      // 评估 if 条件
      if (stepDef.if) {
        const shouldExecute = evaluateCondition(stepDef.if, {
          previousStepStatus: prevStatus,
          vars: binding ? { ...env, ...binding } : env,
        })
        if (!shouldExecute) {
          jobRun.steps.push({
            id: genId(),
            stepDefId: stepDef.id,
            status: 'skipped',
            matrixVars: binding,
          })
          prevStatus = 'skipped'
          continue
        }
      }

      // 创建 StepRun
      const stepRun: StepRun = {
        id: genId(),
        stepDefId: stepDef.id,
        status: 'pending',
        startedAt: new Date().toISOString(),
        matrixVars: binding,
      }
      jobRun.steps.push(stepRun)
      executableSteps.push({ stepDef, binding, stepRun })
    }

    // 🆕 修复 #4：按 strategy 限流执行（默认 5 并发）
    const max = (jobDef.strategy && 'max' in jobDef.strategy) ? jobDef.strategy.max : 5

    // 🆕 2026-06-10 修复 #2：捕获 runId，让 recordTriggeredBy 关联到当前 run
    //   历史 bug：useTaskTrigger 只存 triggeredBy，没有 runId → Tasks.vue 按 triggeredBy 粗聚拢
    //     多个 run 的 task 会混在一起
    //   修复：把当前 runId 传给 recordTriggeredBy，让 Tasks.vue 能按 run 精确分组
    //   注意：优先用外部传入的 _runId（来自 runWorkflow / scheduleDependentJobs），
    //     否则回退到 jobRun.id（向后兼容）
    const _runId = _runIdOverride || jobRun.id

    // 工具：执行单个 step（提交 + 状态更新）
    const runOneStep = async (ex: ExecutableStep): Promise<void> => {
      const { stepDef, binding, stepRun } = ex
      try {
        // 提交任务到后端
        const action = applyEnvToAction(stepDef.action, env, binding)
        const task = await submitAction(action)
        stepRun.taskId = task.id
        stepRun.status = 'running'  // 已提交，等 WS 回调
        recordTriggeredBy(task.id, 'automation', _runId)
        // 🆕 2026-06-10 修复 v4：setTaskMetadata 让 task 对象本身带 triggeredBy + runId
        // 用途：useTasksList.applyTaskCreated 收到 WS 事件时能通过 taskId 找到元数据，
        //   merge 进 tasks.value 里的 task 对象 → displayedItems 直接读 t.triggeredBy / t.runId
        // 不依赖 localStorage（跨 session / 清空 localStorage 也不影响）
        setTaskMetadata(task.id, 'automation', _runId)
      } catch (e) {
        // 提交失败（网络错误、参数错误等）— 直接标记为 failure
        stepRun.status = 'failure'
        stepRun.error = e instanceof Error ? e.message : String(e)
        stepRun.errorAnalysis = analyzeError(stepRun.error!, { phase: 'submission' })
        stepRun.completedAt = new Date().toISOString()
        stepRun.durationMs = Date.now() - new Date(stepRun.startedAt!).getTime()
      }
    }

    // 限流执行器：N 个 worker 共享 cursor 轮转拉取
    let cursor = 0
    const worker = async (): Promise<void> => {
      while (cursor < executableSteps.length) {
        const idx = cursor++
        const ex = executableSteps[idx]
        if (ex) await runOneStep(ex)
      }
    }
    const workerCount = Math.min(max, executableSteps.length)
    await Promise.all(Array.from({ length: workerCount }, () => worker()))

    // 检查 Job 是否已经全部完成（没有 running/pending 的 step）
    const allDone = jobRun.steps.every((s) => isTerminalStep(s.status))
    if (allDone) {
      jobRun.conclusion = computeJobConclusion(jobRun.steps, continueOnErrorMap)
      jobRun.status = jobRun.conclusion === 'success' ? 'success' : 'failure'
      jobRun.completedAt = new Date().toISOString()
      jobRun.durationMs = jobRun.startedAt
        ? Date.now() - new Date(jobRun.startedAt).getTime()
        : 0
    }

    return jobRun
  }

  /**
   * 取消当前运行。
   */
  function cancelCurrentRun(): void {
    if (!currentRun.value || currentRun.value.status !== 'running') return

    // 将所有非终态的 step 标记为 cancelled
    for (const job of currentRun.value.jobs) {
      for (const step of job.steps) {
        if (!isTerminalStep(step.status)) {
          step.status = 'cancelled'
          step.completedAt = new Date().toISOString()
        }
      }
      // 重新计算 job 结论
      if (job.status === 'running' || job.status === 'queued' || job.status === 'pending') {
        // 取消时所有未完成的都算 cancelled
        job.conclusion = 'cancelled'
        job.status = 'cancelled'
        job.completedAt = new Date().toISOString()
      }
    }

    currentRun.value.status = 'cancelled'
    currentRun.value.completedAt = new Date().toISOString()
    store.updateRun(currentRun.value.id, currentRun.value)
    isRunning.value = false
  }

  // ==================== 内部辅助 ====================

  /**
   * 检查一个 Job 是否全部完成，更新其 conclusion。
   */
  function checkJobCompletion(job: JobRun): void {
    const allTerminal = job.steps.every((s) => isTerminalStep(s.status))
    if (!allTerminal) return

    // 构建 continueOnError 映射（从 definition 获取，这里用简化逻辑）
    const coMap = new Map<string, boolean>()
    // 从 currentRun 的 definition 中获取...
    if (currentRun.value) {
      const def = store.getDefinition(currentRun.value.workflowDefId)
      if (def) {
        const jobDef = def.jobs.find((j) => j.id === job.jobDefId)
        if (jobDef) {
          for (const step of jobDef.steps) {
            coMap.set(step.id, step.continueOnError ?? false)
          }
        }
      }
    }

    job.conclusion = computeJobConclusion(job.steps, coMap)
    job.status = job.conclusion === 'success' ? 'success' : 'failure'
    job.completedAt = new Date().toISOString()
    job.durationMs = job.startedAt
      ? Date.now() - new Date(job.startedAt).getTime()
      : 0

    // 派发下一批依赖此 job 的 Jobs
    scheduleDependentJobs(job.jobDefId)
  }

  /**
   * 当一个 Job 完成后，检查并启动依赖它的下游 Jobs。
   */
  function scheduleDependentJobs(_completedJobDefId: string): void {
    if (!currentRun.value || currentRun.value.status !== 'running') return

    const def = store.getDefinition(currentRun.value.workflowDefId)
    if (!def) return

    const completedJobIds = new Set(
      currentRun.value.jobs
        .filter((j) => isTerminalStep(j.status))
        .map((j) => j.jobDefId),
    )

    const readyIds = getNextReadyJobs(def.jobs, completedJobIds)

    for (const readyId of readyIds) {
      // 检查是否已经有这个 job 的 Run
      const existing = currentRun.value.jobs.find((j) => j.jobDefId === readyId)
      if (existing) continue

      // 启动新的 Job
      const jobDef = def.jobs.find((j) => j.id === readyId)!
      // 🆕 修复 #3：先构造 jobRun 立即 push，再调 executeJob
      const jobRun: JobRun = {
        id: genId(),
        jobDefId: jobDef.id,
        status: 'running',
        startedAt: new Date().toISOString(),
        steps: [],
      }
      currentRun.value!.jobs.push(jobRun)
      // 🆕 修复 #2：传 currentRun.value.id（同 run 的所有 job 共享）
      executeJob(jobDef, def.env ?? {}, jobRun, currentRun.value!.id).then(() => {
        store.updateRun(currentRun.value!.id, currentRun.value!)
      })
    }
  }

  /**
   * 检查整个 Workflow 是否全部完成。
   */
  function checkWorkflowCompletion(): void {
    if (!currentRun.value) return
    const status = inferWorkflowStatus(currentRun.value.jobs)
    if (status !== 'running') {
      currentRun.value.status = status
      currentRun.value.completedAt = new Date().toISOString()
      currentRun.value.durationMs = currentRun.value.startedAt
        ? Date.now() - new Date(currentRun.value.startedAt!).getTime()
        : 0
      isRunning.value = false
    }
  }

  // ==================== Action 提交 ====================

  /**
   * 将 ActionSpec 提交为 EncvTask。
   */
  async function submitAction(action: ActionSpec): Promise<EncvTask> {
    if (action.type !== 'encv_task') {
      throw new Error(`Unsupported action type: ${action.type}`)
    }
    const spec = action as EncvTaskActionSpec
    return createTask(
      spec.taskType,
      spec.params.sourcePath ?? '',
      spec.params.targetPath,
      spec.params.password ?? '',
      spec.params.version,
      spec.pluginName,
      // 🆕 2026-06-10：把 spec.params.extraFields 透传给后端 createTask
      // 历史 bug：遍历 ExtraFields 生成的 case 完全丢失了加密选项（fn_rounds / stream_preset / 等）
      spec.params.extraFields ?? {},
      spec.params.secondaryPassword,
      spec.params.cipherMode,
      spec.params.compressionMode,
    )
  }

  /**
   * 将 env 变量和 matrix 绑定注入到 ActionSpec 中。
   */
  function applyEnvToAction(
    action: ActionSpec,
    env: Record<string, string>,
    binding?: Record<string, string>,
  ): ActionSpec {
    if (action.type !== 'encv_task') return action

    const mergedVars = { ...env, ...binding }
    const params = { ...action.params }

    // 简单变量替换：对字符串类型的 params 做模板替换
    for (const [key, val] of Object.entries(params)) {
      if (typeof val === 'string' && val.includes('${{')) {
        ;(params as any)[key] = val.replace(/\$\{\{\s*(\w+)\s*\}\}/g, (_m, v) => mergedVars[v] ?? val)
      }
    }

    return { ...action, params }
  }

  return {
    // Store 透传
    definitions: store.definitions,
    runs: store.runs,
    createDefinition: store.createDefinition,
    updateDefinition: store.updateDefinition,
    deleteDefinition: store.deleteDefinition,
    getDefinition: store.getDefinition,
    registerBuiltinTemplates: store.registerBuiltinTemplates,

    // 执行
    isRunning,
    currentRun,
    totalSteps,
    completedSteps,
    successSteps,
    failedSteps,
    runWorkflow,
    cancelCurrentRun,

    // 生命周期
    startListening,
    stopListening,
  }
}
