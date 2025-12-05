package reader

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/v2/container/block"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// verifyFragmentAt 从给定的 ReaderAt 的特定偏移量处验证一个片段。
// 线程安全（不修改接收者状态）。
func (r *fileContainerReader) verifyFragmentAt(readerAt io.ReaderAt, blockStartOffset int64, frag *types.Fragment_v2) error {
	headerSize := int64(binary.Size(block.BlockHeader_v2{}))
	headerReader := io.NewSectionReader(readerAt, blockStartOffset, headerSize)
	header, err := block.ReadBlockHeader_v2(headerReader)
	if err != nil {
		return fmt.Errorf("failed to read block header at offset %d: %w", blockStartOffset, err)
	}

	// 核心验证：CRC 与 长度
	if header.CRC32 != frag.DataCRC32 {
		return fmt.Errorf("crc mismatch for fragment '%s' (expected %08x, got %08x)", frag.ID, frag.DataCRC32, header.CRC32)
	}
	if header.Length != frag.Length {
		return fmt.Errorf("length mismatch for fragment '%s' (expected %d, got %d)", frag.ID, frag.Length, header.Length)
	}
	return nil
}

// findAndOpenFragmentRecovery 扫描目录以查找匹配的文件
func (r *fileContainerReader) findAndOpenFragmentRecovery(frag *types.Fragment_v2) (*os.File, error) {
	log.Printf("INFO: Entering recovery mode for fragment '%s' (CRC: %08x)", frag.ID, frag.DataCRC32)

	entries, err := os.ReadDir(r.containerDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read container directory '%s' for recovery: %w", r.containerDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// skip main file itself
		if entry.Name() == filepath.Base(r.mainFilePath) {
			continue
		}

		candidatePath := filepath.Join(r.containerDir, entry.Name())
		candidateFile, err := os.Open(candidatePath)
		if err != nil {
			// 忽略无法打开的条目
			continue
		}

		// 验证候选文件：校验在偏移 0 处的块头是否匹配 frag
		if err := r.verifyFragmentAt(candidateFile, 0, frag); err == nil {
			log.Printf("INFO: Recovery successful: found fragment '%s' at '%s'", frag.ID, candidatePath)
			return candidateFile, nil
		}
		_ = candidateFile.Close()
	}

	return nil, fmt.Errorf("recovery failed: could not find a valid file for fragment '%s' (crc %08x)", frag.ID, frag.DataCRC32)
}

// pooledFileHandleWrapper 是一个 ReadCloser 包装器，确保在 Close 时归还文件句柄到池中。
type pooledFileHandleWrapper struct {
	io.ReadCloser
	file *os.File
}

func (w *pooledFileHandleWrapper) Close() error {
	err := w.ReadCloser.Close()
	// 无论上层 ReadCloser 关闭是否成功，都必须归还句柄
	// 对于 io.NopCloser，Close 是 no-op
	if putErr := globalFileHandlePool.Put(w.file); putErr != nil {
		// 归还失败，记录日志，但不覆盖原始错误
		log.Printf("ERROR: failed to put file handle back to pool: %v", putErr)
	}
	return err
}
