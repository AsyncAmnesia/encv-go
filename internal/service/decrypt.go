// internal/service/decrypt.go

package service

import (
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

// DecryptedContent 包含解密后的所有内容
type DecryptedContent struct {
	Index      types.Index
	DataStream io.ReadCloser
}

// DecryptContainer 从一个加密容器文件路径中解密内容
// 直接处理文件路径，不创建临时文件
func DecryptContainer(containerPath, password string) (*DecryptedContent, error) {
	// 1. 检测容器类型
	detectedExt, err := container.DetectContainerTypeFromFile(containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect container type for %s: %w", containerPath, err)
	}

	var packedData *container.PackedData

	// 获取 magic map，避免在 case 内重复调用
	magicMap := container.GetContainerMagicMap()
	subMagicMap := container.GetSubChunkMagicMap()

	// 2. 根据类型选择不同的处理方式
	switch detectedExt {
	case config.GlobalConfig.BinExtGroup.Video:
		// 对于视频，直接使用文件路径创建 LocalReader
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

		packedData = &container.PackedData{
			KVIData:    reader.KVIData,
			DataStream: reader, // reader 本身就是 io.ReadCloser
		}

	case config.GlobalConfig.BinExtGroup.Image:
		// 对于图像，使用流式处理
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

	// 3. 解析 KVI
	index, err := types.UnmarshalKVI(packedData.KVIData)
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
	key := crypto.GenerateKey(password, salt)

	decryptedStream, err := crypto.GetDecryptReader(packedData.DataStream, key)
	if err != nil {
		packedData.DataStream.Close()
		return nil, fmt.Errorf("failed to create decrypt reader: %w", err)
	}

	// 5. 返回结果，确保关闭 packedData.DataStream
	return &DecryptedContent{
		Index:      index,
		DataStream: newReadCloser(decryptedStream, packedData.DataStream.Close),
	}, nil
}

// ExtractKVI 从容器文件中提取原始的 KVI 数据，无需密码。
// 【简化重构】与 DecryptContainer 逻辑保持一致
func ExtractKVI(containerPath string) ([]byte, error) {
	// 1. 检测容器类型
	detectedExt, err := container.DetectContainerTypeFromFile(containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect container type for %s: %w", containerPath, err)
	}

	// 获取 magic map，避免在 case 内重复调用
	magicMap := container.GetContainerMagicMap()
	subMagicMap := container.GetSubChunkMagicMap()

	// 2. 根据类型选择不同的处理方式
	switch detectedExt {
	case config.GlobalConfig.BinExtGroup.Video:
		// 【关键修复】对于视频，直接使用文件路径创建 LocalReader
		mainMagic := magicMap[detectedExt]
		subMagic := subMagicMap[detectedExt]

		reader, err := chunked.LocalReader(containerPath, mainMagic, subMagic)
		if err != nil {
			return nil, fmt.Errorf("failed to create chunked reader for %s: %w", containerPath, err)
		}
		defer reader.Close()
		return reader.KVIData, nil

	case config.GlobalConfig.BinExtGroup.Image:
		// 对于图像，使用流式处理
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
