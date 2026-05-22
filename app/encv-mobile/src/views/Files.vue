<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-button v-if="currentPath !== '/'" @click="goUp">
            <ion-icon :icon="arrowBack" slot="icon-only"></ion-icon>
          </ion-button>
        </ion-buttons>
        <ion-title>{{ t('files.title') }}</ion-title>
      </ion-toolbar>
      <ion-toolbar v-if="currentPath !== '/' && !isSearching">
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
      <ion-toolbar>
        <ion-searchbar
          v-model="searchQuery"
          :placeholder="t('files.searchPlaceholder')"
          @ionInput="handleSearchInput"
          @ionClear="handleSearchClear"
        ></ion-searchbar>
        <ion-toggle
          v-if="searchQuery"
          slot="end"
          v-model="searchRecursive"
          @ionChange="handleSearchToggle"
          class="recursive-toggle"
        >
          {{ t('files.recursive') }}
        </ion-toggle>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <ion-refresher slot="fixed" @ionRefresh="handleRefresh" v-if="!searchQuery">
        <ion-refresher-content></ion-refresher-content>
      </ion-refresher>

      <div v-if="loading || isSearching" class="loading-container">
        <ion-spinner name="crescent"></ion-spinner>
        <p>{{ isSearching ? t('files.searching') : (connecting ? t('files.connecting') : t('files.loading')) }}</p>
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

      <div v-else-if="displayFiles.length === 0" class="empty-state">
        <ion-icon :icon="searchQuery ? search : folderOpen" class="empty-icon"></ion-icon>
        <h3>{{ searchQuery ? t('files.noSearchResults') : t('files.emptyDir') }}</h3>
        <p>{{ searchQuery ? t('files.noSearchResultsDesc') : t('files.emptyDirDesc') }}</p>
      </div>

      <ion-list v-else>
        <ion-item
          v-for="file in displayFiles"
          :key="file.path"
          @click="handleFileClick(file)"
          v-longpress="() => handleLongPress(file)"
        >
          <ion-icon
            :icon="getFileIcon(file)"
            :color="getFileIconColor(file)"
            slot="start"
          ></ion-icon>
          <ion-label>
            <h2>{{ file.name }}</h2>
            <p v-if="searchQuery && !file.isDirectory" class="search-path">{{ file.path }}</p>
            <p v-if="!file.isDirectory && file.size">{{ formatFileSize(file.size) }}<span v-if="file.modified && !searchQuery"> · {{ file.modified }}</span></p>
            <p v-else-if="file.isDirectory">{{ t('files.directory') }}</p>
          </ion-label>
          <ion-badge v-if="file.isEncrypted || getFileCategory(file.name, file.isEncrypted) === 'encrypted'" color="warning" slot="end">
            ENCV
          </ion-badge>
          <div v-if="searchQuery && !file.isDirectory" class="open-folder-btn" @click.stop="openContainingFolder(file)">
            <ion-icon :icon="folderOpen" class="open-folder-icon"></ion-icon>
          </div>
        </ion-item>
      </ion-list>
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
  IonSearchbar,
  IonToggle,
  actionSheetController,
  alertController,
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
  search,
} from 'ionicons/icons'
import {
  listFiles,
  searchFiles,
  formatFileSize,
  getFileCategory,
  PermissionDeniedError,
  NotFoundError,
  deleteFile,
  createTask,
} from '@/api/encv'
import type { FileItem } from '@/api/encv'
import { eventBus } from '@/composables/useEventBus'
import { useI18n } from '@/composables/useI18n'
import { vLongpress } from '@/directives/longpress'
import { isNative, requestStoragePermission, openInPlayer } from '@/plugins/GoProcess'
import { showToast } from '@/composables/useToast'

const { t } = useI18n()
const router = useRouter()
const serverOnline = ref(false)
const noPermission = ref(false)
const files = ref<FileItem[]>([])
const currentPath = ref('/')
const loading = ref(false)
const connecting = ref(false)

const searchQuery = ref('')
const searchRecursive = ref(false)
const searchResults = ref<FileItem[] | null>(null)
const isSearching = ref(false)
let searchTimer: ReturnType<typeof setTimeout> | null = null
let searchGeneration = 0

const searchCache = new Map<string, { timestamp: number; results: FileItem[] }>()
const CACHE_TTL = 30000

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

