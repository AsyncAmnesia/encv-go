// internal/service/encrypt.go (完全重写)

package service

import (
	"context"
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
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// EncryptFile 加密单个文件，并根据类型和配置进行打包
func EncryptFile(ctx context.Context, inputPath string, outputDir string, salt []byte) error {
	// 【修改 1】提取文件名、基础名和原始后缀
	filename := filepath.Base(inputPath)
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
	originalExt := filepath.Ext(filename) // 例如 ".jpg" 或 ".mp4"

	// 1. 检测文件类型
	mimeType, err := utils.DetectFileMIMEType(inputPath)
	if err != nil {
		return fmt.Errorf("failed to detect MIME type: %w", err)
	}

	// 2. 根据类型分发处理加密
	switch {
	case utils.IsVideoType(mimeType):
		return encryptVideo(ctx, inputPath, baseName, originalExt, outputDir, salt)
	case utils.IsImageType(mimeType):
		return encryptImage(ctx, inputPath, baseName, originalExt, outputDir, salt)
	case utils.IsTextType(mimeType):
		return encryptText(ctx, inputPath, baseName, originalExt, outputDir, salt)
	default:
		return fmt.Errorf("unsupported file type: %s", mimeType)
	}
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
		filename, md5, written, err := chunked.WriteSubChunk(subMagic, finalPath, chunkIndex, dataReader, originalMD5)
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

	if err := chunked.WriteMainChunk(mainMagic, finalPath, kviData, mainDataReader, originalMD5); err != nil {
		return fmt.Errorf("failed to write main chunk: %w", err)
	}

	return nil
}
