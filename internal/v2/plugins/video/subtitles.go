// internal/v2/plugins/video/subtitles.go

package video

import (
	"fmt"
	"log"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// HandleSubtitlesForEncryption 处理加密时的字幕逻辑
func HandleSubtitlesForEncryption(cfg *config.Config, vIndex *types.VideoIndex, outputDir, encryptedBaseName string) error {
	subtitleTracks, err := utils.DiscoverSubtitleTracks(vIndex.OriginalInputPath, cfg.TrackExtensions)
	if err != nil {
		log.Printf("Warning: subtitle discovery failed for %s: %v", vIndex.OriginalInputPath, err)
		// 不返回错误，继续无字幕打包
		vIndex.SubtitleTracks = nil
		return nil
	}
	vIndex.SubtitleTracks = subtitleTracks
	_, err = utils.CopyAndRenameSubtitles(subtitleTracks, vIndex.OriginalInputPath, outputDir, encryptedBaseName)

	return err
}

// RestoreSubtitlesForDecryption 处理解密时的字幕还原
func RestoreSubtitlesForDecryption(index *types.VideoIndex, containerDir, outputDir string) error {

	// containerDir := filepath.Dir(containerPath)
	if err := utils.RestoreSubtitlesFromKVI(index, containerDir, outputDir); err != nil {
		// 返回警告而不是错误，允许主流程继续
		return fmt.Errorf("warning: failed to restore subtitles: %w", err)
	}

	return nil
}
