package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/Soltus/encv-go/internal/config"
	mobileservice "github.com/Soltus/encv-go/internal/service"
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/container/detector"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	pluginInterfaces "github.com/Soltus/encv-go/internal/v2/plugins/interfaces"
	alistencryptplugin "github.com/Soltus/encv-go/internal/v2/plugins/alistencrypt"
	"github.com/Soltus/encv-go/internal/alistencrypt"
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

	if tag := c.Query("tag"); tag != "" {
		taggedPaths := GlobalTagStore.GetFilesByTag(tag)
		taggedSet := make(map[string]bool, len(taggedPaths))
		for _, p := range taggedPaths {
			taggedSet[p] = true
		}
		filtered := make([]mobileservice.FileInfo, 0, len(files))
		for _, f := range files {
			if taggedSet[f.Path] {
				filtered = append(filtered, f)
			}
		}
		files = filtered
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

func (s *Server) handleCreateDirectoryGin(c *gin.Context) {
	var req struct {
		ParentPath string `json:"parent_path"`
		Name       string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	slog.Info("API: create directory", "parent_path", req.ParentPath, "name", req.Name)

	err := s.mobileSvc.CreateDirectory(req.ParentPath, req.Name)
	if err != nil {
		writeServiceErrorGin(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "created"})
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

func (s *Server) handleTextPreviewExtsGin(c *gin.Context) {
	builtIn := config.GetTextPreviewExtensions()
	var custom []string
	if s.cfg.Preview != nil {
		custom = s.cfg.Preview.TextExtensions
	}
	c.JSON(http.StatusOK, gin.H{
		"extensions":        builtIn,
		"custom_extensions": custom,
	})
}

func (s *Server) handleFileInfoGin(c *gin.Context) {
	queryPath := c.Query("path")
	if queryPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'path' query parameter is required"})
		return
	}

	result, err := s.mobileSvc.GetFileInfo(queryPath)
	if err != nil {
		if _, ok := err.(*mobileservice.NotFoundError); ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleRenameFileGin(c *gin.Context) {
	var req mobileservice.RenameFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	slog.Info("API: rename file original_name", "path", req.Path, "newName", req.NewName,
		"hasPassword", req.Password != "")

	result, err := s.mobileSvc.RenameFile(&req)
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
		Type             string            `json:"type"`
		SourcePath       string            `json:"sourcePath"`
		TargetPath       string            `json:"targetPath,omitempty"`
		Password         string            `json:"password,omitempty"`
		SecondaryPassword string           `json:"secondaryPassword,omitempty"`
		Version          int               `json:"version,omitempty"`
		ExtraFields      map[string]string `json:"extraFields,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	slog.Info("API: create task", "type", req.Type, "source", req.SourcePath,
		"target", req.TargetPath, "version", req.Version,
		"hasPassword", req.Password != "",
		"hasSecondaryPassword", req.SecondaryPassword != "",
		"hasExtraFields", len(req.ExtraFields) > 0)
	task := s.mobileSvc.GetTaskManager().CreateWithExtras(
		req.Type, req.SourcePath, req.TargetPath,
		req.Password, req.SecondaryPassword, req.Version, req.ExtraFields,
	)

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

func (s *Server) handleRemoveTaskGin(c *gin.Context) {
	id := c.Param("id")

	if err := s.mobileSvc.GetTaskManager().RemoveTask(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleTestWebDAVGin(c *gin.Context) {
	var req struct {
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid JSON"})
		return
	}

	result, err := s.mobileSvc.TestWebDAV(req.URL, req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
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

func (s *Server) handleTestLocalWebDAVGin(c *gin.Context) {
	if s.webdavFS == nil {
		c.JSON(http.StatusOK, gin.H{
			"available": false,
			"error":     "WebDAV not enabled",
		})
		return
	}

	result := gin.H{
		"available":    true,
		"url":          fmt.Sprintf("http://127.0.0.1:%d%s", s.cfg.Server.Port, s.webdavPath),
		"authRequired": s.cfg.Webdav.Username != "" || s.cfg.Webdav.Password != "",
		"details": gin.H{
			"propfindRoot": "fail",
			"authWorks":    "skip",
			"dirReadable":  "fail",
		},
	}

	webdavURL := fmt.Sprintf("http://127.0.0.1:%d%s", s.cfg.Server.Port, s.webdavPath)
	details := gin.H{
		"propfindRoot": "fail",
		"authWorks":    "skip",
		"dirReadable":  "fail",
	}

	propfindBody := `<?xml version="1.0" encoding="UTF-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/></d:prop></d:propfind>`
	req, err := http.NewRequest("PROPFIND", webdavURL, bytes.NewBufferString(propfindBody))
	if err == nil {
		req.Header.Set("Content-Type", "application/xml; charset=utf-8")
		req.Header.Set("Depth", "1")
		if s.cfg.Webdav.Username != "" {
			req.SetBasicAuth(s.cfg.Webdav.Username, s.cfg.Webdav.Password)
		}
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusMultiStatus {
				details["propfindRoot"] = "ok"
				details["dirReadable"] = "ok"
			} else if resp.StatusCode == http.StatusUnauthorized {
				details["propfindRoot"] = "ok"
				details["dirReadable"] = "fail"
			}
		}
	}

	if s.cfg.Webdav.Username != "" {
		details["authWorks"] = "fail"
		req2, err := http.NewRequest("PROPFIND", webdavURL, bytes.NewBufferString(propfindBody))
		if err == nil {
			req2.Header.Set("Content-Type", "application/xml; charset=utf-8")
			req2.Header.Set("Depth", "0")
			req2.SetBasicAuth(s.cfg.Webdav.Username, s.cfg.Webdav.Password)
			client := &http.Client{Timeout: 3 * time.Second}
			resp2, err := client.Do(req2)
			if err == nil {
				defer resp2.Body.Close()
				if resp2.StatusCode == http.StatusMultiStatus {
					details["authWorks"] = "ok"
				}
			}
		}
	}

	result["details"] = details
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleIndexClearGin(c *gin.Context) {
	s.mobileSvc.ClearIndex()
	c.JSON(http.StatusOK, gin.H{"status": "cleared"})
}

func (s *Server) handleStreamExternalFileGin(c *gin.Context) {
	queryPath := c.Query("path")

	slog.Info("API: stream external file", "path", queryPath)

	err := s.mobileSvc.StreamExternalFile(c.Writer, c.Request, queryPath)
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

func (s *Server) handleBuildInfoGin(c *gin.Context) {
	info, err := utils.GetBuildInfo()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	info["app_version"] = s.version
	c.JSON(http.StatusOK, info)
}

func (s *Server) handleGetContainerVersionsGin(c *gin.Context) {
	c.JSON(200, gin.H{
		"versions": []gin.H{
			{"version": 2, "status": "deprecated", "label": "V2 (已弃用)"},
			{"version": 3, "status": "stable", "label": "V3"},
			{"version": 4, "status": "recommended", "label": "V4 (推荐)"},
		},
		"default": s.cfg.GetEffectiveDefaultVersion(),
	})
}

func (s *Server) handleFFmpegStatusGin(c *gin.Context) {
	ffmpegOk, ffprobeOk, errMsg, ffmpegDetail, ffprobeDetail := utils.CheckFFmpegAvailable()
	c.JSON(http.StatusOK, gin.H{
		"ffmpeg_available":   ffmpegOk,
		"ffprobe_available":  ffprobeOk,
		"error":              errMsg,
		"ffmpeg_detail":      ffmpegDetail,
		"ffprobe_detail":     ffprobeDetail,
	})
}

func (s *Server) handleTagsListGin(c *gin.Context) {
	allTags := GlobalTagStore.GetAllTags()
	type tagEntry struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	result := make([]tagEntry, 0, len(allTags))
	for name, count := range allTags {
		result = append(result, tagEntry{Name: name, Count: count})
	}
	c.JSON(http.StatusOK, gin.H{"tags": result})
}

func (s *Server) handleTagsMutateGin(c *gin.Context) {
	var req struct {
		Path   string `json:"path"`
		Tag    string `json:"tag"`
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}
	if req.Path == "" || req.Tag == "" || req.Action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path, tag and action are required"})
		return
	}

	switch req.Action {
	case "add":
		GlobalTagStore.AddTag(req.Path, req.Tag)
		c.JSON(http.StatusOK, gin.H{"message": "tag added"})
	case "remove":
		GlobalTagStore.RemoveTag(req.Path, req.Tag)
		c.JSON(http.StatusOK, gin.H{"message": "tag removed"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be 'add' or 'remove'"})
	}
}

type PluginMeta struct {
	Name                  string   `json:"name"`
	SupportedExtensions   []string `json:"supportedExtensions"`
	SupportedMimePrefixes []string `json:"supportedMimePrefixes"`
	ContainerExtension    string   `json:"containerExtension"`
	TaskOptions           gin.H    `json:"taskOptions"`
}

func (s *Server) handlePluginsGin(c *gin.Context) {
	var metas []PluginMeta
	for _, p := range plugins.Plugins {
		opts := p.GetTaskOptions()
		metas = append(metas, PluginMeta{
			Name:                  p.Name(),
			SupportedExtensions:   p.SupportedExtensions(),
			SupportedMimePrefixes: p.SupportedMimePrefixes(),
			ContainerExtension:    p.GetContainerExtension(),
			TaskOptions:           taskOptionsToGinH(opts),
		})
	}
	c.JSON(200, gin.H{"plugins": metas})
}

func taskOptionsToGinH(opts pluginInterfaces.TaskOptions) gin.H {
	return gin.H{
		"passwordStrategy":     string(opts.PasswordStrategy),
		"supportVersionSelect": opts.SupportVersionSelect,
		"supportedVersions":    opts.SupportedVersions,
		"defaultVersion":       opts.DefaultVersion,
		"extraFields":          opts.ExtraFields,
	}
}

func (s *Server) handlePredictPluginGin(c *gin.Context) {
	var req struct {
		SourcePath string `json:"sourcePath"`
		Type       string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	var candidates []plugins.PluginCandidate
	if req.Type == "encrypt" {
		candidates = plugins.FindAllEncryptingPlugins(req.SourcePath)
	} else {
		targetPlugin, err := plugins.FindDecryptingPlugin(req.SourcePath)
		if err != nil || targetPlugin == nil {
			c.JSON(200, gin.H{"candidates": []gin.H{}, "pluginName": nil, "taskOptions": nil, "error": err.Error()})
			return
		}
		opts := targetPlugin.GetTaskOptions()
		candidates = []plugins.PluginCandidate{{
			Plugin: targetPlugin, Name: targetPlugin.Name(), MatchType: "container", Priority: 0,
		}}
		c.JSON(200, gin.H{
			"candidates": []gin.H{{"name": targetPlugin.Name(), "matchType": "container", "priority": 0, "taskOptions": taskOptionsToGinH(opts)}},
			"pluginName":  targetPlugin.Name(),
			"taskOptions": taskOptionsToGinH(opts),
		})
		return
	}

	candidateList := make([]gin.H, 0, len(candidates))
	for _, cand := range candidates {
		opts := cand.Plugin.GetTaskOptions()
		candidateList = append(candidateList, gin.H{
			"name":        cand.Name,
			"matchType":   cand.MatchType,
			"priority":    cand.Priority,
			"taskOptions": taskOptionsToGinH(opts),
		})
	}

	firstName := ""
	var firstOptsH gin.H
	if len(candidateList) > 0 {
		firstName = candidateList[0]["name"].(string)
		firstOptsH = candidateList[0]["taskOptions"].(gin.H)
	}

	c.JSON(200, gin.H{
		"candidates": candidateList,
		"pluginName":  firstName,
		"taskOptions": firstOptsH,
	})
}

func (s *Server) handleContainerExtensionsGin(c *gin.Context) {
	extMap := plugins.GetContainerExtensionsMap()
	conflicts := plugins.ValidateExtensionUniqueness()

	var conflictList []gin.H
	for _, c := range conflicts {
		conflictList = append(conflictList, gin.H{
			"extension":   c.Extension,
			"pluginNames": c.PluginNames,
		})
	}

	c.JSON(200, gin.H{
		"extensions": extMap,
		"conflicts":  conflictList,
	})
}

func (s *Server) writeSSEEvent(c *gin.Context, flusher http.Flusher, data string) {
	c.Writer.Write([]byte("data: " + data + "\n\n"))
	if flusher != nil {
		flusher.Flush()
	}
}

func (s *Server) handleListFilesStreamGin(c *gin.Context) {
	queryPath := c.Query("path")
	if queryPath == "" {
		queryPath = "/"
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	c.Status(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		flusher = nil
	}

	absPath, err := utils.SafeURLToAbsPath(s.servingDir, queryPath)
	if err != nil {
		s.writeSSEEvent(c, flusher, `{"error":"invalid path"}`)
		s.writeSSEEvent(c, flusher, `[DONE]`)
		return
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		errMsg := fmt.Sprintf(`{"error":"cannot read directory: %s"}`, err.Error())
		s.writeSSEEvent(c, flusher, errMsg)
		s.writeSSEEvent(c, flusher, `[DONE]`)
		return
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		filePath := queryPath + "/" + entry.Name()
		if queryPath == "/" {
			filePath = "/" + entry.Name()
		}

		info, err := entry.Info()
		if err != nil {
			fi := mobileservice.FileInfo{
				Name:        entry.Name(),
				Path:        filePath,
				IsDirectory: entry.IsDir(),
				Size:        0,
				Modified:    "",
			}
			data, _ := json.Marshal(fi)
			s.writeSSEEvent(c, flusher, string(data))
			continue
		}

		isEncrypted := false
		if !entry.IsDir() {
			entryAbsPath := filepath.Join(absPath, entry.Name())
			if _, detectErr := detector.DetectContainer(entryAbsPath); detectErr == nil {
				isEncrypted = true
			}
		}

		fi := mobileservice.FileInfo{
			Name:        entry.Name(),
			Path:        filePath,
			IsDirectory: entry.IsDir(),
			IsEncrypted: isEncrypted,
			Size:        info.Size(),
			Modified:    info.ModTime().Format(time.RFC3339),
		}
		data, _ := json.Marshal(fi)
		s.writeSSEEvent(c, flusher, string(data))
	}

	s.writeSSEEvent(c, flusher, `[DONE]`)
}

func (s *Server) handleAlistEncryptStreamGin(c *gin.Context) {
	queryPath := c.Query("path")
	password := c.Query("password")
	if queryPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'path' query parameter is required"})
		return
	}

	absPath, err := utils.SafeURLToAbsPath(s.servingDir, queryPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}

	slog.Info("API: alist-encrypt stream", "path", absPath)

	var plugin alistencryptplugin.AlistEncryptPlugin
	if err := plugin.ServeStream(c.Writer, c.Request, absPath, password); err != nil {
		slog.Error("API: alist-encrypt stream failed", "error", err)
		writeServiceErrorGin(c, err)
	}
}

func (s *Server) handleAlistDecodeFilenameGin(c *gin.Context) {
	encoded := c.Query("encoded")
	password := c.Query("password")
	encType := c.DefaultQuery("enc_type", "aesctr")

	if encoded == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'encoded' query parameter is required"})
		return
	}

	plainName := alistencrypt.DecodeName(encoded, password, encType)
	c.JSON(http.StatusOK, gin.H{
		"plain_name": plainName,
		"success":    plainName != "",
	})
}

func (s *Server) handlePluginFilesStreamGin(c *gin.Context) {
	queryPath := c.Query("path")
	if queryPath == "" {
		queryPath = "/"
	}
	extensionsStr := c.Query("extensions")
	if extensionsStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'extensions' query parameter is required"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	c.Status(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		flusher = nil
	}

	absPath, err := utils.SafeURLToAbsPath(s.servingDir, queryPath)
	if err != nil {
		s.writeSSEEvent(c, flusher, `{"error":"invalid path"}`)
		s.writeSSEEvent(c, flusher, `[DONE]`)
		return
	}

	extSet := make(map[string]bool)
	for _, ext := range strings.Split(extensionsStr, ",") {
		e := strings.TrimSpace(strings.ToLower(ext))
		if e != "" {
			extSet[e] = true
		}
	}

	const maxResults = 500
	count := 0

	err = filepath.WalkDir(absPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if count >= maxResults {
			return fs.SkipAll
		}

		name := d.Name()
		if strings.HasPrefix(name, ".") {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(name))
		if !extSet[ext] {
			return nil
		}

		relPath, _ := filepath.Rel(absPath, path)
		filePath := queryPath + "/" + relPath
		if queryPath == "/" {
			filePath = "/" + relPath
		}

		info, err := d.Info()
		if err != nil {
			fi := mobileservice.FileInfo{
				Name:        name,
				Path:        filePath,
				IsDirectory: false,
				Size:        0,
				Modified:    "",
			}
			data, _ := json.Marshal(fi)
			s.writeSSEEvent(c, flusher, string(data))
			count++
			return nil
		}

		isEncrypted := false
		if _, detectErr := detector.DetectContainer(path); detectErr == nil {
			isEncrypted = true
		}

		fi := mobileservice.FileInfo{
			Name:        name,
			Path:        filePath,
			IsDirectory: false,
			IsEncrypted: isEncrypted,
			Size:        info.Size(),
			Modified:    info.ModTime().Format(time.RFC3339),
		}
		data, _ := json.Marshal(fi)
		s.writeSSEEvent(c, flusher, string(data))
		count++
		return nil
	})

	if err != nil && count < maxResults {
		errMsg := fmt.Sprintf(`{"error":"walk failed: %s"}`, err.Error())
		s.writeSSEEvent(c, flusher, errMsg)
	}

	s.writeSSEEvent(c, flusher, `[DONE]`)
}
