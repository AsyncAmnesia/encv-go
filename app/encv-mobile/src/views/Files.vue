<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-button v-if="currentPath !== '/'" @click="goUp">
            <ion-icon :icon="arrowBack" slot="icon-only"></ion-icon>
          </ion-button>
        </ion-buttons>
        <ion-title>{{ isPickerMode ? t('files.selectFile') : t('files.title') }}</ion-title>
        <ion-buttons v-if="isPickerMode" slot="end">
          <ion-button @click="handleCancelPicker">{{ t('files.cancelSelect') }}</ion-button>
        </ion-buttons>
      </ion-toolbar>
      <ion-toolbar v-if="currentPath !== '/'">
        <div class="breadcrumb-scroll">
          <div class="breadcrumb">
            <span class="breadcrumb-item" @click="navigateTo('/')">{{ t('files.root') }}</span>
            <span v-for="(segment, index) in pathSegments" :key="index" class="breadcrumb-segment">
              <ion-icon :icon="chevronForward" class="breadcrumb-sep"></ion-icon>
              <span class="breadcrumb-item" @click="navigateTo(segment.path)">{{ segment.name }}</span>
            </span>
          </div>
        </div>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <ion-refresher slot="fixed" @ionRefresh="handleRefresh">
        <ion-refresher-content></ion-refresher-content>
      </ion-refresher>

      <div v-if="loading" class="loading-container">
        <ion-spinner name="crescent"></ion-spinner>
        <p>{{ connecting ? t('files.connecting') : t('files.loading') }}</p>
      </div>

      <div v-else-if="noPermission" class="empty-state">
        <ion-icon :icon="lockClosed" class="empty-icon"></ion-icon>
        <h3>{{ t('files.noPermission') }}</h3>
        <p>{{ t('files.noPermissionDesc') }}</p>
        <ion-button @click="handleRequestStorage">
          <ion-icon :icon="folderOpen" slot="start"></ion-icon>
          {{ t('files.grantPermission') }}
        </ion-button>
      </div>

      <div v-else-if="!serverOnline" class="empty-state">
        <ion-icon :icon="cloudOffline" class="empty-icon"></ion-icon>
        <h3>{{ t('files.serverOffline') }}</h3>
        <p>{{ t('files.serverOfflineDesc') }}</p>
        <ion-button @click="retryConnection">
          <ion-icon :icon="refresh" slot="start"></ion-icon>
          {{ t('files.retry') }}
        </ion-button>
      </div>

      <div v-else-if="files.length === 0" class="empty-state">
        <ion-icon :icon="folderOpen" class="empty-icon"></ion-icon>
        <h3>{{ t('files.emptyDir') }}</h3>
        <p>{{ t('files.emptyDirDesc') }}</p>
      </div>

      <ion-list v-else>
        <ion-item
          v-for="file in sortedFiles"
          :key="file.path"
          @click="handleFileClick(file)"
          @longpress.prevent="handleLongPress(file)"
          detail
        >
          <ion-icon
            :icon="getFileIcon(file)"
            :color="getFileIconColor(file)"
            slot="start"
          ></ion-icon>
          <ion-label>
            <h2>{{ file.name }}</h2>
            <p v-if="!file.isDirectory && file.size">{{ formatFileSize(file.size) }}<span v-if="file.modified"> · {{ file.modified }}</span></p>
            <p v-else-if="file.isDirectory">{{ t('files.directory') }}</p>
          </ion-label>
          <ion-badge v-if="getFileCategory(file.name) === 'encrypted'" color="warning" slot="end">
            ENCV
          </ion-badge>
        </ion-item>
      </ion-list>

      <div v-if="isPickerMode && !loading && serverOnline && files.length > 0" class="picker-hint">
        {{ t('files.tapToSelect') }}
      </div>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  IonPage,
  IonHeader,
  IonToolbar,
  IonTitle,
  IonButtons,
  IonButton,
  IonContent,
  IonRefresher,
  IonRefresherContent,
  IonList,
  IonItem,
  IonIcon,
  IonLabel,
  IonBadge,
  IonSpinner,
  actionSheetController,
  alertController,
  toastController,
} from '@ionic/vue'
import {
  arrowBack,
  chevronForward,
  folder,
  folderOpen,
  videocam,
  musicalNotes,
  image,
  document,
  documentText,
  lockClosed,
  cloudOffline,
  refresh,
  trash,
} from 'ionicons/icons'
import {
  listFiles,
  formatFileSize,
  getFileCategory,
  PermissionDeniedError,
  deleteFile,
  createTask,
} from '@/api/encv'
import type { FileItem } from '@/api/encv'
import { eventBus } from '@/composables/useEventBus'
import { useI18n } from '@/composables/useI18n'
import { useFilePicker } from '@/composables/useFilePicker'
import { isNative, requestStoragePermission } from '@/plugins/GoProcess'

