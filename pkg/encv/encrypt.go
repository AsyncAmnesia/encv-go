package encv

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/processor"
	"github.com/Soltus/encv-go/internal/utils"
)

// detectContainerFormat 使用 ffprobe 检测视频文件的真实容器格式
func detectContainerFormat(filePath string) (string, error) {
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

// --- 2. 核心加密逻辑 ---

// Encrypt 加密单个视频文件或整个目录。
func Encrypt(inputPath string, opts EncryptOptions) error {
	if opts.Password == "" || opts.OutputDir == "" {
		return ErrMissingOptions
	}
	if len(opts.TrackExtensions) == 0 {
		opts.TrackExtensions = []string{".ass", ".srt", ".dm.ass"}
	}

	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	salt := make([]byte, crypto.SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		return fmt.Errorf("failed to stat input path: %w", err)
	}

	if info.IsDir() {
		return encryptDir(inputPath, opts, salt)
	}
	return encryptFile(inputPath, opts, salt)
}

// encryptFile 加密单个文件 (已修改以支持带优先级的多字幕)
func encryptFile(inputPath string, opts EncryptOptions, salt []byte) error {
	originalFilename := filepath.Base(inputPath)
	baseName := strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))

	// 1. 检测真实格式
	detectedFormat, err := detectContainerFormat(inputPath)
	if err != nil {
		return fmt.Errorf("failed to detect format for %s: %w", inputPath, err)
	}

	// 2. 【关键修改】生成临时文件名，不带任何加密后缀
	// processor.ProcessVideo 会在文件内部写入魔法数字，所以文件名后缀不重要
	tempEncPath := filepath.Join(opts.OutputDir, baseName+".encv_tmp_enc")   // 临时加密文件
	tempIndexPath := filepath.Join(opts.OutputDir, baseName+".encv_tmp_kvi") // 临时 KVI 文件
	// 【关键修复】使用 defer 确保无论如何都会清理临时文件
	defer func() {
		fmt.Println("-> Cleaning up temporary files...")
		// 忽略 os.Remove 的错误，因为清理失败不应该覆盖原始错误
		_ = os.Remove(tempEncPath)
		_ = os.Remove(tempIndexPath)
	}()

	fmt.Printf("-> Detected format: %s\n", detectedFormat)

	// 3. 调用 processor 处理视频
	if err := processor.ProcessVideo(inputPath, tempEncPath, tempIndexPath, opts.Password, salt, opts.TrackExtensions, originalFilename, baseName); err != nil {
		return fmt.Errorf("failed to process video: %w", err)
	}
	fmt.Printf("-> Successfully processed video and tracks to temporary files.\n")

	// 4. 打包前的准备：读取 KVI 和加密视频流
	kviData, err := os.ReadFile(tempIndexPath)
	if err != nil {
		return fmt.Errorf("failed to read KVI data: %w", err)
	}

	encFile, err := os.Open(tempEncPath)
	if err != nil {
		return fmt.Errorf("failed to open temp encrypted file: %w", err)
	}
	defer encFile.Close()

	// 【修正】使用正确的API构建最终文件名
	finalContainerPath := filepath.Join(opts.OutputDir, baseName+config.GetVideoEncExtension())

	if config.IsSccgvChunkingEnabled() {
		// --- 分片逻辑 ---
		fmt.Println("-> Chunking is enabled. Preparing to create chunked container...")

		// 4.1. 【关键修复】获取加密后文件的大小，用于判断是否需要分片
		encryptedFileInfo, err := os.Stat(tempEncPath)
		if err != nil {
			return fmt.Errorf("failed to stat encrypted temp file: %w", err)
		}
		totalEncryptedSize := encryptedFileInfo.Size()

		// 4.2. 计算原始文件的 MD5
		originalMD5, err := utils.CalculateOriginalMD5(inputPath)
		if err != nil {
			return fmt.Errorf("failed to calculate original file MD5: %w", err)
		}

		// 4.3. 主分片路径就是最终的容器路径
		mainChunkPath := finalContainerPath

		// 4.4. 获取分片大小
		chunkSize := config.GetSccgvChunkSize()

		// 4.5. 写入主分片
		fmt.Printf("-> Writing main chunk: %s\n", filepath.Base(mainChunkPath))
		if err := container.WriteMainChunk(mainChunkPath, kviData, io.LimitReader(encFile, int64(chunkSize)), originalMD5); err != nil {
			return fmt.Errorf("failed to write main chunk: %w", err)
		}

		// 4.6. 【关键修复】判断是否需要创建子分片
		// 如果总大小小于等于分片大小，主分片已经包含了所有数据，无需再分片
		if totalEncryptedSize > int64(chunkSize) {
			fmt.Println("-> File is larger than chunk size, creating sub-chunks...")
			chunkIndex := 2
			for {
				subChunkPath := fmt.Sprintf("%s%s%d", mainChunkPath, container.SubChunkSuffix, chunkIndex)
				fmt.Printf("-> Writing sub-chunk: %s\n", filepath.Base(subChunkPath))
				written, err := container.WriteSubChunk(mainChunkPath, chunkIndex, io.LimitReader(encFile, int64(chunkSize)), originalMD5)
				if err != nil {
					return fmt.Errorf("failed to write sub chunk %d: %w", chunkIndex, err)
				}
				if written == 0 { // EOF
					break
				}
				chunkIndex++
			}
		} else {
			fmt.Printf("-> Encrypted file size (%d) is not larger than chunk size (%d), no sub-chunks needed.\n", totalEncryptedSize, chunkSize)
		}

		fmt.Printf("✅ Encryption and chunking complete. Main chunk: %s\n", mainChunkPath)

	} else {
		// --- 单文件逻辑 ---
		fmt.Println("-> Chunking is disabled. Creating single-file container...")

		// 4.1. 关闭临时文件，因为 Pack 函数需要文件路径
		encFile.Close()
		// 重新打开，因为 defer 已经安排了关闭
		// 更好的方法是重写 Pack 函数以接受 io.Reader，但为了最小化改动，我们先用路径
		if err := container.Pack(tempEncPath, tempIndexPath, finalContainerPath); err != nil {
			return fmt.Errorf("failed to pack into container: %w", err)
		}

		fmt.Printf("✅ Encryption and packing complete. Final file: %s\n", finalContainerPath)
	}

	return nil
}

// encryptDir 加密目录中的所有视频文件
func encryptDir(inputDir string, opts EncryptOptions, salt []byte) error {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return fmt.Errorf("failed to read input directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !isVideoFile(entry.Name()) {
			continue
		}

		fullPath := filepath.Join(inputDir, entry.Name())
		if err := encryptFile(fullPath, opts, salt); err != nil {
			fmt.Printf("Error processing %s: %v\n", entry.Name(), err)
		}
	}
	return nil
}

// isVideoFile 检查文件是否是已知的视频类型
func isVideoFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".mp4" || ext == ".mkv" || ext == ".mov" || ext == ".webm"
}
