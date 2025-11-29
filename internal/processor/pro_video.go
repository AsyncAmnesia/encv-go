package processor

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

type VideoProcessor struct{}

// 实现 Processor 接口
func (p *VideoProcessor) SupportedMimePrefixes() []string {
	return []string{
		"video/",                           // 匹配 video/mp4, video/webm 等
		"application/vnd.rn-realmedia-vbr", // 精确匹配 RealMedia
		"application/vnd.apple.mpegurl",    // 精确匹配 HLS
	}
}

// 实现 Processor 接口
func (p *VideoProcessor) ShouldProcess(inputPath string) bool {
	return true
}

// 实现 Processor 接口
func (p *VideoProcessor) Process(inputPath string) (types.Index, error) {
	fmt.Printf("-> Analyzing video: %s\n", filepath.Base(inputPath))

	var tempPath string
	var err error

	// 1. 检查是否可以跳过预处理
	ext := strings.ToLower(filepath.Ext(inputPath))
	isMP4 := ext == ".mp4" || ext == ".mov" || ext == ".m4v"

	if isMP4 {
		fmt.Println("-> Detected MP4 container, checking for fast-start...")
		if fast, err := isMP4FastStart(inputPath); err == nil && fast {
			fmt.Println("-> Input is already a fast-start MP4, skipping pre-processing.")
			tempPath = inputPath // 直接使用原始文件路径
		} else {
			if err != nil {
				fmt.Printf("-> Warning: Could not verify fast-start status (%v), proceeding with pre-processing.\n", err)
			} else {
				fmt.Println("-> Input is not a fast-start MP4, pre-processing is required.")
			}
		}
	}

	// 2. 如果 tempPath 仍为空，说明需要执行预处理
	if tempPath == "" {
		tempPath, err = PreprocessVideoWithFFmpeg(inputPath)
		if err != nil {
			return nil, fmt.Errorf("pre-processing failed: %w", err)
		}
		// 【关键】只有在我们创建了临时文件时，才 defer 删除
		defer os.Remove(tempPath)
	}

	// 3. 获取元数据
	metadata, err := getProcessedMetadata(tempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	metadata.OriginalInputPath = inputPath

	fmt.Println("-> Analysis complete.")
	return metadata, nil
}

// 使用 ffprobe 检测视频文件的真实格式
func DetectVideoFormat(filePath string) (string, error) {
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

// 获取并处理视频元数据
func getProcessedMetadata(path string) (*types.VideoIndex, error) {
	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var rawMeta types.FFProbeRawMetadata
	if err := json.Unmarshal(out.Bytes(), &rawMeta); err != nil {
		return nil, err
	}

	var width, height int
	for _, s := range rawMeta.Streams {
		if s.CodecType == "video" {
			width = s.Width
			height = s.Height
			break
		}
	}

	fileInfo, _ := os.Stat(path)
	mimeType, err := utils.DetectFileMIMEType(path)
	if err != nil {
		return nil, fmt.Errorf("failed to DetectFileMIMEType: %w", err)
	}

	originalMD5, err := utils.FileMD5(path)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate MD5 for original video %s: %w", path, err)
	}

	return &types.VideoIndex{
		Kind:             types.IndexKindVideo,
		Version:          types.KviVersion,
		Width:            width,
		Height:           height,
		OriginalFileSize: fileInfo.Size(),
		OriginalFileMD5:  originalMD5,
		DurationSeconds:  parseDuration(rawMeta.Format.Duration),
		Resolution:       fmt.Sprintf("%dx%d", width, height),
		Format:           strings.TrimPrefix(filepath.Ext(path), "."),
		MimeType:         mimeType,
	}, nil
}

// 解析时长字符串
func parseDuration(d string) float64 {
	var h, m, s float64
	fmt.Sscanf(d, "%f:%f:%f", &h, &m, &s)
	return h*3600 + m*60 + s
}

// isMP4FastStart 检查 MP4 文件是否是流式友好的（即 moov atom 在 mdat atom 之前）。
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
