import { useCallback, useState, useInitData, useLynxGlobalEventListener, useRef } from "@lynx-js/react";
import { HomePage } from "./components/HomePage";
import { PlayerPage } from "./components/PlayerPage";
import { PlaylistPage } from "./components/PlaylistPage";
import { SettingsPage } from "./components/SettingsPage";
import "../App.css";

interface InitData {
  filePath: string;
  fileName: string;
  mimeType: string;
  isExternal: boolean;
  mediaType: "video" | "audio";
}

type AppView = "home" | "player" | "playlist" | "settings";

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

export interface PlaybackItem {
  filePath: string;
  fileName: string;
  mimeType: string;
  isExternal: boolean;
  mediaType: "video" | "audio";
}

export function AppComponent() {
  const initData = useInitData<InitData>();
  const [currentView, setCurrentView] = useState<AppView>("home");
  const [playbackItem, setPlaybackItem] = useState<PlaybackItem | null>(null);
  const [playlist, setPlaylist] = useState<PlaybackItem[]>([]);
  const [playlistIndex, setPlaylistIndex] = useState(-1);

  const navigateTo = useCallback((view: AppView) => {
    setCurrentView(view);
  }, []);

  const playItem = useCallback((item: PlaybackItem) => {
    setPlaybackItem(item);
    setCurrentView("player");
  }, []);

  const playFromPlaylist = useCallback((items: PlaybackItem[], index: number) => {
    setPlaylist(items);
    setPlaylistIndex(index);
    if (items[index]) {
      setPlaybackItem(items[index]);
    }
    setCurrentView("player");
  }, []);

  const handleNextTrack = useCallback(() => {
    if (playlist.length === 0 || playlistIndex >= playlist.length - 1) return;
    const nextIdx = playlistIndex + 1;
    setPlaylistIndex(nextIdx);
    setPlaybackItem(playlist[nextIdx]);
  }, [playlist, playlistIndex]);

  const handlePrevTrack = useCallback(() => {
    if (playlist.length === 0 || playlistIndex <= 0) return;
    const prevIdx = playlistIndex - 1;
    setPlaylistIndex(prevIdx);
    setPlaybackItem(playlist[prevIdx]);
  }, [playlist, playlistIndex]);

  useLynxGlobalEventListener("backend:ready", (event: any) => {
    lynxLog.info("Backend ready, port=" + String(event?.detail?.port ?? event?.port));
  });

  const handleInitPlayback = useCallback(() => {
    if (!initData) return;
    const item: PlaybackItem = {
      filePath: initData.filePath,
      fileName: initData.fileName || "Unknown",
      mimeType: initData.mimeType,
      isExternal: initData.isExternal,
      mediaType: initData.mediaType || "video",
    };
    setPlaybackItem(item);
    setCurrentView("player");
  }, [initData]);

  if (currentView === "player") {
    return (
      <page style={{ width: "100%", height: "100%" }}>
        <PlayerPage
          item={playbackItem}
          playlist={playlist}
          playlistIndex={playlistIndex}
          onBack={() => setCurrentView("home")}
          onNext={handleNextTrack}
          onPrev={handlePrevTrack}
          onPlaylist={() => setCurrentView("playlist")}
          onSettings={() => setCurrentView("settings")}
        />
      </page>
    );
  }

  if (currentView === "playlist") {
    return (
      <page style={{ width: "100%", height: "100%" }}>
        <PlaylistPage
          playlist={playlist}
          currentIndex={playlistIndex}
          onSelect={(index: number) => {
            setPlaylistIndex(index);
            setPlaybackItem(playlist[index]);
            setCurrentView("player");
          }}
          onBack={() => setCurrentView("player")}
        />
      </page>
    );
  }

  if (currentView === "settings") {
    return (
      <page style={{ width: "100%", height: "100%" }}>
        <SettingsPage
          onBack={() => setCurrentView("home")}
        />
      </page>
    );
  }

  return (
    <page style={{ width: "100%", height: "100%" }}>
      <HomePage
        onPlayItem={playItem}
        onPlayFromList={playFromPlaylist}
        onInitPlayback={handleInitPlayback}
        hasInitData={!!initData?.filePath}
        onSettings={() => setCurrentView("settings")}
      />
    </page>
  );
}
