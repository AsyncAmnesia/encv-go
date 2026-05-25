package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var (
	binDirOnce  sync.Once
	binDirCache string
)

func GetBinDir() string {
	binDirOnce.Do(func() {
		if envDir := os.Getenv("ENCV_BIN_DIR"); envDir != "" {
			ffprobePath := filepath.Join(envDir, "ffprobe")
			if _, err := os.Stat(ffprobePath); err == nil {
				binDirCache = envDir
				return
			}
		}
		if exePath, err := os.Executable(); err == nil {
			dir := filepath.Dir(exePath)
			ffprobePath := filepath.Join(dir, "ffprobe")
			if _, err := os.Stat(ffprobePath); err == nil {
				binDirCache = dir
				return
			}
		}
		if path, err := exec.LookPath("ffprobe"); err == nil {
			binDirCache = filepath.Dir(path)
			return
		}
	})
	return binDirCache
}

var (
	isMobileOnce sync.Once
	isMobileVal  bool
)

func IsMobile() bool {
	isMobileOnce.Do(func() {
		isMobileVal = os.Getenv("ENCV_MOBILE") == "1"
	})
	return isMobileVal
}

func FFProbeCmd(args ...string) *exec.Cmd {
	binDir := GetBinDir()
	if binDir != "" {
		ffprobePath := filepath.Join(binDir, "ffprobe")
		if _, err := os.Stat(ffprobePath); err == nil {
			return exec.Command(ffprobePath, args...)
		}
	}
	return exec.Command("ffprobe", args...)
}

func FFmpegCmd(args ...string) *exec.Cmd {
	binDir := GetBinDir()
	if binDir != "" {
		ffmpegPath := filepath.Join(binDir, "ffmpeg")
		if _, err := os.Stat(ffmpegPath); err == nil {
			return exec.Command(ffmpegPath, args...)
		}
	}
	return exec.Command("ffmpeg", args...)
}

func FFmpegCmdContext(ctx context.Context, args ...string) *exec.Cmd {
	binDir := GetBinDir()
	if binDir != "" {
		ffmpegPath := filepath.Join(binDir, "ffmpeg")
		if _, err := os.Stat(ffmpegPath); err == nil {
			return exec.CommandContext(ctx, ffmpegPath, args...)
		}
	}
	return exec.CommandContext(ctx, "ffmpeg", args...)
}

func classifyNativeError(err error) string {
	if nativeErr, ok := err.(*NativeError); ok {
		switch nativeErr.Type {
		case NativeErrorDlopen:
			return fmt.Sprintf("[ENGINE_LOAD_FAILED] %s", nativeErr.Detail)
		case NativeErrorSymbol:
			return fmt.Sprintf("[ENGINE_SYMBOL_MISSING] %s", nativeErr.Detail)
		case NativeErrorExit:
			return fmt.Sprintf("[ENGINE_EXIT_ERROR] exit code %d: %s", nativeErr.ExitCode, nativeErr.Detail)
		}
	}
	return err.Error()
}

func FFProbeOutput(args ...string) ([]byte, error) {
	if IsMobile() {
		result, err := callFFprobeNative(args)
		if err != nil {
			return nil, fmt.Errorf("ffprobe: %s", classifyNativeError(err))
		}
		if result.exitCode != 0 {
			return nil, fmt.Errorf("ffprobe failed (exit %d): %s", result.exitCode, truncateString(result.stderr, 200))
		}
		return []byte(result.stdout), nil
	}
	cmd := FFProbeCmd(args...)
	return cmd.Output()
}

func FFmpegRun(args ...string) error {
	if IsMobile() {
		result, err := callFFmpegNative(args)
		if err != nil {
			return fmt.Errorf("ffmpeg: %s", classifyNativeError(err))
		}
		if result.exitCode != 0 {
			return fmt.Errorf("ffmpeg failed (exit %d): %s", result.exitCode, truncateString(result.stderr, 200))
		}
		return nil
	}
	cmd := FFmpegCmd(args...)
	return cmd.Run()
}

func FFmpegRunWithStderr(args ...string) (string, error) {
	if IsMobile() {
		result, err := callFFmpegNative(args)
		if err != nil {
			return "", fmt.Errorf("ffmpeg: %s", classifyNativeError(err))
		}
		if result.exitCode != 0 {
			return result.stderr, fmt.Errorf("ffmpeg failed (exit %d): %s", result.exitCode, truncateString(result.stderr, 200))
		}
		return result.stderr, nil
	}
	cmd := FFmpegCmd(args...)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	return stderrBuf.String(), err
}

func FFmpegRunWithContext(ctx context.Context, args ...string) (string, error) {
	if IsMobile() {
		result, err := callFFmpegNative(args)
		if err != nil {
			return "", fmt.Errorf("ffmpeg: %s", classifyNativeError(err))
		}
		if result.exitCode != 0 {
			return result.stderr, fmt.Errorf("ffmpeg failed (exit %d): %s", result.exitCode, truncateString(result.stderr, 200))
		}
		return result.stderr, nil
	}
	cmd := FFmpegCmdContext(ctx, args...)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	return stderrBuf.String(), err
}

func DetectVideoFormat(filePath string) (string, error) {
	output, err := FFProbeOutput("-v", "error", "-show_entries", "format=format_name", "-of", "default=noprint_wrappers=1:nokey=1", filePath)
	if err != nil {
		return "", fmt.Errorf("ffprobe failed: %w", err)
	}

	formatName := strings.TrimSpace(string(output))
	if formatName == "" {
		return "", fmt.Errorf("could not determine container format")
	}

	switch {
	case strings.Contains(formatName, "matroska"):
		return "mkv", nil
	case strings.Contains(formatName, "mp4"):
		return "mp4", nil
	default:
		parts := strings.Split(formatName, ",")
		return strings.ToLower(parts[0]), nil
	}
}

func truncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
