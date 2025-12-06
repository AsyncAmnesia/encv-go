package chunked

import (
	"bytes"
	"crypto/aes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// SubChunkInfo 存储了子分片的元数据，用于快速索引
type SubChunkInfo struct {
	Path     string
	Size     int64
	Header   int64 // 容器头部大小（KVI相关部分）
	DataSize int64 // 纯净加密数据的大小（跳过加密头部后）
}

// VirtualSeeker 是一个 io.ReadSeeker 的实现，它将多个分片文件虚拟成一个连续的可寻址流。
type VirtualSeeker struct {
	dirPath     string
	subMagic    string
	chunks      []SubChunkInfo
	totalSize   int64
	currentIdx  int
	currentFile *os.File
	pos         int64 // 当前的虚拟流位置
}

// NewVirtualSeeker 创建一个新的 VirtualSeeker，复用 GetKVIDataOnly 的逻辑来计算偏移
func NewVirtualSeeker(dirPath, mainChunkPath, mainMagic, subMagic string) (*VirtualSeeker, error) {
	log.Printf("NewVirtualSeeker -> mainChunkPath=%s", mainChunkPath)

	// 1. 使用已验证的函数获取KVI数据
	kviData, err := GetKVIDataOnly(mainChunkPath, mainMagic)
	if err != nil {
		// 如果失败，尝试两种内部实现的后备方案
		kviData, err = GetKVIDataOnly_SingleContainer(mainChunkPath, mainMagic)
		if err != nil {
			kviData, err = GetKVIDataOnly_ChunkedContainer(mainChunkPath, mainMagic)
			if err != nil {
				return nil, fmt.Errorf("failed to get KVI data using all methods: %w", err)
			}
		}
	}
	log.Printf("DEBUG VirtualSeeker: Got KVI data via GetKVIDataOnly, length: %d bytes", len(kviData))

	// 2. 打开主分片文件，通过两次定位操作来“测量”总头部大小
	mainFile, err := os.Open(mainChunkPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open main chunk: %w", err)
	}
	defer mainFile.Close()

	// 第一步：通过 GetKVIDataOnly 内部的逻辑，让文件指针停留在 KVI数据之后。
	// 我们需要模拟这个读取过程，或者更简单：重新调用一遍，但记录下结束位置。
	// 注意：GetKVIDataOnly 读取后，文件指针应位于“加密数据”的起始处。
	// 我们通过重新打开文件并调用一个能返回结束位置的函数来实现。

	// 为简化，这里我们直接重新计算：打开文件，按 GetKVIDataOnly 逻辑读取，并记录最终位置。
	calcFile, err := os.Open(mainChunkPath)
	if err != nil {
		return nil, fmt.Errorf("failed to reopen for offset calculation: %w", err)
	}
	// 复用 GetKVIDataOnly 的核心步骤，但仅用于定位
	magicBytes := []byte(mainMagic)
	magicBuf := make([]byte, len(magicBytes))
	if _, err := io.ReadFull(calcFile, magicBuf); err != nil {
		calcFile.Close()
		return nil, fmt.Errorf("calc: failed to read magic: %w", err)
	}
	if !bytes.Equal(magicBuf, magicBytes) {
		calcFile.Close()
		return nil, fmt.Errorf("calc: magic mismatch")
	}
	var storedKviLen uint64
	if err := binary.Read(calcFile, binary.LittleEndian, &storedKviLen); err != nil {
		calcFile.Close()
		return nil, fmt.Errorf("calc: failed to read KVI length: %w", err)
	}
	// 跳过KVI数据
	if _, err := calcFile.Seek(int64(storedKviLen), io.SeekCurrent); err != nil {
		calcFile.Close()
		return nil, fmt.Errorf("calc: failed to skip KVI data: %w", err)
	}
	// **现在 calcFile 的指针位置就是加密数据的起始位置**
	encryptedDataStart, err := calcFile.Seek(0, io.SeekCurrent)
	calcFile.Close()
	if err != nil {
		return nil, fmt.Errorf("calc: failed to get data start position: %w", err)
	}
	log.Printf("DEBUG VirtualSeeker: Calculated encrypted data start at: %d", encryptedDataStart)

	// 3. 获取文件总大小，计算加密数据大小
	mainFileInfo, err := mainFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat main chunk file: %w", err)
	}
	encryptedDataSize := mainFileInfo.Size() - encryptedDataStart

	log.Printf("DEBUG VirtualSeeker: Main chunk - header=%d, dataStart=%d, dataSize=%d, fileSize=%d",
		encryptedDataStart, encryptedDataStart, encryptedDataSize, mainFileInfo.Size())

	// 4. 解析KVI数据（使用已经获取到的kviData）
	index, err := utils.UnmarshalKVI(kviData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal KVI: %w", err)
	}
	videoIndex, ok := index.(*types.VideoIndex)
	if !ok {
		return nil, fmt.Errorf("KVI is not a VideoIndex")
	}

	// 5. 构建分片元数据列表
	chunks := []SubChunkInfo{
		{
			Path:     mainChunkPath,
			Size:     mainFileInfo.Size(),
			Header:   encryptedDataStart, // 加密数据开始位置
			DataSize: encryptedDataSize,  // 加密数据大小
		},
	}

	// 6. 处理子分片（使用简化的动态探测逻辑）
	for _, sub := range videoIndex.SubChunks {
		subPath := filepath.Join(dirPath, sub.Filename)
		subHeaderSize, subDataSize, err := probeSubChunkHeader(subPath, subMagic)
		if err != nil {
			return nil, fmt.Errorf("failed to probe sub-chunk %s: %w", subPath, err)
		}
		chunks = append(chunks, SubChunkInfo{
			Path:     subPath,
			Size:     subHeaderSize + subDataSize,
			Header:   subHeaderSize,
			DataSize: subDataSize,
		})
		log.Printf("DEBUG VirtualSeeker: Sub-chunk %s - header=%d, dataSize=%d",
			filepath.Base(subPath), subHeaderSize, subDataSize)
	}

	// 7. 计算总大小并验证（后续代码保持不变）
	var totalSize int64
	for _, c := range chunks {
		totalSize += c.DataSize
	}
	log.Printf("DEBUG VirtualSeeker: Final calculated totalSize = %d, chunks count = %d", totalSize, len(chunks))
	for i, c := range chunks {
		log.Printf("DEBUG VirtualSeeker: Chunk %d (%s): Header=%d, DataSize=%d",
			i, filepath.Base(c.Path), c.Header, c.DataSize)
	}

	// 验证总大小
	originalSize := videoIndex.GetOriginalFileSize()
	if totalSize != originalSize {
		log.Printf("WARN VirtualSeeker: Size mismatch! Virtual=%d, Original=%d, Diff=%d",
			totalSize, originalSize, totalSize-originalSize)
		// 检查差异是否在合理范围内（加密填充通常小于16字节）
		diff := totalSize - originalSize
		if diff < 0 || diff > aes.BlockSize {
			log.Printf("ERROR VirtualSeeker: Size mismatch too large, but continuing anyway")
		}
	}
	// ... [验证总大小等后续逻辑] ...
	return &VirtualSeeker{
		dirPath:    dirPath,
		subMagic:   subMagic,
		chunks:     chunks,
		totalSize:  totalSize,
		currentIdx: -1,
		pos:        0,
	}, nil
}

