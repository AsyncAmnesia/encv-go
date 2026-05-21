import { useCallback, useEffect, useState } from "@lynx-js/react";
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
  const [playerState, setPlayerState] = useState<PlayerState>("idle");
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [fileName, setFileName] = useState("");
  const [errorMessage, setErrorMessage] = useState("");
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showControls, setShowControls] = useState(true);

  useEffect(() => {
    const initDataStr = globalThis.__lynx_view_global?.lynxCoreInject?.customSection?.initData;
    if (initDataStr) {
      let initData: InitData;
      if (typeof initDataStr === "string") {
        initData = JSON.parse(initDataStr);
      } else {
        initData = initDataStr as unknown as InitData;
      }
      setFileName(initData.fileName || "Unknown");
      startPlayback(initData);
    }

    const onStateChange = (event: { detail: { state: string; error?: string } }) => {
      const state = event.detail.state;
      const error = event.detail.error;
      setPlayerState(state as PlayerState);
      if (error) setErrorMessage(error);
      if (state === "playing" || state === "paused") {
        setShowControls(true);
      }
    };

    const onPositionUpdate = (event: { detail: { position: number; duration: number } }) => {
      setPosition(event.detail.position);
      setDuration(event.detail.duration);
    };

    const onBackendReady = (event: { detail: { port: number } }) => {
      console.info("Backend ready, port:", event.detail.port);
    };

    globalThis.addEventListener("mpv:state-change", onStateChange as EventListener);
    globalThis.addEventListener("mpv:position-update", onPositionUpdate as EventListener);
    globalThis.addEventListener("backend:ready", onBackendReady as EventListener);

    return () => {
      globalThis.removeEventListener("mpv:state-change", onStateChange as EventListener);
      globalThis.removeEventListener("mpv:position-update", onPositionUpdate as EventListener);
      globalThis.removeEventListener("backend:ready", onBackendReady as EventListener);
    };
  }, []);

  const startPlayback = useCallback(async (initData: InitData) => {
    setPlayerState("loading");
    try {
      const status = await NativeModules.GoBackendModule.getBackendStatus({});
      let streamUrl: string;
      if (initData.isExternal || !status.running) {
        await NativeModules.GoBackendModule.startBackend({});
        streamUrl = await NativeModules.GoBackendModule.getStreamUrl({
          path: initData.filePath,
          isExternal: initData.isExternal,
        });
      } else {
        streamUrl = await NativeModules.GoBackendModule.getStreamUrl({
          path: initData.filePath,
          isExternal: initData.isExternal,
        });
      }
      await NativeModules.MpvPlayerModule.play({ url: streamUrl });
    } catch (e: any) {
      setPlayerState("error");
      setErrorMessage(e?.message || String(e));
    }
  }, []);

  const handlePlayPause = useCallback(() => {
    if (playerState === "playing") {
      NativeModules.MpvPlayerModule.pause({});
      setPlayerState("paused");
    } else if (playerState === "paused") {
      NativeModules.MpvPlayerModule.resume({});
      setPlayerState("playing");
    }
  }, [playerState]);

  const handleSeek = useCallback(
    (positionMs: number) => {
      NativeModules.MpvPlayerModule.seekTo({ positionMs });
      setPosition(positionMs);
    },
    []
  );

  const handleFullscreen = useCallback(() => {
    const next = !isFullscreen;
    setIsFullscreen(next);
    NativeModules.MpvPlayerModule.setFullscreen({ enabled: next });
  }, [isFullscreen]);

  const handleToggleControls = useCallback(() => {
    setShowControls((prev) => !prev);
  }, [showControls]);

  return (
    <page>
      <view className="PlayerContainer" bindtap={handleToggleControls}>
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
