package processor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/types"
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

// ProcessVideo 处理单个视频文件
func ProcessVideo(inputPath, outputEncPath, outputIndexPath, password string, salt []byte, trackExtensions []string, originalFilename string) error {
	fmt.Printf("-> Processing %s...\n", filepath.Base(inputPath))

	// --- Step 1: Pre-processing with FFmpeg ---
	fmt.Println("-> Step 1: Pre-processing video for streaming...")
	tempFile, err := os.CreateTemp("", "encv-pre-*.mp4")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	tempPath := tempFile.Name()
	// 关闭文件句柄，让 FFmpeg 可以自由写入
	tempFile.Close()

	isMkv := strings.ToLower(filepath.Ext(inputPath)) == ".mkv"
	var ffmpegCmd *exec.Cmd
	// --- 关键修正 1: 添加 "-y" 参数强制覆盖 ---
	if isMkv {
		ffmpegCmd = exec.Command("ffmpeg", "-y", "-i", inputPath, "-c", "copy", tempPath)
	} else {
		ffmpegCmd = exec.Command("ffmpeg", "-y", "-i", inputPath, "-c", "copy", "-movflags", "+faststart", tempPath)
	}

	ffmpegCmd.Stderr = os.Stderr // 显示 FFmpeg 错误
	// --- 关键修正 2: 检查 FFmpeg 的执行错误 ---
	if err := ffmpegCmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}
	fmt.Println("-> Pre-processing complete.")

	// --- Step 2: Find and copy track files ---
	fmt.Println("-> Step 2: Searching for and copying track files...")
	videoDir := filepath.Dir(inputPath)
	videoBaseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputDir := filepath.Dir(outputEncPath)

	var subtitleTracks []types.SubtitleTrack
	sortedExts := sortExtensionsByLength(trackExtensions)

	files, _ := os.ReadDir(videoDir)
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		fileName := f.Name()
		fileNameWithoutExt := stripKnownExtensions(fileName, sortedExts)

		if fileNameWithoutExt == videoBaseName {
			srcTrackPath := filepath.Join(videoDir, fileName)
			destTrackPath := filepath.Join(outputDir, fileName)
			fmt.Printf("-> Found and copying track: %s\n", fileName)
			if err := copyFile(srcTrackPath, destTrackPath); err != nil {
				return fmt.Errorf("failed to copy track file %s: %w", fileName, err)
			}

			lang := "und"
			title := fileName
			if strings.Contains(fileName, "chi") || strings.Contains(fileName, "zh") {
				lang = "chi"
			} else if strings.Contains(fileName, "eng") {
				lang = "eng"
			}
			subtitleTracks = append(subtitleTracks, types.SubtitleTrack{
				Language: lang,
				Title:    title,
				Filename: fileName,
			})
		}
	}

	// --- Step 3: Encrypt video ---
	fmt.Println("-> Step 3: Encrypting processed video file...")
	key := crypto.GenerateKey(password, salt)

	inputFile, err := os.Open(tempPath)
	if err != nil {
		return fmt.Errorf("failed to open temp file for encryption: %w", err)
	}
	defer inputFile.Close()

	outputFile, err := os.Create(outputEncPath)
	if err != nil {
		return fmt.Errorf("failed to create encrypted output file: %w", err)
	}
	defer outputFile.Close()

	iv, err := crypto.EncryptStream(inputFile, outputFile, key)
	if err != nil {
		return fmt.Errorf("encryption stream failed: %w", err)
	}

	// --- Step 4: Get metadata and write index file ---
	fmt.Println("-> Step 4: Writing index file...")
	metadata, err := getProcessedMetadata(tempPath)
	if err != nil {
		return fmt.Errorf("failed to get metadata: %w", err)
	}

	index := types.VideoIndex{
		VideoID:          fmt.Sprintf("vid-%d", os.Getuid()),
		OriginalFileSize: metadata.FileSize,
		Format:           metadata.Format,
		Encryption: types.EncryptionInfo{
			Algorithm:  crypto.Algorithm,
			IVBase64:   crypto.Base64Encode(iv),
			SaltBase64: crypto.Base64Encode(salt),
		},
		SeekTable:        []interface{}{},
		DurationSeconds:  metadata.Duration,
		Resolution:       metadata.Resolution,
		OriginalFilename: originalFilename,
		Subtitles:        subtitleTracks,
	}

	indexData, _ := json.MarshalIndent(index, "", "  ")
	return os.WriteFile(outputIndexPath, indexData, 0644)
}

// sortExtensionsByLength 按长度降序排序扩展名
func sortExtensionsByLength(exts []string) []string {
	sorted := make([]string, len(exts))
	copy(sorted, exts)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})
	return sorted
}

// stripKnownExtensions 剥离已知的扩展名
func stripKnownExtensions(filename string, exts []string) string {
	name := filename
	for _, ext := range exts {
		if strings.HasSuffix(name, ext) {
			name = strings.TrimSuffix(name, ext)
			break
		}
	}
	return name
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// getProcessedMetadata 获取并处理视频元数据
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

// parseDuration 解析时长字符串
func parseDuration(d string) float64 {
	var h, m, s float64
	fmt.Sscanf(d, "%f:%f:%f", &h, &m, &s)
	return h*3600 + m*60 + s
}
