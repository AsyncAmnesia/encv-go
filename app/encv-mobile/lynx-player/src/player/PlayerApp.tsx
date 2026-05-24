import { useState, useEffect, useCallback, useRef } from '@lynx-js/react'
import { useInitData } from '@lynx-js/react'
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
  warn: (msg: string) => {
    try {
      console.warn(msg)
      globalThis.NativeModules.LogBridgeModule.log('warn', msg, () => {})
    } catch (_e) {
      console.warn(msg)
    }
  },
}

function classifyError(err: string): string {
  const lower = err.toLowerCase()
  if (lower.includes('network') || lower.includes('connection') || lower.includes('timeout') || lower.includes('http 4') || lower.includes('http 5')) {
    return '网络错误'
  }
  if (lower.includes('not found') || lower.includes('no such file') || lower.includes('enoent') || lower.includes('does not exist')) {
    return '文件不存在'
  }
  if (lower.includes('decode') || lower.includes('codec') || lower.includes('format') || lower.includes('unsupported')) {
    return '解码失败'
  }
  if (lower.includes('empty') || lower.includes('为空') || lower.includes('地址')) {
    return '地址无效'
  }
  if (lower.includes('permission') || lower.includes('forbidden') || lower.includes('denied') || lower.includes('access')) {
    return '权限不足'
  }
  return '未知错误'
}

export function PlayerApp() {
  const initData = useInitData() as Record<string, any>
  const filePath = (initData.filePath as string) || ''
  const fileName = (initData.fileName as string) || 'Unknown'
  const isExternal = !!initData.isExternal

  const [playerState, setPlayerState] = useState<PlayerState>('idle')
  const [position, setPosition] = useState(0)
  const [duration, setDuration] = useState(0)
  const [errorMessage, setErrorMessage] = useState('')
  const [errorType, setErrorType] = useState('')
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [showControls, setShowControls] = useState(true)
  const [playbackRate, setPlaybackRate] = useState(1)
  const [locked, setLocked] = useState(false)
  const [streamUrl, setStreamUrl] = useState('')
  const [mediaType, setMediaType] = useState<'video' | 'audio'>(
    (initData.mediaType as string) === 'audio' ? 'audio' : 'video'
  )

  const hideTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  lynxLog.info('PlayerApp: initData=' + JSON.stringify(initData) + ', filePath=' + filePath + ', fileName=' + fileName + ', isExternal=' + String(isExternal))

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

  const setError = useCallback((msg: string, type?: string) => {
    lynxLog.error('setError: ' + msg + ' (type=' + (type || classifyError(msg)) + ')')
    setErrorMessage(msg)
    setErrorType(type || classifyError(msg))
    setPlayerState('error')
  }, [])

  const startPlayback = useCallback(
    (data: { filePath: string; isExternal: boolean; mediaType: string }) => {
      lynxLog.info('startPlayback: filePath=' + data.filePath + ' isExternal=' + String(data.isExternal) + ' mediaType=' + data.mediaType + ' fileName=' + fileName)
      if (!data.filePath) {
        lynxLog.error('startPlayback: filePath is empty!')
        setError('文件路径为空', '地址无效')
        return
      }
      setPlayerState('loading')
      setErrorMessage('')
      setErrorType('')

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
          const resolvedUrl = await new Promise<string>((resolve) => {
            globalThis.NativeModules.GoBackendModule.getStreamUrl(
              data.filePath,
              data.isExternal,
              resolve
            )
          })
          lynxLog.info('startPlayback: step3 url=' + resolvedUrl)

          if (!resolvedUrl || !resolvedUrl.startsWith('http')) {
            setError('无法获取播放地址: ' + resolvedUrl + ' (path=' + data.filePath + ', isExternal=' + String(data.isExternal) + ')', '地址无效')
            return
          }
          setStreamUrl(resolvedUrl)

          lynxLog.info('startPlayback: step4 mpv.play url=' + resolvedUrl)
          const playResult = await new Promise<any>((resolve) => {
            globalThis.NativeModules.MpvPlayerModule.play(resolvedUrl, resolve)
          })
          lynxLog.info('startPlayback: step4 result=' + JSON.stringify(playResult))
          if (playResult !== undefined && playResult !== null && playResult !== true) {
            const errMsg = typeof playResult === 'string' ? playResult : JSON.stringify(playResult)
            if (errMsg && errMsg !== 'true') {
              setError('MPV 返回错误: ' + errMsg)
              return
            }
          }
          lynxLog.info('startPlayback: all steps done, playing')
        } catch (e: any) {
          const msg = e?.message || String(e)
          lynxLog.error('startPlayback caught: ' + msg)
          setError('播放异常: ' + msg)
        }
      })()
    },
    [fileName, setError]
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
      setErrorType('')
      startPlayback({ filePath, isExternal, mediaType })
    }
    resetHideTimer()
  }, [playerState, filePath, isExternal, mediaType, startPlayback, resetHideTimer])

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
      lynxLog.info('mpv:state-change: ' + JSON.stringify(event))
      if (state) {
        if (state === 'surface_ready') {
          lynxLog.info('MPV surface ready')
          setErrorMessage('')
          setErrorType('')
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
          setErrorType('')
          setPlayerState('audio_only')
          return
        }
        if (state === 'error') {
          const errDetail = error || 'MPV 进入错误状态'
          lynxLog.error('mpv:state-change error state: ' + errDetail + ', filePath=' + filePath)
          setError(errDetail)
          return
        }
        setPlayerState(state as PlayerState)
      }
      if (error) {
        lynxLog.error('mpv:state-change error field: ' + error)
        setError(String(error))
      }
      if (state === 'playing' || state === 'paused') {
        setErrorMessage('')
        setErrorType('')
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
    if (filePath) {
      startPlayback({ filePath, isExternal, mediaType })
    } else {
      lynxLog.error('autoPlay: no filePath in initData: ' + JSON.stringify(initData).substring(0, 200))
      setError('文件路径为空 (initData=' + JSON.stringify(initData).substring(0, 200) + ')', '地址无效')
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
        errorType={errorType || undefined}
        mediaType={mediaType}
        playbackRate={playbackRate}
        locked={locked}
        streamUrl={streamUrl || undefined}
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
