package detector

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/Soltus/encv-go/internal/v2/container/envelope"
	"github.com/Soltus/encv-go/internal/v2/container/manifest"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// 从字节数组判断是否为 ENCV 容器，适用于网络内容
func IsEncvContainerFromBytes(data []byte) (bool, error) {
	size := types.EnvelopeFooterSize_v2
	if len(data) < size {
		return false, nil
	}
	footerData := data[len(data)-size:]

	// 使用 envelope 包的新权威解析函数
	_, err := envelope.ParseEnvelopeFooterFromBytes(footerData)
	return err == nil, nil
}

// DetectContainer 分析给定的容器文件，返回其描述符
// 这是判断一个文件是否为有效 ENCV 容器的权威函数。
func DetectContainer(filePath string) (*types.ContainerDescriptor, error) {
	fmt.Printf("DEBUG: [Detector] Starting detection for file: %s\n", filePath)

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("DEBUG: [Detector] FAILED: Could not open file: %v\n", err)
		return nil, fmt.Errorf("failed to open file for detection: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("could not stat file: %w", err)
	}

	// 1. 【关键重构】使用 io.SectionReader 读取文件末尾的 Footer，彻底消除 Seek
	fmt.Printf("DEBUG: [Detector] Reading footer from the END of the file using SectionReader...\n")

	footerSize := int64(binary.Size(types.EnvelopeFooter_v2{}))
	if fileInfo.Size() < footerSize {
		fmt.Printf("DEBUG: [Detector] FAILED: File size %d is smaller than footer size %d.\n", fileInfo.Size(), footerSize)
		return nil, fmt.Errorf("file is not a valid ENCV container")
	}

	// 创建一个只包含文件末尾 Footer 部分的 SectionReader
	footerReader := io.NewSectionReader(file, fileInfo.Size()-footerSize, footerSize)

	footer := &types.EnvelopeFooter_v2{}
	if err := binary.Read(footerReader, types.ByteOrder_v2, footer); err != nil {
		fmt.Printf("DEBUG: [Detector] FAILED: binary.Read failed while reading footer: %v\n", err)
		return nil, fmt.Errorf("failed to read footer: %w", err)
	}
	fmt.Printf("DEBUG: [Detector] Read footer from file '%s'. Magic: '%s'\n", filePath, string(footer.Magic[:]))

	if !bytes.Equal(footer.Magic[:], types.MagicFooter_v2[:]) {
		fmt.Printf("DEBUG: [Detector] FAILED: Footer magic number mismatch. Expected: %s, Got: %s\n", types.MagicFooter_v2, string(footer.Magic[:]))
		return nil, fmt.Errorf("file is not a valid ENCV container")
	}
	fmt.Printf("DEBUG: [Detector] Footer magic number is valid. ManifestOffset: %d\n", footer.ManifestOffset)

	// 2. 【新增】探测 Header 版本，用于日志记录
	_, _, err = types.DetectHeaderInfoFromReaderAt(file)
	if err != nil {
		return nil, fmt.Errorf("failed to detect header: %w", err)
	}

	// 3. 根据 Footer 中的绝对偏移量读取 Manifest
	// 注意：无论是 V2 还是 V3，Footer 中的 ManifestOffset 都是绝对偏移量。
	// manifest.ReadManifestAt 会 Seek 到该偏移量并读取 Block，因此兼容 V3。
	fmt.Printf("DEBUG: [Detector] Reading manifest at absolute offset %d...\n", footer.ManifestOffset)
	manifest, _, _, _, err := manifest.ReadManifestFromFile(filePath)
	if err != nil {
		fmt.Printf("DEBUG: [Detector] FAILED: manifest.ReadManifestFromFile returned an error: %v\n", err)
		return nil, fmt.Errorf("failed to read manifest for detection: %w", err)
	}
	fmt.Printf("DEBUG: [Detector] Manifest read successfully. Contains %d fragments.\n", len(manifest.Fragments))

	// 遍历所有 fragments，只要有一个 SeekableStream，就认为容器是可寻址的
	for _, frag := range manifest.Fragments {
		if frag.Type == types.FragmentType_SeekableStream {
			fmt.Printf("DEBUG: [Detector] Detection successful. Container is SEEKABLE.\n")
			return &types.ContainerDescriptor{FilePath: filePath, IsSeekable: true}, nil
		}
	}
	// 如果没有 SeekableStream，那么它就是原子的（或者为空，这也是原子的）
	fmt.Printf("DEBUG: [Detector] Detection successful. Container is NOT SEEKABLE (atomic).\n")
	return &types.ContainerDescriptor{FilePath: filePath, IsSeekable: false}, nil
}

// DetectIndexKind 读取容器文件并返回其内部索引的类型（例如 "video", "archive"）。
// 此函数现在完全依赖接口，实现了类型安全、高效且可扩展的检测。
func DetectIndexKind(filePath string) (types.IndexKind, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("invalid container (cannot open file): %w", err)
	}
	defer file.Close()
	footer, err := envelope.ReadEnvelopeFooter_v2(file)
	if err != nil {
		return "", fmt.Errorf("invalid container (cannot read footer): %w", err)
	}

	manifestBytes, err := manifest.ReadManifestAt(filePath, int64(footer.ManifestOffset), int64(footer.ManifestLength))
	if err != nil {
		return "", fmt.Errorf("invalid container (cannot read manifest): %w", err)
	}

	var newManifest types.Manifest_v2
	if err := json.Unmarshal(manifestBytes, &newManifest); err == nil && newManifest.Kind != "" {
		return newManifest.Kind, nil
	}

	return "", fmt.Errorf("could not determine index kind from manifest")
}
