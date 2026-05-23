<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import PlayerControls from '../components/PlayerControls.vue'

// TODO: Verify NativeModules access pattern in vue-lynx — confirm globalThis.NativeModules works the same as @lynx-js/react

type PlayerState = 'idle' | 'loading' | 'playing' | 'paused' | 'ended' | 'error'

const SPEED_OPTIONS = [0.5, 0.75, 1, 1.25, 1.5, 2]

const lynxLog = {
  info: (msg: string) => {
    try {
      console.info(msg)
      globalThis.NativeModules.LogBridgeModule.log('info', msg, () => {})
    } catch (_e) {
      console.info(msg)
    }
  },
  error: (msg: string) => {
    try {
      console.error(msg)
      globalThis.NativeModules.LogBridgeModule.log('error', msg, () => {})
    } catch (_e) {
      console.error(msg)
    }
  },
}

const router = useRouter()
const route = useRoute()

const filePath = computed(() => (route.query.filePath as string) || '')
const fileName = computed(() => (route.query.fileName as string) || 'Unknown')
const mimeType = computed(() => (route.query.mimeType as string) || '')
const isExternal = computed(() => route.query.isExternal === 'true')
const mediaType = ref<'video' | 'audio'>(
  (route.query.mediaType as string) === 'audio' ? 'audio' : 'video'
)

const playerState = ref<PlayerState>('idle')
const position = ref(0)
const duration = ref(0)
const errorMessage = ref('')
const isFullscreen = ref(false)
const showControls = ref(true)
const playbackRate = ref(1)
const locked = ref(false)

const playlist = ref<any[]>([])
const playlistIndex = ref(-1)

let hideTimer: ReturnType<typeof setTimeout> | null = null

function resetHideTimer() {
  if (hideTimer) {
    clearTimeout(hideTimer)
    hideTimer = null
  }
  showControls.value = true
  if (playerState.value === 'playing') {
    hideTimer = setTimeout(() => {
      showControls.value = false
    }, 5000)
  }
}

function startPlayback(data: {
  filePath: string
  isExternal: boolean
  mediaType: string
}) {
  lynxLog.info('startPlayback called, data=' + JSON.stringify(data))
  if (!data.filePath) {
    lynxLog.error('startPlayback: filePath is empty!')
    playerState.value = 'error'
    errorMessage.value = '文件路径为空'
    return
  }
  playerState.value = 'loading'
  errorMessage.value = ''

  ;(async () => {
    try {
      lynxLog.info('startPlayback: step1 getBackendStatus')
      const status = await new Promise<any>((resolve) => {
        globalThis.NativeModules.GoBackendModule.getBackendStatus(resolve)
      })
      lynxLog.info('startPlayback: step1 result=' + JSON.stringify(status))

      if (data.isExternal || !status.running) {
        lynxLog.info('startPlayback: step2 startBackend')
        await new Promise<any>((resolve) => {
          globalThis.NativeModules.GoBackendModule.startBackend(resolve)
        })
        lynxLog.info('startPlayback: step2 done')
      }

      lynxLog.info('startPlayback: step3 getStreamUrl path=' + data.filePath)
      const streamUrl = await new Promise<string>((resolve) => {
        globalThis.NativeModules.GoBackendModule.getStreamUrl(
          data.filePath,
          data.isExternal,
          resolve
        )
      })
      lynxLog.info('startPlayback: step3 url=' + streamUrl)

      lynxLog.info('startPlayback: step4 mpv.play url=' + streamUrl)
      await new Promise<any>((resolve) => {
        globalThis.NativeModules.MpvPlayerModule.play(streamUrl, resolve)
      })
      lynxLog.info('startPlayback: all steps done, playing')
    } catch (e: any) {
      lynxLog.error('startPlayback caught: ' + (e?.message || String(e)))
      playerState.value = 'error'
      errorMessage.value = e?.message || String(e)
    }
  })()
}

function handlePlayPause() {
  if (playerState.value === 'playing') {
    globalThis.NativeModules.MpvPlayerModule.pause(() => {})
    playerState.value = 'paused'
  } else if (playerState.value === 'paused') {
    globalThis.NativeModules.MpvPlayerModule.resume(() => {})
    playerState.value = 'playing'
  } else {
    errorMessage.value = ''
    startPlayback({
      filePath: filePath.value,
      isExternal: isExternal.value,
      mediaType: mediaType.value,
    })
  }
  resetHideTimer()
}

function handleSeek(positionMs: number) {
  globalThis.NativeModules.MpvPlayerModule.seekTo(positionMs, () => {})
  position.value = positionMs
  resetHideTimer()
}

function handleSeekRelative(deltaMs: number) {
  const newPos = Math.max(0, Math.min(position.value + deltaMs, duration.value))
  globalThis.NativeModules.MpvPlayerModule.seekTo(newPos, () => {})
  position.value = newPos
  resetHideTimer()
}

function handleToggleFullscreen() {
  const next = !isFullscreen.value
  isFullscreen.value = next
  globalThis.NativeModules.MpvPlayerModule.setFullscreen(next, () => {})
  resetHideTimer()
}

