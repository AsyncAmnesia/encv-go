<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button default-href="/tabs/settings"></ion-back-button>
        </ion-buttons>
        <ion-title>{{ t('devtools.title') }}</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <ion-list>
        <ion-list-header>
          <ion-label>{{ t('devtools.debugTools') }}</ion-label>
        </ion-list-header>
        <ion-item>
          <ion-icon :icon="bugOutline" slot="start"></ion-icon>
          <ion-toggle :checked="vconsoleEnabled" @ionChange="handleVConsoleToggle">{{ t('devtools.vconsole') }}</ion-toggle>
        </ion-item>
        <ion-item button @click="handleExportLogs" detail>
          <ion-icon :icon="downloadOutline" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('devtools.exportLogs') }}</h3>
            <p>{{ t('devtools.exportLogsDesc') }}</p>
          </ion-label>
        </ion-item>
        <ion-item button @click="handleOpenLogViewer" detail>
          <ion-icon :icon="readerOutline" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('devtools.openLog') }}</h3>
            <p>{{ t('devtools.openLogDesc') }}</p>
          </ion-label>
        </ion-item>
        <ion-item button @click="handleClearLogs" detail>
          <ion-icon :icon="trashOutline" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('devtools.clearLogs') }}</h3>
            <p>{{ t('devtools.clearLogsDesc') }}</p>
          </ion-label>
        </ion-item>
      </ion-list>

      <ion-list>
        <ion-list-header>
          <ion-label>{{ t('devtools.composePrototypes') }}</ion-label>
        </ion-list-header>
        <p class="section-hint">{{ t('devtools.composePrototypesHint') }}</p>

        <div class="prototype-cards">
          <div
            v-for="proto in prototypes"
            :key="proto.id"
            class="prototype-card"
            @click="handlePrototypeClick(proto)"
          >
            <div class="proto-header">
              <div class="proto-icon-wrap" :style="{ background: proto.accentColor }">
                <ion-icon :icon="proto.icon" class="proto-icon"></ion-icon>
              </div>
              <div class="proto-title-area">
                <h3 class="proto-title">{{ proto.name }}</h3>
                <p class="proto-route">{{ proto.route }}</p>
              </div>
              <ion-icon :icon="chevronForward" class="proto-arrow"></ion-icon>
            </div>
            <div class="proto-compose-path">
              <span class="path-label">Compose</span>
              <code class="path-value">{{ proto.composePath }}</code>
            </div>
            <p class="proto-desc">{{ proto.description }}</p>
          </div>
        </div>
      </ion-list>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons, IonBackButton,
  IonContent, IonList, IonListHeader, IonItem, IonIcon, IonLabel, IonToggle,
  alertController,
} from '@ionic/vue'
import {
  bugOutline, downloadOutline, readerOutline, trashOutline,
  chevronForward, playCircleOutline, musicalNotesOutline,
  colorPaletteOutline, settingsOutline,
} from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import { useDevTools } from '@/composables/useDevTools'
import { showToast } from '@/composables/useToast'
import { isNative, exportLogs, clearLogs, openLogViewer, saveDevLogs } from '@/plugins/GoProcess'
import { getFrontendLogsJson } from '@/composables/useFrontendLogs'
import { Capacitor } from '@capacitor/core'
import { openPlayer } from '@/plugins/GoProcess'

const { t } = useI18n()
const { vconsoleEnabled, toggleVConsole } = useDevTools()

interface ComposePrototype {
  id: string
  name: string
  route: string
  composePath: string
  description: string
  icon: string
  accentColor: string
  action: () => void
}

