// Package ffmpeg 是 encv-go 调 ffmpeg/ffprobe 的统一入口。
//
// 🆕 2026-06-15 重构（拆 Runner 抽象层）：
//   - 旧设计：Runner interface + global SetRunner + 各平台 Runner 实现（Worker / Native / Exec）
//   - 新设计：包级公开函数 Encode / Probe / IsAvailable
//     - 平台差异用 build tag 隔离（internal: encode_android.go / probe_android.go / available_android.go）
//     - 不再有全局可变状态，编译时决定路径选择
//   - Encode：跑 ffmpeg 转码
//     - 沙箱（!android）：os.Exec("/usr/bin/ffmpeg", args...) 直调
//     - 真机（android）：spawn worker (cmd/ffmpeg-worker/ffmpeg_worker.c) 子进程 + JSON-RPC
//   - Probe：跑 ffprobe 提取 metadata
//     - 沙箱：os.Exec("/usr/bin/ffprobe", args...) 直调
//     - 真机：utils.CallFFprobeNative in-process cgo（不走 worker）
//   - IsAvailable：探活
//     - 沙箱：检查 /usr/bin/ffmpeg + /usr/bin/ffprobe
//     - 真机：utils.CheckFFmpegAvailable in-process cgo dlopen 探活
//
// 历史：
//   - 2026-06-11 Phase 1: WorkerRunner 解决真机 cgo hang
//   - 2026-06-11 Phase 2: WorkerRunner 选为首选
//   - 2026-06-15: 重构为 Encode/Probe/IsAvailable 三函数 + 删 Runner 抽象 + 删旧 Go worker
//   - 原 runner.go / worker_runner.go / worker_client.go / exec_runner.go / native_runner.go 全部删除
//   - 原 cmd/ffmpeg-worker/main_android.go / main_exec.go（旧 Go worker）替换为 ffmpeg_worker.c
package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)
// EncodeResult 是 ffmpeg/ffprobe 调用的标准结果。
type EncodeResult struct {
	Stdout     []byte
	Stderr     string
	ExitCode   int
	DurationMs int64
}

// workerRequest/Response 与 cmd/ffmpeg-worker/ffmpeg_worker.c 的 JSON 协议对齐。
// 协议细节：worker 读 stdin JSON，argv[0] 由 worker 内部 prepend "ffmpeg"，
// 写 stdout JSON {"exit_code":N,"stdout":"...","stderr":"...","duration_ms":M,"error":"..."}。
type workerRequest struct {
	Args      []string `json:"args"`
	FFmpegBin string   `json:"ffmpeg_bin,omitempty"`
	LibDir    string   `json:"lib_dir,omitempty"`
	TimeoutMs int      `json:"timeout_ms,omitempty"`
	Mode      string   `json:"mode,omitempty"` // "ffmpeg"（默认） | "ffprobe"
}

type workerResponse struct {
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// locateWorker 找 ffmpeg-worker binary 路径。
//
// 优先级：
//  1. $ENCV_FFMPEG_WORKER（绝对路径或 PATH basename）
//  2. $ENCV_LIB_DIR/libffmpeg-worker.so 或 ffmpeg-worker（真机 nativeLibraryDir）
//  3. $ENCV_BIN_DIR/ffmpeg-worker
//  4. <exe-dir>/ffmpeg-worker
//  5. <exe-dir>/../../cmd/ffmpeg-worker/ffmpeg-worker（dev mode）
//  6. exec.LookPath("ffmpeg-worker")
//
// 真机：Kotlin EncvGoService.kt 设 ENCV_FFMPEG_WORKER = nativeLibraryDir + "/libffmpeg-worker.so"，
// 所以 (2) 命中。
func locateWorker() (string, error) {
	candidates := []string{}
	if v := os.Getenv("ENCV_FFMPEG_WORKER"); v != "" {
		candidates = append(candidates, v)
	}
	if dir := os.Getenv("ENCV_LIB_DIR"); dir != "" {
		candidates = append(candidates, filepath.Join(dir, "libffmpeg-worker.so"))
		candidates = append(candidates, filepath.Join(dir, "ffmpeg-worker"))
	}
	if dir := os.Getenv("ENCV_BIN_DIR"); dir != "" {
		candidates = append(candidates, filepath.Join(dir, "ffmpeg-worker"))
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(exeDir, "ffmpeg-worker"))
		candidates = append(candidates, filepath.Join(exeDir, "..", "..", "cmd", "ffmpeg-worker", "ffmpeg-worker"))
	}
	if path, err := exec.LookPath("ffmpeg-worker"); err == nil {
		candidates = append(candidates, path)
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if !strings.Contains(c, string(os.PathSeparator)) {
			if full, err := exec.LookPath(c); err == nil {
				return full, nil
			}
			continue
		}
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}
	return "", errors.New("ffmpeg-worker binary not found (set ENCV_FFMPEG_WORKER or build it: `go build -o /usr/local/bin/ffmpeg-worker ./cmd/ffmpeg-worker`)")
}

