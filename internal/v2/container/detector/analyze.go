package detector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"os"
	"path/filepath"
	"text/tabwriter"
	"text/template"

	"github.com/Soltus/encv-go/internal/v2/chunker"
	"github.com/Soltus/encv-go/internal/v2/container/block"
	"github.com/Soltus/encv-go/internal/v2/container/manifest"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// AnalyzeContainerV2 可视化分析 v2 容器文件的结构
// printToStdout: 是否同时打印到标准输出（为了兼容CLI调用）
// 返回格式化的HTML内容和错误
func AnalyzeContainerV2(ctx context.Context, containerPath string, printToStdout bool) (string, error) {
	absPath, err := filepath.Abs(containerPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	file, err := os.Open(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file '%s': %w", absPath, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := stat.Size()

	// 【关键】使用 bytes.Buffer 替代 os.Stdout
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintf(&buf, "--- ENCV Container Analysis: %s (Size: %d bytes) ---\n\n", filepath.Base(absPath), fileSize)

	// --- 1. Footer Analysis ---
	fmt.Fprintln(&buf, ">>> 1. Footer Analysis (from end of file)")
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
		fmt.Println(&buf)
	}

	// --- 2. Manifest Analysis ---
	fmt.Fprintln(&buf, ">>> 2. Manifest Analysis")
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
		// log.Printf("  INFO: Scanning for manifest block from file start...\n")
		manifestBytes, readErr = manifest.ExtractManifest_v2(absPath)
		if readErr != nil {
			// 连扫描都失败了，这是一个致命错误，无法继续
			return "", fmt.Errorf("failed to read manifest via both Footer and scanning: %w", readErr)
		}
		manifestSource = "Scanned"
	}

	fmt.Fprintf(&buf, "  Source:\t%s\n", manifestSource)
	fmt.Fprintf(&buf, "  Raw Size:\t%d bytes\n\n", len(manifestBytes))

	// --- 3. Manifest JSON Parsing ---
	fmt.Fprintln(&buf, "  Content (pretty-printed):")
	var manifestObj types.Manifest_v2
	var jsonParseErr error
	if jsonParseErr = json.Unmarshal(manifestBytes, &manifestObj); jsonParseErr != nil {
		fmt.Fprintf(&buf, "    Status: FAILED to unmarshal manifest JSON: %v\n", jsonParseErr)
		fmt.Fprintf(&buf, "    Raw data (first 128 bytes): %q\n", string(manifestBytes[:min(128, len(manifestBytes))]))
	} else {
		prettyJSON, _ := json.MarshalIndent(manifestObj, "    ", "  ")
		fmt.Fprintf(&buf, "    %s\n", string(prettyJSON))
	}
	fmt.Fprintln(&buf)

	// --- 4. Orchestrate Scanning and Validation ---
	scanHTML, scannedBlocks, scanErr := performPhysicalLayoutScan(file, manifestObj, fileSize)
	if scanErr != nil {
		return "", fmt.Errorf("physical layout scan failed: %w", scanErr)
	}
	// 【新增】将扫描结果写入主 buffer
	buf.WriteString(scanHTML)

	// 只有当 JSON 解析成功时，才进行交叉验证
	if jsonParseErr == nil {
		// 【修改】调用返回字符串的辅助函数
		validationHTML, validationErr := performCrossValidation(footer, scannedBlocks, manifestObj)
		if validationErr != nil {
			// 交叉验证失败通常不是致命错误，可以记录但继续
			log.Printf("WARN: Cross-validation failed: %v", validationErr)
			buf.WriteString(">>> 4. Cross-Validation Report\n  [ERROR] Cross-validation encountered an error. See server logs for details.\n")
		} else {
			// 【新增】将验证结果写入主 buffer
			buf.WriteString(validationHTML)
		}
	} else {
		buf.WriteString(">>> 4. Cross-Validation Report\n  [INFO] Manifest parsing failed, skipping cross-validation.\n")
	}

	// 确保所有内容都写入 buffer
	w.Flush()

	// 【关键】将 buffer 的内容转为字符串
	content := buf.String()

	// 如果需要，同时打印到控制台
	if printToStdout {
		fmt.Print(content)
	}

	// 【关键】为了在HTML中安全显示，我们需要进行HTML转义
	// 使用 template.HTMLEscapeString 来防止XSS和格式破坏
	safeContent := template.HTMLEscapeString(content)

	// 用 <pre><code> 包裹，以保留空白和换行
	finalHTML := fmt.Sprintf("<pre><code>%s</code></pre>", safeContent)

	return finalHTML, nil
}

