// internal/v2/plugins/video/content_preprocessor.go

package video

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/v2/reader"
)

// VideoContentPreprocessor 实现 plugins.ContentPreprocessor 接口
type VideoContentPreprocessor struct {
	// 可以在这里注入依赖
}

// Preprocess 预处理视频内容
func (p *VideoContentPreprocessor) Preprocess(inputPath string) (io.ReadCloser, error) {
	// 决定数据源
	ext := strings.ToLower(filepath.Ext(inputPath))
	isMP4 := ext == ".mp4" || ext == ".mov" || ext == ".m4v"

	if isMP4 {
		fmt.Println("-> [CONTENT_PREPROCESSOR] Detected MP4 container, checking for fast-start...")
		if fast, err := isMP4FastStart(inputPath); err == nil && fast {
			fmt.Println("-> [CONTENT_PREPROCESSOR] Input is already a fast-start MP4, using file directly.")
			// 返回文件的 reader
			return os.Open(inputPath)
		} else {
			if err != nil {
				fmt.Printf("-> [CONTENT_PREPROCESSOR] Warning: Could not verify fast-start status (%v), proceeding with pre-processing.\n", err)
			} else {
				fmt.Println("-> [CONTENT_PREPROCESSOR] Input is not a fast-start MP4, pre-processing is required.")
			}
		}
	}

	// 如果需要预处理，创建一个临时文件
	fmt.Println("-> [CONTENT_PREPROCESSOR] Starting FFmpeg for pre-processing to a temp file...")
	tempFilePath, err := PreprocessVideoWithFFmpeg(inputPath)
	if err != nil {
		return nil, fmt.Errorf("pre-processing to temp file failed: %w", err)
	}
	dataReader, err := reader.NewTempFileReadCloser(tempFilePath)
	if err != nil {
		// NewTempFileReadCloser 内部不会删除文件，因为它没有成功打开。
		// 所以这里需要手动删除。
		os.Remove(tempFilePath)
		return nil, err
	}

	fmt.Printf("-> [CONTENT_PREPROCESSOR] SUCCESS: Returning a temp file reader for %s\n", tempFilePath)
	return dataReader, nil
}

// preprocessVideoWithFFmpeg 使用 FFmpeg 预处理视频，返回临时文件路径
func PreprocessVideoWithFFmpeg(inputPath string) (string, error) {
	fmt.Println("-> Step 1: Pre-processing video for streaming...")
	tempFile, err := os.CreateTemp("", "encv-pre-*.mp4")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	isMkv := strings.ToLower(filepath.Ext(inputPath)) == ".mkv"
	var ffmpegCmd *exec.Cmd
	if isMkv {
		ffmpegCmd = exec.Command("ffmpeg", "-y", "-i", inputPath, "-c", "copy", tempPath)
	} else {
		ffmpegCmd = exec.Command("ffmpeg", "-y", "-i", inputPath, "-c", "copy", "-movflags", "+faststart", tempPath)
	}

	ffmpegCmd.Stderr = os.Stderr
	if err := ffmpegCmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg failed: %w", err)
	}
	fmt.Println("-> Pre-processing complete.")
	return tempPath, nil
}

// isMP4FastStart 检查 MP4 文件是否是流式友好的（即 moov atom 在 mdat atom 之前）。
// MP4文件格式中，atom size和type的存储方式是大端序。这里仅判断不影响，项目使用的是小端序
func isMP4FastStart(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// 我们只需要读取文件开头的一部分来找到 moov 和 mdat
	// 1MB 应该足够覆盖大多数情况下的元数据区域
	bufferSize := int64(1024 * 1024)
	reader := io.LimitReader(file, bufferSize)
	header := make([]byte, bufferSize)
	n, err := io.ReadFull(reader, header)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, err
	}
	// 如果文件小于 1MB，只处理实际读取的部分
	header = header[:n]

	offset := int64(0)
	for offset < int64(len(header)) {
		// 每个 atom 至少有 8 字节的头
		if offset+8 > int64(len(header)) {
			break
		}

		atomSize := int64(binary.BigEndian.Uint32(header[offset : offset+4]))
		atomType := string(header[offset+4 : offset+8])

		if atomSize == 1 { // 64-bit size
			if offset+16 > int64(len(header)) {
				break
			}
			atomSize = int64(binary.BigEndian.Uint64(header[offset+8 : offset+16]))
			offset += 16
		} else {
			offset += 8
		}

		if atomSize < 8 { // 无效的 atom size
			break
		}

		switch atomType {
		case "moov":
			// 找到了 moov atom，并且它在我们扫描的范围内，说明它在文件开头
			return true, nil
		case "mdat":
			// 在找到 moov 之前先找到了 mdat，说明不是 fast-start
			return false, nil
		}

		// 移动到下一个 atom
		offset += atomSize - 8 // 减去已经读取的 8 字节头
	}

	// 如果在扫描范围内没有找到 mdat 或 moov，我们保守地认为它不是 fast-start
	return false, nil
}
