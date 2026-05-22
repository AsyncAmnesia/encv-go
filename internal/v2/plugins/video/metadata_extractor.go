// internal/v2/plugins/video/metadata_extractor.go

package video

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Soltus/encv-go/internal/logger"
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// logger 是视频插件的日志记录器
var videoLogger = logger.WithComponent("video.metadata_extractor")

// VideoMetadataExtractor 实现 plugins.MetadataExtractor 接口
type VideoMetadataExtractor struct {
	// 可以在这里注入依赖，例如配置
	settings VideoPluginConfig
	index    *VideoIndex
}

// ExtractMetadata 提取视频元数据
func (e *VideoMetadataExtractor) ExtractMetadata(inputPath string) (types.Index, error) {
	videoLogger.Info("analyzing video",
		slog.String("file", filepath.Base(inputPath)),
		slog.String("path", inputPath),
	)

	metadata, err := extractMetadataFromOriginalFile(inputPath)
	if err != nil {
		videoLogger.Error("failed to extract metadata",
			slog.String("file", filepath.Base(inputPath)),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to get metadata from original file: %w", err)
	}

	// 安全检查：确保 metadata 不为 nil
	if metadata == nil {
		videoLogger.Error("metadata extraction returned nil",
			slog.String("file", inputPath),
		)
		return nil, fmt.Errorf("metadata extraction returned nil for file: %s", inputPath)
	}

	// 将提取出的所有信息复制到共享的 index 中
	if e.index == nil {
		e.index = &VideoIndex{}
	}
	*e.index = *metadata
	e.index.OriginalInputPath = inputPath

	videoLogger.Info("metadata extraction completed",
		slog.String("file", filepath.Base(inputPath)),
		slog.Int("width", metadata.Width),
		slog.Int("height", metadata.Height),
		slog.Float64("duration", metadata.DurationSeconds),
	)

	// 【关键】返回共享 index 的地址
	return e.index, nil
}

func extractMetadataFromOriginalFile(path string) (*VideoIndex, error) {
	videoLogger.Debug("using ffprobe/mkvtoolnix for metadata extraction",
		slog.String("file", filepath.Base(path)),
	)

	// 1. 使用 ffprobe 获取基础元数据
	cmd := utils.FFProbeCmd("-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe failed on original file: %w", err)
	}

	var rawMeta types.FFProbeRawMetadata
	if err := json.Unmarshal(out.Bytes(), &rawMeta); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ffprobe data: %w", err)
	}

	var width, height int
	for _, s := range rawMeta.Streams {
		if s.CodecType == "video" {
			width = s.Width
			height = s.Height
			break
		}
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	originalMD5, err := utils.FileMD5(path)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate MD5 for original file %s: %w", path, err)
	}

	chapters, err := extractChaptersWithFFprobe(path)
	if err != nil {
		videoLogger.Warn("could not extract chapters with ffprobe",
			slog.String("file", filepath.Base(path)),
			slog.Any("error", err),
		)

		// 如果 ffprobe 失败，并且文件是 MKV，再尝试 mkvextract
		format, _ := utils.DetectVideoFormat(path)
		if strings.ToLower(format) == "mkv" {
			videoLogger.Debug("attempting mkvextract for chapters",
				slog.String("file", filepath.Base(path)),
			)
			mkvChapters, err := ExtractChaptersWithMKVExtract(path)
			if err != nil {
				videoLogger.Warn("mkvextract also failed, proceeding without chapters",
					slog.String("file", filepath.Base(path)),
					slog.Any("error", err),
				)
				chapters = nil
			} else {
				chapters = mkvChapters
			}
		} else {
			chapters = nil
		}
	}

	// 3. 构建并返回 VideoIndex
	return &VideoIndex{
		Width:            width,
		Height:           height,
		OriginalFileSize: fileInfo.Size(),
		OriginalFileMD5:  originalMD5,
		OriginalFilename: filepath.Base(path),
		DurationSeconds:  parseDuration(rawMeta.Format.Duration), // 原始时长，可能不精确
		Resolution:       fmt.Sprintf("%dx%d", width, height),
		Format:           rawMeta.Format.FormatName, // 原始格式
		MimeType:         "",                        // 将在 Preprocessor 中权威获取
		Chapters:         chapters,                  // 【关键】从原始文件提取的章节
	}, nil
}

// 解析 HH:MM:SS.ms 格式的时长字符串
func parseDuration(d string) float64 {
	if d == "" {
		return 0
	}
	parts := strings.Split(d, ":")
	if len(parts) < 3 {
		// 尝试直接解析为秒数（某些情况下 ffprobe 可能会这样输出）
		if s, err := strconv.ParseFloat(d, 64); err == nil {
			return s
		}
		return 0
	}

	h, _ := strconv.ParseFloat(parts[0], 64)
	m, _ := strconv.ParseFloat(parts[1], 64)
	s, _ := strconv.ParseFloat(parts[2], 64)

	return h*3600 + m*60 + s
}

// 使用 ffprobe 提取章节
func extractChaptersWithFFprobe(path string) ([]MKVChapterInfo, error) {
	cmd := utils.FFProbeCmd("-v", "error", "-show_chapters", "-of", "json", path)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe command failed: %w", err)
	}

	var probeData struct {
		Chapters []struct {
			ID        int    `json:"id"`
			Title     string `json:"tags.title"`
			StartTime string `json:"start_time"`
			EndTime   string `json:"end_time"`
		} `json:"chapters"`
	}

	if err := json.Unmarshal(output, &probeData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chapter data: %w", err)
	}

	// 如果 ffprobe 返回的章节列表为空，则返回 nil 表示没有章节
	if len(probeData.Chapters) == 0 {
		return nil, nil
	}

	var chapters []MKVChapterInfo
	for _, ch := range probeData.Chapters {
		start, _ := time.ParseDuration(ch.StartTime + "s")
		end, _ := time.ParseDuration(ch.EndTime + "s")
		chapters = append(chapters, MKVChapterInfo{
			ID:        ch.ID,
			Title:     ch.Title,
			StartTime: start,
			EndTime:   end,
		})
	}

	videoLogger.Debug("chapters extracted",
		slog.String("file", filepath.Base(path)),
		slog.Int("count", len(chapters)),
	)
	return chapters, nil
}
