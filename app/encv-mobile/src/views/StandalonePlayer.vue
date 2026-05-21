<template>
  <ion-page>
    <ion-content>
      <div v-if="backendLoading" class="loading-state">
        <ion-spinner name="crescent" class="loading-spinner"></ion-spinner>
        <h3>正在启动后端...</h3>
      </div>

      <div v-else-if="backendError" class="error-state">
        <ion-icon :icon="alertCircle" class="error-icon"></ion-icon>
        <h3>{{ backendError }}</h3>
        <ion-button @click="retryBackend">
          <ion-icon :icon="refresh" slot="start"></ion-icon>
          {{ t('player.retryPlay') }}
        </ion-button>
      </div>

      <div v-else class="player-container">
        <div v-if="isVideo && !playerError" ref="artContainer" class="video-player"></div>

        <div v-if="isVideo && playerError" class="player-error">
          <ion-icon :icon="alertCircle" class="error-icon"></ion-icon>
          <h3>{{ t('player.playError') }}</h3>
          <p>{{ t('player.playErrorDesc') }}</p>
          <ion-button @click="retryPlay">
            <ion-icon :icon="refresh" slot="start"></ion-icon>
            {{ t('player.retryPlay') }}
          </ion-button>
        </div>

        <div v-if="isVideo && !playerError && !isFullscreen" class="video-info">
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

        <div v-if="isAudio" class="audio-player-wrapper">
          <div class="audio-visual">
            <ion-icon :icon="musicalNotes" class="audio-icon"></ion-icon>
          </div>
          <h3>{{ fileName }}</h3>
          <audio
            ref="audioRef"
            :src="streamUrl"
            controls
            autoplay
            class="audio-player"
            @error="handlePlayerError"
          ></audio>
        </div>
      </div>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import Artplayer from 'artplayer'
import {
  IonPage,
  IonContent,
  IonIcon,
  IonButton,
  IonChip,
  IonSpinner,
} from '@ionic/vue'
import { musicalNotes, alertCircle, refresh, resize, time } from 'ionicons/icons'
import { isStandaloneMode, getIntentFileInfo, getBackendStatus } from '@/plugins/GoProcess'
import { setApiBaseUrl, getFileStreamUrl, getExternalStreamUrl, getFileCategory } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'

const { t } = useI18n()

const backendLoading = ref(true)
const backendError = ref('')
const filePath = ref('')
const fileName = ref('')
const fileMimeType = ref('')
const isExternalFile = ref(false)
const playerError = ref(false)
const isFullscreen = ref(false)
const mediaInfo = ref({ duration: '', resolution: '' })
const artContainer = ref<HTMLDivElement | null>(null)
const audioRef = ref<HTMLAudioElement | null>(null)
let art: Artplayer | null = null

const fileCategory = computed(() => {
  if (fileName.value) return getFileCategory(fileName.value)
  if (fileMimeType.value) {
    if (fileMimeType.value.startsWith('video/')) return 'video'
    if (fileMimeType.value.startsWith('audio/')) return 'audio'
    if (fileMimeType.value.startsWith('image/')) return 'image'
  }
  return 'other'
})

const isVideo = computed(() => fileCategory.value === 'video' || fileCategory.value === 'encrypted')
const isAudio = computed(() => fileCategory.value === 'audio')

const streamUrl = computed(() => {
  if (!filePath.value) return ''
  if (isExternalFile.value) return getExternalStreamUrl(filePath.value)
  return getFileStreamUrl(filePath.value)
})

function formatDuration(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (h > 0) {
    return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  }
  return `${m}:${String(s).padStart(2, '0')}`
}

async function handlePlayerError() {
  showToast({ message: t('player.playFailed', { name: fileName.value }), duration: 3000, color: 'danger' })
}

async function initBackend() {
  backendLoading.value = true
  backendError.value = ''

  const { standalone } = await isStandaloneMode()
  if (!standalone) {
    backendError.value = 'Not in standalone mode'
    backendLoading.value = false
    return
  }

  const intentInfo = await getIntentFileInfo()
  filePath.value = intentInfo.path
  fileName.value = intentInfo.name
  fileMimeType.value = intentInfo.mimeType
  isExternalFile.value = true

  if (!filePath.value) {
    backendError.value = 'No file provided'
    backendLoading.value = false
    return
  }

  const status = await getBackendStatus()
  if (status.running && status.port > 0) {
    setApiBaseUrl(`http://127.0.0.1:${status.port}`)
    backendLoading.value = false
  }
}

function handleBackendReady(event: CustomEvent) {
  const { port, running } = event.detail || {}
  if (running && port > 0) {
    setApiBaseUrl(`http://127.0.0.1:${port}`)
    backendLoading.value = false
    backendError.value = ''
  }
}

function handleBackendStatus(event: CustomEvent) {
  const { port, running, error } = event.detail || {}
  if (running && port > 0) {
    setApiBaseUrl(`http://127.0.0.1:${port}`)
    backendLoading.value = false
    backendError.value = ''
  }
  if (error) {
    backendError.value = error
  }
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
      const w = video.videoWidth
      const h = video.videoHeight
      const dur = video.duration
      if (w && h) {
        mediaInfo.value.resolution = `${w}×${h}`
      }
      if (dur && isFinite(dur)) {
        mediaInfo.value.duration = formatDuration(dur)
      }
    }
    console.info('[StandalonePlayer] Video metadata loaded', mediaInfo.value)
  })

  art.on('fullscreen', () => {
    isFullscreen.value = true
  })

  art.on('fullscreenExit', () => {
    isFullscreen.value = false
  })

  art.on('error', () => {
    console.error('[StandalonePlayer] ArtPlayer playback error')
    playerError.value = true
    handlePlayerError()
  })

  console.info('[StandalonePlayer] ArtPlayer initialized')
}

function destroyArtPlayer() {
  if (art) {
    art.destroy()
    art = null
    console.info('[StandalonePlayer] ArtPlayer destroyed')
  }
}

function retryBackend() {
  backendError.value = ''
  initBackend()
}

function retryPlay() {
  playerError.value = false
  mediaInfo.value = { duration: '', resolution: '' }
  destroyArtPlayer()
  initArtPlayer()
}

async function startPlayback() {
  if (!filePath.value) return
  playerError.value = false
  mediaInfo.value = { duration: '', resolution: '' }
  await nextTick()
  if (isVideo.value) {
    initArtPlayer()
  }
}

onMounted(() => {
  initBackend()
  window.addEventListener('encv:backend-ready', handleBackendReady as EventListener)
  window.addEventListener('encv:backend-status', handleBackendStatus as EventListener)
})

onBeforeUnmount(() => {
  window.removeEventListener('encv:backend-ready', handleBackendReady as EventListener)
  window.removeEventListener('encv:backend-status', handleBackendStatus as EventListener)
  destroyArtPlayer()
})

watch(backendLoading, (loading) => {
  if (!loading && filePath.value) {
    startPlayback()
  }
})
</script>

<style scoped>
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

.audio-player-wrapper {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 24px;
  text-align: center;
}

.audio-visual {
  width: 120px;
  height: 120px;
  border-radius: 60px;
  background: var(--ion-color-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 24px;
}

.audio-icon {
  font-size: 48px;
  color: white;
}

.audio-player {
  width: 100%;
  max-width: 400px;
  margin-top: 24px;
}
</style>
