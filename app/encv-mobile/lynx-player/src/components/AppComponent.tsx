import { useCallback, useEffect, useState, useInitData, useLynxGlobalEventListener } from "@lynx-js/react";
import { PlayerControls } from "./PlayerControls";
import "../App.css";

interface InitData {
  filePath: string;
  fileName: string;
  mimeType: string;
  isExternal: boolean;
}

type PlayerState = "idle" | "loading" | "playing" | "paused" | "ended" | "error";

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

export function AppComponent() {
  const initData = useInitData<InitData>();
  const [playerState, setPlayerState] = useState<PlayerState>("idle");
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [fileName, setFileName] = useState("");
  const [errorMessage, setErrorMessage] = useState("");
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showControls, setShowControls] = useState(true);

  lynxLog.info("AppComponent: rendering, initData=" + JSON.stringify(initData));

  useLynxGlobalEventListener("mpv:state-change", (event: any) => {
    const state = event?.state;
    const error = event?.error;
    lynxLog.info("mpv:state-change " + JSON.stringify(event));
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
    lynxLog.info("Backend ready, port=" + String(event?.detail?.port ?? event?.port));
  });

  useEffect(() => {
    if (initData) {
      setFileName(initData.fileName || "Unknown");
      lynxLog.info("fileName set to: " + (initData.fileName || "Unknown"));
    }
  }, [initData]);

  const startPlayback = useCallback(async (data: InitData | undefined) => {
    if (!data) {
      lynxLog.error("startPlayback: no data");
      return;
    }
    setPlayerState("loading");
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
      lynxLog.info("handlePlayPause: pause");
      NativeModules.MpvPlayerModule.pause(() => {});
      setPlayerState("paused");
    } else if (playerState === "paused") {
      lynxLog.info("handlePlayPause: resume");
      NativeModules.MpvPlayerModule.resume(() => {});
      setPlayerState("playing");
    } else {
      lynxLog.info("handlePlayPause: startPlayback");
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

  const handleBack = useCallback(() => {
    lynxLog.info("handleBack: finishing player activity");
    NativeModules.MpvPlayerModule.finish(() => {});
  }, []);

  return (
    <page style={{ width: "100%", height: "100%" }}>
      <view
        className="PlayerContainer"
        bindtap={handleToggleControls}
      >
        <PlayerControls
          state={playerState}
          isFullscreen={isFullscreen}
          fileName={fileName}
          currentTime={position}
          duration={duration}
          showControls={showControls}
          error={errorMessage || undefined}
          onPlayPause={handlePlayPause}
          onSeek={handleSeek}
          onToggleFullscreen={handleFullscreen}
          onBack={handleBack}
        />
      </view>
    </page>
  );
}
