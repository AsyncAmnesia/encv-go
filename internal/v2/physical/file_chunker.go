package physical

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/v2/container/block"
	"github.com/Soltus/encv-go/internal/v2/container/manifest"
	"github.com/Soltus/encv-go/internal/v2/namer"
	"github.com/Soltus/encv-go/internal/v2/types"
	"github.com/Soltus/encv-go/internal/v2/writer"
)

// FileChunkerPhysicalPacker 使用文件分片器进行物理打包
type FileChunkerPhysicalPacker struct {
	chunkSize int64
	namer     namer.ChunkNamer // 【关键修改】注入命名器
}

func NewFileChunkerPhysicalPacker(chunkSize int64, namer namer.ChunkNamer) *FileChunkerPhysicalPacker {
	return &FileChunkerPhysicalPacker{
		chunkSize: chunkSize,
		namer:     namer,
	}
}

// Pack 实现 PhysicalPacker 接口
func (p *FileChunkerPhysicalPacker) Pack(data io.Reader, manifest *types.Manifest_v2, outputDir, baseName string, namer namer.ChunkNamer, startIdx int) (string, error) {
	// 1. 准备主文件路径
	mainChunkPath := filepath.Join(outputDir, namer.GenerateMainChunkName(baseName))
	tempMainChunkPath := mainChunkPath + ".tmp"

	// 2. 创建主文件句柄
	mainFile, err := os.Create(tempMainChunkPath)
	if err != nil {
		return "", fmt.Errorf("failed to create main container file: %w", err)
	}
	defer func() {
		mainFile.Close()
		// 如果函数返回错误，确保删除临时文件
		if err != nil {
			os.Remove(tempMainChunkPath)
		}
	}()

	// 3. 【关键】创建全局哈希器和专用的分片写入工具
	globalHasher := crc32.NewIEEE()
	chunkedWriter := writer.NewChunkedContainerWriter(globalHasher)

	// 4. 循环处理所有数据分片
	buf := make([]byte, p.chunkSize)
	chunkIndex := startIdx
	var currentDataStreamOffset uint64 = 0

	for {
		n, err := io.ReadFull(data, buf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return "", fmt.Errorf("error reading data stream: %w", err)
		}
		if n == 0 {
			break
		}
		chunkData := buf[:n]

		// 更新 manifest 中对应 fragment 的元数据
		if chunkIndex < len(manifest.Fragments) {
			manifest.Fragments[chunkIndex].GlobalStartOffset = currentDataStreamOffset
		}

		if chunkIndex == startIdx {
			// 第一个数据块：写入主文件
			dataCRC32, err := chunkedWriter.WriteDataChunk(mainFile, chunkData)
			if err != nil {
				return "", fmt.Errorf("failed to write first fragment to main file: %w", err)
			}
			if chunkIndex < len(manifest.Fragments) {
				manifest.Fragments[chunkIndex].DataCRC32 = dataCRC32
			}
		} else {
			// 后续数据块：写入 .part 文件
			dataChunkFilename := namer.GenerateDataChunkName(baseName, chunkIndex)
			chunkPath := filepath.Join(outputDir, dataChunkFilename)

			if chunkIndex < len(manifest.Fragments) {
				manifest.Fragments[chunkIndex].PhysicalPath = dataChunkFilename
			}

			// 为 .part 文件创建一个临时的 writer
			partFile, err := os.Create(chunkPath)
			if err != nil {
				return "", fmt.Errorf("failed to create part file %s: %w", chunkPath, err)
			}

			dataCRC32, err := chunkedWriter.WriteDataChunk(partFile, chunkData)
			partFile.Close() // 立即关闭
			if err != nil {
				return "", fmt.Errorf("failed to write data chunk to part file %s: %w", chunkPath, err)
			}
			if chunkIndex < len(manifest.Fragments) {
				manifest.Fragments[chunkIndex].DataCRC32 = dataCRC32
			}
		}

		chunkIndex++
		currentDataStreamOffset += uint64(n)
	}

	// 5. 【关键】使用工具 writer 完成主文件的 Manifest 和 Footer 写入
	if err := chunkedWriter.WriteManifestAndFooter(mainFile, manifest); err != nil {
		return "", fmt.Errorf("failed to write manifest and footer: %w", err)
	}

	// 6. 显式关闭主文件，确保所有数据刷盘
	if err := mainFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close main file: %w", err)
	}

	// 7. 【原子操作】重命名临时主文件为最终文件名
	if err := os.Rename(tempMainChunkPath, mainChunkPath); err != nil {
		return "", fmt.Errorf("failed to atomically rename temp file to final file: %w", err)
	}

	fmt.Printf("✅ [FileChunkerPhysicalPacker] Packed to: %s\n", mainChunkPath)
	return mainChunkPath, nil
}

