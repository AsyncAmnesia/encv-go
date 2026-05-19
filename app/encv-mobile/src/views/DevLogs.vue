<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>{{ t('devlogs.title') }}</ion-title>
      </ion-toolbar>
      <ion-toolbar class="tab-toolbar">
        <ion-segment :value="activeTab" @ionChange="activeTab = $event.detail.value">
          <ion-segment-button value="frontend">
            {{ t('devlogs.frontend') }}
          </ion-segment-button>
          <ion-segment-button value="backend">
            {{ t('devlogs.backend') }}
          </ion-segment-button>
        </ion-segment>
      </ion-toolbar>
    </ion-header>

    <ion-content ref="contentRef" :scroll-events="true" @ionScroll="handleScroll" class="log-content">
      <div class="toolbar-row">
        <div class="level-filters">
          <ion-chip
            v-for="lvl in levels"
            :key="lvl.value"
            :class="{ active: selectedLevel === lvl.value }"
            @click="selectedLevel = lvl.value"
          >
            <ion-label>{{ lvl.label }}</ion-label>
          </ion-chip>
        </div>
        <ion-searchbar
          v-model="searchText"
          :placeholder="t('devlogs.searchPlaceholder')"
          class="log-searchbar"
          mode="ios"
          :debounce="150"
        ></ion-searchbar>
        <ion-button fill="clear" size="small" color="danger" @click="handleClear">
          <ion-icon :icon="trashOutline" slot="icon-only"></ion-icon>
        </ion-button>
      </div>

      <div
        v-if="filteredLogs.length === 0"
        class="empty-state"
      >
        <ion-icon :icon="documentTextOutline" class="empty-icon"></ion-icon>
        <h3>{{ t('devlogs.noLogs') }}</h3>
        <p>{{ t('devlogs.noLogsDesc') }}</p>
      </div>

      <div v-else class="log-list" ref="logListRef">
        <div
          v-for="log in filteredLogs"
          :key="log.id"
          class="log-entry"
        >
          <span class="log-time">[{{ log.timestamp }}]</span>
          <ion-badge :style="{ '--badge-color': levelColor(log.level) }" class="log-level-badge">
            {{ log.level.toUpperCase() }}
          </ion-badge>
          <span class="log-message" v-html="highlightSearch(log.message)"></span>
        </div>
      </div>

      <div v-if="activeTab === 'backend'" class="connection-status">
        <ion-badge :color="serverOnline ? 'success' : 'danger'">
          {{ serverOnline ? t('devlogs.connected') : t('devlogs.disconnected') }}
        </ion-badge>
      </div>
    </ion-content>

    <ion-footer class="status-bar">
      <ion-toolbar>
        <div class="status-inner">
          <span class="status-text">
            {{ t('devlogs.total', { total: currentLogs.length, filtered: filteredLogs.length }) }}
          </span>
          <div class="status-right">
            <ion-toggle
              v-model="autoScroll"
              :label-placement="'start'"
            >
              {{ t('devlogs.autoScroll') }}
            </ion-toggle>
          </div>
        </div>
      </ion-toolbar>
    </ion-footer>

    <ion-alert-controller></ion-alert-controller>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import {
  IonPage,
  IonHeader,
  IonToolbar,
  IonTitle,
  IonContent,
  IonSegment,
  IonSegmentButton,
  IonChip,
  IonLabel as IonLabelComponent,
  IonSearchbar,
  IonButton,
  IonIcon,
  IonBadge,
  IonToggle,
  IonFooter,
  alertController,
} from '@ionic/vue'
import { trashOutline, documentTextOutline } from 'ionicons/icons'
import { eventBus } from '@/composables/useEventBus'
import { useI18n } from '@/composables/useI18n'

const { t } = useI18n()

type LogLevel = 'debug' | 'info' | 'warn' | 'error'

interface LogEntry {
  id: number
  timestamp: string
  level: LogLevel
  message: string
}

interface BackendLogEntry extends LogEntry {
  type?: string
}

