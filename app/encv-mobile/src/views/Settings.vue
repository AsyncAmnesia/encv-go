<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>{{ t('settings.title') }}</ion-title>
        <ion-buttons slot="end" v-if="dirty">
          <ion-button @click="handleResetConfig" color="medium">{{ t('settings.undo') }}</ion-button>
          <ion-button @click="handleSaveConfig" :disabled="configLoading">
            <ion-icon :icon="saveIcon" slot="icon-only"></ion-icon>
          </ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <ion-list>
        <ion-list-header>
          <ion-label>{{ t('settings.appearance') }}</ion-label>
        </ion-list-header>
        <ion-item>
          <ion-icon :icon="moon" slot="start"></ion-icon>
          <ion-toggle :checked="isDark" @ionChange="handleDarkToggle">{{ t('settings.darkMode') }}</ion-toggle>
        </ion-item>
        <ion-item>
          <ion-icon :icon="globeOutline" slot="start"></ion-icon>
          <ion-select :value="locale" @ionChange="handleLocaleChange" interface="action-sheet" mode="ios">
            <ion-select-option value="zh-CN">简体中文</ion-select-option>
            <ion-select-option value="en">English</ion-select-option>
          </ion-select>
        </ion-item>
      </ion-list>

      <ion-list>
        <ion-list-header>
          <ion-label>{{ t('settings.connection') }}</ion-label>
        </ion-list-header>
        <ion-item button @click="goServer" detail>
          <ion-icon :icon="serverIcon" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.serverTitle') }}</h3>
            <p>
              <ion-badge :color="serverOnline ? 'success' : 'danger'">
                {{ serverOnline ? t('settings.online') : t('settings.offline') }}
              </ion-badge>
              <span v-if="serverOnline && backendPort" class="port-info">:{{ backendPort }}</span>
              <span v-if="!serverOnline && connectionError" class="connection-error-inline"> - {{ connectionError }}</span>
            </p>
          </ion-label>
        </ion-item>
      </ion-list>

      <ion-list>
        <ion-list-header>
          <ion-label>{{ t('settings.cache') }}</ion-label>
        </ion-list-header>
        <ion-item button @click="goCache" detail>
          <ion-icon :icon="databaseIcon" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.cacheAndIndex') }}</h3>
            <p>{{ indexStats?.isIndexing ? t('settings.indexing') : t('settings.indexReady') }}</p>
          </ion-label>
        </ion-item>
      </ion-list>

      <div v-if="configLoading && !configLoaded" class="loading-container">
        <ion-spinner name="crescent"></ion-spinner>
        <p>{{ t('settings.loadingConfig') }}</p>
      </div>

      <template v-else-if="configLoaded">
        <template v-for="section in schemaFields" :key="section.key">
          <ion-list v-if="section.key === 'plugin_settings'">
            <ion-list-header>
              <ion-label>{{ section.sectionTitle ? tSectionTitle(section.sectionTitle) : tField(section.key) }}</ion-label>
            </ion-list-header>
            <ion-item button @click="goPlugins" detail>
              <ion-icon :icon="settingsOutline" slot="start"></ion-icon>
              <ion-label>
                <h3>{{ tField(section.key) }}</h3>
              </ion-label>
            </ion-item>
          </ion-list>

          <ion-list v-else-if="section.type !== 'object' || !section.properties">
            <ion-list-header>
              <ion-label>{{ section.sectionTitle ? tSectionTitle(section.sectionTitle) : tField(section.key) }}</ion-label>
            </ion-list-header>
            <ion-item v-if="section.type === 'boolean'">
              <ion-icon :icon="getFieldIcon(section.key, section.type)" slot="start"></ion-icon>
              <ion-toggle
                :checked="!!getValue([section.key])"
                @ionChange="setValue([section.key], !getValue([section.key]))"
              >{{ tField(section.key) }}</ion-toggle>
            </ion-item>
            <ion-item v-else>
              <ion-icon :icon="getFieldIcon(section.key, section.type)" slot="start"></ion-icon>
              <ion-input
                :value="String(getValue([section.key]) ?? '')"
                :type="section.isPassword ? 'password' : section.type === 'integer' ? 'number' : 'text'"
                :label="fieldLabel(section.key, section.required)"
                label-placement="stacked"
                :placeholder="section.description || tField(section.key)"
                @ionInput="handleInput([section.key], section, $event)"
              ></ion-input>
              <ion-button v-if="section.isPath" slot="end" fill="clear" class="browse-btn" @click="handleBrowsePath([section.key], section)">
                <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
              </ion-button>
            </ion-item>
          </ion-list>

          <ion-list v-else>
            <ion-list-header>
              <ion-label>{{ section.sectionTitle ? tSectionTitle(section.sectionTitle) : tField(section.key) }}</ion-label>
            </ion-list-header>

            <template v-for="child in section.properties" :key="child.key">
              <template v-if="child.type === 'object' && child.properties && !child.isMap">
                <ion-item-divider>
                  <ion-label>{{ tField(child.key) }}</ion-label>
                </ion-item-divider>
                <template v-for="grandchild in child.properties" :key="grandchild.key">
                  <ion-item v-if="grandchild.type === 'boolean'">
                    <ion-icon :icon="getFieldIcon(grandchild.key, grandchild.type)" slot="start"></ion-icon>
                    <ion-toggle
                      :checked="!!getValue([section.key, child.key, grandchild.key])"
                      @ionChange="setValue([section.key, child.key, grandchild.key], !getValue([section.key, child.key, grandchild.key]))"
                    >{{ tField(grandchild.key) }}</ion-toggle>
                  </ion-item>
                  <ion-item v-else>
                    <ion-icon :icon="getFieldIcon(grandchild.key, grandchild.type)" slot="start"></ion-icon>
                    <ion-input
                      :value="String(getValue([section.key, child.key, grandchild.key]) ?? '')"
                      :type="grandchild.isPassword ? 'password' : grandchild.type === 'integer' ? 'number' : 'text'"
                      :label="fieldLabel(grandchild.key, grandchild.required)"
                      label-placement="stacked"
                      :placeholder="grandchild.description || tField(grandchild.key)"
                      @ionInput="handleInput([section.key, child.key, grandchild.key], grandchild, $event)"
                    ></ion-input>
                    <ion-button v-if="grandchild.isPath" slot="end" fill="clear" class="browse-btn" @click="handleBrowsePath([section.key, child.key, grandchild.key], grandchild)">
                      <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
                    </ion-button>
                  </ion-item>
                </template>
              </template>

              <template v-else-if="child.isMap">
                <ion-item-divider>
                  <ion-label>{{ tField(child.key) }}</ion-label>
                </ion-item-divider>
                <template v-if="getMapEntries([section.key, child.key]).length > 0">
                  <ion-item v-for="[entryKey, entryVal] in getMapEntries([section.key, child.key])" :key="entryKey">
                    <ion-label>
                      <h3>{{ entryKey }}</h3>
                      <p v-if="entryVal && typeof entryVal === 'object'">
                        <template v-for="itemField in child.mapItemFields" :key="itemField.key">
                          {{ tField(itemField.key) }}: {{ (entryVal as Record<string, unknown>)[itemField.key] || '-' }}&nbsp;
                        </template>
                      </p>
                    </ion-label>
                  </ion-item>
                </template>
                <ion-item v-else>
                  <ion-label class="ion-text-wrap placeholder-text">
                    <p>{{ t('settings.noEntries') }}</p>
                  </ion-label>
                </ion-item>
              </template>

              <ion-item v-else-if="child.type === 'boolean'">
                <ion-icon :icon="getFieldIcon(child.key, child.type)" slot="start"></ion-icon>
                <ion-toggle
                  :checked="!!getValue([section.key, child.key])"
                  @ionChange="setValue([section.key, child.key], !getValue([section.key, child.key]))"
                >{{ tField(child.key) }}</ion-toggle>
              </ion-item>
              <ion-item v-else-if="section.key === 'log' && child.key === 'level'">
                <ion-icon :icon="getFieldIcon(child.key, child.type)" slot="start"></ion-icon>
                <ion-select
                  :value="String(getValue(['log', 'level']) ?? 'info')"
                  :label="tField('level')"
                  label-placement="stacked"
                  interface="action-sheet"
                  mode="ios"
                  @ionChange="setValue(['log', 'level'], $event.detail.value)"
                >
                  <ion-select-option value="debug">DEBUG</ion-select-option>
                  <ion-select-option value="info">INFO</ion-select-option>
                  <ion-select-option value="warn">WARN</ion-select-option>
                  <ion-select-option value="error">ERROR</ion-select-option>
                </ion-select>
              </ion-item>
              <ion-item v-else>
                <ion-icon :icon="getFieldIcon(child.key, child.type)" slot="start"></ion-icon>
                <ion-input
                  :value="String(getValue([section.key, child.key]) ?? '')"
                  :type="child.isPassword ? 'password' : child.type === 'integer' ? 'number' : 'text'"
                  :label="fieldLabel(child.key, child.required)"
                  label-placement="stacked"
                  :placeholder="child.description || tField(child.key)"
                  @ionInput="handleInput([section.key, child.key], child, $event)"
                ></ion-input>
                <ion-button v-if="child.isPath" slot="end" fill="clear" class="browse-btn" @click="handleBrowsePath([section.key, child.key], child)">
                  <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
                </ion-button>
              </ion-item>
            </template>

            <ion-item v-if="!section.properties || section.properties.length === 0">
              <ion-label class="ion-text-wrap placeholder-text">
                <p>{{ t('settings.noEntries') }}</p>
              </ion-label>
            </ion-item>
          </ion-list>
        </template>
      </template>

      <ion-list>
        <ion-item button @click="goAbout" detail>
          <ion-icon :icon="informationCircle" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.about') }}</h3>
            <p>ENCV-go v1.0.0</p>
          </ion-label>
        </ion-item>
      </ion-list>

      <ion-list>
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
          <div class="json-editor-layout">
            <div class="json-annotations" v-if="configAnnotations.length > 0">
              <div class="annotations-title">{{ t('settings.configAnnotations') }}</div>
              <div v-for="ann in configAnnotations" :key="ann.path" class="annotation-item">
                <span class="annotation-path">{{ ann.path }}</span>
                <span class="annotation-desc">{{ ann.description }}</span>
              </div>
            </div>
            <div class="json-textarea-wrapper">
              <textarea
                v-model="jsonText"
                class="json-textarea"
                spellcheck="false"
                @input="validateJson"
              ></textarea>
              <div v-if="jsonError" class="json-error">
                {{ t('settings.jsonError') }}: {{ jsonError }}
              </div>
            </div>
          </div>
        </ion-content>
      </ion-modal>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons, IonButton,
  IonContent, IonList, IonListHeader, IonItem, IonItemDivider,
  IonIcon, IonLabel, IonToggle, IonInput, IonBadge, IonSpinner,
  IonSelect, IonSelectOption, modalController,
} from '@ionic/vue'
import {
  moon, globeOutline, server as serverIcon, save as saveIcon,
  informationCircle,
  key, lockClosed, documentText, terminal, settingsOutline,
  cloudOutline, shieldCheckmark, eyeOutline, speedometerOutline,
  filmOutline, musicalNotesOutline, imagesOutline, readerOutline,
  newspaperOutline, colorWandOutline, gitNetworkOutline, toggleOutline,
  textOutline, personOutline, folderOpen, refreshCircle,
  fileTrayFull as databaseIcon,
} from 'ionicons/icons'
import { useTheme } from '@/composables/useTheme'
import { useServerStatus } from '@/composables/useServerStatus'
import { useConfig } from '@/composables/useConfig'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import { getIndexStats, fetchConfig, updateConfig } from '@/api/encv'
import type { IndexStats } from '@/api/encv'
import type { FieldDef } from '@/config/schemaParser'
import FilePickerModal from '@/components/FilePickerModal.vue'

