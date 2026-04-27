// internal/v2/plugins/video/content_preprocessor.go

package video

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/reader"
)

// VideoContentPreprocessor 实现 plugins.ContentPreprocessor 接口
type VideoContentPreprocessor struct {
	// 可以在这里注入依赖，例如配置
	settings  VideoPluginConfig
	index     *VideoIndex
	outputDir string
	// 【新增】存储分片文件路径集合，用于快速判断是否是分片文件
	splitPartPaths map[string]bool
}

// Preprocess 预处理视频内容，根据策略和文件状态决定最优处理方式
func (p *VideoContentPreprocessor) Preprocess(inputPath string) (io.ReadCloser, error) {
	log.Printf("-> [CONTENT_PREPROCESSOR] Analyzing '%s' for optimal processing...\n", filepath.Base(inputPath))

	// 1. 检测源格式
	format, err := utils.DetectVideoFormat(inputPath)
	if err != nil {
		log.Printf("-> [CONTENT_PREPROCESSOR] Warning: Could not detect format for %s, falling back to transcoding. Error: %v\n", filepath.Base(inputPath), err)
		reader, path, err := p.transcodeToFastStartMP4(inputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to transcode to fast-start MP4: %w", err)
		}
		p.updateWithPreprocessedInfo(path, "mp4")
		return reader, nil
	}

	// 2. 应用策略：MKV 保持
	isMkv := strings.ToLower(format) == "mkv"
	if isMkv && p.settings.KeepMkvForMkvSource {
		// 【关键修复】如果是分片文件且启用了不合并模式，跳过 mkvmerge 重新封装
		// 直接读取原始文件，避免 mkvmerge 失败
		if p.settings.SkipMergeForSplitMKV && p.splitPartPaths != nil && p.splitPartPaths[inputPath] {
			fmt.Println("-> [CONTENT_PREPROCESSOR] Strategy: Split MKV part with SkipMerge enabled. Using original file directly.")
			reader, err := os.Open(inputPath)
			if err != nil {
				return nil, fmt.Errorf("failed to open split MKV part: %w", err)
			}
			p.updateWithPreprocessedInfo(inputPath, "mkv")
			return reader, nil
		}

		// 【关键修复】如果文件是合并后的缓存文件（在缓存目录中），跳过 remux
		// 因为合并过程已经使用 mkvmerge 创建，不需要再次 remux
		// 【修复】使用 filepath 比较，处理路径分隔符差异（/ vs \）
		if p.settings.PluginCacheDir != "" {
			// 统一使用 filepath 处理路径分隔符
			cacheDir := filepath.Clean(p.settings.PluginCacheDir)
			inputDir := filepath.Clean(filepath.Dir(inputPath))
			if cacheDir == inputDir {
				fmt.Println("-> [CONTENT_PREPROCESSOR] Strategy: Source is already a merged MKV in cache dir. Using directly.")
				reader, err := os.Open(inputPath)
				if err != nil {
					return nil, fmt.Errorf("failed to open cached merged MKV: %w", err)
				}
				p.updateWithPreprocessedInfo(inputPath, "mkv")
				return reader, nil
			}
		}

		fmt.Println("-> [CONTENT_PREPROCESSOR] Strategy: Source is MKV and 'keep_mkv' is enabled. Remuxing with mkvmerge.")
		reader, path, err := p.remapWithMKVMerge(inputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to remux MKV: %w", err)
		}
		p.updateWithPreprocessedInfo(path, "mkv")
		return reader, nil
	}

	// 3. 智能处理 MP4
	isMp4 := strings.ToLower(format) == "mp4"
	if isMp4 {
		fmt.Println("-> [CONTENT_PREPROCESSOR] Detected MP4, checking for fast-start...")
		if fast, err := isMP4FastStart(inputPath); err == nil && fast {
			// 【关键优化】已经是 fast-start，直接复制
			fmt.Println("-> [CONTENT_PREPROCESSOR] Input is already a fast-start MP4, using file directly (no processing).")
			reader, _ := os.Open(inputPath)
			p.updateWithPreprocessedInfo(inputPath, "mp4")
			return reader, nil
		} else {
			// 不是 fast-start，需要重新封装
			if err != nil {
				log.Printf("-> [CONTENT_PREPROCESSOR] Warning: Could not verify fast-start status (%v), proceeding with remuxing.\n", err)
			} else {
				fmt.Println("-> [CONTENT_PREPROCESSOR] Input is not a fast-start MP4, remuxing is required.")
			}
			reader, path, err := p.remapMP4ForFastStart(inputPath)
			if err != nil {
				return nil, fmt.Errorf("failed to remux MP4: %w", err)
			}
			p.updateWithPreprocessedInfo(path, "mp4")
			return reader, nil
		}
	}

	// 4. 兜底策略：其他格式转码
	log.Printf("-> [CONTENT_PREPROCESSOR] Strategy: Source is '%s', transcoding to fast-start MP4.\n", format)
	reader, path, err := p.transcodeToFastStartMP4(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to transcode to fast-start MP4: %w", err)
	}
	p.updateWithPreprocessedInfo(path, "mp4")
	return reader, nil
}

