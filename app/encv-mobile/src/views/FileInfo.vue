<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-button @click="router.back()">
            <ion-icon :icon="arrowBack" slot="icon-only"></ion-icon>
          </ion-button>
        </ion-buttons>
        <ion-title>{{ t('files.info') }}</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content class="file-info-content">
      <div v-if="loading" class="loading-container">
        <ion-spinner name="crescent"></ion-spinner>
        <p>{{ t('filePreview.loading') }}</p>
      </div>

      <div v-else-if="error" class="error-state">
        <ion-icon :icon="alertCircle" class="error-icon"></ion-icon>
        <h3>{{ t('filePreview.loadError') }}</h3>
        <p>{{ error }}</p>
        <ion-button @click="loadInfo">{{ t('filePreview.retry') }}</ion-button>
      </div>

      <div v-else-if="info" class="info-scroll">
        <div class="section-card">
          <h4 class="card-title">
            <ion-icon :icon="documentTextOutline" class="title-icon"></ion-icon>
            {{ t('files.info') }}
          </h4>
          <div class="info-grid">
            <div class="info-row">
              <span class="info-label">{{ t('fileInfo.name') || 'Name' }}</span>
              <span class="info-value">{{ info.name }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">{{ t('fileInfo.path') || 'Path' }}</span>
              <span class="info-value code-text">{{ info.path }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">{{ t('fileInfo.size') || 'Size' }}</span>
              <span class="info-value">{{ formatFileSize(info.size) }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">{{ t('fileInfo.modified') || 'Modified' }}</span>
              <span class="info-value">{{ formatTime(info.modified) }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">MIME</span>
              <span class="info-value code-text">{{ info.mime_type }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">{{ t('fileInfo.category') || 'Category' }}</span>
              <ion-badge color="medium">{{ info.category }}</ion-badge>
            </div>
            <div class="info-row" v-if="info.is_encrypted">
              <span class="info-label">{{ t('files.encrypted') }}</span>
              <ion-badge color="warning">Yes</ion-badge>
            </div>
            <div class="info-row decoded-name-row" v-if="decodedName">
              <span class="info-label decoded-label">原始文件名</span>
              <span class="info-value decoded-value">{{ decodedName }}</span>
            </div>
          </div>
        </div>

        <div v-if="info.is_encv_container && containerData" class="section-card container-card">
          <h4 class="card-title">
            <ion-icon :icon="lockClosed" class="title-icon primary"></ion-icon>
            ENCV Container
          </h4>
          <div class="info-grid">
            <div class="info-row">
              <span class="info-label">{{ t('fileInfo.version') }}</span>
              <span class="info-value">V{{ containerData.version ?? '?' }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">{{ t('fileInfo.containerId') }}</span>
              <span class="info-value code-text">{{ containerData.container_id ? containerData.container_id : '(auto)' }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">{{ t('fileInfo.containerType') }}</span>
              <ion-badge color="primary">{{ containerData.container_type ?? '-' }}</ion-badge>
            </div>
            <div class="info-row">
              <span class="info-label">{{ t('fileInfo.seekable') }}</span>
              <ion-badge :color="containerData.is_seekable ? 'success' : 'medium'">
                {{ containerData.is_seekable ? 'Yes' : 'No' }}
              </ion-badge>
            </div>
            <div class="info-row" v-if="containerData.original_duration != null">
              <span class="info-label">{{ t('fileInfo.duration') }}</span>
              <span class="info-value">{{ formatDuration(containerData.original_duration) }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">{{ t('fileInfo.segmentCount') }}</span>
              <span class="info-value">{{ containerData.segment_count ?? 0 }}</span>
            </div>
          </div>
        </div>

        <div v-if="info.is_encv_container && containerData" class="section-card manifest-card">
          <div class="manifest-header" @click="showManifest = !showManifest">
            <h4 class="card-title inline-title">
              <ion-icon :icon="listOutline" class="title-icon"></ion-icon>
              {{ t('fileInfo.manifest') }}
            </h4>
            <ion-icon :icon="showManifest ? chevronDown : chevronForward"></ion-icon>
          </div>
          <pre v-if="showManifest" class="manifest-json"><code>{{ manifestJson }}</code></pre>
        </div>
      </div>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons,
  IonButton, IonIcon, IonContent, IonSpinner, IonBadge,
} from '@ionic/vue'
import {
  arrowBack, documentTextOutline, alertCircle,
  lockClosed, listOutline, chevronDown, chevronForward,
} from 'ionicons/icons'
import { getApiBaseUrl, formatFileSize, proxySafeEncode } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'
import { isAlistEncrypted, loadDecodedName, getDecodedName } from '@/features/alist-encrypt/useAlistEncrypt'
import type { FileItem } from '@/api/encv'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()

interface ContainerData {
  version: number
  container_id: string
  container_type: string
  is_seekable: boolean
  original_duration?: number | null
  segment_count?: number | null
  segments?: unknown[]
  manifest_size?: number
  header?: Record<string, unknown>
  manifest?: unknown
}

interface FileInfo {
  name: string
  path: string
  size: number
  modified: string
  mime_type: string
  category: string
  is_directory: boolean
  is_encrypted: boolean
  is_encv_container: boolean
  container?: ContainerData
}

const loading = ref(true)
const error = ref('')
const info = ref<FileInfo | null>(null)
const containerData = ref<ContainerData | null>(null)
const showManifest = ref(false)
const manifestJson = ref('')
const decodedName = ref<string | null>(null)
const isAlistEnc = ref(false)

function formatDuration(seconds: number): string {
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

function formatTime(isoStr: string): string {
  try {
    return new Date(isoStr).toLocaleString()
  } catch {
    return isoStr
  }
}

async function loadInfo() {
  const path = (route.query.path as string) || ''
  if (!path) {
    error.value = t('filePreview.noPath')
    loading.value = false
    return
  }

  loading.value = true
  error.value = ''
  showManifest.value = false

  try {
    const baseUrl = getApiBaseUrl()
    const resp = await fetch(`${baseUrl}/api/file/info?path=${proxySafeEncode(path)}`)
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
    const data = await resp.json() as FileInfo
    info.value = data
    containerData.value = data.container || null

    if (data.container?.manifest) {
      try {
        const str = JSON.stringify(data.container.manifest, null, 2)
        manifestJson.value = /^[\x20-\x7E\t\n\r]*$/.test(str) ? str : '(contains non-printable characters)'
      } catch {
        manifestJson.value = '(invalid)'
      }
    } else {
      manifestJson.value = '(none)'
    }

    const fileItem: FileItem = {
      name: data.name,
      path: data.path,
      isDirectory: data.is_directory,
      isEncrypted: data.is_encrypted,
      size: data.size,
    }
    isAlistEnc.value = isAlistEncrypted(fileItem)
    if (isAlistEnc.value) {
      loadDecodedName(fileItem).then((name) => {
        decodedName.value = name || getDecodedName(data.path) || null
      }).catch(() => {})
    } else {
      decodedName.value = null
    }
  } catch (e: any) {
    console.error('[FileInfo] failed:', e)
    error.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}

onMounted(() => loadInfo())
</script>

<style scoped>
.file-info-content {
  --background: #1e1e2e;
}

.info-scroll {
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-card {
  background: rgba(255, 255, 255, 0.04);
  border-radius: 10px;
  padding: 16px;
  border-left: 3px solid var(--ion-color-medium);
}
.section-card.container-card { border-left-color: var(--ion-color-primary); }
.section-card.manifest-card { border-left-color: var(--ion-color-tertiary); }

.card-title {
  margin: 0 0 12px;
  font-size: 15px;
  font-weight: 600;
  color: var(--ion-text-color);
  display: flex;
  align-items: center;
  gap: 8px;
}
.inline-title { margin: 0; }
.title-icon { color: var(--ion-color-medium); }
.title-icon.primary { color: var(--ion-color-primary); }

.info-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.info-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
}

.info-label {
  color: var(--ion-text-secondary);
  font-weight: 500;
  min-width: 80px;
  flex-shrink: 0;
}

.info-value {
  color: var(--ion-text-color);
  font-weight: 400;
  text-align: right;
  word-break: break-all;
}

.code-text {
  font-family: monospace;
  font-size: 11px;
}

.manifest-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;
  user-select: none;
  -webkit-tap-highlight-color: transparent;
  margin-bottom: 0;
}
.manifest-header:hover .card-title { opacity: 0.8; }

.manifest-json {
  margin: 10px 0 0;
  padding: 10px 12px;
  max-height: 350px;
  overflow: auto;
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  line-height: 1.5;
  color: #888;
  white-space: pre-wrap;
  word-break: break-all;
  background: rgba(0, 0, 0, 0.2);
  border-radius: 6px;
}

.loading-container, .error-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 60%;
  color: #888;
  text-align: center;
  gap: 12px;
}
.error-icon { font-size: 48px; opacity: 0.5; color: #e74c3c; }

.decoded-name-row {
  margin-top: 4px;
  padding-top: 8px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}
.decoded-label {
  color: var(--ion-color-medium-shade, #999);
  font-style: italic;
}
.decoded-value {
  color: var(--ion-color-medium-shade, #999);
  font-style: italic;
  font-size: 12px;
}
</style>
