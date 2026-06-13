//go:build android
// +build android

// ffmpeg-worker for real Android device.
//
// 与 main_exec.go 的区别：
//   - 沙箱/桌面：os.Exec(ffmpeg_bin) — 系统有 ffmpeg binary
//   - 真机：cgo dlopen libffmpeg.so (经 utils.CallFFmpegNative) — Android 端没有 ffmpeg binary
//
// 真机调用链：
//   父进程 (libencv-go.so)
//     └─ os.Exec → fork+exec "libffmpeg-worker.so" (本 worker)
//          └─ CallFFmpegNative → cgo dlopen libffmpeg.so → ffmpeg_run
//   父进程 ctx cancel → exec.CommandContext → SIGKILL worker 进程 → 父进程 unblock
//
// IPC 协议同 main_exec.go: JSON over stdin/stdout。
//
// 软超时 timeoutMs：worker 内部启 goroutine，N ms 后 os.Exit(124) 自我了断。
// 父进程的 ctx cancel 才是硬保证（SIGKILL 整个 worker 进程）。
//
// 2026-06-11 Phase 2: split by build tag — see main_exec.go header for context.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Soltus/encv-go/internal/utils"
)

type workerRequest struct {
	Args      []string `json:"args"`
	FFmpegBin string   `json:"ffmpeg_bin,omitempty"`
	LibDir    string   `json:"lib_dir,omitempty"`
	TimeoutMs int      `json:"timeout_ms,omitempty"`
}

type workerResponse struct {
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

func main() {
	req, err := readRequest(os.Stdin)
	if err != nil {
		writeResponse(workerResponse{Error: fmt.Sprintf("read request: %v", err)})
		os.Exit(1)
	}

	// 软超时：cgo 阻塞没法 cancel；worker 自身在 N ms 后自爆，
	// 父进程仍以 SIGKILL 作硬保证。
	if req.TimeoutMs > 0 {
		ms := req.TimeoutMs
		go func() {
			time.Sleep(time.Duration(ms) * time.Millisecond)
			fmt.Fprintf(os.Stderr, "[ffmpeg-worker] soft timeout %dms exceeded; self-exit (父进程仍会 SIGKILL)\n", ms)
			os.Exit(124)
		}()
	}

	start := time.Now()
	resp := runOnAndroid(req)
	resp.DurationMs = time.Since(start).Milliseconds()
	writeResponse(resp)
}

// runOnAndroid 把 cgo dlopen 的 ffmpeg 调用转成 workerResponse。
// 与 native_runner.go 的 NativeRunner 区别：worker 进程是子进程，可被父进程 SIGKILL。
func runOnAndroid(req *workerRequest) workerResponse {
	// ENCV_LIB_DIR 由父进程透传，utils.CallFFmpegNative 内部 Getenv 读取
	res, err := utils.CallFFmpegNative(req.Args)
	if err != nil {
		// 把 NativeError 分类错误码透出，前端可分类提示
		if ne, ok := err.(*utils.NativeError); ok {
			switch ne.Type {
			case utils.NativeErrorDlopen:
				return workerResponse{
					ExitCode: -1,
					Stderr:   ne.Detail,
					Error:    fmt.Sprintf("[ENGINE_LOAD_FAILED] %s", ne.Detail),
				}
			case utils.NativeErrorSymbol:
				return workerResponse{
					ExitCode: -2,
					Stderr:   ne.Detail,
					Error:    fmt.Sprintf("[ENGINE_SYMBOL_MISSING] %s", ne.Detail),
				}
			case utils.NativeErrorExit:
				return workerResponse{
					ExitCode: ne.ExitCode,
					Stderr:   ne.Detail,
					Error:    fmt.Sprintf("[ENGINE_EXIT_ERROR] %s", ne.Detail),
				}
			}
		}
		return workerResponse{ExitCode: -1, Stderr: err.Error(), Error: err.Error()}
	}
	return workerResponse{
		ExitCode: res.ExitCode,
		Stdout:   res.Stdout,
		Stderr:   res.Stderr,
	}
}

func readRequest(r io.Reader) (*workerRequest, error) {
	// stdin 是 JSON，可能是单行或多行（bufio 安全）
	data, err := io.ReadAll(bufio.NewReader(r))
	if err != nil {
		return nil, err
	}
	var req workerRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w (raw=%q)", err, string(data[:min(200, len(data))]))
	}
	return &req, nil
}

func writeResponse(resp workerResponse) {
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
