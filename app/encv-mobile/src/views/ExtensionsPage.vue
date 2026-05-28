<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button :text="t('tabs.home')" default-href="/tabs/home"></ion-back-button>
        </ion-buttons>
        <ion-title>{{ t('extensions.title') }}</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content class="extensions-content">
      <div v-if="isLoading" class="loading-container">
        <ion-spinner name="crescent"></ion-spinner>
        <p>{{ t('settings.loading') }}</p>
      </div>

      <template v-else>
        <div v-if="extensions.length === 0" class="empty-state">
          <ion-icon :icon="layersOutline" class="empty-icon"></ion-icon>
          <h3>{{ t('extensions.noExtensions') }}</h3>
          <p>{{ t('extensions.hint') }}</p>
        </div>

        <div class="extensions-list">
          <ion-card v-for="ext in extensions" :key="ext.id" class="extension-card">
            <ion-card-header>
              <div class="ext-header-row">
                <div class="ext-title-area">
                  <ion-icon :icon="filmOutline" class="ext-icon"></ion-icon>
                  <ion-card-title>{{ ext.name }}</ion-card-title>
                </div>
                <ion-badge :color="ext.installed ? 'success' : 'medium'">
                  {{ ext.installed ? t('extensions.installed') : t('extensions.notInstalled') }}
                </ion-badge>
              </div>
            </ion-card-header>

            <div class="extension-card-content">
              <p class="ext-desc">{{ ext.description }}</p>
              <p class="ext-size">{{ t('extensions.sizeHint', { size: ext.sizeDisplay }) }}</p>
            </div>

            <div class="extension-card-footer">
              <template v-if="!ext.installed">
                <ion-button
                  color="primary"
                  size="small"
                  @click="handleInstall(ext.id)"
                  :disabled="isInstalling"
                >
                  <ion-icon :icon="addOutline" slot="start"></ion-icon>
                  {{ isInstalling ? t('extensions.installing') : t('extensions.installFromLocal') }}
                </ion-button>
              </template>
              <template v-else>
                <ion-button
                  fill="outline"
                  size="small"
                  @click="handleToggleEnabled(ext.id, ext.enabled)"
                >
                  <ion-icon :icon="ext.enabled ? closeCircle : checkmarkCircle" slot="start"></ion-icon>
                  {{ ext.enabled ? t('extensions.disable') : t('extensions.enable') }}
                </ion-button>
                <ion-button
                  color="danger"
                  fill="outline"
                  size="small"
                  @click="handleUninstall(ext.id)"
                >
                  <ion-icon :icon="closeCircle" slot="start"></ion-icon>
                  {{ t('extensions.uninstall') }}
                </ion-button>
              </template>
            </div>
          </ion-card>
        </div>

        <div class="install-section">
          <ion-button expand="block" fill="outline" @click="handleInstallFromFile" :disabled="isInstalling || !isNativePlatform()">
            <ion-icon :icon="cloudUploadOutline" slot="start"></ion-icon>
            {{ isInstalling ? t('extensions.installing') : t('extensions.selectApk') }}
          </ion-button>
          <p class="install-hint">{{ t('extensions.installFromLocalHint') }}</p>
          <div v-if="isNativePlatform()" style="margin-top: 12px; display: flex; flex-direction: column; gap: 6px;">
            <ion-button expand="block" fill="outline" color="warning" @click="handleDebugInstall" size="small">
              🔧 installPlugin实际调用
            </ion-button>
            <ion-button expand="block" fill="outline" color="warning" @click="handleDebugKotlinReflect" size="small">
              🔧 kotlin-reflect健康检查
            </ion-button>
            <ion-button expand="block" fill="outline" color="warning" @click="handleDebugApkValidation" size="small">
              🔧 APK元数据+签名校验
            </ion-button>
            <ion-button expand="block" fill="outline" color="warning" @click="handleDebugValidationStrategy" size="small">
              🔧 ValidationStrategy状态
            </ion-button>
          </div>
        </div>
      </template>

      <div v-if="installError" class="error-banner">
        <ion-icon :icon="informationCircle"></ion-icon>
        <span>{{ installError }}</span>
      </div>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  IonPage,
  IonHeader,
  IonToolbar,
  IonButtons,
  IonBackButton,
  IonTitle,
  IonContent,
  IonCard,
  IonCardHeader,
  IonCardTitle,
  IonButton,
  IonBadge,
  IonIcon,
  IonSpinner,
  alertController,
} from '@ionic/vue'
import {
  filmOutline,
  addOutline,
  informationCircle,
  checkmarkCircle,
  closeCircle,
  cloudUploadOutline,
  layersOutline,
} from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import { Capacitor } from '@capacitor/core'
import { isNative, pickAndInstallPlugin, checkInstalledPlugins, debugInstallFlow, debugKotlinReflect, debugApkValidation, debugValidationStrategy } from '@/plugins/GoProcess'
import { showToast } from '@/composables/useToast'

const { t } = useI18n()

interface ExtensionInfo {
  id: string
  name: string
  description: string
  installed: boolean
  enabled: boolean
  sizeDisplay: string
}

const extensions = ref<ExtensionInfo[]>([])
const isLoading = ref(true)
const installError = ref('')
const isInstalling = ref(false)

function isNativePlatform(): boolean {
  return isNative()
}

onMounted(async () => {
  await loadExtensions()
})

