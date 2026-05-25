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
	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// 从字节数组判断是否为 ENCV 容器，适用于网络内容
func IsEncvContainerFromBytes(data []byte) (bool, error) {
	if len(data) < 6 {
		return false, nil
	}
	version := types.DetectHeaderVersion(data[:6])
	if version == 0 {
		return false, nil
	}

	if version == 4 {
		footerSize := types.EnvelopeFooterSize_v4
		if len(data) < footerSize {
			return false, nil
		}
		footerData := data[len(data)-footerSize:]
		footer := &types.EnvelopeFooterV4{}
		if err := binary.Read(bytes.NewReader(footerData), binary.LittleEndian, footer); err != nil {
			return false, nil
		}
		return bytes.Equal(footer.Magic[:], types.MagicFooter_v2[:]), nil
	}

	size := types.EnvelopeFooterSize_v2
	if len(data) < size {
		return false, nil
	}
	footerData := data[len(data)-size:]
	_, err := envelope.ParseEnvelopeFooterFromBytes(footerData)
	return err == nil, nil
}

// DetectContainer 分析给定的容器文件，返回其描述符
// 这是判断一个文件是否为有效 ENCV 容器的权威函数。
func DetectContainer(filePath string) (*types.ContainerDescriptor, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for detection: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("could not stat file: %w", err)
	}

	version, _, err := types.DetectHeaderInfoFromReaderAt(file)
	if err != nil {
		return nil, fmt.Errorf("file is not a valid ENCV container: %w", err)
	}

	if version == 4 {
		return detectV4Container(file, fileInfo)
	}

	return detectV3Container(file, fileInfo)
}

func detectV4Container(file *os.File, fileInfo os.FileInfo) (*types.ContainerDescriptor, error) {
	footerSize := int64(types.EnvelopeFooterSize_v4)
	if fileInfo.Size() < footerSize {
		return nil, fmt.Errorf("file is too small for v4 footer")
	}

	footerReader := io.NewSectionReader(file, fileInfo.Size()-footerSize, footerSize)
	footer := &types.EnvelopeFooterV4{}
	if err := binary.Read(footerReader, binary.LittleEndian, footer); err != nil {
		return nil, fmt.Errorf("failed to read v4 footer: %w", err)
	}

	if !bytes.Equal(footer.Magic[:], types.MagicFooter_v2[:]) {
		return nil, fmt.Errorf("file is not a valid ENCV container (v4 footer magic mismatch)")
	}

	header, err := types.ReadHeaderV4(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read v4 header: %w", err)
	}

	isSeekable := header.IsSeekable == 1
	return &types.ContainerDescriptor{FilePath: file.Name(), IsSeekable: isSeekable}, nil
}

func detectV3Container(file *os.File, fileInfo os.FileInfo) (*types.ContainerDescriptor, error) {
	footerSize := int64(binary.Size(types.EnvelopeFooter_v2{}))
	if fileInfo.Size() < footerSize {
		return nil, fmt.Errorf("file is too small for v3 footer")
	}

	footerReader := io.NewSectionReader(file, fileInfo.Size()-footerSize, footerSize)
	footer := &types.EnvelopeFooter_v2{}
	if err := binary.Read(footerReader, types.ByteOrder_v2, footer); err != nil {
		return nil, fmt.Errorf("failed to read footer: %w", err)
	}

	if !bytes.Equal(footer.Magic[:], types.MagicFooter_v2[:]) {
		return nil, fmt.Errorf("file is not a valid ENCV container")
	}

	manifest, _, _, _, err := manifest.ReadManifestFromFile(file.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest for detection: %w", err)
	}

	for _, frag := range manifest.Fragments {
		if frag.Type == types.FragmentType_SeekableStream {
			return &types.ContainerDescriptor{FilePath: file.Name(), IsSeekable: true}, nil
		}
	}
	return &types.ContainerDescriptor{FilePath: file.Name(), IsSeekable: false}, nil
}

// DetectIndexKind 读取容器文件并返回其内部索引的类型（例如 "video", "archive"）。
// 此函数现在完全依赖接口，实现了类型安全、高效且可扩展的检测。
func indexKindToContainerType(kind types.IndexKind) uint16 {
	switch kind {
	case "video":
		return types.ContainerTypeVideo
	case "audio":
		return types.ContainerTypeAudio
	case "image":
		return types.ContainerTypeImage
	case "PDF", "WPS":
		return types.ContainerTypeDocument
	case "text":
		return types.ContainerTypeText
	default:
		return types.ContainerTypeUnknown
	}
}

