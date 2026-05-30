<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button default-href="/tabs/settings"></ion-back-button>
        </ion-buttons>
        <ion-title>{{ t('settings.pluginSettings') }}</ion-title>
        <ion-buttons slot="end" v-if="dirty">
          <ion-button @click="handleResetConfig" color="medium">{{ t('settings.undo') }}</ion-button>
          <ion-button @click="handleSaveConfig" :disabled="configLoading || suffixConflict.length > 0">
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

      <template v-else-if="configLoaded && pluginSection">
        <ion-list>
          <ion-list-header>
            <ion-label>{{ pluginSection.sectionTitle ? tSectionTitle(pluginSection.sectionTitle) : tField(pluginSection.key) }}</ion-label>
          </ion-list-header>

          <template v-for="child in pluginSection.properties" :key="child.key">
            <template v-if="child.type === 'object' && child.properties && !child.isMap">
              <ion-item-divider>
                <ion-label>{{ tField(child.key) }}</ion-label>
              </ion-item-divider>
              <template v-for="grandchild in child.properties" :key="grandchild.key">
                <template v-if="isFieldVisible(grandchild)">
                  <ion-item v-if="grandchild.type === 'boolean'">
                    <ion-icon :icon="getFieldIcon(grandchild.key, grandchild.type)" slot="start"></ion-icon>
                    <ion-toggle
                      :checked="!!getValue([pluginSection.key, child.key, grandchild.key])"
                      @ionChange="setValue([pluginSection.key, child.key, grandchild.key], !getValue([pluginSection.key, child.key, grandchild.key]))"
                    >{{ tField(grandchild.key) }}<span v-if="shouldShowBadge(grandchild)" class="config-badge" :class="grandchild.isV4 ? 'badge-v4' : grandchild.platform === 'mobile' ? 'badge-mobile' : 'badge-server'">{{ grandchild.isV4 ? 'v4' : grandchild.platform === 'mobile' ? '移动端' : '服务端' }}</span></ion-toggle>
                  </ion-item>
                  <ion-item v-else-if="grandchild.isSelect && grandchild.selectOptions">
                    <ion-icon :icon="getFieldIcon(grandchild.key, grandchild.type)" slot="start"></ion-icon>
                    <ion-label>
                      <h3>{{ tField(grandchild.key) }}<span v-if="shouldShowBadge(grandchild)" class="config-badge" :class="grandchild.isV4 ? 'badge-v4' : grandchild.platform === 'mobile' ? 'badge-mobile' : 'badge-server'">{{ grandchild.isV4 ? 'v4' : grandchild.platform === 'mobile' ? '移动端' : '服务端' }}</span></h3>
                    </ion-label>
                    <div class="preset-cards" slot="end">
                      <div
                        v-for="opt in grandchild.selectOptions"
                        :key="opt.value"
                        class="preset-card"
                        :class="{ 'preset-card-active': getValue([pluginSection.key, child.key, grandchild.key]) === opt.value }"
                        @click="setValue([pluginSection.key, child.key, grandchild.key], opt.value)"
                      >
                        <div class="preset-card-title">{{ opt.label }}</div>
                        <div class="preset-card-desc">{{ opt.description }}</div>
                      </div>
                    </div>
                  </ion-item>
                  <ion-item v-else>
                    <ion-icon :icon="getFieldIcon(grandchild.key, grandchild.type)" slot="start"></ion-icon>
                    <ion-input
                      :value="String(getValue([pluginSection.key, child.key, grandchild.key]) ?? '')"
                      :type="grandchild.isPassword ? 'password' : grandchild.type === 'integer' ? 'number' : 'text'"
                      :label="fieldLabel(grandchild.key, grandchild.required)"
                      label-placement="stacked"
                      :placeholder="grandchild.description || tField(grandchild.key)"
                      @ionInput="handleInput([pluginSection.key, child.key, grandchild.key], grandchild, $event)"
                    ></ion-input>
                    <ion-button v-if="grandchild.isPath" slot="end" fill="clear" class="browse-btn" @click="handleBrowsePath([pluginSection.key, child.key, grandchild.key], grandchild)">
                      <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
                    </ion-button>
                  </ion-item>
                </template>
              </template>
            </template>

            <template v-else-if="child.isMap">
              <ion-item-divider>
                <ion-label>{{ tField(child.key) }}</ion-label>
              </ion-item-divider>
              <template v-if="getMapEntries([pluginSection.key, child.key]).length > 0">
                <ion-item v-for="[entryKey, entryVal] in getMapEntries([pluginSection.key, child.key])" :key="entryKey">
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

            <template v-else-if="isFieldVisible(child)">
              <ion-item v-if="child.type === 'boolean'">
                <ion-icon :icon="getFieldIcon(child.key, child.type)" slot="start"></ion-icon>
                <ion-toggle
                  :checked="!!getValue([pluginSection.key, child.key])"
                  @ionChange="setValue([pluginSection.key, child.key], !getValue([pluginSection.key, child.key]))"
                >{{ tField(child.key) }}<span v-if="shouldShowBadge(child)" class="config-badge" :class="child.isV4 ? 'badge-v4' : child.platform === 'mobile' ? 'badge-mobile' : 'badge-server'">{{ child.isV4 ? 'v4' : child.platform === 'mobile' ? '移动端' : '服务端' }}</span></ion-toggle>
              </ion-item>
              <ion-item v-else-if="child.isSelect && child.selectOptions">
                <ion-icon :icon="getFieldIcon(child.key, child.type)" slot="start"></ion-icon>
                <ion-label>
                  <h3>{{ tField(child.key) }}<span v-if="shouldShowBadge(child)" class="config-badge" :class="child.isV4 ? 'badge-v4' : child.platform === 'mobile' ? 'badge-mobile' : 'badge-server'">{{ child.isV4 ? 'v4' : child.platform === 'mobile' ? '移动端' : '服务端' }}</span></h3>
                </ion-label>
                <div class="preset-cards" slot="end">
                  <div
                    v-for="opt in child.selectOptions"
                    :key="opt.value"
                    class="preset-card"
                    :class="{ 'preset-card-active': getValue([pluginSection.key, child.key]) === opt.value }"
                    @click="setValue([pluginSection.key, child.key], opt.value)"
                  >
                    <div class="preset-card-title">{{ opt.label }}</div>
                    <div class="preset-card-desc">{{ opt.description }}</div>
                  </div>
                </div>
              </ion-item>
              <ion-item v-else>
                <ion-icon :icon="getFieldIcon(child.key, child.type)" slot="start"></ion-icon>
                <ion-input
                  :value="String(getValue([pluginSection.key, child.key]) ?? '')"
                  :type="child.isPassword ? 'password' : child.type === 'integer' ? 'number' : 'text'"
                  :label="fieldLabel(child.key, child.required)"
                  label-placement="stacked"
                  :placeholder="child.description || tField(child.key)"
                  @ionInput="handleInput([pluginSection.key, child.key], child, $event)"
                ></ion-input>
                <ion-button v-if="child.isPath" slot="end" fill="clear" class="browse-btn" @click="handleBrowsePath([pluginSection.key, child.key], child)">
                  <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
                </ion-button>
              </ion-item>
            </template>
          </template>

          <ion-item v-if="!pluginSection.properties || pluginSection.properties.length === 0">
            <ion-label class="ion-text-wrap placeholder-text">
              <p>{{ t('settings.noEntries') }}</p>
            </ion-label>
          </ion-item>
        </ion-list>

        <div v-if="suffixConflict.length > 0" class="suffix-conflict-warning" :class="{ 'api-unavailable': suffixConflict.includes(UNAVAILABLE) }">
          <ion-icon :icon="warningOutline"></ion-icon>
          <span v-if="suffixConflict.includes(UNAVAILABLE)">{{ t('settings.suffixCheckUnavailable') }}</span>
          <span v-else>{{ t('settings.suffixConflictWarning', { suffix: String(getFieldValue(['plugin_settings', 'alist_encrypt', 'suffix']) ?? ''), plugins: suffixConflict.join(', ') }) }}</span>
        </div>

        <ion-list v-if="isNative()">
        </ion-list>
      </template>

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
import { ref, computed, onMounted, watch } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons, IonButton,
  IonBackButton, IonContent, IonList, IonListHeader, IonItem, IonItemDivider,
  IonIcon, IonLabel, IonToggle, IonInput, IonSpinner, IonModal,
  modalController,
} from '@ionic/vue'
import {
  save as saveIcon, settingsOutline, shieldCheckmark, speedometerOutline,
  filmOutline, musicalNotesOutline, imagesOutline, readerOutline,
  newspaperOutline, eyeOutline, folderOpen,
  documentText, toggleOutline, lockClosed, textOutline,
  colorPaletteOutline, layersOutline, warningOutline,
} from 'ionicons/icons'
import { useConfig } from '@/composables/useConfig'
import { usePluginExtensions } from '@/composables/usePluginExtensions'
import { useServerStatus } from '@/composables/useServerStatus'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import { fetchConfig, updateConfig } from '@/api/encv'
import type { FieldDef } from '@/config/schemaParser'
import { isNative } from '@/plugins/GoProcess'
import FilePickerModal from '@/components/FilePickerModal.vue'

