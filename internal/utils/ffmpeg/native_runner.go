//go:build android
// +build android

package ffmpeg

import (
	"context"

	"github.com/Soltus/encv-go/internal/utils"
)

// NativeRunner calls ffmpeg/ffprobe via in-process cgo dlopen on libffmpeg.so.
// Used as the FALLBACK when the ffmpeg-worker subprocess is not available
// (e.g. ffmpeg-worker binary not shipped in the AAR, or ENCV_FFMPEG_WORKER
// not set by Kotlin EncvGoService).
//
// ⚠️ cgo call blocks the OS thread — context cancel cannot interrupt an
// in-flight cgo call. Phase 2 (2026-06-11) prefers WorkerRunner to avoid this.
type NativeRunner struct{}

func NewNativeRunner() *NativeRunner { return &NativeRunner{} }

func (r *NativeRunner) Run(ctx context.Context, args []string) error {
	_, _, code, err := r.RunWithOutput(ctx, args)
	if err != nil {
		return err
	}
	if code != 0 {
		return &utils.NativeError{
			Type:     utils.NativeErrorExit,
			ExitCode: code,
			Detail:   "ffmpeg_run returned non-zero",
		}
	}
	return nil
}

func (r *NativeRunner) RunWithOutput(ctx context.Context, args []string) ([]byte, string, int, error) {
	res, err := utils.CallFFmpegNative(args)
	if err != nil {
		return nil, "", -1, err
	}
	return []byte(res.Stdout), res.Stderr, res.ExitCode, nil
}

func (r *NativeRunner) Probe(args ...string) ([]byte, error) {
	_, _, _, err := r.RunWithOutput(context.Background(), args)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// Available 探测 libffmpeg.so / libffprobe.so 是否能 dlopen + dlsym 关键 symbol。
// 委托给 utils.CheckFFmpegAvailable（内部用 RTLD_NOW | RTLD_LOCAL 试 dlopen，
// 不会执行 ffmpeg_run，因此 cgo 不会阻塞 OS thread）。
//
// Phase 2 行为：init() 已经优先选 WorkerRunner（worker binary 存在的话），
// 所以本方法实际只在以下场景被调用：
//   1) init() 决定走 NativeRunner（worker 不在）
//   2) 运行时有人调 ffmpeg.Available() 探测健康
//   3) mock_generator 之类上层逻辑检查 runtime capability
//
// 2026-06-11: 加这个方法是因为 Runner interface 要求 Available()，
// 之前 Phase 1 重写时漏写导致 native_runner.go 编译失败。
func (r *NativeRunner) Available() (ffmpegOk bool, ffprobeOk bool, errMsg string) {
	ok, ok2, msg, _, _ := utils.CheckFFmpegAvailable()
	return ok, ok2, msg
}

// init prefers WorkerRunner (subprocess isolation) over NativeRunner (in-process
// cgo). WorkerRunner is safe because the parent can SIGKILL the worker on ctx
// cancel; NativeRunner cannot interrupt an in-flight cgo call.
//
// 2026-06-11 Phase 2: changed from "always NativeRunner" to "WorkerRunner first
// if available, else NativeRunner". Worker binary availability is decided by
// worker_client.locateWorker() (checks $ENCV_FFMPEG_WORKER / $ENCV_LIB_DIR /
// exe-dir / PATH). On real device, Kotlin EncvGoService.kt sets:
//   ENCV_FFMPEG_WORKER = nativeLibraryDir + "/libffmpeg-worker.so"
// so this typically picks WorkerRunner. Falls back to NativeRunner only if
// the worker binary is not shipped.
func init() {
	if wr := NewWorkerRunner(); wr != nil {
		if ok, _, _ := wr.Available(); ok {
			SetRunner(wr)
			return
		}
	}
	SetRunner(&NativeRunner{})
}
