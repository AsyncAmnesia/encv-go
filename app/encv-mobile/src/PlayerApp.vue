<template>
  <ion-app>
    <Suspense>
      <template #default>
        <StandalonePlayer v-if="currentView === 'player'" @open-settings="currentView = 'settings'" />
        <PlayerSettings v-else @close="currentView = 'player'" />
      </template>
      <template #fallback>
        <div style="display:flex;justify-content:center;align-items:center;height:100vh;background:#1a1a2e;color:#fff;font-family:sans-serif;">
          <div style="text-align:center;">
            <div style="font-size:24px;margin-bottom:12px;">⏳</div>
            <div>Loading...</div>
          </div>
        </div>
      </template>
    </Suspense>
  </ion-app>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { IonApp } from '@ionic/vue'
import StandalonePlayer from '@/views/StandalonePlayer.vue'
import PlayerSettings from '@/views/PlayerSettings.vue'

const currentView = ref<'player' | 'settings'>('player')
</script>