// runWorkerJSON 通过 worker binary 跑 args（mode="ffmpeg" 走 ffmpeg_run，mode="ffprobe" 走 ffprobe_run）。
// ctx cancel + 硬 timer SIGKILL 兜底（防 cgo 阻塞父进程）。
//
// 🆕 2026-06-15：替代旧 workerClient.RunWithOutput（go 文件已删除）；协议与 ffmpeg_worker.c 同步。
func runWorkerJSON(ctx context.Context, workerBin string, mode string, args []string) (*EncodeResult, error) {
	timeoutMs := 0
	if deadline, ok := ctx.Deadline(); ok {
		timeoutMs = int(time.Until(deadline).Milliseconds())
		if timeoutMs < 0 {
			timeoutMs = 0
		}
	}
	req := workerRequest{
		Args:      args,
		FFmpegBin: locateFFmpegSystem(),
		LibDir:    os.Getenv("ENCV_LIB_DIR"),
		TimeoutMs: timeoutMs,
		Mode:      mode,
	}
	reqBytes, _ := json.Marshal(req)

	cmd := exec.CommandContext(ctx, workerBin)
	cmd.Stdin = bytes.NewReader(reqBytes)
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start worker: %w", err)
	}

	// 硬 timer SIGKILL 兜底（防 ctx cancel 失败时 cgo 阻塞父进程）
	var hardTimer *time.Timer
	if timeoutMs > 0 {
		hardTimer = time.AfterFunc(time.Duration(timeoutMs+500)*time.Millisecond, func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		})
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var runErr error
	select {
	case runErr = <-done:
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		<-done
		runErr = ctx.Err()
	}
	if hardTimer != nil {
		hardTimer.Stop()
	}

	// 解析 worker stdout（最后一行 JSON）
	stdout := stdoutBuf.String()
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		if stdout == "" {
			return &EncodeResult{Stderr: stderrBuf.String(), ExitCode: exitCode}, runErr
		}
	}

	var resp workerResponse
	for _, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if err := json.Unmarshal([]byte(line), &resp); err == nil {
			break
		}
	}

	finalErr := runErr
	if finalErr == nil && resp.Error != "" {
		finalErr = fmt.Errorf("ffmpeg worker reported: %s", resp.Error)
	}

	return &EncodeResult{
		Stdout:     []byte(resp.Stdout),
		Stderr:     resp.Stderr + stderrBuf.String(),
		ExitCode:   resp.ExitCode,
		DurationMs: resp.DurationMs,
	}, finalErr
}

// locateFFmpegSystem 找系统 ffmpeg binary（沙箱走）。
// 真机返回 ""（不参与决策，worker 自己会用 lib_dir）。
func locateFFmpegSystem() string {
	if v := os.Getenv("ENCV_FFMPEG_BIN"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v
		}
	}
	if dir := os.Getenv("ENCV_BIN_DIR"); dir != "" {
		p := filepath.Join(dir, "ffmpeg")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		return path
	}
	return ""
}

// DetectVideoFormat 检测视频文件的容器格式（mp4 / mkv / 其他）。
//
// 内部用 ffmpeg.Probe（沙箱调系统 ffprobe binary，真机 in-process cgo 调 libffprobe.so）。
// 短时调用（< 100ms），用 background ctx 即可。
func DetectVideoFormat(filePath string) (string, error) {
	output, err := Probe(context.Background(),
		"-v", "error",
		"-show_entries", "format=format_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath,
	)
	if err != nil {
		return "", fmt.Errorf("ffprobe failed: %w", err)
	}

	formatName := strings.TrimSpace(string(output))
	if formatName == "" {
		return "", fmt.Errorf("could not determine container format")
	}

	switch {
	case strings.Contains(formatName, "matroska"):
		return "mkv", nil
	case strings.Contains(formatName, "mp4"):
		return "mp4", nil
	default:
		parts := strings.Split(formatName, ",")
		return strings.ToLower(parts[0]), nil
	}
}
