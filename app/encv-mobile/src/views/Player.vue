<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>{{ fileName || t('player.title') }}</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <div v-if="!filePath" class="empty-state">
        <ion-icon :icon="playCircle" class="empty-icon"></ion-icon>
        <h3>{{ t('player.noMedia') }}</h3>
        <p>{{ t('player.noMediaDesc') }}</p>
        <ion-button router-link="/tabs/files">
          <ion-icon :icon="folder" slot="start"></ion-icon>
          {{ t('player.browseFiles') }}
        </ion-button>
      </div>

      <div v-else class="player-container">
        <div v-if="isVideo && !playerError" ref="artContainer" class="video-player"></div>

        <div v-if="isVideo && playerError" class="player-error">
          <ion-icon :icon="alertCircle" class="error-icon"></ion-icon>
          <h3>{{ t('player.playError') }}</h3>
          <p>{{ t('player.playErrorDesc') }}</p>
          <div class="error-actions">
            <ion-button router-link="/tabs/files" fill="outline">
              <ion-icon :icon="folder" slot="start"></ion-icon>
              {{ t('player.backToFiles') }}
            </ion-button>
            <ion-button @click="retryPlay">
              <ion-icon :icon="refresh" slot="start"></ion-icon>
              {{ t('player.retryPlay') }}
            </ion-button>
          </div>
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
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRoute } from 'vue-router'
import Artplayer from 'artplayer'
import {
  IonPage,
  IonHeader,
  IonToolbar,
  IonTitle,
  IonContent,
  IonIcon,
  IonButton,
  IonChip,
  toastController,
} from '@ionic/vue'
import { playCircle, folder, musicalNotes, alertCircle, refresh, resize, time } from 'ionicons/icons'
import { getFileStreamUrl, getFileCategory } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'

const route = useRoute()
const { t } = useI18n()

const filePath = computed(() => (route.query.path as string) || '')
const fileName = computed(() => (route.query.name as string) || '')

const fileCategory = computed(() => {
  if (!fileName.value) return 'other'
  return getFileCategory(fileName.value)
})

const isVideo = computed(() => fileCategory.value === 'video' || fileCategory.value === 'encrypted')
const isAudio = computed(() => fileCategory.value === 'audio')

const streamUrl = computed(() => {
  if (!filePath.value) return ''
  return getFileStreamUrl(filePath.value)
})

const artContainer = ref<HTMLDivElement | null>(null)
const audioRef = ref<HTMLAudioElement | null>(null)
const playerError = ref(false)
const isFullscreen = ref(false)
const mediaInfo = ref({ duration: '', resolution: '' })
let art: Artplayer | null = null

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
  const toast = await toastController.create({
    message: t('player.playFailed', { name: fileName.value }),
    duration: 3000,
    color: 'danger',
  })
  await toast.present()
}

function initArtPlayer() {
  if (!artContainer.value || !streamUrl.value) return

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
    console.info('[Player] Video metadata loaded', mediaInfo.value)
  })

  art.on('fullscreen', () => {
    isFullscreen.value = true
  })

  art.on('fullscreenExit', () => {
    isFullscreen.value = false
  })

  art.on('error', () => {
    console.error('[Player] ArtPlayer playback error')
    playerError.value = true
    handlePlayerError()
  })

  console.info('[Player] ArtPlayer initialized')
}

function destroyArtPlayer() {
  if (art) {
    art.destroy()
    art = null
    console.info('[Player] ArtPlayer destroyed')
  }
}

function retryPlay() {
  playerError.value = false
  mediaInfo.value = { duration: '', resolution: '' }
  destroyArtPlayer()
  initArtPlayer()
}

onMounted(() => {
  if (isVideo.value) {
    playerError.value = false
    mediaInfo.value = { duration: '', resolution: '' }
    initArtPlayer()
  }
})

onBeforeUnmount(() => {
  destroyArtPlayer()
})

watch(streamUrl, (newUrl) => {
  if (isVideo.value && art && newUrl) {
    art.switchUrl(newUrl)
    console.info('[Player] ArtPlayer switched URL:', newUrl)
  }
})
</script>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 24px;
  text-align: center;
  color: var(--encv-text-secondary);
}

.empty-icon {
  font-size: 80px;
  margin-bottom: 16px;
  opacity: 0.4;
}

.player-container {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.video-player {
  width: 100%;
  height: 30vh;
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

.error-actions {
  display: flex;
  gap: 12px;
  margin-top: 16px;
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
