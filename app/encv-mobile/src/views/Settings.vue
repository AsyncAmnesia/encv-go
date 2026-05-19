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
          <ion-select :value="locale" @ionChange="handleLocaleChange" interface="action-sheet">
            <ion-select-option value="zh-CN">简体中文</ion-select-option>
            <ion-select-option value="en">English</ion-select-option>
          </ion-select>
        </ion-item>
      </ion-list>

      <ion-list>
        <ion-list-header>
          <ion-label>{{ t('settings.connection') }}</ion-label>
        </ion-list-header>
        <ion-item>
          <ion-icon :icon="serverIcon" slot="start"></ion-icon>
          <ion-input
            v-model="serverUrl"
            :label="t('settings.serverUrl')"
            label-placement="stacked"
            placeholder="http://127.0.0.1:2025"
            @ionBlur="saveServerUrl"
          ></ion-input>
        </ion-item>
        <ion-item>
          <ion-icon :icon="serverIcon" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.status') }}</h3>
            <p>
              <ion-badge :color="serverOnline ? 'success' : 'danger'">
                {{ serverOnline ? t('settings.online') : t('settings.offline') }}
              </ion-badge>
            </p>
          </ion-label>
          <ion-button fill="outline" slot="end" @click="checkServer">
            <ion-icon :icon="refreshIcon" slot="start"></ion-icon>
            {{ t('settings.check') }}
          </ion-button>
        </ion-item>
      </ion-list>

      <div v-if="configLoading && !configLoaded" class="loading-container">
        <ion-spinner name="crescent"></ion-spinner>
        <p>{{ t('settings.loadingConfig') }}</p>
      </div>

      <template v-else-if="configLoaded">
        <template v-for="section in schemaFields" :key="section.key">
          <ion-list v-if="section.type !== 'object' || !section.properties">
            <ion-list-header>
              <ion-label>{{ section.sectionTitle ? tSectionTitle(section.sectionTitle) : tField(section.key) }}</ion-label>
            </ion-list-header>
            <ion-item v-if="section.type === 'boolean'">
              <ion-toggle
                :checked="!!getValue([section.key])"
                @ionChange="setValue([section.key], !getValue([section.key]))"
              >{{ tField(section.key) }}</ion-toggle>
            </ion-item>
            <ion-item v-else>
              <ion-input
                :value="String(getValue([section.key]) ?? '')"
                :type="section.isPassword ? 'password' : section.type === 'integer' ? 'number' : 'text'"
                :label="tField(section.key)"
                label-placement="stacked"
                :placeholder="section.description || tField(section.key)"
                @ionInput="handleInput([section.key], section, $event)"
              ></ion-input>
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
                    <ion-toggle
                      :checked="!!getValue([section.key, child.key, grandchild.key])"
                      @ionChange="setValue([section.key, child.key, grandchild.key], !getValue([section.key, child.key, grandchild.key]))"
                    >{{ tField(grandchild.key) }}</ion-toggle>
                  </ion-item>
                  <ion-item v-else>
                    <ion-input
                      :value="String(getValue([section.key, child.key, grandchild.key]) ?? '')"
                      :type="grandchild.isPassword ? 'password' : grandchild.type === 'integer' ? 'number' : 'text'"
                      :label="tField(grandchild.key)"
                      label-placement="stacked"
                      :placeholder="grandchild.description || tField(grandchild.key)"
                      @ionInput="handleInput([section.key, child.key, grandchild.key], grandchild, $event)"
                    ></ion-input>
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
                <ion-toggle
                  :checked="!!getValue([section.key, child.key])"
                  @ionChange="setValue([section.key, child.key], !getValue([section.key, child.key]))"
                >{{ tField(child.key) }}</ion-toggle>
              </ion-item>
              <ion-item v-else>
                <ion-input
                  :value="String(getValue([section.key, child.key]) ?? '')"
                  :type="child.isPassword ? 'password' : child.type === 'integer' ? 'number' : 'text'"
                  :label="tField(child.key)"
                  label-placement="stacked"
                  :placeholder="child.description || tField(child.key)"
                  @ionInput="handleInput([section.key, child.key], child, $event)"
                ></ion-input>
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
        <ion-list-header>
          <ion-label>{{ t('settings.about') }}</ion-label>
        </ion-list-header>
        <ion-item>
          <ion-icon :icon="informationCircle" slot="start"></ion-icon>
          <ion-label>
            <h3>ENCV-go</h3>
            <p>Version 1.0.0</p>
          </ion-label>
        </ion-item>
        <ion-item>
          <ion-icon :icon="codeSlash" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.engine') }}</h3>
            <p>ENCV-go Daemon</p>
          </ion-label>
        </ion-item>
        <ion-item button @click="openGitHub">
          <ion-icon :icon="logoGithub" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.github') }}</h3>
            <p>{{ t('settings.sourceCode') }}</p>
          </ion-label>
          <ion-icon :icon="openOutline" slot="end"></ion-icon>
        </ion-item>
      </ion-list>

      <ion-list>
        <ion-list-header>
          <ion-label color="danger">{{ t('settings.dangerZone') }}</ion-label>
        </ion-list-header>
        <ion-item button @click="handleClearCache">
          <ion-icon :icon="trash" color="danger" slot="start"></ion-icon>
          <ion-label color="danger">{{ t('settings.clearCache') }}</ion-label>
        </ion-item>
        <ion-item button @click="handleResetSettings">
          <ion-icon :icon="refreshCircle" color="danger" slot="start"></ion-icon>
          <ion-label color="danger">{{ t('settings.resetSettings') }}</ion-label>
        </ion-item>
      </ion-list>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  IonPage,
  IonHeader,
  IonToolbar,
  IonTitle,
  IonButtons,
  IonButton,
  IonContent,
  IonList,
  IonListHeader,
  IonItem,
  IonItemDivider,
  IonIcon,
  IonLabel,
  IonToggle,
  IonInput,
  IonBadge,
  IonSpinner,
  IonSelect,
  IonSelectOption,
  alertController,
  toastController,
} from '@ionic/vue'
import {
  moon,
  globeOutline,
  server as serverIcon,
  refresh as refreshIcon,
  save as saveIcon,
  informationCircle,
  codeSlash,
  logoGithub,
  openOutline,
  trash,
  refreshCircle,
} from 'ionicons/icons'
import { useTheme } from '@/composables/useTheme'
import { useServerStatus } from '@/composables/useServerStatus'
import { useConfig } from '@/composables/useConfig'
import { useI18n } from '@/composables/useI18n'
import { setApiBaseUrl, getServerUrl } from '@/api/encv'
import type { FieldDef } from '@/config/schemaParser'

