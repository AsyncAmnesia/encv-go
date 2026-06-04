<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>OpenList - v{{ version }}</ion-title>
        <ion-buttons slot="end">
          <ion-button @click="openPasswordDialog">
            <ion-icon :icon="keyOutline" slot="icon-only" />
          </ion-button>
          <ion-button @click="goToConfig">
            <ion-icon :icon="codeSlashOutline" slot="icon-only" />
          </ion-button>
          <ion-button @click="goToWebView">
            <ion-icon :icon="globeOutline" slot="icon-only" />
          </ion-button>
          <ion-button @click="goToSettings">
            <ion-icon :icon="settingsOutline" slot="icon-only" />
          </ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <!-- 复用 @encvgo/components 共享状态卡 -->
      <OpenListStatusCard :runtime="runtime" />

      <!-- 复用 @encvgo/components 共享日志列表 -->
      <OpenListLogList :logs="logs" />

      <!-- 启动 / 停止 FAB -->
      <ion-fab vertical="bottom" horizontal="end" slot="fixed">
        <ion-fab-button
          :color="runtime.running ? 'danger' : 'primary'"
          @click="toggleService"
          :disabled="isControlling"
        >
          <ion-spinner v-if="isControlling" name="crescent" />
          <ion-icon v-else :icon="runtime.running ? powerOutline : playOutline" />
        </ion-fab-button>
      </ion-fab>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  IonPage,
  IonHeader,
  IonToolbar,
  IonTitle,
  IonButtons,
  IonButton,
  IonContent,
  IonIcon,
  IonFab,
  IonFabButton,
  IonSpinner,
  modalController,
} from '@ionic/vue'
import {
  keyOutline,
  codeSlashOutline,
  settingsOutline,
  globeOutline,
  powerOutline,
  playOutline,
} from 'ionicons/icons'
import { OpenListStatusCard, OpenListLogList, type OpenListRuntime, type OpenListLog } from '@encvgo/components'
import { OpenListNative, logBuffer } from '@/plugins/openlist-native'
import PwdEditDialog from '@/components/PwdEditDialog.vue'

const router = useRouter()

const runtime = ref<OpenListRuntime>({
  running: false,
  port: 0,
  pid: 0,
  dataSizeBytes: 0,
  lastError: '',
  lastUpdateTs: 0,
  dataDir: '',
  isInstalled: true,
})
const version = ref('unknown')
const isControlling = ref(false)
const logs = ref<OpenListLog[]>([])

let refreshTimer: ReturnType<typeof setInterval> | null = null
let unsubscribeLog: (() => void) | null = null

onMounted(async () => {
  // 订阅日志流
  unsubscribeLog = logBuffer.subscribe((all) => {
    logs.value = [...all]
  })

  // 初始刷新
  await refreshStatus()
  version.value = OpenListNative.getVersion()

  // 定时刷新
  refreshTimer = setInterval(refreshStatus, 3000)

  // 记录启动日志
  logBuffer.info('OpenList Web UI 已启动')
  if (runtime.value.running) {
    logBuffer.info(`后端运行中，端口 ${runtime.value.port}`)
  } else {
    logBuffer.info('后端未运行')
  }
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
  if (unsubscribeLog) {
    unsubscribeLog()
    unsubscribeLog = null
  }
})

async function refreshStatus() {
  runtime.value = OpenListNative.getStatus()
}

async function toggleService() {
  if (isControlling.value) return
  isControlling.value = true
  try {
    if (runtime.value.running) {
      logBuffer.info('正在停止 OpenList...')
      const ok = OpenListNative.stopOpenList()
      logBuffer[ok ? 'info' : 'error'](ok ? '已停止' : '停止失败')
    } else {
      logBuffer.info('正在启动 OpenList...')
      const port = OpenListNative.startOpenList()
      if (port > 0) {
        logBuffer.info(`已启动，端口 ${port}`)
      } else {
        logBuffer.error('启动失败')
      }
    }
    setTimeout(refreshStatus, 1000)
  } finally {
    isControlling.value = false
  }
}

async function openPasswordDialog() {
  const modal = await modalController.create({
    component: PwdEditDialog,
    componentProps: {
      onConfirm: async (password: string) => {
        logBuffer.info('设置管理员密码...')
        const ok = OpenListNative.setPassword(password)
        logBuffer[ok ? 'info' : 'error'](ok ? '密码已设置' : '设置失败')
      },
    },
  })
  await modal.present()
}

function goToConfig() {
  router.push('/config')
}
function goToWebView() {
  router.push('/webview')
}
function goToSettings() {
  router.push('/settings')
}
</script>

<style scoped>
ion-fab {
  margin-bottom: env(safe-area-inset-bottom, 0);
}
</style>
