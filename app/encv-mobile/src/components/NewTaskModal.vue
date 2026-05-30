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
      <!-- 任务类型 -->
      <ion-list>
        <ion-item>
          <ion-select :model-value="taskType" @ionChange="(e: any) => emit('updateTaskType', e.detail.value)" interface="action-sheet" :label="t('tasks.taskType')" label-placement="stacked">
            <ion-select-option value="encrypt">{{ t('tasks.encrypt') }}</ion-select-option>
            <ion-select-option value="decrypt">{{ t('tasks.decrypt') }}</ion-select-option>
          </ion-select>
        </ion-item>

        <!-- 源路径（带浏览按钮） -->
        <ion-item>
          <ion-input :model-value="src" @ionInput="(e: any) => emit('updateSourcePath', e.detail.value)" :label="t('tasks.sourcePath')" label-placement="stacked" placeholder="/path/to/file"></ion-input>
          <ion-button slot="end" fill="clear" class="browse-btn" @click="handleBrowseSource">
            <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
          </ion-button>
        </ion-item>

        <!-- 目标路径（带浏览按钮） -->
        <ion-item>
          <ion-input :model-value="tgt" @ionInput="(e: any) => emit('updateTargetPath', e.detail.value)" :label="t('tasks.targetPath')" label-placement="stacked" :placeholder="t('tasks.targetPathPlaceholder')"></ion-input>
          <ion-button slot="end" fill="clear" class="browse-btn" @click="handleBrowseTarget">
            <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
          </ion-button>
        </ion-item>
      </ion-list>

      <!-- 插件选择（当有多个候选时显示） -->
      <div v-if="cands.length > 1" class="plugin-selector">
        <ion-list>
          <ion-item>
            <ion-label>{{ t('tasks.selectPlugin') }}</ion-label>
            <ion-select :model-value="selectedIdx" @ionChange="(e: any) => emit('selectPlugin', e.detail.value)" interface="action-sheet" placement="bottom" style="width: 100%; max-width: 200px;">
              <ion-select-option v-for="(c, idx) in cands" :key="idx" :value="idx">{{ c.name }}</ion-select-option>
            </ion-select>
          </ion-item>
        </ion-list>
      </div>

      <!-- 插件提示 -->
      <div v-if="cands.length === 1 && pluginName" class="plugin-hint">
        <ion-icon :icon="checkmarkCircle" color="success" class="hint-icon"></ion-icon>
        <span>{{ t('tasks.willBeHandledBy', { plugin: pluginName }) }}</span>
      </div>

      <!-- 密码策略提示 -->
      <div v-if="taskOptions?.passwordStrategy === 'independent'" class="plugin-hint password-strategy-hint">
        {{ t('tasks.usesIndependentPassword') }}
      </div>
      <div v-else-if="cands.length > 0 && taskOptions" class="plugin-hint">
        {{ t('tasks.usesGlobalPassword') }}
      </div>

      <!-- 容器版本选择（仅在 taskType='encrypt' 且有版本选项时显示）-->
      <div v-if="taskType === 'encrypt' && vers && vers.length > 0">
        <ContainerVersionSelector :model-value="ver" @update:model-value="(v: number) => emit('updateVersion', v)" :versions="vers" />
      </div>

      <!-- 密码字段 -->
      <ion-item>
        <ion-input :model-value="pwdPrimary" @ionInput="(e: any) => emit('updatePrimaryOverride', e.detail.value)" :label="t('tasks.passwordOverride')" label-placement="stacked" type="password" :placeholder="t('tasks.passwordOverrideHelp')"></ion-input>
      </ion-item>

      <ion-item>
        <ion-input :model-value="pwdSecondary" @ionInput="(e: any) => emit('updateSecondaryPassword', e.detail.value)" :label="t('tasks.secondaryPassword')" label-placement="stacked" type="password" :placeholder="t('tasks.secondaryPasswordHelp')"></ion-input>
      </ion-item>

      <!-- 额外字段 -->
      <template v-for="field in extraFlds" :key="field.key">
        <ion-item v-if="!field.condition || field.condition === taskType">
          <ion-input :model-value="getExtra(field.key)" @ionInput="(e: any) => emit('updateExtraValue', { key: field.key, value: e.detail.value })" :label="field.label" type="text" :placeholder="field.help"></ion-input>
        </ion-item>
      </template>

      <!-- 提交按钮 -->
      <ion-button expand="block" @click="emit('submit')" :disabled="!src">
        <ion-icon :icon="lockClosed" slot="start"></ion-icon>
        {{ t('tasks.createTask') }}
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
  IonIcon,
  IonLabel,
  modalController,
} from '@ionic/vue'
import { folderOpen, lockClosed, checkmarkCircle } from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import ContainerVersionSelector from '@/components/ContainerVersionSelector.vue'
import FilePickerModal from '@/components/FilePickerModal.vue'
import type { PluginCandidate, TaskOptions, TaskField, ContainerVersionInfo } from '@/api/encv'

