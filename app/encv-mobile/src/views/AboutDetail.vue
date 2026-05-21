<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button default-href="/tabs/settings"></ion-back-button>
        </ion-buttons>
        <ion-title>{{ t('settings.about') }}</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content>
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
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons, IonBackButton,
  IonContent, IonList, IonListHeader, IonItem, IonIcon, IonLabel,
  alertController,
} from '@ionic/vue'
import {
  informationCircle, codeSlash, logoGithub, openOutline,
  trash, refreshCircle,
} from 'ionicons/icons'
import { useTheme } from '@/composables/useTheme'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import { ref } from 'vue'

const { isDark, toggleDark } = useTheme()
const { t } = useI18n()
const serverUrl = ref('http://127.0.0.1:2025')

function openGitHub() {
  window.open('https://github.com/Soltus/encv-go', '_blank')
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
          showToast({
            message: t('settings.cacheCleared'),
            duration: 1500,
            color: 'success',
          })
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
          showToast({
            message: t('settings.settingsReset'),
            duration: 1500,
            color: 'success',
          })
        },
      },
    ],
  })
  await alert.present()
}
</script>
