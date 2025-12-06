// internal/v2/plugins/registry.go

package plugins

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/v2/namer"
	"github.com/Soltus/encv-go/internal/v2/plugins/image"
	pluginInterfaces "github.com/Soltus/encv-go/internal/v2/plugins/interfaces"
	"github.com/Soltus/encv-go/internal/v2/plugins/pdf"
	"github.com/Soltus/encv-go/internal/v2/plugins/text"
	"github.com/Soltus/encv-go/internal/v2/plugins/video"
	"github.com/Soltus/encv-go/internal/v2/plugins/wps"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// Plugins 是所有已注册插件的列表，顺序代表优先级
var Plugins = []Plugin{
	&video.VideoPlugin{},
	&image.ImagePlugin{},
	&wps.WPSPlugin{},
	&pdf.PDFPlugin{},
	&text.TextPlugin{},
}

// Plugin 定义了加解密插件的完整接口
type Plugin interface {
	Intialize(ctx context.Context) error
	// --- 提供处理策略 ---
	GetMetadataExtractor() pluginInterfaces.MetadataExtractor
	GetContentPreprocessor() pluginInterfaces.ContentPreprocessor

	// --- 文件识别与处理 ---
	// GetChunkNamer 返回插件用于其容器的 ChunkNamer。
	GetChunkNamer() namer.ChunkNamer

	//  返回此插件支持的 MIME 类型前缀列表，用于匹配
	SupportedMimePrefixes() []string
	// 用于排除
	ShouldProcess(inputPath string) bool

	// --- 加密逻辑 ---

	// GetContainerExtension 返回此插件创建的容器文件扩展名
	GetContainerExtension() string
	// 加密预处理器
	PreEncryptProcessor(index types.Index, inputPath, inputRootDir, outputDir string) error
	// 将处理后的文件加密到指定容器
	Encrypt(dataReader io.Reader) error
	// 加密后处理器
	PostEncryptProcessor() error
	// Packer 打包器 TODO

	// --- 解密逻辑 ---

	//  判断此插件是否能解密给定的容器文件
	CanDecrypt(containerPath string) bool
	// Unpacker 解包器 TODO
	// 解密预处理器
	PreDecryptProcessor(containerPath, outputDir string) error
	// Decrypt 解密容器文件到指定目录
	Decrypt(containerPath, outputDir string) error
	// 解密后处理器
	PostDecryptProcessor(containerPath string) error
}

// FindEncryptingPlugin 为给定的输入文件查找合适的加密插件
func FindEncryptingPlugin(inputPath string) (Plugin, error) {
	// 1. 获取文件的 MIME 类型
	ext := strings.ToLower(filepath.Ext(inputPath))
	mimeType := mime.TypeByExtension(ext)

	// 如果无法确定 MIME 类型，则无法找到合适的插件
	if mimeType == "" {
		return nil, fmt.Errorf("could not determine MIME type for file: %s", inputPath)
	}

	// 2. 筛选出所有支持此 MIME 类型的候选插件
	var candidates []Plugin
	for _, p := range Plugins {
		for _, prefix := range p.SupportedMimePrefixes() {
			if strings.HasPrefix(mimeType, prefix) {
				candidates = append(candidates, p)
				break // 找到一个匹配的前缀就足够了，进入下一个插件
			}
		}
	}

	// 3. 在候选插件中，按注册顺序使用 ShouldProcess 进行最终确认
	for _, p := range candidates {
		if p.ShouldProcess(inputPath) {
			return p, nil
		}
	}

	// 如果没有插件通过所有检查，返回错误
	return nil, fmt.Errorf("no suitable plugin found to encrypt file: %s (MIME type: %s)", inputPath, mimeType)
}

