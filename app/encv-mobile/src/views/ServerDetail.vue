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
          <ion-label class="ion-text-wrap">
            <h3>{{ t('settings.serverUrl') }}</h3>
            <p class="readonly-url" @click="copyToClipboard(serverUrl)">{{ serverUrl }}</p>
          </ion-label>
          <ion-button slot="end" fill="clear" size="small" @click="copyToClipboard(serverUrl)">
            <ion-icon :icon="copyIcon" slot="icon-only"></ion-icon>
          </ion-button>
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
            <ion-button v-if="isRestarting" fill="outline" size="small" color="medium" disabled>
              <ion-spinner slot="icon-only" name="crescent"></ion-spinner>
            </ion-button>
            <ion-button v-else-if="serverOnline" fill="outline" size="small" color="danger" @click="handleStop" :disabled="isStopping">
              <ion-spinner v-if="isStopping" slot="icon-only" name="crescent"></ion-spinner>
              <ion-icon v-else :icon="stopIcon" slot="icon-only"></ion-icon>
            </ion-button>
            <ion-button v-else fill="outline" size="small" color="warning" @click="handleRestart">
              <ion-icon :icon="playIcon" slot="icon-only"></ion-icon>
            </ion-button>
          </div>
        </ion-item>
      </ion-list>

      <ion-list v-if="serviceUrls.length > 0">
        <ion-list-header>
          <ion-label>{{ t('settings.serviceAddresses') }}</ion-label>
        </ion-list-header>
        <ion-item v-for="svc in serviceUrls" :key="svc.label">
          <ion-icon :icon="svc.icon" slot="start"></ion-icon>
          <ion-label class="ion-text-wrap">
            <h3>{{ svc.label }}</h3>
            <p class="readonly-url" @click="copyToClipboard(svc.url)">{{ svc.url }}</p>
          </ion-label>
          <ion-button v-if="svc.isWebdav" slot="end" fill="clear" size="small" @click="handleTestWebdav" :disabled="webdavTesting">
            <ion-spinner v-if="webdavTesting" slot="icon-only" name="crescent"></ion-spinner>
            <ion-icon v-else :icon="pulseIcon" slot="icon-only"></ion-icon>
          </ion-button>
          <ion-button v-else slot="end" fill="clear" size="small" @click="copyToClipboard(svc.url)">
            <ion-icon :icon="copyIcon" slot="icon-only"></ion-icon>
          </ion-button>
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
        <ion-item>
          <ion-icon :icon="batteryOptimizationIcon" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.batteryOptimization') }}</h3>
            <p>{{ permBatteryOpt ? t('settings.granted') : t('settings.denied') }}</p>
          </ion-label>
          <ion-button v-if="!permBatteryOpt" fill="outline" size="small" @click="handleRequestBatteryOpt">
            {{ t('settings.request') }}
          </ion-button>
        </ion-item>
      </ion-list>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons, IonBackButton,
  IonContent, IonList, IonListHeader, IonItem, IonIcon, IonLabel,
  IonBadge, IonButton, alertController, IonSpinner,
} from '@ionic/vue'
import {
  server as serverIcon, refresh as refreshIcon,
  stop as stopIcon, play as playIcon,
  notifications as notificationsIcon, folderOpen,
  copy as copyIcon, shieldCheckmark, cloudOutline, globeOutline,
  batteryCharging as batteryOptimizationIcon,
  pulse as pulseIcon,
} from 'ionicons/icons'
import { useServerStatus } from '@/composables/useServerStatus'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import { getServerUrl, fetchConfig, testLocalWebDAV } from '@/api/encv'
import { isNative, requestNotificationPermission, requestStoragePermission, requestBatteryOptimization, checkPermissions } from '@/plugins/GoProcess'

interface ServiceUrl {
  label: string
  url: string
  icon: string
  isWebdav?: boolean
}

const {
  isOnline: serverOnline,
  lastError: connectionError,
  checkStatus,
  restartBackend,
  stopBackend,
  backendPort,
  isRestarting,
  isStopping,
} = useServerStatus()
const { t } = useI18n()

const serverUrl = ref(getServerUrl())
const isNativePlatform = ref(isNative())
const permNotifications = ref(false)
const permStorage = ref(false)
const permBatteryOpt = ref(false)
const configData = ref<Record<string, unknown> | null>(null)
const webdavTesting = ref(false)
let permissionCheckTimer: number | null = null

