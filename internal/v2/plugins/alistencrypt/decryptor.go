package alistencrypt

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

)

func DecryptFile(containerPath, outputDir, password, encType string) (string, error) {
	ext := strings.ToLower(filepath.Ext(containerPath))
	if ext != "" && ext != ".bin" && ext != ".alist" && ext != ".enc" {
		return "", &DecryptionError{Reason: "invalid format", Err: ErrInvalidFormat}
	}

	f, err := os.Open(containerPath)
	if err != nil {
		return "", fmt.Errorf("failed to open container file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat container file: %w", err)
	}
	fileSize := info.Size()

	dr, err := NewDecryptReader(f, password, fileSize)
	if err != nil {
		if strings.Contains(err.Error(), "failed to parse V2 header") || strings.Contains(err.Error(), "failed to create cipher") {
			return "", &DecryptionError{Reason: "password mismatch", Err: ErrInvalidPassword}
		}
		return "", fmt.Errorf("failed to create decrypt reader: %w", err)
	}

	decryptedData, err := io.ReadAll(dr)
	if err != nil {
		return "", fmt.Errorf("failed to read decrypted data: %w", err)
	}

	baseName := filepath.Base(containerPath)
	originalName := tryDecodeFilename(baseName, password, encType)
	if originalName == "" {
		originalName = strings.TrimSuffix(baseName, ext)
	}

	outputPath := filepath.Join(outputDir, originalName)

	outFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	if _, err := outFile.Write(decryptedData); err != nil {
		return "", fmt.Errorf("failed to write decrypted data: %w", err)
	}

	return outputPath, nil
}

func tryDecodeFilename(encodedName, password, encType string) string {
	if len(encodedName) < 2 {
		return ""
	}

	fileName := filepath.Base(encodedName)
	ext := filepath.Ext(fileName)
	encPart := strings.TrimSuffix(fileName, ext)

	decoded := DecodeName(encPart, password, encType)
	if decoded == "" {
		return ""
	}

	// ★ 关键修复: DecodeName 已经返回带扩展名的明文文件名（如 "CAD放样.mp4"），
	// 不再追加原 .bin 后缀（之前 bug 会变成 "CAD放样.mp4.bin"）
	return decoded
}
