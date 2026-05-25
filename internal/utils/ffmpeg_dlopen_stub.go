//go:build !android

package utils

import "fmt"

type NativeErrorType int

const (
	NativeErrorDlopen NativeErrorType = iota
	NativeErrorSymbol
	NativeErrorExit
)

type NativeError struct {
	Type     NativeErrorType
	Detail   string
	ExitCode int
}

func (e *NativeError) Error() string {
	return e.Detail
}

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

func CheckFFmpegAvailable() (ffmpegOk bool, ffprobeOk bool, errMsg string, ffmpegDetail string, ffprobeDetail string) {
	return false, false, "not available on this platform", "", ""
}
