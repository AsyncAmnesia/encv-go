package packer

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"

	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/types"
)

// Packer 定义了将加密数据打包成特定类型容器的接口。
type Packer interface {
	// Pack 接收基础信息、加密数据流和 Index，完成所有打包工作，包括生成最终文件名。
	Pack(ctx context.Context, baseName, outputDir string, encryptedDataReader io.Reader, index types.Index) error
}

var (
	registry = make(map[reflect.Type]Packer)
	mu       sync.RWMutex
)

func Register(indexType types.Index, p Packer) {
	mu.Lock()
	defer mu.Unlock()
	t := reflect.TypeOf(indexType)
	if _, exists := registry[t]; exists {
		panic(fmt.Sprintf("packer for type '%s' already registered", t))
	}
	registry[t] = p
}

// GetPacker 根据给定的 types.Index 实例查找对应的 Packer
func GetPacker(index types.Index) (Packer, error) {
	mu.RLock()
	defer mu.RUnlock()

	t := reflect.TypeOf(index)
	if p, ok := registry[t]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("no packer found for index type: %T", index)
}

// InitUnpackers 初始化并注册所有解包器
func InitPackers() {
	// 为了让 Register 能获取到正确的类型，我们需要传入实例的指针
	Register(&types.VideoIndex{}, &VideoPacker{})
	Register(&types.ImageIndex{}, &GenericPacker{})
	Register(&types.TextIndex{}, &GenericPacker{}) // 复用 GenericUnpacker
}

// BasePacker 提供打包单文件容器的工具方法。
type BasePacker struct {
	Packer
}

// WriteSingleFileContainer 将 KVI 和数据流打包成一个单文件容器。
func (b *BasePacker) WriteSingleFileContainer(finalPath string, magic []byte, kviData []byte, dataReader io.Reader) error {
	outputFile, err := os.Create(finalPath)
	if err != nil {
		return fmt.Errorf("failed to create final container file: %w", err)
	}
	defer outputFile.Close()

	if err := container.Pack(outputFile, magic, kviData, dataReader); err != nil {
		return fmt.Errorf("failed to pack container: %w", err)
	}
	return nil
}

// BaseChunkedPacker 提供打包分块容器的工具方法。
// type BaseChunkedPacker struct {
// 	BasePacker
// }

// WriteAllChunks 将 KVI 和数据流打包成一个分块容器。
// func (b *BaseChunkedPacker) WriteAllChunks(chunkSize int64, finalPath string, mainMagic, subMagic []byte, index *types.VideoIndex, encryptedDataReader io.Reader, originalMD5 string) error {

// 	// 1. 将数据流暂存到临时文件
// 	tmpFile, err := os.CreateTemp("", "encv-pack-*.tmp")
// 	if err != nil {
// 		return fmt.Errorf("failed to create temp file: %w", err)
// 	}
// 	tmpPath := tmpFile.Name()
// 	defer os.Remove(tmpPath)

// 	size, err := io.Copy(tmpFile, encryptedDataReader)
// 	if err != nil {
// 		tmpFile.Close()
// 		return fmt.Errorf("failed to write data to temp file: %w", err)
// 	}
// 	if err := tmpFile.Close(); err != nil {
// 		return fmt.Errorf("failed to close temp file: %w", err)
// 	}

// 	encFile, err := os.Open(tmpPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to open temp file for reading: %w", err)
// 	}
// 	defer encFile.Close()

// 	// 【新增日志 1】检查文件大小和分块大小
// 	log.Printf("DEBUG: WriteAllChunks: Total encrypted size = %d bytes, chunkSize = %d bytes.", size, chunkSize)

// 	// 先创建所有子分片，并收集元数据
// 	var subChunks []types.SubChunkInfo
// 	chunkIndex := 2

// 	for {
// 		offset := int64(chunkIndex-1) * int64(chunkSize)
// 		if offset >= size {
// 			break
// 		}
// 		_, err := encFile.Seek(offset, io.SeekStart)
// 		if err != nil {
// 			return fmt.Errorf("failed to seek in temp file: %w", err)
// 		}

// 		dataReader := io.LimitReader(encFile, int64(chunkSize))
// 		// 【关键】获取 WriteSubChunk 的返回值
// 		filename, md5, written, err := chunked.WriteSubChunk(subMagic, finalPath, chunkIndex, dataReader, originalMD5)
// 		if err != nil {
// 			log.Printf("Warning: Failed to write sub-chunk %d: %v. Skipping.", chunkIndex, err)
// 			if written == 0 {
// 				break
// 			}
// 			chunkIndex++
// 			continue
// 		}
// 		if written == 0 {
// 			break
// 		}

// 		// 【关键】收集子分片信息
// 		subChunks = append(subChunks, types.SubChunkInfo{
// 			Index:    chunkIndex,
// 			Filename: filename,
// 			MD5:      md5,
// 		})

// 		chunkIndex++
// 	}
// 	// 【新增日志 2】检查最终收集到的子分片数量
// 	log.Printf("DEBUG: WriteAllChunks: Finished loop. Collected %d sub-chunks.", len(subChunks))

// 	// 将收集到的子分片信息存入 KVI
// 	index.subChunks = subChunks

// 	// 现在 KVI 完整了，序列化它
// 	kviData, err := json.Marshal(index)
// 	if err != nil {
// 		return fmt.Errorf("failed to marshal VideoIndex to JSON: %w", err)
// 	}

// 	// log.Printf("DEBUG: KVI data before writing to file (len=%d): %s", len(kviData), string(kviData))

// 	// 【修复 6】最后，写入主分片
// 	_, err = encFile.Seek(0, io.SeekStart)
// 	if err != nil {
// 		return fmt.Errorf("failed to seek to start of temp file: %w", err)
// 	}
// 	mainDataReader := io.LimitReader(encFile, int64(chunkSize))
// 	if err := chunked.WriteMainChunk(mainMagic, finalPath, kviData, mainDataReader, originalMD5); err != nil {
// 		return fmt.Errorf("failed to write main chunk: %w", err)
// 	}

// 	fmt.Printf("-> Successfully packed chunked container: %s\n", finalPath)
// 	return nil
// }
