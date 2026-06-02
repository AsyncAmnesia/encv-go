<template>
  <ion-item v-if="field.type === 'boolean'" class="config-field" :class="{ 'field-modified': isCustomized }">
    <ion-icon :icon="icon" slot="start"></ion-icon>
    <ion-label>
      {{ label }}
      <span v-if="field.required" class="required-mark">*</span>
      <span v-if="field.isV4" class="config-badge badge-v4">v4</span>
      <span v-else-if="field.platform === 'mobile'" class="config-badge badge-mobile">{{ t('settings.mobileOnly') }}</span>
    </ion-label>
    <ion-toggle slot="end" :checked="!!modelValue" @ionChange="$emit('update:modelValue', $event.detail.checked)"></ion-toggle>
    <ion-icon :icon="cloudOutline" slot="end" class="sync-indicator"></ion-icon>
    <ion-note v-if="field.description || hasDefault" slot="helper" class="field-description">
      <template v-if="field.description">{{ field.description }}</template>
      <span v-if="hasDefault" class="default-hint"> ({{ t('settings.default') }}: {{ formatDefault(defaultVal) }})</span>
    </ion-note>
    <ion-badge v-if="isTaskOverridable" slot="end" color="light" class="task-override-badge">{{ t('settings.taskOverridable') }}</ion-badge>
    <ion-button v-if="isCustomized" slot="end" fill="clear" size="small" class="reset-btn" @click="$emit('reset')">
      <ion-icon :icon="refreshOutline" slot="icon-only"></ion-icon>
    </ion-button>
  </ion-item>

  <ion-item v-else-if="field.isSelect && field.selectOptions && field.selectOptions.length > 2" class="config-field" :class="{ 'field-modified': isCustomized }">
    <ion-icon :icon="icon" slot="start"></ion-icon>
    <ion-label>
      <h3>{{ label }}<span v-if="field.required" class="required-mark">*</span>
        <span v-if="field.isV4" class="config-badge badge-v4">v4</span>
        <span v-else-if="field.platform === 'mobile'" class="config-badge badge-mobile">{{ t('settings.mobileOnly') }}</span>
      </h3>
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
    <ion-icon :icon="cloudOutline" slot="end" class="sync-indicator"></ion-icon>
    <ion-note v-if="hasDefault" slot="helper" class="default-hint">
      {{ t('settings.default') }}: {{ defaultOptionLabel }}
    </ion-note>
    <ion-button v-if="isCustomized" slot="end" fill="clear" size="small" class="reset-btn" @click="$emit('reset')">
      <ion-icon :icon="refreshOutline" slot="icon-only"></ion-icon>
    </ion-button>
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
    <ion-icon :icon="cloudOutline" slot="end" class="sync-indicator"></ion-icon>
    <ion-note v-if="field.description || hasDefault" slot="helper" class="field-description">
      <template v-if="field.description">{{ field.description }}</template>
      <span v-if="hasDefault" class="default-hint"> ({{ t('settings.default') }}: {{ defaultOptionLabel }})</span>
    </ion-note>
    <ion-button v-if="isCustomized" slot="end" fill="clear" size="small" class="reset-btn" @click="$emit('reset')">
      <ion-icon :icon="refreshOutline" slot="icon-only"></ion-icon>
    </ion-button>
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
    <ion-icon :icon="cloudOutline" slot="end" class="sync-indicator"></ion-icon>
    <ion-badge v-if="isTaskOverridable" slot="end" color="light" class="task-override-badge">{{ t('settings.taskOverridable') }}</ion-badge>
    <ion-button v-if="field.isPassword" slot="end" fill="clear" class="browse-btn" @click="showPassword = !showPassword">
      <ion-icon :icon="showPassword ? eyeOffOutline : eyeOutline" slot="icon-only"></ion-icon>
    </ion-button>
    <ion-button v-else-if="field.isPath" slot="end" fill="clear" class="browse-btn" @click="$emit('browse')">
      <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
    </ion-button>
    <ion-button v-if="isCustomized" slot="end" fill="clear" size="small" class="reset-btn" @click="$emit('reset')">
      <ion-icon :icon="refreshOutline" slot="icon-only"></ion-icon>
    </ion-button>
    <ion-note v-if="field.description || hasDefault" slot="helper" class="field-description">
      <template v-if="field.description">{{ field.description }}</template>
      <span v-if="hasDefault" class="default-hint"> ({{ t('settings.default') }}: {{ formatDefault(defaultVal) }})</span>
    </ion-note>
  </ion-item>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { IonIcon, IonItem, IonInput, IonToggle, IonButton, IonNote, IonBadge, IonLabel, IonSelect, IonSelectOption } from '@ionic/vue'
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

.field-description {
  font-size: 12px;
  color: var(--ion-color-medium);
  white-space: normal;
}

.field-description-text {
  font-size: 12px;
  color: var(--ion-color-medium);
  white-space: normal;
  margin-top: 2px;
}

.default-hint {
  font-size: 11px;
  color: var(--ion-color-medium);
}

.required-mark {
  color: var(--ion-color-danger);
  margin-left: 2px;
}

.sync-indicator {
  font-size: 14px;
  color: var(--ion-color-primary);
  opacity: 0.4;
  margin-left: 4px;
}

.config-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 11px;
  font-weight: 600;
  margin-left: 6px;
  vertical-align: middle;
}

.badge-mobile {
  background: #8c61ff;
  color: white;
}

.badge-v4 {
  background: #2dd36f;
  color: white;
}

.task-override-badge {
  font-size: 10px;
  --padding-start: 6px;
  --padding-end: 6px;
  --padding-top: 2px;
  --padding-bottom: 2px;
  margin-left: 8px;
}

.reset-btn {
  --padding-start: 6px;
  --padding-end: 6px;
  min-width: 32px;
  min-height: 32px;
}

.browse-btn {
  --padding-start: 8px;
  --padding-end: 8px;
  min-width: 44px;
  min-height: 44px;
}

.preset-cards {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}

.preset-card {
  flex: 1;
  padding: 12px;
  border: 2px solid #e0e0e0;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.preset-card-active {
  border-color: var(--ion-color-primary);
  background: rgba(var(--ion-color-primary-rgb), 0.08);
}

.preset-card-title {
  font-weight: 600;
  font-size: 14px;
}

.preset-card-desc {
  font-size: 12px;
  color: var(--ion-color-medium);
  margin-top: 4px;
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
