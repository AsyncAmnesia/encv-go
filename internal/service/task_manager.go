package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	pluginInterfaces "github.com/Soltus/encv-go/internal/v2/plugins/interfaces"
	"github.com/Soltus/encv-go/internal/v2/plugins/video"
	"github.com/Soltus/encv-go/internal/v2/types"
	"github.com/google/uuid"
)

type MobileTask struct {
	ID                string            `json:"id"`
	Type              string            `json:"type"`
	SourcePath        string            `json:"sourcePath"`
	TargetPath        string            `json:"targetPath,omitempty"`
	Password          string            `json:"password,omitempty"`
	SecondaryPassword string            `json:"secondaryPassword,omitempty"`
	ExtraFields       map[string]string `json:"extraFields,omitempty"`
	PluginName        string            `json:"pluginName,omitempty"`
	Status            string            `json:"status"`
	Progress          int               `json:"progress"`
	Phase             string            `json:"phase,omitempty"`
	Speed             string            `json:"speed,omitempty"`
	Eta               string            `json:"eta,omitempty"`
	Error             string            `json:"error,omitempty"`
	ErrorDetail       string            `json:"errorDetail,omitempty"`
	Warning           string            `json:"warning,omitempty"`
	WarningDetail     string            `json:"warningDetail,omitempty"`
	ContainerVersion  int               `json:"containerVersion,omitempty"`
	CreatedAt         time.Time         `json:"createdAt"`
	CompletedAt       *time.Time        `json:"completedAt,omitempty"`
	cancelFn          context.CancelFunc
}

type TaskManager struct {
	tasks       map[string]*MobileTask
	mu          sync.RWMutex
	servingDir  string
	cfg         *config.Config
	stopCh      chan struct{}
	wg          sync.WaitGroup
	broadcaster Broadcaster
	persistPath string
}

func NewTaskManager(servingDir string, cfg *config.Config, broadcaster Broadcaster) *TaskManager {
	persistPath := filepath.Join(servingDir, ".encv-tasks.json")

	tm := &TaskManager{
		tasks:       make(map[string]*MobileTask),
		servingDir:  servingDir,
		cfg:         cfg,
		stopCh:      make(chan struct{}),
		broadcaster: broadcaster,
		persistPath: persistPath,
	}

	tm.loadTasks()

	tm.wg.Add(1)
	go tm.worker()
	return tm
}

func (tm *TaskManager) saveTasks() {
	tm.mu.RLock()
	taskList := make([]*MobileTask, 0, len(tm.tasks))
	for _, t := range tm.tasks {
		taskList = append(taskList, t)
	}
	tm.mu.RUnlock()

	data, err := json.MarshalIndent(taskList, "", "  ")
	if err != nil {
		slog.Warn("Failed to marshal tasks for persistence", "error", err)
		return
	}

	tmpPath := tm.persistPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		slog.Warn("Failed to write tasks temp file", "error", err)
		return
	}

	if err := os.Rename(tmpPath, tm.persistPath); err != nil {
		slog.Warn("Failed to rename tasks file", "error", err)
		return
	}
}

func (tm *TaskManager) loadTasks() {
	data, err := os.ReadFile(tm.persistPath)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("Failed to read tasks file", "error", err)
		}
		return
	}

	var taskList []*MobileTask
	if err := json.Unmarshal(data, &taskList); err != nil {
		slog.Warn("Failed to unmarshal tasks", "error", err)
		return
	}

	for _, t := range taskList {
		switch t.Status {
		case "running", "queued":
			t.Status = "failed"
			t.Error = "interrupted by restart"
			now := time.Now()
			t.CompletedAt = &now
		case "cancelling":
			t.Status = "cancelled"
			if t.CompletedAt == nil {
				now := time.Now()
				t.CompletedAt = &now
			}
		}
		t.cancelFn = nil
		t.Speed = ""
		t.Eta = ""
		tm.tasks[t.ID] = t
	}

	slog.Info("Loaded persisted tasks", "count", len(taskList))
}

func (tm *TaskManager) Stop() {
	close(tm.stopCh)
	tm.wg.Wait()
}

func (tm *TaskManager) Create(taskType, sourcePath, targetPath, password string, version int, pluginName string) *MobileTask {
	task := &MobileTask{
		ID:               uuid.New().String(),
		Type:             taskType,
		SourcePath:       sourcePath,
		TargetPath:       targetPath,
		Password:         password,
		Status:           "queued",
		Progress:         0,
		ContainerVersion: version,
		PluginName:       pluginName,
		CreatedAt:        time.Now(),
	}

	tm.mu.Lock()
	tm.tasks[task.ID] = task
	tm.mu.Unlock()

	tm.saveTasks()

	slog.Info("Task created", "id", task.ID, "type", taskType, "source", sourcePath, "target", targetPath, "version", version)
	if tm.broadcaster != nil {
		tm.broadcaster.Broadcast("task:created", task)
	}
	return task
}

