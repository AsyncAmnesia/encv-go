package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Soltus/encv-go/internal/service"
	"github.com/Soltus/encv-go/internal/v2/types"
)

func writeServiceError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	switch err.(type) {
	case *service.PermissionError:
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error(), "code": "PERMISSION_DENIED"})
	case *service.ForbiddenError:
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
	case *service.NotFoundError:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
	case *service.BadRequestError:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
	case *service.UnsupportedMediaTypeError:
		w.WriteHeader(http.StatusUnsupportedMediaType)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
	default:
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleServerShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "shutting_down"})
	go func() {
		slog.Info("Shutdown requested via API")
		if s.server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			s.server.Shutdown(ctx)
		}
		os.Exit(0)
	}()
}

func (s *Server) handleListFilesAPI(w http.ResponseWriter, r *http.Request) {
	queryPath := r.URL.Query().Get("path")
	slog.Info("API: list files", "path", queryPath)

	files, err := s.mobileSvc.ListFiles(queryPath)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	slog.Info("API: list files result", "path", queryPath, "count", len(files))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"files": files})
}

func (s *Server) handleDeleteFileAPI(w http.ResponseWriter, r *http.Request) {
	queryPath := r.URL.Query().Get("path")
	slog.Warn("API: delete file", "path", queryPath)

	err := s.mobileSvc.DeleteFile(queryPath)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "deleted"})
}