function handleCycleSpeed() {
  const currentIdx = SPEED_OPTIONS.indexOf(playbackRate.value)
  const nextIdx = (currentIdx + 1) % SPEED_OPTIONS.length
  const nextRate = SPEED_OPTIONS[nextIdx]
  playbackRate.value = nextRate
  globalThis.NativeModules.MpvPlayerModule.setProperty(
    'speed',
    String(nextRate),
    () => {}
  )
  resetHideTimer()
}

function handleToggleLock() {
  locked.value = !locked.value
  resetHideTimer()
}

function handleToggleControls() {
  if (locked.value) {
    locked.value = false
    return
  }
  showControls.value = !showControls.value
  resetHideTimer()
}

function handleBackToHome() {
  globalThis.NativeModules.MpvPlayerModule.pause(() => {})
  router.push({ name: 'home' })
}

function handleNext() {
  if (playlist.value.length === 0 || playlistIndex.value >= playlist.value.length - 1)
    return
  const nextIdx = playlistIndex.value + 1
  playlistIndex.value = nextIdx
  const nextItem = playlist.value[nextIdx]
  if (nextItem) {
    mediaType.value = nextItem.mediaType || 'video'
    startPlayback({
      filePath: nextItem.filePath,
      isExternal: nextItem.isExternal || false,
      mediaType: nextItem.mediaType || 'video',
    })
  }
}

function handlePrev() {
  if (playlist.value.length === 0 || playlistIndex.value <= 0) return
  const prevIdx = playlistIndex.value - 1
  playlistIndex.value = prevIdx
  const prevItem = playlist.value[prevIdx]
  if (prevItem) {
    mediaType.value = prevItem.mediaType || 'video'
    startPlayback({
      filePath: prevItem.filePath,
      isExternal: prevItem.isExternal || false,
      mediaType: prevItem.mediaType || 'video',
    })
  }
}

function handlePlaylist() {
  router.push({ name: 'playlist' })
}

function handleSettings() {
  router.push({ name: 'settings' })
}

const hasPlaylist = computed(() => playlist.value.length > 1)
const hasNext = computed(
  () => hasPlaylist.value && playlistIndex.value < playlist.value.length - 1
)
const hasPrev = computed(() => hasPlaylist.value && playlistIndex.value > 0)

function onMpvStateChange(event: any) {
  const state = event?.state
  const error = event?.error
  lynxLog.info('mpv:state-change ' + JSON.stringify(event))
  if (state) {
    if (state === 'surface_ready') {
      lynxLog.info('MPV surface ready, native will auto-play pending URL')
      errorMessage.value = ''
      return
    }
    if (state === 'waiting_surface') {
      playerState.value = 'loading'
      return
    }
    if (state === 'mpv_ready') {
      lynxLog.info('MPV engine ready')
      return
    }
    if (state === 'audio_only') {
      mediaType.value = 'audio'
      errorMessage.value = ''
      playerState.value = state as PlayerState
      return
    }
    playerState.value = state as PlayerState
  }
  if (error) errorMessage.value = error
  if (state === 'playing' || state === 'paused') {
    errorMessage.value = ''
    showControls.value = true
    resetHideTimer()
  }
}

function onMpvPositionUpdate(event: any) {
  const pos = event?.position ?? 0
  const dur = event?.duration ?? 0
  position.value = pos
  duration.value = dur
}

onMounted(() => {
  try {
    globalThis.addEventListener('mpv:state-change', onMpvStateChange)
    globalThis.addEventListener('mpv:position-update', onMpvPositionUpdate)
  } catch (_e) {
    lynxLog.error('Failed to register global event listeners')
  }
})

watch(playerState, () => {
  resetHideTimer()
})

onMounted(() => {
  if (filePath.value) {
    startPlayback({
      filePath: filePath.value,
      isExternal: isExternal.value,
      mediaType: mediaType.value,
    })
  }
})

onUnmounted(() => {
  if (hideTimer) {
    clearTimeout(hideTimer)
    hideTimer = null
  }
  try {
    globalThis.removeEventListener('mpv:state-change', onMpvStateChange)
    globalThis.removeEventListener('mpv:position-update', onMpvPositionUpdate)
  } catch (_e) {
    // ignore
  }
  try {
    globalThis.NativeModules.MpvPlayerModule.pause(() => {})
  } catch (_e) {
    // ignore
  }
})
</script>

<template>
  <view class="PlayerContainer" @tap="handleToggleControls">
    <PlayerControls
      :state="playerState"
      :is-fullscreen="isFullscreen"
      :file-name="fileName"
      :current-time="position"
      :duration="duration"
      :show-controls="showControls"
      :error="errorMessage || undefined"
      :media-type="mediaType"
      :playback-rate="playbackRate"
      :locked="locked"
      :has-next="hasNext"
      :has-prev="hasPrev"
      @play-pause="handlePlayPause"
      @seek="handleSeek"
      @seek-relative="handleSeekRelative"
      @toggle-fullscreen="handleToggleFullscreen"
      @cycle-speed="handleCycleSpeed"
      @toggle-lock="handleToggleLock"
      @back="handleBackToHome"
      @next="handleNext"
      @prev="handlePrev"
      @playlist="handlePlaylist"
      @settings="handleSettings"
    />
  </view>
</template>

<style scoped>
.PlayerContainer {
  width: 100%;
  height: 100%;
  background-color: #111;
  flex-direction: column;
  position: relative;
}
</style>
