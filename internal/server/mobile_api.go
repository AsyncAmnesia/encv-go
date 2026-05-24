package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	mobileservice "github.com/Soltus/encv-go/internal/service"
	"github.com/Soltus/encv-go/internal/v2/types"
)

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

func writeServiceErrorGin(c *gin.Context, err error) {
	switch err.(type) {
	case *mobileservice.PermissionError:
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case *mobileservice.ForbiddenError:
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case *mobileservice.NotFoundError:
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case *mobileservice.BadRequestError:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case *mobileservice.UnsupportedMediaTypeError:
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (s *Server) handlePingGin(c *gin.Context) {
	c.JSON(http.StatusOK, types.PingResponse{
		Status:        types.ServiceStatuses.OK,
		Version:       s.version,
		InstanceID:    s.instanceID,
		ServerDirPath: s.servingDir,
		WebdavDirPath: s.webdavDir,
	})
}

func (s *Server) handleHealthGin(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) handleServerShutdownGin(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "shutting_down"})
	go func() {
		slog.Info("Shutdown requested via API")
		if s.server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.server.Shutdown(ctx); err != nil {
				slog.Error("Server shutdown error", "error", err)
			}
		}
		s.readerService.Cleanup()
		os.Exit(0)
	}()
}

func (s *Server) handleListFilesGin(c *gin.Context) {
	queryPath := c.Query("path")
	slog.Info("API: list files", "path", queryPath)

	files, err := s.mobileSvc.ListFiles(queryPath)
	if err != nil {
		writeServiceErrorGin(c, err)
		return
	}

	slog.Info("API: list files result", "path", queryPath, "count", len(files))
	c.JSON(http.StatusOK, gin.H{"files": files})
}

