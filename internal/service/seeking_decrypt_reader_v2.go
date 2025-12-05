package service

// import (
// 	"bytes"
// 	"context"
// 	"crypto/aes"
// 	"crypto/cipher"
// 	"encoding/binary"
// 	"fmt"
// 	"io"
// 	"log"
// 	"math/big"
// 	"os"
// 	"path/filepath"
// 	"sync"

// 	"github.com/Soltus/encv-go/internal/config"
// 	"github.com/Soltus/encv-go/internal/container"
// 	"github.com/Soltus/encv-go/internal/container/chunked"
// 	"github.com/Soltus/encv-go/internal/crypto"
// 	"github.com/Soltus/encv-go/internal/types"
// 	"github.com/Soltus/encv-go/internal/utils"
// )

// // 从主分片路径创建一个可寻址的解密流，用于处理分片容器。
// // 这是 open-stream 功能的核心。
// func NewSeekableDecryptReaderFromContainer(ctx context.Context, mainChunkPath string) (ReadSeekerCloser, types.Index, int64, error) {
// 	log.Printf("NewSeekableDecryptReaderFromContainer -> mainChunkPath=%s ", mainChunkPath)
// 	cfg := config.FromContext(ctx)

// 	// 1. 检测容器类型并获取魔法数字
// 	detectedType, err := container.DetectMainOrSubContainerType(ctx, mainChunkPath)
// 	if err != nil {
// 		return nil, nil, 0, fmt.Errorf("failed to detect container type: %w", err)
// 	}

// 	// 2. 检查是否是子分片，如果是则报错
// 	if detectedType == container.SubChunkType {
// 		return nil, nil, 0, fmt.Errorf("cannot create decrypt reader from sub-chunk, need main container")
// 	}

// 	// 3. 获取魔法数字
// 	magicMap, err := container.GetContainerMagicMap(ctx)
// 	if err != nil {
// 		return nil, nil, 0, err
// 	}
// 	mainMagic := magicMap[detectedType]

// 	// 4. 获取 KVI 数据以派生密钥和 IV
// 	kviData, err := chunked.GetKVIDataOnly(mainChunkPath, mainMagic)
// 	if err != nil {
// 		return nil, nil, 0, fmt.Errorf("failed to get KVI data: %w", err)
// 	}

// 	// 5. 解析 KVI
// 	index, err := utils.UnmarshalKVI(kviData)
// 	if err != nil {
// 		return nil, nil, 0, fmt.Errorf("failed to parse KVI: %w", err)
// 	}

// 	// 6. 派生密钥和 IV
// 	encInfo := index.GetEncryptionInfo()
// 	salt, err := crypto.Base64Decode(encInfo.SaltBase64)
// 	if err != nil {
// 		return nil, nil, 0, fmt.Errorf("failed to decode salt: %w", err)
// 	}
// 	iv, err := crypto.Base64Decode(encInfo.IVBase64)
// 	if err != nil {
// 		return nil, nil, 0, fmt.Errorf("failed to decode IV from KVI: %w", err)
// 	}

// 	totalDerivedKey, err := crypto.GenerateKey([]byte(cfg.Password), salt, crypto.KeySize+crypto.IVLength)
// 	if err != nil {
// 		return nil, nil, 0, fmt.Errorf("failed to generate derived key: %w", err)
// 	}
// 	encKey := totalDerivedKey[:crypto.KeySize]

// 	// 7. 判断是否是分块容器（通过检查KVI中是否有子分片信息）
// 	var dataSeeker ReadSeekerCloser

// 	if index.HasSubChunks() {
// 		// 分块容器：需要 VirtualSeeker
// 		subMagicMap, err := container.GetSubChunkMagicMap(ctx)
// 		if err != nil {
// 			return nil, nil, 0, err
// 		}
// 		subMagic := subMagicMap[detectedType]

