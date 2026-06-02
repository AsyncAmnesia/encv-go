<template>
  <div class="local-openlist-card" :class="cardClass">
    <div class="card-header">
      <div class="card-title-row">
        <ion-icon :icon="serverIcon" class="card-icon"></ion-icon>
        <span class="card-title">{{ t('remote.localOpenListTitle') }}</span>
        <ion-badge :color="badgeColor" class="status-badge">{{ statusLabel }}</span>
      </div>
    </div>

    <div v-if="state === 'not_installed'" class="card-body card-body-medium">
      <p class="status-line">{{ t('remote.localOpenListNotInstalled') }}</p>
      <ion-button size="small" fill="solid" color="primary" @click="goToSettings">
        <ion-icon :icon="settingsIcon" slot="start"></ion-icon>
        {{ t('remote.localOpenListGoSettings') }}
      </ion-button>
    </div>

    <div v-else-if="state === 'port_conflict'" class="card-body card-body-danger">
      <p class="status-line">{{ t('remote.localOpenListPortConflict', { port: String(port || 5244) }) }}</p>
      <ion-button size="small" fill="solid" color="danger" @click="goToSettings">
        <ion-icon :icon="settingsIcon" slot="start"></ion-icon>
        {{ t('remote.localOpenListGoSettings') }}
      </ion-button>
    </div>

    <div v-else-if="state === 'running'" class="card-body card-body-success">
      <div class="info-grid">
        <div class="info-item">
          <span class="info-label">{{ t('remote.localOpenListPid') }}</span>
          <span class="info-value">{{ pid }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">{{ t('remote.localOpenListPort') }}</span>
          <span class="info-value">{{ port }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">{{ t('remote.localOpenListDataSize') }}</span>
          <span class="info-value">{{ formattedDataSize }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">{{ t('remote.localOpenListHeartbeat') }}</span>
          <span class="info-value" :class="{ 'heartbeat-fresh': isHeartbeatFresh, 'heartbeat-stale': !isHeartbeatFresh }">
            {{ heartbeatLabel }}
          </span>
        </div>
      </div>
      <ion-button size="small" fill="solid" color="primary" class="open-webui-btn" @click="openWebUi">
        <ion-icon :icon="openIcon" slot="start"></ion-icon>
        {{ t('remote.localOpenListOpenWebUi') }}
      </ion-button>
    </div>

    <div v-else class="card-body card-body-medium">
      <p class="status-line">{{ t('remote.localOpenListStopped') }}</p>
    </div>

    <div v-if="error" class="card-error">{{ t('remote.localOpenListLoadFailed') }}: {{ error }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { IonIcon, IonButton, IonBadge } from '@ionic/vue'
import { server as serverIcon, settings as settingsIcon, open as openIcon } from 'ionicons/icons'
import { fetchLocalOpenListStatus, formatFileSize } from '@/api/encv'
import type { LocalOpenListState } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'

const POLL_INTERVAL_MS = 5000
const HEARTBEAT_FRESH_MS = 5000

const { t } = useI18n()
const router = useRouter()

const state = ref<LocalOpenListState>('not_installed')
const running = ref(false)
const pid = ref(0)
const port = ref(5244)
const dataDirSize = ref(0)
const lastHeartbeat = ref(0)
const error = ref('')

let pollTimer: ReturnType<typeof setInterval> | null = null
let nowTickTimer: ReturnType<typeof setInterval> | null = null
const nowMs = ref(Date.now())

const cardClass = computed(() => {
  if (state.value === 'running') return 'state-running'
  if (state.value === 'port_conflict') return 'state-conflict'
  return 'state-idle'
})

const badgeColor = computed(() => {
  if (state.value === 'running') return 'success'
  if (state.value === 'port_conflict') return 'danger'
  return 'medium'
})

const statusLabel = computed(() => {
  if (state.value === 'running') return t('remote.localOpenListRunning')
  if (state.value === 'port_conflict') return t('remote.localOpenListPortConflict', { port: String(port.value || 5244) })
  if (state.value === 'not_installed') return t('remote.localOpenListNotInstalled')
  return t('remote.localOpenListStopped')
})

const formattedDataSize = computed(() => formatFileSize(dataDirSize.value))

const isHeartbeatFresh = computed(() => {
  if (!lastHeartbeat.value) return false
  return nowMs.value - lastHeartbeat.value <= HEARTBEAT_FRESH_MS
})

const heartbeatLabel = computed(() => {
  if (!lastHeartbeat.value) return '-'
  const deltaSec = Math.max(0, Math.floor((nowMs.value - lastHeartbeat.value) / 1000))
  if (deltaSec <= 5) return t('remote.localOpenListHeartbeatFresh')
  return t('remote.localOpenListHeartbeatStale', { seconds: String(deltaSec) })
})

async function pollStatus() {
  try {
    const status = await fetchLocalOpenListStatus()
    state.value = status.state
    running.value = status.running
    pid.value = status.pid
    port.value = status.port || 5244
    dataDirSize.value = status.dataDirSize
    lastHeartbeat.value = status.lastHeartbeat
    error.value = ''
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    console.debug('[LocalOpenListStatusCard] poll failed:', msg)
    error.value = msg
    state.value = 'not_installed'
  }
}

function goToSettings() {
  router.push('/tabs/settings')
}

function openWebUi() {
  window.open(`http://127.0.0.1:${port.value || 5244}/#/login`, '_system')
}

onMounted(() => {
  pollStatus()
  pollTimer = setInterval(pollStatus, POLL_INTERVAL_MS)
  nowTickTimer = setInterval(() => { nowMs.value = Date.now() }, 1000)
})

onUnmounted(() => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  if (nowTickTimer) {
    clearInterval(nowTickTimer)
    nowTickTimer = null
  }
})
</script>

<style scoped>
.local-openlist-card {
  margin: 12px 12px 0;
  padding: 12px 14px;
  border-radius: 10px;
  background: var(--ion-background-color, #ffffff);
  border: 1px solid var(--ion-color-light-shade, #e0e0e0);
  border-left-width: 3px;
  transition: border-color 0.2s;
}

body.dark .local-openlist-card {
  border-color: #2a2a2c;
  background: #1f1f21;
}

.local-openlist-card.state-running {
  border-left-color: var(--ion-color-success);
}

.local-openlist-card.state-conflict {
  border-left-color: var(--ion-color-danger);
}

.local-openlist-card.state-idle {
  border-left-color: var(--ion-color-medium);
}

.card-header {
  margin-bottom: 8px;
}

.card-title-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.card-icon {
  font-size: 16px;
  color: var(--ion-color-primary);
  flex-shrink: 0;
}

.card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ion-text-color, #000);
  flex: 1 1 auto;
}

.status-badge {
  font-size: 11px;
  flex-shrink: 0;
}

.card-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-top: 4px;
}

.status-line {
  margin: 0;
  font-size: 13px;
  color: var(--ion-text-color, #000);
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 6px 14px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.info-label {
  font-size: 11px;
  color: var(--ion-color-medium);
}

.info-value {
  font-size: 13px;
  color: var(--ion-text-color, #000);
  font-weight: 500;
  font-family: monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.heartbeat-fresh {
  color: var(--ion-color-success);
}

.heartbeat-stale {
  color: var(--ion-color-warning);
}

.open-webui-btn {
  align-self: flex-start;
}

.card-error {
  margin-top: 6px;
  font-size: 11px;
  color: var(--ion-color-danger);
  word-break: break-word;
}
</style>