async function loadExtensions() {
  isLoading.value = true
  try {
    const COMBOLITE_PLUGIN_ID_MAP: Record<string, string> = {
      'mpv-player': 'com.encvgo.plugin.mpv',
    }

    const installedMap = Capacitor.isNativePlatform() ? await checkInstalledPlugins() : {}
    extensions.value = [
      {
        id: 'mpv-player',
        name: t('extensions.mpvPlayer'),
        description: t('extensions.mpvPlayerDesc'),
        installed: !!installedMap[COMBOLITE_PLUGIN_ID_MAP['mpv-player'] || 'mpv-player'],
        enabled: true,
        sizeDisplay: '~35 MB',
      },
    ]
  } catch (e) {
    console.error('Failed to load extensions:', e)
  } finally {
    isLoading.value = false
  }
}

async function handleInstallFromFile() {
  if (!isNativePlatform()) return

  isInstalling.value = true
  installError.value = ''

  try {
    const result = await Promise.race([
      pickAndInstallPlugin(),
      new Promise<never>((_, reject) => setTimeout(() => reject(new Error('Installation timeout')), 120000)),
    ])
    if (result.success) {
      const alert = await alertController.create({
        header: t('extensions.installSuccess'),
        message: `${result.fileName || ''}\n${t('extensions.installHint')}`,
        buttons: [t('common.confirm')],
      })
      await alert.present()
      await loadExtensions()
    } else {
      installError.value = result.error || t('extensions.installFailed')
    }
  } catch (e: any) {
    installError.value = e.message || t('extensions.installFailed')
  } finally {
    isInstalling.value = false
  }
}

async function handleInstall(_id: string) {
  await handleInstallFromFile()
}

async function handleToggleEnabled(id: string, currentEnabled: boolean) {
  console.log('Toggle extension:', id, '→', !currentEnabled)
  await loadExtensions()
}

async function handleUninstall(id: string) {
  const alert = await alertController.create({
    header: t('extensions.uninstallConfirm'),
    buttons: [
      { text: t('common.cancel'), role: 'cancel' },
      {
        text: t('common.confirm'),
        role: 'confirm',
        handler: async () => {
          console.log('Uninstall extension:', id)
          await loadExtensions()
        },
      },
    ],
  })
  await alert.present()
}

async function showDebugResult(header: string, result: Record<string, any>) {
  const debugText = result.debugLog || Object.entries(result)
    .map(([k, v]) => `${k}: ${v}`)
    .join('\n')
  const alert = await alertController.create({
    header,
    message: `<pre style="font-size:12px;white-space:pre-wrap;max-height:60vh;overflow:auto;">${debugText}</pre>`,
    buttons: [
      {
        text: '复制',
        handler: async () => {
          try {
            await navigator.clipboard.writeText(debugText)
            showToast({ message: '已复制诊断信息', duration: 1500, color: 'success' })
          } catch {
            showToast({ message: '复制失败', duration: 1500, color: 'danger' })
          }
          return false
        },
      },
      'OK',
    ],
  })
  await alert.present()
}

async function handleDebugInstall() {
  try {
    const result = await debugInstallFlow()
    await showDebugResult('🔧 installPlugin诊断', result)
  } catch (e: any) {
    await showDebugResult('🔧 诊断失败', { debugLog: e?.message || String(e) })
  }
}

async function handleDebugKotlinReflect() {
  try {
    const result = await debugKotlinReflect()
    await showDebugResult('🔧 kotlin-reflect诊断', result)
  } catch (e: any) {
    await showDebugResult('🔧 诊断失败', { debugLog: e?.message || String(e) })
  }
}

async function handleDebugApkValidation() {
  try {
    const result = await debugApkValidation()
    await showDebugResult('🔧 APK校验诊断', result)
  } catch (e: any) {
    await showDebugResult('🔧 诊断失败', { debugLog: e?.message || String(e) })
  }
}

async function handleDebugValidationStrategy() {
  try {
    const result = await debugValidationStrategy()
    await showDebugResult('🔧 ValidationStrategy诊断', result)
  } catch (e: any) {
    await showDebugResult('🔧 诊断失败', { debugLog: e?.message || String(e) })
  }
}
</script>

<style scoped>
.extensions-content {
  --background: var(--ion-background-color);
}

.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
  color: var(--encv-text-secondary);
}

.loading-container p {
  margin-top: 12px;
  font-size: 14px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 48px 24px;
  text-align: center;
}

.empty-icon {
  font-size: 56px;
  color: var(--ion-color-medium);
  margin-bottom: 16px;
}

.empty-state h3 {
  font-size: 17px;
  font-weight: 600;
  color: var(--ion-text-color);
  margin: 0 0 8px;
}

.empty-state p {
  font-size: 14px;
  color: var(--encv-text-secondary);
  margin: 0;
}

.extensions-list {
  padding: 8px 16px;
}

.extension-card {
  border-radius: 12px;
  margin: 0 0 12px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
}

.ext-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.ext-title-area {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

.ext-icon {
  font-size: 24px;
  color: #8b5cf6;
  flex-shrink: 0;
}

.ext-title-area ion-card-title {
  font-size: 16px;
  font-weight: 600;
  margin: 0;
}

.ext-desc {
  font-size: 13px;
  color: var(--ion-text-color);
  line-height: 1.5;
  margin: 0 0 6px;
}

.ext-size {
  font-size: 12px;
  color: var(--encv-text-secondary);
  margin: 0;
}

.extension-card-footer {
  display: flex;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid var(--ion-color-light, #f4f5f8);
}

.install-section {
  padding: 16px 20px 32px;
}

.install-hint {
  text-align: center;
  font-size: 12px;
  color: var(--encv-text-secondary);
  margin: 8px 0 0;
}

.error-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 12px 16px;
  padding: 10px 14px;
  border-radius: 8px;
  background: rgba(var(--ion-color-danger-rgb), 0.1);
  color: var(--ion-color-danger);
  font-size: 13px;
}
</style>
