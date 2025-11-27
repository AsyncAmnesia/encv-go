// internal/container/chunked/reader.go

package chunked

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// ChunkedReader 将多个分片文件抽象成一个连续的、可关闭的数据流
type ChunkedReader struct {
	// KVI 数据
	KVIData []byte
	// 用于读取加密数据的 readers 列表
	dataReaders []io.Reader
	// 需要关闭的文件句柄列表
	closers []io.Closer
	// 当前正在读取的 reader 索引
	currentReaderIndex int
	// 当前 reader
	currentReader io.Reader
}

// SubChunkStreamProvider 是一个函数类型，用于根据子分片信息提供数据流
type SubChunkStreamProvider func(subChunkInfo types.SubChunkInfo) (io.ReadCloser, error)

// StreamingChunkedReader 将一个主分片流和多个子分片流抽象成一个连续的、可关闭的数据流
type StreamingChunkedReader struct {
	// KVI 数据
	KVIData []byte
	// 用于获取子分片流的函数
	provider SubChunkStreamProvider
	// 主分片的数据流（已跳过头部和KVI）
	mainDataReader io.Reader
	// 子分片信息列表
	subChunks []types.SubChunkInfo
	// 当前正在读取的子分片索引
	currentSubChunkIndex int
	// 当前正在读取的流
	currentReader io.Reader
	// 需要关闭的流
	closer io.Closer
}

// 为路由路径设计的
func StreamingReader(mainDataReader io.Reader, kviData []byte, provider SubChunkStreamProvider) (*StreamingChunkedReader, error) {
	// 解析 KVI 以获取子分片信息
	index, err := utils.UnmarshalKVI(kviData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal KVI: %w", err)
	}

	videoIndex, ok := index.(*types.VideoIndex)
	if !ok {
		return nil, fmt.Errorf("KVI is not a VideoIndex, cannot process sub-chunks")
	}

	return &StreamingChunkedReader{
		KVIData:              kviData,
		provider:             provider,
		mainDataReader:       mainDataReader,
		subChunks:            videoIndex.SubChunks,
		currentSubChunkIndex: -1, // -1 表示还在读取主分片
		currentReader:        mainDataReader,
	}, nil
}

// 为本地路径设计的
func LocalReader(mainChunkPath, mainMagic, subMagic string) (*ChunkedReader, error) {
	// 1. 打开主分片并读取头
	mainFile, err := os.Open(mainChunkPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open main chunk: %w", err)
	}

	header, err := ReadMainHeader(mainFile, mainMagic)
	if err != nil {
		mainFile.Close()
		return nil, err
	}

	// 2. 读取 KVI 数据
	kviData := make([]byte, header.KVILength)
	if _, err := io.ReadFull(mainFile, kviData); err != nil {
		mainFile.Close()
		return nil, fmt.Errorf("failed to read KVI data: %w", err)
	}

	// 3. 准备 readers 和 closers
	cr := &ChunkedReader{
		KVIData:     kviData,
		dataReaders: []io.Reader{mainFile}, // 主分片的剩余部分是第一个数据源
		closers:     []io.Closer{mainFile},
	}

	// 2. 【关键】解析 KVI 以获取子分片信息
	index, err := utils.UnmarshalKVI(kviData)
	if err != nil {
		mainFile.Close()
		return nil, fmt.Errorf("failed to unmarshal KVI: %w", err)
	}

	videoIndex, ok := index.(*types.VideoIndex)
	if !ok {
		mainFile.Close()
		return nil, fmt.Errorf("KVI is not a VideoIndex, cannot process sub-chunks")
	}

	// 4. 【关键更改】使用 KVI 中的子分片信息来加载和验证
	dir := filepath.Dir(mainChunkPath)
	for _, subChunkInfo := range videoIndex.SubChunks {
		subChunkPath := filepath.Join(dir, subChunkInfo.Filename)

		// 【验证】计算并比较 MD5
		actualMD5, err := utils.FileMD5(subChunkPath)
		if err != nil {
			cr.Close()
			return nil, fmt.Errorf("failed to calculate MD5 for sub-chunk %s: %w", subChunkPath, err)
		}
		if actualMD5 != subChunkInfo.MD5 {
			cr.Close()
			return nil, fmt.Errorf("MD5 mismatch for sub-chunk %s: expected %s, got %s", subChunkPath, subChunkInfo.MD5, actualMD5)
		}
		log.Printf("-> [ChunkedReader] MD5 verified for sub-chunk: %s", subChunkPath)

		file, err := os.Open(subChunkPath)
		if err != nil {
			cr.Close()
			return nil, fmt.Errorf("failed to open sub-chunk %s: %w", subChunkPath, err)
		}

		// 验证子分片头
		_, err = ReadSubHeader(file, subMagic)
		if err != nil {
			file.Close()
			cr.Close()
			return nil, fmt.Errorf("invalid sub-chunk header in %s: %w", subChunkPath, err)
		}

		// Seek 到数据部分
		if _, err := file.Seek(ChunkHeaderSize, io.SeekStart); err != nil {
			file.Close()
			cr.Close()
			return nil, fmt.Errorf("failed to seek in sub-chunk %s: %w", subChunkPath, err)
		}

		cr.dataReaders = append(cr.dataReaders, file)
		cr.closers = append(cr.closers, file)
	}

	// 6. 初始化第一个 reader
	cr.currentReaderIndex = 0
	cr.currentReader = cr.dataReaders[0]

	return cr, nil
}

// Read 实现 io.Reader 接口
func (cr *ChunkedReader) Read(p []byte) (n int, err error) {
	for cr.currentReaderIndex < len(cr.dataReaders) {
		n, err = cr.currentReader.Read(p)
		if err != io.EOF {
			return n, err
		}
		// 当前 reader 读完，切换到下一个
		cr.currentReaderIndex++
		if cr.currentReaderIndex < len(cr.dataReaders) {
			cr.currentReader = cr.dataReaders[cr.currentReaderIndex]
		}
	}
	// 所有 reader 都读完了
	return 0, io.EOF
}

// Close 实现 io.Closer 接口，关闭所有文件句柄
func (cr *ChunkedReader) Close() error {
	var errs []error
	for _, c := range cr.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to close one or more chunk files: %v", errs)
	}
	return nil
}

// Read 实现 io.Reader 接口
func (scr *StreamingChunkedReader) Read(p []byte) (n int, err error) {
	for {
		n, err = scr.currentReader.Read(p)
		// 如果没有错误，或者错误不是 EOF，直接返回
		if err != io.EOF {
			return n, err
		}

		// 如果是 EOF，需要切换到下一个流
		// 先关闭当前流（如果需要）
		if scr.closer != nil {
			scr.closer.Close()
			scr.closer = nil
		}

		// 切换到下一个子分片
		scr.currentSubChunkIndex++
		if scr.currentSubChunkIndex >= len(scr.subChunks) {
			// 所有子分片都读完了
			return 0, io.EOF
		}

		// 从 provider 获取下一个子分片的流
		subChunkInfo := scr.subChunks[scr.currentSubChunkIndex]
		nextReader, err := scr.provider(subChunkInfo)
		if err != nil {
			return 0, fmt.Errorf("failed to get stream for sub-chunk %s: %w", subChunkInfo.Filename, err)
		}

		scr.currentReader = nextReader
		scr.closer = nextReader // 标记这个流需要被关闭
		// 循环继续，尝试从新的 reader 中读取数据
	}
}

// Close 实现 io.Closer 接口
func (scr *StreamingChunkedReader) Close() error {
	if scr.closer != nil {
		return scr.closer.Close()
	}
	return nil
}
