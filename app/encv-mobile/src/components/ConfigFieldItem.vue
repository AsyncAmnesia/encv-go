<template>
  <ion-item v-if="field.type === 'boolean'" class="config-field" :class="{ 'field-modified': isCustomized }">
    <ion-icon :icon="icon" slot="start"></ion-icon>
    <ion-label class="ion-text-wrap">
      <div class="field-label-row">
        <span class="field-label-text">
          {{ label }}
          <span v-if="field.required" class="required-mark">*</span>
        </span>
        <span v-if="field.isV4" class="config-badge badge-v4">v4</span>
        <span v-else-if="field.platform === 'mobile'" class="config-badge badge-mobile">{{ t('settings.mobileOnly') }}</span>
        <ion-icon :icon="cloudOutline" class="sync-indicator" :title="t('settings.synced')"></ion-icon>
      </div>
    </ion-label>
    <ion-toggle slot="end" :checked="!!modelValue" @ionChange="$emit('update:modelValue', $event.detail.checked)"></ion-toggle>
    <ion-button v-if="isCustomized" slot="end" fill="clear" size="small" class="reset-btn" @click="$emit('reset')">
      <ion-icon :icon="refreshOutline" slot="icon-only"></ion-icon>
    </ion-button>
    <ion-note v-if="field.description || hasDefault || isTaskOverridable" slot="helper" class="field-description">
      <template v-if="field.description">{{ field.description }}</template>
      <span v-if="hasDefault" class="default-hint"> ({{ t('settings.default') }}: {{ formatDefault(defaultVal) }})</span>
      <span v-if="isTaskOverridable" class="override-hint"> · {{ t('settings.taskOverridable') }}</span>
    </ion-note>
  </ion-item>

  <ion-item v-else-if="field.isSelect && field.selectOptions && field.selectOptions.length > 2" class="config-field select-card-field" :class="{ 'field-modified': isCustomized }">
    <ion-icon :icon="icon" slot="start"></ion-icon>
    <ion-label class="ion-text-wrap">
      <div class="field-label-row">
        <span class="field-label-text">
          {{ label }}
          <span v-if="field.required" class="required-mark">*</span>
        </span>
        <span v-if="field.isV4" class="config-badge badge-v4">v4</span>
        <span v-else-if="field.platform === 'mobile'" class="config-badge badge-mobile">{{ t('settings.mobileOnly') }}</span>
        <ion-icon :icon="cloudOutline" class="sync-indicator" :title="t('settings.synced')"></ion-icon>
      </div>
      <p v-if="field.description" class="field-description-text">{{ field.description }}</p>
    </ion-label>
    <div class="preset-cards" slot="end">
      <div
        v-for="opt in field.selectOptions"
        :key="opt.value"
        class="preset-card"
        :class="{ 'preset-card-active': String(modelValue) === opt.value }"
        @click="$emit('update:modelValue', opt.value)"
      >
        <div class="preset-card-title">{{ opt.label }}</div>
        <div v-if="opt.description" class="preset-card-desc">{{ opt.description }}</div>
      </div>
    </div>
    <ion-button v-if="isCustomized" slot="end" fill="clear" size="small" class="reset-btn" @click="$emit('reset')">
      <ion-icon :icon="refreshOutline" slot="icon-only"></ion-icon>
    </ion-button>
    <ion-note v-if="hasDefault" slot="helper" class="default-hint">
      {{ t('settings.default') }}: {{ defaultOptionLabel }}
    </ion-note>
  </ion-item>

  <ion-item v-else-if="field.isSelect" class="config-field" :class="{ 'field-modified': isCustomized }">
    <ion-icon :icon="icon" slot="start"></ion-icon>
    <ion-select
      :value="String(modelValue ?? '')"
      :label="labelWithRequired"
      label-placement="stacked"
      interface="action-sheet"
      mode="ios"
      @ionChange="$emit('update:modelValue', $event.detail.value)"
    >
      <ion-select-option v-for="opt in field.selectOptions" :key="opt.value" :value="opt.value">
        {{ opt.label }}
      </ion-select-option>
    </ion-select>
    <ion-button v-if="isCustomized" slot="end" fill="clear" size="small" class="reset-btn" @click="$emit('reset')">
      <ion-icon :icon="refreshOutline" slot="icon-only"></ion-icon>
    </ion-button>
    <ion-note v-if="field.description || hasDefault" slot="helper" class="field-description">
      <span class="meta-row">
        <ion-icon :icon="cloudOutline" class="sync-indicator-inline" :title="t('settings.synced')"></ion-icon>
        <span v-if="field.isV4" class="config-badge badge-v4">v4</span>
        <span v-else-if="field.platform === 'mobile'" class="config-badge badge-mobile">{{ t('settings.mobileOnly') }}</span>
        <template v-if="field.description">{{ field.description }}</template>
      </span>
      <span v-if="hasDefault" class="default-hint"> ({{ t('settings.default') }}: {{ defaultOptionLabel }})</span>
    </ion-note>
  </ion-item>

  <ion-item v-else class="config-field" :class="{ 'field-modified': isCustomized }">
    <ion-icon :icon="icon" slot="start"></ion-icon>
    <ion-input
      :value="String(modelValue ?? '')"
      :type="inputType"
      :label="labelWithRequired"
      label-placement="stacked"
      :placeholder="placeholder"
      @ionInput="$emit('input', $event)"
    ></ion-input>
    <ion-button v-if="field.isPassword" slot="end" fill="clear" class="browse-btn" @click="showPassword = !showPassword">
      <ion-icon :icon="showPassword ? eyeOffOutline : eyeOutline" slot="icon-only"></ion-icon>
    </ion-button>
    <ion-button v-else-if="field.isPath" slot="end" fill="clear" class="browse-btn" @click="$emit('browse')">
      <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
    </ion-button>
    <ion-button v-if="isCustomized" slot="end" fill="clear" size="small" class="reset-btn" @click="$emit('reset')">
      <ion-icon :icon="refreshOutline" slot="icon-only"></ion-icon>
    </ion-button>
    <ion-note v-if="field.description || hasDefault || isTaskOverridable" slot="helper" class="field-description">
      <span class="meta-row">
        <ion-icon :icon="cloudOutline" class="sync-indicator-inline" :title="t('settings.synced')"></ion-icon>
        <span v-if="field.isV4" class="config-badge badge-v4">v4</span>
        <span v-else-if="field.platform === 'mobile'" class="config-badge badge-mobile">{{ t('settings.mobileOnly') }}</span>
        <template v-if="field.description">{{ field.description }}</template>
      </span>
      <span v-if="hasDefault" class="default-hint"> ({{ t('settings.default') }}: {{ formatDefault(defaultVal) }})</span>
      <span v-if="isTaskOverridable" class="override-hint"> · {{ t('settings.taskOverridable') }}</span>
    </ion-note>
  </ion-item>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { IonIcon, IonItem, IonInput, IonToggle, IonButton, IonNote, IonLabel, IonSelect, IonSelectOption } from '@ionic/vue'
