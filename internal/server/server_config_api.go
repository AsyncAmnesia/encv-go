package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var configMu sync.Mutex

func (s *Server) handleConfigAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		s.handleGetConfig(w, r)
	case http.MethodPut:
		s.handlePutConfig(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	configMu.Lock()
	defer configMu.Unlock()

	if s.configPath == "" {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "config path not available"})
		return
	}

	data, err := os.ReadFile(s.configPath)
	if err != nil {
		slog.Error("Failed to read config file", "path", s.configPath, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to read config"})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	configMu.Lock()
	defer configMu.Unlock()

	if s.configPath == "" {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "config path not available"})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to read request body"})
		return
	}
	defer r.Body.Close()

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}

	indented, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to format config"})
		return
	}

	if err := os.WriteFile(s.configPath, append(indented, '\n'), 0644); err != nil {
		slog.Error("Failed to write config file", "path", s.configPath, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to write config"})
		return
	}

	slog.Info("Config updated via API", "path", s.configPath)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "config updated"})
}

func (s *Server) handleConfigSchemaAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	w.Header().Set("Content-Type", "application/json")

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
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{"error": "schema file not found"})
}
