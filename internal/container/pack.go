package container

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Pack 是一个通用的打包函数，将 KVI 数据和加密数据流打包到容器格式中。
// 它不关心文件路径或具体类型，只负责按格式写入数据。
func Pack(writer io.Writer, magic []byte, kviData []byte, dataReader io.Reader) error {
	// 1. 写入魔法数字
	if _, err := writer.Write(magic); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}

	// 2. 写入 KVI 长度
	kviLen := uint64(len(kviData))
	if err := binary.Write(writer, binary.LittleEndian, &kviLen); err != nil {
		return fmt.Errorf("failed to write KVI length: %w", err)
	}

	// 3. 写入 KVI 数据
	if _, err := writer.Write(kviData); err != nil {
		return fmt.Errorf("failed to write KVI data: %w", err)
	}

	// 4. 复制加密数据流
	if _, err := io.Copy(writer, dataReader); err != nil {
		return fmt.Errorf("failed to write encrypted data: %w", err)
	}

	return nil
}

// Unpack 从 ENCV 容器文件中解包出 KVI 和加密视频流
// 【关键修复】增加 expectedMagic 参数，使其能处理不同类型的容器
func Unpack(file io.Reader, expectedMagic string) (*PackedData, error) {
	// 读取魔法数字
	magicBytes := make([]byte, len(expectedMagic))
	if _, err := io.ReadFull(file, magicBytes); err != nil {
		return nil, fmt.Errorf("failed to read container magic number: %w", err)
	}

	// 与传入的期望魔法数字进行比较
	if string(magicBytes) != expectedMagic {
		return nil, fmt.Errorf("invalid container file: magic number mismatch (expected '%s', got '%s')", expectedMagic, string(magicBytes))
	}

	// 读取 KVI 长度
	var kviLen uint64
	if err := binary.Read(file, binary.LittleEndian, &kviLen); err != nil {
		return nil, fmt.Errorf("failed to read KVI length: %w", err)
	}

	// 读取 KVI 数据
	kviData := make([]byte, kviLen)
	if _, err := io.ReadFull(file, kviData); err != nil {
		return nil, fmt.Errorf("failed to read KVI data: %w", err)
	}

	// 剩余的数据流就是加密内容
	dataStream := io.NopCloser(file)

	return &PackedData{
		KVIData:    kviData,
		DataStream: dataStream,
	}, nil
}