// FileChunkerPhysicalUnpacker 使用文件分片器进行物理解包
type FileChunkerPhysicalUnpacker struct {
	namer namer.ChunkNamer // 【关键修改】注入命名器
}

func NewFileChunkerPhysicalUnpacker(namer namer.ChunkNamer) *FileChunkerPhysicalUnpacker {
	return &FileChunkerPhysicalUnpacker{namer: namer}
}

// Unpack 实现 PhysicalUnpacker 接口
// 【关键修复】此函数现在使用统一的逻辑来重建任何类型的容器
func (u *FileChunkerPhysicalUnpacker) Unpack(mainContainerPath string) (string, func(), error) {
	// 1. 准备工作：解析名称，创建临时文件
	baseName, err := u.namer.ParseFirstChunkName(mainContainerPath)
	if err != nil {
		// 如果解析失败，说明它可能不是标准分片文件，我们使用文件本身作为 baseName
		baseName = mainContainerPath
	}

	tempFile, err := os.CreateTemp(filepath.Dir(baseName), filepath.Base(baseName)+"_unified_*.tmp")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	unifiedPath := tempFile.Name()

	cleanup := func() {
		fmt.Printf("DEBUG: [Unpack] Cleaning up temp file: %s\n", unifiedPath)
		tempFile.Close()
		os.Remove(unifiedPath)
	}

	// 2. 发现：找到并解析 Manifest
	originalManifest, err := u.findAndParseManifest(mainContainerPath)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to find and parse manifest: %w", err)
	}

	// 3. 【核心】统一的重建逻辑：根据原始 Manifest 重建一个完整的、连续的容器
	newManifestBytes, err := u.rebuildToSingleFile(mainContainerPath, originalManifest, tempFile)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to rebuild container: %w", err)
	}

	// 4. 定稿：写入新的 Footer
	if err := u.writeFinalFooter(tempFile, newManifestBytes); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to write final footer: %w", err)
	}

	// 成功，关闭文件并返回清理函数
	tempFile.Close()
	fmt.Printf("DEBUG: [Unpacker] Successfully unified container to: %s\n", unifiedPath)
	return unifiedPath, cleanup, nil
}

