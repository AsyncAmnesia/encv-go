package packer

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"log"
// 	"os"
// 	"path/filepath"
// 	"sort"

// 	"github.com/Soltus/encv-go/internal/config"
// 	"github.com/Soltus/encv-go/internal/container"
// 	"github.com/Soltus/encv-go/internal/container/chunked"
// 	"github.com/Soltus/encv-go/internal/types"
// 	"github.com/Soltus/encv-go/internal/utils"
// )

// type VideoPacker struct {
// 	BasePacker
// }

// func (p *VideoPacker) Pack(ctx context.Context, baseName, outputDir string, encryptedDataReader io.Reader, index types.Index) error {
// 	vIndex, ok := index.(*types.VideoIndex)
// 	if !ok {
// 		return fmt.Errorf("VideoPacker received an invalid index type: %T", index)
// 	}

// 	cfg := config.FromContext(ctx)

// 	// 1. 视频特有逻辑：字幕处理
// 	if err := p.handleSubtitles(ctx, vIndex, baseName, outputDir); err != nil {
// 		return err // 字幕处理失败是致命错误
// 	}

// 	finalPath := filepath.Join(outputDir, baseName+cfg.GetVideoEncExtension())

// 	// 3. 决策：获取魔法数字
// 	magicMap, _ := container.GetContainerMagicMap(ctx)
// 	subMagicMap, _ := container.GetSubChunkMagicMap(ctx)
// 	mainMagic := magicMap[cfg.BinExtGroup.Video]
// 	subMagic := subMagicMap[cfg.BinExtGroup.Video]
// 	chunkSize := cfg.GetSccgvChunkSizeBytes()

// 	// 委托：调用基类的工具方法完成打包
// 	if chunkSize > 0 {
// 		// 【修复 1】不再在这里序列化 KVI，而是将 vIndex 传递给 WriteAllChunks
// 		// WriteAllChunks 内部会负责创建子分片、更新 vIndex、序列化 KVI 并写入主分片
// 		return p.WriteAllChunks(chunkSize, finalPath, []byte(mainMagic), []byte(subMagic), vIndex, encryptedDataReader, vIndex.OriginalFileMD5)
// 	} else {
// 		kviData, err := json.Marshal(index)
// 		if err != nil {
// 			return fmt.Errorf("failed to marshal KVI: %w", err)
// 		}
// 		return p.WriteSingleFileContainer(finalPath, []byte(mainMagic), kviData, encryptedDataReader)
// 	}
// }

// // 将 KVI 和数据流打包成一个分块容器。
// func (p *VideoPacker) WriteAllChunks(chunkSize int64, finalPath string, mainMagic, subMagic []byte, index *types.VideoIndex, encryptedDataReader io.Reader, originalMD5 string) error {

// 	// 1. 将数据流暂存到临时文件
// 	tmpFile, err := os.CreateTemp("", "encv-pack-*.tmp")
// 	if err != nil {
// 		return fmt.Errorf("failed to create temp file: %w", err)
// 	}
// 	tmpPath := tmpFile.Name()
// 	defer os.Remove(tmpPath)

// 	size, err := io.Copy(tmpFile, encryptedDataReader)
// 	if err != nil {
// 		tmpFile.Close()
// 		return fmt.Errorf("failed to write data to temp file: %w", err)
// 	}
// 	if err := tmpFile.Close(); err != nil {
// 		return fmt.Errorf("failed to close temp file: %w", err)
// 	}

// 	encFile, err := os.Open(tmpPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to open temp file for reading: %w", err)
// 	}
// 	defer encFile.Close()

// 	// 【新增日志 1】检查文件大小和分块大小
// 	log.Printf("DEBUG: WriteAllChunks: Total encrypted size = %d bytes, chunkSize = %d bytes.", size, chunkSize)

// 	// 先创建所有子分片，并收集元数据
// 	var subChunks []types.SubChunkInfo
// 	chunkIndex := 2

// 	for {
// 		offset := int64(chunkIndex-1) * int64(chunkSize)
// 		if offset >= size {
// 			break
// 		}
// 		_, err := encFile.Seek(offset, io.SeekStart)
// 		if err != nil {
// 			return fmt.Errorf("failed to seek in temp file: %w", err)
// 		}

