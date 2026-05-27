<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-button v-if="currentPath !== '/'" @click="goUp">
            <ion-icon :icon="arrowBack" slot="icon-only"></ion-icon>
          </ion-button>
        </ion-buttons>
        <ion-buttons slot="end">
          <ion-button fill="clear" @click="openSideDrawer()">
            <ion-icon :icon="menuOutline" slot="icon-only" />
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
        <div v-if="!searchQuery" class="sort-btn-wrapper">
          <ion-button fill="clear" size="small" @click="cycleSort">
            <ion-icon :icon="swapVertical" slot="start"></ion-icon>
            {{ sortLabel }}
          </ion-button>
        </div>
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

    <ion-menu side="start" menu-id="plugin-menu" content-id="main-content">
      <ion-header>
        <ion-toolbar>
          <ion-title>插件分类</ion-title>
        </ion-toolbar>
      </ion-header>
      <ion-content>
        <ion-list>
          <ion-item button @click="exitPluginMode()" detail>
            <ion-icon :icon="folder" slot="start" color="primary"></ion-icon>
            <ion-label><h2>所有文件</h2><p>{{ currentPath || '/' }}</p></ion-label>
          </ion-item>
          <ion-list-header>文件类型</ion-list-header>
          <ion-item v-for="plugin in plugins" :key="plugin.name" button detail @click="openPluginView(plugin)">
            <ion-icon :icon="getPluginIcon(plugin.name)" slot="start" color="primary" />
            <ion-label>
              <h2>{{ plugin.name }}</h2>
              <p>{{ plugin.supportedExtensions.length }} 种格式 · 容器 {{ plugin.containerExtension }}</p>
            </ion-label>
          </ion-item>
        </ion-list>

        <ion-list v-if="tags.length > 0" style="margin-top: 16px">
          <ion-list-header>标签</ion-list-header>
          <ion-item v-for="tag in tags" :key="tag.name" button @click="handleTagFilter(tag.name)">
            <ion-icon :icon="pricetagOutline" slot="start" color="success" />
            <ion-label>
              <h2>{{ tag.name }}</h2>
              <p>{{ tag.count }} 个文件</p>
            </ion-label>
          </ion-item>
        </ion-list>
      </ion-content>
    </ion-menu>

    <ion-content id="main-content">
      <ion-refresher slot="fixed" @ionRefresh="handleRefresh" v-if="!searchQuery">
        <ion-refresher-content></ion-refresher-content>
      </ion-refresher>

      <template v-if="(loading || isSearching || noPermission || !serverOnline || displayFiles.length === 0) && !selectedPlugin">
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
        <div v-else class="empty-state">
          <ion-icon :icon="searchQuery ? search : folderOpen" class="empty-icon"></ion-icon>
          <h3>{{ searchQuery ? t('files.noSearchResults') : t('files.emptyDir') }}</h3>
          <p>{{ searchQuery ? t('files.noSearchResultsDesc') : t('files.emptyDirDesc') }}</p>
        </div>
      </template>

      <template v-else>
        <div v-if="selectedPlugin">
          <ion-toolbar>
            <ion-buttons slot="start">
              <ion-back-button @click="exitPluginMode()" />
            </ion-buttons>
            <ion-title>{{ selectedPlugin.name }} 文件</ion-title>
          </ion-toolbar>
          <div v-if="!pluginLoaded" class="loading-container">
            <ion-spinner name="crescent"></ion-spinner>
            <p>{{ t('files.loading') }}</p>
          </div>
          <template v-else>
            <div v-if="pluginFiles.length === 0" class="empty-state">
              <ion-icon :icon="folderOpen" class="empty-icon"></ion-icon>
              <h3>{{ t('files.emptyDir') }}</h3>
              <p>{{ t('settings.emptyPluginDesc', { name: selectedPlugin?.name }) || '该类型下暂无文件' }}</p>
            </div>
            <template v-else>
            <ion-segment v-model="pluginTab" value="source">
              <ion-segment-button value="source">未加密</ion-segment-button>
              <ion-segment-button value="container">已加密</ion-segment-button>
            </ion-segment>
            <ion-item button detail @click="showPluginFilters = !showPluginFilters">
              <ion-icon :icon="filterOutline" slot="start"></ion-icon>
              <ion-label>筛选</ion-label>
              <ion-badge v-if="activeFilterCount > 0" slot="end" color="primary">{{ activeFilterCount }}</ion-badge>
            </ion-item>
            <ion-list v-if="showPluginFilters" :inset="true">
              <ion-item>
                <ion-label position="stacked">大小范围</ion-label>
                <div style="display:flex;gap:8px;align-items:center;width:100%">
                  <ion-input type="number" placeholder="最小"
                    :value="sizeFilterMin !== null ? String(sizeFilterMin) : ''"
                    @ionInput="sizeFilterMin = $event.detail.value ? Number($event.detail.value) : null">
                  </ion-input>
                  <span>~</span>
                  <ion-input type="number" placeholder="最大"
                    :value="sizeFilterMax !== null ? String(sizeFilterMax) : ''"
                    @ionInput="sizeFilterMax = $event.detail.value ? Number($event.detail.value) : null">
                  </ion-input>
                  <ion-button fill="clear" size="small" @click="sizeFilterMin=null;sizeFilterMax=null">清除</ion-button>
                </div>
                <div class="filter-chips">
                  <ion-chip v-for="p in SIZE_PRESETS" :key="p.label" outline @click="applySizePreset(p)">{{ p.label }}</ion-chip>
                </div>
              </ion-item>
              <ion-item>
                <ion-label position="stacked">修改时间</ion-label>
                <div style="display:flex;gap:8px;align-items:center;width:100%">
                  <ion-input type="date" placeholder="起始"
                    :value="timeFilterFrom || ''"
                    @ionInput="timeFilterFrom = ($event.detail.value as string) || null">
                  </ion-input>
                  <span>~</span>
                  <ion-input type="date" placeholder="结束"
                    :value="timeFilterTo || ''"
                    @ionInput="timeFilterTo = ($event.detail.value as string) || null">
                  </ion-input>
                  <ion-button fill="clear" size="small" @click="timeFilterFrom=null;timeFilterTo=null">清除</ion-button>
                </div>
                <div class="filter-chips">
                  <ion-chip v-for="p in TIME_PRESETS" :key="p.label" outline @click="applyTimePreset(p)">{{ p.label }}</ion-chip>
                </div>
              </ion-item>
              <ion-item button @click="clearAllPluginFilters">
                <ion-icon :icon="closeCircleOutline" slot="start" color="danger"></ion-icon>
                <ion-label color="danger">清除所有筛选</ion-label>
              </ion-item>
            </ion-list>
            <ion-list :inset="true">
            <ion-item v-for="file in filteredPluginFiles" :key="file.path" button @click="handleFileClick(file)" v-longpress="() => handleLongPress(file)">
              <div slot="start" class="file-thumbnail-slot lazy-thumb-target" :data-file-path="file.path">
                <img
                  v-if="isImageFile(file) && thumbnailUrls[file.path]"
                  :src="thumbnailUrls[file.path]"
                  class="file-thumb"
                  loading="lazy"
                  @error="onThumbError(file.path)"
                />
                <ion-icon
                  v-else
                  :icon="getFileIcon(file)"
                  :color="getFileIconColor(file)"
                  :class="{ 'thumb-fallback': isImageFile(file) }"
                ></ion-icon>
              </div>
              <ion-label>
                <h2>{{ file.name }}</h2>
                <p v-if="!file.isDirectory && file.size">{{ formatFileSize(file.size) }}<span v-if="file.modified"> · {{ formatDateTime(file.modified) }}</span></p>
                <p v-else-if="file.isDirectory">{{ t('files.directory') }}</p>
                <div v-if="!file.isDirectory && file._tags && file._tags.length > 0" class="file-tag-chips">
                  <ion-chip v-for="tag in file._tags" :key="tag" size="small" color="tertiary" outline>{{ tag }}</ion-chip>
                </div>
              </ion-label>
              <ion-badge v-if="file.isEncrypted || getFileCategory(file.name, file.isEncrypted) === 'encrypted'" color="warning" slot="end">
                ENCV
              </ion-badge>
            </ion-item>
            <ion-item v-if="filteredPluginFiles.length === 0">
              <ion-label>无匹配文件</ion-label>
            </ion-item>
          </ion-list>
          </template>
          </template>
        </div>

        <ion-list>
          <ion-item
            v-for="file in displayFiles"
            :key="file.path"
            @click="handleFileClick(file)"
            v-longpress="() => handleLongPress(file)"
          >
            <div slot="start" class="file-thumbnail-slot lazy-thumb-target" :data-file-path="file.path">
                <img
                  v-if="isImageFile(file) && thumbnailUrls[file.path]"
                  :src="thumbnailUrls[file.path]"
                  class="file-thumb"
                  loading="lazy"
                  @error="onThumbError(file.path)"
                />
                <ion-icon
                  v-else
                  :icon="getFileIcon(file)"
                  :color="getFileIconColor(file)"
                  :class="{ 'thumb-fallback': isImageFile(file) }"
                ></ion-icon>
              </div>
            <ion-label>
              <h2>{{ file.name }}</h2>
              <p v-if="searchQuery && !file.isDirectory" class="search-path">{{ file.path }}</p>
              <p v-if="!file.isDirectory && file.size">{{ formatFileSize(file.size) }}<span v-if="file.modified && !searchQuery"> · {{ formatDateTime(file.modified) }}</span></p>
              <p v-else-if="file.isDirectory">{{ t('files.directory') }}</p>
              <div v-if="!file.isDirectory && !searchQuery && file._tags && file._tags.length > 0" class="file-tag-chips">
                <ion-chip v-for="tag in file._tags" :key="tag" size="small" color="tertiary" outline>{{ tag }}</ion-chip>
              </div>
            </ion-label>
            <ion-badge v-if="file.isEncrypted || getFileCategory(file.name, file.isEncrypted) === 'encrypted'" color="warning" slot="end">
              ENCV
            </ion-badge>
            <ion-button v-if="searchQuery" slot="end" fill="clear" class="open-folder-btn" @click.stop="openContainingFolder(file)">
              <ion-icon :icon="folderOpen" class="open-folder-icon" slot="icon-only"></ion-icon>
            </ion-button>
          </ion-item>
        </ion-list>
      </template>

      <ion-alert :is-open="showRenameDialog" header="重命名"
        :inputs="[{ name: 'name', type: 'text', placeholder: '新文件名', value: renameValue }]"
        :buttons="[
          { text: '取消', role: 'cancel' },
          { text: '确定', handler: (d: any) => { renameValue = d.name; handleRename(selectedFile!); } }
        ]"
        @didDismiss="showRenameDialog = false" />
      <ion-modal :is-open="showTagDialog" @didDismiss="showTagDialog = false">
        <ion-header>
          <ion-toolbar>
            <ion-title>标签管理</ion-title>
            <ion-buttons slot="end">
              <ion-button @click="showTagDialog = false">完成</ion-button>
            </ion-buttons>
          </ion-toolbar>
        </ion-header>
        <ion-content>
          <div class="tag-editor-content">
            <div v-if="editingFileTags.length > 0" class="existing-tags">
              <ion-chip v-for="tag in editingFileTags" :key="tag" color="primary" outline>
                {{ tag }}
                <ion-icon :icon="closeCircle" @click="handleRemoveTag(tag)"></ion-icon>
              </ion-chip>
            </div>
            <p v-else class="no-tags-hint">暂无标签</p>
            <div class="tag-input-row">
              <ion-input v-model="newTagInput" placeholder="输入新标签名" enterkeyhint="go" @keyup.enter="handleAddNewTag()"></ion-input>
              <ion-button fill="solid" color="primary" @click="handleAddNewTag()" :disabled="!newTagInput.trim()">
                添加
              </ion-button>
            </div>
          </div>
        </ion-content>
      </ion-modal>
      <ion-alert :is-open="showMoveDialog" header="移动到"
        :inputs="[{ name: 'target', type: 'text', placeholder: '目标路径', value: moveTargetPath }]"
        :buttons="[
          { text: '取消', role: 'cancel' },
          { text: '移动', handler: (d: any) => { moveTargetPath = d.target; handleMove(selectedFile!); } }
        ]"
        @didDismiss="showMoveDialog = false" />

    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { onIonViewWillEnter } from '@ionic/vue'
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
  menuController,
  IonAlert,
  IonMenu,
  IonSegment,
  IonSegmentButton,
  IonListHeader,
  IonBackButton,
  IonChip,
  IonModal,
  IonInput,
} from '@ionic/vue'
import {
  arrowBack,
  chevronForward,
  folder,
  folderOpen,
  videocam,
  image,
  lockClosed,
  cloudOffline,
  refresh,
  trash,
  search,
  informationCircle,
  menuOutline,
  pricetagOutline,
  createOutline,
  copyOutline,
  arrowForwardOutline,
  shareOutline,
  closeCircle,
  closeCircleOutline,
  filterOutline,
  swapVertical,
} from 'ionicons/icons'
import {
  listFiles,
  listFilesStream,
  listPluginFilesStream,
  searchFiles,
  formatFileSize,
  getFileCategory,
  PermissionDeniedError,
  NotFoundError,
  deleteFile,
  renameFile,
  copyFile,
  moveFile,
  fetchPlugins,
  fetchTags,
  addTag,
  removeTag,
  listFilesByTag,
} from '@/api/encv'
import type { FileItem, PluginMeta, TagInfo } from '@/api/encv'
import { eventBus } from '@/composables/useEventBus'
import { useI18n } from '@/composables/useI18n'
import { formatDateTime } from '@/composables/useDateFormat'
import { useThumbnailCache } from '@/composables/useThumbnailCache'
import {
  isImageFile,
  getFileIcon,
  getFileIconColor,
  useFileListSort,
  sortFiles,
} from '@/composables/useFileList'
import { vLongpress } from '@/directives/longpress'
import { isNative, requestStoragePermission, openPlayer, openExternal, getLocalFilePath } from '@/plugins/GoProcess'
import { getExternalStreamUrl } from '@/api/encv'
import { showToast } from '@/composables/useToast'
import { Share } from '@capacitor/share'
import { PLAY_MODE, type PlayMode, VIDEO_DEFAULT, AUDIO_DEFAULT } from '@/constants/player'

