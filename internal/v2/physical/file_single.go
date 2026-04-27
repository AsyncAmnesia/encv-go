package physical

import (
	"context"
	"fmt"
	"io"
	"log"
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
func (p *SinglePhysicalPacker) Pack(manifest *types.Manifest_v2, req *PackRequest) (string, error) {
	outputPath := filepath.Join(req.OutputDir, req.FinalFileName)
	tempPath := outputPath + ".tmp"

	var header *types.EnvelopeHeaderV3
	var err error

	// 【修正】Header 配置：支持 SpecialID 为 None/nil
	if req.HeaderVersion == 3 {
		// 如果 SpecialID 为 nil，使用空字节切片或 nil
		// CreateHeaderV3 内部应能处理这种情况
		idData := req.SpecialID
		header, err = types.CreateHeaderV3(
			true,
			req.SpecialIDType,
			idData,
		)
		if err != nil {
			return "", fmt.Errorf("failed to prepare v3 header: %w", err)
		}
	}

	// 创建 Writer (Writer 内部负责写入 Header 并 Hash)
	tempWriter, err := writer.NewSingleFileContainerWriter(tempPath, header)
	if err != nil {
		return "", fmt.Errorf("failed to create container writer: %w", err)
	}

	finalPath, err := p.writeAndClose(req.EncryptedDataReader, manifest, *tempWriter, tempPath, outputPath)
	if err != nil {
		return "", err
	}

	log.Printf("✅ [SinglePhysicalPacker] Packed to: %s\n", finalPath)
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
		// 【关键修复】跳过 Metadata Fragments
		if frag.Type == types.FragmentType_Metadata {
			log.Printf("DEBUG: Skipping metadata fragment '%s' (not part of data stream)", frag.ID)
			continue
		}

		chunkData := make([]byte, frag.Length)
		if _, err := io.ReadFull(data, chunkData); err != nil {
			return "", fmt.Errorf("failed to read data for fragment %d: %w", i, err)
		}
		if err := tempWriter.WriteFragment(&frag, chunkData); err != nil {
			return "", fmt.Errorf("failed to write fragment %d: %w", i, err)
		}
	}

	if err := tempWriter.WriteManifest(manifest); err != nil {
		return "", fmt.Errorf("failed to write manifest: %w", err)
	}

	if err := tempWriter.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	if err := os.Rename(tempPath, outputPath); err != nil {
		return "", fmt.Errorf("failed to rename temp file: %w", err)
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