// probeSubChunkHeader 动态探测子分片的头部大小和数据大小
func probeSubChunkHeader(subPath, expectedSubMagic string) (headerSize, dataSize int64, err error) {
	file, err := os.Open(subPath)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	// 尝试读取魔法数字
	magicBytes := []byte(expectedSubMagic)
	magicBuf := make([]byte, len(magicBytes))
	n, err := io.ReadFull(file, magicBuf)
	currentPos := int64(n)

	if err == nil && bytes.Equal(magicBuf, magicBytes) {
		// 有魔法数字，尝试读取KVI长度
		var kviLen uint64
		if err := binary.Read(file, binary.LittleEndian, &kviLen); err == nil {
			currentPos += 8 + int64(kviLen)
			// 跳过KVI数据
			if _, err := file.Seek(int64(kviLen), io.SeekCurrent); err != nil {
				// 跳过失败，我们仍使用当前计算的位置
			}
		}
		// 注意：子分片没有 crypto.MagicNumber 加密头部
	} else {
		// 没有识别到魔法数字，假定头部为0
		currentPos = 0
		file.Seek(0, io.SeekStart) // 重置指针
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return 0, 0, err
	}
	headerSize = currentPos
	dataSize = fileInfo.Size() - currentPos
	return headerSize, dataSize, nil
}

