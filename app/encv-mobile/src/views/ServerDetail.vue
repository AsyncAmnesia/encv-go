<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button default-href="/tabs/settings"></ion-back-button>
        </ion-buttons>
        <ion-title>{{ t('settings.serverTitle') }}</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <ion-list>
        <ion-list-header>
          <ion-label>{{ t('settings.connection') }}</ion-label>
        </ion-list-header>
        <ion-item>
          <ion-icon :icon="serverIcon" slot="start"></ion-icon>
          <ion-input
            v-model="serverUrl"
            :label="t('settings.serverUrl')"
            label-placement="stacked"
            placeholder="http://127.0.0.1:2025"
            @ionBlur="saveServerUrl"
          ></ion-input>
        </ion-item>
        <ion-item>
          <ion-icon :icon="serverIcon" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.status') }}</h3>
            <p>
              <ion-badge :color="serverOnline ? 'success' : 'danger'">
                {{ serverOnline ? t('settings.online') : t('settings.offline') }}
              </ion-badge>
              <span v-if="serverOnline && backendPort" class="port-info">:{{ backendPort }}</span>
            </p>
            <p v-if="!serverOnline && connectionError" class="connection-error">
              {{ connectionError }}
            </p>
          </ion-label>
          <div slot="end" class="server-controls">
            <ion-button fill="outline" size="small" @click="checkServer">
              <ion-icon :icon="refreshIcon" slot="icon-only"></ion-icon>
            </ion-button>
            <ion-button v-if="serverOnline" fill="outline" size="small" color="danger" @click="handleStop">
              <ion-icon :icon="stopIcon" slot="icon-only"></ion-icon>
            </ion-button>
            <ion-button v-if="!serverOnline" fill="outline" size="small" color="warning" @click="handleRestart">
              <ion-icon :icon="playIcon" slot="icon-only"></ion-icon>
            </ion-button>
          </div>
        </ion-item>
      </ion-list>

      <ion-list v-if="isNativePlatform">
        <ion-list-header>
          <ion-label>{{ t('settings.permissions') }}</ion-label>
        </ion-list-header>
        <ion-item>
          <ion-icon :icon="notificationsIcon" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.notificationPermission') }}</h3>
            <p>{{ permNotifications ? t('settings.granted') : t('settings.denied') }}</p>
          </ion-label>
          <ion-button v-if="!permNotifications" fill="outline" size="small" @click="handleRequestNotification">
            {{ t('settings.request') }}
          </ion-button>
        </ion-item>
        <ion-item>
          <ion-icon :icon="folderOpen" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.storagePermission') }}</h3>
            <p>{{ permStorage ? t('settings.granted') : t('settings.denied') }}</p>
          </ion-label>
          <ion-button v-if="!permStorage" fill="outline" size="small" @click="handleRequestStorage">
            {{ t('settings.request') }}
          </ion-button>
        </ion-item>
      </ion-list>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons, IonBackButton,
  IonContent, IonList, IonListHeader, IonItem, IonIcon, IonLabel,
  IonInput, IonBadge, IonButton, alertController, toastController,
} from '@ionic/vue'
import {
  server as serverIcon, refresh as refreshIcon,
  stop as stopIcon, play as playIcon,
  notifications as notificationsIcon, folderOpen,
} from 'ionicons/icons'
import { useServerStatus } from '@/composables/useServerStatus'
import { useI18n } from '@/composables/useI18n'
import { setApiBaseUrl, getServerUrl } from '@/api/encv'
import { isNative, requestNotificationPermission, requestStoragePermission, checkPermissions } from '@/plugins/GoProcess'

const { isOnline: serverOnline, lastError: connectionError, checkStatus, restartBackend, stopBackend, backendPort } = useServerStatus()
const { t } = useI18n()

const serverUrl = ref(getServerUrl())
const isNativePlatform = ref(isNative())
const permNotifications = ref(false)
const permStorage = ref(false)
let permissionCheckTimer: number | null = null

async function refreshPermissions() {
  const perms = await checkPermissions()
  permNotifications.value = perms.notifications
  permStorage.value = perms.storage
}

async function handleRequestNotification() {
  await requestNotificationPermission()
  // 请求后设置定时器，稍后自动刷新权限状态
  if (permissionCheckTimer) clearTimeout(permissionCheckTimer)
  permissionCheckTimer = window.setTimeout(() => refreshPermissions(), 1000)
  setTimeout(() => refreshPermissions(), 3000)
  setTimeout(() => refreshPermissions(), 5000)
}

async function handleRequestStorage() {
  await requestStoragePermission()
  // 请求后设置定时器，稍后自动刷新权限状态
  if (permissionCheckTimer) clearTimeout(permissionCheckTimer)
  permissionCheckTimer = window.setTimeout(() => refreshPermissions(), 1000)
  setTimeout(() => refreshPermissions(), 3000)
  setTimeout(() => refreshPermissions(), 5000)
}

function saveServerUrl() {
  const url = serverUrl.value.trim()
  if (url) {
    setApiBaseUrl(url)
    checkStatus()
  }
}

async function checkServer() {
  await checkStatus()
  const toast = await toastController.create({
    message: serverOnline.value ? t('settings.serverOnline') : t('settings.serverOffline'),
    duration: 1500,
    color: serverOnline.value ? 'success' : 'danger',
  })
  await toast.present()
}

async function handleRestart() {
  const toast = await toastController.create({
    message: t('settings.restarting'),
    duration: 30000,
  })
  await toast.present()
  const success = await restartBackend()
  await toast.dismiss()
  const result = await toastController.create({
    message: success ? t('settings.restartSuccess') : t('settings.restartFailed'),
    duration: 2000,
    color: success ? 'success' : 'danger',
  })
  await result.present()
}

async function handleStop() {
  const alert = await alertController.create({
    header: t('settings.stopConfirm'),
    buttons: [
      { text: t('settings.cancel'), role: 'cancel' },
      {
        text: t('settings.stop'),
        role: 'destructive',
        handler: async () => {
          const success = await stopBackend()
          const toast = await toastController.create({
            message: success ? t('settings.stopped') : t('settings.stopFailed'),
            duration: 2000,
            color: success ? 'success' : 'danger',
          })
          await toast.present()
        },
      },
    ],
  })
  await alert.present()
}

onMounted(async () => {
  if (isNativePlatform.value) {
    await refreshPermissions()
  }
})

onUnmounted(() => {
  if (permissionCheckTimer) clearTimeout(permissionCheckTimer)
})
</script>

<style scoped>
.connection-error {
  color: var(--ion-color-danger);
  font-size: 12px;
  margin-top: 4px;
}
.port-info {
  font-size: 12px;
  opacity: 0.7;
  margin-left: 4px;
}
.server-controls {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}
</style>
