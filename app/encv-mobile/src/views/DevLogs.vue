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
      🆕 2026-06-14 v3：只用 @ionScrollEnd，不再监听 @ionScroll
      根因：@ionScroll 在 Android WebView 上 60Hz 触发 + console.log 转发 logcat
      导致主线程被同步 logcat 桥锁死，前端卡死。ionScrollEnd 是 Ionic 内置去抖
      事件（~100ms 单次触发），是 Pinned-to-bottom 模式的工业标准输入。
    -->
    <ion-content
      ref="contentRef"
      class="log-content"
      :scroll-events="true"
      @ionScrollEnd="onContentScrollEnd"
    >
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

      <!-- 浮动「↓ N 条新日志」按钮：用户离开底部时显示，点击跳回最新 -->
      <transition name="fade">
        <button
          v-if="!nearBottom && unreadCount > 0"
          type="button"
          class="scrollToBottomBtn"
          :title="t('devlogs.scrollToBottom')"
          :aria-label="t('devlogs.scrollToBottom')"
          @click="onJumpToBottom"
        >
          <ion-icon :icon="arrowDownOutline" class="scrollToBottomIcon" />
          <span class="scrollToBottomBadge">{{ unreadCount > 99 ? '99+' : unreadCount }}</span>
        </button>
      </transition>
    </ion-content>

    <ion-footer class="status-bar">
      <ion-toolbar>
        <div class="status-inner">
          <span class="status-text">{{ t('devlogs.total', { total: String(totalCurrent), filtered: String(filteredCurrent) }) }}</span>
          <div class="status-right">
            <ion-toggle
            v-model="hardPaused"
            :label-placement="'start'"
            :title="t('devlogs.autoScrollHint')"
          >{{ t('devlogs.autoScroll') }}</ion-toggle>
          </div>
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
  IonIcon, IonBadge, IonToggle, IonFooter, alertController,
  onIonViewWillEnter,
} from '@ionic/vue'
import { trashOutline, copyOutline, arrowDownOutline } from 'ionicons/icons'
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
// 🆕 2026-06-14 v3 pinned-to-bottom 模式（v2 卡死后回滚）：
//   - hardPaused = true  → 硬性覆盖（即便在底部也不自动滚；底栏 toggle 控制）
//   - hardPaused = false → 智能 pinned：nearBottom=true 时自动滚，false 时累积 unread
const hardPaused = ref(false)
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

// ── Pinned-to-bottom 状态机（2026-06-14 v4 极简版 + 重试）─────────────────
// 核心策略（v3 卡死/不滚后的回滚 + 修对）：
//  1. 只在 @ionScrollEnd 触发时重算 nearBottom（不监听 60Hz @ionScroll）
//  2. 滚动元素不缓存（每次重查 shadow root .inner-scroll）—— v3 缓存 hostEl 是
//     在浏览器/真机都不滚的真因（el.scrollTop 写到 ion-content 元素上而非真滚动元素）
//  3. scrollToBottom 内 nextTick + rAF + rAF retry（Ionic shadow DOM 异步挂载）
//  4. 全部 0 个 console.log（v2 的卡死根因：logcat 桥同步转发阻塞主线程）
//  5. 用 el.scrollTop = el.scrollHeight 直赋值（不依赖 scrollTo 行为）
//  6. watcher flush:'post'（v3 注释"默认 pre 等价 nextTick"是错的；pre 是 DOM 未 patch 就触发）
// ───────────────────────────────────────────────────────────────────────────
const NEAR_BOTTOM_THRESHOLD_PX = 80
const nearBottom = ref(true)
const unreadCount = ref(0)

function onTabClick(tab: 'frontend' | 'backend') {
  if (activeTab.value === tab) {
    scrollToTop()
  }
}

function onTabChange(event: CustomEvent) {
  activeTab.value = (event.detail.value || 'frontend') as 'frontend' | 'backend'
}

