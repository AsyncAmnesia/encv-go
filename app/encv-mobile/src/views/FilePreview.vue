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
        <div v-if="isEncryptedPreview" class="file-info-card">
          <div class="info-grid">
            <div class="info-row">
              <span class="info-label">{{ t('fileInfo.name') || 'Name' }}</span>
              <span class="info-value">{{ fileName }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">{{ t('fileInfo.size') || 'Size' }}</span>
              <span class="info-value">{{ formatFileSize(fileSize) }}</span>
            </div>
            <div class="info-row" v-if="fileModified">
              <span class="info-label">{{ t('fileInfo.modified') || 'Modified' }}</span>
              <span class="info-value">{{ fileModified }}</span>
            </div>
            <div class="info-row" v-if="fileMimeType">
              <span class="info-label">MIME</span>
              <span class="info-value code-text">{{ fileMimeType }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">{{ t('fileInfo.category') || 'Category' }}</span>
              <ion-badge color="medium">{{ fileCategory }}</ion-badge>
            </div>
            <div class="info-row">
              <span class="info-label">{{ t('files.encrypted') }}</span>
              <ion-badge color="warning">Yes</ion-badge>
            </div>
          </div>
        </div>

        <div v-if="previewType === 'image'" class="preview-wrapper image-preview">
          <img :src="streamUrl" class="preview-image" />
        </div>

        <div v-else-if="previewType === 'pdf'" class="preview-wrapper pdf-preview">
          <iframe :src="pdfPreviewUrl" class="preview-iframe"></iframe>
        </div>

        <div v-else-if="previewType === 'text'" class="preview-wrapper text-preview">
          <iframe :src="textPreviewUrl" class="preview-iframe"></iframe>
        </div>

        <div v-else-if="previewType === 'container'" class="preview-wrapper container-info">
          <div class="container-card">
            <h4 class="card-title">
              <ion-icon :icon="lockClosed" class="title-icon"></ion-icon>
              ENCV Container
            </h4>
            <div v-if="containerInfo?.error" class="container-error">
              <ion-icon :icon="alertCircle" class="error-icon"></ion-icon>
              <p>{{ containerInfo.error }}</p>
            </div>
            <div class="info-grid" v-if="containerInfo && !containerInfo.error">
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
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons,
  IonButton, IonIcon, IonContent, IonSpinner, IonBadge,
} from '@ionic/vue'
import {
  arrowBack, documentTextOutline,
  alertCircle, informationCircle,
  helpCircleOutline, lockClosed,
  chevronDown, chevronForward,
} from 'ionicons/icons'
import { getFileStreamUrl, getFileCategory, getFileExtension, formatFileSize, fetchTextPreviewExts, getApiBaseUrl, getFilePreviewUrl } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'

type PreviewType = 'image' | 'pdf' | 'text' | 'container' | 'unsupported'

interface ContainerInfo {
  version?: number
  container_id?: string
  container_type?: string
  is_seekable?: boolean
  original_duration?: number
  segment_count?: number
  segments?: unknown[]
  error?: string
}

const { t } = useI18n()
const router = useRouter()
const route = useRoute()

const filePath = ref('')
const fileName = ref('')
const fileSize = ref(0)
const loading = ref(true)
const error = ref('')
const previewType = ref<PreviewType>('unsupported')
const streamUrl = ref('')
const textPreviewUrl = ref('')
const pdfPreviewUrl = ref('')
const showManifest = ref(false)
const containerInfo = ref<ContainerInfo | null>(null)
const manifestJson = ref('')
const fileModified = ref('')
const fileMimeType = ref('')
const fileCategory = ref('')

const isEncryptedPreview = computed(() => route.query.isEncrypted === 'true')

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
  showManifest.value = false
  containerInfo.value = null

  const isEncrypted = route.query.isEncrypted === 'true'

  if (isEncrypted) {
    try {
      const baseUrl = getApiBaseUrl()
      const resp = await fetch(`${baseUrl}/api/file/info?path=${encodeURIComponent(path)}`)
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
      const info = await resp.json()

      fileSize.value = info.size || 0
      fileModified.value = info.modified || ''
      fileMimeType.value = info.mime_type || ''
      fileCategory.value = info.category || ''

      if (info.is_encv_container && info.container) {
        const containerType = info.container.container_type
        containerInfo.value = info.container
        try {
          const str = JSON.stringify(info.container.manifest || info.container, null, 2)
          manifestJson.value = /^[\x20-\x7E\t\n\r]*$/.test(str) ? str : '(contains non-printable characters)'
        } catch {
          manifestJson.value = '(invalid)'
        }

        switch (containerType) {
          case 'image':
            previewType.value = 'image'
            streamUrl.value = getFileStreamUrl(path)
            break
          case 'video':
          case 'audio':
            router.push({ path: '/player', query: { path, name: fileName.value } })
            loading.value = false
            return
          case 'document':
          case 'text':
            const ext = getFileExtension(fileName.value)
            if (ext === 'pdf') {
              previewType.value = 'pdf'
              pdfPreviewUrl.value = getFilePreviewUrl('pdf.html', path)
            } else {
              previewType.value = 'text'
              textPreviewUrl.value = getFilePreviewUrl('text.html', path)
            }
            break
          default:
            previewType.value = 'container'
        }
      } else {
        previewType.value = 'unsupported'
      }
    } catch (e: any) {
      console.error('Failed to load encrypted file:', e)
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
    return
  }

  previewType.value = await determinePreviewType(fileName.value, isEncrypted)

  try {
    if (previewType.value === 'image') {
      console.info('Loading stream preview:', fileName.value)
      streamUrl.value = getFileStreamUrl(path)
    } else if (previewType.value === 'pdf') {
      console.info('Loading PDF preview:', fileName.value)
      pdfPreviewUrl.value = getFilePreviewUrl('pdf.html', path)
    } else if (previewType.value === 'text') {
      console.info('Loading text preview:', fileName.value)
      textPreviewUrl.value = getFilePreviewUrl('text.html', path)
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

.pdf-preview, .text-preview {
  flex: 1;
}

.preview-iframe {
  width: 100%;
  height: 100%;
  border: none;
  flex: 1;
}

.container-info {
  padding: 16px;
  gap: 12px;
  overflow-y: auto;
}

.file-info-card {
  background: rgba(255, 255, 255, 0.04);
  border-radius: 8px;
  padding: 14px;
  margin: 12px 16px 0;
  border-left: 3px solid var(--ion-color-primary);
}

.container-error {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  background: rgba(231, 76, 60, 0.1);
  border-radius: 6px;
  color: #e74c3c;
  margin-bottom: 12px;
}
.container-error .error-icon { font-size: 20px; }
.container-error p { margin: 0; font-size: 13px; }

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
