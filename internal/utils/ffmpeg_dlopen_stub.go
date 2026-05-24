//go:build !android

package utils

import "fmt"

type nativeResult struct {
	exitCode int
	stdout   string
	stderr   string
}

func callFFmpegNative(args []string) (*nativeResult, error) {
	return nil, fmt.Errorf("ffmpeg native not available on this platform")
}

func callFFprobeNative(args []string) (*nativeResult, error) {
	return nil, fmt.Errorf("ffprobe native not available on this platform")
}
