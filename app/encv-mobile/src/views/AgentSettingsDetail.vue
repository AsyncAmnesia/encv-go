<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button default-href="/tabs/settings"></ion-back-button>
        </ion-buttons>
        <ion-title>{{ t('settings.agentSettings') }}</ion-title>
        <ion-buttons slot="end" v-if="dirty">
          <ion-button @click="handleResetConfig" color="medium">{{ t('settings.undo') }}</ion-button>
          <ion-button @click="handleSaveConfig" :disabled="configLoading">
            <ion-icon :icon="saveIcon" slot="icon-only"></ion-icon>
          </ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <div v-if="configLoading && !configLoaded" class="loading-container">
        <ion-spinner name="crescent"></ion-spinner>
        <p>{{ t('settings.loadingConfig') }}</p>
      </div>

      <template v-else-if="configLoaded && agentSection">
        <ion-list>
          <ion-list-header>
            <ion-label>{{ t('settings.agentSettings') }}</ion-label>
            <ion-badge slot="end" color="primary" class="scope-badge scope-synced">
              <ion-icon :icon="cloudOutline" class="scope-badge-icon"></ion-icon>
              <span class="scope-text">{{ t('settings.synced') }}</span>
            </ion-badge>
          </ion-list-header>

          <template v-for="child in agentSection.properties" :key="child.key">
            <template v-if="child.key === 'enabled_tools'">
              <ion-item lines="none" class="config-field">
                <ion-icon :icon="listIcon" slot="start"></ion-icon>
                <ion-input
                  :value="toolsText"
                  :label="fieldLabel(child.key)"
                  label-placement="stacked"
                  :placeholder="'list_files, read_file, delete_file, exec_command'"
                  @ionInput="handleToolsInput($event)"
                ></ion-input>
              </ion-item>
              <ion-item v-if="toolsChips.length > 0" lines="none" class="tools-chip-item">
                <ion-label class="ion-text-wrap">
                  <div class="tools-chip-row">
                    <ion-chip
                      v-for="tool in toolsChips"
                      :key="tool"
                      @click="removeTool(tool)"
                      class="tool-chip"
                    >
                      {{ tool }}
                      <ion-icon :icon="closeIcon"></ion-icon>
                    </ion-chip>
                  </div>
                </ion-label>
              </ion-item>
              <ion-item v-if="child.description" lines="none">
                <ion-note class="field-description">
                  {{ child.description }}
                </ion-note>
              </ion-item>
            </template>

            <ConfigFieldItem
              v-else
              :field="child"
              :model-value="getValue(['agent_settings', child.key])"
              :label="fieldLabel(child.key, child.required)"
              :placeholder="child.description || fieldLabel(child.key)"
              :icon="getFieldIcon(child.key, child.type)"
              @update:model-value="setValue(['agent_settings', child.key], $event)"
              @input="handleInput(['agent_settings', child.key], child, $event)"
              @reset="resetFieldToDefault(['agent_settings', child.key], child)"
            />
          </template>
        </ion-list>

        <ion-list>
          <ion-item button :disabled="testing" @click="handleTestConnection">
            <ion-icon :icon="flashIcon" slot="start"></ion-icon>
            <ion-label>
              <h3>{{ t('settings.testConnection') }}</h3>
              <p v-if="testResult" :class="testResultClass">
                {{ testResult }}
              </p>
            </ion-label>
            <ion-spinner v-if="testing" slot="end" name="crescent"></ion-spinner>
          </ion-item>
        </ion-list>
      </template>

      <ion-list v-if="configLoaded && agentSection">
        <ion-item button @click="openJsonEditor">
          <ion-icon :icon="documentText" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.editRawConfig') }}</h3>
          </ion-label>
        </ion-item>
      </ion-list>

      <ion-modal :is-open="showJsonEditor" @didDismiss="showJsonEditor = false">
        <ion-header>
          <ion-toolbar>
            <ion-title>{{ t('settings.editRawConfig') }}</ion-title>
            <ion-buttons slot="end">
              <ion-button @click="showJsonEditor = false">{{ t('settings.cancel') }}</ion-button>
              <ion-button @click="handleSaveJson" :disabled="!!jsonError" color="primary">{{ t('settings.saveConfig') }}</ion-button>
            </ion-buttons>
          </ion-toolbar>
        </ion-header>
        <ion-content class="json-editor-content">
          <textarea
            v-model="jsonText"
            class="json-textarea"
            spellcheck="false"
            @input="validateJson"
          ></textarea>
          <div v-if="jsonError" class="json-error">
            {{ t('settings.jsonError') }}: {{ jsonError }}
          </div>
        </ion-content>
      </ion-modal>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons, IonButton,
  IonBackButton, IonContent, IonList, IonListHeader, IonItem,
  IonIcon, IonLabel, IonInput, IonSpinner, IonModal,
  IonNote, IonChip,
} from '@ionic/vue'
import {
  save as saveIcon, sparklesOutline, cloudOutline, settingsOutline,
  documentText, lockClosed, speedometerOutline, key, globeOutline,
  optionsOutline, listOutline, flashOutline, closeCircleOutline,
} from 'ionicons/icons'
import { useConfig } from '@/composables/useConfig'
import { useServerStatus } from '@/composables/useServerStatus'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import { fetchConfig, updateConfig } from '@/api/encv'
import type { FieldDef } from '@/config/schemaParser'
import ConfigFieldItem from '@/components/ConfigFieldItem.vue'

