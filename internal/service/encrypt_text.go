package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/processor"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// encryptText 处理文本加密和打包
func encryptText(ctx context.Context, inputPath, baseName, originalExt, outputDir string, salt []byte) error {
	cfg := config.FromContext(ctx)
	// 1. 调用 processor 获取文本信息
	info, err := processor.ProcessText(inputPath)
	if err != nil {
		return fmt.Errorf("processor failed: %w", err)
	}

	// 2. 加密
	tempEncPath := filepath.Join(outputDir, baseName+".tmp_enc")
	iv, err := crypto.EncryptFile(inputPath, tempEncPath, cfg.Password, salt)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}
	defer os.Remove(tempEncPath)

	// 3. 构建 Index
	index := &types.TextIndex{
		Kind:             types.IndexKindText,
		Version:          types.KviVersion,
		OriginalFilename: filepath.Base(inputPath),
		Encryption: types.EncryptionInfo{
			Algorithm:  crypto.Algorithm,
			IVBase64:   crypto.Base64Encode(iv),
			SaltBase64: crypto.Base64Encode(salt),
		},
		MimeType:         info.MimeType,
		Format:           info.Format,
		OriginalFileSize: info.OriginalFileSize,
	}

	// 4. 为图像容器生成带倒序后缀的最终路径
	reversedExt := utils.GenerateReversedExt(originalExt)
	finalPath := filepath.Join(outputDir, baseName+"."+reversedExt+cfg.GetTextEncExtension())
	return container.PackWithIndex(ctx, tempEncPath, finalPath, index)
}
