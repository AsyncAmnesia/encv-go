<template>
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
    <pre v-if="task.errorDetail && task.errorDetail !== task.error" class="error-detail-pre selectable-text">{{ formatErrorDetail(task.errorDetail) }}</pre>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { IonButton, IonIcon } from '@ionic/vue'
import { closeCircle, copyOutline } from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import { copyToClipboard } from '@/composables/useClipboard'
import { showToast } from '@/composables/useToast'
import type { EncvTask } from '@/api/encv'

const props = defineProps<{ task: EncvTask }>()
const { t } = useI18n()
const copied = ref(false)

function formatErrorDetail(detail: string): string {
  try { return JSON.stringify(JSON.parse(detail), null, 2) }
  catch { return detail }
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

.error-section-title { color: var(--ion-color-danger); }

.error-section {
  background: rgba(var(--ion-color-danger-rgb), 0.04);
  border-radius: 8px;
  padding: 12px;
}

.error-msg {
  font-size: 13px;
  color: var(--ion-color-danger);
  margin-top: 4px;
  word-break: break-word;
}

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
</style>