func (s *Server) handleDeleteFileGin(c *gin.Context) {
	queryPath := c.Query("path")
	slog.Warn("API: delete file", "path", queryPath)

	err := s.mobileSvc.DeleteFile(queryPath)
	if err != nil {
		writeServiceErrorGin(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (s *Server) handleReadFileContentGin(c *gin.Context) {
	queryPath := c.Query("path")
	slog.Info("API: read file content", "path", queryPath)

	result, err := s.mobileSvc.ReadFileContent(queryPath)
	if err != nil {
		writeServiceErrorGin(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Server) handleGetTasksGin(c *gin.Context) {
	taskList := s.mobileSvc.GetTaskManager().List()
	c.JSON(http.StatusOK, gin.H{"tasks": taskList})
}

func (s *Server) handleCreateTaskGin(c *gin.Context) {
	var req struct {
		Type       string `json:"type"`
		SourcePath string `json:"sourcePath"`
		TargetPath string `json:"targetPath,omitempty"`
		Password   string `json:"password,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	slog.Info("API: create task", "type", req.Type, "source", req.SourcePath, "target", req.TargetPath)
	task := s.mobileSvc.GetTaskManager().Create(req.Type, req.SourcePath, req.TargetPath, req.Password)

	c.JSON(http.StatusCreated, task)
}

func (s *Server) handleCancelTaskGin(c *gin.Context) {
	id := c.Param("id")

	task, err := s.mobileSvc.GetTaskManager().Cancel(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (s *Server) handleRetryTaskGin(c *gin.Context) {
	id := c.Param("id")

	task, err := s.mobileSvc.GetTaskManager().Retry(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (s *Server) handleTestWebDAVGin(c *gin.Context) {
	var req struct {
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	err := s.mobileSvc.TestWebDAV(req.URL, req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handlePermissionsGin(c *gin.Context) {
	storage := s.mobileSvc.CheckStoragePermission()
	slog.Info("API: check permissions", "storage", storage)
	c.JSON(http.StatusOK, gin.H{"storage": storage})
}

func (s *Server) canUseWebdavIndex() bool {
	return s.webdavFS != nil && s.webdavFS.Dir() == s.servingDir
}

func (s *Server) handleSearchFilesGin(c *gin.Context) {
	queryPath := c.Query("path")
	keyword := c.Query("keyword")
	recursive := c.Query("recursive") == "true"

	slog.Info("API: search files", "path", queryPath, "keyword", keyword, "recursive", recursive)

	if recursive && keyword != "" {
		var files []mobileservice.FileInfo

		mobileFiles, err := s.mobileSvc.SearchFiles(queryPath, keyword, true)
		if err != nil {
			writeServiceErrorGin(c, err)
			return
		}

		if s.canUseWebdavIndex() {
			for _, f := range mobileFiles {
				if !s.webdavFS.IsContainerExtension(f.Name) {
					files = append(files, f)
				}
			}
			entries := s.webdavFS.SearchInIndex(keyword, queryPath, 200)
			for _, e := range entries {
				files = append(files, mobileservice.FileInfo{
					Name:        e.Name,
					Path:        e.Path,
					IsDirectory: e.IsDir,
					Size:        e.Size,
				})
			}
		} else {
			files = mobileFiles
		}

		slog.Info("API: search files result", "path", queryPath, "keyword", keyword, "count", len(files))
		c.JSON(http.StatusOK, gin.H{"files": files})
		return
	}

	files, err := s.mobileSvc.SearchFiles(queryPath, keyword, recursive)
	if err != nil {
		writeServiceErrorGin(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"files": files})
}

func (s *Server) handleIndexStatsGin(c *gin.Context) {
	stats := s.mobileSvc.GetIndexStats()
	if stats.TotalFiles == 0 && !stats.IsIndexing {
		s.mobileSvc.RebuildIndex()
		stats = s.mobileSvc.GetIndexStats()
	}
	stats.Source = "mobile"

	if s.canUseWebdavIndex() {
		wdStats := s.webdavFS.GetIndexStats()
		ordinaryFiles := stats.TotalFiles
		containerPhysicalCount := 0
		if ordinaryFiles > 0 && wdStats.Containers > 0 {
			containerPhysicalCount = wdStats.Containers
		}
		stats.TotalFiles = ordinaryFiles - containerPhysicalCount + wdStats.TotalFiles
		if stats.TotalFiles < 0 {
			stats.TotalFiles = 0
		}
		stats.Containers = wdStats.Containers
		stats.Source = "webdav"
	}

	c.JSON(http.StatusOK, stats)
}

func (s *Server) handleIndexRebuildGin(c *gin.Context) {
	s.mobileSvc.RebuildIndex()
	source := "mobile"
	if s.canUseWebdavIndex() {
		source = "webdav"
	}
	c.JSON(http.StatusOK, gin.H{"status": "indexing", "source": source})
}

func (s *Server) handleIndexClearGin(c *gin.Context) {
	s.mobileSvc.ClearIndex()
	c.JSON(http.StatusOK, gin.H{"status": "cleared"})
}

func (s *Server) handleStreamExternalFileGin(c *gin.Context) {
	queryPath := c.Query("path")
	decodedPath, err := url.QueryUnescape(queryPath)
	if err != nil {
		decodedPath = queryPath
	}

	slog.Info("API: stream external file", "path", decodedPath)

	err = s.mobileSvc.StreamExternalFile(c.Writer, c.Request, decodedPath)
	if err != nil {
		writeServiceErrorGin(c, err)
		return
	}
}

func (s *Server) handleRemoteInfoGin(c *gin.Context) {
	cfg := s.cfg
	port := s.actualPort
	if port <= 0 {
		port = cfg.Server.Port
	}

	webdavInfo := gin.H{
		"enabled":  cfg.Webdav.Root != "",
		"username": cfg.Webdav.Username,
		"root":     cfg.Webdav.Root,
	}
	if cfg.Webdav.Root != "" {
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
		webdavInfo["url"] = fmt.Sprintf("http://127.0.0.1:%d%s", port, root)
	} else {
		webdavInfo["url"] = ""
	}

	openlistSites := make(map[string]interface{})
	for siteId, siteCfg := range cfg.Proxy.Sites {
		proxyURL := fmt.Sprintf("http://127.0.0.1:%d/openlist/sites/%s/", port, siteId)
		openlistSites[siteId] = map[string]string{
			"host":        siteCfg.Host,
			"description": siteCfg.Description,
			"proxyUrl":    proxyURL,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"webdav":        webdavInfo,
		"openlistSites": openlistSites,
	})
}

func (s *Server) handleListOpenlistSitesGin(c *gin.Context) {
	sites := make(map[string]interface{})
	for siteId, siteCfg := range s.cfg.Proxy.Sites {
		sites[siteId] = map[string]string{
			"host":        siteCfg.Host,
			"description": siteCfg.Description,
		}
	}
	c.JSON(http.StatusOK, gin.H{"sites": sites})
}

func (s *Server) handleAddOpenlistSiteGin(c *gin.Context) {
	var req struct {
		SiteID      string `json:"siteId"`
		Host        string `json:"host"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if !isValidSiteID(req.SiteID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "site id must contain only letters, digits and underscores"})
		return
	}
	if req.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host is required"})
		return
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()

	if _, exists := s.cfg.Proxy.Sites[req.SiteID]; exists {
		c.JSON(http.StatusConflict, gin.H{"error": "site id already exists"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "site added"})
}

func (s *Server) handleUpdateOpenlistSiteGin(c *gin.Context) {
	siteId := c.Param("id")

	if !isValidSiteID(siteId) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "site id must contain only letters, digits and underscores"})
		return
	}

	var req struct {
		Host        string `json:"host"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()

	if _, exists := s.cfg.Proxy.Sites[siteId]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}

	s.cfg.Proxy.Sites[siteId] = types.ProxySiteConfig{
		Host:        req.Host,
		Description: req.Description,
	}

	if err := s.writeConfigToFile(); err != nil {
		slog.Error("Failed to write config after updating openlist site", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "site updated"})
}

func (s *Server) handleDeleteOpenlistSiteGin(c *gin.Context) {
	siteId := c.Param("id")

	if !isValidSiteID(siteId) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "site id must contain only letters, digits and underscores"})
		return
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()

	if _, exists := s.cfg.Proxy.Sites[siteId]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}

	delete(s.cfg.Proxy.Sites, siteId)

	if err := s.writeConfigToFile(); err != nil {
		slog.Error("Failed to write config after deleting openlist site", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "site deleted"})
}

func (s *Server) handleFileExistsGin(c *gin.Context) {
	queryPath := c.Query("path")
	exists, err := s.mobileSvc.FileExists(queryPath)
	if err != nil {
		writeServiceErrorGin(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"exists": exists})
}

func (s *Server) handleEncryptOutputExistsGin(c *gin.Context) {
	sourcePath := c.Query("sourcePath")
	targetDir := c.Query("targetDir")
	if sourcePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sourcePath is required"})
		return
	}
	exists, outputPath, err := s.mobileSvc.CheckEncryptOutputExists(sourcePath, targetDir)
	if err != nil {
		writeServiceErrorGin(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"exists": exists, "outputPath": outputPath})
}

func (s *Server) handleAPILogsGin(c *gin.Context) {
	var req struct {
		Level     string `json:"level"`
		Message   string `json:"message"`
		Tag       string `json:"tag,omitempty"`
		Timestamp int64  `json:"timestamp,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	msg := req.Message
	if req.Tag != "" {
		msg = "[" + req.Tag + "] " + msg
	}
	switch req.Level {
	case "error":
		slog.Error(msg)
	case "warn":
		slog.Warn(msg)
	case "debug":
		slog.Debug(msg)
	default:
		slog.Info(msg)
	}
	c.Status(http.StatusNoContent)
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
