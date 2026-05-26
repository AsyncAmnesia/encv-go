// internal/v2/container/manifest_v2.go
package manifest

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/Soltus/encv-go/internal/logger"
	"github.com/Soltus/encv-go/internal/v2/container/block"
	"github.com/Soltus/encv-go/internal/v2/container/envelope"
	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// manifestLogger 是 manifest 包的日志记录器
var manifestLogger = logger.WithComponent("manifest")

var manifestBlockBufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 32*1024)
	},
}

// EncryptManifest 加密原始 Manifest 字节
func EncryptManifest(plainData []byte) ([]byte, error) {
	return crypto.EncryptSystemPayload(plainData)
}

// DecryptManifest 解密加密的 Manifest 字节
func DecryptManifest(encryptedData []byte) ([]byte, error) {
	return crypto.DecryptSystemPayload(encryptedData)
}

// DeserializeFromJSON_v2 从 JSON 字节反序列化清单
func DeserializeFromJSON_v2(data []byte) (*types.Manifest_v2, error) {
	var manifest types.Manifest_v2
	err := json.Unmarshal(data, &manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}
	return &manifest, nil
}

// ExtractKVI_v2 从 v2 容器文件中直接扫描并提取 KVI 块的数据，不依赖清单。
func ExtractKVI_v2(containerPath string) ([]byte, error) {
	file, err := os.Open(containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file '%s': %w", containerPath, err)
	}
	defer file.Close()

	// log.Printf("INFO: Scanning '%s' for KVI block...", containerPath)

	// 1. 【关键修复】探测 Header 并跳过
	_, headerSize, err := types.DetectHeaderInfoFromReaderAt(file)
	if err != nil {
		return nil, fmt.Errorf("failed to detect header size: %w", err)
	}
	if _, err := file.Seek(headerSize, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to data stream start: %w", err)
	}

	// 从头开始扫描，直到找到 KVI 块
	for {
		// 记录当前块的起始偏移量
		blockStartOffset, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("failed to get current offset: %w", err)
		}

		// 读取块头
		header, err := block.ReadBlockHeader_v2(file)
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("reached end of file but KVI block was not found in '%s'", containerPath)
			}
			return nil, fmt.Errorf("failed to read block header: %w", err)
		}

		// 如果是 KVI 块，就读取它的数据
		if header.Type == types.BlockTypeKVI_v2 {
			manifestLogger.Debug("found KVI block",
				slog.Int64("offset", blockStartOffset),
				slog.Uint64("length", header.Length),
			)
			// 使用 ReadBlockData_v2 来读取并验证 CRC
			kviData, err := block.ReadBlockData_v2(file, header)
			if err != nil {
				return nil, fmt.Errorf("failed to read KVI block data: %w", err)
			}
			return kviData, nil
		}

		// 如果不是 KVI 块，就跳过它
		_, err = file.Seek(int64(header.Length), io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("failed to seek past block data: %w", err)
		}
	}
}

// ExtractManifest_v2 从 v2 容器文件中直接扫描并提取 Manifest 块的数据。
func ExtractManifest_v2(containerPath string) ([]byte, error) {
	file, err := os.Open(containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file '%s': %w", containerPath, err)
	}
	defer file.Close()

	// log.Printf("INFO: Scanning '%s' for Manifest block...", containerPath)

	// 1. 【关键修复】探测 Header 并跳过
	_, headerSize, err := types.DetectHeaderInfoFromReaderAt(file)
	if err != nil {
		return nil, fmt.Errorf("failed to detect header size: %w", err)
	}
	if _, err := file.Seek(headerSize, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to data stream start: %w", err)
	}

	for {
		header, err := block.ReadBlockHeader_v2(file)
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("reached end of file but Manifest block was not found in '%s'", containerPath)
			}
			return nil, fmt.Errorf("failed to read block header: %w", err)
		}

		if header.Type == types.BlockTypeManifest_v2 {
			// log.Printf("DEBUG: Found Manifest block.")
			manifestData, err := block.ReadBlockData_v2(file, header)
			if err != nil {
				return nil, fmt.Errorf("failed to read Manifest block data: %w", err)
			}
			return manifestData, nil
		}

		_, err = file.Seek(int64(header.Length), io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("failed to seek past block data: %w", err)
		}
	}
}

// 从指定偏移量读取 Manifest
func ReadManifestAt(filePath string, offset int64, length int64) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	// 【关键修复】使用 readAndDecryptManifest 读取并解密 Manifest
	manifest, err := readAndDecryptManifest(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read and decrypt manifest: %w", err)
	}

	// 重新序列化为 JSON 字节返回（保持接口兼容）
	return manifest.SerializeToJSON_v2()
}

// ParseManifestFromBytes 从 JSON 字节切片中解析 Manifest
func ParseManifestFromBytes(data []byte) (*types.Manifest_v2, error) {
	return DeserializeFromJSON_v2(data)
}

