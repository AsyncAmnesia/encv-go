import { ref, onMounted, onUnmounted } from 'vue'
import { eventBus } from './useEventBus'
import { isNative, getOpenListRuntime } from '@/plugins/GoProcess'

export function useOpenListBridge() {
  const runtime = ref<{
    isInstalled: boolean
    running: boolean
    port: number
    pid: number
    dataSizeBytes: number
    lastError: string
    lastUpdateTs: number
  }>({
    isInstalled: false,
    running: false,
    port: 0,
    pid: 0,
    dataSizeBytes: 0,
    lastError: '',
    lastUpdateTs: 0,
  })
  let pollTimer: ReturnType<typeof setInterval> | null = null

  async function pollOnce() {
    console.error('[SAT-DBG][OpenList][Frontend] pollOnce() begin')
    try {
      if (isNative()) {
        const result = await getOpenListRuntime()
        console.error('[SAT-DBG][OpenList][Frontend] pollOnce() got:', JSON.stringify(result))
        runtime.value = result
        if (result.isInstalled) {
          eventBus.emit('openlist:status', {
            running: result.running,
            port: result.port,
            pid: result.pid,
            dataSizeBytes: result.dataSizeBytes,
            isInstalled: true,
            lastError: result.lastError,
            lastUpdateTs: result.lastUpdateTs,
          })
          if (result.lastError) {
            eventBus.emit('openlist:error', { type: 'runtime_error', message: result.lastError })
          }
        } else {
          eventBus.emit('openlist:status', {
            running: false,
            port: 0,
            pid: 0,
            dataSizeBytes: 0,
            isInstalled: false,
            lastError: 'not installed',
            lastUpdateTs: 0,
          })
        }
      }
    } catch (e: any) {
      console.error('[SAT-DBG][OpenList][Frontend] pollOnce() error:', e?.message || e)
      eventBus.emit('openlist:error', { type: 'bridge_error', message: e?.message || String(e) })
    }
  }

  onMounted(() => {
    console.error('[SAT-DBG][OpenList][Frontend] useOpenListBridge mounted')
    if (isNative()) {
      pollOnce()
      pollTimer = setInterval(pollOnce, 3000)
    }
  })

  onUnmounted(() => {
    console.error('[SAT-DBG][OpenList][Frontend] useOpenListBridge unmounted')
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  })

  return { runtime, pollOnce }
}
