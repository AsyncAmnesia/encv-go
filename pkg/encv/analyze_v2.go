// pkg/encv/analyze_v2.go

package encv

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/Soltus/encv-go/internal/v2/chunker"
	"github.com/Soltus/encv-go/internal/v2/container/block"
	"github.com/Soltus/encv-go/internal/v2/container/manifest"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// AnalyzeContainerV2 可视化分析 v2 容器文件的结构
func AnalyzeContainerV2(ctx context.Context, containerPath string) error {
	absPath, err := filepath.Abs(containerPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	file, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("failed to open file '%s': %w", absPath, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := stat.Size()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Printf("--- ENCV Container Analysis: %s (Size: %d bytes) ---\n\n", filepath.Base(absPath), fileSize)

	// --- 1. Footer Analysis ---
	fmt.Println(">>> 1. Footer Analysis (from end of file)")
	footer, footerErr := manifest.ReadFooterFromFile(absPath)
	if footerErr != nil {
		fmt.Printf("  Status: FAILED\n  Reason: %v\n\n", footerErr)
	} else {
		fmt.Fprintf(w, "  Status:\tOK\n")
		fmt.Fprintf(w, "  Magic:\t%s\n", string(footer.Magic[:]))
		fmt.Fprintf(w, "  Manifest Offset:\t%d\n", footer.ManifestOffset)
		fmt.Fprintf(w, "  Manifest Length:\t%d\n", footer.ManifestLength)
		fmt.Fprintf(w, "  Manifest CRC32:\t%08x\n", footer.ManifestCRC32)
		fmt.Fprintf(w, "  Global CRC32:\t%08x\n", footer.GlobalCRC32)
		w.Flush()
		fmt.Println()
	}

	// --- 2. Manifest Analysis ---
	fmt.Println(">>> 2. Manifest Analysis")
	var manifestBytes []byte
	var manifestSource string
	var readErr error

	// 【关键修复】封装正确的 Manifest 读取逻辑
	readManifestFromFooter := func(f *os.File, footer *types.EnvelopeFooter_v2) ([]byte, error) {
		_, err := f.Seek(int64(footer.ManifestOffset), io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("failed to seek to manifest offset: %w", err)
		}
		// 【关键】使用正确的块读取函数
		header, err := block.ReadBlockHeader_v2(f)
		if err != nil {
			return nil, fmt.Errorf("failed to read manifest block header: %w", err)
		}
		if header.Type != types.BlockTypeManifest_v2 {
			return nil, fmt.Errorf("block at footer-specified offset is not a manifest")
		}
		return block.ReadBlockData_v2(f, header)
	}

	if footerErr == nil {
		// Footer 有效，尝试通过 Footer 读取
		manifestBytes, readErr = readManifestFromFooter(file, footer)
		if readErr != nil {
			fmt.Printf("  WARN: Failed to read manifest via Footer (%v). Falling back to scan.\n", readErr)
		} else {
			manifestSource = "From Footer"
		}
	}

	// 如果通过 Footer 读取失败，或者 Footer 本身无效，则降级到扫描
	if manifestBytes == nil {
		fmt.Printf("  INFO: Scanning for manifest block from file start...\n")
		manifestBytes, readErr = manifest.ExtractManifest_v2(absPath)
		if readErr != nil {
			// 连扫描都失败了，这是一个致命错误，无法继续
			return fmt.Errorf("failed to read manifest via both Footer and scanning: %w", readErr)
		}
		manifestSource = "Scanned"
	}

	fmt.Printf("  Source:\t%s\n", manifestSource)
	fmt.Printf("  Raw Size:\t%d bytes\n\n", len(manifestBytes))

	// --- 3. Manifest JSON Parsing ---
	fmt.Printf("  Content (pretty-printed):\n")
	var manifestObj types.Manifest_v2
	var jsonParseErr error
	if jsonParseErr = json.Unmarshal(manifestBytes, &manifestObj); jsonParseErr != nil {
		fmt.Printf("    Status: FAILED to unmarshal manifest JSON: %v\n", jsonParseErr)
		fmt.Printf("    Raw data (first 128 bytes): %q\n", string(manifestBytes[:min(128, len(manifestBytes))]))
	} else {
		prettyJSON, _ := json.MarshalIndent(manifestObj, "    ", "  ")
		fmt.Printf("    %s\n", string(prettyJSON))
	}
	fmt.Println()

	// --- 4. Orchestrate Scanning and Validation ---
	// 【关键重构】调用辅助函数
	scannedBlocks, scanErr := performPhysicalLayoutScan(file, manifestObj, fileSize, w)
	if scanErr != nil {
		// 扫描本身失败，这是一个严重错误
		return fmt.Errorf("physical layout scan failed: %w", scanErr)
	}

	// 只有当 JSON 解析成功时，才进行交叉验证
	if jsonParseErr == nil {
		performCrossValidation(footer, scannedBlocks, manifestObj, w)
	} else {
		fmt.Println(">>> 4. Cross-Validation Report")
		fmt.Println("  [INFO] Manifest parsing failed, skipping cross-validation.")
	}

	return nil
}

// --- 辅助函数 ---

// scannedBlock 用于记录扫描到的块信息
type scannedBlock struct {
	offset int64
	header *block.BlockHeader_v2
	crc    uint32
}

// performPhysicalLayoutScan 执行物理块的扫描，并返回扫描结果
func performPhysicalLayoutScan(file *os.File, manifestObj types.Manifest_v2, fileSize int64, w *tabwriter.Writer) ([]scannedBlock, error) {
	fmt.Println(">>> 3. Physical Layout Scan (Block-by-Block)")

	// 如果 Manifest 解析失败，默认为单文件容器
	isChunked := manifestObj.Version != 0 && !chunker.IsSingleFileContainer(&manifestObj)

	if isChunked {
		fmt.Println("  [INFO] Detected a physical chunked container. Analyzing main file layout.")
	} else {
		fmt.Println("  [INFO] Detected a single-file container. Scanning for all blocks.")
	}

	fmt.Fprintln(w, "  Offset\t\tType\t\tLength\tCRC32")
	fmt.Fprintln(w, "  ------\t\t----\t\t------\t------")

	var scannedBlocks []scannedBlock

	if isChunked {
		// --- 模式 1: 物理分片容器 ---
		var firstChunkLength uint64
		var firstChunkManifestCRC uint32
		for _, frag := range manifestObj.Fragments {
			if frag.ID == "video_chunk_0" {
				firstChunkLength = frag.Length
				firstChunkManifestCRC = frag.DataCRC32
				break
			}
		}
		if firstChunkLength == 0 {
			return nil, fmt.Errorf("could not find length for video_chunk_0 in manifest")
		}

		crc, err := streamCRC32(file, firstChunkLength)
		if err != nil {
			return nil, fmt.Errorf("failed to stream CRC for first chunk: %w", err)
		}
		if crc != firstChunkManifestCRC {
			fmt.Printf("  [WARN] CRC mismatch for video_chunk_0! Calculated: %08x, Manifest: %08x\n", crc, firstChunkManifestCRC)
		}

		scannedBlocks = append(scannedBlocks, scannedBlock{
			offset: 0,
			header: &block.BlockHeader_v2{Type: types.BlockTypeData_v2, Length: firstChunkLength},
			crc:    crc,
		})
		fmt.Fprintf(w, "  0\t\t%s\t\t%d\t%08x (raw stream)\n", getBlockTypeName(uint32(types.BlockTypeData_v2)), firstChunkLength, crc)

		// 复用已验证的逻辑，从头开始扫描并定位 Manifest
		fmt.Println("  [INFO] Re-scanning for manifest block from file start (reliable method)...")
		manifestOffset, manifestBytes, err := scanForManifestWithOffset(file)
		if err != nil {
			return nil, fmt.Errorf("failed to locate manifest in main file using reliable scan: %w", err)
		}

		manifestScannedBlock := scannedBlock{
			offset: manifestOffset,
			header: &block.BlockHeader_v2{Type: types.BlockTypeManifest_v2, Length: uint64(len(manifestBytes))},
			crc:    crc32.ChecksumIEEE(manifestBytes),
		}
		scannedBlocks = append(scannedBlocks, manifestScannedBlock)
		fmt.Fprintf(w, "  %d\t\t%s\t\t%d\t%08x\n", manifestScannedBlock.offset, getBlockTypeName(uint32(manifestScannedBlock.header.Type)), manifestScannedBlock.header.Length, manifestScannedBlock.crc)

	} else {
		// --- 模式 2: 单文件容器 ---
		fmt.Println("  [INFO] Scanning for all blocks with streaming CRC calculation.")
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek to start for layout scan: %w", err)
		}

		for {
			currentOffset, err := file.Seek(0, io.SeekCurrent)
			if err != nil {
				return nil, fmt.Errorf("failed to get current offset: %w", err)
			}
			if currentOffset >= fileSize {
				break
			}

			header, err := block.ReadBlockHeader_v2(file)
			if err != nil {
				if err == io.EOF {
					log.Printf("WARN: Reached EOF while expecting a block header at offset %d.", currentOffset)
					break
				}
				return nil, fmt.Errorf("failed to read block header at offset %d: %w", currentOffset, err)
			}

			hasher := crc32.NewIEEE()
			buf := make([]byte, 32*1024)
			var remaining uint64 = header.Length

			for remaining > 0 {
				readSize := int64(len(buf))
				if remaining < uint64(len(buf)) {
					readSize = int64(remaining)
				}

				n, err := file.Read(buf[:readSize])
				if err != nil && err != io.EOF {
					return nil, fmt.Errorf("failed to read block data at offset %d: %w", currentOffset, err)
				}
				if n == 0 {
					break
				}

				if _, err := hasher.Write(buf[:n]); err != nil {
					return nil, fmt.Errorf("failed to write to hasher for block at offset %d: %w", currentOffset, err)
				}
				remaining -= uint64(n)
			}

			crc := hasher.Sum32()
			scannedBlocks = append(scannedBlocks, scannedBlock{offset: currentOffset, header: header, crc: crc})
			fmt.Fprintf(w, "  %d\t\t%s\t\t%d\t%08x\n", currentOffset, getBlockTypeName(uint32(header.Type)), header.Length, crc)
		}
	}
	w.Flush()
	fmt.Println()
	return scannedBlocks, nil
}

// performCrossValidation 执行扫描结果与元数据之间的交叉验证
func performCrossValidation(footer *types.EnvelopeFooter_v2, scannedBlocks []scannedBlock, manifestObj types.Manifest_v2, w *tabwriter.Writer) {
	fmt.Println(">>> 4. Cross-Validation Report")

	// 1. Footer 验证
	if footer != nil {
		manifestBlock := findBlockByType(scannedBlocks, uint32(types.BlockTypeManifest_v2))
		if manifestBlock != nil {
			if uint64(manifestBlock.offset) == footer.ManifestOffset {
				fmt.Printf("  [OK] Footer ManifestOffset (%d) matches scanned block offset (%d).\n", footer.ManifestOffset, manifestBlock.offset)
			} else {
				fmt.Printf("  [ERROR] Mismatch! Footer ManifestOffset is %d, but scanned Manifest block is at %d.\n", footer.ManifestOffset, manifestBlock.offset)
			}
			if uint32(manifestBlock.crc) == footer.ManifestCRC32 {
				fmt.Printf("  [OK] Footer ManifestCRC32 (%08x) matches scanned block CRC32 (%08x).\n", footer.ManifestCRC32, manifestBlock.crc)
			} else {
				fmt.Printf("  [ERROR] Mismatch! Footer ManifestCRC32 is %08x, but scanned block CRC32 is %08x.\n", footer.ManifestCRC32, manifestBlock.crc)
			}
		} else {
			fmt.Printf("  [ERROR] Footer is valid, but no Manifest block was found during scan.\n")
		}
	} else {
		fmt.Println("  [INFO] Footer is invalid, skipping Footer-related validation.")
	}

	// 2. 数据块数量验证
	dataBlocks := findAllBlocksByType(scannedBlocks, uint32(types.BlockTypeData_v2))
	manifestDataFrags := countFragmentsByType(manifestObj.Fragments, string(types.FragmentType_SeekableStream))
	if len(dataBlocks) == manifestDataFrags {
		fmt.Printf("  [OK] Scanned Data Blocks (%d) count matches Manifest SeekableStream Fragments (%d).\n", len(dataBlocks), manifestDataFrags)
	} else {
		fmt.Printf("  [ERROR] Mismatch! Found %d Data Blocks in file, but Manifest lists %d SeekableStream Fragments.\n", len(dataBlocks), manifestDataFrags)
	}

	// 3. 偏移量映射验证
	fmt.Println("\n  --- Fragment Offset Mapping ---")
	fmt.Fprintln(w, "    Fragment ID\t\tGlobal Start Offset\tPhysical Block Offset")
	fmt.Fprintln(w, "    -----------\t\t------------------\t-------------------")

	fragIndex := 0
	for _, frag := range manifestObj.Fragments {
		if frag.Type == types.FragmentType_SeekableStream {
			physicalOffset := "<not found>"
			if fragIndex < len(dataBlocks) {
				physicalOffset = fmt.Sprintf("%d", dataBlocks[fragIndex].offset)
			}
			fmt.Fprintf(w, "    %s\t\t%d\t\t%s\n", frag.ID, frag.GlobalStartOffset, physicalOffset)
			fragIndex++
		}
	}
	w.Flush()
}

// scanForManifestWithOffset 扫描文件，返回 Manifest 的偏移量和数据
func scanForManifestWithOffset(file *os.File) (int64, []byte, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, nil, fmt.Errorf("failed to seek to start for manifest scan: %w", err)
	}
	for {
		currentOffset, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to get current offset during manifest scan: %w", err)
		}
		header, err := block.ReadBlockHeader_v2(file)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to read block header during scan: %w", err)
		}
		if header.Type == types.BlockTypeManifest_v2 {
			data, err := block.ReadBlockData_v2(file, header)
			if err != nil {
				return 0, nil, fmt.Errorf("failed to read manifest data: %w", err)
			}
			return currentOffset, data, nil
		}
		if _, err := file.Seek(int64(header.Length), io.SeekCurrent); err != nil {
			return 0, nil, fmt.Errorf("failed to skip block data: %w", err)
		}
	}
}

