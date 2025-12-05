// internal/v2/plugins/registry.go

package plugins

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	// "github.com/Soltus/encv-go/internal/v2/plugins/generic"
	"github.com/Soltus/encv-go/internal/v2/namer"
	"github.com/Soltus/encv-go/internal/v2/plugins/video"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// Plugin 定义了加解密插件的完整接口
type Plugin interface {
	Intialize(ctx context.Context) error
	// --- 文件识别与处理 ---
	// GetChunkNamer 返回插件用于其容器的 ChunkNamer。
	GetChunkNamer() namer.ChunkNamer

	// SupportedMimePrefixes 返回此插件支持的 MIME 类型前缀列表
	SupportedMimePrefixes() []string
	// ShouldProcess 对路径进行最终检查，以决定是否处理
	ShouldProcess(inputPath string) bool
	// ProcessFile 处理单个文件，返回索引和文件数据的读取器
	ProcessFile(inputPath string) (types.Index, io.ReadCloser, error)

	// --- 加密逻辑 ---

	// GetContainerExtension 返回此插件创建的容器文件扩展名
	GetContainerExtension() string
	// 加密预处理器
	PreEncryptProcessor(index types.Index, inputPath, inputRootDir, outputDir string) error
	// 将处理后的文件加密到指定容器
	Encrypt(index types.Index, dataReader io.Reader, inputPath, inputRootDir, outputDir string) error
	// 加密后处理器
	PostEncryptProcessor(index types.Index, outputDir string) error
	// Packer 打包器 TODO

	// --- 解密逻辑 ---

	// CanDecrypt 判断此插件是否能解密给定的容器文件
	CanDecrypt(containerPath string) bool
	// Unpacker 解包器 TODO
	// 解密预处理器
	PreDecryptProcessor(containerPath, outputDir string) error
	// Decrypt 解密容器文件到指定目录
	Decrypt(containerPath, outputDir string) error
	// 解密后处理器
	PostDecryptProcessor(containerPath, outputDir string) error
}

// Plugins 是所有已注册插件的列表，顺序代表优先级
var Plugins = []Plugin{
	&video.VideoPlugin{}, // 从 plugins/video 包导入
	// &generic.GenericPlugin{}, // 从 plugins/generic 包导入
}

// FindEncryptingPlugin 为给定的输入文件查找合适的加密插件
func FindEncryptingPlugin(inputPath string) (Plugin, error) {
	for _, p := range Plugins {
		if p.ShouldProcess(inputPath) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no suitable plugin found to encrypt file: %s", inputPath)
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

// EncryptFileWithPlugin 是一个新的辅助函数，封装了完整的加密流程
func EncryptFileWithPlugin(ctx context.Context, plugin Plugin, inputPath, inputRootDir, outputDir string) error {
	plugin.Intialize(ctx)
	// 1. 处理文件，获取 Index 和 DataReader
	index, dataReader, err := plugin.ProcessFile(inputPath)
	if err != nil {
		return fmt.Errorf("plugin failed to process file '%s': %w", inputPath, err)
	}
	defer dataReader.Close()

	// 2. 执行预处理器
	if err := plugin.PreEncryptProcessor(index, inputPath, inputRootDir, outputDir); err != nil {
		return fmt.Errorf("pre-encryption failed for '%s': %w", inputPath, err)
	}

	// 3. 执行核心加密
	if err := plugin.Encrypt(index, dataReader, inputPath, inputRootDir, outputDir); err != nil {
		return fmt.Errorf("encryption failed for '%s': %w", inputPath, err)
	}

	// 4. 执行后处理器
	if err := plugin.PostEncryptProcessor(index, outputDir); err != nil {
		return fmt.Errorf("post-encryption failed for '%s': %w", inputPath, err)
	}

	return nil
}

// DecryptContainerWithPlugin 是一个新的辅助函数，封装了完整的解密流程
func DecryptContainerWithPlugin(ctx context.Context, plugin Plugin, containerPath, outputDir string) error {
	plugin.Intialize(ctx)
	// 1. 执行预处理器
	if err := plugin.PreDecryptProcessor(containerPath, outputDir); err != nil {
		return fmt.Errorf("pre-decryption failed for '%s': %w", containerPath, err)
	}

	// 2. 执行核心解密
	if err := plugin.Decrypt(containerPath, outputDir); err != nil {
		return fmt.Errorf("decryption failed for '%s': %w", containerPath, err)
	}
	// 4. 执行后处理器
	if err := plugin.PostDecryptProcessor(containerPath, outputDir); err != nil {
		return fmt.Errorf("post-encryption failed for '%s': %w", containerPath, err)
	}

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