func debugFileHeader(filePath string, magic string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 读取前100字节
	buf := make([]byte, 100)
	n, err := io.ReadFull(file, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return err
	}

	fmt.Printf("DEBUG %s first %d bytes:\n", filepath.Base(filePath), n)
	for i := 0; i < n; i += 16 {
		end := i + 16
		if end > n {
			end = n
		}
		fmt.Printf("  %04x: % x\n", i, buf[i:end])
	}

	// 验证魔法数字
	if len(magic) <= n && string(buf[:len(magic)]) == magic {
		fmt.Printf("  Magic '%s' verified at position 0\n", magic)
	}

	// 尝试读取KVI长度
	if len(magic)+8 <= n {
		kviLenBytes := buf[len(magic) : len(magic)+8]
		var kviLen uint64
		buf := bytes.NewReader(kviLenBytes)
		if err := binary.Read(buf, binary.LittleEndian, &kviLen); err == nil {
			fmt.Printf("  KVI length (LittleEndian): %d (0x%x)\n", kviLen, kviLen)
		}
		// 尝试大端序
		buf = bytes.NewReader(kviLenBytes)
		if err := binary.Read(buf, binary.BigEndian, &kviLen); err == nil {
			fmt.Printf("  KVI length (BigEndian): %d (0x%x)\n", kviLen, kviLen)
		}
	}

	return nil
}

// 【关键修复】修复Seek方法的边界判断逻辑
func (vs *VirtualSeeker) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = vs.pos + offset
	case io.SeekEnd:
		newPos = vs.totalSize + offset
	default:
		return 0, fmt.Errorf("invalid whence value: %d", whence)
	}

	if newPos < 0 || newPos > vs.totalSize {
		return 0, fmt.Errorf("seek position out of bounds")
	}

	// 查找目标分片
	accumulated := int64(0)
	targetIdx := -1
	var relativePos int64

	// 【修复】边界条件：使用 <= 而不是 < 来处理正好在分片末尾的情况
	for i, chunk := range vs.chunks {
		chunkEnd := accumulated + chunk.DataSize

		// 当 newPos 在 [accumulated, chunkEnd) 区间内时，属于这个分片
		// 特别注意：当 newPos == chunkEnd 时，属于下一个分片的开始（索引为0）
		if newPos >= accumulated && (i == len(vs.chunks)-1 || newPos < chunkEnd) {
			targetIdx = i
			relativePos = newPos - accumulated
			break
		}
		accumulated = chunkEnd // 更新为当前分片的结束位置
	}

	if targetIdx == -1 {
		// 如果没找到，应该是最后一个分片的末尾位置
		targetIdx = len(vs.chunks) - 1
		relativePos = vs.chunks[targetIdx].DataSize
	}

	// 如果切换分片，关闭当前文件
	if vs.currentIdx != targetIdx {
		if vs.currentFile != nil {
			vs.currentFile.Close()
			vs.currentFile = nil
		}
		vs.currentIdx = targetIdx
	}

	// 打开目标分片文件（如果需要）
	if vs.currentFile == nil && targetIdx >= 0 && targetIdx < len(vs.chunks) {
		file, err := os.Open(vs.chunks[targetIdx].Path)
		if err != nil {
			return 0, fmt.Errorf("failed to open chunk %s: %w", vs.chunks[targetIdx].Path, err)
		}
		vs.currentFile = file
	}

	if vs.currentFile != nil {
		// 计算物理文件中的位置：头部偏移 + 相对位置
		physicalPos := vs.chunks[targetIdx].Header + relativePos
		_, err := vs.currentFile.Seek(physicalPos, io.SeekStart)
		if err != nil {
			return 0, fmt.Errorf("failed to seek within chunk %s: %w", vs.chunks[targetIdx].Path, err)
		}
	}

	vs.pos = newPos
	log.Printf("DEBUG VirtualSeeker.Seek: pos=%d->%d, chunk=%d, relative=%d", vs.pos, newPos, targetIdx, relativePos)
	return newPos, nil
}