// --- 辅助函数 ---

// scannedBlock 用于记录扫描到的块信息
type scannedBlock struct {
	offset int64
	header *block.BlockHeader_v2
	crc    uint32
}

// performPhysicalLayoutScan 执行物理块的扫描，并返回扫描结果和格式化字符串
func performPhysicalLayoutScan(file *os.File, manifestObj types.Manifest_v2, fileSize int64) (string, []scannedBlock, error) {
	// 【关键】使用 bytes.Buffer 来捕获输出
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(&buf, ">>> 3. Physical Layout Scan (Block-by-Block)")

	isChunked := manifestObj.Version != 0 && !chunker.IsSingleFileContainer(&manifestObj)
	if isChunked {
		fmt.Fprintln(&buf, "  [INFO] Detected a physical chunked container. Analyzing main file layout.")
	} else {
		fmt.Fprintln(&buf, "  [INFO] Detected a single-file container. Scanning for all blocks.")
	}

	fmt.Fprintln(w, "  Offset\t\tType\t\tLength\tCRC32")
	fmt.Fprintln(w, "  ------\t\t----\t\t------\t------")

	var scannedBlocks []scannedBlock

	// ... (中间的逻辑保持不变，但所有 fmt.Fprintf/os.Stdout 改为 fmt.Fprintf/&buf) ...
	// --- 模式 1: 物理分片容器 ---
	if isChunked {
		// ... (逻辑不变，输出改为 &buf) ...
		var firstChunkLength uint64
		var firstChunkManifestCRC uint32
		for _, frag := range manifestObj.Fragments {
			if frag.ID == "logical_fragment_0" {
				firstChunkLength = frag.Length
				firstChunkManifestCRC = frag.DataCRC32
				break
			}
		}
		if firstChunkLength == 0 {
			return "", nil, fmt.Errorf("could not find length for logical_fragment_0 in manifest")
		}
		crc, err := streamCRC32(file, firstChunkLength)
		if err != nil {
			return "", nil, fmt.Errorf("failed to stream CRC for first chunk: %w", err)
		}
		if crc != firstChunkManifestCRC {
			fmt.Fprintf(&buf, "  [WARN] CRC mismatch for logical_fragment_0! Calculated: %08x, Manifest: %08x\n", crc, firstChunkManifestCRC)
		}
		scannedBlocks = append(scannedBlocks, scannedBlock{
			offset: 0,
			header: &block.BlockHeader_v2{Type: types.BlockTypeData_v2, Length: firstChunkLength},
			crc:    crc,
		})
		fmt.Fprintf(w, "  0\t\t%s\t\t%d\t%08x (raw stream)\n", getBlockTypeName(uint32(types.BlockTypeData_v2)), firstChunkLength, crc)

		fmt.Fprintln(&buf, "  [INFO] Re-scanning for manifest block from file start (reliable method)...")
		manifestOffset, manifestBytes, err := scanForManifestWithOffset(file)
		if err != nil {
			return "", nil, fmt.Errorf("failed to locate manifest in main file using reliable scan: %w", err)
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
		fmt.Fprintln(&buf, "  [INFO] Scanning for all blocks with streaming CRC calculation.")
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return "", nil, fmt.Errorf("failed to seek to start for layout scan: %w", err)
		}
		for {
			currentOffset, err := file.Seek(0, io.SeekCurrent)
			if err != nil {
				return "", nil, fmt.Errorf("failed to get current offset: %w", err)
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
				return "", nil, fmt.Errorf("failed to read block header at offset %d: %w", currentOffset, err)
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
					return "", nil, fmt.Errorf("failed to read block data at offset %d: %w", currentOffset, err)
				}
				if n == 0 {
					break
				}
				if _, err := hasher.Write(buf[:n]); err != nil {
					return "", nil, fmt.Errorf("failed to write to hasher for block at offset %d: %w", currentOffset, err)
				}
				remaining -= uint64(n)
			}
			crc := hasher.Sum32()
			scannedBlocks = append(scannedBlocks, scannedBlock{offset: currentOffset, header: header, crc: crc})
			fmt.Fprintf(w, "  %d\t\t%s\t\t%d\t%08x\n", currentOffset, getBlockTypeName(uint32(header.Type)), header.Length, crc)
		}
	}

	// 【关键】确保所有内容都写入 buffer 并返回
	w.Flush()
	return buf.String(), scannedBlocks, nil
}

