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
          <ion-label>{{ t('settings.logSettings') }}</ion-label>
        </ion-list-header>
        <ion-item>
          <ion-icon :icon="terminal" slot="start"></ion-icon>
          <ion-select
            :value="logLevel"
            label="日志级别"
            label-placement="stacked"
            interface="action-sheet"
            mode="ios"
            @ionChange="handleLogLevelChange($event.detail.value)"
          >
            <ion-select-option value="debug">DEBUG</ion-select-option>
            <ion-select-option value="info">INFO</ion-select-option>
            <ion-select-option value="warn">WARN</ion-select-option>
            <ion-select-option value="error">ERROR</ion-select-option>
          </ion-select>
        </ion-item>
        <ion-item>
          <ion-icon :icon="documentText" slot="start"></ion-icon>
          <ion-input
            :value="logFile"
            label="日志文件路径"
            label-placement="stacked"
            placeholder="留空则只输出到控制台"
            @ionChange="handleLogFileChange($event.detail.value ?? '')"
          ></ion-input>
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
                <ion-icon :icon="iconMap[proto.icon]" class="proto-icon"></ion-icon>
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
import { computed } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons, IonBackButton,
  IonContent, IonList, IonListHeader, IonItem, IonIcon, IonLabel, IonToggle,
  IonSelect, IonSelectOption, IonInput,
  alertController,
} from '@ionic/vue'
import {
  bugOutline, downloadOutline, readerOutline, trashOutline,
  chevronForward, playCircleOutline, musicalNotesOutline,
  colorPaletteOutline, settingsOutline, terminal, documentText,
} from 'ionicons/icons'
import { useRouter } from 'vue-router'
import { useI18n } from '@/composables/useI18n'
import { useDevTools } from '@/composables/useDevTools'
import { useConfig } from '@/composables/useConfig'
import { showToast } from '@/composables/useToast'
import { isNative, exportLogs, clearLogs, openLogViewer, saveDevLogs } from '@/plugins/GoProcess'
import { getFrontendLogsJson } from '@/composables/useFrontendLogs'
import { getAllPrototypes } from './prototypes/registry'

const { t } = useI18n()
const router = useRouter()
const { vconsoleEnabled, toggleVConsole } = useDevTools()
const { getFieldValue, setFieldValue, saveConfig } = useConfig()

const logLevel = computed(() => String(getFieldValue(['log', 'level']) ?? 'info'))
const logFile = computed(() => String(getFieldValue(['log', 'file']) ?? ''))

async function handleLogLevelChange(value: string) {
  setFieldValue(['log', 'level'], value)
  await saveLogConfig()
}

async function handleLogFileChange(value: string) {
  setFieldValue(['log', 'file'], value)
  await saveLogConfig()
}

async function saveLogConfig() {
  try {
    await saveConfig()
    showToast({ message: t('settings.configSaved'), duration: 1500, color: 'success' })
  } catch (e) {
    const detail = e instanceof Error ? e.message : String(e)
    showToast({ message: t('settings.configSaveFailed') + ': ' + detail, duration: 3000, color: 'danger' })
  }
}

const prototypes = getAllPrototypes()

const iconMap: Record<string, string> = {
  'play-circle': playCircleOutline,
  'settings': settingsOutline,
  'musical-notes': musicalNotesOutline,
  'color-palette': colorPaletteOutline,
}

function handlePrototypeClick(proto: typeof prototypes[0]) {
  router.push(`/tabs/settings/devtools/prototype/${proto.id}`)
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
