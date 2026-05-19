import { ref, onMounted, onUnmounted } from 'vue'
import { checkServerStatus } from '@/api/encv'
import { eventBus } from './useEventBus'
import { useWebSocket } from './useWebSocket'

const isOnline = ref(false)
let initialized = false

function onServerStatus(data: { online: boolean }) {
  isOnline.value = data.online
}

async function checkStatus() {
  isOnline.value = await checkServerStatus()
}

export function useServerStatus() {
  const { connect, connectionState } = useWebSocket()

  onMounted(async () => {
    if (!initialized) {
      initialized = true
      await checkStatus()
      connect()
      eventBus.on('server:status', onServerStatus)
    }
  })

  onUnmounted(() => {
    // keep connection alive across page navigations
  })

  return {
    isOnline,
    checkStatus,
    connectionState,
  }
}
