import { useCallback, useEffect, useState, useLynxGlobalEventListener, useRef } from "@lynx-js/react";
import { PlayerControls } from "./PlayerControls";
import type { PlaybackItem } from "./AppComponent";

type PlayerState = "idle" | "loading" | "playing" | "paused" | "ended" | "error";

const SPEED_OPTIONS = [0.5, 0.75, 1, 1.25, 1.5, 2];

const lynxLog = {
  info: (msg: string) => {
    try {
      console.info(msg);
      NativeModules.LogBridge.log("info", msg, () => {});
    } catch (_e) {
      console.info(msg);
    }
  },
  error: (msg: string) => {
    try {
      console.error(msg);
      NativeModules.LogBridge.log("error", msg, () => {});
    } catch (_e) {
      console.error(msg);
    }
  },
};

interface PlayerPageProps {
  item: PlaybackItem | null;
  playlist: PlaybackItem[];
  playlistIndex: number;
  onBack: () => void;
  onNext: () => void;
  onPrev: () => void;
  onPlaylist: () => void;
  onSettings: () => void;
}

export function PlayerPage({
  item,
  playlist,
  playlistIndex,
  onBack,
  onNext,
  onPrev,
  onPlaylist,
  onSettings,
}: PlayerPageProps) {
  const [playerState, setPlayerState] = useState<PlayerState>("idle");
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [fileName, setFileName] = useState("");
  const [errorMessage, setErrorMessage] = useState("");
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showControls, setShowControls] = useState(true);
  const [mediaType, setMediaType] = useState<"video" | "audio">("video");
  const [playbackRate, setPlaybackRate] = useState(1);
  const [locked, setLocked] = useState(false);
  const hideTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const resetHideTimer = useCallback(() => {
    if (hideTimerRef.current) {
      clearTimeout(hideTimerRef.current);
    }
    setShowControls(true);
    if (playerState === "playing") {
      hideTimerRef.current = setTimeout(() => {
        setShowControls(false);
      }, 5000);
    }
  }, [playerState]);

  useLynxGlobalEventListener("mpv:state-change", (event: any) => {
    const state = event?.state;
    const error = event?.error;
    lynxLog.info("mpv:state-change " + JSON.stringify(event));
    if (state) {
      if (state === "surface_ready") {
        lynxLog.info("MPV surface ready, native will auto-play pending URL");
        setErrorMessage("");
        return;
      }
      if (state === "waiting_surface") {
        setPlayerState("loading");
        return;
      }
      if (state === "mpv_ready") {
        lynxLog.info("MPV engine ready");
        return;
      }
      if (state === "audio_only") {
        setMediaType("audio");
        setErrorMessage("");
        setPlayerState(state as PlayerState);
        return;
      }
      setPlayerState(state as PlayerState);
    }
    if (error) setErrorMessage(error);
    if (state === "playing" || state === "paused") {
      setErrorMessage("");
      setShowControls(true);
      resetHideTimer();
    }
  });

  useLynxGlobalEventListener("mpv:position-update", (event: any) => {
    const pos = event?.position ?? 0;
    const dur = event?.duration ?? 0;
    setPosition(pos);
    setDuration(dur);
  });

  useEffect(() => {
    if (item) {
      setFileName(item.fileName || "Unknown");
      if (item.mediaType) {
        setMediaType(item.mediaType);
      }
      startPlayback(item);
    }
  }, [item]);

  useEffect(() => {
    resetHideTimer();
  }, [playerState, resetHideTimer]);

  const startPlayback = useCallback(async (data: PlaybackItem | null) => {
    lynxLog.info("startPlayback called, data=" + JSON.stringify(data));
    if (!data || !data.filePath) {
      lynxLog.error("startPlayback: filePath is empty!");
      setPlayerState("error");
      setErrorMessage(data ? "文件路径为空" : "未收到播放数据");
      return;
    }
    setPlayerState("loading");
    setErrorMessage("");
    try {
      lynxLog.info("startPlayback: step1 getBackendStatus");
      const status = await new Promise<any>((resolve) => {
        NativeModules.GoBackendModule.getBackendStatus(resolve);
      });
      lynxLog.info("startPlayback: step1 result=" + JSON.stringify(status));

      if (data.isExternal || !status.running) {
        lynxLog.info("startPlayback: step2 startBackend");
        await new Promise<any>((resolve) => {
          NativeModules.GoBackendModule.startBackend(resolve);
        });
        lynxLog.info("startPlayback: step2 done");
      }

      lynxLog.info("startPlayback: step3 getStreamUrl path=" + data.filePath);
      const streamUrl = await new Promise<string>((resolve) => {
        NativeModules.GoBackendModule.getStreamUrl(data.filePath, data.isExternal, resolve);
      });
      lynxLog.info("startPlayback: step3 url=" + streamUrl);

      lynxLog.info("startPlayback: step4 mpv.play url=" + streamUrl);
      await new Promise<any>((resolve) => {
        NativeModules.MpvPlayerModule.play(streamUrl, resolve);
      });
      lynxLog.info("startPlayback: all steps done, playing");
    } catch (e: any) {
      lynxLog.error("startPlayback caught: " + (e?.message || String(e)));
      setPlayerState("error");
      setErrorMessage(e?.message || String(e));
    }
  }, []);

  const handlePlayPause = useCallback(() => {
    if (playerState === "playing") {
      NativeModules.MpvPlayerModule.pause(() => {});
      setPlayerState("paused");
    } else if (playerState === "paused") {
      NativeModules.MpvPlayerModule.resume(() => {});
      setPlayerState("playing");
    } else {
      setErrorMessage("");
      startPlayback(item);
    }
    resetHideTimer();
  }, [playerState, item, startPlayback, resetHideTimer]);

  const handleSeek = useCallback(
    (positionMs: number) => {
      NativeModules.MpvPlayerModule.seekTo(positionMs, () => {});
      setPosition(positionMs);
      resetHideTimer();
    },
    [resetHideTimer]
  );

  const handleSeekRelative = useCallback(
    (deltaMs: number) => {
      const newPos = Math.max(0, Math.min(position + deltaMs, duration));
      NativeModules.MpvPlayerModule.seekTo(newPos, () => {});
      setPosition(newPos);
      resetHideTimer();
    },
    [position, duration, resetHideTimer]
  );

  const handleFullscreen = useCallback(() => {
    const next = !isFullscreen;
    setIsFullscreen(next);
    NativeModules.MpvPlayerModule.setFullscreen(next, () => {});
    resetHideTimer();
  }, [isFullscreen, resetHideTimer]);

  const handleCycleSpeed = useCallback(() => {
    const currentIdx = SPEED_OPTIONS.indexOf(playbackRate);
    const nextIdx = (currentIdx + 1) % SPEED_OPTIONS.length;
    const nextRate = SPEED_OPTIONS[nextIdx];
    setPlaybackRate(nextRate);
    NativeModules.MpvPlayerModule.setProperty("speed", String(nextRate), () => {});
    resetHideTimer();
  }, [playbackRate, resetHideTimer]);

  const handleToggleLock = useCallback(() => {
    setLocked((prev) => !prev);
    resetHideTimer();
  }, [resetHideTimer]);

  const handleToggleControls = useCallback(() => {
    if (locked) {
      setLocked(false);
      return;
    }
    setShowControls((prev) => !prev);
    resetHideTimer();
  }, [locked, resetHideTimer]);

  const handleBackToHome = useCallback(() => {
    NativeModules.MpvPlayerModule.pause(() => {});
    onBack();
  }, [onBack]);

  const hasPlaylist = playlist.length > 1;
  const hasNext = hasPlaylist && playlistIndex < playlist.length - 1;
  const hasPrev = hasPlaylist && playlistIndex > 0;

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
        hasNext={hasNext}
        hasPrev={hasPrev}
        onPlayPause={handlePlayPause}
        onSeek={handleSeek}
        onSeekRelative={handleSeekRelative}
        onToggleFullscreen={handleFullscreen}
        onCycleSpeed={handleCycleSpeed}
        onToggleLock={handleToggleLock}
        onBack={handleBackToHome}
        onNext={onNext}
        onPrev={onPrev}
        onPlaylist={onPlaylist}
      />
    </view>
  );
}