// Read 实现了 io.Reader 接口
func (vs *VirtualSeeker) Read(p []byte) (n int, err error) {
	if vs.pos >= vs.totalSize {
		return 0, io.EOF
	}

	// 初始化：如果还没打开任何文件，先定位到当前位置
	if vs.currentIdx == -1 {
		_, err := vs.Seek(vs.pos, io.SeekStart)
		if err != nil {
			return 0, err
		}
	}

	totalRead := 0
	for totalRead < len(p) && vs.pos < vs.totalSize {
		// 确保当前文件已打开
		if vs.currentFile == nil {
			if vs.currentIdx < 0 || vs.currentIdx >= len(vs.chunks) {
				return totalRead, fmt.Errorf("invalid chunk index: %d", vs.currentIdx)
			}

			file, err := os.Open(vs.chunks[vs.currentIdx].Path)
			if err != nil {
				return totalRead, fmt.Errorf("failed to open chunk %s: %w", vs.chunks[vs.currentIdx].Path, err)
			}
			vs.currentFile = file

			// 定位到当前分片的正确位置
			currentChunkStart := vs.getChunkStart(vs.currentIdx)
			relativePos := vs.pos - currentChunkStart
			physicalPos := vs.chunks[vs.currentIdx].Header + relativePos
			_, err = vs.currentFile.Seek(physicalPos, io.SeekStart)
			if err != nil {
				vs.currentFile.Close()
				vs.currentFile = nil
				return totalRead, fmt.Errorf("failed to seek in chunk %s: %w", vs.chunks[vs.currentIdx].Path, err)
			}
		}

		// 从当前文件读取
		nRead, errRead := vs.currentFile.Read(p[totalRead:])
		totalRead += nRead
		vs.pos += int64(nRead)

		// 处理读取错误或文件结束
		if errRead != nil {
			if errRead == io.EOF {
				// 当前分片读完，切换到下一个分片
				vs.currentFile.Close()
				vs.currentFile = nil
				vs.currentIdx++

				if vs.currentIdx >= len(vs.chunks) {
					// 所有分片都已读完
					return totalRead, io.EOF
				}

				// 继续循环，处理下一个分片
				continue
			}
			return totalRead, errRead
		}
	}

	return totalRead, nil
}

// getChunkStart 计算指定分片在虚拟流中的起始位置
func (vs *VirtualSeeker) getChunkStart(chunkIdx int) int64 {
	var start int64
	for i := 0; i < chunkIdx && i < len(vs.chunks); i++ {
		start += vs.chunks[i].DataSize
	}
	return start
}

// Close 实现了 io.Closer 接口
func (vs *VirtualSeeker) Close() error {
	if vs.currentFile != nil {
		return vs.currentFile.Close()
	}
	return nil
}

// CreateSeekablePackedData 创建一个包含可寻址数据流的 PackedData
func CreateSeekablePackedData(mainChunkPath, mainMagic, subMagic string) (*container.PackedData, error) {
	dirPath := filepath.Dir(mainChunkPath)
	virtualSeeker, err := NewVirtualSeeker(dirPath, mainChunkPath, mainMagic, subMagic)
	if err != nil {
		return nil, fmt.Errorf("[CreateSeekablePackedData] failed to create virtual seeker: %w", err)
	}

	kviData, err := GetKVIDataOnly(mainChunkPath, mainMagic)
	if err != nil {
		virtualSeeker.Close()
		return nil, fmt.Errorf("failed to get KVI data: %w", err)
	}

	return &container.PackedData{
		KVIData:    kviData,
		DataStream: virtualSeeker,
	}, nil
}

func GetKVIDataOnly(mainChunkPath, mainMagic string) ([]byte, error) {
	bb, err := GetKVIDataOnly_ChunkedContainer(mainChunkPath, mainMagic)
	if err != nil {
		return GetKVIDataOnly_SingleContainer(mainChunkPath, mainMagic)
	}
	return bb, nil
}