const { isOnline: serverOnline } = useServerStatus()
const { schemaFields, loading: configLoading, dirty, loadConfig, saveConfig, resetConfig, getFieldValue, setFieldValue } = useConfig()
const { getConflictingPlugins, load: loadExtensions, UNAVAILABLE } = usePluginExtensions()
const { t, tField, tSectionTitle } = useI18n()

const configLoaded = ref(false)
const suffixConflict = ref<string[]>([])

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

const pluginSection = computed<FieldDef | undefined>(() => {
  return schemaFields.value.find(s => s.key === 'plugin_settings')
})

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

  if (path.length === 3 && path[0] === 'plugin_settings' && path[1] === 'alist_encrypt' && path[2] === 'suffix') {
    checkSuffixConflict(val)
  }
}

function checkSuffixConflict(suffix: string) {
  if (!suffix || suffix === '.') {
    suffixConflict.value = []
    return
  }
  const conflicts = getConflictingPlugins(suffix)
  suffixConflict.value = conflicts
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
  plugin_cache_dir: folderOpen,
  ext: documentText,
  container_chunk_size_mb: filmOutline,
  light_container_main_chunk_enabled: layersOutline,
  track_extensions: eyeOutline,
  keep_mkv_for_mkv_source: filmOutline,
  verify_after_pack: shieldCheckmark,
  skip_merge_for_split_mkv: filmOutline,
  allow_no_reencode: speedometerOutline,
  default_stream_preset: colorPaletteOutline,
  video: filmOutline,
  audio: musicalNotesOutline,
  image: imagesOutline,
  wps: readerOutline,
  pdf: newspaperOutline,
  text: textOutline,
  disable_signature_verification: shieldCheckmark,
}