// 		virtualSeeker, err := chunked.NewVirtualSeeker(filepath.Dir(mainChunkPath), mainChunkPath, mainMagic, subMagic)
// 		if err != nil {
// 			return nil, nil, 0, fmt.Errorf("[NewSeekableDecryptReaderFromContainer] failed to create virtual seeker: %w", err)
// 		}
// 		dataSeeker = virtualSeeker
// 		log.Printf("DEBUG: Chunked container detected, using VirtualSeeker")
// 	} else {
// 		// 单文件容器：直接读取文件，跳过头部
// 		file, err := os.Open(mainChunkPath)
// 		if err != nil {
// 			return nil, nil, 0, fmt.Errorf("failed to open single file container: %w", err)
// 		}

// 		// 计算头部大小
// 		headerSize := int64(len(mainMagic) + 8) // 魔法数字 + KVI长度(8字节)

// 		// 读取魔法数字进行验证
// 		magicBuf := make([]byte, len(mainMagic))
// 		if _, err := io.ReadFull(file, magicBuf); err != nil {
// 			file.Close()
// 			return nil, nil, 0, fmt.Errorf("failed to read magic: %w", err)
// 		}

// 		if !bytes.Equal(magicBuf, []byte(mainMagic)) {
// 			file.Close()
// 			return nil, nil, 0, fmt.Errorf("magic mismatch in container")
// 		}

// 		// 读取KVI长度并跳过
// 		var storedKviLen uint64
// 		if err := binary.Read(file, binary.LittleEndian, &storedKviLen); err != nil {
// 			file.Close()
// 			return nil, nil, 0, fmt.Errorf("failed to read KVI length: %w", err)
// 		}

// 		// 验证KVI长度是否匹配
// 		if storedKviLen != uint64(len(kviData)) {
// 			file.Close()
// 			return nil, nil, 0, fmt.Errorf("KVI length mismatch: expected %d, got %d", len(kviData), storedKviLen)
// 		}

// 		// 计算数据起始位置
// 		dataStartPos := headerSize + int64(storedKviLen)

// 		// 创建一个SectionReader来读取数据部分
// 		// 首先获取文件大小
// 		fileInfo, err := file.Stat()
// 		if err != nil {
// 			file.Close()
// 			return nil, nil, 0, fmt.Errorf("failed to get file info: %w", err)
// 		}

// 		// 计算数据部分的大小
// 		dataSize := fileInfo.Size() - dataStartPos
// 		if dataSize <= 0 {
// 			file.Close()
// 			return nil, nil, 0, fmt.Errorf("container has no data (data size: %d)", dataSize)
// 		}

// 		// 创建包装器，用于读取数据部分
// 		dataSeeker = &singleFileSeeker{
// 			file:       file,
// 			start:      dataStartPos,
// 			size:       dataSize,
// 			readOffset: 0,
// 		}

// 		log.Printf("DEBUG: Single file container detected, data start=%d, size=%d", dataStartPos, dataSize)
// 	}

// 	// 8. 创建解密流
// 	decryptedStream, err := NewSeekableCTRStreamFixed(dataSeeker, encKey, iv)
// 	if err != nil {
// 		dataSeeker.Close()
// 		return nil, nil, 0, fmt.Errorf("failed to create seeking decrypt reader: %w", err)
// 	}

// 	// 9. 返回结果
// 	originalSize := index.GetOriginalFileSize()
// 	return decryptedStream, index, originalSize, nil
// }

// // 为单文件容器实现的ReadSeekerCloser包装器
// type singleFileSeeker struct {
// 	file       *os.File
// 	start      int64 // 数据起始位置
// 	size       int64 // 数据大小
// 	readOffset int64 // 当前读取位置（相对于数据起始位置）
// }

// func (s *singleFileSeeker) Read(p []byte) (n int, err error) {
// 	// 计算在文件中的实际位置
// 	actualPos := s.start + s.readOffset

// 	// 确保不会读取超出数据范围
// 	remaining := s.size - s.readOffset
// 	if remaining <= 0 {
// 		return 0, io.EOF
// 	}

// 	// 限制读取长度
// 	if int64(len(p)) > remaining {
// 		p = p[:remaining]
// 	}

// 	// 定位到正确位置
// 	if _, err := s.file.Seek(actualPos, io.SeekStart); err != nil {
// 		return 0, err
// 	}

