<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>{{ t('webdav.title') }}</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <div v-if="configs.length === 0" class="empty-state">
        <ion-icon :icon="cloud" class="empty-icon"></ion-icon>
        <h3>{{ t('webdav.noServers') }}</h3>
        <p>{{ t('webdav.noServersDesc') }}</p>
      </div>

      <ion-list v-else>
        <ion-item-sliding v-for="config in configs" :key="config.id">
          <ion-item @click="editConfig(config)">
            <ion-icon :icon="cloud" color="primary" slot="start"></ion-icon>
            <ion-label>
              <h2>{{ config.name }}</h2>
              <p>{{ config.url }}</p>
              <p v-if="config.mountPath">{{ t('webdav.mount') }}: {{ config.mountPath }}</p>
            </ion-label>
            <ion-badge :color="config.id === testingId ? 'warning' : 'medium'" slot="end">
              {{ config.id === testingId ? t('webdav.testing') : t('webdav.saved') }}
            </ion-badge>
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

      <ion-modal :is-open="showModal" @didDismiss="showModal = false">
        <ion-header>
          <ion-toolbar>
            <ion-title>{{ editingId ? t('webdav.edit') : t('webdav.add') }} {{ t('webdav.title') }}</ion-title>
            <ion-buttons slot="end">
              <ion-button @click="showModal = false">{{ t('settings.cancel') }}</ion-button>
            </ion-buttons>
          </ion-toolbar>
        </ion-header>
        <ion-content class="ion-padding">
          <ion-list>
            <ion-item>
              <ion-input
                v-model="formName"
                :label="t('webdav.name')"
                label-placement="stacked"
                placeholder="My WebDAV Server"
              ></ion-input>
            </ion-item>
            <ion-item>
              <ion-input
                v-model="formUrl"
                :label="t('webdav.serverUrl')"
                label-placement="stacked"
                placeholder="https://dav.example.com"
              ></ion-input>
            </ion-item>
            <ion-item>
              <ion-input
                v-model="formUsername"
                :label="t('webdav.username')"
                label-placement="stacked"
                placeholder="user"
              ></ion-input>
            </ion-item>
            <ion-item>
              <ion-input
                v-model="formPassword"
                :type="showPassword ? 'text' : 'password'"
                :label="t('webdav.password')"
                label-placement="stacked"
                placeholder="password"
              ></ion-input>
              <ion-button fill="clear" slot="end" @click="showPassword = !showPassword">
                <ion-icon :icon="showPassword ? eyeOff : eye"></ion-icon>
              </ion-button>
            </ion-item>
            <ion-item>
              <ion-input
                v-model="formMountPath"
                :label="t('webdav.mountPath')"
                label-placement="stacked"
                placeholder="/webdav"
              ></ion-input>
            </ion-item>
          </ion-list>

          <ion-button expand="block" @click="testConnection" :disabled="testing || !formUrl">
            <ion-icon :icon="flash" slot="start"></ion-icon>
            {{ testing ? t('webdav.testing') : t('webdav.testConnection') }}
          </ion-button>

          <ion-button expand="block" class="ion-margin-top" @click="saveConfig" :disabled="!formName || !formUrl">
            <ion-icon :icon="save" slot="start"></ion-icon>
            {{ t('webdav.save') }}
          </ion-button>
        </ion-content>
      </ion-modal>
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
  IonContent,
  IonList,
  IonItem,
  IonItemSliding,
  IonItemOptions,
  IonItemOption,
  IonIcon,
  IonLabel,
  IonBadge,
  IonFab,
  IonFabButton,
  IonModal,
  IonButtons,
  IonButton,
  IonInput,
  toastController,
} from '@ionic/vue'
import {
  add,
  cloud,
  flash,
  save,
  eye,
  eyeOff,
} from 'ionicons/icons'
import {
  getWebDAVConfigs,
  saveWebDAVConfigs,
  testWebDAVConnection,
} from '@/api/encv'
import type { WebDAVConfig } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'

const { t } = useI18n()

const configs = ref<WebDAVConfig[]>([])
const showModal = ref(false)
const editingId = ref('')
const testing = ref(false)
const testingId = ref('')
const showPassword = ref(false)

const formName = ref('')
const formUrl = ref('')
const formUsername = ref('')
const formPassword = ref('')
const formMountPath = ref('')

function loadConfigs() {
  configs.value = getWebDAVConfigs()
}

function openNewConfig() {
  editingId.value = ''
  formName.value = ''
  formUrl.value = ''
  formUsername.value = ''
  formPassword.value = ''
  formMountPath.value = '/webdav'
  showModal.value = true
}

function editConfig(config: WebDAVConfig) {
  editingId.value = config.id
  formName.value = config.name
  formUrl.value = config.url
  formUsername.value = config.username
  formPassword.value = config.password
  formMountPath.value = config.mountPath
  showModal.value = true
}

function saveConfig() {
  if (!formName.value || !formUrl.value) return

  let updated: WebDAVConfig[]
  if (editingId.value) {
    updated = configs.value.map(c =>
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
    updated = [...configs.value, newConfig]
  }

  saveWebDAVConfigs(updated)
  configs.value = updated
  showModal.value = false

  toastController.create({
    message: t('webdav.configSaved'),
    duration: 1500,
    color: 'success',
  }).then(t => t.present())
}

async function testConfig(config: WebDAVConfig) {
  testingId.value = config.id
  const success = await testWebDAVConnection({
    name: config.name,
    url: config.url,
    username: config.username,
    password: config.password,
    mountPath: config.mountPath,
  })
  testingId.value = ''

  const toast = await toastController.create({
    message: success ? t('webdav.connectionSuccess') : t('webdav.connectionFailed'),
    duration: 2000,
    color: success ? 'success' : 'danger',
  })
  await toast.present()
}

async function testConnection() {
  if (!formUrl.value) return
  testing.value = true
  const success = await testWebDAVConnection({
    name: formName.value,
    url: formUrl.value,
    username: formUsername.value,
    password: formPassword.value,
    mountPath: formMountPath.value,
  })
  testing.value = false

  const toast = await toastController.create({
    message: success ? t('webdav.connectionSuccess') : t('webdav.connectionFailed'),
    duration: 2000,
    color: success ? 'success' : 'danger',
  })
  await toast.present()
}

function deleteConfig(id: string) {
  const updated = configs.value.filter(c => c.id !== id)
  saveWebDAVConfigs(updated)
  configs.value = updated
}

onMounted(() => {
  loadConfigs()
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
</style>
