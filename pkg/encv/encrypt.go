// pkg/encv/encrypt.go

package encv

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/service"
	"github.com/Soltus/encv-go/internal/utils"
)

// Encrypt 加密单个视频文件或整个目录。
func Encrypt(ctx context.Context, inputPath string) error {
	cfg := config.FromContext(ctx)
	if err := os.MkdirAll(cfg.OutputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	salt := make([]byte, crypto.SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		return fmt.Errorf("failed to stat input path: %w", err)
	}

	if info.IsDir() {
		return encryptDir(ctx, inputPath, salt)
	}
	return encryptSingleFile(ctx, inputPath, salt)
}

// encryptSingleFile 调用服务层加密单个文件
func encryptSingleFile(ctx context.Context, inputPath string, salt []byte) error {
	fmt.Printf("-> Encrypting: %s\n", inputPath)
	cfg := config.FromContext(ctx)
	if err := service.EncryptFile(ctx, inputPath, cfg.OutputPath, salt); err != nil {
		return fmt.Errorf("encryption failed for %s: %w", inputPath, err)
	}
	fmt.Printf("✅ Success: %s\n", inputPath)
	return nil
}

// encryptDir 遍历目录加密所有支持的文件
func encryptDir(ctx context.Context, inputDir string, salt []byte) error {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return fmt.Errorf("failed to read input directory: %w", err)
	}

	fmt.Printf("-> Scanning directory '%s' for supported files...\n", inputDir)
	supportedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// 【关键修复】先构建完整路径
		fullPath := filepath.Join(inputDir, entry.Name())

		// 【关键修复】使用完整路径进行检测
		if !utils.IsSupportedFile(fullPath) {
			// fmt.Printf("-> Skipping unsupported file: %s\n", entry.Name()) // 取消注释以查看跳过的文件
			continue
		}

		supportedCount++
		if err := encryptSingleFile(ctx, fullPath, salt); err != nil {
			fmt.Printf("Error: %v\n", err) // 打印错误但继续处理其他文件
		}
	}

	if supportedCount == 0 {
		fmt.Println("-> Warning: No supported video or image files found in the directory.")
	} else {
		fmt.Printf("-> Found and processed %d supported file(s).\n", supportedCount)
	}

	return nil
}
