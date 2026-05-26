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
	"github.com/Soltus/encv-go/internal/v2/container/manifest"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// SingleFileContainerWriter 是 ContainerWriter_v2 的一个具体实现，专用于单文件容器
type SingleFileContainerWriter struct {
	file                    *os.File
	fragments               []types.Fragment
	manifestOffset          uint64
	manifestLength          uint64
	currentDataStreamOffset uint64      // 用于追踪连续数据流的偏移量
	globalHasher            hash.Hash32 // 全局哈希器
	manifestBytes           []byte      // 缓存序列化后的 Manifest
	manifestCRC             uint32      // 缓存 Manifest CRC 以免重复计算
	headerVersion           int         // 3 or 4
	v4Header                *types.EnvelopeHeaderV4
}

// 创建一个新的文件容器写入器，他会在关闭的时候自动写入 Footer
func NewSingleFileContainerWriter(outputPath string, header *types.EnvelopeHeaderV3) (*SingleFileContainerWriter, error) {
	file, err := os.Create(outputPath)
	if err != nil {
		return nil, err
	}

	headerSize := 0
	if header != nil {
		if err := types.WriteHeaderV3(file, header); err != nil {
			return nil, err
		}
		headerSize = types.EnvelopeHeaderSize_v3
	}

	globalHasher := crc32.NewIEEE()
	if headerSize > 0 {
		headerBytes := make([]byte, headerSize)
		if _, err := file.Seek(0, io.SeekStart); err == nil {
			if _, err := io.ReadFull(file, headerBytes); err == nil {
				globalHasher.Write(headerBytes)
			}
		}
		file.Seek(int64(headerSize), io.SeekStart)
	}

	return &SingleFileContainerWriter{file: file, globalHasher: globalHasher, manifestCRC: 0, headerVersion: 3}, nil
}

func NewSingleFileContainerWriterV4(outputPath string, header *types.EnvelopeHeaderV4) (*SingleFileContainerWriter, error) {
	file, err := os.Create(outputPath)
	if err != nil {
		return nil, err
	}

	headerSize := 0
	if header != nil {
		if err := types.WriteHeaderV4(file, header); err != nil {
			return nil, err
		}
		headerSize = types.EnvelopeHeaderSize_v4
	}

	globalHasher := crc32.NewIEEE()
	if headerSize > 0 {
		headerBytes := make([]byte, headerSize)
		if _, err := file.Seek(0, io.SeekStart); err == nil {
			if _, err := io.ReadFull(file, headerBytes); err == nil {
				globalHasher.Write(headerBytes)
			}
		}
		file.Seek(int64(headerSize), io.SeekStart)
	}

	return &SingleFileContainerWriter{file: file, globalHasher: globalHasher, manifestCRC: 0, headerVersion: 4, v4Header: header}, nil
}

func (w *SingleFileContainerWriter) WriteKVI(kviData []byte) error {
	// 1. 写入文件 (并获得 CRC)
	crcVal, err := block.WriteBlock(w.file, types.BlockTypeKVI_v2, kviData)
	if err != nil {
		return err
	}

	// 2. 写入 Hasher
	// 【关键修复】必须使用上面返回的 crcVal，而不是 0
	header := &block.BlockHeader_v2{
		Type:   types.BlockTypeKVI_v2,
		Length: uint64(len(kviData)),
		CRC32:  crcVal, // 修正：使用实际计算的 CRC
	}
	return block.WriteBlockToHasherFromHeader(w.globalHasher, header, kviData)
}

