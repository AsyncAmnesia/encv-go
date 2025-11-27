package container

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/types"
)

// PackWithIndex 将加密文件和任意类型的索引打包成容器
func PackWithIndex(dataPath, finalPath string, index types.Index) error {
	// 1. 打开加密数据文件
	dataFile, err := os.Open(dataPath)
	if err != nil {
		return fmt.Errorf("failed to open data file: %w", err)
	}
	defer dataFile.Close()

	// 2. 创建最终容器文件
	outFile, err := os.Create(finalPath)
	if err != nil {
		return fmt.Errorf("failed to create container file: %w", err)
	}
	defer outFile.Close()

	// 【关键修复】根据 Index 类型选择正确的魔法数字
	var ext string
	switch i := index.(type) {
	case *types.VideoIndex:
		ext = config.GlobalConfig.BinExtGroup.Video
	case *types.ImageIndex:
		ext = config.GlobalConfig.BinExtGroup.Image
	case *types.TextIndex:
		ext = config.GlobalConfig.BinExtGroup.Text
	default:
		// 如果还有其他类型，可以在这里添加
		return fmt.Errorf("unsupported index type for packing: %T", i)
	}

	magicMap := GetContainerMagicMap()
	magic, ok := magicMap[ext]
	if !ok {
		return fmt.Errorf("no magic number found for extension: %s", ext)
	}

	// 3. 写入魔法数字
	if _, err := outFile.Write([]byte(magic)); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}

	// 4. 序列化并写入 KVI
	kviData, err := json.Marshal(index)
	if err != nil {
		return fmt.Errorf("failed to marshal KVI: %w", err)
	}
	kviLen := uint64(len(kviData))
	if err := binary.Write(outFile, binary.LittleEndian, &kviLen); err != nil {
		return fmt.Errorf("failed to write KVI length: %w", err)
	}
	if _, err := outFile.Write(kviData); err != nil {
		return fmt.Errorf("failed to write KVI data: %w", err)
	}

	// 5. 复制加密数据流
	if _, err := io.Copy(outFile, dataFile); err != nil {
		return fmt.Errorf("failed to write encrypted data: %w", err)
	}

	fmt.Printf("-> Successfully packed container: %s\n", finalPath)
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
