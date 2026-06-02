import { ref, onMounted, onUnmounted } from 'vue'
import { eventBus } from './useEventBus'
import { isNative, getOpenListFullState } from '@/plugins/GoProcess'

export function useOpenListBridge() {
  const status = ref<{ running: boolean; port: number; pid: number; dataSizeBytes: number }>({
    running: false, port: 0, pid: 0, dataSizeBytes: 0
  })
  let pollTimer: ReturnType<typeof setInterval> | null = null

  async function pollStatus() {
    console.error('[SAT-DBG][OpenList][Bridge] pollStatus() called')
    try {
      if (isNative()) {
        const result = await getOpenListFullState()
        console.error('[SAT-DBG][OpenList][Bridge] getOpenListFullState returned:', JSON.stringify(result))
        status.value = {
          running: result.running ?? false,
          port: result.port ?? 0,
          pid: result.pid ?? 0,
          dataSizeBytes: result.dataSizeBytes ?? 0,
        }
        eventBus.emit('openlist:status', status.value)
        if (result.lastStartError) {
          eventBus.emit('openlist:error', { type: 'start_error', message: result.lastStartError })
        }
      }
    } catch (e: any) {
      console.error('[SAT-DBG][OpenList][Bridge] pollStatus() error:', e?.message || e)
      eventBus.emit('openlist:error', { type: 'bridge_error', message: e?.message || String(e) })
    }
  }

  onMounted(() => {
    console.error('[SAT-DBG][OpenList][Bridge] useOpenListBridge mounted')
    if (isNative()) {
      // Initial poll
      pollStatus()
      // Poll every 3 seconds
      pollTimer = setInterval(pollStatus, 3000)
    }
  })

  onUnmounted(() => {
    console.error('[SAT-DBG][OpenList][Bridge] useOpenListBridge unmounted')
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  })

  return { status }
}