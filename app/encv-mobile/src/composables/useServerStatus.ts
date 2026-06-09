import { ref, onMounted, onUnmounted } from 'vue'
import { checkServerStatus, setApiBaseUrl, DEFAULT_API_BASE_URL } from '@/api/encv'
import { eventBus } from './useEventBus'
import { useWebSocket } from './useWebSocket'
import { isNative, restartBackend, stopBackend, getBackendStatus } from '@/plugins/GoProcess'
import { useApiBaseProbe } from './useApiBaseProbe'

const isOnline = ref(false)
const lastError = ref('')
const backendPort = ref(0)
const isRestarting = ref(false)
const isStopping = ref(false)
let initialized = false
let nativeBridgeListenerAdded = false

function onServerStatus(data: { online: boolean }) {
  if (isRestarting.value && !data.online) return
  isOnline.value = data.online
  if (data.online) {
    lastError.value = ''
  }
}

function onConnectionError(data: { error: string }) {
  if (isRestarting.value) return
  lastError.value = data.error
}

async function checkStatus() {
  const result = await checkServerStatus()
  isOnline.value = result.online
  lastError.value = result.error || ''
  if (result.online) {
    isRestarting.value = false
    isStopping.value = false
  }
  return result
}

/**
 * 手动重连：先跑 probe 探测链，命中后用新 baseUrl 重建 WS。
 * 用于：
 *   - 冷启动后仍 offline（探测失败） → 再次尝试
 *   - 用户在 Settings 改了 baseUrl → 立即让状态同步
 *   - WS 死掉且 HTTP 链路也没救 → 重探
 *
 * 与 useApiBaseProbe.probe() 的区别：本函数额外处理 WS 重连 + eventBus 通知。
 */
async function manualReconnect(): Promise<{ ok: boolean; baseUrl?: string; error?: string }> {
  isRestarting.value = true
  useWebSocket().disconnect()
  try {
    const result = await useApiBaseProbe().probe({ force: true })
    // probe 成功 → setApiBaseUrl 已写，再用 checkStatus 探一次确认"链路真通"
    const check = await checkStatus()
    if (check.online) {
      useWebSocket().connect()
      eventBus.emit('api-base:connected', { baseUrl: result.baseUrl, source: result.source })
      return { ok: true, baseUrl: result.baseUrl }
    }
    // 探测说通但 check 失败——罕见（race / 服务端崩），保留探测结果
    return { ok: false, baseUrl: result.baseUrl, error: check.error || 'post-probe health check failed' }
  } catch (e) {
    const errMsg = e instanceof Error ? e.message : String(e)
    lastError.value = errMsg
    isOnline.value = false
    eventBus.emit('api-base:disconnected', { error: errMsg })
    return { ok: false, error: errMsg }
  } finally {
    isRestarting.value = false
  }
}

/**
 * App 切回前台时触发一次探测（节流内置）
 *
 * 场景：用户在聊天过程中把 app 切到后台几分钟，网络环境可能已变
 *  （切 WiFi / 出隧道 / VPN 切换）。切回前台时再跑一次 probe，
 *  若 baseUrl 变了 → setApiBaseUrl → api-base:connected → useWebSocket 重连。
 *
 * 关键约束：
 *   - 只在 'visible' 切换时触发，避免 'hidden' 误触发
 *   - probe 内部 10s 节流，避免切应用瞬间多次触发
 *   - native 模式（APK 内嵌 backend）下 backend port 不会变，跳过探测
 */
let _visibilityListenerAdded = false
function setupVisibilityProbe() {
  if (_visibilityListenerAdded) return
  if (typeof document === 'undefined') return
  _visibilityListenerAdded = true

  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState !== 'visible') return
    if (isNative()) {
      // APK 模式：backend 跑在设备本地 127.0.0.1，IP 不会变；只需重连 WS
      if (!isOnline.value) {
        console.info('[useServerStatus] visibility visible + offline → reconnect')
        useWebSocket().forceReconnect()
      }
      return
    }
    // web/dev 模式：跑探测（cached → loopback → LAN 候选）
    console.info('[useServerStatus] visibility visible → probe()')
    useApiBaseProbe().probe().then((result) => {
      if (result.baseUrl) {
        // probe 成功 → api-base:connected 事件会自动 fire（useApiBaseProbe.commit）
        // 重新 check status 以更新 isOnline
        checkStatus().then((check) => {
          if (check.online) {
            useWebSocket().connect()
          }
        })
      }
    }).catch((e) => {
      // 全失败（all-candidates-failed）不弹错 — 保留旧值
      console.debug('[useServerStatus] visibility probe failed:', e instanceof Error ? e.message : String(e))
    })
  })
}

