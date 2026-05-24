import { useState, useEffect, useCallback, useRef } from '@lynx-js/react'
import { useInitData, useLynxGlobalEventListener } from '@lynx-js/react'
import { PlayerControls } from './PlayerControls'

type PlayerState = 'idle' | 'loading' | 'playing' | 'paused' | 'ended' | 'error' | 'audio_only'

const SPEED_OPTIONS = [0.5, 0.75, 1, 1.25, 1.5, 2]
const LOADING_TIMEOUT_MS = 15000

function lynxLogInfo(msg: string) {
  'background only';
  console.info(msg);
  try {
    NativeModules.LogBridge.log('info', msg, () => {});
  } catch (_e) {}
}

function lynxLogError(msg: string) {
  'background only';
  console.error(msg);
  try {
    NativeModules.LogBridge.log('error', msg, () => {});
  } catch (_e) {}
}

function lynxLogWarn(msg: string) {
  'background only';
  console.warn(msg);
  try {
    NativeModules.LogBridge.log('warn', msg, () => {});
  } catch (_e) {}
}

function classifyError(err: string): string {
  const lower = err.toLowerCase()
  if (lower.includes('播放失败') || lower.includes('corrupt') || lower.includes('damaged') || lower.includes('invalid data') || lower.includes('demux')) {
    return '文件损坏'
  }
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
  const [streamUrl, setStreamUrl] = useState('')
  const [locked, setLocked] = useState(false)
  const [mediaType, setMediaType] = useState<'video' | 'audio'>(
    (initData.mediaType as string) === 'audio' ? 'audio' : 'video'
  )

  const hideTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const loadingTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

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

  const clearLoadingTimer = useCallback(() => {
    if (loadingTimerRef.current) {
      clearTimeout(loadingTimerRef.current)
      loadingTimerRef.current = null
    }
  }, [])

  const setError = useCallback((msg: string, type?: string) => {
    lynxLogError('setError: ' + msg + ' (type=' + (type || classifyError(msg)) + ')')
    setErrorMessage(msg)
    setErrorType(type || classifyError(msg))
    setPlayerState('error')
    clearLoadingTimer()
  }, [clearLoadingTimer])

  useLynxGlobalEventListener('mpv:state-change', useCallback((event: any) => {
    const state = event?.state
    const error = event?.error
    lynxLogInfo('mpv:state-change: ' + JSON.stringify(event))

    if (error) {
      lynxLogError('mpv:state-change error: ' + error)
      setError(String(error))
      return
    }

    if (state) {
      if (state === 'surface_ready') {
        lynxLogInfo('MPV surface ready')
        setErrorMessage('')
        setErrorType('')
        return
      }
      if (state === 'waiting_surface') {
        setPlayerState('loading')
        return
      }
      if (state === 'mpv_ready') {
        lynxLogInfo('MPV engine ready')
        return
      }
      if (state === 'audio_only') {
        setMediaType('audio')
        setErrorMessage('')
        setErrorType('')
        setPlayerState('audio_only')
        clearLoadingTimer()
        return
      }
      if (state === 'error') {
        const errDetail = error || 'MPV 进入错误状态'
        lynxLogError('mpv:state-change error state: ' + errDetail + ', filePath=' + filePath)
        setError(errDetail)
        return
      }
      setPlayerState(state as PlayerState)
      if (state === 'playing' || state === 'paused') {
        setErrorMessage('')
        setErrorType('')
        setShowControls(true)
        clearLoadingTimer()
      }
    }
  }, [filePath, setError, clearLoadingTimer]))

  useLynxGlobalEventListener('mpv:position-update', useCallback((event: any) => {
    setPosition(event?.position ?? 0)
    setDuration(event?.duration ?? 0)
  }, []))

  const startPlayback = useCallback(
    (data: { filePath: string; isExternal: boolean; mediaType: string }) => {
      'background only';
      lynxLogInfo('startPlayback: filePath=' + data.filePath + ' isExternal=' + String(data.isExternal) + ' mediaType=' + data.mediaType + ' fileName=' + fileName)
      if (!data.filePath) {
        lynxLogError('startPlayback: filePath is empty!')
        setError('文件路径为空', '地址无效')
        return
      }
      setPlayerState('loading')
      setErrorMessage('')
      setErrorType('')
      clearLoadingTimer()
      loadingTimerRef.current = setTimeout(() => {
        lynxLogWarn('loading timeout after ' + LOADING_TIMEOUT_MS + 'ms')
        setError('播放超时，请检查网络或文件', '网络错误')
      }, LOADING_TIMEOUT_MS)

      ;(async () => {
        try {
          lynxLogInfo('startPlayback: step1 getBackendStatus')
          const status = await new Promise<any>((resolve) => {
            NativeModules.GoBackendModule.getBackendStatus(resolve)
          })
          lynxLogInfo('startPlayback: step1 result=' + JSON.stringify(status))

          if (data.isExternal || !status.running) {
            lynxLogInfo('startPlayback: step2 startBackend')
            await new Promise<any>((resolve) => {
              NativeModules.GoBackendModule.startBackend(resolve)
            })
            lynxLogInfo('startPlayback: step2 done')
          }

          lynxLogInfo('startPlayback: step3 getStreamUrl path=' + data.filePath)
          const resolvedUrl = await new Promise<string>((resolve) => {
            NativeModules.GoBackendModule.getStreamUrl(
              data.filePath,
              data.isExternal,
              resolve
            )
          })
          lynxLogInfo('startPlayback: step3 url=' + resolvedUrl)

          if (!resolvedUrl || !resolvedUrl.startsWith('http')) {
            setError('无法获取播放地址: ' + resolvedUrl + ' (path=' + data.filePath + ', isExternal=' + String(data.isExternal) + ')', '地址无效')
            return
          }
          setStreamUrl(resolvedUrl)

          lynxLogInfo('startPlayback: step4 mpv.play url=' + resolvedUrl)
          const playResult = await new Promise<any>((resolve) => {
            NativeModules.MpvPlayerModule.play(resolvedUrl, resolve)
          })
          lynxLogInfo('startPlayback: step4 result=' + JSON.stringify(playResult))
          if (playResult !== undefined && playResult !== null && playResult !== true) {
            const errMsg = typeof playResult === 'string' ? playResult : JSON.stringify(playResult)
            if (errMsg && errMsg !== 'true') {
              setError('MPV 返回错误: ' + errMsg)
              return
            }
          }
          lynxLogInfo('startPlayback: all steps done, playing')
        } catch (e: any) {
          const msg = e?.message || String(e)
          lynxLogError('startPlayback caught: ' + msg)
          setError('播放异常: ' + msg)
        }
      })()
    },
    [fileName, setError, clearLoadingTimer]
  )

  const handlePlayPause = useCallback(() => {
    if (playerState === 'playing') {
      NativeModules.MpvPlayerModule.pause(() => {})
      setPlayerState('paused')
    } else if (playerState === 'paused') {
      NativeModules.MpvPlayerModule.resume(() => {})
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
      NativeModules.MpvPlayerModule.seekTo(ms, () => {})
      setPosition(ms)
      resetHideTimer()
    },
    [resetHideTimer]
  )

  const handleSeekRelative = useCallback(
    (deltaMs: number) => {
      const newPos = Math.max(0, Math.min(position + deltaMs, duration))
      NativeModules.MpvPlayerModule.seekTo(newPos, () => {})
      setPosition(newPos)
      resetHideTimer()
    },
    [position, duration, resetHideTimer]
  )

  const handleToggleFullscreen = useCallback(() => {
    const next = !isFullscreen
    setIsFullscreen(next)
    NativeModules.MpvPlayerModule.setFullscreen(next, () => {})
    resetHideTimer()
  }, [isFullscreen, resetHideTimer])

  const handleCycleSpeed = useCallback(() => {
    const currentIdx = SPEED_OPTIONS.indexOf(playbackRate)
    const nextIdx = (currentIdx + 1) % SPEED_OPTIONS.length
    const nextRate = SPEED_OPTIONS[nextIdx]
    setPlaybackRate(nextRate)
    NativeModules.MpvPlayerModule.setProperty('speed', String(nextRate), () => {})
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
    NativeModules.MpvPlayerModule.pause(() => {})
    try {
      NativeModules.GoBackendModule.closePlayer(() => {})
    } catch (_e) {}
  }, [])

  useEffect(() => {
    lynxLogInfo('PlayerApp: initData=' + JSON.stringify(initData) + ', filePath=' + filePath + ', fileName=' + fileName + ', isExternal=' + String(isExternal))
  }, [])

  useEffect(() => {
    resetHideTimer()
  }, [playerState, resetHideTimer])

  useEffect(() => {
    if (filePath) {
      startPlayback({ filePath, isExternal, mediaType })
    } else {
      lynxLogError('autoPlay: no filePath in initData: ' + JSON.stringify(initData).substring(0, 200))
      setError('文件路径为空 (initData=' + JSON.stringify(initData).substring(0, 200) + ')', '地址无效')
    }
  }, [])

  useEffect(() => {
    return () => {
      if (hideTimerRef.current) {
        clearTimeout(hideTimerRef.current)
      }
      clearLoadingTimer()
    }
  }, [])

  useEffect(() => {
    return () => {
      try {
        NativeModules.MpvPlayerModule.pause(() => {})
      } catch (_e) {}
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
