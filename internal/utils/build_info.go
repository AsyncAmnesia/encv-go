//go:build android

package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	buildInfoOnce sync.Once
	buildInfoData map[string]interface{}
	buildInfoErr  error
)

func GetBuildInfo() (map[string]interface{}, error) {
	buildInfoOnce.Do(func() {
		libDir := os.Getenv("ENCV_LIB_DIR")
		if libDir == "" {
			buildInfoErr = fmt.Errorf("ENCV_LIB_DIR not set")
			return
		}
		path := filepath.Join(libDir, "build-info.json")
		data, err := os.ReadFile(path)
		if err != nil {
			buildInfoErr = fmt.Errorf("failed to read build-info.json: %w", err)
			return
		}
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			buildInfoErr = fmt.Errorf("failed to parse build-info.json: %w", err)
			return
		}
		buildInfoData = result
	})
	return buildInfoData, buildInfoErr
}
