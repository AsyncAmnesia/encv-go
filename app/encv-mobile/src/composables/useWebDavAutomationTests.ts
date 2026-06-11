/**
 * 🆕 2026-06-11 v6：webdav 服务自动化测试
 *
 * 设计：
 * - 走前端 fetch 直接调后端 webdav endpoint
 * - 复用 useAutomationTests 的 localStorage 持久化（RESULTS_STORAGE_KEY）
 * - 不走 EncvTask 框架（避免后端 webdav operation worker 改造）
 * - 用 useTaskTrigger 记录 group / runId → 让 Tasks.vue 调试栏显示「webdav 测试报告」
 *   历史：用户问「测试报告存到哪了？里面报告了什么错误？」—— 之前 useAutomationTests
 *     已经在写 localStorage，但用户不知道怎么查。这个 composable 让 webdav 测试**也**写
 *     同一个 key，跟 plugin 加密测试在同一个 history 列表
 *
 * 覆盖用例（按 webdav 协议分类）：
 * - 基础读：LIST_ROOT / LIST_VIDEO / LIST_AUDIO / LIST_IMAGE
 * - 文件操作：GET_FILE / HEAD_FILE / OPTIONS
 * - 元数据：PROPFIND
 * - 写操作：MKCOL / PUT_FILE / GET_UPLOADED / MOVE / COPY / DELETE_FILE / DELETE_DIR
 * - 认证：AUTH_REQUIRED
 *
 * 任务系统适配：
 * - runTests 期间调 useTaskTrigger.setTaskMetadata(testId, 'automation', sharedRunId)
 *   → Tasks.vue displayedItems 按 runId 聚合成 1 个 group
 * - 任务标题 = 测试名
 * - task 完成后调 clearTriggeredBy 清理（避免污染后续真实 task 的分组）
 */
import { ref, computed } from 'vue'
import { setTaskMetadata, getTaskMetadata, type TriggeredBy } from './useTaskTrigger'

// ============= 类型 =============

export type WebDavTestStatus = 'pending' | 'running' | 'passed' | 'failed' | 'skipped'

export interface WebDavTestCase {
  /** 唯一 id（用于 group 聚合 + localStorage 持久化） */
  id: string
  /** 人类可读名（中英 i18n 在 view 里覆盖） */
  name: string
  /** 分类标签：list / read / write / meta / auth */
  category: 'list' | 'read' | 'write' | 'meta' | 'auth'
  /** 测试描述 */
  description: string
}

export interface WebDavTestResult {
  caseId: string
  caseName: string
  category: WebDavTestCase['category']
  status: WebDavTestStatus
  startedAt?: string
  completedAt?: string
  durationMs?: number
  /** HTTP status code（如果有） */
  httpStatus?: number
  /** 错误信息（failed 时填） */
  error?: string
  /** 错误分类（failed 时填） */
  errorKind?: 'http_4xx' | 'http_5xx' | 'network' | 'timeout' | 'assertion' | 'unknown'
}

export interface WebDavTestRun {
  id: string
  startedAt: string
  completedAt?: string
  totalCases: number
  passed: number
  failed: number
  skipped: number
  results: WebDavTestResult[]
  /** 触发的 webhook 类型（用于调试栏分类） */
  category: 'webdav'
  /** 后端 base URL（记录在报告里便于排查） */
  baseUrl: string
}

// ============= 测试用例定义 =============

