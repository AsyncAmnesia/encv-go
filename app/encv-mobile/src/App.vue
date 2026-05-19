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

const { initTheme } = useTheme()
const { connect, disconnect } = useWebSocket()

onMounted(() => {
  initTheme()
  connect()
})

onUnmounted(() => {
  disconnect()
})
</script>
