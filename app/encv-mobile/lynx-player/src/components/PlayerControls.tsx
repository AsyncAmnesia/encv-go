import { useState, useEffect, useCallback, useRef } from "@lynx-js/react";

const ACCENT = "#ffad00";

interface PlayerControlsProps {
  playing: boolean;
  position: number;
  duration: number;
  fileName: string;
  onPlayPause: () => void;
  onSeek: (positionMs: number) => void;
  onFullscreen: () => void;
  onSettings: () => void;
}

function formatTime(ms: number): string {
  if (!ms || ms < 0) return "00:00";
  const totalSec = Math.floor(ms / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (h > 0) {
    return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  }
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

export function PlayerControls(props: PlayerControlsProps) {
  const {
    playing,
    position,
    duration,
    fileName,
    onPlayPause,
    onSeek,
    onFullscreen,
    onSettings,
  } = props;

  const [visible, setVisible] = useState(true);
  const hideTimer = useRef<any>(null);

  const resetHideTimer = useCallback(() => {
    if (hideTimer.current) {
      clearTimeout(hideTimer.current);
    }
    hideTimer.current = setTimeout(() => {
      setVisible(false);
    }, 3000);
  }, []);

  const handleTap = useCallback(() => {
    setVisible((prev) => {
      const next = !prev;
      if (next) {
        resetHideTimer();
      }
      return next;
    });
  }, [resetHideTimer]);

  useEffect(() => {
    resetHideTimer();
    return () => {
      if (hideTimer.current) {
        clearTimeout(hideTimer.current);
      }
    };
  }, [resetHideTimer]);

  const handleInteraction = useCallback(() => {
    setVisible(true);
    resetHideTimer();
  }, [resetHideTimer]);

  const progress = duration > 0 ? position / duration : 0;

  const handleProgressTap = useCallback(
    (e: any) => {
      const ratio = e.detail?.x ? e.detail.x / 300 : 0;
      const clamped = Math.max(0, Math.min(1, ratio));
      const seekPos = clamped * duration;
      onSeek(seekPos);
      handleInteraction();
    },
    [duration, onSeek, handleInteraction]
  );

  if (!visible) {
    return (
      <view style={styles.overlay} bindtap={handleTap}>
        <view style={styles.tapArea} />
      </view>
    );
  }

  return (
    <view style={styles.overlay} bindtap={handleTap}>
      <view style={styles.topBar}>
        <text style={styles.title} numberOfLines={1}>
          {fileName}
        </text>
      </view>

      <view style={styles.centerArea}>
        <view style={styles.playButton} bindtap={onPlayPause}>
          <text style={styles.playIcon}>{playing ? "⏸" : "▶"}</text>
        </view>
      </view>

      <view style={styles.bottomBar}>
        <text style={styles.timeText}>{formatTime(position)}</text>

        <view style={styles.progressContainer} bindtap={handleProgressTap}>
          <view style={styles.progressTrack}>
            <view
              style={[
                styles.progressFill,
                { width: Math.max(0, progress * 100) + "%" },
              ]}
            />
          </view>
        </view>

        <text style={styles.timeText}>{formatTime(duration)}</text>

        <view style={styles.actionButtons}>
          <view style={styles.iconButton} bindtap={onSettings}>
            <text style={styles.iconText}>⚙</text>
          </view>
          <view style={styles.iconButton} bindtap={onFullscreen}>
            <text style={styles.iconText}>⛶</text>
          </view>
        </view>
      </view>
    </view>
  );
}

const styles = {
  overlay: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: "space-between",
  } as const,
  tapArea: {
    flex: 1,
  } as const,
  topBar: {
    flexDirection: "row",
    alignItems: "center",
    paddingHorizontal: 12,
    paddingVertical: 8,
    backgroundColor: "rgba(0,0,0,0.6)",
  } as const,
  title: {
    color: "#ffffff",
    fontSize: 14,
    flex: 1,
  } as const,
  centerArea: {
    flexDirection: "row",
    justifyContent: "center",
    alignItems: "center",
  } as const,
  playButton: {
    width: 56,
    height: 56,
    borderRadius: 28,
    backgroundColor: ACCENT,
    justifyContent: "center",
    alignItems: "center",
  } as const,
  playIcon: {
    color: "#000000",
    fontSize: 24,
  } as const,
  bottomBar: {
    flexDirection: "row",
    alignItems: "center",
    paddingHorizontal: 12,
    paddingVertical: 8,
    backgroundColor: "rgba(0,0,0,0.6)",
  } as const,
  timeText: {
    color: "#ffffff",
    fontSize: 12,
    minWidth: 48,
  } as const,
  progressContainer: {
    flex: 1,
    height: 20,
    justifyContent: "center",
    paddingHorizontal: 4,
  } as const,
  progressTrack: {
    height: 4,
    backgroundColor: "rgba(255,255,255,0.3)",
    borderRadius: 2,
  } as const,
  progressFill: {
    height: 4,
    backgroundColor: ACCENT,
    borderRadius: 2,
  } as const,
  actionButtons: {
    flexDirection: "row",
    alignItems: "center",
  } as const,
  iconButton: {
    width: 36,
    height: 36,
    justifyContent: "center",
    alignItems: "center",
  } as const,
  iconText: {
    color: "#ffffff",
    fontSize: 18,
  } as const,
};
