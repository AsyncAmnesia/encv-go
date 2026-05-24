import { useCallback } from '@lynx-js/react'
import { ProgressBar } from './ProgressBar'

interface PlayerControlsProps {
  state: string
  isFullscreen: boolean
  fileName: string
  currentTime: number
  duration: number
  showControls: boolean
  error?: string
  errorType?: string
  mediaType: 'video' | 'audio'
  playbackRate: number
  locked: boolean
  streamUrl?: string
  onPlayPause: () => void
  onSeek: (ms: number) => void
  onSeekRelative: (ms: number) => void
  onToggleFullscreen: () => void
  onCycleSpeed: () => void
  onToggleLock: () => void
  onBack: () => void
}

function formatTime(ms: number): string {
  if (!isFinite(ms) || ms < 0) return '0:00'
  const totalSec = Math.floor(ms / 1000)
  const h = Math.floor(totalSec / 3600)
  const m = Math.floor((totalSec % 3600) / 60)
  const s = totalSec % 60
  if (h > 0) return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
  return `${m}:${s.toString().padStart(2, '0')}`
}

export function PlayerControls(props: PlayerControlsProps) {
  const {
    state, fileName, currentTime, duration, showControls,
    error, errorType, mediaType, playbackRate, locked, streamUrl,
    onPlayPause, onSeek, onSeekRelative, onToggleFullscreen,
    onCycleSpeed, onToggleLock, onBack,
  } = props

  const isPlaying = state === 'playing'
  const progress = duration > 0 ? currentTime / duration : 0
  const currentTimeStr = formatTime(currentTime)
  const durationStr = formatTime(duration)
  const isError = !!error
  const isLoading = state === 'loading'

  const handleSeek = useCallback(
    (progressVal: number) => {
      onSeek(progressVal * duration)
    },
    [duration, onSeek]
  )

  if (isError) {
    return (
      <view className="ErrorContainer">
        <view className="PlayBtn" bindtap={onPlayPause}>
          <view className="PlayBtnInner">
            <text className="PlayIcon">🔄</text>
          </view>
        </view>
        <text className="ErrorTitle">⚠ 播放失败</text>
        {errorType ? <text className="ErrorType">[{errorType}]</text> : null}
        <text className="ErrorDetail">{fileName}</text>
        {error ? <text className="ErrorDetail">{error}</text> : null}
        {streamUrl ? <text className="ErrorDetail">{streamUrl}</text> : null}
      </view>
    )
  }

  if (isLoading) {
    return (
      <view className="VideoOverlay">
        <view className="TopGradient" />
        <view className="TopBar">
          <view className="CtrlBtn" bindtap={onBack}>
            <text className="IconMd">✕</text>
          </view>
          <text className="TitleText">{fileName}</text>
          <view className="TopBarSpacer" />
        </view>
        <view className="CenterArea">
          <view className="LoadingSpinner" />
        </view>
      </view>
    )
  }

  if (locked) {
    return (
      <view className="LockedOverlay">
        <view className="TopGradient" />
        <view className="LockBar">
          <view className="CtrlBtn" bindtap={onToggleLock}>
            <text className="IconSm">🔒</text>
          </view>
          <view className="FlexSpacer" />
        </view>
        <view className="BottomGradient" />
        <view className="BottomBar">
          <ProgressBar progress={progress} currentTime={currentTimeStr} duration={durationStr} onSeek={handleSeek} />
        </view>
      </view>
    )
  }

  if (mediaType === 'audio') {
    return (
      <view className="AudioOverlay">
        <view className="TopBar">
          <view className="CtrlBtn" bindtap={onBack}>
            <text className="IconMd">✕</text>
          </view>
          <text className="TitleText">{fileName}</text>
          <view className="TopBarSpacer" />
        </view>
        <view className="AudioCoverContainer">
          <view className="AudioCover">
            <text className="AudioCoverIcon">🎵</text>
          </view>
          <text className="AudioTitle">{fileName}</text>
        </view>
        <view className="AudioBottomSection">
          <ProgressBar progress={progress} currentTime={currentTimeStr} duration={durationStr} onSeek={handleSeek} />
          <view className="AudioPlayRow">
            <view className="SeekBtn" bindtap={() => onSeekRelative(-10000)}>
              <view className="SeekBtnInner">
                <text className="SeekIcon">-10</text>
              </view>
            </view>
            <view className="PlayBtn" bindtap={onPlayPause}>
              <view className="PlayBtnInner">
                <text className="PlayIcon">{isPlaying ? '❚❚' : '▶'}</text>
              </view>
            </view>
            <view className="SeekBtn" bindtap={() => onSeekRelative(10000)}>
              <view className="SeekBtnInner">
                <text className="SeekIcon">+10</text>
              </view>
            </view>
            <view className="SpeedChip" bindtap={onCycleSpeed}>
              <text className="SpeedText">{playbackRate}x</text>
            </view>
          </view>
        </view>
      </view>
    )
  }

  return (
    <view className="VideoOverlay">
      <view className="TopGradient" />
      <view className="TopBar">
        <view className="CtrlBtn" bindtap={onBack}>
          <text className="IconMd">✕</text>
        </view>
        <text className="TitleText">{fileName}</text>
        <view className="CtrlBtn" bindtap={onToggleLock}>
          <text className="IconSm">🔒</text>
        </view>
      </view>
      {showControls && (
        <view className="CenterArea">
          <view className="CenterControls">
            <view className="SeekBtn" bindtap={() => onSeekRelative(-10000)}>
              <view className="SeekBtnInner">
                <text className="SeekIcon">-10</text>
              </view>
            </view>
            <view className="PlayBtn" bindtap={onPlayPause}>
              <view className="PlayBtnInner">
                <text className="PlayIcon">{isPlaying ? '❚❚' : '▶'}</text>
              </view>
            </view>
            <view className="SeekBtn" bindtap={() => onSeekRelative(10000)}>
              <view className="SeekBtnInner">
                <text className="SeekIcon">+10</text>
              </view>
            </view>
          </view>
        </view>
      )}
      <view className="BottomGradient" />
      <view className="BottomBar">
        <ProgressBar progress={progress} currentTime={currentTimeStr} duration={durationStr} onSeek={handleSeek} />
        <view className="BottomActions">
          <view className="SpeedChip" bindtap={onCycleSpeed}>
            <text className="SpeedText">{playbackRate}x</text>
          </view>
          <view className="FlexSpacer" />
          <view className="CtrlBtn" bindtap={onToggleFullscreen}>
            <text className="IconMd">⛶</text>
          </view>
        </view>
      </view>
    </view>
  )
}
