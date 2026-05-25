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
            <p>{{ appVersion }}</p>
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

      <div v-if="buildInfoLoading" class="build-info-loading">
        <ion-spinner name="crescent"></ion-spinner>
      </div>
      <div v-else-if="buildInfoError" class="build-info-error">
        <ion-icon :icon="warningOutline" color="warning"></ion-icon>
        <span>{{ t('about.failedToLoad') }}</span>
      </div>
      <template v-else>
        <ion-list>
          <ion-list-header>
            <ion-label>{{ t('about.nativeEngine') }}</ion-label>
          </ion-list-header>
          <ion-item v-if="buildInfo">
            <ion-icon :icon="videocamOutline" slot="start" class="lib-icon ffmpeg-icon"></ion-icon>
            <ion-label>
              <div class="lib-title-row">
                <h3 class="lib-name">FFmpeg</h3>
                <ion-badge color="medium" class="lib-badge version-badge">{{ buildInfo.ffmpeg_version }}</ion-badge>
                <ion-badge color="danger" class="lib-badge license-badge">{{ buildInfo.ffmpeg_license }}</ion-badge>
              </div>
              <p class="lib-desc">{{ t('about.ffmpegDesc') }}</p>
            </ion-label>
          </ion-item>
          <ion-item v-if="buildInfo">
            <ion-icon :icon="hardwareChipOutline" slot="start" class="lib-icon x264-icon"></ion-icon>
            <ion-label>
              <div class="lib-title-row">
                <h3 class="lib-name">x264</h3>
                <ion-badge color="medium" class="lib-badge version-badge">{{ buildInfo.x264_version }}</ion-badge>
                <ion-badge color="danger" class="lib-badge license-badge">{{ buildInfo.x264_license }}</ion-badge>
              </div>
              <p class="lib-desc">{{ t('about.x264Desc') }}</p>
            </ion-label>
          </ion-item>
        </ion-list>

        <ion-list>
          <ion-list-header>
            <ion-label>{{ t('about.backendLibs') }}</ion-label>
          </ion-list-header>
          <ion-item>
            <ion-icon :icon="serverOutline" slot="start" class="lib-icon go-icon"></ion-icon>
            <ion-label>
              <div class="lib-title-row">
                <h3 class="lib-name">Go</h3>
                <ion-badge color="medium" class="lib-badge version-badge">{{ goVersion }}</ion-badge>
                <ion-badge color="tertiary" class="lib-badge license-badge">BSD-3-Clause</ion-badge>
              </div>
              <p class="lib-desc">{{ t('about.goRuntimeDesc') }}</p>
            </ion-label>
          </ion-item>
          <ion-item>
            <ion-icon :icon="globeOutline" slot="start"></ion-icon>
            <ion-label>
              <div class="lib-title-row">
                <h3 class="lib-name">Gin</h3>
                <ion-badge color="medium" class="lib-badge version-badge">v1.12.0</ion-badge>
                <ion-badge color="tertiary" class="lib-badge license-badge">MIT</ion-badge>
              </div>
              <p class="lib-desc">{{ t('about.ginDesc') }}</p>
            </ion-label>
          </ion-item>
          <ion-item>
            <ion-icon :icon="terminalOutline" slot="start"></ion-icon>
            <ion-label>
              <div class="lib-title-row">
                <h3 class="lib-name">Cobra</h3>
                <ion-badge color="medium" class="lib-badge version-badge">v1.10.2</ion-badge>
                <ion-badge color="tertiary" class="lib-badge license-badge">Apache-2.0</ion-badge>
              </div>
              <p class="lib-desc">{{ t('about.cobraDesc') }}</p>
            </ion-label>
          </ion-item>
          <ion-item>
            <ion-icon :icon="filmOutline" slot="start"></ion-icon>
            <ion-label>
              <div class="lib-title-row">
                <h3 class="lib-name">go-mp4</h3>
                <ion-badge color="medium" class="lib-badge version-badge">v1.4.1</ion-badge>
                <ion-badge color="tertiary" class="lib-badge license-badge">MIT</ion-badge>
              </div>
              <p class="lib-desc">{{ t('about.goMp4Desc') }}</p>
            </ion-label>
          </ion-item>
          <ion-item>
            <ion-icon :icon="imagesOutline" slot="start"></ion-icon>
            <ion-label>
              <div class="lib-title-row">
                <h3 class="lib-name">go-exif</h3>
                <ion-badge color="medium" class="lib-badge version-badge">v3.0.1</ion-badge>
                <ion-badge color="tertiary" class="lib-badge license-badge">MIT</ion-badge>
              </div>
              <p class="lib-desc">{{ t('about.goExifDesc') }}</p>
            </ion-label>
          </ion-item>
          <ion-item>
            <ion-icon :icon="eyeOutline" slot="start"></ion-icon>
            <ion-label>
              <div class="lib-title-row">
                <h3 class="lib-name">fsnotify</h3>
                <ion-badge color="medium" class="lib-badge version-badge">v1.9.0</ion-badge>
                <ion-badge color="tertiary" class="lib-badge license-badge">BSD-3-Clause</ion-badge>
              </div>
              <p class="lib-desc">{{ t('about.fsnotifyDesc') }}</p>
            </ion-label>
          </ion-item>
          <ion-item>
            <ion-icon :icon="swapHorizontalOutline" slot="start"></ion-icon>
            <ion-label>
              <div class="lib-title-row">
                <h3 class="lib-name">gorilla/websocket</h3>
                <ion-badge color="medium" class="lib-badge version-badge">v1.5.3</ion-badge>
                <ion-badge color="tertiary" class="lib-badge license-badge">BSD-2-Clause</ion-badge>
              </div>
              <p class="lib-desc">{{ t('about.websocketDesc') }}</p>
            </ion-label>
          </ion-item>
          <ion-item>
            <ion-icon :icon="speedometerOutline" slot="start"></ion-icon>
            <ion-label>
              <div class="lib-title-row">
                <h3 class="lib-name">go-humanize</h3>
                <ion-badge color="medium" class="lib-badge version-badge">v1.0.1</ion-badge>
                <ion-badge color="tertiary" class="lib-badge license-badge">MIT</ion-badge>
              </div>
              <p class="lib-desc">{{ t('about.humanizeDesc') }}</p>
            </ion-label>
          </ion-item>
        </ion-list>

        <ion-list>
          <ion-list-header>
            <ion-label>{{ t('about.frontendLibs') }}</ion-label>
          </ion-list-header>
          <ion-item>
            <ion-icon :icon="logoVimeo" slot="start" class="lib-icon vue-icon"></ion-icon>
            <ion-label>
              <div class="lib-title-row">
                <h3 class="lib-name">Vue</h3>
                <ion-badge color="medium" class="lib-badge version-badge">v3.5</ion-badge>
                <ion-badge color="tertiary" class="lib-badge license-badge">MIT</ion-badge>
              </div>
              <p class="lib-desc">{{ t('about.vueDesc') }}</p>
            </ion-label>
          </ion-item>
          <ion-item>
            <ion-icon :icon="logoIonic" slot="start" class="lib-icon ionic-icon"></ion-icon>
            <ion-label>
              <div class="lib-title-row">
                <h3 class="lib-name">Ionic</h3>
                <ion-badge color="medium" class="lib-badge version-badge">v8.8</ion-badge>
                <ion-badge color="tertiary" class="lib-badge license-badge">MIT</ion-badge>
              </div>
              <p class="lib-desc">{{ t('about.ionicDesc') }}</p>
            </ion-label>
          </ion-item>
          <ion-item>
            <ion-icon :icon="capacitorIcon" slot="start" class="lib-icon capacitor-icon"></ion-icon>
            <ion-label>
              <div class="lib-title-row">
                <h3 class="lib-name">Capacitor</h3>
                <ion-badge color="medium" class="lib-badge version-badge">v8.3</ion-badge>
                <ion-badge color="tertiary" class="lib-badge license-badge">MIT</ion-badge>
              </div>
              <p class="lib-desc">{{ t('about.capacitorDesc') }}</p>
            </ion-label>
          </ion-item>
          <ion-item>
            <ion-icon :icon="playCircleOutline" slot="start"></ion-icon>
            <ion-label>
              <div class="lib-title-row">
                <h3 class="lib-name">Artplayer</h3>
                <ion-badge color="medium" class="lib-badge version-badge">v5.4</ion-badge>
                <ion-badge color="tertiary" class="lib-badge license-badge">MIT</ion-badge>
              </div>
              <p class="lib-desc">{{ t('about.artplayerDesc') }}</p>
            </ion-label>
          </ion-item>
          <ion-item>
            <ion-icon :icon="gitBranchOutline" slot="start"></ion-icon>
            <ion-label>
              <div class="lib-title-row">
                <h3 class="lib-name">vue-router</h3>
                <ion-badge color="medium" class="lib-badge version-badge">v4.6</ion-badge>
                <ion-badge color="tertiary" class="lib-badge license-badge">MIT</ion-badge>
              </div>
              <p class="lib-desc">{{ t('about.vueRouterDesc') }}</p>
            </ion-label>
          </ion-item>
        </ion-list>
      </template>

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
  IonBadge, IonSpinner, alertController,
} from '@ionic/vue'
import {
  informationCircle, codeSlash, logoGithub, openOutline,
  trash, refreshCircle, videocamOutline, hardwareChipOutline,
  serverOutline, warningOutline, globeOutline, terminalOutline,
  filmOutline, imagesOutline, eyeOutline, swapHorizontalOutline,
  speedometerOutline, playCircleOutline, gitBranchOutline,
  logoVimeo,
} from 'ionicons/icons'
import { useTheme } from '@/composables/useTheme'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import { fetchBuildInfo, type BuildInfo } from '@/api/encv'
import { ref, onMounted } from 'vue'

