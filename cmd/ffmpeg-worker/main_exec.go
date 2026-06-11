//go:build !android
// +build !android

// ffmpeg-worker is a standalone subprocess that runs ONE ffmpeg invocation
// in an isolated OS process. The main encv-go process spawns it via os.Exec
// and can SIGKILL it on context timeout, which is impossible for an in-process
// cgo call to libffmpeg.so.
//
// IPC: JSON over stdin/stdout.
//
// Input (stdin):
//   {
//     "args": ["-i", "input.mp4", "-c", "copy", "output.mkv"],  // ffmpeg args (argv[0] "ffmpeg" is prepended)
//     "ffmpeg_bin": "/usr/bin/ffmpeg",                          // optional, for sandbox; on Android use libffmpeg.so
//     "lib_dir": "/path/to/jniLibs/arm64-v8a",                  // optional, for Android cgo path
//     "timeout_ms": 5000                                         // optional, soft warning if exceeded
//   }
//
// Output (stdout, single line of JSON then EOF):
//   {
//     "exit_code": 0,
//     "stdout": "...",
//     "stderr": "...",
//     "duration_ms": 1234,
//     "error": ""                                                 // non-empty on ffmpeg error
//   }
//
// Build tags:
//   main_exec.go   !android → sandbox/dev: os.Exec(ffmpeg_bin)
//   main_android.go android  → real device: cgo dlopen libffmpeg.so via utils.CallFFmpegNative
//
// History:
//   2026-06-11 Phase 1 refactor: created to solve "real device cgo call
//   hangs the gin handler, frontend spinner forever".
//   2026-06-11 Phase 2: split by build tag — Android now uses worker subprocess
//   wrapping cgo dlopen, so the main encv-go process can SIGKILL the worker
//   on ctx cancel (the cgo call inside worker is then torn down with the
//   worker process, instead of blocking the main gin handler).
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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

	bin, err := resolveFFmpegBin(req)
	if err != nil {
		writeResponse(workerResponse{Error: err.Error()})
		os.Exit(1)
	}

	// 软超时：worker 自身检测，超过 N 秒主动 KILL ffmpeg（避免父进程必须 SIGKILL worker）
	// 父进程的 ctx timeout 才是硬保证（SIGKILL worker）
	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	start := time.Now()
	var cmd *exec.Cmd
	if timeout > 0 {
		cmd = exec.Command(bin, req.Args...)
	} else {
		cmd = exec.Command(bin, req.Args...)
	}
	stdoutBuf := &strings.Builder{}
	stderrBuf := &strings.Builder{}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		writeResponse(workerResponse{Error: fmt.Sprintf("start ffmpeg: %v", err), DurationMs: time.Since(start).Milliseconds()})
		os.Exit(1)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var runErr error
	if timeout > 0 {
		select {
		case runErr = <-done:
			// 正常完成
		case <-time.After(timeout):
			_ = cmd.Process.Kill()
			runErr = fmt.Errorf("ffmpeg timeout after %s", timeout)
			<-done // 等 goroutine 退出
		}
	} else {
		runErr = <-done
	}

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	resp := workerResponse{
		ExitCode:   exitCode,
		Stdout:     stdoutBuf.String(),
		Stderr:     stderrBuf.String(),
		DurationMs: time.Since(start).Milliseconds(),
	}
	if runErr != nil {
		resp.Error = runErr.Error()
	}
	writeResponse(resp)
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

// resolveFFmpegBin 决定 ffmpeg 调用方式
//   1) 请求中显式指定 ffmpeg_bin → 直接用（exec 路径，沙箱）
//   2) 显式指定 lib_dir → 走 cgo 调 libffmpeg.so (real device，Phase 1 用 goroutine timeout 兜底，Phase 2 改进)
//   3) 都没有 → 错误
func resolveFFmpegBin(req *workerRequest) (string, error) {
	if req.FFmpegBin != "" {
		return req.FFmpegBin, nil
	}
	if req.LibDir != "" {
		// Phase 1: 暂时不实现 cgo 子进程化（需单独 build tag + cgo 代码重复）
		// 直接报错，让父进程走 NativeRunner 路径
		return "", fmt.Errorf("lib_dir mode not yet implemented in worker (Phase 2); parent must use cgo directly")
	}
	// fallback: 找 PATH 里的 ffmpeg
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("ffmpeg binary not found: set ffmpeg_bin or install ffmpeg in PATH")
}

// stub for min() on older Go (we use Go 1.21+)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ensure filepath is referenced (for Phase 2 cgo path)
var _ = filepath.Join
