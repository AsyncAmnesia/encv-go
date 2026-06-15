<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>{{ t('devlogs.title') }}</ion-title>
      </ion-toolbar>
      <ion-toolbar class="tab-toolbar">
        <ion-segment :value="activeTab" @ionChange="onTabChange">
          <ion-segment-button value="frontend" @click="onTabClick('frontend')">
            {{ t('devlogs.frontend') }}
          </ion-segment-button>
          <ion-segment-button value="backend" @click="onTabClick('backend')">
            {{ t('devlogs.backend') }}
          </ion-segment-button>
        </ion-segment>
      </ion-toolbar>
      <div class="toolbar-row">
        <div class="level-filters">
          <button
            v-for="lvl in levelOptions"
            :key="lvl.value"
            class="level-btn"
            :class="{ active: selectedLevels.has(lvl.value), [lvl.value]: true }"
            @click="toggleLevel(lvl.value)"
          >{{ lvl.label }}</button>
        </div>
        <div class="toolbar-actions">
          <!-- v6 纯手动挡：▶ 跟随 / ⏸ 暂停 开关按钮 -->
          <ion-button
            fill="clear"
            size="small"
            :color="autoScrollEnabled ? 'primary' : 'medium'"
            :title="autoScrollEnabled ? t('devlogs.autoScrollOn') : t('devlogs.autoScrollOff')"
            data-testid="devlogs-auto-scroll-toggle"
            @click="toggleAutoScroll"
          >
            <ion-icon
              :icon="autoScrollEnabled ? pauseOutline : playOutline"
              slot="icon-only"
            ></ion-icon>
          </ion-button>
          <ion-button fill="clear" size="small" @click="handleCopy">
            <ion-icon :icon="copyOutline" slot="icon-only"></ion-icon>
          </ion-button>
          <ion-button fill="clear" size="small" color="danger" @click="handleClear">
            <ion-icon :icon="trashOutline" slot="icon-only"></ion-icon>
          </ion-button>
        </div>
      </div>
      <div class="search-row">
        <ion-searchbar
          v-model="searchText"
          :placeholder="t('devlogs.searchPlaceholder')"
          class="log-searchbar"
          mode="ios"
          :debounce="150"
        ></ion-searchbar>
      </div>
    </ion-header>

    <!--
      v6 纯手动挡：toolbar ▶/⏸ 开关 + 浮动 ↓ 按钮
      详见脚本顶部 autoScrollEnabled 注释
    -->
    <ion-content ref="contentRef" class="log-content">
      <div v-if="activeTab === 'frontend'" class="log-list">
        <div v-if="filteredFrontend.length === 0" class="empty-logs">
          <p>{{ t('devlogs.noLogs') }}</p>
        </div>
        <div
          v-for="log in filteredFrontend"
          :key="log.id"
          class="log-entry"
          :class="[log.level]"
        >
          <span class="log-time">[{{ log.timestamp }}]</span>
          <ion-badge :color="getBadgeColor(log.level)" class="level-badge">{{ log.level.toUpperCase() }}</ion-badge>
          <span class="log-msg" v-html="highlightMatch(log.message, searchText)"></span>
        </div>
      </div>

      <div v-else class="log-list">
        <div class="conn-indicator">
          <ion-badge :color="serverOnline ? 'success' : 'danger'">
            {{ serverOnline ? t('devlogs.connected') : t('devlogs.disconnected') }}
          </ion-badge>
        </div>
        <div v-if="filteredBackend.length === 0" class="empty-logs">
          <p>{{ t('devlogs.noLogs') }}</p>
        </div>
        <div
          v-for="log in filteredBackend"
          :key="log.id"
          class="log-entry"
          :class="[log.level]"
        >
          <span class="log-time">[{{ log.timestamp }}]</span>
          <ion-badge :color="getBadgeColor(log.level)" class="level-badge">{{ log.level.toUpperCase() }}</ion-badge>
          <span class="log-msg" v-html="highlightMatch(log.message, searchText)"></span>
        </div>
      </div>
    </ion-content>

    <!-- 浮动「↓」按钮：v6 移到 ion-content 外部，用 position: fixed 定位视口右下角 -->
    <transition name="fade">
      <button
        v-if="!autoScrollEnabled"
        type="button"
        class="scrollToBottomBtn"
        :title="t('devlogs.scrollToBottom')"
        :aria-label="t('devlogs.scrollToBottom')"
        @click="onJumpToBottom"
      >
        <ion-icon :icon="arrowDownOutline" class="scrollToBottomIcon" />
      </button>
    </transition>

    <ion-footer class="status-bar">
      <ion-toolbar>
        <div class="status-inner">
          <span class="status-text">{{ t('devlogs.total', { total: String(totalCurrent), filtered: String(filteredCurrent) }) }}</span>
          <!-- v6: 显示自动滚动状态（⏸ 暂停 / ▶ 跟随） -->
          <span class="status-text auto-scroll-status" :class="{ paused: !autoScrollEnabled }">
            {{ autoScrollEnabled ? t('devlogs.autoScrollOn') : t('devlogs.autoScrollOff') }}
          </span>
        </div>
      </ion-toolbar>
    </ion-footer>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonContent,
  IonSegment, IonSegmentButton, IonSearchbar, IonButton,
  IonIcon, IonBadge, IonFooter, alertController,
} from '@ionic/vue'
import { trashOutline, copyOutline, arrowDownOutline, playOutline, pauseOutline } from 'ionicons/icons'
import { eventBus } from '@/composables/useEventBus'
import { useI18n } from '@/composables/useI18n'
import { useRealtimeTransport } from '@/composables/useRealtimeTransport'
import { useFrontendLogs, type LogEntry } from '@/composables/useFrontendLogs'
import { showToast } from '@/composables/useToast'
import { copyToClipboard } from '@/composables/useClipboard'
import { checkServerStatus } from '@/api/encv'

