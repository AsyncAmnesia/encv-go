// internal/v2/writer/chunked_container_writer.go

package writer

import (
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"os"

	"github.com/Soltus/encv-go/internal/v2/container/block"
	"github.com/Soltus/encv-go/internal/v2/container/manifest"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// ChunkedContainerWriter 提供构建物理分片容器所需的工具方法
type ChunkedContainerWriter struct {
	globalHasher hash.Hash32
}

func NewChunkedContainerWriter(globalHasher hash.Hash32) *ChunkedContainerWriter {
	return &ChunkedContainerWriter{
		globalHasher: globalHasher,
	}
}

// WriteDataChunk 将一个数据块写入指定的目标 writer，并返回其 CRC
func (w *ChunkedContainerWriter) WriteDataChunk(targetWriter io.Writer, data []byte) (uint32, error) {
	// 1. 写入文件并获取 CRC
	crcVal, err := block.WriteBlock_v2(targetWriter, types.BlockTypeData_v2, data)
	if err != nil {
		return 0, err
	}

	// 2. 写入全局 Hasher（复用 Header/CRC）
	header := &block.BlockHeader_v2{
		Type:   types.BlockTypeData_v2,
		Length: uint64(len(data)),
		CRC32:  crcVal,
	}
	if err := block.WriteBlockToHasherFromHeader(w.globalHasher, header, data); err != nil {
		return 0, err
	}

	return crcVal, nil
}

// WriteManifestAndFooter 将 Manifest 和 Footer 写入主容器文件的末尾
func (w *ChunkedContainerWriter) WriteManifestAndFooter(mainFile *os.File, manifestObj *types.Manifest_v2) error {
	manifestBytes, err := manifestObj.SerializeToJSON_v2()
	if err != nil {
		return err
	}

	// 2. 【关键新增】加密 Manifest
	encryptedManifestBytes, err := manifest.EncryptManifest(manifestBytes)
	if err != nil {
		return fmt.Errorf("failed to encrypt manifest: %w", err)
	}

	// 写入 Manifest 块并获取 CRC
	// 3. 写入加密块到文件 (计算并写入 Header + EncryptedData)
	// WriteBlock_v2 会计算 EncryptedData 的 CRC
	crcVal, err := block.WriteBlock_v2(mainFile, types.BlockTypeManifest_v2, encryptedManifestBytes)
	if err != nil {
		return fmt.Errorf("failed to write manifest block: %w", err)
	}

	// 4. 将加密块 Header 写入 Global Hasher（复用 CRC）
	// 注意：写入 Hasher 的 Header CRC 必须与写入文件的一致
	manifestBlockHeader := &block.BlockHeader_v2{
		Type:   types.BlockTypeManifest_v2,
		Length: uint64(len(encryptedManifestBytes)),
		CRC32:  crcVal,
	}
	if err := block.WriteBlockToHasherFromHeader(w.globalHasher, manifestBlockHeader, encryptedManifestBytes); err != nil {
		return err
	}

	// 5. 写入 Footer
	manifestOffset, err := mainFile.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	// 计算块总大小 (Header + EncryptedData)
	manifestBlockSize := block.GetBlockHeader_v2_Size() + int64(len(encryptedManifestBytes))

	footer := &types.EnvelopeFooter_v2{
		Magic:          types.MagicFooter_v2,
		ManifestOffset: uint64(manifestOffset - manifestBlockSize),
		ManifestLength: uint64(len(encryptedManifestBytes)), // 记录加密长度
		ManifestCRC32:  crcVal,                              // 记录加密数据的 CRC
		GlobalCRC32:    w.globalHasher.Sum32(),
	}
	return binary.Write(mainFile, types.ByteOrder_v2, footer)
}