function getFieldIcon(fieldKey: string, fieldType: string): string {
  if (fieldIconMap[fieldKey]) return fieldIconMap[fieldKey]
  if (fieldType === 'boolean') return toggleOutline
  if (fieldType === 'integer') return speedometerOutline
  if (fieldKey.includes('password')) return lockClosed
  return settingsOutline
}

function isFieldVisible(field: FieldDef): boolean {
  if (!field.platform || field.platform === 'both') return true
  if (field.platform === 'mobile') return isNative()
  if (field.platform === 'desktop') return !isNative()
  return true
}

function shouldShowBadge(field: FieldDef): boolean {
  return !!field.isV4 || field.platform === 'mobile'
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

onMounted(async () => {
  if (serverOnline.value) {
    await loadConfig()
    configLoaded.value = true
    try { await loadExtensions() } catch {}
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
.browse-btn {
  --padding-start: 8px;
  --padding-end: 8px;
  min-width: 44px;
  min-height: 44px;
}
.config-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 11px;
  font-weight: 600;
  margin-left: 6px;
}
.badge-server { background: #3880ff; color: white; }
.badge-mobile { background: #8c61ff; color: white; }
.badge-v4 { background: #2dd36f; color: white; }
.preset-cards {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}
.preset-card {
  flex: 1;
  padding: 12px;
  border: 2px solid #e0e0e0;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.2s;
}
.preset-card-active {
  border-color: #3880ff;
  background: rgba(56, 128, 255, 0.08);
}
.preset-card-title {
  font-weight: 600;
  font-size: 14px;
}
.preset-card-desc {
  font-size: 12px;
  color: #666;
  margin-top: 4px;
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
.suffix-conflict-warning {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 8px 16px;
  padding: 10px 14px;
  background: rgba(255, 152, 0, 0.1);
  border-radius: 8px;
  border-left: 3px solid #e65100;
  color: #e65100;
  font-size: 13px;
}
.suffix-conflict-warning.api-unavailable {
  background: rgba(var(--ion-color-danger-rgb), 0.08);
  border-left-color: var(--ion-color-danger);
  color: var(--ion-color-danger);
}
.suffix-conflict-warning ion-icon {
  font-size: 20px;
  flex-shrink: 0;
}
</style>
