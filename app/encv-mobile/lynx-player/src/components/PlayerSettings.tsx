import { useState, useCallback } from "@lynx-js/react";

const ACCENT = "#ffad00";

interface PlayerSettingsProps {
  onClose: () => void;
}

export function PlayerSettings(props: PlayerSettingsProps) {
  const { onClose } = props;
  const [autoplay, setAutoplay] = useState(true);
  const [hardwareDecode, setHardwareDecode] = useState(true);
  const [darkMode, setDarkMode] = useState(true);

  const handleAutoplay = useCallback(() => {
    setAutoplay((prev) => !prev);
  }, []);

  const handleHardwareDecode = useCallback(() => {
    const next = !hardwareDecode;
    setHardwareDecode(next);
    NativeModules.MpvPlayerModule.setProperty(
      "hwdec",
      next ? "auto" : "no"
    );
  }, [hardwareDecode]);

  const handleDarkMode = useCallback(() => {
    setDarkMode((prev) => !prev);
  }, []);

  return (
    <view style={styles.overlay}>
      <view style={styles.panel}>
        <view style={styles.header}>
          <text style={styles.headerTitle}>Settings</text>
          <view style={styles.closeButton} bindtap={onClose}>
            <text style={styles.closeIcon}>✕</text>
          </view>
        </view>

        <view style={styles.divider} />

        <view style={styles.row} bindtap={handleAutoplay}>
          <text style={styles.rowLabel}>Autoplay</text>
          <view
            style={[
              styles.toggle,
              autoplay ? styles.toggleOn : styles.toggleOff,
            ]}
          >
            <view
              style={[
                styles.toggleKnob,
                autoplay ? styles.toggleKnobOn : styles.toggleKnobOff,
              ]}
            />
          </view>
        </view>

        <view style={styles.row} bindtap={handleHardwareDecode}>
          <text style={styles.rowLabel}>Hardware Decode</text>
          <view
            style={[
              styles.toggle,
              hardwareDecode ? styles.toggleOn : styles.toggleOff,
            ]}
          >
            <view
              style={[
                styles.toggleKnob,
                hardwareDecode ? styles.toggleKnobOn : styles.toggleKnobOff,
              ]}
            />
          </view>
        </view>

        <view style={styles.row} bindtap={handleDarkMode}>
          <text style={styles.rowLabel}>Dark Mode</text>
          <view
            style={[
              styles.toggle,
              darkMode ? styles.toggleOn : styles.toggleOff,
            ]}
          >
            <view
              style={[
                styles.toggleKnob,
                darkMode ? styles.toggleKnobOn : styles.toggleKnobOff,
              ]}
            />
          </view>
        </view>
      </view>
    </view>
  );
}

const styles = {
  overlay: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    backgroundColor: "rgba(0,0,0,0.7)",
    justifyContent: "center",
    alignItems: "center",
  } as const,
  panel: {
    width: 280,
    backgroundColor: "#1a1a1a",
    borderRadius: 12,
    paddingVertical: 8,
  } as const,
  header: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    paddingHorizontal: 16,
    paddingVertical: 12,
  } as const,
  headerTitle: {
    color: "#ffffff",
    fontSize: 16,
    fontWeight: "bold",
  } as const,
  closeButton: {
    width: 32,
    height: 32,
    justifyContent: "center",
    alignItems: "center",
  } as const,
  closeIcon: {
    color: "#ffffff",
    fontSize: 16,
  } as const,
  divider: {
    height: 1,
    backgroundColor: "#333333",
    marginHorizontal: 16,
  } as const,
  row: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    paddingHorizontal: 16,
    paddingVertical: 14,
  } as const,
  rowLabel: {
    color: "#ffffff",
    fontSize: 14,
  } as const,
  toggle: {
    width: 44,
    height: 24,
    borderRadius: 12,
    justifyContent: "center",
  } as const,
  toggleOn: {
    backgroundColor: ACCENT,
  } as const,
  toggleOff: {
    backgroundColor: "#555555",
  } as const,
  toggleKnob: {
    width: 20,
    height: 20,
    borderRadius: 10,
    backgroundColor: "#ffffff",
  } as const,
  toggleKnobOn: {
    marginLeft: 22,
  } as const,
  toggleKnobOff: {
    marginLeft: 2,
  } as const,
};
