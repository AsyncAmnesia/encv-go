package processor

import (
	"fmt"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// ProcessText 分析文本文件，返回其元数据
func ProcessText(inputPath string) (*types.TextIndex, error) {
	fmt.Printf("-> Analyzing text: %s\n", filepath.Base(inputPath))

	fileSize := utils.GetFileSize(inputPath)
	mimeType, err := utils.DetectFileMIMEType(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to DetectFileMIMEType: %w", err)
	}

	ext := filepath.Ext(inputPath)
	if len(ext) == 0 {
		return nil, fmt.Errorf("cannot determine text format from filename: %s", inputPath)
	}
	format := ext[1:] // 去掉 '.'

	fmt.Printf("-> [Info] Detected MIME type '%s' with format '%s'.\n", mimeType, format)

	return &types.TextIndex{
		MimeType:         mimeType,
		Format:           format,
		OriginalFileSize: fileSize,
	}, nil
}
