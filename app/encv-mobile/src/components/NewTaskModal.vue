<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>{{ t('tasks.newTask') }}</ion-title>
        <ion-buttons slot="end">
          <ion-button @click="handleClose">{{ t('tasks.close') }}</ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>
    <ion-content class="ion-padding">
      <!-- L0 诊断：确认渲染 -->
      <p v-if="false" style="color:red;font-weight:bold;background:#fff;padding:8px;border:2px solid red">
        RENDER-OK src={{ src }} cands={{ cands.length }}
      </p>

      <!-- L0 核心：任务类型 + 路径 -->
      <ion-list>
        <ion-item>
          <ion-select :model-value="taskType" @ionChange="(e) => emit('updateTaskType', (e as CustomEvent).detail.value)" interface="action-sheet" :label="t('tasks.taskType')" label-placement="stacked">
            <ion-select-option value="encrypt">{{ t('tasks.encrypt') }}</ion-select-option>
            <ion-select-option value="decrypt">{{ t('tasks.decrypt') }}</ion-select-option>
          </ion-select>
        </ion-item>

        <ion-item>
          <ion-input :model-value="src" @ionInput="(e) => emit('updateSourcePath', (e as CustomEvent).detail.value)" :label="t('tasks.sourcePath')" label-placement="stacked" placeholder="/path/to/file"></ion-input>
        </ion-item>

        <ion-item>
          <ion-input :model-value="tgt" @ionInput="(e) => emit('updateTargetPath', (e as CustomEvent).detail.value)" :label="t('tasks.targetPath')" label-placement="stacked" :placeholder="t('tasks.targetPathPlaceholder')"></ion-input>
        </ion-item>
      </ion-list>

      <!-- L5 插件提示（安全访问 taskOptions）-->
      <div v-if="cands.length === 1 && pluginName" style="padding:8px 16px;font-size:12px;color:#666;background:#f8f8f8;border-radius:6px;margin:4px 16px">
        {{ t('tasks.willBeHandledBy', { plugin: pluginName }) }}
      </div>
      <div v-else-if="cands.length > 1" style="padding:8px 16px;font-size:12px;color:#666;background:#f8f8f8;border-radius:6px;margin:4px 16px">
        {{ cands.length }} candidates · {{ t('tasks.willBeHandledBy', { plugin: pluginName || '?' }) }}
      </div>

      <!-- L3 容器版本选择（安全守卫）-->
      <ion-item v-if="safeTaskOptions && safeTaskOptions.supportVersionSelect && taskType === 'encrypt'">
        <ContainerVersionSelector :model-value="ver" @update:model-value="(v: number) => emit('updateVersion', v)" :versions="vers" />
      </ion-item>

      <!-- L2 密码字段 -->
      <ion-item>
        <ion-input :model-value="pwdPrimary" @ionInput="(e) => emit('updatePrimaryOverride', (e as CustomEvent).detail.value)" :label="t('tasks.passwordOverride')" label-placement="stacked" type="password" :placeholder="t('tasks.passwordOverrideHelp')"></ion-input>
      </ion-item>

      <ion-item>
        <ion-input :model-value="pwdSecondary" @ionInput="(e) => emit('updateSecondaryPassword', (e as CustomEvent).detail.value)" :label="t('tasks.secondaryPassword')" label-placement="stacked" type="password" :placeholder="t('tasks.secondaryPasswordHelp')"></ion-input>
      </ion-item>

      <!-- L4 额外字段 -->
      <template v-for="field in extraFlds" :key="field.key">
        <ion-item v-if="!field.condition || field.condition === taskType">
          <ion-input :model-value="getExtra(field.key)" @ionInput="(e) => emit('updateExtraValue', { key: field.key, value: (e as CustomEvent).detail.value })" :label="field.label" type="text" :placeholder="field.help"></ion-input>
        </ion-item>
      </template>

      <!-- L0 提交按钮 -->
      <ion-button expand="block" @click="emit('submit')" :disabled="!src">
        Create Task
      </ion-button>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { computed } from 'vue'
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
  IonSelect,
  IonSelectOption,
  IonInput,
  modalController,
} from '@ionic/vue'
import { useI18n } from '@/composables/useI18n'
import ContainerVersionSelector from '@/components/ContainerVersionSelector.vue'
import type { PluginCandidate, TaskOptions, TaskField } from '@/api/encv'

const { t } = useI18n()

const props = defineProps<{
  taskType: string | any
  sourcePath: string | any
  targetPath: string | any
  candidates: PluginCandidate[] | any
  predictedPlugin: string | null
  taskOptions: TaskOptions | null | any
  primaryOverride: string | any
  secondaryPassword: string | any
  version: number | any
  versionOptions: any[] | any
  extraValues: Record<string, string> | any
  filteredExtraFields: TaskField[]
}>()

const emit = defineEmits<{
  (e: 'updateTaskType', v: string): void
  (e: 'updateSourcePath', v: string): void
  (e: 'updateTargetPath', v: string): void
  (e: 'updateVersion', v: number): void
  (e: 'updatePrimaryOverride', v: string): void
  (e: 'updateSecondaryPassword', v: string): void
  (e: 'updateExtraValue', payload: { key: string; value: string }): void
  (e: 'submit'): void
}>()

function safeStr(v: any): string { return typeof v === 'string' ? v : '' }
function safeNum(v: any): number { return typeof v === 'number' ? v : 0 }
function safeArr(v: any): any[] { return Array.isArray(v) ? v : [] }

const src = computed(() => safeStr(props.sourcePath))
const tgt = computed(() => safeStr(props.targetPath))
const cands = computed(() => safeArr(props.candidates))
const pluginName = computed(() => safeStr(props.predictedPlugin))

const pwdPrimary = computed(() => safeStr(props.primaryOverride))
const pwdSecondary = computed(() => safeStr(props.secondaryPassword))
const ver = computed(() => safeNum(props.version))
const vers = computed(() => safeArr(props.versionOptions))
const extraFlds = computed(() => safeArr(props.filteredExtraFields))

const safeTaskOptions = computed<TaskOptions | null>(() => {
  if (!props.taskOptions) return null
  return props.taskOptions
})

function getExtra(key: string): string {
  const ev = props.extraValues
  if (!ev || typeof ev !== 'object') return ''
  return (ev as Record<string, string>)[key] ?? ''
}

async function handleClose() {
  await modalController.dismiss()
}
</script>
