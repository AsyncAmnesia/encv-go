import { useCallback, useMemo } from "@lynx-js/react";

interface PlayerControlsProps {
  fileName: string;
  playerState: string;
  position: number;
  duration: number;
  errorMessage: string;
  isFullscreen: boolean;
  onPlayPause: () => void;
  onSeek: (positionMs: number) => void;
  onFullscreen: () => void;
}

function formatTime(ms: number): string {
  if (!ms || ms <= 0) return "0:00";
  const totalSeconds = Math.floor(ms / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
  }
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
}

export function PlayerControls({
  fileName,
  playerState,
  position,
  duration,
  errorMessage,
  isFullscreen,
  onPlayPause,
  onSeek,
  onFullscreen,
}: PlayerControlsProps) {
  const progress = useMemo(() => {
    if (!duration || duration <= 0) return 0;
    return position / duration;
  }, [position, duration]);

  const handleSliderTap = useCallback(
    (event: any) => {
      if (!duration || duration <= 0) return;
      const ratio = event.detail?.x / (event.detail?.width || 1);
      const seekPosition = Math.floor(ratio * duration);
      onSeek(seekPosition);
    },
    [duration, onSeek]
  );

  const playPauseIcon = playerState === "playing" ? "⏸" : "▶";
  const fullscreenIcon = isFullscreen ? "⤓" : "⤢";

  if (playerState === "error") {
    return (
      <view className="ErrorOverlay">
        <text className="ErrorTitle">Playback Error</text>
        <text className="ErrorMessage">{errorMessage || "Unknown error occurred"}</text>
      </view>
    );
  }

  if (playerState === "loading") {
    return (
      <view className="LoadingIndicator">
        <text className="LoadingText">Loading...</text>
      </view>
    );
  }

  return (
    <view className="ControlsOverlay">
      <view className="TopBar">
        <text className="FileName" text-maxline="1">
          {fileName}
        </text>
        <text className="FullscreenButton" bindtap={onFullscreen}>
          {fullscreenIcon}
        </text>
      </view>

      <view className="CenterControls">
        <text className="PlayButton" bindtap={onPlayPause}>
          {playPauseIcon}
        </text>
      </view>

      <view className="ProgressBar">
        <text className="TimeLabel">{formatTime(position)}</text>
        <view className="SliderTrack" bindtap={handleSliderTap}>
          <view
            className="SliderFill"
            style={{ width: `${progress * 100}%` }}
          />
        </view>
        <text className="TimeLabel">{formatTime(duration)}</text>
      </view>
    </view>
  );
}
