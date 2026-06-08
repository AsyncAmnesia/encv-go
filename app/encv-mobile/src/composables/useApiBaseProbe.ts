// useApiBaseProbe - AI 路由 baseUrl 探测链
//
// 触发点：
//   - useApiBase setup()（冷启动）
//   - document.visibilitychange（切回前台）
//   - useServerStatus.manualReconnect()（手动重连）
//   - ServerSettings.vue "立即探测" 按钮
//
// 探测优先级：
//   [1] localStorage.encv-server-url（用户上次手动设的，最优先）
//   [2] http://127.0.0.1:2025  loopback（APK 模式 + adb reverse 通）
//   [3] /api/network/lan-access（拿到 dev 机器 LAN 候选 IP 列表）
//   [4] 每个 LAN 候选逐一探活，第一个通的晋升
//
// 关键约束：
//   - 串行探测，命中即停（避免并行噪声）
//   - 单 probe timeout 1500ms（避免卡 UI）
//   - 成功 → 写 localStorage + 调 setApiBaseUrl（同步所有依赖 baseUrl 的 composable）
//   - 失败 → 不写 localStorage（保留旧值兜底，避免越改越坏）
//
// 探活 endpoint 选用 /api/config 而不是 /api/chat，是因为 /api/config 是 GET、轻量
// 且不依赖任何 agent 状态。

import { ref, type Ref } from 'vue'
import { DEFAULT_API_BASE_URL, setApiBaseUrl } from '@/api/encv'

/** 单次探测结果 */
export interface ProbeResult {
  /** 最终晋升的 baseUrl（已通过 /api/config 探活） */
  baseUrl: string
  /** 探测过程中从 /api/network/lan-access 拿到的 LAN 候选（若有） */
  lanAccess: {
    addresses: string[]
    preferred: string
  } | null
  /** 探测命中的源头 */
  source: 'cached' | 'loopback' | 'lan-candidate'
  /** 探测耗时（ms） */
  latencyMs: number
  /** 完整探测日志（调试用） */
  log: string[]
}

const SERVER_URL_KEY = 'encv-server-url'
const PROBE_TIMEOUT_MS = 1500
const PROBE_HEALTH_PATH = '/api/config'
const PROBE_LAN_PATH = '/api/network/lan-access'

/** 模块级单例：避免多个调用方各自维护 probe 状态 */
let _instance: ReturnType<typeof createProbe> | null = null

