<template>
  <ion-app>
    <div v-if="serviceGuardBlocked" class="service-guard-blocked">
      <div class="guard-content">
        <ion-icon :icon="warningOutline" class="guard-icon"></ion-icon>
        <h2>{{ t('app.serviceGuardTitle') }}</h2>
        <p class="guard-message">{{ t('app.serviceGuardMessage') }}</p>
        <code class="guard-detail">{{ serviceGuardDetail }}</code>
        <pre v-if="serviceGuardHint" class="guard-hint">{{ serviceGuardHint }}</pre>
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
import { checkServiceGuard } from '@/api/encv'
import type { ServiceGuardResult } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'

const { initTheme, detectP3Support } = useTheme()
const { t } = useI18n()
const { connect, disconnect } = useWebSocket()

const serviceGuardBlocked = ref(false)
const serviceGuardDetail = ref('')
const serviceGuardHint = ref('')

class ServiceGuardError extends Error {
  code: string
  payload: ServiceGuardResult

  constructor(message: string, code: string, payload: ServiceGuardResult) {
    super(message)
    this.name = 'ServiceGuardError'
    this.code = code
    this.payload = payload
  }
}

async function runServiceGuard(): Promise<void> {
  try {
    await checkServiceGuard()
  } catch (e: any) {
    if (e?.code === 'SERVICE_GUARD_BLOCKED' || e instanceof ServiceGuardError) {
      const payload: ServiceGuardResult = e.payload || {}
      serviceGuardDetail.value = payload.detail || e.message || 'Unknown guard error'
      serviceGuardHint.value = payload.hint || ''
      serviceGuardBlocked.value = true
      throw e
    }
    console.warn('[App] Service guard: API error, allowing entry —', e?.message)
  }
}

async function retryServiceGuard() {
  try {
    await runServiceGuard()
    serviceGuardBlocked.value = false
    connect()
  } catch {
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
  detectP3Support()
  autoInitVConsole()
  registerFileFeature(createAlistEncryptFeature())

  if (!isNative()) {
    try {
      await runServiceGuard()
    } catch {
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

.guard-message {
  white-space: pre-line;
  text-align: left;
  font-size: 13px;
  max-height: 50vh;
  overflow-y: auto;
}

.guard-detail {
  display: block;
  font-size: 12px;
  color: var(--ion-color-danger);
  background: rgba(var(--ion-color-danger-rgb), 0.08);
  border-radius: 8px;
  padding: 10px 14px;
  margin: 0 0 12px;
  text-align: left;
  word-break: break-all;
  white-space: pre-wrap;
}

.guard-hint {
  display: block;
  font-size: 11px;
  color: var(--ion-color-medium);
  background: rgba(var(--ion-color-medium-rgb), 0.06);
  border-radius: 6px;
  padding: 8px 12px;
  margin: 0 0 20px;
  text-align: left;
  white-space: pre-wrap;
  font-family: monospace;
}

.guard-retry-btn {
  --border-radius: 8px;
}
</style>

<style>
/* 通用 ion-toggle 暗黑模式适配 — 非 scoped，作用于所有 toggle */
ion-toggle {
  --track-background: #424242;
  --track-background-checked: var(--ion-color-primary);
  --handle-background: var(--ion-color-primary);
  --handle-background-checked: #ffffff;
}

/* 覆盖 ion-item 内部 .ion-color 上下文导致的 ON 状态手柄变黑 */
ion-toggle.toggle-checked::part(handle) {
  background: #ffffff;
}

/* 背景高斯模糊：ion-content/ion-header/ion-toolbar/卡片 */
ion-content,
ion-header,
ion-toolbar,
.encv-blur-surface {
  --backdrop-filter: blur(var(--encv-bg-blur, 0px));
  backdrop-filter: blur(var(--encv-bg-blur, 0px));
  -webkit-backdrop-filter: blur(var(--encv-bg-blur, 0px));
}

ion-content {
  --background: var(--ion-background-color);
  background: var(--ion-background-color);
}

ion-toolbar {
  --background: rgba(var(--ion-background-color-rgb, 255, 255, 255), 0.85);
  background: rgba(var(--ion-background-color-rgb, 255, 255, 255), 0.85);
}

body.dark ion-toolbar {
  --background: rgba(26, 26, 26, 0.85);
  background: rgba(26, 26, 26, 0.85);
}

/* P3 瑰彩显示：增强颜色饱和度与对比度 */
@media (color-gamut: p3) {
  :root {
    color-scheme: light dark;
  }
  ion-card,
  .preset-card,
  .config-field,
  .task-card,
  .theme-color-picker {
    --encv-color-gamut: p3;
  }
  .p3-enhanced ion-icon {
    color: color(display-p3 1 0 0);
  }
  .p3-enhanced .preset-card-active {
    background: color(display-p3 var(--ion-color-primary-rgb) / 0.08);
  }
}

/* 强制 P3 模式时使用 display-p3 色空间 */
:root {
  --encv-color-gamut: srgb;
}
:root[style*="--encv-color-gamut: display-p3"] {
  --encv-color-gamut: display-p3;
}
</style>
