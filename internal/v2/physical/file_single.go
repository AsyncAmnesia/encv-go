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
	// 【防御性编程】FinalFileName 对于单文件打包是必需的
	if req.FinalFileName == "" {
		return "", fmt.Errorf("SinglePhysicalPacker requires a non-empty FinalFileName in the PackRequest")
	}

	outputPath := filepath.Join(req.OutputDir, req.FinalFileName)
	tempPath := outputPath + ".tmp"
	// 1. 创建全能的 writer
	tempWriter, err := writer.NewSingleFileContainerWriter(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to create container writer: %w", err)
	}

	finalPath, err := p.writeAndClose(data, manifest, *tempWriter, tempPath, outputPath)
	if err != nil {
		// 如果 writeAndClose 失败，它会自己清理临时文件
		return "", err
	}

	fmt.Printf("✅ [SinglePhysicalPacker] Packed to: %s\n", finalPath)
	return finalPath, nil
}

func (p *SinglePhysicalPacker) writeAndClose(data io.Reader, manifest *types.Manifest_v2, tempWriter writer.SingleFileContainerWriter, tempPath, outputPath string) (string, error) {
	// 确保临时文件在函数退出时被清理，除非操作成功
	var success bool
	defer func() {
		if !success {
			tempWriter.Close()
			os.Remove(tempPath)
		}
	}()

	// 2. 遍历并写入 fragments
	for i, frag := range manifest.Fragments {
		chunkData := make([]byte, frag.Length)
		if _, err := io.ReadFull(data, chunkData); err != nil {
			return "", fmt.Errorf("failed to read data for fragment %d: %w", i, err)
		}
		if err := tempWriter.WriteFragment(&frag, chunkData); err != nil {
			return "", fmt.Errorf("failed to write fragment %d: %w", i, err)
		}
	}

	// 3. 写入 Manifest
	if err := tempWriter.WriteManifest(manifest); err != nil {
		return "", fmt.Errorf("failed to write manifest: %w", err)
	}

	// 4. 【关键修复】在重命名之前，显式并唯一地关闭 writer
	if err := tempWriter.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer (write footer): %w", err)
	}

	// 5. 【关键修复】文件句柄已关闭，现在可以安全地进行原子重命名
	if err := os.Rename(tempPath, outputPath); err != nil {
		return "", fmt.Errorf("failed to atomically rename temp file to final file: %w", err)
	}

	// 标记成功，以防止 defer 清理函数删除最终文件
	success = true
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
