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
      🆕 2026-06-14 v5：pinned-to-bottom-on-scroll 最简模型
      核心交互（用户 6-14 反馈简化）：
        - @ionScroll（60Hz scroll 事件）→ 检测到非程序化滚动时立即禁用 autoScrollEnabled
        - 切 tab（onIonViewWillLeave）→ 禁用
        - 切后台（visibilitychange hidden）→ 禁用
        - 切回 tab / 切回前台 → 保持禁用（用户离开过、可能错过日志）
        - 浮动按钮点击 → 重新启用 + 平滑滚到底
      唯一状态：autoScrollEnabled（bool）。无 nearBottom / unreadCount / hardPaused。
      🆕 v5.1 修正（用户实测反馈）：用 @ionScroll 替代 @ionScrollStart——
        @ionScrollStart 在桌面浏览器只响应触摸手势、不响应 wheel 滚轮（@ionScrollStart
        主要给移动端用）。改回 @ionScroll 60Hz 捕获 wheel/touchpad/触摸全场景。
        v2 卡死真因是 console.log 转发 logcat，不是 @ionScroll 本身——v5 已 0 console.log。
    -->
    <ion-content
      ref="contentRef"
      class="log-content"
      :scroll-events="true"
      @ionScroll="onContentScroll"
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

      <!-- 浮动「↓」按钮：autoScrollEnabled=false 时显示，点击恢复跟随 -->
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
    </ion-content>

    <ion-footer class="status-bar">
      <ion-toolbar>
        <div class="status-inner">
          <span class="status-text">{{ t('devlogs.total', { total: String(totalCurrent), filtered: String(filteredCurrent) }) }}</span>
          <!-- v5: 不再有手动 toggle，autoScrollEnabled 由用户手势/生命周期自动管理 -->
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
  onIonViewWillEnter, onIonViewWillLeave,
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
// 🆕 2026-06-14 v5 pinned-to-bottom-on-scroll 最简模型：
//  - autoScrollEnabled = true  → 新日志到达自动滚到底
//  - autoScrollEnabled = false → 禁用跟随（新日志不滚、不累积）
// 触发 disable：用户手势(@ionScroll 60Hz 滚轮/触摸/touchpad) / 切 tab / 切后台
// 触发 enable：浮动按钮点击（同时平滑滚到底）
// 切回 tab / 切回前台：保持当前状态（不重置，让用户主动恢复）
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

// ── Pinned-to-bottom-on-scroll 状态机（2026-06-14 v5 最简版）──────────────
// 核心策略（用户反馈 v4 复杂后再次简化）：
//  1. 单一布尔状态 autoScrollEnabled——v4 的 nearBottom / unreadCount / hardPaused 三元
//     状态、nearBottom 阈值、80px 缓冲全部删除
//  2. 触发 disable：用户手势(@ionScroll 60Hz 滚轮/触摸/touchpad) / 切 tab / 切后台
//  3. 触发 enable：浮动按钮点击（同时平滑滚到底）
//  4. programmaticScrollInProgress 短窗口 flag：程序化 scrollTop 也会触发
//     @ionScroll 60Hz 持续事件，双 rAF 清除避免误判（v2 我栽过这个坑，
//     但 v5/v5.1 已 0 console.log，纯 DOM 滚动 2-rAF 足够消化）
//  5. 滚动元素不缓存 + retry rAF（v4 已修对）：ensureScrollEl 每次重查 shadow root
// ───────────────────────────────────────────────────────────────────────────
let programmaticScrollInProgress = false

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
 * v5 简化：单一守卫 `if (!autoScrollEnabled.value) return`，无 nearBottom 阈值/累积
 * v4 保留：nextTick + rAF + retry rAF（Ionic shadow DOM 异步挂载）
 * v5/v5.1 新增：programmaticScrollInProgress flag 跨过 @ionScroll 60Hz 事件窗口
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
  // 标记程序化滚动（防止 @ionScroll 60Hz 持续事件误判为用户手势）
  programmaticScrollInProgress = true
  try {
    if (smooth) el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
    else el.scrollTop = el.scrollHeight
  } finally {
    // 双 rAF 跨过 @ionScroll 60Hz 事件窗口（v2 在 console.log×60Hz @ionScroll
    // 栽过；v5/v5.1 已 0 console.log，纯 DOM 滚动 2-rAF 足够消化）
    requestAnimationFrame(() => requestAnimationFrame(() => {
      programmaticScrollInProgress = false
    }))
  }
}

/** 浮动按钮点击：恢复 autoScroll + 平滑滚到底（强制，不受当前状态限制） */
async function onJumpToBottom() {
  autoScrollEnabled.value = true
  await scrollToBottom(true)
}

/**
 * 用户滚动（@ionScroll 60Hz 触发，覆盖桌面 wheel/touchpad + 移动端触摸）
 * 唯一目的：立即禁用 autoScrollEnabled——用户希望看的是他滚到的位置，不希望被新日志覆盖
 * 程序化滚动被 programmaticScrollInProgress flag 屏蔽
 * v5.1 关键：从 @ionScrollStart 改回 @ionScroll（前者只响应移动端触摸，桌面 wheel 不触发）
 */
function onContentScroll(_e?: CustomEvent) {
  if (programmaticScrollInProgress) return
  autoScrollEnabled.value = false
}

/**
 * 新日志到达的统一处理（被 frontend/backend 两个 watcher 调用）
 * v5 单一守卫：autoScrollEnabled=false 时直接 return（不滚、不累积——避免 v4 那种
 * unreadCount++ 心智负担）
 */
function handleNewLog() {
  if (!autoScrollEnabled.value) return
  void scrollToBottom(false)
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
 * v5 tab 生命周期：
 *  - onIonViewWillEnter: 切回 tab → 禁用 autoScroll（用户离开过、可能错过日志，
 *    切回不主动覆盖——避免"用户想看老日志结果被新日志滚走"）
 *  - onIonViewWillLeave: 切出 tab → 禁用 autoScroll
 * 两者都是 disable——切回不重置是 v5 与 v4 关键差异（v4 会重算 nearBottom 并自动滚到底）
 */
onIonViewWillEnter(() => {
  autoScrollEnabled.value = false
})

onIonViewWillLeave(() => {
  autoScrollEnabled.value = false
})

/**
 * v5 App 前后台切换：
 *  - hidden → 禁用 autoScroll（用户切后台期间产生的日志不应在切回时覆盖当前视图）
 *  - visible → 保持当前状态（不重置——让用户主动决定）
 */
function onVisibilityChange() {
  if (typeof document === 'undefined') return
  if (document.visibilityState === 'hidden') {
    autoScrollEnabled.value = false
  }
  // visible: 不动 autoScrollEnabled——用户离开时是 false 切回还是 false，离开时是 true 切回还是 true
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
 * v5 简化暴露：只暴露 autoScrollEnabled ref + 5 个核心方法
 */
defineExpose({
  // refs
  autoScrollEnabled,
  activeTab,
  // 方法
  handleNewLog,
  onJumpToBottom,
  scrollToBottom,
  onContentScroll,
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
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
