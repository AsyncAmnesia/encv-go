<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>DIAG: NewTaskModal</ion-title>
        <ion-buttons slot="end">
          <ion-button @click="handleClose">Close</ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>
    <ion-content class="ion-padding">
      <p>If you can see this, ion-content renders correctly.</p>
      <p>taskType = {{ val(taskType) }}</p>
      <p>sourcePath = {{ val(sourcePath) }}</p>
      <p>candidates count = {{ arrLen(candidates) }}</p>

      <ion-list>
        <ion-item>
          <ion-label>Task Type</ion-label>
          <ion-select :model-value="val(taskType)" @ionChange="(e: Event) => cb('onUpdateTaskType', (e as CustomEvent).detail.value)" interface="action-sheet">
            <ion-select-option value="encrypt">Encrypt</ion-select-option>
            <ion-select-option value="decrypt">Decrypt</ion-select-option>
          </ion-select>
        </ion-item>
        <ion-item>
          <ion-input :model-value="val(sourcePath)" @ionInput="(e: Event) => cb('onUpdateSourcePath', (e as CustomEvent).detail.value)" label="Source Path" label-placement="stacked" placeholder="/path/to/file"></ion-input>
        </ion-item>
        <ion-item>
          <ion-input :model-value="val(targetPath)" @ionInput="(e: Event) => cb('onUpdateTargetPath', (e as CustomEvent).detail.value)" label="Target Path" label-placement="stacked" placeholder="Leave empty for default"></ion-input>
        </ion-item>
      </ion-list>

      <div v-if="arrLen(candidates) === 1 && predictedPlugin" class="plugin-hint">
        <p>Plugin: {{ predictedPlugin }}</p>
      </div>
      <div v-else-if="arrLen(candidates) > 1" class="plugin-hint">
        <p>{{ arrLen(candidates) }} candidates available</p>
        <ion-select :value="numVal(selectedPluginIndex)" @ionChange="(e: Event) => cb('onUpdateSelectedIndex', (e as CustomEvent).detail.value)">
          <ion-select-option v-for="(cand, idx) in arr(candidates)" :key="cand.name" :value="idx">{{ cand.name }}</ion-select-option>
        </ion-select>
      </div>

      <ion-button expand="block" @click="cb('onSubmit')" :disabled="!val(sourcePath)">
        Create Task
      </ion-button>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import {
  IonContent,
  IonHeader,
  IonPage,
  IonToolbar,
  IonTitle,
  IonButtons,
  IonButton,
  IonList,
  IonItem,
  IonLabel,
  IonSelect,
  IonSelectOption,
  IonInput,
  modalController,
} from '@ionic/vue'
import type { PluginCandidate, TaskOptions } from '@/api/encv'
import type { Ref } from 'vue'
import { isRef } from 'vue'

const props = defineProps<{
  taskType: string | Ref<string>
  sourcePath: string | Ref<string>
  targetPath: string | Ref<string>
  sourcePathError: string | Ref<string>
  targetPathError: string | Ref<string>
  candidates: PluginCandidate[] | Ref<PluginCandidate[]>
  selectedPluginIndex: number | Ref<number>
  predictedPlugin: string | null
  taskOptions: TaskOptions | null | Ref<TaskOptions>
  extraValues: Record<string, string> | Ref<Record<string, string>>
  primaryOverride: string | Ref<string>
  secondaryPassword: string | Ref<string>
  version: number | Ref<number>
  versionOptions: any[] | Ref<any[]>
  filteredExtraFields: any[]
  onUpdateTaskType?: Function
  onUpdateSourcePath?: Function
  onUpdateTargetPath?: Function
  onUpdateSelectedIndex?: Function
  onUpdateVersion?: Function
  onUpdateExtraValue?: Function
  onUpdatePrimaryOverride?: Function
  onUpdateSecondaryPassword?: Function
  onBrowseSource?: Function
  onBrowseTarget?: Function
  onSubmit?: Function
}>()

function val(v: string | Ref<string>): string {
  return typeof v === 'object' && v !== null ? v.value : (v as string)
}
function numVal(v: number | Ref<number>): number {
  return typeof v === 'object' && v !== null ? v.value : (v as number)
}
function arr(v: any[] | Ref<any[]>): any[] {
  return isRef(v) ? v.value : v
}
function arrLen(v: any[] | Ref<any[]>): number {
  return arr(v).length
}

function cb(name: string, ...args: any[]) {
  const fn = (props as Record<string, unknown>)[name] as Function | undefined
  if (fn) fn(...args)
}

async function handleClose() {
  await modalController.dismiss()
}
</script>

<style scoped>
.plugin-hint {
  padding: 8px 16px;
  font-size: 13px;
  color: var(--ion-color-medium);
  background: var(--ion-color-step-50, #f8f8f8);
  border-radius: 6px;
  margin: 8px 16px;
}
</style>
