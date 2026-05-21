<template>
  <ion-app>
    <Suspense>
      <template #default>
        <StandalonePlayer />
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
import { IonApp } from '@ionic/vue'
import StandalonePlayer from '@/views/StandalonePlayer.vue'
</script>
