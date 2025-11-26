package container

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Soltus/encv-go/internal/crypto"
)

// Pack 将加密视频流和 KVI 打包成一个 ENCV 容器文件
func Pack(videoPath, kviPath, outputPath string) error {
	// ... (文件打开和读取逻辑保持不变) ...
	videoFile, err := os.Open(videoPath)
	if err != nil {
		return err
	}
	defer videoFile.Close()

	kviData, err := os.ReadFile(kviPath)
	if err != nil {
		return err
	}

	// ... (创建输出文件逻辑保持不变) ...
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// 【修改】使用 crypto 包中定义的魔法数字
	// 写入魔法数字
	if _, err := outFile.Write([]byte(crypto.ContainerMagicNumber)); err != nil {
		return fmt.Errorf("failed to write container magic number: %w", err)
	}

	// 写入 KVI 长度
	kviLen := uint64(len(kviData))
	if err := binary.Write(outFile, binary.LittleEndian, kviLen); err != nil {
		return fmt.Errorf("failed to write KVI length: %w", err)
	}

	// 写入 KVI
	if _, err := outFile.Write(kviData); err != nil {
		return fmt.Errorf("failed to write KVI data: %w", err)
	}

	// 写入加密视频流
	if _, err := io.Copy(outFile, videoFile); err != nil {
		return fmt.Errorf("failed to write video stream: %w", err)
	}

	return nil
}

// PackedData 包含解包后的数据
type PackedData struct {
	KVIData     []byte
	VideoStream io.ReadCloser // 视频流使用 io.ReadCloser，避免将大文件读入内存
}

// Unpack 从 ENCV 容器文件中解包出 KVI 和加密视频流
func Unpack(file io.Reader) (*PackedData, error) {
	// 【修改】使用 crypto 包中定义的魔法数字
	// 读取并验证魔法数字
	magic := make([]byte, len(crypto.ContainerMagicNumber))
	if _, err := io.ReadFull(file, magic); err != nil {
		return nil, fmt.Errorf("failed to read container magic number: %w", err)
	}
	if string(magic) != crypto.ContainerMagicNumber {
		return nil, errors.New("invalid container file: magic number mismatch")
	}

	// ... (后续逻辑保持不变) ...
	var kviLen uint64
	if err := binary.Read(file, binary.LittleEndian, &kviLen); err != nil {
		return nil, fmt.Errorf("failed to read KVI length: %w", err)
	}

	kviData := make([]byte, kviLen)
	if _, err := io.ReadFull(file, kviData); err != nil {
		return nil, fmt.Errorf("failed to read KVI data: %w", err)
	}

	videoStream := io.NopCloser(file)

	return &PackedData{
		KVIData:     kviData,
		VideoStream: videoStream,
	}, nil
}