func (s *Server) handleReadFileContent(w http.ResponseWriter, r *http.Request) {
	queryPath := r.URL.Query().Get("path")
	slog.Info("API: read file content", "path", queryPath)

	result, err := s.mobileSvc.ReadFileContent(queryPath)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleGetTasks(w http.ResponseWriter, r *http.Request) {
	taskList := s.mobileSvc.GetTaskManager().List()

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
		TargetPath string `json:"targetPath,omitempty"`
		Password   string `json:"password,omitempty"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}

	slog.Info("API: create task", "type", req.Type, "source", req.SourcePath, "target", req.TargetPath)
	task := s.mobileSvc.GetTaskManager().Create(req.Type, req.SourcePath, req.TargetPath, req.Password)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	id = strings.TrimSuffix(id, "/cancel")

	task, err := s.mobileSvc.GetTaskManager().Cancel(id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(task)
}

func (s *Server) handleRetryTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	id = strings.TrimSuffix(id, "/retry")

	task, err := s.mobileSvc.GetTaskManager().Retry(id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(task)
}

func (s *Server) handleTestWebDAV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

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

	err = s.mobileSvc.TestWebDAV(req.URL, req.Username, req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

func (s *Server) handlePermissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	storage := s.mobileSvc.CheckStoragePermission()
	slog.Info("API: check permissions", "storage", storage)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{
		"storage": storage,
	})
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

func (s *Server) handleSearchFilesAPI(w http.ResponseWriter, r *http.Request) {
	queryPath := r.URL.Query().Get("path")
	keyword := r.URL.Query().Get("keyword")
	recursive := r.URL.Query().Get("recursive") == "true"

	slog.Info("API: search files", "path", queryPath, "keyword", keyword, "recursive", recursive)

	files, err := s.mobileSvc.SearchFiles(queryPath, keyword, recursive)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"files": files})
}

func (s *Server) handleIndexStats(w http.ResponseWriter, r *http.Request) {
	stats := s.mobileSvc.GetIndexStats()
	if stats.TotalFiles == 0 && !stats.IsIndexing {
		s.mobileSvc.RebuildIndex()
		stats = s.mobileSvc.GetIndexStats()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleIndexRebuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}
	s.mobileSvc.RebuildIndex()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "indexing"})
}

func (s *Server) handleIndexClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}
	s.mobileSvc.ClearIndex()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}

func (s *Server) handleStreamExternalFile(w http.ResponseWriter, r *http.Request) {
	queryPath := r.URL.Query().Get("path")
	decodedPath, err := url.QueryUnescape(queryPath)
	if err != nil {
		decodedPath = queryPath
	}

	slog.Info("API: stream external file", "path", decodedPath)

	err = s.mobileSvc.StreamExternalFile(w, r, decodedPath)
	if err != nil {
		writeServiceError(w, err)
		return
	}
}

func (s *Server) handleRemoteInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	cfg := s.cfg

	webdavInfo := map[string]interface{}{
		"enabled":  cfg.Webdav.Port > 0,
		"username": cfg.Webdav.Username,
		"root":     cfg.Webdav.Root,
	}
	if cfg.Webdav.Port > 0 {
		root := cfg.Webdav.Root
		if root == "" {
			root = "/webdav/"
		}
		if !strings.HasPrefix(root, "/") {
			root = "/" + root
		}
		if !strings.HasSuffix(root, "/") {
			root += "/"
		}
		webdavInfo["url"] = fmt.Sprintf("http://127.0.0.1:%d%s", cfg.Webdav.Port, root)
	} else {
		webdavInfo["url"] = ""
	}

	openlistSites := make(map[string]interface{})
	for siteId, siteCfg := range cfg.Proxy.Sites {
		proxyURL := fmt.Sprintf("http://127.0.0.1:%d/openlist/sites/%s/", cfg.Server.Port, siteId)
		openlistSites[siteId] = map[string]string{
			"host":        siteCfg.Host,
			"description": siteCfg.Description,
			"proxyUrl":    proxyURL,
		}
	}

	response := map[string]interface{}{
		"webdav":        webdavInfo,
		"openlistSites": openlistSites,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleOpenlistSites(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleAddOpenlistSite(w, r)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleOpenlistSiteByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/remote/openlist/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "site id required"})
		return
	}
	siteId := parts[0]

	if !isValidSiteID(siteId) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "site id must contain only letters, digits and underscores"})
		return
	}

	switch r.Method {
	case http.MethodPut:
		s.handleUpdateOpenlistSite(w, r, siteId)
	case http.MethodDelete:
		s.handleDeleteOpenlistSite(w, r, siteId)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
	}
}

func isValidSiteID(id string) bool {
	if id == "" {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func (s *Server) handleAddOpenlistSite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID      string `json:"siteId"`
		Host        string `json:"host"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	if !isValidSiteID(req.SiteID) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "site id must contain only letters, digits and underscores"})
		return
	}
	if req.Host == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "host is required"})
		return
	}

	configMu.Lock()
	defer configMu.Unlock()

	if _, exists := s.cfg.Proxy.Sites[req.SiteID]; exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "site id already exists"})
		return
	}

	if s.cfg.Proxy.Sites == nil {
		s.cfg.Proxy.Sites = make(map[string]types.ProxySiteConfig)
	}
	s.cfg.Proxy.Sites[req.SiteID] = types.ProxySiteConfig{
		Host:        req.Host,
		Description: req.Description,
	}

	if err := s.writeConfigToFile(); err != nil {
		slog.Error("Failed to write config after adding openlist site", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to save config"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "site added"})
}

func (s *Server) handleUpdateOpenlistSite(w http.ResponseWriter, r *http.Request, siteId string) {
	var req struct {
		Host        string `json:"host"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	configMu.Lock()
	defer configMu.Unlock()

	if _, exists := s.cfg.Proxy.Sites[siteId]; !exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "site not found"})
		return
	}

	s.cfg.Proxy.Sites[siteId] = types.ProxySiteConfig{
		Host:        req.Host,
		Description: req.Description,
	}

	if err := s.writeConfigToFile(); err != nil {
		slog.Error("Failed to write config after updating openlist site", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to save config"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "site updated"})
}

func (s *Server) handleDeleteOpenlistSite(w http.ResponseWriter, r *http.Request, siteId string) {
	configMu.Lock()
	defer configMu.Unlock()

	if _, exists := s.cfg.Proxy.Sites[siteId]; !exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "site not found"})
		return
	}

	delete(s.cfg.Proxy.Sites, siteId)

	if err := s.writeConfigToFile(); err != nil {
		slog.Error("Failed to write config after deleting openlist site", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to save config"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "site deleted"})
}

func (s *Server) handleFileExistsAPI(w http.ResponseWriter, r *http.Request) {
	queryPath := r.URL.Query().Get("path")
	exists, err := s.mobileSvc.FileExists(queryPath)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"exists": exists})
}

func (s *Server) writeConfigToFile() error {
	if s.configPath == "" {
		return fmt.Errorf("config path not available")
	}
	indented, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(s.configPath, append(indented, '\n'), 0644)
}
