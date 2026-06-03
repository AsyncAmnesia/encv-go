<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button default-href="/home" />
        </ion-buttons>
        <ion-title>OpenList Web UI</ion-title>
        <ion-buttons slot="end">
          <!-- 状态指示器（点一下重试）-->
          <ion-button @click="reload" :disabled="state === 'probing'">
            <ion-icon
              :icon="stateIcon"
              :color="stateColor"
              slot="icon-only"
            />
          </ion-button>
          <ion-button @click="openExternal" v-if="!isSandbox && state === 'connected'">
            <ion-icon :icon="openOutline" slot="icon-only" />
          </ion-button>
        </ion-buttons>
      </ion-toolbar>

      <!-- 状态条（带连接状态 + 最后一次错误） -->
      <ion-toolbar
        v-if="state === 'error' || state === 'timeout' || state === 'probing'"
        :color="stateColor"
        class="status-bar"
      >
        <ion-title size="small" class="status-title">
          <ion-spinner
            v-if="state === 'probing'"
            name="crescent"
            class="status-spinner"
          />
          <ion-icon
            v-else-if="state === 'error'"
            :icon="alertCircleOutline"
            class="status-icon-inline"
          />
          <ion-icon
            v-else-if="state === 'timeout'"
            :icon="timerOutline"
            class="status-icon-inline"
          />
          {{ stateText }}
        </ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <!-- iframe 永远在 DOM 里（让浏览器开始加载），覆盖层控制视觉 -->
      <iframe
        v-show="state === 'connected' || state === 'loading'"
        :src="iframeUrl"
        class="openlist-iframe"
        :class="{ 'iframe-loading': state === 'loading' }"
        @error="onError"
        @load="onLoad"
        ref="frameRef"
        sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
      ></iframe>

      <!--
        防御性状态 UI 覆盖层
        覆盖在 iframe 之上，遮挡加载/错误/超时/探测的视觉空白
      -->
      <div
        v-if="state !== 'connected' && state !== 'loading'"
        class="state-overlay"
      >
        <!-- 探测中 -->
        <div v-if="state === 'probing'" class="state-card state-probing">
          <ion-spinner name="crescent" class="state-spinner" />
          <p class="state-title">正在连接 OpenList 后端…</p>
          <p class="state-hint">127.0.0.1:5244（{{ isSandbox ? 'Vite proxy' : 'localhost' }}）</p>
        </div>

        <!-- 错误：连接失败/502/404 -->
        <div v-else-if="state === 'error'" class="state-card state-error">
          <ion-icon :icon="cloudOfflineOutline" class="state-icon" />
          <p class="state-title">OpenList 后端未运行</p>
          <p class="state-hint">{{ lastError || '后端不可达（连接被拒绝或 502）' }}</p>

          <div class="state-command-block">
            <p class="state-hint-small">在另一个终端启动 OpenList 后端：</p>
            <code class="state-cmd">bash scripts/dev-openlist.sh</code>
          </div>

          <div v-if="retryCount > 0" class="state-retry-info">
            已重试 {{ retryCount }} 次
          </div>

          <div class="state-actions">
            <ion-button @click="reload" color="primary">
              <ion-icon :icon="refreshOutline" slot="start" />
              重试
            </ion-button>
            <ion-button @click="copyCommand" fill="clear" size="default">
              <ion-icon :icon="copyOutline" slot="start" />
              复制启动命令
            </ion-button>
          </div>
        </div>

        <!-- 超时：连接慢/被防火墙挡 -->
        <div v-else-if="state === 'timeout'" class="state-card state-timeout">
          <ion-icon :icon="timerOutline" class="state-icon" />
          <p class="state-title">连接超时</p>
          <p class="state-hint">OpenList 后端响应超过 5 秒</p>

          <div class="state-actions">
            <ion-button @click="reload" color="primary">
              <ion-icon :icon="refreshOutline" slot="start" />
              再试一次
            </ion-button>
          </div>
        </div>
      </div>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
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
  IonSpinner,
  toastController,
} from '@ionic/vue'
import {
  refreshOutline,
  openOutline,
  cloudOfflineOutline,
  alertCircleOutline,
  timerOutline,
  checkmarkCircleOutline,
  copyOutline,
} from 'ionicons/icons'
import { OpenListNative, logBuffer } from '@/plugins/openlist-native'

type IframeState = 'probing' | 'loading' | 'connected' | 'error' | 'timeout'

const router = useRouter()

const port = ref(0)
const state = ref<IframeState>('probing')
const lastError = ref('')
const retryCount = ref(0)
const frameRef = ref<HTMLIFrameElement | null>(null)

const PROBE_TIMEOUT_MS = 5000

/**
 * 沙箱 dev / 真机 prod 区分
 *  - dev:   走 Vite proxy /openlist-spa → 127.0.0.1:5244
 *  - prod:  直连 127.0.0.1:5244（同设备，OpenList 与 Capacitor 同进程域）
 */
const isSandbox = computed(() => import.meta.env.DEV)

const iframeUrl = computed(() => {
  const hash = '#/login'
  if (isSandbox.value) {
    return `/openlist-spa/${hash}`
  }
  return `http://127.0.0.1:${port.value || 5244}/${hash}`
})

const stateText = computed(() => {
  switch (state.value) {
    case 'probing': return '连接中…'
    case 'error': return '连接失败'
    case 'timeout': return '连接超时'
    default: return ''
  }
})

const stateColor = computed(() => {
  switch (state.value) {
    case 'connected': return 'success'
    case 'error': return 'danger'
    case 'timeout': return 'warning'
    case 'probing': return 'medium'
    default: return 'primary'
  }
})

