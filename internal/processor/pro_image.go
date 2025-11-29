package processor

import (
	"fmt"
	"image"
	_ "image/gif"  // 注册 GIF 解码器
	_ "image/jpeg" // 注册 JPEG 解码器
	_ "image/png"  // 注册 PNG 解码器
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp" // 注册 WebP 解码器

	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

type ImageProcessor struct{}

// 实现 Processor 接口
func (p *ImageProcessor) SupportedMimePrefixes() []string {
	return []string{"image/"}
}

// 实现 Processor 接口
func (p *ImageProcessor) ShouldProcess(inputPath string) bool {
	return true
}

// 实现 Processor 接口
func (p *ImageProcessor) Process(inputPath string) (types.Index, error) {
	fmt.Printf("-> Analyzing image: %s\n", filepath.Base(inputPath))

	fileSize := utils.GetFileSize(inputPath)
	mimeType, err := utils.DetectFileMIMEType(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to DetectFileMIMEType: %w", err)
	}
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image file: %w", err)
	}
	defer file.Close()

	config, formatName, err := image.DecodeConfig(file)
	width, height, finalFormat := 0, 0, ""

	if err == nil {
		width, height = config.Width, config.Height
		finalFormat = strings.ToLower(formatName)
		fmt.Printf("-> [Info] Decoded format '%s' with dimensions %dx%d.\n", finalFormat, width, height)
	} else {
		fmt.Printf("-> [Warning] Could not decode image config. Falling back to filename-based detection.\n")
		ext := strings.ToLower(filepath.Ext(inputPath))
		if len(ext) == 0 {
			return nil, fmt.Errorf("cannot determine image format from filename: %s", inputPath)
		}
		finalFormat = ext[1:]
		fmt.Printf("-> [Fallback] Detected format '%s' from extension. Dimensions will be 0.\n", finalFormat)
	}

	// 返回一个部分初始化的 ImageIndex
	return &types.ImageIndex{
		Kind:             types.IndexKindImage,
		Version:          types.KviVersion,
		OriginalFileSize: fileSize,
		Width:            width,
		Height:           height,
		Format:           finalFormat,
		MimeType:         mimeType,
		// Encryption, OriginalFilename, EncryptedFileMD5 等字段保持零值，由 encrypt 命令填充
	}, nil
}

// ProcessImage 分析图像文件，返回其元数据
// func ProcessImage(inputPath string) (*types.ImageIndex, error) {
// 	fmt.Printf("-> Analyzing image: %s\n", filepath.Base(inputPath))

// 	fileSize := utils.GetFileSize(inputPath) // 获取文件大小
// 	mimeType, err := utils.DetectFileMIMEType(inputPath)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to DetectFileMIMEType: %w", err)
// 	}
// 	file, err := os.Open(inputPath)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to open image file: %w", err)
// 	}
// 	defer file.Close()

// 	config, formatName, err := image.DecodeConfig(file)

// 	width, height := 0, 0
// 	finalFormat := ""

// 	if err == nil {
// 		// 解码成功，使用解码器提供的信息
// 		width, height = config.Width, config.Height
// 		finalFormat = strings.ToLower(formatName)
// 		fmt.Printf("-> [Info] Decoded format '%s' with dimensions %dx%d.\n", finalFormat, width, height)
// 	} else {
// 		// 解码失败（如 .ico），降级到从文件名推断
// 		fmt.Printf("-> [Warning] Could not decode image config (unsupported format). Falling back to filename-based detection.\n")

// 		ext := strings.ToLower(filepath.Ext(inputPath))
// 		if len(ext) == 0 {
// 			return nil, fmt.Errorf("cannot determine image format from filename: %s", inputPath)
// 		}
// 		finalFormat = ext[1:] // 去掉 '.'

// 		fmt.Printf("-> [Fallback] Detected format '%s' from extension. Dimensions will be 0.\n", finalFormat)
// 	}

// 	// 返回最终结果
// 	return &types.ImageIndex{
// 		Width:            width,
// 		Height:           height,
// 		Format:           finalFormat,
// 		MimeType:         mimeType,
// 		OriginalFileSize: fileSize,
// 	}, nil
// }