// ReadManifestFromFile 直接从文件路径读取并解析 Manifest
// 这是一个低级函数，用于避免包之间的循环依赖
// 返回：Manifest, Footer, HeaderVersion, HeaderSize, Error
func ReadManifestFromFile(filePath string) (*types.Manifest_v2, *types.EnvelopeFooter_v2, int, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// 1. 探测 Header
	headerVersion, headerSize, err := types.DetectHeaderInfoFromReaderAt(file)
	manifestLogger.Debug("reading manifest from file",
		slog.String("file", filePath),
		slog.Int("header_version", headerVersion),
		slog.Int64("header_size", headerSize),
	)
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("failed to detect header info: %w", err)
	}

	if headerVersion == 4 {
		return readManifestV4(file, filePath, headerSize)
	}

	// 2. 【V2/V3 路径】使用 envelope 包读取 Footer
	footer, err := envelope.ReadEnvelopeFooter_v2(file)
	if err != nil {
		return nil, nil, headerVersion, headerSize, fmt.Errorf("failed to read envelope footer: %w", err)
	}

	// 3. 定位到 Manifest
	// 注意：Footer.ManifestOffset 是相对于文件开头的绝对偏移量
	if _, err := file.Seek(int64(footer.ManifestOffset), io.SeekStart); err != nil {
		return nil, nil, headerVersion, headerSize, fmt.Errorf("failed to seek to manifest: %w", err)
	}

	// 4. 读取并解密 Manifest
	manifest, err := readAndDecryptManifest(file)
	if err != nil {
		return nil, nil, headerVersion, headerSize, fmt.Errorf("failed to read/decrypt manifest: %w", err)
	}

	return manifest, footer, headerVersion, headerSize, nil
}

func readManifestV4(file *os.File, filePath string, headerSize int64) (*types.Manifest_v2, *types.EnvelopeFooter_v2, int, int64, error) {
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, nil, 4, headerSize, fmt.Errorf("failed to stat file: %w", err)
	}

	footerSize := int64(types.EnvelopeFooterSize_v4)
	if fileInfo.Size() < footerSize {
		return nil, nil, 4, headerSize, fmt.Errorf("file too small for v4 footer")
	}

	footerReader := io.NewSectionReader(file, fileInfo.Size()-footerSize, footerSize)
	v4Footer, err := types.ReadFooterV4(footerReader)
	if err != nil {
		return nil, nil, 4, headerSize, fmt.Errorf("failed to read v4 footer: %w", err)
	}
	if string(v4Footer.Magic[:]) != string(types.MagicFooter_v2[:]) {
		return nil, nil, 4, headerSize, fmt.Errorf("v4 footer magic mismatch")
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, nil, 4, headerSize, fmt.Errorf("failed to seek to header: %w", err)
	}
	v4Header, err := types.ReadHeaderV4(file)
	if err != nil {
		return nil, nil, 4, headerSize, fmt.Errorf("failed to read v4 header: %w", err)
	}

	if v4Header.ManifestLength == 0 || v4Header.ManifestOffset == 0 {
		return nil, nil, 4, headerSize, fmt.Errorf("v4 header has invalid manifest offset/length")
	}

	obfuscatedManifest := make([]byte, v4Header.ManifestLength)
	if _, err := file.Seek(int64(v4Header.ManifestOffset), io.SeekStart); err != nil {
		return nil, nil, 4, headerSize, fmt.Errorf("failed to seek to v4 manifest: %w", err)
	}
	if _, err := io.ReadFull(file, obfuscatedManifest); err != nil {
		return nil, nil, 4, headerSize, fmt.Errorf("failed to read v4 manifest data: %w", err)
	}

	plainManifest, err := crypto.DeobfuscateManifest(obfuscatedManifest)
	if err != nil {
		return nil, nil, 4, headerSize, fmt.Errorf("failed to deobfuscate v4 manifest: %w", err)
	}

	v4Manifest, err := types.DeserializeManifest_v4(plainManifest)
	if err != nil {
		return nil, nil, 4, headerSize, fmt.Errorf("failed to deserialize v4 manifest: %w", err)
	}

	v2Manifest := adaptV4ToV2Manifest(v4Manifest, v4Header)

	return v2Manifest, nil, 4, headerSize, nil
}