// 	// 读取数据
// 	n, err = s.file.Read(p)
// 	if err == nil {
// 		s.readOffset += int64(n)
// 	}
// 	return n, err
// }

// func (s *singleFileSeeker) Seek(offset int64, whence int) (int64, error) {
// 	var newOffset int64

// 	switch whence {
// 	case io.SeekStart:
// 		newOffset = offset
// 	case io.SeekCurrent:
// 		newOffset = s.readOffset + offset
// 	case io.SeekEnd:
// 		newOffset = s.size + offset
// 	default:
// 		return 0, fmt.Errorf("invalid whence: %d", whence)
// 	}

// 	// 边界检查
// 	if newOffset < 0 || newOffset > s.size {
// 		return 0, fmt.Errorf("seek offset %d out of range [0, %d]", newOffset, s.size)
// 	}

// 	s.readOffset = newOffset
// 	return newOffset, nil
// }

// func (s *singleFileSeeker) Close() error {
// 	return s.file.Close()
// }

// // 从主分片路径创建一个可寻址的解密流，用于处理分片容器。
// // 这是 open-stream 功能的核心。
// // func NewSeekableDecryptReaderFromContainer(ctx context.Context, mainChunkPath string) (ReadSeekerCloser, types.Index, int64, error) {
// // 	log.Printf("NewSeekableDecryptReaderFromContainer -> mainChunkPath=%s ", mainChunkPath)
// // 	cfg := config.FromContext(ctx)

// // 	// 1. 检测容器类型并获取魔法数字
// // 	detectedExt, err := container.DetectMainOrSubContainerType(ctx, mainChunkPath)
// // 	if err != nil {
// // 		return nil, nil, 0, fmt.Errorf("failed to detect container type: %w", err)
// // 	}
// // 	magicMap, err := container.GetContainerMagicMap(ctx)
// // 	if err != nil {
// // 		return nil, nil, 0, err
// // 	}
// // 	mainMagic := magicMap[detectedExt]
// // 	subMagicMap, err := container.GetSubChunkMagicMap(ctx)
// // 	if err != nil {
// // 		return nil, nil, 0, err
// // 	}
// // 	subMagic := subMagicMap[detectedExt]

// // 	// 2. 创建 VirtualSeeker，它现在提供纯净的加密数据流
// // 	virtualSeeker, err := chunked.NewVirtualSeeker(filepath.Dir(mainChunkPath), mainChunkPath, mainMagic, subMagic)
// // 	if err != nil {
// // 		return nil, nil, 0, fmt.Errorf("[NewSeekableDecryptReaderFromContainer] failed to create virtual seeker: %w", err)
// // 	}

// // 	// 3. 获取 KVI 数据以派生密钥和 IV
// // 	kviData, err := chunked.GetKVIDataOnly(mainChunkPath, mainMagic)
// // 	if err != nil {
// // 		virtualSeeker.Close()
// // 		return nil, nil, 0, fmt.Errorf("failed to get KVI data: %w", err)
// // 	}

// // 	// 4. 解析 KVI
// // 	index, err := utils.UnmarshalKVI(kviData)
// // 	if err != nil {
// // 		virtualSeeker.Close()
// // 		return nil, nil, 0, fmt.Errorf("failed to parse KVI: %w", err)
// // 	}

// // 	// 5. 派生密钥和 IV
// // 	encInfo := index.GetEncryptionInfo()
// // 	salt, err := crypto.Base64Decode(encInfo.SaltBase64)
// // 	if err != nil {
// // 		virtualSeeker.Close()
// // 		return nil, nil, 0, fmt.Errorf("failed to decode salt: %w", err)
// // 	}
// // 	iv, err := crypto.Base64Decode(encInfo.IVBase64)
// // 	if err != nil {
// // 		virtualSeeker.Close()
// // 		return nil, nil, 0, fmt.Errorf("failed to decode IV from KVI: %w", err)
// // 	}

