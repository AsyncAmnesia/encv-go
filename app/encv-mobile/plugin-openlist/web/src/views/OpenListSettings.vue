<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button default-href="/home" />
        </ion-buttons>
        <ion-title>设置</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <ion-list>
        <ion-item>
          <ion-label>
            <h2>OpenList 版本</h2>
            <p>{{ version }}</p>
          </ion-label>
        </ion-item>
        <ion-item>
          <ion-label>
            <h2>数据目录</h2>
            <p class="mono-text">{{ dataDir }}</p>
          </ion-label>
        </ion-item>
        <ion-item button @click="openWebUi">
          <ion-icon :icon="globeOutline" slot="start" />
          <ion-label>
            <h2>打开 Web UI</h2>
            <p>http://127.0.0.1:{{ port }}</p>
          </ion-label>
        </ion-item>
        <ion-item button @click="goHome">
          <ion-icon :icon="homeOutline" slot="start" />
          <ion-label>
            <h2>返回主控</h2>
          </ion-label>
        </ion-item>
      </ion-list>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  IonPage,
  IonHeader,
  IonToolbar,
  IonTitle,
  IonBackButton,
  IonButtons,
  IonContent,
  IonList,
  IonItem,
  IonLabel,
  IonIcon,
} from '@ionic/vue'
import { globeOutline, homeOutline } from 'ionicons/icons'
import { OpenListNative } from '@/plugins/openlist-native'

const router = useRouter()

const version = ref('unknown')
const dataDir = ref('')
const port = ref(0)

onMounted(() => {
  version.value = OpenListNative.getVersion()
  dataDir.value = OpenListNative.getDataDir()
  port.value = OpenListNative.getPort()
})

function openWebUi() {
  window.open(`http://127.0.0.1:${port.value || 5244}/#/login`, '_blank')
}

function goHome() {
  router.push('/home')
}
</script>

<style scoped>
.mono-text {
  font-family: monospace;
  font-size: 11px;
  word-break: break-all;
}
</style>
