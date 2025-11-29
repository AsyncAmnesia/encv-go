// internal/service/decrypt.go

package service

// 解密会自动寻找解包器，新容器只需在 internal/unpacker 注册

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/postdecrypt"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/unpacker"
	"github.com/Soltus/encv-go/internal/utils"
)

// DecryptOptions 包含解密操作所需的选项。
type DecryptOptions struct {
	// OutputDir 指定解密后文件的输出目录。
	OutputDir string
}

// DecryptFileOrDir 是解密单个文件或整个目录的核心逻辑。
// 这个函数是 internal 的，不对外暴露。
func DecryptFileOrDir(ctx context.Context, inputPath string, opts DecryptOptions) error {
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
	content, err := DecryptContainer(ctx, anyChunkPath)
	if err != nil {
		return fmt.Errorf("decryption failed for %s: %w", anyChunkPath, err)
	}
	defer content.DataStream.Close()

	// 2. 获取原始文件名，并进行健壮性检查
	originalFilename := content.Index.GetOriginalFilename()

	// 【关键修复】如果原始文件名为空，则从容器文件名派生一个
	finalFilename := originalFilename
	if finalFilename == "" {
		fmt.Printf("-> Warning: Original filename not found in container, deriving from container name.\n")
		// 从容器文件名派生一个基础名，例如 "321..sccgv" -> "321"
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

	// 6. 【关键修改】使用 PostDecrypter 接口进行后处理
	postProcessor, err := postdecrypt.GetPostDecrypter(content.Index)
	if err != nil {
		// 如果没有找到后处理器，说明它可能不需要后处理，这不是一个错误
		return nil
	}

	if err := postProcessor.PostDecrypt(ctx, content, anyChunkPath, opts.OutputDir); err != nil {
		// 后处理失败通常不应中断主流程，打印警告即可
		fmt.Printf("-> Post-deprocessing warning: %v\n", err)
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

// DecryptContainer 从一个加密容器文件路径中解密内容
// 直接处理文件路径，不创建临时文件
func DecryptContainer(ctx context.Context, containerPath string) (*types.DecryptedContent, error) {
	cfg := config.FromContext(ctx)
	// 1. 【关键修改】使用新的增强检测函数
	detectedExt, err := container.DetectMainOrSubContainerType(ctx, containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect container type for %s: %w", containerPath, err)
	}

	// 2. 【关键修改】如果检测到是子分片，直接返回明确的错误
	if detectedExt == container.SubChunkType {
		return nil, fmt.Errorf("cannot decrypt a sub-chunk directly. Please provide the main container file (e.g., the file without '.encv2', '.encv3', etc.)")
	}

	// 3. 【关键修改】使用 Unpacker 接口进行解包
	unpacker, err := unpacker.GetUnpacker(detectedExt)
	if err != nil {
		return nil, fmt.Errorf("failed to get unpacker for type '%s': %w", detectedExt, err)
	}

	packedData, err := unpacker.Unpack(ctx, containerPath, detectedExt)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack container: %w", err)
	}

	// 【新增防御性检查】
	if packedData == nil || packedData.DataStream == nil {
		return nil, fmt.Errorf("internal error: unpacker returned nil data for container %s", containerPath)
	}

	// 4. 【通用逻辑】解析 KVI
	index, err := utils.UnmarshalKVI(packedData.KVIData)
	if err != nil {
		packedData.DataStream.Close()
		return nil, fmt.Errorf("failed to parse KVI: %w", err)
	}

	// 5. 【通用逻辑】创建解密流
	salt, err := crypto.Base64Decode(index.GetEncryptionInfo().SaltBase64)
	if err != nil {
		packedData.DataStream.Close()
		return nil, fmt.Errorf("failed to decode salt: %w", err)
	}
	key, err := crypto.GenerateKey([]byte(cfg.Password), salt, crypto.KeySize) // 只传入 KeySize
	if err != nil {
		packedData.DataStream.Close()
		return nil, fmt.Errorf("failed to GenerateKey: %w", err)
	}
	decryptedStream, err := crypto.GetDecryptReader(packedData.DataStream, key)
	if err != nil {
		packedData.DataStream.Close()
		return nil, fmt.Errorf("failed to create decrypt reader: %w", err)
	}

	// 6. 返回结果
	return &types.DecryptedContent{
		Index:      index,
		DataStream: newReadCloser(decryptedStream, packedData.DataStream.Close),
	}, nil
}

// ExtractKVI 从容器文件中提取原始的 KVI 数据，无需密码。
// 与 DecryptContainer 逻辑保持一致
func ExtractKVI(ctx context.Context, containerPath string) ([]byte, error) {
	// 1. 检测容器类型
	detectedExt, err := container.DetectContainerTypeFromFile(ctx, containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect container type for %s: %w", containerPath, err)
	}

	// 2. 【关键修改】通过接口获取解包器
	up, err := unpacker.GetUnpacker(detectedExt)
	if err != nil {
		return nil, fmt.Errorf("failed to get unpacker for type '%s': %w", detectedExt, err)
	}

	// 3. 解包以获取 PackedData
	packedData, err := up.Unpack(ctx, containerPath, detectedExt)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack container for KVI extraction: %w", err)
	}
	defer packedData.DataStream.Close() // 确保资源被释放

	// 4. 返回 KVI 数据
	return packedData.KVIData, nil
}

// --- 辅助函数 ---

// newReadCloser 组合一个 io.Reader 和一个 Close 函数
type readCloser struct {
	io.Reader
	closeFunc func() error
}

func (rc *readCloser) Close() error {
	return rc.closeFunc()
}

func newReadCloser(r io.Reader, closeFunc func() error) io.ReadCloser {
	return &readCloser{Reader: r, closeFunc: closeFunc}
}
