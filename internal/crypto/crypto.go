package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"errors"
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

	// MagicNumber 是用于标识 encv 加密文件的魔法数字
	MagicNumber = "encv-magic-v1"
	// MagicNumberLength 是魔法数字的字节长度
	MagicNumberLength = len(MagicNumber)
	// IVLength 是 AES CTR 模式 IV 的标准长度
	IVLength = aes.BlockSize
)

// GenerateKey 使用 PBKDF2 从密码和盐值生成密钥
func GenerateKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, Iterations, KeySize, sha256.New)
}

// EncryptStream 加密数据流，并将魔法标识和IV写入文件头
// iv 由调用者生成，以便同时写入 KVI 文件
func EncryptStream(r io.Reader, w io.Writer, key []byte, iv []byte) error {
	if len(iv) != IVLength {
		return errors.New("invalid IV length")
	}

	// 1. 写入魔法标识
	if _, err := w.Write([]byte(MagicNumber)); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}

	// 2. 写入 IV
	if _, err := w.Write(iv); err != nil {
		return fmt.Errorf("failed to write IV: %w", err)
	}

	// 3. 创建加密器并进行加密
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher block: %w", err)
	}

	stream := cipher.NewCTR(block, iv)

	// 使用 io.Copy 从输入流读取，加密后写入输出流
	if _, err := io.CopyBuffer(streamWriter{stream: stream, writer: w}, r, nil); err != nil {
		return fmt.Errorf("failed to encrypt stream: %w", err)
	}

	return nil
}

// DecryptStream 从加密流中读取魔法标识和IV，验证后解密数据
func DecryptStream(r io.Reader, w io.Writer, key []byte) error {
	// 1. 读取并验证魔法标识
	magic := make([]byte, MagicNumberLength)
	if _, err := io.ReadFull(r, magic); err != nil {
		return fmt.Errorf("failed to read magic number: %w", err)
	}
	if string(magic) != MagicNumber {
		return fmt.Errorf("invalid file format: not an encv encrypted file")
	}

	// 2. 读取 IV
	iv := make([]byte, IVLength)
	if _, err := io.ReadFull(r, iv); err != nil {
		return fmt.Errorf("failed to read IV: %w", err)
	}

	// 3. 创建解密器并解密
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher block: %w", err)
	}

	stream := cipher.NewCTR(block, iv)

	// 使用 io.Copy 从输入流读取，解密后写入输出流
	if _, err := io.CopyBuffer(w, streamReader{stream: stream, reader: r}, nil); err != nil {
		return fmt.Errorf("failed to decrypt stream: %w", err)
	}

	return nil
}

// --- 辅助类型，用于实现 io.Writer/io.Reader 接口 ---

type streamWriter struct {
	stream cipher.Stream
	writer io.Writer
}

func (sw streamWriter) Write(p []byte) (n int, err error) {
	sw.stream.XORKeyStream(p, p)
	return sw.writer.Write(p)
}

type streamReader struct {
	stream cipher.Stream
	reader io.Reader
}

func (sr streamReader) Read(p []byte) (n int, err error) {
	n, err = sr.reader.Read(p)
	if err != nil {
		return
	}
	sr.stream.XORKeyStream(p, p[:n])
	return
}

// Base64Encode 编码为 Base64 字符串
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode 从 Base64 字符串解码
func Base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
