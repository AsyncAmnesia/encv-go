// internal/container/detect.go

package container

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Soltus/encv-go/internal/config"
)

const (
	// 魔法数字本身是格式规范的一部分，不应由用户配置
	MagicVideo         = "encv-sccgv-chunk-main-v1" // SCCGV 主分片
	MagicVideoSubChunk = "encv-sccgv-chunk-sub-v1"  // SCCGV 子分片
	MagicText          = "encv-sccgt-container-v1"
	MagicAudio         = "encv-sccga-container-v1"
	MagicImage         = "encv-sccgi-container-v1"
	MagicIFrame        = "encv-sccgf-container-v1"
)

// 定义一个特殊的类型，用于表示子分片
const SubChunkType = "sub_chunk"

// DetectMainOrSubContainerType 检测文件是主容器、子分片，还是未知文件。
// 它返回一个字符串类型，用于后续的 switch 判断。
func DetectMainOrSubContainerType(ctx context.Context, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for detection: %w", err)
	}
	defer file.Close()

	// 读取文件头
	header := make([]byte, 64) // 足够容纳所有魔法数字
	bytesRead, err := file.Read(header)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read file header: %w", err)
	}
	headerStr := string(header[:bytesRead])

	// 1. 优先检查是否是子分片
	subMagicMap, err := GetSubChunkMagicMap(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get sub-chunk magic map: %w", err)
	}
	for _, magic := range subMagicMap {
		if len(headerStr) >= len(magic) && headerStr[:len(magic)] == magic {
			return SubChunkType, nil // 明确返回子分片类型
		}
	}

	// 2. 如果不是子分片，再检查是否是主容器
	magicMap, err := GetContainerMagicMap(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get main container magic map: %w", err)
	}
	for ext, magic := range magicMap {
		if len(headerStr) >= len(magic) && headerStr[:len(magic)] == magic {
			return ext, nil // 返回主容器类型，如 "video", "image"
		}
	}

	// 3. 都不是，返回错误
	return "", fmt.Errorf("file '%s' is not a recognized ENCV container or sub-chunk", filePath)
}

// getContainerMagicMap 动态构建容器扩展名到魔法数字的映射
// 这是获取所有容器魔法数字的公共入口
func GetContainerMagicMap(ctx context.Context) (map[string]string, error) {
	cfg := config.FromContext(ctx)
	// 【关键修复】检查从 context 中获取的配置是否为 nil
	// 这是在某些初始化顺序错误或 context 传递错误时防止 panic 的关键防线
	if cfg == nil {
		return nil, errors.New("configuration not found in context, cannot detect container")
	}

	// 从 GlobalConfig 读取用户定义的后缀，并映射到固定的魔法数字
	return map[string]string{
		cfg.BinExtGroup.Video:  MagicVideo,
		cfg.BinExtGroup.Text:   MagicText,
		cfg.BinExtGroup.Audio:  MagicAudio,
		cfg.BinExtGroup.Image:  MagicImage,
		cfg.BinExtGroup.Iframe: MagicIFrame,
	}, nil
}

// GetSubChunkMagicMap 获取子分片的魔法数字映射
func GetSubChunkMagicMap(ctx context.Context) (map[string]string, error) {
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return nil, errors.New("configuration not found in context, cannot get sub-chunk magic map")
	}

	// 目前只有视频有子分片
	return map[string]string{
		cfg.BinExtGroup.Video: MagicVideoSubChunk,
	}, nil
}

// DetectContainerType 从文件头字节切片中检测容器类型
func DetectContainerType(ctx context.Context, header []byte) (string, error) {
	magicMap, err := GetContainerMagicMap(ctx)
	if err != nil {
		// 如果无法获取魔法数字映射，则无法检测，返回错误
		return "", fmt.Errorf("failed to get magic map for detection: %w", err)
	}

	// 遍历所有已知的魔法数字进行匹配
	for ext, magic := range magicMap {
		// bytes.HasPrefix 是安全的，即使 header 比 magic 短，它也会返回 false
		if bytes.HasPrefix(header, []byte(magic)) {
			return ext, nil
		}
	}

	// 如果没有匹配到，说明不是已知的容器类型
	return "", nil // 返回空字符串和 nil 错误，表示“不是容器”，而不是“发生了错误”
}

// DetectContainerTypeFromFile 从文件路径中检测容器类型
func DetectContainerTypeFromFile(ctx context.Context, filePath string) (string, error) {
	// 1. 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		// 如果文件无法打开（例如，权限不足、不存在），则不是一个有效的容器
		return "", nil // 返回“不是容器”
	}
	defer file.Close()

	// 2. 获取魔法数字映射并计算所需的最大头部大小
	magicMap, err := GetContainerMagicMap(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get magic map for file '%s': %w", filePath, err)
	}

	maxMagicLen := 0
	for _, magic := range magicMap {
		if len(magic) > maxMagicLen {
			maxMagicLen = len(magic)
		}
	}

	// 3. 安全地读取文件头部
	// 使用 io.ReadFull 并处理 ErrUnexpectedEOF 是处理小文件的标准、健壮模式
	header := make([]byte, maxMagicLen)
	bytesRead, err := io.ReadFull(file, header)

	// 只要不是致命的 I/O 错误，我们都应该继续。
	// io.EOF 和 io.ErrUnexpectedEOF 表示文件比我们期望的短，这是正常情况。
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", nil // 读取错误，视为“不是容器”
	}

	// 4. 将切片截断为实际读取的长度，防止越界
	// 这是最关键的一步，确保后续的 bytes.HasPrefix 不会访问到未初始化的内存
	actualHeader := header[:bytesRead]

	// 5. 调用核心检测函数
	return DetectContainerType(ctx, actualHeader)
}
