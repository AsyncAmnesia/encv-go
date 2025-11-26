// internal/container/chunk.go

package container

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Soltus/encv-go/internal/crypto"
)

const (
	// 32 (Magic) + 32 (OriginalFileMD5) + 8 (DataSize) = 72
	ChunkHeaderSize = 32 + 32 + 8
	SubChunkSuffix  = ".encv" // 子分片后缀
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

// multiReadCloser 是一个 io.ReadCloser，它将多个 io.Closer 组合在一起
type multiReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (mrc *multiReadCloser) Close() error {
	var errs []error
	for _, c := range mrc.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to close one or more chunks: %v", errs)
	}
	return nil
}

// WriteMainChunk 写入主分片
func WriteMainChunk(filename string, kviData []byte, videoStream io.Reader, originalFileMD5 string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// 准备头部数据
	var magicArray [32]byte
	copy(magicArray[:], crypto.SccgvMainChunkMagic)

	var md5Array [32]byte
	copy(md5Array[:], originalFileMD5)

	header := MainChunkHeader{
		ChunkedFileHeader: ChunkedFileHeader{
			Magic:           magicArray,
			OriginalFileMD5: md5Array,
			DataSize:        uint64(len(kviData)),
		},
		KVILength: uint64(len(kviData)),
	}

	// 写入头部
	if err := binary.Write(file, binary.LittleEndian, header); err != nil {
		return err
	}

	// 写入 KVI
	if _, err := file.Write(kviData); err != nil {
		return err
	}

	// 写入视频流
	_, err = io.Copy(file, videoStream)
	return err
}

// WriteSubChunk 写入子分片
func WriteSubChunk(mainChunkPath string, chunkIndex int, videoStream io.Reader, originalFileMD5 string) (int64, error) {
	subChunkPath := fmt.Sprintf("%s%s%d", mainChunkPath, SubChunkSuffix, chunkIndex)
	file, err := os.Create(subChunkPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// 准备头部数据
	var magicArray [32]byte
	copy(magicArray[:], crypto.SccgvSubChunkMagic)

	var md5Array [32]byte
	copy(md5Array[:], originalFileMD5)

	header := ChunkedFileHeader{
		Magic:           magicArray,
		OriginalFileMD5: md5Array,
		DataSize:        0, // 占位符
	}

	// 写入头部
	if err := binary.Write(file, binary.LittleEndian, header); err != nil {
		return 0, err
	}

	// 写入视频流并计数
	written, err := io.Copy(file, videoStream)
	if err != nil {
		return 0, err
	}

	// 回写 DataSize
	_, err = file.Seek(int64(len(crypto.SccgvMainChunkMagic)), io.SeekStart)
	if err != nil {
		return 0, err
	}
	header.DataSize = uint64(written)
	if err := binary.Write(file, binary.LittleEndian, header.DataSize); err != nil {
		return 0, err
	}

	return written, nil
}

// FindMainChunk 根据任意一个分片文件，找到主分片文件
func FindMainChunk(anyChunkPath string) (string, error) {
	// 1. 从提供的分片中读取集合标识符（原始文件的 MD5）。
	originalMD5, err := readMD5FromChunk(anyChunkPath)
	if err != nil {
		return "", fmt.Errorf("failed to read MD5 from chunk %s: %w", anyChunkPath, err)
	}

	// 2. 【关键修复】检查提供的文件本身是否就是主分片。
	magic, err := readMagicFromChunk(anyChunkPath)
	if err == nil {
		// 【核心修复】只比较有效长度的字节
		if len(magic) >= len(crypto.SccgvMainChunkMagic) && string(magic[:len(crypto.SccgvMainChunkMagic)]) == crypto.SccgvMainChunkMagic {
			// 它就是主分片，直接返回。
			return anyChunkPath, nil
		}
	}

	// 3. 如果不是，扫描目录以查找主分片。
	dir := filepath.Dir(anyChunkPath)
	basePrefix := getChunkPrefix(anyChunkPath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), filepath.Base(basePrefix)) {
			continue
		}

		chunkPath := filepath.Join(dir, entry.Name())

		magic, err := readMagicFromChunk(chunkPath)
		if err != nil {
			continue
		}

		// 【核心修复】只比较有效长度的字节
		if len(magic) < len(crypto.SccgvMainChunkMagic) || string(magic[:len(crypto.SccgvMainChunkMagic)]) != crypto.SccgvMainChunkMagic {
			continue
		}

		// 这是一个主分片候选。验证它是否属于同一个集合。
		candidateMD5, err := readMD5FromChunk(chunkPath)
		if err != nil {
			continue
		}

		if candidateMD5 == originalMD5 {
			return chunkPath, nil
		}
	}

	return "", errors.New("main chunk not found in the directory")
}