const router = useRouter()
const { isDark, toggleDark } = useTheme()
const { isOnline: serverOnline, lastError: connectionError, checkStatus, backendPort } = useServerStatus()
const { schemaFields, loading: configLoading, dirty, loadConfig, saveConfig, resetConfig, getFieldValue, setFieldValue } = useConfig()
const { t, tField, tSectionTitle, setLocale, locale } = useI18n()

const configLoaded = ref(false)
const indexStats = ref<IndexStats | null>(null)

const showJsonEditor = ref(false)
const jsonText = ref('')
const jsonError = ref('')
const configAnnotations = ref<{ path: string; description: string }[]>([])

function extractAnnotations(schema: any, prefix: string = ''): { path: string; description: string }[] {
  const result: { path: string; description: string }[] = []
  if (!schema || typeof schema !== 'object') return result
  if (schema.properties) {
    for (const [key, val] of Object.entries(schema.properties)) {
      const prop = val as any
      const fullPath = prefix ? `${prefix}.${key}` : key
      if (prop.description) {
        result.push({ path: fullPath, description: prop.description })
      }
      if (prop.properties) {
        result.push(...extractAnnotations(prop, fullPath))
      }
    }
  }
  if (schema.$defs) {
    for (const [key, val] of Object.entries(schema.$defs)) {
      result.push(...extractAnnotations(val, `$defs.${key}`))
    }
  }
  return result
}

