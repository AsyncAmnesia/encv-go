<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-button @click="router.back()">
            <ion-icon :icon="arrowBack" slot="icon-only"></ion-icon>
          </ion-button>
        </ion-buttons>
        <ion-title>{{ fileName }}</ion-title>
        <ion-buttons slot="end">
          <ion-button v-if="previewType === 'text'" @click="copyContent" fill="clear" size="small">
            <ion-icon :icon="copied ? checkmarkCircle : copyOutline" slot="icon-only"></ion-icon>
          </ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>

    <ion-content class="preview-content">
      <div v-if="loading" class="loading-container">
        <ion-spinner name="crescent"></ion-spinner>
        <p>{{ t('filePreview.loading') }}</p>
      </div>

      <div v-else-if="error" class="error-state">
        <ion-icon :icon="alertCircle" class="error-icon"></ion-icon>
        <h3>{{ t('filePreview.loadError') }}</h3>
        <p>{{ error }}</p>
        <ion-button @click="loadFile">{{ t('filePreview.retry') }}</ion-button>
      </div>

      <template v-else>
        <div v-if="previewType === 'image'" class="preview-wrapper image-preview">
          <img :src="streamUrl" class="preview-image" />
        </div>

        <div v-else-if="previewType === 'pdf'" class="preview-wrapper pdf-preview">
          <iframe :src="streamUrl" class="preview-iframe"></iframe>
        </div>

        <div v-else-if="previewType === 'text'" class="preview-wrapper">
          <div class="file-meta">
            <span class="meta-item"><ion-icon :icon="documentTextOutline"></ion-icon> {{ filePath }}</span>
            <span class="meta-item"><ion-icon :icon="informationCircle"></ion-icon> {{ formatFileSize(fileSize) }}</span>
            <span v-if="encoding !== 'utf-8'" class="meta-item warn">
              <ion-icon :icon="warning"></ion-icon> {{ encoding }}
            </span>
          </div>
          <pre class="file-content"><code>{{ content }}</code></pre>
        </div>

        <div v-else-if="previewType === 'container'" class="preview-wrapper container-info">
          <div class="container-card">
            <h4 class="card-title">
              <ion-icon :icon="lockClosed" class="title-icon"></ion-icon>
              ENCV Container
            </h4>
            <div class="info-grid" v-if="containerInfo">
              <div class="info-row">
                <span class="info-label">{{ t('fileInfo.version') }}</span>
                <span class="info-value">V{{ containerInfo.version ?? '?' }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">{{ t('fileInfo.containerId') }}</span>
                <span class="info-value code-text">{{ containerInfo.container_id ?? '-' }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">{{ t('fileInfo.containerType') }}</span>
                <span class="info-value">{{ containerInfo.container_type ?? '-' }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">{{ t('fileInfo.seekable') }}</span>
                <ion-badge :color="containerInfo.is_seekable ? 'success' : 'medium'">
                  {{ containerInfo.is_seekable ? 'Yes' : 'No' }}
                </ion-badge>
              </div>
              <div class="info-row" v-if="containerInfo.original_duration">
                <span class="info-label">{{ t('fileInfo.duration') }}</span>
                <span class="info-value">{{ formatDuration(containerInfo.original_duration) }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">{{ t('fileInfo.segmentCount') }}</span>
                <span class="info-value">{{ containerInfo.segment_count ?? 0 }}</span>
              </div>
            </div>
          </div>

          <div class="manifest-section">
            <div class="manifest-header" @click="showManifest = !showManifest">
              <span>{{ t('fileInfo.manifest') }}</span>
              <ion-icon :icon="showManifest ? chevronDown : chevronForward"></ion-icon>
            </div>
            <pre v-if="showManifest && containerInfo" class="manifest-json"><code>{{ manifestJson }}</code></pre>
          </div>
        </div>

        <div v-else class="preview-wrapper unsupported-preview">
          <ion-icon :icon="helpCircleOutline" class="unsupported-icon"></ion-icon>
          <h3>{{ fileName }}</h3>
          <div class="unsupported-meta">
            <span class="meta-item"><ion-icon :icon="documentTextOutline"></ion-icon> {{ filePath }}</span>
            <span class="meta-item"><ion-icon :icon="informationCircle"></ion-icon> {{ formatFileSize(fileSize) }}</span>
          </div>
          <p class="unsupported-msg">{{ t('filePreview.unsupported') }}</p>
        </div>
      </template>
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
  arrowBack, copyOutline, documentTextOutline,
  alertCircle, informationCircle, warning, helpCircleOutline,
  lockClosed, checkmarkCircle, chevronDown, chevronForward,
} from 'ionicons/icons'
import { readFileContent, getFileStreamUrl, getFileCategory, getFileExtension, formatFileSize, fetchTextPreviewExts } from '@/api/encv'
import type { FileContentResponse } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'

type PreviewType = 'image' | 'pdf' | 'text' | 'container' | 'unsupported'

interface ContainerInfo {
  version: number
  container_id: string
  container_type: string
  is_seekable: boolean
  original_duration?: number
  segment_count?: number
  segments: unknown[]
}

const { t } = useI18n()
const router = useRouter()
const route = useRoute()

const filePath = ref('')
const fileName = ref('')
const content = ref('')
const fileSize = ref(0)
const encoding = ref('utf-8')
const loading = ref(true)
const error = ref('')
const previewType = ref<PreviewType>('unsupported')
const streamUrl = ref('')
const copied = ref(false)
const showManifest = ref(false)
const containerInfo = ref<ContainerInfo | null>(null)
const manifestJson = ref('')

function formatDuration(seconds: number): string {
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

async function determinePreviewType(name: string, isEncrypted?: boolean): Promise<PreviewType> {
  const category = getFileCategory(name, isEncrypted)
  const ext = getFileExtension(name)

  if (category === 'image') return 'image'
  if (ext === 'pdf') return 'pdf'
  if (category === 'encrypted') return 'container'

  const textExts = await fetchTextPreviewExts()
  if (textExts.has(ext)) return 'text'
  if (category === 'document' || category === 'other') return 'text'
  return 'unsupported'
}

async function loadFile() {
  const path = (route.query.path as string) || ''
  const name = (route.query.name as string) || ''
  if (!path) {
    error.value = t('filePreview.noPath')
    loading.value = false
    return
  }
  filePath.value = path
  fileName.value = name || path.split('/').pop() || path
  loading.value = true
  error.value = ''
  copied.value = false
  showManifest.value = false
  containerInfo.value = null

  const isEncrypted = route.query.isEncrypted === 'true'
  previewType.value = await determinePreviewType(fileName.value, isEncrypted)

  try {
    if (previewType.value === 'image' || previewType.value === 'pdf') {
      console.info('Loading stream preview:', fileName.value)
      streamUrl.value = getFileStreamUrl(path)
    } else if (previewType.value === 'text') {
      console.info('Loading text preview:', fileName.value)
      const data: FileContentResponse = await readFileContent(path)
      content.value = data.content
      fileSize.value = data.size
      encoding.value = data.encoding
    } else if (previewType.value === 'container') {
      console.info('Loading container info:', fileName.value)
      const baseUrl = import.meta.env.DEV ? '' : (await import('@/api/encv')).getApiBaseUrl()
      const resp = await fetch(`${baseUrl}/api/file/info?path=${encodeURIComponent(path)}`)
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
      const info = await resp.json()
      containerInfo.value = info.container || null
      fileSize.value = info.size || 0
      if (info.container) {
        manifestJson.value = JSON.stringify(info.container.manifest || info.container, null, 2)
      }
    } else {
      console.info('Unsupported file type:', fileName.value)
      fileSize.value = 0
    }
  } catch (e: any) {
    console.error('Failed to load file:', e)
    error.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}

async function copyContent() {
  try {
    await navigator.clipboard.writeText(content.value)
    copied.value = true
    showToast({
      message: t('filePreview.copied'),
      duration: 1500,
      position: 'middle',
      color: 'success',
    })
    setTimeout(() => { copied.value = false }, 2000)
  } catch {
    const textArea = document.createElement('textarea')
    textArea.value = content.value
    document.body.appendChild(textArea)
    textArea.select()
    document.execCommand('copy')
    document.body.removeChild(textArea)
    showToast({
      message: t('filePreview.copied'),
      duration: 1500,
      position: 'middle',
      color: 'success',
    })
  }
}

onMounted(() => loadFile())
</script>

<style scoped>
.preview-content {
  --background: #1e1e2e;
}

.preview-wrapper {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.image-preview {
  align-items: center;
  justify-content: center;
  padding: 8px;
}

.preview-image {
  width: 100%;
  max-height: 100%;
  object-fit: contain;
  border-radius: 4px;
}

.pdf-preview {
  flex: 1;
}

.preview-iframe {
  width: 100%;
  height: 100%;
  border: none;
  flex: 1;
}

.file-meta {
  display: flex;
  gap: 12px;
  padding: 8px 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 11px;
  color: #888;
  flex-wrap: wrap;
  align-items: center;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}
.meta-item.warn { color: #f39c12; }

.file-content {
  margin: 0;
  padding: 10px 14px;
  flex: 1;
  overflow: auto;
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.6;
  color: #cdd6f4;
  background: transparent;
  white-space: pre-wrap;
  word-break: break-all;
  tab-size: 2;
}

.file-content code {
  font-family: inherit;
  background: none;
  padding: 0;
}

.container-info {
  padding: 16px;
  gap: 12px;
  overflow-y: auto;
}

.container-card {
  background: rgba(255, 255, 255, 0.04);
  border-radius: 8px;
  padding: 14px;
  border-left: 3px solid var(--ion-color-primary);
}

.card-title {
  margin: 0 0 12px;
  font-size: 15px;
  font-weight: 600;
  color: var(--ion-text-color);
  display: flex;
  align-items: center;
  gap: 8px;
}
.title-icon { color: var(--ion-color-primary); }

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
}

.info-value {
  color: var(--ion-text-color);
  font-weight: 400;
}

.code-text {
  font-family: monospace;
  font-size: 12px;
  word-break: break-all;
}

.manifest-section {
  background: rgba(255, 255, 255, 0.04);
  border-radius: 8px;
  overflow: hidden;
}

.manifest-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  color: var(--ion-text-secondary);
  user-select: none;
}
.manifest-header:hover { color: var(--ion-text-color); }

.manifest-json {
  margin: 0;
  padding: 10px 14px;
  max-height: 300px;
  overflow: auto;
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  line-height: 1.5;
  color: #888;
  white-space: pre-wrap;
  word-break: break-all;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

.unsupported-preview {
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: #888;
  text-align: center;
  padding: 24px;
}

.unsupported-icon {
  font-size: 64px;
  opacity: 0.4;
  color: #888;
}

.unsupported-preview h3 {
  margin: 0;
  color: #cdd6f4;
  font-size: 16px;
  word-break: break-all;
}

.unsupported-meta {
  display: flex;
  gap: 12px;
  font-size: 11px;
  color: #888;
  flex-wrap: wrap;
  justify-content: center;
}

.unsupported-msg {
  color: #666;
  font-size: 13px;
  margin: 8px 0 0;
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
</style>
