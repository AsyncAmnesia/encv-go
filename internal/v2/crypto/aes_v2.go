// internal/v2/crypto/aes_v2.go
package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

var (
	ErrInvalidKeyLength_v2 = errors.New("invalid key length")
	ErrInvalidIVLength_v2  = errors.New("invalid IV length")
)

const (
	// Algorithm 加密算法
	Algorithm_v2 = "aes-256-ctr"
	// KeySize 密钥长度
	KeySize_v2 = 32
	// IVSize_v2 是 AES CTR 模式 IV 的标准长度
	IVSize_v2 = aes.BlockSize
	// SaltSize 盐值长度
	SaltSize_v2 = 32
	// Iterations PBKDF2 迭代次数
	Iterations_v2 = 100000
)

// GenerateKey_v2 使用 PBKDF2 从密码和盐派生密钥
func GenerateKey_v2(password string, salt []byte, keyLen int) []byte {
	if keyLen <= 0 {
		keyLen = KeySize_v2 // 默认 AES-256
	}
	return pbkdf2.Key([]byte(password), salt, 100000, keyLen, sha256.New)
}

// GenerateSalt_v2 生成一个随机盐
func GenerateSalt_v2(size int) ([]byte, error) {
	salt := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// EncryptStream_v2 使用 AES-CTR 加密一个 io.Reader
func EncryptStream_v2(src io.Reader, dst io.Writer, key, iv []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher block: %w", err)
	}
	if len(key) != block.BlockSize() && len(key) != 2*block.BlockSize() && len(key) != 4*block.BlockSize() {
		return ErrInvalidKeyLength_v2
	}
	if len(iv) != block.BlockSize() {
		return ErrInvalidIVLength_v2
	}

	stream := cipher.NewCTR(block, iv)
	writer := &cipher.StreamWriter{S: stream, W: dst}

	_, err = io.Copy(writer, src)
	return err
}

// DecryptStream_v2 使用 AES-CTR 解密一个 io.Reader
func DecryptStream_v2(src io.Reader, dst io.Writer, key, iv []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher block: %w", err)
	}
	if len(key) != block.BlockSize() && len(key) != 2*block.BlockSize() && len(key) != 4*block.BlockSize() {
		return ErrInvalidKeyLength_v2
	}
	if len(iv) != block.BlockSize() {
		return ErrInvalidIVLength_v2
	}

	stream := cipher.NewCTR(block, iv)
	reader := &cipher.StreamReader{S: stream, R: src}

	_, err = io.Copy(dst, reader)
	return err
}

// GenerateIV_v2 生成一个随机 IV
func GenerateIV_v2(size int) ([]byte, error) {
	iv := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	return iv, nil
}

// Base64Encode_v2 编码为 Base64 字符串
func Base64Encode_v2(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode_v2 从 Base64 字符串解码
func Base64Decode_v2(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// DecryptBytes_v2 解密一个完整的字节切片，使用 CTR 模式以匹配加密端
func DecryptBytes_v2(ciphertext, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %w", err)
	}

	// 【关键修正】使用 CTR 模式进行解密
	// CTR 模式不需要填充，也不需要检查密文长度是否为块大小的倍数
	stream := cipher.NewCTR(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	return plaintext, nil
}

// EncryptBytes_v2 使用 CTR 模式加密一个完整的字节切片
func EncryptBytes_v2(plaintext, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %w", err)
	}

	// CTR 模式的加密和解密操作是完全相同的
	stream := cipher.NewCTR(block, iv)
	ciphertext := make([]byte, len(plaintext))
	stream.XORKeyStream(ciphertext, plaintext)

	return ciphertext, nil
}

// DecryptReader_v2 包装一个加密的 io.Reader，返回一个解密后的 io.Reader
// 它使用 CTR 模式，非常适合流式处理
func DecryptReader_v2(src io.Reader, key, iv []byte) (io.Reader, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// 创建 CTR 流
	stream := cipher.NewCTR(block, iv)

	// 返回一个 StreamReader，它会在读取时自动解密
	return &cipher.StreamReader{S: stream, R: src}, nil
}

// EncryptReaderToBytes_v2 读取 io.Reader 的全部内容，使用 AES-CTR 加密，并返回加密后的字节切片。
// 这是一个便利函数，用于将流式加密的结果一次性读入内存。
func EncryptReaderToBytes_v2(src io.Reader, key, iv []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := EncryptStream_v2(src, &buf, key, iv); err != nil {
		return nil, fmt.Errorf("failed to encrypt stream to bytes: %w", err)
	}
	return buf.Bytes(), nil
}

// DecryptReaderToBytes_v2 读取 io.Reader 的全部内容，使用 AES-CTR 解密，并返回解密后的字节切片。
func DecryptReaderToBytes_v2(src io.Reader, key, iv []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := DecryptStream_v2(src, &buf, key, iv); err != nil {
		return nil, fmt.Errorf("[DecryptReaderToBytes_v2] failed to decrypt stream to bytes: %w", err)
	}
	return buf.Bytes(), nil
}

// EncryptWriter_v2 包装一个 io.Writer，返回一个加密后的 io.Writer
// 它使用 CTR 模式，非常适合流式处理
func EncryptWriter_v2(dst io.Writer, key, iv []byte) (io.Writer, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// 创建 CTR 流
	stream := cipher.NewCTR(block, iv)

	// 返回一个 StreamWriter，它会在写入时自动加密
	return &cipher.StreamWriter{S: stream, W: dst}, nil
}
