package encv

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/types"
)

// copyFile 复制文件 (恢复此辅助函数，用于还原字幕)
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

// decryptSingleFile 解密单个容器文件并还原其关联的字幕
func decryptSingleFile(containerPath, password, outputDir string) error {
	fmt.Printf("-> Processing container file: %s\n", containerPath)

	// 1. 打开容器文件
	containerFile, err := os.Open(containerPath)
	if err != nil {
		return fmt.Errorf("failed to open container file: %w", err)
	}
	defer containerFile.Close()

	// 2. 解包，从中提取 KVI 数据和加密视频流
	packedData, err := container.Unpack(containerFile)
	if err != nil {
		return fmt.Errorf("failed to unpack container: %w", err)
	}
	defer packedData.VideoStream.Close()

	// 3. 解析 KVI 数据
	var index types.VideoIndex
	if err := json.Unmarshal(packedData.KVIData, &index); err != nil {
		return fmt.Errorf("failed to parse KVI from container: %w", err)
	}

	// 4. 确定输出文件名
	originalFilename := filepath.Base(index.OriginalFilename)
	if originalFilename == "" || originalFilename == "unknown" {
		fmt.Println("-> [Warning] 'original_filename' in KVI is missing. Inferring a default name.")
		baseName := filepath.Base(containerPath)
		baseName = strings.TrimSuffix(baseName, "."+config.GetVideoEncExtension())
		originalFilename = baseName + "." + index.Format
	}
	outputVideoPath := filepath.Join(outputDir, originalFilename)
	fmt.Printf("-> Decrypting to: %s\n", outputVideoPath)

	// 5. 准备解密密钥
	salt, err := crypto.Base64Decode(index.Encryption.SaltBase64)
	if err != nil {
		return fmt.Errorf("failed to decode salt: %w", err)
	}
	key := crypto.GenerateKey(password, salt)

	// 6. 创建输出文件
	decFile, err := os.Create(outputVideoPath)
	if err != nil {
		return fmt.Errorf("failed to create output video file: %w", err)
	}
	defer decFile.Close()

	// 7. 解密视频流并直接写入输出文件
	if err := crypto.DecryptStream(packedData.VideoStream, decFile, key); err != nil {
		os.Remove(outputVideoPath) // 解密失败，删除不完整的输出文件
		return fmt.Errorf("failed to decrypt video stream: %w", err)
	}

	fmt.Printf("-> Successfully decrypted video: %s\n", originalFilename)

	// --- 【新增】8. 还原字幕文件 ---
	if len(index.Subtitles) > 0 {
		fmt.Println("-> Restoring associated subtitle files...")
		sourceDir := filepath.Dir(containerPath) // 字幕文件与容器文件在同一目录

		for _, track := range index.Subtitles {
			// track.Title 是加密后的文件名, track.Filename 是原始文件名
			sourceTrackName := track.Title
			destTrackName := track.Filename

			if sourceTrackName == "" || destTrackName == "" {
				fmt.Printf("-> [Warning] Incomplete subtitle info in KVI (title: '%s', filename: '%s'), skipping.\n", sourceTrackName, destTrackName)
				continue
			}

			sourceTrackPath := filepath.Join(sourceDir, sourceTrackName)
			destTrackPath := filepath.Join(outputDir, destTrackName)

			// 检查源字幕文件是否存在
			if _, err := os.Stat(sourceTrackPath); os.IsNotExist(err) {
				fmt.Printf("-> [Warning] Track file not found at source: %s. Skipping.\n", sourceTrackPath)
				continue
			}

			// 复制文件
			if err := copyFile(sourceTrackPath, destTrackPath); err != nil {
				fmt.Printf("-> [Error] Failed to restore subtitle '%s': %v\n", destTrackName, err)
			} else {
				fmt.Printf("-> Successfully restored subtitle '%s'\n", destTrackName)
			}
		}
	} else {
		fmt.Println("-> No subtitles found in KVI to restore.")
	}

	return nil
}

// Decrypt ... (保持不变) ...
func Decrypt(inputPath string, opts DecryptOptions) error {
	if opts.Password == "" || opts.OutputDir == "" {
		return ErrMissingOptions
	}

	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		return fmt.Errorf("failed to stat input path: %w", err)
	}

	if !info.IsDir() {
		if !config.IsContainerFile(inputPath) {
			return fmt.Errorf("input file '%s' is not a recognized ENCV container file", inputPath)
		}
		fmt.Printf("-> Input file: %s\n", inputPath)
		fmt.Printf("-> Target output directory: %s\n", opts.OutputDir)
		return decryptSingleFile(inputPath, opts.Password, opts.OutputDir)
	}

	fmt.Printf("-> Processing all container files in directory: %s\n", inputPath)
	entries, err := os.ReadDir(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	containerFiles := []os.DirEntry{}
	for _, entry := range entries {
		if !entry.IsDir() && config.IsContainerFile(entry.Name()) {
			containerFiles = append(containerFiles, entry)
		}
	}

	if len(containerFiles) == 0 {
		fmt.Println("-> No ENCV container files found in the directory.")
		return nil
	}

	for _, entry := range containerFiles {
		containerPath := filepath.Join(inputPath, entry.Name())
		fmt.Printf("\n--- Attempting to decrypt %s ---\n", entry.Name())
		if err := decryptSingleFile(containerPath, opts.Password, opts.OutputDir); err != nil {
			fmt.Printf("-> [Error] Failed to decrypt %s: %v\n", entry.Name(), err)
		} else {
			fmt.Printf("-> Successfully decrypted %s\n", entry.Name())
		}
	}
	return nil
}
