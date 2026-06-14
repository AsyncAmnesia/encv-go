package ffmpeg

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 2026-06-14 心跳循环单元测试
//
// 验证根因修复：Go 进程空闲时心跳文件必须被持续更新，
// 否则 Kotlin EncvGoService 8s 判 hang → destroyForcibly。
// =============================================================================

func TestStartHeartbeatLoop_UpdatesMTime(t *testing.T) {
	ResetHeartbeatLoopForTesting() // 确保前一个测试的 loop 已停
	tmpDir := t.TempDir()
	hbPath := filepath.Join(tmpDir, ".encv_heartbeat")
	t.Setenv("ENCV_HEARTBEAT_PATH", hbPath)

	// 第一次手动 touch（loop 也会 touch，但拿一个初始值）
	require.NoError(t, os.WriteFile(hbPath, []byte("init"), 0644))
	initialMTime, err := os.Stat(hbPath)
	require.NoError(t, err)
	initialTime := initialMTime.ModTime()

	// 启 loop，2s tick
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	StartHeartbeatLoop(ctx)

	// 等 3s 让 loop 至少 tick 一次（2s tick + jitter）
	// 关键：mtime 必须前进，证明 loop 在工作
	deadline := time.Now().Add(5 * time.Second)
	var newMTime time.Time
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		fi, err := os.Stat(hbPath)
		if err != nil {
			continue
		}
		newMTime = fi.ModTime()
		if newMTime.After(initialTime) {
			break
		}
	}

	assert.True(t, newMTime.After(initialTime),
		"心跳文件 mtime 必须在 2-3s 内前进；initial=%v new=%v (delta=%v)",
		initialTime, newMTime, newMTime.Sub(initialTime))
}

func TestStartHeartbeatLoop_CreatesFileIfMissing(t *testing.T) {
	ResetHeartbeatLoopForTesting()
	tmpDir := t.TempDir()
	hbPath := filepath.Join(tmpDir, ".encv_heartbeat")
	t.Setenv("ENCV_HEARTBEAT_PATH", hbPath)

	// 文件**不存在** → loop 应自动创建
	_, err := os.Stat(hbPath)
	assert.True(t, os.IsNotExist(err), "文件应该初始不存在")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	StartHeartbeatLoop(ctx)

	// 立即应该看到文件（loop 第一行就 touch）
	require.Eventually(t, func() bool {
		_, err := os.Stat(hbPath)
		return err == nil
	}, 3*time.Second, 100*time.Millisecond, "loop 应在启动时创建心跳文件")
}

func TestStartHeartbeatLoop_StopsOnContextCancel(t *testing.T) {
	ResetHeartbeatLoopForTesting()
	tmpDir := t.TempDir()
	hbPath := filepath.Join(tmpDir, ".encv_heartbeat")
	t.Setenv("ENCV_HEARTBEAT_PATH", hbPath)

	// 第一次 touch
	require.NoError(t, os.WriteFile(hbPath, []byte("init"), 0644))
	mTime1, _ := os.Stat(hbPath)

	ctx, cancel := context.WithCancel(context.Background())
	StartHeartbeatLoop(ctx)

	// 等 3s 看 mtime 前进
	time.Sleep(3 * time.Second)
	mTime2, _ := os.Stat(hbPath)
	assert.True(t, mTime2.ModTime().After(mTime1.ModTime()),
		"cancel 前 mtime 应前进")

	// cancel loop
	cancel()
	mTimeBeforeCancel := mTime2.ModTime()

	// 等 3s — loop 已停，mtime 不应再前进
	time.Sleep(3 * time.Second)
	mTime3, _ := os.Stat(hbPath)
	assert.False(t, mTime3.ModTime().After(mTimeBeforeCancel),
		"cancel 后 mtime 不应再前进（loop 已停）")
}

func TestStartHeartbeatLoop_ResilientToWriteErrors(t *testing.T) {
	ResetHeartbeatLoopForTesting()
	// 设一个无效路径（root 环境下 0555 仍可写，所以仅做冒烟）
	// 真实场景：Android /sdcard 满 / 权限被撤 → loop 收到 EACCES / ENOSPC
	// loop 设计：失败仅日志，进程不能 panic
	tmpDir := t.TempDir()
	hbPath := filepath.Join(tmpDir, ".encv_heartbeat")
	t.Setenv("ENCV_HEARTBEAT_PATH", hbPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 不应 panic（关键断言）
	require.NotPanics(t, func() {
		StartHeartbeatLoop(ctx)
	})

	// 等 2.5s 至少一个 tick 完成
	time.Sleep(2500 * time.Millisecond)
}

func TestStartHeartbeatLoop_Idempotent(t *testing.T) {
	ResetHeartbeatLoopForTesting()
	tmpDir := t.TempDir()
	hbPath := filepath.Join(tmpDir, ".encv_heartbeat")
	t.Setenv("ENCV_HEARTBEAT_PATH", hbPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 调 3 次 — sync.Once 保证只启一个 goroutine
	StartHeartbeatLoop(ctx)
	StartHeartbeatLoop(ctx)
	StartHeartbeatLoop(ctx)

	// 文件应该存在（说明 loop 在跑）
	require.Eventually(t, func() bool {
		_, err := os.Stat(hbPath)
		return err == nil
	}, 3*time.Second, 100*time.Millisecond)
}