// findAndParseManifest 封装了查找和解析 Manifest 的逻辑
func (u *FileChunkerPhysicalUnpacker) findAndParseManifest(mainContainerPath string) (*types.Manifest_v2, error) {
	// 尝试从 Footer 读取
	if footer, err := readFooter(mainContainerPath); err == nil {
		manifestBytes, err := manifest.ReadManifestAt(mainContainerPath, int64(footer.ManifestOffset), int64(footer.ManifestLength))
		if err == nil {
			var manifest types.Manifest_v2
			if err := json.Unmarshal(manifestBytes, &manifest); err == nil {
				return &manifest, nil
			}
		}
	}
	// 备用方案：线性扫描
	manifestBytes, _, err := extractManifestWithScan(mainContainerPath)
	if err != nil {
		return nil, err
	}
	var manifest types.Manifest_v2
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// rebuildToSingleFile 【核心修复】统一的重建函数
// 它遍历原始 Manifest 的所有 fragments，将它们的数据从各自的物理位置读取出来，
// 连续地写入到目标文件，并生成一个描述新布局的、正确的 Manifest。
func (u *FileChunkerPhysicalUnpacker) rebuildToSingleFile(sourcePath string, originalManifest *types.Manifest_v2, destFile *os.File) ([]byte, error) {
	containerDir := filepath.Dir(sourcePath)

	// 1. 创建一个新的 Manifest，复制 KVI 等元数据
	newManifest := &types.Manifest_v2{
		Version:    originalManifest.Version,
		KVI:        originalManifest.KVI,
		Redundancy: originalManifest.Redundancy,
	}

	var dataStreamOffset uint64 = 0

	// 2. 遍历原始 Manifest 的 fragments
	for _, frag := range originalManifest.Fragments {
		if frag.Type != types.FragmentType_SeekableStream {
			continue
		}

		// a. 确定 fragment 的物理文件路径
		var physicalPath string
		if frag.PhysicalPath == "" {
			physicalPath = sourcePath // 在主文件中
		} else {
			physicalPath = filepath.Join(containerDir, frag.PhysicalPath) // 在 .part 文件中
		}

		// b. 打开物理文件并读取数据块
		physicalFile, err := os.Open(physicalPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open physical file '%s' for fragment '%s': %w", physicalPath, frag.ID, err)
		}

		// c. 读取并验证块头
		var header block.BlockHeader_v2
		if err := binary.Read(physicalFile, binary.LittleEndian, &header); err != nil {
			physicalFile.Close()
			return nil, fmt.Errorf("failed to read block header for fragment '%s': %w", frag.ID, err)
		}

		// d. 读取块数据
		chunkData := make([]byte, header.Length)
		if _, err := io.ReadFull(physicalFile, chunkData); err != nil {
			physicalFile.Close()
			return nil, fmt.Errorf("failed to read data for fragment '%s': %w", frag.ID, err)
		}
		physicalFile.Close()

		// e. 【关键】将裸数据作为一个完整的 Data Block 写入到目标文件
		if err := block.WriteBlock_v2(destFile, types.BlockTypeData_v2, chunkData); err != nil {
			return nil, fmt.Errorf("failed to write data block for fragment '%s': %w", frag.ID, err)
		}

		// f. 为新的 Manifest 创建一个描述连续布局的 fragment
		newFrag := types.Fragment_v2{
			ID:                frag.ID,
			Type:              frag.Type,
			Length:            header.Length, // 使用从块头读取的实际长度
			GlobalStartOffset: dataStreamOffset,
			DataCRC32:         header.CRC32, // 使用从块头读取的实际 CRC
		}
		newManifest.Fragments = append(newManifest.Fragments, newFrag)

		// g. 更新数据流偏移量
		dataStreamOffset += header.Length
	}

	// 3. 将新创建的 Manifest 序列化为 JSON
	newManifestBytes, err := json.Marshal(newManifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new manifest: %w", err)
	}

	// 4. 将新的 Manifest 块写入到临时文件末尾
	if err := block.WriteBlock_v2(destFile, types.BlockTypeManifest_v2, newManifestBytes); err != nil {
		return nil, fmt.Errorf("failed to write new manifest block to unified file: %w", err)
	}

	return newManifestBytes, nil
}

// writeFinalFooter 计算并写入最终的 Footer
func (u *FileChunkerPhysicalUnpacker) writeFinalFooter(file *os.File, manifestBytes []byte) error {
	currentEOF, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get current file size: %w", err)
	}

	manifestBlockSize := int64(binary.Size(block.BlockHeader_v2{})) + int64(len(manifestBytes))
	newManifestOffset := currentEOF - manifestBlockSize

	footer := &types.EnvelopeFooter_v2{
		ManifestOffset: uint64(newManifestOffset),
		ManifestLength: uint64(len(manifestBytes)),
	}
	copy(footer.Magic[:], types.MagicFooter_v2[:])

	return binary.Write(file, binary.LittleEndian, footer)
}

// --- 以下是之前已经验证过的辅助函数 ---

// readFooter 是一个辅助函数，用于从文件末尾读取并验证 Footer
func readFooter(filePath string) (*types.EnvelopeFooter_v2, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	footer := &types.EnvelopeFooter_v2{}
	footerSize := int64(binary.Size(footer))
	footerReader := io.NewSectionReader(file, fileInfo.Size()-footerSize, footerSize)

	err = binary.Read(footerReader, types.ByteOrder_v2, footer)
	if err != nil {
		return nil, fmt.Errorf("failed to read footer bytes from file '%s': %w", filePath, err)
	}

	if !bytes.Equal(footer.Magic[:], types.MagicFooter_v2[:]) {
		return nil, fmt.Errorf("file '%s' is not a valid ENCV container (footer magic mismatch at end of file)", filePath)
	}

	return footer, nil
}

// extractManifestWithScan 复制 manifest-v2 的线性扫描逻辑
func extractManifestWithScan(filePath string) ([]byte, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open file '%s': %w", filePath, err)
	}
	defer file.Close()

	fmt.Printf("INFO: [Unpacker] Scanning '%s' for Manifest block...\n", filePath)

	currentOffset := int64(0)
	for {
		header, err := block.ReadBlockHeader_v2(file)
		if err != nil {
			if err == io.EOF {
				return nil, 0, fmt.Errorf("reached end of file but Manifest block was not found in '%s'", filePath)
			}
			return nil, 0, fmt.Errorf("failed to read block header: %w", err)
		}

		if header.Type == types.BlockTypeManifest_v2 {
			fmt.Printf("DEBUG: [Unpacker] Found Manifest block at offset %d.\n", currentOffset)
			manifestData, err := block.ReadBlockData_v2(file, header)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to read Manifest block data: %w", err)
			}
			return manifestData, currentOffset, nil
		}

		_, err = file.Seek(int64(header.Length), io.SeekCurrent)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to seek past block data: %w", err)
		}
		currentOffset += block.GetBlockHeader_v2_Size() + int64(header.Length)
	}
}
