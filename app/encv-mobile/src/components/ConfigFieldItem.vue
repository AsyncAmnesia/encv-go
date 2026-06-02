<template>
  <ion-item v-if="field.type === 'boolean'">
    <ion-icon :icon="icon" slot="start"></ion-icon>
    <ion-toggle :checked="!!modelValue" @ionChange="$emit('update:modelValue', $event.detail.checked)">{{ label }}</ion-toggle>
    <ion-badge v-if="isTaskOverridable" slot="end" color="light" class="task-override-badge">{{ t('settings.taskOverridable') }}</ion-badge>
    <ion-note slot="helper" class="default-value-note">
      {{ t('settings.default') }}: {{ formatDefault(defaultVal) }}
      <span :class="isCustomized ? 'customized-tag' : 'default-tag'">（{{ isCustomized ? t('settings.customized') : t('settings.defaultValue') }}）</span>
    </ion-note>
  </ion-item>
  <ion-item v-else>
    <ion-icon :icon="icon" slot="start"></ion-icon>
    <ion-input
      :value="String(modelValue ?? '')"
      :type="inputType"
      :label="label"
      label-placement="stacked"
      :placeholder="placeholder"
      @ionInput="$emit('input', $event)"
    ></ion-input>
    <ion-badge v-if="isTaskOverridable" slot="end" color="light" class="task-override-badge">{{ t('settings.taskOverridable') }}</ion-badge>
    <ion-button v-if="field.isPassword" slot="end" fill="clear" class="browse-btn" @click="showPassword = !showPassword">
      <ion-icon :icon="showPassword ? eyeOffOutline : eyeOutline" slot="icon-only"></ion-icon>
    </ion-button>
    <ion-button v-else-if="field.isPath" slot="end" fill="clear" class="browse-btn" @click="$emit('browse')">
      <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
    </ion-button>
    <ion-note slot="helper" class="default-value-note">
      {{ t('settings.default') }}: {{ formatDefault(defaultVal) }}
      <span :class="isCustomized ? 'customized-tag' : 'default-tag'">（{{ isCustomized ? t('settings.customized') : t('settings.defaultValue') }}）</span>
    </ion-note>
  </ion-item>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { IonIcon, IonItem, IonInput, IonToggle, IonButton, IonNote, IonBadge } from '@ionic/vue'
import { eyeOutline, eyeOffOutline, folderOpen } from 'ionicons/icons'
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
}>()

const { t } = useI18n()
const showPassword = ref(false)

const defaultVal = computed(() => getDefaultValue(props.field))

const isTaskOverridable = computed(() => TASK_OVERRIDABLE.has(props.field.key))

const isCustomized = computed(() => {
  const current = props.modelValue
  const def = defaultVal.value
  if (current === def) return false
  if (current == null && (def === '' || def === 0 || def === false)) return false
  return String(current) !== String(def)
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
.default-value-note {
  font-size: 12px;
  color: var(--ion-color-medium);
}

.default-tag {
  color: var(--ion-color-medium);
  font-size: 11px;
}

.customized-tag {
  color: var(--ion-color-primary);
  font-size: 11px;
}

.task-override-badge {
  font-size: 10px;
  --padding-start: 6px;
  --padding-end: 6px;
  --padding-top: 2px;
  --padding-bottom: 2px;
  margin-left: 8px;
}
</style>

<style>
body.dark .task-override-badge {
  --ion-color-light: #3a3a3c;
  --ion-color-light-rgb: 58, 58, 60;
  --ion-color-light-contrast: #e0e0e0;
  --ion-color-light-contrast-rgb: 224, 224, 224;
}
</style>
