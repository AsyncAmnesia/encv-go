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
      <p style="color:red;font-weight:bold">✅ ion-content OK | taskType={{ val(taskType) }} | sourcePath={{ val(sourcePath) }} | candidates={{ arrLen(candidates) }}</p>
      <ion-list>
        <ion-item>
          <ion-select
            :model-value="val(taskType)"
            @ionChange="(e: Event) => cb('onUpdateTaskType', (e as CustomEvent).detail.value)"
            interface="action-sheet"
            :label="t('tasks.taskType')"
            label-placement="stacked"
          >
            <ion-select-option value="encrypt">{{ t('tasks.encrypt') }}</ion-select-option>
            <ion-select-option value="decrypt">{{ t('tasks.decrypt') }}</ion-select-option>
          </ion-select>
        </ion-item>
        <ion-item>
          <ion-input
            :model-value="val(sourcePath)"
            @ionInput="(e: Event) => cb('onUpdateSourcePath', (e as CustomEvent).detail.value)"
            :label="t('tasks.sourcePath')"
            label-placement="stacked"
            placeholder="/path/to/file"
            :error-text="val(sourcePathError)"
            :class="{ 'ion-invalid': !!val(sourcePathError), 'ion-touched': !!val(sourcePathError) }"
          ></ion-input>
          <ion-button slot="end" fill="clear" class="browse-btn" @click="cb('onBrowseSource')">
            <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
          </ion-button>
        </ion-item>
        <ion-item>
          <ion-input
            :model-value="val(targetPath)"
            @ionInput="(e: Event) => cb('onUpdateTargetPath', (e as CustomEvent).detail.value)"
            :label="t('tasks.targetPath')"
            label-placement="stacked"
            :placeholder="t('tasks.targetPathPlaceholder')"
            :error-text="val(targetPathError)"
            :class="{ 'ion-invalid': !!val(targetPathError), 'ion-touched': !!val(targetPathError) }"
          ></ion-input>
          <ion-button slot="end" fill="clear" class="browse-btn" @click="cb('onBrowseTarget')">
            <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
          </ion-button>
        </ion-item>
      </ion-list>

      <!-- 场景 A: 单一候选 -->
      <ion-note v-if="arrLen(candidates) === 1 && predictedPlugin && !val(sourcePathError)"
               class="plugin-hint">
        <ion-icon :icon="informationCircle"></ion-icon>
        {{ t('tasks.willBeHandledBy', { plugin: predictedPlugin ?? '' }) }}
        <span v-if="taskOptions" class="password-strategy-hint">
          <template v-if="optsVal(taskOptions, 'passwordStrategy') === 'global' || optsVal(taskOptions, 'passwordStrategy') !== 'independent'">
            · {{ t('tasks.usesGlobalPassword') }}
          </template>
          <template v-else-if="optsVal(taskOptions, 'passwordStrategy') === 'independent'">
            · {{ t('tasks.usesIndependentPassword') }}
          </template>
        </span>
      </ion-note>

      <!-- 场景 B: 多候选 -->
      <div v-else-if="arrLen(candidates) > 1 && !val(sourcePathError)" class="plugin-selector">
        <ion-item>
          <ion-select
            :value="numVal(selectedPluginIndex)"
            @ionChange="(e: Event) => cb('onUpdateSelectedIndex', (e as CustomEvent).detail.value)"
            label-placement="stacked"
            :label="t('tasks.selectPlugin')"
            interface="action-sheet"
          >
            <ion-select-option
              v-for="(cand, idx) in arr(candidates)"
              :key="cand.name"
              :value="idx"
            >
              {{ formatPluginLabel(cand) }}
            </ion-select-option>
          </ion-select>
        </ion-item>
        <ion-note class="plugin-hint">
          <ion-icon :icon="informationCircle"></ion-icon>
          {{ t('tasks.willBeHandledBy', { plugin: predictedPlugin ?? '' }) }}
          <span v-if="taskOptions" class="password-strategy-hint">
            <template v-if="optsVal(taskOptions, 'passwordStrategy') === 'global' || optsVal(taskOptions, 'passwordStrategy') !== 'independent'">
              · {{ t('tasks.usesGlobalPassword') }}
            </template>
            <template v-else-if="optsVal(taskOptions, 'passwordStrategy') === 'independent'">
              · {{ t('tasks.usesIndependentPassword') }}
            </template>
          </span>
        </ion-note>
      </div>

      <!-- 容器版本选择 -->
      <ion-item v-if="optsBool(taskOptions, 'supportVersionSelect') && val(taskType) === 'encrypt'">
        <ContainerVersionSelector :model-value="numVal(version)" @update:model-value="(v: number) => cb('onUpdateVersion', v)" :versions="arr(versionOptions)" />
      </ion-item>

      <!-- 插件声明的额外字段 -->
      <template v-for="field in filteredExtraFields" :key="field.key">
        <ion-item v-if="!field.condition || field.condition === val(taskType)">
          <ion-input
            :model-value="objVal(extraValues, field.key)"
            @ionInput="(e: Event) => cb('onUpdateExtraValue', { key: field.key, value: (e as CustomEvent).detail.value })"
            :label="t(field.label)"
            :type="field.type as 'text' | 'password' | 'email' | 'number' | 'tel' | 'url'"
            :placeholder="t(field.help)"
          ></ion-input>
        </ion-item>
      </template>

      <!-- 密码覆盖（Global 插件）-->
      <ion-item v-if="!taskOptions || optsVal(taskOptions, 'passwordStrategy') === 'global' || optsVal(taskOptions, 'passwordStrategy') !== 'independent'">
        <ion-input
          :model-value="val(primaryOverride)"
          @ionInput="(e: Event) => cb('onUpdatePrimaryOverride', (e as CustomEvent).detail.value)"
          :label="t('tasks.passwordOverride')"
          label-placement="stacked"
          type="password"
          :placeholder="t('tasks.passwordOverrideHelp')"
        ></ion-input>
        <ion-badge color="medium" slot="end">{{ t('tasks.optional') }}</ion-badge>
      </ion-item>

      <!-- 二级密码 -->
      <ion-item>
        <ion-input
          :model-value="val(secondaryPassword)"
          @ionInput="(e: Event) => cb('onUpdateSecondaryPassword', (e as CustomEvent).detail.value)"
          :label="t('tasks.secondaryPassword')"
          label-placement="stacked"
          type="password"
          :placeholder="t('tasks.secondaryPasswordHelp')"
        ></ion-input>
        <ion-badge color="medium" slot="end">{{ t('tasks.optional') }}</ion-badge>
      </ion-item>

      <ion-button expand="block" @click="cb('onSubmit')" :disabled="!val(sourcePath) || !!val(sourcePathError) || !!val(targetPathError)">
        <ion-icon :icon="lockClosed" slot="start"></ion-icon>
        {{ t('tasks.createTask') }}
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
  IonIcon,
  IonList,
  IonItem,
  IonSelect,
  IonSelectOption,
  IonInput,
  IonNote,
  IonBadge,
  modalController,
} from '@ionic/vue'
import { folderOpen, lockClosed, informationCircle } from 'ionicons/icons'
import ContainerVersionSelector from '@/components/ContainerVersionSelector.vue'
import { useI18n } from '@/composables/useI18n'
import type { PluginCandidate, TaskField, TaskOptions } from '@/api/encv'
import type { Ref } from 'vue'
import { isRef } from 'vue'

