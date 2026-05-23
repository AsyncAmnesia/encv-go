package video

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/reader"
)

type VideoContentPreprocessor struct {
	settings       VideoPluginConfig
	index          *VideoIndex
	outputDir      string
	splitPartPaths map[string]bool
	ctx            context.Context
	onFFmpegProgress func(percent float64, speed string)
}

var (
	ffmpegTimeRe  = regexp.MustCompile(`time=(\d+):(\d+):(\d+\.\d+)`)
	ffmpegSpeedRe = regexp.MustCompile(`speed=\s*([\d.]+)\w*`)
	encoderCache  struct {
		sync.Once
		preferred string
	}
)

func detectPreferredEncoder() string {
	encoders := []struct {
		name   string
		args   []string
	}{
		{"h264_nvenc", []string{"-f", "lavfi", "-i", "nullsrc=s=256x256:d=0.1", "-c:v", "h264_nvenc", "-f", "null", "-"}},
		{"h264_mediacodec", []string{"-f", "lavfi", "-i", "nullsrc=s=256x256:d=0.1", "-c:v", "h264_mediacodec", "-f", "null", "-"}},
		{"libx264", nil},
	}

	for _, enc := range encoders {
		if enc.args == nil {
			return enc.name
		}
		if err := utils.FFmpegRun(append([]string{"-y", "-threads", "1"}, enc.args...)...); err == nil {
			log.Printf("-> [CONTENT_PREPROCESSOR] Detected available encoder: %s\n", enc.name)
			return enc.name
		}
	}
	return "libx264"
}

func getPreferredEncoder() string {
	encoderCache.Do(func() {
		encoderCache.preferred = detectPreferredEncoder()
	})
	return encoderCache.preferred
}

