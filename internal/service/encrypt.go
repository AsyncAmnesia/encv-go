// internal/service/encrypt.go (完全重写)

package service

// 加密会自动寻找打包器，新容器只需在 internal/packer 注册

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/packer"
	"github.com/Soltus/encv-go/internal/processor"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// Encrypt 是加密文件或目录的统一入口。
func Encrypt(ctx context.Context, inputPath string) error {
	cfg := config.FromContext(ctx)
	if err := os.MkdirAll(cfg.OutputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	salt := generateSalt()

	info, err := os.Stat(inputPath)
	if err != nil {
		return fmt.Errorf("failed to stat input path: %w", err)
	}

	if info.IsDir() {
		return encryptDir(ctx, inputPath, salt)
	}
	return encryptSingleFile(ctx, inputPath, salt)
}

// encryptDirectory 遍历目录加密所有支持的文件。
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

		fullPath := filepath.Join(inputDir, entry.Name())

		// 【关键修复】先检测 MIME 类型，再判断是否支持
		mimeType, err := utils.DetectFileMIMEType(fullPath)
		if err != nil {
			fmt.Printf("-> Skipping file '%s' due to MIME detection error: %v\n", fullPath, err)
			continue
		}

		if !processor.IsMimeTypeSupported(mimeType) {
			// fmt.Printf("-> Skipping unsupported file: %s (MIME: %s)\n", entry.Name(), mimeType) // 取消注释以进行调试
			continue
		}

		supportedCount++
		if err := encryptSingleFile(ctx, fullPath, salt); err != nil {
			fmt.Printf("Error: %v\n", err) // 打印错误但继续处理其他文件
		}
	}

	if supportedCount == 0 {
		fmt.Println("-> Warning: No supported files found in the directory.")
	} else {
		fmt.Printf("-> Found and processed %d supported file(s).\n", supportedCount)
	}

	return nil
}

// encryptSingleFile 调用服务层加密单个文件
func encryptSingleFile(ctx context.Context, inputPath string, salt []byte) error {
	fmt.Printf("-> Encrypting: %s\n", inputPath)
	cfg := config.FromContext(ctx)
	// 直接调用您原有的 EncryptFile 逻辑，并传入 salt
	if err := EncryptFile(ctx, inputPath, cfg.OutputPath, salt); err != nil {
		return fmt.Errorf("encryption failed for %s: %w", inputPath, err)
	}
	fmt.Printf("✅ Success: %s\n", inputPath)
	return nil
}

// EncryptFile 是加密单个文件的统一入口。
func EncryptFile(ctx context.Context, inputPath string, outputDir string, salt []byte) error {
	// 1. 检测文件的真实 MIME 类型
	mimeType, err := utils.DetectFileMIMEType(inputPath)
	if err != nil {
		return fmt.Errorf("failed to detect MIME type for '%s': %w", inputPath, err)
	}

	// 2. 使用 MIME 类型获取正确的处理器
	p, err := processor.GetProcessor(mimeType)
	if err != nil {
		log.Printf("Skipping file '%s' (MIME: %s): %v", inputPath, mimeType, err)
		return nil // 返回 nil 以便批量处理继续
	}

	// 即使 MIME 类型匹配，也要让处理器自己决定是否要处理这个文件
	if !p.ShouldProcess(inputPath) {
		log.Printf("Skipping file '%s' as it's excluded by the processor's rules.", inputPath)
		return nil
	}

	// 3. 【关键】从处理器获取元数据和数据流
	index, sourceReader, err := p.Process(inputPath)
	if err != nil {
		return fmt.Errorf("failed to process file '%s': %w", inputPath, err)
	}
	// 【关键】确保流被关闭，以释放文件句柄或等待进程结束
	defer sourceReader.Close()

	if err := types.ValidateIndex(index, "After Processor"); err != nil {
		return err
	}

	// 4. 调用重构后的通用加密逻辑
	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	return doEncrypt(ctx, sourceReader, baseName, outputDir, salt, index)
}

// doEncrypt 是核心的加密函数，它处理所有实现了 types.Index 的对象
// 【关键重构在这里】
func doEncrypt(ctx context.Context, sourceReader io.Reader, baseName, outputDir string, salt []byte, index types.Index) error {
	cfg := config.FromContext(ctx)

	// --- 2. 通用加密 (流式) ---
	// 加密仍然需要写入临时文件，因为打包器需要多次 Seek 和读取
	tempEncPath := filepath.Join(outputDir, baseName+".tmp_enc")
	tempEncFile, err := os.Create(tempEncPath)
	if err != nil {
		return fmt.Errorf("failed to create temp encrypted file: %w", err)
	}
	defer os.Remove(tempEncPath) // 确保函数退出时删除临时文件
	defer tempEncFile.Close()

	// 【关键修改】调用新的 EncryptStream，并获取返回的 IV
	iv, err := crypto.EncryptStream(sourceReader, tempEncFile, []byte(cfg.Password), salt)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	// --- 3. 填充所有 Index 共有的加密后信息 ---
	encMD5, _ := utils.FileMD5(tempEncPath)
	encInfo := types.EncryptionInfo{
		Algorithm:  crypto.Algorithm,
		IVBase64:   crypto.Base64Encode(iv),
		SaltBase64: crypto.Base64Encode(salt),
	}
	index.UpdateCommonInfo(encInfo, index.GetOriginalFilename(), encMD5)

	if err := types.ValidateIndex(index, "After Encrypt"); err != nil {
		return err
	}

	// 从 index 中获取原始文件名
	originalFilename := index.GetOriginalFilename()

	// 使用工具函数获取干净的基础名和反转扩展名
	cleanBaseName := utils.GetBaseNameWithoutExt(originalFilename)
	originalExt := filepath.Ext(originalFilename)
	reversedExt := ""
	if originalExt != "" {
		reversedExt = utils.GenerateReversedExt(originalExt)
	}

	// 构建最终的 BaseName，它将被传递给 Packer
	finalBaseName := fmt.Sprintf("%s.%s", cleanBaseName, reversedExt)

	// --- 4. 使用 Packer 接口进行打包 ---
	packer, err := packer.GetPacker(index)
	if err != nil {
		return fmt.Errorf("failed to get packer for type '%s': %w", index.GetKind(), err)
	}
	// 重新打开临时加密文件以供打包器读取
	encFile, err := os.Open(tempEncPath)
	if err != nil {
		return fmt.Errorf("failed to open encrypted temp file for packing: %w", err)
	}
	defer encFile.Close()

	// 调用 Packer，所有路径生成和打包细节都由它处理
	if err := packer.Pack(ctx, finalBaseName, outputDir, encFile, index); err != nil {
		return fmt.Errorf("failed to pack container: %w", err)
	}

	if err := types.ValidateIndex(index, "After Pack"); err != nil {
		return err
	}
	return nil
}

// generateSalt 生成加密用的盐。
func generateSalt() []byte {
	salt := make([]byte, crypto.SaltSize)
	if _, err := rand.Read(salt); err != nil {
		// 这是一个严重错误，应该 panic
		panic(fmt.Sprintf("failed to generate salt: %v", err))
	}
	return salt
}
