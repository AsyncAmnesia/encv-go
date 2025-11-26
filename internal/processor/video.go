package processor

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
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

// ProcessVideo 处理单个视频文件 (职责已修改：仅发现和记录字幕)
func ProcessVideo(inputPath, outputEncPath, outputIndexPath, password string, salt []byte, trackExtensions []string, originalFilename string, encBaseName string) error {
	fmt.Printf("-> Processing %s...\n", filepath.Base(inputPath))

	// --- Step 1: Pre-processing with FFmpeg ---
	fmt.Println("-> Step 1: Pre-processing video for streaming...")
	tempFile, err := os.CreateTemp("", "encv-pre-*.mp4")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
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
		return fmt.Errorf("ffmpeg failed: %w", err)
	}
	fmt.Println("-> Pre-processing complete.")

	// --- Step 2: Find, sort, copy and record track files (新核心逻辑) ---
	fmt.Println("-> Step 2: Processing subtitle tracks...")
	videoDir := filepath.Dir(inputPath)
	videoBaseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputDir := filepath.Dir(outputEncPath) // 加密文件和字幕的输出目录

	var subtitleTracks []types.SubtitleTrack
	sortedExts := sortExtensionsByLength(trackExtensions)

	// a. 发现所有匹配的字幕
	files, _ := os.ReadDir(videoDir)
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

		subBaseName := StripKnownExtensions(fileName, sortedExts)
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
		// 定义一个本地 map 用于排序，避免循环依赖
		containerExtensionMap := map[string]string{
			"mp4": "4pm",
			"mkv": "vkm",
			"flv": "vfl",
		}
		pureVideoBaseName := videoBaseName
		for suffix, _ := range containerExtensionMap {
			if strings.HasSuffix(videoBaseName, "."+suffix) {
				pureVideoBaseName = strings.TrimSuffix(videoBaseName, "."+suffix)
				break
			}
		}

		// 排序
		sort.Slice(subtitleTracks, func(i, j int) bool {
			subI := &subtitleTracks[i]
			subJ := &subtitleTracks[j]
			pureBaseNameI := StripKnownExtensions(subI.Filename, sortedExts)
			pureBaseNameJ := StripKnownExtensions(subJ.Filename, sortedExts)
			isIPureMatch := pureBaseNameI == pureVideoBaseName
			isJPureMatch := pureBaseNameJ == pureVideoBaseName
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
		for i := range subtitleTracks { // 使用 range 获取指针以修改原切片
			sub := &subtitleTracks[i]
			originalSubFilename := sub.Filename
			originalSubPath := filepath.Join(videoDir, originalSubFilename)

			if _, err := os.Stat(originalSubPath); os.IsNotExist(err) {
				fmt.Printf("-> Warning: Original subtitle file not found at %s, skipping.\n", originalSubPath)
				continue
			}

			// 构造新的字幕文件名
			ext := filepath.Ext(originalSubFilename)
			var newSubFilename string
			if i == 0 {
				newSubFilename = fmt.Sprintf("%s%s", encBaseName, ext)
			} else {
				newSubFilename = fmt.Sprintf("%s.%d%s", encBaseName, i+1, ext)
			}
			newSubPath := filepath.Join(outputDir, newSubFilename)

			// 复制文件
			fmt.Printf("-> Copying subtitle '%s' to '%s'.\n", originalSubFilename, newSubFilename)
			if err := copyFile(originalSubPath, newSubPath); err != nil {
				return fmt.Errorf("failed to copy subtitle from %s to %s: %w", originalSubPath, newSubPath, err)
			}

			// 【关键】将重命名后的文件名写入 title 字段
			sub.Title = newSubFilename
		}
	}

	// --- Step 3: Encrypt video ---
	fmt.Println("-> Step 3: Encrypting processed video file...")
	key := crypto.GenerateKey(password, salt)

	// 这里生成 IV
	iv := make([]byte, crypto.IVLength)
	if _, err := rand.Read(iv); err != nil {
		return fmt.Errorf("failed to generate IV: %w", err)
	}

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

	if err := crypto.EncryptStream(inputFile, outputFile, key, iv); err != nil {
		return fmt.Errorf("encryption stream failed: %w", err)
	}

	// --- Step 3.5: Calculate MD5 of the encrypted file ---
	fmt.Println("-> Calculating MD5 of encrypted file...")
	encFileForHash, err := os.Open(outputEncPath)
	if err != nil {
		return fmt.Errorf("failed to open encrypted file for MD5 calculation: %w", err)
	}
	defer encFileForHash.Close()

	hasher := md5.New()
	if _, err := io.Copy(hasher, encFileForHash); err != nil {
		return fmt.Errorf("failed to calculate MD5 hash: %w", err)
	}
	md5Sum := hex.EncodeToString(hasher.Sum(nil))

	// --- Step 4: Get metadata and write index file ---
	fmt.Println("-> Step 4: Writing index file...")
	metadata, err := getProcessedMetadata(tempPath)
	if err != nil {
		return fmt.Errorf("failed to get metadata: %w", err)
	}

	// KVI 文件仍然需要记录 IV，以备不时之需（例如，解密器需要单独验证）
	// 但主要的解密流程将不再依赖它
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
		EncryptedFileMD5: md5Sum,
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

// stripKnownExtensions 剥离已知的扩展名 (注意 S 大写，变为公开函数)
func StripKnownExtensions(filename string, exts []string) string {
	name := filename
	for _, ext := range exts {
		if strings.HasSuffix(name, ext) {
			name = strings.TrimSuffix(name, ext)
			break
		}
	}
	return name
}

// copyFile 复制文件 (此函数现在在 processor 中不再被调用，但可以保留以备他用)
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
