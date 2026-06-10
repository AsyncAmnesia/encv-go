import { ref } from 'vue'
import { getWebSocketUrl, isOpenPreviewBrowser } from '@/api/encv'
import { eventBus } from './useEventBus'

export type ConnectionState = 'connecting' | 'connected' | 'disconnected'

interface WsMessage {
  type: string
  data: any
}

const connectionState = ref<ConnectionState>('disconnected')
let ws: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let heartbeatTimer: ReturnType<typeof setInterval> | null = null
let pongTimeoutTimer: ReturnType<typeof setTimeout> | null = null
let reconnectDelay = 1000
const MAX_RECONNECT_DELAY = 30000
const HEARTBEAT_INTERVAL = 30000
const PONG_TIMEOUT = 10000

const KNOWN_WS_EVENTS = new Set([
  'task:update', 'task:progress', 'task:created', 'task:completed',
  'file:change', 'server:status', 'server:connection-error',
  'log:message',
])

function resetTimers() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  if (heartbeatTimer) {
    clearInterval(heartbeatTimer)
    heartbeatTimer = null
  }
  if (pongTimeoutTimer) {
    clearTimeout(pongTimeoutTimer)
    pongTimeoutTimer = null
  }
}

function startHeartbeat() {
  stopHeartbeat()
  heartbeatTimer = setInterval(() => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'ping' }))
      pongTimeoutTimer = setTimeout(() => {
        forceReconnect()
      }, PONG_TIMEOUT)
    }
  }, HEARTBEAT_INTERVAL)
}

function stopHeartbeat() {
  if (heartbeatTimer) {
    clearInterval(heartbeatTimer)
    heartbeatTimer = null
  }
  if (pongTimeoutTimer) {
    clearTimeout(pongTimeoutTimer)
    pongTimeoutTimer = null
  }
}

function handleMessage(event: MessageEvent) {
  try {
    const msg: WsMessage = JSON.parse(event.data)

    if (msg.type === 'pong') {
      if (pongTimeoutTimer) {
        clearTimeout(pongTimeoutTimer)
        pongTimeoutTimer = null
      }
      return
    }

    if (KNOWN_WS_EVENTS.has(msg.type)) {
      eventBus.emit(msg.type as any, msg.data)
    }
    eventBus.emit('ws:message', { type: msg.type, data: msg.data })
  } catch (e) {
    console.error('[ENCV-WS] Failed to parse message:', e instanceof Error ? `${e.name}: ${e.message}` : String(e), 'raw=', event.data?.slice(0, 200))
  }
}

function connect() {
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
    return
  }

  // 🆕 2026-06-10 沙箱 OpenPreview 浏览器：trae 反代 :16000 不支持 WebSocket upgrade
  //   （实测：WebSocket upgrade → 502 "WebSocket upgrade failed"）。
  //   此时 new WebSocket('wss://trae.cn/ws') → 1006 异常关闭 →
  //   onclose emit `server:status` `{online:false}` → 覆盖 HTTP 探测的 true →
  //   UI 永远显示"离线"+"Connection closed (code: 1006)"。
  //   修复：OpenPreview 浏览器下不连 WS，直接 emit online:true 让 UI 显示"在线"，
  //   业务侧需要的实时功能（DevLogs 实时流、agent 流式 chat）由调用方自己降级
  //   （HTTP 轮询 / 提示用户切到沙箱本地或 APK 真机）。
  if (isOpenPreviewBrowser()) {
    console.info('[ENCV-WS] OpenPreview browser detected, skipping WebSocket (trae reverse-proxy :16000 does not support WS upgrade). Use HTTP polling / fallback paths.')
    connectionState.value = 'disconnected'
    // 只 emit online:true，**不** emit connection-error（避免 UI 误显"Connection closed"）。
    // 业务侧需要的实时功能（DevLogs 实时流、agent 流式 chat）由调用方自己降级
    // （HTTP 轮询 / 提示用户切到沙箱本地或 APK 真机）。
    eventBus.emit('server:status', { online: true })
    return
  }

  const url = getWebSocketUrl()
  connectionState.value = 'connecting'
  console.info(`[ENCV-WS] connecting to ${url} (dev=${import.meta.env.DEV}, origin=${location.origin})`)

  try {
    ws = new WebSocket(url)
  } catch (e) {
    console.error('[ENCV-WS] Failed to create WebSocket:', e instanceof Error ? `${e.name}: ${e.message}` : String(e), `url=${url}`)
    connectionState.value = 'disconnected'
    scheduleReconnect()
    return
  }

  ws.onopen = () => {
    connectionState.value = 'connected'
    reconnectDelay = 1000
    startHeartbeat()
    console.info(`[ENCV-WS] connected to ${url}`)
    eventBus.emit('server:status', { online: true })
  }

  ws.onmessage = handleMessage

  ws.onclose = (event) => {
    connectionState.value = 'disconnected'
    stopHeartbeat()
    eventBus.emit('server:status', { online: false })
    if (!event.wasClean) {
      eventBus.emit('server:connection-error', { error: `Connection closed (code: ${event.code})` })
      scheduleReconnect()
    }
  }

  ws.onerror = () => {
    connectionState.value = 'disconnected'
    console.error('[ENCV-WS] WebSocket error:', `url=${url}`, `readyState=${ws?.readyState}`)
    eventBus.emit('server:connection-error', { error: `Failed to connect to ${url}` })
  }
}

function disconnect() {
  resetTimers()
  if (ws) {
    ws.onclose = null
    ws.onerror = null
    ws.onmessage = null
    ws.onopen = null
    if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
      ws.close(1000, 'Client disconnect')
    }
    ws = null
  }
  connectionState.value = 'disconnected'
}

function scheduleReconnect() {
  if (reconnectTimer) return
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    connect()
  }, reconnectDelay)
  reconnectDelay = Math.min(reconnectDelay * 2, MAX_RECONNECT_DELAY)
}

function forceReconnect() {
  disconnect()
  reconnectDelay = 1000
  connect()
}

// ─── baseUrl 变更监听 ────────────────────────────────────────
//
// 监听两个信号：
//  1. eventBus 'api-base:connected'（同进程内 probe 成功 → 强制重连 WS）
//  2. window 'storage' 事件 'encv-server-url' 键变化（跨 tab / 跨进程）
//
// 触发后等 800ms 让 setApiBaseUrl 写完 localStorage 稳定，再 forceReconnect。
let _apiBaseListenersAdded = false
function ensureApiBaseListeners() {
  if (_apiBaseListenersAdded) return
  if (typeof window === 'undefined') return
  _apiBaseListenersAdded = true
  eventBus.on('api-base:connected', (data: { baseUrl: string; source: string }) => {
    console.info(`[ENCV-WS] api-base:connected (${data.source}) → forceReconnect`)
    // 给 setApiBaseUrl 写 localStorage 留 100ms 余量
    setTimeout(() => forceReconnect(), 100)
  })
  window.addEventListener('storage', (e: StorageEvent) => {
    if (e.key !== 'encv-server-url') return
    if (!e.newValue) return
    console.info(`[ENCV-WS] storage event: encv-server-url changed → forceReconnect`)
    setTimeout(() => forceReconnect(), 100)
  })
}

function send(type: string, data: any) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type, data }))
  } else {
    console.debug('[ENCV-WS] Cannot send, connection not open')
  }
}

export function useWebSocket() {
  ensureApiBaseListeners()
  return {
    connectionState,
    connect,
    disconnect,
    send,
    forceReconnect,
  }
}
