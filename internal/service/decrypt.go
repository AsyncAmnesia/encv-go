// internal/service/decrypt.go

package service

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/container/chunked"
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// DecryptContainer 从一个加密容器文件路径中解密内容
// 直接处理文件路径，不创建临时文件
func DecryptContainer(ctx context.Context, containerPath string) (*types.DecryptedContent, error) {
	cfg := config.FromContext(ctx)
	// 1. 检测容器类型
	detectedExt, err := container.DetectContainerTypeFromFile(ctx, containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect container type for %s: %w", containerPath, err)
	}

	var packedData *container.PackedData

	// 获取 magic map，避免在 case 内重复调用
	magicMap, err := container.GetContainerMagicMap(ctx)
	// subMagicMap := container.GetSubChunkMagicMap()

	// 2. 根据类型选择不同的处理方式
	switch detectedExt {
	case cfg.BinExtGroup.Video:
		packedData, err = decryptVideo(ctx, containerPath, detectedExt)
		if err != nil {
			return nil, fmt.Errorf("failed to decryptVideo for %s: %w", containerPath, err)
		}

	case cfg.BinExtGroup.Image, cfg.BinExtGroup.Text:
		file, err := os.Open(containerPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open image container: %w", err)
		}
		// 【关键修复】移除 defer file.Close()，关闭责任转移给返回的 DataStream

		packedData, err = container.Unpack(file, magicMap[detectedExt])
		if err != nil {
			file.Close() // 如果 Unpack 失败，需要在这里手动关闭文件
			return nil, fmt.Errorf("failed to unpack image container: %w", err)
		}
		// 【关键修复】确保 packedData.DataStream 关闭时，底层的 file 也被关闭
		// 我们创建一个新的 readCloser 来组合关闭操作
		packedData.DataStream = newReadCloser(packedData.DataStream, file.Close)

	default:
		return nil, fmt.Errorf("unsupported container type: %s", detectedExt)
	}

	// 【新增防御性检查】
	if packedData == nil {
		return nil, fmt.Errorf("internal error: unpacking resulted in a nil packedData for container %s", containerPath)
	}
	if packedData.DataStream == nil {
		return nil, fmt.Errorf("internal error: unpacking resulted in a nil DataStream for container %s", containerPath)
	}

	// 3. 解析 KVI
	index, err := utils.UnmarshalKVI(packedData.KVIData)
	if err != nil {
		packedData.DataStream.Close()
		return nil, fmt.Errorf("failed to parse KVI: %w", err)
	}

	// 4. 创建解密流
	salt, err := crypto.Base64Decode(index.GetEncryptionInfo().SaltBase64)
	if err != nil {
		packedData.DataStream.Close()
		return nil, fmt.Errorf("failed to decode salt: %w", err)
	}
	key := crypto.GenerateKey(cfg.Password, salt)

	decryptedStream, err := crypto.GetDecryptReader(packedData.DataStream, key)
	if err != nil {
		packedData.DataStream.Close()
		return nil, fmt.Errorf("failed to create decrypt reader: %w", err)
	}

	// 5. 返回结果，确保关闭 packedData.DataStream
	return &types.DecryptedContent{
		Index:      index,
		DataStream: newReadCloser(decryptedStream, packedData.DataStream.Close),
	}, nil
}

// ExtractKVI 从容器文件中提取原始的 KVI 数据，无需密码。
// 与 DecryptContainer 逻辑保持一致
func ExtractKVI(ctx context.Context, containerPath string) ([]byte, error) {
	cfg := config.FromContext(ctx)
	// 1. 检测容器类型
	detectedExt, err := container.DetectContainerTypeFromFile(ctx, containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect container type for %s: %w", containerPath, err)
	}

	// 获取 magic map，避免在 case 内重复调用
	magicMap, err := container.GetContainerMagicMap(ctx)
	subMagicMap, err := container.GetSubChunkMagicMap(ctx)

	// 2. 根据类型选择不同的处理方式
	switch detectedExt {
	case cfg.BinExtGroup.Video:
		// 【关键修复】对于视频，直接使用文件路径创建 LocalReader
		mainMagic := magicMap[detectedExt]
		subMagic := subMagicMap[detectedExt]

		reader, err := chunked.LocalReader(containerPath, mainMagic, subMagic)
		if err != nil {
			return nil, fmt.Errorf("failed to create chunked reader for %s: %w", containerPath, err)
		}
		defer reader.Close()
		return reader.KVIData, nil

	case cfg.BinExtGroup.Image, cfg.BinExtGroup.Text:
		file, err := os.Open(containerPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open image container: %w", err)
		}
		defer file.Close()

		packedData, err := container.Unpack(file, magicMap[detectedExt])
		if err != nil {
			return nil, fmt.Errorf("failed to unpack image container: %w", err)
		}
		defer packedData.DataStream.Close()
		return packedData.KVIData, nil

	default:
		return nil, fmt.Errorf("unsupported container type for KVI extraction: %s", detectedExt)
	}
}

// RestoreSubtitles 为视频还原字幕
func RestoreSubtitles(index *types.VideoIndex, containerDir, outputDir string) error {
	return utils.RestoreSubtitlesFromKVI(index, containerDir, outputDir)
}

// --- 辅助函数 ---

// newReadCloser 组合一个 io.Reader 和一个 Close 函数
type readCloser struct {
	io.Reader
	closeFunc func() error
}

func (rc *readCloser) Close() error {
	return rc.closeFunc()
}

func newReadCloser(r io.Reader, closeFunc func() error) io.ReadCloser {
	return &readCloser{Reader: r, closeFunc: closeFunc}
}
