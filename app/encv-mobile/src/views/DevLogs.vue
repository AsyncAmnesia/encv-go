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

    <ion-content class="log-content">
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

      <div v-if="activeTab === 'frontend'" ref="logListRef" class="log-list">
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

      <div v-else ref="logListRef" class="log-list">
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

    <ion-footer class="status-bar">
      <ion-toolbar>
        <div class="status-inner">
          <span class="status-text">{{ t('devlogs.total', { total: totalCurrent, filtered: filteredCurrent }) }}</span>
          <div class="status-right">
            <ion-toggle v-model="autoScroll" :label-placement="'start'">{{ t('devlogs.autoScroll') }}</ion-toggle>
          </div>
        </div>
      </ion-toolbar>
    </ion-footer>

    <ion-alert-controller></ion-alert-controller>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonContent,
  IonSegment, IonSegmentButton, IonSearchbar, IonButton,
  IonIcon, IonBadge, IonToggle, IonFooter, alertController,
} from '@ionic/vue'
import { trashOutline } from 'ionicons/icons'
import { eventBus } from '@/composables/useEventBus'
import { useI18n } from '@/composables/useI18n'

const { t } = useI18n()

const activeTab = ref<'frontend' | 'backend'>('frontend')
const searchText = ref('')
const autoScroll = ref(true)

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

interface LogEntry { id: number; timestamp: string; level: string; message: string }

let nextId = 0
const frontendLogs = ref<LogEntry[]>([])
const backendLogs = ref<LogEntry[]>([])
const serverOnline = ref(false)

const origConsole = { debug: console.debug, info: console.info, warn: console.warn, error: console.error, log: console.log }

function addLog(level: string, args: any[]) {
  frontendLogs.value.push({
    id: ++nextId,
    timestamp: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
    level,
    message: args.map((a) => typeof a === 'string' ? a : JSON.stringify(a)).join(' '),
  })
}

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

let isUserScrolling = false

function scrollToBottom() {
  if (!autoScroll.value || isUserScrolling) return
  requestAnimationFrame(() => {
    const list = document.querySelector('.log-list:last-of-type') as HTMLElement | null
    if (list) list.scrollTop = list.scrollHeight
  })
}

watch([filteredFrontend, filteredBackend], () => scrollToBottom(), { deep: true })

async function handleClear() {
  const alert = await alertController.create({
    header: t('devlogs.clearConfirm'),
    buttons: [
      { text: t('common.cancel'), role: 'cancel' },
      {
        text: t('common.confirm'), role: 'destructive',
        handler: () => { if (activeTab.value === 'frontend') frontendLogs.value = []; else backendLogs.value = [] },
      },
    ],
  })
  await alert.present()
}

function onWsMessage(data: any) {
  const msg = typeof data === 'string' ? data : JSON.stringify(data)
  backendLogs.value.push({ id: ++nextId, timestamp: new Date().toLocaleTimeString('zh-CN', { hour12: false }), level: 'info', message: msg })
}

function onServerStatus(data: any) {
  serverOnline.value = data?.online ?? false
}

console.debug = (...args: any[]) => { origConsole.debug(...args); addLog('debug', args) }
console.info = (...args: any[]) => { origConsole.info(...args); addLog('info', args) }
console.warn = (...args: any[]) => { origConsole.warn(...args); addLog('warn', args) }
console.error = (...args: any[]) => { origConsole.error(...args); addLog('error', args) }
console.log = (...args: any[]) => { origConsole.log(...args); addLog('info', args) }

onMounted(() => {
  eventBus.on('ws:message', onWsMessage)
  eventBus.on('server:status', onServerStatus)
})

onUnmounted(() => {
  eventBus.off('ws:message', onWsMessage)
  eventBus.off('server:status', onServerStatus)
  console.debug = origConsole.debug
  console.info = origConsole.info
  console.warn = origConsole.warn
  console.error = origConsole.error
  console.log = origConsole.log
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
  background: var(--ion-background-color);
  flex-wrap: nowrap;
  overflow-x: auto;
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
  flex: 1;
  min-width: 100px;
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
</style>
