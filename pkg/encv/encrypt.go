package encv

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/processor"
)

// --- 1. 新增：格式检测和映射 ---

var containerExtensionMap = map[string]string{
	"mp4":  "4pm",
	"mov":  "4pm",
	"mkv":  "vkm",
	"webm": "vkm",
}

// detectContainerFormat 使用 ffprobe 检测视频文件的真实容器格式
func detectContainerFormat(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=format_name", "-of", "default=noprint_wrappers=1:nokey=1", filePath)
	output, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("ffprobe failed: %s", string(ee.Stderr))
		}
		if strings.Contains(err.Error(), "executable file not found") {
			return "", fmt.Errorf("ffprobe not found. Please install FFmpeg and ensure it's in your PATH")
		}
		return "", fmt.Errorf("failed to run ffprobe: %w", err)
	}

	formatName := strings.TrimSpace(string(output))
	if formatName == "" {
		return "", fmt.Errorf("could not determine container format")
	}

	switch {
	case strings.Contains(formatName, "matroska"):
		return "mkv", nil
	case strings.Contains(formatName, "mp4"):
		return "mp4", nil
	default:
		parts := strings.Split(formatName, ",")
		return strings.ToLower(parts[0]), nil
	}
}

func getNewExtension(originalFormat string) string {
	if ext, ok := containerExtensionMap[originalFormat]; ok {
		return ext
	}
	return originalFormat
}

// --- 2. 核心加密逻辑 ---

// Encrypt 加密单个视频文件或整个目录。
func Encrypt(inputPath string, opts EncryptOptions) error {
	if opts.Password == "" || opts.OutputDir == "" {
		return ErrMissingOptions
	}
	if len(opts.TrackExtensions) == 0 {
		opts.TrackExtensions = []string{".ass", ".srt", ".dm.ass"}
	}

	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	salt := make([]byte, crypto.SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		return fmt.Errorf("failed to stat input path: %w", err)
	}

	if info.IsDir() {
		return encryptDir(inputPath, opts, salt)
	}
	return encryptFile(inputPath, opts, salt)
}

// encryptFile 加密单个文件
func encryptFile(inputPath string, opts EncryptOptions, salt []byte) error {
	originalFilename := filepath.Base(inputPath)
	detectedFormat, err := detectContainerFormat(inputPath)
	if err != nil {
		return fmt.Errorf("failed to detect format for %s: %w", inputPath, err)
	}
	newExt := getNewExtension(detectedFormat)
	baseName := strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))

	outputEncPath := filepath.Join(opts.OutputDir, fmt.Sprintf("%s.%s.enc", baseName, newExt))
	outputIndexPath := filepath.Join(opts.OutputDir, fmt.Sprintf("%s.%s.kvi", baseName, newExt))

	fmt.Printf("-> Detected format: %s, New extension: .%s\n", detectedFormat, newExt)

	// 确保 processor.ProcessVideo 接受 originalFilename 参数
	return processor.ProcessVideo(inputPath, outputEncPath, outputIndexPath, opts.Password, salt, opts.TrackExtensions, originalFilename)
}

// encryptDir 加密目录中的所有视频文件
func encryptDir(inputDir string, opts EncryptOptions, salt []byte) error {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return fmt.Errorf("failed to read input directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !isVideoFile(entry.Name()) {
			continue
		}

		fullPath := filepath.Join(inputDir, entry.Name())
		if err := encryptFile(fullPath, opts, salt); err != nil {
			fmt.Printf("Error processing %s: %v\n", entry.Name(), err)
		}
	}
	return nil
}

// isVideoFile 检查文件是否是已知的视频类型
func isVideoFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".mp4" || ext == ".mkv" || ext == ".mov" || ext == ".webm"
}
