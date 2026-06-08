//go:build android

package ffmpeg

import (
	"context"
	"fmt"

	"github.com/Soltus/encv-go/internal/utils"
)

type NativeRunner struct{}

func (r *NativeRunner) Run(_ context.Context, args []string) error {
	result, err := utils.CallFFmpegNative(args)
	if err != nil {
		return classifyError(err, "ffmpeg")
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("ffmpeg failed (exit %d): %s", result.ExitCode, truncateString(result.Stderr, 200))
	}
	return nil
}

func (r *NativeRunner) RunWithOutput(_ context.Context, args []string) ([]byte, string, int, error) {
	result, err := utils.CallFFmpegNative(args)
	if err != nil {
		return nil, "", -1, classifyError(err, "ffmpeg")
	}
	return []byte(result.Stdout), result.Stderr, result.ExitCode, nil
}

func (r *NativeRunner) Probe(args ...string) ([]byte, error) {
	result, err := utils.CallFFprobeNative(args)
	if err != nil {
		return nil, classifyError(err, "ffprobe")
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("ffprobe failed (exit %d): %s", result.ExitCode, result.Stderr)
	}
	return []byte(result.Stdout), nil
}

func (r *NativeRunner) Available() (bool, bool, string) {
	ffmpegOk, ffprobeOk, errMsg, _, _ := utils.CheckFFmpegAvailable()
	return ffmpegOk, ffprobeOk, errMsg
}

func classifyError(err error, tool string) error {
	if nativeErr, ok := err.(*utils.NativeError); ok {
		switch nativeErr.Type {
		case utils.NativeErrorDlopen:
			return fmt.Errorf("%s: [ENGINE_LOAD_FAILED] %s", tool, nativeErr.Detail)
		case utils.NativeErrorSymbol:
			return fmt.Errorf("%s: [ENGINE_SYMBOL_MISSING] %s", tool, nativeErr.Detail)
		case utils.NativeErrorExit:
			return fmt.Errorf("%s: [ENGINE_EXIT_ERROR] exit code %d: %s", tool, nativeErr.ExitCode, nativeErr.Detail)
		}
	}
	return fmt.Errorf("%s: %w", tool, err)
}

func truncateString(s string, maxLen int) string {
	s = trimString(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func trimString(s string) string {
	var i int
	for i = len(s) - 1; i >= 0; i-- {
		if s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' {
			break
		}
	}
	return s[:i+1]
}

func init() {
	SetRunner(&NativeRunner{})
}
