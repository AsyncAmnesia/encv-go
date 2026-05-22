declare let NativeModules: {
  MpvPlayerModule: {
    play(url: string, callback: (result: any) => void): void;
    pause(callback: (result: any) => void): void;
    resume(callback: (result: any) => void): void;
    seekTo(positionMs: number, callback: (result: any) => void): void;
    setFullscreen(enabled: boolean, callback: (result: any) => void): void;
    setOrientation(orientation: string, callback: (result: any) => void): void;
    finish(callback: (result: any) => void): void;
    getDuration(callback: (durationMs: number) => void): void;
    getCurrentPosition(callback: (positionMs: number) => void): void;
    isPlaying(callback: (playing: boolean) => void): void;
    setProperty(key: string, value: string, callback: (result: any) => void): void;
  };
  GoBackendModule: {
    getBackendStatus(callback: (result: { running: boolean; port: number }) => void): void;
    startBackend(callback: (result: any) => void): void;
    getStreamUrl(path: string, isExternal: boolean, callback: (url: string) => void): void;
  };
  LogBridgeModule: {
    log(level: string, msg: string, callback: (result: any) => void): void;
  };
};