// performCrossValidation 执行扫描结果与元数据之间的交叉验证，并返回格式化字符串
func performCrossValidation(footer *types.EnvelopeFooter_v2, scannedBlocks []scannedBlock, manifestObj types.Manifest_v2) (string, error) {
	// 【关键】使用 bytes.Buffer 来捕获输出
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(&buf, ">>> 4. Cross-Validation Report")

	// ... (所有逻辑保持不变，但所有 fmt.Fprintf/os.Stdout 改为 fmt.Fprintf/&buf) ...
	if footer != nil {
		manifestBlock := findBlockByType(scannedBlocks, uint32(types.BlockTypeManifest_v2))
		if manifestBlock != nil {
			if uint64(manifestBlock.offset) == footer.ManifestOffset {
				fmt.Fprintf(&buf, "  [OK] Footer ManifestOffset (%d) matches scanned block offset (%d).\n", footer.ManifestOffset, manifestBlock.offset)
			} else {
				fmt.Fprintf(&buf, "  [ERROR] Mismatch! Footer ManifestOffset is %d, but scanned Manifest block is at %d.\n", footer.ManifestOffset, manifestBlock.offset)
			}
			if uint32(manifestBlock.crc) == footer.ManifestCRC32 {
				fmt.Fprintf(&buf, "  [OK] Footer ManifestCRC32 (%08x) matches scanned block CRC32 (%08x).\n", footer.ManifestCRC32, manifestBlock.crc)
			} else {
				fmt.Fprintf(&buf, "  [ERROR] Mismatch! Footer ManifestCRC32 is %08x, but scanned block CRC32 is %08x.\n", footer.ManifestCRC32, manifestBlock.crc)
			}
		} else {
			fmt.Fprintf(&buf, "  [ERROR] Footer is valid, but no Manifest block was found during scan.\n")
		}
	} else {
		fmt.Fprintln(&buf, "  [INFO] Footer is invalid, skipping Footer-related validation.")
	}

	var dataBlockFrags int
	for _, frag := range manifestObj.Fragments {
		if frag.Type == types.FragmentType_SeekableStream || frag.Type == types.FragmentType_AtomicFile {
			dataBlockFrags++
		}
	}
	dataBlocks := findAllBlocksByType(scannedBlocks, uint32(types.BlockTypeData_v2))
	if len(dataBlocks) == dataBlockFrags {
		fmt.Fprintf(&buf, "  [OK] Scanned Data Blocks (%d) count matches Manifest Data Fragments (%d).\n", len(dataBlocks), dataBlockFrags)
	} else {
		fmt.Fprintf(&buf, "  [ERROR] Mismatch! Found %d Data Blocks in file, but Manifest lists %d Data Fragments (SeekableStream + AtomicFile).\n", len(dataBlocks), dataBlockFrags)
	}

	fmt.Fprintln(&buf, "\n  --- Fragment Offset Mapping ---")
	fmt.Fprintln(w, "    Fragment ID\t\tType\t\tGlobal Start Offset\tPhysical Block Offset")
	fmt.Fprintln(w, "    -----------\t\t----\t\t------------------\t-------------------")
	fragIndex := 0
	for _, frag := range manifestObj.Fragments {
		if frag.Type == types.FragmentType_SeekableStream || frag.Type == types.FragmentType_AtomicFile {
			physicalOffset := "<not found>"
			if fragIndex < len(dataBlocks) {
				physicalOffset = fmt.Sprintf("%d", dataBlocks[fragIndex].offset)
			}
			fmt.Fprintf(w, "    %s\t\t%s\t\t%d\t\t%s\n", frag.ID, frag.Type, frag.GlobalStartOffset, physicalOffset)
			fragIndex++
		}
	}

	// 【关键】确保所有内容都写入 buffer 并返回
	w.Flush()
	return buf.String(), nil
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
