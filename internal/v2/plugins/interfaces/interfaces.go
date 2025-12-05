package interfaces

import (
	"io"

	"github.com/Soltus/encv-go/internal/v2/types"
)

// MetadataExtractor 定义了如何从文件中提取元数据的策略
type MetadataExtractor interface {
	// ExtractMetadata 从给定的文件路径提取元数据
	ExtractMetadata(inputPath string) (types.Index, error)
}

// ContentPreprocessor 定义了如何预处理文件内容的策略
// 例如，视频需要 FFmpeg 转码，而图片或文档可能不需要任何处理
type ContentPreprocessor interface {
	// Preprocess 接收原始文件路径，返回一个处理后的内容读取器
	// 调用者负责关闭返回的 io.ReadCloser
	Preprocess(inputPath string) (io.ReadCloser, error)
}
