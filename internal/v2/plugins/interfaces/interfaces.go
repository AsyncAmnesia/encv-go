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

// VerifyOptions 定义 Verify 方法的可选行为参数
type VerifyOptions struct {
	SkipSizeCheck bool // 跳过精确文件大小比对（用于重编码/转码模式，此时原始文件与解密文件大小天然不同）
}

// ContentVerifier 定义了插件校验解密内容完整性的能力。
// 这是一个可选接口，插件应根据自身特性（如是否支持随机访问、文件大小）来实现。
type ContentVerifier interface {
	// Verify 校验解密后的文件与原始文件是否一致。
	// originalPath: 原始输入文件路径。
	// decryptedPath: 经过加密再解密后的文件路径（通常位于临时目录）。
	// opts: 可选的验证选项，不传时使用默认严格模式。
	// 返回 error 表示校验失败。
	Verify(originalPath, decryptedPath string, opts ...*VerifyOptions) error
}

// FragmentBuilder 定义了自定义逻辑分片策略的接口（如视频 GOP 对齐）
type FragmentBuilder interface {
	// BuildFragments 根据逻辑文件大小生成分片元数据
	BuildFragments(logicalFileSize int64) ([]types.Fragment, error)
}
