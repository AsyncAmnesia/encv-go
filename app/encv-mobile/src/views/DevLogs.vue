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

    <ion-content
      ref="contentRef"
      class="log-content"
      :class="{ 'scrollbar-visible': scrollbarVisible }"
      :scroll-events="true"
      @ionScroll="onContentScroll"
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
// 🆕 2026-06-14 pinned-to-bottom 模式：
//   - hardPaused = true  → 硬性覆盖（即便在底部也不自动滚；底栏 toggle 控制）
//   - hardPaused = false → 智能 pinned：nearBottom=true 时自动滚，false 时累积 unread
const hardPaused = ref(false)
const contentRef = ref<InstanceType<typeof IonContent> | null>(null)
const scrollbarVisible = ref(false)

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

// ── Pinned-to-bottom 状态机（2026-06-14 v2 重构）────────────────────────────
// 核心问题：Android WebView 上
//  1. <ion-content> 的 @wheel/@touchstart 监听被 Ionic gesture 拦截，不冒泡
//     → 不能用 userGestureActive 区分程序化 vs 用户滚动
//  2. ion-content 的 getScrollElement() 在 Capacitor Android 上可能返回外层元素
//     → 必须 DOM walk fallback 找 shadow DOM .inner-scroll
//  3. el.scrollTo({ behavior: 'auto' }) 在 Android WebView 不等价于 scrollTop=scrollHeight
//     → 必须直接赋值 scrollTop
//  4. Vue 3 watch 默认 flush:'pre'（DOM 未 patch）→ 必须 flush:'post'
//
// 解决方案：
//  - programmaticScrollInProgress 时间窗 flag（程序化滚动期间忽略 ionScroll）
//  - 优先 contentRef.getScrollElement()，失败则 DOM walk
//  - 直接 scrollTop = scrollHeight（instant）
//  - flush:'post' watcher（DOM 已 patch）
//  - await nextTick + requestAnimationFrame（等 layout 完成）
// ───────────────────────────────────────────────────────────────────────────
const NEAR_BOTTOM_THRESHOLD_PX = 80
const nearBottom = ref(true)
const unreadCount = ref(0)
let programmaticScrollInProgress = false
let programmaticScrollTimer: ReturnType<typeof setTimeout> | null = null
let scrollbarTimer: ReturnType<typeof setTimeout> | null = null

function onTabClick(tab: 'frontend' | 'backend') {
  if (activeTab.value === tab) {
    scrollToTop()
  }
}

function onTabChange(event: CustomEvent) {
  activeTab.value = (event.detail.value || 'frontend') as 'frontend' | 'backend'
}

/**
 * 找 ion-content 实际滚动的元素
 * 优先级：
 *   1. contentRef.getScrollElement()（@ionic/vue 8 官方 API）
 *   2. shadow DOM .inner-scroll（ion-content 内部真实滚动元素）
 *   3. DOM walk 找最近的 overflow:auto/scroll 祖先
 *  返回 null 表示完全找不到（不应发生，但兜底）
 */
async function getScrollEl(): Promise<HTMLElement | null> {
  if (!contentRef.value) return null
  // 优先级 1：官方 API
  try {
    const el = contentRef.value as any
    if (typeof el.getScrollElement === 'function') {
      const r = await el.getScrollElement()
      if (r) return r as HTMLElement
    }
  } catch { /* fall through */ }
  // 优先级 2：shadow DOM .inner-scroll
  const ionContentEl = (contentRef.value as any).$el as HTMLElement | undefined
  if (ionContentEl?.shadowRoot) {
    const inner = ionContentEl.shadowRoot.querySelector('.inner-scroll') as HTMLElement | null
    if (inner) return inner
  }
  // 优先级 3：DOM walk
  const logList = document.querySelector('.log-list') as HTMLElement | null
  if (logList) {
    let current: HTMLElement | null = logList.parentElement
    while (current && current !== document.body) {
      const style = window.getComputedStyle(current)
      if (style.overflowY === 'auto' || style.overflowY === 'scroll') {
        return current
      }
      current = current.parentElement
    }
  }
  return null
}

async function scrollToTop() {
  const el = await getScrollEl()
  if (el) el.scrollTop = 0
}

/**
 * 滚动到底部（程序化）
 * 关键修复：
 *  - await nextTick → 等 Vue 把新行 patch 到 DOM
 *  - await requestAnimationFrame → 等浏览器完成 layout（scrollHeight 更新）
 *  - 直接 scrollTop = scrollHeight（避开 Android WebView scrollTo 行为歧义）
 *  - 设 programmaticScrollInProgress 屏蔽 ionScroll 反馈
 */
async function scrollToBottom(smooth = false) {
  // eslint-disable-next-line no-console
  console.log('[DevLogs] scrollToBottom called, smooth=', smooth, 'nearBottom=', nearBottom.value, 'unreadCount=', unreadCount.value)
  programmaticScrollInProgress = true
  if (programmaticScrollTimer) clearTimeout(programmaticScrollTimer)
  try {
    await nextTick()
    await new Promise<void>((r) => requestAnimationFrame(() => r()))
    const el = await getScrollEl()
    // eslint-disable-next-line no-console
    console.log('[DevLogs] scrollEl=', el ? `${el.tagName}.${el.className || ''} scrollH=${el.scrollHeight} scrollT=${el.scrollTop} clientH=${el.clientHeight}` : 'NULL')
    if (!el) {
      console.warn('[DevLogs] scrollToBottom aborted: scroll element not found')
      return
    }
    if (smooth) {
      el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
    } else {
      // 直赋值（instant，不依赖 UA scrollTo 行为）
      el.scrollTop = el.scrollHeight
      // 二次保险：再 read+set（部分 WebView 第一次赋值时被 layout 限制）
      requestAnimationFrame(() => { el.scrollTop = el.scrollHeight })
    }
    nearBottom.value = true
    unreadCount.value = 0
  } finally {
    // 100ms 后解除屏蔽（覆盖 ionScroll 事件从程序化滚动传播过来的时间）
    programmaticScrollTimer = setTimeout(() => {
      programmaticScrollInProgress = false
    }, 100)
  }
}

