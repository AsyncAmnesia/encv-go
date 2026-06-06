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
              v-else-if="child.key !== 'openai_model'"
              :field="child"
              :model-value="getValue(['agent_settings', child.key])"
              :label="fieldLabel(child.key, child.required)"
              :placeholder="child.description || fieldLabel(child.key)"
              :icon="getFieldIcon(child.key, child.type)"
              @update:model-value="setValue(['agent_settings', child.key], $event)"
              @input="handleInput(['agent_settings', child.key], child, $event)"
              @reset="resetFieldToDefault(['agent_settings', child.key], child)"
            />

            <!-- openai_model：动态模型选择器（从供应商 API 获取） -->
            <div v-else-if="child.key === 'openai_model'" class="config-field config-field-card">
              <div class="field-label-row">
                <ion-icon :icon="sparklesOutline" class="field-icon"></ion-icon>
                <span class="field-label-text">{{ fieldLabel(child.key, child.required) }}</span>
                <ion-icon :icon="cloudOutline" class="sync-indicator" :title="t('settings.synced')"></ion-icon>
              </div>
              <p v-if="child.description" class="field-description-text">{{ child.description }}</p>

              <!-- 加载中 -->
              <div v-if="settingsModelsLoading" class="model-loading">
                <ion-spinner name="crescent" class="model-spinner"></ion-spinner>
                <span>{{ t('agent.loadingModels') }}...</span>
              </div>

              <!-- 加载失败：显示错误 + 手动输入回退 -->
              <div v-else-if="settingsModelsError" class="model-error-state">
                <p class="model-error-text">{{ settingsModelsError }}</p>
                <input
                  type="text"
                  class="model-fallback-input"
                  :value="String(getValue(['agent_settings', 'openai_model']) ?? '')"
                  :placeholder="t('agent.modelFallbackPlaceholder') || '输入模型名称'"
                  @input="handleModelManualInput($event)"
                />
              </div>

              <!-- 正常：动态 preset-cards -->
              <div v-else-if="settingsModelOptions.length > 0" class="preset-cards">
                <div
                  v-for="opt in settingsModelOptions"
                  :key="opt.id"
                  class="preset-card"
                  :class="{ 'preset-card-active': String(getValue(['agent_settings', 'openai_model'])) === opt.id }"
                  @click="setValue(['agent_settings', 'openai_model'], opt.id)"
                >
                  <div class="preset-card-title">{{ opt.name || opt.id }}</div>
                  <div v-if="opt.provider && opt.provider !== 'unknown'" class="preset-card-desc">{{ opt.provider }}</div>
                </div>
              </div>

              <!-- 空列表 -->
              <div v-else class="model-empty">
                <p>{{ t('agent.noModelsAvailable') || '无可用模型' }}</p>
                <input
                  type="text"
                  class="model-fallback-input"
                  :value="String(getValue(['agent_settings', 'openai_model']) ?? '')"
                  :placeholder="t('agent.modelFallbackPlaceholder') || '输入模型名称'"
                  @input="handleModelManualInput($event)"
                />
              </div>
            </div>
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

// ─── 动态模型选择（openai_model 字段） ──────────────────────
interface SettingsModelOption { id: string; name: string; provider: string }
const settingsModelOptions = ref<SettingsModelOption[]>([])
const settingsModelsLoading = ref(true)
const settingsModelsError = ref('')

async function fetchSettingsModels() {
  settingsModelsLoading.value = true
  settingsModelsError.value = ''
  try {
    const res = await fetch('/agent-api/api/models')
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const data = await res.json()
    if (data.error && data.error === 'no_api_key') {
      settingsModelsError.value = t('agent.noApiKeyHint') || '未配置 API Key，请先填写上方 API Key 后保存'
    } else if (data.error) {
      settingsModelsError.value = data.note || data.error
    } else {
      settingsModelOptions.value = (data.models || []).map((m: any) => ({
        id: m.id, name: m.name || m.id, provider: m.provider || 'unknown',
      }))
    }
  } catch (e: any) {
    console.error('[AgentSettings] fetchModels failed:', e)
    settingsModelsError.value = e?.message || String(e)
  } finally {
    settingsModelsLoading.value = false
  }
}

function handleModelManualInput(event: Event) {
  const val = (event.target as HTMLInputElement).value
  setValue(['agent_settings', 'openai_model'], val)
}

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
    // 动态获取模型列表（不阻塞页面渲染）
    fetchSettingsModels()
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

/* ── 动态模型选择器 ─────────────────────────────────────── */
.model-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 0;
  color: var(--ion-color-medium);
  font-size: 13px;
}
.model-spinner {
  width: 18px;
  height: 18px;
}
.model-error-state,
.model-empty {
  padding: 8px 0;
}
.model-error-text {
  color: var(--ion-color-danger, #eb445a);
  font-size: 12px;
  margin: 0 0 6px;
}
.model-fallback-input {
  width: 100%;
  padding: 8px 10px;
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.3);
  border-radius: 8px;
  background: var(--ion-background-color);
  color: var(--ion-text-color);
  font-size: 13px;
  font-family: inherit;
  outline: none;
  box-sizing: border-box;
}
.model-fallback-input:focus {
  border-color: var(--ion-color-primary);
}
.model-empty p {
  color: var(--ion-color-medium);
  font-size: 12px;
  margin: 0 0 6px;
}

/* 动态 preset-cards（与 ConfigFieldItem 保持一致） */
.preset-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(80px, 1fr));
  gap: 8px;
  margin-top: 10px;
  width: 100%;
}
.preset-card {
  padding: 10px 8px;
  border: 2px solid var(--ion-color-light-shade, #e0e0e0);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s;
  text-align: center;
  background: var(--ion-background-color, transparent);
}
.preset-card-active {
  border-color: var(--ion-color-primary);
  background: rgba(var(--ion-color-primary-rgb), 0.08);
}
.preset-card-title {
  font-weight: 600;
  font-size: 13px;
}
.preset-card-desc {
  font-size: 11px;
  color: var(--ion-color-medium);
  margin-top: 3px;
  line-height: 1.3;
}
</style>
