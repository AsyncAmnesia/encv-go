// pkg/encv/decrypt.go

package encv

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/service"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// Decrypt ... (保持不变) ...
func Decrypt(ctx context.Context, inputPath string, opts DecryptOptions) error {
	if err := validateDecryptOpts(opts); err != nil {
		return err
	}
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		return fmt.Errorf("failed to stat input path: %w", err)
	}

	if !info.IsDir() {
		return decryptSingleFile(ctx, inputPath, opts)
	}
	return decryptDir(ctx, inputPath, opts)
}

// decryptSingleFile 调用服务层解密单个文件集
func decryptSingleFile(ctx context.Context, anyChunkPath string, opts DecryptOptions) error {
	fmt.Printf("-> Decrypting: %s\n", anyChunkPath)
	cfg := config.FromContext(ctx)
	// 1. 调用服务层解密
	content, err := service.DecryptContainer(ctx, anyChunkPath)
	if err != nil {
		return fmt.Errorf("decryption failed for %s: %w", anyChunkPath, err)
	}
	defer content.DataStream.Close()

	// 2. 构建期望的输出文件路径
	originalFilename := content.Index.GetOriginalFilename()
	outputPath := filepath.Join(opts.OutputDir, originalFilename)

	// 4. 【关键修复】使用新的工具函数安全地创建文件
	// 它会返回实际创建的文件路径（可能已重命名）
	outputFile, actualOutputPath, err := utils.CreateFileForOutput(outputPath, cfg.Recover)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	// 5. 创建输出文件并写入解密流
	if _, err := io.Copy(outputFile, content.DataStream); err != nil {
		// 如果写入失败，删除可能已经创建的空文件
		os.Remove(actualOutputPath)
		return fmt.Errorf("failed to write decrypted data: %w", err)
	}

	fmt.Printf("✅ Success: %s\n", outputPath)

	// 4. 如果是视频，还原字幕
	if vIndex, ok := content.Index.(*types.VideoIndex); ok {
		containerDir := filepath.Dir(anyChunkPath)
		if err := service.RestoreSubtitles(vIndex, containerDir, opts.OutputDir); err != nil {
			fmt.Printf("-> Warning: Failed to restore subtitles: %v\n", err)
		}
	}

	return nil
}

// decryptDir 遍历目录解密所有容器文件
func decryptDir(ctx context.Context, inputDir string, opts DecryptOptions) error {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return fmt.Errorf("failed to read input directory: %w", err)
	}

	fmt.Printf("-> Scanning directory '%s' for ENCV container files (by magic number)...\n", inputDir)
	processedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(inputDir, entry.Name())

		// 【关键修复】使用魔法数字进行可靠检测
		isContainer, err := utils.IsEncryptedContainer(ctx, fullPath)
		if err != nil {
			// 如果是读取错误，打印警告并跳过
			fmt.Printf("-> Warning: Could not check file '%s': %v. Skipping.\n", entry.Name(), err)
			continue
		}

		if !isContainer {
			// 不是容器，跳过
			continue
		}

		// 是容器，开始解密
		processedCount++
		if err := decryptSingleFile(ctx, fullPath, opts); err != nil {
			fmt.Printf("Error decrypting '%s': %v\n", entry.Name(), err) // 打印错误但继续处理其他文件
		}
	}

	if processedCount == 0 {
		fmt.Println("-> Warning: No valid ENCV container files found in the directory.")
	} else {
		fmt.Printf("-> Found and processed %d container file(s).\n", processedCount)
	}

	return nil
}
func validateDecryptOpts(opts DecryptOptions) error {
	if opts.OutputDir == "" {
		return ErrMissingOptions
	}
	return nil
}
