package processor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

type TextProcessor struct{}

func (p *TextProcessor) SupportedMimePrefixes() []string {
	return []string{
		"text/",                  // 匹配 text/plain, text/html 等
		"application/json",       // 精确匹配 JSON
		"application/javascript", // 精确匹配 JS
		"application/x-sh",       // 精确匹配 Shell
		"application/x-yaml",     // 精确匹配 YAML
	}
}

// ShouldProcess 实现了 Processor 接口的新方法
func (p *TextProcessor) ShouldProcess(inputPath string) bool {
	// 获取文件扩展名（小写，不带点）
	ext := strings.ToLower(filepath.Ext(inputPath))
	if len(ext) > 0 {
		ext = ext[1:]
	}

	// 这些文件在业务上被视为视频的一部分，而非独立的文本文件
	excludedSubtitles := map[string]struct{}{
		"ass": {},
		"srt": {},
		"vtt": {},
	}

	if _, shouldExclude := excludedSubtitles[ext]; shouldExclude {
		return false
	}

	return true
}

// 实现 Processor 接口
func (p *TextProcessor) Process(inputPath string) (types.Index, io.ReadCloser, error) {
	fmt.Printf("-> Analyzing text: %s\n", filepath.Base(inputPath))

	fileSize := utils.GetFileSize(inputPath)
	mimeType, err := utils.DetectFileMIMEType(inputPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to DetectFileMIMEType: %w", err)
	}

	ext := filepath.Ext(inputPath)
	if len(ext) == 0 {
		return nil, nil, fmt.Errorf("cannot determine text format from filename: %s", inputPath)
	}
	format := ext[1:] // 去掉 '.'

	fmt.Printf("-> [Info] Detected MIME type '%s' with format '%s'.\n", mimeType, format)

	file, err := os.Open(inputPath)
	if err != nil {
		return nil, nil, err
	}
	originalMD5, err := utils.FileMD5(inputPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to calculate MD5 for original video %s: %w", inputPath, err)
	}
	return &types.TextIndex{
		MimeType:         mimeType,
		Format:           format,
		OriginalFileSize: fileSize,
		OriginalFilename: filepath.Base(inputPath),
		OriginalFileMD5:  originalMD5,
	}, file, nil
}