import { eyeOutline, eyeOffOutline, folderOpen, cloudOutline, refreshOutline } from 'ionicons/icons'
import type { FieldDef } from '@/config/schemaParser'
import { getDefaultValue } from '@/config/schemaParser'
import { useI18n } from '@/composables/useI18n'

const TASK_OVERRIDABLE = new Set(['password', 'output_path', 'recover'])

const props = defineProps<{
  field: FieldDef
  modelValue: unknown
  label: string
  placeholder?: string
  icon?: string | { name: string; ios: string; md: string }
}>()

defineEmits<{
  'update:modelValue': [value: unknown]
  input: [event: CustomEvent]
  browse: []
  reset: []
}>()

const { t } = useI18n()
const showPassword = ref(false)

const defaultVal = computed(() => getDefaultValue(props.field))

const isTaskOverridable = computed(() => TASK_OVERRIDABLE.has(props.field.key))

const hasDefault = computed(() => props.field.default !== undefined)

const isCustomized = computed(() => {
  const current = props.modelValue
  const def = defaultVal.value
  if (current === def) return false
  if (current == null && (def === '' || def === 0 || def === false)) return false
  return String(current) !== String(def)
})

const labelWithRequired = computed(() => {
  return props.label + (props.field.required ? ' *' : '')
})

