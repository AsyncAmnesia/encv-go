// internal/container/chunked/metadata.go

package chunked

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"unsafe"
)

const (
	// ChunkHeaderSize 是子分片头的大小
	ChunkHeaderSize = int64(unsafe.Sizeof(ChunkedFileHeader{})) // 72 bytes
	// MainChunkHeaderSize 是主分片头的大小
	MainChunkHeaderSize = int64(unsafe.Sizeof(MainChunkHeader{})) // 80 bytes
)

// ChunkedFileHeader 是所有分片共有的文件头
type ChunkedFileHeader struct {
	Magic           [32]byte
	OriginalFileMD5 [32]byte
	DataSize        uint64
}

// MainChunkHeader 是主分片特有的头部
type MainChunkHeader struct {
	ChunkedFileHeader
	KVILength uint64 // KVI data length
}

// ReadMainHeader 从主分片文件中读取并验证头部
// 【修复版】兼容新旧两种魔法数字格式
func ReadMainHeader(file io.ReadSeeker, expectedMagic string) (*MainChunkHeader, error) {
	var header MainChunkHeader
	if err := binary.Read(file, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read main chunk header: %w", err)
	}

	// 【关键修复】更健壮的魔法数字验证
	// 【最终修复】只取与期望魔法数字等长的部分进行比较，完全忽略填充
	actualMagic := string(header.Magic[:len(expectedMagic)])
	if actualMagic != expectedMagic {
		return nil, fmt.Errorf("invalid main chunk magic number: expected '%s', got '%s'", expectedMagic, actualMagic)
	}

	return &header, nil
}

// ReadSubHeader 从子分片文件中读取并验证头部
// 【修复版】兼容新旧两种魔法数字格式
func ReadSubHeader(file io.ReadSeeker, expectedMagic string) (*ChunkedFileHeader, error) {
	var header ChunkedFileHeader
	if err := binary.Read(file, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read sub chunk header: %w", err)
	}

	// 【关键修复】更健壮的魔法数字验证
	// 【最终修复】只取与期望魔法数字等长的部分进行比较，完全忽略填充
	actualMagic := string(header.Magic[:len(expectedMagic)])
	if actualMagic != expectedMagic {
		return nil, fmt.Errorf("invalid sub chunk magic number: expected '%s', got '%s'", expectedMagic, actualMagic)
	}

	return &header, nil
}

// ReadMD5FromChunk 从任意分片文件中读取原始文件的MD5
func ReadMD5FromChunk(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = file.Seek(int64(32), io.SeekStart) // Skip magic
	if err != nil {
		return "", err
	}

	md5Bytes := make([]byte, 32)
	_, err = io.ReadFull(file, md5Bytes)
	return string(md5Bytes), err
}