const { t } = useI18n()
const transport = useRealtimeTransport()

const activeTab = ref<'frontend' | 'backend'>('frontend')
const searchText = ref('')
// 自动滚动：true 跟随 / false 暂停
// 唯一交互入口：toolbar 开关按钮（toggleAutoScroll）和浮动 ↓ 按钮（onJumpToBottom）
// 纯手动挡：不监听 scroll 事件、不在 tab 切换 / 前后台切换时 auto-disable
// 理由：浏览器预览=手机浏览器无 wheel；项目用 Capacitor 高刷 WebView 90/120Hz，
// @ionScroll/@ionScrollStart 在移动端 + 高刷下完全不可靠
const autoScrollEnabled = ref(true)
const contentRef = ref<InstanceType<typeof IonContent> | null>(null)

const selectedLevels = ref<Set<string>>(new Set(['debug', 'info', 'warn', 'error']))
const levelOptions = [
  { value: 'all', label: t('devlogs.all') },
  { value: 'debug', label: 'DEBUG' },
  { value: 'info', label: 'INFO' },
  { value: 'warn', label: 'WARN' },
  { value: 'error', label: 'ERROR' },
]

function toggleLevel(level: string) {
  const s = new Set(selectedLevels.value)
  if (level === 'all') {
    if (s.has('all')) { s.clear() }
    else { s.add('all'); for (const o of levelOptions) if (o.value !== 'all') s.add(o.value) }
  } else {
    if (s.has(level)) { s.delete(level); s.delete('all') }
    else { s.add(level); if (s.size === levelOptions.length - 1) s.add('all') }
    if (s.size === levelOptions.length - 1) s.add('all')
  }
  selectedLevels.value = s
}

let nextId = 0
const { logs: frontendLogs, clearLogs: clearFrontendLogs } = useFrontendLogs()
const backendLogs = ref<LogEntry[]>([])
const serverOnline = ref(false)

function getBadgeColor(level: string): string {
  switch (level) {
    case 'debug': return 'medium'
    case 'info': return 'success'
    case 'warn': return 'warning'
    case 'error': return 'danger'
    default: return 'medium'
  }
}