function addNativeBridgeListener() {
  if (nativeBridgeListenerAdded) return
  nativeBridgeListenerAdded = true

  if (typeof window !== 'undefined') {
    const syncStatus = (event: Event) => {
      const customEvent = event as CustomEvent
      const detail = customEvent.detail || {}
      console.log('[ENCV] Backend status from native bridge:', detail)
      if (typeof detail.running === 'boolean') {
        isOnline.value = detail.running
      }
      if (detail.port && detail.port > 0) {
        backendPort.value = detail.port
        const newUrl = `http://127.0.0.1:${detail.port}`
        if (DEFAULT_API_BASE_URL !== newUrl) {
          setApiBaseUrl(newUrl)
        }
        isOnline.value = true
        lastError.value = ''
        isRestarting.value = false
        isStopping.value = false
        useWebSocket().connect()
      }
      if (detail.error) {
        lastError.value = detail.error
        isOnline.value = false
        isRestarting.value = false
        isStopping.value = false
      }
      if (detail.running === false && !detail.port) {
        backendPort.value = 0
        isStopping.value = false
      }
    }

    window.addEventListener('encv:backend-ready', syncStatus as EventListener)
    window.addEventListener('encv:backend-status', syncStatus as EventListener)
  }
}

async function handleRestart(): Promise<boolean> {
  if (!isNative()) {
    isOnline.value = false
    useWebSocket().disconnect()
    return false
  }
  isRestarting.value = true
  isStopping.value = false
  isOnline.value = false
  lastError.value = ''
  useWebSocket().disconnect()
  try {
    const result = await restartBackend()
    isRestarting.value = false
    if (result.success) {
      isOnline.value = true
      const status = await getBackendStatus()
      if (status.running && status.port > 0) {
        backendPort.value = status.port
        const newUrl = `http://127.0.0.1:${status.port}`
        if (DEFAULT_API_BASE_URL !== newUrl) {
          setApiBaseUrl(newUrl)
        }
        lastError.value = ''
        useWebSocket().connect()
      }
    } else {
      lastError.value = result.lastError || lastError.value
    }
    return result.success
  } catch (e) {
    isRestarting.value = false
    lastError.value = e instanceof Error ? e.message : String(e)
    return false
  }
}

async function handleStop(): Promise<boolean> {
  if (!isNative()) return false
  isStopping.value = true
  isRestarting.value = false
  isOnline.value = false
  lastError.value = ''
  useWebSocket().disconnect()
  try {
    const result = await stopBackend()
    isStopping.value = false
    backendPort.value = 0
    return result.success
  } catch (e) {
    isStopping.value = false
    lastError.value = e instanceof Error ? e.message : String(e)
    return false
  }
}

export function useServerStatus() {
  const { connect, connectionState } = useWebSocket()

  addNativeBridgeListener()
  setupVisibilityProbe()

  onMounted(async () => {
    if (!initialized) {
      initialized = true
      if (isNative()) {
        const status = await getBackendStatus()
        if (status.running && status.port > 0) {
          isOnline.value = true
          backendPort.value = status.port
          lastError.value = status.lastError || ''
          connect()
        } else if (status.lastError) {
          lastError.value = status.lastError
        }
      } else {
        // Web/dev 模式：跑探测链（cached → loopback → LAN 候选）
        // 探测成功 → 写 localStorage + setApiBaseUrl + connect
        // 探测失败 → 保留旧值兜底，不弹死错误
        // 🆕 2026-06-09 沙箱 mock 浏览器：probe 在 trae 沙箱下也可能 throw（fallback 路径
        //   已 graceful，但保险起见这里也包 try-catch —— 任何未预期 throw 都不能让
        //   [App] onErrorCaptured 抓到，导致整个 SPA 渲染错误边界）
        try {
          const result = await useApiBaseProbe().probe()
          if (result.baseUrl) {
            const check = await checkStatus()
            if (check.online) {
              connect()
              eventBus.emit('api-base:connected', { baseUrl: result.baseUrl, source: result.source })
            } else {
              // 罕见：探测到 URL 但 health check 失败
              lastError.value = check.error || 'post-probe health check failed'
              isOnline.value = false
            }
          } else {
            // 全失败，尝试 legacy checkStatus 兜底
            const fallback = await checkStatus()
            if (fallback.online) connect()
          }
        } catch (probeErr) {
          // 🆕 任何意外 throw → 兜底 legacy checkStatus，不让 [App] 错误边界捕获
          console.warn('[useServerStatus] probe threw unexpectedly, falling back:', probeErr instanceof Error ? probeErr.message : String(probeErr))
          try {
            const fallback = await checkStatus()
            if (fallback.online) connect()
          } catch (fallbackErr) {
            console.debug('[useServerStatus] legacy fallback also failed (expected in trae sandbox):', fallbackErr instanceof Error ? fallbackErr.message : String(fallbackErr))
          }
        }
      }
      eventBus.on('server:status', onServerStatus)
      eventBus.on('server:connection-error', onConnectionError)
    }
  })

  onUnmounted(() => {
  })

  return {
    isOnline,
    lastError,
    backendPort,
    isRestarting,
    isStopping,
    checkStatus,
    connectionState,
    restartBackend: handleRestart,
    stopBackend: handleStop,
    // 手动重连：跑探测链 + 重建 WS（用于 Settings "立即探测" / 错误 banner "重试"）
    manualReconnect,
    // 直接暴露 probe composable，UI 可调 probe() / setManual() / resetToDefault()
    probe: useApiBaseProbe,
  }
}
