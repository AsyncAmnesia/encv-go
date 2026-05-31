<template>
  <ion-page>
    <ion-header class="modal-header">
      <ion-toolbar>
        <ion-title>{{ t('tasks.newTask') }}</ion-title>
        <ion-buttons slot="end">
          <ion-button @click="handleClose" fill="clear" size="small" color="medium">
            {{ t('tasks.close') }}
          </ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>

    <ion-content class="ion-padding">
      <div class="form-section">
        <!-- 任务类型 -->
        <div class="field-group">
          <ion-select
            :model-value="taskType"
            @ionChange="(e: any) => { emit('updateTaskType', e.detail.value); props.onUpdateTaskType?.(e.detail.value) }"
            interface="action-sheet"
            :label="t('tasks.taskType')"
            label-placement="stacked"
            class="task-type-select"
          >
            <ion-select-option value="encrypt">{{ t('tasks.encrypt') }}</ion-select-option>
            <ion-select-option value="decrypt">{{ t('tasks.decrypt') }}</ion-select-option>
          </ion-select>
        </div>

        <!-- 源路径（带浏览按钮） -->
        <div class="field-group path-field">
          <ion-input
            :model-value="src"
            @ionInput="(e: any) => { emit('updateSourcePath', e.detail.value); props.onUpdateSourcePath?.(e.detail.value) }"
            :label="t('tasks.sourcePath')"
            label-placement="stacked"
            placeholder="/path/to/file"
            class="path-input"
          ></ion-input>
          <ion-button slot="end" fill="clear" class="browse-btn" @click="handleBrowseSource">
            <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
          </ion-button>
        </div>

        <!-- 目标路径（带浏览按钮） -->
        <div class="field-group path-field">
          <ion-input
            :model-value="tgt"
            @ionInput="(e: any) => { emit('updateTargetPath', e.detail.value); props.onUpdateTargetPath?.(e.detail.value) }"
            :label="t('tasks.targetPath')"
            label-placement="stacked"
            :placeholder="t('tasks.targetPathPlaceholder')"
            class="path-input"
          ></ion-input>
          <ion-button slot="end" fill="clear" class="browse-btn" @click="handleBrowseTarget">
            <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
          </ion-button>
        </div>
      </div>

      <!-- 插件信息区域 -->
      <div v-if="isPredicting" class="plugin-section predicting">
        <ion-spinner name="crescent" class="predict-spinner"></ion-spinner>
        <span class="predict-text">{{ t('tasks.analyzingFile') }}</span>
      </div>

      <div v-else-if="cands.length > 1" class="plugin-section multi-plugin">
        <div class="section-label">{{ t('tasks.selectPlugin') }}</div>
        <ion-select
          :model-value="selectedIdx"
          @ionChange="(e: any) => { emit('selectPlugin', e.detail.value); props.onSelectPlugin?.(e.detail.value) }"
          interface="action-sheet"
          placement="bottom"
          class="plugin-select"
        >
          <ion-select-option v-for="(c, idx) in cands" :key="idx" :value="idx">
            {{ c.name }}
            <span class="match-type-badge" :class="'mt-' + c.matchType">{{ c.matchType }}</span>
          </ion-select-option>
        </ion-select>
      </div>

      <div v-else-if="cands.length === 1 && pluginName" class="plugin-section single-plugin">
        <ion-icon :icon="checkmarkCircle" color="success" class="plugin-check"></ion-icon>
        <div class="plugin-info">
          <span class="plugin-name">{{ pluginName }}</span>
          <span class="plugin-match-type">{{ getMatchTypeLabel(cands[0].matchType) }}</span>
        </div>
      </div>

      <!-- 密码策略提示 -->
      <div v-if="!isPredicting && taskOpts" class="plugin-hint" :class="{ 'strategy-independent': taskOpts.passwordStrategy === 'independent' }">
        {{ taskOpts.passwordStrategy === 'independent' ? t('tasks.usesIndependentPassword') : t('tasks.usesGlobalPassword') }}
      </div>

      <!-- 容器版本选择（仅插件声明 SupportVersionSelect 时显示） -->
      <div v-if="taskType === 'encrypt' && taskOpts?.supportVersionSelect && vers && vers.length > 0" class="version-section">
        <ContainerVersionSelector
          :model-value="ver"
          @update:model-value="(v: number) => { emit('updateVersion', v); props.onUpdateVersion?.(v) }"
          :versions="vers"
        />
      </div>

      <!-- 密码字段（仅 PasswordGlobal 策略显示） -->
      <div v-if="!taskOpts || taskOpts.passwordStrategy === 'global'" class="form-section password-section">
        <ion-item lines="none" class="password-item">
          <ion-input
            :model-value="pwdPrimary"
            @ionInput="(e: any) => { emit('updatePrimaryOverride', e.detail.value); props.onUpdatePrimaryOverride?.(e.detail.value) }"
            :label="t('tasks.passwordOverride')"
            label-placement="stacked"
            type="password"
            :placeholder="t('tasks.passwordOverrideHelp')"
          ></ion-input>
        </ion-item>
        <ion-item lines="none" class="password-item">
          <ion-input
            :model-value="pwdSecondary"
            @ionInput="(e: any) => { emit('updateSecondaryPassword', e.detail.value); props.onUpdateSecondaryPassword?.(e.detail.value) }"
            :label="t('tasks.secondaryPassword')"
            label-placement="stacked"
            type="password"
            :placeholder="t('tasks.secondaryPasswordHelp')"
          ></ion-input>
        </ion-item>
      </div>

      <!-- 额外字段（声明式渲染，按 type 分支） -->
      <template v-for="field in extraFlds" :key="field.key">
        <!-- bool 类型: 开关 -->
        <ion-item
          v-if="(!field.condition || field.condition === taskType) && field.type === 'bool'"
          lines="none"
          class="extra-field-item"
        >
          <ion-label>{{ t(field.label) }}</ion-label>
          <ion-toggle
            slot="end"
            :checked="getExtra(field.key) === 'true' || getExtra(field.key) === true"
            @ionChange="(e: any) => { const v = e.detail.checked ? 'true' : 'false'; emit('updateExtraValue', { key: field.key, value: v }); props.onUpdateExtraValue?.({ key: field.key, value: v }) }"
            class="extra-field-toggle"
          />
          <ion-note v-if="field.help" slot="helper">{{ t(field.help) }}</ion-note>
        </ion-item>

        <!-- select 类型: 下拉选择 -->
        <ion-item
          v-else-if="(!field.condition || field.condition === taskType) && field.type === 'select'"
          lines="none"
          class="extra-field-item"
        >
          <ion-select
            :model-value="getExtra(field.key)"
            @ionChange="(e: any) => { emit('updateExtraValue', { key: field.key, value: e.detail.value }); props.onUpdateExtraValue?.({ key: field.key, value: e.detail.value }) }"
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

        <!-- string / password 类型: 文本输入（原有逻辑） -->
        <ion-item
          v-else-if="!field.condition || field.condition === taskType"
          lines="none"
          class="extra-field-item"
        >
          <ion-input
            :model-value="getExtra(field.key)"
            @ionInput="(e: any) => { emit('updateExtraValue', { key: field.key, value: e.detail.value }); props.onUpdateExtraValue?.({ key: field.key, value: e.detail.value }) }"
            :label="t(field.label)"
            :type="field.type === 'password' ? 'password' : 'text'"
            :placeholder="t(field.help)"
          ></ion-input>
        </ion-item>
      </template>

      <!-- 提交按钮 -->
      <ion-button
        expand="block"
        class="submit-btn"
        :disabled="!src || isPredicting"
        @click="() => { emit('submit'); props.onSubmit?.() }"
      >
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
  IonItem,
  IonLabel,
  IonSelect,
  IonSelectOption,
  IonInput,
  IonIcon,
  IonSpinner,
  IonToggle,
  IonNote,
  modalController,
} from '@ionic/vue'
import { folderOpen, lockClosed, checkmarkCircle } from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import ContainerVersionSelector from '@/components/ContainerVersionSelector.vue'
import FilePickerModal from '@/components/FilePickerModal.vue'
import type { PluginCandidate, ContainerVersionInfo, TaskField, TaskOptions } from '@/api/encv'