function highlightMatch(text: string, query: string): string {
  if (!query.trim()) return text.replace(/</g, '&lt;').replace(/>/g, '&gt;')
  try {
    const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const re = new RegExp(`(${escaped})`, 'gi')
    return text.replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(re, '<mark>$1</mark>')
  } catch { return text }
}

const filteredFrontend = computed(() => {
  let logs = frontendLogs.value
  if (!selectedLevels.value.has('all')) {
    const lvls = Array.from(selectedLevels.value)
    logs = logs.filter((l) => lvls.includes(l.level))
  }
  if (searchText.value) logs = logs.filter((l) => l.message.toLowerCase().includes(searchText.value.toLowerCase()))
  return logs
})

const filteredBackend = computed(() => {
  let logs = backendLogs.value
  if (!selectedLevels.value.has('all')) {
    const lvls = Array.from(selectedLevels.value)
    logs = logs.filter((l) => lvls.includes(l.level))
  }
  if (searchText.value) logs = logs.filter((l) => l.message.toLowerCase().includes(searchText.value.toLowerCase()))
  return logs
})

const totalCurrent = computed(() => activeTab.value === 'frontend' ? frontendLogs.value.length : backendLogs.value.length)
const filteredCurrent = computed(() => activeTab.value === 'frontend' ? filteredFrontend.value.length : filteredBackend.value.length)

/** 重复点击当前 tab 按钮时滚到顶部（VS Code / Chrome DevTools 行为） */
function onTabClick(tab: 'frontend' | 'backend') {
  if (activeTab.value === tab) {
    scrollToTop()
  }
}

function onTabChange(event: CustomEvent) {
  activeTab.value = (event.detail.value || 'frontend') as 'frontend' | 'backend'
}

/**
 * 找 ion-content 实际滚动的元素（每次重查，不缓存）
 * 返回 shadow DOM 内的 .inner-scroll 元素——这是真正可滚动的容器
 * 失败由 scrollToBottom / onJumpToBottom 的 rAF retry 处理
 */
function ensureScrollEl(): HTMLElement | null {
  if (!contentRef.value) return null
  const hostEl = ((contentRef.value as any).$el || (contentRef.value as any)) as HTMLElement | undefined
  if (!hostEl || !hostEl.shadowRoot) return null
  return hostEl.shadowRoot.querySelector('.inner-scroll') as HTMLElement | null
}

function scrollToTop() {
  const el = ensureScrollEl()
  if (el) el.scrollTop = 0
}

/**
 * 滚动到底部（程序化）
 * 单一守卫：autoScrollEnabled=false 时直接 return
 * nextTick + rAF + retry rAF 是为了等 Ionic shadow DOM 异步挂载完成
 * smooth=true 时用 scrollTo 触发平滑滚动；smooth=false 时直接赋值即生效
 */
async function scrollToBottom(smooth = false) {
  if (!autoScrollEnabled.value) return
  await nextTick()
  await new Promise<void>((r) => requestAnimationFrame(() => r()))
  let el = ensureScrollEl()
  if (!el) {
    await new Promise<void>((r) => requestAnimationFrame(() => r()))
    el = ensureScrollEl()
  }
  if (!el) return
  if (smooth) el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
  else el.scrollTop = el.scrollHeight
}

/** 切换自动滚动状态（toolbar ▶/⏸ 开关） */
function toggleAutoScroll() {
  autoScrollEnabled.value = !autoScrollEnabled.value
}

/** 浮动「↓」按钮：开启跟随 + 平滑滚到底 */
async function onJumpToBottom() {
  autoScrollEnabled.value = true
  await scrollToBottom(true)
}

/** 新日志到达的统一处理（被 frontend/backend 两个 watcher 调用） */
function handleNewLog() {
  if (!autoScrollEnabled.value) return
  void scrollToBottom(false)
}

/**
 * 监听前端/后端日志数组长度变化
 * flush:'post' 确保 DOM patch 完成、ion-content shadow DOM 更新后再滚到底
 * （'pre' 触发时 scrollHeight 还没增大，滚不到底）
 * activeTab 切换时仅响应当前 tab 的日志
 */