/**
 * 找 ion-content 实际滚动的元素（同步、无缓存、零强制 layout）
 * 关键变更：v4 删 v3 第 266 行的 `cachedScrollEl = hostEl` 兜底——这是 v3 在浏览器/真机都不滚的真因
 * （Ionic shadow DOM 异步挂载，v3 把 hostEl 缓存住，el.scrollTop 写到 ion-content 元素上
 *  而非真正的 .inner-scroll，永久不滚）。
 *
 * 优先级（每次重查，不缓存）：
 *   1. hostEl.shadowRoot 存在 → 查 .inner-scroll
 *   2. 任何失败直接返回 null（不兜底！）
 *
 * 失败由 scrollToBottom / onJumpToBottom 的 rAF retry 处理。
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
 * v4 修复：
 *  - v3 注释错误：v3 用 nextTick 一次就跳过了 layout pass——Ionic 内部 shadow DOM 更新
 *    要到下一个 rAF 才完成（connectedCallback → .inner-scroll 渲染）。
 *  - v4 重试机制：nextTick → rAF → ensureScrollEl → 失败再 rAF → 再 ensureScrollEl
 *  - 不缓存 el（每次重查，O(1) selector 查询，60Hz 1000 条/秒无压力）
 */
async function scrollToBottom(smooth = false) {
  if (hardPaused.value) return
  if (!nearBottom.value) { unreadCount.value++; return }
  await nextTick()
  await new Promise<void>((r) => requestAnimationFrame(() => r()))
  let el = ensureScrollEl()
  if (!el) {
    // 第一次 shadowRoot 还没挂上（Ionic 异步）——等下一个 rAF 再试
    await new Promise<void>((r) => requestAnimationFrame(() => r()))
    el = ensureScrollEl()
  }
  if (!el) return
  if (smooth) el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
  else el.scrollTop = el.scrollHeight
  nearBottom.value = true
  unreadCount.value = 0
}

/** 浮动按钮点击：平滑滚到底部（强制滚动，不受 nearBottom 状态限制） */
async function onJumpToBottom() {
  await nextTick()
  await new Promise<void>((r) => requestAnimationFrame(() => r()))
  let el = ensureScrollEl()
  if (!el) {
    await new Promise<void>((r) => requestAnimationFrame(() => r()))
    el = ensureScrollEl()
  }
  if (!el) return
  el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
  nearBottom.value = true
  unreadCount.value = 0
}

/** 重算 nearBottom（仅在 ionScrollEnd 触发时调用，已去抖） */
function updateNearBottom() {
  const el = ensureScrollEl()
  if (!el) return
  const distance = el.scrollHeight - el.scrollTop - el.clientHeight
  const wasNear = nearBottom.value
  nearBottom.value = distance < NEAR_BOTTOM_THRESHOLD_PX
  if (!wasNear && nearBottom.value) unreadCount.value = 0
}

/**
 * 新日志到达的统一处理（被 frontend/backend 两个 watcher 调用）
 * - 硬性覆盖：不滚、不累积（toggle 关闭时的硬性暂停）
 * - 在底部：自动滚到底
 * - 不在底部：累积 unread，浮动按钮显示
 */
function handleNewLog() {
  if (hardPaused.value) return
  if (nearBottom.value) {
    void scrollToBottom(false)
  } else {
    unreadCount.value++
  }
}

/**
 * v3 关键变更：只用 ionScrollEnd（Ionic 内置去抖 ~100ms），不再监听 60Hz 的 ionScroll
 * 工业标准：VS Code console / Chrome DevTools / Postman 全部用 scrollend 决定
 * 是否"用户已停止滚动"，避免每帧重算状态。
 */
function onContentScrollEnd() {
  void updateNearBottom()
}

