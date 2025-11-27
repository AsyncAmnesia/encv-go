package utils

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
)

// --- MIME 类型分类集合 ---
// 使用 map[string]struct{} 来表示集合，这是 Go 的惯用法，内存效率高

var imageMimeTypes = map[string]struct{}{
	"image/jpeg":    {},
	"image/tiff":    {},
	"image/png":     {},
	"image/gif":     {},
	"image/bmp":     {},
	"image/svg+xml": {},
	"image/x-icon":  {},
	"image/webp":    {},
	"image/avif":    {},
}

var videoMimeTypes = map[string]struct{}{
	"video/mp4":                        {},
	"video/x-matroska":                 {},
	"video/x-msvideo":                  {},
	"video/quicktime":                  {},
	"application/vnd.rn-realmedia-vbr": {},
	"video/webm":                       {},
	"video/x-flv":                      {},
	"application/vnd.apple.mpegurl":    {},
}

var audioMimeTypes = map[string]struct{}{
	"audio/mpeg":     {},
	"audio/flac":     {},
	"audio/ogg":      {},
	"audio/mp4":      {}, // m4a
	"audio/wav":      {},
	"audio/opus":     {},
	"audio/x-ms-wma": {},
}

// --- 类别判断函数 ---

// IsImageType 检查给定的 MIME 类型是否为图像类型
func IsImageType(mimeType string) bool {
	_, ok := imageMimeTypes[mimeType]
	return ok
}

// IsVideoType 检查给定的 MIME 类型是否为视频类型
func IsVideoType(mimeType string) bool {
	_, ok := videoMimeTypes[mimeType]
	return ok
}

// IsAudioType 检查给定的 MIME 类型是否为音频类型
func IsAudioType(mimeType string) bool {
	_, ok := audioMimeTypes[mimeType]
	return ok
}

// 根据 URL 文件扩展名获取 Content-Type
func GetContentTypeFromExtension(fileURL string) string {
	ext := strings.ToLower(filepath.Ext(fileURL))
	if len(ext) > 0 {
		ext = ext[1:]
	}
	if ct, ok := config.ContentTypes[ext]; ok {
		return ct
	}
	return "application/octet-stream"
}

// DetectFileMIMEType 检测文件的 MIME 类型
// 优先使用文件头嗅探，如果失败则回退到扩展名检测
func DetectFileMIMEType(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for MIME detection: %w", err)
	}
	defer file.Close()

	// 1. 优先使用文件头嗅探
	buffer := make([]byte, 512)
	bytesRead, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read file header for MIME detection: %w", err)
	}

	mimeType := http.DetectContentType(buffer[:bytesRead])

	// 2. 【关键修复】如果嗅探结果是通用类型，则回退到扩展名检测
	if mimeType == "application/octet-stream" {
		ext := strings.ToLower(filepath.Ext(filePath))
		if len(ext) > 0 {
			// 尝试从 mime 标准库中查找
			mimeType = mime.TypeByExtension(ext)
			// 如果标准库没有，再从我们的配置中查找
			if mimeType == "" {
				if ct, ok := config.ContentTypes[ext[1:]]; ok {
					mimeType = ct
				}
			}
		}
	}

	return mimeType, nil
}

// 检查MIME类型是否在支持列表中
func IsSupportedType(mimeType string) bool {
	for _, ct := range config.ContentTypes {
		if ct == mimeType {
			return true
		}
	}
	return false
}

func IsSupportedFile(filePath string) bool {
	mimeType, err := DetectFileMIMEType(filePath)
	// 【关键修改】打印错误，以便调试
	if err != nil {
		fmt.Printf("-> [DEBUG] Error detecting MIME for '%s': %v\n", filePath, err)
		return false
	}

	// 【修改】提供更详细的调试信息
	fmt.Printf("-> [DEBUG] Detected MIME for '%s': %s\n", filePath, mimeType)

	return IsSupportedType(mimeType)
}
