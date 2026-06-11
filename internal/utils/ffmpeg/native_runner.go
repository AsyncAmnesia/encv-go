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
