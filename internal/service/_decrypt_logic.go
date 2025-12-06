// internal/service/decrypt_logic.go
// 业务逻辑层：处理文件遍历、路径解析和单个文件的解压流程。

package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/postdecrypt"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// decryptFileOrDir 判断输入是文件还是目录，并调用相应的处理函数。
func decryptFileOrDir(ctx context.Context, inputPath string, opts DecryptOptions) error {
	info, err := os.Stat(inputPath)
	if err != nil {
		return fmt.Errorf("failed to stat input path: %w", err)
	}

	if !info.IsDir() {
		return decryptSingleFile(ctx, inputPath, opts)
	}
	return decryptDir(ctx, inputPath, opts)
}

// decryptSingleFile 处理单个容器文件的完整解压流程。
func decryptSingleFile(ctx context.Context, anyChunkPath string, opts DecryptOptions) error {
	fmt.Printf("-> Decrypting: %s\n", anyChunkPath)
	cfg := config.FromContext(ctx)

	// 1. 从容器中解密内容
	content, err := DecryptContainer(ctx, anyChunkPath)
	if err != nil {
		return fmt.Errorf("decryption failed for %s: %w", anyChunkPath, err)
	}
	defer content.DataStream.Close()

	// 2. 获取原始文件名，并进行健壮性检查
	originalFilename := content.Index.GetOriginalFilename()
	finalFilename := originalFilename
	if finalFilename == "" {
		fmt.Printf("-> Warning: Original filename not found in container, deriving from container name.\n")
		finalFilename = utils.GetBaseNameWithoutExt(anyChunkPath)
	}

	// 3. 构建期望的输出文件路径
	outputPath := filepath.Join(opts.OutputDir, finalFilename)

	// 4. 安全地创建输出文件
	outputFile, actualOutputPath, err := utils.CreateFileForOutput(outputPath, cfg.Recover)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	// 5. 写入解密流
	if _, err := io.Copy(outputFile, content.DataStream); err != nil {
		os.Remove(actualOutputPath) // 失败时清理
		return fmt.Errorf("failed to write decrypted data: %w", err)
	}

	fmt.Printf("✅ Success: %s\n", actualOutputPath)

	// 6. 执行后处理（如解压关联的字幕文件）
	if err := runPostDecryption(ctx, content, anyChunkPath, opts.OutputDir); err != nil {
		// 后处理失败通常不应中断主流程，打印警告即可
		fmt.Printf("-> Post-deprocessing warning: %v\n", err)
	}

	return nil
}

// decryptDir 遍历目录，解密所有找到的容器文件。
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

		isContainer, err := utils.IsEncryptedContainer(ctx, fullPath)
		if err != nil {
			fmt.Printf("-> Warning: Could not check file '%s': %v. Skipping.\n", entry.Name(), err)
			continue
		}

		if !isContainer {
			continue
		}

		processedCount++
		if err := decryptSingleFile(ctx, fullPath, opts); err != nil {
			fmt.Printf("Error decrypting '%s': %v\n", entry.Name(), err)
		}
	}

	if processedCount == 0 {
		fmt.Println("-> Warning: No valid ENCV container files found in the directory.")
	} else {
		fmt.Printf("-> Found and processed %d container file(s).\n", processedCount)
	}

	return nil
}

// runPostDecryption 尝试获取并执行后处理器。
func runPostDecryption(ctx context.Context, content *types.DecryptedContent, containerPath, outputDir string) error {
	postProcessor, err := postdecrypt.GetPostDecrypter(content.Index)
	if err != nil {
		// 如果没有找到后处理器，说明它可能不需要后处理，这不是一个错误
		return nil
	}
	return postProcessor.PostDecrypt(ctx, content, containerPath, outputDir)
}

// findPrimaryFile 在目录中查找主文件（如视频、图片），优先于字幕等次要文件。
// 注意：此函数在当前代码库中未被调用，但作为工具函数保留，以备将来使用。
func findPrimaryFile(dir string) (string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("could not read temp directory: %w", err)
	}

	// 定义文件类型的优先级，数字越小优先级越高
	priority := map[string]int{
		// 视频
		".mp4": 1, ".mkv": 1, ".avi": 1, ".mov": 1, ".flv": 1, ".wmv": 1,
		// 图片
		".jpg": 2, ".jpeg": 2, ".png": 2, ".gif": 2, ".bmp": 2, ".webp": 2,
		// 音频
		".mp3": 3, ".wav": 3, ".flac": 3, ".aac": 3, ".ogg": 3,
		// 其他（如文本、字幕）优先级最低或忽略
	}

	var bestFile string
	bestPriority := 999 // 初始化为一个很大的值

	for _, f := range files {
		if f.IsDir() || strings.HasPrefix(f.Name(), ".") {
			continue
		}

		ext := strings.ToLower(filepath.Ext(f.Name()))
		if p, ok := priority[ext]; ok {
			if p < bestPriority {
				bestPriority = p
				bestFile = f.Name()
			}
		}
	}

	if bestFile == "" {
		// 如果没有找到任何有优先级的文件，回退到旧逻辑：打开第一个非隐藏文件
		// 这可以处理一些未知的文件类型
		for _, f := range files {
			if !f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
				return filepath.Join(dir, f.Name()), nil
			}
		}
		return "", fmt.Errorf("could not find a primary file to open in temp directory: %s", dir)
	}

	return filepath.Join(dir, bestFile), nil
}
