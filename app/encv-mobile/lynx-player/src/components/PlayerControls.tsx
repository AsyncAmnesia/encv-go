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
  playbackRate: number;
  locked: boolean;
  onPlayPause: () => void;
  onSeek: (positionMs: number) => void;
  onSeekRelative: (deltaMs: number) => void;
  onToggleFullscreen: () => void;
  onCycleSpeed: () => void;
  onToggleLock: () => void;
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
    <view className="ProgressRow">
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
  playbackRate,
  locked,
  onPlayPause,
  onSeek,
  onSeekRelative,
  onToggleFullscreen,
  onCycleSpeed,
  onToggleLock,
  onBack,
}: Omit<PlayerControlsProps, 'error' | 'mediaType'>) {
  const isPlaying = state === 'playing';
  if (locked) {
    return (
      <view style={{ flex: 1, flexDirection: 'column', justifyContent: 'flex-end' }}>
        <view className="BottomGradient" />
        <view className="LockBar">
          <Button onClick={onToggleLock} className="CtrlBtn">
            <text className="IconSm">&#x1F512;</text>
          </Button>
          <view style={{ flex: 1 }} />
        </view>
        <ProgressBar currentTime={currentTime} duration={duration} onSeek={onSeek} />
      </view>
    );
  }
  return (
    <view style={{ flex: 1, flexDirection: 'column', justifyContent: 'space-between' }}>
      <view className="TopGradient" />
      <view className="TopBar">
        <Button onClick={onBack} className="CtrlBtn">
          <text className="IconMd">&#x2715;</text>
        </Button>
        <text className="TitleText">{fileName}</text>
        <Button onClick={onToggleLock} className="CtrlBtn">
          <text className="IconSm">&#x1F512;</text>
        </Button>
      </view>
      <view className="CenterArea">
        {showControls && (
          <view className="CenterControls">
            <Button onClick={() => onSeekRelative(-10000)} className="SeekBtn">
              <view className="SeekBtnInner">
                <text className="SeekIcon">-10</text>
              </view>
            </Button>
            <Button onClick={onPlayPause} className="PlayBtn">
              <view className="PlayBtnInner">
                <text className="PlayIcon">{isPlaying ? '\u275A\u275A' : '\u25B6'}</text>
              </view>
            </Button>
            <Button onClick={() => onSeekRelative(10000)} className="SeekBtn">
              <view className="SeekBtnInner">
                <text className="SeekIcon">+10</text>
              </view>
            </Button>
          </view>
        )}
      </view>
      <view className="BottomGradient" />
      <view className="BottomBar">
        <ProgressBar currentTime={currentTime} duration={duration} onSeek={onSeek} />
        <view className="BottomActions">
          <Button onClick={onCycleSpeed} className="SpeedChip">
            <text className="SpeedText">{playbackRate}x</text>
          </Button>
          <view style={{ flex: 1 }} />
          <Button onClick={onToggleFullscreen} className="CtrlBtn">
            <text className="IconMd">{isFullscreen ? '\u2913' : '\u2912'}</text>
          </Button>
        </view>
      </view>
    </view>
  );
}

function AudioControls({
  state,
  fileName,
  currentTime,
  duration,
  playbackRate,
  onPlayPause,
  onSeek,
  onSeekRelative,
  onCycleSpeed,
  onBack,
}: Omit<PlayerControlsProps, 'error' | 'mediaType' | 'isFullscreen' | 'showControls' | 'onToggleFullscreen' | 'onToggleLock' | 'locked'>) {
  const isPlaying = state === 'playing';
  return (
    <view style={{ flex: 1, flexDirection: 'column', justifyContent: 'space-between' }}>
      <view className="TopBar">
        <Button onClick={onBack} className="CtrlBtn">
          <text className="IconMd">&#x2715;</text>
        </Button>
        <text className="TitleText">{fileName}</text>
        <view style={{ width: 44 }} />
      </view>
      <view className="AudioCoverContainer">
        <view className="AudioCover">
          <text className="AudioCoverIcon">&#x1F3B5;</text>
        </view>
        <text className="AudioTitle">{fileName}</text>
      </view>
      <view className="AudioBottomSection">
        <ProgressBar currentTime={currentTime} duration={duration} onSeek={onSeek} />
        <view className="AudioPlayRow">
          <Button onClick={() => onSeekRelative(-10000)} className="SeekBtn">
            <view className="SeekBtnInner">
              <text className="SeekIcon">-10</text>
            </view>
          </Button>
          <Button onClick={onPlayPause} className="PlayBtn">
            <view className="PlayBtnInner">
              <text className="PlayIcon">{isPlaying ? '\u275A\u275A' : '\u25B6'}</text>
            </view>
          </Button>
          <Button onClick={() => onSeekRelative(10000)} className="SeekBtn">
            <view className="SeekBtnInner">
              <text className="SeekIcon">+10</text>
            </view>
          </Button>
          <Button onClick={onCycleSpeed} className="SpeedChip">
            <text className="SpeedText">{playbackRate}x</text>
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
  playbackRate,
  locked,
  onPlayPause,
  onSeek,
  onSeekRelative,
  onToggleFullscreen,
  onCycleSpeed,
  onToggleLock,
  onBack,
}: PlayerControlsProps) {
  if (error && state !== 'loading') {
    return (
      <view className="ErrorContainer">
        <Button onClick={onPlayPause} className="PlayBtn">
          <view className="PlayBtnInner">
            <text className="PlayIcon">&#x1F504;</text>
          </view>
        </Button>
        <text className="ErrorTitle">&#x26A0; 播放失败</text>
        <text className="ErrorDetail">{error || "未知错误，点击重试"}</text>
      </view>
    );
  }

  if (state === 'loading' || state === 'idle') {
    return (
      <view className="CenterArea">
        <view className="LoadingSpinner" />
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
        playbackRate={playbackRate}
        onPlayPause={onPlayPause}
        onSeek={onSeek}
        onSeekRelative={onSeekRelative}
        onCycleSpeed={onCycleSpeed}
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
      playbackRate={playbackRate}
      locked={locked}
      onPlayPause={onPlayPause}
      onSeek={onSeek}
      onSeekRelative={onSeekRelative}
      onToggleFullscreen={onToggleFullscreen}
      onCycleSpeed={onCycleSpeed}
      onToggleLock={onToggleLock}
      onBack={onBack}
    />
  );
}
