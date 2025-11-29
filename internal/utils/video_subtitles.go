package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Soltus/encv-go/internal/types"
)

// DiscoverSubtitleTracks 发现视频关联的字幕轨道，只返回信息
func DiscoverSubtitleTracks(inputPath string, trackExtensions []string) ([]types.SubtitleTrack, error) {
	fmt.Println("-> Discovering subtitle tracks...")
	videoDir := filepath.Dir(inputPath)
	videoBaseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))

	files, _ := os.ReadDir(videoDir)
	sortedExts := SortExtensionsByLength(trackExtensions)
	var tracks []types.SubtitleTrack

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		fileName := f.Name()

		isSubtitle := false
		for _, ext := range sortedExts {
			if strings.HasSuffix(fileName, ext) {
				isSubtitle = true
				break
			}
		}
		if !isSubtitle {
			continue
		}

		subBaseName := StripKnownExtensions(fileName, sortedExts)
		if (strings.HasPrefix(subBaseName, videoBaseName) || subBaseName == videoBaseName) && subBaseName != "" {
			fmt.Printf("-> Found track: %s\n", fileName)
			lang := "und"
			if strings.Contains(fileName, "chi") || strings.Contains(fileName, "zh") {
				lang = "chi"
			} else if strings.Contains(fileName, "eng") {
				lang = "eng"
			}
			tracks = append(tracks, types.SubtitleTrack{
				Language: lang,
				Filename: fileName,
			})
		}
	}
	return tracks, nil
}

// CopyAndRenameSubtitles 将发现的字幕复制到输出目录并重命名
func CopyAndRenameSubtitles(tracks []types.SubtitleTrack, videoPath, outputDir, encBaseName string) ([]types.SubtitleTrack, error) {
	if len(tracks) == 0 {
		return nil, nil
	}

	videoDir := filepath.Dir(videoPath)
	// 排序以确保命名一致
	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].Filename < tracks[j].Filename
	})

	var kviTracks []types.SubtitleTrack
	for i, track := range tracks {
		originalPath := filepath.Join(videoDir, track.Filename)

		ext := filepath.Ext(track.Filename)
		newFilename := fmt.Sprintf("%s%s", encBaseName, ext)
		if i > 0 {
			newFilename = fmt.Sprintf("%s.%d%s", encBaseName, i+1, ext)
		}

		newPath := filepath.Join(outputDir, newFilename)
		if err := CopyFile(originalPath, newPath); err != nil {
			return nil, fmt.Errorf("failed to copy subtitle from %s to %s: %w", originalPath, newPath, err)
		}
		fmt.Printf("-> Copied subtitle to '%s'\n", newFilename)

		kviTracks = append(kviTracks, types.SubtitleTrack{
			Language: track.Language,
			Title:    newFilename,    // 加密后的文件名
			Filename: track.Filename, // 原始文件名
		})
	}
	return kviTracks, nil
}

// RestoreSubtitlesFromKVI 根据 KVI 中的信息，将字幕从容器目录恢复到输出目录
func RestoreSubtitlesFromKVI(index *types.VideoIndex, containerDir, outputDir string) error {
	if len(index.SubtitleTrack) == 0 {
		return nil
	}
	fmt.Println("-> Restoring subtitles...")
	for _, sub := range index.SubtitleTrack {
		// sub.Title 是加密后存储在容器目录中的字幕文件名 (e.g., "myvideo.sccgv.srt")
		// sub.Filename 是需要恢复成的原始文件名 (e.g., "myvideo.zh.srt")
		srcPath := filepath.Join(containerDir, sub.Title)
		dstPath := filepath.Join(outputDir, sub.Filename)

		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			fmt.Printf("-> Warning: Subtitle file '%s' not found in container directory, skipping.\n", sub.Title)
			continue
		}

		fmt.Printf("-> Restoring subtitle: %s\n", sub.Filename)
		if err := CopyFile(srcPath, dstPath); err != nil {
			fmt.Printf("-> Warning: Failed to restore subtitle '%s': %v\n", sub.Filename, err)
		}
	}
	return nil
}