// 一个辅助函数，用于用预处理后的文件信息更新 index
func (p *VideoContentPreprocessor) updateWithPreprocessedInfo(preprocessedPath, finalFormat string) {
	// 1. 更新最终格式
	p.index.Format = finalFormat

	// 2. 权威获取 MIME 类型
	if mimeType, err := utils.DetectFileMIMEType(preprocessedPath); err == nil {
		p.index.MimeType = mimeType
	}

	// 3. 权威获取时长
	cmd := exec.Command("ffprobe", "-v", "quiet", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", preprocessedPath)
	output, err := cmd.Output()
	if err == nil {
		if d, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64); err == nil {
			p.index.DurationSeconds = d
			log.Printf("-> [CONTENT_PREPROCESSOR] Updated duration to: %f seconds\n", d)
		}
	}

	// 4. 提取章节
	// 【关键修复】对于 MKV 文件，优先使用 mkvextract 提取章节，因为它更可靠
	var chapters []MKVChapterInfo
	if finalFormat == "mkv" {
		chapters, err = ExtractChaptersWithMKVExtract(preprocessedPath)
		if err != nil {
			log.Printf("-> [CONTENT_PREPROCESSOR] Warning: mkvextract failed, falling back to ffprobe: %v\n", err)
			chapters, err = extractChaptersWithFFprobe(preprocessedPath)
		}
	} else {
		chapters, err = extractChaptersWithFFprobe(preprocessedPath)
	}
	if err == nil && chapters != nil {
		p.index.Chapters = chapters
		log.Printf("-> [CONTENT_PREPROCESSOR] Updated %d chapters.\n", len(chapters))
	} else {
		log.Printf("-> [CONTENT_PREPROCESSOR] Warning: Could not extract chapters from preprocessed file: %v\n", err)
	}

	// 5. 提取关键帧偏移
	keyFrames, err := extractKeyFrameOffsets(preprocessedPath, finalFormat)
	if err == nil {
		p.index.KeyFrameOffsets = keyFrames
		log.Printf("-> [CONTENT_PREPROCESSOR] Found %d keyframes for intelligent splitting.\n", len(keyFrames))
	} else {
		log.Printf("-> [CONTENT_PREPROCESSOR] Warning: Could not extract keyframes for intelligent splitting: %v\n", err)
		p.index.KeyFrameOffsets = nil // 如果失败，则置为 nil
	}
}

