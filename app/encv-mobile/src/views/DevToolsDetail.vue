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

      <!-- 沙箱预览：dev 专属入口，生产构建整段 v-if false 移除 -->
      <ion-list v-if="isDev">
        <ion-list-header>
          <ion-label>{{ t('devtools.sandboxPreview') }}</ion-label>
          <ion-badge slot="end" color="warning" class="scope-badge scope-dev">
            <ion-icon :icon="bugOutline" class="scope-badge-icon"></ion-icon>
            <span class="scope-text">DEV</span>
          </ion-badge>
        </ion-list-header>
        <p class="section-hint">{{ t('devtools.sandboxPreviewHint') }}</p>
        <ion-item button detail @click="openPreviewOpenList">
          <ion-icon :icon="eyeOutline" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('devtools.previewOpenList') }}</h3>
            <p>{{ t('devtools.previewOpenListDesc') }}</p>
          </ion-label>
        </ion-item>
      </ion-list>

      <ion-list v-if="configLoaded">
        <ion-list-header>
          <ion-label>{{ t('settings.logSettings') }}</ion-label>
          <ion-badge slot="end" color="primary" class="scope-badge scope-synced">
            <ion-icon :icon="cloudOutline" class="scope-badge-icon"></ion-icon>
            <span class="scope-text">{{ t('settings.synced') }}</span>
          </ion-badge>
        </ion-list-header>
        <div v-if="logLevelField && logLevelField.selectOptions && logLevelField.selectOptions.length > 2" class="log-level-card">
          <div class="field-label-row">
            <ion-icon :icon="terminal" class="field-icon"></ion-icon>
            <span class="field-label-text">{{ tField('level') }}</span>
            <span class="required-mark">*</span>
            <ion-icon :icon="cloudOutline" class="sync-indicator" :title="t('settings.synced')"></ion-icon>
            <ion-button v-if="isLogLevelCustomized" fill="clear" size="small" class="reset-btn" @click="resetLogLevelToDefault">
              <ion-icon :icon="refreshOutline" slot="icon-only"></ion-icon>
            </ion-button>
          </div>
          <div class="preset-cards">
            <div
              v-for="opt in logLevelField.selectOptions"
              :key="opt.value"
              class="preset-card"
              :class="{ 'preset-card-active': logLevel === opt.value }"
              @click="handleLogLevelChange(opt.value)"
            >
              <div class="preset-card-title">{{ opt.label }}</div>
              <div v-if="opt.description" class="preset-card-desc">{{ opt.description }}</div>
            </div>
          </div>
        </div>
        <InputWithHistory
          :model-value="logFile"
          :label="tField('file')"
          :placeholder="t('devtools.logFilePlaceholder')"
          :icon="documentText"
          history-key="config.log.file"
          @update:model-value="handleLogFileChange"
        />
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
  IonButton, IonBadge, alertController,
} from '@ionic/vue'
import {
  bugOutline, downloadOutline, readerOutline, trashOutline,
  chevronForward, playCircleOutline, musicalNotesOutline,
  colorPaletteOutline, settingsOutline, terminal, documentText,
  cloudOutline, refreshOutline, eyeOutline,
} from 'ionicons/icons'
import { useRouter } from 'vue-router'
import { useI18n } from '@/composables/useI18n'
import { useDevTools } from '@/composables/useDevTools'
import { useConfig } from '@/composables/useConfig'
import { getDefaultValue } from '@/config/schemaParser'
import { showToast } from '@/composables/useToast'
import { isNative, exportLogs, clearLogs, openLogViewer, saveDevLogs } from '@/plugins/GoProcess'
import { getFrontendLogsJson } from '@/composables/useFrontendLogs'
import { getAllPrototypes } from './prototypes/registry'
import InputWithHistory from '@/components/InputWithHistory.vue'

const { t, tField } = useI18n()
const router = useRouter()
const { vconsoleEnabled, toggleVConsole } = useDevTools()
const { schemaFields, getFieldValue, setFieldValue, saveConfig, resetFieldToDefault } = useConfig()

const configLoaded = computed(() => schemaFields.value.length > 0)

const logLevel = computed(() => String(getFieldValue(['log', 'level']) ?? 'info'))
const logFile = computed(() => String(getFieldValue(['log', 'file']) ?? ''))