func GetKVIDataOnly_SingleContainer(mainChunkPath, mainMagic string) ([]byte, error) {
	file, err := os.Open(mainChunkPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// 1. 读取并验证魔法数字
	magicBytes := []byte(mainMagic)
	magicBuf := make([]byte, len(magicBytes))
	if _, err := io.ReadFull(file, magicBuf); err != nil {
		return nil, fmt.Errorf("failed to read magic: %w", err)
	}
	if !bytes.Equal(magicBuf, magicBytes) {
		return nil, fmt.Errorf("magic mismatch. expected %x, got %x", magicBytes, magicBuf)
	}

	// 2. 直接读取KVI长度 (uint64, LittleEndian)
	var storedKviLen uint64
	// 必须使用 binary.Read 来正确解码二进制格式的 uint64
	if err := binary.Read(file, binary.LittleEndian, &storedKviLen); err != nil {
		return nil, fmt.Errorf("failed to read KVI length from binary: %w", err)
	}

	// 3. 严格的长度校验
	// 3.1 基础合理性检查
	const maxReasonableKVISize = 10 * 1024 * 1024 // 10MB
	if storedKviLen > maxReasonableKVISize {
		return nil, fmt.Errorf("KVI length %d exceeds maximum reasonable size %d", storedKviLen, maxReasonableKVISize)
	}

	// 3.2 检查文件剩余大小
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("failed to get file pos: %w", err)
	}
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	remainingBytes := fileInfo.Size() - currentPos
	if int64(storedKviLen) > remainingBytes {
		// 此处是关键差异：不再信任header.KVILength，而是信任从二进制直接读取的storedKviLen
		return nil, fmt.Errorf("KVI length %d exceeds remaining file size %d. Possible header corruption.", storedKviLen, remainingBytes)
	}

	// 4. 读取KVI数据
	kviData := make([]byte, storedKviLen)
	if _, err := io.ReadFull(file, kviData); err != nil {
		return nil, fmt.Errorf("failed to read KVI data (len=%d): %w", storedKviLen, err)
	}

	// 5. （可选）移除前缀以保持兼容
	var finalKviData []byte
	const kviPrefixSize = 1 + 4 + 4 // 9 bytes: '(' + uint32 + uint32
	if len(kviData) >= kviPrefixSize && kviData[0] == '(' {
		finalKviData = kviData[kviPrefixSize:]
	} else {
		finalKviData = kviData
	}

	// 调试：记录成功读取的信息
	// log.Printf("DEBUG GetKVIDataOnly: magic=%s, readKviLen=%d, remainingFile=%d, kviDataLen=%d, finalKviDataLen=%d",
	//     mainMagic, storedKviLen, remainingBytes, len(kviData), len(finalKviData))
	return finalKviData, nil
}

func GetKVIDataOnly_ChunkedContainer(mainChunkPath, mainMagic string) ([]byte, error) {
	file, err := os.Open(mainChunkPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	header, err := ReadMainHeader(file, mainMagic)
	if err != nil {
		return nil, err
	}

	// 验证 KVILength 的合理性
	if header.KVILength < 0 {
		return nil, fmt.Errorf("invalid KVI length: %d", header.KVILength)
	}

	// 检查文件剩余大小是否足够
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("failed to get current file position: %w", err)
	}

	remainingBytes := fileInfo.Size() - currentPos
	if int64(header.KVILength) > remainingBytes {
		return nil, fmt.Errorf("KVI length %d exceeds remaining file size %d",
			header.KVILength, remainingBytes)
	}

	// 限制最大读取大小，防止内存溢出
	const maxReasonableSize = 100 * 1024 * 1024 // 100MB
	if header.KVILength > maxReasonableSize {
		return nil, fmt.Errorf("KVI length %d exceeds maximum reasonable size",
			header.KVILength)
	}

	// 安全分配内存
	kviDataWithPrefix := make([]byte, header.KVILength)
	if _, err := io.ReadFull(file, kviDataWithPrefix); err != nil {
		return nil, err
	}

	var finalKviData []byte
	const kviPrefixSize = 1 + 4 + 4
	if len(kviDataWithPrefix) >= kviPrefixSize && kviDataWithPrefix[0] == '(' {
		finalKviData = kviDataWithPrefix[kviPrefixSize:]
	} else {
		finalKviData = kviDataWithPrefix
	}
	return finalKviData, nil
}

// 动态计算 baseOffset
func CalculateBaseOffset(mainChunkPath string) (int64, error) {
	// 打开主分片文件
	file, err := os.Open(mainChunkPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// 读取主分片头部
	var header MainChunkHeader
	if err := binary.Read(file, binary.LittleEndian, &header); err != nil {
		return 0, err
	}

	// baseOffset 是主分片中，数据区开始的位置 = 文件头大小 + KVI 数据大小
	baseOffset := int64(MainChunkHeaderSize) + int64(header.KVILength)

	log.Printf("Calculated base offset: MainChunkHeaderSize(%d) + KVILength(%d) = %d",
		MainChunkHeaderSize, header.KVILength, baseOffset)

	return baseOffset, nil
}
