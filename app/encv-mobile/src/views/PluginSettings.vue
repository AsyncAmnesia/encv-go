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
                <ion-item v-if="grandchild.type === 'boolean'">
                  <ion-icon :icon="getFieldIcon(grandchild.key, grandchild.type)" slot="start"></ion-icon>
                  <ion-toggle
                    :checked="!!getValue([pluginSection.key, child.key, grandchild.key])"
                    @ionChange="setValue([pluginSection.key, child.key, grandchild.key], !getValue([pluginSection.key, child.key, grandchild.key]))"
                  >{{ tField(grandchild.key) }}</ion-toggle>
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
                </ion-item>
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

            <ion-item v-else-if="child.type === 'boolean'">
              <ion-icon :icon="getFieldIcon(child.key, child.type)" slot="start"></ion-icon>
              <ion-toggle
                :checked="!!getValue([pluginSection.key, child.key])"
                @ionChange="setValue([pluginSection.key, child.key], !getValue([pluginSection.key, child.key]))"
              >{{ tField(child.key) }}</ion-toggle>
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
            </ion-item>
          </template>

          <ion-item v-if="!pluginSection.properties || pluginSection.properties.length === 0">
            <ion-label class="ion-text-wrap placeholder-text">
              <p>{{ t('settings.noEntries') }}</p>
            </ion-label>
          </ion-item>
        </ion-list>
      </template>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons, IonButton,
  IonBackButton, IonContent, IonList, IonListHeader, IonItem, IonItemDivider,
  IonIcon, IonLabel, IonToggle, IonInput, IonSpinner,
} from '@ionic/vue'
import {
  save as saveIcon, settingsOutline, shieldCheckmark, speedometerOutline,
  filmOutline, musicalNotesOutline, imagesOutline, readerOutline,
  newspaperOutline, colorWandOutline, eyeOutline, folderOpen,
  documentText, toggleOutline, lockClosed, textOutline,
} from 'ionicons/icons'
import { useConfig } from '@/composables/useConfig'
import { useServerStatus } from '@/composables/useServerStatus'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import type { FieldDef } from '@/config/schemaParser'

const { isOnline: serverOnline } = useServerStatus()
const { schemaFields, loading: configLoading, dirty, loadConfig, saveConfig, resetConfig, getFieldValue, setFieldValue } = useConfig()
const { t, tField, tSectionTitle } = useI18n()

const configLoaded = ref(false)

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
}

function fieldLabel(key: string, required?: boolean): string {
  return tField(key) + (required ? ' *' : '')
}

const fieldIconMap: Record<string, string> = {
  plugin_cache_dir: folderOpen,
  ext: documentText,
  chunk_size_mb: speedometerOutline,
  light_main_chunk_enabled: colorWandOutline,
  track_extensions: eyeOutline,
  keep_mkv_for_mkvSource: filmOutline,
  verify_after_pack: shieldCheckmark,
  skip_merge_for_split_mkv: filmOutline,
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
</style>