// 		dataReader := io.LimitReader(encFile, int64(chunkSize))
// 		// 【关键】获取 WriteSubChunk 的返回值
// 		filename, md5, written, err := chunked.WriteSubChunk(subMagic, finalPath, chunkIndex, dataReader, originalMD5)
// 		if err != nil {
// 			log.Printf("Warning: Failed to write sub-chunk %d: %v. Skipping.", chunkIndex, err)
// 			if written == 0 {
// 				break
// 			}
// 			chunkIndex++
// 			continue
// 		}
// 		if written == 0 {
// 			break
// 		}

// 		// 【关键】收集子分片信息
// 		subChunks = append(subChunks, types.SubChunkInfo{
// 			Index:    chunkIndex,
// 			Filename: filename,
// 			MD5:      md5,
// 		})

// 		chunkIndex++
// 	}
// 	// 【新增日志 2】检查最终收集到的子分片数量
// 	log.Printf("DEBUG: WriteAllChunks: Finished loop. Collected %d sub-chunks.", len(subChunks))

// 	// 将收集到的子分片信息存入 KVI
// 	index.SubChunks = subChunks

// 	// 现在 KVI 完整了，序列化它
// 	kviData, err := json.Marshal(index)
// 	if err != nil {
// 		return fmt.Errorf("failed to marshal VideoIndex to JSON: %w", err)
// 	}

// 	// log.Printf("DEBUG: KVI data before writing to file (len=%d): %s", len(kviData), string(kviData))

// 	// 【修复 6】最后，写入主分片
// 	_, err = encFile.Seek(0, io.SeekStart)
// 	if err != nil {
// 		return fmt.Errorf("failed to seek to start of temp file: %w", err)
// 	}
// 	mainDataReader := io.LimitReader(encFile, int64(chunkSize))
// 	if err := chunked.WriteMainChunk(mainMagic, finalPath, kviData, mainDataReader, originalMD5); err != nil {
// 		return fmt.Errorf("failed to write main chunk: %w", err)
// 	}

// 	fmt.Printf("-> Successfully packed chunked container: %s\n", finalPath)
// 	return nil
// }

// // handleSubtitles 封装了字幕处理逻辑
// func (p *VideoPacker) handleSubtitles(ctx context.Context, vIndex *types.VideoIndex, baseName, outputDir string) error {
// 	cfg := config.FromContext(ctx)
// 	subtitleTracks, err := utils.DiscoverSubtitleTracks(vIndex.OriginalInputPath, cfg.TrackExtensions)
// 	if err != nil {
// 		log.Printf("Warning: subtitle discovery failed for %s: %v", vIndex.OriginalInputPath, err)
// 		// 不返回错误，继续无字幕打包
// 		vIndex.SubtitleTracks = nil
// 		return nil
// 	}
// 	vIndex.SubtitleTracks = subtitleTracks

// 	return p.copyAndRenameSubtitles(vIndex, outputDir, baseName)
// }

// func (p *VideoPacker) copyAndRenameSubtitles(index *types.VideoIndex, outputDir, encBaseName string) error {
// 	if len(index.SubtitleTracks) == 0 {
// 		return nil
// 	}

// 	videoDir := filepath.Dir(index.OriginalInputPath)
// 	sort.Slice(index.SubtitleTracks, func(i, j int) bool {
// 		return index.SubtitleTracks[i].Filename < index.SubtitleTracks[j].Filename
// 	})

// 	var kviTracks []types.SubtitleTracks
// 	for i, track := range index.SubtitleTracks {
// 		originalPath := filepath.Join(videoDir, track.Filename)
// 		subExt := filepath.Ext(track.Filename)

// 		newFilename := encBaseName + subExt
// 		if i > 0 {
// 			langTag := track.Language
// 			if langTag == "" {
// 				langTag = fmt.Sprintf("%d", i+1)
// 			}
// 			newFilename = encBaseName + "." + langTag + subExt
// 		}

// 		newPath := filepath.Join(outputDir, newFilename)
// 		if err := utils.CopyFile(originalPath, newPath); err != nil {
// 			return fmt.Errorf("failed to copy subtitle from %s to %s: %w", originalPath, newPath, err)
// 		}
// 		fmt.Printf("-> Subtitle renamed: %s\n", newFilename)

// 		// 更新 KVI 中的字幕轨道信息
// 		kviTracks = append(kviTracks, types.SubtitleTracks{
// 			Language: track.Language,
// 			Title:    newFilename,
// 			Filename: track.Filename,
// 		})
// 	}
// 	// 用更新后的轨道信息覆盖原来的
// 	index.SubtitleTracks = kviTracks
// 	return nil
// }
