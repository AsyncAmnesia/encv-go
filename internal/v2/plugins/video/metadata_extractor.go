// internal/v2/plugins/video/metadata_extractor.go

package video

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// VideoMetadataExtractor 实现 plugins.MetadataExtractor 接口
type VideoMetadataExtractor struct {
	// 可以在这里注入依赖，例如配置
}

// ExtractMetadata 提取视频元数据
func (e *VideoMetadataExtractor) ExtractMetadata(inputPath string) (types.Index, error) {
	fmt.Printf("-> [METADATA_EXTRACTOR] Analyzing video: %s\n", filepath.Base(inputPath))

	metadata, err := getProcessedMetadata(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	metadata.OriginalInputPath = inputPath

	return metadata, nil
}

// 获取并处理视频元数据
func getProcessedMetadata(path string) (*VideoIndex, error) {
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

	format, err := utils.DetectVideoFormat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to detect video format: %w", err)
	}

	originalMD5, err := utils.FileMD5(path)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate MD5 for original video %s: %w", path, err)
	}

	return &VideoIndex{
		Width:            width,
		Height:           height,
		OriginalFileSize: fileInfo.Size(),
		OriginalFileMD5:  originalMD5,
		OriginalFilename: filepath.Base(path),
		DurationSeconds:  parseDuration(rawMeta.Format.Duration),
		Resolution:       fmt.Sprintf("%dx%d", width, height),
		Format:           format,
		MimeType:         mimeType,
	}, nil
}

// 解析时长字符串
func parseDuration(d string) float64 {
	var h, m, s float64
	fmt.Sscanf(d, "%f:%f:%f", &h, &m, &s)
	return h*3600 + m*60 + s
}