const { t } = useI18n()

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
  filteredExtraFields: TaskField[]
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
function objVal(obj: Record<string, string> | Ref<Record<string, string> | null>, key: string): string {
  const raw = isRef(obj) ? obj.value : obj
  if (!raw) return ''
  return raw[key] ?? ''
}
function optsVal(opts: TaskOptions | null | Ref<TaskOptions | null>, key: keyof TaskOptions): TaskOptions[keyof TaskOptions] | undefined {
  if (!opts) return undefined
  const raw = isRef(opts) ? opts.value : opts
  if (!raw) return undefined
  return raw[key]
}
function optsBool(opts: TaskOptions | null | Ref<TaskOptions | null>, key: keyof TaskOptions): boolean {
  return !!optsVal(opts, key)
}

function cb(name: string, ...args: any[]) {
  const fn = (props as Record<string, unknown>)[name] as Function | undefined
  if (fn) fn(...args)
}

function formatPluginLabel(cand: { name: string; matchType: string }): string {
  const nameMap: Record<string, string> = {
    video: 'Video 插件',
    text: 'Text 插件',
    audio: 'Audio 插件',
    image: 'Image 插件',
    pdf: 'PDF 插件',
    wps: 'WPS 插件',
    alist_encrypt: 'Alist-Encrypt',
  }
  const baseName = nameMap[cand.name] ?? cand.name
  return cand.matchType === 'general' ? `${baseName}（通用）` : baseName
}

async function handleClose() {
  await modalController.dismiss()
}
</script>

<style scoped>
.plugin-selector {
  margin-bottom: 8px;
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
