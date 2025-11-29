// internal/service/encrypt.go (完全重写)

package service

// 加密会自动寻找打包器，新容器只需在 internal/packer 注册

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/container/chunked"
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
// 【保留您的逻辑，但调用重构后的 doEncrypt】
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

	// 3. 使用处理器获取文件元数据
	index, err := p.Process(inputPath)
	if err != nil {
		return fmt.Errorf("failed to process file '%s': %w", inputPath, err)
	}

	// 4. 调用重构后的通用加密逻辑
	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	return doEncrypt(ctx, inputPath, baseName, outputDir, salt, index)
}

// doEncrypt 是核心的加密函数，它处理所有实现了 types.Index 的对象
// 【关键重构在这里】
func doEncrypt(ctx context.Context, inputPath, baseName, outputDir string, salt []byte, index types.Index) error {
	cfg := config.FromContext(ctx)

	// --- 2. 通用加密 ---
	tempEncPath := filepath.Join(outputDir, baseName+".tmp_enc")
	iv, err := crypto.EncryptFile(inputPath, tempEncPath, cfg.Password, salt)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}
	defer os.Remove(tempEncPath)

	// --- 3. 【关键修改】填充所有 Index 共有的加密后信息，消灭 switch ---
	encMD5, _ := utils.FileMD5(tempEncPath)
	encInfo := types.EncryptionInfo{
		Algorithm:  crypto.Algorithm,
		IVBase64:   crypto.Base64Encode(iv),
		SaltBase64: crypto.Base64Encode(salt),
	}
	// 使用我们商定的统一接口，一行搞定
	index.UpdateCommonInfo(encInfo, filepath.Base(inputPath), encMD5)

	// --- 4. 【关键修改】使用 Packer 接口进行打包 ---
	packer, err := packer.GetPacker(index)
	if err != nil {
		return fmt.Errorf("failed to get packer for type '%s': %w", index.GetKind(), err)
	}

	encFile, err := os.Open(tempEncPath)
	if err != nil {
		return fmt.Errorf("failed to open encrypted temp file for packing: %w", err)
	}
	defer encFile.Close()

	// 调用 Packer，所有路径生成和打包细节都由它处理
	if err := packer.Pack(ctx, baseName, outputDir, encFile, index); err != nil {
		return fmt.Errorf("failed to pack container: %w", err)
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

// createChunkedContainer 创建分片容器
func createChunkedContainer(ctx context.Context, encryptedPath, finalPath string, index *types.VideoIndex, originalVideoPath string) error {
	cfg := config.FromContext(ctx)
	// 1. 打开加密文件，并确保它支持 Seek
	encFile, err := os.Open(encryptedPath)
	if err != nil {
		return fmt.Errorf("failed to open encrypted file for chunking: %w", err)
	}
	defer encFile.Close()

	// 2. 获取配置和魔法数字
	chunkSize := cfg.GetSccgvChunkSizeBytes()
	magicMap, err := container.GetContainerMagicMap(ctx)
	subMagicMap, err := container.GetSubChunkMagicMap(ctx)
	mainMagic := magicMap[cfg.BinExtGroup.Video] // 后续其他分片容器这里需要修改
	subMagic := subMagicMap[cfg.BinExtGroup.Video]

	// 计算原始文件的 MD5 (用于所有分片头)
	originalMD5, err := utils.FileMD5(originalVideoPath)
	if err != nil {
		return fmt.Errorf("failed to calculate original file MD5: %w", err)
	}
	// 【调试】获取加密文件大小，用于边界检查
	encFileInfo, err := encFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat encrypted file: %w", err)
	}
	encFileSize := encFileInfo.Size()
	log.Printf("-> [Chunking Debug] Encrypted file size: %d bytes", encFileSize)
	log.Printf("-> [Chunking Debug] Original file size: %d bytes", index.OriginalFileSize)

	// 3. 【关键改动】先创建所有子分片，并收集元数据
	var subChunks []types.SubChunkInfo
	chunkIndex := 2

	for {
		// 计算当前子分片在加密文件中的偏移量
		offset := int64(chunkIndex-1) * int64(chunkSize)

		// 【关键修复】检查偏移量是否已超出文件范围
		if offset >= encFileSize {
			log.Printf("-> [Chunking Debug] Offset %d is beyond file size %d. Chunking finished.", offset, encFileSize)
			break
		}

		// Seek 到指定位置
		_, err := encFile.Seek(offset, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek in encrypted file to offset %d: %w", offset, err)
		}

		// 限制读取器为当前分片的大小
		dataReader := io.LimitReader(encFile, int64(chunkSize))

		// 调用修改后的 WriteSubChunk，获取元数据
		filename, md5, written, err := chunked.WriteSubChunk([]byte(subMagic), finalPath, chunkIndex, dataReader, originalMD5)
		if err != nil {
			log.Printf("!!! [Chunking Warning] Failed to write sub-chunk %d (size: %d): %v. Skipping this chunk.", chunkIndex, written, err)
			// 如果是写入 0 字节，说明真的到文件末尾了，可以退出
			if written == 0 {
				break
			}
			// 否则，继续尝试下一个分片
			chunkIndex++
			continue
		}

		// 如果写入大小为0，说明已经到达文件末尾，退出循环
		if written == 0 {
			log.Printf("-> [Chunking Debug] Wrote 0 bytes for chunk %d, assuming end of file.", chunkIndex)
			break
		}

		// log.Printf("-> [Chunking Debug] Successfully created sub-chunk %d: %s (size: %d, md5: %s)", chunkIndex, filename, written, md5)

		// 收集子分片信息
		subChunks = append(subChunks, types.SubChunkInfo{
			Index:    chunkIndex,
			Filename: filename,
			MD5:      md5,
		})

		chunkIndex++
	}

	// 4. 【关键改动】将收集到的子分片信息存入 KVI
	index.SubChunks = subChunks

	// 5. 【关键改动】现在 KVI 完整了，序列化它
	kviData, err := json.Marshal(index)
	if err != nil {
		return fmt.Errorf("failed to marshal VideoIndex to JSON: %w", err)
	}

	// 6. 【关键改动】最后，写入主分片
	// 将文件指针重置到文件开头
	_, err = encFile.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek to start of encrypted file: %w", err)
	}

	// 主分片的数据是加密文件的第一个 chunk
	mainDataReader := io.LimitReader(encFile, int64(chunkSize))

	if err := chunked.WriteMainChunk([]byte(mainMagic), finalPath, kviData, mainDataReader, originalMD5); err != nil {
		return fmt.Errorf("failed to write main chunk: %w", err)
	}

	return nil
}