func DetectContainerType(path string) (uint16, error) {
	file, err := os.Open(path)
	if err != nil {
		return types.ContainerTypeUnknown, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	version, _, err := types.DetectHeaderInfoFromReaderAt(file)
	if err != nil {
		return types.ContainerTypeUnknown, fmt.Errorf("failed to detect header version: %w", err)
	}

	if version == 4 {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return types.ContainerTypeUnknown, fmt.Errorf("failed to seek to start: %w", err)
		}
		header, err := types.ReadHeaderV4(file)
		if err != nil {
			return types.ContainerTypeUnknown, fmt.Errorf("failed to read v4 header: %w", err)
		}
		return header.ContainerType, nil
	}

	kind, err := DetectIndexKind(path)
	if err != nil {
		return types.ContainerTypeUnknown, fmt.Errorf("failed to detect index kind: %w", err)
	}
	return indexKindToContainerType(kind), nil
}

func DetectIsSeekable(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	version, _, err := types.DetectHeaderInfoFromReaderAt(file)
	if err != nil {
		return false, fmt.Errorf("failed to detect header version: %w", err)
	}

	if version == 4 {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return false, fmt.Errorf("failed to seek to start: %w", err)
		}
		header, err := types.ReadHeaderV4(file)
		if err != nil {
			return false, fmt.Errorf("failed to read v4 header: %w", err)
		}
		return header.IsSeekable == 1, nil
	}

	kind, err := DetectIndexKind(path)
	if err != nil {
		return false, fmt.Errorf("failed to detect index kind: %w", err)
	}
	return kind == "video", nil
}

func DetectV4Header(path string) (*types.EnvelopeHeaderV4, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	version, _, err := types.DetectHeaderInfoFromReaderAt(file)
	if err != nil {
		return nil, fmt.Errorf("failed to detect header version: %w", err)
	}

	if version != 4 {
		return nil, fmt.Errorf("file is not a v4 container (detected version: %d)", version)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to start: %w", err)
	}

	header, err := types.ReadHeaderV4(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read v4 header: %w", err)
	}

	return header, nil
}

func DetectIndexKind(filePath string) (types.IndexKind, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("invalid container (cannot open file): %w", err)
	}
	defer file.Close()

	version, _, err := types.DetectHeaderInfoFromReaderAt(file)
	if err != nil {
		return "", fmt.Errorf("invalid container (cannot detect header): %w", err)
	}

	var manifestBytes []byte

	if version == 4 {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return "", fmt.Errorf("failed to seek to start: %w", err)
		}
		header, err := types.ReadHeaderV4(file)
		if err != nil {
			return "", fmt.Errorf("failed to read v4 header: %w", err)
		}

		obfuscatedBytes := make([]byte, header.ManifestLength)
		if _, err := file.Seek(int64(header.ManifestOffset), io.SeekStart); err != nil {
			return "", fmt.Errorf("failed to seek to manifest: %w", err)
		}
		if _, err := io.ReadFull(file, obfuscatedBytes); err != nil {
			return "", fmt.Errorf("failed to read manifest: %w", err)
		}

		deobfuscated, err := crypto.DeobfuscateManifest(obfuscatedBytes)
		if err != nil {
			return "", fmt.Errorf("failed to deobfuscate manifest: %w", err)
		}

		manifestBytes = deobfuscated
	} else {
		footer, err := envelope.ReadEnvelopeFooter_v2(file)
		if err != nil {
			return "", fmt.Errorf("invalid container (cannot read footer): %w", err)
		}
		manifestBytes, err = manifest.ReadManifestAt(filePath, int64(footer.ManifestOffset), int64(footer.ManifestLength))
		if err != nil {
			return "", fmt.Errorf("invalid container (cannot read manifest): %w", err)
		}
	}

	var newManifest types.Manifest_v2
	if err := json.Unmarshal(manifestBytes, &newManifest); err == nil && newManifest.Kind != "" {
		return newManifest.Kind, nil
	}

	return "", fmt.Errorf("could not determine index kind from manifest")
}
