import { useState, useEffect, useCallback } from "@lynx-js/react";
import { VideoPlayer } from "./components/VideoPlayer";

const ACCENT = "#ffad00";

export default function App() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [streamUrl, setStreamUrl] = useState("");
  const [fileName, setFileName] = useState("");

  const parseInitData = useCallback((): InitData | null => {
    try {
      const raw = global.__lynx_view_global.lynxCoreInject.customSection.initData;
      if (!raw) return null;
      return JSON.parse(raw) as InitData;
    } catch {
      return null;
    }
  }, []);

  const ensureBackend = useCallback((): Promise<string> => {
    return new Promise((resolve, reject) => {
      NativeModules.GoBackendModule.getBackendStatus((status) => {
        if (status === "running") {
          resolve(status);
          return;
        }
        NativeModules.GoBackendModule.startBackend();
        const timeout = setTimeout(() => {
          reject(new Error("Backend start timeout"));
        }, 15000);
        const check = () => {
          NativeModules.GoBackendModule.getBackendStatus((s) => {
            if (s === "running") {
              clearTimeout(timeout);
              resolve(s);
            } else {
              setTimeout(check, 500);
            }
          });
        };
        setTimeout(check, 500);
      });
    });
  }, []);

  const resolveStreamUrl = useCallback(
    (filePath: string): Promise<string> => {
      return new Promise((resolve, reject) => {
        const isExternal = filePath.startsWith("/sdcard/") || filePath.startsWith("/storage/");
        NativeModules.GoBackendModule.getStreamUrl(
          filePath,
          isExternal,
          (url) => {
            if (url) {
              resolve(url);
            } else {
              reject(new Error("Failed to get stream URL"));
            }
          }
        );
      });
    },
    []
  );

  useEffect(() => {
    let cancelled = false;

    async function init() {
      try {
        const data = parseInitData();
        if (!data) {
          setError("No init data provided");
          setLoading(false);
          return;
        }

        setFileName(data.fileName || "Unknown");

        await ensureBackend();
        if (cancelled) return;

        const url = await resolveStreamUrl(data.filePath);
        if (cancelled) return;

        setStreamUrl(url);
        setLoading(false);
      } catch (e: any) {
        if (!cancelled) {
          setError(e?.message || "Initialization failed");
          setLoading(false);
        }
      }
    }

    init();
    return () => {
      cancelled = true;
    };
  }, []);

  if (loading) {
    return (
      <view style={styles.container}>
        <view style={styles.loadingContainer}>
          <view style={styles.spinner} />
          <text style={styles.loadingText}>Loading player...</text>
        </view>
      </view>
    );
  }

  if (error) {
    return (
      <view style={styles.container}>
        <view style={styles.errorContainer}>
          <text style={styles.errorIcon}>⚠</text>
          <text style={styles.errorText}>{error}</text>
        </view>
      </view>
    );
  }

  return (
    <view style={styles.container}>
      <VideoPlayer streamUrl={streamUrl} fileName={fileName} />
    </view>
  );
}

const styles = {
  container: {
    flex: 1,
    backgroundColor: "#000000",
  } as const,
  loadingContainer: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
  } as const,
  spinner: {
    width: 40,
    height: 40,
    borderRadius: 20,
    borderWidth: 3,
    borderColor: "#333333",
    borderTopColor: ACCENT,
  } as const,
  loadingText: {
    color: "#ffffff",
    fontSize: 14,
    marginTop: 12,
  } as const,
  errorContainer: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    padding: 24,
  } as const,
  errorIcon: {
    fontSize: 48,
    marginBottom: 12,
  } as const,
  errorText: {
    color: "#ff4444",
    fontSize: 14,
    textAlign: "center",
  } as const,
};