const defaultOptionLabel = computed(() => {
  if (!props.field.selectOptions || !props.field.default) return formatDefault(defaultVal.value)
  const opt = props.field.selectOptions.find(o => o.value === String(props.field.default))
  return opt ? opt.label : formatDefault(defaultVal.value)
})

function formatDefault(val: unknown): string {
  if (val === undefined || val === null) return '-'
  if (typeof val === 'boolean') return val ? 'true' : 'false'
  return String(val)
}

const inputType = computed(() => {
  if (!props.field.isPassword) return props.field.type === 'integer' ? 'number' : 'text'
  return showPassword.value ? 'text' : 'password'
})
</script>

<style scoped>
.config-field {
  --min-height: 56px;
}

.config-field.field-modified {
  border-left: 3px solid var(--ion-color-primary);
  --padding-start: 13px;
}

.field-label-row {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}

.field-label-text {
  flex: 1 1 auto;
  min-width: 0;
}

.field-description {
  font-size: 12px;
  color: var(--ion-color-medium);
  white-space: normal;
  line-height: 1.4;
}

.meta-row {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}

.field-description-text {
  font-size: 12px;
  color: var(--ion-color-medium);
  white-space: normal;
  margin: 2px 0 0;
}

.default-hint {
  font-size: 11px;
  color: var(--ion-color-medium);
}

.override-hint {
  font-size: 11px;
  color: var(--ion-color-primary);
  font-weight: 500;
}

.required-mark {
  color: var(--ion-color-danger);
  margin-left: 2px;
}

.sync-indicator {
  font-size: 12px;
  color: var(--ion-color-primary);
  opacity: 0.4;
  flex-shrink: 0;
}

.sync-indicator-inline {
  font-size: 11px;
  color: var(--ion-color-primary);
  opacity: 0.5;
  flex-shrink: 0;
}

.config-badge {
  display: inline-block;
  padding: 1px 6px;
  border-radius: 8px;
  font-size: 10px;
  font-weight: 600;
  flex-shrink: 0;
  line-height: 1.4;
}

.badge-mobile {
  background: #8c61ff;
  color: white;
}

.badge-v4 {
  background: #2dd36f;
  color: white;
}

.reset-btn {
  --padding-start: 4px;
  --padding-end: 4px;
  min-width: 28px;
  min-height: 28px;
  margin: 0;
}

.reset-btn ion-icon {
  font-size: 16px;
}

.browse-btn {
  --padding-start: 6px;
  --padding-end: 6px;
  min-width: 40px;
  min-height: 40px;
  margin: 0;
}

.preset-cards {
  display: flex;
  gap: 8px;
  margin-top: 8px;
  width: 100%;
  flex-wrap: wrap;
}

.preset-card {
  flex: 1 1 0;
  min-width: 80px;
  padding: 10px 8px;
  border: 2px solid var(--ion-color-light-shade, #e0e0e0);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s;
  text-align: center;
}

.preset-card-active {
  border-color: var(--ion-color-primary);
  background: rgba(var(--ion-color-primary-rgb), 0.08);
}

.preset-card-title {
  font-weight: 600;
  font-size: 13px;
}

.preset-card-desc {
  font-size: 11px;
  color: var(--ion-color-medium);
  margin-top: 3px;
  line-height: 1.3;
}

/* 移动端：preset-card 垂直堆叠 */
@media (max-width: 599px) {
  .preset-cards {
    flex-direction: column;
    gap: 6px;
  }
  .preset-card {
    flex: 1 1 auto;
    width: 100%;
    padding: 10px 12px;
    text-align: left;
  }
  .preset-card-title {
    font-size: 14px;
  }
  .preset-card-desc {
    font-size: 12px;
  }
  .config-badge {
    font-size: 9px;
    padding: 1px 5px;
  }
  .sync-indicator {
    font-size: 11px;
  }
}
</style>

<style>
body.dark .task-override-badge {
  --ion-color-light: #3a3a3c;
  --ion-color-light-rgb: 58, 58, 60;
  --ion-color-light-contrast: #e0e0e0;
  --ion-color-light-contrast-rgb: 224, 224, 224;
}

body.dark .preset-card {
  border-color: #3a3a3c;
}

body.dark .preset-card-active {
  border-color: var(--ion-color-primary);
  background: rgba(var(--ion-color-primary-rgb), 0.12);
}
</style>
