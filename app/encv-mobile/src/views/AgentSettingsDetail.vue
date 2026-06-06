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

            <!-- openai_api_key：使用通用密码输入框（InputWithHistory），保存时自动加密 -->
            <InputWithHistory
              v-else-if="child.key === 'openai_api_key'"
              :model-value="apiKeyPlainValue"
              :label="fieldLabel(child.key, child.required)"
              :placeholder="t('agent.apiKeyPlaceholder') || 'sk-...'"
              :icon="key"
              input-type="password"
              :history-key="'config.agent_api_key'"
              :is-customized="isApiKeyCustomized"
              @update:model-value="handleApiKeyInput($event)"
            />

            <!-- API Key 状态徽标 + 后端 base + 测试按钮（紧跟 input 显示） -->
            <ion-item v-if="child.key === 'openai_api_key'" lines="none" class="apiKeyStatusItem">
              <ion-icon :icon="bugIcon" slot="start" class="apiKeyStatusIcon"></ion-icon>
              <ion-label class="ion-text-wrap">
                <div class="apiKeyStatusRow">
                  <ion-badge :color="apiKeyStatusBadge.color" class="apiKeyStatusBadge">
                    <ion-icon
                      v-if="apiKeyStatusBadge.spinning"
                      :icon="apiKeyStatusBadge.icon"
                      class="apiKeyStatusBadgeIcon"
                    ></ion-icon>
                    <ion-icon
                      v-else
                      :icon="apiKeyStatusBadge.icon"
                      class="apiKeyStatusBadgeIcon"
                    ></ion-icon>
                    <span class="apiKeyStatusBadgeText">{{ apiKeyStatusBadge.label }}</span>
                  </ion-badge>
                  <ion-spinner v-if="roundtripRunning" name="crescent" class="apiKeySpinner"></ion-spinner>
                </div>
                <p v-if="apiKeyStatusDetail" class="apiKeyStatusDetail">{{ apiKeyStatusDetail }}</p>
                <p class="apiKeyBackendLine">
                  <span class="apiKeyBackendLabel">{{ t('agent.apiKeyBackendLabel') }}:</span>
                  <code class="apiKeyBackendBase">{{ agentApiBaseCtx.base }}</code>
                  <span class="apiKeyBackendSource">({{ agentApiBaseLabel }})</span>
                </p>
              </ion-label>
              <ion-button
                slot="end"
                size="small"
                fill="outline"
                :disabled="roundtripRunning"
                @click="handleRoundtripTest"
              >
                <ion-icon :icon="refreshIcon" slot="start"></ion-icon>
                {{ t('agent.apiKeyActionRoundtrip') }}
              </ion-button>
            </ion-item>
            <ion-item v-if="child.key === 'openai_api_key' && apiKeyStatusDetail" lines="none">
              <ion-button slot="end" size="small" fill="clear" color="medium" @click="goToDevLogs">
                <ion-icon :icon="bugIcon" slot="start"></ion-icon>
                {{ t('agent.apiKeyViewLogs') }}
              </ion-button>
            </ion-item>

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

        <!-- Task 25: Sync Doctor — 脱敏诊断报告 -->
        <ion-list>
          <ion-item button :disabled="doctorRunning" @click="handleRunSyncDoctor">
            <ion-icon :icon="medkitIcon" slot="start"></ion-icon>
            <ion-label>
              <h3>{{ t('agent.syncDoctor') }}</h3>
              <p>{{ doctorIssuesCount > 0 ? t('agent.syncDoctorResult') + ' (' + doctorIssuesCount + ')' : t('agent.syncDoctorEmpty') }}</p>
            </ion-label>
            <ion-spinner v-if="doctorRunning" slot="end" name="crescent"></ion-spinner>
          </ion-item>
          <ion-item v-if="doctorReportJson" lines="none" class="doctor-result-item">
            <div class="doctor-result-wrap">
              <pre class="doctor-result-pre">{{ doctorReportJson }}</pre>
              <div class="doctor-result-actions">
                <ion-button size="small" fill="outline" @click="handleCopyDoctorJson">
                  <ion-icon :icon="copyIcon" slot="start"></ion-icon>
                  {{ t('agent.syncDoctorCopy') }}
                </ion-button>
              </div>
            </div>
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
  IonIcon, IonLabel, IonInput, IonSpinner, IonModal, IonNote, IonChip,
  IonBadge,
} from '@ionic/vue'
import {
  save as saveIcon, sparklesOutline, cloudOutline, settingsOutline,
  documentText, lockClosed, speedometerOutline, key, globeOutline,
  optionsOutline, listOutline, flashOutline, closeCircleOutline,
  checkmarkCircle, lockOpenOutline, alertCircleOutline,
  refreshOutline, bugOutline, medkitOutline, copyOutline,
} from 'ionicons/icons'
import { useConfig } from '@/composables/useConfig'
import { useServerStatus } from '@/composables/useServerStatus'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import { getDeviceId } from '@/composables/useDeviceId'
import { fetchConfig, updateConfig } from '@/api/encv'
import { getAgentApiBase, getAgentApiBaseContext } from '@/composables/useAgentApiBase'
import { devlogApiError, devlogApiInfo } from '@/composables/devlogApiError'
import { runSyncDoctor, type DoctorReport } from '@/composables/useAgent'
import type { FieldDef } from '@/config/schemaParser'
import ConfigFieldItem from '@/components/ConfigFieldItem.vue'
import InputWithHistory from '@/components/InputWithHistory.vue'
import { useRouter } from 'vue-router'

