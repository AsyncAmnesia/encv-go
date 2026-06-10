/**
 * 自动化测试 composable
 *
 * 提供：
 * - loadPlugins(): 从后端拉取 plugin 列表
 * - generateTestCases(): 动态笛卡尔积生成测试用例
 * - runTests(): 顺序提交任务到后端
 *
 * 核心设计：零硬编码 cipher mode / compression / version
 * - version 从 plugin.taskOptions.supportedVersions 派生
 * - v4 才有 cipherMode (0/1) × compressionMode (none/zstd) 的笛卡尔积
 * - v2/v3 不带 cipher/compression
 * - 默认源文件是 01-plain-media/video/sample.mp4（跨 plugin 通用）
 *
 * 真机安全：所有 source 走 withSafetyBoundary({ forceAutomation: true })
 * 强制改写到 /storage/emulated/0/encv-automation/* 命名空间。
 *
 * 触发者标签：通过 recordTriggeredBy 登记 'automation'，Tasks.vue badge 自动显示。
 */
import { ref } from 'vue'
import {
  createTask,
  fetchPlugins,
  type PluginMeta,
  type TaskType,
  type EncvTask,
} from '@/api/encv'
import { usePathResolver } from '@/composables/usePathResolver'
import { recordTriggeredBy, type TriggeredBy } from '@/composables/useTaskTrigger'
import { analyzeError, type ErrorAnalysis } from '@/composables/useErrorAnalyzer'

export type { TriggeredBy }

export interface TestCaseSpec {
  id: string
  taskType: TaskType
  pluginName: string
  sourcePath: string
  version: number
  cipherMode?: number
  compressionMode?: 'none' | 'zstd'
  expectedBehavior: 'success' | 'might-fail'
}

export interface TestCaseResult {
  spec: TestCaseSpec
  status: 'pending' | 'running' | 'passed' | 'failed' | 'skipped'
  taskId?: string
  error?: string
  durationMs?: number
  /** 错误分析（仅 status === 'failed' 时有值） */
  errorAnalysis?: ErrorAnalysis
  /** 提交时的快照（sourcePath, version, cipher, compression） */
  submittedSourcePath?: string
  submittedAt?: string
}

export interface TestProgress {
  total: number
  completed: number
  passed: number
  failed: number
  /** 跳过用例数（暂未启用，未来给 might-fail + 已知不支持的版本使用） */
  skipped: number
}

export interface GenerateTestCaseOptions {
  sourceFile: string
  includeDeprecated?: boolean
}

export const DEFAULT_AUTOMATION_SOURCE = '/storage/emulated/0/encv-automation/01-plain-media/video/sample.mp4'

export function useAutomationTests() {
  const { withSafetyBoundary } = usePathResolver()
  const plugins = ref<PluginMeta[]>([])
  const isLoadingPlugins = ref(false)
  const isRunning = ref(false)
  const progress = ref<TestProgress>({ total: 0, completed: 0, passed: 0, failed: 0, skipped: 0 })
  const results = ref<TestCaseResult[]>([])
  const lastError = ref<string | null>(null)
  const testCases = ref<TestCaseSpec[]>([])

  async function loadPlugins(): Promise<void> {
    isLoadingPlugins.value = true
    lastError.value = null
    try {
      plugins.value = await fetchPlugins()
    } catch (e) {
      lastError.value = e instanceof Error ? e.message : String(e)
    } finally {
      isLoadingPlugins.value = false
    }
  }

  /**
   * 动态笛卡尔积生成测试用例
   *
   * 遍历每个 plugin × taskType × versions × (v4 cipher × compression)
   * 零硬编码 cipher mode / compression / version——从 PluginMeta.taskOptions 派生。
   */
  function generateTestCases(opts: GenerateTestCaseOptions): TestCaseSpec[] {
    const cases: TestCaseSpec[] = []
    for (const plugin of plugins.value) {
      const pluginOpts = plugin.taskOptions
      if (!pluginOpts) continue

      const versions: number[] =
        pluginOpts.supportVersionSelect && pluginOpts.supportedVersions
          ? pluginOpts.supportedVersions
          : [pluginOpts.defaultVersion]

      for (const taskType of ['encrypt', 'decrypt'] as const) {
        for (const version of versions) {
          // includeDeprecated 默认 true：包含 v2/v3（用于回归）
          // 用户关闭后跳过这些版本
          if (opts.includeDeprecated === false && version <= 3) continue

          const isV4 = version === 4
          // v4 encrypt 才有 cipher + compression 笛卡尔积
          // decrypt 不需要（解密时由文件头决定）
          const cipherModes: Array<number | undefined> =
            isV4 && taskType === 'encrypt' ? [0, 1] : [undefined]
          const compressionModes: Array<'none' | 'zstd' | undefined> =
            isV4 && taskType === 'encrypt' ? ['none', 'zstd'] : [undefined]

          for (const cipherMode of cipherModes) {
            for (const compressionMode of compressionModes) {
              const idParts = [
                plugin.name,
                taskType,
                `v${version}`,
                cipherMode !== undefined ? `c${cipherMode}` : '',
                compressionMode !== undefined ? compressionMode : '',
              ].filter(Boolean)
              cases.push({
                id: idParts.join('-'),
                taskType,
                pluginName: plugin.name,
                sourcePath: opts.sourceFile,
                version,
                cipherMode,
                compressionMode,
                expectedBehavior: version <= 3 ? 'might-fail' : 'success',
              })
            }
          }
        }
      }
    }
    testCases.value = cases
    return cases
  }

  /**
   * 顺序执行所有测试用例，逐个提交任务。
   * 每个用例独立错误隔离：一个失败不影响其他。
   */
  async function runTests(specs: TestCaseSpec[]): Promise<void> {
    isRunning.value = true
    progress.value = { total: specs.length, completed: 0, passed: 0, failed: 0, skipped: 0 }
    results.value = []

    for (const spec of specs) {
      const result: TestCaseResult = {
        spec,
        status: 'running',
        submittedAt: new Date().toISOString(),
      }
      results.value = [...results.value, result]
      const start = Date.now()

      try {
        // 真机安全：强制改写到 encv-automation 命名空间
        const safeSource = withSafetyBoundary(spec.sourcePath, { forceAutomation: true })
        result.submittedSourcePath = safeSource
        const task: EncvTask = await createTask(
          spec.taskType,
          safeSource,
          undefined, // targetPath 让后端决定
          'automation-test-pwd', // 全局 password
          spec.version,
          spec.pluginName,
          {},
          undefined, // secondaryPassword
          spec.cipherMode,
          spec.compressionMode,
        )
        recordTriggeredBy(task.id, 'automation')
        result.taskId = task.id
        result.status = 'pending' // 任务已提交，等 WS 回调
        result.durationMs = Date.now() - start
        progress.value.passed++
      } catch (e) {
        result.status = 'failed'
        const errMsg = e instanceof Error ? e.message : String(e)
        result.error = errMsg
        // 调用错误分析器生成结构化错误链路 + 修复建议
        result.errorAnalysis = analyzeError(errMsg, { phase: 'submission' })
        result.durationMs = Date.now() - start
        progress.value.failed++
      }

      progress.value.completed++
    }

    isRunning.value = false
  }

  function reset(): void {
    isRunning.value = false
    progress.value = { total: 0, completed: 0, passed: 0, failed: 0, skipped: 0 }
    results.value = []
    testCases.value = []
    lastError.value = null
  }

  return {
    plugins,
    isLoadingPlugins,
    isRunning,
    progress,
    results,
    lastError,
    testCases,
    loadPlugins,
    generateTestCases,
    runTests,
    reset,
  }
}
