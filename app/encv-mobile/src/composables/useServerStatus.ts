import { ref, onMounted, onUnmounted } from 'vue'
import { checkServerStatus } from '@/api/encv'
import { eventBus } from './useEventBus'
import { useWebSocket } from './useWebSocket'

const isOnline = ref(false)
const lastError = ref('')
let initialized = false
let retryTimer: ReturnType<typeof setTimeout> | null = null

function onServerStatus(data: { online: boolean }) {
  isOnline.value = data.online
  if (data.online) {
    lastError.value = ''
    if (retryTimer) {
      clearTimeout(retryTimer)
      retryTimer = null
    }
  }
}

function onConnectionError(data: { error: string }) {
  lastError.value = data.error
}

async function checkStatus() {
  const result = await checkServerStatus()
  isOnline.value = result.online
  lastError.value = result.error || ''
  return result
}

async function waitForServer(maxRetries = 15, intervalMs = 1000): Promise<boolean> {
  for (let i = 0; i < maxRetries; i++) {
    const result = await checkStatus()
    if (result.online) return true
    await new Promise(r => setTimeout(r, intervalMs))
  }
  return false
}

function scheduleRetry() {
  if (retryTimer) return
  retryTimer = setTimeout(async () => {
    retryTimer = null
    const result = await checkStatus()
    if (result.online) {
      useWebSocket().connect()
    } else {
      scheduleRetry()
    }
  }, 3000)
}

export function useServerStatus() {
  const { connect, connectionState } = useWebSocket()

  onMounted(async () => {
    if (!initialized) {
      initialized = true
      const online = await waitForServer()
      connect()
      if (!online) {
        scheduleRetry()
      }
      eventBus.on('server:status', onServerStatus)
      eventBus.on('server:connection-error', onConnectionError)
    }
  })

  onUnmounted(() => {
  })

  return {
    isOnline,
    lastError,
    checkStatus,
    connectionState,
  }
}
