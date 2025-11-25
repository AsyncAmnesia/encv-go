package encv

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/types"
)

// --- 1. 解密单个文件的辅助函数 ---

func decryptSingleFile(encPath, password, outputDir string) error {
	fmt.Printf("-> Processing file: %s\n", encPath)

	// --- 根据新映射逻辑寻找 kvi 文件 ---
	baseFilename := strings.TrimSuffix(filepath.Base(encPath), ".enc") // e.g., "sample.4pm"
	indexPath := filepath.Join(filepath.Dir(encPath), baseFilename+".kvi")

	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("corresponding index file not found: %w", err)
	}

	var index types.VideoIndex
	if err := json.Unmarshal(indexData, &index); err != nil {
		return fmt.Errorf("failed to unmarshal index: %w", err)
	}

	// --- Step 2: 解密视频文件 ---
	salt, err := crypto.Base64Decode(index.Encryption.SaltBase64)
	if err != nil {
		return fmt.Errorf("failed to decode salt: %w", err)
	}
	key := crypto.GenerateKey(password, salt)

	iv, err := crypto.Base64Decode(index.Encryption.IVBase64)
	if err != nil {
		return fmt.Errorf("failed to decode IV: %w", err)
	}

	// 从索引文件中获取真实的原始文件名
	originalFilename := filepath.Base(index.OriginalFilename)
	if originalFilename == "" || originalFilename == "unknown" {
		fmt.Println("-> [Warning] 'original_filename' in index is missing. Inferring a default name.")
		originalFilename = baseFilename + ".mkv" // 回退
	}
	outputVideoPath := filepath.Join(outputDir, originalFilename)
	fmt.Printf("-> Decrypting to: %s\n", outputVideoPath)

	encFile, err := os.Open(encPath)
	if err != nil {
		return fmt.Errorf("failed to open encrypted file: %w", err)
	}
	defer encFile.Close()

	decFile, err := os.Create(outputVideoPath)
	if err != nil {
		return fmt.Errorf("failed to create output video file: %w", err)
	}
	defer decFile.Close()

	if err := crypto.DecryptStream(encFile, decFile, key, iv); err != nil {
		return fmt.Errorf("failed to decrypt video stream: %w", err)
	}

	// --- Step 3: 复制轨道文件 ---
	sourceDir := filepath.Dir(encPath)
	for _, track := range index.Subtitles {
		sourceTrackPath := filepath.Join(sourceDir, track.Filename)
		destTrackPath := filepath.Join(outputDir, track.Filename)

		if _, err := os.Stat(sourceTrackPath); os.IsNotExist(err) {
			fmt.Printf("Warning: Track file not found at source: %s\n", sourceTrackPath)
			continue
		}

		if err := copyFile(sourceTrackPath, destTrackPath); err != nil {
			fmt.Printf("Warning: Failed to copy track %s: %v\n", track.Filename, err)
		}
	}

	return nil
}

// --- 2. 主 Decrypt 函数 ---

// Decrypt 解密单个视频文件或整个目录。
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
		// --- 单文件处理 ---
		fmt.Printf("-> Input file: %s\n", inputPath)
		fmt.Printf("-> Target output directory: %s\n", opts.OutputDir)
		return decryptSingleFile(inputPath, opts.Password, opts.OutputDir)
	}

	// --- 目录处理 ---
	fmt.Printf("-> Processing all encrypted files in directory: %s\n", inputPath)
	entries, err := os.ReadDir(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	encFiles := []os.DirEntry{}
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".enc" {
			encFiles = append(encFiles, entry)
		}
	}

	if len(encFiles) == 0 {
		fmt.Println("-> No .enc files found in the directory.")
		return nil
	}

	for _, entry := range encFiles {
		encPath := filepath.Join(inputPath, entry.Name())
		baseFilename := strings.TrimSuffix(entry.Name(), ".enc")
		indexPath := filepath.Join(inputPath, baseFilename+".kvi")

		if _, err := os.Stat(indexPath); err == nil {
			fmt.Printf("\n--- Found pair: %s & %s.kvi ---\n", entry.Name(), baseFilename)
			if err := decryptSingleFile(encPath, opts.Password, opts.OutputDir); err != nil {
				fmt.Printf("-> [Error] Failed to decrypt %s: %v\n", entry.Name(), err)
			} else {
				fmt.Printf("-> Successfully decrypted %s\n", entry.Name())
			}
		} else {
			fmt.Printf("-> [Skipping] %s has no corresponding %s.kvi file.\n", entry.Name(), baseFilename)
		}
	}
	return nil
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
