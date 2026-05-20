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
import { isNative, requestNotificationPermission, requestStoragePermission, checkPermissions } from '@/plugins/GoProcess'

const { initTheme } = useTheme()
const { connect, disconnect } = useWebSocket()

const FIRST_LAUNCH_KEY = 'encv-first-launch-done'

async function requestEssentialPermissions() {
  if (!isNative()) return

  const done = localStorage.getItem(FIRST_LAUNCH_KEY)
  if (done) return

  await requestNotificationPermission()
  await requestStoragePermission()
  localStorage.setItem(FIRST_LAUNCH_KEY, '1')
}

onMounted(async () => {
  initTheme()
  connect()
  await requestEssentialPermissions()
})

onUnmounted(() => {
  disconnect()
})
</script>
