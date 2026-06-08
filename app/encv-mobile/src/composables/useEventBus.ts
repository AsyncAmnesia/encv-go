type Handler<T = any> = (data: T) => void

export interface EncvEvents {
  'task:update': { id: string; type: string; status: string; progress: number }
  'task:progress': { id: string; progress: number; phase: string; speed: string; eta: string }
  'task:created': { id: string; type: string; sourcePath: string }
  'task:completed': { id: string; error?: string }
  'task:refresh': Record<string, never>
  'file:change': { path: string; action: 'create' | 'delete' | 'modify' }
  'server:status': { online: boolean }
  'server:connection-error': { error: string }
  'log:message': { level: string; message: string }
  'ws:message': { type: string; data: any }
  'openlist:status': {
    running: boolean
    port: number
    pid: number
    dataSizeBytes: number
    isInstalled: boolean
    lastError: string
    lastUpdateTs: number
  }
  'openlist:log': { level: number; message: string; timestamp: number }
  'openlist:error': { type: string; message: string; code?: number }
  /**
   * api-base:connected — useApiBaseProbe 探测成功 + WS 已重建后 emit。
   * 其它 composable / view 可监听此事件做后续动作（如刷新 agent list、re-mount 工具等）。
   */
  'api-base:connected': { baseUrl: string; source: 'cached' | 'loopback' | 'lan-candidate' }
  /**
   * api-base:disconnected — 探测失败（all-candidates-failed）后 emit。
   * UI 监听后显示错误 banner。
   */
  'api-base:disconnected': { error: string }
}

type EventKey = keyof EncvEvents

const listeners = new Map<string, Set<Handler>>()

function on<K extends EventKey>(event: K, handler: Handler<EncvEvents[K]>) {
  if (!listeners.has(event)) {
    listeners.set(event, new Set())
  }
  listeners.get(event)!.add(handler)
}

function off<K extends EventKey>(event: K, handler: Handler<EncvEvents[K]>) {
  listeners.get(event)?.delete(handler)
}

function emit<K extends EventKey>(event: K, data: EncvEvents[K]) {
  listeners.get(event)?.forEach(handler => {
    try {
      handler(data)
    } catch (e) {
      console.error(`Event bus error on "${event}":`, e)
    }
  })
}

function clear() {
  listeners.clear()
}

export const eventBus = {
  on,
  off,
  emit,
  clear,
}