const stateIcon = computed(() => {
  switch (state.value) {
    case 'connected': return checkmarkCircleOutline
    case 'error': return cloudOfflineOutline
    case 'timeout': return timerOutline
    case 'probing': return refreshOutline
    case 'loading': return refreshOutline
    default: return refreshOutline
  }
})

onMounted(async () => {
  port.value = OpenListNative.getPort()
  if (isSandbox.value) {
    await probeBackend()
  } else {
    // 真机模式：OpenList 与 Capacitor 同设备，假设后端可达
    state.value = 'loading'
  }
})

onUnmounted(() => {
  // 清理：可以在这里 abort 任何 in-flight 请求
})

/**
 * 沙箱后端可达性探测
 * - 区分 error（连接被拒/502）和 timeout（超时）
 * - 探测成功后切到 loading，等待 iframe @load 切到 connected
 */
async function probeBackend() {
  state.value = 'probing'
  lastError.value = ''

  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), PROBE_TIMEOUT_MS)

  try {
    const start = Date.now()
    const res = await fetch('/openlist-spa/api/public/settings', {
      method: 'HEAD',
      mode: 'cors',
      signal: controller.signal,
    })
    clearTimeout(timer)
    const elapsed = Date.now() - start

    if (res.status >= 500) {
      // 502/503/504 等：Vite proxy 通但后端拒
      state.value = 'error'
      lastError.value = `后端返回 ${res.status}（${elapsed}ms）`
      logBuffer.error(lastError.value)
    } else {
      // 200/302/404 都算"后端活着"（404 也意味着 OpenList 在响应）
      state.value = 'loading'
      logBuffer.info(`OpenList 后端已连接（${elapsed}ms, status=${res.status}）`)
    }
  } catch (e: any) {
    clearTimeout(timer)
    if (e?.name === 'AbortError') {
      state.value = 'timeout'
      lastError.value = `超过 ${PROBE_TIMEOUT_MS}ms 未响应`
      logBuffer.warn('OpenList 后端探测超时')
    } else {
      state.value = 'error'
      lastError.value = String(e?.message || e)
      logBuffer.error('OpenList 后端探测失败：' + lastError.value)
    }
  }
}

function reload() {
  retryCount.value++
  if (isSandbox.value) {
    probeBackend()
  } else {
    const frame = frameRef.value
    if (frame) {
      // 真机模式：直接重载 iframe
      state.value = 'loading'
      const oldSrc = frame.src
      frame.src = 'about:blank'
      nextTick(() => {
        frame.src = oldSrc
      })
    }
  }
}

function onError() {
  logBuffer.error('iframe 加载失败')
  if (isSandbox.value) {
    state.value = 'error'
    lastError.value = 'iframe 加载失败'
  }
}

function onLoad() {
  logBuffer.info('iframe 加载完成')
  state.value = 'connected'
}

function openExternal() {
  // 真机模式：跳出 Capacitor WebView 单独打开
  const url = `http://127.0.0.1:${port.value || 5244}/`
  window.open(url, '_blank')
}

async function copyCommand() {
  const cmd = 'bash scripts/dev-openlist.sh'
  try {
    await navigator.clipboard.writeText(cmd)
    const toast = await toastController.create({
      message: '启动命令已复制到剪贴板',
      duration: 1800,
      position: 'bottom',
    })
    await toast.present()
  } catch {
    logBuffer.warn('剪贴板复制失败')
  }
}
</script>

<style scoped>
.openlist-iframe {
  width: 100%;
  height: 100%;
  border: none;
  display: block;
  background: #fff;
}
.iframe-loading {
  opacity: 0.6;
}

.state-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--ion-background-color, #ffffff);
  z-index: 10;
  padding: 24px;
}

.state-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  max-width: 360px;
  gap: 8px;
}

.state-spinner {
  width: 48px;
  height: 48px;
  margin-bottom: 12px;
  color: var(--ion-color-medium);
}

.state-icon {
  font-size: 56px;
  margin-bottom: 8px;
}
.state-error .state-icon { color: var(--ion-color-danger); }
.state-timeout .state-icon { color: var(--ion-color-warning); }

.state-title {
  font-size: 17px;
  font-weight: 600;
  margin: 0;
  color: var(--ion-text-color, #000);
}
.state-hint {
  font-size: 13px;
  color: var(--ion-color-medium);
  margin: 0 0 4px 0;
  line-height: 1.5;
  word-break: break-word;
}
.state-hint-small {
  font-size: 12px;
  color: var(--ion-color-medium);
  margin: 0 0 4px 0;
}

.state-command-block {
  margin: 12px 0 4px 0;
  padding: 12px 16px;
  background: var(--ion-color-light);
  border-radius: 8px;
  width: 100%;
  box-sizing: border-box;
}

.state-cmd {
  display: inline-block;
  padding: 6px 10px;
  background: var(--ion-background-color, #fff);
  border: 1px solid var(--ion-color-light-shade, #e0e0e0);
  border-radius: 4px;
  font-family: monospace;
  font-size: 12px;
  color: var(--ion-text-color, #000);
  user-select: all;
  word-break: break-all;
}

.state-retry-info {
  font-size: 11px;
  color: var(--ion-color-medium);
  margin-top: 4px;
}

.state-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: center;
  margin-top: 12px;
}

.status-bar {
  --background: var(--ion-color-light);
  --color: var(--ion-color-medium);
}
.status-bar[color="danger"] {
  --background: var(--ion-color-danger);
  --color: #fff;
}
.status-bar[color="warning"] {
  --background: var(--ion-color-warning);
  --color: #000;
}

.status-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 500;
}
.status-spinner {
  width: 14px;
  height: 14px;
}
.status-icon-inline {
  font-size: 14px;
}
</style>
