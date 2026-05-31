<template>
  <ion-app>
    <div v-if="serviceGuardBlocked" class="service-guard-blocked">
      <div class="guard-content">
        <ion-icon :icon="warningOutline" class="guard-icon"></ion-icon>
        <h2>{{ t('app.serviceGuardTitle') }}</h2>
        <p>{{ t('app.serviceGuardMessage') }}</p>
        <code class="guard-detail">{{ serviceGuardDetail }}</code>
        <ion-button @click="retryServiceGuard" class="guard-retry-btn">
          <ion-icon :icon="refreshOutline" slot="start"></ion-icon>
          {{ t('app.serviceGuardRetry') }}
        </ion-button>
      </div>
    </div>
    <ion-router-outlet v-else />
  </ion-app>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { IonApp, IonRouterOutlet, IonIcon, IonButton } from '@ionic/vue'
import { warningOutline, refreshOutline } from 'ionicons/icons'
import { useTheme } from '@/composables/useTheme'
import { useWebSocket } from '@/composables/useWebSocket'
import { isNative, requestNotificationPermission, requestStoragePermission } from '@/plugins/GoProcess'
import { hijackConsole } from '@/composables/useFrontendLogs'
import { autoInitVConsole } from '@/composables/useDevTools'
import { registerFileFeature } from '@/composables/useFileFeatures'
import { createAlistEncryptFeature } from '@/features/alist-encrypt'
import { listFiles } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'

const { initTheme } = useTheme()
const { t } = useI18n()
const { connect, disconnect } = useWebSocket()

const serviceGuardBlocked = ref(false)
const serviceGuardDetail = ref('')

const MOCK_DIR_MARKER = '01-plain-media'

async function checkServiceDirectory(): Promise<boolean> {
  try {
    const files = await listFiles('/')
    const hasMarker = files.some(f => f.name === MOCK_DIR_MARKER && f.isDirectory)
    if (!hasMarker) {
      const dirNames = files.slice(0, 10).map(f => f.name).join(', ')
      serviceGuardDetail.value = `server.dir missing "${MOCK_DIR_MARKER}", got: [${dirNames}]`
      return false
    }
    return true
  } catch (e: any) {
    serviceGuardDetail.value = e?.message || String(e)
    return false
  }
}

async function retryServiceGuard() {
  const ok = await checkServiceDirectory()
  if (ok) {
    serviceGuardBlocked.value = false
    connect()
  }
}

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
  initTheme()
  autoInitVConsole()
  registerFileFeature(createAlistEncryptFeature())

  if (!isNative()) {
    const ok = await checkServiceDirectory()
    if (!ok) {
      serviceGuardBlocked.value = true
      console.error('[App] Service guard: blocked — mock service directory not detected')
      return
    }
  }

  connect()
  await requestEssentialPermissions()
  applyScreenOrientation()
})

onUnmounted(() => {
  disconnect()
})
</script>

<style scoped>
.service-guard-blocked {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  width: 100%;
  background: var(--ion-background-color);
  padding: 24px;
}

.guard-content {
  text-align: center;
  max-width: 400px;
}

.guard-icon {
  font-size: 64px;
  color: var(--ion-color-warning);
  margin-bottom: 16px;
}

.guard-content h2 {
  font-size: 20px;
  font-weight: 700;
  color: var(--ion-text-color);
  margin: 0 0 8px;
}

.guard-content p {
  font-size: 14px;
  color: var(--encv-text-secondary);
  margin: 0 0 16px;
  line-height: 1.5;
}

.guard-detail {
  display: block;
  font-size: 12px;
  color: var(--ion-color-danger);
  background: rgba(var(--ion-color-danger-rgb), 0.08);
  border-radius: 8px;
  padding: 10px 14px;
  margin: 0 0 20px;
  text-align: left;
  word-break: break-all;
  white-space: pre-wrap;
}

.guard-retry-btn {
  --border-radius: 8px;
}
</style>
