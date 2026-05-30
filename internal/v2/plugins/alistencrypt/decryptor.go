package alistencrypt

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/alistencrypt"
)

func DecryptFile(containerPath, outputDir, password, encType string) error {
	ext := strings.ToLower(filepath.Ext(containerPath))
	if ext != "" && ext != ".bin" && ext != ".alist" && ext != ".enc" {
		return &alistencrypt.DecryptionError{Reason: "invalid format", Err: alistencrypt.ErrInvalidFormat}
	}

	f, err := os.Open(containerPath)
	if err != nil {
		return fmt.Errorf("failed to open container file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat container file: %w", err)
	}
	fileSize := info.Size()

	dr, err := alistencrypt.NewDecryptReader(f, password, fileSize)
	if err != nil {
		if strings.Contains(err.Error(), "failed to parse V2 header") || strings.Contains(err.Error(), "failed to create cipher") {
			return &alistencrypt.DecryptionError{Reason: "password mismatch", Err: alistencrypt.ErrInvalidPassword}
		}
		return fmt.Errorf("failed to create decrypt reader: %w", err)
	}

	decryptedData, err := io.ReadAll(dr)
	if err != nil {
		return fmt.Errorf("failed to read decrypted data: %w", err)
	}

	baseName := filepath.Base(containerPath)
	originalName := tryDecodeFilename(baseName, password, encType)
	if originalName == "" {
		originalName = strings.TrimSuffix(baseName, ext)
	}

	outputPath := filepath.Join(outputDir, originalName)

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	if _, err := outFile.Write(decryptedData); err != nil {
		return fmt.Errorf("failed to write decrypted data: %w", err)
	}

	return nil
}

func tryDecodeFilename(encodedName, password, encType string) string {
	if len(encodedName) < 2 {
		return ""
	}

	fileName := filepath.Base(encodedName)
	ext := filepath.Ext(fileName)
	encPart := strings.TrimSuffix(fileName, ext)

	decoded := alistencrypt.DecodeName(encPart, password, encType)
	if decoded == "" {
		return ""
	}

	return decoded + ext
}
