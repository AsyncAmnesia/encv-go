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
          <p>{{ playerErrorMsg }}</p>
          <p v-if="streamUrl" class="debug-url">{{ streamUrl }}</p>
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
import { StatusBar, Style } from '@capacitor/status-bar'
import { ScreenOrientation } from '@capacitor/screen-orientation'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons, IonButton,
  IonIcon, IonContent, IonChip, IonSpinner,
} from '@ionic/vue'
import { arrowBack, alertCircle, refresh, resize, time } from 'ionicons/icons'
import { getFileStreamUrl } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import { isNative } from '@/plugins/GoProcess'

const TAG = '[ArtPlayer]'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const loading = ref(true)
const error = ref('')
const filePath = ref('')
const fileName = ref('')
const playerError = ref(false)
const playerErrorMsg = ref('')
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

function hideNativeControls() {
  if (!art?.video) return
  art.video.removeAttribute('controls')
  art.video.controls = false
  art.video.setAttribute('playsinline', '')
  art.video.setAttribute('webkit-playsinline', '')
  art.video.setAttribute('x5-playsinline', '')
  art.video.setAttribute('x5-video-player-type', 'h5')
}

async function handleFullscreenEnter() {
  isFullscreen.value = true
  if (!isNative()) return
  try {
    await StatusBar.hide()
    const video = art?.video
    if (video?.videoWidth && video?.videoHeight) {
      const ratio = video.videoWidth / video.videoHeight
      if (ratio > 1.3) {
        await ScreenOrientation.lock({ orientation: 'landscape' })
      } else if (ratio < 0.77) {
        await ScreenOrientation.lock({ orientation: 'portrait' })
      } else {
        await ScreenOrientation.lock({ orientation: 'landscape' })
      }
    } else {
      await ScreenOrientation.lock({ orientation: 'landscape' })
    }
  } catch (e) {
    console.debug(TAG, 'handleFullscreenEnter error:', e)
  }
}

async function handleFullscreenExit() {
  isFullscreen.value = false
  if (!isNative()) return
  try {
    await ScreenOrientation.lock({ orientation: 'portrait' })
    await StatusBar.show()
    await StatusBar.setStyle({ style: Style.Default })
  } catch (e) {
    console.debug(TAG, 'handleFullscreenExit error:', e)
  }
}

function initArtPlayer() {
  console.info(TAG, 'initArtPlayer called')
  console.info(TAG, 'artContainer:', artContainer.value ? `exists (${artContainer.value.clientWidth}x${artContainer.value.clientHeight})` : 'null')
  console.info(TAG, 'streamUrl:', streamUrl.value || '(empty)')
  console.info(TAG, 'filePath:', filePath.value || '(empty)')

  const styleEl = document.getElementById('artplayer-style')
  console.info(TAG, 'artplayer-style element:', styleEl ? `exists (${styleEl.textContent?.length ?? 0} chars)` : 'NOT FOUND')

  if (!artContainer.value) {
    console.error(TAG, 'initArtPlayer: artContainer is null, cannot init')
    playerError.value = true
    playerErrorMsg.value = '播放器容器未就绪'
    return
  }

  if (!streamUrl.value) {
    console.error(TAG, 'initArtPlayer: streamUrl is empty, cannot init')
    playerError.value = true
    playerErrorMsg.value = '播放地址为空'
    return
  }

  const containerWidth = artContainer.value.clientWidth || window.innerWidth
  artContainer.value.style.minHeight = '200px'
  artContainer.value.style.maxHeight = `${window.innerHeight - 56}px`
  console.info(TAG, 'container size:', containerWidth)

  try {
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
      fullscreenWeb: !isNative(),
      miniProgressBar: true,
      setting: true,
      playbackRate: true,
      aspectRatio: true,
      flip: true,
      lock: true,
      autoOrientation: true,
      autoPlayback: true,
      subtitleOffset: true,
      fastForward: true,
      hotkey: true,
      gesture: true,
      moreVideoAttr: {
        controls: false,
        preload: 'metadata',
        playsInline: true,
      },
    })
    console.info(TAG, 'Artplayer instance created successfully, id:', art.id)
  } catch (e: any) {
    console.error(TAG, 'Artplayer constructor failed:', e?.message || String(e))
    playerError.value = true
    playerErrorMsg.value = `ArtPlayer 初始化失败: ${e?.message || String(e)}`
    return
  }

  art.on('ready', () => {
    console.info(TAG, 'Artplayer ready event fired')
    hideNativeControls()
  })

  art.on('video:loadedmetadata', () => {
    const video = art?.video
    console.info(TAG, 'video:loadedmetadata, videoWidth:', video?.videoWidth, 'videoHeight:', video?.videoHeight, 'duration:', video?.duration)
    if (video) {
      if (video.videoWidth && video.videoHeight) {
        mediaInfo.value.resolution = `${video.videoWidth}×${video.videoHeight}`
        const ratio = video.videoHeight / video.videoWidth
        const containerWidth = artContainer.value?.clientWidth || window.innerWidth
        const naturalHeight = Math.round(containerWidth * ratio)
        const maxHeight = window.innerHeight - 56
        const finalHeight = Math.min(naturalHeight, maxHeight)
        if (artContainer.value) {
          artContainer.value.style.height = `${finalHeight}px`
        }
      }
      if (video.duration && isFinite(video.duration)) {
        mediaInfo.value.duration = formatDuration(video.duration)
      }
    }
    hideNativeControls()
  })

  art.on('video:play', () => {
    console.info(TAG, 'video:play')
    hideNativeControls()
  })

  art.on('video:playing', () => {
    console.info(TAG, 'video:playing')
    hideNativeControls()
  })

  art.on('fullscreen', (state: boolean) => {
    if (state) {
      handleFullscreenEnter()
    } else {
      handleFullscreenExit()
    }
  })

  art.on('error', () => {
    const video = art?.video
    const networkState = video?.networkState
    const readyState = video?.readyState
    const src = video?.src
    const currentSrc = video?.currentSrc
    console.error(TAG, 'Artplayer error event, networkState:', networkState, 'readyState:', readyState, 'src:', src, 'currentSrc:', currentSrc)
    playerError.value = true
    playerErrorMsg.value = `播放失败 (network=${networkState}, ready=${readyState})`
    showToast({ message: t('player.playFailed', { name: fileName.value }), duration: 3000, color: 'danger' })
  })

  art.on('destroy', () => {
    console.info(TAG, 'Artplayer destroy event')
  })

  nextTick(() => {
    hideNativeControls()
  })

  setTimeout(() => {
    hideNativeControls()
  }, 500)

  setTimeout(() => {
    hideNativeControls()
  }, 2000)
}

