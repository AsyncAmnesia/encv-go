<template>
  <ion-item v-if="field.type === 'boolean'">
    <ion-icon :icon="icon" slot="start"></ion-icon>
    <ion-toggle :checked="!!modelValue" @ionChange="$emit('update:modelValue', $event.detail.checked)">{{ label }}</ion-toggle>
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
    <ion-button v-if="field.isPassword" slot="end" fill="clear" class="browse-btn" @click="showPassword = !showPassword">
      <ion-icon :icon="showPassword ? eyeOffOutline : eyeOutline" slot="icon-only"></ion-icon>
    </ion-button>
    <ion-button v-else-if="field.isPath" slot="end" fill="clear" class="browse-btn" @click="$emit('browse')">
      <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
    </ion-button>
  </ion-item>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { IonIcon, IonItem, IonInput, IonToggle, IonButton } from '@ionic/vue'
import { eyeOutline, eyeOffOutline, folderOpen } from 'ionicons/icons'
import type { FieldDef } from '@/config/schemaParser'

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

const showPassword = ref(false)

const inputType = computed(() => {
  if (!props.field.isPassword) return props.field.type === 'integer' ? 'number' : 'text'
  return showPassword.value ? 'text' : 'password'
})
</script>