// extractKeyFrameOffsets 提取关键帧偏移
// MP4 通常是 Length-Prefix 封装，Binary Scan 只会命中 moov 中的 SPS/PPS (伪关键帧)，导致分片错误。
func extractKeyFrameOffsets(filePath string, format string) ([]uint64, error) {
	if strings.ToLower(format) == "mkv" {
		// 对于MKV文件，使用更可靠的方法
		return extractKeyFrameOffsetsFromMKV(filePath)
	}

	//  尝试 FFProbe
	fmt.Println("-> [DIAG] Attempting optimized FFProbe extraction...")
	offsets, err := extractKeyFrameOffsetsWithFFProbe(filePath)
	if err == nil && len(offsets) > 0 {
		return offsets, nil
	}
	log.Printf("-> [DIAG] FFProbe failed or empty (%v). Attempting binary NAL scan...\n", err)

	//  回退：返回空，让打包逻辑使用智能的大小计算
	return nil, fmt.Errorf("all keyframe extraction methods failed")
}

func extractKeyFrameOffsetsWithFFProbe(filePath string) ([]uint64, error) {
	fmt.Println("-> [DIAG] Optimized: Extracting exact keyframe positions in a single pass.")

	// 单次命令：
	// -select_streams v:0: 仅选择第一个视频流
	// -skip_frame nokey: 【关键】直接丢弃非关键帧，只处理 I 帧
	// -show_entries frame=pkt_pts_time,pkt_pos: 获取时间戳和字节位置
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-skip_frame", "nokey",
		"-show_entries", "frame=pkt_pts_time,pkt_pos",
		"-of", "csv=p=0",
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe keyframe command failed: %w", err)
	}

	var offsets []uint64
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 输出格式为：pts_time,pkt_pos
		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			posStr := strings.TrimSpace(parts[1])

			// 容错处理：有些容器可能无法提供 pkt_pos
			if posStr == "N/A" || posStr == "" {
				// 如果某个关键帧没有位置信息，跳过它（虽然少见，但在某些流格式中可能发生）
				// 或者可以选择记录警告
				continue
			}

			if offset, err := strconv.ParseUint(posStr, 10, 64); err == nil {
				offsets = append(offsets, offset)
			}
			// 解析失败通常意味着格式问题，忽略该帧
		}
	}

	if len(offsets) == 0 {
		return nil, fmt.Errorf("no valid keyframes found with ffprobe")
	}

	log.Printf("-> [DIAG] SUCCESS: Extracted %d exact keyframe positions in a single pass.\n", len(offsets))
	return offsets, nil
}

// remapWithMKVMerge 使用 mkvmerge 对 MKV 进行规范化
func (p *VideoContentPreprocessor) remapWithMKVMerge(inputPath string) (io.ReadCloser, string, error) {
	// 【关键诊断】在运行 mkvmerge 之前，检查原始文件
	fmt.Println("-> [DIAG] Checking original file for Cues...")
	if hasCues, err := checkFileForCues(inputPath); err == nil {
		if hasCues {
			fmt.Println("-> [DIAG] Original file HAS Cues.")
		} else {
			fmt.Println("-> [DIAG] Original file DOES NOT have Cues. This is likely the root cause.")
		}
	} else {
		log.Printf("-> [DIAG] WARNING: Could not check original file for Cues: %v\n", err)
	}
	tempFile, err := os.CreateTemp(p.outputDir, "encv-pre-*.mkv")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp file for MKV remuxing: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	// 【关键修复】不再获取具体的 ID，而是使用 'video' 类型指定符，让 mkvmerge 自己找
	// 尝试 1: mkvmerge with iframes for all video tracks
	fmt.Println("-> [DIAG] Attempting to create Cues for all video tracks with 'iframes' mode.")
	cmd := exec.Command("mkvmerge", "--cues", "video:iframes", "-o", tempPath, inputPath)
	log.Printf("-> [DIAG] Executing command: %s\n", cmd.String())
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(tempPath)
		return nil, tempPath, fmt.Errorf("mkvmerge remuxing failed: %w", err)
	}

	// 检查 Cues 是否真的生成了
	hasCues, _ := checkFileForCues(tempPath)
	if !hasCues {
		log.Printf("-> [DIAG] FAILED: 'video:iframes' succeeded but created no Cues. Trying with 'all' as a last resort.\n")

		// 尝试 2: 使用 'all' 模式强制创建
		fmt.Println("-> [DIAG] Attempting to create Cues for all video tracks with 'all' mode.")
		cmdAll := exec.Command("mkvmerge", "--cues", "video:all", "-o", tempPath, inputPath)
		log.Printf("-> [DIAG] Executing command: %s\n", cmdAll.String())
		cmdAll.Stderr = os.Stderr
		if err := cmdAll.Run(); err != nil {
			os.Remove(tempPath)
			return nil, tempPath, fmt.Errorf("mkvmerge remuxing failed with both 'iframes' and 'all': %w", err)
		}
		fmt.Println("-> [DIAG] SUCCESS: mkvmerge with 'video:all' Cues succeeded.")
	} else {
		fmt.Println("-> [DIAG] SUCCESS: mkvmerge with 'video:iframes' Cues succeeded and Cues were created.")
	}

	log.Printf("-> [CONTENT_PREPROCESSOR] SUCCESS: Remuxed MKV to %s\n", tempPath)
	reader, _ := reader.NewTempFileReadCloser(tempPath)
	return reader, tempPath, nil
}