function destroyArtPlayer() {
  if (art) {
    console.info(TAG, 'destroyArtPlayer: destroying art instance')
    art.destroy()
    art = null
  }
}

function retryPlay() {
  console.info(TAG, 'retryPlay called')
  playerError.value = false
  playerErrorMsg.value = ''
  mediaInfo.value = { duration: '', resolution: '' }
  destroyArtPlayer()
  nextTick(() => initArtPlayer())
}

async function startPlayback() {
  console.info(TAG, 'startPlayback called, filePath:', filePath.value, 'streamUrl:', streamUrl.value)
  if (!filePath.value) {
    console.error(TAG, 'startPlayback: filePath is empty')
    return
  }
  loading.value = false
  playerError.value = false
  playerErrorMsg.value = ''
  mediaInfo.value = { duration: '', resolution: '' }
  await nextTick()
  console.info(TAG, 'startPlayback: nextTick done, artContainer:', artContainer.value ? 'exists' : 'null')
  initArtPlayer()
}

onMounted(() => {
  filePath.value = (route.query.path as string) || ''
  fileName.value = (route.query.name as string) || ''
  console.info(TAG, 'onMounted: filePath=', filePath.value, 'fileName=', fileName.value)

  if (!filePath.value) {
    error.value = 'No file provided'
    loading.value = false
    return
  }

  startPlayback()
})

onBeforeUnmount(async () => {
  console.info(TAG, 'onBeforeUnmount')
  destroyArtPlayer()
  if (isNative()) {
    try {
      await ScreenOrientation.lock({ orientation: 'portrait' })
      await StatusBar.show()
      await StatusBar.setStyle({ style: Style.Default })
    } catch {}
  }
})
</script>

<style scoped>
:deep(video) {
  outline: none !important;
}

:deep(video::-webkit-media-controls) {
  display: none !important;
}

:deep(video::-webkit-media-controls-enclosure) {
  display: none !important;
}

:deep(video::-webkit-media-controls-panel) {
  display: none !important;
}

:deep(.art-video-player) {
  --art-control-height: 44px;
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
  position: relative;
  overflow: hidden;
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

.debug-url {
  font-size: 11px;
  color: var(--encv-text-secondary);
  word-break: break-all;
  margin-top: 8px;
  opacity: 0.7;
}

.error-icon {
  font-size: 64px;
  margin-bottom: 16px;
  color: var(--ion-color-danger);
  opacity: 0.7;
}
</style>
