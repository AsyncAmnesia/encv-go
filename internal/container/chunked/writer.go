// internal/container/chunked/writer.go

package chunked

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/utils"
)

// WriteMainChunk 写入主分片文件
func WriteMainChunk(mainMagic []byte, filename string, kviData []byte, videoStream io.Reader, originalFileMD5 string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create main chunk file: %w", err)
	}
	defer file.Close()

	// 准备头部数据
	var magicArray [32]byte
	copy(magicArray[:], mainMagic)

	var md5Array [32]byte
	copy(md5Array[:], originalFileMD5)

	header := MainChunkHeader{
		ChunkedFileHeader: ChunkedFileHeader{
			Magic:           magicArray,
			OriginalFileMD5: md5Array,
			DataSize:        uint64(len(kviData)), // DataSize 在这里是 KVI 的长度
		},
		KVILength: uint64(len(kviData)),
	}

	// 写入头部
	if err := binary.Write(file, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("failed to write main chunk header: %w", err)
	}

	// 写入 KVI
	if _, err := file.Write(kviData); err != nil {
		return fmt.Errorf("failed to write KVI data: %w", err)
	}

	// 写入视频流
	if _, err := io.Copy(file, videoStream); err != nil {
		return fmt.Errorf("failed to write video data: %w", err)
	}

	return nil
}

// WriteSubChunk 写入子分片文件
// 【修改】返回文件名、MD5、大小和错误
func WriteSubChunk(subMagic []byte, mainChunkPath string, chunkIndex int, videoStream io.Reader, originalFileMD5 string) (string, string, int64, error) {
	// 1. 构建子分片文件名
	subChunkPath := fmt.Sprintf("%s.encv%d", mainChunkPath, chunkIndex)
	file, err := os.Create(subChunkPath)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to create sub-chunk file %d: %w", chunkIndex, err)
	}
	defer file.Close()

	// 2. 准备并写入头部
	var magicArray [32]byte
	copy(magicArray[:], subMagic)

	var md5Array [32]byte
	copy(md5Array[:], originalFileMD5)

	// 先写入一个占位符头部
	header := ChunkedFileHeader{
		Magic:           magicArray,
		OriginalFileMD5: md5Array,
		DataSize:        0, // 占位符
	}
	if err := binary.Write(file, binary.LittleEndian, &header); err != nil {
		return "", "", 0, fmt.Errorf("failed to write sub-chunk header: %w", err)
	}

	// 3. 写入视频流并计数
	written, err := io.Copy(file, videoStream)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to write sub-chunk data: %w", err)
	}

	// 4. 回写 DataSize
	_, err = file.Seek(int64(len(subMagic)), io.SeekStart) // Skip magic to reach DataSize
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to seek in sub-chunk to update DataSize: %w", err)
	}
	header.DataSize = uint64(written)
	if err := binary.Write(file, binary.LittleEndian, header.DataSize); err != nil {
		return "", "", 0, fmt.Errorf("failed to update sub-chunk DataSize: %w", err)
	}

	// 5. 【新增】计算并返回 MD5
	// file.Close() 会被 defer 调用，确保文件已刷新到磁盘
	md5Hash, err := utils.FileMD5(subChunkPath)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to calculate MD5 for sub-chunk %s: %w", subChunkPath, err)
	}

	// 6. 返回文件名、MD5和大小
	filename := filepath.Base(subChunkPath)
	return filename, md5Hash, written, nil
}
