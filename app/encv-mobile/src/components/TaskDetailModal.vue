<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>{{ t('tasks.taskDetail') }}</ion-title>
        <ion-buttons slot="end">
          <ion-button @click="handleClose" fill="clear" size="small" color="medium">
            {{ t('tasks.close') }}
          </ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>

    <ion-content class="ion-padding">
      <div class="detail-section">
        <div class="section-title">{{ t('tasks.basicInfo') }}</div>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">{{ t('tasks.fileName') }}</span>
            <span class="info-value file-name">{{ fileName }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">{{ t('tasks.taskType') }}</span>
            <ion-badge :color="task.type === 'encrypt' ? 'primary' : 'warning'">{{ task.type === 'encrypt' ? t('tasks.encrypt') : t('tasks.decrypt') }}</ion-badge>
          </div>
          <div class="info-item" v-if="task.pluginName">
            <span class="info-label">{{ t('tasks.handledBy') }}</span>
            <ion-badge color="primary">{{ task.pluginName }}</ion-badge>
          </div>
          <div class="info-item" v-if="task.containerVersion">
            <span class="info-label">{{ t('tasks.containerVersion') }}</span>
            <span class="info-value">V{{ task.containerVersion }}</span>
          </div>
        </div>
      </div>

      <div class="detail-section">
        <div class="section-title">{{ t('tasks.timeline') }}</div>
        <div class="timeline">
          <div
            v-for="(event, idx) in timelineEvents"
            :key="idx"
            class="timeline-event"
            :class="{
              'event-current': event.isCurrent,
              'event-completed': event.completed,
              'event-error': event.error,
              'event-expandable': event.hasExpandableDetail,
            }"
            @click="toggleStep(idx)"
          >
            <div class="timeline-dot"></div>
            <div class="timeline-content">
              <div class="event-header">
                <span class="event-phase">{{ event.phaseLabel }}</span>
                <span class="event-time">{{ event.time }}</span>
                <ion-icon
                  v-if="event.hasExpandableDetail"
                  :icon="expandedSteps.has(idx) ? chevronDown : chevronForward"
                  class="expand-icon"
                  color="medium"
                />
              </div>
              <p v-if="event.summary" class="event-detail">{{ event.summary }}</p>
              <div v-if="expandedSteps.has(idx) && event.expandDetail" class="event-expand">
                <div v-if="event.expandDetail.duration" class="expand-row">
                  <span class="expand-label">{{ t('tasks.duration') }}</span>
                  <span class="expand-value">{{ event.expandDetail.duration }}</span>
                </div>
                <div v-if="event.expandDetail.startedAt" class="expand-row">
                  <span class="expand-label">{{ t('tasks.startedAt') }}</span>
                  <span class="expand-value">{{ event.expandDetail.startedAt }}</span>
                </div>
                <div v-if="event.expandDetail.completedAt" class="expand-row">
                  <span class="expand-label">{{ t('tasks.completedAt') }}</span>
                  <span class="expand-value">{{ event.expandDetail.completedAt }}</span>
                </div>
                <div v-if="event.expandDetail.outputPath" class="expand-row">
                  <span class="expand-label">{{ t('tasks.outputFile') }}</span>
                  <span class="expand-value expand-path">{{ event.expandDetail.outputPath }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="detail-section" v-if="task.status === 'running' || task.status === 'cancelling'">
        <div class="section-title">{{ t('tasks.progress') }}</div>
        <ion-progress-bar :value="task.progress / 100"></ion-progress-bar>
        <div class="progress-stats">
          <span>{{ task.progress }}%</span>
          <span v-if="task.speed">{{ task.speed }}</span>
          <span v-if="task.eta">ETA: {{ task.eta }}</span>
        </div>
      </div>

      <div class="detail-section" v-if="task.status === 'completed'">
        <div class="section-title completed-section-title">
          <ion-icon :icon="checkmarkCircle" color="success"></ion-icon>
          {{ t('tasks.phaseCompleted') }}
        </div>
        <p class="completed-duration" v-if="durationStr">{{ t('tasks.duration') }}: {{ durationStr }}</p>

        <div v-if="outputInfo" class="output-block">
          <div class="output-header">
            <ion-icon :icon="documentTextOutline" color="primary"></ion-icon>
            <span class="output-label">{{ t('tasks.outputFile') }}</span>
          </div>
          <div class="output-name" :title="outputInfo.name">{{ outputInfo.name }}</div>
          <div class="output-meta">
            <span>{{ outputInfo.sizeLabel }}</span>
            <span class="output-sep">·</span>
            <span>{{ outputInfo.dirLabel }}</span>
          </div>
          <div class="output-actions">
            <ion-button
              v-if="canPreviewOutput"
              size="small"
              color="primary"
              @click="handleOpenOutput"
            >
              <ion-icon :icon="playCircleOutline" slot="start"></ion-icon>
              {{ t('tasks.openOutput') }}
            </ion-button>
            <ion-button
              size="small"
              color="medium"
              fill="outline"
              @click="handleLocateOutput"
            >
              <ion-icon :icon="folderOpenOutline" slot="start"></ion-icon>
              {{ t('tasks.locateInFiles') }}
            </ion-button>
          </div>
        </div>
      </div>

      <div class="detail-section error-section" v-if="task.error">
        <div class="section-title error-section-title">
          <ion-icon :icon="closeCircle" color="danger"></ion-icon>
          {{ t('tasks.error') }}
          <ion-button fill="clear" size="small" class="copy-error-btn" @click="copyErrorDetail">
            <ion-icon :icon="copyOutline" slot="icon-only" size="small"></ion-icon>
            {{ copied ? t('tasks.copied') : t('tasks.copyError') }}
          </ion-button>
        </div>
        <p class="error-msg selectable-text">{{ task.error }}</p>
        <pre v-if="task.errorDetail && task.errorDetail !== task.error" class="error-detail-pre selectable-text">{{ task.errorDetail }}</pre>
      </div>

      <div class="detail-section warning-section" v-if="task.warning">
        <div class="section-title warning-section-title">
          <ion-icon :icon="warningOutline" color="warning"></ion-icon>
          {{ t('tasks.warnings') }}
        </div>
        <p>{{ task.warning }}</p>
        <pre v-if="task.warningDetail" class="warning-detail-pre">{{ formatWarningDetail(task.warningDetail) }}</pre>
      </div>

      <div class="action-buttons">
        <ion-button
          v-if="task.status === 'running'"
          expand="block"
          color="warning"
          @click="handleCancel"
        >
          <ion-icon :icon="closeCircle" slot="start"></ion-icon>
          {{ t('tasks.cancel') }}
        </ion-button>
        <ion-button
          v-if="task.status === 'failed'"
          expand="block"
          color="primary"
          @click="handleRetry"
        >
          <ion-icon :icon="refresh" slot="start"></ion-icon>
          {{ t('tasks.retry') }}
        </ion-button>
        <ion-button
          v-if="['completed', 'failed', 'cancelled'].includes(task.status)"
          expand="block"
          color="danger"
          fill="outline"
          @click="handleRemove"
        >
          <ion-icon :icon="trash" slot="start"></ion-icon>
          {{ t('tasks.remove') }}
        </ion-button>
      </div>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  IonPage,
  IonHeader,
  IonToolbar,
  IonTitle,
  IonButtons,
  IonButton,
  IonContent,
  IonIcon,
  IonBadge,
  IonProgressBar,
  modalController,
} from '@ionic/vue'
import {
  checkmarkCircle,
  closeCircle,
  warningOutline,
  refresh,
  trash,
  copyOutline,
  documentTextOutline,
  playCircleOutline,
  folderOpenOutline,
  chevronDown,
  chevronForward,
} from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import { formatDateTime, formatDuration } from '@/composables/useDateFormat'
import { copyToClipboard } from '@/composables/useClipboard'
import { showToast } from '@/composables/useToast'
import type { EncvTask } from '@/api/encv'

