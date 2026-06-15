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
    <ion-content ref="contentRef" class="log-content" @scroll="onLogScroll">
      <!--
        🆕 虚拟滚动：ion-content 的 scroll 事件触发 VirtualLogList 重算可见 items
        DOM 节点数恒定 ~30（视口内 + overscan），切 tab 成本 = O(visible) 而非 O(N)
      -->
      <div v-if="activeTab === 'frontend'" class="log-list">
        <div v-if="filteredFrontend.length === 0" class="empty-logs">
          <p>{{ t('devlogs.noLogs') }}</p>
        </div>
        <VirtualLogList v-else :items="filteredFrontend" :scroll-el="scrollEl">
          <template #default="{ item }">
            <span class="log-time">[{{ item.timestamp }}]</span>
            <ion-badge :color="getBadgeColor(item.level)" class="level-badge">{{ item.level.toUpperCase() }}</ion-badge>
            <span class="log-msg" v-html="highlightMatch(item.message, searchText)"></span>
          </template>
        </VirtualLogList>
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
        <VirtualLogList v-else :items="filteredBackend" :scroll-el="scrollEl">
          <template #default="{ item }">
            <span class="log-time">[{{ item.timestamp }}]</span>
            <ion-badge :color="getBadgeColor(item.level)" class="level-badge">{{ item.level.toUpperCase() }}</ion-badge>
            <span class="log-msg" v-html="highlightMatch(item.message, searchText)"></span>
          </template>
        </VirtualLogList>
      </div>
    </ion-content>

    <!--
      浮动「↑/↓」按钮组：v6.1 加 ↑ 滚顶按钮，对称 ↓ 重启跟随+滚底
      两者独立条件：
        - ↑ 滚顶：scrollTop > 阈值（约 200px）时显示，点击 = ion-content.scrollTop = 0
        - ↓ 滚底：autoScrollEnabled=false 时显示（已在 v6 落地）
    -->
    <div class="scroll-buttons">
      <transition name="fade">
        <button
          v-if="showScrollToTop"
          type="button"
          class="scrollToTopBtn"
          :title="t('devlogs.scrollToTop')"
          :aria-label="t('devlogs.scrollToTop')"
          @click="onJumpToTop"
        >
          <ion-icon :icon="arrowUpOutline" class="scrollToTopIcon" />
        </button>
      </transition>
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
    </div>

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
import { ref, shallowRef, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonContent,
  IonSegment, IonSegmentButton, IonSearchbar, IonButton,
  IonIcon, IonBadge, IonFooter, alertController,
} from '@ionic/vue'
import { trashOutline, copyOutline, arrowDownOutline, arrowUpOutline, playOutline, pauseOutline } from 'ionicons/icons'
import VirtualLogList from '@/components/VirtualLogList.vue'
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
// 🆕 2026-06-14 性能优化：shallowRef + buffer cap 5000 + rAF coalesce
// 解决"后端持续刷新大量日志时切 tab 卡 1-2 秒"问题
//   - shallowRef：避免对每条 log 的深响应（5000 items × Vue proxy 性能差）
//   - buffer cap：超过 5000 自动丢弃最早的，防止 OOM
//   - rAF coalesce：同一帧内多条 WS 消息合并为 1 次赋值 → 1 次 virtualizer 重算
// 自动滚动：true 跟随 / false 暂停
// 唯一交互入口：toolbar 开关按钮（toggleAutoScroll）和浮动 ↓ 按钮（onJumpToBottom）
// 纯手动挡：不监听 scroll 事件、不在 tab 切换 / 前后台切换时 auto-disable
// 理由：浏览器预览=手机浏览器无 wheel；项目用 Capacitor 高刷 WebView 90/120Hz，
// @ionScroll/@ionScrollStart 在移动端 + 高刷下完全不可靠
const autoScrollEnabled = ref(true)
const contentRef = ref<InstanceType<typeof IonContent> | null>(null)
/** ion-content 的 .inner-scroll 元素（虚拟列表的 scroll 容器） */
const scrollEl = ref<HTMLElement | null>(null)
/** 队列未 flush 的后端日志（rAF 内合并） */
let pendingBackendLogs: LogEntry[] = []
/** rAF flush 调度标志 */
let flushScheduled = false
/** 后端日志 buffer 上限（超出后丢弃最早的） */
const MAX_BACKEND_LOGS = 5000

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
// 🆕 性能优化：shallowRef 避免对 5000 条 log 内部字段做深响应代理
// 配合下文的 buffer cap + rAF coalesce，单帧多条 WS 消息只触发 1 次 virtualizer 重算
const backendLogs = shallowRef<LogEntry[]>([])
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
 * 同步更新 scrollEl ref——虚拟列表的 useVirtualizer 通过 getScrollElement 观察它
 * 失败由 scrollToBottom / onJumpToBottom 的 rAF retry 处理
 */
