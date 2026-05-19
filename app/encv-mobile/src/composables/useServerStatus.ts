import { ref, onMounted, onUnmounted } from 'vue'
import { checkServerStatus } from '@/api/encv'
import { eventBus } from './useEventBus'
import { useWebSocket } from './useWebSocket'

const isOnline = ref(false)
const lastError = ref('')
let initialized = false

function onServerStatus(data: { online: boolean }) {
  isOnline.value = data.online
  if (data.online) {
    lastError.value = ''
  }
}

function onConnectionError(data: { error: string }) {
  lastError.value = data.error
}

async function checkStatus() {
  const result = await checkServerStatus()
  isOnline.value = result.online
  lastError.value = result.error || ''
}

export function useServerStatus() {
  const { connect, connectionState } = useWebSocket()

  onMounted(async () => {
    if (!initialized) {
      initialized = true
      await checkStatus()
      connect()
      eventBus.on('server:status', onServerStatus)
      eventBus.on('server:connection-error', onConnectionError)
    }
  })

  onUnmounted(() => {
    // keep connection alive across page navigations
  })

  return {
    isOnline,
    lastError,
    checkStatus,
    connectionState,
  }
}
