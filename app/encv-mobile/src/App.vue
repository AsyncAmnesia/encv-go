<template>
  <ion-app>
    <ion-router-outlet />
  </ion-app>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { IonApp, IonRouterOutlet } from '@ionic/vue'
import { useTheme } from '@/composables/useTheme'
import { useWebSocket } from '@/composables/useWebSocket'
import { isNative, requestNotificationPermission, requestStoragePermission } from '@/plugins/GoProcess'
import { hijackConsole } from '@/composables/useFrontendLogs'
import { autoInitVConsole } from '@/composables/useDevTools'

const { initTheme } = useTheme()
const { connect, disconnect } = useWebSocket()

const FIRST_LAUNCH_KEY = 'encv-first-launch-done'

async function requestEssentialPermissions() {
  if (!isNative()) return

  const done = localStorage.getItem(FIRST_LAUNCH_KEY)
  if (done) return

  console.info('[App] First launch, requesting essential permissions')
  const notifResult = await requestNotificationPermission()
  console.info('[App] Notification permission:', notifResult.granted ? 'granted' : 'denied')
  const storageResult = await requestStoragePermission()
  console.info('[App] Storage permission:', storageResult.granted ? 'granted' : 'denied')
  localStorage.setItem(FIRST_LAUNCH_KEY, '1')
}

onMounted(async () => {
  hijackConsole()
  initTheme()
  autoInitVConsole()
  connect()
  await requestEssentialPermissions()
  applyScreenOrientation()
})

async function applyScreenOrientation() {
  if (!isNative()) return
  const orientation = localStorage.getItem('encv_screen_orientation') || 'auto'
  try {
    const { ScreenOrientation } = await import('@capacitor/screen-orientation')
    if (orientation === 'portrait') {
      await ScreenOrientation.lock({ orientation: 'portrait' })
    } else if (orientation === 'landscape') {
      await ScreenOrientation.lock({ orientation: 'landscape' })
    } else {
      await ScreenOrientation.unlock()
    }
  } catch (e) {
    console.warn('[App] Failed to apply screen orientation:', e)
  }
}

onUnmounted(() => {
  disconnect()
})
</script>