watch(
  () => frontendLogs.value.length,
  () => {
    if (activeTab.value === 'frontend') handleNewLog()
  },
  { flush: 'post' },
)
watch(
  () => backendLogs.value.length,
  () => {
    if (activeTab.value === 'backend') handleNewLog()
  },
  { flush: 'post' },
)

async function handleCopy() {
  const logs = activeTab.value === 'frontend' ? filteredFrontend.value : filteredBackend.value
  const text = logs.map((l) => `[${l.timestamp}] ${l.level.toUpperCase()} ${l.message}`).join('\n')
  const ok = await copyToClipboard(text)
  if (ok) {
    showToast({
      message: t('devlogs.copied', { count: String(logs.length) }),
      duration: 1500,
      color: 'success',
    })
  } else {
    showToast({ message: t('devlogs.copyFailed'), duration: 1500, color: 'danger' })
  }
}

async function handleClear() {
  const alert = await alertController.create({
    header: t('devlogs.clearConfirm'),
    buttons: [
      { text: t('common.cancel'), role: 'cancel' },
      {
        text: t('common.confirm'), role: 'destructive',
        handler: () => { if (activeTab.value === 'frontend') clearFrontendLogs(); else backendLogs.value = [] },
      },
    ],
  })
  await alert.present()
}

function onWsMessage(data: any) {
  if (data && data.type === 'log' && data.data) {
    const logData = data.data
    const level = ['debug', 'info', 'warn', 'error'].includes(logData.level) ? logData.level : 'info'
    const message = String(logData.message || logData.msg || '')
    if (!message && !logData.message) return
    backendLogs.value.push({
      id: ++nextId,
      timestamp: logData.timestamp || new Date().toLocaleTimeString('zh-CN', { hour12: false }),
      level,
      message,
    })
    return
  }
  if (data && data.type && data.type !== 'log' && data.type !== 'pong' && data.type !== 'server:status') {
    const msg = typeof data === 'string' ? data : JSON.stringify(data)
    backendLogs.value.push({ id: ++nextId, timestamp: new Date().toLocaleTimeString('zh-CN', { hour12: false }), level: 'debug', message: msg })
  }
}

function onServerStatus(data: any) {
  serverOnline.value = data?.online ?? false
}

onMounted(async () => {
  await nextTick()

  // transport 已在 App.vue 启动为 useWebSocket 单例，DevLogs 只读 connectionState 不再 connect
  eventBus.on('ws:message', onWsMessage)
  eventBus.on('server:status', onServerStatus)

  serverOnline.value = transport.connectionState.value === 'connected'
  if (!serverOnline.value) {
    const result = await checkServerStatus()
    serverOnline.value = result.online
  }

  // 写入一条启动日志（INFO/WARN 取决于 server 状态）
  backendLogs.value.push({
    id: ++nextId,
    timestamp: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
    level: serverOnline.value ? 'info' : 'warn',
    message: `DevLogs ready, server ${serverOnline.value ? 'online' : 'offline'} (transport=${transport.connectionState.value})`,
  })
})

onUnmounted(() => {
  eventBus.off('ws:message', onWsMessage)
  eventBus.off('server:status', onServerStatus)
})

/** 暴露给单元测试（生产环境无副作用） */
defineExpose({
  autoScrollEnabled,
  activeTab,
  handleNewLog,
  toggleAutoScroll,
  onJumpToBottom,
  scrollToBottom,
  setActiveTab(tab: 'frontend' | 'backend') { activeTab.value = tab },
})
</script>

<style scoped>
.tab-toolbar {
  --padding-start: 8px;
  --padding-end: 8px;
  --min-height: 44px;
}
.tab-toolbar ion-segment {
  --background: rgba(var(--ion-background-color-rgb, 255, 255, 255), 0.08);
}

.toolbar-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--ion-border-color, rgba(255, 255, 255, 0.08));
  background: var(--ion-background-color);
}

.toolbar-actions {
  display: flex;
  gap: 2px;
  flex-shrink: 0;
}

.search-row {
  padding: 0 10px 4px;
  border-bottom: 1px solid var(--ion-border-color, rgba(255, 255, 255, 0.08));
  background: var(--ion-background-color);
}

