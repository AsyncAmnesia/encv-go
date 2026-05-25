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

          <div v-if="listTestResults[config.id]" class="list-test-result-area" :class="{ 'result-error': !listTestResults[config.id].success, 'result-ok': listTestResults[config.id].success }">
            <div class="result-items">
              <div class="result-item">
                <span class="result-label">{{ listTestResults[config.id].reachable ? t('webdav.reachable') : t('webdav.notReachable') }}</span>
                <ion-badge :color="listTestResults[config.id].reachable ? 'success' : 'danger'" class="mini-badge">
                  {{ listTestResults[config.id].reachable ? 'OK' : 'FAIL' }}
                </ion-badge>
              </div>
              <div class="result-item">
                <span class="result-label">{{ listTestResults[config.id].is_webdav ? t('webdav.isWebDAV') : t('webdav.notWebDAV') }}</span>
                <ion-badge :color="listTestResults[config.id].is_webdav ? 'success' : 'danger'" class="mini-badge">
                  {{ listTestResults[config.id].is_webdav ? 'OK' : 'FAIL' }}
                </ion-badge>
              </div>
              <div v-if="listTestResults[config.id].error" class="result-error-inline">
                {{ listTestResults[config.id].error }}
              </div>
            </div>
          </div>
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

          <div v-if="testResult" class="test-result-area" :class="{ 'result-error': !testResult.success, 'result-ok': testResult.success }">
            <h4 class="result-title">{{ t('webdav.testResult') }}</h4>

            <div class="result-items">
              <div class="result-item">
                <span class="result-label">{{ testResult.reachable ? t('webdav.reachable') : t('webdav.notReachable') }}</span>
                <ion-badge :color="testResult.reachable ? 'success' : 'danger'">
                  {{ testResult.reachable ? 'OK' : 'FAIL' }}
                </ion-badge>
              </div>

              <div class="result-item">
                <span class="result-label">{{ testResult.is_webdav ? t('webdav.isWebDAV') : t('webdav.notWebDAV') }}</span>
                <ion-badge :color="testResult.is_webdav ? 'success' : 'danger'">
                  {{ testResult.is_webdav ? 'OK' : 'FAIL' }}
                </ion-badge>
              </div>

              <div v-if="testResult.is_webdav" class="result-item">
                <span class="result-label">{{ testResult.auth_ok ? t('webdav.authOK') : t('webdav.authFailed') }}</span>
                <ion-badge :color="testResult.auth_ok ? 'success' : 'danger'">
                  {{ testResult.auth_ok ? 'OK' : 'FAIL' }}
                </ion-badge>
              </div>

              <div v-if="testResult.is_webdav && testResult.status_code === 207" class="result-item">
                <span class="result-label">{{ testResult.dir_readable ? t('webdav.dirReadable') : t('webdav.dirNotReadable') }}</span>
                <ion-badge :color="testResult.dir_readable ? 'success' : 'danger'">
                  {{ testResult.dir_readable ? 'OK' : 'FAIL' }}
                </ion-badge>
              </div>

              <div class="result-item">
                <span class="result-label">{{ t('webdav.statusCode') }}</span>
                <span class="result-value">HTTP {{ testResult.status_code }}</span>
              </div>

              <div v-if="testResult.dav_header" class="result-item">
                <span class="result-label">{{ t('webdav.davHeader') }}</span>
                <span class="result-value code-text">{{ testResult.dav_header }}</span>
              </div>
            </div>

            <div v-if="testResult.error" class="result-error-msg">
              <p>{{ t('webdav.testDetail') }}:</p>
              <p class="error-detail">{{ testResult.error }}</p>
            </div>
          </div>

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
import type { WebDAVConfig, WebDAVTestResult } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'

const { t } = useI18n()

const configs = ref<WebDAVConfig[]>([])
const showModal = ref(false)
const editingId = ref('')
const testing = ref(false)
const testingId = ref('')
const showPassword = ref(false)
const testResult = ref<WebDAVTestResult | null>(null)
const listTestResults = ref<Record<string, WebDAVTestResult>>({})

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
  testResult.value = null
  showModal.value = true
}

function editConfig(config: WebDAVConfig) {
  editingId.value = config.id
  formName.value = config.name
  formUrl.value = config.url
  formUsername.value = config.username
  formPassword.value = config.password
  formMountPath.value = config.mountPath
  testResult.value = null
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

  showToast({
    message: t('webdav.configSaved'),
    duration: 1500,
    color: 'success',
  })
}

async function testConfig(config: WebDAVConfig) {
  testingId.value = config.id
  listTestResults.value[config.id] = { success: false, reachable: false, is_webdav: false, auth_ok: false, dir_readable: false, status_code: 0 }
  try {
    const result = await testWebDAVConnection({
      name: config.name,
      url: config.url,
      username: config.username,
      password: config.password,
      mountPath: config.mountPath,
    })
    testingId.value = ''
    listTestResults.value[config.id] = result
  } catch (e) {
    testingId.value = ''
    listTestResults.value[config.id] = {
      success: false,
      reachable: false,
      is_webdav: false,
      auth_ok: false,
      dir_readable: false,
      status_code: 0,
      error: e instanceof Error ? e.message : String(e),
    }
  }
}

async function testConnection() {
  if (!formUrl.value) return
  testing.value = true
  testResult.value = null
  try {
    const result = await testWebDAVConnection({
      name: formName.value,
      url: formUrl.value,
      username: formUsername.value,
      password: formPassword.value,
      mountPath: formMountPath.value,
    })
    testing.value = false
    testResult.value = result
  } catch (e) {
    testing.value = false
    testResult.value = {
      success: false,
      reachable: false,
      is_webdav: false,
      auth_ok: false,
      dir_readable: false,
      status_code: 0,
      error: e instanceof Error ? e.message : String(e),
    }
  }
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

.test-result-area {
  margin-top: 12px;
  padding: 14px;
  border-radius: 8px;
  background: var(--ion-background-color);
}

.result-ok {
  border-left: 3px solid var(--ion-color-success);
}

.result-error {
  border-left: 3px solid var(--ion-color-danger);
}

.result-title {
  margin: 0 0 10px;
  font-size: 15px;
  font-weight: 600;
  color: var(--ion-text-color);
}

.result-items {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.result-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 13px;
}

.result-label {
  color: var(--ion-text-color);
  font-weight: 500;
}

.result-value {
  color: var(--ion-text-secondary);
  font-size: 13px;
  font-weight: 400;
}

.code-text {
  font-family: monospace;
  font-size: 12px;
}

.result-error-msg {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid rgba(var(--ion-color-danger-rgb), 0.15);
}

.result-error-msg p:first-child {
  margin: 0 0 4px;
  font-size: 13px;
  font-weight: 600;
  color: var(--ion-color-danger);
}

.error-detail {
  margin: 0;
  font-size: 13px;
  color: var(--ion-color-medium);
  line-height: 1.5;
  word-break: break-word;
}

.list-test-result-area {
  padding: 10px 14px;
  background: var(--ion-background-color);
}

.list-test-result-area.result-ok {
  border-left: 3px solid var(--ion-color-success);
}

.list-test-result-area.result-error {
  border-left: 3px solid var(--ion-color-danger);
}

.mini-badge {
  font-size: 11px;
  --padding-start: 6px;
  --padding-end: 6px;
}

.result-error-inline {
  color: var(--ion-color-danger);
  font-size: 12px;
  margin-top: 4px;
  word-break: break-word;
}
</style>
