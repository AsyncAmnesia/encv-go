// internal/service/decrypt_service.go
// 公共 API 层：提供解密服务，封装了所有解密相关的操作。

package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/utils"
)

// DecryptMode 定义解压的模式
type DecryptMode string

const (
	// ModePreview 预览模式：解压到临时目录并用默认程序打开
	ModePreview DecryptMode = "preview"
	// ModeToFolder 解压到指定文件夹
	ModeToFolder DecryptMode = "to-folder"
	// ModeHere 解压到当前文件夹
	ModeHere DecryptMode = "here"
	// ModeToSubfolder 解压到同名文件夹
	ModeToSubfolder DecryptMode = "to-subfolder"
)

// DecryptOptions 包含解密操作所需的选项。
type DecryptOptions struct {
	// OutputDir 指定解密后文件的输出目录。
	OutputDir string
	Mode      DecryptMode // 解压模式
}

// Decrypter 提供解密服务。
type Decrypter struct{}

// NewDecrypter 创建一个新的 Decrypter 实例。
func NewDecrypter() *Decrypter {
	return &Decrypter{}
}

// Decrypt 根据 DecryptOptions 执行解密操作。
// 它支持解密单个文件或整个目录，并根据模式决定输出路径。
func (d *Decrypter) Decrypt(ctx context.Context, inputPath string, opts DecryptOptions) error {
	// 1. 根据模式确定最终的输出目录
	finalOutputDir, err := resolveOutputDir(inputPath, opts)
	if err != nil {
		return fmt.Errorf("failed to resolve output directory: %w", err)
	}
	// 确保输出目录存在
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", opts.OutputDir, err)
	}
	// 2. 创建包含最终路径的选项
	finalOpts := opts
	finalOpts.OutputDir = finalOutputDir

	// 3. 调用核心逻辑执行解密
	return decryptFileOrDir(ctx, inputPath, finalOpts)
}

// resolveOutputDir 根据 DecryptMode 计算出最终的输出目录。
func resolveOutputDir(inputPath string, opts DecryptOptions) (string, error) {
	switch opts.Mode {
	case ModeToFolder:
		if opts.OutputDir == "" {
			return "", fmt.Errorf("output directory must be specified for 'to-folder' mode")
		}
		return opts.OutputDir, nil
	case ModeHere:
		return filepath.Dir(inputPath), nil
	case ModeToSubfolder:
		baseName := utils.GetBaseNameWithoutExt(inputPath)
		return filepath.Join(filepath.Dir(inputPath), baseName), nil
	default:
		// 默认行为，为了向后兼容
		return opts.OutputDir, nil
	}
}
