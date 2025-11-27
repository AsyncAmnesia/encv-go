// internal/service/encrypt.go (完全重写)

package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/container/chunked"
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/processor"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// EncryptFile 加密单个文件，并根据类型和配置进行打包
func EncryptFile(inputPath, outputDir, password string, salt []byte, trackExtensions []string) error {
	// 【修改 1】提取文件名、基础名和原始后缀
	filename := filepath.Base(inputPath)
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
	originalExt := filepath.Ext(filename) // 例如 ".jpg" 或 ".mp4"

	// 1. 检测文件类型
	mimeType, err := utils.DetectFileMIMEType(inputPath)
	if err != nil {
		return fmt.Errorf("failed to detect MIME type: %w", err)
	}

	// 2. 根据类型分发处理
	switch {
	case utils.IsVideoType(mimeType):
		return encryptVideo(inputPath, baseName, originalExt, outputDir, password, salt, trackExtensions)
	case utils.IsImageType(mimeType):
		return encryptImage(inputPath, baseName, originalExt, outputDir, password, salt)
	default:
		return fmt.Errorf("unsupported file type: %s", mimeType)
	}
}

// encryptVideo 处理视频加密和打包
func encryptVideo(inputPath, baseName, originalExt, outputDir, password string, salt []byte, trackExtensions []string) error {
	fmt.Printf("--> Starting encryption for video: %s\n", baseName)

	// 1. 【分析阶段】获取视频元数据
	metadata, err := processor.ProcessVideo(inputPath)
	if err != nil {
		return fmt.Errorf("video analysis failed: %w", err)
	}

	// 2. 【分析阶段】发现字幕轨道
	subtitleInfos, err := utils.DiscoverSubtitleTracks(inputPath, trackExtensions)
	if err != nil {
		return fmt.Errorf("subtitle discovery failed: %w", err)
	}

	// 3. 【加密阶段】加密预处理后的视频流
	tempPreprocessedPath, err := processor.PreprocessVideoWithFFmpeg(inputPath)
	if err != nil {
		return fmt.Errorf("pre-processing failed: %w", err)
	}
	defer os.Remove(tempPreprocessedPath)

	tempEncPath := filepath.Join(outputDir, baseName+".tmp_enc")
	iv, err := crypto.EncryptFile(tempPreprocessedPath, tempEncPath, password, salt)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}
	defer os.Remove(tempEncPath)

	// 4. 【字幕处理阶段】复制并重命名字幕文件
	// 【关键修正】计算出加密视频的基础名，用于字幕重命名
	reversedExt := utils.GenerateReversedExt(originalExt)
	encBaseName := baseName + "." + reversedExt

	kviSubtitleTracks, err := copyAndRenameSubtitles(subtitleInfos, inputPath, outputDir, encBaseName)
	if err != nil {
		return fmt.Errorf("failed to process subtitles: %w", err)
	}

	// 5. 【打包阶段】构建 VideoIndex
	index := &types.VideoIndex{
		Kind:             types.IndexKindVideo,
		Version:          types.KviVersion,
		VideoID:          fmt.Sprintf("vid-%d", os.Getuid()),
		OriginalFileSize: metadata.OriginalFileSize,
		Format:           metadata.Format,
		MimeType:         metadata.MimeType,
		Encryption: types.EncryptionInfo{
			Algorithm:  crypto.Algorithm,
			IVBase64:   crypto.Base64Encode(iv),
			SaltBase64: crypto.Base64Encode(salt),
		},
		SeekTable:        []interface{}{}, // TODO
		DurationSeconds:  metadata.DurationSeconds,
		Resolution:       metadata.Resolution,
		OriginalFilename: filepath.Base(inputPath),
		Subtitles:        kviSubtitleTracks,
	}

	// 6. 【打包阶段】根据配置选择打包方式
	finalPath := filepath.Join(outputDir, encBaseName+config.GetVideoEncExtension())

	if config.IsSccgvChunkingEnabled() {
		return createChunkedContainer(tempEncPath, finalPath, index, inputPath)
	}
	return container.PackWithIndex(tempEncPath, finalPath, index)
}