const { isOnline: serverOnline } = useServerStatus()
const { schemaFields, loading: configLoading, dirty, loadConfig, saveConfig, resetConfig, getFieldValue, setFieldValue, resetFieldToDefault } = useConfig()
const { t, tField } = useI18n()
const router = useRouter()

const configLoaded = ref(false)
const testing = ref(false)
const testResult = ref('')
const testResultSuccess = ref(false)

const listIcon = listOutline
const flashIcon = flashOutline
const closeIcon = closeCircleOutline
const lockOpenIcon = lockOpenOutline
const lockIcon = lockClosed
const checkmarkIcon = checkmarkCircle
const alertIcon = alertCircleOutline
const refreshIcon = refreshOutline
const bugIcon = bugOutline
const medkitIcon = medkitOutline
const copyIcon = copyOutline

// ─── API Key 状态机（spec F.3 状态反馈 UI）────────────────────
type ApiKeyStatus =
  | 'empty'           // 未配置
  | 'plaintext'       // 加载到明文（明文储存格式或刚解密回填）
  | 'encrypted'       // 已加密储存（enc:xxx）
  | 'decrypting'      // 解密中
  | 'encrypting'      // 加密中
  | 'decrypt-failed'  // 解密失败
  | 'encrypt-failed'  // 加密失败
  | 'test-failed'     // 连通性测试失败
  | 'roundtrip-ok'    // 往返测试成功
  | 'roundtrip-mismatch' // 往返测试不一致

const apiKeyStatus = ref<ApiKeyStatus>('empty')
const apiKeyStatusDetail = ref('') // 详细错误信息（用于展开）
const roundtripRunning = ref(false)

const apiKeyPlainValue = ref('') // 用户正在编辑的明文（内存中，password input 自动掩码）

// 是否已自定义（非默认空值）
const isApiKeyCustomized = computed(() => {
  return apiKeyPlainValue.value.length > 0
})