function getPlayMode(mediaType: 'video' | 'audio'): PlayMode {
  const key = mediaType === 'video' ? 'encv_player_video' : 'encv_player_audio'
  const stored = localStorage.getItem(key)
  if (stored === PLAY_MODE.ARTPLAYER || stored === PLAY_MODE.MPV_PLUGIN || stored === PLAY_MODE.EXTERNAL) return stored as PlayMode
  return mediaType === 'video' ? VIDEO_DEFAULT : AUDIO_DEFAULT
}

function playMedia(file: FileItem, category: string) {
  const isVideo = category === 'video'
  const mediaType = isVideo ? 'video' : 'audio'
  const mimeType = isVideo ? 'video/*' : 'audio/*'
  const mode = getPlayMode(mediaType)

  switch (mode) {
    case PLAY_MODE.ARTPLAYER:
      router.push({ path: '/player', query: { path: file.path, name: file.name } })
      break
    case PLAY_MODE.MPV_PLUGIN:
      if (isNative()) {
        openPlayer(file.path, file.name, mimeType)
      } else {
        router.push({ path: '/player', query: { path: file.path, name: file.name } })
      }
      break
    case PLAY_MODE.EXTERNAL:
      if (isNative()) {
        const url = getExternalStreamUrl(file.path)
        openExternal(url, mimeType)
      } else {
        router.push({ path: '/player', query: { path: file.path, name: file.name } })
      }
      break
  }
}

