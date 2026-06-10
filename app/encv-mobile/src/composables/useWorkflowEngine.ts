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
import { recordTriggeredBy, type TriggeredBy } from '@/composables/useTaskTrigger'
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

  function onTaskCompleted(data: { id: string; error?: string }) {
    if (!currentRun.value || currentRun.value.status !== 'running') return

    // 在所有 jobs/steps 中查找匹配的 taskId
    let found = false
    for (const job of currentRun.value.jobs) {
      for (const step of job.steps) {
        if (step.taskId !== data.id || step.status !== 'running') continue
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
    }
  }

  function startListening() {
    eventBus.on('task:completed', onTaskCompleted)
  }

  function stopListening() {
    eventBus.off('task:completed', onTaskCompleted)
  }

  // ==================== 执行核心 ====================

  /**
   * 触发一次工作流运行。
   *
   * 流程：
   * 1. 创建 WorkflowRun 实例
   * 2. 解析 DAG → 分层执行计划
   * 3. 按层依次派发就绪的 Job
   * 4. 每个 Job 内按 strategy 展开 Steps 并提交到后端
   * 5. WS 回调驱动后续状态转移
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

      // 3. 按 layer 顺序执行
      const completedJobIds = new Set<string>()

      for (const layerJobIds of layers) {
        // 为当前层的每个 Job 创建 JobRun
        const layerJobRuns: JobRun[] = []

        for (const jobId of layerJobIds) {
          const jobDef = def.jobs.find((j) => j.id === jobId)!
          const jobRun = await executeJob(jobDef, def.env ?? {})
          run.jobs.push(jobRun)
          layerJobRuns.push(jobRun)
          completedJobIds.add(jobId)
        }

        // 等待当前层所有 Job 完成（非 matrix/sequential 的 job 可能立即完成）
        // 注意：实际完成依赖 WS 回调，这里只做初始派发
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
   * 执行单个 Job：展开 Steps 并逐个或并行提交。
   */
  async function executeJob(
    jobDef: JobDefinition,
    env: Record<string, string>,
  ): Promise<JobRun> {
    const jobRun: JobRun = {
      id: genId(),
      jobDefId: jobDef.id,
      status: 'running',
      startedAt: new Date().toISOString(),
      steps: [],
    }

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

    // 评估条件 + 执行每个 step
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

      try {
        // 提交任务到后端
        const action = applyEnvToAction(stepDef.action, env, binding)
        const task = await submitAction(action)
        stepRun.taskId = task.id
        stepRun.status = 'running'  // 已提交，等 WS 回调
        recordTriggeredBy(task.id, 'automation')
      } catch (e) {
        // 提交失败（网络错误、参数错误等）— 直接标记为 failure
        stepRun.status = 'failure'
        stepRun.error = e instanceof Error ? e.message : String(e)
        stepRun.errorAnalysis = analyzeError(stepRun.error!, { phase: 'submission' })
        stepRun.completedAt = new Date().toISOString()
        stepRun.durationMs = Date.now() - new Date(stepRun.startedAt!).getTime()
      }

      jobRun.steps.push(stepRun)
      if (isTerminalStep(stepRun.status)) {
        prevStatus = stepRun.status
      }
    }

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
      executeJob(jobDef, def.env ?? {}).then((jobRun) => {
        currentRun.value!.jobs.push(jobRun)
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
      {},
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