// 状态徽标展示
const apiKeyStatusBadge = computed(() => {
  switch (apiKeyStatus.value) {
    case 'empty':
      return { color: 'medium' as const, icon: lockOpenIcon, label: t('agent.apiKeyStatusEmpty') }
    case 'plaintext':
      return { color: 'warning' as const, icon: lockOpenIcon, label: t('agent.apiKeyStatusPlaintext') }
    case 'encrypted':
      return { color: 'success' as const, icon: lockIcon, label: t('agent.apiKeyStatusEncrypted') }
    case 'decrypting':
      return { color: 'primary' as const, icon: refreshIcon, label: t('agent.apiKeyStatusDecrypting'), spinning: true }
    case 'encrypting':
      return { color: 'primary' as const, icon: refreshIcon, label: t('agent.apiKeyStatusEncrypting'), spinning: true }
    case 'decrypt-failed':
      return { color: 'danger' as const, icon: alertIcon, label: t('agent.apiKeyStatusDecryptFailed') }
    case 'encrypt-failed':
      return { color: 'danger' as const, icon: alertIcon, label: t('agent.apiKeyStatusEncryptFailed') }
    case 'test-failed':
      return { color: 'danger' as const, icon: alertIcon, label: t('agent.apiKeyStatusTestFailed') }
    case 'roundtrip-ok':
      return { color: 'success' as const, icon: checkmarkIcon, label: t('agent.apiKeyStatusRoundtripOk') }
    case 'roundtrip-mismatch':
      return { color: 'danger' as const, icon: alertIcon, label: t('agent.apiKeyStatusRoundtripMismatch') }
    default:
      return { color: 'medium' as const, icon: lockOpenIcon, label: '' }
  }
})

// Agent API base 当前解析（用于 UI 展示"实际打到哪里"）
const agentApiBaseCtx = computed(() => getAgentApiBaseContext())
const agentApiBaseLabel = computed(() => {
  switch (agentApiBaseCtx.value.source) {
    case 'dev-gateway':       return t('agent.apiKeyBackendDev')
    case 'native-default':    return t('agent.apiKeyBackendNative')
    case 'user-configured':   return t('agent.apiKeyBackendUser')
    case 'web-fallback':      return t('agent.apiKeyBackendFallback')
  }
})

/**
 * 加载配置后自动解密 API Key（如果存储的是加密格式）
 * 解密后的明文存入 apiKeyPlainValue，由 InputWithHistory(password) 的 type="password" 自动掩码显示
 *
 * 状态机驱动：
 *   - 'empty'      → 没存
 *   - 'plaintext'  → 存的是明文（旧数据或 dev 模式手动）
 *   - 'decrypting' → 正在调 /api/decrypt-key
 *   - 'encrypted'  → 解密成功（明文已回填到 input 内存）
 *   - 'decrypt-failed' → /api/decrypt-key 返回非 2xx
 */
async function decryptAndLoadApiKey() {
  const stored = String(getFieldValue(['agent_settings', 'openai_api_key']) ?? '')

  // 1. 决定初始状态（不依赖 network）
  if (!stored) {
    apiKeyPlainValue.value = ''
    apiKeyStatus.value = 'empty'
    apiKeyStatusDetail.value = ''
    return
  }
  if (!stored.startsWith('enc:')) {
    // 明文存储 → 不需要网络，但提示"未加密"以防误以为是加密的
    apiKeyPlainValue.value = stored
    apiKeyStatus.value = 'plaintext'
    apiKeyStatusDetail.value = 'Stored without enc: prefix; encryption skipped on save'
    return
  }

  // 2. encrypted 格式 → 调 /api/decrypt-key
  apiKeyStatus.value = 'decrypting'
  apiKeyStatusDetail.value = ''
  let deviceId = ''
  try {
    deviceId = await getDeviceId()
    const res = await fetch(`${getAgentApiBase()}/api/decrypt-key`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ encrypted: stored, deviceId }),
    })
    if (res.ok) {
      const { decrypted } = await res.json()
      apiKeyPlainValue.value = decrypted || ''
      apiKeyStatus.value = 'encrypted'
      apiKeyStatusDetail.value = ''
      devlogApiInfo(`decrypt-key OK (${(decrypted || '').length} chars)`, { kind: 'decrypt' })
    } else {
      let body = ''
      try { body = await res.text() } catch { /* ignore */ }
      const detail = `HTTP ${res.status}${body ? `: ${body.slice(0, 200)}` : ''}`
      apiKeyPlainValue.value = ''
      apiKeyStatus.value = 'decrypt-failed'
      apiKeyStatusDetail.value = detail
      devlogApiError(new Error(`decrypt-key ${detail}`), {
        kind: 'decrypt',
        endpoint: '/api/decrypt-key',
        status: res.status,
        body,
        deviceId,
      })
    }
  } catch (e) {
    const detail = e instanceof Error ? e.message : String(e)
    apiKeyPlainValue.value = ''
    apiKeyStatus.value = 'decrypt-failed'
    apiKeyStatusDetail.value = detail
    devlogApiError(e, {
      kind: 'decrypt',
      endpoint: '/api/decrypt-key',
      deviceId,
    })
  }
}

