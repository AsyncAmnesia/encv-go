package service

import (
	"fmt"

	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/container/chunked"
)

// 对于视频，直接使用文件路径创建 LocalReader
func decryptVideo(containerPath string, detectedExt string) (*container.PackedData, error) {

	// 获取 magic map
	magicMap := container.GetContainerMagicMap()
	subMagicMap := container.GetSubChunkMagicMap()
	mainMagic, ok := magicMap[detectedExt]
	if !ok {
		return nil, fmt.Errorf("internal error: detected video extension '%s' not in magic map", detectedExt)
	}
	subMagic, ok := subMagicMap[detectedExt]
	if !ok {
		return nil, fmt.Errorf("internal error: detected video extension '%s' not in sub-magic map", detectedExt)
	}

	reader, err := chunked.LocalReader(containerPath, mainMagic, subMagic)
	if err != nil {
		return nil, fmt.Errorf("failed to create chunked reader for %s: %w", containerPath, err)
	}

	// 【关键修复】增加防御性检查，防止 LocalReader 返回 (nil, nil)
	if reader == nil {
		return nil, fmt.Errorf("internal error: chunked.LocalReader returned a nil reader without an error for file %s", containerPath)
	}

	return &container.PackedData{
		KVIData:    reader.KVIData,
		DataStream: reader, // reader 本身就是 io.ReadCloser
	}, nil
}
