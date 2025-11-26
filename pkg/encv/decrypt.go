package encv

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/utils"
)

// decryptSingleFile 解密单个容器文件（或分片文件集）并还原其关联的字幕
func decryptSingleFile(anyChunkPath, password, outputDir string) error {
	fmt.Printf("-> Processing file: %s\n", anyChunkPath)

	// 1. 【关键修改】根据任意一个分片，找到主分片
	mainChunkPath, err := container.FindMainChunk(anyChunkPath)
	if err != nil {
		return fmt.Errorf("failed to find main chunk for '%s': %w", anyChunkPath, err)
	}
	fmt.Printf("-> Found main chunk: %s\n", filepath.Base(mainChunkPath))

	// 2. 【关键修改】使用新的分片解包函数
	packedData, err := container.UnpackChunked(mainChunkPath)
	if err != nil {
		return fmt.Errorf("failed to unpack main chunk: %w", err)
	}
	defer packedData.VideoStream.Close()

	// 3. 解析 KVI 数据
	index, err := crypto.UnmarshalKVI(packedData.KVIData)
	if err != nil {
		return fmt.Errorf("failed to parse KVI from container: %w", err)
	}

	// 4. 确定输出文件名
	originalFilename := filepath.Base(index.OriginalFilename)
	if originalFilename == "" || originalFilename == "unknown" {
		fmt.Println("-> [Warning] 'original_filename' in KVI is missing. Inferring a default name.")
		// 从主分片路径推断基础名
		baseName := strings.TrimSuffix(filepath.Base(mainChunkPath), config.GetVideoEncExtension())
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

	// --- 8. 还原字幕文件 ---
	if len(index.Subtitles) > 0 {
		fmt.Println("-> Restoring associated subtitle files...")
		// 字幕文件与主分片在同一目录
		sourceDir := filepath.Dir(mainChunkPath)

		for _, track := range index.Subtitles {
			sourceTrackName := track.Title
			destTrackName := track.Filename

			if sourceTrackName == "" || destTrackName == "" {
				fmt.Printf("-> [Warning] Incomplete subtitle info in KVI (title: '%s', filename: '%s'), skipping.\n", sourceTrackName, destTrackName)
				continue
			}

			sourceTrackPath := filepath.Join(sourceDir, sourceTrackName)
			destTrackPath := filepath.Join(outputDir, destTrackName)

			if _, err := os.Stat(sourceTrackPath); os.IsNotExist(err) {
				fmt.Printf("-> [Warning] Track file not found at source: %s. Skipping.\n", sourceTrackPath)
				continue
			}

			if err := utils.CopyFile(sourceTrackPath, destTrackPath); err != nil {
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

// Decrypt ... (修改以支持分片) ...
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

	// 【关键修改】收集所有容器文件并排序，确保处理顺序一致
	var containerFiles []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && config.IsContainerFile(entry.Name()) {
			containerFiles = append(containerFiles, entry)
		}
	}
	sort.Slice(containerFiles, func(i, j int) bool {
		return containerFiles[i].Name() < containerFiles[j].Name()
	})

	if len(containerFiles) == 0 {
		fmt.Println("-> No ENCV container files found in the directory.")
		return nil
	}

	// 【关键修改】使用 map 来跟踪已处理的文件集，避免重复解密
	processedPrefixes := make(map[string]bool)

	for _, entry := range containerFiles {
		// 从文件名中提取基础前缀（例如 "movie.sccgv"）
		basePrefix := getBasePrefix(entry.Name())
		if processedPrefixes[basePrefix] {
			continue // 这个文件集已经处理过了
		}

		containerPath := filepath.Join(inputPath, entry.Name())
		fmt.Printf("\n--- Attempting to decrypt file set: %s ---\n", basePrefix)
		if err := decryptSingleFile(containerPath, opts.Password, opts.OutputDir); err != nil {
			fmt.Printf("-> [Error] Failed to decrypt %s: %v\n", basePrefix, err)
		} else {
			fmt.Printf("-> Successfully decrypted %s\n", basePrefix)
		}

		// 标记这个文件集为已处理
		processedPrefixes[basePrefix] = true
	}

	return nil
}

// getBasePrefix 从容器文件名中提取基础前缀
// 例如: "movie.sccgv.enc2" -> "movie.sccgv"
//
//	"movie.sccgv"      -> "movie.sccgv"
func getBasePrefix(fileName string) string {
	// 检查是否是子分片
	if idx := strings.LastIndex(fileName, ".encv"); idx > 0 {
		return fileName[:idx]
	}
	// 如果不是子分片，说明它就是主分片或单文件，直接返回其名称
	return fileName
}