interface NewTaskState {
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
}

const { t } = useI18n()

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

const props = withDefaults(defineProps<{
  state?: NewTaskState
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
  onUpdateTaskType?: (v: string) => void
  onUpdateSourcePath?: (v: string) => void
  onUpdateTargetPath?: (v: string) => void
  onUpdateVersion?: (v: number) => void
  onUpdatePrimaryOverride?: (v: string) => void
  onUpdateSecondaryPassword?: (v: string) => void
  onUpdateExtraValue?: (payload: { key: string; value: string }) => void
  onSelectPlugin?: (index: number) => void
  onSubmit?: () => void
}>(), {
  state: undefined,
  onUpdateTaskType: undefined,
  onUpdateSourcePath: undefined,
  onUpdateTargetPath: undefined,
  onUpdateVersion: undefined,
  onUpdatePrimaryOverride: undefined,
  onUpdateSecondaryPassword: undefined,
  onUpdateExtraValue: undefined,
  onSelectPlugin: undefined,
  onSubmit: undefined,
})

const src = computed(() => props.state?.sourcePath ?? props.sourcePath ?? '')
const tgt = computed(() => props.state?.targetPath ?? props.targetPath ?? '')
const taskType = computed(() => props.state?.taskType ?? props.taskType ?? 'encrypt')
const cands = computed(() => {
  const arr = props.state?.candidates ?? props.candidates
  return Array.isArray(arr) ? arr : []
})
const pluginName = computed(() => props.state?.predictedPlugin ?? props.predictedPlugin ?? '')
const pwdPrimary = computed(() => props.state?.primaryOverride ?? props.primaryOverride ?? '')
const pwdSecondary = computed(() => props.state?.secondaryPassword ?? props.secondaryPassword ?? '')
const ver = computed(() => props.state?.version ?? (typeof props.version === 'number' ? props.version : 4))
const vers = computed(() => {
  const arr = props.state?.versionOptions ?? props.versionOptions
  return Array.isArray(arr) ? arr : []
})
const extraFlds = computed(() => {
  const arr = props.state?.filteredExtraFields ?? props.filteredExtraFields
  return Array.isArray(arr) ? arr : []
})
const selectedIdx = computed(() =>
  typeof (props.state?.selectedPluginIndex ?? props.selectedPluginIndex) === 'number'
    ? (props.state!.selectedPluginIndex)
    : 0
)
const taskOpts = computed(() => props.state?.taskOptions ?? props.taskOptions ?? null)