const serviceUrls = computed<ServiceUrl[]>(() => {
  if (!configData.value || !serverOnline.value) return []
  const result: ServiceUrl[] = []
  const cfg = configData.value
  const baseUrl = serverUrl.value

  result.push({
    label: t('settings.httpServerSettings'),
    url: baseUrl,
    icon: cloudOutline,
  })

  const adminCfg = cfg.admin as Record<string, unknown> | undefined
  if (adminCfg) {
    result.push({
      label: t('settings.adminServerSettings'),
      url: `${baseUrl}/admin`,
      icon: shieldCheckmark,
    })
  }

  const webdavCfg = cfg.webdav as Record<string, unknown> | undefined
  if (webdavCfg && typeof webdavCfg.root === 'string' && webdavCfg.root) {
    const root = webdavCfg.root.startsWith('/') ? webdavCfg.root : '/' + webdavCfg.root
    result.push({
      label: t('settings.webdavServerSettings'),
      url: `${baseUrl}${root}`,
      icon: globeOutline,
      isWebdav: true,
    })
  }

  return result
})

async function copyToClipboard(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    showToast({ message: t('remote.copied'), duration: 1000, color: 'success' })
  } catch {
    showToast({ message: t('devlogs.copyFailed'), duration: 1500, color: 'danger' })
  }
}

async function refreshPermissions() {
  const perms = await checkPermissions()
  permNotifications.value = perms.notifications
  permStorage.value = perms.storage
  permBatteryOpt.value = perms.batteryOptimization
}

async function handleRequestNotification() {
  await requestNotificationPermission()
  if (permissionCheckTimer) clearTimeout(permissionCheckTimer)
  permissionCheckTimer = window.setTimeout(() => refreshPermissions(), 1000)
  setTimeout(() => refreshPermissions(), 3000)
  setTimeout(() => refreshPermissions(), 5000)
}

async function handleRequestStorage() {
  await requestStoragePermission()
  if (permissionCheckTimer) clearTimeout(permissionCheckTimer)
  permissionCheckTimer = window.setTimeout(() => refreshPermissions(), 1000)
  setTimeout(() => refreshPermissions(), 3000)
  setTimeout(() => refreshPermissions(), 5000)
}

async function handleRequestBatteryOpt() {
  await requestBatteryOptimization()
  if (permissionCheckTimer) clearTimeout(permissionCheckTimer)
  permissionCheckTimer = window.setTimeout(() => refreshPermissions(), 1000)
  setTimeout(() => refreshPermissions(), 3000)
  setTimeout(() => refreshPermissions(), 5000)
}

async function handleTestWebdav() {
  webdavTesting.value = true
  try {
    const result = await testLocalWebDAV()
    if (!result.available) {
      showToast({
        message: t('settings.webdavTestFailed') + ': ' + (result.error || 'WebDAV not enabled'),
        duration: 4000,
        color: 'danger',
      })
      return
    }
    const details = result.details
    if (details && details.propfindRoot === 'ok' && details.dirReadable === 'ok') {
      const authInfo = result.authRequired
        ? (details.authWorks === 'ok' ? ' ✅' : ' ❌')
        : ''
      showToast({
        message: t('settings.webdavTestSuccess') + authInfo,
        duration: 3000,
        color: 'success',
      })
    } else {
      const parts: string[] = []
      if (details) {
        if (details.propfindRoot !== 'ok') parts.push('PROPFIND: ❌')
        if (details.authWorks === 'fail') parts.push('Auth: ❌')
        if (details.dirReadable !== 'ok') parts.push('Dir: ❌')
      }
      showToast({
        message: t('settings.webdavTestFailed') + (parts.length ? ' ' + parts.join(' ') : ''),
        duration: 5000,
        color: 'warning',
      })
    }
  } catch (e) {
    const detail = e instanceof Error ? e.message : String(e)
    showToast({
      message: t('settings.webdavTestFailed') + ': ' + detail,
      duration: 4000,
      color: 'danger',
    })
  } finally {
    webdavTesting.value = false
  }
}

async function checkServer() {
  await checkStatus()
  showToast({
    message: serverOnline.value ? t('settings.serverOnline') : t('settings.serverOffline'),
    duration: 1500,
    color: serverOnline.value ? 'success' : 'danger',
  })
}

async function handleRestart() {
  const toast = await showToast({
    message: t('settings.restarting'),
    duration: 30000,
  })
  const success = await restartBackend()
  await toast.dismiss()
  showToast({
    message: success ? t('settings.restartSuccess') : t('settings.restartFailed'),
    duration: 2000,
    color: success ? 'success' : 'danger',
  })
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
          showToast({
            message: success ? t('settings.stopped') : t('settings.stopFailed'),
            duration: 2000,
            color: success ? 'success' : 'danger',
          })
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
  if (serverOnline.value) {
    try {
      configData.value = await fetchConfig()
    } catch {}
  }
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
.readonly-url {
  font-family: 'Courier New', Courier, monospace;
  font-size: 13px;
  color: var(--ion-color-primary);
  word-break: break-all;
  cursor: pointer;
  user-select: all;
}
</style>
