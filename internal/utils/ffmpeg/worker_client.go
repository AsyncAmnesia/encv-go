// workerClient spawns ffmpeg-worker as a subprocess for true OS-process
// isolation. On context timeout, the worker is SIGKILL'd (or SIGTERM then
// SIGKILL after grace period), guaranteeing the main encv-go process never
// blocks on ffmpeg.
//
// Search order for the worker binary:
//   1) $ENCV_FFMPEG_WORKER (absolute path or basename on PATH)
//   2) $ENCV_BIN_DIR/ffmpeg-worker
//   3) <exe-dir>/ffmpeg-worker
//   4) <exe-dir>/../../cmd/ffmpeg-worker/ffmpeg-worker (dev mode relative to internal/utils)
//   5) LookPath("ffmpeg-worker")
//
// 真机环境：Kotlin 在 EncvGoService.kt 启动 libencv-go.so 时设
//   ENCV_FFMPEG_WORKER = applicationInfo.nativeLibraryDir + "/libffmpeg-worker.so"
// （同 ENCV_LIB_DIR 路径），所以 (1) 命中。
//
// 2026-06-11 Phase 1 refactor.
// 2026-06-11 Phase 2: 去掉 !android build tag — Android 端也走 worker
// 子进程（cgo 调用挪到 worker 内部），父进程可被 SIGKILL worker 解锁。
package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type workerClient struct {
	binPath string
	ffmpeg  string // system ffmpeg binary for sandbox; empty on Android
}

var (
	wcOnce     sync.Once
	wcInstance *workerClient
	wcErr      error
)

func getWorkerClient() (*workerClient, error) {
	wcOnce.Do(func() {
		bin, err := locateWorker()
		if err != nil {
			wcErr = err
			return
		}
		wcInstance = &workerClient{
			binPath: bin,
			ffmpeg:  locateFFmpeg(),
		}
	})
	return wcInstance, wcErr
}

func locateWorker() (string, error) {
	candidates := []string{}
	if v := os.Getenv("ENCV_FFMPEG_WORKER"); v != "" {
		candidates = append(candidates, v)
	}
	if dir := os.Getenv("ENCV_LIB_DIR"); dir != "" {
		// 🆕 2026-06-11 Phase 2：真机上 worker binary 跟 libffmpeg.so 一起放在
		// nativeLibraryDir（Kotlin 端 EncvGoService.kt 把 ENCV_FFMPEG_WORKER
		// 也设成 nativeLibraryDir/libffmpeg-worker.so），所以 ENCV_LIB_DIR
		// 跟 ENCV_BIN_DIR 等价 — 但为不污染 ENCV_BIN_DIR 语义，独立查找。
		candidates = append(candidates, filepath.Join(dir, "libffmpeg-worker.so"))
		candidates = append(candidates, filepath.Join(dir, "ffmpeg-worker"))
	}
	if dir := os.Getenv("ENCV_BIN_DIR"); dir != "" {
		candidates = append(candidates, filepath.Join(dir, "ffmpeg-worker"))
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(exeDir, "ffmpeg-worker"))
		// dev mode: backend runs from <repo>/bin/encv, worker is in <repo>/cmd/ffmpeg-worker/
		candidates = append(candidates, filepath.Join(exeDir, "..", "..", "cmd", "ffmpeg-worker", "ffmpeg-worker"))
	}
	if path, err := exec.LookPath("ffmpeg-worker"); err == nil {
		candidates = append(candidates, path)
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		// 兼容 PATH 中的 basename
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

func locateFFmpeg() string {
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

// Available reports whether the worker subprocess is usable.
//
// Phase 2（2026-06-11）Available 判定变化：
//   - 沙箱：worker binary 存在 AND 系统 ffmpeg binary 存在 → true
//   - 真机：worker binary 存在 AND ENCV_LIB_DIR 存在（worker 用 lib_dir 走 cgo）→ true
//
// 之前 false 永远命中（真机 ffmpeg binary 不存在）→ 强制走 NativeRunner cgo → hang。
// 现在 worker 内部自己决定走 ffmpeg_bin 还是 lib_dir，父进程只看 worker 能不能用。
func (c *workerClient) Available() bool {
	if c == nil || c.binPath == "" {
		return false
	}
	// worker binary 找到即可信，剩下的由 worker main_android.go / main_exec.go 自己处理
	return true
}

// Run runs the worker as a subprocess with hard SIGKILL timeout.
// Returns (stdout, stderr, exitCode, err). On context timeout, err is non-nil
// and the worker process is reaped (not leaked).
func (c *workerClient) Run(ctx context.Context, args []string) error {
	_, _, exitCode, err := c.RunWithOutput(ctx, args)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("ffmpeg worker exit %d", exitCode)
	}
	return nil
}

func (c *workerClient) RunWithOutput(ctx context.Context, args []string) ([]byte, string, int, error) {
	if c == nil || c.binPath == "" {
		return nil, "", -1, errors.New("worker client not initialized")
	}

	timeoutMs := 0
	if deadline, ok := ctx.Deadline(); ok {
		timeoutMs = int(time.Until(deadline).Milliseconds())
		if timeoutMs < 0 {
			timeoutMs = 0
		}
	}
	req := workerRequest{
		Args:      args,
		FFmpegBin: c.ffmpeg,
		LibDir:    os.Getenv("ENCV_LIB_DIR"), // 🆕 Phase 2：传给 worker，让 worker 走 cgo 路径
		TimeoutMs: timeoutMs,
	}
	reqBytes, _ := json.Marshal(req)

	cmd := exec.CommandContext(ctx, c.binPath)
	cmd.Stdin = bytes.NewReader(reqBytes)
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, "", -1, fmt.Errorf("start worker: %w", err)
	}

	// 双保险：父进程 ctx cancel → exec.CommandContext 发 SIGKILL
	// 内部 worker 也有 timeoutMs 软超时（双保险）
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var runErr error
	select {
	case runErr = <-done:
	case <-ctx.Done():
		// exec.CommandContext 已发 SIGKILL（Go 1.20+ 默认 SIGKILL on cancel）
		_ = cmd.Process.Kill()
		<-done
		runErr = ctx.Err()
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
		// 即使 worker 被 SIGKILL，stdout 可能为空，直接返回
		if stdout == "" {
			return nil, stderrBuf.String(), exitCode, runErr
		}
	}

	// stdout 最后一行是 JSON 响应（worker 用 fmt.Println 输出，会带 \n）
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

	// 把 worker 报告的 error 透出
	finalErr := runErr
	if finalErr == nil && resp.Error != "" {
		finalErr = fmt.Errorf("ffmpeg worker reported: %s", resp.Error)
	}

	return []byte(resp.Stdout), resp.Stderr + stderrBuf.String(), resp.ExitCode, finalErr
}

// Probe is a thin convenience wrapper.
func (c *workerClient) Probe(args ...string) ([]byte, error) {
	stdout, _, _, err := c.RunWithOutput(context.Background(), args)
	if err != nil {
		return nil, err
	}
	return stdout, nil
}

// workerRequest/Response must match cmd/ffmpeg-worker/main_*.go
// (duplicated here to avoid cyclic import; both must stay in sync).
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

// ensure io is referenced for compile guards
var _ = io.EOF
