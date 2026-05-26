package detector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"text/tabwriter"
	"text/template"

	"github.com/Soltus/encv-go/internal/logger"
	"github.com/Soltus/encv-go/internal/v2/chunker"
	"github.com/Soltus/encv-go/internal/v2/container/block"
	"github.com/Soltus/encv-go/internal/v2/container/envelope"
	"github.com/Soltus/encv-go/internal/v2/types"
	"github.com/fxamacker/cbor/v2"
)

// detectorLogger 是 detector 包的日志记录器
var detectorLogger = logger.WithComponent("detector")

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

	// --- 0. Header Analysis (V3 新增) ---
	fmt.Fprintln(&buf, ">>> 0. Header Analysis (from beginning of file)")
	headerSize, headerErr := analyzeHeader(&buf, file)
	if headerErr != nil {
		fmt.Fprintf(&buf, "  Status: FAILED\n  Reason: %v\n\n", headerErr)
		// 如果头部完全读不出来，可能不是 V3，尝试回退或直接报错。
		// 这里我们假设如果是 V2，analyzeHeader 会返回 V2 的尺寸 (16)
	}
	w.Flush()

	// --- 1. Footer Analysis ---
	fmt.Fprintln(&buf, ">>> 1. Footer Analysis (from end of file)")
	detectedVersion, _, _ := types.DetectHeaderInfoFromReaderAt(file)
	var footer *types.EnvelopeFooter_v2
	var footerErr error
	if detectedVersion == 4 {
		footerSize := int64(types.EnvelopeFooterSize_v4)
		stat, _ := file.Stat()
		if stat.Size() >= footerSize {
			footerReader := io.NewSectionReader(file, stat.Size()-footerSize, footerSize)
			v4Footer, v4Err := types.ReadFooterV4(footerReader)
			if v4Err != nil {
				fmt.Fprintf(&buf, "  Status: FAILED\n  Reason: %v\n\n", v4Err)
			} else {
				fmt.Fprintf(w, "  Status:\tOK\n")
				fmt.Fprintf(w, "  Magic:\t%s\n", string(v4Footer.Magic[:]))
				fmt.Fprintf(w, "  Global CRC32:\t%08x\n", v4Footer.GlobalCRC32)
				w.Flush()
				fmt.Fprintln(&buf)
			}
		} else {
			fmt.Fprintf(&buf, "  Status: FAILED\n  Reason: file too small for v4 footer\n\n")
		}
		footerErr = fmt.Errorf("v4 container: footer not in v2 format")
	} else {
		footer, footerErr = envelope.ReadEnvelopeFooter_v2(file)
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
	}

	// --- 2. Manifest Analysis ---
	fmt.Fprintln(&buf, ">>> 2. Manifest Analysis")
	var manifestBytes []byte
	var manifestSource string
	var readErr error

	// 尝试通过 Footer 读取 (Footer Offset 是绝对值，通常兼容)
	if footerErr == nil {
		manifestBytes, readErr = readManifestFromFooter(file, footer)
		if readErr != nil {
			fmt.Printf("  WARN: Failed to read manifest via Footer (%v). Falling back to scan.\n", readErr)
		} else {
			manifestSource = "From Footer"
		}
	}

	// 降级到扫描 (需要 Header Size)
	if manifestBytes == nil {
		// 【关键】扫描时需要传入 headerSize
		manifestBytes, _, readErr = extractManifestWithScan(absPath, headerSize)
		if readErr != nil {
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
	scanHTML, scannedBlocks, scanErr := performPhysicalLayoutScan(file, manifestObj, fileSize, headerSize)
	if scanErr != nil {
		return "", fmt.Errorf("physical layout scan failed: %w", scanErr)
	}
	// 【新增】将扫描结果写入主 buffer
	buf.WriteString(scanHTML)

	// 只有当 JSON 解析成功时，才进行交叉验证
	if jsonParseErr == nil {
		var validationHTML string
		var validationErr error
		if detectedVersion == 4 {
			validationHTML, validationErr = performCrossValidationV4(file, headerSize, scannedBlocks)
		} else {
			validationHTML, validationErr = performCrossValidation(footer, scannedBlocks, manifestObj)
		}
		if validationErr != nil {
			// 交叉验证失败通常不是致命错误，可以记录但继续
			detectorLogger.Warn("cross-validation failed",
				slog.Any("error", validationErr),
			)
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

// analyzeHeader 读取并分析头部，返回头部大小
func analyzeHeader(buf *bytes.Buffer, file *os.File) (int64, error) {
	w := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)

	version, _, err := types.DetectHeaderInfoFromReaderAt(file)
	if err != nil {
		return 0, fmt.Errorf("failed to analyze header: %w", err)
	}

	switch version {
	case 4:
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return 0, err
		}
		header, err := types.ReadHeaderV4(file)
		if err != nil {
			return 0, fmt.Errorf("failed to read v4 header: %w", err)
		}
		fmt.Fprintf(w, "  Version:\t%d (V4)\n", header.Version)
		fmt.Fprintf(w, "  Magic:\t%s\n", string(header.Magic[:]))
		flagStr := ""
		if header.Flags&types.FlagIsMainContainer != 0 {
			flagStr += "MainContainer "
		}
		if header.Flags&types.FlagIsPhysicalChunk != 0 {
			flagStr += "PhysicalChunk "
		}
		fmt.Fprintf(w, "  Flags:\t0x%04x (%s)\n", header.Flags, flagStr)
		fmt.Fprintf(w, "  ID Type:\t%d\n", header.IDType)
		fmt.Fprintf(w, "  ID Length:\t%d bytes\n", header.IDLength)
		fmt.Fprintf(w, "  Header CRC32:\t%08x (Verified OK)\n", header.HeaderCRC32)
		fmt.Fprintf(w, "  ContainerType:\t%s\n", header.ContainerType)
		fmt.Fprintf(w, "  IsSeekable:\t%v\n", header.IsSeekable)
		fmt.Fprintf(w, "  ManifestOffset:\t%d\n", header.ManifestOffset)
		fmt.Fprintf(w, "  ManifestLength:\t%d\n", header.ManifestLength)
		w.Flush()
		return types.EnvelopeHeaderSize_v4, nil

	case 3:
		// 2. Seek 回到文件开头，因为 ReadHeaderV3 需要从头读取
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return 0, err
		}

		// 3. 使用 types.ReadHeaderV3 读取并校验头部
		// 该函数内部会处理 CRC32 校验和二进制反序列化
		header, err := types.ReadHeaderV3(file)
		if err != nil {
			return 0, fmt.Errorf("failed to read v3 header: %w", err)
		}

		// 4. 填充分析报告
		fmt.Fprintf(w, "  Version:\t%d (V3)\n", header.Version)
		fmt.Fprintf(w, "  Magic:\t%s\n", string(header.Magic[:]))

		// 解析 Flags
		flagStr := ""
		if header.Flags&types.FlagIsMainContainer != 0 {
			flagStr += "MainContainer "
		}
		if header.Flags&types.FlagIsPhysicalChunk != 0 {
			flagStr += "PhysicalChunk "
		}
		fmt.Fprintf(w, "  Flags:\t0x%04x (%s)\n", header.Flags, flagStr)

		fmt.Fprintf(w, "  ID Type:\t%d\n", header.IDType)
		fmt.Fprintf(w, "  ID Length:\t%d bytes\n", header.IDLength)
		// CRC32 已在 ReadHeaderV3 中校验通过
		fmt.Fprintf(w, "  Header CRC32:\t%08x (Verified OK)\n", header.HeaderCRC32)

		// 尝试解析 SpecialID (如果是 CBOR)
		if header.IDType == uint32(types.IDType_CBOR) && header.IDLength > 0 {
			specialIDBytes := header.SpecialID[:header.IDLength]
			var meta map[string]interface{}
			if err := cbor.Unmarshal(specialIDBytes, &meta); err == nil {
				fmt.Fprintf(buf, "  SpecialID Content (CBOR):\n")
				for k, v := range meta {
					fmt.Fprintf(w, "    %s:\t%v\n", k, v)
				}
			} else {
				fmt.Fprintf(buf, "  SpecialID Content:\t[CBOR Parse Failed: %v]\n", err)
			}
		}

		w.Flush()
		return types.EnvelopeHeaderSize_v3, nil

	case 2:
		fmt.Fprintf(buf, "  Version:\t2 (V2)\n")
		return types.EnvelopeHeaderSize_v2, nil
	}

	return 0, fmt.Errorf("unknown header version")
}

// scannedBlock 用于记录扫描到的块信息
type scannedBlock struct {
	offset int64
	header *block.BlockHeader_v2
	crc    uint32
}

func extractManifestWithScan(filePath string, headerSize int64) ([]byte, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open file '%s': %w", filePath, err)
	}
	defer file.Close()

	// 调用通用扫描函数
	offset, data, err := scanForManifestWithOffset(file, headerSize)
	if err != nil {
		return nil, 0, err
	}
	return data, offset, nil
}