func adaptV4ToV2Manifest(v4 *types.Manifest_v4, header *types.EnvelopeHeaderV4) *types.Manifest_v2 {
	fragments := make([]types.Fragment_v2, len(v4.Segments))
	for i, seg := range v4.Segments {
		nonce, _ := base64.StdEncoding.DecodeString(seg.Nonce)
		encDataSize := seg.Size - uint64(types.SegmentHeaderSize) - uint64(len(nonce))
		fragments[i] = types.Fragment_v2{
			ID:       seg.ID,
			Type:     types.FragmentType_SeekableStream,
			Length:   encDataSize,
			PhysicalOffset: seg.Offset + uint64(types.SegmentHeaderSize) + uint64(len(nonce)),
			DataCRC32:      0,
		}
	}

	var kind types.IndexKind
	switch v4.ContainerType {
	case "video":
		kind = "video"
	case "audio":
		kind = "audio"
	case "image":
		kind = "image"
	case "document":
		kind = "PDF"
	case "text":
		kind = "text"
	default:
		kind = types.IndexKind(v4.ContainerType)
	}

	return &types.Manifest_v2{
		Version:   int64(header.Version),
		Kind:      kind,
		KVI:       v4.KVI,
		Fragments: fragments,
	}
}

// ScanManifestFromFile 从头扫描文件，寻找 Manifest 块
func ScanManifestFromFile(filePath string) (*types.Manifest_v2, int, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, 0, err
	}
	defer file.Close()

	version, headerSize, err := types.DetectHeaderInfoFromReaderAt(file)
	if err != nil {
		return nil, 0, 0, err
	}

	if _, err := file.Seek(headerSize, io.SeekStart); err != nil {
		return nil, 0, 0, err
	}

	for {
		header, err := block.ReadBlockHeader_v2(file)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, 0, 0, err
		}

		if header.Type == types.BlockTypeManifest_v2 {
			// 调用解密辅助函数
			data, err := readBlockDataAndDecrypt(file, header)
			if err != nil {
				return nil, 0, 0, err
			}

			var manifest types.Manifest_v2
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, 0, 0, err
			}
			return &manifest, version, headerSize, nil
		}

		if _, err := file.Seek(int64(header.Length), io.SeekCurrent); err != nil {
			return nil, 0, 0, err
		}
	}

	return nil, 0, 0, fmt.Errorf("manifest block not found")
}

// readAndDecryptManifest 从当前位置读取并解密 Manifest
// 这是一个通用的内部函数，可以被 Scan 和 Footer 路径复用
func readAndDecryptManifest(r io.Reader) (*types.Manifest_v2, error) {
	// 1. 读取 Block Header
	header, err := block.ReadBlockHeader_v2(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read block header: %w", err)
	}

	// 2. 读取并解密数据
	plainData, err := readBlockDataAndDecrypt(r, header)
	if err != nil {
		return nil, err
	}

	// 3. 反序列化
	var manifest types.Manifest_v2
	if err := json.Unmarshal(plainData, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	return &manifest, nil
}

// readBlockDataAndDecrypt 读取加密块数据并尝试解密
func readBlockDataAndDecrypt(r io.Reader, header *block.BlockHeader_v2) ([]byte, error) {
	// 1. 读取原始数据 (包含 IV + CipherText)
	rawData := manifestBlockBufPool.Get().([]byte)
	if cap(rawData) < int(header.Length) {
		rawData = make([]byte, header.Length)
	}
	rawData = rawData[:header.Length]
	defer manifestBlockBufPool.Put(rawData[:cap(rawData)])

	if _, err := io.ReadFull(r, rawData); err != nil {
		return nil, fmt.Errorf("failed to read block data: %w", err)
	}

	// 2. 尝试解密 (AES-256-CTR)
	// CTR 模式不需要 Padding，长度必须一致
	plainData, err := DecryptManifest(rawData)
	if err != nil {
		// 如果解密失败，可能是旧版本未加密的 Manifest，或者是损坏数据
		// 尝试直接解析 JSON
		if err := json.Unmarshal(rawData, &types.Manifest_v2{}); err == nil {
			out := append([]byte(nil), rawData...)
			return out, nil // 是未加密的旧版本
		}
		return nil, fmt.Errorf("failed to decrypt manifest block (and raw parse failed): %w", err)
	}

	// 3. 校验 (可选，如果需要对 PlainData 做二次校验，但 BlockHeader 的 CRC 是针对密文的)
	return plainData, nil
}

// ReadEncryptedManifestBlock 直接从加密的字节数组（Header + Data）中读取并解密 Manifest
// 这是一个辅助函数，用于 remote reader 或其他内存操作场景。
// 它封装了读取 Header、读取/解密 Data 以及反序列化的完整流程。
func ReadEncryptedManifestBlock(encryptedData []byte) (*types.Manifest_v2, error) {
	reader := bytes.NewReader(encryptedData)

	// 1. 读取 Block Header
	header, err := block.ReadBlockHeader_v2(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read block header: %w", err)
	}

	// 2. 读取并解密数据 (复用已有的通用逻辑)
	plainData, err := readBlockDataAndDecrypt(reader, header)
	if err != nil {
		return nil, fmt.Errorf("failed to read/decrypt manifest: %w", err)
	}

	// 3. 反序列化 JSON
	var manifest types.Manifest_v2
	if err := json.Unmarshal(plainData, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	return &manifest, nil
}