function handleApiKeyInput(val: string) {
  apiKeyPlainValue.value = val
  setFieldValue(['agent_settings', 'openai_api_key'], val)
}

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
    // 保存前加密 API Key（防止明文写入 config.user.json）
    const rawKey = String(getFieldValue(['agent_settings', 'openai_api_key']) ?? '')
    if (rawKey && !rawKey.startsWith('enc:')) {
      // 状态：开始加密
      apiKeyStatus.value = 'encrypting'
      apiKeyStatusDetail.value = ''
      let deviceId = ''
      try {
        deviceId = await getDeviceId()
        const encRes = await fetch(`${getAgentApiBase()}/api/encrypt-key`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ key: rawKey, deviceId }),
        })
        if (encRes.ok) {
          const { encrypted } = await encRes.json()
          setFieldValue(['agent_settings', 'openai_api_key'], encrypted)
          apiKeyPlainValue.value = '' // 清除明文缓存
          apiKeyStatus.value = 'encrypted'
          apiKeyStatusDetail.value = ''
          devlogApiInfo(`encrypt-key OK (${encrypted.length} chars)`, { kind: 'encrypt' })
        } else {
          let body = ''
          try { body = await encRes.text() } catch { /* ignore */ }
          const detail = `HTTP ${encRes.status}${body ? `: ${body.slice(0, 200)}` : ''}`
          apiKeyStatus.value = 'encrypt-failed'
          apiKeyStatusDetail.value = detail
          devlogApiError(new Error(`encrypt-key ${detail}`), {
            kind: 'encrypt',
            endpoint: '/api/encrypt-key',
            status: encRes.status,
            body,
            deviceId,
          })
          // 不抛：让用户至少能把明文存下来（降级路径），但状态徽标已变红
        }
      } catch (e) {
        const detail = e instanceof Error ? e.message : String(e)
        apiKeyStatus.value = 'encrypt-failed'
        apiKeyStatusDetail.value = detail
        devlogApiError(e, {
          kind: 'encrypt',
          endpoint: '/api/encrypt-key',
          deviceId,
        })
      }
    }
    await saveConfig()
    showToast({ message: t('settings.configSaved'), duration: 1500, color: 'success' })
  } catch (e) {
    const detail = e instanceof Error ? e.message : String(e)
    showToast({ message: t('settings.configSaveFailed') + ': ' + detail, duration: 3000, color: 'danger' })
  }
}

/**
 * 往返测试：明文 → /encrypt-key → /decrypt-key → 比对原文
 *
 * 用于诊断"加密看似成功但解密时数据丢失 / 哈希被改" 等问题。
 * 与 handleSaveConfig 完全独立：不修改任何持久化数据。
 */
