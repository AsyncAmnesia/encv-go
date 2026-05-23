package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
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
