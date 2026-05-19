import { ref, onMounted, onUnmounted } from 'vue'
import { checkServerStatus, setApiBaseUrl, DEFAULT_API_BASE_URL } from '@/api/encv'
import { eventBus } from './useEventBus'
import { useWebSocket } from './useWebSocket'
import { isNative, restartBackend, stopBackend, getBackendStatus } from '@/plugins/GoProcess'

const isOnline = ref(false)
const lastError = ref('')
const backendPort = ref(0)
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
        backendPort.value = detail.port
        const newUrl = `http://127.0.0.1:${detail.port}`
        if (DEFAULT_API_BASE_URL !== newUrl) {
          setApiBaseUrl(newUrl)
        }
        isOnline.value = true
        lastError.value = ''
        useWebSocket().connect()
      }
      if (detail.error) {
        lastError.value = detail.error
        isOnline.value = false
      }
    }) as EventListener)
  }
}

async function handleRestart(): Promise<boolean> {
  if (!isNative()) {
    isOnline.value = false
    useWebSocket().disconnect()
    return false
  }
  isOnline.value = false
  lastError.value = ''
  useWebSocket().disconnect()
  try {
    const result = await restartBackend()
    return result.success
  } catch (e) {
    lastError.value = e instanceof Error ? e.message : String(e)
    return false
  }
}

async function handleStop(): Promise<boolean> {
  if (!isNative()) return false
  isOnline.value = false
  lastError.value = ''
  useWebSocket().disconnect()
  try {
    const result = await stopBackend()
    return result.success
  } catch (e) {
    lastError.value = e instanceof Error ? e.message : String(e)
    return false
  }
}

export function useServerStatus() {
  const { connect, connectionState } = useWebSocket()

  addNativeBridgeListener()

  onMounted(async () => {
    if (!initialized) {
      initialized = true
      if (isNative()) {
        const status = await getBackendStatus()
        if (status.running && status.port > 0) {
          isOnline.value = true
          backendPort.value = status.port
          connect()
        }
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
    backendPort,
    checkStatus,
    connectionState,
    restartBackend: handleRestart,
    stopBackend: handleStop,
  }
}
