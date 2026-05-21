declare namespace NativeModules {
  const MpvPlayerModule: {
    play(params: { url: string }): Promise<boolean>;
    pause(params: {}): Promise<boolean>;
    resume(params: {}): Promise<boolean>;
    seekTo(params: { positionMs: number }): Promise<boolean>;
    setFullscreen(params: { enabled: boolean }): Promise<boolean>;
    setOrientation(params: { orientation: string }): Promise<boolean>;
    getDuration(params: {}): Promise<number>;
    getCurrentPosition(params: {}): Promise<number>;
    isPlaying(params: {}): Promise<boolean>;
    setProperty(params: { key: string; value: string }): Promise<boolean>;
  };
  const GoBackendModule: {
    getBackendStatus(params: {}): Promise<{ running: boolean; port: number }>;
    startBackend(params: {}): Promise<boolean>;
    getStreamUrl(params: { path: string; isExternal: boolean }): Promise<string>;
  };
}

interface LynxInitData {
  filePath: string;
  fileName: string;
  mimeType: string;
  isExternal: boolean;
}

declare global {
  interface Window {
    __lynx_view_global?: {
      lynxCoreInject?: {
        customSection?: {
          initData?: LynxInitData;
        };
      };
    };
  }
}

export {};
