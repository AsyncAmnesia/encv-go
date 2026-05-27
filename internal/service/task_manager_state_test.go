package service

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func defaultTestConfig() *config.Config {
	return &config.Config{
		Password: "test-password",
		Server:   config.DefaultConfig().Server,
	}
}

func newTestTaskManager(broadcaster Broadcaster, servingDir ...string) *TaskManager {
	dir := "/tmp/test-serving"
	if len(servingDir) > 0 && servingDir[0] != "" {
		dir = servingDir[0]
	}
	return &TaskManager{
		tasks:       make(map[string]*MobileTask),
		servingDir:  dir,
		cfg:         nil,
		stopCh:      make(chan struct{}),
		broadcaster: broadcaster,
	}
}

func TestTaskManager_Create(t *testing.T) {
	mb := new(MockBroadcaster)
	mb.On("Broadcast", "task:created", mock.Anything).Return()
	tm := newTestTaskManager(mb)

	task := tm.Create("encrypt", "/video.mp4", "", "", 0)

	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "encrypt", task.Type)
	assert.Equal(t, "/video.mp4", task.SourcePath)
	assert.Equal(t, "queued", task.Status)
	assert.Equal(t, 0, task.Progress)
	assert.False(t, task.CreatedAt.IsZero())

	stored, err := tm.Get(task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, stored.ID)

	mb.AssertCalled(t, "Broadcast", "task:created", mock.Anything)
}

func TestTaskManager_CreateBroadcastsEvent(t *testing.T) {
	mb := new(MockBroadcaster)
	mb.On("Broadcast", "task:created", mock.Anything).Return()
	tm := newTestTaskManager(mb)

	tm.Create("decrypt", "/test.encv", "", "", 0)

	calls := mb.FindCalls("task:created")
	assert.Len(t, calls, 1)
	assert.Equal(t, "task:created", calls[0].MsgType)
}

func TestTaskManager_List(t *testing.T) {
	mb := new(MockBroadcaster)
	mb.On("Broadcast", mock.Anything, mock.Anything).Return()
	tm := newTestTaskManager(mb)

	tm.Create("encrypt", "/a.mp4", "", "", 0)
	tm.Create("decrypt", "/b.encv", "", "", 0)

	list := tm.List()
	assert.Len(t, list, 2)
}

func TestTaskManager_ListEmpty(t *testing.T) {
	mb := new(MockBroadcaster)
	tm := newTestTaskManager(mb)

	list := tm.List()
	assert.Empty(t, list)
}

func TestTaskManager_GetNotFound(t *testing.T) {
	mb := new(MockBroadcaster)
	tm := newTestTaskManager(mb)

	_, err := tm.Get("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "task not found", err.Error())
}

func TestTaskManager_Cancel_QueuedTask(t *testing.T) {
	mb := new(MockBroadcaster)
	mb.On("Broadcast", mock.Anything, mock.Anything).Return()
	tm := newTestTaskManager(mb)

	task := tm.Create("encrypt", "/test.mp4", "", "", 0)
	assert.Equal(t, "queued", task.Status)

	cancelled, err := tm.Cancel(task.ID)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", cancelled.Status)

	stored, _ := tm.Get(task.ID)
	assert.Equal(t, "cancelled", stored.Status)
}

func TestTaskManager_Cancel_RunningTask(t *testing.T) {
	mb := new(MockBroadcaster)
	mb.On("Broadcast", mock.Anything, mock.Anything).Return()
	tm := newTestTaskManager(mb)

	task := tm.Create("encrypt", "/test.mp4", "", "", 0)
	tm.mu.Lock()
	task.Status = "running"
	tm.mu.Unlock()

	cancelled, err := tm.Cancel(task.ID)
	require.NoError(t, err)
	assert.Equal(t, "cancelling", cancelled.Status)
}

func TestTaskManager_Cancel_NotFound(t *testing.T) {
	mb := new(MockBroadcaster)
	tm := newTestTaskManager(mb)

	_, err := tm.Cancel("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "task not found", err.Error())
}

func TestTaskManager_CancelBroadcastsUpdate(t *testing.T) {
	mb := new(MockBroadcaster)
	mb.On("Broadcast", mock.Anything, mock.Anything).Return()
	tm := newTestTaskManager(mb)

	task := tm.Create("encrypt", "/test.mp4", "", "", 0)
	mb.calls_ = nil

	_, _ = tm.Cancel(task.ID)

	calls := mb.FindCalls("task:update")
	assert.Len(t, calls, 1)
	data, ok := calls[0].Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, task.ID, data["id"])
	assert.Equal(t, "cancelled", data["status"])
}

func TestTaskManager_Retry(t *testing.T) {
	mb := new(MockBroadcaster)
	mb.On("Broadcast", mock.Anything, mock.Anything).Return()
	tm := newTestTaskManager(mb)

	task := tm.Create("encrypt", "/test.mp4", "", "", 0)
	tm.mu.Lock()
	task.Status = "failed"
	task.Error = "something went wrong"
	task.Progress = 50
	tm.mu.Unlock()

	retried, err := tm.Retry(task.ID)
	require.NoError(t, err)
	assert.Equal(t, "queued", retried.Status)
	assert.Empty(t, retried.Error)
	assert.Equal(t, 0, retried.Progress)
}

func TestTaskManager_Retry_NotFound(t *testing.T) {
	mb := new(MockBroadcaster)
	tm := newTestTaskManager(mb)

	_, err := tm.Retry("nonexistent")
	assert.Error(t, err)
}

func TestTaskManager_RetryBroadcastsUpdate(t *testing.T) {
	mb := new(MockBroadcaster)
	mb.On("Broadcast", mock.Anything, mock.Anything).Return()
	tm := newTestTaskManager(mb)

	task := tm.Create("encrypt", "/test.mp4", "", "", 0)
	tm.mu.Lock()
	task.Status = "failed"
	tm.mu.Unlock()
	mb.calls_ = nil

	_, _ = tm.Retry(task.ID)

	calls := mb.FindCalls("task:update")
	assert.Len(t, calls, 1)
	data, ok := calls[0].Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "queued", data["status"])
}

