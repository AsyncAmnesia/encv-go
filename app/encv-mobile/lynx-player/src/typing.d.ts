declare let NativeModules: {
  GoBackendModule: {
    getBackendStatus(callback: (result: { running: boolean; port: number }) => void): void;
    startBackend(callback: (result: any) => void): void;
    getStreamUrl(path: string, isExternal: boolean, callback: (url: string) => void): void;
    closePlayer(callback: (result: any) => void): void;
  };
  MpvPlayerModule: {
    play(url: string, callback: (result: any) => void): void;
    pause(callback: (result: any) => void): void;
    resume(callback: (result: any) => void): void;
    seekTo(positionMs: number, callback: (result: any) => void): void;
    setFullscreen(enabled: boolean, callback: (result: any) => void): void;
    setProperty(key: string, value: string, callback: (result: any) => void): void;
  };
  LogBridge: {
    log(level: string, msg: string, callback: (result: any) => void): void;
  };
  PlayerBridgeModule: {
    playFile: (filePath: string, fileName: string, mimeType: string) => Promise<boolean>;
    playFileExternal: (filePath: string, fileName: string, mimeType: string) => Promise<boolean>;
    isMpvAvailable: () => Promise<boolean>;
    getExtensionStatus: () => Promise<{ mpvPlayer: { installed: boolean; enabled: boolean; version: string } }>;
    pickAndInstallPlugin: (options: { mimeType?: string; title?: string }) => Promise<{ success: boolean; error?: string }>;
    uninstallPlugin: (options: { pluginId: string }) => Promise<{ success: boolean; error?: string }>;
  };
};
