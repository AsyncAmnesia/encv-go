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
              <p>{{ plugin.supportedExtensions?.length ?? 0 }} 种格式 · 容器 {{ plugin.containerExtension }}</p>
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

      <!-- 播放错误展示区域 -->
      <div v-if="playError" class="play-error-banner">
        <div class="play-error-header">
          <ion-icon :icon="alertCircle" color="danger"></ion-icon>
          <span class="play-error-file">{{ playErrorFile }}</span>
          <ion-button fill="clear" size="small" color="medium" @click="clearPlayError">
            <ion-icon :icon="close" slot="icon-only"></ion-icon>
          </ion-button>
        </div>
        <p class="play-error-message">{{ playError }}</p>
        <div v-if="playErrorDetail" class="play-error-detail-row">
          <ion-button fill="clear" size="small" color="medium" @click="togglePlayErrorDetail">
            {{ t('common.showDetail') }}
          </ion-button>
        </div>
      </div>

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
        <div v-if="selectedPlugin" class="plugin-view">
            <div class="plugin-header">
              <div class="plugin-header-top">
                <ion-button fill="clear" size="small" class="plugin-back-btn" @click="exitPluginMode()">
                  <ion-icon :icon="arrowBack" slot="icon-only"></ion-icon>
                </ion-button>
                <span class="plugin-title">{{ selectedPlugin?.name }} 文件</span>
                <ion-segment v-model="pluginTab" value="source" class="plugin-segment">
                  <ion-segment-button value="source">未加密</ion-segment-button>
                  <ion-segment-button value="container">已加密</ion-segment-button>
                </ion-segment>
              </div>
              <ion-item button detail @click="showPluginFilters = !showPluginFilters" class="filter-toggle-item">
                <ion-icon :icon="filterOutline" slot="start"></ion-icon>
                <ion-label>筛选与排序</ion-label>
                <ion-badge v-if="activeFilterCount > 0" slot="end" color="primary">{{ activeFilterCount }}</ion-badge>
                <ion-badge slot="end" color="medium">{{ pluginSortLabel }}</ion-badge>
              </ion-item>
            </div>
            <div v-if="!pluginLoaded" class="loading-container">
              <ion-spinner name="crescent"></ion-spinner>
              <p>{{ t('files.loading') }}</p>
            </div>
            <template v-else>
            <div v-if="pluginFiles.length === 0" class="empty-state">
              <ion-icon :icon="folderOpen" class="empty-icon"></ion-icon>
              <h3>{{ t('files.emptyDir') }}</h3>
              <p>{{ t('settings.emptyPluginDesc', { name: selectedPlugin?.name ?? '' }) || '该类型下暂无文件' }}</p>
            </div>
            <template v-else>
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
                  <ion-button fill="clear" size="small" @click="sizeFilterMin=null;sizeFilterMax=null">
                    <ion-icon :icon="closeCircleOutline" slot="icon-only"></ion-icon>
                  </ion-button>
                </div>
                <div class="filter-chips">
                  <ion-chip v-for="p in SIZE_PRESETS" :key="p.label" :button="true" outline @click.stop="applySizePreset(p)">{{ p.label }}</ion-chip>
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
                  <ion-button fill="clear" size="small" @click="timeFilterFrom=null;timeFilterTo=null">
                    <ion-icon :icon="closeCircleOutline" slot="icon-only"></ion-icon>
                  </ion-button>
                </div>
                <div class="filter-chips">
                  <ion-chip v-for="p in TIME_PRESETS" :key="p.label" :button="true" outline @click.stop="applyTimePreset(p)">{{ p.label }}</ion-chip>
                </div>
              </ion-item>
              <ion-item>
                <ion-label position="stacked">排序方式</ion-label>
                <div class="filter-chips">
                  <ion-chip v-for="s in ['name', 'size', 'time'] as const" :key="s"
                    :button="true" :outline="pluginSortBy !== s" :color="pluginSortBy === s ? 'primary' : undefined"
                    @click.stop="pluginSortBy = s">
                    {{ s === 'name' ? '名称' : s === 'size' ? '大小' : '时间' }}
                  </ion-chip>
                </div>
                <div class="filter-chips" style="margin-top:4px">
                  <ion-chip :button="true" :outline="!!pluginSortDesc" :color="!pluginSortDesc ? 'primary' : undefined" @click.stop="pluginSortDesc = false">升序 ↑</ion-chip>
                  <ion-chip :button="true" :outline="!pluginSortDesc" :color="!!pluginSortDesc ? 'primary' : undefined" @click.stop="pluginSortDesc = true">降序 ↓</ion-chip>
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
              <ion-badge v-if="file.isEncrypted" color="warning" slot="end">
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

        <div v-if="!selectedPlugin && !searchQuery" class="main-sort-bar">
          <ion-item button detail @click="showMainSort = !showMainSort">
            <ion-icon :icon="swapVerticalOutline" slot="start"></ion-icon>
            <ion-label>排序</ion-label>
            <ion-badge slot="end" color="medium">{{ mainSortLabel }}</ion-badge>
          </ion-item>
          <ion-list v-if="showMainSort" :inset="true">
            <ion-item>
              <ion-label position="stacked">排序方式</ion-label>
              <div class="filter-chips">
                <ion-chip v-for="s in ['name', 'size', 'time'] as const" :key="s"
                  :button="true" :outline="sortBy !== s" :color="sortBy === s ? 'primary' : undefined"
                  @click.stop="sortBy = s">
                  {{ s === 'name' ? '名称' : s === 'size' ? '大小' : '时间' }}
                </ion-chip>
              </div>
              <div class="filter-chips" style="margin-top:4px">
                <ion-chip :button="true" :outline="!!sortDesc" :color="!sortDesc ? 'primary' : undefined" @click.stop="sortDesc = false">升序 ↑</ion-chip>
                <ion-chip :button="true" :outline="!sortDesc" :color="!!sortDesc ? 'primary' : undefined" @click.stop="sortDesc = true">降序 ↓</ion-chip>
              </div>
            </ion-item>
          </ion-list>
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
              <p v-for="sub in fileSubtitles[file.path]" :key="'sub-' + sub.text" class="real-name" :style="{ color: sub.color || 'var(--ion-color-danger)' }">{{ sub.text }}</p>
              <div v-if="!file.isDirectory && !searchQuery && file._tags && file._tags.length > 0" class="file-tag-chips">
                <ion-chip v-for="tag in file._tags" :key="tag" size="small" color="tertiary" outline>{{ tag }}</ion-chip>
              </div>
            </ion-label>
            <ion-badge v-if="file.isEncrypted" color="warning" slot="end">
              ENCV
            </ion-badge>
            <ion-badge v-for="badge in fileBadges[file.path]" :key="'badge-' + badge.text" :color="badge.color" slot="end">
              {{ badge.text }}
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
  IonChip,
  IonModal,
  IonInput,
} from '@ionic/vue'
import {
  arrowBack,
  chevronForward,
  folder,
  folderOpen,
  eyeOutline,
  playCircle,
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
  swapVerticalOutline,
  alertCircle,
  close,
  filmOutline,
  musicalNotesOutline,
  imageOutline,
  documentTextOutline,
  documentOutline,
  cubeOutline,
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
import { useNewTaskModal } from '@/composables/useNewTaskModal'
import { useI18n } from '@/composables/useI18n'
import { usePathResolver } from '@/composables/usePathResolver'
import { formatDateTime } from '@/composables/useDateFormat'
import { useThumbnailCache } from '@/composables/useThumbnailCache'
import { useFileFeatures } from '@/composables/useFileFeatures'
import { preloadSubtitles } from '@/features/alist-encrypt'
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

const ALL_VALID_MODES: PlayMode[] = [
  PLAY_MODE.ARTPLAYER,
  PLAY_MODE.MPV_PLUGIN,
  PLAY_MODE.MPV_ACTIVITY,
  PLAY_MODE.MPV_FRAGMENT,
  PLAY_MODE.MPV_COMPOSE,
  PLAY_MODE.EXTERNAL,
]

function isValidPlayMode(value: string): value is PlayMode {
  return (ALL_VALID_MODES as readonly string[]).includes(value)
}

function getPlayMode(mediaType: 'video' | 'audio'): PlayMode {
  const key = mediaType === 'video' ? 'encv_player_video' : 'encv_player_audio'
  const stored = localStorage.getItem(key)
  if (stored && isValidPlayMode(stored)) return stored
  return mediaType === 'video' ? VIDEO_DEFAULT : AUDIO_DEFAULT
}

const playError = ref<string>('')
const playErrorDetail = ref<string>('')
const playErrorFile = ref<string>('')

async function playMedia(file: FileItem, category: string) {
  const isVideo = category === 'video'
  const mediaType = isVideo ? 'video' : 'audio'
  const mimeType = isVideo ? 'video/*' : 'audio/*'
  const mode = getPlayMode(mediaType)

  console.info('[Files] playMedia: file=', file.path, 'mode=', mode, 'category=', category)
  playError.value = ''
  playErrorDetail.value = ''
  playErrorFile.value = ''

  switch (mode) {
    case PLAY_MODE.ARTPLAYER:
      router.push({ path: '/player', query: { path: file.path, name: file.name } })
      break
    case PLAY_MODE.MPV_PLUGIN:
    case PLAY_MODE.MPV_ACTIVITY:
    case PLAY_MODE.MPV_FRAGMENT:
    case PLAY_MODE.MPV_COMPOSE:
      if (isNative()) {
        const result = await openPlayer(file.path, file.name, mimeType, mode)
        if (!result.success) {
          console.error('[Files] playMedia failed:', result.error, result.errorDetail)
          playError.value = result.error || '播放失败'
          playErrorDetail.value = result.errorDetail || ''
          playErrorFile.value = file.name
        }
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
    default:
      console.debug('[Files] Unknown play mode:', mode, '— falling back to artplayer')
      router.push({ path: '/player', query: { path: file.path, name: file.name } })
      break
  }
}

function clearPlayError() {
  playError.value = ''
  playErrorDetail.value = ''
  playErrorFile.value = ''
}

function togglePlayErrorDetail() {
  if (playErrorDetail.value) {
    const expanded = playErrorDetail.value
    playErrorDetail.value = ''
    playError.value = playError.value + '\n' + expanded
  }
}

const { t } = useI18n()
const { resolveFileItem } = usePathResolver()
const { openNewTask } = useNewTaskModal()
const { thumbnailUrls, setupLazyThumbnails, onThumbError } = useThumbnailCache()
const { sortBy, sortDesc } = useFileListSort()
const showMainSort = ref(false)

const mainSortLabel = computed(() => {
  const map: Record<string, string> = { name: '名称', size: '大小', time: '时间' }
  return (map[sortBy.value] || '名称') + (sortDesc.value ? '↓' : '↑')
})
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
const fileBadges = ref<Record<string, any[]>>({})
const fileSubtitles = ref<Record<string, any[]>>({})

const { getBadges, getSubtitles, getAllActions } = useFileFeatures()

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

  if (file.isEncrypted) {
    router.push({
      path: '/tabs/preview',
      query: { path: file.path, name: file.name, isEncrypted: 'true' },
    })
    return
  }

  const category = getFileCategory(file.name)
  console.info('[Files] Click:', file.name, 'category:', category)
  if (category === 'video' || category === 'audio') {
    playMedia(file, category)
  } else {
    router.push({
      path: '/tabs/preview',
      query: { path: file.path, name: file.name, isEncrypted: 'false' },
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
  const category = file.isDirectory ? 'directory' : getFileCategory(file.name)

  const buttons: any[] = []

  // ===== Section 1: 查看 / 打开 =====
  buttons.push({
    text: t('files.info'),
    icon: informationCircle,
    cssClass: 'action-section-view',
    handler: () => {
      router.push({ path: '/tabs/file-info', query: { path: file.path, name: file.name } })
    },
  })

  if (file.isDirectory) {
    buttons.push({
      text: t('files.open'),
      icon: folderOpen,
      cssClass: 'action-section-view',
      handler: () => {
        const newPath = currentPath.value === '/'
          ? '/' + file.name
          : currentPath.value + '/' + file.name
        navigateTo(newPath)
      },
    })
  } else if (file.isEncrypted) {
    buttons.push({
      text: t('files.preview'),
      icon: eyeOutline,
      cssClass: 'action-section-view',
      handler: () => {
        router.push({
          path: '/tabs/preview',
          query: { path: file.path, name: file.name, isEncrypted: 'true' },
        })
      },
    })
  } else {
    const isMedia = category === 'video' || category === 'audio'

    const featureActions = await getAllActions(file)
    for (const fa of featureActions) {
      buttons.push({
        text: fa.text(),
        icon: fa.icon,
        cssClass: 'action-section-view',
        ...(fa.color ? { role: undefined, cssClass: `action-section-view action-color-${fa.color}` } : {}),
        handler: () => {
          fa.handler(file)
        },
      })
    }

    buttons.push({
      text: isMedia ? t('files.play') : t('files.preview'),
      icon: isMedia ? playCircle : eyeOutline,
      cssClass: 'action-section-view',
      handler: () => {
        if (isMedia) {
          playMedia(file, category)
        } else {
          router.push({
            path: '/tabs/preview',
            query: { path: file.path, name: file.name, isEncrypted: 'false' },
          })
        }
      },
    })
  }

  // ===== Section 2: 加密 / 解密 =====
  if (!file.isDirectory) {
    if (file.isEncrypted) {
      buttons.push({
        text: t('files.decrypt'),
        icon: lockClosed,
        cssClass: 'action-section-crypto',
        handler: () => {
          handleDecryptFile(file)
        },
      })
    } else {
      buttons.push({
        text: t('files.encrypt'),
        icon: lockClosed,
        cssClass: 'action-section-crypto',
        handler: () => {
          handleEncryptFile(file)
        },
      })
    }
  } else {
    buttons.push({
      text: t('files.encrypt'),
      icon: lockClosed,
      cssClass: 'action-section-crypto',
      handler: () => {
        handleEncryptFile(file)
      },
    })
  }

  // ===== Section 3: 文件管理 =====
  buttons.push({
    text: '重命名',
    icon: createOutline,
    cssClass: 'action-section-manage',
    handler: () => {
      selectedFile.value = file
      renameValue.value = file.name
      showRenameDialog.value = true
    },
  })
  buttons.push({
    text: '复制',
    icon: copyOutline,
    cssClass: 'action-section-manage',
    handler: () => {
      handleCopy(file)
    },
  })
  buttons.push({
    text: '移动',
    icon: arrowForwardOutline,
    cssClass: 'action-section-manage',
    handler: () => {
      selectedFile.value = file
      moveTargetPath.value = currentPath.value
      showMoveDialog.value = true
    },
  })
  buttons.push({
    text: '分享',
    icon: shareOutline,
    cssClass: 'action-section-manage',
    handler: () => {
      handleShare(file)
    },
  })
  buttons.push({
    text: '标签管理',
    icon: pricetagOutline,
    cssClass: 'action-section-manage',
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

  // ===== Section 4: 危险操作 =====
  if (!file.isDirectory) {
    buttons.push({
      text: t('files.delete'),
      icon: trash,
      role: 'destructive',
      cssClass: 'action-section-danger',
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
    cssClass: 'file-action-sheet',
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

  const badgesMap: Record<string, any[]> = {}
  const subtitlesMap: Record<string, any[]> = {}
  for (const f of files.value) {
    const badges = await getBadges(f)
    if (badges.length > 0) badgesMap[f.path] = badges
    const subs = await getSubtitles(f)
    if (subs.length > 0) subtitlesMap[f.path] = subs
  }
  fileBadges.value = badgesMap
  fileSubtitles.value = subtitlesMap

  preloadSubtitles(files.value)
  setupLazyThumbnails()
}

function handleEncryptFile(file: FileItem) {
  const resolvedPath = resolveFileItem(file)
  openNewTask(resolvedPath, 'encrypt')
}

function handleDecryptFile(file: FileItem) {
  const resolvedPath = resolveFileItem(file)
  openNewTask(resolvedPath, 'decrypt')
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
  const icons: Record<string, string> = { video: filmOutline, audio: musicalNotesOutline, image: imageOutline, pdf: documentTextOutline, text: documentOutline, wps: documentOutline }
  return icons[name] || cubeOutline
}

async function searchPluginFiles(
  plugin: PluginMeta,
  onItem?: (file: FileItem) => void
): Promise<FileItem[]> {
  if (!plugin.supportedExtensions || plugin.supportedExtensions.length === 0) return []
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

const pluginSortBy = ref<'name' | 'size' | 'time'>('name')
const pluginSortDesc = ref(false)

const pluginSortLabel = computed(() => {
  const map: Record<string, string> = { name: '名称', size: '大小', time: '时间' }
  return (map[pluginSortBy.value] || '名称') + (pluginSortDesc.value ? '↓' : '↑')
})

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
  timeFilterFrom.value = formatDateInput(from)
  if (preset.days === 0) {
    timeFilterTo.value = formatDateInput(now)
  } else {
    timeFilterTo.value = null
  }
}

function formatDateInput(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}
function clearAllPluginFilters() {
  sizeFilterMin.value = null
  sizeFilterMax.value = null
  timeFilterFrom.value = null
  timeFilterTo.value = null
  pluginSortBy.value = 'name'
  pluginSortDesc.value = false
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
    switch (pluginSortBy.value) {
      case 'name': cmp = a.name.localeCompare(b.name); break
      case 'size': cmp = (a.size || 0) - (b.size || 0); break
      case 'time': cmp = (Number(a.modified) || 0) - (Number(b.modified) || 0); break
    }
    return pluginSortDesc.value ? -cmp : cmp
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
  console.error('[SAT-DBG][Files] onMounted | ts=', Date.now())
  loadFiles()
  loadPlugins()
  loadTags()
  console.error('[SAT-DBG][Files] eventBus.on(file:change) | ts=', Date.now())
  eventBus.on('file:change', onFileChange)
  window.addEventListener('encv:backend-ready', onBackendReadyWindow as EventListener)
})

onIonViewWillEnter(() => {
  console.error('[SAT-DBG][Files] onIonViewWillEnter | files.length=', files.value.length, 'loading=', loading.value, 'ts=', Date.now())
  if (files.value.length === 0 && !loading.value && !connecting.value) {
    loadFiles()
  }
})

onUnmounted(() => {
  console.error('[SAT-DBG][Files] onUnmounted | ts=', Date.now())
  console.error('[SAT-DBG][Files] eventBus.off(file:change) | ts=', Date.now())
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
/* 播放错误展示区域 */
.play-error-banner {
  background: rgba(var(--ion-color-danger-rgb), 0.08);
  border-left: 3px solid var(--ion-color-danger);
  border-radius: 6px;
  margin: 8px 12px;
  padding: 10px 12px;
}

.play-error-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.play-error-file {
  font-weight: 500;
  color: var(--ion-color-danger);
  font-size: 14px;
}

.play-error-message {
  color: var(--ion-color-danger);
  font-size: 12px;
  margin-top: 4px;
  margin-bottom: 0;
}

.play-error-detail-row {
  margin-top: 6px;
}

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
}
.plugin-header {
  padding: 0 4px;
}
.plugin-header-top {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 4px 4px;
}
.plugin-back-btn {
  --padding-start: 4px;
  --padding-end: 4px;
  --padding-top: 4px;
  --padding-bottom: 4px;
  min-width: 32px;
  height: 32px;
  flex-shrink: 0;
}
.plugin-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--ion-color-dark);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex-shrink: 1;
  min-width: 0;
}
.plugin-segment {
  flex-shrink: 0;
  --segment-height: 28px;
  margin: 0;
}
.plugin-segment ion-segment-button {
  --padding-start: 10px;
  --padding-end: 10px;
  font-size: 12px;
  min-height: 28px;
  line-height: 1;
}
.filter-toggle-item {
  --padding-start: 12px;
  --padding-end: 12px;
  --min-height: 40px;
}
.main-sort-bar {
  padding: 0 4px;
}

.real-name {
  color: var(--ion-color-danger);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* ===== 长按菜单分区样式 (action-sheet overlay，需 :global) ===== */
:global(.file-action-sheet .action-button) {
  padding: 12px 16px;
  font-size: 15px;
}

:global(.file-action-sheet .action-section-view) {
  --color: var(--ion-color-primary);
}
:global(.file-action-sheet .action-section-view .action-button-icon) {
  color: var(--ion-color-primary) !important;
}

:global(.file-action-sheet .action-section-crypto) {
  --color: var(--ion-color-warning);
}
:global(.file-action-sheet .action-section-crypto .action-button-icon) {
  color: #e6a000 !important;
}

:global(.file-action-sheet .action-section-manage) {
  --color: var(--ion-color-medium);
}
:global(.file-action-sheet .action-section-manage .action-button-icon) {
  color: var(--ion-color-medium-shade) !important;
}

:global(.file-action-sheet .action-section-danger) {
  --color: var(--ion-color-danger);
}
:global(.file-action-sheet .action-section-danger .action-button-icon) {
  color: var(--ion-color-danger) !important;
}</style>