func (tm *TaskManager) CreateWithExtras(taskType, sourcePath, targetPath, password, secondaryPassword string, version int, pluginName string, extras map[string]string) *MobileTask {
	task := tm.Create(taskType, sourcePath, targetPath, password, version, pluginName)
	task.SecondaryPassword = secondaryPassword
	task.ExtraFields = extras
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
		if task.cancelFn != nil {
			task.cancelFn()
		}
	} else {
		task.Status = "cancelled"
		now := time.Now()
		task.CompletedAt = &now
	}
	if tm.broadcaster != nil {
		tm.broadcaster.Broadcast("task:update", map[string]interface{}{
			"id":     id,
			"status": task.Status,
		})
	}
	return task, nil
}

func (tm *TaskManager) updateProgress(id string, progress int, phase, speed, eta string) {
	tm.mu.Lock()
	if task, ok := tm.tasks[id]; ok {
		task.Progress = progress
		task.Phase = phase
		task.Speed = speed
		task.Eta = eta
	}
	tm.mu.Unlock()
	if tm.broadcaster != nil {
		tm.broadcaster.Broadcast("task:progress", map[string]interface{}{
			"id":       id,
			"progress": progress,
			"phase":    phase,
			"speed":    speed,
			"eta":      eta,
		})
	}
}

func formatSpeed(bytesPerSec float64) string {
	if bytesPerSec <= 0 {
		return ""
	}
	if bytesPerSec < 1024 {
		return fmt.Sprintf("%.1f B/s", bytesPerSec)
	}
	if bytesPerSec < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
	}
	return fmt.Sprintf("%.1f MB/s", bytesPerSec/(1024*1024))
}

