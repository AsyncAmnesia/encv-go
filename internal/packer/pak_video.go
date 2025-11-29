package packer

import (
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"sort"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

type VideoPacker struct {
	BaseChunkedPacker
}

func (p *VideoPacker) Pack(ctx context.Context, baseName, outputDir string, encryptedDataReader io.Reader, index types.Index) error {
	vIndex, ok := index.(*types.VideoIndex)
	if !ok {
		return fmt.Errorf("VideoPacker received an invalid index type: %T", index)
	}

	cfg := config.FromContext(ctx)

	// 1. 视频特有逻辑：字幕处理
	if err := p.handleSubtitles(ctx, vIndex, baseName, outputDir); err != nil {
		return err // 字幕处理失败是致命错误
	}

	finalPath := filepath.Join(outputDir, baseName+cfg.GetVideoEncExtension())

	// 3. 决策：获取魔法数字
	magicMap, _ := container.GetContainerMagicMap(ctx)
	subMagicMap, _ := container.GetSubChunkMagicMap(ctx)
	mainMagic := magicMap[cfg.BinExtGroup.Video]
	subMagic := subMagicMap[cfg.BinExtGroup.Video]

	// 委托：调用基类的工具方法完成打包
	// 【修复 1】不再在这里序列化 KVI，而是将 vIndex 传递给 WriteAllChunks
	// WriteAllChunks 内部会负责创建子分片、更新 vIndex、序列化 KVI 并写入主分片
	return p.WriteAllChunks(ctx, finalPath, []byte(mainMagic), []byte(subMagic), vIndex, encryptedDataReader, vIndex.OriginalFileMD5)
}

// handleSubtitles 封装了字幕处理逻辑
func (p *VideoPacker) handleSubtitles(ctx context.Context, vIndex *types.VideoIndex, baseName, outputDir string) error {
	cfg := config.FromContext(ctx)
	subtitleTracks, err := utils.DiscoverSubtitleTracks(vIndex.OriginalInputPath, cfg.TrackExtensions)
	if err != nil {
		log.Printf("Warning: subtitle discovery failed for %s: %v", vIndex.OriginalInputPath, err)
		// 不返回错误，继续无字幕打包
		vIndex.SubtitleTrack = nil
		return nil
	}
	vIndex.SubtitleTrack = subtitleTracks

	return p.copyAndRenameSubtitles(vIndex, outputDir, baseName)
}

func (p *VideoPacker) copyAndRenameSubtitles(index *types.VideoIndex, outputDir, encBaseName string) error {
	if len(index.SubtitleTrack) == 0 {
		return nil
	}

	videoDir := filepath.Dir(index.OriginalInputPath)
	sort.Slice(index.SubtitleTrack, func(i, j int) bool {
		return index.SubtitleTrack[i].Filename < index.SubtitleTrack[j].Filename
	})

	var kviTracks []types.SubtitleTrack
	for i, track := range index.SubtitleTrack {
		originalPath := filepath.Join(videoDir, track.Filename)
		subExt := filepath.Ext(track.Filename)

		newFilename := encBaseName + subExt
		if i > 0 {
			langTag := track.Language
			if langTag == "" {
				langTag = fmt.Sprintf("%d", i+1)
			}
			newFilename = encBaseName + "." + langTag + subExt
		}

		newPath := filepath.Join(outputDir, newFilename)
		if err := utils.CopyFile(originalPath, newPath); err != nil {
			return fmt.Errorf("failed to copy subtitle from %s to %s: %w", originalPath, newPath, err)
		}
		fmt.Printf("-> Subtitle renamed: %s\n", newFilename)

		// 更新 KVI 中的字幕轨道信息
		kviTracks = append(kviTracks, types.SubtitleTrack{
			Language: track.Language,
			Title:    newFilename,
			Filename: track.Filename,
		})
	}
	// 用更新后的轨道信息覆盖原来的
	index.SubtitleTrack = kviTracks
	return nil
}
