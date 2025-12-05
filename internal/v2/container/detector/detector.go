package detector

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/Soltus/encv-go/internal/v2/container/manifest"
	"github.com/Soltus/encv-go/internal/v2/types"
)

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

	// 2. 后续逻辑保持不变
	fmt.Printf("DEBUG: [Detector] Reading manifest using manifest.ReadManifestFromFile...\n")
	manifest, _, err := manifest.ReadManifestFromFile(filePath)
	if err != nil {
		fmt.Printf("DEBUG: [Detector] FAILED: manifest.ReadManifestFromFile returned an error: %v\n", err)
		return nil, fmt.Errorf("failed to read manifest for detection: %w", err)
	}
	fmt.Printf("DEBUG: [Detector] Manifest read successfully. Contains %d fragments.\n", len(manifest.Fragments))

	for _, frag := range manifest.Fragments {
		if frag.Type == types.FragmentType_SeekableStream {
			fmt.Printf("DEBUG: [Detector] Detection successful. Container is SEEKABLE.\n")
			return &types.ContainerDescriptor{FilePath: filePath, IsSeekable: true}, nil
		}
	}

	fmt.Printf("DEBUG: [Detector] Detection successful. Container is NOT SEEKABLE (atomic).\n")
	return &types.ContainerDescriptor{FilePath: filePath, IsSeekable: false}, nil
}

// DetectIndexKind 读取容器文件并返回其内部索引的类型（例如 "video", "archive"）。
// 此函数现在完全依赖接口，实现了类型安全、高效且可扩展的检测。
func DetectIndexKind(filePath string) (types.IndexKind, error) {
	footer, err := manifest.ReadFooterFromFile(filePath)
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