// readManifestFromFooter 封装从 Footer 指定位置读取 Manifest 的逻辑
func readManifestFromFooter(f *os.File, footer *types.EnvelopeFooter_v2) ([]byte, error) {
	// 1. 跳转到 Footer 指定的 Manifest 偏移量 (绝对偏移)
	// 注意：V2 和 V3 的 Footer Offset 都是绝对地址，所以不需要额外计算 HeaderSize
	_, err := f.Seek(int64(footer.ManifestOffset), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to manifest offset %d: %w", footer.ManifestOffset, err)
	}

	// 2. 读取 Block Header
	header, err := block.ReadBlockHeader_v2(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest block header: %w", err)
	}

	// 3. 校验块类型
	if header.Type != types.BlockTypeManifest_v2 {
		return nil, fmt.Errorf("block at footer-specified offset is not a manifest (type: %d)", header.Type)
	}

	// 4. 读取 Block Data
	return block.ReadBlockData_v2(f, header)
}

// performPhysicalLayoutScan 执行物理块的扫描，并返回扫描结果和格式化字符串
func performPhysicalLayoutScan(file *os.File, manifestObj types.Manifest_v2, fileSize int64, headerSize int64) (string, []scannedBlock, error) {
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

	// --- 模式 1: 物理分片容器 ---
	if isChunked {
		var firstChunkLength uint64
		for _, frag := range manifestObj.Fragments {
			if frag.ID == "logical_fragment_0" {
				firstChunkLength = frag.Length
				break
			}
		}
		if firstChunkLength == 0 {
			return "", nil, fmt.Errorf("could not find length for logical_fragment_0 in manifest")
		}

		// 【修复】流式 CRC 需要跳过 Header
		crc, err := streamCRC32(file, headerSize, firstChunkLength)
		if err != nil {
			return "", nil, fmt.Errorf("failed to stream CRC for first chunk: %w", err)
		}

		// 注意：对于分片容器，第一个分片通常就是数据块，没有额外的 Block Header (在 V2 逻辑中是 raw stream，在 V3 也是 raw stream)
		// 除非分片本身内部又封装了 Block。根据之前的 file_chunker 代码，分片直接写数据。
		scannedBlocks = append(scannedBlocks, scannedBlock{
			offset: headerSize, // 【修复】起始偏移包含头部
			header: &block.BlockHeader_v2{Type: types.BlockTypeData_v2, Length: firstChunkLength},
			crc:    crc,
		})
		fmt.Fprintf(w, "  %d\t\t%s\t\t%d\t%08x (raw stream)\n", headerSize, types.GetBlockTypeName(uint32(types.BlockTypeData_v2)), firstChunkLength, crc)

		fmt.Fprintln(&buf, "  [INFO] Re-scanning for manifest block...")
		// 【修复】扫描需要传入 headerSize
		manifestOffset, manifestBytes, err := scanForManifestWithOffset(file, headerSize)
		if err != nil {
			return "", nil, fmt.Errorf("failed to locate manifest in main file: %w", err)
		}
		manifestScannedBlock := scannedBlock{
			offset: manifestOffset,
			header: &block.BlockHeader_v2{Type: types.BlockTypeManifest_v2, Length: uint64(len(manifestBytes))},
			crc:    crc32.ChecksumIEEE(manifestBytes),
		}
		scannedBlocks = append(scannedBlocks, manifestScannedBlock)
		fmt.Fprintf(w, "  %d\t\t%s\t\t%d\t%08x\n", manifestScannedBlock.offset, types.GetBlockTypeName(uint32(manifestScannedBlock.header.Type)), manifestScannedBlock.header.Length, manifestScannedBlock.crc)

	} else {
		// --- 模式 2: 单文件容器 ---
		fmt.Fprintln(&buf, "  [INFO] Scanning for all blocks.")

		// 【修复】Seek 到 Header 结束位置
		if _, err := file.Seek(headerSize, io.SeekStart); err != nil {
			return "", nil, fmt.Errorf("failed to seek past header: %w", err)
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
					return "", nil, fmt.Errorf("failed to read block data: %w", err)
				}
				if n == 0 {
					break
				}
				if _, err := hasher.Write(buf[:n]); err != nil {
					return "", nil, fmt.Errorf("failed to write to hasher: %w", err)
				}
				remaining -= uint64(n)
			}
			crc := hasher.Sum32()
			scannedBlocks = append(scannedBlocks, scannedBlock{offset: currentOffset, header: header, crc: crc})
			fmt.Fprintf(w, "  %d\t\t%s\t\t%d\t%08x\n", currentOffset, types.GetBlockTypeName(uint32(header.Type)), header.Length, crc)
		}
	}

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