func getBlockTypeName(blockType uint32) string {
	switch blockType {
	case uint32(types.BlockTypeData_v2):
		return "Data"
	case uint32(types.BlockTypeManifest_v2):
		return "Manifest"
	default:
		return fmt.Sprintf("Unknown(%d)", blockType)
	}
}

func findBlockByType(blocks []scannedBlock, blockType uint32) *scannedBlock {
	for _, b := range blocks {
		if b.header.Type == uint16(blockType) {
			return &b
		}
	}
	return nil
}

func findAllBlocksByType(blocks []scannedBlock, blockType uint32) []scannedBlock {
	var result []scannedBlock
	for _, b := range blocks {
		if b.header.Type == uint16(blockType) {
			result = append(result, b)
		}
	}
	return result
}

func countFragmentsByType(fragments []types.Fragment_v2, fragType string) int {
	count := 0
	for _, f := range fragments {
		if f.Type == types.FragmentType_v2(fragType) {
			count++
		}
	}
	return count
}

// streamCRC32 流式地计算从文件开头起指定长度数据的 CRC32，内存占用极低
func streamCRC32(file *os.File, length uint64) (uint32, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("failed to seek to start for CRC stream: %w", err)
	}

	hasher := crc32.NewIEEE()
	buf := make([]byte, 32*1024) // 32KB buffer

	var remaining uint64 = length
	for remaining > 0 {
		readSize := int64(len(buf))
		if remaining < uint64(len(buf)) {
			readSize = int64(remaining)
		}

		n, err := file.Read(buf[:readSize])
		if err != nil && err != io.EOF {
			return 0, fmt.Errorf("failed to read from file for CRC stream: %w", err)
		}
		if n == 0 {
			break
		}

		if _, err := hasher.Write(buf[:n]); err != nil {
			return 0, fmt.Errorf("failed to write to hasher: %w", err)
		}
		remaining -= uint64(n)
	}

	return hasher.Sum32(), nil
}

