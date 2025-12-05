package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// Algorithm 加密算法
	Algorithm = "aes-256-ctr"
	// KeySize 密钥长度
	KeySize = 32
	// IVLength 是 AES CTR 模式 IV 的标准长度
	IVLength = aes.BlockSize
	// SaltSize 盐值长度
	SaltSize = 32
	// Iterations PBKDF2 迭代次数
	Iterations = 100000

	// MagicNumber 是流加密层，它告诉程序：“我包裹着的数据是加密的，请用正确的密钥和 IV 来解密我。”
	// 其他则是容器封装层，它告诉程序：“我是一个 encv 容器，请用解析 encv 的方法来处理我。”
	MagicNumber = "encv-MagicNumber-v1"
	// encv 容器
	ContainerMagicNumber = "encv-ContainerMagicNumber-v1"
)

// GenerateKey 使用 PBKDF2 从密码和盐值生成密钥
func GenerateKey(password, salt []byte, keyLen int) ([]byte, error) {
	if len(password) == 0 {
		return nil, fmt.Errorf("password cannot be empty")
	}
	if len(salt) == 0 {
		return nil, fmt.Errorf("salt cannot be empty")
	}
	if keyLen <= 0 {
		return nil, fmt.Errorf("key length must be positive")
	}
	return pbkdf2.Key(password, salt, Iterations, keyLen, sha256.New), nil
}

// EncryptFile 加密文件，并将魔法标识和IV写入文件头。
// 它内部调用 EncryptStream 来完成实际工作。
func EncryptFile(inputPath, outputPath string, password, salt []byte) (iv []byte, err error) {
	// 1. 打开输入文件
	inFile, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open input file '%s': %w", inputPath, err)
	}
	defer inFile.Close()

	// 2. 创建输出文件
	outFile, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file '%s': %w", outputPath, err)
	}
	// 确保在出错时关闭并删除文件
	defer func() {
		if err != nil {
			outFile.Close()
			os.Remove(outputPath)
		}
	}()

	// 3. 调用核心的流式加密函数
	iv, err = EncryptStream(inFile, outFile, password, salt)
	if err != nil {
		return nil, err
	}

	// 4. 成功后，关闭输出文件
	if err := outFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close output file: %w", err)
	}

	return iv, nil
}

// EncryptStream 加密数据流，并将魔法标识和IV写入文件头。
// 它内部使用 Argon2 从 key 和 salt 派生出加密密钥和 IV。
// 返回的 IV 应由调用者存入 KVI。
func EncryptStream(r io.Reader, w io.Writer, password, salt []byte) (iv []byte, err error) {
	// 1. 派生密钥和 IV
	// 我们派生一个足够长的密钥，然后将其切分为加密密钥和 IV。
	// 总长度 = AES密钥长度 + IV长度
	totalKeyLen := KeySize + IVLength
	derivedKey, err := GenerateKey(password, salt, totalKeyLen) // 【关键修复】传入总长度
	if err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	encKey := derivedKey[:KeySize]
	iv = derivedKey[KeySize:] // 现在 iv 可以正确获取到 16 字节

	// 2. 写入魔法标识
	if _, err := w.Write([]byte(MagicNumber)); err != nil {
		return nil, fmt.Errorf("failed to write magic number: %w", err)
	}

	// 3. 写入 IV
	if _, err := w.Write(iv); err != nil {
		return nil, fmt.Errorf("failed to write IV: %w", err)
	}

	// 4. 创建加密器并进行加密
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %w", err)
	}

	stream := cipher.NewCTR(block, iv)

	// 使用 io.Copy 从输入流读取，加密后写入输出流
	if _, err := io.CopyBuffer(streamWriter{stream: stream, writer: w}, r, nil); err != nil {
		return nil, fmt.Errorf("failed to encrypt stream: %w", err)
	}

	return iv, nil
}

// 【新增】GetDecryptReader 创建一个 io.Reader，它会透明地解密来自底层加密流的数据。
// 它会处理从流中读取和验证魔法数字和 IV 的逻辑。
func GetDecryptReader(r io.Reader, key []byte) (io.Reader, error) {
	// 1. 读取并验证魔法标识
	magic := make([]byte, len(MagicNumber))
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, fmt.Errorf("failed to read magic number: %w", err)
	}
	if string(magic) != MagicNumber {
		return nil, fmt.Errorf("invalid file format: not an encv encrypted container")
	}

	// 2. 读取 IV
	iv := make([]byte, IVLength)
	if _, err := io.ReadFull(r, iv); err != nil {
		return nil, fmt.Errorf("failed to read IV: %w", err)
	}

	// 3. 创建解密器
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %w", err)
	}

	stream := cipher.NewCTR(block, iv)

	// 4. 返回一个封装了解密逻辑的 reader
	return &streamReader{stream: stream, reader: r}, nil
}

// DecryptStream 从加密流中读取魔法标识和IV，验证后解密数据
// 【重构】现在它是对 GetDecryptReader 和 io.Copy 的一个便利封装，并返回写入的字节数。
func DecryptStream(r io.Reader, w io.Writer, key []byte) (int64, error) {
	// 1. 使用 GetDecryptReader 来处理所有设置逻辑（读取魔法数字和IV）
	decryptReader, err := GetDecryptReader(r, key)
	if err != nil {
		// GetDecryptReader 已经返回了包装好的错误信息
		return 0, err
	}

	// 2. 将解密后的数据从 decryptReader 复制到 w
	// io.Copy 会返回实际写入的字节数
	written, err := io.Copy(w, decryptReader)
	if err != nil {
		// 如果复制失败，返回错误和已写入的字节数
		return written, fmt.Errorf("failed to copy decrypted data: %w", err)
	}

	// 3. 返回成功写入的总字节数
	return written, nil
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

// 【新增】Close 方法使 streamWriter 实现 io.WriteCloser
func (sw streamWriter) Close() error {
	// 对于 CTR 模式，不需要特殊的清理操作
	return nil
}

// GenerateIV 生成一个随机的初始化向量 (IV)
func GenerateIV() ([]byte, error) {
	iv := make([]byte, IVLength)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}
	return iv, nil
}

// Base64Encode 编码为 Base64 字符串
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode 从 Base64 字符串解码
func Base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
