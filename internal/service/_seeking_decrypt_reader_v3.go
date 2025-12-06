package service

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/container/chunked"
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// ============================ 核心纯解密器 ============================
type CTRDecryptor struct {
	stream cipher.Stream
}

func NewCTRDecryptor(key, iv []byte) (*CTRDecryptor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}
	stream := cipher.NewCTR(block, iv)
	return &CTRDecryptor{stream: stream}, nil
}

func (d *CTRDecryptor) DecryptBytes(ciphertext []byte) {
	d.stream.XORKeyStream(ciphertext, ciphertext)
}

// ============================ 单文件容器阅读器 ============================
type SingleFileDecryptReader struct {
	file               *os.File
	dataStartPos       int64
	decryptor          *CTRDecryptor
	currentOffset      int64
	totalDecryptedSize int64
	mu                 sync.RWMutex
}

func NewSingleFileDecryptReader(filePath string, containerMagic []byte, key, iv []byte) (*SingleFileDecryptReader, error) {
	log.Printf("DEBUG SingleFile: Opening file: %s", filePath)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// 1. 解析容器头部
	magicBuf := make([]byte, len(containerMagic))
	if _, err := io.ReadFull(file, magicBuf); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read container magic: %w", err)
	}
	if !bytes.Equal(magicBuf, containerMagic) {
		file.Close()
		return nil, fmt.Errorf("container magic mismatch: expected %x, got %x", containerMagic, magicBuf)
	}
	log.Printf("DEBUG SingleFile: Container magic verified")

	var kviLen uint64
	if err := binary.Read(file, binary.LittleEndian, &kviLen); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read KVI length: %w", err)
	}
	log.Printf("DEBUG SingleFile: KVI length: %d", kviLen)

	if _, err := file.Seek(int64(kviLen), io.SeekCurrent); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to skip KVI data: %w", err)
	}

	// 2. 跳过加密头部
	cryptoMagic := []byte(crypto.MagicNumber)
	magicBuf = make([]byte, len(cryptoMagic))
	if _, err := io.ReadFull(file, magicBuf); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read encryption magic: %w", err)
	}
	if !bytes.Equal(magicBuf, cryptoMagic) {
		file.Close()
		return nil, fmt.Errorf("encryption magic mismatch: expected %s, got %s", cryptoMagic, magicBuf)
	}
	log.Printf("DEBUG SingleFile: Encryption magic verified")

	discardIV := make([]byte, crypto.IVLength)
	if _, err := io.ReadFull(file, discardIV); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read encryption IV: %w", err)
	}
	log.Printf("DEBUG SingleFile: Skipped encryption IV (length: %d)", crypto.IVLength)

	dataStartPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		file.Close()
		return nil, err
	}

	// 3. 计算解密数据的总大小
	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	totalDecryptedSize := fileInfo.Size() - dataStartPos
	if totalDecryptedSize <= 0 {
		file.Close()
		return nil, fmt.Errorf("no encrypted data found (dataStartPos: %d, fileSize: %d)", dataStartPos, fileInfo.Size())
	}

	log.Printf("DEBUG SingleFile: Data start position: %d, total decrypted size: %d", dataStartPos, totalDecryptedSize)

	// 4. 创建核心解密器
	decryptor, err := NewCTRDecryptor(key, iv)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to create decryptor: %w", err)
	}

	return &SingleFileDecryptReader{
		file:               file,
		dataStartPos:       dataStartPos,
		decryptor:          decryptor,
		currentOffset:      0,
		totalDecryptedSize: totalDecryptedSize,
	}, nil
}

func (r *SingleFileDecryptReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	actualFilePos := r.dataStartPos + r.currentOffset
	remaining := r.totalDecryptedSize - r.currentOffset

	if remaining <= 0 {
		return 0, io.EOF
	}

	if int64(len(p)) > remaining {
		p = p[:remaining]
	}

	_, err := r.file.Seek(actualFilePos, io.SeekStart)
	if err != nil {
		return 0, err
	}

	n, err := r.file.Read(p)
	if n > 0 {
		r.decryptor.DecryptBytes(p[:n])
		r.currentOffset += int64(n)
	}

	return n, err
}

