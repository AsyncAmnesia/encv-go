// pkg/encv/encrypt.go

package encv

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/service"
	"github.com/Soltus/encv-go/internal/utils"
)

// Encrypt 加密单个视频文件或整个目录。
func Encrypt(inputPath string, opts EncryptOptions) error {
	if err := validateEncryptOpts(opts); err != nil {
		return err
	}
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
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
		return encryptDir(inputPath, opts, salt)
	}
	return encryptSingleFile(inputPath, opts, salt)
}

// encryptSingleFile 调用服务层加密单个文件
func encryptSingleFile(inputPath string, opts EncryptOptions, salt []byte) error {
	fmt.Printf("-> Encrypting: %s\n", inputPath)
	// 【关键修复】传入 TrackExtensions
	if err := service.EncryptFile(inputPath, opts.OutputDir, opts.Password, salt, opts.TrackExtensions); err != nil {
		return fmt.Errorf("encryption failed for %s: %w", inputPath, err)
	}
	fmt.Printf("✅ Success: %s\n", inputPath)
	return nil
}

// encryptDir 遍历目录加密所有支持的文件
func encryptDir(inputDir string, opts EncryptOptions, salt []byte) error {
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
		if err := encryptSingleFile(fullPath, opts, salt); err != nil {
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

func validateEncryptOpts(opts EncryptOptions) error {
	if opts.Password == "" || opts.OutputDir == "" {
		return ErrMissingOptions
	}
	return nil
}
