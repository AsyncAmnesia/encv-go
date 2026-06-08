package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Soltus/encv-go/internal/config"
	mobileservice "github.com/Soltus/encv-go/internal/service"
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/container/detector"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	alistencrypt "github.com/Soltus/encv-go/internal/v2/plugins/alistencrypt"
	pluginInterfaces "github.com/Soltus/encv-go/internal/v2/plugins/interfaces"
	"github.com/Soltus/encv-go/internal/v2/types"
	"github.com/gin-gonic/gin"
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
	queryPath := utils.DecodeGinQueryParam(c.Query("path"))
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
	queryPath := utils.DecodeGinQueryParam(c.Query("path"))
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

func (s *Server) handleUploadFileGin(c *gin.Context) {
	targetPath := utils.DecodeGinQueryParam(c.Query("path"))
	if targetPath == "" {
		targetPath = "/"
	}
	slog.Info("API: upload file", "target_path", targetPath)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or invalid 'file' form field"})
		return
	}
	defer file.Close()

	if header.Filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filename is empty"})
		return
	}

	result, err := s.mobileSvc.UploadFile(targetPath, header.Filename, file, 0)
	if err != nil {
		writeServiceErrorGin(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

func (s *Server) handleServiceGuardGin(c *gin.Context) {
	// 收集诊断上下文（任何守卫失败都带上 → 调试不再靠"猜"）
	cwd, _ := os.Getwd()
	envDevPreview := os.Getenv("ENCV_DEV_PREVIEW") == "1"
	envMobile := os.Getenv("ENCV_MOBILE") == "1"

	// 规范：mock 脚本与启动脚本在仓库里的固定路径
	mockScriptRel := "app/encv-mobile/scripts/generate-mock-files.ts"
	previewScriptRel := "app/encv-mobile/scripts/start-preview.sh"
	makefileRel := "Makefile"
	docsMockSpec := ".trae/documents/mock-real-files-plan.md"

	// 根因分类：A) 路径不可读 B) 缺 marker 目录
	type guardContext struct {
		Os          string `json:"os"`
		Arch        string `json:"arch"`
		Cwd         string `json:"cwd"`
		ServingDir  string `json:"servingDir"`
		ServingDirExists bool   `json:"servingDirExists"`
		EnvDevPreview bool  `json:"envDevPreview"`
		EnvMobile      bool  `json:"envMobile"`
		MockScript   string `json:"mockScript"`
		PreviewScript string `json:"previewScript"`
		Makefile     string `json:"makefile"`
		Docs         string `json:"docs"`
	}

	ctx := guardContext{
		Os:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		Cwd:            cwd,
		ServingDir:     s.servingDir,
		ServingDirExists: fileExists(s.servingDir),
		EnvDevPreview:  envDevPreview,
		EnvMobile:      envMobile,
		MockScript:     mockScriptRel,
		PreviewScript:  previewScriptRel,
		Makefile:     makefileRel,
		Docs:         docsMockSpec,
	}

	files, err := s.mobileSvc.ListFiles("/")
	if err != nil {
		// 根因 A: servingDir 不可读（Linux 沙箱常见 — 路径不存在或权限不足）
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"ready":  false,
			"detail": "failed to list root directory",
			"error":  err.Error(),
			"context": ctx,
			"remediation": []gin.H{
				{
					"scenario": "A1 — Linux 沙箱开发 (推荐)",
					"steps": []string{
						"切换到 Makefile dev-mobile 目标（自动 ENCV_DEV_PREVIEW=1 + mock 生成）：",
						"  make dev-mobile",
						"或手动：",
						"  cd app/encv-mobile && npx tsx scripts/generate-mock-files.ts --dir /storage/emulated/0",
						"  ENCV_DEV_PREVIEW=1 go run ./cmd/encv start",
					},
				},
				{
					"scenario": "A2 — Android 真机 (Capacitor)",
					"steps": []string{
						"路径 /storage/emulated/0 在 Android 上是合法路径，但需 SD 卡权限。",
						"如使用沙盒路径或 scoped storage，改用：",
						"  ENCV_DEV_PREVIEW=1 ENCV_MOCK_ROOT=/sdcard npx tsx scripts/generate-mock-files.ts",
					},
				},
				{
					"scenario": "A3 — 桌面端正常模式 (无 mobile overlay)",
					"steps": []string{
						"不要设 ENCV_DEV_PREVIEW / ENCV_MOBILE，servingDir 自动 = 当前目录。",
						"在此目录运行 mock 生成：",
						"  npx tsx scripts/generate-mock-files.ts --dir \"$(pwd)\"",
					},
				},
			},
		})
		return
	}

	const marker = "01-plain-media"
	hasMarker := false
	var dirNames []string
	for _, f := range files {
		dirNames = append(dirNames, f.Name)
		if f.Name == marker && f.IsDirectory {
			hasMarker = true
		}
	}

	if !hasMarker {
		preview := len(dirNames) > 10
		if !preview {
			preview = len(dirNames) == len(files)
		}
		displayNames := dirNames
		if preview && len(displayNames) > 10 {
			displayNames = displayNames[:10]
		}

		// 修复命令：自动用当前 servingDir 替换占位符
		mockCmd := "cd app/encv-mobile && npx tsx scripts/generate-mock-files.ts --dir " + s.servingDir
		makeCmd := "make dev-mobile    # ENCV_DEV_PREVIEW=1 + 自动 mock 生成"
		previewCmd := "bash app/encv-mobile/scripts/start-preview.sh    # 一键前台启动（含 mock + air + vite）"

		// 根因 B: servingDir 可读但缺 01-plain-media marker
		c.JSON(http.StatusForbidden, gin.H{
			"ready":  false,
			"marker": marker,
			"found":  displayNames,
			"detail": fmt.Sprintf("server.dir 缺少约定的 marker 目录 %q（这是 ENCV mock 数据规范的前缀，详见 %s）", marker, mockScriptRel),
			"context": ctx,
			"remediation": []gin.H{
				{
					"scenario": "B1 — 一键修复（推荐）",
					"command":  previewCmd,
					"explain":  "脚本会先 pkill 残留、npm install、生成 mock、启动 air+ENCV_DEV_PREVIEW=1、启动 Vite、保持前台运行（便于 OpenPreview 激活）",
				},
				{
					"scenario": "B2 — Makefile 目标",
					"command":  makeCmd,
					"explain":  "执行 make dev-mobile 即可：先 mock 生成到 /storage/emulated/0，再 ENCV_MOBILE=1 ENCV_DEV_PREVIEW=1 go run",
				},
				{
					"scenario": "B3 — 手动精确生成 mock 到当前 servingDir",
					"command":  mockCmd,
					"explain":  "规范脚本：scripts/generate-mock-files.ts 会在 " + s.servingDir + " 下创建 01-plain-media 04 个分类（plain/ae/container/boundary）",
				},
				{
					"scenario": "B4 — 如果不想用 mock（极少见）",
					"steps": []string{
						"在 " + s.servingDir + " 下手动建一个空目录：",
						"  mkdir -p " + s.servingDir + "/01-plain-media",
						"（不推荐，违反 mock 规范，CI 会判违规）",
					},
				},
			},
			"expectations": gin.H{
				"after_fix_servingDir": s.servingDir,
				"after_fix_should_contain": []string{
					"01-plain-media/   (普通媒体：image/video/audio/document)",
					"02-alist-encrypt/ (AE 加密文件)",
					"03-encv-containers/ (ENCV v4 容器)",
					"04-boundary-test/  (边界测试：长文件名/控制字符/特殊字符)",
				},
				"verify_cmd": "curl -s http://localhost:2025/api/service-guard  # 期望 HTTP 200 + ready:true",
			},
		})
		return
	}

	// OK: 一切就绪 — 列出 marker 实际内容（让用户立刻看到 mock 数据布局）
	markerChildren := make([]string, 0, 4)
	for _, f := range files {
		if f.IsDirectory && (f.Name == "01-plain-media" || f.Name == "02-alist-encrypt" || f.Name == "03-encv-containers" || f.Name == "04-boundary-test") {
			markerChildren = append(markerChildren, f.Name)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"ready":      true,
		"servingDir": s.servingDir,
		"marker":     marker,
		"context":    ctx,
		"mockCategories": markerChildren,
		"nextStep":   "前端访问 http://localhost:5173/ （前提是已 OpenPreview 激活）",
	})
}