func (w *SingleFileContainerWriter) WriteFragment(frag *types.Fragment, data []byte) error {
	// 1. 记录 PhysicalOffset
	if w.file != nil {
		if pos, err := w.file.Seek(0, io.SeekCurrent); err == nil {
			frag.PhysicalOffset = uint64(pos)
		}
	}

	// 2. 写入文件 (获得 CRC)
	crc, err := block.WriteBlock(w.file, types.BlockTypeData_v2, data)
	if err != nil {
		return fmt.Errorf("failed to write data block: %w", err)
	}

	// 3. 写入 Hasher
	header := &block.BlockHeader_v2{Type: types.BlockTypeData_v2, Length: uint64(len(data)), CRC32: crc}
	block.WriteBlockToHasherFromHeader(w.globalHasher, header, data)

	// 4. 更新状态
	w.fragments = append(w.fragments, types.Fragment{
		ID:                frag.ID,
		Type:              frag.Type,
		Length:            uint64(len(data)),
		GlobalStartOffset: w.currentDataStreamOffset,
		DataCRC32:         crc,
		PhysicalPath:      "",
		PhysicalOffset:    frag.PhysicalOffset,
	})
	w.currentDataStreamOffset += uint64(len(data))
	return nil
}

func (w *SingleFileContainerWriter) WriteManifest(manifestObj *types.Manifest) error {
	manifestObj.Fragments = w.fragments
	manifestBytes, err := manifestObj.SerializeToJSON()
	if err != nil {
		return err
	}
	w.manifestBytes = manifestBytes
	w.manifestLength = uint64(len(manifestBytes))

	// 2. 【关键新增】加密 Manifest
	encryptedManifestBytes, err := manifest.EncryptManifest(manifestBytes)
	if err != nil {
		return err
	}
	// 缓存加密后的数据，用于 Footer 计算
	w.manifestBytes = encryptedManifestBytes
	w.manifestLength = uint64(len(encryptedManifestBytes))

	// 3. 写入加密块到文件
	crcVal, err := block.WriteBlock(w.file, types.BlockTypeManifest_v2, encryptedManifestBytes)
	if err != nil {
		return err
	}

	// 4. 将加密块 Header 写入 Global Hasher
	// 确保 Header 和 EncryptedData 都在 Global Hash 中
	manifestBlockHeader := &block.BlockHeader_v2{
		Type:   types.BlockTypeManifest_v2,
		Length: uint64(len(encryptedManifestBytes)),
		CRC32:  crcVal,
	}
	if err := block.WriteBlockToHasherFromHeader(w.globalHasher, manifestBlockHeader, encryptedManifestBytes); err != nil {
		return err
	}

	// 5. 缓存 CRC (用于 Footer)
	w.manifestCRC = crcVal
	return nil
}

// Close 写入 Footer 并关闭文件
func (w *SingleFileContainerWriter) Close() error {
	defer w.file.Close()

	fileInfo, err := w.file.Stat()
	if err != nil {
		return err
	}

	manifestBlockSize := block.GetBlockHeader_v2_Size() + int64(len(w.manifestBytes))
	manifestBlockStart := fileInfo.Size() - manifestBlockSize

	if w.headerVersion == 4 {
		w.v4Header.ManifestOffset = uint64(manifestBlockStart + block.GetBlockHeader_v2_Size())
		w.v4Header.ManifestLength = w.manifestLength

		if _, err := w.file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek to header for v4 rewrite: %w", err)
		}
		if err := types.WriteHeaderV4(w.file, w.v4Header); err != nil {
			return fmt.Errorf("failed to rewrite v4 header: %w", err)
		}

		if _, err := w.file.Seek(0, io.SeekEnd); err != nil {
			return fmt.Errorf("failed to seek to end for v4 footer: %w", err)
		}

		footer := &types.EnvelopeFooterV4{
			Magic:       types.MagicFooter_v2,
			GlobalCRC32: w.globalHasher.Sum32(),
		}
		return types.WriteFooterV4(w.file, footer)
	}

	footer := &types.EnvelopeFooter_v2{
		Magic:          types.MagicFooter_v2,
		ManifestOffset: uint64(manifestBlockStart),
		ManifestLength: w.manifestLength,
		ManifestCRC32:  w.manifestCRC,
		GlobalCRC32:    w.globalHasher.Sum32(),
	}
	return binary.Write(w.file, types.ByteOrder_v2, footer)
}
