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
          <ion-button @click="openExternal" v-if="!isSandbox">
            <ion-icon :icon="openOutline" slot="icon-only" />
          </ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <!--
        始终渲染 iframe，让用户看到 OpenList SPA 真实 UI。
        沙箱 dev：iframe 走 Vite 代理 /openlist-spa → 127.0.0.1:5244
        真机 prod：iframe 直连 http://127.0.0.1:5244
        后端未启动时，iframe 区域会显示连接失败/空白（用户可自己起 dev-openlist.sh）
      -->
      <iframe
        v-show="!showFallback"
        :src="iframeUrl"
        class="openlist-iframe"
        @error="onError"
        @load="onLoad"
        ref="frameRef"
      ></iframe>

      <!--
        后端未启动时的降级 UI（仅在 sandbox dev 且后端连不上时显示）
        真机模式下不显示（因为 plugin-openlist Content() 内 WebView 与 OpenList
        后端同设备，理论始终可达）
      -->
      <div v-if="showFallback" class="empty-state">
        <ion-icon :icon="cloudOfflineOutline" class="empty-icon" />
        <p class="empty-title">OpenList 后端未运行</p>
        <p class="empty-hint">
          沙箱浏览器模式下，OpenList 后端需单独启动：
        </p>
        <code class="empty-cmd">bash scripts/dev-openlist.sh</code>
        <p class="empty-hint-small">
          启动后端后点击「重试」或刷新此页
        </p>
        <ion-button @click="retry" color="primary" class="ion-margin-top">
          <ion-icon :icon="refreshOutline" slot="start" />
          重试
        </ion-button>
        <ion-button @click="openSettings" fill="clear" size="small" class="ion-margin-top">
          返回设置
        </ion-button>
      </div>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
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
} from '@ionic/vue'
import {
  refreshOutline,
  openOutline,
  cloudOfflineOutline,
} from 'ionicons/icons'
import { OpenListNative, logBuffer } from '@/plugins/openlist-native'

const router = useRouter()

const port = ref(0)
const frameRef = ref<HTMLIFrameElement | null>(null)
const showFallback = ref(false)
const retryCount = ref(0)

/**
 * 是否沙箱开发模式（import.meta.env.PROD 由 Vite 自动注入）
 *  - dev:   Vite dev server，proxy /openlist-spa → 127.0.0.1:5244
 *  - prod:  真机/模拟器 WebView，OpenList 与 Capacitor 同设备 → 直连 127.0.0.1:5244
 */
const isSandbox = computed(() => import.meta.env.DEV)

/**
 * iframe URL 策略：
 *   - 沙箱 dev：/openlist-spa/#/login（Vite proxy 反代到 127.0.0.1:5244）
 *   - 真机 prod：http://127.0.0.1:{port || 5244}/#/login（直接连 OpenList 后端）
 */
const iframeUrl = computed(() => {
  const hash = '#/login'
  if (isSandbox.value) {
    return `/openlist-spa/${hash}`
  }
  return `http://127.0.0.1:${port.value || 5244}/${hash}`
})

onMounted(async () => {
  port.value = OpenListNative.getPort()
  if (isSandbox.value) {
    // 沙箱：主动探测后端可达性
    await probeBackend()
  } else {
    // 真机：window.OpenListNative 存在，假设后端可达
    showFallback.value = false
  }
})

/**
 * 沙箱模式下主动探测 OpenList 后端可达性
 * - 用 no-cors fetch 探测 /openlist-spa/，Vite proxy 会尝试连 127.0.0.1:5244
 * - 不可达 → 触发 onError → showFallback=true
 * - 可达 → 正常渲染 SPA
 */
async function probeBackend() {
  showFallback.value = false
  try {
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), 2000)
    await fetch('/openlist-spa/', {
      method: 'HEAD',
      mode: 'no-cors',
      signal: controller.signal,
    })
    clearTimeout(timer)
    logBuffer.info('OpenList 后端已连接')
  } catch (e) {
    logBuffer.warn('OpenList 后端未连接：' + String(e))
    showFallback.value = true
  }
}

function reload() {
  retryCount.value++
  if (isSandbox.value) {
    probeBackend()
  } else {
    const frame = frameRef.value
    if (frame) {
      frame.src = frame.src
    }
  }
}

function retry() {
  reload()
}

function openExternal() {
  // 真机模式下，让用户能跳出 Capacitor WebView 单独访问 OpenList
  const url = `http://127.0.0.1:${port.value || 5244}/`
  window.open(url, '_blank')
}

function openSettings() {
  router.push('/home')
}

function onError() {
  logBuffer.error('iframe 加载失败')
  if (isSandbox.value) {
    showFallback.value = true
  }
}

function onLoad() {
  logBuffer.info('iframe 加载完成')
  showFallback.value = false
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
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 24px;
  gap: 8px;
  text-align: center;
}
.empty-icon {
  font-size: 56px;
  color: var(--ion-color-medium);
  margin-bottom: 8px;
}
.empty-title {
  font-size: 16px;
  font-weight: 600;
  margin: 0;
}
.empty-hint {
  font-size: 13px;
  color: var(--ion-color-medium);
  margin: 4px 0 8px 0;
}
.empty-hint-small {
  font-size: 12px;
  color: var(--ion-color-medium);
  margin: 4px 0 0 0;
}
.empty-cmd {
  display: inline-block;
  padding: 6px 10px;
  background: var(--ion-color-light);
  border-radius: 4px;
  font-family: monospace;
  font-size: 12px;
  margin: 4px 0;
}
</style>