const { t } = useI18n()
const { thumbnailUrls, setupLazyThumbnails, onThumbError } = useThumbnailCache()
const { sortBy, sortDesc, sortLabel, cycleSort } = useFileListSort()
const router = useRouter()
const serverOnline = ref(false)
const noPermission = ref(false)
const files = ref<FileItem[]>([])
const plugins = ref<PluginMeta[]>([])
const tags = ref<TagInfo[]>([])
const showRenameDialog = ref(false)
const showTagDialog = ref(false)
const showMoveDialog = ref(false)
const selectedPlugin = ref<PluginMeta | null>(null)
const selectedFile = ref<FileItem | null>(null)
const renameValue = ref('')
const moveTargetPath = ref('')
const editingFileTags = ref<string[]>([])
const newTagInput = ref('')
const fileTagMap = ref<Record<string, string[]>>({})

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
  const raw = searchResults.value !== null ? searchResults.value : sortedFiles.value
  const tagMap = fileTagMap.value
  return raw.map(f => ({ ...f, _tags: tagMap[f.path] || [] }))
})

const sortedFiles = computed(() => {
  return sortFiles(files.value, sortBy.value, sortDesc.value)
})

let loadGeneration = 0

async function loadFiles() {
  console.info('[Files] Loading files (stream), path:', currentPath.value)
  const gen = ++loadGeneration
  loading.value = true
  connecting.value = false
  noPermission.value = false
  files.value = []

  for (let attempt = 0; attempt <= MAX_RETRIES; attempt++) {
    if (gen !== loadGeneration) return

    try {
      const result = await listFilesStream(currentPath.value, (file) => {
        if (gen !== loadGeneration) return
        files.value.push(file)
        if (files.value.length === 1 && loading.value) {
          loading.value = false
          console.info('[Files] First item arrived, UI unlocked')
        }
      })

      serverOnline.value = true
      noPermission.value = false
      loading.value = false
      connecting.value = false
      console.info('[Files] Stream complete, total:', result.files.length, 'files')

      loadFileTagsForCurrentDir()
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
    loadFileTagsForCurrentDir()
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
  if (category === 'video' || category === 'audio') {
    playMedia(file, category)
  } else {
    router.push({
      path: '/tabs/preview',
      query: { path: file.path, name: file.name, isEncrypted: String(!!file.isEncrypted) },
    })
  }
}

function handleSearchInput() {
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
  if (selectedPlugin.value) return

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

  buttons.push({
    text: t('files.info'),
    icon: informationCircle,
    handler: () => {
      router.push({ path: '/tabs/file-info', query: { path: file.path, name: file.name } })
    },
  })

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
      text: t('files.preview'),
      icon: image,
      handler: () => {
        router.push({
          path: '/tabs/preview',
          query: { path: file.path, name: file.name, isEncrypted: 'true' },
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
          playMedia(file, category)
        } else {
          router.push({
            path: '/tabs/preview',
            query: { path: file.path, name: file.name, isEncrypted: String(!!file.isEncrypted) },
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
    text: '重命名',
    icon: createOutline,
    handler: () => {
      selectedFile.value = file
      renameValue.value = file.name
      showRenameDialog.value = true
    },
  })
  buttons.push({
    text: '复制',
    icon: copyOutline,
    handler: () => {
      handleCopy(file)
    },
  })
  buttons.push({
    text: '移动',
    icon: arrowForwardOutline,
    handler: () => {
      selectedFile.value = file
      moveTargetPath.value = currentPath.value
      showMoveDialog.value = true
    },
  })
  buttons.push({
    text: '分享',
    icon: shareOutline,
    handler: () => {
      handleShare(file)
    },
  })
  buttons.push({
    text: '标签管理',
    icon: pricetagOutline,
    handler: async () => {
      selectedFile.value = file
      newTagInput.value = ''
      editingFileTags.value = []
      showTagDialog.value = true
      try {
        const allTags = await fetchTags()
        editingFileTags.value = allTags
          .filter(t => t.count > 0)
          .map(t => t.name)
          .slice(0, 10)
      } catch {}
    },
  })

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

async function handleCopy(file: FileItem) {
  const ext = file.name.includes('.') ? '.' + file.name.split('.').pop() : ''
  const baseName = ext ? file.name.slice(0, -ext.length) : file.name
  const destName = `${baseName}_copy${ext}`
  const destPath = currentPath.value === '/' ? `/${destName}` : `${currentPath.value}/${destName}`
  try {
    await copyFile(file.path, destPath)
    await loadFiles()
  } catch (e) { showToast({ message: `复制失败: ${e}` }) }
}

async function handleRename(file: FileItem) {
  if (!renameValue.value.trim() || renameValue.value === file.name) return
  try {
    await renameFile(file.path, renameValue.value.trim())
    showRenameDialog.value = false
    await loadFiles()
  } catch (e) { showToast({ message: `重命名失败: ${e}` }) }
}

async function handleMove(file: FileItem) {
  if (!moveTargetPath.value || moveTargetPath.value === file.path) return
  try {
    const destPath = moveTargetPath.value.endsWith('/') ? `${moveTargetPath.value}${file.name}` : `${moveTargetPath.value}/${file.name}`
    await moveFile(file.path, destPath)
    showMoveDialog.value = false
    await loadFiles()
  } catch (e) { showToast({ message: `移动失败: ${e}` }) }
}

async function handleShare(file: FileItem) {
  if (isNative()) {
    try {
      const localPath = await getLocalFilePath(file.path)
      if (localPath) {
        await Share.share({ title: file.name, url: 'file://' + localPath })
      } else {
        showToast({ message: '仅支持本地文件分享', duration: 2500, color: 'warning' })
      }
    } catch (e) { showToast({ message: '分享失败或已取消' }) }
  } else {
    navigator.clipboard.writeText(getExternalStreamUrl(file.path)).then(() => showToast({ message: '链接已复制到剪贴板' })).catch(() => showToast({ message: '复制失败' }))
  }
}

async function handleAddNewTag() {
  if (!selectedFile.value || !newTagInput.value.trim()) return
  const tag = newTagInput.value.trim()
  if (editingFileTags.value.includes(tag)) {
    newTagInput.value = ''
    return
  }
  try {
    await addTag(selectedFile.value.path, tag)
    editingFileTags.value.push(tag)
    newTagInput.value = ''
  } catch (e) { showToast({ message: '添加标签失败' }) }
}

async function handleRemoveTag(tag: string) {
  if (!selectedFile.value) return
  try {
    await removeTag(selectedFile.value.path, tag)
    editingFileTags.value = editingFileTags.value.filter(t => t !== tag)
  } catch (e) { showToast({ message: '移除标签失败' }) }
}

async function loadFileTagsForCurrentDir() {
  try {
    const allTags = await fetchTags()
    const map: Record<string, string[]> = {}
    for (const tag of allTags) {
      if (tag.count > 0) {
        for (const f of files.value) {
          if (!map[f.path]) map[f.path] = []
          map[f.path].push(tag.name)
        }
      }
    }
    fileTagMap.value = map
  } catch {}
  setupLazyThumbnails()
}

function handleEncryptFile(file: FileItem) {
  router.push({
    path: '/tabs/tasks',
    query: { action: 'new', type: 'encrypt', source: file.path },
  })
}

function handleDecryptFile(file: FileItem) {
  router.push({
    path: '/tabs/tasks',
    query: { action: 'new', type: 'decrypt', source: file.path },
  })
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

async function loadPlugins() {
  try { plugins.value = await fetchPlugins() } catch {}
}
async function loadTags() {
  try { tags.value = await fetchTags() } catch {}
}

function openPluginView(plugin: PluginMeta) {
  files.value = []
  loading.value = true
  pluginLoaded.value = false
  selectedPlugin.value = plugin
  menuController.close()
}

async function exitPluginMode() {
  selectedPlugin.value = null
  await menuController.close()
  files.value = []
  loading.value = true
  await loadFiles()
}

async function openSideDrawer() {
  await menuController.open('plugin-menu')
}

function getPluginIcon(name: string): string {
  const icons: Record<string, string> = { video: 'film-outline', audio: 'musical-notes-outline', image: 'image-outline', pdf: 'document-text-outline', text: 'document-outline', wps: 'document-outline' }
  return icons[name] || 'cube-outline'
}

async function searchPluginFiles(
  plugin: PluginMeta,
  onItem?: (file: FileItem) => void
): Promise<FileItem[]> {
  if (plugin.supportedExtensions.length === 0) return []
  const result = await listPluginFilesStream(
    currentPath.value,
    plugin.supportedExtensions,
    (file) => { onItem?.(file) }
  )
  return result.files
}

async function handleTagFilter(tagName: string) {
  menuController.close()
  files.value = []
  loading.value = true
  selectedPlugin.value = null
  try {
    files.value = await listFilesByTag(tagName, currentPath.value)
    loadFileTagsForCurrentDir()
  } catch (e) { showToast({ message: `筛选失败: ${e}` }) }
  finally { loading.value = false }
}

const pluginTab = ref<'source' | 'container'>('source')
const pluginFiles = ref<FileItem[]>([])
const pluginLoaded = ref(false)
let pluginLoadGeneration = 0

const sizeFilterMin = ref<number | null>(null)
const sizeFilterMax = ref<number | null>(null)
const timeFilterFrom = ref<string | null>(null)
const timeFilterTo = ref<string | null>(null)
const showPluginFilters = ref(false)

const SIZE_PRESETS = [
  { label: '< 1MB', max: 1024 * 1024 },
  { label: '1MB - 10MB', min: 1024 * 1024, max: 10 * 1024 * 1024 },
  { label: '10MB - 100MB', min: 10 * 1024 * 1024, max: 100 * 1024 * 1024 },
  { label: '> 100MB', min: 100 * 1024 * 1024 },
] as const
const TIME_PRESETS = [
  { label: '今天', days: 0 },
  { label: '近 3 天', days: 3 },
  { label: '近 7 天', days: 7 },
  { label: '近 30 天', days: 30 },
] as const

const activeFilterCount = computed(() => {
  let c = 0
  if (sizeFilterMin.value !== null) c++
  if (sizeFilterMax.value !== null) c++
  if (timeFilterFrom.value !== null) c++
  if (timeFilterTo.value !== null) c++
  return c
})

function applySizePreset(preset: typeof SIZE_PRESETS[number]) {
  sizeFilterMin.value = 'min' in preset ? (preset as { min?: number }).min ?? null : null
  sizeFilterMax.value = 'max' in preset ? (preset as { max?: number }).max ?? null : null
}
function applyTimePreset(preset: typeof TIME_PRESETS[number]) {
  const now = new Date()
  const from = new Date(now)
  from.setDate(from.getDate() - preset.days)
  from.setHours(0, 0, 0, 0)
  timeFilterFrom.value = from.toISOString()
  if (preset.days === 0) {
    timeFilterTo.value = now.toISOString()
  } else {
    timeFilterTo.value = null
  }
}
function clearAllPluginFilters() {
  sizeFilterMin.value = null
  sizeFilterMax.value = null
  timeFilterFrom.value = null
  timeFilterTo.value = null
}
const filteredPluginFiles = computed(() => {
  if (!selectedPlugin.value) return []
  let list: FileItem[]
  if (pluginTab.value === 'container') {
    list = pluginFiles.value.filter(f => f.isEncrypted || selectedPlugin.value?.containerExtension && f.name.endsWith(selectedPlugin.value.containerExtension))
  } else {
    list = pluginFiles.value.filter(f => !f.isEncrypted)
  }
  const query = searchQuery.value.trim().toLowerCase()
  if (query) {
    list = list.filter(f => f.name.toLowerCase().includes(query))
  }
  if (sizeFilterMin.value !== null) {
    list = list.filter(f => (f.size || 0) >= sizeFilterMin.value!)
  }
  if (sizeFilterMax.value !== null) {
    list = list.filter(f => (f.size || 0) <= sizeFilterMax.value!)
  }
  if (timeFilterFrom.value !== null) {
    const from = new Date(timeFilterFrom.value).getTime()
    list = list.filter(f => (f.modified ? new Date(f.modified).getTime() : 0) >= from)
  }
  if (timeFilterTo.value !== null) {
    const to = new Date(timeFilterTo.value).getTime()
    list = list.filter(f => (f.modified ? new Date(f.modified).getTime() : 0) <= to)
  }
  list.sort((a, b) => {
    if (a.isDirectory && !b.isDirectory) return -1
    if (!a.isDirectory && b.isDirectory) return 1
    let cmp = 0
    switch (sortBy.value) {
      case 'name': cmp = a.name.localeCompare(b.name); break
      case 'size': cmp = (a.size || 0) - (b.size || 0); break
      case 'time': cmp = (Number(a.modified) || 0) - (Number(b.modified) || 0); break
    }
    return sortDesc.value ? -cmp : cmp
  })
  const tagMap = fileTagMap.value
  return list.map(f => ({ ...f, _tags: tagMap[f.path] || [] }))
})

watch(selectedPlugin, async (plugin) => {
  if (plugin) {
    const gen = ++pluginLoadGeneration
    pluginTab.value = 'source'
    pluginLoaded.value = false
    pluginFiles.value = []
    console.info('[Files] Loading plugin files (stream):', plugin.name)
    try {
      const results = await searchPluginFiles(plugin, (file) => {
        if (gen !== pluginLoadGeneration) return
        pluginFiles.value.push(file)
        if (pluginFiles.value.length === 1 && !pluginLoaded.value) {
          console.info('[Files] First plugin item arrived, UI unlocked')
        }
      })
      if (gen !== pluginLoadGeneration) return
      pluginFiles.value = results
    } catch (e) {
      console.error('[Files] Plugin stream load failed:', e)
    }
    if (gen === pluginLoadGeneration) {
      pluginLoaded.value = true
      setupLazyThumbnails()
    }
  }
})

function onBackendReady(data: { port?: number; running?: boolean }) {
  if (data.running || data.port) {
    loadFiles()
  }
}

onMounted(() => {
  loadFiles()
  loadPlugins()
  loadTags()
  eventBus.on('file:change', onFileChange)
  window.addEventListener('encv:backend-ready', onBackendReadyWindow as EventListener)
})

onIonViewWillEnter(() => {
  if (files.value.length === 0 && !loading.value && !connecting.value) {
    loadFiles()
  }
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
  --padding-start: 8px;
  --padding-end: 8px;
  min-width: 44px;
  min-height: 44px;
  margin: 0;
}

.open-folder-icon {
  font-size: 20px;
  color: var(--ion-color-primary);
}

.tag-editor-content {
  padding: 16px;
}
.existing-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
}
.no-tags-hint {
  color: var(--ion-text-secondary);
  font-size: 14px;
  margin-bottom: 16px;
}
.tag-input-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
.tag-input-row ion-input {
  --padding-start: 12px;
  flex: 1;
}
.file-tag-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 4px;
}
.file-thumbnail-slot {
  width: 48px;
  height: 48px;
  border-radius: 8px;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.file-thumb {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 8px;
}
.thumb-fallback {
  opacity: 0.4;
}
.sort-btn-wrapper {
  display: flex;
  justify-content: flex-end;
  padding: 0 16px 4px;
}
.sort-btn-wrapper ion-button {
  --padding-start: 8px;
  --padding-end: 8px;
  font-size: 13px;
  --color: var(--ion-color-medium);
}
ion-item {
  contain: layout style;
}
.file-tag-chips {
  contain: content;
}
.filter-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}
.filter-chips ion-chip {
  font-size: 12px;
  --padding-start: 8px;
  --padding-end: 8px;
  cursor: pointer;
}</style>
