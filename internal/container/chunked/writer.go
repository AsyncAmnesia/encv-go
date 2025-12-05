// internal/container/chunked/writer.go

package chunked

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unsafe"

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
			// 由于主分片的视频数据大小在写入时无法预先得知（videoStream 是一个 io.Reader）将主分片的 DataSize 字段约定为 0，表示这个字段在主分片中无效。
			// 视频数据大小由文件总大小减去其他部分得出
			DataSize: 0,
		},
		KVILength: uint64(len(kviData)),
	}
	// 【新增关键调试日志】
	// log.Printf("DEBUG: About to write MainChunkHeader. KVILength should be %d. Full header: %+v", len(kviData), header)

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

// WriteSubChunk 写入子分片文件，返回文件名、MD5、大小和错误
func WriteSubChunk(subMagic []byte, mainChunkPath string, chunkIndex int, dataReader io.Reader, originalFileMD5 string) (string, string, int64, error) {
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

	// 先创建并写入头部，此时 DataSize 还未知，先写 0
	header := ChunkedFileHeader{
		Magic:           magicArray,
		OriginalFileMD5: md5Array,
		DataSize:        0, // 暂时为 0
	}
	if err := binary.Write(file, binary.LittleEndian, &header); err != nil {
		return "", "", 0, fmt.Errorf("failed to write sub-chunk header: %w", err)
	}

	// 3. 写入视频流
	written, err := io.Copy(file, dataReader)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to write sub-chunk data: %w", err)
	}
	// 3. 【高级修复】数据写入完毕，现在我们知道大小了，回头更新头部的 DataSize 字段
	// 计算DataSize字段在文件中的偏移量
	dataSizeOffset := int64(unsafe.Offsetof(header.DataSize))
	_, err = file.Seek(dataSizeOffset, io.SeekStart)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to seek to DataSize field for update: %w", err)
	}
	finalDataSize := uint32(written)
	if err := binary.Write(file, binary.LittleEndian, finalDataSize); err != nil {
		return "", "", 0, fmt.Errorf("failed to update DataSize in header: %w", err)
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