const logLevelField = computed(() => {
  const logSection = schemaFields.value.find((s) => s.key === 'log')
  if (!logSection || !logSection.properties) return null
  return logSection.properties.find((p) => p.key === 'level') || null
})

const logDefault = computed(() => {
  if (!logLevelField.value) return 'info'
  return String(getDefaultValue(logLevelField.value))
})

const isLogLevelCustomized = computed(() => logLevel.value !== logDefault.value)

function resetLogLevelToDefault() {
  if (!logLevelField.value) return
  resetFieldToDefault(['log', 'level'], logLevelField.value)
  saveLogConfig()
}

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

// 沙箱预览：强制整页跳转，绕过 Vue Router 拦截
// 为什么不用 <router-link>：<router-link> 只走 in-app 路由，/openlist-ui/ 不在路由表
// 为什么不用 <a href>：Vue Router 4 在某些 setup 下会拦截 plain <a> 点击事件，
//   导致 router 试图导航到 /openlist-ui/ 失败、渲空 <ion-router-outlet>
// 为什么不用 window.open(_, '_blank')：会破坏 OpenPreview 会话（用户需手动切回 tab）
// 为什么用 window.location.assign：触发完整页面加载，浏览器原生处理同源跳转
const isDev = import.meta.env.DEV
function openPreviewOpenList() {
  window.location.assign('/openlist-ui/')
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

.scope-badge {
  font-size: 10px;
  --padding-start: 6px;
  --padding-end: 8px;
  --padding-top: 3px;
  --padding-bottom: 3px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  gap: 3px;
  flex-shrink: 0;
}
.scope-badge-icon {
  font-size: 12px;
}
.scope-synced {
  --background: rgba(var(--ion-color-primary-rgb), 0.12);
  --color: var(--ion-color-primary);
}
.scope-dev {
  --background: rgba(var(--ion-color-warning-rgb), 0.18);
  --color: var(--ion-color-warning-shade);
}

.log-level-card {
  padding: 12px 16px;
  border-bottom: 1px solid var(--ion-color-light-shade, #e0e0e0);
}

body.dark .log-level-card {
  border-bottom-color: #2a2a2c;
}

.field-icon {
  font-size: 18px;
  color: var(--ion-color-medium);
  flex-shrink: 0;
}

.field-label-row {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}

.field-label-text {
  flex: 1 1 auto;
  min-width: 0;
  font-weight: 500;
  font-size: 15px;
}

.required-mark {
  color: var(--ion-color-danger);
  margin-left: 2px;
}

.sync-indicator {
  font-size: 12px;
  color: var(--ion-color-primary);
  opacity: 0.4;
  flex-shrink: 0;
}

.reset-btn {
  --padding-start: 4px;
  --padding-end: 4px;
  min-width: 28px;
  min-height: 28px;
  margin: 0;
}

.reset-btn ion-icon {
  font-size: 16px;
}

.preset-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(80px, 1fr));
  gap: 8px;
  margin-top: 10px;
  width: 100%;
}

.preset-card {
  padding: 10px 8px;
  border: 2px solid var(--ion-color-light-shade, #e0e0e0);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s;
  text-align: center;
  background: var(--ion-background-color, transparent);
}

.preset-card-active {
  border-color: var(--ion-color-primary);
  background: rgba(var(--ion-color-primary-rgb), 0.08);
}

.preset-card-title {
  font-weight: 600;
  font-size: 13px;
}

.preset-card-desc {
  font-size: 11px;
  color: var(--ion-color-medium);
  margin-top: 3px;
  line-height: 1.3;
}

@media (max-width: 599px) {
  .preset-cards {
    grid-template-columns: repeat(auto-fit, minmax(70px, 1fr));
    gap: 6px;
  }
  .preset-card-title {
    font-size: 12px;
  }
  .preset-card-desc {
    font-size: 10px;
  }
}
</style>

<style>
body.dark .preset-card {
  border-color: #3a3a3c;
}

body.dark .preset-card-active {
  border-color: var(--ion-color-primary);
  background: rgba(var(--ion-color-primary-rgb), 0.12);
}
</style>

<style scoped>
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