// // 	totalDerivedKey, err := crypto.GenerateKey([]byte(cfg.Password), salt, crypto.KeySize+crypto.IVLength)
// // 	if err != nil {
// // 		virtualSeeker.Close()
// // 		return nil, nil, 0, fmt.Errorf("failed to generate derived key: %w", err)
// // 	}
// // 	encKey := totalDerivedKey[:crypto.KeySize]

// // 	decryptedStream, err := NewSeekableCTRStreamFixed(virtualSeeker, encKey, iv)
// // 	if err != nil {
// // 		virtualSeeker.Close()
// // 		return nil, nil, 0, fmt.Errorf("failed to create seeking decrypt reader: %w", err)
// // 	}

// // 	// 7. 返回结果
// // 	originalSize := index.GetOriginalFileSize()
// // 	return decryptedStream, index, originalSize, nil
// // }

// // ReadSeekerCloser 组合了 io.Reader, io.Seeker, 和 io.Closer 接口
// type ReadSeekerCloser interface {
// 	io.Reader
// 	io.Seeker
// 	io.Closer
// }

// // SeekableCTRStreamFixed 修复版本
// // type SeekableCTRStreamFixed struct {
// // 	src       io.ReadSeeker // VirtualSeeker
// // 	block     cipher.Block
// // 	iv        []byte // 从KVI获取的IV
// // 	blockSize int

// //		// 状态
// //		encryptionHeaderSize    int64
// //		encryptionHeaderSkipped bool
// //		pos                     int64
// //	}
// type SeekableCTRStreamFixed struct {
// 	src                     io.ReadSeeker
// 	block                   cipher.Block
// 	iv                      []byte
// 	blockSize               int
// 	encryptionHeaderSize    int64
// 	encryptionHeaderSkipped bool
// 	pos                     int64
// 	mu                      sync.RWMutex
// }

// // SeekableCTRStreamFixed 实现可寻址的CTR模式解密流
// func NewSeekableCTRStreamFixed(dataReader io.ReadSeeker, key, iv []byte) (ReadSeekerCloser, error) {
// 	block, err := aes.NewCipher(key)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &SeekableCTRStreamFixed{
// 		src:                  dataReader,
// 		block:                block,
// 		iv:                   iv,
// 		blockSize:            aes.BlockSize,
// 		encryptionHeaderSize: int64(len(crypto.MagicNumber) + crypto.IVLength),
// 		pos:                  0,
// 	}, nil
// }

// func (s *SeekableCTRStreamFixed) skipEncryptionHeader() error {
// 	if s.encryptionHeaderSkipped {
// 		return nil
// 	}

// 	// 回到开始位置
// 	_, err := s.src.Seek(0, io.SeekStart)
// 	if err != nil {
// 		return err
// 	}

// 	// 读取并验证魔法数字
// 	magicBuf := make([]byte, len(crypto.MagicNumber))
// 	if _, err := io.ReadFull(s.src, magicBuf); err != nil {
// 		return fmt.Errorf("failed to read magic number: %w", err)
// 	}

// 	if string(magicBuf) != crypto.MagicNumber {
// 		return fmt.Errorf("invalid encryption format")
// 	}

// 	// 读取IV（但使用KVI的IV）
// 	streamIV := make([]byte, crypto.IVLength)
// 	if _, err := io.ReadFull(s.src, streamIV); err != nil {
// 		return fmt.Errorf("failed to read IV: %w", err)
// 	}

// 	s.encryptionHeaderSkipped = true
// 	s.pos = 0

// 	return nil
// }

// func (s *SeekableCTRStreamFixed) Read(dst []byte) (int, error) {
// 	// 确保加密头部已跳过
// 	if !s.encryptionHeaderSkipped {
// 		if err := s.skipEncryptionHeader(); err != nil {
// 			return 0, err
// 		}
// 	}

// 	// 【关键修复】直接使用 s.pos 作为加密数据的偏移量
// 	// 因为 s.src 已经跳过了加密头部
// 	effectivePos := s.pos

// 	// 计算CTR
// 	blockIndex := effectivePos / int64(s.blockSize)
// 	byteOffsetInBlock := effectivePos % int64(s.blockSize)

