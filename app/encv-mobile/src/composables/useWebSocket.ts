import { ref } from 'vue'
import { getWebSocketUrl } from '@/api/encv'
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
        console.debug('[ENCV-WS] Pong timeout, reconnecting...')
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
    console.error('[ENCV-WS] Failed to parse message:', e)
  }
}

function connect() {
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
    return
  }

  const url = getWebSocketUrl()
  connectionState.value = 'connecting'

  try {
    ws = new WebSocket(url)
  } catch (e) {
    console.error('[ENCV-WS] Failed to create WebSocket:', e)
    connectionState.value = 'disconnected'
    scheduleReconnect()
    return
  }

  ws.onopen = () => {
    connectionState.value = 'connected'
    reconnectDelay = 1000
    startHeartbeat()
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
    eventBus.emit('server:connection-error', { error: 'Failed to connect to server' })
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

function send(type: string, data: any) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type, data }))
  } else {
    console.debug('[ENCV-WS] Cannot send, connection not open')
  }
}

export function useWebSocket() {
  return {
    connectionState,
    connect,
    disconnect,
    send,
    forceReconnect,
  }
}