function useLogCapture() {
  const logs = ref<LogEntry[]>([])
  let idCounter = 0

  const originalConsole = {
    log: console.log,
    debug: console.debug,
    info: console.info,
    warn: console.warn,
    error: console.error,
  }

  function formatTimestamp(): string {
    const now = new Date()
    const h = String(now.getHours()).padStart(2, '0')
    const m = String(now.getMinutes()).padStart(2, '0')
    const s = String(now.getSeconds()).padStart(2, '0')
    const ms = String(now.getMilliseconds()).padStart(3, '0')
    return `${h}:${m}:${s}.${ms}`
  }

  function addLog(level: LogLevel, args: any[]) {
    const message = args.map(arg => {
      if (typeof arg === 'object') {
        try {
          return JSON.stringify(arg)
        } catch {
          return String(arg)
        }
      }
      return String(arg)
    }).join(' ')
    logs.value.push({
      id: ++idCounter,
      timestamp: formatTimestamp(),
      level,
      message,
    })
  }

  console.log = (...args: any[]) => {
    originalConsole.log(...args)
    addLog('info', args)
  }
  console.debug = (...args: any[]) => {
    originalConsole.debug(...args)
    addLog('debug', args)
  }
  console.info = (...args: any[]) => {
    originalConsole.info(...args)
    addLog('info', args)
  }
  console.warn = (...args: any[]) => {
    originalConsole.warn(...args)
    addLog('warn', args)
  }
  console.error = (...args: any[]) => {
    originalConsole.error(...args)
    addLog('error', args)
  }

  function restore() {
    console.log = originalConsole.log
    console.debug = originalConsole.debug
    console.info = originalConsole.info
    console.warn = originalConsole.warn
    console.error = originalConsole.error
  }

  function clear() {
    logs.value = []
  }

  return { logs, clear, restore }
}

const activeTab = ref<'frontend' | 'backend'>('frontend')
const selectedLevel = ref<LogLevel | 'all'>('all')
const searchText = ref('')
const autoScroll = ref(true)
const serverOnline = ref(false)
const contentRef = ref<any>(null)
const logListRef = ref<HTMLElement | null>(null)

const { logs: frontendLogs, clear: clearFrontend, restore: restoreFrontend } = useLogCapture()
const backendLogs = ref<BackendLogEntry[]>([])

const levels = [
  { value: 'all' as const, label: t('devlogs.all') },
  { value: 'debug' as const, label: t('devlogs.debug') },
  { value: 'info' as const, label: t('devlogs.info') },
  { value: 'warn' as const, label: t('devlogs.warn') },
  { value: 'error' as const, label: t('devlogs.error') },
]

const currentLogs = computed(() => activeTab.value === 'frontend' ? frontendLogs.value : backendLogs.value)

const filteredLogs = computed(() => {
  let result = currentLogs.value
  if (selectedLevel.value !== 'all') {
    result = result.filter(log => log.level === selectedLevel.value)
  }
  if (searchText.value.trim()) {
    const query = searchText.value.toLowerCase()
    result = result.filter(log =>
      log.message.toLowerCase().includes(query) ||
      log.timestamp.includes(query) ||
      log.level.toLowerCase().includes(query)
    )
  }
  return result
})

function levelColor(level: LogLevel): string {
  switch (level) {
    case 'debug': return '#888'
    case 'info': return '#2ecc71'
    case 'warn': return '#f39c12'
    case 'error': return '#e74c3c'
  }
}