async function handleRoundtripTest() {
  const rawKey = apiKeyPlainValue.value || String(getFieldValue(['agent_settings', 'openai_api_key']) ?? '').replace(/^enc:/, '')
  if (!rawKey) {
    showToast({ message: t('agent.apiKeyStatusEmpty'), duration: 1500, color: 'warning' })
    return
  }
  roundtripRunning.value = true
  apiKeyStatus.value = 'encrypting'
  apiKeyStatusDetail.value = ''

  let deviceId = ''
  try {
    deviceId = await getDeviceId()
    // 1. encrypt
    const encRes = await fetch(`${getAgentApiBase()}/api/encrypt-key`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ key: rawKey, deviceId }),
    })
    if (!encRes.ok) {
      let body = ''
      try { body = await encRes.text() } catch { /* ignore */ }
      const detail = `encrypt HTTP ${encRes.status}${body ? `: ${body.slice(0, 200)}` : ''}`
      apiKeyStatus.value = 'encrypt-failed'
      apiKeyStatusDetail.value = detail
      devlogApiError(new Error(detail), {
        kind: 'roundtrip',
        endpoint: '/api/encrypt-key',
        status: encRes.status,
        body,
        deviceId,
      })
      return
    }
    const { encrypted } = await encRes.json()
    devlogApiInfo(`roundtrip encrypt → ${encrypted.length} chars`, { kind: 'roundtrip' })

    // 2. decrypt
    apiKeyStatus.value = 'decrypting'
    const decRes = await fetch(`${getAgentApiBase()}/api/decrypt-key`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ encrypted, deviceId }),
    })
    if (!decRes.ok) {
      let body = ''
      try { body = await decRes.text() } catch { /* ignore */ }
      const detail = `decrypt HTTP ${decRes.status}${body ? `: ${body.slice(0, 200)}` : ''}`
      apiKeyStatus.value = 'decrypt-failed'
      apiKeyStatusDetail.value = detail
      devlogApiError(new Error(detail), {
        kind: 'roundtrip',
        endpoint: '/api/decrypt-key',
        status: decRes.status,
        body,
        deviceId,
      })
      return
    }
    const { decrypted } = await decRes.json()

    // 3. 比对
    if (decrypted === rawKey) {
      apiKeyStatus.value = 'roundtrip-ok'
      apiKeyStatusDetail.value = `${rawKey.length} chars match`
      devlogApiInfo('roundtrip OK', { kind: 'roundtrip' })
    } else {
      apiKeyStatus.value = 'roundtrip-mismatch'
      apiKeyStatusDetail.value = `original=${rawKey.length} chars, decrypted=${(decrypted || '').length} chars`
      devlogApiError(new Error('roundtrip mismatch'), {
        kind: 'roundtrip',
        endpoint: '/api/decrypt-key',
        deviceId,
        extra: { originalLen: rawKey.length, decryptedLen: (decrypted || '').length },
      })
    }
  } catch (e) {
    const detail = e instanceof Error ? e.message : String(e)
    apiKeyStatus.value = 'test-failed'
    apiKeyStatusDetail.value = detail
    devlogApiError(e, { kind: 'roundtrip', endpoint: '/api/encrypt-key', deviceId })
  } finally {
    roundtripRunning.value = false
  }
}

function goToDevLogs() {
  router.push('/tabs/devlogs')
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

// ─── Task 25: Sync Doctor ────────────────────────────────
// 后端 /api/sync/doctor 返回的 DoctorReport 在前端只读展示
// （用于 bug 报告 / 自检）。不修改任何持久化数据。
const doctorRunning = ref(false)
const doctorReportJson = ref('')
const doctorIssuesCount = ref(0)

async function handleRunSyncDoctor() {
  if (doctorRunning.value) return
  doctorRunning.value = true
  try {
    const report: DoctorReport = await runSyncDoctor()
    // 2 空格缩进，方便用户 / 客服直接复制到 issue
    doctorReportJson.value = JSON.stringify(report, null, 2)
    doctorIssuesCount.value = Array.isArray(report?.issues) ? report.issues.length : 0
    devlogApiInfo(`sync doctor OK (${doctorIssuesCount.value} issues)`, {
      kind: 'sync-doctor',
      version: report?.version,
    })
  } catch (e) {
    const detail = e instanceof Error ? e.message : String(e)
    showToast({ message: t('agent.syncDoctorFailed', { msg: detail }), duration: 3000, color: 'danger' })
    devlogApiError(e, { kind: 'sync-doctor', endpoint: '/api/sync/doctor' })
  } finally {
    doctorRunning.value = false
  }
}

async function handleCopyDoctorJson() {
  if (!doctorReportJson.value) return
  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(doctorReportJson.value)
    } else {
      // Fallback: 用一个隐藏 textarea + execCommand
      const ta = document.createElement('textarea')
      ta.value = doctorReportJson.value
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    showToast({ message: t('agent.syncDoctorCopied'), duration: 1500, color: 'success' })
  } catch (e) {
    const detail = e instanceof Error ? e.message : String(e)
    showToast({ message: t('agent.syncDoctorCopyFailed') + ': ' + detail, duration: 2000, color: 'danger' })
    devlogApiError(e, { kind: 'sync-doctor-copy' })
  }
}

