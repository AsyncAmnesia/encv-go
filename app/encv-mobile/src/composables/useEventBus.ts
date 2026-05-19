type Handler<T = any> = (data: T) => void

export interface EncvEvents {
  'task:update': { id: string; type: string; status: string; progress: number }
  'task:created': { id: string; type: string; sourcePath: string }
  'task:completed': { id: string; error?: string }
  'file:change': { path: string; action: 'create' | 'delete' | 'modify' }
  'server:status': { online: boolean }
  'log:message': { level: string; message: string }
  'ws:message': { type: string; data: any }
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
