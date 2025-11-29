package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/processor"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// encryptVideo 处理视频加密和打包
func encryptVideo(ctx context.Context, inputPath, baseName, originalExt, outputDir string, salt []byte) error {
	fmt.Printf("--> Starting encryption for video: %s\n", baseName)
	cfg := config.FromContext(ctx)
	// 1. 【分析阶段】获取视频元数据
	metadata, err := processor.ProcessVideo(inputPath)
	if err != nil {
		return fmt.Errorf("video analysis failed: %w", err)
	}

	// 2. 【分析阶段】发现字幕轨道
	subtitleInfos, err := utils.DiscoverSubtitleTracks(inputPath, cfg.TrackExtensions)
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
	iv, err := crypto.EncryptFile(tempPreprocessedPath, tempEncPath, cfg.Password, salt)
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
	finalPath := filepath.Join(outputDir, encBaseName+cfg.GetVideoEncExtension())

	if cfg.IsSccgvChunkingEnabled() {
		return createChunkedContainer(ctx, tempEncPath, finalPath, index, inputPath)
	}
	return container.PackWithIndex(ctx, tempEncPath, finalPath, index)
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
