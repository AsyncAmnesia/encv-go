<template>
  <div class="detail-section">
    <div class="section-title">{{ t('tasks.basicInfo') }}</div>
    <div class="info-grid">
      <div class="info-item">
        <span class="info-label">{{ t('tasks.taskId') }}</span>
        <span class="info-value task-id-value" @click="copyTaskId" title="Click to copy">
          {{ task.id }}
          <ion-icon :icon="copyOutline" class="copy-icon"></ion-icon>
        </span>
      </div>
      <div class="info-item">
        <span class="info-label">{{ t('tasks.fileName') }}</span>
        <span class="info-value file-name">{{ fileName }}</span>
      </div>
      <div class="info-item">
        <span class="info-label">{{ t('tasks.taskType') }}</span>
        <ion-badge :color="task.type === 'encrypt' ? 'primary' : 'warning'">
          {{ task.type === 'encrypt' ? t('tasks.encrypt') : t('tasks.decrypt') }}
        </ion-badge>
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
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { IonBadge, IonIcon } from '@ionic/vue'
import { copyOutline } from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import type { EncvTask } from '@/api/encv'

const props = defineProps<{ task: EncvTask }>()
const { t } = useI18n()

async function copyTaskId() {
  try {
    await navigator.clipboard.writeText(props.task.id)
    showToast({ message: t('tasks.idCopied'), duration: 1500, color: 'success' })
  } catch {
    showToast({ message: t('tasks.idCopyFailed'), duration: 1500, color: 'danger' })
  }
}

const fileName = computed(() => {
  const parts = props.task.sourcePath.split('/')
  return parts[parts.length - 1] || props.task.sourcePath
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

.task-id-value {
  font-family: monospace;
  font-size: 12px;
  color: var(--encv-text-secondary);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  word-break: break-all;
}

.task-id-value:hover {
  color: var(--ion-color-primary);
}

.copy-icon {
  font-size: 14px;
  opacity: 0.5;
  flex-shrink: 0;
}

.task-id-value:hover .copy-icon {
  opacity: 1;
}
</style>