const isPredicting = computed(() => {
  return src.value.length > 0 && cands.value.length === 0 && !pluginName.value
})

function getMatchTypeLabel(matchType: string): string {
  switch (matchType) {
    case 'mime': return 'MIME'
    case 'extension': return 'Extension'
    case 'general': return 'General'
    case 'container': return 'Container'
    default: return matchType
  }
}

function getExtra(key: string): string {
  const ev = props.state?.extraValues ?? props.extraValues
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
    emit('updateSourcePath', data.path)
    props.onUpdateSourcePath?.(data.path)
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
    props.onUpdateTargetPath?.(data.path)
  }
}

async function handleClose() {
  await modalController.dismiss()
}
</script>

<style scoped>
.modal-header ion-toolbar {
  --padding-start: 8px;
  --padding-end: 4px;
}

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

/* 插件区域 */
.plugin-section {
  margin: 10px 0;
  padding: 10px 14px;
  border-radius: 10px;
  background: var(--ion-color-step-50, #f8f9fa);
}

.plugin-section.predicting {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px;
  justify-content: center;
}

.predict-spinner {
  width: 20px;
  height: 20px;
  --color: var(--ion-color-primary);
}

.predict-text {
  font-size: 13px;
  color: var(--ion-color-medium);
}

.plugin-section.multi-plugin {
  padding: 10px 14px;
}

.section-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--ion-color-medium-shade);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 6px;
}

.plugin-select {
  width: 100%;
  --padding-start: 0;
  max-width: none;
}

.plugin-section.single-plugin {
  display: flex;
  align-items: center;
  gap: 10px;
}

.plugin-check {
  font-size: 20px;
  flex-shrink: 0;
}

.plugin-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.plugin-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--ion-text-color);
}

.plugin-match-type {
  font-size: 11px;
  color: var(--ion-color-primary);
  background: rgba(var(--ion-color-primary-rgb), 0.1);
  padding: 1px 8px;
  border-radius: 10px;
  align-self: flex-start;
}

.match-type-badge {
  font-size: 10px;
  opacity: 0.7;
  margin-left: 6px;
}

.mt-mime { color: #3880ff; }
.mt-extension { color: #2dd36f; }
.mt-general { color: #ffc409; }
.mt-container { color: #eb445a; }

/* 密码策略提示 */
.plugin-hint {
  padding: 8px 14px;
  font-size: 12px;
  color: var(--ion-color-medium);
  background: rgba(var(--ion-color-primary-rgb), 0.06);
  border-radius: 8px;
  margin: 8px 0;
  border-left: 3px solid var(--ion-color-medium);
}

.plugin-hint.strategy-independent {
  border-left-color: var(--ion-color-primary);
  color: var(--ion-color-primary);
  font-weight: 500;
}

/* 版本选择 */
.version-section {
  margin: 10px 0;
}

/* 密码字段 */
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
  --track-background: #424242;
  --track-background-checked: #3880ff;
  --handle-background: #3880ff;
  --handle-background-checked: #ffffff;
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

/* 提交按钮 */
.submit-btn {
  margin-top: 20px;
  --border-radius: 10px;
  height: 48px;
  font-weight: 600;
  letter-spacing: 0.3px;
}

.submit-btn:disabled {
  opacity: 0.5;
}
</style>
