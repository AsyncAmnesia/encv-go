// internal/v2/writer/single_file_container_writer.go
package writer

import (
	"encoding/binary"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"os"

	"github.com/Soltus/encv-go/internal/v2/container/block"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// SingleFileContainerWriter 是 ContainerWriter_v2 的一个具体实现，专用于单文件容器
type SingleFileContainerWriter struct {
	file                    *os.File
	fragments               []types.Fragment_v2
	manifestOffset          uint64
	manifestLength          uint64
	currentDataStreamOffset uint64      // 【新增】用于追踪连续数据流的偏移量
	globalHasher            hash.Hash32 // 【新增】全局哈希器
	manifestBytes           []byte      // 【新增】缓存序列化后的 Manifest
}

// NewFileContainerWriter_v2 创建一个新的文件容器写入器，他会在关闭的时候自动写入 Footer
func NewSingleFileContainerWriter(outputPath string) (*SingleFileContainerWriter, error) {
	file, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	return &SingleFileContainerWriter{
		file:         file,
		globalHasher: crc32.NewIEEE(), // 初始化全局哈希器
	}, nil
}

// WriteKVI 实现 ContainerWriter_v2 接口
func (w *SingleFileContainerWriter) WriteKVI(kviData []byte) error {
	if err := block.WriteBlock_v2(w.file, types.BlockTypeKVI_v2, kviData); err != nil {
		return err
	}
	// 【新增】同时写入全局哈希器
	return block.WriteBlockToHasher(w.globalHasher, types.BlockTypeKVI_v2, kviData)
}

// WriteFragment 实现 ContainerWriter_v2 接口
func (w *SingleFileContainerWriter) WriteFragment(frag *types.Fragment_v2, data []byte) error {
	dataCRC32 := crc32.ChecksumIEEE(data)

	// 2. 【关键修复】创建一个完整的、新的 fragment，包含所有必要信息
	newFrag := types.Fragment_v2{
		ID:                frag.ID,
		Type:              frag.Type,
		Length:            uint64(len(data)),
		GlobalStartOffset: w.currentDataStreamOffset, // 【关键修复】使用连续数据流的偏移量
		DataCRC32:         dataCRC32,
		PhysicalPath:      frag.PhysicalPath,
	}
	w.fragments = append(w.fragments, newFrag)

	// 3. 写入数据块
	if err := block.WriteBlock_v2(w.file, types.BlockTypeData_v2, data); err != nil {
		return fmt.Errorf("failed to write data block for fragment %s: %w", frag.ID, err)
	}
	// 【新增】同时写入全局哈希器
	if err := block.WriteBlockToHasher(w.globalHasher, types.BlockTypeData_v2, data); err != nil {
		return fmt.Errorf("failed to write data block to global hasher: %w", err)
	}

	// 4. 【关键修复】更新数据流偏移量
	w.currentDataStreamOffset += newFrag.Length

	return nil
}

// WriteManifest 实现 ContainerWriter_v2 接口
func (w *SingleFileContainerWriter) WriteManifest(manifest *types.Manifest_v2) error {
	// 更新 manifest 中的 fragments 列表
	manifest.Fragments = w.fragments

	manifestBytes, err := manifest.SerializeToJSON_v2() // 【修改】缓存序列化结果
	if err != nil {
		return fmt.Errorf("failed to serialize manifest: %w", err)
	}
	w.manifestBytes = manifestBytes

	// 记录 Manifest 的位置和长度
	offset, err := w.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get current offset for manifest: %w", err)
	}
	w.manifestOffset = uint64(offset)
	w.manifestLength = uint64(len(w.manifestBytes))

	return block.WriteBlock_v2(w.file, types.BlockTypeManifest_v2, w.manifestBytes)
}

// Close 实现 ContainerWriter_v2 接口，完成所有收尾工作
func (w *SingleFileContainerWriter) Close() error {
	defer w.file.Close()

	// 【新增】将 Manifest 块也写入全局哈希器
	if err := block.WriteBlockToHasher(w.globalHasher, types.BlockTypeManifest_v2, w.manifestBytes); err != nil {
		return fmt.Errorf("failed to write manifest block to global hasher: %w", err)
	}
	// 获取文件信息以计算 ManifestOffset
	fileInfo, err := w.file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file before writing footer: %w", err)
	}
	manifestBlockSize := int64(binary.Size(block.BlockHeader_v2{})) + int64(len(w.manifestBytes))
	manifestOffset := fileInfo.Size() - manifestBlockSize

	footer := &types.EnvelopeFooter_v2{
		Magic:          types.MagicFooter_v2,
		ManifestOffset: uint64(manifestOffset),
		ManifestLength: uint64(len(w.manifestBytes)),
		ManifestCRC32:  crc32.ChecksumIEEE(w.manifestBytes),
		GlobalCRC32:    w.globalHasher.Sum32(), // 【新增】使用计算出的全局 CRC
	}
	// 在关闭文件前，计算并写入 Footer
	return binary.Write(w.file, types.ByteOrder_v2, footer)
}
