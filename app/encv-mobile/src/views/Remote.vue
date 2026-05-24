<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>{{ t('remote.title') }}</ion-title>
      </ion-toolbar>
      <ion-toolbar>
        <ion-segment v-model="activeTab" @ionChange="onTabChange">
          <ion-segment-button value="webdav">
            <ion-label>WebDAV</ion-label>
          </ion-segment-button>
          <ion-segment-button value="openlist">
            <ion-label>Openlist</ion-label>
          </ion-segment-button>
        </ion-segment>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <div v-if="activeTab === 'webdav'">
        <div v-if="webdavConfigs.length === 0 && !builtInWebdav" class="empty-state">
          <ion-icon :icon="cloud" class="empty-icon"></ion-icon>
          <h3>{{ t('webdav.noServers') }}</h3>
          <p>{{ t('webdav.noServersDesc') }}</p>
        </div>

        <ion-list v-else>
          <ion-item v-if="builtInWebdav" class="built-in-item">
            <ion-icon :icon="home" color="primary" slot="start"></ion-icon>
            <ion-label>
              <h2>{{ t('remote.builtInWebdav') }}</h2>
              <p>{{ builtInWebdav.url }}</p>
              <p v-if="builtInWebdav.username">{{ t('webdav.username') }}: {{ builtInWebdav.username }}</p>
            </ion-label>
            <ion-badge :color="builtInWebdav.enabled ? 'success' : 'medium'" slot="end">
              {{ builtInWebdav.enabled ? t('remote.enabled') : t('remote.disabled') }}
            </ion-badge>
          </ion-item>

          <ion-item-sliding v-for="config in webdavConfigs" :key="config.id">
            <ion-item @click="editConfig(config)">
              <ion-icon :icon="cloud" color="primary" slot="start"></ion-icon>
              <ion-label>
                <h2>{{ config.name }}</h2>
                <p>{{ config.url }}</p>
                <p v-if="config.mountPath">{{ t('webdav.mount') }}: {{ config.mountPath }}</p>
              </ion-label>
              <ion-badge color="medium" slot="end">{{ t('webdav.saved') }}</ion-badge>
            </ion-item>
            <ion-item-options side="end">
              <ion-item-option color="primary" @click="testConfig(config)">
                {{ t('webdav.test') }}
              </ion-item-option>
              <ion-item-option color="danger" @click="deleteConfig(config.id)">
                {{ t('webdav.delete') }}
              </ion-item-option>
            </ion-item-options>
          </ion-item-sliding>
        </ion-list>

        <ion-fab vertical="bottom" horizontal="end" slot="fixed">
          <ion-fab-button @click="openNewConfig">
            <ion-icon :icon="add"></ion-icon>
          </ion-fab-button>
        </ion-fab>
      </div>

      <div v-if="activeTab === 'openlist'">
        <div v-if="openlistSiteKeys.length === 0" class="empty-state">
          <ion-icon :icon="globe" class="empty-icon"></ion-icon>
          <h3>{{ t('remote.noOpenlistSites') }}</h3>
          <p>{{ t('remote.noOpenlistSitesDesc') }}</p>
        </div>

        <ion-list v-else>
          <ion-item-sliding v-for="key in openlistSiteKeys" :key="key">
            <ion-item @click="editSite(key)">
              <ion-icon :icon="globe" color="primary" slot="start"></ion-icon>
              <ion-label>
                <h2>{{ key }}</h2>
                <p v-if="openlistSites[key].description">{{ openlistSites[key].description }}</p>
                <p>{{ t('remote.host') }}: {{ openlistSites[key].host }}</p>
                <p class="proxy-url">{{ t('remote.proxyUrl') }}: {{ openlistSites[key].proxyUrl }}</p>
              </ion-label>
            </ion-item>
            <ion-item-options side="end">
              <ion-item-option color="primary" @click.stop="copyProxyUrl(openlistSites[key].proxyUrl)">
                {{ t('remote.copied') }}
              </ion-item-option>
              <ion-item-option color="danger" @click.stop="handleDeleteSite(key)">
                {{ t('webdav.delete') }}
              </ion-item-option>
            </ion-item-options>
          </ion-item-sliding>
        </ion-list>

        <ion-fab vertical="bottom" horizontal="end" slot="fixed">
          <ion-fab-button @click="openNewSite">
            <ion-icon :icon="add"></ion-icon>
          </ion-fab-button>
        </ion-fab>
      </div>

      <ion-modal :is-open="showWebdavModal" @didDismiss="showWebdavModal = false">
        <ion-header>
          <ion-toolbar>
            <ion-title>{{ editingId ? t('webdav.edit') : t('webdav.add') }} WebDAV</ion-title>
            <ion-buttons slot="end">
              <ion-button @click="showWebdavModal = false">{{ t('settings.cancel') }}</ion-button>
            </ion-buttons>
          </ion-toolbar>
        </ion-header>
        <ion-content class="ion-padding">
          <ion-list>
            <ion-item>
              <ion-input v-model="formName" :label="t('webdav.name')" label-placement="stacked" placeholder="My WebDAV Server"></ion-input>
            </ion-item>
            <ion-item>
              <ion-input v-model="formUrl" :label="t('webdav.serverUrl')" label-placement="stacked" placeholder="https://dav.example.com"></ion-input>
            </ion-item>
            <ion-item>
              <ion-input v-model="formUsername" :label="t('webdav.username')" label-placement="stacked" placeholder="user"></ion-input>
            </ion-item>
            <ion-item>
              <ion-input v-model="formPassword" :type="showPassword ? 'text' : 'password'" :label="t('webdav.password')" label-placement="stacked" placeholder="password"></ion-input>
              <ion-button fill="clear" slot="end" @click="showPassword = !showPassword">
                <ion-icon :icon="showPassword ? eyeOff : eye"></ion-icon>
              </ion-button>
            </ion-item>
            <ion-item>
              <ion-input v-model="formMountPath" :label="t('webdav.mountPath')" label-placement="stacked" placeholder="/webdav"></ion-input>
            </ion-item>
          </ion-list>
          <ion-button expand="block" @click="testConnection" :disabled="testing || !formUrl">
            <ion-icon :icon="flash" slot="start"></ion-icon>
            {{ testing ? t('webdav.testing') : t('webdav.testConnection') }}
          </ion-button>
          <ion-button expand="block" class="ion-margin-top" @click="saveConfig" :disabled="!formName || !formUrl">
            <ion-icon :icon="saveIcon" slot="start"></ion-icon>
            {{ t('webdav.save') }}
          </ion-button>
        </ion-content>
      </ion-modal>

      <ion-modal :is-open="showSiteModal" @didDismiss="showSiteModal = false">
        <ion-header>
          <ion-toolbar>
            <ion-title>{{ editingSiteId ? t('remote.editSite') : t('remote.addSite') }}</ion-title>
            <ion-buttons slot="end">
              <ion-button @click="showSiteModal = false">{{ t('settings.cancel') }}</ion-button>
            </ion-buttons>
          </ion-toolbar>
        </ion-header>
        <ion-content class="ion-padding">
          <ion-list>
            <ion-item>
              <ion-input
                v-model="formSiteId"
                :label="t('remote.siteId')"
                label-placement="stacked"
                :placeholder="t('remote.siteIdPlaceholder')"
                :disabled="!!editingSiteId"
                :error-text="formSiteIdError"
                :class="{ 'ion-invalid': !!formSiteIdError, 'ion-touched': !!formSiteIdError }"
                @ionInput="validateSiteId"
              ></ion-input>
            </ion-item>
            <ion-item>
              <ion-input
                v-model="formHost"
                :label="t('remote.host')"
                label-placement="stacked"
                :placeholder="t('remote.hostPlaceholder')"
                :error-text="formHostError"
                :class="{ 'ion-invalid': !!formHostError, 'ion-touched': !!formHostError }"
                @ionInput="validateHost"
              ></ion-input>
            </ion-item>
            <ion-item>
              <ion-input v-model="formDescription" :label="t('remote.description')" label-placement="stacked" :placeholder="t('remote.descriptionPlaceholder')"></ion-input>
            </ion-item>
          </ion-list>
          <ion-button expand="block" @click="saveSite" :disabled="!formSiteId || !formHost || !!formSiteIdError || !!formHostError">
            {{ t('webdav.save') }}
          </ion-button>
        </ion-content>
      </ion-modal>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonContent,
  IonSegment, IonSegmentButton, IonLabel,
  IonList, IonItem, IonItemSliding, IonItemOptions, IonItemOption,
  IonIcon, IonBadge, IonFab, IonFabButton,
  IonModal, IonButtons, IonButton, IonInput,
} from '@ionic/vue'
import { add, cloud, flash, save as saveIcon, eye, eyeOff, home, globe } from 'ionicons/icons'
import {
  getWebDAVConfigs, saveWebDAVConfigs, testWebDAVConnection,
  fetchRemoteInfo, addOpenlistSite, updateOpenlistSite, deleteOpenlistSite,
} from '@/api/encv'
import type { WebDAVConfig, RemoteWebDAVInfo, OpenlistSiteInfo } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'