func (p *VideoContentPreprocessor) runFFmpegCmd(cmd *exec.Cmd, tempPath string) error {
	if utils.IsMobile() {
		return p.runFFmpegCmdMobile(cmd.Args[1:], tempPath)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	var totalDuration float64
	if p.index != nil && p.index.DurationSeconds > 0 {
		totalDuration = p.index.DurationSeconds
	}

	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if p.ctx != nil {
			select {
			case <-p.ctx.Done():
				cmd.Process.Kill()
				return p.ctx.Err()
			default:
			}
		}

		if totalDuration > 0 && p.onFFmpegProgress != nil {
			var timeSec float64
			if m := ffmpegTimeRe.FindStringSubmatch(line); len(m) > 3 {
				h, _ := strconv.ParseFloat(m[1], 64)
				mn, _ := strconv.ParseFloat(m[2], 64)
				s, _ := strconv.ParseFloat(m[3], 64)
				timeSec = h*3600 + mn*60 + s
			}

			var speedStr string
			if m := ffmpegSpeedRe.FindStringSubmatch(line); len(m) > 1 {
				speedStr = m[1] + "x"
			}

			if timeSec > 0 {
				percent := (timeSec / totalDuration) * 100
				if percent > 99 {
					percent = 99
				}
				p.onFFmpegProgress(percent, speedStr)
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		if p.ctx != nil {
			select {
			case <-p.ctx.Done():
				return p.ctx.Err()
			default:
			}
		}
		return fmt.Errorf("ffmpeg command failed: %w", err)
	}

	return nil
}

func (p *VideoContentPreprocessor) runFFmpegCmdMobile(args []string, tempPath string) error {
	if p.ctx != nil {
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		default:
		}
	}

	_, err := utils.FFmpegRunWithStderr(args...)
	if err != nil {
		return err
	}

	if _, statErr := os.Stat(tempPath); statErr != nil {
		return fmt.Errorf("ffmpeg completed but output file not found: %s", tempPath)
	}

	return nil
}

func (p *VideoContentPreprocessor) Preprocess(inputPath string) (io.ReadCloser, error) {
	log.Printf("-> [CONTENT_PREPROCESSOR] Analyzing '%s' for optimal processing...\n", filepath.Base(inputPath))

	if p.ctx != nil {
		select {
		case <-p.ctx.Done():
			return nil, p.ctx.Err()
		default:
		}
	}

	format, err := utils.DetectVideoFormat(inputPath)
	if err != nil {
		log.Printf("-> [CONTENT_PREPROCESSOR] Warning: Could not detect format for %s, falling back to transcoding. Error: %v\n", filepath.Base(inputPath), err)
		r, path, err := p.transcodeToFastStartMP4(inputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to transcode to fast-start MP4: %w", err)
		}
		p.updateWithPreprocessedInfo(path, "mp4")
		return r, nil
	}

	isMkv := strings.ToLower(format) == "mkv"
	if isMkv && p.settings.KeepMkvForMkvSource {
		if p.settings.SkipMergeForSplitMKV && p.splitPartPaths != nil && p.splitPartPaths[inputPath] {
			fmt.Println("-> [CONTENT_PREPROCESSOR] Strategy: Split MKV part with SkipMerge enabled. Using original file directly.")
			r, err := os.Open(inputPath)
			if err != nil {
				return nil, fmt.Errorf("failed to open split MKV part: %w", err)
			}
			p.updateWithPreprocessedInfo(inputPath, "mkv")
			return r, nil
		}

		if p.settings.PluginCacheDir != "" {
			cacheDir := filepath.Clean(p.settings.PluginCacheDir)
			inputDir := filepath.Clean(filepath.Dir(inputPath))
			if cacheDir == inputDir {
				fmt.Println("-> [CONTENT_PREPROCESSOR] Strategy: Source is already a merged MKV in cache dir. Using directly.")
				r, err := os.Open(inputPath)
				if err != nil {
					return nil, fmt.Errorf("failed to open cached merged MKV: %w", err)
				}
				p.updateWithPreprocessedInfo(inputPath, "mkv")
				return r, nil
			}
		}

		fmt.Println("-> [CONTENT_PREPROCESSOR] Strategy: Source is MKV and 'keep_mkv' is enabled. Remuxing with mkvmerge.")
		r, path, err := p.remapWithMKVMerge(inputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to remux MKV: %w", err)
		}
		p.updateWithPreprocessedInfo(path, "mkv")
		return r, nil
	}

	isMp4 := strings.ToLower(format) == "mp4"
	if isMp4 {
		fmt.Println("-> [CONTENT_PREPROCESSOR] Detected MP4, checking for fast-start...")
		if fast, err := isMP4FastStart(inputPath); err == nil && fast {
			fmt.Println("-> [CONTENT_PREPROCESSOR] Input is already a fast-start MP4, using file directly (no processing).")
			r, _ := os.Open(inputPath)
			p.updateWithPreprocessedInfo(inputPath, "mp4")
			return r, nil
		} else {
			if err != nil {
				log.Printf("-> [CONTENT_PREPROCESSOR] Warning: Could not verify fast-start status (%v), proceeding with remuxing.\n", err)
			} else {
				fmt.Println("-> [CONTENT_PREPROCESSOR] Input is not a fast-start MP4, remuxing is required.")
			}
			r, path, err := p.remapMP4ForFastStart(inputPath)
			if err != nil {
				return nil, fmt.Errorf("failed to remux MP4: %w", err)
			}
			p.updateWithPreprocessedInfo(path, "mp4")
			return r, nil
		}
	}

	log.Printf("-> [CONTENT_PREPROCESSOR] Strategy: Source is '%s', transcoding to fast-start MP4.\n", format)
	r, path, err := p.transcodeToFastStartMP4(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to transcode to fast-start MP4: %w", err)
	}
	p.updateWithPreprocessedInfo(path, "mp4")
	return r, nil
}

func (p *VideoContentPreprocessor) updateWithPreprocessedInfo(preprocessedPath, finalFormat string) {
	p.index.Format = finalFormat

	if mimeType, err := utils.DetectFileMIMEType(preprocessedPath); err == nil {
		p.index.MimeType = mimeType
	}

	output, err := utils.FFProbeOutput("-v", "quiet", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", preprocessedPath)
	if err == nil {
		if d, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64); err == nil {
			p.index.DurationSeconds = d
			log.Printf("-> [CONTENT_PREPROCESSOR] Updated duration to: %f seconds\n", d)
		}
	}

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

	keyFrames, err := extractKeyFrameOffsets(preprocessedPath, finalFormat)
	if err == nil {
		p.index.KeyFrameOffsets = keyFrames
		log.Printf("-> [CONTENT_PREPROCESSOR] Found %d keyframes for intelligent splitting.\n", len(keyFrames))
	} else {
		log.Printf("-> [CONTENT_PREPROCESSOR] Warning: Could not extract keyframes for intelligent splitting: %v\n", err)
		p.index.KeyFrameOffsets = nil
	}
}

func extractKeyFrameOffsets(filePath string, format string) ([]uint64, error) {
	if strings.ToLower(format) == "mkv" {
		return extractKeyFrameOffsetsFromMKV(filePath)
	}

	fmt.Println("-> [DIAG] Attempting optimized FFProbe extraction...")
	offsets, err := extractKeyFrameOffsetsWithFFProbe(filePath)
	if err == nil && len(offsets) > 0 {
		return offsets, nil
	}
	log.Printf("-> [DIAG] FFProbe failed or empty (%v). Attempting binary NAL scan...\n", err)

	return nil, fmt.Errorf("all keyframe extraction methods failed")
}

func extractKeyFrameOffsetsWithFFProbe(filePath string) ([]uint64, error) {
	fmt.Println("-> [DIAG] Optimized: Extracting exact keyframe positions in a single pass.")

	output, err := utils.FFProbeOutput(
		"-v", "error",
		"-select_streams", "v:0",
		"-skip_frame", "nokey",
		"-show_entries", "frame=pkt_pts_time,pkt_pos",
		"-of", "csv=p=0",
		filePath,
	)
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

		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			posStr := strings.TrimSpace(parts[1])

			if posStr == "N/A" || posStr == "" {
				continue
			}

			if offset, err := strconv.ParseUint(posStr, 10, 64); err == nil {
				offsets = append(offsets, offset)
			}
		}
	}

	if len(offsets) == 0 {
		return nil, fmt.Errorf("no valid keyframes found with ffprobe")
	}

	log.Printf("-> [DIAG] SUCCESS: Extracted %d exact keyframe positions in a single pass.\n", len(offsets))
	return offsets, nil
}