function highlightSearch(message: string): string {
  if (!searchText.value.trim()) return escapeHtml(message)
  const escaped = escapeHtml(searchText.value)
  const regex = new RegExp(`(${escaped.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi')
  return escapeHtml(message).replace(regex, '<mark>$1</mark>')
}

function escapeHtml(text: string): string {
  const div = document.createElement('div')
  div.textContent = text
  return div.innerHTML
}

let isUserScrolling = false

function handleScroll(event: CustomEvent) {
  const el = event.target as any
  if (el && el.scrollHeight - el.scrollTop - el.clientHeight > 30) {
    isUserScrolling = true
  } else {
    isUserScrolling = false
  }
}

async function scrollToBottom() {
  if (!autoScroll.value || isUserScrolling) return
  await nextTick()
  try {
    const contentEl = contentRef.value?.$el
    if (contentEl) {
      contentEl.scrollToBottom(200)
    }
  } catch {}
}

watch([frontendLogs, backendLogs, () => autoScroll.value], () => {
  scrollToBottom()
}, { deep: true })

async function handleClear() {
  const alert = await alertController.create({
    header: t('devlogs.clear'),
    message: t('devlogs.clearConfirm'),
    buttons: [
      { text: t('devlogs.cancel'), role: 'cancel' },
      {
        text: t('devlogs.clear'),
        role: 'destructive',
        handler: () => {
          if (activeTab.value === 'frontend') {
            clearFrontend()
          } else {
            backendLogs.value = []
          }
        },
      },
    ],
  })
  await alert.present()
}

function onWsMessage(data: any) {
  let level: LogLevel = 'info'
  let msg = ''
  if (typeof data === 'string') {
    msg = data
  } else if (data && typeof data === 'object') {
    msg = JSON.stringify(data)
    if (data.type) {
      msg = `[${data.type}] ${JSON.stringify(data.data ?? data)}`
    }
    if (data.level && ['debug', 'info', 'warn', 'error'].includes(data.level)) {
      level = data.level as LogLevel
    }
  } else {
    msg = String(data)
  }
  const now = new Date()
  const ts = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}.${String(now.getMilliseconds()).padStart(3, '0')}`
  backendLogs.value.push({
    id: Date.now() + Math.random(),
    timestamp: ts,
    level,
    message: msg,
  })
}

function onServerStatus(data: { online: boolean }) {
  serverOnline.value = data.online
  const now = new Date()
  const ts = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}.${String(now.getMilliseconds()).padStart(3, '0')}`
  backendLogs.value.push({
    id: Date.now() + Math.random(),
    timestamp: ts,
    level: data.online ? 'info' : 'warn',
    message: `Server ${data.online ? 'online' : 'offline'}`,
  })
}

onMounted(() => {
  eventBus.on('ws:message', onWsMessage)
  eventBus.on('server:status', onServerStatus)
})

onUnmounted(() => {
  eventBus.off('ws:message', onWsMessage)
  eventBus.off('server:status', onServerStatus)
  restoreFrontend()
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
  gap: 6px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--ion-border-color, rgba(255, 255, 255, 0.08));
  flex-wrap: wrap;
  background: var(--ion-background-color);
}

.level-filters {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  flex-shrink: 0;
}

.level-filters ion-chip {
  --background: transparent;
  --color: var(--ion-text-color-step-400, #999);
  font-size: 11px;
  height: 26px;
  margin: 0;
  cursor: pointer;
  transition: all 0.15s ease;
  border: 1px solid transparent;
}

.level-filters ion-chip.active {
  --background: rgba(var(--ion-color-primary-rgb, 38, 132, 255), 0.15);
  --color: var(--ion-color-primary, #3880ff);
  border-color: var(--ion-color-primary, #3880ff);
}

.log-searchbar {
  flex: 1;
  min-width: 120px;
  --border-radius: 12px;
  --background: rgba(255, 255, 255, 0.06);
  --placeholder-color: var(--ion-text-color-step-350, #aaa);
  --color: var(--ion-text-color);
  --icon-color: var(--ion-text-color-step-400, #999);
  padding-top: 0;
  padding-bottom: 0;
}

.log-searchbar .searchbar-search-icon {
  display: none !important;
}

.log-list {
  font-family: 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.5;
  padding: 4px 8px;
}

.log-entry {
  display: flex;
  align-items: baseline;
  gap: 6px;
  padding: 2px 4px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
  word-break: break-all;
}

.log-time {
  color: var(--ion-text-color-step-300, #777);
  white-space: nowrap;
  flex-shrink: 0;
  user-select: none;
}

.log-level-badge {
  --padding-start: 4px;
  --padding-end: 4px;
  --padding-top: 1px;
  --padding-bottom: 1px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.5px;
  font-family: 'Courier New', monospace;
  background: var(--badge-color);
  color: #fff;
  flex-shrink: 0;
  opacity: 0.9;
}

.log-message {
  color: var(--ion-text-color, #ddd);
  flex: 1;
  min-width: 0;
}

.log-message :deep(mark) {
  background-color: rgba(241, 196, 15, 0.35);
  color: inherit;
  border-radius: 2px;
  padding: 0 1px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 50%;
  padding: 24px;
  text-align: center;
  color: var(--ion-text-color-step-350, #999);
}

.empty-icon {
  font-size: 48px;
  margin-bottom: 12px;
  opacity: 0.35;
}

.connection-status {
  position: sticky;
  bottom: 0;
  left: 0;
  right: 0;
  padding: 4px 10px;
  text-align: center;
  background: var(--ion-background-color);
  border-top: 1px solid var(--ion-border-color, rgba(255, 255, 255, 0.08));
  z-index: 1;
}

.status-bar {
  --background: var(--ion-toolbar-background, rgba(var(--ion-background-color-rgb), 0.92));
  --border-width: 1px 0 0 0;
  backdrop-filter: blur(8px);
}

.status-bar ion-toolbar {
  --padding-start: 12px;
  --padding-end: 12px;
  --min-height: 40px;
}

.status-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 2px 0;
}

.status-text {
  font-size: 12px;
  color: var(--ion-text-color-step-350, #999);
  white-space: nowrap;
}

.status-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-right ion-toggle {
  --height: 20px;
}
</style>