const { t } = useI18n()
const router = useRouter()
const { isPickerMode, confirmSelection, cancelPicking } = useFilePicker()
const serverOnline = ref(false)
const noPermission = ref(false)
const files = ref<FileItem[]>([])
const currentPath = ref('/')
const loading = ref(false)
const connecting = ref(false)

const MAX_RETRIES = isNative() ? 15 : 3
const RETRY_DELAY = 1000

const pathSegments = computed(() => {
  if (currentPath.value === '/') return []
  const parts = currentPath.value.split('/').filter(Boolean)
  return parts.map((name, index) => ({
    name,
    path: '/' + parts.slice(0, index + 1).join('/'),
  }))
})

const sortedFiles = computed(() => {
  return [...files.value].sort((a, b) => {
    if (a.isDirectory && !b.isDirectory) return -1
    if (!a.isDirectory && b.isDirectory) return 1
    return a.name.localeCompare(b.name)
  })
})

function getFileIcon(file: FileItem) {
  if (file.isDirectory) return folder
  const category = getFileCategory(file.name)
  switch (category) {
    case 'video': return videocam
    case 'audio': return musicalNotes
    case 'image': return image
    case 'document': return document
    case 'encrypted': return lockClosed
    default: return documentText
  }
}

function getFileIconColor(file: FileItem) {
  if (file.isDirectory) return 'primary'
  const category = getFileCategory(file.name)
  switch (category) {
    case 'video': return 'danger'
    case 'audio': return 'tertiary'
    case 'image': return 'success'
    case 'encrypted': return 'warning'
    default: return 'medium'
  }
}

let loadGeneration = 0

async function loadFiles() {
  console.info('[Files] Loading files, path:', currentPath.value)
  const gen = ++loadGeneration
  loading.value = true
  connecting.value = false
  noPermission.value = false

  for (let attempt = 0; attempt <= MAX_RETRIES; attempt++) {
    if (gen !== loadGeneration) return

    try {
      files.value = await listFiles(currentPath.value)
      serverOnline.value = true
      noPermission.value = false
      loading.value = false
      connecting.value = false
      console.info('[Files] Loaded', files.value.length, 'files')
      return
    } catch (error) {
      if (error instanceof PermissionDeniedError) {
        serverOnline.value = true
        noPermission.value = true
        loading.value = false
        connecting.value = false
        return
      }

      if (attempt < MAX_RETRIES) {
        connecting.value = true
        await new Promise(r => setTimeout(r, RETRY_DELAY))
      }
    }
  }

  if (gen !== loadGeneration) return
  serverOnline.value = false
  loading.value = false
  connecting.value = false
}

async function handleRefresh(event: CustomEvent) {
  try {
    files.value = await listFiles(currentPath.value)
    serverOnline.value = true
    noPermission.value = false
  } catch (error) {
    if (error instanceof PermissionDeniedError) {
      serverOnline.value = true
      noPermission.value = true
    }
  }
  ;(event.target as any)?.complete?.()
}

async function retryConnection() {
  await loadFiles()
}

async function handleRequestStorage() {
  console.info('[Files] Requesting storage permission')
  await requestStoragePermission()
  setTimeout(() => loadFiles(), 1500)
}

function navigateTo(path: string) {
  currentPath.value = path
  loadFiles()
}

function goUp() {
  if (currentPath.value === '/') return
  const parts = currentPath.value.split('/').filter(Boolean)
  parts.pop()
  currentPath.value = parts.length === 0 ? '/' : '/' + parts.join('/')
  loadFiles()
}

async function handleFileClick(file: FileItem) {
  if (isPickerMode.value) {
    if (file.isDirectory) {
      const newPath = currentPath.value === '/'
        ? '/' + file.name
        : currentPath.value + '/' + file.name
      navigateTo(newPath)
      return
    }
    confirmSelection(file.path, file.name)
    router.back()
    return
  }

  if (file.isDirectory) {
    const newPath = currentPath.value === '/'
      ? '/' + file.name
      : currentPath.value + '/' + file.name
    navigateTo(newPath)
    return
  }

  const category = getFileCategory(file.name)
  console.info('[Files] Click:', file.name, 'category:', category)
  if (category === 'video' || category === 'audio' || category === 'encrypted') {
    router.push({
      path: '/tabs/player',
      query: { path: file.path, name: file.name },
    })
  } else {
    router.push({
      path: '/tabs/preview',
      query: { path: file.path, name: file.name },
    })
  }
}

function handleCancelPicker() {
  cancelPicking()
  router.back()
}