const capacitorIcon = terminalOutline
const logoIonic = globeOutline

const { isDark, toggleDark } = useTheme()
const { t } = useI18n()
const serverUrl = ref('http://127.0.0.1:2025')

const buildInfo = ref<BuildInfo | null>(null)
const buildInfoLoading = ref(true)
const buildInfoError = ref(false)
const appVersion = ref('v1.0.0')
const goVersion = ref('go1.x')

onMounted(async () => {
  try {
    const info = await fetchBuildInfo()
    buildInfo.value = info
    if (info.app_version) {
      appVersion.value = info.app_version
    }
  } catch {
    buildInfoError.value = true
  } finally {
    buildInfoLoading.value = false
  }
})

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

<style scoped>
.lib-icon {
  font-size: 24px;
  margin-right: 8px;
}

.ffmpeg-icon {
  color: var(--ion-color-primary);
}

.x264-icon {
  color: var(--ion-color-danger);
}

.go-icon {
  color: var(--ion-color-tertiary);
}

.vue-icon {
  color: #42b883;
}

.ionic-icon {
  color: var(--ion-color-primary);
}

.capacitor-icon {
  color: var(--ion-color-primary);
}

.lib-title-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.lib-name {
  margin: 0;
  font-weight: 600;
}

.lib-desc {
  margin: 2px 0 0;
  font-size: 12px;
  opacity: 0.6;
}

.lib-badge {
  font-size: 10px;
  font-weight: 600;
  --padding-start: 6px;
  --padding-end: 6px;
  --padding-top: 2px;
  --padding-bottom: 2px;
}

.license-badge {
  --background: rgba(var(--ion-color-danger-rgb), 0.12);
  --color: var(--ion-color-danger);
}

.version-badge {
  --background: rgba(var(--ion-color-medium-rgb), 0.12);
  --color: var(--ion-color-medium-shade);
}

.build-info-loading {
  display: flex;
  justify-content: center;
  padding: 24px;
}

.build-info-error {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  font-size: 13px;
  opacity: 0.6;
}
</style>
