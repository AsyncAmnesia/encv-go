package processor

import (
	"bytes"
	"encoding/json"
	"fmt"
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

	// 复用原来的预处理和元数据获取逻辑
	tempPath, err := PreprocessVideoWithFFmpeg(inputPath)
	if err != nil {
		return nil, fmt.Errorf("pre-processing failed: %w", err)
	}
	defer os.Remove(tempPath)

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