// fileExists 判断路径是否存在（避免守卫 error 里少这一上下文）
func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// pluginOpenlistUpstream plugin-openlist 独立 Vite dev server
// （由 pm2 app `plugin-openlist-vite` 拉起，独立于 encv-mobile vite :5173）
const pluginOpenlistUpstream = "http://127.0.0.1:5174"

// handlePluginOpenlistProxyGin 反向代理 /api/preview/plugin-openlist/* → :5174/*
//
// 为什么需要：
//   - plugin-openlist 是独立 Capacitor 插件的 Vite dev server（:5174），
//     跟 encv-mobile 主 vite (5173) 没有父子关系。
//   - 前端点击 "预览 OpenList Plugin" 不能直接跳 :5174（破坏 OpenPreview 会话，
//     且 Capacitor native 端 127.0.0.1 指向设备本身，不通）。
//   - vite.config.ts 的 openlist-ui-proxy 只能代理 :5244（OpenList 真实前端），
//     不能代理 :5174（plugin-openlist 是另一个独立 vite 进程）。
//
// 方案：encv-go 后端（独立后端）做 reverse proxy 协调，
// 前端跳相对路径 /api/preview/plugin-openlist/ → encv-go → :5174。
//
// 完全不依赖 vite，符合"独立后端协调"原则。
func (s *Server) handlePluginOpenlistProxyGin(c *gin.Context) {
	target, err := url.Parse(pluginOpenlistUpstream)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid upstream: " + err.Error()})
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	// Director 改写 host + path，透传 header
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
		// req.URL.Path 已经被 ReverseProxy 设为原始请求路径，
		// 我们要 strip 掉 /api/preview/plugin-openlist 前缀
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/api/preview/plugin-openlist")
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
	}
	// 自定义 ErrorHandler：upstream 不可达时返回明确错误（不要 502 难诊断）
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		slog.Error("[plugin-openlist proxy] upstream error", "err", err, "url", req.URL.String())
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		rw.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(rw, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>plugin-openlist 未运行</title></head>
<body style="font-family:system-ui;padding:24px;max-width:560px;margin:auto">
<h2>plugin-openlist dev server 未运行</h2>
<p>upstream: %s</p>
<p>err: %s</p>
<p>启动方式（pm2 统一管理）：<code>npx pm2 start ecosystem.config.cjs --only plugin-openlist-vite</code></p>
<p>或直接：<code>cd app/encv-mobile/plugin-openlist/web && pnpm dev</code></p>
</body></html>`, pluginOpenlistUpstream, err.Error())
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func (s *Server) handleReadFileContentGin(c *gin.Context) {
	queryPath := utils.DecodeGinQueryParam(c.Query("path"))
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
	queryPath := utils.DecodeGinQueryParam(c.Query("path"))
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
		Type              string            `json:"type"`
		SourcePath        string            `json:"sourcePath"`
		TargetPath        string            `json:"targetPath,omitempty"`
		Password          string            `json:"password,omitempty"`
		SecondaryPassword string            `json:"secondaryPassword,omitempty"`
		Version           int               `json:"version,omitempty"`
		PluginName        string            `json:"pluginName,omitempty"`
		ExtraFields       map[string]string `json:"extraFields,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	slog.Info("API: create task", "type", req.Type, "source", req.SourcePath,
		"target", req.TargetPath, "version", req.Version,
		"pluginName", req.PluginName,
		"hasPassword", req.Password != "",
		"hasSecondaryPassword", req.SecondaryPassword != "",
		"hasExtraFields", len(req.ExtraFields) > 0)
	task := s.mobileSvc.GetTaskManager().CreateWithExtras(
		req.Type, req.SourcePath, req.TargetPath,
		req.Password, req.SecondaryPassword, req.Version, req.PluginName, req.ExtraFields,
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

func (s *Server) handleClearCompletedTasksGin(c *gin.Context) {
	count := s.mobileSvc.GetTaskManager().ClearCompleted()
	c.JSON(http.StatusOK, gin.H{"ok": true, "removed": count})
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
	queryPath := utils.DecodeGinQueryParam(c.Query("path"))
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
	queryPath := utils.DecodeGinQueryParam(c.Query("path"))

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
	builtinOrder := []string{}
	for siteId, siteCfg := range cfg.Proxy.Sites {
		proxyURL := fmt.Sprintf("http://127.0.0.1:%d/openlist/sites/%s/", port, siteId)
		openlistSites[siteId] = map[string]interface{}{
			"host":        siteCfg.Host,
			"description": siteCfg.Description,
			"proxyUrl":    proxyURL,
			"built_in":    siteCfg.BuiltIn,
		}
		if siteCfg.BuiltIn {
			builtinOrder = append([]string{siteId}, builtinOrder...)
		} else {
			builtinOrder = append(builtinOrder, siteId)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"webdav":        webdavInfo,
		"openlistSites": openlistSites,
		"openlistOrder": builtinOrder,
	})
}

func (s *Server) handleListOpenlistSitesGin(c *gin.Context) {
	sites := make(map[string]interface{})
	builtinOrder := []string{}
	for siteId, siteCfg := range s.cfg.Proxy.Sites {
		sites[siteId] = map[string]interface{}{
			"host":        siteCfg.Host,
			"description": siteCfg.Description,
			"built_in":    siteCfg.BuiltIn,
		}
		if siteCfg.BuiltIn {
			builtinOrder = append([]string{siteId}, builtinOrder...)
		} else {
			builtinOrder = append(builtinOrder, siteId)
		}
	}
	c.JSON(http.StatusOK, gin.H{"sites": sites, "order": builtinOrder})
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
	queryPath := utils.DecodeGinQueryParam(c.Query("path"))
	exists, err := s.mobileSvc.FileExists(queryPath)
	if err != nil {
		writeServiceErrorGin(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"exists": exists})
}

func (s *Server) handleEncryptOutputExistsGin(c *gin.Context) {
	sourcePath := utils.DecodeGinQueryParam(c.Query("sourcePath"))
	targetDir := utils.DecodeGinQueryParam(c.Query("targetDir"))
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

	raw, err := json.Marshal(s.cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	var generic map[string]interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return fmt.Errorf("failed to unmarshal config for filtering: %w", err)
	}

	if proxy, ok := generic["proxy"].(map[string]interface{}); ok {
		if sites, ok := proxy["sites"].(map[string]interface{}); ok {
			for id, raw := range sites {
				if entry, ok := raw.(map[string]interface{}); ok {
					if builtin, _ := entry["built_in"].(bool); builtin {
						delete(sites, id)
					}
				}
			}
		}
	}

	indented, err := json.MarshalIndent(generic, "", "  ")
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
		"ffmpeg_available":  ffmpegOk,
		"ffprobe_available": ffprobeOk,
		"error":             errMsg,
		"ffmpeg_detail":     ffmpegDetail,
		"ffprobe_detail":    ffprobeDetail,
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
		// ★ 关键修复: 先用 SafeResolveToAbsPath 把前端传来的路径解析为绝对路径，
		// 防止 mobile 模式下 servingDir=/storage/emulated/0 时，插件拿不到真实文件
		absSourcePath, err := utils.SafeResolveToAbsPath(s.servingDir, req.SourcePath)
		if err != nil {
			c.JSON(200, gin.H{"candidates": []gin.H{}, "pluginName": nil, "taskOptions": nil, "error": fmt.Sprintf("invalid path: %v", err)})
			return
		}
		candidates = plugins.FindAllEncryptingPlugins(absSourcePath)
	} else {
		// ★ 关键修复: 同上，解密前必须先 resolve 绝对路径
		absSourcePath, err := utils.SafeResolveToAbsPath(s.servingDir, req.SourcePath)
		if err != nil {
			c.JSON(200, gin.H{"candidates": []gin.H{}, "pluginName": nil, "taskOptions": nil, "error": fmt.Sprintf("invalid path: %v", err)})
			return
		}
		targetPlugin, err := plugins.FindDecryptingPlugin(absSourcePath)
		if err != nil || targetPlugin == nil {
			c.JSON(200, gin.H{"candidates": []gin.H{}, "pluginName": nil, "taskOptions": nil, "error": err.Error()})
			return
		}
		opts := targetPlugin.GetTaskOptions()
		candidates = []plugins.PluginCandidate{{
			Plugin: targetPlugin, Name: targetPlugin.Name(), MatchType: "container", Priority: 0,
		}}
		c.JSON(200, gin.H{
			"candidates":  []gin.H{{"name": targetPlugin.Name(), "matchType": "container", "priority": 0, "taskOptions": taskOptionsToGinH(opts)}},
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
		"candidates":  candidateList,
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
	queryPath := utils.DecodeGinQueryParam(c.Query("path"))
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

	absPath, err := utils.SafeResolveToAbsPath(s.servingDir, queryPath)
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
	queryPath := utils.DecodeGinQueryParam(c.Query("path"))
	password := c.Query("password")
	if queryPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'path' query parameter is required"})
		return
	}

	absPath, err := utils.SafeResolveToAbsPath(s.servingDir, queryPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}

	slog.Info("API: alist-encrypt stream", "path", absPath)

	// 走统一范式：构造 FileContentProvider，调 ContentHandler.ServeFile
	// 与 v4 容器预览共享同一套 HTTP 协议处理（Range/206/Content-Length/Content-Range）
	var plugin alistencrypt.AlistEncryptPlugin
	rc, size, _, showName, err := plugin.Stream(absPath, password)
	if err != nil {
		slog.Error("API: alist-encrypt stream open failed", "error", err)
		writeServiceErrorGin(c, err)
		return
	}
	sr, ok := rc.(*alistencrypt.SeekableDecryptReader)
	if !ok {
		_ = rc.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal: unexpected reader type"})
		return
	}
	prov := alistencrypt.NewAlistEncryptFileProvider(sr, size, showName)
	defer prov.Close()
	s.contentHandler.ServeFile(c.Writer, c.Request, prov)
}

func (s *Server) handleAlistDecodeFilenameGin(c *gin.Context) {
	encoded := utils.DecodeGinQueryParam(c.Query("encoded"))
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
	queryPath := utils.DecodeGinQueryParam(c.Query("path"))
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

	absPath, err := utils.SafeResolveToAbsPath(s.servingDir, queryPath)
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
