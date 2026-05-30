<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>New Task</ion-title>
        <ion-buttons slot="end">
          <ion-button @click="handleClose">Close</ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>
    <ion-content class="ion-padding">
      <p style="color:red;font-weight:bold;background:#fff;padding:8px;border:2px solid red">RENDER-OK source={{ src }}</p>

      <ion-list>
        <ion-item>
          <ion-label>Task Type</ion-label>
          <ion-select :model-value="taskType" @ionChange="(e) => emit('updateTaskType', (e as CustomEvent).detail.value)" interface="action-sheet">
            <ion-select-option value="encrypt">Encrypt</ion-select-option>
            <ion-select-option value="decrypt">Decrypt</ion-select-option>
          </ion-select>
        </ion-item>

        <ion-item>
          <ion-input
            :model-value="src"
            @ionInput="(e) => emit('updateSourcePath', (e as CustomEvent).detail.value)"
            label="Source Path"
            label-placement="stacked"
            placeholder="/path/to/file"
          ></ion-input>
        </ion-item>

        <ion-item>
          <ion-input
            :model-value="tgt"
            @ionInput="(e) => emit('updateTargetPath', (e as CustomEvent).detail.value)"
            label="Target Path"
            label-placement="stacked"
            placeholder="Leave empty for default"
          ></ion-input>
        </ion-item>
      </ion-list>

      <p v-if="cands.length === 1" style="color:green;padding:8px">Plugin: {{ pluginName }}</p>
      <p v-else-if="cands.length > 1" style="color:orange;padding:8px">{{ cands.length }} candidates</p>

      <ion-button expand="block" @click="emit('submit')" :disabled="!src">
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
import type { PluginCandidate } from '@/api/encv'

const props = defineProps<{
  taskType: string | any
  sourcePath: string | any
  targetPath: string | any
  candidates: PluginCandidate[] | any
  predictedPlugin: string | null
}>()

const emit = defineEmits<{
  (e: 'updateTaskType', v: string): void
  (e: 'updateSourcePath', v: string): void
  (e: 'updateTargetPath', v: string): void
  (e: 'submit'): void
  (e: 'close'): void
}>()

const src = typeof props.sourcePath === 'string' ? props.sourcePath : ''
const tgt = typeof props.targetPath === 'string' ? props.targetPath : ''
const cands = Array.isArray(props.candidates) ? props.candidates : []
const pluginName = typeof props.predictedPlugin === 'string' ? props.predictedPlugin : ''

async function handleClose() {
  await modalController.dismiss()
}
</script>