func (p *VideoContentPreprocessor) remapWithMKVMerge(inputPath string) (io.ReadCloser, string, error) {
	if utils.IsMobile() {
		return p.remapMKVWithFFmpeg(inputPath)
	}

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

	fmt.Println("-> [DIAG] Attempting to create Cues for all video tracks with 'iframes' mode.")
	cmd := exec.Command("mkvmerge", "--cues", "video:iframes", "-o", tempPath, inputPath)
	log.Printf("-> [DIAG] Executing command: %s\n", cmd.String())

	if p.ctx != nil {
		select {
		case <-p.ctx.Done():
			os.Remove(tempPath)
			return nil, tempPath, p.ctx.Err()
		default:
		}
	}

	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(tempPath)
		return nil, tempPath, fmt.Errorf("mkvmerge remuxing failed: %w", err)
	}

	hasCues, _ := checkFileForCues(tempPath)
	if !hasCues {
		log.Printf("-> [DIAG] FAILED: 'video:iframes' succeeded but created no Cues. Trying with 'all' as a last resort.\n")

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
	r, _ := reader.NewTempFileReadCloser(tempPath)
	return r, tempPath, nil
}

func (p *VideoContentPreprocessor) remapMKVWithFFmpeg(inputPath string) (io.ReadCloser, string, error) {
	tempFile, err := os.CreateTemp(p.outputDir, "encv-pre-*.mkv")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp file for MKV remuxing: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	args := []string{"-y", "-i", inputPath, "-c", "copy", "-reserve_index_space", "500", tempPath}
	if err := utils.FFmpegRun(args...); err != nil {
		os.Remove(tempPath)
		return nil, tempPath, fmt.Errorf("ffmpeg MKV remuxing failed: %w", err)
	}

	log.Printf("-> [CONTENT_PREPROCESSOR] SUCCESS: Remuxed MKV with ffmpeg to %s\n", tempPath)
	r, _ := reader.NewTempFileReadCloser(tempPath)
	return r, tempPath, nil
}