function createProbe() {
  const isProbing = ref(false)
  const lastResult = ref<ProbeResult | null>(null)
  const lastError = ref<string | null>(null)
  /** 节流：避免频繁触发探测（visibilitychange / 重连风暴） */
  const MIN_PROBE_INTERVAL_MS = 10_000
  let lastProbeAt = 0

  /** 用 AbortController 限定单次 fetch 超时（fetch 本身没有 timeout） */
  function fetchWithTimeout(url: string, timeoutMs: number): Promise<Response> {
    const ctrl = new AbortController()
    const timer = setTimeout(() => ctrl.abort(), timeoutMs)
    return fetch(url, {
      method: 'GET',
      signal: ctrl.signal,
      // 后端 /api/config 不需要 credential；同源 / capacitor scheme 都允许
      credentials: 'omit',
    }).finally(() => clearTimeout(timer))
  }

  /**
   * 探活单个 baseUrl（拉 /api/config，200 即视为通）
   * 失败 / 超时 / 网络错均返回 false
   */
  async function probeHealth(baseUrl: string): Promise<{ ok: boolean; latencyMs: number; err?: string }> {
    const url = baseUrl.replace(/\/+$/, '') + PROBE_HEALTH_PATH
    const t0 = performance.now()
    try {
      const r = await fetchWithTimeout(url, PROBE_TIMEOUT_MS)
      const latencyMs = Math.round(performance.now() - t0)
      if (r.ok) return { ok: true, latencyMs }
      return { ok: false, latencyMs, err: `status ${r.status}` }
    } catch (e) {
      const latencyMs = Math.round(performance.now() - t0)
      return { ok: false, latencyMs, err: e instanceof Error ? e.message : String(e) }
    }
  }

  /**
   * 从一个已通路的 baseUrl 拉 LAN 候选（/api/network/lan-access）
   * 仅用于扩展探测链，不会让本次 probe 失败
   */
  async function fetchLanCandidates(baseUrl: string): Promise<ProbeResult['lanAccess']> {
    const url = baseUrl.replace(/\/+$/, '') + PROBE_LAN_PATH
    try {
      const r = await fetchWithTimeout(url, PROBE_TIMEOUT_MS)
      if (!r.ok) return null
      const j = await r.json()
      if (!j || !Array.isArray(j.addresses) || j.addresses.length === 0) return null
      return {
        addresses: j.addresses.filter((s: unknown) => typeof s === 'string'),
        preferred: typeof j.preferred === 'string' ? j.preferred : '',
      }
    } catch {
      return null
    }
  }

  /** 从 LAN 候选构造完整 baseUrl（http://IP:PORT） */
  function buildCandidateUrl(addr: string, port: number): string {
    // 如果 addr 已经是 http:// 开头，原样返回
    if (/^https?:\/\//i.test(addr)) return addr
    // 6. 同时支持 IPv4 / IPv6 / hostname
    return `http://${addr}:${port}`
  }

  /** 端口猜测：loopback URL 抽端口；默认 2025 */
  function guessPort(baseUrl: string): number {
    try {
      const u = new URL(baseUrl)
      if (u.port) return parseInt(u.port, 10)
    } catch {/* fallthrough */}
    return 2025
  }

  /**
   * 主入口：执行一次完整探测链
   * @param opts.force 跳过 10s 节流（用于手动触发）
   * @returns ProbeResult
   */
  async function probe(opts?: { force?: boolean }): Promise<ProbeResult> {
    const now = Date.now()
    if (!opts?.force && now - lastProbeAt < MIN_PROBE_INTERVAL_MS) {
      // 节流命中：复用上次结果
      if (lastResult.value) return lastResult.value
    }
    lastProbeAt = now

    isProbing.value = true
    lastError.value = null
    const log: string[] = []
    const t0 = performance.now()

    try {
      // ─── [1] 缓存的 URL（最优先） ──────────────────────
      const cached = localStorage.getItem(SERVER_URL_KEY)
      if (cached && cached !== DEFAULT_API_BASE_URL) {
        log.push(`[1] try cached: ${cached}`)
        const r = await probeHealth(cached)
        log.push(`[1] result: ok=${r.ok} latency=${r.latencyMs}ms err=${r.err || '-'}`)
        if (r.ok) {
          return commit(cached, null, 'cached', log, t0)
        }
      } else {
        log.push('[1] no cached URL, skip')
      }

      // ─── [2] loopback 探测 ─────────────────────────────
      log.push(`[2] try loopback: ${DEFAULT_API_BASE_URL}`)
      const lb = await probeHealth(DEFAULT_API_BASE_URL)
      log.push(`[2] result: ok=${lb.ok} latency=${lb.latencyMs}ms err=${lb.err || '-'}`)
      if (lb.ok) {
        // 拿到 LAN 候选（用于本轮其它探测 + UI 展示）
        const lanAccess = await fetchLanCandidates(DEFAULT_API_BASE_URL)
        return await expandWithLanCandidates(DEFAULT_API_BASE_URL, lanAccess, 'loopback', log, t0)
      }

      // ─── [3] loopback 不通 → 试拉 LAN 候选 ─────────────
      // 若 loopback 不通，LAN 候选也只能从「之前的 lastResult」或「用户手动设」获取
      // 这里退回到：如果 lastResult 有 lanAccess，复用它再试
      const prev = lastResult.value?.lanAccess
      if (prev && prev.addresses.length > 0) {
        log.push(`[3] reuse lastResult.lanAccess (${prev.addresses.length} candidates)`)
        return await tryLanCandidates(DEFAULT_API_BASE_URL, prev.addresses, guessPort(DEFAULT_API_BASE_URL), 'lan-candidate', log, t0)
      }

      // ─── [4] 真的没招了 ─────────────────────────────────
      log.push('[4] no candidates available, all-failed')
      lastError.value = 'all-candidates-failed'
      throw new Error('all-candidates-failed')
    } finally {
      isProbing.value = false
    }
  }

  /**
   * 拿到一个通路的 baseUrl 后，再用它的 lanAccess 候选继续探
   * 仅当 loopback 通 + 拿到 LAN 列表时进入
   */
  async function expandWithLanCandidates(
    primaryBase: string,
    lanAccess: ProbeResult['lanAccess'],
    primarySource: ProbeResult['source'],
    log: string[],
    t0: number,
  ): Promise<ProbeResult> {
    // 如果没有 LAN 候选，直接返回 primary
    if (!lanAccess || lanAccess.addresses.length === 0) {
      log.push('[expand] no lan candidates, commit primary')
      return commit(primaryBase, lanAccess, primarySource, log, t0)
    }
    const port = guessPort(primaryBase)
    // 排除 loopback（已在 step 2 试过，避免重复）
    const candidates = lanAccess.addresses.filter((a) => {
      if (typeof a !== 'string') return false
      if (a === '127.0.0.1' || a === '::1' || a === 'localhost') return false
      return true
    })
    log.push(`[expand] try ${candidates.length} lan candidates (port ${port})`)
    return await tryLanCandidates(primaryBase, candidates, port, primarySource, log, t0)
  }

  /**
   * 顺序试 LAN 候选；第一个通的晋升
   */
  async function tryLanCandidates(
    fallback: string,
    candidates: string[],
    port: number,
    fallbackSource: ProbeResult['source'],
    log: string[],
    t0: number,
  ): Promise<ProbeResult> {
    for (const addr of candidates) {
      const url = buildCandidateUrl(addr, port)
      log.push(`[lan] try ${url}`)
      const r = await probeHealth(url)
      log.push(`[lan] result: ok=${r.ok} latency=${r.latencyMs}ms err=${r.err || '-'}`)
      if (r.ok) {
        // 拿 LAN 列表用于本次 commit（如果是从 fallback 进入的，手动补一份）
        const lanAccess = await fetchLanCandidates(url)
        return commit(url, lanAccess, 'lan-candidate', log, t0)
      }
    }
    log.push(`[lan] all ${candidates.length} candidates failed, fallback to ${fallback}`)
    // 走到这里：所有 LAN 都死，但 loopback 是通的——保留 loopback 结果
    // 若连 loopback 都不通，caller 早已抛错，不会进入本函数
    const lanAccess = await fetchLanCandidates(fallback)
    return commit(fallback, lanAccess, fallbackSource, log, t0)
  }

  /**
   * 提交一次成功探测：写 localStorage + 调 setApiBaseUrl + 广播事件
   */
  function commit(
    baseUrl: string,
    lanAccess: ProbeResult['lanAccess'],
    source: ProbeResult['source'],
    log: string[],
    t0: number,
  ): ProbeResult {
    const latencyMs = Math.round(performance.now() - t0)
    const result: ProbeResult = {
      baseUrl,
      lanAccess,
      source,
      latencyMs,
      log,
    }
    lastResult.value = result
    // 同步到 encv.ts 的 localStorage（保持单一数据源）
    setApiBaseUrl(baseUrl)
    // console.debug 而非 console.error —— 探测成功是预期路径，不应污染红色错误日志
    console.debug('[useApiBaseProbe] commit', { baseUrl, source, latencyMs })
    return result
  }

  /**
   * 手动重置到默认 loopback：清 localStorage + 再探测一次
   * UI "恢复默认" 按钮用
   */
  async function resetToDefault(): Promise<ProbeResult> {
    localStorage.removeItem(SERVER_URL_KEY)
    lastProbeAt = 0
    return await probe({ force: true })
  }

  /**
   * 用户手动指定一个 URL：写 localStorage + 验证一次（不验证也接受——容许离线设置）
   */
  function setManual(url: string): void {
    // 基本格式校验
    if (!/^https?:\/\/[^\s/$.?#].[^\s]*$/i.test(url)) {
      throw new Error(`invalid baseUrl format: ${url}`)
    }
    setApiBaseUrl(url)
    lastProbeAt = 0
  }

  return {
    probe,
    resetToDefault,
    setManual,
    isProbing: isProbing as Ref<boolean>,
    lastResult: lastResult as Ref<ProbeResult | null>,
    lastError: lastError as Ref<string | null>,
  }
}

/** 取得模块级单例 */
export function useApiBaseProbe() {
  if (!_instance) {
    _instance = createProbe()
  }
  return _instance
}

/**
 * @internal 仅供单测使用：重置模块级单例。
 * 不导出给生产代码使用。
 */
export function __resetApiBaseProbeForTest(): void {
  _instance = null
}