func (r *SingleFileDecryptReader) Seek(offset int64, whence int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = r.currentOffset + offset
	case io.SeekEnd:
		newOffset = r.totalDecryptedSize + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}

	if newOffset < 0 || newOffset > r.totalDecryptedSize {
		return 0, fmt.Errorf("seek offset %d out of range [0, %d]", newOffset, r.totalDecryptedSize)
	}

	log.Printf("DEBUG SingleFileSeek: from %d to %d (whence: %d)", r.currentOffset, newOffset, whence)
	r.currentOffset = newOffset
	return newOffset, nil
}

func (r *SingleFileDecryptReader) Close() error {
	log.Printf("DEBUG SingleFile: Closing reader")
	return r.file.Close()
}

// ============================ 分片容器阅读器包装器 ============================
type ChunkedContainerDecryptReader struct {
	vs        *chunked.VirtualSeeker
	decryptor *CTRDecryptor
	mu        sync.RWMutex
	readCount int // 调试用：记录读取次数
	seekCount int // 调试用：记录寻址次数
}

func NewChunkedContainerDecryptReader(vs *chunked.VirtualSeeker, key, iv []byte) (*ChunkedContainerDecryptReader, error) {
	log.Printf("DEBUG Chunked: Creating decrypt reader for VirtualSeeker")

	decryptor, err := NewCTRDecryptor(key, iv)
	if err != nil {
		return nil, fmt.Errorf("failed to create decryptor: %w", err)
	}

	return &ChunkedContainerDecryptReader{
		vs:        vs,
		decryptor: decryptor,
		readCount: 0,
		seekCount: 0,
	}, nil
}

func (r *ChunkedContainerDecryptReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.readCount++
	// log.Printf("DEBUG ChunkedRead[%d]: Attempting to read %d bytes", r.readCount, len(p))

	n, err := r.vs.Read(p)
	// log.Printf("DEBUG ChunkedRead[%d]: Actually read %d bytes, error: %v", r.readCount, n, err)

	if n > 0 {
		r.decryptor.DecryptBytes(p[:n])
		// log.Printf("DEBUG ChunkedRead[%d]: Decrypted %d bytes", r.readCount, n)
	}

	return n, err
}

func (r *ChunkedContainerDecryptReader) Seek(offset int64, whence int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seekCount++
	log.Printf("DEBUG ChunkedSeek[%d]: Seeking to offset %d, whence %d", r.seekCount, offset, whence)

	// 先获取当前位置
	currentPos, _ := r.vs.Seek(0, io.SeekCurrent)
	log.Printf("DEBUG ChunkedSeek[%d]: Current position before seek: %d", r.seekCount, currentPos)

	newPos, err := r.vs.Seek(offset, whence)
	log.Printf("DEBUG ChunkedSeek[%d]: New position after seek: %d, error: %v", r.seekCount, newPos, err)

	return newPos, err
}

func (r *ChunkedContainerDecryptReader) Close() error {
	log.Printf("DEBUG Chunked: Closing reader (read operations: %d, seek operations: %d)", r.readCount, r.seekCount)
	return r.vs.Close()
}

