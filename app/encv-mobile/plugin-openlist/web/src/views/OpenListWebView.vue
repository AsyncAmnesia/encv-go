<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button default-href="/home" />
        </ion-buttons>
        <ion-title>OpenList Web UI</ion-title>
        <ion-buttons slot="end">
          <ion-button @click="reload">
            <ion-icon :icon="refreshOutline" slot="icon-only" />
          </ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <div v-if="!isRunning" class="empty-state">
        <p>OpenList 未运行</p>
        <ion-button @click="startAndWait" color="primary">启动并加载</ion-button>
      </div>
      <iframe
        v-else
        :src="url"
        class="openlist-iframe"
        @error="onError"
        @load="onLoad"
      ></iframe>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  IonPage,
  IonHeader,
  IonToolbar,
  IonTitle,
  IonBackButton,
  IonButtons,
  IonButton,
  IonContent,
  IonIcon,
} from '@ionic/vue'
import { refreshOutline } from 'ionicons/icons'
import { OpenListNative, logBuffer } from '@/plugins/openlist-native'

const isRunning = ref(false)
const port = ref(0)

const url = computed(() => {
  return `http://127.0.0.1:${port.value || 5244}/#/login`
})

onMounted(() => {
  isRunning.value = OpenListNative.getIsRunning()
  port.value = OpenListNative.getPort()
})

async function startAndWait() {
  logBuffer.info('启动 OpenList...')
  const p = OpenListNative.startOpenList()
  if (p > 0) {
    // 等待 2s 让服务初始化
    setTimeout(() => {
      isRunning.value = true
      port.value = p
      logBuffer.info(`已加载 ${url.value}`)
    }, 2000)
  } else {
    logBuffer.error('启动失败')
  }
}

function reload() {
  // 重新触发 iframe load
  const frame = document.querySelector('.openlist-iframe') as HTMLIFrameElement | null
  if (frame) {
    frame.src = frame.src
  }
}

function onError() {
  logBuffer.error('iframe 加载失败')
}

function onLoad() {
  logBuffer.info('iframe 加载完成')
}
</script>

<style scoped>
.openlist-iframe {
  width: 100%;
  height: 100%;
  border: none;
  display: block;
}
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  gap: 16px;
}
</style>
