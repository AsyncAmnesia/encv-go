import { ref, onMounted, onUnmounted } from 'vue'
import { checkServerStatus, setApiBaseUrl, DEFAULT_API_BASE_URL } from '@/api/encv'
import { eventBus } from './useEventBus'
import { useWebSocket } from './useWebSocket'
import { isNative, restartBackend, stopBackend, getBackendStatus } from '@/plugins/GoProcess'

const isOnline = ref(false)
const lastError = ref('')
const backendPort = ref(0)
const isRestarting = ref(false)
const isStopping = ref(false)
let initialized = false
let nativeBridgeListenerAdded = false

function onServerStatus(data: { online: boolean }) {
  if (isRestarting.value && !data.online) return
  isOnline.value = data.online
  if (data.online) {
    lastError.value = ''
  }
}

function onConnectionError(data: { error: string }) {
  if (isRestarting.value) return
  lastError.value = data.error
}

async function checkStatus() {
  const result = await checkServerStatus()
  isOnline.value = result.online
  lastError.value = result.error || ''
  if (result.online) {
    isRestarting.value = false
    isStopping.value = false
  }
  return result
}

function addNativeBridgeListener() {
  if (nativeBridgeListenerAdded) return
  nativeBridgeListenerAdded = true

  if (typeof window !== 'undefined') {
    const syncStatus = (event: Event) => {
      const customEvent = event as CustomEvent
      const detail = customEvent.detail || {}
      console.log('[ENCV] Backend status from native bridge:', detail)
      if (typeof detail.running === 'boolean') {
        isOnline.value = detail.running
      }
      if (detail.port && detail.port > 0) {
        backendPort.value = detail.port
        const newUrl = `http://127.0.0.1:${detail.port}`
        if (DEFAULT_API_BASE_URL !== newUrl) {
          setApiBaseUrl(newUrl)
        }
        isOnline.value = true
        lastError.value = ''
        isRestarting.value = false
        isStopping.value = false
        useWebSocket().connect()
      }
      if (detail.error) {
        lastError.value = detail.error
        isOnline.value = false
        isRestarting.value = false
        isStopping.value = false
      }
      if (detail.running === false && !detail.port) {
        backendPort.value = 0
        isStopping.value = false
      }
    }

    window.addEventListener('encv:backend-ready', syncStatus as EventListener)
    window.addEventListener('encv:backend-status', syncStatus as EventListener)
  }
}

async function handleRestart(): Promise<boolean> {
  if (!isNative()) {
    isOnline.value = false
    useWebSocket().disconnect()
    return false
  }
  isRestarting.value = true
  isStopping.value = false
  isOnline.value = false
  lastError.value = ''
  useWebSocket().disconnect()
  try {
    const result = await restartBackend()
    isRestarting.value = false
    if (result.success) {
      isOnline.value = true
      const status = await getBackendStatus()
      if (status.running && status.port > 0) {
        backendPort.value = status.port
        const newUrl = `http://127.0.0.1:${status.port}`
        if (DEFAULT_API_BASE_URL !== newUrl) {
          setApiBaseUrl(newUrl)
        }
        lastError.value = ''
        useWebSocket().connect()
      }
    } else {
      lastError.value = result.lastError || lastError.value
    }
    return result.success
  } catch (e) {
    isRestarting.value = false
    lastError.value = e instanceof Error ? e.message : String(e)
    return false
  }
}

async function handleStop(): Promise<boolean> {
  if (!isNative()) return false
  isStopping.value = true
  isRestarting.value = false
  isOnline.value = false
  lastError.value = ''
  useWebSocket().disconnect()
  try {
    const result = await stopBackend()
    isStopping.value = false
    backendPort.value = 0
    return result.success
  } catch (e) {
    isStopping.value = false
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
          lastError.value = status.lastError || ''
          connect()
        } else if (status.lastError) {
          lastError.value = status.lastError
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
    isRestarting,
    isStopping,
    checkStatus,
    connectionState,
    restartBackend: handleRestart,
    stopBackend: handleStop,
  }
}