func (p *VideoContentPreprocessor) remapMP4ForFastStart(inputPath string) (io.ReadCloser, string, error) {
	fmt.Println("-> [CONTENT_PREPROCESSOR] Remuxing MP4 for fast-start...")
	tempFile, err := os.CreateTemp(p.outputDir, "encv-pre-*.mp4")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp file for MP4 remuxing: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	ffmpegCmd := utils.FFmpegCmd("-y", "-threads", "2", "-i", inputPath, "-c", "copy", "-movflags", "+faststart", tempPath)
	if err := p.runFFmpegCmd(ffmpegCmd, tempPath); err != nil {
		os.Remove(tempPath)
		return nil, tempPath, fmt.Errorf("ffmpeg failed to remux MP4: %w", err)
	}

	log.Printf("-> [CONTENT_PREPROCESSOR] SUCCESS: Remuxed to %s\n", tempPath)
	r, _ := reader.NewTempFileReadCloser(tempPath)
	return r, tempPath, nil
}

func (p *VideoContentPreprocessor) transcodeToFastStartMP4(inputPath string) (io.ReadCloser, string, error) {
	fmt.Println("-> [CONTENT_PREPROCESSOR] Transcoding video to H.264/AAC MP4...")
	tempFile, err := os.CreateTemp(p.outputDir, "encv-pre-*.mp4")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp file for transcoding: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	encoder := getPreferredEncoder()

	var args []string
	args = append(args, "-y", "-i", inputPath, "-threads", "2")

	switch encoder {
	case "h264_nvenc":
		args = append(args, "-c:v", "h264_nvenc", "-preset", "p4", "-qp", "28", "-profile:v", "high")
	case "h264_mediacodec":
		args = append(args, "-c:v", "h264_mediacodec", "-b:v", "5M")
	default:
		args = append(args, "-c:v", "libx264", "-preset", "medium", "-crf", "23", "-profile:v", "high")
	}

	args = append(args, "-c:a", "aac", "-movflags", "+faststart", tempPath)

	ffmpegCmd := utils.FFmpegCmd(args...)
	log.Printf("-> [CONTENT_PREPROCESSOR] Using encoder: %s, command: %s\n", encoder, ffmpegCmd.String())

	if err := p.runFFmpegCmd(ffmpegCmd, tempPath); err != nil {
		if encoder != "libx264" {
			log.Printf("-> [CONTENT_PREPROCESSOR] Encoder %s failed, falling back to libx264: %v\n", encoder, err)
			os.Remove(tempPath)

			tempFile2, err2 := os.CreateTemp(p.outputDir, "encv-pre-*.mp4")
			if err2 != nil {
				return nil, "", fmt.Errorf("failed to create temp file for fallback transcoding: %w", err2)
			}
			tempPath = tempFile2.Name()
			tempFile2.Close()

			fallbackCmd := utils.FFmpegCmd("-y", "-i", inputPath, "-threads", "2",
				"-c:v", "libx264", "-preset", "medium", "-crf", "23", "-profile:v", "high",
				"-c:a", "aac", "-movflags", "+faststart", tempPath)
			if err2 := p.runFFmpegCmd(fallbackCmd, tempPath); err2 != nil {
				os.Remove(tempPath)
				return nil, tempPath, fmt.Errorf("ffmpeg failed to transcode video (fallback): %w", err2)
			}
		} else {
			os.Remove(tempPath)
			return nil, tempPath, fmt.Errorf("ffmpeg failed to transcode video: %w", err)
		}
	}

	log.Printf("-> [CONTENT_PREPROCESSOR] SUCCESS: Transcoded to %s\n", tempPath)
	r, _ := reader.NewTempFileReadCloser(tempPath)
	return r, tempPath, nil
}

func isMP4FastStart(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	bufferSize := int64(1024 * 1024)
	r := io.LimitReader(file, bufferSize)
	header := make([]byte, bufferSize)
	n, err := io.ReadFull(r, header)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, err
	}
	header = header[:n]

	offset := int64(0)
	for offset < int64(len(header)) {
		if offset+8 > int64(len(header)) {
			break
		}

		atomSize := int64(binary.BigEndian.Uint32(header[offset : offset+4]))
		atomType := string(header[offset+4 : offset+8])

		if atomSize == 1 {
			if offset+16 > int64(len(header)) {
				break
			}
			atomSize = int64(binary.BigEndian.Uint64(header[offset+8 : offset+16]))
			offset += 16
		} else {
			offset += 8
		}

		if atomSize < 8 {
			break
		}

		switch atomType {
		case "moov":
			return true, nil
		case "mdat":
			return false, nil
		}

		offset += atomSize - 8
	}

	return false, nil
}

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