func performCrossValidationV4(file *os.File, headerSize int64, scannedBlocks []scannedBlock) (string, error) {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(&buf, ">>> 4. Cross-Validation Report (V4)")

	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := stat.Size()

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("failed to seek: %w", err)
	}
	v4Header, err := types.ReadHeaderV4(file)
	if err != nil {
		return "", fmt.Errorf("failed to read v4 header for validation: %w", err)
	}

	footerSize := int64(types.EnvelopeFooterSize_v4)
	footerReader := io.NewSectionReader(file, fileSize-footerSize, footerSize)
	v4Footer, err := types.ReadFooterV4(footerReader)
	if err != nil {
		return "", fmt.Errorf("failed to read v4 footer for validation: %w", err)
	}

	headerCRC, err := streamCRC32(file, 0, uint64(headerSize))
	if err != nil {
		return "", fmt.Errorf("failed to compute header CRC: %w", err)
	}
	if headerCRC == v4Header.HeaderCRC32 {
		fmt.Fprintf(&buf, "  [OK] Header CRC32 matches computed value (%08x).\n", headerCRC)
	} else {
		fmt.Fprintf(&buf, "  [ERROR] Header CRC32 mismatch! Stored: %08x, Computed: %08x.\n", v4Header.HeaderCRC32, headerCRC)
	}

	dataBlocks := findAllBlocksByType(scannedBlocks, uint32(types.BlockTypeData_v2))
	totalDataCRC := crc32.NewIEEE()
	for _, b := range dataBlocks {
		totalDataCRC.Write([]byte{byte(b.crc), byte(b.crc >> 8), byte(b.crc >> 16), byte(b.crc >> 24)})
	}

	fmt.Fprintf(&buf, "  [INFO] V4 Footer GlobalCRC32: %08x\n", v4Footer.GlobalCRC32)
	fmt.Fprintf(&buf, "  [INFO] Scanned %d data blocks, manifest stored in V4 Header (offset=%d, length=%d).\n",
		len(dataBlocks), v4Header.ManifestOffset, v4Header.ManifestLength)

	w.Flush()
	return buf.String(), nil
}

// scanForManifestWithOffset 扫描文件，返回 Manifest 的偏移量和数据
func scanForManifestWithOffset(file *os.File, headerSize int64) (int64, []byte, error) {
	if _, err := file.Seek(headerSize, io.SeekStart); err != nil {
		return 0, nil, fmt.Errorf("failed to seek to start for scan: %w", err)
	}

	for {
		currentOffset, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to get current offset: %w", err)
		}
		header, err := block.ReadBlockHeader_v2(file)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to read block header: %w", err)
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
func streamCRC32(file *os.File, startOffset int64, length uint64) (uint32, error) {
	if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
		return 0, fmt.Errorf("failed to seek to start for CRC stream: %w", err)
	}

	hasher := crc32.NewIEEE()
	buf := make([]byte, 32*1024)

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
