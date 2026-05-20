package service

import (
	"errors"
	"sync"
	"time"

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
	tasks map[string]*MobileTask
	mu    sync.RWMutex
}

func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks: make(map[string]*MobileTask),
	}
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

func (tm *TaskManager) Cancel(id string) (*MobileTask, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.tasks[id]
	if !ok {
		return nil, errors.New("task not found")
	}

	task.Status = "cancelled"
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
	return task, nil
}
