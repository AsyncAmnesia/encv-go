package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// Algorithm 加密算法
	Algorithm = "aes-256-ctr"
	// KeySize 密钥长度
	KeySize = 32
	// IVSize 初始化向量长度
	IVSize = 16
	// SaltSize 盐值长度
	SaltSize = 32
	// Iterations PBKDF2 迭代次数
	Iterations = 100000
)

// GenerateKey 使用 PBKDF2 从密码和盐值生成密钥
func GenerateKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, Iterations, KeySize, sha256.New)
}

// EncryptStream 加密一个 io.Reader 并写入 io.Writer
func EncryptStream(src io.Reader, dst io.Writer, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %w", err)
	}

	iv := make([]byte, IVSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	multiWriter := &cipher.StreamWriter{S: stream, W: dst}

	if _, err := io.Copy(multiWriter, src); err != nil {
		return nil, fmt.Errorf("failed to encrypt stream: %w", err)
	}

	return iv, nil
}

// DecryptStream 解密一个 io.Reader 并写入 io.Writer
func DecryptStream(src io.Reader, dst io.Writer, key []byte, iv []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher block: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	streamReader := &cipher.StreamReader{S: stream, R: src}

	if _, err := io.Copy(dst, streamReader); err != nil {
		return fmt.Errorf("failed to decrypt stream: %w", err)
	}

	return nil
}

// Base64Encode 编码为 Base64 字符串
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode 从 Base64 字符串解码
func Base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
