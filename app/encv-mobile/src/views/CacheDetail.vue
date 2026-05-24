<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button default-href="/tabs/settings"></ion-back-button>
        </ion-buttons>
        <ion-title>{{ t('settings.cacheAndIndex') }}</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <ion-list>
        <ion-list-header>
          <ion-label>{{ t('settings.indexStatus') }}</ion-label>
        </ion-list-header>

        <ion-item>
          <ion-icon :icon="documentText" slot="start" color="primary"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.totalFiles') }}</h3>
            <p>{{ stats?.totalFiles ?? '-' }}</p>
          </ion-label>
        </ion-item>

        <ion-item>
          <ion-icon :icon="folder" slot="start" color="primary"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.totalDirs') }}</h3>
            <p>{{ stats?.totalDirs ?? '-' }}</p>
          </ion-label>
        </ion-item>

        <ion-item>
          <ion-icon :icon="speedometer" slot="start" color="primary"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.totalSize') }}</h3>
            <p>{{ stats ? formatFileSize(stats.totalSize) : '-' }}</p>
          </ion-label>
        </ion-item>

        <ion-item>
          <ion-icon :icon="time" slot="start" color="primary"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.indexedAt') }}</h3>
            <p>{{ stats?.indexedAt || t('settings.never') }}</p>
          </ion-label>
        </ion-item>

        <ion-item>
          <ion-icon :icon="timer" slot="start" color="primary"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.lastBuildTime') }}</h3>
            <p>{{ stats?.lastBuildMs ? `${stats.lastBuildMs} ms` : '-' }}</p>
          </ion-label>
        </ion-item>

        <ion-item>
          <ion-icon :icon="stats?.isIndexing ? sync : checkmarkCircle" slot="start" :color="stats?.isIndexing ? 'warning' : 'success'"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.indexState') }}</h3>
            <p>{{ stats?.isIndexing ? t('settings.indexing') : t('settings.idle') }}</p>
          </ion-label>
          <ion-spinner v-if="stats?.isIndexing" name="crescent" slot="end"></ion-spinner>
        </ion-item>

        <ion-item v-if="stats?.source">
          <ion-icon :icon="server" slot="start" color="primary"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.indexSource') }}</h3>
            <p>{{ stats.source === 'webdav' ? 'WebDAV' : t('settings.mobileIndex') }}</p>
          </ion-label>
        </ion-item>

        <ion-item v-if="stats?.containers">
          <ion-icon :icon="lockClosed" slot="start" color="warning"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.encryptedContainers') }}</h3>
            <p>{{ stats.containers }}</p>
          </ion-label>
        </ion-item>
      </ion-list>

      <ion-list>
        <ion-list-header>
          <ion-label>{{ t('settings.actions') }}</ion-label>
        </ion-list-header>

        <ion-item button @click="handleRebuild" :disabled="stats?.isIndexing">
          <ion-icon :icon="sync" slot="start" color="primary"></ion-icon>
          <ion-label>
            <h3>{{ stats?.isIndexing ? t('settings.indexing') : t('settings.rebuildIndex') }}</h3>
          </ion-label>
          <ion-spinner v-if="stats?.isIndexing" name="crescent" slot="end"></ion-spinner>
        </ion-item>

        <ion-item button @click="handleClearIndex" :disabled="stats?.isIndexing">
          <ion-icon :icon="trash" slot="start" color="danger"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.clearIndex') }}</h3>
          </ion-label>
        </ion-item>
      </ion-list>

      <ion-list>
        <ion-list-header>
          <ion-label>{{ t('settings.searchCache') }}</ion-label>
        </ion-list-header>

        <ion-item>
          <ion-icon :icon="search" slot="start" color="primary"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.cacheEntries') }}</h3>
            <p>{{ searchCacheSize }}</p>
          </ion-label>
        </ion-item>

        <ion-item button @click="handleClearSearchCache">
          <ion-icon :icon="trash" slot="start" color="danger"></ion-icon>
          <ion-label>
            <h3>{{ t('settings.clearSearchCache') }}</h3>
          </ion-label>
        </ion-item>
      </ion-list>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import {
  IonPage,
  IonHeader,
  IonToolbar,
  IonTitle,
  IonButtons,
  IonBackButton,
  IonContent,
  IonList,
  IonListHeader,
  IonItem,
  IonIcon,
  IonLabel,
  IonSpinner,
  alertController,
} from '@ionic/vue'
import {
  documentText,
  folder,
  speedometer,
  time,
  timer,
  sync,
  checkmarkCircle,
  trash,
  search,
  server,
  lockClosed,
} from 'ionicons/icons'
import {
  getIndexStats,
  rebuildIndex,
  clearIndex,
  formatFileSize,
} from '@/api/encv'
import type { IndexStats } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'

const { t } = useI18n()
const stats = ref<IndexStats | null>(null)
const searchCacheSize = ref(0)
let pollTimer: ReturnType<typeof setInterval> | null = null

async function loadStats() {
  try {
    stats.value = await getIndexStats()
  } catch {
    stats.value = null
  }
}

function updateSearchCacheSize() {
  try {
    let count = 0
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i)
      if (key?.startsWith('search-cache:')) count++
    }
    searchCacheSize.value = count
  } catch {
    searchCacheSize.value = 0
  }
}

async function handleRebuild() {
  try {
    await rebuildIndex()
    await loadStats()
    showToast({ message: t('settings.rebuildStarted'), duration: 1500, color: 'success' })
  } catch {
    showToast({ message: t('settings.rebuildFailed'), duration: 2000, color: 'danger' })
  }
}

async function handleClearIndex() {
  const alert = await alertController.create({
    header: t('settings.clearIndex'),
    message: t('settings.clearIndexConfirm'),
    buttons: [
      { text: t('files.cancelSelect'), role: 'cancel' },
      {
        text: t('settings.clearIndex'),
        role: 'destructive',
        handler: async () => {
          try {
            await clearIndex()
            await loadStats()
          } catch {
            showToast({ message: t('settings.clearFailed'), duration: 2000, color: 'danger' })
          }
        },
      },
    ],
  })
  await alert.present()
}

function handleClearSearchCache() {
  const keysToRemove: string[] = []
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i)
    if (key?.startsWith('search-cache:')) keysToRemove.push(key)
  }
  keysToRemove.forEach(key => localStorage.removeItem(key))
  searchCacheSize.value = 0
}

onMounted(() => {
  loadStats()
  updateSearchCacheSize()
  pollTimer = setInterval(() => {
    if (stats.value?.isIndexing) {
      loadStats()
    }
  }, 2000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>
