// internal/v2/writer/chunked_container_writer.go

package writer

import (
	"encoding/binary"
	"hash"
	"hash/crc32"
	"io"
	"os"

	"github.com/Soltus/encv-go/internal/v2/container/block"
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
	if err := block.WriteBlock_v2(targetWriter, types.BlockTypeData_v2, data); err != nil {
		return 0, err
	}
	if err := block.WriteBlockToHasher(w.globalHasher, types.BlockTypeData_v2, data); err != nil {
		return 0, err
	}
	return crc32.ChecksumIEEE(data), nil
}

// WriteManifestAndFooter 将 Manifest 和 Footer 写入主容器文件的末尾
func (w *ChunkedContainerWriter) WriteManifestAndFooter(mainFile *os.File, manifest *types.Manifest_v2) error {
	manifestBytes, err := manifest.SerializeToJSON_v2()
	if err != nil {
		return err
	}

	// 写入 Manifest 块
	if err := block.WriteBlock_v2(mainFile, types.BlockTypeManifest_v2, manifestBytes); err != nil {
		return err
	}
	if err := block.WriteBlockToHasher(w.globalHasher, types.BlockTypeManifest_v2, manifestBytes); err != nil {
		return err
	}

	// 写入 Footer
	manifestOffset, err := mainFile.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	manifestBlockSize := int64(binary.Size(block.BlockHeader_v2{})) + int64(len(manifestBytes))
	footer := &types.EnvelopeFooter_v2{
		Magic:          types.MagicFooter_v2,
		ManifestOffset: uint64(manifestOffset - manifestBlockSize),
		ManifestLength: uint64(len(manifestBytes)),
		ManifestCRC32:  crc32.ChecksumIEEE(manifestBytes),
		GlobalCRC32:    w.globalHasher.Sum32(),
	}
	return binary.Write(mainFile, types.ByteOrder_v2, footer)
}
