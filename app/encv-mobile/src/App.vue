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

/* 背景高斯模糊 + 全面透明化设计规范 */
ion-content,
ion-header,
ion-toolbar,
.encv-blur-surface {
  --backdrop-filter: blur(var(--encv-bg-blur, 0px));
  backdrop-filter: blur(var(--encv-bg-blur, 0px));
  -webkit-backdrop-filter: blur(var(--encv-bg-blur, 0px));
}

/* 瑰彩显示：CSS 滤镜增强对比度与饱和度（网页端也生效） */
ion-page {
  filter: var(--encv-vivid-filter, none);
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

/* ===== Tab bar 半透明毛玻璃 + 微光动画 ===== */
ion-tab-bar {
  --background: rgba(var(--ion-background-color-rgb, 255, 255, 255), 0.78);
  --color: var(--ion-text-color);
  --color-selected: var(--ion-color-primary);
  --border: none;
  background: rgba(var(--ion-background-color-rgb, 255, 255, 255), 0.78);
  backdrop-filter: blur(20px) saturate(1.8);
  -webkit-backdrop-filter: blur(20px) saturate(1.8);
  border-top: 1px solid rgba(var(--ion-text-color-rgb), 0.08);
  box-shadow: 0 -4px 20px rgba(0, 0, 0, 0.04);
  position: relative;
  overflow: hidden;
}

body.dark ion-tab-bar {
  --background: rgba(26, 26, 26, 0.85);
  background: rgba(26, 26, 26, 0.85);
}

ion-tab-bar::before {
  content: '';
  position: absolute;
  top: 0;
  left: -50%;
  width: 50%;
  height: 100%;
  background: linear-gradient(90deg,
    transparent,
    rgba(255, 255, 255, 0.08),
    transparent);
  animation: encvTabBarShine 4s ease-in-out infinite;
  pointer-events: none;
  z-index: 0;
}

ion-tab-bar > * {
  position: relative;
  z-index: 1;
}

ion-tab-button {
  --background: transparent;
  --background-focused: rgba(var(--ion-color-primary-rgb), 0.12);
  --background-hover: rgba(var(--ion-color-primary-rgb), 0.06);
  --color: var(--ion-color-medium);
  --color-selected: var(--ion-color-primary);
  transition: color 0.2s ease, transform 0.2s ease;
  background: transparent;
  font-weight: 500;
}

ion-tab-button.tab-selected {
  transform: translateY(-1px);
  font-weight: 600;
}

ion-tab-button ion-icon {
  transition: transform 0.25s cubic-bezier(0.34, 1.56, 0.64, 1);
}

ion-tab-button.tab-selected ion-icon {
  transform: scale(1.15);
  filter: drop-shadow(0 0 4px rgba(var(--ion-color-primary-rgb), 0.4));
}

@keyframes encvTabBarShine {
  0% { left: -50%; }
  100% { left: 100%; }
}

/* ===== 核心透明化规范：ion-item / ion-list / 卡片 ===== */
ion-list {
  --background: transparent;
  background: transparent;
}

ion-item {
  --background: rgba(var(--ion-background-color-rgb, 255, 255, 255), 0.55);
  background: rgba(var(--ion-background-color-rgb, 255, 255, 255), 0.55);
  backdrop-filter: blur(var(--encv-bg-blur, 8px));
  -webkit-backdrop-filter: blur(var(--encv-bg-blur, 8px));
}

body.dark ion-item {
  --background: rgba(30, 30, 30, 0.6);
  background: rgba(30, 30, 30, 0.6);
}

ion-list-header {
  --background: transparent;
  background: rgba(var(--ion-background-color-rgb, 255, 255, 255), 0.3);
}

body.dark ion-list-header {
  background: rgba(30, 30, 30, 0.3);
}

.home-card {
  background: rgba(var(--ion-background-color-rgb, 255, 255, 255), 0.6) !important;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}

body.dark .home-card {
  background: rgba(30, 30, 30, 0.65) !important;
}

.player-card {
  background: linear-gradient(135deg, rgba(var(--ion-color-primary-rgb), 0.12), rgba(var(--ion-color-primary-rgb), 0.04)) !important;
  backdrop-filter: blur(12px);
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

/* 强制 P3 模式：当用户手动开启时，通过 CSS 变量应用 display-p3 色域 */
:root {
  --encv-color-gamut: srgb;
}

/* 当 --encv-color-gamut 为 display-p3 时，强制使用 P3 色彩空间渲染关键元素 */
@supports (color: color(display-p3 1 0 0)) {
  :root:has([style*="--encv-color-gamut: display-p3"]) ion-page,
  :root[style*="--encv-color-gamut: display-p3"] ion-page {
    color-gamut: display-p3;
  }

  :root:has([style*="--encv-color-gamut: display-p3"]) *,
  :root[style*="--encv-color-gamut: display-p3"] * {
    color-gamut: display-p3;
  }
}

/* 降级方案：不支持 :has() 时，用 class 方式触发 */
.encv-force-p3 {
  color-gamut: display-p3 !important;
}
.encv-force-p3 * {
  color-gamut: display-p3 !important;
}

/* ============================================
   ENCV Toast 系统 — 顶部展示 + 堆叠 + Ionic 官方动画
   完全不覆盖 Ionic overlay 布局，只调整视觉外观
   ============================================ */
.encv-toast {
  --background: transparent;
  --box-shadow: none;
  --color: var(--ion-text-color);
  --border-radius: 14px;
}

.encv-toast .toast-wrapper {
  border-radius: 14px;
  padding: 12px 16px;
  display: flex;
  align-items: center;
  gap: 10px;
  box-shadow:
    0 4px 24px rgba(0, 0, 0, 0.12),
    0 1px 4px rgba(0, 0, 0, 0.06);
  margin: 6px 16px 0;
  max-width: 380px;
}

.encv-toast .toast-message {
  font-size: 13.5px;
  font-weight: 500;
  letter-spacing: 0.01em;
  flex: 1;
  color: inherit;
  line-height: 1.4;
}

.encv-toast .toast-button {
  --padding-start: 6px;
  --padding-end: 6px;
  --border-radius: 50%;
  min-width: 28px;
  min-height: 28px;
  font-size: 15px;
  color: var(--ion-color-medium);
  margin-left: 2px;
  flex-shrink: 0;
}

.encv-toast--primary {
  --background: rgba(var(--ion-color-primary-rgb), 0.92);
  --color: #ffffff;
}
body.dark .encv-toast--primary {
  --background: rgba(var(--ion-color-primary-rgb), 0.88);
}

.encv-toast--success {
  --background: rgba(34, 197, 94, 0.92);
  --color: #ffffff;
}

.encv-toast--danger,
.encv-toast--error {
  --background: rgba(239, 68, 68, 0.92);
  --color: #ffffff;
}

.encv-toast--warning {
  --background: rgba(245, 158, 11, 0.92);
  --color: #1a1a1a;
}

.encv-toast--medium {
  --background: rgba(115, 115, 128, 0.9);
  --color: #ffffff;
}
</style>