const props = defineProps<{ task: EncvTask }>()
const { t } = useI18n()
const router = useRouter()
const copied = ref(false)
const expandedSteps = ref(new Set<number>())

const PREVIEWABLE_VIDEO = new Set(['mp4', 'webm', 'mov', 'm4v', 'mkv'])

const fileName = computed(() => {
  const parts = props.task.sourcePath.split('/')
  return parts[parts.length - 1] || props.task.sourcePath
})

const outputInfo = computed(() => {
  const op = props.task.outputPath
  if (!op) return null
  const name = op.split('/').pop() || op
  return {
    fullPath: op,
    name,
    dirLabel: dirOf(op),
    sizeLabel: '',
  }
})

const canPreviewOutput = computed(() => {
  if (!outputInfo.value) return false
  const ext = outputInfo.value.name.split('.').pop()?.toLowerCase() || ''
  return PREVIEWABLE_VIDEO.has(ext)
})

function dirOf(p: string): string {
  const idx = p.lastIndexOf('/')
  if (idx < 0) return '/'
  return p.slice(0, idx) || '/'
}

function toggleStep(idx: number) {
  if (expandedSteps.value.has(idx)) {
    expandedSteps.value.delete(idx)
  } else {
    expandedSteps.value.add(idx)
  }
}

