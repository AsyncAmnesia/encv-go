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

      <ion-list>
        <ion-list-header>
          <ion-label>{{ t('about.thirdPartyLibs') }}</ion-label>
        </ion-list-header>

        <div v-if="buildInfoLoading" class="build-info-loading">
          <ion-spinner name="crescent"></ion-spinner>
        </div>
        <div v-else-if="buildInfoError" class="build-info-error">
          <ion-icon :icon="warningOutline" color="warning"></ion-icon>
          <span>{{ t('about.failedToLoad') }}</span>
        </div>
        <template v-else-if="buildInfo">

          <ion-accordion-group>
            <ion-accordion value="ffmpeg">
              <ion-item slot="header" class="lib-header-item">
                <ion-icon :icon="videocamOutline" slot="start" class="lib-icon ffmpeg-icon"></ion-icon>
                <ion-label>
                  <div class="lib-title-row">
                    <h3 class="lib-name">FFmpeg</h3>
                    <ion-badge color="medium" class="lib-badge version-badge">{{ buildInfo.ffmpeg_version }}</ion-badge>
                    <ion-badge color="danger" class="lib-badge license-badge">{{ buildInfo.ffmpeg_license }}</ion-badge>
                    <ion-badge color="primary" class="lib-badge arch-badge">{{ buildInfo.abi }}</ion-badge>
                  </div>
                  <p class="lib-desc">{{ t('about.ffmpegDesc') }}</p>
                </ion-label>
              </ion-item>
              <div class="accordion-content" slot="content">
                <div class="build-section">
                  <h4 class="section-title">{{ t('about.buildConfig') }}</h4>
                  <div class="config-grid">
                    <div class="config-item">
                      <span class="config-label">{{ t('about.ndkVersion') }}</span>
                      <span class="config-value mono">{{ buildInfo.ndk_version }}</span>
                    </div>
                    <div class="config-item">
                      <span class="config-label">{{ t('about.apiLevel') }}</span>
                      <span class="config-value mono">{{ buildInfo.api_level }}</span>
                    </div>
                    <div class="config-item">
                      <span class="config-label">{{ t('about.linking') }}</span>
                      <span class="config-value">{{ t('about.staticLinking') }}</span>
                    </div>
                    <div class="config-item">
                      <span class="config-label">{{ t('about.cflags') }}</span>
                      <span class="config-value mono cflags-value">{{ buildInfo.cflags }}</span>
                    </div>
                    <div class="config-item">
                      <span class="config-label">{{ t('about.buildDate') }}</span>
                      <span class="config-value mono">{{ formatDate(buildInfo.build_date) }}</span>
                    </div>
                  </div>
                </div>

                <div class="build-section">
                  <h4 class="section-title">{{ t('about.decoders') }}</h4>
                  <div class="tag-list">
                    <span v-for="d in buildInfo.enabled_decoders" :key="d" class="tech-tag decoder-tag">{{ d }}</span>
                  </div>
                </div>

                <div class="build-section">
                  <h4 class="section-title">{{ t('about.encoders') }}</h4>
                  <div class="tag-list">
                    <span v-for="e in buildInfo.enabled_encoders" :key="e" class="tech-tag encoder-tag">{{ e }}</span>
                  </div>
                </div>

                <div class="build-section">
                  <h4 class="section-title">{{ t('about.muxers') }}</h4>
                  <div class="tag-list">
                    <span v-for="m in buildInfo.enabled_muxers" :key="m" class="tech-tag muxer-tag">{{ m }}</span>
                  </div>
                </div>

                <div class="build-section">
                  <h4 class="section-title">{{ t('about.demuxers') }}</h4>
                  <div class="tag-list">
                    <span v-for="d in buildInfo.enabled_demuxers" :key="d" class="tech-tag demuxer-tag">{{ d }}</span>
                  </div>
                </div>

                <div class="build-section">
                  <h4 class="section-title">{{ t('about.parsers') }}</h4>
                  <div class="tag-list">
                    <span v-for="p in buildInfo.enabled_parsers" :key="p" class="tech-tag parser-tag">{{ p }}</span>
                  </div>
                </div>

                <div class="build-section">
                  <h4 class="section-title">{{ t('about.protocols') }}</h4>
                  <div class="tag-list">
                    <span v-for="p in buildInfo.enabled_protocols" :key="p" class="tech-tag protocol-tag">{{ p }}</span>
                  </div>
                </div>

                <div v-if="buildInfo.enabled_filters && buildInfo.enabled_filters.length > 0" class="build-section">
                  <h4 class="section-title">{{ t('about.filters') }}</h4>
                  <div class="tag-list">
                    <span v-for="f in buildInfo.enabled_filters" :key="f" class="tech-tag filter-tag">{{ f }}</span>
                  </div>
                </div>

                <div class="build-section">
                  <h4 class="section-title">Static Libraries</h4>
                  <div class="tag-list">
                    <span v-for="lib in buildInfo.static_libs" :key="lib" class="tech-tag lib-tag">{{ lib }}</span>
                  </div>
                </div>
              </div>
            </ion-accordion>

            <ion-accordion value="x264">
              <ion-item slot="header" class="lib-header-item">
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
              <div class="accordion-content" slot="content">
                <div class="build-section">
                  <h4 class="section-title">{{ t('about.configureOpts') }}</h4>
                  <div class="config-grid">
                    <div class="config-item full-width">
                      <span class="config-value mono cflags-value">{{ buildInfo.x264_configure_opts }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </ion-accordion>

            <ion-accordion value="go">
              <ion-item slot="header" class="lib-header-item">
                <ion-icon :icon="serverOutline" slot="start" class="lib-icon go-icon"></ion-icon>
                <ion-label>
                  <div class="lib-title-row">
                    <h3 class="lib-name">{{ t('about.goRuntime') }}</h3>
                    <ion-badge color="tertiary" class="lib-badge license-badge">BSD</ion-badge>
                  </div>
                  <p class="lib-desc">{{ t('about.goRuntimeDesc') }}</p>
                </ion-label>
              </ion-item>
              <div class="accordion-content" slot="content">
                <div class="build-section">
                  <div class="config-grid">
                    <div class="config-item">
                      <span class="config-label">Version</span>
                      <span class="config-value mono">{{ goVersion }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </ion-accordion>
          </ion-accordion-group>

        </template>
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
  IonAccordion, IonAccordionGroup, IonBadge, IonSpinner,
  alertController,
} from '@ionic/vue'
import {
  informationCircle, codeSlash, logoGithub, openOutline,
  trash, refreshCircle, videocamOutline, hardwareChipOutline,
  serverOutline, warningOutline,
} from 'ionicons/icons'
import { useTheme } from '@/composables/useTheme'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import { fetchBuildInfo, type BuildInfo } from '@/api/encv'
import { ref, onMounted } from 'vue'

const { isDark, toggleDark } = useTheme()
const { t } = useI18n()
const serverUrl = ref('http://127.0.0.1:2025')

const buildInfo = ref<BuildInfo | null>(null)
const buildInfoLoading = ref(true)
const buildInfoError = ref(false)
const appVersion = ref('v1.0.0')
const goVersion = ref('go1.x')

function formatDate(dateStr: string): string {
  if (!dateStr) return ''
  try {
    const d = new Date(dateStr)
    return d.toLocaleDateString() + ' ' + d.toLocaleTimeString()
  } catch {
    return dateStr
  }
}

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
.lib-header-item {
  --padding-start: 12px;
}

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

.arch-badge {
  --background: rgba(var(--ion-color-primary-rgb), 0.12);
  --color: var(--ion-color-primary);
}

.version-badge {
  --background: rgba(var(--ion-color-medium-rgb), 0.12);
  --color: var(--ion-color-medium-shade);
}

.accordion-content {
  padding: 8px 16px 16px;
}

.build-section {
  margin-bottom: 14px;
}

.build-section:last-child {
  margin-bottom: 4px;
}

.section-title {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  opacity: 0.55;
  margin: 0 0 8px;
}

.config-grid {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.config-item {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 12px;
}

.config-item.full-width {
  flex-direction: column;
  gap: 2px;
}

.config-label {
  font-size: 13px;
  opacity: 0.7;
  white-space: nowrap;
  flex-shrink: 0;
}

.config-value {
  font-size: 13px;
  text-align: right;
  word-break: break-all;
}

.config-value.mono {
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', 'JetBrains Mono', monospace;
  font-size: 12px;
}

.cflags-value {
  font-size: 11px;
  line-height: 1.4;
  opacity: 0.8;
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.tech-tag {
  font-size: 11px;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', 'JetBrains Mono', monospace;
  padding: 2px 8px;
  border-radius: 4px;
  white-space: nowrap;
}

.decoder-tag {
  background: rgba(var(--ion-color-success-rgb), 0.1);
  color: var(--ion-color-success-shade);
}

.encoder-tag {
  background: rgba(var(--ion-color-primary-rgb), 0.1);
  color: var(--ion-color-primary-shade);
}

.muxer-tag {
  background: rgba(var(--ion-color-warning-rgb), 0.1);
  color: var(--ion-color-warning-shade);
}

.demuxer-tag {
  background: rgba(var(--ion-color-tertiary-rgb), 0.1);
  color: var(--ion-color-tertiary-shade);
}

.parser-tag {
  background: rgba(var(--ion-color-medium-rgb), 0.1);
  color: var(--ion-color-medium-shade);
}

.protocol-tag {
  background: rgba(var(--ion-color-danger-rgb), 0.08);
  color: var(--ion-color-danger-shade);
}

.filter-tag {
  background: rgba(var(--ion-color-success-rgb), 0.08);
  color: var(--ion-color-success-shade);
}

.lib-tag {
  background: rgba(var(--ion-color-medium-rgb), 0.08);
  color: var(--ion-color-medium);
  font-size: 10px;
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
