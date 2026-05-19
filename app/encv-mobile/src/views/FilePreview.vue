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
          <ion-button @click="copyContent" fill="clear" size="small">
            <ion-icon :icon="copyOutline" slot="icon-only"></ion-icon>
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

      <div v-else class="preview-wrapper">
        <div class="file-meta">
          <span class="meta-item"><ion-icon :icon="documentTextOutline"></ion-icon> {{ filePath }}</span>
          <span class="meta-item"><ion-icon :icon="informationCircle"></ion-icon> {{ formatFileSize(fileSize) }}</span>
          <span v-if="encoding !== 'utf-8'" class="meta-item warn">
            <ion-icon :icon="warning"></ion-icon> {{ encoding }}
          </span>
        </div>
        <pre class="file-content"><code>{{ content }}</code></pre>
      </div>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons,
  IonButton, IonIcon, IonContent, IonSpinner, toastController,
} from '@ionic/vue'
import {
  arrowBack, copyOutline, documentTextOutline,
  alertCircle, informationCircle, warning,
} from 'ionicons/icons'
import { readFileContent, formatFileSize } from '@/api/encv'
import type { FileContentResponse } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'

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

async function loadFile() {
  const path = (route.query.path as string) || ''
  if (!path) {
    error.value = t('filePreview.noPath')
    loading.value = false
    return
  }
  filePath.value = path
  fileName.value = path.split('/').pop() || path
  loading.value = true
  error.value = ''
  try {
    const data: FileContentResponse = await readFileContent(path)
    content.value = data.content
    fileSize.value = data.size
    encoding.value = data.encoding
  } catch (e: any) {
    error.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}

async function copyContent() {
  try {
    await navigator.clipboard.writeText(content.value)
    const toast = await toastController.create({
      message: t('filePreview.copied'),
      duration: 1500,
      position: 'middle',
      color: 'success',
    })
    await toast.present()
  } catch {
    const textArea = document.createElement('textarea')
    textArea.value = content.value
    document.body.appendChild(textArea)
    textArea.select()
    document.execCommand('copy')
    document.body.removeChild(textArea)
    const toast = await toastController.create({
      message: t('filePreview.copied'),
      duration: 1500,
      position: 'middle',
      color: 'success',
    })
    await toast.present()
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