const { t } = useI18n()

const props = defineProps<{
  taskType: string
  sourcePath: string
  targetPath: string
  candidates: PluginCandidate[]
  predictedPlugin: string | null
  taskOptions: TaskOptions | null
  primaryOverride: string
  secondaryPassword: string
  version: number
  versionOptions: ContainerVersionInfo[]
  extraValues: Record<string, string>
  filteredExtraFields: TaskField[]
  selectedPluginIndex: number
}>()

const emit = defineEmits<{
  (e: 'updateTaskType', v: string): void
  (e: 'updateSourcePath', v: string): void
  (e: 'updateTargetPath', v: string): void
  (e: 'updateVersion', v: number): void
  (e: 'updatePrimaryOverride', v: string): void
  (e: 'updateSecondaryPassword', v: string): void
  (e: 'updateExtraValue', payload: { key: string; value: string }): void
  (e: 'selectPlugin', index: number): void
  (e: 'submit'): void
}>()

const src = computed(() => props.sourcePath || '')
const tgt = computed(() => props.targetPath || '')
const cands = computed(() => Array.isArray(props.candidates) ? props.candidates : [])
const pluginName = computed(() => props.predictedPlugin || '')
const pwdPrimary = computed(() => props.primaryOverride || '')
const pwdSecondary = computed(() => props.secondaryPassword || '')
const ver = computed(() => typeof props.version === 'number' ? props.version : 4)
const vers = computed(() => Array.isArray(props.versionOptions) ? props.versionOptions : [])
const extraFlds = computed(() => Array.isArray(props.filteredExtraFields) ? props.filteredExtraFields : [])
const selectedIdx = computed(() => typeof props.selectedPluginIndex === 'number' ? props.selectedPluginIndex : 0)

function getExtra(key: string): string {
  if (!props.extraValues || typeof props.extraValues !== 'object') return ''
  return props.extraValues[key] || ''
}

async function handleBrowseSource() {
  const modal = await modalController.create({
    component: FilePickerModal,
    componentProps: { mode: 'file' as const },
  })
  await modal.present()
  const { data, role } = await modal.onDidDismiss()
  if (role === 'select' && data) {
    emit('updateSourcePath', data.path)
  }
}

async function handleBrowseTarget() {
  const modal = await modalController.create({
    component: FilePickerModal,
    componentProps: { mode: 'folder' as const },
  })
  await modal.present()
  const { data, role } = await modal.onDidDismiss()
  if (role === 'select' && data) {
    emit('updateTargetPath', data.path)
  }
}

async function handleClose() {
  await modalController.dismiss()
}
</script>

<style scoped>
.plugin-selector {
  margin: 8px 0;
}

.plugin-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  font-size: 12px;
  color: var(--ion-color-medium);
  background: var(--ion-color-step-50, #f8f8f8);
  border-radius: 6px;
  margin: 4px 16px;
}

.hint-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.password-strategy-hint {
  color: var(--ion-color-primary);
  font-weight: 500;
}

.browse-btn {
  --padding-start: 8px;
  --padding-end: 8px;
  min-width: 44px;
  min-height: 44px;
}
</style>