async function handleLongPress(file: FileItem) {
  const category = file.isDirectory ? 'directory' : getFileCategory(file.name)

  const buttons: any[] = []

  if (file.isDirectory) {
    buttons.push({
      text: t('files.open'),
      icon: folderOpen,
      handler: () => {
        const newPath = currentPath.value === '/'
          ? '/' + file.name
          : currentPath.value + '/' + file.name
        navigateTo(newPath)
      },
    })
    buttons.push({
      text: t('files.encrypt'),
      icon: lockClosed,
      handler: () => {
        handleEncryptFile(file)
      },
    })
  } else if (category === 'encrypted') {
    buttons.push({
      text: t('files.play'),
      icon: videocam,
      handler: () => {
        router.push({
          path: '/tabs/player',
          query: { path: file.path, name: file.name },
        })
      },
    })
    buttons.push({
      text: t('files.decrypt'),
      icon: lockClosed,
      handler: () => {
        handleDecryptFile(file)
      },
    })
    buttons.push({
      text: t('files.delete'),
      icon: trash,
      role: 'destructive',
      handler: () => {
        handleDeleteFile(file)
      },
    })
  } else {
    const isMedia = category === 'video' || category === 'audio'
    buttons.push({
      text: isMedia ? t('files.play') : t('files.preview'),
      icon: isMedia ? videocam : image,
      handler: () => {
        if (isMedia) {
          router.push({
            path: '/tabs/player',
            query: { path: file.path, name: file.name },
          })
        } else {
          router.push({
            path: '/tabs/preview',
            query: { path: file.path, name: file.name },
          })
        }
      },
    })
    buttons.push({
      text: t('files.encrypt'),
      icon: lockClosed,
      handler: () => {
        handleEncryptFile(file)
      },
    })
    buttons.push({
      text: t('files.delete'),
      icon: trash,
      role: 'destructive',
      handler: () => {
        handleDeleteFile(file)
      },
    })
  }

  buttons.push({
    text: t('files.cancelSelect'),
    role: 'cancel',
  })

  const actionSheet = await actionSheetController.create({
    header: file.name,
    buttons,
  })
  await actionSheet.present()
}

async function handleEncryptFile(file: FileItem) {
  try {
    await createTask('encrypt', file.path)
    const toast = await toastController.create({
      message: t('tasks.taskCreated'),
      duration: 1500,
      color: 'success',
    })
    await toast.present()
  } catch {
    const toast = await toastController.create({
      message: t('tasks.taskCreateFailed'),
      duration: 2000,
      color: 'danger',
    })
    await toast.present()
  }
}

async function handleDecryptFile(file: FileItem) {
  try {
    await createTask('decrypt', file.path)
    const toast = await toastController.create({
      message: t('tasks.taskCreated'),
      duration: 1500,
      color: 'success',
    })
    await toast.present()
  } catch {
    const toast = await toastController.create({
      message: t('tasks.taskCreateFailed'),
      duration: 2000,
      color: 'danger',
    })
    await toast.present()
  }
}

async function handleDeleteFile(file: FileItem) {
  const alert = await alertController.create({
    header: t('files.delete'),
    message: t('files.deleteConfirm', { name: file.name }),
    buttons: [
      {
        text: t('files.cancelSelect'),
        role: 'cancel',
      },
      {
        text: t('files.delete'),
        role: 'destructive',
        handler: async () => {
          try {
            await deleteFile(file.path)
            await loadFiles()
          } catch {
            const toast = await toastController.create({
              message: t('files.deleteFailed'),
              duration: 2000,
              color: 'danger',
            })
            await toast.present()
          }
        },
      },
    ],
  })
  await alert.present()
}

function onFileChange(data: { path: string; action: string }) {
  const dir = data.path.substring(0, data.path.lastIndexOf('/')) || '/'
  if (dir === currentPath.value) {
    loadFiles()
  }
}

function onBackendReady(data: { port?: number; running?: boolean }) {
  if (data.running || data.port) {
    loadFiles()
  }
}

onMounted(() => {
  loadFiles()
  eventBus.on('file:change', onFileChange)
  window.addEventListener('encv:backend-ready', onBackendReadyWindow as EventListener)
})

onUnmounted(() => {
  eventBus.off('file:change', onFileChange)
  window.removeEventListener('encv:backend-ready', onBackendReadyWindow as EventListener)
})

function onBackendReadyWindow(event: Event) {
  const detail = (event as CustomEvent).detail || {}
  onBackendReady(detail)
}
</script>

<style scoped>
.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 50%;
  color: var(--encv-text-secondary);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 50%;
  padding: 24px;
  text-align: center;
  color: var(--encv-text-secondary);
}

.empty-icon {
  font-size: 64px;
  margin-bottom: 16px;
  opacity: 0.5;
}

.breadcrumb-scroll {
  --background: transparent;
}

.breadcrumb {
  display: flex;
  align-items: center;
  padding: 0 16px;
  white-space: nowrap;
}

.breadcrumb-item {
  cursor: pointer;
  color: var(--ion-color-primary);
  font-size: 14px;
}

.breadcrumb-item:hover {
  text-decoration: underline;
}

.breadcrumb-sep {
  font-size: 14px;
  margin: 0 4px;
  color: var(--encv-text-secondary);
}

.breadcrumb-segment {
  display: flex;
  align-items: center;
}

.picker-hint {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  padding: 12px 16px;
  text-align: center;
  font-size: 14px;
  color: var(--ion-color-primary);
  background: var(--ion-background-color, #fff);
  border-top: 1px solid var(--ion-color-light, #f4f5f8);
  z-index: 10;
}
</style>
