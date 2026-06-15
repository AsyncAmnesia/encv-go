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
      🆕 2026-06-14 v6：纯手动挡（彻底删除所有自动检测）
      核心策略（用户实测反馈）：
        - 浏览器预览 = 手机浏览器（无 mouse wheel，只有 touch）
        - 项目启用 Capacitor 高刷 WebView 90/120Hz
        - @ionScroll/@ionScrollStart 在移动端 + 高刷下完全不可靠
        - 程序化 flag 屏蔽 + 切 tab 禁用 + 切后台禁用等自动检测全部作废
      交互（v6）：
        - toolbar ▶ 跟随 / ⏸ 暂停 开关：用户主动切换 autoScrollEnabled
        - 浮动 ↓ 按钮：autoScroll=false 时显示，点击 = 重新开启 + 平滑滚到底
        - 唯一守卫：handleNewLog 内部 if (!autoScrollEnabled.value) return
      唯一状态：autoScrollEnabled（bool）。0 自动检测 / 0 programmatic flag / 0 生命周期禁用。
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

    <!--
      浮动「↓」按钮：v6 移到 ion-content 外部（ion-page 直接子节点），
      用 position: fixed 定位视口右下角——彻底规避 ion-content shadow DOM
      内部的 position: absolute 被遮挡 / 不可见问题（v5 按钮死活不显示的真因）
    -->
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
// 🆕 2026-06-14 v6 纯手动挡模型：
//  - autoScrollEnabled = true  → 新日志到达自动滚到底
//  - autoScrollEnabled = false → 不滚（用户已手动暂停跟随）
// 唯一交互入口：toolbar 开关按钮（点击 toggle）或浮动 ↓ 按钮（开启+滚到底）
// 0 自动检测：浏览器预览 = 手机浏览器（无 wheel）+ 90/120Hz 高刷
// @ionScroll/@ionScrollStart 完全不可靠 → 不用；programmatic flag / visibilitychange /
// onIonViewWillEnter 等禁用逻辑也全部作废（用户实测反馈）
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

// ── v6 纯手动挡：唯一交互入口 ─────────────────────────────────────────────
// 核心策略：删除 v5 的所有自动检测（@ionScroll / programmatic flag / 生命周期禁用），
// 改为 toolbar 显式开关 + 浮动 ↓ 按钮。
//
// v5→v6 移除清单：
//   - @ionScroll/@ionScrollStart 事件绑定
//   - programmaticScrollInProgress 标志 + 双 rAF 清理
//   - onContentScroll() 处理器
//   - onIonViewWillEnter/onIonViewWillLeave 强制 disable
//   - visibilitychange hidden 强制 disable
//   - document.addEventListener('visibilitychange', ...)
// ───────────────────────────────────────────────────────────────────────────

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
 * v6 简化：单一守卫 `if (!autoScrollEnabled.value) return`，无 programmatic flag
 * v4 保留：nextTick + rAF + retry rAF（Ionic shadow DOM 异步挂载）
 *
 * v6 关键：不再需要 programmatic flag 屏蔽 @ionScroll——v6 完全不监听 scroll 事件，
 * 程序化 scrollTop 不会触发任何回调，纯 DOM 操作 0 干扰。
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

/**
 * v6 toolbar 开关按钮：切换 autoScrollEnabled
 * 用户主动控制：完全手动挡，无任何自动检测
 */
function toggleAutoScroll() {
  autoScrollEnabled.value = !autoScrollEnabled.value
}

/** 浮动「↓」按钮点击：开启跟随 + 平滑滚到底（强制，不受当前状态限制） */
async function onJumpToBottom() {
  autoScrollEnabled.value = true
  await scrollToBottom(true)
}

/**
 * 新日志到达的统一处理（被 frontend/backend 两个 watcher 调用）
 * v6 唯一守卫：autoScrollEnabled=false 时直接 return（不滚、不累积——v5/v4 同样）
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

  // v6 纯手动挡：tab 切换、App 前后台切换都不再 auto-disable
  // 用户的 autoScrollEnabled 状态由 toolbar 开关按钮显式控制
})

onUnmounted(() => {
  eventBus.off('ws:message', onWsMessage)
  eventBus.off('server:status', onServerStatus)
})

/**
 * 暴露给单元测试的滚动状态机（生产环境无用，仅用于单测验证 pinned 模式）
 * v6 简化暴露：autoScrollEnabled ref + 4 个核心方法
 */
defineExpose({
  // refs
  autoScrollEnabled,
  activeTab,
  // 方法
  handleNewLog,
  toggleAutoScroll,
  onJumpToBottom,
  scrollToBottom,
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

/* ── 浮动「↓」按钮（2026-06-14 v6 重做） ──
   v5 改动史：position: absolute + 在 ion-content 内部 → 被 shadow DOM 遮挡不显示
   v6 修复：position: fixed + 移到 ion-content 外部（ion-page 直接子节点）+
   z-index 9999 绝对安全——视口右下角永远可见。
   位置避开 status-bar：bottom: 64px（status-bar 高度约 44px + 20px 间距） */
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

/* v6: status-bar 显示自动滚动状态 */
.auto-scroll-status { font-weight: 500; }
.auto-scroll-status.paused { color: var(--ion-color-warning); }
</style>
