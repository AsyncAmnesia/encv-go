// internal/v2/writer/block_writer.go

package writer

import (
	"hash"
	"io"
	"os"

	"github.com/Soltus/encv-go/internal/v2/container/block"
)

// BlockWriter 定义了写入单个数据块的底层接口
type BlockWriter interface {
	// WriteBlock 写入一个完整的块（头+数据）到其底层写入器
	WriteBlock(blockType uint16, data []byte) error
	// Close 关闭底层写入器（如果需要）
	Close() error
}

// fileBlockWriter 是 BlockWriter 的一个实现，用于写入文件并更新全局哈希
type fileBlockWriter struct {
	writer io.Writer
	closer io.Closer
	hasher hash.Hash32 // 可选的全局哈希器
}

// NewFileBlockWriter 创建一个用于文件的 BlockWriter
func NewFileBlockWriter(file *os.File, globalHasher hash.Hash32) BlockWriter {
	return &fileBlockWriter{
		writer: file,
		closer: file,
		hasher: globalHasher,
	}
}

// WriteBlock 写入一个完整的块到其底层写入器
func (w *fileBlockWriter) WriteBlock(blockType uint16, data []byte) error {
	// 写入文件
	if err := block.WriteBlock_v2(w.writer, blockType, data); err != nil {
		return err
	}
	// 写入哈希器（如果存在）
	if w.hasher != nil {
		if err := block.WriteBlockToHasher(w.hasher, blockType, data); err != nil {
			return err
		}
	}
	return nil
}

func (w *fileBlockWriter) Close() error {
	if w.closer != nil {
		return w.closer.Close()
	}
	return nil
}