const { isOnline: serverOnline } = useServerStatus()
const { schemaFields, loading: configLoading, dirty, loadConfig, saveConfig, resetConfig, getFieldValue, setFieldValue, resetFieldToDefault } = useConfig()
const { t, tField } = useI18n()

const configLoaded = ref(false)
const testing = ref(false)
const testResult = ref('')
const testResultSuccess = ref(false)

const listIcon = listOutline
const flashIcon = flashOutline
const closeIcon = closeCircleOutline

const showJsonEditor = ref(false)
const jsonText = ref('')
const jsonError = ref('')

const agentSection = computed<FieldDef | undefined>(() => {
  return schemaFields.value.find(s => s.key === 'agent_settings')
})

const toolsChips = computed<string[]>(() => {
  const raw = getFieldValue(['agent_settings', 'enabled_tools'])
  if (Array.isArray(raw)) return raw.filter((s): s is string => typeof s === 'string')
  return []
})

const toolsText = computed(() => toolsChips.value.join(', '))

const testResultClass = computed(() => testResultSuccess.value ? 'test-result-success' : 'test-result-failed')

function getValue(path: string[]): unknown {
  return getFieldValue(path)
}

function setValue(path: string[], value: unknown) {
  setFieldValue(path, value)
}

function handleInput(path: string[], field: FieldDef, event: CustomEvent) {
  const val = (event.target as HTMLInputElement).value
  if (field.type === 'integer') {
    setFieldValue(path, val ? Number(val) : 0)
  } else {
    setFieldValue(path, val)
  }
}

function handleToolsInput(event: CustomEvent) {
  const raw = (event.target as HTMLInputElement).value || ''
  const list = raw.split(',').map(s => s.trim()).filter(s => s.length > 0)
  setFieldValue(['agent_settings', 'enabled_tools'], list)
}

function removeTool(name: string) {
  const next = toolsChips.value.filter(t => t !== name)
  setFieldValue(['agent_settings', 'enabled_tools'], next)
}

function fieldLabel(key: string, _required?: boolean): string {
  return tField(key)
}

const fieldIconMap: Record<string, string> = {
  openai_api_key: key,
  openai_base_url: globeOutline,
  openai_model: sparklesOutline,
  openlist_base_url: globeOutline,
  openlist_token: lockClosed,
  default_container_version: speedometerOutline,
  enabled_tools: optionsOutline,
  system_prompt: documentText,
  max_tool_calls_per_turn: speedometerOutline,
}

function getFieldIcon(fieldKey: string, fieldType: string): string {
  if (fieldIconMap[fieldKey]) return fieldIconMap[fieldKey]
  if (fieldType === 'boolean') return settingsOutline
  if (fieldType === 'integer') return speedometerOutline
  if (fieldKey.includes('password')) return lockClosed
  return settingsOutline
}

async function handleSaveConfig() {
  try {
    await saveConfig()
    showToast({ message: t('settings.configSaved'), duration: 1500, color: 'success' })
  } catch (e) {
    const detail = e instanceof Error ? e.message : String(e)
    showToast({ message: t('settings.configSaveFailed') + ': ' + detail, duration: 3000, color: 'danger' })
  }
}

function handleResetConfig() {
  resetConfig()
}

