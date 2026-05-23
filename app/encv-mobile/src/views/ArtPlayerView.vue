<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-button @click="goBack" fill="clear">
            <ion-icon :icon="arrowBack" slot="icon-only"></ion-icon>
          </ion-button>
        </ion-buttons>
        <ion-title>{{ fileName || 'Player' }}</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <div v-if="loading" class="loading-state">
        <ion-spinner name="crescent" class="loading-spinner"></ion-spinner>
        <p>{{ t('player.loading') }}</p>
      </div>

      <div v-else-if="error" class="error-state">
        <ion-icon :icon="alertCircle" class="error-icon"></ion-icon>
        <h3>{{ error }}</h3>
        <ion-button @click="retryPlay">
          <ion-icon :icon="refresh" slot="start"></ion-icon>
          {{ t('player.retryPlay') }}
        </ion-button>
      </div>

      <div v-else class="player-container">
        <div v-if="!playerError" ref="artContainer" class="video-player"></div>

        <div v-if="playerError" class="player-error">
          <ion-icon :icon="alertCircle" class="error-icon"></ion-icon>
          <h3>{{ t('player.playError') }}</h3>
          <p>{{ t('player.playErrorDesc') }}</p>
          <ion-button @click="retryPlay">
            <ion-icon :icon="refresh" slot="start"></ion-icon>
            {{ t('player.retryPlay') }}
          </ion-button>
        </div>

        <div v-if="!playerError && !isFullscreen" class="video-info">
          <h3>{{ fileName }}</h3>
          <div v-if="mediaInfo.duration || mediaInfo.resolution" class="media-meta">
            <ion-chip v-if="mediaInfo.resolution" outline>
              <ion-icon :icon="resize" slot="start"></ion-icon>
              {{ mediaInfo.resolution }}
            </ion-chip>
            <ion-chip v-if="mediaInfo.duration" outline>
              <ion-icon :icon="time" slot="start"></ion-icon>
              {{ mediaInfo.duration }}
            </ion-chip>
          </div>
          <p v-if="filePath" class="video-path">{{ filePath }}</p>
        </div>
      </div>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Artplayer from 'artplayer'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons, IonButton,
  IonIcon, IonContent, IonChip, IonSpinner,
} from '@ionic/vue'
import { arrowBack, alertCircle, refresh, resize, time } from 'ionicons/icons'
import { getFileStreamUrl } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import { isNative } from '@/plugins/GoProcess'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const loading = ref(true)
const error = ref('')
const filePath = ref('')
const fileName = ref('')
const playerError = ref(false)
const isFullscreen = ref(false)
const mediaInfo = ref({ duration: '', resolution: '' })
const artContainer = ref<HTMLDivElement | null>(null)
let art: Artplayer | null = null

const streamUrl = computed(() => {
  if (!filePath.value) return ''
  return getFileStreamUrl(filePath.value)
})

function formatDuration(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${m}:${String(s).padStart(2, '0')}`
}

function goBack() {
  destroyArtPlayer()
  router.back()
}

function initArtPlayer() {
  if (!artContainer.value || !streamUrl.value) return

  const containerWidth = artContainer.value.clientWidth || window.innerWidth
  const containerHeight = Math.round(containerWidth * 9 / 16)
  artContainer.value.style.height = `${containerHeight}px`

  art = new Artplayer({
    container: artContainer.value,
    url: streamUrl.value,
    autoplay: true,
    autoSize: true,
    autoMini: true,
    mutex: true,
    playsInline: true,
    theme: '#ffad00',
    volume: 0.7,
    fullscreen: true,
    miniProgressBar: true,
  })

  art.on('video:loadedmetadata', () => {
    const video = art?.video
    if (video) {
      if (video.videoWidth && video.videoHeight) {
        mediaInfo.value.resolution = `${video.videoWidth}×${video.videoHeight}`
      }
      if (video.duration && isFinite(video.duration)) {
        mediaInfo.value.duration = formatDuration(video.duration)
      }
    }
  })

  art.on('fullscreen', () => {
    isFullscreen.value = true
    handleFullscreenEnter()
  })

  art.on('fullscreenExit', () => {
    isFullscreen.value = false
    handleFullscreenExit()
  })

  art.on('error', () => {
    playerError.value = true
    showToast({ message: t('player.playFailed', { name: fileName.value }), duration: 3000, color: 'danger' })
  })

  nextTick(() => {
    if (art?.video) {
      art.video.removeAttribute('controls')
      art.video.controls = false
    }
  })
}

function handleFullscreenEnter() {
  const video = art?.video
  if (!video?.videoWidth || !video?.videoHeight) return
  const ratio = video.videoWidth / video.videoHeight
  if (isNative()) {
    const cap = (window as any).Capacitor
    const { GoProcess } = cap?.Plugins || {}
    if (GoProcess) {
      const orientation = ratio > 1.3 ? 'landscape' : ratio < 0.77 ? 'portrait' : 'landscape'
      GoProcess.setScreenOrientation({ orientation }).catch(() => {})
    }
  }
}

function handleFullscreenExit() {
  if (isNative()) {
    const cap = (window as any).Capacitor
    const { GoProcess } = cap?.Plugins || {}
    if (GoProcess) {
      GoProcess.setScreenOrientation({ orientation: 'portrait' }).catch(() => {})
    }
  }
}

function destroyArtPlayer() {
  if (art) {
    art.destroy()
    art = null
  }
}

function retryPlay() {
  playerError.value = false
  mediaInfo.value = { duration: '', resolution: '' }
  destroyArtPlayer()
  nextTick(() => initArtPlayer())
}

async function startPlayback() {
  if (!filePath.value) return
  loading.value = false
  playerError.value = false
  mediaInfo.value = { duration: '', resolution: '' }
  await nextTick()
  initArtPlayer()
}

onMounted(() => {
  filePath.value = (route.query.path as string) || ''
  fileName.value = (route.query.name as string) || ''

  if (!filePath.value) {
    error.value = 'No file provided'
    loading.value = false
    return
  }

  startPlayback()
})

onBeforeUnmount(() => {
  destroyArtPlayer()
})
</script>

<style scoped>
:deep(video::-webkit-media-controls) {
  display: none !important;
}
:deep(video::-webkit-media-controls-enclosure) {
  display: none !important;
}
:deep(video::-webkit-media-controls-panel) {
  display: none !important;
}

.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 24px;
  text-align: center;
  color: var(--encv-text-secondary);
}

.loading-spinner {
  width: 48px;
  height: 48px;
  margin-bottom: 16px;
}

.error-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 24px;
  text-align: center;
  color: var(--encv-text-secondary);
}

.player-container {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.video-player {
  width: 100%;
  background: #000;
}

.video-info {
  padding: 16px;
}

.media-meta {
  display: flex;
  gap: 8px;
  margin-top: 8px;
  flex-wrap: wrap;
}

.video-path {
  font-size: 12px;
  color: var(--encv-text-secondary);
  word-break: break-all;
  margin-top: 8px;
}

.player-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 30vh;
  padding: 24px;
  text-align: center;
  color: var(--encv-text-secondary);
}

.error-icon {
  font-size: 64px;
  margin-bottom: 16px;
  color: var(--ion-color-danger);
  opacity: 0.7;
}
</style>
