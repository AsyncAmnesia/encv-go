import { ref, onMounted, onUnmounted } from 'vue'
import { checkServerStatus } from '@/api/encv'

const isOnline = ref(false)
let pollingTimer: ReturnType<typeof setInterval> | null = null
let activeCount = 0

async function checkStatus() {
  isOnline.value = await checkServerStatus()
}

function startPolling(intervalMs = 10000) {
  if (pollingTimer) return
  checkStatus()
  pollingTimer = setInterval(checkStatus, intervalMs)
}

function stopPolling() {
  if (pollingTimer) {
    clearInterval(pollingTimer)
    pollingTimer = null
  }
}

export function useServerStatus() {
  onMounted(() => {
    activeCount++
    if (activeCount === 1) {
      startPolling()
    }
  })

  onUnmounted(() => {
    activeCount--
    if (activeCount <= 0) {
      activeCount = 0
      stopPolling()
    }
  })

  return {
    isOnline,
    checkStatus,
  }
}