const { t } = useI18n()

const activeTab = ref<'webdav' | 'openlist'>('webdav')
const webdavConfigs = ref<WebDAVConfig[]>([])
const builtInWebdav = ref<RemoteWebDAVInfo | null>(null)
const openlistSites = ref<Record<string, OpenlistSiteInfo>>({})
const openlistSiteKeys = computed(() => Object.keys(openlistSites.value))

const showWebdavModal = ref(false)
const editingId = ref('')
const testing = ref(false)
const showPassword = ref(false)
const formName = ref('')
const formUrl = ref('')
const formUsername = ref('')
const formPassword = ref('')
const formMountPath = ref('')

const showSiteModal = ref(false)
const editingSiteId = ref('')
const formSiteId = ref('')
const formHost = ref('')
const formDescription = ref('')
const formSiteIdError = ref('')
const formHostError = ref('')

function onTabChange() {
  if (activeTab.value === 'openlist') {
    loadRemoteInfo()
  }
}

async function loadRemoteInfo() {
  try {
    const info = await fetchRemoteInfo()
    if (info.webdav && info.webdav.enabled) {
      builtInWebdav.value = info.webdav
    } else {
      builtInWebdav.value = null
    }
    openlistSites.value = info.openlistSites || {}
  } catch {
    // silent
  }
}

