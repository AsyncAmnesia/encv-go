package crypto

import (
	"fmt"
	"io"
	"os"

	"github.com/Soltus/encv-go/internal/v2/types"
)

// EncryptToTempFile 将一个数据流加密，并保存到一个临时文件中。
// 它返回临时文件的路径、生成的盐和初始化向量（IV）。
// 调用者负责在使用完毕后删除这个临时文件。
func EncryptToTempFile(src io.Reader, password string, outputDir string) (tempPath string, salt []byte, iv []byte, err error) {
	// 1. 生成加密参数
	salt, err = GenerateSalt_v2(types.SaltSize_v2)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	iv, err = GenerateIV_v2(types.IVSize_v2)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to generate iv: %w", err)
	}
	key := GenerateKey_v2(password, salt, types.KeySize_v2)

	// 2. 创建临时文件
	tempFile, err := os.CreateTemp(outputDir, "*.enc.tmp")
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath = tempFile.Name()

	// 3. 执行加密
	err = EncryptStream_v2(src, tempFile, key, iv)
	if err != nil {
		tempFile.Close()
		os.Remove(tempPath) // 失败时清理
		return "", nil, nil, fmt.Errorf("failed to encrypt stream: %w", err)
	}

	// 4. 关闭文件并返回结果
	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath) // 失败时清理
		return "", nil, nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	return tempPath, salt, iv, nil
}