// 	// 计算目标IV
// 	initialIVInt := new(big.Int).SetBytes(s.iv)
// 	blockOffsetInt := big.NewInt(blockIndex)
// 	targetIVInt := new(big.Int).Add(initialIVInt, blockOffsetInt)
// 	targetIV := targetIVInt.Bytes()

// 	if len(targetIV) < s.blockSize {
// 		paddedIV := make([]byte, s.blockSize)
// 		copy(paddedIV[s.blockSize-len(targetIV):], targetIV)
// 		targetIV = paddedIV
// 	}

// 	// 创建临时CTR流
// 	tempStream := cipher.NewCTR(s.block, targetIV)

// 	// 丢弃偏移部分
// 	if byteOffsetInBlock > 0 {
// 		discard := make([]byte, byteOffsetInBlock)
// 		tempStream.XORKeyStream(discard, discard)
// 	}

// 	// 读取并解密
// 	n, err := s.src.Read(dst)
// 	if n > 0 {
// 		tempStream.XORKeyStream(dst[:n], dst[:n])
// 		s.pos += int64(n)
// 	}

// 	return n, err
// }

// func (s *SeekableCTRStreamFixed) Seek(offset int64, whence int) (int64, error) {
// 	// 首先跳过加密头部
// 	if !s.encryptionHeaderSkipped {
// 		if err := s.skipEncryptionHeader(); err != nil {
// 			return 0, err
// 		}
// 	}

// 	// 计算新位置
// 	var newPos int64
// 	switch whence {
// 	case io.SeekStart:
// 		newPos = offset
// 	case io.SeekCurrent:
// 		newPos = s.pos + offset
// 	case io.SeekEnd:
// 		// 需要知道总大小
// 		totalSize, err := s.getTotalDecryptedSize()
// 		if err != nil {
// 			return 0, err
// 		}
// 		newPos = totalSize + offset
// 	default:
// 		return 0, fmt.Errorf("invalid whence")
// 	}

// 	if newPos < 0 {
// 		return 0, fmt.Errorf("negative position")
// 	}

// 	// 计算需要跳过的字节数
// 	if newPos == s.pos {
// 		return s.pos, nil
// 	}

// 	// 【关键】对于前向查找，我们可以直接移动底层流
// 	if newPos > s.pos {
// 		skipBytes := newPos - s.pos

// 		// 移动底层流
// 		currentVsPos, _ := s.src.Seek(0, io.SeekCurrent)
// 		_, err := s.src.Seek(currentVsPos+skipBytes, io.SeekStart)
// 		if err != nil {
// 			return s.pos, err
// 		}

// 		s.pos = newPos
// 		return s.pos, nil
// 	}

// 	// 【关键】对于后向查找，需要重置并重新开始
// 	// 这是CTR模式的限制
// 	if newPos < s.pos {
// 		// 重置状态
// 		s.encryptionHeaderSkipped = false
// 		s.pos = 0

// 		// 重置底层流
// 		_, err := s.src.Seek(0, io.SeekStart)
// 		if err != nil {
// 			return 0, err
// 		}

// 		// 然后前向查找到目标位置
// 		return s.Seek(newPos, io.SeekStart)
// 	}

// 	return s.pos, nil
// }

// func (s *SeekableCTRStreamFixed) getTotalDecryptedSize() (int64, error) {
// 	// 保存当前位置
// 	// currentPos := s.pos
// 	currentVsPos, _ := s.src.Seek(0, io.SeekCurrent)

// 	// 移动到末尾
// 	_, err := s.src.Seek(0, io.SeekEnd)
// 	if err != nil {
// 		return 0, err
// 	}

// 	endVsPos, _ := s.src.Seek(0, io.SeekCurrent)

// 	// 恢复位置
// 	s.src.Seek(currentVsPos, io.SeekStart)

// 	// 总解密大小 = 底层流大小 - 加密头部
// 	return endVsPos - s.encryptionHeaderSize, nil
// }

// // Close 实现 io.Closer 接口。
// func (s *SeekableCTRStreamFixed) Close() error {
// 	if closer, ok := s.src.(io.Closer); ok {
// 		return closer.Close()
// 	}
// 	return nil
// }
