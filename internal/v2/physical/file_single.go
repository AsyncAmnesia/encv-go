package physical

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/v2/types"
	"github.com/Soltus/encv-go/internal/v2/writer"
)

// SinglePhysicalPacker 将数据打包成一个单一、完整的容器文件
type SinglePhysicalPacker struct{}

func NewSinglePhysicalPacker() *SinglePhysicalPacker {
	return &SinglePhysicalPacker{}
}

// Pack 实现 PhysicalPacker 接口
// SinglePhysicalPacker namer 和 startIdx 参数暂时无用，保留接口兼容性
func (p *SinglePhysicalPacker) Pack(data io.Reader, manifest *types.Manifest_v2, req *PackRequest) (string, error) {
	outputPath := filepath.Join(req.OutputDir, req.FinalFileName)
	tempPath := outputPath + ".tmp"

	// 1. 创建全能的 writer
	tempWriter, err := writer.NewSingleFileContainerWriter(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to create container writer: %w", err)
	}
	defer func() { // 统一的错误清理
		tempWriter.Close()
		os.Remove(tempPath)
	}()

	// 2. 遍历并写入 fragments
	for i, frag := range manifest.Fragments {
		if frag.Type != types.FragmentType_AtomicFile {
			continue
		}
		chunkData := make([]byte, frag.Length)
		if _, err := io.ReadFull(data, chunkData); err != nil {
			return tempPath, fmt.Errorf("failed to read data for fragment %d: %w", i, err)
		}
		if err := tempWriter.WriteFragment(&frag, chunkData); err != nil {
			return tempPath, fmt.Errorf("failed to write fragment %d: %w", i, err)
		}
	}

	// 3. 写入 Manifest
	if err := tempWriter.WriteManifest(manifest); err != nil {
		return tempPath, fmt.Errorf("failed to write manifest: %w", err)
	}

	// 4. 关闭 writer (它会自动写入 Footer)
	if err := tempWriter.Close(); err != nil {
		return tempPath, fmt.Errorf("failed to close writer (write footer): %w", err)
	}

	// 5. 原子重命名
	if err := os.Rename(tempPath, outputPath); err != nil {
		return tempPath, fmt.Errorf("failed to atomically rename temp file to final file: %w", err)
	}

	fmt.Printf("✅ [SinglePhysicalPacker] Packed to: %s\n", outputPath)
	return outputPath, nil
}

// 单一文件物理解包器
type SinglePhysicalUnpacker struct{}

func NewSinglePhysicalUnpacker() *SinglePhysicalUnpacker {
	return &SinglePhysicalUnpacker{}
}

func (u *SinglePhysicalUnpacker) Unpack(ctx context.Context, mainContainerPath string) (string, func(), error) {
	// 直接返回原始路径，无需清理，忽略 ctx
	return mainContainerPath, func() {}, nil
}
