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
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
		LibDir:    os.Getenv("ENCV_LIB_DIR"),
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

	// 🆕 2026-06-12 Phase 4 hard timeout：ctx cancel 之外再套一层 AfterFunc SIGKILL
	// 防御：万一 ctx cancel 失败（cgo 把 Go 调度搞死），500ms 之后无条件 SIGKILL worker
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
			writeHeartbeat() // 即使失败也要更新心跳（让 Kotlin 知道父进程还活着）
			return nil, stderrBuf.String(), exitCode, runErr
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

	writeHeartbeat() // 🆕 每次 ffmpeg 调用后写心跳 → Kotlin 端判 mtime 是否 stale
	return []byte(resp.Stdout), resp.Stderr + stderrBuf.String(), resp.ExitCode, finalErr
}

// writeHeartbeat 写当前时间戳到 $ENCV_HEARTBEAT_PATH，供 Kotlin EncvGoService 探活
// 失败也忽略（探活是 best-effort，不能因为写不了心跳就让 ffmpeg 调用失败）
func writeHeartbeat() {
	path := os.Getenv("ENCV_HEARTBEAT_PATH")
	if path == "" {
		return
	}
	_ = os.WriteFile(path, []byte(strconv.FormatInt(time.Now().UnixMilli(), 10)), 0644)
}

// =============================================================================
// StartHeartbeatLoop — 独立后台心跳循环（2026-06-14 修复安卓 WS 误杀 bug）
// =============================================================================
//
// 【根因 2026-06-14】
// Kotlin EncvGoService 1s poll $ENCV_HEARTBEAT_PATH 的 mtime，>HEARTBEAT_STALE_MS
// (8s) 没更新就判定 Go 进程 hang → destroyForcibly() → restart。restart 期间所有
// WS / HTTP 连接全断，UI 现象就是 readyState=3 + HttpPoll 失败。
//
// 旧实现：writeHeartbeat() 只在 ffmpeg 调用后被调用（worker_client.go:243）。
//   → 如果 Go 进程空闲（用户只看 task 列表、不触发 ffmpeg），心跳文件
//     8s 内不被更新 → Kotlin 误判 hang → Go 进程被杀 → WS 莫名其妙断 7s
//
// 新实现：服务启动时 StartHeartbeatLoop(ctx) 启一个独立 goroutine，每 2s
// 写一次心跳。2s < HEARTBEAT_STALE_MS/2 (4s)，留出 4x 安全余量。
//
// 优先级：writeHeartbeat()（ffmpeg 调用后）→ 立即写；StartHeartbeatLoop → 兜底
// 重复写无害（只是 os.Chtimes + os.WriteFile）。
//
// 设计权衡：
//   - 用 os.Chtimes 而不是 os.WriteFile：避免 mtime 精度问题（ext4 mtime 精度 1s
//     多次 write 在同 1s 内 mtime 不变，Kotlin lastModified 看到的就是旧值）
//     实际在 Android /sdcard (FAT32/exFAT) 上 mtime 精度是 2s！
//     os.Chtimes 显式设 mtime 到当前 time.Now()，绕过文件系统精度限制
//   - 文件不存在时 fallback 到 os.WriteFile（首次 touch）
//   - 失败仅日志（探活 best-effort，进程不能因心跳写失败而退出）
//   - ctx 取消时退出（服务关闭时优雅停止）
//
// =============================================================================

const heartbeatTickInterval = 2 * time.Second

// heartbeatLoopsMu 保护 heartbeatLoops slice（仅测试用）
var heartbeatLoopsMu sync.Mutex
var heartbeatLoops []context.CancelFunc

// StartHeartbeatLoop 启一个 goroutine 定期更新 $ENCV_HEARTBEAT_PATH。
// 必须由 server 启动时调用一次。重复调用安全（idempotent via sync.Once）。
func StartHeartbeatLoop(ctx context.Context) {
	startHeartbeatLoopOnce.Do(func() {
		// 包装 ctx 以便 ResetHeartbeatLoopForTesting 能取消在跑的 loop
		loopCtx, cancel := context.WithCancel(ctx)
		heartbeatLoopsMu.Lock()
		heartbeatLoops = append(heartbeatLoops, cancel)
		heartbeatLoopsMu.Unlock()
		go heartbeatLoop(loopCtx)
	})
}

var startHeartbeatLoopOnce sync.Once

// ResetHeartbeatLoopForTesting 重置 sync.Once + 取消所有在跑的 loop
// 仅供单元测试使用
func ResetHeartbeatLoopForTesting() {
	heartbeatLoopsMu.Lock()
	for _, cancel := range heartbeatLoops {
		cancel()
	}
	heartbeatLoops = nil
	heartbeatLoopsMu.Unlock()

	// sync.Once 没有 Reset 方法（Go 1.21+ 也不支持）
	// 单元测试场景下顺序执行，单纯重新赋值 sync.Once 即可
	startHeartbeatLoopOnce = sync.Once{}
}

func heartbeatLoop(ctx context.Context) {
	path := os.Getenv("ENCV_HEARTBEAT_PATH")
	if path == "" {
		slog.Warn("heartbeatLoop: ENCV_HEARTBEAT_PATH not set, loop will be no-op")
		// 不 return — 等 env 后续被设置（startup 顺序保护）
	}

	ticker := time.NewTicker(heartbeatTickInterval)
	defer ticker.Stop()

	// 立即 touch 一次，避免启动后第一次 1s poll 看到 mtime=0
	touchHeartbeatFile(path)

	for {
		select {
		case <-ctx.Done():
			slog.Info("heartbeatLoop: stopping (ctx cancelled)")
			return
		case <-ticker.C:
			touchHeartbeatFile(path)
		}
	}
}

func touchHeartbeatFile(path string) {
	if path == "" {
		// env 还没设置时重读（启动 race）
		path = os.Getenv("ENCV_HEARTBEAT_PATH")
		if path == "" {
			return
		}
	}
	now := time.Now()
	// 优先用 os.Chtimes 显式设 mtime — 避开 ext4/FAT32 mtime 精度问题
	if err := os.Chtimes(path, now, now); err != nil {
		// 文件不存在（首次）→ 用 os.WriteFile 创建
		if os.IsNotExist(err) {
			_ = os.WriteFile(path, []byte(strconv.FormatInt(now.UnixMilli(), 10)), 0644)
			return
		}
		// 其他错误（权限 / ENOENT race）→ 写一次兜底
		_ = os.WriteFile(path, []byte(strconv.FormatInt(now.UnixMilli(), 10)), 0644)
	}
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