async function handleTestConnection() {
  testing.value = true
  testResult.value = ''
  testResultSuccess.value = false
  try {
    // 走 /agent-api/* 命名空间转发到 agent 后端 :5245
    // 不走 encv-go 的 /api/agent/test（encv-go 当前没这端点，会 404）
    const response = await fetch('/agent-api/test', {
      method: 'GET',
      headers: { 'Content-Type': 'application/json' },
    })
    if (!response.ok) {
      let detail = `HTTP ${response.status}`
      try {
        const body = await response.text()
        if (body) detail += `: ${body}`
      } catch {}
      throw new Error(detail)
    }
    const data = await response.json().catch(() => ({}))
    const openaiOk = data.openai === true || data.openai === 'ok'
    const openlistOk = data.openlist === true || data.openlist === 'ok'
    if (openaiOk && openlistOk) {
      testResultSuccess.value = true
      testResult.value = t('settings.testConnectionSuccess')
      showToast({ message: t('settings.testConnectionSuccess'), duration: 2000, color: 'success' })
    } else {
      const failed: string[] = []
      if (!openaiOk) failed.push('OpenAI')
      if (!openlistOk) failed.push('OpenList')
      const detail = failed.join(', ') + (data.detail ? `: ${data.detail}` : '')
      testResultSuccess.value = false
      testResult.value = t('settings.testConnectionFailed', { detail })
      showToast({ message: testResult.value, duration: 3000, color: 'danger' })
    }
  } catch (e) {
    const detail = e instanceof Error ? e.message : String(e)
    testResultSuccess.value = false
    testResult.value = t('settings.testConnectionFailed', { detail })
    showToast({ message: testResult.value, duration: 3000, color: 'danger' })
  } finally {
    testing.value = false
  }
}

function openJsonEditor() {
  const agentVal = getFieldValue(['agent_settings'])
  jsonText.value = JSON.stringify(agentVal ?? {}, null, 2)
  jsonError.value = ''
  showJsonEditor.value = true
}

function validateJson() {
  try {
    JSON.parse(jsonText.value)
    jsonError.value = ''
  } catch (e) {
    jsonError.value = e instanceof Error ? e.message : String(e)
  }
}

async function handleSaveJson() {
  try {
    const parsed = JSON.parse(jsonText.value)
    const cfg = await fetchConfig()
    ;(cfg as Record<string, unknown>).agent_settings = parsed
    await updateConfig(cfg)
    showJsonEditor.value = false
    showToast({ message: t('settings.configSaved'), duration: 1500, color: 'success' })
    await loadConfig()
  } catch (e) {
    const detail = e instanceof Error ? e.message : String(e)
    showToast({ message: t('settings.configSaveFailed') + ': ' + detail, duration: 3000, color: 'danger' })
  }
}

onMounted(async () => {
  if (serverOnline.value) {
    await loadConfig()
    configLoaded.value = true
  }
})
</script>

<style scoped>
.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 24px;
  color: var(--encv-text-secondary);
}

.scope-badge {
  font-size: 10px;
  --padding-start: 6px;
  --padding-end: 8px;
  --padding-top: 3px;
  --padding-bottom: 3px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  gap: 3px;
}
.scope-badge-icon {
  font-size: 12px;
}
.scope-synced {
  --background: rgba(var(--ion-color-primary-rgb), 0.12);
  --color: var(--ion-color-primary);
}
@media (max-width: 599px) {
  .scope-badge {
    --padding-start: 5px;
    --padding-end: 5px;
    --padding-top: 2px;
    --padding-bottom: 2px;
  }
  .scope-badge .scope-text {
    display: none;
  }
}

.tools-chip-item {
  --min-height: 0;
  --padding-start: 16px;
}
.tools-chip-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.tool-chip {
  --background: rgba(var(--ion-color-primary-rgb), 0.12);
  --color: var(--ion-color-primary);
  font-size: 12px;
  cursor: pointer;
}
.tool-chip ion-icon {
  margin-left: 4px;
  font-size: 14px;
  opacity: 0.7;
}

.field-description {
  font-size: 12px;
  color: var(--ion-color-medium);
  white-space: normal;
  line-height: 1.4;
}

.test-result-success {
  color: var(--ion-color-success);
  font-size: 12px;
}
.test-result-failed {
  color: var(--ion-color-danger);
  font-size: 12px;
}

.json-editor-content {
  --background: var(--ion-background-color);
}
.json-textarea {
  width: 100%;
  min-height: 400px;
  padding: 12px 16px;
  font-family: 'Courier New', Courier, monospace;
  font-size: 13px;
  line-height: 1.5;
  background: var(--ion-background-color);
  color: var(--ion-text-color);
  border: none;
  outline: none;
  resize: none;
  box-sizing: border-box;
}
.json-error {
  padding: 8px 16px;
  background: rgba(var(--ion-color-danger-rgb), 0.1);
  color: var(--ion-color-danger);
  font-size: 12px;
  font-family: monospace;
}
</style>
