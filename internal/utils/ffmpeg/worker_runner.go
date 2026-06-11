// WorkerRunner implements the Runner interface by delegating to a subprocess
// (ffmpeg-worker binary). This gives true OS-process isolation: the main
// encv-go process can SIGKILL the worker on context timeout, which is
// impossible for an in-process cgo call to libffmpeg.so.
//
// 平台差异（2026-06-11 Phase 2 解决）：
//   - 沙箱 / dev preview：worker binary 走 os.Exec(ffmpeg_bin) 调系统 ffmpeg
//   - 真机：worker binary（libffmpeg-worker.so）内部 cgo dlopen libffmpeg.so
//
// Project architecture（实测）：
//   - 项目不用 gomobile bind（用户 2026-06-11 明确反馈）
//   - encv-go mobile 是独立 Go binary（cmd/encv-mobile/main.go）
//   - Android 端走 CI cross-compile 产物 libencv-go.so（Kotlin ProcessBuilder 启）
//   - 跟 OpenList fork libopenlist.so 模式一致（改名 .so 是为了过 AGP jniLibs 打包）
//
// init() 路径：
//   - native_runner.go (android build)   改 init：WorkerRunner 优先 → NativeRunner fallback
//   - exec_runner.go (!android build)   改 init：WorkerRunner 优先 → ExecRunner fallback
//   都调 NewWorkerRunner() 检查 workerClient.Available()。
//
// 2026-06-11 Phase 1: created to solve sandbox cgo/subprocess 隔离
// 2026-06-11 Phase 2: 去 !android build tag；Available() 不再需要 ffmpeg binary 真存在（真机走 lib_dir）
package ffmpeg

import "context"

type WorkerRunner struct {
	client *workerClient
}

func NewWorkerRunner() *WorkerRunner {
	c, _ := getWorkerClient()
	return &WorkerRunner{client: c}
}

func (r *WorkerRunner) Run(ctx context.Context, args []string) error {
	return r.client.Run(ctx, args)
}

func (r *WorkerRunner) RunWithOutput(ctx context.Context, args []string) ([]byte, string, int, error) {
	return r.client.RunWithOutput(ctx, args)
}

func (r *WorkerRunner) Probe(args ...string) ([]byte, error) {
	return r.client.Probe(args...)
}

// Available reports whether the worker subprocess is usable.
//   - false → init() falls back to NativeRunner (Android cgo) or
//     ExecRunner (sandbox without worker built).
//   - true → WorkerRunner is the active runner.
//
// Phase 2 判定（2026-06-11）：
//   - 沙箱：worker binary 在 + ffmpeg binary 在 → true
//   - 真机：worker binary 在（worker 内部走 lib_dir cgo 路径）→ true
//   - 都没有：false
//
// Available() 当前在 worker_client.go 实现：只看 binPath != ""（worker binary 找到即可信）。
// 剩下的 ffmpeg_bin / lib_dir 选择由 worker main_android.go / main_exec.go 自己处理。
func (r *WorkerRunner) Available() (ffmpegOk bool, ffprobeOk bool, errMsg string) {
	if r.client == nil || !r.client.Available() {
		return false, false, "ffmpeg-worker binary not found"
	}
	// Phase 2：worker binary 存在即视为可用。具体 ffmpeg 怎么调（system binary
	// 还是 lib_dir cgo）由 worker 内部决定。
	return true, true, ""
}
