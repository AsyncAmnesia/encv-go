import { useState, useEffect, useCallback } from "@lynx-js/react";
import { PlayerControls } from "./PlayerControls";
import { PlayerSettings } from "./PlayerSettings";

interface VideoPlayerProps {
  streamUrl: string;
  fileName: string;
}

export function VideoPlayer(props: VideoPlayerProps) {
  const { streamUrl, fileName } = props;
  const [playing, setPlaying] = useState(false);
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [fullscreen, setFullscreen] = useState(false);
  const [showSettings, setShowSettings] = useState(false);

  const startPlayback = useCallback(() => {
    NativeModules.MpvPlayerModule.play(streamUrl);
    setPlaying(true);
  }, [streamUrl]);

  useEffect(() => {
    startPlayback();
  }, [startPlayback]);

  useEffect(() => {
    const interval = setInterval(() => {
      NativeModules.MpvPlayerModule.getCurrentPosition((pos) => {
        setPosition(pos);
      });
      NativeModules.MpvPlayerModule.getDuration((dur) => {
        setDuration(dur);
      });
      NativeModules.MpvPlayerModule.isPlaying((p) => {
        setPlaying(p);
      });
    }, 500);
    return () => clearInterval(interval);
  }, []);

  const handlePlayPause = useCallback(() => {
    if (playing) {
      NativeModules.MpvPlayerModule.pause();
      setPlaying(false);
    } else {
      NativeModules.MpvPlayerModule.resume();
      setPlaying(true);
    }
  }, [playing]);

  const handleSeek = useCallback(
    (positionMs: number) => {
      NativeModules.MpvPlayerModule.seekTo(positionMs);
      setPosition(positionMs);
    },
    []
  );

  const handleFullscreen = useCallback(() => {
    const next = !fullscreen;
    NativeModules.MpvPlayerModule.setFullscreen(next);
    setFullscreen(next);
  }, [fullscreen]);

  const handleSettingsClose = useCallback(() => {
    setShowSettings(false);
  }, []);

  return (
    <view style={styles.container}>
      <view style={styles.surfaceContainer} />
      <PlayerControls
        playing={playing}
        position={position}
        duration={duration}
        fileName={fileName}
        onPlayPause={handlePlayPause}
        onSeek={handleSeek}
        onFullscreen={handleFullscreen}
        onSettings={() => setShowSettings(true)}
      />
      {showSettings && <PlayerSettings onClose={handleSettingsClose} />}
    </view>
  );
}

const styles = {
  container: {
    flex: 1,
    position: "relative",
  } as const,
  surfaceContainer: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
  } as const,
};
