<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>Settings</ion-title>
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
          <ion-label>Server</ion-label>
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
  IonContent,
  IonList,
  IonListHeader,
  IonItem,
  IonIcon,
  IonLabel,
  IonToggle,
  IonInput,
  IonBadge,
  IonButton,
  alertController,
  toastController,
} from '@ionic/vue'
import {
  moon,
  server,
  refresh,
  informationCircle,
  codeSlash,
  logoGithub,
  openOutline,
  trash,
  refreshCircle,
} from 'ionicons/icons'
import { useTheme } from '@/composables/useTheme'
import { useServerStatus } from '@/composables/useServerStatus'
import { setApiBaseUrl, getServerUrl } from '@/api/encv'

const { isDark, toggleDark } = useTheme()
const { isOnline: serverOnline, checkStatus } = useServerStatus()

const serverUrl = ref(getServerUrl())

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

onMounted(() => {
  checkStatus()
})
</script>
