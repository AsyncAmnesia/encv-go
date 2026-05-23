import { useState, useEffect, useCallback, useRef } from '@lynx-js/react'
import { PlayerControls } from './PlayerControls'

type PlayerState = 'idle' | 'loading' | 'playing' | 'paused' | 'ended' | 'error' | 'audio_only'

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

function getInitData(): Record<string, any> {
  try {
    const lynxObj = (globalThis as any).lynx
    return lynxObj?.__globalProps || {}
  } catch {
    return {}
  }
}

export function PlayerApp() {
  const initData = getInitData()
  const streamUrl = (initData.streamUrl as string) || ''
  const fileName = (initData.fileName as string) || 'Unknown'
  const mimeType = (initData.mimeType as string) || ''
  const isExternal = !!initData.isExternal

  const [playerState, setPlayerState] = useState<PlayerState>('idle')
  const [position, setPosition] = useState(0)
  const [duration, setDuration] = useState(0)
  const [errorMessage, setErrorMessage] = useState('')
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [showControls, setShowControls] = useState(true)
  const [playbackRate, setPlaybackRate] = useState(1)
  const [locked, setLocked] = useState(false)
  const [mediaType, setMediaType] = useState<'video' | 'audio'>(
    (initData.mediaType as string) === 'audio' ? 'audio' : 'video'
  )

  const hideTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const resetHideTimer = useCallback(() => {
    if (hideTimerRef.current) {
      clearTimeout(hideTimerRef.current)
      hideTimerRef.current = null
    }
    setShowControls(true)
    if (playerState === 'playing') {
      hideTimerRef.current = setTimeout(() => {
        setShowControls(false)
      }, 5000)
    }
  }, [playerState])

  const startPlayback = useCallback(
    (data: { streamUrl: string; mediaType: string }) => {
      lynxLog.info('startPlayback called, data=' + JSON.stringify(data))
      if (!data.streamUrl) {
        lynxLog.error('startPlayback: streamUrl is empty!')
        setPlayerState('error')
        setErrorMessage('播放地址为空')
        return
      }
      setPlayerState('loading')
      setErrorMessage('')

      ;(async () => {
        try {
          lynxLog.info('startPlayback: playing url=' + data.streamUrl)
          await new Promise<any>((resolve) => {
            globalThis.NativeModules.MpvPlayerModule.play(data.streamUrl, resolve)
          })
          lynxLog.info('startPlayback: all steps done, playing')
        } catch (e: any) {
          lynxLog.error('startPlayback caught: ' + (e?.message || String(e)))
          setPlayerState('error')
          setErrorMessage(e?.message || String(e))
        }
      })()
    },
    []
  )

  const handlePlayPause = useCallback(() => {
    if (playerState === 'playing') {
      globalThis.NativeModules.MpvPlayerModule.pause(() => {})
      setPlayerState('paused')
    } else if (playerState === 'paused') {
      globalThis.NativeModules.MpvPlayerModule.resume(() => {})
      setPlayerState('playing')
    } else {
      setErrorMessage('')
      startPlayback({ streamUrl, mediaType })
    }
    resetHideTimer()
  }, [playerState, streamUrl, mediaType, startPlayback, resetHideTimer])

  const handleSeek = useCallback(
    (ms: number) => {
      globalThis.NativeModules.MpvPlayerModule.seekTo(ms, () => {})
      setPosition(ms)
      resetHideTimer()
    },
    [resetHideTimer]
  )

  const handleSeekRelative = useCallback(
    (deltaMs: number) => {
      const newPos = Math.max(0, Math.min(position + deltaMs, duration))
      globalThis.NativeModules.MpvPlayerModule.seekTo(newPos, () => {})
      setPosition(newPos)
      resetHideTimer()
    },
    [position, duration, resetHideTimer]
  )

  const handleToggleFullscreen = useCallback(() => {
    const next = !isFullscreen
    setIsFullscreen(next)
    globalThis.NativeModules.MpvPlayerModule.setFullscreen(next, () => {})
    resetHideTimer()
  }, [isFullscreen, resetHideTimer])

  const handleCycleSpeed = useCallback(() => {
    const currentIdx = SPEED_OPTIONS.indexOf(playbackRate)
    const nextIdx = (currentIdx + 1) % SPEED_OPTIONS.length
    const nextRate = SPEED_OPTIONS[nextIdx]
    setPlaybackRate(nextRate)
    globalThis.NativeModules.MpvPlayerModule.setProperty('speed', String(nextRate), () => {})
    resetHideTimer()
  }, [playbackRate, resetHideTimer])

  const handleToggleLock = useCallback(() => {
    setLocked((prev) => !prev)
    resetHideTimer()
  }, [resetHideTimer])

  const handleToggleControls = useCallback(() => {
    if (locked) {
      setLocked(false)
      return
    }
    setShowControls((prev) => !prev)
    resetHideTimer()
  }, [locked, resetHideTimer])

  const handleBack = useCallback(() => {
    globalThis.NativeModules.MpvPlayerModule.pause(() => {})
    try {
      globalThis.NativeModules.GoBackendModule.closePlayer(() => {})
    } catch (_e) {}
  }, [])

  useEffect(() => {
    const onMpvStateChange = (event: any) => {
      const state = event?.state
      const error = event?.error
      lynxLog.info('mpv:state-change ' + JSON.stringify(event))
      if (state) {
        if (state === 'surface_ready') {
          lynxLog.info('MPV surface ready')
          setErrorMessage('')
          return
        }
        if (state === 'waiting_surface') {
          setPlayerState('loading')
          return
        }
        if (state === 'mpv_ready') {
          lynxLog.info('MPV engine ready')
          return
        }
        if (state === 'audio_only') {
          setMediaType('audio')
          setErrorMessage('')
          setPlayerState('audio_only')
          return
        }
        setPlayerState(state as PlayerState)
      }
      if (error) setErrorMessage(error)
      if (state === 'playing' || state === 'paused') {
        setErrorMessage('')
        setShowControls(true)
      }
    }

    const onMpvPositionUpdate = (event: any) => {
      setPosition(event?.position ?? 0)
      setDuration(event?.duration ?? 0)
    }

    const lynxRuntime = (globalThis as any).lynx
    const globalEventEmitter = lynxRuntime?.getJSModule?.('GlobalEventEmitter')

    if (globalEventEmitter) {
      globalEventEmitter.addListener('mpv:state-change', onMpvStateChange)
      globalEventEmitter.addListener('mpv:position-update', onMpvPositionUpdate)
    } else {
      lynxLog.error('GlobalEventEmitter not available')
    }

    return () => {
      if (globalEventEmitter) {
        globalEventEmitter.removeListener('mpv:state-change', onMpvStateChange)
        globalEventEmitter.removeListener('mpv:position-update', onMpvPositionUpdate)
      }
      try {
        globalThis.NativeModules.MpvPlayerModule.pause(() => {})
      } catch (_e) {}
    }
  }, [])

  useEffect(() => {
    resetHideTimer()
  }, [playerState, resetHideTimer])

  useEffect(() => {
    if (streamUrl) {
      startPlayback({ streamUrl, mediaType })
    } else {
      setPlayerState('error')
      setErrorMessage('播放地址为空')
    }
  }, [])

  useEffect(() => {
    return () => {
      if (hideTimerRef.current) {
        clearTimeout(hideTimerRef.current)
      }
    }
  }, [])

  return (
    <view className="PlayerContainer" bindtap={handleToggleControls}>
      <PlayerControls
        state={playerState}
        isFullscreen={isFullscreen}
        fileName={fileName}
        currentTime={position}
        duration={duration}
        showControls={showControls}
        error={errorMessage || undefined}
        mediaType={mediaType}
        playbackRate={playbackRate}
        locked={locked}
        onPlayPause={handlePlayPause}
        onSeek={handleSeek}
        onSeekRelative={handleSeekRelative}
        onToggleFullscreen={handleToggleFullscreen}
        onCycleSpeed={handleCycleSpeed}
        onToggleLock={handleToggleLock}
        onBack={handleBack}
      />
    </view>
  )
}