function loadConfigs() {
  webdavConfigs.value = getWebDAVConfigs()
}

function openNewConfig() {
  editingId.value = ''
  formName.value = ''
  formUrl.value = ''
  formUsername.value = ''
  formPassword.value = ''
  formMountPath.value = '/webdav'
  showWebdavModal.value = true
}

function editConfig(config: WebDAVConfig) {
  editingId.value = config.id
  formName.value = config.name
  formUrl.value = config.url
  formUsername.value = config.username
  formPassword.value = config.password
  formMountPath.value = config.mountPath
  showWebdavModal.value = true
}

function saveConfig() {
  if (!formName.value || !formUrl.value) return
  let updated: WebDAVConfig[]
  if (editingId.value) {
    updated = webdavConfigs.value.map(c =>
      c.id === editingId.value
        ? { ...c, name: formName.value, url: formUrl.value, username: formUsername.value, password: formPassword.value, mountPath: formMountPath.value }
        : c
    )
  } else {
    const newConfig: WebDAVConfig = {
      id: Date.now().toString(),
      name: formName.value,
      url: formUrl.value,
      username: formUsername.value,
      password: formPassword.value,
      mountPath: formMountPath.value,
    }
    updated = [...webdavConfigs.value, newConfig]
  }
  saveWebDAVConfigs(updated)
  webdavConfigs.value = updated
  showWebdavModal.value = false
  showToast({ message: t('webdav.configSaved'), duration: 1500, color: 'success' })
}

