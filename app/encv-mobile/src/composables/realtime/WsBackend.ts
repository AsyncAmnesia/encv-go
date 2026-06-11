/**
 * WsBackend — WebSocket transport 实现
 *
 * 历史：
 *   - 之前 [useWebSocket.ts](file:///workspace/app/encv-mobile/src/composables/useWebSocket.ts) 整个文件
 *   - 现在迁到 realtime/WsBackend.ts 内部 backend
 *   - 公共 useRealtimeTransport() 单例负责生命周期
 *
 * 行为：
 *   - 选举 ws 模式后 → 调 start() 连 WS
 *   - 收到消息 → 解析后 emit('task:update', data) 等
 *   - 断线 → exp backoff 重连（1s → 2s → 4s → ... → max 30s）
 *   - 心跳：30s 一次 ping，10s 内没回 pong → 强制重连
 *
 * 防御（不应到达，但保留）：
 *   - OpenPreview 浏览器理论上被 electMode 分流到 http-poll
 *   - 但如果 _forcedMode = 'ws' 强制切过来，调用 start() 时再 emit online:true 静默退出
 */

import { getWebSocketUrl, isOpenPreviewBrowser } from '@/api/encv'
import type { Backend, EventEmitter } from './Backend'

interface WsMessage {
  type: string
  data: any
}

/** ws 已知事件集合（其他事件也通过 ws:message 透传） */
const KNOWN_WS_EVENTS = new Set([
  'task:update', 'task:progress', 'task:created', 'task:completed',
  'file:change', 'server:status', 'server:connection-error',
  'log:message',
])

export interface WsBackendOptions {
  /** WS 连接成功回调（给 transport 改 connectionState） */
  onConnected?: () => void
  /** WS 断开回调（不区分 clean close） */
  onDisconnected?: () => void
}

export function createWsBackend(
  emit: EventEmitter,
  options: WsBackendOptions = {},
): Backend {
  let ws: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let heartbeatTimer: ReturnType<typeof setInterval> | null = null
  let pongTimeoutTimer: ReturnType<typeof setTimeout> | null = null
  let reconnectDelay = 1000
  let running = false

  const MAX_RECONNECT_DELAY = 30000
  const HEARTBEAT_INTERVAL = 30000
  const PONG_TIMEOUT = 10000

  function resetTimers() {
    if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null }
    if (heartbeatTimer) { clearInterval(heartbeatTimer); heartbeatTimer = null }
    if (pongTimeoutTimer) { clearTimeout(pongTimeoutTimer); pongTimeoutTimer = null }
  }

  function startHeartbeat() {
    stopHeartbeat()
    heartbeatTimer = setInterval(() => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'ping' }))
        pongTimeoutTimer = setTimeout(() => { forceReconnect() }, PONG_TIMEOUT)
      }
    }, HEARTBEAT_INTERVAL)
  }

  function stopHeartbeat() {
    if (heartbeatTimer) { clearInterval(heartbeatTimer); heartbeatTimer = null }
    if (pongTimeoutTimer) { clearTimeout(pongTimeoutTimer); pongTimeoutTimer = null }
  }

  function handleMessage(event: MessageEvent) {
    try {
      const msg: WsMessage = JSON.parse(event.data)

      if (msg.type === 'pong') {
        if (pongTimeoutTimer) { clearTimeout(pongTimeoutTimer); pongTimeoutTimer = null }
        return
      }

      if (KNOWN_WS_EVENTS.has(msg.type)) {
        emit(msg.type, msg.data)
      }
      emit('ws:message', { type: msg.type, data: msg.data })
    } catch (e) {
      console.error('[WsBackend] Failed to parse message:', e instanceof Error ? `${e.name}: ${e.message}` : String(e), 'raw=', event.data?.slice(0, 200))
    }
  }

  function connect() {
    if (!running) return
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
      return
    }

    // 🆕 2026-06-10 沙箱 OpenPreview 浏览器：trae 反代 :16000 不支持 WebSocket upgrade
    //   防御：理论上 electMode 已分流，但 _forcedMode 强制时可能落到这里
    if (isOpenPreviewBrowser()) {
      console.info('[WsBackend] OpenPreview browser detected, skipping WebSocket.')
      emit('server:status', { online: true })
      return
    }

    const url = getWebSocketUrl()
    console.info(`[WsBackend] connecting to ${url} (origin=${typeof location !== 'undefined' ? location.origin : 'n/a'})`)

    try {
      ws = new WebSocket(url)
    } catch (e) {
      console.error('[WsBackend] Failed to create WebSocket:', e instanceof Error ? `${e.name}: ${e.message}` : String(e), `url=${url}`)
      scheduleReconnect()
      return
    }

    ws.onopen = () => {
      reconnectDelay = 1000
      startHeartbeat()
      console.info(`[WsBackend] connected to ${url}`)
      options.onConnected?.()
      emit('server:status', { online: true })
    }

    ws.onmessage = handleMessage

    ws.onclose = (event) => {
      options.onDisconnected?.()
      emit('server:status', { online: false })
      if (running) { stopHeartbeat(); scheduleReconnect() }
      if (!event.wasClean) {
        emit('server:connection-error', { error: `Connection closed (code: ${event.code})` })
      }
    }

    ws.onerror = () => {
      console.error('[WsBackend] WebSocket error:', `url=${url}`, `readyState=${ws?.readyState}`)
      emit('server:connection-error', { error: `Failed to connect to ${url}` })
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
  }

  function scheduleReconnect() {
    if (reconnectTimer) return
    if (!running) return
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

  return {
    start() {
      running = true
      reconnectDelay = 1000
      connect()
    },
    stop() {
      running = false
      disconnect()
    },
    reset() {
      disconnect()
      reconnectDelay = 1000
    },
  }
}