function handleOpenOutput() {
  if (!outputInfo.value) return
  const ext = outputInfo.value.name.split('.').pop()?.toLowerCase() || ''
  if (PREVIEWABLE_VIDEO.has(ext)) {
    router.push({
      path: '/player',
      query: { path: outputInfo.value.fullPath, name: outputInfo.value.name },
    })
  } else {
    showToast({ message: t('tasks.previewUnsupportedExt') + ': ' + ext, duration: 2000, color: 'medium' })
  }
  modalController.dismiss({ action: 'opened', id: props.task.id, outputPath: outputInfo.value.fullPath })
}

function handleLocateOutput() {
  if (!outputInfo.value) return
  const dir = dirOf(outputInfo.value.fullPath)
  router.push({ path: '/tabs/files', query: { path: dir, highlight: outputInfo.value.name } })
  modalController.dismiss({ action: 'located', id: props.task.id, outputPath: outputInfo.value.fullPath })
}

const durationStr = computed(() => {
  if (!props.task.createdAt) return ''
  const created = new Date(props.task.createdAt).getTime()
  if (isNaN(created)) return ''
  if (props.task.completedAt) {
    const completed = new Date(props.task.completedAt).getTime()
    if (isNaN(completed)) return ''
    return formatDuration(completed - created)
  }
  return ''
})

interface TimelineExpandDetail {
  startedAt?: string
  completedAt?: string
  duration?: string
  outputPath?: string
}

interface TimelineEvent {
  phase: string
  phaseLabel: string
  time: string
  summary?: string
  isCurrent: boolean
  completed: boolean
  error?: boolean
  hasExpandableDetail: boolean
  expandDetail?: TimelineExpandDetail
}

const timelineEvents = computed(() => {
  const events: TimelineEvent[] = []
  const steps = props.task.steps ?? []

  events.push({
    phase: 'created',
    phaseLabel: t('tasks.timelineCreated'),
    time: formatDateTime(props.task.createdAt),
    isCurrent: false,
    completed: true,
    hasExpandableDetail: false,
  })

  if (steps.length > 0) {
    for (const step of steps) {
      const isCurrent = step.phase === props.task.phase && !step.completedAt
      const completed = !!step.completedAt
      const startedMs = new Date(step.startedAt).getTime()
      const completedMs = step.completedAt ? new Date(step.completedAt).getTime() : 0
      const stepDuration = completed && completedMs > startedMs ? formatDuration(completedMs - startedMs) : undefined

      const expandDetail: TimelineExpandDetail = {}
      let hasExpand = false
      if (step.startedAt) { expandDetail.startedAt = formatDateTime(step.startedAt); hasExpand = true }
      if (step.completedAt) { expandDetail.completedAt = formatDateTime(step.completedAt); hasExpand = true }
      if (stepDuration) { expandDetail.duration = stepDuration; hasExpand = true }
      if (step.detail) { expandDetail.outputPath = step.detail; hasExpand = true }

      events.push({
        phase: step.phase,
        phaseLabel: getPhaseLabel(step.phase),
        time: isCurrent ? t('tasks.timelineInProgress') : (completed ? stepDuration ?? t('tasks.timelineDone') : ''),
        summary: isCurrent && props.task.speed ? `${props.task.progress}% · ${props.task.speed}` + (props.task.eta ? ` · ETA ${props.task.eta}` : '') : undefined,
        isCurrent,
        completed,
        hasExpandableDetail: hasExpand,
        expandDetail: hasExpand ? expandDetail : undefined,
      })
    }
  } else {
    const phases = ['analyzing', 'initializing', 'preprocessing', 'encrypting', 'decrypting', 'packing', 'verifying']
    const phaseOrder = phases.indexOf(props.task.phase ?? '')

    for (let i = 0; i < phases.length; i++) {
      const p = phases[i]
      const isCurrent = p === props.task.phase
      const isPast = !isCurrent && (phaseOrder > i || ['completed', 'failed', 'cancelled'].includes(props.task.status))

      events.push({
        phase: p,
        phaseLabel: getPhaseLabel(p),
        time: isCurrent ? t('tasks.timelineInProgress') : (isPast ? t('tasks.timelineDone') : ''),
        summary: isCurrent && props.task.speed ? `${props.task.progress}% · ${props.task.speed}` + (props.task.eta ? ` · ETA ${props.task.eta}` : '') : undefined,
        isCurrent,
        completed: isPast,
        hasExpandableDetail: false,
      })
    }
  }

  if (props.task.status === 'completed') {
    events.push({
      phase: 'done',
      phaseLabel: t('tasks.phaseCompleted'),
      time: props.task.completedAt ? formatDateTime(props.task.completedAt) : '',
      isCurrent: false,
      completed: true,
      hasExpandableDetail: false,
    })
  }

  if (props.task.status === 'failed' || props.task.status === 'cancelled') {
    const last = events[events.length - 1]
    last.error = true
    last.phaseLabel = props.task.status === 'failed' ? t('tasks.failed') : t('tasks.cancelled')
    last.summary = props.task.error
  }

  return events
})