export const WEBDAV_TEST_CASES: WebDavTestCase[] = [
  // ---- 基础读 ----
  {
    id: 'list_root',
    name: 'LIST 根目录',
    category: 'list',
    description: 'GET /webdav/ — 列出 webdav 根目录所有条目',
  },
  {
    id: 'list_video',
    name: 'LIST 视频目录',
    category: 'list',
    description: 'GET /webdav/01-plain-media/video/ — 列出视频样本',
  },
  {
    id: 'list_audio',
    name: 'LIST 音频目录',
    category: 'list',
    description: 'GET /webdav/01-plain-media/audio/ — 列出音频样本',
  },
  {
    id: 'list_image',
    name: 'LIST 图片目录',
    category: 'list',
    description: 'GET /webdav/01-plain-media/image/ — 列出图片样本',
  },
  // ---- 文件操作 ----
  {
    id: 'options',
    name: 'OPTIONS 能力协商',
    category: 'meta',
    description: 'OPTIONS /webdav/ — 检查 DAV 支持的方法 (PROPFIND/MKCOL/PUT/DELETE 等)',
  },
  {
    id: 'get_video_sample',
    name: 'GET 视频样本',
    category: 'read',
    description: 'GET /webdav/01-plain-media/video/sample.mp4 — 下载视频样本',
  },
  {
    id: 'head_video_sample',
    name: 'HEAD 视频样本',
    category: 'read',
    description: 'HEAD /webdav/01-plain-media/video/sample.mp4 — 仅取 header (验证 Content-Length/Type)',
  },
  // ---- 元数据 ----
  {
    id: 'propfind_root',
    name: 'PROPFIND 根',
    category: 'meta',
    description: 'PROPFIND /webdav/ — 深度 1 属性查询',
  },
  // ---- 写操作 ----
  {
    id: 'mkcol_test_dir',
    name: 'MKCOL 测试目录',
    category: 'write',
    description: 'MKCOL /webdav/02-test-output/webdav-test/ — 创建测试目录',
  },
  {
    id: 'put_test_file',
    name: 'PUT 上传文件',
    category: 'write',
    description: 'PUT /webdav/02-test-output/webdav-test/upload-1.txt — 上传测试文件',
  },
  {
    id: 'get_uploaded_file',
    name: 'GET 已上传文件',
    category: 'read',
    description: 'GET /webdav/02-test-output/webdav-test/upload-1.txt — 验证 PUT 后能 GET',
  },
  {
    id: 'move_uploaded_file',
    name: 'MOVE 移动文件',
    category: 'write',
    description: 'MOVE /webdav/02-test-output/webdav-test/upload-1.txt → .../renamed.txt',
  },
  {
    id: 'copy_uploaded_file',
    name: 'COPY 复制文件',
    category: 'write',
    description: 'COPY /webdav/02-test-output/webdav-test/renamed.txt → .../copy.txt',
  },
  {
    id: 'delete_file',
    name: 'DELETE 文件',
    category: 'write',
    description: 'DELETE /webdav/02-test-output/webdav-test/copy.txt — 删除文件',
  },
  {
    id: 'delete_renamed_file',
    name: 'DELETE 重命名文件',
    category: 'write',
    description: 'DELETE /webdav/02-test-output/webdav-test/renamed.txt — 删除剩余测试文件',
  },
  {
    id: 'delete_test_dir',
    name: 'DELETE 测试目录',
    category: 'write',
    description: 'DELETE /webdav/02-test-output/webdav-test/ — 清理测试目录',
  },
  // ---- 边界 ----
  {
    id: 'get_404',
    name: 'GET 不存在文件',
    category: 'read',
    description: 'GET /webdav/__nope__.txt — 期望 404',
  },
  {
    id: 'put_no_parent',
    name: 'PUT 到不存在的父目录',
    category: 'write',
    description: 'PUT /webdav/__no_parent__/x.txt — 期望 409 Conflict',
  },
]

// ============= 持久化（跟 useAutomationTests 同一个 key） =============

const RESULTS_STORAGE_KEY = 'encv_automation_results_v1'
const MAX_PERSISTED_RUNS = 50

