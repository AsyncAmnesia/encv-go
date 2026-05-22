package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	"github.com/google/uuid"
)

type MobileTask struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	SourcePath string    `json:"sourcePath"`
	Status     string    `json:"status"`
	Progress   int       `json:"progress"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

type TaskManager struct {
	tasks      map[string]*MobileTask
	mu         sync.RWMutex
	servingDir string
	cfg        *config.Config
	stopCh     chan struct{}
	wg         sync.WaitGroup
	wsHub      *WSHub
}

func NewTaskManager(servingDir string, cfg *config.Config, wsHub *WSHub) *TaskManager {
	tm := &TaskManager{
		tasks:      make(map[string]*MobileTask),
		servingDir: servingDir,
		cfg:        cfg,
		stopCh:     make(chan struct{}),
		wsHub:      wsHub,
	}
	tm.wg.Add(1)
	go tm.worker()
	return tm
}

func (tm *TaskManager) Stop() {
	close(tm.stopCh)
	tm.wg.Wait()
}

func (tm *TaskManager) Create(taskType, sourcePath string) *MobileTask {
	task := &MobileTask{
		ID:         uuid.New().String(),
		Type:       taskType,
		SourcePath: sourcePath,
		Status:     "queued",
		Progress:   0,
		CreatedAt:  time.Now(),
	}

	tm.mu.Lock()
	tm.tasks[task.ID] = task
	tm.mu.Unlock()

	slog.Info("Task created", "id", task.ID, "type", taskType, "source", sourcePath)
	if tm.wsHub != nil {
		tm.wsHub.Broadcast("task:created", task)
	}
	return task
}

func (tm *TaskManager) List() []*MobileTask {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*MobileTask, 0, len(tm.tasks))
	for _, t := range tm.tasks {
		result = append(result, t)
	}
	return result
}

func (tm *TaskManager) Get(id string) (*MobileTask, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, ok := tm.tasks[id]
	if !ok {
		return nil, errors.New("task not found")
	}
	return task, nil
}

func (tm *TaskManager) Cancel(id string) (*MobileTask, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.tasks[id]
	if !ok {
		return nil, errors.New("task not found")
	}

	if task.Status == "running" {
		task.Status = "cancelling"
	} else {
		task.Status = "cancelled"
	}
	if tm.wsHub != nil {
		tm.wsHub.Broadcast("task:update", map[string]interface{}{
			"id":     id,
			"status": task.Status,
		})
	}
	return task, nil
}

func (tm *TaskManager) Retry(id string) (*MobileTask, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.tasks[id]
	if !ok {
		return nil, errors.New("task not found")
	}

	task.Status = "queued"
	task.Error = ""
	task.Progress = 0
	if tm.wsHub != nil {
		tm.wsHub.Broadcast("task:update", map[string]interface{}{
			"id":       id,
			"status":   "queued",
			"progress": 0,
		})
	}
	return task, nil
}

func (tm *TaskManager) worker() {
	defer tm.wg.Done()

	for {
		select {
		case <-tm.stopCh:
			return
		default:
		}

		task := tm.dequeue()
		if task == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		tm.processTask(task)
	}
}

func (tm *TaskManager) dequeue() *MobileTask {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, task := range tm.tasks {
		if task.Status == "queued" {
			task.Status = "running"
			return task
		}
	}
	return nil
}

func (tm *TaskManager) resolveAbsPath(sourcePath string) string {
	cleaned := filepath.Clean(sourcePath)
	if strings.HasPrefix(cleaned, "..") {
		return ""
	}
	relPath := strings.TrimPrefix(cleaned, "/")
	return filepath.Join(tm.servingDir, relPath)
}

func (tm *TaskManager) processTask(task *MobileTask) {
	slog.Info("Processing task", "id", task.ID, "type", task.Type, "source", task.SourcePath)

	if tm.wsHub != nil {
		tm.wsHub.Broadcast("task:update", map[string]interface{}{
			"id":       task.ID,
			"type":     task.Type,
			"status":   "running",
			"progress": 0,
		})
	}

	absPath := tm.resolveAbsPath(task.SourcePath)
	if absPath == "" {
		tm.failTask(task.ID, "invalid source path")
		return
	}

	slog.Info("Resolved path", "source", task.SourcePath, "absPath", absPath)

	switch task.Type {
	case "encrypt":
		tm.processEncrypt(task, absPath)
	case "decrypt":
		tm.processDecrypt(task, absPath)
	default:
		tm.failTask(task.ID, fmt.Sprintf("unknown task type: %s", task.Type))
	}
}

func (tm *TaskManager) processEncrypt(task *MobileTask, absPath string) {
	info, err := os.Stat(absPath)
	if err != nil {
		tm.failTask(task.ID, fmt.Sprintf("source file not found: %v", err))
		return
	}

	if info.IsDir() {
		tm.failTask(task.ID, "directory encryption is not supported yet")
		return
	}

	outputDir := filepath.Dir(absPath)
	ctx := config.NewContext(context.Background(), tm.cfg)

	plugin, err := plugins.FindEncryptingPlugin(absPath)
	if err != nil {
		tm.failTask(task.ID, fmt.Sprintf("no encrypting plugin found: %v", err))
		return
	}

	if err := plugin.Initialize(ctx); err != nil {
		tm.failTask(task.ID, fmt.Sprintf("plugin initialization failed: %v", err))
		return
	}

	err = plugins.EncryptFileWithPlugin(ctx, plugin, absPath, tm.servingDir, outputDir)
	if err != nil {
		tm.failTask(task.ID, fmt.Sprintf("encryption failed: %v", err))
		return
	}

	tm.mu.Lock()
	if task.Status != "cancelling" {
		task.Status = "completed"
		task.Progress = 100
	}
	tm.mu.Unlock()

	slog.Info("Task completed", "id", task.ID, "type", task.Type)
	if tm.wsHub != nil {
		tm.wsHub.Broadcast("task:completed", map[string]interface{}{
			"id":     task.ID,
			"status": "completed",
		})
		tm.wsHub.Broadcast("file:change", map[string]interface{}{
			"path": task.SourcePath,
		})
	}
}

func (tm *TaskManager) processDecrypt(task *MobileTask, absPath string) {
	info, err := os.Stat(absPath)
	if err != nil {
		tm.failTask(task.ID, fmt.Sprintf("source file not found: %v", err))
		return
	}

	if info.IsDir() {
		tm.failTask(task.ID, "directory decryption is not supported yet")
		return
	}

	outputDir := filepath.Dir(absPath)
	ctx := config.NewContext(context.Background(), tm.cfg)

	plugin, err := plugins.FindDecryptingPlugin(absPath)
	if err != nil {
		tm.failTask(task.ID, fmt.Sprintf("no decrypting plugin found: %v", err))
		return
	}

	if err := plugin.Initialize(ctx); err != nil {
		tm.failTask(task.ID, fmt.Sprintf("plugin initialization failed: %v", err))
		return
	}

	err = plugins.DecryptContainerWithPlugin(ctx, plugin, absPath, outputDir)
	if err != nil {
		tm.failTask(task.ID, fmt.Sprintf("decryption failed: %v", err))
		return
	}

	tm.mu.Lock()
	if task.Status != "cancelling" {
		task.Status = "completed"
		task.Progress = 100
	}
	tm.mu.Unlock()

	slog.Info("Task completed", "id", task.ID, "type", task.Type)
	if tm.wsHub != nil {
		tm.wsHub.Broadcast("task:completed", map[string]interface{}{
			"id":     task.ID,
			"status": "completed",
		})
		tm.wsHub.Broadcast("file:change", map[string]interface{}{
			"path": task.SourcePath,
		})
	}
}

func (tm *TaskManager) failTask(id, errMsg string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if task, ok := tm.tasks[id]; ok {
		task.Status = "failed"
		task.Error = errMsg
		slog.Error("Task failed", "id", id, "error", errMsg)
		if tm.wsHub != nil {
			tm.wsHub.Broadcast("task:completed", map[string]interface{}{
				"id":     id,
				"status": "failed",
				"error":  errMsg,
			})
		}
	}
}