func formatDuration(seconds float64) string {
	if seconds <= 0 {
		return ""
	}
	d := time.Duration(seconds * float64(time.Second))
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func (tm *TaskManager) RemoveTask(id string) error {
	tm.mu.Lock()
	if _, ok := tm.tasks[id]; !ok {
		tm.mu.Unlock()
		return errors.New("task not found")
	}

	delete(tm.tasks, id)
	tm.mu.Unlock()

	tm.saveTasks()

	slog.Info("Task removed", "id", id)
	if tm.broadcaster != nil {
		tm.broadcaster.Broadcast("task:removed", map[string]interface{}{
			"id": id,
		})
	}
	return nil
}

func (tm *TaskManager) ClearCompleted() int {
	tm.mu.Lock()
	removed := 0
	for id, task := range tm.tasks {
		if task.Status == "completed" || task.Status == "failed" || task.Status == "cancelled" {
			delete(tm.tasks, id)
			removed++
		}
	}
	tm.mu.Unlock()

	if removed > 0 {
		tm.saveTasks()
		slog.Info("Cleared completed tasks", "count", removed)
		if tm.broadcaster != nil {
			tm.broadcaster.Broadcast("task:cleared", map[string]interface{}{
				"count": removed,
			})
		}
	}
	return removed
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
	if tm.broadcaster != nil {
		tm.broadcaster.Broadcast("task:update", map[string]interface{}{
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

	tm.updateProgress(task.ID, 0, "queued", "", "")

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

func (tm *TaskManager) getPasswordContext(ctx context.Context, primaryPassword string) context.Context {
	if primaryPassword != "" {
		cfgCopy := *tm.cfg
		cfgCopy.Password = primaryPassword
		return config.NewContext(ctx, &cfgCopy)
	}
	return config.NewContext(ctx, tm.cfg)
}

func (tm *TaskManager) processEncrypt(task *MobileTask, absPath string) {
	taskID := task.ID

	ctx, cancel := context.WithCancel(context.Background())
	task.cancelFn = cancel
	defer cancel()

	tm.updateProgress(taskID, 5, "analyzing", "", "")

	info, err := os.Stat(absPath)
	if err != nil {
		tm.failTask(taskID, fmt.Sprintf("source file not found: %v", err))
		return
	}

	if info.IsDir() {
		tm.failTask(taskID, "directory encryption is not supported yet")
		return
	}

	effectiveVersion := task.ContainerVersion
	if effectiveVersion == 0 {
		effectiveVersion = tm.cfg.GetEffectiveDefaultVersion()
	}
	if !types.IsValidVersion(effectiveVersion) {
		tm.failTask(taskID, fmt.Sprintf("invalid container version: %d", effectiveVersion))
		return
	}
	if types.IsDeprecatedVersion(effectiveVersion) && tm.cfg.IsStrictMode() {
		tm.failTask(taskID, fmt.Sprintf("container version %d is deprecated and strict mode is enabled", effectiveVersion))
		return
	}

	outputDir := filepath.Dir(absPath)
	if task.TargetPath != "" {
		targetAbs := tm.resolveAbsPath(task.TargetPath)
		if targetAbs != "" {
			outputDir = targetAbs
			if mkdirErr := os.MkdirAll(outputDir, 0755); mkdirErr != nil {
				tm.failTask(taskID, fmt.Sprintf("failed to create target directory: %v", mkdirErr))
				return
			}
		}
	}

	requiredSpace := info.Size() * 3 / 2
	if freeSpace, diskErr := getAvailableDiskSpace(outputDir); diskErr == nil && freeSpace < requiredSpace {
		tm.failTask(taskID, fmt.Sprintf("insufficient disk space: need %s, available %s", formatSpeed(float64(requiredSpace)), formatSpeed(float64(freeSpace))))
		return
	}

	tm.updateProgress(taskID, 10, "initializing", "", "")

	var plugin plugins.Plugin
	if task.PluginName != "" {
		plugin, err = plugins.FindPluginByName(task.PluginName)
		if err != nil {
			tm.failTask(taskID, fmt.Sprintf("specified plugin not found: %v", err))
			return
		}
	} else {
		plugin, err = plugins.FindEncryptingPlugin(absPath)
		if err != nil {
			tm.failTask(taskID, fmt.Sprintf("no encrypting plugin found: %v", err))
			return
		}
		task.PluginName = plugin.Name()
	}

	var primaryPassword string
	isPasswordIndependent := false
	if resolver, ok := plugin.(pluginInterfaces.TaskPasswordResolver); ok {
		primaryPassword = resolver.ResolveTaskPassword(task.Password, task.ExtraFields)
		opts := plugin.GetTaskOptions()
		isPasswordIndependent = opts.PasswordStrategy == pluginInterfaces.PasswordIndependent
	} else {
		primaryPassword = task.Password
		if primaryPassword == "" {
			primaryPassword = tm.cfg.Password
		}
	}
	if primaryPassword == "" && !isPasswordIndependent {
		tm.failTask(taskID, "encryption requires a password")
		return
	}

	passwordCtx := tm.getPasswordContext(ctx, primaryPassword)

	if setter, ok := plugin.(pluginInterfaces.TaskExtraFieldsSetter); ok {
		setter.SetTaskExtraFields(task.ExtraFields)
	}

	if task.SecondaryPassword != "" {
		slog.Debug("task has secondary password (L2) — reserved for future dual-password crypto support",
			"taskId", taskID)
	}

	tm.updateProgress(taskID, 15, "preprocessing", "", "")

	fileSize := info.Size()
	stopMonitor := make(chan struct{})
	go tm.monitorFileProgress(taskID, outputDir, fileSize, stopMonitor)

	err = plugins.EncryptFileWithPlugin(passwordCtx, plugin, absPath, tm.servingDir, outputDir)
	close(stopMonitor)

	if err != nil {
		tm.cleanupTempFiles(outputDir)
		if ctx.Err() != nil || task.Status == "cancelling" {
			tm.failTask(taskID, "task cancelled")
		} else {
			tm.failTask(taskID, fmt.Sprintf("encryption failed: %v", err))
		}
		return
	}

	tm.mu.Lock()
	if task.Status != "cancelling" {
		task.Status = "completed"
		task.Progress = 100
		task.Phase = "completed"
		task.Speed = ""
		task.Eta = ""
		now := time.Now()
		task.CompletedAt = &now

		sourceBaseName := filepath.Base(absPath)
		ext := filepath.Ext(sourceBaseName)
		baseNameWithoutExt := strings.TrimSuffix(sourceBaseName, ext)
		if outputFile := findEncryptedOutputFile(outputDir, baseNameWithoutExt); outputFile != "" {
			task.ContainerVersion = detectContainerVersion(outputFile)
		}

		if warnings := video.LastVerifyWarnings(); len(warnings) > 0 {
			task.Warning = fmt.Sprintf("%d verification warning(s)", len(warnings))
			detailBytes, _ := json.Marshal(warnings)
			task.WarningDetail = string(detailBytes)
		}
	}
	tm.mu.Unlock()

	tm.saveTasks()

	slog.Info("Task completed", "id", task.ID, "type", task.Type)
	if tm.broadcaster != nil {
		tm.broadcaster.Broadcast("task:completed", map[string]interface{}{
			"id":     task.ID,
			"status": "completed",
		})
		tm.broadcaster.Broadcast("file:change", map[string]interface{}{
			"path": task.SourcePath,
		})
	}
}

func (tm *TaskManager) monitorFileProgress(taskID, outputDir string, totalSize int64, stopCh chan struct{}) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	startTime := time.Now()
	var lastSize int64
	var lastTime time.Time
	estimatedTotal := totalSize * 2

	for {
		select {
		case <-ticker.C:
			var currentSize int64
			filepath.WalkDir(outputDir, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				if info, err := d.Info(); err == nil {
					name := info.Name()
					if strings.HasPrefix(name, "encv-pre-") || strings.HasSuffix(name, ".tmp") {
						return nil
					}
					currentSize += info.Size()
				}
				return nil
			})

			now := time.Now()
			elapsed := now.Sub(startTime).Seconds()
			if elapsed <= 0 || currentSize <= 0 {
				continue
			}

			avgSpeed := float64(currentSize) / elapsed
			var instantSpeed float64
			if !lastTime.IsZero() && currentSize > lastSize {
				dt := now.Sub(lastTime).Seconds()
				if dt > 0 {
					instantSpeed = float64(currentSize-lastSize) / dt
				}
			}
			lastSize = currentSize
			lastTime = now

			speed := instantSpeed
			if speed <= 0 {
				speed = avgSpeed
			}

			if currentSize > estimatedTotal/2 && estimatedTotal < currentSize*3 {
				estimatedTotal = currentSize * 2
			}

			rawProgress := float64(currentSize) / float64(estimatedTotal)
			if rawProgress > 0.95 {
				rawProgress = 0.95
			}
			progress := int(rawProgress*80) + 15
			if progress > 95 {
				progress = 95
			}

			speedStr := formatSpeed(speed)
			remaining := float64(estimatedTotal-currentSize) / speed
			if remaining < 0 {
				remaining = 0
			}
			etaStr := formatDuration(remaining)

			tm.mu.RLock()
			task, ok := tm.tasks[taskID]
			currentPhase := ""
			if ok {
				currentPhase = task.Phase
			}
			tm.mu.RUnlock()

			phase := "encrypting"
			if currentPhase == "preprocessing" {
				phase = "preprocessing"
			}

			tm.updateProgress(taskID, progress, phase, speedStr, etaStr)

		case <-stopCh:
			return
		}
	}
}

func (tm *TaskManager) processDecrypt(task *MobileTask, absPath string) {
	taskID := task.ID

	ctx, cancel := context.WithCancel(context.Background())
	task.cancelFn = cancel
	defer cancel()

	tm.updateProgress(taskID, 5, "analyzing", "", "")

	task.ContainerVersion = detectContainerVersion(absPath)

	info, err := os.Stat(absPath)
	if err != nil {
		tm.failTask(taskID, fmt.Sprintf("source file not found: %v", err))
		return
	}

	if info.IsDir() {
		tm.failTask(taskID, "directory decryption is not supported yet")
		return
	}

	outputDir := filepath.Dir(absPath)
	if task.TargetPath != "" {
		targetAbs := tm.resolveAbsPath(task.TargetPath)
		if targetAbs != "" {
			outputDir = targetAbs
			if mkdirErr := os.MkdirAll(outputDir, 0755); mkdirErr != nil {
				tm.failTask(taskID, fmt.Sprintf("failed to create target directory: %v", mkdirErr))
				return
			}
		}
	}

	tm.updateProgress(taskID, 10, "initializing", "", "")

	plugin, err := plugins.FindDecryptingPlugin(absPath)
	if err != nil {
		tm.failTask(taskID, fmt.Sprintf("no decrypting plugin found: %v", err))
		return
	}
	task.PluginName = plugin.Name()

	var primaryPassword string
	if resolver, ok := plugin.(pluginInterfaces.TaskPasswordResolver); ok {
		primaryPassword = resolver.ResolveTaskPassword(task.Password, task.ExtraFields)
	} else {
		primaryPassword = task.Password
		if primaryPassword == "" {
			primaryPassword = tm.cfg.Password
		}
	}

	passwordCtx := tm.getPasswordContext(ctx, primaryPassword)

	if setter, ok := plugin.(pluginInterfaces.TaskExtraFieldsSetter); ok {
		setter.SetTaskExtraFields(task.ExtraFields)
	}

	if task.SecondaryPassword != "" {
		slog.Debug("task has secondary password (L2) — reserved for future dual-password crypto support",
			"taskId", taskID, "context", "decrypt")
	}

	tm.updateProgress(taskID, 15, "preprocessing", "", "")

	fileSize := info.Size()
	stopMonitor := make(chan struct{})
	go tm.monitorFileProgress(taskID, outputDir, fileSize, stopMonitor)

	err = plugins.DecryptContainerWithPlugin(passwordCtx, plugin, absPath, outputDir)
	close(stopMonitor)

	if err != nil {
		tm.cleanupTempFiles(outputDir)
		if ctx.Err() != nil || task.Status == "cancelling" {
			tm.failTask(taskID, "task cancelled")
		} else {
			tm.failTask(taskID, fmt.Sprintf("decryption failed: %v", err))
		}
		return
	}

	tm.mu.Lock()
	if task.Status != "cancelling" {
		task.Status = "completed"
		task.Progress = 100
		task.Phase = "completed"
		task.Speed = ""
		task.Eta = ""
		now := time.Now()
		task.CompletedAt = &now
	}
	tm.mu.Unlock()

	tm.saveTasks()

	slog.Info("Task completed", "id", task.ID, "type", task.Type)
	if tm.broadcaster != nil {
		tm.broadcaster.Broadcast("task:completed", map[string]interface{}{
			"id":     task.ID,
			"status": "completed",
		})
		tm.broadcaster.Broadcast("file:change", map[string]interface{}{
			"path": task.SourcePath,
		})
	}
}

func (tm *TaskManager) failTask(id, errMsg string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	friendlyMsg := simplifyErrorMessage(errMsg)

	if task, ok := tm.tasks[id]; ok {
		task.Status = "failed"
		task.Error = friendlyMsg
		task.ErrorDetail = errMsg
		now := time.Now()
		task.CompletedAt = &now
		slog.Error("Task failed", "id", id, "error", errMsg)
		if tm.broadcaster != nil {
			tm.broadcaster.Broadcast("task:completed", map[string]interface{}{
			"id":          id,
			"status":      "failed",
			"error":       friendlyMsg,
			"errorDetail": errMsg,
		})
			tm.broadcaster.Broadcast("log", map[string]interface{}{
				"level":   "error",
				"message": fmt.Sprintf("[Task %s] %s", id, errMsg),
			})
		}
	}
}

func simplifyErrorMessage(errMsg string) string {
	if strings.Contains(errMsg, "ENGINE_LOAD_FAILED") || strings.Contains(errMsg, "ENGINE_SYMBOL_MISSING") {
		return "video engine unavailable, please reinstall the app"
	}
	if strings.Contains(errMsg, "video engine unavailable") {
		return errMsg
	}
	if strings.Contains(errMsg, "cannot access file") {
		return errMsg
	}
	if strings.Contains(errMsg, "No such file") || strings.Contains(errMsg, "source file not found") {
		return "source file not found, it may have been moved or deleted"
	}
	if strings.Contains(errMsg, "Permission denied") {
		return "permission denied, cannot access the file"
	}
	if strings.Contains(errMsg, "ffprobe failed") {
		return "failed to read video metadata"
	}
	if strings.Contains(errMsg, "ffmpeg failed") {
		return "video encoding failed"
	}
	if strings.Contains(errMsg, "encryption failed") || strings.Contains(errMsg, "plugin failed") {
		return "encryption processing failed"
	}
	if len(errMsg) > 120 {
		return errMsg[:120] + "..."
	}
	return errMsg
}

func getAvailableDiskSpace(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}

func (tm *TaskManager) cleanupTempFiles(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "encv-pre-") {
			os.Remove(filepath.Join(dir, name))
		}
	}
}

func detectContainerVersion(filePath string) int {
	file, err := os.Open(filePath)
	if err != nil {
		return 0
	}
	defer file.Close()

	version, _, err := types.DetectHeaderInfoFromReaderAt(file)
	if err != nil {
		return 0
	}
	return version
}

func findEncryptedOutputFile(outputDir string, sourceBaseName string) string {
	extensions := []string{".sccgt", ".sccgv", ".sccgi", ".sccga", ".sccgpdf", ".sccgwps"}
	for _, ext := range extensions {
		candidate := filepath.Join(outputDir, sourceBaseName+ext)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, sourceBaseName) && plugins.IsContainer(name) {
			return filepath.Join(outputDir, name)
		}
	}
	return ""
}