function ensureScrollEl(): HTMLElement | null {
  if (!contentRef.value) return null
  const hostEl = ((contentRef.value as any).$el || (contentRef.value as any)) as HTMLElement | undefined
  if (!hostEl || !hostEl.shadowRoot) return null
  const el = hostEl.shadowRoot.querySelector('.inner-scroll') as HTMLElement | null
  if (el && el !== scrollEl.value) scrollEl.value = el
  return el
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

/** 浮动「↑」按钮：滚到顶部（不影响 autoScrollEnabled 状态） */
function onJumpToTop() {
  const el = ensureScrollEl()
  if (el) el.scrollTop = 0
}

/** 浮动「↑」按钮显示条件：滚离顶部 200px 以上时显示，避免无意义闪烁 */
const showScrollToTop = ref(false)
/** 跟踪 ion-content 滚动以控制 ↑ 按钮显示 */
function onLogScroll() {
  const el = ensureScrollEl()
  if (!el) { showScrollToTop.value = false; return }
  showScrollToTop.value = el.scrollTop > 200
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

/**
 * 🆕 性能优化：rAF coalesce 后端日志
 * 把单帧内多条 WS 消息合并为 1 次 shallowRef 赋值，避免触发 N 次 virtualizer 重算
 * 100 条/秒 WS 持续刷新：旧实现 = 100 次重算/秒；新实现 = 60 次重算/秒（每帧一次）
 */
function queueBackendLog(entry: LogEntry) {
  pendingBackendLogs.push(entry)
  if (flushScheduled) return
  flushScheduled = true
  requestAnimationFrame(flushPendingBackendLogs)
}

function flushPendingBackendLogs() {
  flushScheduled = false
  if (pendingBackendLogs.length === 0) return
  const toAdd = pendingBackendLogs
  pendingBackendLogs = []

  const arr = backendLogs.value
  // 超出 cap：丢弃最早（slice 末尾 keep 条 + 本帧新增）
  if (arr.length + toAdd.length > MAX_BACKEND_LOGS) {
    const keep = Math.max(0, MAX_BACKEND_LOGS - toAdd.length)
    backendLogs.value = arr.length > keep ? [...arr.slice(-keep), ...toAdd] : [...toAdd]
  } else {
    backendLogs.value = [...arr, ...toAdd]
  }
}

function onWsMessage(data: any) {
  if (data && data.type === 'log' && data.data) {
    const logData = data.data
    const level = ['debug', 'info', 'warn', 'error'].includes(logData.level) ? logData.level : 'info'
    const message = String(logData.message || logData.msg || '')
    if (!message && !logData.message) return
    queueBackendLog({
      id: ++nextId,
      timestamp: logData.timestamp || new Date().toLocaleTimeString('zh-CN', { hour12: false }),
      level,
      message,
    })
    return
  }
  if (data && data.type && data.type !== 'log' && data.type !== 'pong' && data.type !== 'server:status') {
    const msg = typeof data === 'string' ? data : JSON.stringify(data)
    queueBackendLog({ id: ++nextId, timestamp: new Date().toLocaleTimeString('zh-CN', { hour12: false }), level: 'debug', message: msg })
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

  // 写入一条启动日志（INFO/WARN 取决于 server 状态）——首条直接 push 即可
  backendLogs.value = [{
    id: ++nextId,
    timestamp: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
    level: serverOnline.value ? 'info' : 'warn',
    message: `DevLogs ready, server ${serverOnline.value ? 'online' : 'offline'} (transport=${transport.connectionState.value})`,
  }, ...backendLogs.value]
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
  /**
   * 测试专用：替换后端日志数组
   * 走 setBackendLogs 显式赋值（Vue 自动 unwrap ref 导致 vm.backendLogs.value 无法访问）
   */
  setBackendLogs(arr: LogEntry[]) { backendLogs.value = arr },
  getBackendLogs(): LogEntry[] { return backendLogs.value },
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

/* .log-entry 及其 .error/.warn/.info/.debug 变体样式已在 VirtualLogList.vue 中定义
   .log-time / .log-msg / .level-badge 仍属本组件作用域（slot 渲染本组件） */

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
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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

/* 浮动按钮容器：position: fixed 列布局，bottom 偏移让出 status-bar（44px + 20px） */
.scroll-buttons {
  position: fixed;
  right: 16px;
  bottom: 64px;
  z-index: 9999;
  display: flex;
  flex-direction: column;
  gap: 8px;
  pointer-events: none;
}
.scroll-buttons > * { pointer-events: auto; }

/* 浮动「↑」按钮：滚顶，scrollTop > 200 时显示 */
.scrollToTopBtn {
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
.scrollToTopBtn:hover { transform: scale(1.06); }
.scrollToTopBtn:active { transform: scale(0.94); }
.scrollToTopIcon { font-size: 20px; }

/* 浮动「↓」按钮：滚底，position: fixed 视口右下角永远可见 */
.scrollToBottomBtn {
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

/* 搜索关键词高亮（v-html 注入 <mark>，在 log-msg 内部） */
.log-msg :deep(mark) {
  background: rgba(241, 196, 15, 0.35);
  color: inherit;
  border-radius: 2px;
  padding: 0 1px;
}

/* status-bar 自动滚动状态文字：暂停时 warning 色 */
.auto-scroll-status { font-weight: 500; }
.auto-scroll-status.paused { color: var(--ion-color-warning); }
</style>
