import { useCallback, useEffect, useState, useInitData, useLynxGlobalEventListener } from "@lynx-js/react";
import { PlayerControls } from "./components/PlayerControls";
import "./App.css";

interface InitData {
  filePath: string;
  fileName: string;
  mimeType: string;
  isExternal: boolean;
}

type PlayerState = "idle" | "loading" | "playing" | "paused" | "ended" | "error";

export function App() {
  const initData = useInitData<InitData>();
  const [playerState, setPlayerState] = useState<PlayerState>("idle");
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [fileName, setFileName] = useState("");
  const [errorMessage, setErrorMessage] = useState("");
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showControls, setShowControls] = useState(true);

  useLynxGlobalEventListener("mpv:state-change", (event: any) => {
    const state = event?.state;
    const error = event?.error;
    console.info("mpv:state-change", JSON.stringify(event));
    if (state) setPlayerState(state as PlayerState);
    if (error) setErrorMessage(error);
    if (state === "playing" || state === "paused") {
      setShowControls(true);
    }
  });

  useLynxGlobalEventListener("mpv:position-update", (event: any) => {
    const pos = event?.position ?? 0;
    const dur = event?.duration ?? 0;
    setPosition(pos);
    setDuration(dur);
  });

  useLynxGlobalEventListener("backend:ready", (event: any) => {
    console.info("Backend ready, port:", event?.detail?.port ?? event?.port);
  });

  useEffect(() => {
    if (initData) {
      setFileName(initData.fileName || "Unknown");
    }
  }, [initData]);

  const startPlayback = useCallback(async (data: InitData | undefined) => {
    if (!data) return;
    setPlayerState("loading");
    try {
      const status = await new Promise<any>((resolve) => {
        NativeModules.GoBackendModule.getBackendStatus(resolve);
      });
      if (data.isExternal || !status.running) {
        await new Promise<any>((resolve) => {
          NativeModules.GoBackendModule.startBackend(resolve);
        });
      }
      const streamUrl = await new Promise<string>((resolve) => {
        NativeModules.GoBackendModule.getStreamUrl(data.filePath, data.isExternal, resolve);
      });
      await new Promise<any>((resolve) => {
        NativeModules.MpvPlayerModule.play(streamUrl, resolve);
      });
    } catch (e: any) {
      console.info("startPlayback error:", e?.message || String(e));
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
      startPlayback(initData);
    }
  }, [playerState, initData, startPlayback]);

  const handleSeek = useCallback(
    (positionMs: number) => {
      NativeModules.MpvPlayerModule.seekTo(positionMs, () => {});
      setPosition(positionMs);
    },
    []
  );

  const handleFullscreen = useCallback(() => {
    const next = !isFullscreen;
    setIsFullscreen(next);
    NativeModules.MpvPlayerModule.setFullscreen(next, () => {});
  }, [isFullscreen]);

  const handleToggleControls = useCallback(() => {
    setShowControls((prev) => !prev);
  }, [showControls]);

  const isOverlay = playerState === "loading" || playerState === "error" || playerState === "idle";

  return (
    <page style={{ flex: 1 }}>
      <view
        className="PlayerContainer"
        bindtap={handleToggleControls}
        style={isOverlay ? { justifyContent: "center" } : undefined}
      >
        {showControls && (
          <PlayerControls
            fileName={fileName}
            playerState={playerState}
            position={position}
            duration={duration}
            errorMessage={errorMessage}
            isFullscreen={isFullscreen}
            onPlayPause={handlePlayPause}
            onSeek={handleSeek}
            onFullscreen={handleFullscreen}
          />
        )}
      </view>
    </page>
  );
}
