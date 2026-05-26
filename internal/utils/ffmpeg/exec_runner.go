//go:build !android

package ffmpeg

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

// ExecRunner 通过 exec.Command 调用系统安装的 ffmpeg/ffprobe 二进制文件。
type ExecRunner struct {
	binDirOnce sync.Once
	binDir     string
}

func (r *ExecRunner) findBinDir() string {
	r.binDirOnce.Do(func() {
		if envDir := os.Getenv("ENCV_BIN_DIR"); envDir != "" {
			ffprobePath := filepath.Join(envDir, "ffprobe")
			if _, err := os.Stat(ffprobePath); err == nil {
				r.binDir = envDir
				return
			}
		}
		if exePath, err := os.Executable(); err == nil {
			dir := filepath.Dir(exePath)
			ffprobePath := filepath.Join(dir, "ffprobe")
			if _, err := os.Stat(ffprobePath); err == nil {
				r.binDir = dir
				return
			}
		}
		if path, err := exec.LookPath("ffprobe"); err == nil {
			r.binDir = filepath.Dir(path)
		}
	})
	return r.binDir
}

func (r *ExecRunner) findBin(name string) string {
	binDir := r.findBinDir()
	if binDir != "" {
		path := filepath.Join(binDir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return name
}

func (r *ExecRunner) Run(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, r.findBin("ffmpeg"), args...)
	return cmd.Run()
}

func (r *ExecRunner) RunWithOutput(_ context.Context, args []string) ([]byte, string, int, error) {
	bin := "ffmpeg"
	if len(args) > 0 && (args[0] == "-version" || strings.HasPrefix(args[0], "-")) {
		bin = r.findBin("ffmpeg")
	} else if len(args) > 0 {
		bin = r.findBin("ffmpeg")
	}
	cmd := exec.Command(bin, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}
	return stdoutBuf.Bytes(), stderrBuf.String(), exitCode, err
}

func (r *ExecRunner) Probe(args ...string) ([]byte, error) {
	cmd := exec.Command(r.findBin("ffprobe"), args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}
	return output, nil
}

func (r *ExecRunner) Available() (bool, bool, string) {
	ffmpegPath := r.findBin("ffmpeg")
	ffprobePath := r.findBin("ffprobe")

	var errMsgs []string

	_, ffmpegErr := os.Stat(ffmpegPath)
	ffmpegOk := ffmpegErr == nil
	if !ffmpegOk {
		errMsgs = append(errMsgs, fmt.Sprintf("ffmpeg not found at %s", ffmpegPath))
	}

	_, ffprobeErr := os.Stat(ffprobePath)
	ffprobeOk := ffprobeErr == nil
	if !ffprobeOk {
		errMsgs = append(errMsgs, fmt.Sprintf("ffprobe not found at %s", ffprobePath))
	}

	errMsg := ""
	if len(errMsgs) > 0 {
		errMsg = strings.Join(errMsgs, "; ")
	}

	return ffmpegOk, ffprobeOk, errMsg
}

// DetectVideoFormat 检测视频文件的容器格式。
// 从原 utils/video.go 迁移过来，因为它是纯业务逻辑而非平台相关。
func DetectVideoFormat(filePath string) (string, error) {
	output, err := Probe("-v", "error", "-show_entries", "format=format_name", "-of", "default=noprint_wrappers=1:nokey=1", filePath)
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

func init() {
	SetRunner(&ExecRunner{})
}
