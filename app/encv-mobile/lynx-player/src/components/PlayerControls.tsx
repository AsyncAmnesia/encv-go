import React from '@lynx-js/react';
import {
  SliderRoot,
  SliderTrack,
  SliderIndicator,
  SliderThumb,
  Button,
} from '@lynx-js/lynx-ui';

interface PlayerControlsProps {
  state: string;
  isFullscreen: boolean;
  fileName: string;
  currentTime: number;
  duration: number;
  showControls: boolean;
  error?: string | null;
  mediaType: "video" | "audio";
  onPlayPause: () => void;
  onSeek: (positionMs: number) => void;
  onToggleFullscreen: () => void;
  onBack: () => void;
}

function formatTime(ms: number): string {
  if (!isFinite(ms) || ms < 0) return '0:00';
  const totalSec = Math.floor(ms / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (h > 0) return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  return `${m}:${s.toString().padStart(2, '0')}`;
}

function ProgressBar({ currentTime, duration, onSeek }: { currentTime: number; duration: number; onSeek: (ms: number) => void }) {
  const progress = duration > 0 ? currentTime / duration : 0;
  return (
    <view className="BottomBar">
      <text className="TimeLabel">{formatTime(currentTime)}</text>
      <SliderRoot
        value={progress}
        onValueChange={(val) => {
          if (duration > 0) onSeek(val * duration);
        }}
        onValueCommit={(val) => {
          if (duration > 0) onSeek(val * duration);
        }}
        className="PlayerSlider"
      >
        <SliderTrack className="SliderTrackOuter">
          <SliderIndicator className="SliderFill" />
          <SliderThumb className="SliderThumbWrapper">
            <view className="SliderThumbDot" />
          </SliderThumb>
        </SliderTrack>
      </SliderRoot>
      <text className="TimeLabelEnd">{formatTime(duration)}</text>
    </view>
  );
}

function VideoControls({
  state,
  isFullscreen,
  fileName,
  currentTime,
  duration,
  showControls,
  onPlayPause,
  onSeek,
  onToggleFullscreen,
  onBack,
}: Omit<PlayerControlsProps, 'error' | 'mediaType'>) {
  const isPlaying = state === 'playing';
  return (
    <view style={{ flex: 1, flexDirection: 'column', justifyContent: 'space-between' }}>
      <view className="TopBar">
        <Button onClick={onBack} className="TopBarBtn">
          <text className="BackButton">✕</text>
        </Button>
        <text className="TitleText">{fileName}</text>
        <Button onClick={onToggleFullscreen} className="TopBarBtn">
          <text className="FullscreenButton">
            {isFullscreen ? '⤓' : '⤢'}
          </text>
        </Button>
      </view>
      <view className="CenterArea">
        {showControls && (
          <Button onClick={onPlayPause} className="CenterPlayBtn">
            {({ active }) => (
              <view className="PlayButtonCircle">
                <text className="PlayIconLarge">{isPlaying ? '⏸' : '▶'}</text>
              </view>
            )}
          </Button>
        )}
      </view>
      <ProgressBar currentTime={currentTime} duration={duration} onSeek={onSeek} />
    </view>
  );
}

function AudioControls({
  state,
  fileName,
  currentTime,
  duration,
  onPlayPause,
  onSeek,
  onBack,
}: Omit<PlayerControlsProps, 'error' | 'mediaType' | 'isFullscreen' | 'showControls' | 'onToggleFullscreen'>) {
  const isPlaying = state === 'playing';
  return (
    <view style={{ flex: 1, flexDirection: 'column', justifyContent: 'space-between' }}>
      <view className="TopBar">
        <Button onClick={onBack} className="TopBarBtn">
          <text className="BackButton">✕</text>
        </Button>
        <text className="TitleText">{fileName}</text>
      </view>
      <view className="AudioCoverContainer">
        <view className="AudioCover">
          <text className="AudioCoverIcon">🎵</text>
        </view>
        <text className="AudioTitle">{fileName}</text>
      </view>
      <view className="AudioBottomSection">
        <ProgressBar currentTime={currentTime} duration={duration} onSeek={onSeek} />
        <view className="AudioPlayControls">
          <Button onClick={onPlayPause} className="CenterPlayBtn">
            <view className="PlayButtonCircle">
              <text className="PlayIconLarge">{isPlaying ? '⏸' : '▶'}</text>
            </view>
          </Button>
        </view>
      </view>
    </view>
  );
}

export function PlayerControls({
  state,
  isFullscreen,
  fileName,
  currentTime,
  duration,
  showControls,
  error,
  mediaType,
  onPlayPause,
  onSeek,
  onToggleFullscreen,
  onBack,
}: PlayerControlsProps) {
  if (error && state !== 'loading') {
    return (
      <view className="ErrorContainer">
        <Button onClick={onPlayPause} className="ErrorRetryBtn">
          <view className="PlayButtonCircle">
            <text className="PlayIconLarge">🔄</text>
          </view>
        </Button>
        <text className="ErrorTitle">⚠ 播放失败</text>
        <text className="ErrorDetail">{error || "未知错误，点击重试"}</text>
      </view>
    );
  }

  if (state === 'loading') {
    return (
      <view className="CenterArea">
        <view className="LoadingDots">
          <view className="Dot DotDim1" />
          <view className="Dot DotDim2" />
          <view className="Dot DotDim3" />
        </view>
        <text className="IdleTitle">正在加载...</text>
      </view>
    );
  }

  if (state === 'idle') {
    return (
      <view className="CenterArea">
        <Button onClick={onPlayPause} className="IdlePlayBtn">
          <view className="PlayButtonCircle">
            <text className="PlayIconLarge">▶</text>
          </view>
        </Button>
        <text className="IdleTitle">{fileName || "等待文件信息..."}</text>
      </view>
    );
  }

  if (mediaType === 'audio') {
    return (
      <AudioControls
        state={state}
        isFullscreen={isFullscreen}
        fileName={fileName}
        currentTime={currentTime}
        duration={duration}
        showControls={showControls}
        onPlayPause={onPlayPause}
        onSeek={onSeek}
        onToggleFullscreen={onToggleFullscreen}
        onBack={onBack}
      />
    );
  }

  return (
    <VideoControls
      state={state}
      isFullscreen={isFullscreen}
      fileName={fileName}
      currentTime={currentTime}
      duration={duration}
      showControls={showControls}
      onPlayPause={onPlayPause}
      onSeek={onSeek}
      onToggleFullscreen={onToggleFullscreen}
      onBack={onBack}
    />
  );
}
