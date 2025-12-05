package chunker

import (
	"bufio"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/v2/types"
)

// PhysicalChunk 描述一个物理分片的元数据
type PhysicalChunk struct {
	// Filename 是物理分片的文件名，例如 "myvideo.4pm.sccgv.part0"
	Filename string `json:"filename"`
	// Size 是物理分片的大小（字节）
	Size int64 `json:"size"`
	// CRC32 是物理分片内容的 CRC32 校验和
	CRC32 uint32 `json:"crc32"`
}

// Chunker 定义物理分片器的接口
type Chunker interface {
	// Chunk 将源文件分割成多个物理分片
	Chunk(sourcePath, outputDir, baseName string) ([]PhysicalChunk, error)
}

// FixedSizeChunker 是按固定大小分片的实现
type FixedSizeChunker struct {
	chunkSize int64
}

// NewFixedSizeChunker 创建一个固定大小的分片器
func NewFixedSizeChunker(chunkSizeBytes int64) *FixedSizeChunker {
	return &FixedSizeChunker{chunkSize: chunkSizeBytes}
}

// Chunk 实现 Chunker 接口
func (c *FixedSizeChunker) Chunk(sourcePath, outputDir, baseName string) ([]PhysicalChunk, error) {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open source file %s: %w", sourcePath, err)
	}
	defer sourceFile.Close()

	_, err = sourceFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat source file: %w", err)
	}

	var chunks []PhysicalChunk
	buffer := make([]byte, c.chunkSize)
	chunkIndex := 0

	reader := bufio.NewReader(sourceFile)
	for {
		n, err := io.ReadFull(reader, buffer)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("error reading source file: %w", err)
		}

		if n == 0 {
			break // 文件读取完毕
		}

		// 1. 计算当前块的 CRC32
		crc := crc32.ChecksumIEEE(buffer[:n])

		// 2. 生成分片文件名
		chunkFilename := fmt.Sprintf("%s.part%d", baseName, chunkIndex)
		chunkPath := filepath.Join(outputDir, chunkFilename)

		// 3. 写入分片文件
		if err := os.WriteFile(chunkPath, buffer[:n], 0644); err != nil {
			return nil, fmt.Errorf("failed to write chunk file %s: %w", chunkPath, err)
		}

		chunks = append(chunks, PhysicalChunk{
			Filename: chunkFilename,
			Size:     int64(n),
			CRC32:    crc,
		})

		chunkIndex++
	}

	return chunks, nil
}

// isSingleFileContainer 判断是否为单文件容器
// 核心思想：如果任何 fragment 引用了外部文件，则不是单文件容器。
func IsSingleFileContainer(manifest *types.Manifest_v2) bool {
	for _, frag := range manifest.Fragments {
		// 检查所有可能指向外部文件的字段
		// PhysicalPath 是物理分片时使用的字段
		// Filename 是另一种可能的分片策略使用的字段
		if frag.PhysicalPath != "" || frag.Filename != "" {
			return false // 发现了外部文件引用，这是物理分片容器
		}
	}
	return true // 所有 fragment 都没有外部文件引用，这是单文件容器
}