const { isDark, toggleDark } = useTheme()
const { isOnline: serverOnline, checkStatus } = useServerStatus()
const { schemaFields, loading: configLoading, dirty, loadConfig, saveConfig, resetConfig, getFieldValue, setFieldValue } = useConfig()
const { t, tField, tSectionTitle, setLocale, locale } = useI18n()

const serverUrl = ref(getServerUrl())
const configLoaded = ref(false)

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

function handleDarkToggle() {
  toggleDark()
}

function handleLocaleChange(event: CustomEvent) {
  setLocale(event.detail.value as 'zh-CN' | 'en')
}

function saveServerUrl() {
  const url = serverUrl.value.trim()
  if (url) {
    setApiBaseUrl(url)
    checkStatus()
  }
}

async function checkServer() {
  await checkStatus()
  const toast = await toastController.create({
    message: serverOnline.value ? t('settings.serverOnline') : t('settings.serverOffline'),
    duration: 1500,
    color: serverOnline.value ? 'success' : 'danger',
  })
  await toast.present()
}

function openGitHub() {
  window.open('https://github.com/encv-go', '_blank')
}

async function handleSaveConfig() {
  try {
    await saveConfig()
    const toast = await toastController.create({
      message: t('settings.configSaved'),
      duration: 1500,
      color: 'success',
    })
    await toast.present()
  } catch {
    const toast = await toastController.create({
      message: t('settings.configSaveFailed'),
      duration: 2000,
      color: 'danger',
    })
    await toast.present()
  }
}

function handleResetConfig() {
  resetConfig()
}

async function handleClearCache() {
  const alert = await alertController.create({
    header: t('settings.clearCache'),
    message: t('settings.clearCacheConfirm'),
    buttons: [
      { text: t('settings.cancel'), role: 'cancel' },
      {
        text: t('settings.clear'),
        role: 'destructive',
        handler: () => {
          const themePref = localStorage.getItem('encv-theme-preference')
          const serverPref = localStorage.getItem('encv-server-url')
          const webdavPref = localStorage.getItem('encv-webdav-configs')
          const localePref = localStorage.getItem('encv-locale')
          localStorage.clear()
          if (themePref) localStorage.setItem('encv-theme-preference', themePref)
          if (serverPref) localStorage.setItem('encv-server-url', serverPref)
          if (webdavPref) localStorage.setItem('encv-webdav-configs', webdavPref)
          if (localePref) localStorage.setItem('encv-locale', localePref)
          toastController.create({
            message: t('settings.cacheCleared'),
            duration: 1500,
            color: 'success',
          }).then(t => t.present())
        },
      },
    ],
  })
  await alert.present()
}

async function handleResetSettings() {
  const alert = await alertController.create({
    header: t('settings.resetSettings'),
    message: t('settings.resetConfirm'),
    buttons: [
      { text: t('settings.cancel'), role: 'cancel' },
      {
        text: t('settings.reset'),
        role: 'destructive',
        handler: () => {
          localStorage.clear()
          serverUrl.value = 'http://127.0.0.1:2025'
          if (isDark.value) toggleDark()
          toastController.create({
            message: t('settings.settingsReset'),
            duration: 1500,
            color: 'success',
          }).then(t => t.present())
        },
      },
    ],
  })
  await alert.present()
}

onMounted(async () => {
  checkStatus()
  await loadConfig()
  configLoaded.value = true
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
