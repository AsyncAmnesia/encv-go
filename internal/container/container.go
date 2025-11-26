package container

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	// ContainerMagicNumber 是 .encv 文件的魔法数字
	ContainerMagicNumber = "encv-container-v1"
)

// Pack 将加密视频文件和 KVI 文件打包成一个 .encv 容器文件
func Pack(videoFilePath, kviFilePath, outputContainerPath string) error {
	// 1. 打开输入文件
	videoFile, err := os.Open(videoFilePath)
	if err != nil {
		return fmt.Errorf("failed to open video file %s: %w", videoFilePath, err)
	}
	defer videoFile.Close()

	kviFile, err := os.Open(kviFilePath)
	if err != nil {
		return fmt.Errorf("failed to open KVI file %s: %w", kviFilePath, err)
	}
	defer kviFile.Close()

	// 2. 读取 KVI 数据到内存
	kviData, err := io.ReadAll(kviFile)
	if err != nil {
		return fmt.Errorf("failed to read KVI data: %w", err)
	}

	// 3. 创建输出容器文件
	outFile, err := os.Create(outputContainerPath)
	if err != nil {
		return fmt.Errorf("failed to create container file %s: %w", outputContainerPath, err)
	}
	defer outFile.Close()

	// 4. 写入文件头
	// 4.1 写入魔法数字
	if _, err := outFile.Write([]byte(ContainerMagicNumber)); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}

	// 4.2 写入 KVI 数据长度
	kviLen := uint64(len(kviData))
	if err := binary.Write(outFile, binary.LittleEndian, kviLen); err != nil {
		return fmt.Errorf("failed to write KVI length: %w", err)
	}

	// 4.3 写入 KVI 数据
	if _, err := outFile.Write(kviData); err != nil {
		return fmt.Errorf("failed to write KVI data: %w", err)
	}

	// 5. 复制加密视频数据
	if _, err := io.Copy(outFile, videoFile); err != nil {
		return fmt.Errorf("failed to copy video data: %w", err)
	}

	fmt.Printf("-> Successfully packed %s and %s into %s\n", videoFilePath, kviFilePath, outputContainerPath)
	return nil
}

// PackedData 包含解包后的数据
type PackedData struct {
	KVIData     []byte
	VideoStream io.ReadCloser // 视频流使用 io.ReadCloser，避免将大文件读入内存
}

// Unpack 从一个 io.Reader 中解包出 KVI 数据和视频流
func Unpack(reader io.Reader) (*PackedData, error) {
	// 1. 验证魔法数字
	magicBuf := make([]byte, len(ContainerMagicNumber))
	if _, err := io.ReadFull(reader, magicBuf); err != nil {
		return nil, fmt.Errorf("failed to read magic number: %w", err)
	}
	if string(magicBuf) != ContainerMagicNumber {
		return nil, errors.New("invalid container file: magic number mismatch")
	}

	// 2. 读取 KVI 数据长度
	var kviLen uint64
	if err := binary.Read(reader, binary.LittleEndian, &kviLen); err != nil {
		return nil, fmt.Errorf("failed to read KVI length: %w", err)
	}

	// 3. 读取 KVI 数据
	kviData := make([]byte, kviLen)
	if _, err := io.ReadFull(reader, kviData); err != nil {
		return nil, fmt.Errorf("failed to read KVI data: %w", err)
	}

	// 4. 视频流就是 reader 中剩余的所有数据
	// 使用 io.NopCloser 包装 reader，使其成为一个 ReadCloser
	videoStream := io.NopCloser(reader)

	return &PackedData{
		KVIData:     kviData,
		VideoStream: videoStream,
	}, nil
}