.level-filters {
  display: flex;
  gap: 3px;
  flex-shrink: 0;
}

.level-btn {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  border: 1px solid var(--ion-text-color-step-400, #666);
  background: transparent;
  color: var(--ion-text-color-step-400, #666);
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.15s ease;
  letter-spacing: 0.3px;
  font-family: inherit;
}

.level-btn.active.all,
.level-btn.active.debug { background: rgba(136, 136, 136, 0.2); color: #aaa; border-color: #888; }
.level-btn.active.info { background: rgba(46, 204, 113, 0.15); color: #2ecc71; border-color: #2ecc71; }
.level-btn.active.warn { background: rgba(243, 156, 18, 0.15); color: #f39c12; border-color: #f39c12; }
.level-btn.active.error { background: rgba(231, 76, 60, 0.15); color: #e74c3c; border-color: #e74c3c; }

.log-searchbar {
  --border-radius: 12px;
  --background: rgba(255, 255, 255, 0.06);
  --placeholder-color: var(--ion-text-color-step-350, #aaa);
  --color: var(--ion-text-color);
  padding-top: 0;
  padding-bottom: 0;
}
.log-searchbar .searchbar-search-icon { display: none !important; }

.log-content { --background: var(--ion-background-color); }

.log-list {
  font-family: 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.6;
  padding: 4px 6px;
  min-height: 200px;
}

.conn-indicator {
  text-align: center;
  padding: 4px 0 8px;
}

.log-entry {
  display: flex;
  align-items: baseline;
  gap: 5px;
  padding: 1px 2px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
  word-break: break-all;
}

.log-entry.debug .log-msg { color: var(--ion-text-color-step-300, #777); }
.log-entry.info .log-msg { color: var(--ion-text-color, #ddd); }
.log-entry.warn .log-msg { color: #f39c12; }
.log-entry.error .log-msg { color: #e74c3c; }

.log-time {
  color: var(--ion-text-color-step-400, #555);
  white-space: nowrap;
  flex-shrink: 0;
  user-select: none;
  font-size: 11px;
}

.level-badge {
  --padding-start: 4px;
  --padding-end: 4px;
  --padding-top: 0;
  --padding-bottom: 0;
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.5px;
  height: 16px;
  flex-shrink: 0;
}

.log-msg {
  flex: 1;
  min-width: 0;
}
.log-msg :deep(mark) {
  background: rgba(241, 196, 15, 0.35);
  color: inherit;
  border-radius: 2px;
  padding: 0 1px;
}

.empty-logs {
  text-align: center;
  padding: 40px 20px;
  color: var(--ion-text-color-step-400, #555);
  font-size: 13px;
}

.status-bar {
  --background: var(--ion-toolbar-background, rgba(var(--ion-background-color-rgb), 0.92));
  --border-width: 1px 0 0 0;
  backdrop-filter: blur(8px);
}
.status-bar ion-toolbar { --padding-start: 12px; --padding-end: 12px; --min-height: 38px; }

.status-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}
.status-text { font-size: 11px; color: var(--ion-text-color-step-400, #666); }

/* 浮动「↓」按钮：position: fixed 定位视口右下角，z-index 9999 确保永远可见
   bottom: 64px 避开 status-bar（44px + 20px 间距） */
.scrollToBottomBtn {
  position: fixed;
  right: 16px;
  bottom: 64px;
  z-index: 9999;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border: 0;
  border-radius: 50%;
  background: var(--ion-toolbar-background, var(--ion-background-color));
  color: var(--ion-color-primary);
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.18), 0 1px 3px rgba(0, 0, 0, 0.12);
  cursor: pointer;
  padding: 0;
  transition: transform 0.12s, box-shadow 0.12s;
}
.scrollToBottomBtn:hover { transform: scale(1.06); }
.scrollToBottomBtn:active { transform: scale(0.94); }
.scrollToBottomIcon { font-size: 20px; }
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }

/* status-bar 自动滚动状态文字：暂停时 warning 色 */
.auto-scroll-status { font-weight: 500; }
.auto-scroll-status.paused { color: var(--ion-color-warning); }
</style>