async function openJsonEditor() {
  try {
    const cfg = await fetchConfig()
    jsonText.value = JSON.stringify(cfg, null, 2)
    jsonError.value = ''
    try {
      const schemaResp = await fetch('/api/config/schema')
      if (schemaResp.ok) {
        const schema = await schemaResp.json()
        configAnnotations.value = extractAnnotations(schema)
      }
    } catch {}
    showJsonEditor.value = true
  } catch {
    showToast({ message: t('settings.configSaveFailed'), duration: 2000, color: 'danger' })
  }
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
    await updateConfig(parsed)
    showJsonEditor.value = false
    showToast({ message: t('settings.configSaved'), duration: 1500, color: 'success' })
    await loadConfig()
  } catch (e) {
    const detail = e instanceof Error ? e.message : String(e)
    showToast({ message: t('settings.configSaveFailed') + ': ' + detail, duration: 3000, color: 'danger' })
  }
}

function goServer() {
  router.push('/tabs/settings/server')
}

function goAbout() {
  router.push('/tabs/settings/about')
}

function goCache() {
  router.push('/tabs/settings/cache')
}

function goPlugins() {
  router.push('/tabs/settings/plugins')
}

function getValue(path: string[]): unknown {
  return getFieldValue(path)
}

function setValue(path: string[], value: unknown) {
  setFieldValue(path, value)
}

