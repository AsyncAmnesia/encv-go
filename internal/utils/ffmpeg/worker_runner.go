//go:build !android
// +build !android

package ffmpeg

import "context"

// WorkerRunner implements the Runner interface by delegating to a subprocess
// (ffmpeg-worker binary). This gives true OS-process isolation: the main
// encv-go process can SIGKILL the worker on context timeout, which is
// impossible for an in-process cgo call to libffmpeg.so.
//
// Use case: sandbox / dev preview where we have a system ffmpeg binary AND
// can build the worker. On real Android device (gomobile bind product), the
// worker binary is not shipped (Phase 2), so this runner returns "unavailable"
// and the init() falls back to NativeRunner (cgo).
//
// 2026-06-11 Phase 1 refactor: created to address "real device cgo hangs
// gin handler, frontend spinner forever" issue, by making sandbox fully
// subprocess-isolated. Real device is documented as Phase 2 (requires AAR
// build system changes in app/encv-mobile repo).
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
//   - false → init() falls back to NativeRunner (cgo on real device) or
//     ExecRunner (sandbox without worker built).
//   - true → WorkerRunner is the active runner.
//
// In real Android device (gomobile bind), this returns false because the
// worker binary is not yet shipped in the AAR (Phase 2 deferred).
func (r *WorkerRunner) Available() (ffmpegOk bool, ffprobeOk bool, errMsg string) {
	if r.client == nil || !r.client.Available() {
		return false, false, "ffmpeg-worker binary not found"
	}
	if r.client.ffmpeg == "" {
		return false, false, "ffmpeg binary not found (ENCV_FFMPEG_BIN or PATH)"
	}
	return true, true, ""
}
