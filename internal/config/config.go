package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/types"
)

// LoadUserConfig 从当前目录加载 config.user.json
func LoadUserConfig() (*types.UserConfig, error) {
	configPath := filepath.Join(".", "config.user.json")
	cfg := &types.UserConfig{
		TrackExtensions: []string{".ass", ".srt", ".dm.ass"}, // 默认值
	}

	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}