function getMapEntries(path: string[]): [string, Record<string, unknown>][] {
  const val = getFieldValue(path)
  if (!val || typeof val !== 'object') return []
  return Object.entries(val as Record<string, unknown>) as [string, Record<string, unknown>][]
}

function handleInput(path: string[], field: FieldDef, event: CustomEvent) {
  const val = (event.target as HTMLInputElement).value
  if (field.type === 'integer') {
    setFieldValue(path, val ? Number(val) : 0)
  } else {
    setFieldValue(path, val)
  }
}

async function handleBrowsePath(path: string[], field: FieldDef) {
  const isFolder = field.key !== 'file'
  const currentVal = String(getFieldValue(path) || '/')
  const modal = await modalController.create({
    component: FilePickerModal,
    componentProps: {
      mode: isFolder ? 'folder' : 'file',
      initialPath: currentVal,
    },
  })
  await modal.present()
  const { data, role } = await modal.onDidDismiss()
  if (role === 'select' && data) {
    setFieldValue(path, data.path)
  }
}

function fieldLabel(key: string, required?: boolean): string {
  return tField(key) + (required ? ' *' : '')
}

const fieldIconMap: Record<string, string> = {
  password: key,
  recover: refreshCircle,
  output_path: folderOpen,
  plugin_settings: settingsOutline,
  server: cloudOutline,
  admin: shieldCheckmark,
  webdav: globeOutline,
  proxy: gitNetworkOutline,
  log: terminal,
  port: speedometerOutline,
  dir: folderOpen,
  username: personOutline,
  root: documentText,
  level: speedometerOutline,
  file: documentText,
  console: terminal,
  host: serverIcon,
  description: textOutline,
  sites: globeOutline,
  disable_signature_verification: shieldCheckmark,
  ext: documentText,
  chunk_size_mb: speedometerOutline,
  light_main_chunk_enabled: colorWandOutline,
  track_extensions: eyeOutline,
  keep_mkv_for_mkvSource: filmOutline,
  verify_after_pack: shieldCheckmark,
  plugin_cache_dir: folderOpen,
  skip_merge_for_split_mkv: filmOutline,
  video: filmOutline,
  audio: musicalNotesOutline,
  image: imagesOutline,
  wps: readerOutline,
  pdf: newspaperOutline,
  text: textOutline,
}