// UnpackChunked 从主分片中解包数据，并创建一个包含所有分片的视频流
func UnpackChunked(mainChunkPath string) (*PackedData, error) {
	// 1. 打开主分片文件
	mainFile, err := os.Open(mainChunkPath)
	if err != nil {
		return nil, err
	}

	// 2. 读取并验证主分片头
	var header MainChunkHeader
	if err := binary.Read(mainFile, binary.LittleEndian, &header); err != nil {
		mainFile.Close()
		return nil, err
	}
	if len(header.Magic) < len(crypto.SccgvMainChunkMagic) || string(header.Magic[:len(crypto.SccgvMainChunkMagic)]) != crypto.SccgvMainChunkMagic {
		mainFile.Close()
		return nil, errors.New("invalid main chunk magic number")
	}

	// 3. 读取 KVI 数据
	kviData := make([]byte, header.KVILength)
	if _, err := io.ReadFull(mainFile, kviData); err != nil {
		mainFile.Close()
		return nil, err
	}

	// 4. 【关键修复】此时 mainFile 的指针已经位于加密视频流的开始
	//    我们将它作为 MultiReader 的第一个 reader
	var readers []io.Reader
	var closers []io.Closer
	readers = append(readers, mainFile)
	closers = append(closers, mainFile)

	// 5. 查找并打开所有子分片
	dir := filepath.Dir(mainChunkPath)
	baseName := filepath.Base(mainChunkPath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		mainFile.Close()
		return nil, err
	}

	var subChunkPaths []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), baseName+SubChunkSuffix) {
			continue
		}
		subChunkPaths = append(subChunkPaths, filepath.Join(dir, entry.Name()))
	}

	sort.Slice(subChunkPaths, func(i, j int) bool {
		numI := ParseChunkNumber(subChunkPaths[i])
		numJ := ParseChunkNumber(subChunkPaths[j])
		return numI < numJ
	})

	// 6. 打开每个子分片，并添加到 readers 和 closers 列表中
	for _, path := range subChunkPaths {
		file, err := os.Open(path)
		if err != nil {
			// 如果打开任何一个子分片失败，关闭所有已打开的文件
			for _, c := range closers {
				c.Close()
			}
			return nil, fmt.Errorf("failed to open sub-chunk %s: %w", path, err)
		}

		magic, err := readMagicFromChunk(path)
		if err != nil {
			file.Close()
			for _, c := range closers {
				c.Close()
			}
			return nil, fmt.Errorf("failed to read magic from sub-chunk %s: %w", path, err)
		}

		if len(magic) < len(crypto.SccgvSubChunkMagic) || string(magic[:len(crypto.SccgvSubChunkMagic)]) != crypto.SccgvSubChunkMagic {
			file.Close()
			for _, c := range closers {
				c.Close()
			}
			return nil, fmt.Errorf("invalid magic number for sub-chunk %s", path)
		}

		// Seek 到子分片的数据部分
		if _, err := file.Seek(ChunkHeaderSize, io.SeekStart); err != nil {
			file.Close()
			for _, c := range closers {
				c.Close()
			}
			return nil, fmt.Errorf("failed to seek in sub-chunk %s: %w", path, err)
		}

		readers = append(readers, file)
		closers = append(closers, file)
	}

	// 7. 创建一个可以统一关闭所有文件的 Reader
	videoStream := &multiReadCloser{
		Reader:  io.MultiReader(readers...),
		closers: closers,
	}

	return &PackedData{
		KVIData:     kviData,
		VideoStream: videoStream,
	}, nil
}

// --- Helper Functions ---

func readMD5FromChunk(path string) (string, error) {
	file, err := os.Open(path)
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

func readMagicFromChunk(path string) ([32]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return [32]byte{}, err
	}
	defer file.Close()

	var magicArray [32]byte
	_, err = io.ReadFull(file, magicArray[:])
	return magicArray, err
}

// getChunkPrefix 获取分片的基础路径（主分片的路径）
func getChunkPrefix(anyChunkPath string) string {
	base := filepath.Base(anyChunkPath)
	if idx := strings.LastIndex(base, SubChunkSuffix); idx > 0 {
		return filepath.Join(filepath.Dir(anyChunkPath), base[:idx])
	}
	return anyChunkPath
}

// createMultiChunkReader 创建一个可以顺序读取所有分片的 io.Reader
func createMultiChunkReader(mainChunkPath string) (io.Reader, error) {
	dir := filepath.Dir(mainChunkPath)
	baseName := filepath.Base(mainChunkPath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var chunkPaths []string
	chunkPaths = append(chunkPaths, mainChunkPath)

	var subChunks []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), baseName+SubChunkSuffix) {
			continue
		}
		subChunks = append(subChunks, filepath.Join(dir, entry.Name()))
	}

	sort.Slice(subChunks, func(i, j int) bool {
		numI := ParseChunkNumber(subChunks[i])
		numJ := ParseChunkNumber(subChunks[j])
		return numI < numJ
	})

	chunkPaths = append(chunkPaths, subChunks...)

	var readers []io.Reader
	for i, path := range chunkPaths {
		file, err := os.Open(path)
		if err != nil {
			// 清理已打开的文件
			for j := 0; j < i; j++ {
				if closer, ok := readers[j].(io.Closer); ok {
					closer.Close()
				}
			}
			return nil, err
		}

		var expectedMagic string
		if i == 0 {
			expectedMagic = crypto.SccgvMainChunkMagic
		} else {
			expectedMagic = crypto.SccgvSubChunkMagic
		}

		magic, err := readMagicFromChunk(path)
		if err != nil {
			file.Close()
			return nil, err
		}
		// 【核心修复】只比较有效长度的字节
		if len(magic) < len(expectedMagic) || string(magic[:len(expectedMagic)]) != expectedMagic {
			file.Close()
			return nil, fmt.Errorf("invalid magic number for chunk %s, expected %s", path, expectedMagic)
		}

		if _, err := file.Seek(ChunkHeaderSize, io.SeekStart); err != nil {
			file.Close()
			return nil, err
		}
		readers = append(readers, file)
	}

	return io.MultiReader(readers...), nil
}

// ParseChunkNumber 从子分片文件名中解析序号
func ParseChunkNumber(path string) int {
	base := filepath.Base(path)
	if idx := strings.LastIndex(base, SubChunkSuffix); idx > 0 {
		numStr := base[idx+len(SubChunkSuffix):]
		num, _ := strconv.Atoi(numStr)
		return num
	}
	return 0 // Main chunk is 0
}
