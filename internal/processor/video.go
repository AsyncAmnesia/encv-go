package processor

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// FFProbeRawMetadata 用于直接解析 ffprobe 的 JSON 输出
type FFProbeRawMetadata struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"streams"`
}

// ProcessedMetadata 存储我们处理过的、格式化后的元数据
type ProcessedMetadata struct {
	FileSize   int64
	Duration   float64
	Resolution string
	Format     string
}

// preprocessVideoWithFFmpeg 使用 FFmpeg 预处理视频，返回临时文件路径
func preprocessVideoWithFFmpeg(inputPath string) (string, error) {
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

// discoverAndProcessSubtitles 发现、处理并复制字幕文件
func discoverAndProcessSubtitles(inputPath string, trackExtensions []string, encBaseName string) ([]types.SubtitleTrack, error) {
	fmt.Println("-> Step 2: Processing subtitle tracks...")
	videoDir := filepath.Dir(inputPath)
	videoBaseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputDir := filepath.Dir(filepath.Dir(inputPath)) // 假设输出目录是输入目录的父目录，或者从外部传入更健壮

	// a. 发现所有匹配的字幕
	files, _ := os.ReadDir(videoDir)
	sortedExts := utils.SortExtensionsByLength(trackExtensions)
	var subtitleTracks []types.SubtitleTrack

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		fileName := f.Name()

		isSubtitle := false
		for _, ext := range sortedExts {
			if strings.HasSuffix(fileName, ext) {
				isSubtitle = true
				break
			}
		}
		if !isSubtitle {
			continue
		}

		subBaseName := utils.StripKnownExtensions(fileName, sortedExts)
		if (strings.HasPrefix(subBaseName, videoBaseName) || subBaseName == videoBaseName) && subBaseName != "" {
			fmt.Printf("-> Found track: %s\n", fileName)
			lang := "und"
			if strings.Contains(fileName, "chi") || strings.Contains(fileName, "zh") {
				lang = "chi"
			} else if strings.Contains(fileName, "eng") {
				lang = "eng"
			}
			subtitleTracks = append(subtitleTracks, types.SubtitleTrack{
				Language: lang,
				Title:    "",       // 稍后填充
				Filename: fileName, // 记录原始文件名
			})
		}
	}

	// b. 如果有字幕，则进行排序、复制和重命名
	if len(subtitleTracks) > 0 {
		if err := sortAndCopySubtitles(&subtitleTracks, videoBaseName, videoDir, outputDir, encBaseName, sortedExts); err != nil {
			return nil, err
		}
	}

	return subtitleTracks, nil
}

// sortAndCopySubtitles 排序、复制并更新字幕切片信息
func sortAndCopySubtitles(subtitleTracks *[]types.SubtitleTrack, videoBaseName, videoDir, outputDir, encBaseName string, sortedExts []string) error {
	// 排序
	sort.Slice(*subtitleTracks, func(i, j int) bool {
		subI := &(*subtitleTracks)[i]
		subJ := &(*subtitleTracks)[j]
		pureBaseNameI := utils.StripKnownExtensions(subI.Filename, sortedExts)
		pureBaseNameJ := utils.StripKnownExtensions(subJ.Filename, sortedExts)
		isIPureMatch := pureBaseNameI == videoBaseName
		isJPureMatch := pureBaseNameJ == videoBaseName
		if isIPureMatch && !isJPureMatch {
			return true
		}
		if !isIPureMatch && isJPureMatch {
			return false
		}
		if len(pureBaseNameI) != len(pureBaseNameJ) {
			return len(pureBaseNameI) < len(pureBaseNameJ)
		}
		isIDm := strings.HasPrefix(subI.Filename[len(pureBaseNameI):], ".dm.")
		isJDm := strings.HasPrefix(subJ.Filename[len(pureBaseNameJ):], ".dm.")
		if isIDm && !isJDm {
			return true
		}
		if !isIDm && isJDm {
			return false
		}
		return subI.Filename < subJ.Filename
	})

	// 复制、重命名并更新 title
	for i := range *subtitleTracks {
		sub := &(*subtitleTracks)[i]
		originalSubFilename := sub.Filename
		originalSubPath := filepath.Join(videoDir, originalSubFilename)

		if _, err := os.Stat(originalSubPath); os.IsNotExist(err) {
			fmt.Printf("-> Warning: Original subtitle file not found at %s, skipping.\n", originalSubPath)
			continue
		}

		ext := filepath.Ext(originalSubFilename)
		var newSubFilename string
		if i == 0 {
			newSubFilename = fmt.Sprintf("%s%s", encBaseName, ext)
		} else {
			newSubFilename = fmt.Sprintf("%s.%d%s", encBaseName, i+1, ext)
		}
		newSubPath := filepath.Join(outputDir, newSubFilename)

		fmt.Printf("-> Copying subtitle '%s' to '%s'.\n", originalSubFilename, newSubFilename)
		if err := utils.CopyFile(originalSubPath, newSubPath); err != nil {
			return fmt.Errorf("failed to copy subtitle from %s to %s: %w", originalSubPath, newSubPath, err)
		}

		sub.Title = newSubFilename
	}
	return nil
}

// encryptVideoFile 加密视频文件并返回 IV
func encryptVideoFile(inputPath, outputEncPath, password string, salt []byte) ([]byte, error) {
	fmt.Println("-> Step 3: Encrypting processed video file...")
	key := crypto.GenerateKey(password, salt)
	iv := make([]byte, crypto.IVLength)
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	inputFile, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open input file for encryption: %w", err)
	}
	defer inputFile.Close()

	outputFile, err := os.Create(outputEncPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create encrypted output file: %w", err)
	}
	defer outputFile.Close()

	if err := crypto.EncryptStream(inputFile, outputFile, key, iv); err != nil {
		return nil, fmt.Errorf("encryption stream failed: %w", err)
	}
	return iv, nil
}

// createKVIFile 将 VideoIndex 结构体写入 KVI 文件
func createKVIFile(indexPath string, index *types.VideoIndex) error {
	fmt.Println("-> Step 4: Writing index file...")
	indexData, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}
	return os.WriteFile(indexPath, indexData, 0644)
}

// 获取并处理视频元数据
func getProcessedMetadata(path string) (*ProcessedMetadata, error) {
	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var rawMeta FFProbeRawMetadata
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

	return &ProcessedMetadata{
		FileSize:   fileInfo.Size(),
		Duration:   parseDuration(rawMeta.Format.Duration),
		Resolution: fmt.Sprintf("%dx%d", width, height),
		Format:     strings.TrimPrefix(filepath.Ext(path), "."),
	}, nil
}

// 解析时长字符串
func parseDuration(d string) float64 {
	var h, m, s float64
	fmt.Sscanf(d, "%f:%f:%f", &h, &m, &s)
	return h*3600 + m*60 + s
}