// encryptImage 处理图像加密和打包
func encryptImage(inputPath, baseName, originalExt, outputDir, password string, salt []byte) error {
	// 1. 调用 processor 获取图像信息
	info, err := processor.ProcessImage(inputPath)
	if err != nil {
		return fmt.Errorf("processor failed: %w", err)
	}

	// 2. 加密原始图像流
	tempEncPath := filepath.Join(outputDir, baseName+".tmp_enc")
	iv, err := crypto.EncryptFile(inputPath, tempEncPath, password, salt)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}
	defer os.Remove(tempEncPath)

	// 3. 构建 ImageIndex
	index := &types.ImageIndex{
		Kind:             types.IndexKindImage,
		Version:          types.KviVersion,
		OriginalFilename: filepath.Base(inputPath),
		Encryption: types.EncryptionInfo{
			Algorithm:  crypto.Algorithm,
			IVBase64:   crypto.Base64Encode(iv),
			SaltBase64: crypto.Base64Encode(salt),
		},
		Width:            info.Width,
		Height:           info.Height,
		Format:           info.Format,
		MimeType:         info.MimeType,
		OriginalFileSize: info.OriginalFileSize,
	}

	// 4. 【修改 4】为图像容器生成带倒序后缀的最终路径
	reversedExt := utils.GenerateReversedExt(originalExt)
	finalPath := filepath.Join(outputDir, baseName+"."+reversedExt+config.GetImageEncExtension())
	return container.PackWithIndex(tempEncPath, finalPath, index)
}

// copyAndRenameSubtitles 复制并重命名字幕文件，并更新 subtitleTracks 中的 Title
func copyAndRenameSubtitles(subtitleTracks []types.SubtitleTrack, videoPath, outputDir, encBaseName string) ([]types.SubtitleTrack, error) {
	if len(subtitleTracks) == 0 {
		return nil, nil
	}

	videoDir := filepath.Dir(videoPath)
	// 排序以确保命名一致
	sort.Slice(subtitleTracks, func(i, j int) bool {
		return subtitleTracks[i].Filename < subtitleTracks[j].Filename
	})

	var kviTracks []types.SubtitleTrack
	for i, track := range subtitleTracks {
		originalPath := filepath.Join(videoDir, track.Filename)
		subExt := filepath.Ext(track.Filename) // 例如 ".srt"

		// 【关键逻辑】生成新的字幕文件名: 加密视频基础名 + 原始字幕后缀
		// 例如: my_movie.4pm.srt
		newFilename := encBaseName + subExt
		if i > 0 { // 如果有多个字幕，用语言或序号区分
			langTag := track.Language
			if langTag == "" {
				langTag = fmt.Sprintf("%d", i+1)
			}
			newFilename = encBaseName + "." + langTag + subExt
		}

		newPath := filepath.Join(outputDir, newFilename)
		if err := utils.CopyFile(originalPath, newPath); err != nil {
			return nil, fmt.Errorf("failed to copy subtitle from %s to %s: %w", originalPath, newPath, err)
		}
		fmt.Printf("-> Subtitle renamed: %s\n", newFilename)

		// 更新 KVI 中的字幕轨道信息
		kviTracks = append(kviTracks, types.SubtitleTrack{
			Language: track.Language,
			Title:    newFilename,    // KVI 中存储重命名后的文件名
			Filename: track.Filename, // 保留原始文件名
		})
	}
	return kviTracks, nil
}

// createChunkedContainer 创建分片容器
func createChunkedContainer(encryptedPath, finalPath string, index *types.VideoIndex, originalVideoPath string) error {
	// 1. 打开加密文件，并确保它支持 Seek
	encFile, err := os.Open(encryptedPath)
	if err != nil {
		return fmt.Errorf("failed to open encrypted file for chunking: %w", err)
	}
	defer encFile.Close()

	// 2. 获取配置和魔法数字
	chunkSize := config.GetSccgvChunkSize()
	magicMap := container.GetContainerMagicMap()
	subMagicMap := container.GetSubChunkMagicMap()
	mainMagic := magicMap[config.GlobalConfig.BinExtGroup.Video]
	subMagic := subMagicMap[config.GlobalConfig.BinExtGroup.Video]

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

		log.Printf("-> [Chunking Debug] Successfully created sub-chunk %d: %s (size: %d, md5: %s)", chunkIndex, filename, written, md5)

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