// remapMP4ForFastStart 将非 fast-start 的 MP4 重新封装
func (p *VideoContentPreprocessor) remapMP4ForFastStart(inputPath string) (io.ReadCloser, string, error) {
	fmt.Println("-> [CONTENT_PREPROCESSOR] Remuxing MP4 for fast-start...")
	tempFile, err := os.CreateTemp(p.outputDir, "encv-pre-*.mp4")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp file for MP4 remuxing: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	// 使用 ffmpeg 的 -c copy 和 -movflags +faststart 来重新封装，速度快且无质量损失
	ffmpegCmd := exec.Command("ffmpeg", "-y", "-i", inputPath, "-c", "copy", "-movflags", "+faststart", tempPath)
	ffmpegCmd.Stderr = os.Stderr
	if err := ffmpegCmd.Run(); err != nil {
		os.Remove(tempPath)
		return nil, tempPath, fmt.Errorf("ffmpeg failed to remux MP4: %w", err)
	}

	log.Printf("-> [CONTENT_PREPROCESSOR] SUCCESS: Remuxed to %s\n", tempPath)
	reader, _ := reader.NewTempFileReadCloser(tempPath)
	return reader, tempPath, nil
}

// transcodeToFastStartMP4 将其他格式转码为 H.264 MP4
func (p *VideoContentPreprocessor) transcodeToFastStartMP4(inputPath string) (io.ReadCloser, string, error) {
	fmt.Println("-> [CONTENT_PREPROCESSOR] Transcoding video to H.264/AAC MP4...")
	tempFile, err := os.CreateTemp(p.outputDir, "encv-pre-*.mp4")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp file for transcoding: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	// 使用硬件加速（如果可用）和合理的预设进行转码
	// TODO 这里会修改为可配置参数
	ffmpegCmd := exec.Command(
		"ffmpeg",
		"-y", // 覆盖输出文件
		"-i", inputPath,
		// 视频流：
		"-c:v", "h264_nvenc", // 或者 "libx264" 作为软件编码备选
		"-preset", "p4", // 编码速度与压缩率的平衡
		"-qp", "28", // 恒定质量，数值越小质量越高
		"-profile:v", "high",
		// 音频流：转换为 AAC
		"-c:a", "aac",
		"-movflags", "+faststart", // 确保转码后的 MP4 是 fast-start
		tempPath,
	)
	ffmpegCmd.Stderr = os.Stderr
	if err := ffmpegCmd.Run(); err != nil {
		os.Remove(tempPath) // 失败时清理临时文件
		return nil, tempPath, fmt.Errorf("ffmpeg failed to transcode video: %w", err)
	}

	log.Printf("-> [CONTENT_PREPROCESSOR] SUCCESS: Transcoded to %s\n", tempPath)
	reader, _ := reader.NewTempFileReadCloser(tempPath)
	return reader, tempPath, nil
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

// copyFile 复制文件的内容
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