const displayFiles = computed(() => {
  if (searchResults.value !== null) return searchResults.value
  return sortedFiles.value
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
  const category = getFileCategory(file.name, file.isEncrypted)
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
  const category = getFileCategory(file.name, file.isEncrypted)
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

      if (error instanceof NotFoundError) {
        serverOnline.value = true
        loading.value = false
        connecting.value = false
        if (currentPath.value !== '/') {
          showToast({ message: t('files.pathNotFound'), duration: 2000, color: 'warning' })
          goUp()
        }
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
    if (error instanceof NotFoundError) {
      serverOnline.value = true
      if (currentPath.value !== '/') {
        goUp()
      }
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
  searchQuery.value = ''
  searchResults.value = null
  loadFiles()
}

function openContainingFolder(file: FileItem) {
  const parentDir = file.path.substring(0, file.path.lastIndexOf('/')) || '/'
  searchQuery.value = ''
  searchResults.value = null
  navigateTo(parentDir)
}

function goUp() {
  if (currentPath.value === '/') return
  const parts = currentPath.value.split('/').filter(Boolean)
  parts.pop()
  currentPath.value = parts.length === 0 ? '/' : '/' + parts.join('/')
  searchQuery.value = ''
  searchResults.value = null
  loadFiles()
}

async function handleFileClick(file: FileItem) {
  if (file.isDirectory) {
    const newPath = currentPath.value === '/'
      ? '/' + file.name
      : currentPath.value + '/' + file.name
    navigateTo(newPath)
    return
  }

  const category = getFileCategory(file.name, file.isEncrypted)
  console.info('[Files] Click:', file.name, 'category:', category)
  if (category === 'video' || category === 'audio' || category === 'encrypted') {
    if (isNative()) {
      const mimeType = category === 'encrypted' ? 'video/*' : category === 'video' ? 'video/*' : 'audio/*'
      openInPlayer(file.path, file.name, mimeType)
    } else {
      router.push({
        path: '/player',
        query: { path: file.path, name: file.name },
      })
    }
  } else {
    router.push({
      path: '/tabs/preview',
      query: { path: file.path, name: file.name },
    })
  }
}

function handleSearchInput() {
  if (searchTimer) clearTimeout(searchTimer)
  const query = searchQuery.value.trim()
  if (!query) {
    searchGeneration++
    searchResults.value = null
    isSearching.value = false
    return
  }
  searchTimer = setTimeout(() => performSearch(), searchRecursive.value ? 600 : 300)
}

function handleSearchClear() {
  searchGeneration++
  searchQuery.value = ''
  searchResults.value = null
  isSearching.value = false
}

function handleSearchToggle() {
  if (searchQuery.value.trim()) {
    performSearch()
  }
}

async function performSearch() {
  const query = searchQuery.value.trim()
  if (!query) return

  if (isSearching.value) {
    searchGeneration++
  }
  const gen = ++searchGeneration

  const cacheKey = `${currentPath.value}:${query}:${searchRecursive.value}`
  const cached = searchCache.get(cacheKey)
  if (cached && Date.now() - cached.timestamp < CACHE_TTL) {
    searchResults.value = cached.results
    return
  }

  isSearching.value = true
  try {
    const results = await searchFiles(currentPath.value, query, searchRecursive.value)
    if (gen !== searchGeneration) return
    searchResults.value = results
    searchCache.set(cacheKey, { timestamp: Date.now(), results })
  } catch {
    if (gen !== searchGeneration) return
    searchResults.value = []
  }
  isSearching.value = false
}

async function handleLongPress(file: FileItem) {
  const category = file.isDirectory ? 'directory' : getFileCategory(file.name, file.isEncrypted)

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
        if (isNative()) {
          openInPlayer(file.path, file.name, 'video/*')
        } else {
          router.push({
            path: '/player',
            query: { path: file.path, name: file.name },
          })
        }
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
          if (isNative()) {
            const mimeType = category === 'audio' ? 'audio/*' : 'video/*'
            openInPlayer(file.path, file.name, mimeType)
          } else {
            router.push({
              path: '/player',
              query: { path: file.path, name: file.name },
            })
          }
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
  const alert = await alertController.create({
    header: t('files.encrypt'),
    message: t('files.encryptPrompt', { name: file.name }),
    buttons: [
      { text: t('files.cancelSelect'), role: 'cancel' },
      {
        text: t('files.encrypt'),
        handler: async () => {
          try {
            await createTask('encrypt', file.path)
            showToast({ message: t('tasks.taskCreated'), duration: 1500, color: 'success' })
          } catch {
            showToast({ message: t('tasks.taskCreateFailed'), duration: 2000, color: 'danger' })
          }
        },
      },
    ],
  })
  await alert.present()
}

async function handleDecryptFile(file: FileItem) {
  const alert = await alertController.create({
    header: t('files.decrypt'),
    message: t('files.decryptPrompt', { name: file.name }),
    buttons: [
      { text: t('files.cancelSelect'), role: 'cancel' },
      {
        text: t('files.decrypt'),
        handler: async () => {
          try {
            await createTask('decrypt', file.path)
            showToast({ message: t('tasks.taskCreated'), duration: 1500, color: 'success' })
          } catch {
            showToast({ message: t('tasks.taskCreateFailed'), duration: 2000, color: 'danger' })
          }
        },
      },
    ],
  })
  await alert.present()
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
            showToast({ message: t('files.deleteFailed'), duration: 2000, color: 'danger' })
          }
        },
      },
    ],
  })
  await alert.present()
}

function onFileChange() {
  searchCache.clear()
  loadFiles()
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
  if (searchTimer) clearTimeout(searchTimer)
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

.recursive-toggle {
  margin-right: 8px;
  font-size: 12px;
}

.search-path {
  font-size: 11px;
  color: var(--encv-text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.open-folder-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 44px;
  min-height: 44px;
  margin-right: -12px;
  cursor: pointer;
}

.open-folder-icon {
  font-size: 20px;
  color: var(--ion-color-primary);
}
</style>
