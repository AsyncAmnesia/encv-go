package encv

import (
	"github.com/Soltus/encv-go/internal/v2/container/manifest"
)

// ExtractKVI_v2 从 v2 容器文件中直接扫描并提取 KVI 块的数据，不依赖清单。
func ExtractKVI_v2(containerPath string) ([]byte, error) {
	return manifest.ExtractKVI_v2(containerPath)
}

// ExtractManifest_v2 从 v2 容器文件中直接扫描并提取 Manifest 块的数据。
func ExtractManifest_v2(containerPath string) ([]byte, error) {
	return manifest.ExtractManifest_v2(containerPath)
}