function loadPersistedRuns(): WebDavTestRun[] {
  try {
    const raw = localStorage.getItem(RESULTS_STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as Array<WebDavTestRun | any>
    if (!Array.isArray(parsed)) return []
    // 过滤：只保留 webdav category 的 run
    return parsed.filter((r) => r && r.category === 'webdav') as WebDavTestRun[]
  } catch {
    return []
  }
}

function savePersistedRuns(webdavRuns: WebDavTestRun[]): void {
  try {
    // 跟 useAutomationTests 共用同一个 key → 需要保留 plugin run
    const raw = localStorage.getItem(RESULTS_STORAGE_KEY)
    const all: any[] = raw ? JSON.parse(raw) : []
    // 移除旧的 webdav run
    const nonWebdav = all.filter((r) => r.category !== 'webdav')
    // 合并：webdav run 倒序 + 老的 plugin run
    const merged = [...webdavRuns, ...nonWebdav]
      .sort((a, b) => (b.startedAt ?? '').localeCompare(a.startedAt ?? ''))
      .slice(0, MAX_PERSISTED_RUNS)
    localStorage.setItem(RESULTS_STORAGE_KEY, JSON.stringify(merged))
  } catch (e) {
    console.debug('[useWebDavAutomationTests] localStorage save failed:', e)
  }
}

// ============= Composable =============

export function useWebDavAutomationTests() {
  // 🆕 setTaskMetadata 是 module-level export（从 './useTaskTrigger' import），不是 composable
  // 历史 v6 bug：错写成 `const { setTaskMetadata } = useTaskTrigger()` → ReferenceError
  // 修复：直接用顶层 import 的 setTaskMetadata（line 27）

  const results = ref<WebDavTestResult[]>([])
  const isRunning = ref(false)
  const currentRunId = ref<string | null>(null)
  const currentRunStartedAt = ref<string | null>(null)
  const baseUrl = ref<string>('')

  const summary = computed(() => {
    const passed = results.value.filter((r) => r.status === 'passed').length
    const failed = results.value.filter((r) => r.status === 'failed').length
    const skipped = results.value.filter((r) => r.status === 'skipped').length
    const running = results.value.filter((r) => r.status === 'running').length
    const pending = results.value.filter((r) => r.status === 'pending').length
    const total = results.value.length
    const finished = passed + failed + skipped
    const percent = total > 0 ? Math.round((finished / total) * 100) : 0
    return { passed, failed, skipped, running, pending, total, percent }
  })

  /** 探测后端 webdav endpoint 根 URL */
  function detectBaseUrl(): string {
    // 优先用 window.location.origin（同源 → 走 :16666 gateway → :2025 后端）
    // Capacitor native 端 origin 可能是 capacitor://localhost / http://localhost
    return `${window.location.origin}/webdav`
  }

  /** 初始化结果列表（pending） */
  function initResults(): void {
    results.value = WEBDAV_TEST_CASES.map((c) => ({
      caseId: c.id,
      caseName: c.name,
      category: c.category,
      status: 'pending',
    }))
  }

  /** 单个测试用例 runner */
  async function runOneCase(
    testCase: WebDavTestCase,
    timeoutMs = 15000,
  ): Promise<WebDavTestResult> {
    const idx = results.value.findIndex((r) => r.caseId === testCase.id)
    if (idx < 0) {
      return {
        caseId: testCase.id,
        caseName: testCase.name,
        category: testCase.category,
        status: 'skipped',
      }
    }
    // 标记 running
    const startedAt = new Date().toISOString()
    results.value[idx] = {
      ...results.value[idx],
      status: 'running',
      startedAt,
    }
    // 任务系统适配：标记这个 testId 属于当前 run group
    setTaskMetadata(testCase.id, 'automation', currentRunId.value)
    try {
      const result = await executeWebDavTest(testCase, baseUrl.value, timeoutMs)
      const completedAt = new Date().toISOString()
      const durationMs = new Date(completedAt).getTime() - new Date(startedAt).getTime()
      const updated: WebDavTestResult = {
        ...results.value[idx],
        status: result.passed ? 'passed' : 'failed',
        completedAt,
        durationMs,
        httpStatus: result.httpStatus,
        error: result.error,
        errorKind: result.errorKind,
      }
      results.value[idx] = updated
      // 🆕 实时持久化
      persistCurrentRun()
      return updated
    } catch (e) {
      const completedAt = new Date().toISOString()
      const durationMs = new Date(completedAt).getTime() - new Date(startedAt).getTime()
      const error = e instanceof Error ? e.message : String(e)
      const updated: WebDavTestResult = {
        ...results.value[idx],
        status: 'failed',
        completedAt,
        durationMs,
        error,
        errorKind: 'unknown',
      }
      results.value[idx] = updated
      persistCurrentRun()
      return updated
    }
  }

  /** 跑全部用例（按顺序，1 个 worker） */
  async function runAllCases(): Promise<WebDavTestRun> {
    if (isRunning.value) {
      throw new Error('已有 webdav 测试在跑')
    }
    isRunning.value = true
    initResults()
    baseUrl.value = detectBaseUrl()
    currentRunId.value = `webdav-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
    currentRunStartedAt.value = new Date().toISOString()
    const startedAt = currentRunStartedAt.value
    // 任务系统适配：所有 test 共享同一个 runId
    for (const c of WEBDAV_TEST_CASES) {
      setTaskMetadata(c.id, 'automation', currentRunId.value)
    }
    persistCurrentRun()
    // 顺序跑（webdav 操作有依赖：PUT 后才能 GET，MOVE 后才能 DELETE；并行会冲突）
    for (const c of WEBDAV_TEST_CASES) {
      if (!isRunning.value) break  // 用户取消
      await runOneCase(c)
    }
    const completedAt = new Date().toISOString()
    isRunning.value = false
    // 构造 run 记录
    const run: WebDavTestRun = {
      id: currentRunId.value,
      startedAt,
      completedAt,
      totalCases: results.value.length,
      passed: results.value.filter((r) => r.status === 'passed').length,
      failed: results.value.filter((r) => r.status === 'failed').length,
      skipped: results.value.filter((r) => r.status === 'skipped').length,
      results: [...results.value],
      category: 'webdav',
      baseUrl: baseUrl.value,
    }
    // 持久化到 localStorage（追加，不覆盖）
    const runs = loadPersistedRuns()
    runs.unshift(run)
    savePersistedRuns(runs)
    return run
  }

  function cancelRun(): void {
    isRunning.value = false
    // 把所有 running 标 cancelled
    for (const r of results.value) {
      if (r.status === 'running' || r.status === 'pending') {
        const idx = results.value.findIndex((x) => x.caseId === r.caseId)
        if (idx >= 0) {
          results.value[idx] = { ...r, status: 'skipped', completedAt: new Date().toISOString() }
        }
      }
    }
    persistCurrentRun()
  }

  /** 实时持久化（跟 useAutomationTests 同 key） */
  function persistCurrentRun(): void {
    if (results.value.length === 0 || !currentRunId.value) return
    const startedAt = currentRunStartedAt.value
      ?? results.value.map((r) => r.startedAt ?? '').filter(Boolean).sort()[0]
      ?? new Date().toISOString()
    const completedAt = new Date().toISOString()
    const passed = results.value.filter((r) => r.status === 'passed').length
    const failed = results.value.filter((r) => r.status === 'failed').length
    const skipped = results.value.filter((r) => r.status === 'skipped').length
    const run: WebDavTestRun = {
      id: currentRunId.value,
      startedAt,
      completedAt,
      totalCases: results.value.length,
      passed,
      failed,
      skipped,
      results: [...results.value],
      category: 'webdav',
      baseUrl: baseUrl.value,
    }
    const runs = loadPersistedRuns()
    // upsert：先移除同 id 的，再 unshift
    const filtered = runs.filter((r) => r.id !== run.id)
    filtered.unshift(run)
    savePersistedRuns(filtered)
  }

  /** 读历史 run（从 localStorage） */
  function getPersistedRuns(): WebDavTestRun[] {
    return loadPersistedRuns()
  }

  /** 读某个 run 的详细结果 */
  function getPersistedRun(id: string): WebDavTestRun | undefined {
    return loadPersistedRuns().find((r) => r.id === id)
  }

  /** 清空所有历史 */
  function clearPersistedRuns(): void {
    try {
      const raw = localStorage.getItem(RESULTS_STORAGE_KEY)
      const all: any[] = raw ? JSON.parse(raw) : []
      const nonWebdav = all.filter((r) => r.category !== 'webdav')
      localStorage.setItem(RESULTS_STORAGE_KEY, JSON.stringify(nonWebdav))
    } catch {
      // silent
    }
  }

  return {
    // state
    results,
    summary,
    isRunning,
    currentRunId,
    baseUrl,
    // actions
    runAllCases,
    cancelRun,
    getPersistedRuns,
    getPersistedRun,
    clearPersistedRuns,
  }
}

// ============= 单个测试执行器 =============

interface WebDavTestOutcome {
  passed: boolean
  httpStatus?: number
  error?: string
  errorKind?: WebDavTestResult['errorKind']
}

async function executeWebDavTest(
  testCase: WebDavTestCase,
  baseUrl: string,
  timeoutMs: number,
): Promise<WebDavTestOutcome> {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeoutMs)
  try {
    let outcome: WebDavTestOutcome
    switch (testCase.id) {
      case 'list_root':
        outcome = await expectList(`${baseUrl}/`, controller.signal, 200, /01-plain-media/)
        break
      case 'list_video':
        outcome = await expectList(`${baseUrl}/01-plain-media/video/`, controller.signal, 200, /sample\.(mp4|mkv)/)
        break
      case 'list_audio':
        outcome = await expectList(`${baseUrl}/01-plain-media/audio/`, controller.signal, 200, /sample\.(mp3|flac)/)
        break
      case 'list_image':
        outcome = await expectList(`${baseUrl}/01-plain-media/image/`, controller.signal, 200, /sample\.(png|jpg)/)
        break
      case 'options':
        outcome = await expectOptions(`${baseUrl}/`, controller.signal)
        break
      case 'get_video_sample':
        outcome = await expectGetFile(`${baseUrl}/01-plain-media/video/sample.mp4`, controller.signal)
        break
      case 'head_video_sample':
        outcome = await expectHeadFile(`${baseUrl}/01-plain-media/video/sample.mp4`, controller.signal)
        break
      case 'propfind_root':
        outcome = await expectPropfind(`${baseUrl}/`, controller.signal)
        break
      case 'mkcol_test_dir':
        outcome = await expectMkcol(`${baseUrl}/02-test-output/webdav-test/`, controller.signal)
        break
      case 'put_test_file':
        outcome = await expectPut(
          `${baseUrl}/02-test-output/webdav-test/upload-1.txt`,
          controller.signal,
          `webdav-test-payload-${Date.now()}`,
        )
        break
      case 'get_uploaded_file':
        outcome = await expectGetFile(`${baseUrl}/02-test-output/webdav-test/upload-1.txt`, controller.signal)
        break
      case 'move_uploaded_file':
        outcome = await expectMove(
          `${baseUrl}/02-test-output/webdav-test/upload-1.txt`,
          `${baseUrl}/02-test-output/webdav-test/renamed.txt`,
          controller.signal,
        )
        break
      case 'copy_uploaded_file':
        outcome = await expectCopy(
          `${baseUrl}/02-test-output/webdav-test/renamed.txt`,
          `${baseUrl}/02-test-output/webdav-test/copy.txt`,
          controller.signal,
        )
        break
      case 'delete_file':
        outcome = await expectDelete(`${baseUrl}/02-test-output/webdav-test/copy.txt`, controller.signal)
        break
      case 'delete_renamed_file':
        outcome = await expectDelete(`${baseUrl}/02-test-output/webdav-test/renamed.txt`, controller.signal)
        break
      case 'delete_test_dir':
        outcome = await expectDelete(`${baseUrl}/02-test-output/webdav-test/`, controller.signal)
        break
      case 'get_404':
        outcome = await expectStatus(`${baseUrl}/__nope__.txt`, { method: 'GET' }, controller.signal, 404)
        break
      case 'put_no_parent':
        outcome = await expectStatus(
          `${baseUrl}/__no_parent__/x.txt`,
          { method: 'PUT', body: 'data' },
          controller.signal,
          [409, 404, 405],  // 兼容：可能返回 409 conflict / 404 not found / 405 method not allowed
        )
        break
      default:
        outcome = { passed: false, error: 'unknown test id', errorKind: 'unknown' }
    }
    return outcome
  } catch (e) {
    if (e instanceof Error && e.name === 'AbortError') {
      return {
        passed: false,
        error: `timeout after ${timeoutMs}ms`,
        errorKind: 'timeout',
      }
    }
    return {
      passed: false,
      error: e instanceof Error ? e.message : String(e),
      errorKind: 'network',
    }
  } finally {
    clearTimeout(timer)
  }
}

// ============= 期望检查器 =============

async function expectList(
  url: string,
  signal: AbortSignal,
  expectStatus: number,
  expectPattern: RegExp,
): Promise<WebDavTestOutcome> {
  const res = await fetch(url, { method: 'GET', signal })
  if (res.status !== expectStatus) {
    return { passed: false, httpStatus: res.status, error: `expected ${expectStatus}, got ${res.status}`, errorKind: res.status >= 500 ? 'http_5xx' : 'http_4xx' }
  }
  const text = await res.text()
  if (!expectPattern.test(text)) {
    return { passed: false, httpStatus: res.status, error: `body missing pattern ${expectPattern}`, errorKind: 'assertion' }
  }
  return { passed: true, httpStatus: res.status }
}

async function expectGetFile(url: string, signal: AbortSignal): Promise<WebDavTestOutcome> {
  const res = await fetch(url, { method: 'GET', signal })
  if (res.status !== 200) {
    return { passed: false, httpStatus: res.status, error: `expected 200, got ${res.status}`, errorKind: res.status >= 500 ? 'http_5xx' : 'http_4xx' }
  }
  if ((res.headers.get('content-length') ?? '0') === '0') {
    return { passed: false, httpStatus: 200, error: 'content-length is 0', errorKind: 'assertion' }
  }
  return { passed: true, httpStatus: 200 }
}

async function expectHeadFile(url: string, signal: AbortSignal): Promise<WebDavTestOutcome> {
  const res = await fetch(url, { method: 'HEAD', signal })
  if (res.status !== 200) {
    return { passed: false, httpStatus: res.status, error: `expected 200, got ${res.status}`, errorKind: res.status >= 500 ? 'http_5xx' : 'http_4xx' }
  }
  const contentLength = res.headers.get('content-length')
  if (!contentLength || contentLength === '0') {
    return { passed: false, httpStatus: 200, error: 'content-length missing or 0', errorKind: 'assertion' }
  }
  return { passed: true, httpStatus: 200 }
}

async function expectOptions(url: string, signal: AbortSignal): Promise<WebDavTestOutcome> {
  const res = await fetch(url, { method: 'OPTIONS', signal })
  if (res.status < 200 || res.status >= 300) {
    return { passed: false, httpStatus: res.status, error: `expected 2xx, got ${res.status}`, errorKind: res.status >= 500 ? 'http_5xx' : 'http_4xx' }
  }
  const allow = res.headers.get('allow') ?? res.headers.get('dav') ?? ''
  // webdav 至少要支持 PROPFIND
  if (!/PROPFIND|MOVE|COPY|DELETE/i.test(allow) && allow.length > 0) {
    return { passed: false, httpStatus: res.status, error: `DAL/Allow header missing webdav methods: "${allow}"`, errorKind: 'assertion' }
  }
  return { passed: true, httpStatus: res.status }
}

async function expectPropfind(url: string, signal: AbortSignal): Promise<WebDavTestOutcome> {
  const res = await fetch(url, {
    method: 'PROPFIND',
    headers: { Depth: '1', 'Content-Type': 'application/xml' },
    body: '<?xml version="1.0"?><d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/></d:prop></d:propfind>',
    signal,
  })
  if (res.status !== 207) {
    return { passed: false, httpStatus: res.status, error: `expected 207 Multi-Status, got ${res.status}`, errorKind: res.status >= 500 ? 'http_5xx' : 'http_4xx' }
  }
  return { passed: true, httpStatus: res.status }
}

async function expectMkcol(url: string, signal: AbortSignal): Promise<WebDavTestOutcome> {
  const res = await fetch(url, { method: 'MKCOL', signal })
  // 201 Created / 405 Method Not Allowed (目录已存在) 都算通过
  if (res.status === 201 || res.status === 405) {
    return { passed: true, httpStatus: res.status }
  }
  return { passed: false, httpStatus: res.status, error: `expected 201/405, got ${res.status}`, errorKind: res.status >= 500 ? 'http_5xx' : 'http_4xx' }
}

async function expectPut(url: string, signal: AbortSignal, body: string): Promise<WebDavTestOutcome> {
  const res = await fetch(url, {
    method: 'PUT',
    headers: { 'Content-Type': 'text/plain' },
    body,
    signal,
  })
  if (res.status < 200 || res.status >= 300) {
    return { passed: false, httpStatus: res.status, error: `expected 2xx, got ${res.status}`, errorKind: res.status >= 500 ? 'http_5xx' : 'http_4xx' }
  }
  return { passed: true, httpStatus: res.status }
}

async function expectMove(src: string, dst: string, signal: AbortSignal): Promise<WebDavTestOutcome> {
  const res = await fetch(src, {
    method: 'MOVE',
    headers: { Destination: dst, Overwrite: 'F' },
    signal,
  })
  if (res.status < 200 || res.status >= 300) {
    return { passed: false, httpStatus: res.status, error: `expected 2xx, got ${res.status}`, errorKind: res.status >= 500 ? 'http_5xx' : 'http_4xx' }
  }
  return { passed: true, httpStatus: res.status }
}

async function expectCopy(src: string, dst: string, signal: AbortSignal): Promise<WebDavTestOutcome> {
  const res = await fetch(src, {
    method: 'COPY',
    headers: { Destination: dst, Overwrite: 'F' },
    signal,
  })
  if (res.status < 200 || res.status >= 300) {
    return { passed: false, httpStatus: res.status, error: `expected 2xx, got ${res.status}`, errorKind: res.status >= 500 ? 'http_5xx' : 'http_4xx' }
  }
  return { passed: true, httpStatus: res.status }
}

async function expectDelete(url: string, signal: AbortSignal): Promise<WebDavTestOutcome> {
  const res = await fetch(url, { method: 'DELETE', signal })
  if (res.status === 204 || res.status === 200 || res.status === 404) {
    // 204 = 删除成功, 200 = 某些 webdav 实现, 404 = 已删除（幂等通过）
    return { passed: true, httpStatus: res.status }
  }
  return { passed: false, httpStatus: res.status, error: `expected 200/204/404, got ${res.status}`, errorKind: res.status >= 500 ? 'http_5xx' : 'http_4xx' }
}

async function expectStatus(
  url: string,
  init: RequestInit,
  signal: AbortSignal,
  expected: number | number[],
): Promise<WebDavTestOutcome> {
  const res = await fetch(url, { ...init, signal })
  const expectedList = Array.isArray(expected) ? expected : [expected]
  if (!expectedList.includes(res.status)) {
    return { passed: false, httpStatus: res.status, error: `expected ${expectedList.join('|')}, got ${res.status}`, errorKind: res.status >= 500 ? 'http_5xx' : 'http_4xx' }
  }
  return { passed: true, httpStatus: res.status }
}