function getFieldIcon(fieldKey: string, fieldType: string): string {
  if (fieldIconMap[fieldKey]) return fieldIconMap[fieldKey]
  if (fieldType === 'boolean') return toggleOutline
  if (fieldType === 'integer') return speedometerOutline
  if (fieldKey.includes('password')) return lockClosed
  return settingsOutline
}

function handleDarkToggle() {
  toggleDark()
}

function handleLocaleChange(event: CustomEvent) {
  setLocale(event.detail.value as 'zh-CN' | 'en')
}

async function handleSaveConfig() {
  try {
    await saveConfig()
    showToast({
      message: t('settings.configSaved'),
      duration: 1500,
      color: 'success',
    })
  } catch (e) {
    const detail = e instanceof Error ? e.message : String(e)
    showToast({
      message: t('settings.configSaveFailed') + ': ' + detail,
      duration: 3000,
      color: 'danger',
    })
  }
}

function handleResetConfig() {
  resetConfig()
}

onMounted(async () => {
  await checkStatus()
  if (serverOnline.value) {
    await loadConfig()
    configLoaded.value = true
    try { indexStats.value = await getIndexStats() } catch {}
  }
})

watch(serverOnline, async (online) => {
  if (online && !configLoaded.value) {
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
.placeholder-text {
  opacity: 0.5;
  font-style: italic;
}
.port-info {
  font-size: 12px;
  opacity: 0.7;
  margin-left: 4px;
}
.connection-error-inline {
  color: var(--ion-color-danger);
  font-size: 12px;
}
.browse-btn {
  --padding-start: 8px;
  --padding-end: 8px;
  min-width: 44px;
  min-height: 44px;
}
.json-editor-content {
  --background: var(--ion-background-color);
}
.json-editor-layout {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.json-annotations {
  border-bottom: 1px solid var(--ion-color-light);
  max-height: 40%;
  overflow-y: auto;
  padding: 12px 16px;
}
.annotations-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ion-text-color);
  margin-bottom: 8px;
}
.annotation-item {
  display: flex;
  flex-direction: column;
  padding: 4px 0;
  border-bottom: 1px solid rgba(var(--ion-color-medium-rgb), 0.15);
}
.annotation-item:last-child {
  border-bottom: none;
}
.annotation-path {
  font-size: 12px;
  font-weight: 600;
  color: var(--ion-color-primary);
  font-family: monospace;
}
.annotation-desc {
  font-size: 12px;
  color: var(--encv-text-secondary);
  margin-top: 2px;
}
.json-textarea-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 0;
  min-height: 0;
}
.json-textarea {
  flex: 1;
  width: 100%;
  min-height: 200px;
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