// FindDecryptingPlugin 为给定的容器文件查找合适的解密插件
func FindDecryptingPlugin(containerPath string) (Plugin, error) {
	for _, p := range Plugins {
		if p.CanDecrypt(containerPath) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no suitable plugin found to decrypt container: %s", containerPath)
}

// ProcessFileWithPlugin 是一个通用的辅助函数，它使用插件提供的策略来处理文件。
// 这个函数封装了打开文件、提取元数据和预处理内容的通用流程。
func ProcessFileWithPlugin(p Plugin, inputPath string) (types.Index, io.ReadCloser, error) {
	// 1. 使用插件提供的元数据提取器获取索引
	extractor := p.GetMetadataExtractor()
	index, err := extractor.ExtractMetadata(inputPath)
	if err != nil {
		return nil, nil, fmt.Errorf("metadata extraction failed for '%s': %w", inputPath, err)
	}

	// 2. 使用插件提供的内容预处理器获取处理后的数据流
	preprocessor := p.GetContentPreprocessor()
	dataReader, err := preprocessor.Preprocess(inputPath)
	if err != nil {
		return nil, nil, fmt.Errorf("content preprocessing failed for '%s': %w", inputPath, err)
	}

	// 3. 返回索引和数据流
	return index, dataReader, nil
}

// EncryptFileWithPlugin 是一个新的辅助函数，封装了完整的加密流程
func EncryptFileWithPlugin(ctx context.Context, plugin Plugin, inputPath, inputRootDir, outputDir string) error {
	plugin.Intialize(ctx)

	index, dataReader, err := ProcessFileWithPlugin(plugin, inputPath)
	if err != nil {
		return fmt.Errorf("plugin failed to process file '%s': %w", inputPath, err)
	}
	defer dataReader.Close()

	// 2. 执行预处理器
	if err := plugin.PreEncryptProcessor(index, inputPath, inputRootDir, outputDir); err != nil {
		return fmt.Errorf("pre-encryption failed for '%s': %w", inputPath, err)
	}
	log.Printf("✅ [EncryptFileWithPlugin] PreEncryptProcessor successfully.\n")

	// 3. 执行核心加密
	if err := plugin.Encrypt(dataReader); err != nil {
		return fmt.Errorf("encryption failed for '%s': %w", inputPath, err)
	}
	log.Printf("✅ [EncryptFileWithPlugin] Encrypt successfully.\n")

	// 4. 执行后处理器
	if err := plugin.PostEncryptProcessor(); err != nil {
		return fmt.Errorf("post-encryption failed for '%s': %w", inputPath, err)
	}
	log.Printf("✅ [EncryptFileWithPlugin] PostEncryptProcessor successfully.\n")

	return nil
}

// DecryptContainerWithPlugin 是一个新的辅助函数，封装了完整的解密流程
func DecryptContainerWithPlugin(ctx context.Context, plugin Plugin, containerPath, outputDir string) error {
	plugin.Intialize(ctx)
	// 1. 执行预处理器
	if err := plugin.PreDecryptProcessor(containerPath, outputDir); err != nil {
		return fmt.Errorf("pre-decryption failed for '%s': %w", containerPath, err)
	}
	log.Printf("✅ [DecryptContainerWithPlugin] PreDecryptProcessor successfully.\n")

	// 2. 执行核心解密
	if err := plugin.Decrypt(containerPath, outputDir); err != nil {
		return fmt.Errorf("decryption failed for '%s': %w", containerPath, err)
	}
	log.Printf("✅ [DecryptContainerWithPlugin] Decrypt successfully.\n")

	// 4. 执行后处理器
	if err := plugin.PostDecryptProcessor(containerPath); err != nil {
		return fmt.Errorf("post-encryption failed for '%s': %w", containerPath, err)
	}
	log.Printf("✅ [DecryptContainerWithPlugin] PostDecryptProcessor successfully.\n")

	return nil
}

// 遍历文件夹自动选择插件加密，这是使用 EncryptFileWithPlugin 而不是 Plugin.EncryptFile 的原因。
func WalkAndEncrypt(ctx context.Context, walkPath string, inputRootDir, outputDir string) error {
	return filepath.WalkDir(walkPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		plugin, err := FindEncryptingPlugin(path)
		if err != nil {
			fmt.Printf("INFO: Skipping file, no handler found: %s\n", path)
			return nil
		}

		fmt.Printf("INFO: Found plugin '%T' for file: %s\n", plugin, path)
		if err := EncryptFileWithPlugin(ctx, plugin, path, inputRootDir, outputDir); err != nil {
			fmt.Printf("WARN: Failed to encrypt '%s' with plugin '%T': %v. Continuing...\n", path, plugin, err)
		}
		return nil
	})
}
