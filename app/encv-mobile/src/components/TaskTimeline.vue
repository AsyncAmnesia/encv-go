<template>
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
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { IonIcon } from '@ionic/vue'
import { chevronDown, chevronForward } from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import { formatDateTime, formatDuration } from '@/composables/useDateFormat'
import type { EncvTask } from '@/api/encv'

const props = defineProps<{ task: EncvTask }>()
const { t } = useI18n()

const expandedSteps = ref(new Set<number>())

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

function toggleStep(idx: number) {
  if (expandedSteps.value.has(idx)) {
    expandedSteps.value.delete(idx)
  } else {
    expandedSteps.value.add(idx)
  }
}

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
</style>