function getPhaseLabel(phase: string): string {
  switch (phase) {
    case 'analyzing': return t('tasks.phaseAnalyzing')
    case 'initializing': return t('tasks.phaseInitializing')
    case 'preprocessing': return t('tasks.phasePreprocessing')
    case 'encrypting': return t('tasks.phaseEncrypting')
    case 'decrypting': return t('tasks.phaseDecrypting')
    case 'packing': return t('tasks.phasePacking')
    case 'verifying': return t('tasks.phaseVerifying')
    case 'completed': return t('tasks.phaseCompleted')
    default: return phase
  }
}

async function copyErrorDetail() {
  const text = props.task.errorDetail || props.task.error || ''
  if (!text) return
  const ok = await copyToClipboard(text)
  if (ok) {
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  } else {
    showToast({ message: t('tasks.copyFailed'), duration: 2000, color: 'danger' })
  }
}

function formatWarningDetail(detail: string): string {
  try { return JSON.stringify(JSON.parse(detail), null, 2) }
  catch { return detail }
}

async function handleClose() {
  await modalController.dismiss()
}

async function handleCancel() {
  modalController.dismiss({ action: 'cancel', id: props.task.id })
}

async function handleRetry() {
  modalController.dismiss({ action: 'retry', id: props.task.id })
}

async function handleRemove() {
  modalController.dismiss({ action: 'remove', id: props.task.id })
}
</script>

<style scoped>
.detail-section {
  margin-bottom: 20px;
}

.section-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--ion-text-color);
  margin-bottom: 10px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.completed-section-title { color: var(--ion-color-success); }
.error-section-title { color: var(--ion-color-danger); }
.warning-section-title { color: #e65100; }

.info-grid {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 8px 16px;
  align-items: center;
}

.info-item {
  display: contents;
}

.info-label {
  font-size: 12px;
  color: var(--ion-color-medium);
}

.info-value {
  font-size: 14px;
  font-weight: 500;
  justify-self: start;
}

.file-name {
  word-break: break-all;
  max-width: 220px;
}

.timeline {
  position: relative;
  padding-left: 24px;
}

.timeline::before {
  content: '';
  position: absolute;
  left: 7px;
  top: 8px;
  bottom: 8px;
  width: 2px;
  background: var(--ion-color-step-200);
}

.timeline-event {
  position: relative;
  padding-bottom: 16px;
}

.timeline-event:last-child {
  padding-bottom: 4px;
}

.timeline-event.event-expandable {
  cursor: pointer;
}

.timeline-dot {
  position: absolute;
  left: -21px;
  top: 4px;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--ion-color-step-200);
  border: 2px solid var(--ion-color-step-200);
  z-index: 1;
}

