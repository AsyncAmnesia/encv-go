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
import { registerFileFeature } from '@/composables/useFileFeatures'
import { createAlistEncryptFeature } from '@/features/alist-encrypt'

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
    console.debug('[App] Failed to apply screen orientation:', e)
  }
}

onMounted(async () => {
  hijackConsole()
  console.error('[SAT-DBG][App] onMounted | ts=', Date.now())
  initTheme()
  autoInitVConsole()
  console.error('[SAT-DBG][App] ws.connect() | ts=', Date.now())
  connect()
  registerFileFeature(createAlistEncryptFeature())
  await requestEssentialPermissions()
  applyScreenOrientation()
})

onUnmounted(() => {
  console.error('[SAT-DBG][App] onUnmounted → ws.disconnect() | ts=', Date.now())
  disconnect()
})
</script>
