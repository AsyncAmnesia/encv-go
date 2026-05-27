package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Soltus/encv-go/internal/auth"
	"github.com/Soltus/encv-go/internal/config"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleGetConfigGin(c *gin.Context) {
	s.configMu.Lock()
	defer s.configMu.Unlock()

	if s.configPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "config path not available"})
		return
	}

	data, err := os.ReadFile(s.configPath)
	if err != nil {
		slog.Error("Failed to read config file", "path", s.configPath, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read config"})
		return
	}

	c.Data(http.StatusOK, "application/json", data)
}

func (s *Server) handlePutConfigGin(c *gin.Context) {
	s.configMu.Lock()
	defer s.configMu.Unlock()

	if s.configPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "config path not available"})
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	if errMsg := validateWebdavRouteInConfig(raw); errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	existingData, readErr := os.ReadFile(s.configPath)
	if readErr == nil {
		var existing map[string]interface{}
		if json.Unmarshal(existingData, &existing) == nil {
			for k, v := range raw {
				existing[k] = v
			}
			raw = existing
			slog.Info("Merged incoming config with existing file", "path", s.configPath)
		}
	} else {
		slog.Warn("No existing config to merge with (first write)", "path", s.configPath, "error", readErr)
	}

	indented, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to format config"})
		return
	}

	if err := os.WriteFile(s.configPath, append(indented, '\n'), 0644); err != nil {
		slog.Error("Failed to write config file", "path", s.configPath, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write config"})
		return
	}

	slog.Info("Config updated via API", "path", s.configPath)

	var newCfg config.Config
	needsRestart := false
	if err := json.Unmarshal(body, &newCfg); err != nil {
		slog.Warn("Config written but failed to parse for hot reload", "error", err)
		c.JSON(http.StatusOK, gin.H{"message": "config saved (hot reload skipped)"})
		return
	}

	s.cfg.Password = newCfg.Password
	s.cfg.Recover = newCfg.Recover
	s.cfg.OutputPath = newCfg.OutputPath
	s.cfg.PluginSettings = newCfg.PluginSettings

	if newCfg.Admin.Password != s.cfg.Admin.Password {
		s.cfg.Admin.Password = newCfg.Admin.Password
		if newCfg.Admin.Password != "" {
			s.jwtManager = auth.NewJWTManager(newCfg.Admin.Password, 7*24*time.Hour)
		} else {
			s.jwtManager = nil
		}
		slog.Info("Admin password hot-reloaded")
	}

	s.cfg.Webdav.Username = newCfg.Webdav.Username
	s.cfg.Webdav.Password = newCfg.Webdav.Password

	if newCfg.Log.Level != s.cfg.Log.Level {
		s.cfg.Log.Level = newCfg.Log.Level
		slog.Info("Log level hot-reloaded", "level", newCfg.Log.Level)
	}

	if newCfg.Server.Port != s.cfg.Server.Port {
		needsRestart = true
	}
	if newCfg.Webdav.Root != s.cfg.Webdav.Root || newCfg.Webdav.Dir != s.cfg.Webdav.Dir {
		needsRestart = true
	}
	if newCfg.Server.Dir != s.cfg.Server.Dir {
		needsRestart = true
	}

	s.cfg.Server = newCfg.Server
	s.cfg.Webdav.Root = newCfg.Webdav.Root
	s.cfg.Webdav.Dir = newCfg.Webdav.Dir
	s.cfg.Log = newCfg.Log

	msg := "config updated"
	if needsRestart {
		msg = "config saved, some changes require restart to take effect"
	}
	c.JSON(http.StatusOK, gin.H{"message": msg, "needsRestart": needsRestart})
}

func (s *Server) handleConfigSchemaGin(c *gin.Context) {
	schemaPaths := []string{}

	if s.configPath != "" {
		dir := filepath.Dir(s.configPath)
		base := filepath.Base(s.configPath)
		schemaName := strings.TrimSuffix(base, filepath.Ext(base)) + ".schema.json"
		schemaPaths = append(schemaPaths, filepath.Join(dir, schemaName))
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		schemaPaths = append(schemaPaths, filepath.Join(exeDir, "config.schema.json"))
	}

	for _, p := range schemaPaths {
		data, err := os.ReadFile(p)
		if err == nil {
			c.Data(http.StatusOK, "application/json", data)
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "schema file not found"})
}

func validateWebdavRouteInConfig(raw map[string]interface{}) string {
	wd, ok := raw["webdav"].(map[string]interface{})
	if !ok {
		return ""
	}
	root, ok := wd["root"].(string)
	if !ok || root == "" {
		return ""
	}
	cleaned := strings.TrimSpace(root)
	if cleaned == "/" || cleaned == "//" {
		return "webdav root cannot be '/' (would capture all routes and crash server)"
	}
	if !strings.HasPrefix(cleaned, "/") {
		return "webdav root must start with '/'"
	}
	return ""
}