/**
 * 新日志监听
 * v4 关键修复：flush:'post'（v3 注释说"默认 pre 等价 nextTick"是错的——pre 是 DOM 未 patch
 * 就触发，scrollHeight 还没增大；post 等 DOM patch + ion-content 内部 shadow DOM 更新）
 * 看的是数组 length（不是深监听），性能好且不会因搜索/筛选误触发。
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

  // 🆕 2026-06-10：transport 已在 App.vue 启动，无需重复 connect()
  // 之前重复 connect 会导致：
  //   - App.vue connect → useWebSocket 单例
  //   - DevLogs onMounted 又调一次 ws.connect()（idempotent 但冗余）
  // 现在 transport 单例管理生命周期，DevLogs 只读 connectionState

  eventBus.on('ws:message', onWsMessage)
  eventBus.on('server:status', onServerStatus)

  serverOnline.value = transport.connectionState.value === 'connected'
  if (!serverOnline.value) {
    const result = await checkServerStatus()
    serverOnline.value = result.online
  }

  backendLogs.value.push({
    id: ++nextId,
    timestamp: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
    level: serverOnline.value ? 'info' : 'warn',
    message: `DevLogs ready, server ${serverOnline.value ? 'online' : 'offline'} (transport=${transport.connectionState.value})`,
  })

  // App 前后台切换：DOM 滚动位置可能已失效（iOS Safari background tab 清空 layout）
  if (typeof document !== 'undefined') {
    document.addEventListener('visibilitychange', onVisibilityChange)
  }
})

/**
 * tab 切回时（Ionic keep-alive 不会重跑 onMounted）：
 * 重新计算 nearBottom（DOM 滚动位置仍是用户离开时的值）
 */
onIonViewWillEnter(async () => {
  await nextTick()
  updateNearBottom()
  // 切回时如果原本在底部 + autoScroll 开（hardPaused=false）+ 有未读 → 滚到底
  // 否则尊重用户离开时的滚动位置（工业标准：VS Code console、Postman 行为）
  if (nearBottom.value && !hardPaused.value && unreadCount.value > 0) {
    scrollToBottom(false)
  }
})

/** App 从后台恢复时：重算 nearBottom */
function onVisibilityChange() {
  if (typeof document === 'undefined') return
  if (document.visibilityState === 'visible') {
    void nextTick(() => updateNearBottom())
  }
}

onUnmounted(() => {
  eventBus.off('ws:message', onWsMessage)
  eventBus.off('server:status', onServerStatus)
  if (typeof document !== 'undefined') {
    document.removeEventListener('visibilitychange', onVisibilityChange)
  }
})

/**
 * 暴露给单元测试的滚动状态机（生产环境无用，仅用于单测验证 pinned 模式）
 * 覆盖：
 *   - nearBottom / unreadCount / hardPaused refs
 *   - handleNewLog / onJumpToBottom / updateNearBottom / scrollToBottom / onContentScrollEnd
 */
defineExpose({
  // refs
  nearBottom,
  unreadCount,
  hardPaused,
  activeTab,
  // 方法
  handleNewLog,
  onJumpToBottom,
  updateNearBottom,
  scrollToBottom,
  onContentScrollEnd,
  // 测试工具
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
.status-right { display: flex; align-items: center; gap: 6px; }
.status-right ion-toggle { --height: 18px; }

/* ── 浮动「↓ N 条新日志」按钮（2026-06-14 新增） ──
   位置：ion-content 内部右下角（ion-content 默认 position: relative）
   z-index 50：低于浮动 AI 入口（999），不挡关键 UI */
.scrollToBottomBtn {
  position: absolute;
  right: 16px;
  bottom: 16px;
  z-index: 50;
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
.scrollToBottomBadge {
  position: absolute;
  top: -2px;
  right: -2px;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 9px;
  background: var(--ion-color-danger, #eb445a);
  color: #fff;
  font-size: 11px;
  font-weight: 600;
  line-height: 18px;
  text-align: center;
  box-shadow: 0 0 0 2px var(--ion-toolbar-background, var(--ion-background-color));
}
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