// ============================ 主入口函数 ============================
func NewSeekableDecryptReaderFromContainer(ctx context.Context, mainChunkPath string) (ReadSeekerCloser, types.Index, int64, error) {
	log.Printf("NewSeekableDecryptReaderFromContainer -> mainChunkPath=%s", mainChunkPath)
	cfg := config.FromContext(ctx)

	// 1. 检测容器类型
	detectedType, err := container.DetectMainOrSubContainerType(ctx, mainChunkPath)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to detect container type: %w", err)
	}
	log.Printf("DEBUG Main: Detected container type: %s", detectedType)

	if detectedType == container.SubChunkType {
		return nil, nil, 0, fmt.Errorf("cannot create decrypt reader from sub-chunk, need main container")
	}

	magicMap, err := container.GetContainerMagicMap(ctx)
	if err != nil {
		return nil, nil, 0, err
	}
	mainMagic := magicMap[detectedType]
	log.Printf("DEBUG Main: Container magic: %s", mainMagic)

	// 2. 获取并解析 KVI 数据
	log.Printf("DEBUG Main: Getting KVI data from: %s", mainChunkPath)
	kviData, err := chunked.GetKVIDataOnly(mainChunkPath, mainMagic)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to get KVI data: %w", err)
	}
	log.Printf("DEBUG Main: KVI data length: %d bytes", len(kviData))

	index, err := utils.UnmarshalKVI(kviData)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to parse KVI: %w", err)
	}
	log.Printf("DEBUG Main: KVI parsed successfully, original size: %d", index.GetOriginalFileSize())

	// 检查是否有子分片
	hasSubChunks := index.HasSubChunks()
	log.Printf("DEBUG Main: Has sub-chunks: %v", hasSubChunks)
	if hasSubChunks {
		subChunks := index.GetSubChunks()
		log.Printf("DEBUG Main: Number of sub-chunks: %d", len(subChunks))
	}

	// 3. 派生密钥和 IV
	encInfo := index.GetEncryptionInfo()
	salt, err := crypto.Base64Decode(encInfo.SaltBase64)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to decode salt: %w", err)
	}
	log.Printf("DEBUG Main: Salt decoded, length: %d", len(salt))

	iv, err := crypto.Base64Decode(encInfo.IVBase64)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to decode IV from KVI: %w", err)
	}
	log.Printf("DEBUG Main: IV decoded, length: %d", len(iv))

	totalDerivedKey, err := crypto.GenerateKey([]byte(cfg.Password), salt, crypto.KeySize+crypto.IVLength)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to generate derived key: %w", err)
	}
	encKey := totalDerivedKey[:crypto.KeySize]
	log.Printf("DEBUG Main: Derived key, length: %d", len(encKey))

	// 4. 根据容器类型创建不同的解密阅读器
	var decryptReader ReadSeekerCloser

	if hasSubChunks {
		log.Printf("DEBUG Main: Creating chunked container reader")

		subMagicMap, err := container.GetSubChunkMagicMap(ctx)
		if err != nil {
			return nil, nil, 0, err
		}
		subMagic := subMagicMap[detectedType]
		log.Printf("DEBUG Main: Sub-chunk magic: %s", subMagic)

		virtualSeeker, err := chunked.NewVirtualSeeker(filepath.Dir(mainChunkPath), mainChunkPath, mainMagic, subMagic)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("failed to create virtual seeker: %w", err)
		}
		log.Printf("DEBUG Main: VirtualSeeker created successfully")

		decryptReader, err = NewChunkedContainerDecryptReader(virtualSeeker, encKey, iv)
		if err != nil {
			virtualSeeker.Close()
			return nil, nil, 0, fmt.Errorf("failed to create chunked decrypt reader: %w", err)
		}
		log.Printf("DEBUG Main: ChunkedContainerDecryptReader created successfully")
	} else {
		log.Printf("DEBUG Main: Creating single file container reader")
		decryptReader, err = NewSingleFileDecryptReader(mainChunkPath, []byte(mainMagic), encKey, iv)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("failed to create single file decrypt reader: %w", err)
		}
		log.Printf("DEBUG Main: SingleFileDecryptReader created successfully")
	}

	// 5. 返回结果
	originalSize := index.GetOriginalFileSize()
	log.Printf("DEBUG Main: Returning reader, original size: %d", originalSize)
	return decryptReader, index, originalSize, nil
}

// ReadSeekerCloser 组合了 io.Reader, io.Seeker, 和 io.Closer 接口
type ReadSeekerCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}
