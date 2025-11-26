package encv

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/types"
)

// calculateMD5 计算文件的 MD5 哈希值
func calculateMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s for MD5 calculation: %w", filePath, err)
	}
	defer file.Close()

	hasher := md5.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to calculate MD5 hash for %s: %w", filePath, err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// findMatchingKVI 通过快速匹配和 MD5 匹配来查找 KVI 文件
func findMatchingKVI(encPath string) (*types.VideoIndex, string, error) {
	encDir := filepath.Dir(encPath)

	// --- 策略1: 快速匹配 ---
	// 获取加密文件的基础名（去掉 .enc 后缀），并构造 KVI 文件名
	// 例如 "video.mp4.enc" -> "video.mp4.kvi"
	encBaseName := strings.TrimSuffix(filepath.Base(encPath), ".enc")
	kviFastMatchPath := filepath.Join(encDir, encBaseName+".kvi")

	fmt.Printf("-> Attempting fast match for KVI: %s.kvi\n", encBaseName)

	// 检查这个快速匹配的 KVI 文件是否存在
	if _, err := os.Stat(kviFastMatchPath); err == nil {
		// 存在，读取它
		kviData, err := os.ReadFile(kviFastMatchPath)
		if err == nil {
			var index types.VideoIndex
			if json.Unmarshal(kviData, &index) == nil {
				// 进一步验证：计算 enc 文件的 MD5 并与 KVI 中的 MD5 对比，确保万无一失
				encMD5, err := calculateMD5(encPath)
				if err == nil && index.EncryptedFileMD5 == encMD5 {
					fmt.Printf("-> Fast match successful and verified: %s\n", filepath.Base(kviFastMatchPath))
					return &index, kviFastMatchPath, nil
				}
			}
		}
		fmt.Printf("-> Fast match candidate %s.kvi is invalid or does not match MD5.\n", encBaseName)
	}

	// --- 策略2: 兜底的全盘扫描 + MD5 匹配 ---
	fmt.Println("-> Fast match failed, falling back to full directory scan and MD5 check...")
	files, err := os.ReadDir(encDir)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read directory %s: %w", encDir, err)
	}

	encMD5, err := calculateMD5(encPath) // 只计算一次 MD5
	if err != nil {
		return nil, "", fmt.Errorf("failed to calculate MD5 for encrypted file: %w", err)
	}
	fmt.Printf("-> MD5 of encrypted file: %s\n", encMD5)

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".kvi") {
			continue
		}

		kviPath := filepath.Join(encDir, f.Name())
		kviData, err := os.ReadFile(kviPath)
		if err != nil {
			fmt.Printf("-> Warning: Failed to read KVI file %s, skipping.\n", f.Name())
			continue
		}

		var index types.VideoIndex
		if err := json.Unmarshal(kviData, &index); err != nil {
			fmt.Printf("-> Warning: Failed to parse KVI file %s, skipping.\n", f.Name())
			continue
		}

		// 比较 MD5
		if index.EncryptedFileMD5 == encMD5 {
			fmt.Printf("-> Found matching KVI file via full scan: %s\n", f.Name())
			return &index, kviPath, nil
		}
	}

	return nil, "", fmt.Errorf("no KVI file found with a matching MD5 for %s", encPath)
}

// --- 1. 解密单个文件的辅助函数 ---
func decryptSingleFile(encPath, password, outputDir string) error {
	fmt.Printf("-> Processing file: %s\n", encPath)

	// --- 核心改动：通过混合策略寻找 KVI 文件 ---
	index, _, err := findMatchingKVI(encPath)
	if err != nil {
		return fmt.Errorf("failed to find a matching KVI file: %w", err)
	}

	// --- Step 2: 解密视频文件 ---
	salt, err := crypto.Base64Decode(index.Encryption.SaltBase64)
	if err != nil {
		return fmt.Errorf("failed to decode salt: %w", err)
	}
	key := crypto.GenerateKey(password, salt)

	originalFilename := filepath.Base(index.OriginalFilename)
	if originalFilename == "" || originalFilename == "unknown" {
		fmt.Println("-> [Warning] 'original_filename' in index is missing. Inferring a default name.")
		baseFilename := strings.TrimSuffix(filepath.Base(encPath), ".enc")
		originalFilename = baseFilename + ".mkv"
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

	// 【关键修改】调用新的 DecryptStream，它会自己从文件头读取 IV
	if err := crypto.DecryptStream(encFile, decFile, key); err != nil {
		return fmt.Errorf("failed to decrypt video stream: %w", err)
	}

	// --- Step 3: 从 KVI 恢复原始轨道文件 (核心修改) ---
	sourceDir := filepath.Dir(encPath)
	fmt.Println("-> Restoring original subtitle files...")
	for _, track := range index.Subtitles {
		// 【关键修改】使用 title 字段作为源文件名（重命名后的），filename 字段作为目标文件名（原始的）
		sourceTrackName := track.Title
		destTrackName := track.Filename

		if sourceTrackName == "" {
			fmt.Printf("Warning: 'title' is empty for subtitle '%s', skipping.\n", destTrackName)
			continue
		}

		sourceTrackPath := filepath.Join(sourceDir, sourceTrackName)
		destTrackPath := filepath.Join(outputDir, destTrackName)

		if _, err := os.Stat(sourceTrackPath); os.IsNotExist(err) {
			fmt.Printf("Warning: Track file not found at source: %s. Skipping.\n", sourceTrackPath)
			continue
		}

		if err := copyFile(sourceTrackPath, destTrackPath); err != nil {
			fmt.Printf("Warning: Failed to copy track %s: %v\n", destTrackName, err)
		} else {
			fmt.Printf("-> Restored subtitle '%s'\n", destTrackName)
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

	// 【关键修改】不再预先检查 KVI 文件是否存在，直接尝试解密
	// 因为 decryptSingleFile 内部已经包含了完整的查找逻辑
	for _, entry := range encFiles {
		encPath := filepath.Join(inputPath, entry.Name())
		fmt.Printf("\n--- Attempting to decrypt %s ---\n", entry.Name())
		if err := decryptSingleFile(encPath, opts.Password, opts.OutputDir); err != nil {
			fmt.Printf("-> [Error] Failed to decrypt %s: %v\n", entry.Name(), err)
		} else {
			fmt.Printf("-> Successfully decrypted %s\n", entry.Name())
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
