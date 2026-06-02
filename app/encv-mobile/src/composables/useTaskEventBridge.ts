import { onMounted, onUnmounted } from 'vue'
import { eventBus } from '@/composables/useEventBus'

export interface TaskEventBridgeOptions {
  onUpdate?: (data: { id: string; type: string; status: string; progress: number }) => void
  onProgress?: (data: { id: string; progress: number; phase: string; speed: string; eta: string }) => void
  onCreate?: (data: { id: string; type: string; sourcePath: string }) => void
  onComplete?: (data: { id: string; error?: string }) => void
  onRefresh?: () => void
  onFileChange?: (data: { path: string; action: 'create' | 'delete' | 'modify' }) => void
  onServerStatus?: (data: { online: boolean }) => void
  onWsMessage?: (data: { type: string; data: any }) => void
}

export function useTaskEventBridge(options: TaskEventBridgeOptions) {
  onMounted(() => {
    if (options.onUpdate) eventBus.on('task:update', options.onUpdate)
    if (options.onProgress) eventBus.on('task:progress', options.onProgress)
    if (options.onCreate) eventBus.on('task:created', options.onCreate)
    if (options.onComplete) eventBus.on('task:completed', options.onComplete)
    if (options.onRefresh) eventBus.on('task:refresh', options.onRefresh)
    if (options.onFileChange) eventBus.on('file:change', options.onFileChange)
    if (options.onServerStatus) eventBus.on('server:status', options.onServerStatus)
    if (options.onWsMessage) eventBus.on('ws:message', options.onWsMessage)
  })

  onUnmounted(() => {
    if (options.onUpdate) eventBus.off('task:update', options.onUpdate)
    if (options.onProgress) eventBus.off('task:progress', options.onProgress)
    if (options.onCreate) eventBus.off('task:created', options.onCreate)
    if (options.onComplete) eventBus.off('task:completed', options.onComplete)
    if (options.onRefresh) eventBus.off('task:refresh', options.onRefresh)
    if (options.onFileChange) eventBus.off('file:change', options.onFileChange)
    if (options.onServerStatus) eventBus.off('server:status', options.onServerStatus)
    if (options.onWsMessage) eventBus.off('ws:message', options.onWsMessage)
  })
}