async function testConfig(config: WebDAVConfig) {
  try {
    await testWebDAVConnection({ name: config.name, url: config.url, username: config.username, password: config.password, mountPath: config.mountPath })
    showToast({ message: t('webdav.connectionSuccess'), duration: 2000, color: 'success' })
  } catch (e) {
    const detail = e instanceof Error ? e.message : String(e)
    showToast({ message: t('webdav.connectionFailed') + ': ' + detail, duration: 3000, color: 'danger' })
  }
}

async function testConnection() {
  if (!formUrl.value) return
  testing.value = true
  try {
    await testWebDAVConnection({ name: formName.value, url: formUrl.value, username: formUsername.value, password: formPassword.value, mountPath: formMountPath.value })
    testing.value = false
    showToast({ message: t('webdav.connectionSuccess'), duration: 2000, color: 'success' })
  } catch (e) {
    testing.value = false
    const detail = e instanceof Error ? e.message : String(e)
    showToast({ message: t('webdav.connectionFailed') + ': ' + detail, duration: 3000, color: 'danger' })
  }
}

function deleteConfig(id: string) {
  const updated = webdavConfigs.value.filter(c => c.id !== id)
  saveWebDAVConfigs(updated)
  webdavConfigs.value = updated
}

function validateSiteId() {
  const val = formSiteId.value.trim()
  if (!val) {
    formSiteIdError.value = t('tasks.pathRequired')
  } else if (!/^[a-zA-Z0-9_]+$/.test(val)) {
    formSiteIdError.value = t('remote.siteIdInvalid')
  } else {
    formSiteIdError.value = ''
  }
}

function validateHost() {
  const val = formHost.value.trim()
  if (!val) {
    formHostError.value = t('tasks.pathRequired')
  } else {
    formHostError.value = ''
  }
}

function openNewSite() {
  editingSiteId.value = ''
  formSiteId.value = ''
  formHost.value = ''
  formDescription.value = ''
  formSiteIdError.value = ''
  formHostError.value = ''
  showSiteModal.value = true
}

function editSite(key: string) {
  editingSiteId.value = key
  formSiteId.value = key
  formHost.value = openlistSites.value[key]?.host || ''
  formDescription.value = openlistSites.value[key]?.description || ''
  formSiteIdError.value = ''
  formHostError.value = ''
  showSiteModal.value = true
}

async function saveSite() {
  if (!formSiteId.value || !formHost.value) return
  try {
    if (editingSiteId.value) {
      await updateOpenlistSite(editingSiteId.value, formHost.value.trim(), formDescription.value.trim())
    } else {
      await addOpenlistSite(formSiteId.value.trim(), formHost.value.trim(), formDescription.value.trim())
    }
    showSiteModal.value = false
    showToast({ message: t('webdav.configSaved'), duration: 1500, color: 'success' })
    await loadRemoteInfo()
  } catch (e) {
    const detail = e instanceof Error ? e.message : String(e)
    showToast({ message: detail, duration: 3000, color: 'danger' })
  }
}

async function handleDeleteSite(key: string) {
  try {
    await deleteOpenlistSite(key)
    showToast({ message: t('webdav.configSaved'), duration: 1500, color: 'success' })
    await loadRemoteInfo()
  } catch (e) {
    const detail = e instanceof Error ? e.message : String(e)
    showToast({ message: detail, duration: 3000, color: 'danger' })
  }
}

async function copyProxyUrl(url: string) {
  try {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(url)
    }
    showToast({ message: t('remote.copied'), duration: 1500, color: 'success' })
  } catch {
    showToast({ message: url, duration: 3000, color: 'medium' })
  }
}

onMounted(() => {
  loadConfigs()
  loadRemoteInfo()
})
</script>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 50%;
  padding: 24px;
  text-align: center;
  color: var(--encv-text-secondary);
}

.empty-icon {
  font-size: 64px;
  margin-bottom: 16px;
  opacity: 0.5;
}

.built-in-item {
  --background: rgba(var(--ion-color-primary-rgb), 0.05);
}

.proxy-url {
  font-size: 12px;
  color: var(--ion-color-primary);
}
</style>
