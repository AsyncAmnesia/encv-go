<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>Settings</ion-title>
        <ion-buttons slot="end" v-if="dirty">
          <ion-button @click="handleResetConfig" color="medium">Undo</ion-button>
          <ion-button @click="handleSaveConfig" :disabled="configLoading">
            <ion-icon :icon="save" slot="icon-only"></ion-icon>
          </ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <ion-list>
        <ion-list-header>
          <ion-label>Appearance</ion-label>
        </ion-list-header>
        <ion-item>
          <ion-icon :icon="moon" slot="start"></ion-icon>
          <ion-toggle
            :checked="isDark"
            @ionChange="handleDarkToggle"
          >Dark Mode</ion-toggle>
        </ion-item>
      </ion-list>

      <ion-list>
        <ion-list-header>
          <ion-label>Connection</ion-label>
        </ion-list-header>
        <ion-item>
          <ion-icon :icon="server" slot="start"></ion-icon>
          <ion-input
            v-model="serverUrl"
            label="Server URL"
            label-placement="stacked"
            placeholder="http://127.0.0.1:2025"
            @ionBlur="saveServerUrl"
          ></ion-input>
        </ion-item>
        <ion-item>
          <ion-icon :icon="server" slot="start"></ion-icon>
          <ion-label>
            <h3>Status</h3>
            <p>
              <ion-badge :color="serverOnline ? 'success' : 'danger'">
                {{ serverOnline ? 'Online' : 'Offline' }}
              </ion-badge>
            </p>
          </ion-label>
          <ion-button fill="outline" slot="end" @click="checkServer">
            <ion-icon :icon="refresh" slot="start"></ion-icon>
            Check
          </ion-button>
        </ion-item>
      </ion-list>

      <div v-if="configLoading && !configLoaded" class="loading-container">
        <ion-spinner name="crescent"></ion-spinner>
        <p>Loading configuration...</p>
      </div>

      <template v-else-if="configLoaded">
        <template v-for="section in schemaFields" :key="section.key">
          <ion-list v-if="section.type !== 'object' || !section.properties">
            <ion-list-header>
              <ion-label>{{ section.sectionTitle || section.label }}</ion-label>
            </ion-list-header>
            <ion-item v-if="section.type === 'boolean'">
              <ion-toggle
                :checked="!!getValue([section.key])"
                @ionChange="setValue([section.key], !getValue([section.key]))"
              >{{ section.label }}</ion-toggle>
            </ion-item>
            <ion-item v-else>
              <ion-input
                :value="String(getValue([section.key]) ?? '')"
                :type="section.isPassword ? 'password' : section.type === 'integer' ? 'number' : 'text'"
                :label="section.label"
                label-placement="stacked"
                :placeholder="section.description || section.label"
                :helper-text="section.description || undefined"
                @ionInput="handleInput([section.key], section, $event)"
              ></ion-input>
            </ion-item>
          </ion-list>

          <template v-else>
            <ion-accordion-group>
              <ion-accordion :value="section.key">
                <ion-item slot="header" color="light">
                  <ion-label>{{ section.sectionTitle || section.label }}</ion-label>
                </ion-item>
                <div class="accordion-content">
                  <template v-for="child in section.properties" :key="child.key">
                    <ion-list v-if="child.type === 'object' && child.properties && !child.isMap">
                      <ion-list-header>
                        <ion-label>{{ child.label }}</ion-label>
                      </ion-list-header>
                      <template v-for="grandchild in child.properties" :key="grandchild.key">
                        <ion-item v-if="grandchild.type === 'boolean'">
                          <ion-toggle
                            :checked="!!getValue([section.key, child.key, grandchild.key])"
                            @ionChange="setValue([section.key, child.key, grandchild.key], !getValue([section.key, child.key, grandchild.key]))"
                          >{{ grandchild.label }}</ion-toggle>
                        </ion-item>
                        <ion-item v-else>
                          <ion-input
                            :value="String(getValue([section.key, child.key, grandchild.key]) ?? '')"
                            :type="grandchild.isPassword ? 'password' : grandchild.type === 'integer' ? 'number' : 'text'"
                            :label="grandchild.label"
                            label-placement="stacked"
                            :placeholder="grandchild.description || grandchild.label"
                            :helper-text="grandchild.description || undefined"
                            @ionInput="handleInput([section.key, child.key, grandchild.key], grandchild, $event)"
                          ></ion-input>
                        </ion-item>
                      </template>
                    </ion-list>

                    <ion-list v-else-if="child.isMap">
                      <ion-list-header>
                        <ion-label>{{ child.label }}</ion-label>
                      </ion-list-header>
                      <template v-for="(site, siteKey) in (getValue([section.key, child.key]) as Record<string, Record<string, unknown>>)" :key="siteKey">
                        <ion-item>
                          <ion-label>
                            <h3>{{ siteKey }}</h3>
                            <p v-if="site && typeof site === 'object'">
                              <template v-for="itemField in child.mapItemFields" :key="itemField.key">
                                {{ itemField.label }}: {{ (site as Record<string, unknown>)[itemField.key] || '-' }}&nbsp;
                              </template>
                            </p>
                          </ion-label>
                        </ion-item>
                      </template>
                      <ion-item v-if="!getValue([section.key, child.key]) || Object.keys(getValue([section.key, child.key]) as Record<string, unknown>).length === 0">
                        <ion-label class="ion-text-wrap">
                          <p>No entries configured</p>
                        </ion-label>
                      </ion-item>
                    </ion-list>

                    <ion-item v-else-if="child.type === 'boolean'">
                      <ion-toggle
                        :checked="!!getValue([section.key, child.key])"
                        @ionChange="setValue([section.key, child.key], !getValue([section.key, child.key]))"
                      >{{ child.label }}</ion-toggle>
                    </ion-item>
                    <ion-item v-else>
                      <ion-input
                        :value="String(getValue([section.key, child.key]) ?? '')"
                        :type="child.isPassword ? 'password' : child.type === 'integer' ? 'number' : 'text'"
                        :label="child.label"
                        label-placement="stacked"
                        :placeholder="child.description || child.label"
                        :helper-text="child.description || undefined"
                        @ionInput="handleInput([section.key, child.key], child, $event)"
                      ></ion-input>
                    </ion-item>
                  </template>
                </div>
              </ion-accordion>
            </ion-accordion-group>
          </template>
        </template>
      </template>

      <ion-list>
        <ion-list-header>
          <ion-label>About</ion-label>
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
            <h3>Engine</h3>
            <p>ENCV-go Daemon</p>
          </ion-label>
        </ion-item>
        <ion-item button @click="openGitHub">
          <ion-icon :icon="logoGithub" slot="start"></ion-icon>
          <ion-label>
            <h3>GitHub</h3>
            <p>Source code & issues</p>
          </ion-label>
          <ion-icon :icon="openOutline" slot="end"></ion-icon>
        </ion-item>
      </ion-list>

      <ion-list>
        <ion-list-header>
          <ion-label color="danger">Danger Zone</ion-label>
        </ion-list-header>
        <ion-item button @click="handleClearCache">
          <ion-icon :icon="trash" color="danger" slot="start"></ion-icon>
          <ion-label color="danger">Clear Cache</ion-label>
        </ion-item>
        <ion-item button @click="handleResetSettings">
          <ion-icon :icon="refreshCircle" color="danger" slot="start"></ion-icon>
          <ion-label color="danger">Reset All Settings</ion-label>
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
  IonIcon,
  IonLabel,
  IonToggle,
  IonInput,
  IonBadge,
  IonSpinner,
  IonAccordionGroup,
  IonAccordion,
  alertController,
  toastController,
} from '@ionic/vue'
import {
  moon,
  server,
  refresh,
  save,
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
import { setApiBaseUrl, getServerUrl } from '@/api/encv'
import type { FieldDef } from '@/config/schemaParser'

const { isDark, toggleDark } = useTheme()
const { isOnline: serverOnline, checkStatus } = useServerStatus()
const { schemaFields, loading: configLoading, dirty, loadConfig, saveConfig, resetConfig, getFieldValue, setFieldValue } = useConfig()

const serverUrl = ref(getServerUrl())
const configLoaded = ref(false)

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

function handleDarkToggle() {
  toggleDark()
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
    message: serverOnline.value ? 'Server is online' : 'Server is offline',
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
      message: 'Configuration saved',
      duration: 1500,
      color: 'success',
    })
    await toast.present()
  } catch {
    const toast = await toastController.create({
      message: 'Failed to save configuration',
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
    header: 'Clear Cache',
    message: 'This will clear all cached data. Are you sure?',
    buttons: [
      { text: 'Cancel', role: 'cancel' },
      {
        text: 'Clear',
        role: 'destructive',
        handler: () => {
          const themePref = localStorage.getItem('encv-theme-preference')
          const serverPref = localStorage.getItem('encv-server-url')
          const webdavPref = localStorage.getItem('encv-webdav-configs')
          localStorage.clear()
          if (themePref) localStorage.setItem('encv-theme-preference', themePref)
          if (serverPref) localStorage.setItem('encv-server-url', serverPref)
          if (webdavPref) localStorage.setItem('encv-webdav-configs', webdavPref)
          toastController.create({
            message: 'Cache cleared',
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
    header: 'Reset Settings',
    message: 'This will reset all settings to defaults. Are you sure?',
    buttons: [
      { text: 'Cancel', role: 'cancel' },
      {
        text: 'Reset',
        role: 'destructive',
        handler: () => {
          localStorage.clear()
          serverUrl.value = 'http://127.0.0.1:2025'
          if (isDark.value) toggleDark()
          toastController.create({
            message: 'Settings reset to defaults',
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

.accordion-content {
  padding: 0 8px;
}
</style>
