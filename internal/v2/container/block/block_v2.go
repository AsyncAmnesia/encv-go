// internal/v2/container/block_v2.go
package block

import (
	"encoding/binary"
	"fmt"
	"hash"
	"hash/crc32"
	"io"

	"github.com/Soltus/encv-go/internal/v2/types"
)

type BlockHeader_v2 struct {
	Type   uint16
	Length uint64
	CRC32  uint32
}

func GetBlockHeader_v2_Size() int64 {
	return int64(binary.Size(BlockHeader_v2{}))
}

// ReadBlockHeader_v2 从当前位置读取一个块头
func ReadBlockHeader_v2(r io.Reader) (*BlockHeader_v2, error) {
	var header BlockHeader_v2
	err := binary.Read(r, types.ByteOrder_v2, &header)
	if err != nil {
		return nil, fmt.Errorf("failed to read block header: %w", err)
	}
	return &header, nil
}

// ReadBlockData_v2 读取块的数据并校验CRC
func ReadBlockData_v2(r io.Reader, header *BlockHeader_v2) ([]byte, error) {
	data := make([]byte, header.Length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("failed to read block data: %w", err)
	}

	if actualCRC := crc32.ChecksumIEEE(data); actualCRC != header.CRC32 {
		return nil, fmt.Errorf("block data CRC32 mismatch (expected %08x, got %08x)", header.CRC32, actualCRC)
	}

	return data, nil
}

// WriteBlock_v2 写入一个完整的块 (头 + 数据)
func WriteBlock_v2(w io.Writer, blockType uint16, data []byte) error {
	header := &BlockHeader_v2{
		Type:   blockType,
		Length: uint64(len(data)),
		CRC32:  crc32.ChecksumIEEE(data),
	}

	if err := binary.Write(w, types.ByteOrder_v2, header); err != nil {
		return err
	}

	_, err := w.Write(data)
	return err
}

// WriteBlockToHasher 将一个完整的块 (头 + 数据) 写入到哈希器中，用于计算全局CRC
func WriteBlockToHasher(hasher hash.Hash32, blockType uint16, data []byte) error {
	header := &BlockHeader_v2{
		Type:   blockType,
		Length: uint64(len(data)),
		CRC32:  crc32.ChecksumIEEE(data),
	}

	// 将头部字节写入哈希器
	if err := binary.Write(hasher, types.ByteOrder_v2, header); err != nil {
		return err
	}

	// 将数据字节写入哈希器
	_, err := hasher.Write(data)
	return err
}