const prototypes = ref<ComposePrototype[]>([
  {
    id: 'mpv-player-screen',
    name: 'MPV Player Screen',
    route: 'com.encvgo.plugin.mpv.MpvPlayerActivity',
    composePath: 'com.encvgo.plugin.mpv.MpvPlayerScreen',
    description: t('devtools.protoMpvPlayerDesc'),
    icon: playCircleOutline,
    accentColor: 'rgba(139, 92, 246, 0.15)',
    action: () => {
      if (isNative()) {
        openPlayer('', 'MPV UI Preview', 'video/*', 'mpv-plugin')
      }
    },
  },
  {
    id: 'mpv-controls',
    name: 'MPV Controls Overlay',
    route: 'com.encvgo.plugin.mpv.MpvPlayerActivity',
    composePath: 'com.encvgo.plugin.mpv.MpvControls',
    description: t('devtools.protoMpvControlsDesc'),
    icon: settingsOutline,
    accentColor: 'rgba(56, 128, 255, 0.15)',
    action: () => {
      if (isNative()) {
        openPlayer('', 'MPV Controls Preview', 'video/*', 'mpv-plugin')
      }
    },
  },
  {
    id: 'mpv-progress-bar',
    name: 'MPV Progress Bar',
    route: 'com.encvgo.plugin.mpv.MpvPlayerActivity',
    composePath: 'com.encvgo.plugin.mpv.MpvProgressBar',
    description: t('devtools.protoMpvProgressDesc'),
    icon: musicalNotesOutline,
    accentColor: 'rgba(45, 211, 111, 0.15)',
    action: () => {
      if (isNative()) {
        openPlayer('', 'MPV Progress Preview', 'video/*', 'mpv-plugin')
      }
    },
  },
  {
    id: 'mpv-theme',
    name: 'MPV Theme (EncvMpVPlayerTheme)',
    route: 'com.encvgo.plugin.mpv.MpvPlayerActivity',
    composePath: 'com.encvgo.plugin.mpv.theme.EncvMpVPlayerTheme',
    description: t('devtools.protoMpvThemeDesc'),
    icon: colorPaletteOutline,
    accentColor: 'rgba(235, 68, 90, 0.15)',
    action: () => {
      if (isNative()) {
        openPlayer('', 'MPV Theme Preview', 'video/*', 'mpv-plugin')
      }
    },
  },
])

function handlePrototypeClick(proto: ComposePrototype) {
  if (!Capacitor.isNativePlatform()) {
    showToast({ message: t('devtools.prototypeNativeOnly'), duration: 2000, color: 'warning' })
    return
  }
  proto.action()
}

function handleVConsoleToggle(event: CustomEvent) {
  toggleVConsole(event.detail.checked)
}

async function handleExportLogs() {
  if (!isNative()) return
  try {
    await saveDevLogs(getFrontendLogsJson())
    const result = await exportLogs()
    if (result.success) {
      showToast({ message: t('devtools.exportSuccess'), duration: 1500, color: 'success' })
    } else {
      showToast({ message: t('devtools.exportFailed'), duration: 2000, color: 'danger' })
    }
  } catch {
    showToast({ message: t('devtools.exportFailed'), duration: 2000, color: 'danger' })
  }
}

async function handleOpenLogViewer() {
  if (!isNative()) return
  try {
    await openLogViewer()
  } catch {
    showToast({ message: t('devtools.openLogFailed'), duration: 2000, color: 'danger' })
  }
}

async function handleClearLogs() {
  if (!isNative()) return
  const alert = await alertController.create({
    header: t('devtools.clearLogsConfirm'),
    buttons: [
      { text: t('common.cancel'), role: 'cancel' },
      {
        text: t('common.confirm'),
        role: 'confirm',
        handler: async () => {
          const result = await clearLogs()
          if (result.success) {
            showToast({ message: t('devtools.clearSuccess'), duration: 1500, color: 'success' })
          } else {
            showToast({ message: t('devtools.clearFailed'), duration: 2000, color: 'danger' })
          }
        },
      },
    ],
  })
  await alert.present()
}
</script>

<style scoped>
.section-hint {
  font-size: 12px;
  color: var(--encv-text-secondary, #999);
  margin: 0 16px 8px;
  line-height: 1.5;
}

.prototype-cards {
  padding: 0 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.prototype-card {
  background: var(--ion-card-background, var(--ion-item-background, #fff));
  border-radius: 14px;
  padding: 14px 16px;
  cursor: pointer;
  transition: transform 0.15s ease, box-shadow 0.15s ease;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.06);
  border: 1px solid rgba(var(--ion-color-medium-rgb, 128, 128, 128), 0.12);
}

.prototype-card:active {
  transform: scale(0.98);
}

.proto-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.proto-icon-wrap {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.proto-icon {
  font-size: 22px;
  color: var(--ion-text-color, #333);
}

.proto-title-area {
  flex: 1;
  min-width: 0;
}

.proto-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--ion-text-color, #333);
  margin: 0;
}

.proto-route {
  font-size: 11px;
  color: var(--encv-text-secondary, #999);
  margin: 2px 0 0;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.proto-arrow {
  font-size: 18px;
  color: var(--ion-color-medium, #999);
  flex-shrink: 0;
}

.proto-compose-path {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 10px;
  padding: 6px 10px;
  background: rgba(var(--ion-color-medium-rgb, 128, 128, 128), 0.08);
  border-radius: 8px;
}

.path-label {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--ion-color-primary, #3880ff);
  flex-shrink: 0;
}

.path-value {
  font-size: 11px;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  color: var(--ion-text-color, #333);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  background: none;
  padding: 0;
  margin: 0;
}

.proto-desc {
  font-size: 12px;
  color: var(--encv-text-secondary, #999);
  margin: 8px 0 0;
  line-height: 1.4;
}
</style>
