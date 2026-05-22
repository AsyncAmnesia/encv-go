package utils

import (
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

func DetectVideoFormat(filePath string) (string, error) {
	cmd := FFProbeCmd("-v", "error", "-show_entries", "format=format_name", "-of", "default=noprint_wrappers=1:nokey=1", filePath)
	output, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("ffprobe failed: %s", string(ee.Stderr))
		}
		return "", fmt.Errorf("failed to run ffprobe: %w", err)
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
