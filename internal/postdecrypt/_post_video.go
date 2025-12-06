package postdecrypt

// import (
// 	"context"
// 	"fmt"
// 	"path/filepath"

// 	"github.com/Soltus/encv-go/internal/types"
// 	"github.com/Soltus/encv-go/internal/utils"
// )

// // VideoPostDecrypter 处理视频解密后的逻辑
// type VideoPostDecrypter struct{}

// func (p *VideoPostDecrypter) PostDecrypt(ctx context.Context, content *types.DecryptedContent, containerPath, outputDir string) error {
// 	vIndex, ok := content.Index.(*types.VideoIndex)
// 	if !ok {
// 		return fmt.Errorf("internal error: VideoPostDecrypter called with non-VideoIndex type")
// 	}

// 	// 还原字幕
// 	containerDir := filepath.Dir(containerPath)
// 	if err := restoreSubtitles(vIndex, containerDir, outputDir); err != nil {
// 		// 返回警告而不是错误，允许主流程继续
// 		return fmt.Errorf("warning: failed to restore subtitles: %w", err)
// 	}

// 	return nil
// }

// // 为视频还原字幕
// func restoreSubtitles(index *types.VideoIndex, containerDir, outputDir string) error {
// 	return utils.RestoreSubtitlesFromKVI(index, containerDir, outputDir)
// }