/** 浮动按钮点击：平滑滚到底部 */
async function onJumpToBottom() {
  await scrollToBottom(true)
}

/** 重算 nearBottom（仅在 ionScroll 来自用户滚动时调用） */
async function updateNearBottom() {
  const el = await getScrollEl()
  if (!el) return
  const distance = el.scrollHeight - el.scrollTop - el.clientHeight
  const wasNear = nearBottom.value
  nearBottom.value = distance < NEAR_BOTTOM_THRESHOLD_PX
  // eslint-disable-next-line no-console
  console.log('[DevLogs] updateNearBottom: distance=', distance, 'nearBottom=', nearBottom.value)
  if (!wasNear && nearBottom.value) {
    unreadCount.value = 0
  }
}

/**
 * 新日志到达的统一处理（被 frontend/backend 两个 watcher 调用）
 * - 硬性覆盖：不滚、不累积（toggle 关闭时的硬性暂停）
 * - 在底部：自动滚到底
 * - 不在底部：累积 unread，浮动按钮显示
 */
function handleNewLog() {
  // eslint-disable-next-line no-console
  console.log('[DevLogs] handleNewLog: activeTab=', activeTab.value, 'hardPaused=', hardPaused.value, 'nearBottom=', nearBottom.value)
  if (hardPaused.value) return
  if (nearBottom.value) {
    void scrollToBottom(false)
  } else {
    unreadCount.value++
    // eslint-disable-next-line no-console
    console.log('[DevLogs] accumulated unreadCount=', unreadCount.value)
  }
}

function onContentScroll() {
  // 视觉反馈：滚动条可见 2s
  scrollbarVisible.value = true
  if (scrollbarTimer) clearTimeout(scrollbarTimer)
  scrollbarTimer = setTimeout(() => { scrollbarVisible.value = false }, 2000)
  // 屏蔽程序化滚动触发的 ionScroll（避免反馈循环）
  if (programmaticScrollInProgress) {
    // eslint-disable-next-line no-console
    console.log('[DevLogs] onContentScroll: ignored (programmatic scroll in progress)')
    return
  }
  void updateNearBottom()
}

function onContentScrollEnd() {
  if (scrollbarTimer) clearTimeout(scrollbarTimer)
  scrollbarTimer = setTimeout(() => { scrollbarVisible.value = false }, 2000)
}

/**
 * 新日志监听（flush:'post' 确保 DOM 已 patch）
 * - flush:'post' 关键：默认 flush:'pre' 时 watcher 触发时 DOM 未更新，scrollHeight 错误
 * - 看的是数组 length（不是深监听），性能好且不会因搜索/筛选误触发
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
  await updateNearBottom()
  // 切回时如果原本在底部 + autoScroll 开（hardPaused=false）+ 有未读 → 滚到底
  // 否则尊重用户离开时的滚动位置（工业标准：VS Code console、Postman 行为）
  if (nearBottom.value && !hardPaused.value && unreadCount.value > 0) {
    await scrollToBottom(false)
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
  if (programmaticScrollTimer) clearTimeout(programmaticScrollTimer)
  if (scrollbarTimer) clearTimeout(scrollbarTimer)
  if (typeof document !== 'undefined') {
    document.removeEventListener('visibilitychange', onVisibilityChange)
  }
})

/**
 * 暴露给单元测试的滚动状态机（生产环境无用，仅用于单测验证 pinned 模式）
 * 覆盖：
 *   - nearBottom / unreadCount / hardPaused refs
 *   - programmaticScrollInProgress flag（测试可重置）
 *   - handleNewLog / onJumpToBottom / updateNearBottom / scrollToBottom / onContentScroll
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
  onContentScroll,
  // 测试工具
  setActiveTab(tab: 'frontend' | 'backend') { activeTab.value = tab },
  setProgrammaticScrollInProgress(v: boolean) { programmaticScrollInProgress = v },
  isProgrammaticScrollInProgress() { return programmaticScrollInProgress },
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

.log-content::part(scroll) {
  scrollbar-width: none;
  -ms-overflow-style: none;
}
.log-content::part(scroll)::-webkit-scrollbar {
  width: 6px;
  display: none;
}
.log-content.scrollbar-visible::part(scroll) {
  scrollbar-width: thin;
  scrollbar-color: rgba(255, 255, 255, 0.25) transparent;
}
.log-content.scrollbar-visible::part(scroll)::-webkit-scrollbar {
  display: block;
}
.log-content.scrollbar-visible::part(scroll)::-webkit-scrollbar-track {
  background: transparent;
}
.log-content.scrollbar-visible::part(scroll)::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.25);
  border-radius: 3px;
}

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