func TestTaskManager_Dequeue(t *testing.T) {
	mb := new(MockBroadcaster)
	mb.On("Broadcast", mock.Anything, mock.Anything).Return()
	tm := newTestTaskManager(mb)

	task := tm.Create("encrypt", "/test.mp4", "", "", 0)

	dequeued := tm.dequeue()
	require.NotNil(t, dequeued)
	assert.Equal(t, task.ID, dequeued.ID)
	assert.Equal(t, "running", dequeued.Status)

	dequeued2 := tm.dequeue()
	assert.Nil(t, dequeued2)
}

func TestTaskManager_Dequeue_Empty(t *testing.T) {
	mb := new(MockBroadcaster)
	tm := newTestTaskManager(mb)

	result := tm.dequeue()
	assert.Nil(t, result)
}

func TestTaskManager_Dequeue_SkipsNonQueued(t *testing.T) {
	mb := new(MockBroadcaster)
	mb.On("Broadcast", mock.Anything, mock.Anything).Return()
	tm := newTestTaskManager(mb)

	task := tm.Create("encrypt", "/test.mp4", "", "", 0)
	tm.mu.Lock()
	task.Status = "running"
	tm.mu.Unlock()

	result := tm.dequeue()
	assert.Nil(t, result)
}

func TestTaskManager_ResolveAbsPath(t *testing.T) {
	mb := new(MockBroadcaster)
	tm := newTestTaskManager(mb)
	tm.servingDir = "/data/serving"

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple path", "/video.mp4", "/data/serving/video.mp4"},
		{"nested path", "/movies/action.mp4", "/data/serving/movies/action.mp4"},
		{"root path", "/", "/data/serving"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.resolveAbsPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTaskManager_ResolveAbsPath_PathTraversal(t *testing.T) {
	mb := new(MockBroadcaster)
	tm := newTestTaskManager(mb)
	tm.servingDir = "/data/serving"

	result := tm.resolveAbsPath("../../../etc/passwd")
	assert.Empty(t, result, "relative path traversal should return empty string")

	result = tm.resolveAbsPath("/../../../etc/passwd")
	assert.True(t, strings.HasPrefix(result, "/data/serving"), "absolute path traversal should be contained within servingDir, got %s", result)
}

func TestTaskManager_FailTask(t *testing.T) {
	mb := new(MockBroadcaster)
	mb.On("Broadcast", mock.Anything, mock.Anything).Return()
	tm := newTestTaskManager(mb)

	task := tm.Create("encrypt", "/test.mp4", "", "", 0)

	tm.failTask(task.ID, "something broke")

	stored, _ := tm.Get(task.ID)
	assert.Equal(t, "failed", stored.Status)
	assert.Equal(t, "something broke", stored.Error)
}

func TestTaskManager_FailTaskBroadcastsEvent(t *testing.T) {
	mb := new(MockBroadcaster)
	mb.On("Broadcast", mock.Anything, mock.Anything).Return()
	tm := newTestTaskManager(mb)

	task := tm.Create("encrypt", "/test.mp4", "", "", 0)
	mb.calls_ = nil

	tm.failTask(task.ID, "test error")

	calls := mb.FindCalls("task:completed")
	assert.Len(t, calls, 1)
	data, ok := calls[0].Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "failed", data["status"])
	assert.Equal(t, "test error", data["error"])
}

func TestTaskManager_FailTask_NonExistent(t *testing.T) {
	mb := new(MockBroadcaster)
	tm := newTestTaskManager(mb)

	tm.failTask("nonexistent", "error")

	assert.Empty(t, mb.GetCalls())
}

func TestTaskManager_ConcurrentAccess(t *testing.T) {
	mb := new(MockBroadcaster)
	mb.On("Broadcast", mock.Anything, mock.Anything).Return()
	tm := newTestTaskManager(mb)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tm.Create("encrypt", "/test.mp4", "", "", 0)
		}()
	}
	wg.Wait()

	list := tm.List()
	assert.Len(t, list, 100)
}

func TestTaskManager_NilBroadcaster(t *testing.T) {
	tm := newTestTaskManager(nil)

	task := tm.Create("encrypt", "/test.mp4", "", "", 0)
	assert.Equal(t, "queued", task.Status)

	tm.failTask(task.ID, "error")
	stored, _ := tm.Get(task.ID)
	assert.Equal(t, "failed", stored.Status)

	_, err := tm.Cancel(task.ID)
	require.NoError(t, err)

	_, err = tm.Retry(task.ID)
	require.NoError(t, err)
}

func TestTaskManager_WorkerLifecycle(t *testing.T) {
	mb := new(MockBroadcaster)
	mb.On("Broadcast", mock.Anything, mock.Anything).Return()

	cfg := defaultTestConfig()
	tm := NewTaskManager(t.TempDir(), cfg, mb)

	done := make(chan struct{})
	go func() {
		tm.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("TaskManager.Stop() timed out")
	}
}
