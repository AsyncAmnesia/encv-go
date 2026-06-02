<template>
  <div class="decrypt-body">
    <!-- 源容器 -->
    <div class="form-section">
      <div class="field-group path-field">
        <ion-input
          :model-value="state.sourcePath"
          @ionInput="(e: any) => props.onUpdateSourcePath?.(e.detail.value)"
          :label="t('tasks.sourcePath')"
          label-placement="stacked"
          placeholder="/path/to/container.enc"
          class="path-input"
        ></ion-input>
        <ion-button slot="end" fill="clear" class="browse-btn" @click="handleBrowseSource">
          <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
        </ion-button>
      </div>
    </div>

    <!-- 密码字段（仅 PasswordGlobal 策略显示） -->
    <div v-if="!state.taskOptions || state.taskOptions.passwordStrategy === 'global'" class="form-section password-section">
      <ion-item lines="none" class="password-item">
        <ion-input
          :model-value="state.primaryOverride"
          @ionInput="(e: any) => props.onUpdatePrimaryOverride?.(e.detail.value)"
          :label="t('tasks.passwordOverride')"
          label-placement="stacked"
          type="password"
          :placeholder="t('tasks.passwordOverrideHelp')"
        ></ion-input>
      </ion-item>
      <ion-item lines="none" class="password-item">
        <ion-input
          :model-value="state.secondaryPassword"
          @ionInput="(e: any) => props.onUpdateSecondaryPassword?.(e.detail.value)"
          :label="t('tasks.secondaryPassword')"
          label-placement="stacked"
          type="password"
          :placeholder="t('tasks.secondaryPasswordHelp')"
        ></ion-input>
      </ion-item>
    </div>

    <!-- 解密模式专属 extraFields（condition === 'decrypt' 或无 condition） -->
    <template v-for="field in decryptExtraFields" :key="field.key">
      <ion-item
        v-if="field.type === 'bool'"
        lines="none"
        class="extra-field-item"
      >
        <ion-label>{{ t(field.label) }}</ion-label>
        <ion-toggle
          slot="end"
          :checked="getExtra(field.key) === 'true'"
          @ionChange="(e: any) => { const v = e.detail.checked ? 'true' : 'false'; props.onUpdateExtraValue?.({ key: field.key, value: v }) }"
          class="extra-field-toggle"
        />
        <ion-note v-if="field.help" slot="helper">{{ t(field.help) }}</ion-note>
      </ion-item>

      <ion-item
        v-else-if="field.type === 'select'"
        lines="none"
        class="extra-field-item"
      >
        <ion-select
          :model-value="getExtra(field.key)"
          @ionChange="(e: any) => props.onUpdateExtraValue?.({ key: field.key, value: e.detail.value })"
          :label="t(field.label)"
          interface="action-sheet"
          placement="bottom"
          class="extra-field-select"
        >
          <ion-select-option
            v-for="opt in (field.options || [])"
            :key="opt"
            :value="opt"
          >
            {{ field.optionLabels?.[opt] ?? opt }}
          </ion-select-option>
        </ion-select>
        <ion-note v-if="field.help" slot="helper">{{ t(field.help) }}</ion-note>
      </ion-item>

      <ion-item
        v-else
        lines="none"
        class="extra-field-item"
      >
        <ion-input
          :model-value="getExtra(field.key)"
          @ionInput="(e: any) => props.onUpdateExtraValue?.({ key: field.key, value: e.detail.value })"
          :label="t(field.label)"
          :type="field.type === 'password' ? 'password' : 'text'"
          :placeholder="t(field.help)"
        ></ion-input>
      </ion-item>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  IonItem,
  IonLabel,
  IonSelect,
  IonSelectOption,
  IonInput,
  IonIcon,
  IonToggle,
  IonNote,
  IonButton,
  modalController,
} from '@ionic/vue'
import { folderOpen } from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import FilePickerModal from '@/components/FilePickerModal.vue'
import type { TaskField } from '@/api/encv'
import type { NewTaskState } from '@/components/NewTaskState'

const props = withDefaults(defineProps<{
  state: NewTaskState
  onUpdateSourcePath?: (v: string) => void
  onUpdatePrimaryOverride?: (v: string) => void
  onUpdateSecondaryPassword?: (v: string) => void
  onUpdateExtraValue?: (payload: { key: string; value: string }) => void
}>(), {
  onUpdateSourcePath: undefined,
  onUpdatePrimaryOverride: undefined,
  onUpdateSecondaryPassword: undefined,
  onUpdateExtraValue: undefined,
})

const { t } = useI18n()

const decryptExtraFields = computed<TaskField[]>(() => {
  const arr = Array.isArray(props.state.filteredExtraFields) ? props.state.filteredExtraFields : []
  return arr.filter((f) => !f.condition || f.condition === 'decrypt')
})

function getExtra(key: string): string {
  const ev = props.state?.extraValues
  if (!ev || typeof ev !== 'object') return ''
  return ev[key] || ''
}

async function handleBrowseSource() {
  const modal = await modalController.create({
    component: FilePickerModal,
    componentProps: { mode: 'file' as const },
  })
  await modal.present()
  const { data, role } = await modal.onDidDismiss()
  if (role === 'select' && data) {
    props.onUpdateSourcePath?.(data.path)
  }
}
</script>

<style scoped>
.form-section {
  margin-bottom: 12px;
}

.field-group {
  position: relative;
  margin-bottom: 8px;
}

.path-field {
  display: flex;
  align-items: flex-end;
  gap: 0;
}

.path-field .path-input {
  flex: 1;
}

.browse-btn {
  --padding-start: 6px;
  --padding-end: 6px;
  min-width: 40px;
  min-height: 40px;
  margin-bottom: 2px;
  --color: var(--ion-color-medium);
}

.password-section {
  margin-top: 8px;
}

.password-item {
  --background: transparent;
  --padding-start: 0;
  --padding-end: 0;
  --inner-padding-end: 0;
}

.extra-field-item {
  --background: transparent;
  --padding-start: 0;
  --padding-end: 0;
  --inner-padding-end: 0;
  margin-top: 4px;
  color: var(--ion-text-color);
}

.extra-field-toggle {
  --padding-start: 0;
}

.extra-field-item ion-note[slot=helper] {
  color: var(--ion-text-color, inherit);
  opacity: 0.6;
  font-size: 0.8rem;
}

.extra-field-select {
  width: 100%;
  --padding-start: 0;
}
</style>