// searchForManifestBlock 从指定偏移量开始，安全地搜索 Manifest 块，避免被垃圾数据干扰
func searchForManifestBlock(file *os.File, startOffset int64) (*scannedBlock, error) {
	if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to start offset for manifest search: %w", err)
	}

	// 我们要搜索的是 BlockHeader 中的 Type 字段
	manifestTypeBytes := make([]byte, 4)
	types.ByteOrder_v2.PutUint32(manifestTypeBytes, uint32(types.BlockTypeManifest_v2))

	buf := make([]byte, 32*1024)                // 32KB buffer for reading chunks
	searchWindow := make([]byte, 0, len(buf)+4) // Sliding window

	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read during manifest search: %w", err)
		}
		if n == 0 {
			break
		}

		searchWindow = append(searchWindow, buf[:n]...)

		// Search for the manifest type marker in the window
		for i := 0; i < len(searchWindow)-4; i++ {
			if searchWindow[i] == manifestTypeBytes[0] &&
				searchWindow[i+1] == manifestTypeBytes[1] &&
				searchWindow[i+2] == manifestTypeBytes[2] &&
				searchWindow[i+3] == manifestTypeBytes[3] {

				// Found it! Now calculate the exact offset.
				// The current file pointer is at startOffset + i + 4 (after reading the 4 bytes)
				// The block header starts 4 bytes before the Type field.
				blockStartOffset := startOffset + int64(i)

				// Seek to the start of the block header to read it properly
				if _, err := file.Seek(blockStartOffset, io.SeekStart); err != nil {
					return nil, fmt.Errorf("failed to seek to found manifest block start: %w", err)
				}

				header, err := block.ReadBlockHeader_v2(file)
				if err != nil {
					return nil, fmt.Errorf("failed to read manifest block header after finding it: %w", err)
				}

				data, err := block.ReadBlockData_v2(file, header)
				if err != nil {
					return nil, fmt.Errorf("failed to read manifest block data after finding it: %w", err)
				}

				crc := crc32.ChecksumIEEE(data)
				return &scannedBlock{
					offset: blockStartOffset,
					header: header,
					crc:    crc,
				}, nil
			}
		}

		// Keep the last 3 bytes for the next iteration to handle matches spanning buffer boundaries
		if len(searchWindow) > 3 {
			searchWindow = searchWindow[len(searchWindow)-3:]
		}
	}

	return nil, fmt.Errorf("manifest block not found in file")
}
