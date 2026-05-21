declare namespace NodeJS {
  interface Global {
    __lynx_view_global: {
      lynxCoreInject: {
        customSection: {
          initData: string;
        };
      };
    };
  }
}

interface MpvPlayerModule {
  play(url: string): void;
  pause(): void;
  resume(): void;
  seekTo(positionMs: number): void;
  setFullscreen(enabled: boolean): void;
  setOrientation(orientation: string): void;
  getDuration(callback: (durationMs: number) => void): void;
  getCurrentPosition(callback: (positionMs: number) => void): void;
  isPlaying(callback: (playing: boolean) => void): void;
  setProperty(key: string, value: string): void;
}

interface GoBackendModule {
  getBackendStatus(callback: (status: string) => void): void;
  startBackend(): void;
  getStreamUrl(
    path: string,
    isExternal: boolean,
    callback: (url: string) => void
  ): void;
}

declare namespace NativeModules {
  const MpvPlayerModule: MpvPlayerModule;
  const GoBackendModule: GoBackendModule;
}

interface InitData {
  filePath: string;
  fileName: string;
  mimeType: string;
}
