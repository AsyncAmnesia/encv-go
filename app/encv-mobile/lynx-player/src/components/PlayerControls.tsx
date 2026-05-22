import React from '@lynx-js/react';

interface PlayerControlsProps {
  state: string;
  isFullscreen: boolean;
  fileName: string;
  currentTime: number;
  duration: number;
  showControls: boolean;
  error?: string | null;
  onPlayPause: () => void;
  onSeek: (positionMs: number) => void;
  onToggleFullscreen: () => void;
  onBack: () => void;
}

export function PlayerControls({
  state,
  isFullscreen,
  fileName,
  currentTime,
  duration,
  showControls,
  error,
  onPlayPause,
  onSeek,
  onToggleFullscreen,
  onBack,
}: PlayerControlsProps) {
  const formatTime = (ms: number): string => {
    if (!isFinite(ms) || ms < 0) return '0:00';
    const totalSec = Math.floor(ms / 1000);
    const h = Math.floor(totalSec / 3600);
    const m = Math.floor((totalSec % 3600) / 60);
    const s = totalSec % 60;
    if (h > 0) return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
    return `${m}:${s.toString().padStart(2, '0')}`;
  };

  const progressPct = duration > 0 ? (currentTime / duration) * 100 : 0;

  if (error) {
    return (
      <view className="ErrorContainer">
        <view className="PlayButtonCircle" bindtap={onPlayPause}>
          <text className="PlayIconLarge">🔄</text>
        </view>
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
      </view>
    );
  }

  if (state === 'idle') {
    return (
      <view className="CenterArea">
        <view className="PlayButtonCircle" bindtap={onPlayPause}>
          <text className="PlayIconLarge">▶</text>
        </view>
        <text className="IdleTitle">{fileName || "等待文件信息..."}</text>
      </view>
    );
  }

  const isPlaying = state === 'playing';

  return (
    <view style={{ flex: 1, flexDirection: 'column', justifyContent: 'space-between' }}>
      {/* Top Bar */}
      <view className="TopBar">
        <text className="BackButton" bindtap={onBack}>✕</text>
        <text className="TitleText">{fileName}</text>
        <text className="FullscreenButton" bindtap={onToggleFullscreen}>
          {isFullscreen ? '⤓' : '⤢'}
        </text>
      </view>

      {/* Center Area - Play/Pause Button */}
      <view className="CenterArea">
        {showControls && (
          <view className="PlayButtonCircle" bindtap={onPlayPause}>
            <text className="PlayIconLarge">{isPlaying ? '⏸' : '▶'}</text>
          </view>
        )}
      </view>

      {/* Bottom Bar */}
      <view className="BottomBar">
        <text className="TimeLabel">{formatTime(currentTime)}</text>

        <view
          className="SliderContainer"
          bindtap={(e: any) => {
            const rect: any = e.detail || {};
            if (typeof rect.x === 'number') {
              const pct = Math.max(0, Math.min(1, rect.x / 300));
              onSeek(pct * duration);
            }
          }}
        >
          <view className="SliderTrackOuter">
            <view className="SliderBuffered" style={{ width: progressPct + '%' }}>
              <view className="SliderFill" style={{ width: '100%' }} />
            </view>
          </view>
        </view>

        <text className="TimeLabelEnd">{formatTime(duration)}</text>
      </view>
    </view>
  );
}
