package ffmpeg

import (
	"context"
	"fmt"
	"sync"
)

// Runner 抽象了 FFmpeg/FFProbe 的执行方式。
// 桌面端通过 exec.Command 调用二进制文件，Android 通过 dlopen 调用 libffmpeg.so/libffprobe.so。
type Runner interface {
	// Run 执行 ffmpeg 命令，等待完成。返回 error 表示执行失败（包括非零退出码）。
	Run(ctx context.Context, args []string) error

	// RunWithOutput 执行命令并捕获 stdout 和 stderr。
	// exitCode 为进程退出码。当 err != nil 时，exitCode 可能仍有意义（如非零退出）。
	RunWithOutput(ctx context.Context, args []string) (stdout []byte, stderr string, exitCode int, err error)

	// Probe 执行 ffprobe 命令并返回 stdout。便捷方法，内部调用 RunWithOutput 并仅返回 stdout。
	Probe(args ...string) ([]byte, error)

	// Available 检查 ffmpeg 和 ffprobe 是否可用。
	Available() (ffmpegOk bool, ffprobeOk bool, errMsg string)
}

var (
	globalRunner     Runner
	globalRunnerOnce sync.Once
)

func SetRunner(r Runner) {
	globalRunner = r
}

func GetRunner() Runner {
	return globalRunner
}

// MustGetRunner 获取全局 Runner，如果未设置则 panic。
// 用于在明确需要 Runner 但忘记初始化的场景下快速暴露问题。
func MustGetRunner() Runner {
	if globalRunner == nil {
		panic("ffmpeg: global runner not set. Call ffmpeg.SetRunner() during initialization.")
	}
	return globalRunner
}

// Run 是全局 Runner.Run 的便捷包装。
func Run(ctx context.Context, args ...string) error {
	return MustGetRunner().Run(ctx, args)
}

// RunWithOutput 是全局 Runner.RunWithOutput 的便捷包装。
func RunWithOutput(ctx context.Context, args ...string) (stdout []byte, stderr string, exitCode int, err error) {
	return MustGetRunner().RunWithOutput(ctx, args)
}

// Probe 是全局 Runner.Probe 的便捷包装。
func Probe(args ...string) ([]byte, error) {
	return MustGetRunner().Probe(args...)
}

// Available 是全局 Runner.Available 的便捷包装。
func Available() (bool, bool, string) {
	if globalRunner == nil {
		return false, false, "ffmpeg runner not initialized"
	}
	return globalRunner.Available()
}

// ErrNotInitialized 在 Runner 未初始化时返回。
var ErrNotInitialized = fmt.Errorf("ffmpeg: runner not initialized")