.event-completed .timeline-dot {
  background: var(--ion-color-success);
  border-color: var(--ion-color-success);
}

.event-current .timeline-dot {
  background: var(--ion-color-primary);
  border-color: var(--ion-color-primary);
  box-shadow: 0 0 0 4px rgba(var(--ion-color-primary-rgb), 0.2);
  animation: pulse 1.5s infinite;
}

.event-error .timeline-dot {
  background: var(--ion-color-danger);
  border-color: var(--ion-color-danger);
}

@keyframes pulse {
  0%, 100% { box-shadow: 0 0 0 4px rgba(var(--ion-color-primary-rgb), 0.2); }
  50% { box-shadow: 0 0 0 8px rgba(var(--ion-color-primary-rgb), 0.1); }
}

.timeline-content {
  padding-left: 4px;
}

.event-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.event-phase {
  font-size: 13px;
  font-weight: 600;
  color: var(--ion-text-color);
}

.event-time {
  font-size: 11px;
  color: var(--ion-color-medium);
  white-space: nowrap;
}

.expand-icon {
  font-size: 14px;
  flex-shrink: 0;
}

.event-detail {
  font-size: 12px;
  color: var(--ion-color-medium);
  margin-top: 2px;
}

.event-expand {
  margin-top: 6px;
  padding: 8px 10px;
  background: var(--ion-color-step-100, #f0f0f0);
  border-radius: 6px;
  font-size: 11px;
  line-height: 1.6;
}

.expand-row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
}

.expand-row + .expand-row {
  margin-top: 2px;
}

.expand-label {
  color: var(--ion-color-medium);
  flex-shrink: 0;
}

.expand-value {
  color: var(--ion-text-color);
  font-weight: 500;
  text-align: right;
  word-break: break-all;
}

.expand-path {
  font-family: monospace;
  font-size: 10px;
}

.progress-stats {
  display: flex;
  gap: 12px;
  margin-top: 6px;
  font-size: 12px;
  color: var(--ion-color-medium);
  font-weight: 500;
}

.completed-duration {
  font-size: 13px;
  color: var(--ion-color-medium);
  margin-top: 4px;
}

.output-block {
  margin-top: 12px;
  padding: 12px;
  background: rgba(var(--ion-color-primary-rgb), 0.04);
  border: 1px solid rgba(var(--ion-color-primary-rgb), 0.15);
  border-radius: 8px;
}
.output-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 6px;
}
.output-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--ion-color-primary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.output-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--ion-text-color);
  word-break: break-all;
  margin-bottom: 4px;
}
.output-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--ion-color-medium);
  margin-bottom: 10px;
}
.output-sep { opacity: 0.5; }
.output-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.output-actions ion-button {
  --padding-start: 12px;
  --padding-end: 12px;
  height: 36px;
  font-size: 13px;
  font-weight: 500;
}

.error-section { background: rgba(var(--ion-color-danger-rgb), 0.04); border-radius: 8px; padding: 12px; }
.error-msg { font-size: 13px; color: var(--ion-color-danger); margin-top: 4px; word-break: break-word; }
.error-detail-pre {
  background: var(--ion-color-step-100);
  border-radius: 6px;
  padding: 8px 10px;
  margin-top: 6px;
  font-size: 11px;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 150px;
  overflow-y: auto;
  line-height: 1.5;
}
.selectable-text {
  -webkit-user-select: text;
  user-select: text;
}

.copy-error-btn {
  margin-left: auto;
  --color: var(--ion-color-medium);
  --padding-start: 6px;
  --padding-end: 6px;
  font-size: 12px;
  font-weight: 400;
}

.warning-section { background: rgba(255, 152, 0, 0.06); border-radius: 8px; padding: 12px; }
.warning-detail-pre {
  background: var(--ion-color-step-100, #f0f0f0);
  border-radius: 6px;
  padding: 8px 10px;
  margin-top: 6px;
  font-size: 11px;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 120px;
  overflow-y: auto;
  line-height: 1.5;
  color: #666;
}

.action-buttons {
  margin-top: 24px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.action-buttons ion-button {
  --border-radius: 10px;
  height: 44px;
  font-weight: 600;
}
</style>