onMounted(async () => {
  if (serverOnline.value) {
    await loadConfig()
    configLoaded.value = true
    // 解密 API Key 回填到密码框（加密存储 → 解密明文 → password input 自动掩码）
    await decryptAndLoadApiKey()
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

/* 动态 preset-cards（与 ConfigFieldItem 保持一致，限高滚动） */
.preset-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(80px, 1fr));
  gap: 8px;
  margin-top: 10px;
  width: 100%;
  max-height: 280px;
  overflow-y: auto;
  padding-right: 4px;
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

/* ── 模型选择器样式（见上方 .preset-cards） ──────────────── */

/* ── API Key 状态徽标 ─────────────────────────── */
.apiKeyStatusItem {
  --min-height: 0;
  --padding-start: 16px;
  --padding-end: 16px;
  --inner-padding-end: 0;
  margin-top: 4px;
}
.apiKeyStatusIcon {
  color: var(--ion-color-medium);
  font-size: 18px;
  align-self: flex-start;
  margin-top: 4px;
}
.apiKeyStatusRow {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.apiKeyStatusBadge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  padding: 4px 8px;
  border-radius: 10px;
}
.apiKeyStatusBadgeIcon {
  font-size: 12px;
}
.apiKeyStatusBadgeText {
  font-weight: 500;
  letter-spacing: 0.2px;
}
.apiKeySpinner {
  width: 14px;
  height: 14px;
}
.apiKeyStatusDetail {
  font-size: 11px;
  color: var(--ion-color-medium);
  font-family: monospace;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 6px 0 0;
  line-height: 1.4;
}
.apiKeyBackendLine {
  font-size: 11px;
  color: var(--ion-color-medium);
  margin: 8px 0 0;
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px;
}
.apiKeyBackendLabel {
  font-weight: 500;
}
.apiKeyBackendBase {
  font-family: monospace;
  background: rgba(var(--ion-color-medium-rgb), 0.1);
  padding: 1px 4px;
  border-radius: 3px;
  color: var(--ion-text-color);
}
.apiKeyBackendSource {
  color: var(--ion-color-medium);
  font-style: italic;
}

/* ── Task 25: Sync Doctor 结果块 ───────────────────── */
.doctor-result-item {
  --min-height: 0;
  --padding-start: 16px;
  --padding-end: 16px;
  --inner-padding-end: 0;
}
.doctor-result-wrap {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.doctor-result-pre {
  width: 100%;
  max-height: 320px;
  overflow: auto;
  margin: 0;
  padding: 10px 12px;
  border-radius: 8px;
  background: rgba(var(--ion-color-medium-rgb), 0.08);
  color: var(--ion-text-color);
  font-family: 'SF Mono', Menlo, Consolas, 'Courier New', monospace;
  font-size: 11px;
  line-height: 1.5;
  white-space: pre;
  word-break: normal;
  box-sizing: border-box;
}
.doctor-result-actions {
  display: flex;
  justify-content: flex-end;
}
</style>
