package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Soltus/encv-go/internal/utils"
	"github.com/google/uuid"
)

var (
	tasksMu sync.RWMutex
	tasks   = map[string]*MobileTask{}
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

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleListFilesAPI(w http.ResponseWriter, r *http.Request) {
	queryPath := r.URL.Query().Get("path")
	if queryPath == "" {
		queryPath = "/"
	}

	absPath, err := utils.SafeURLToAbsPath(s.servingDir, queryPath)
	if err != nil {
		slog.Error("SafeURLToAbsPath failed", "path", queryPath, "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "directory not found"})
			return
		}
		slog.Error("ReadDir failed", "path", absPath, "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to read directory"})
		return
	}

	type FileInfo struct {
		Name       string `json:"name"`
		Path       string `json:"path"`
		IsDirectory bool  `json:"isDirectory"`
		Size       int64  `json:"size"`
		Modified   string `json:"modified"`
	}

	var files []FileInfo
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			slog.Error("Failed to get file info", "name", entry.Name(), "error", err)
			continue
		}

		filePath := queryPath + "/" + entry.Name()
		if queryPath == "/" {
			filePath = "/" + entry.Name()
		}

		files = append(files, FileInfo{
			Name:        entry.Name(),
			Path:        filePath,
			IsDirectory: entry.IsDir(),
			Size:        info.Size(),
			Modified:    info.ModTime().Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"files": files})
}

func (s *Server) handleDeleteFileAPI(w http.ResponseWriter, r *http.Request) {
	queryPath := r.URL.Query().Get("path")
	if queryPath == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "'path' query parameter is required"})
		return
	}

	absPath, err := utils.SafeURLToAbsPath(s.servingDir, queryPath)
	if err != nil {
		slog.Error("SafeURLToAbsPath failed", "path", queryPath, "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	err = os.Remove(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "file not found"})
			return
		}
		slog.Error("Remove failed", "path", absPath, "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to delete file"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "deleted"})
}

func (s *Server) handleGetTasks(w http.ResponseWriter, r *http.Request) {
	tasksMu.RLock()
	defer tasksMu.RUnlock()

	taskList := make([]*MobileTask, 0, len(tasks))
	for _, t := range tasks {
		taskList = append(taskList, t)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"tasks": taskList})
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to read request body"})
		return
	}
	defer r.Body.Close()

	var req struct {
		Type       string `json:"type"`
		SourcePath string `json:"sourcePath"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}

	task := &MobileTask{
		ID:         uuid.New().String(),
		Type:       req.Type,
		SourcePath: req.SourcePath,
		Status:     "queued",
		Progress:   0,
		CreatedAt:  time.Now(),
	}

	tasksMu.Lock()
	tasks[task.ID] = task
	tasksMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	id = strings.TrimSuffix(id, "/cancel")

	tasksMu.Lock()
	defer tasksMu.Unlock()

	task, ok := tasks[id]
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "task not found"})
		return
	}

	task.Status = "cancelled"

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(task)
}

func (s *Server) handleRetryTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	id = strings.TrimSuffix(id, "/retry")

	tasksMu.Lock()
	defer tasksMu.Unlock()

	task, ok := tasks[id]
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "task not found"})
		return
	}

	task.Status = "queued"
	task.Error = ""
	task.Progress = 0

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(task)
}

func (s *Server) handleTestWebDAV(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleTestWebDAVPost(w, r)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleTestWebDAVPost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to read request body"})
		return
	}
	defer r.Body.Close()

	var req struct {
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	httpReq, err := http.NewRequest(http.MethodGet, req.URL, nil)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "invalid URL"})
		return
	}

	if req.Username != "" || req.Password != "" {
		httpReq.SetBasicAuth(req.Username, req.Password)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}
	resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

func (s *Server) handleMobileFiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListFilesAPI(w, r)
	case http.MethodDelete:
		s.handleDeleteFileAPI(w, r)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleMobileTasks(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case path == "/api/tasks" && r.Method == http.MethodGet:
		s.handleGetTasks(w, r)
	case path == "/api/tasks" && r.Method == http.MethodPost:
		s.handleCreateTask(w, r)
	case strings.HasSuffix(path, "/cancel") && r.Method == http.MethodPost:
		s.handleCancelTask(w, r)
	case strings.HasSuffix(path, "/retry") && r.Method == http.MethodPost:
		s.handleRetryTask(w, r)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
	}
}
