import { ref, onMounted, onUnmounted } from 'vue'
import { checkServerStatus, setApiBaseUrl, DEFAULT_API_BASE_URL } from '@/api/encv'
import { eventBus } from './useEventBus'
import { useWebSocket } from './useWebSocket'

const isOnline = ref(false)
const lastError = ref('')
let initialized = false
let nativeBridgeListenerAdded = false

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
  return result
}

function addNativeBridgeListener() {
  if (nativeBridgeListenerAdded) return
  nativeBridgeListenerAdded = true

  if (typeof window !== 'undefined') {
    window.addEventListener('encv:backend-ready', ((event: CustomEvent) => {
      const detail = event.detail || {}
      console.log('[ENCV] Backend ready from native bridge:', detail)
      if (detail.port && detail.port > 0) {
        const newUrl = `http://127.0.0.1:${detail.port}`
        if (DEFAULT_API_BASE_URL !== newUrl) {
          setApiBaseUrl(newUrl)
        }
      }
      if (detail.error) {
        lastError.value = detail.error
        return
      }
      isOnline.value = true
      lastError.value = ''
      useWebSocket().connect()
    }) as EventListener)
  }
}

export function useServerStatus() {
  const { connect, connectionState } = useWebSocket()

  addNativeBridgeListener()

  onMounted(async () => {
    if (!initialized) {
      initialized = true
      if (isOnline.value) {
        connect()
      } else {
        const result = await checkStatus()
        if (result.online) {
          connect()
        }
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
