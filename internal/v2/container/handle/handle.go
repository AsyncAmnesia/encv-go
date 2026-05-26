package handle

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Soltus/encv-go/internal/v2/container/block"
	"github.com/Soltus/encv-go/internal/v2/container/envelope"
	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/types"
)

type ContainerHandle interface {
	Version() int
	HeaderSize() int64
	ContainerType() uint16
	IsSeekable() bool
	ContainerID() string
	OriginalDuration() float64
	Manifest() *types.Manifest_v2
	ManifestV4() *types.Manifest_v4
	HeaderV2() *types.EnvelopeHeader_v2
	HeaderV3() *types.EnvelopeHeaderV3
	HeaderV4() *types.EnvelopeHeaderV4
	FooterV2() *types.EnvelopeFooter_v2
	FooterV4() *types.EnvelopeFooterV4
	Source() ContainerSource
	Close() error
}

type containerHandle struct {
	source     ContainerSource
	version    int
	headerSize int64

	headerV2 *types.EnvelopeHeader_v2
	headerV3 *types.EnvelopeHeaderV3
	footerV2 *types.EnvelopeFooter_v2

	headerV4 *types.EnvelopeHeaderV4
	footerV4 *types.EnvelopeFooterV4

	manifestV2 *types.Manifest_v2
	manifestV4 *types.Manifest_v4

	containerType    uint16
	isSeekable       bool
	containerID      string
	originalDuration float64
}

func Open(source ContainerSource) (ContainerHandle, error) {
	h := &containerHandle{source: source}

	magic := make([]byte, 6)
	if _, err := source.ReadAt(magic, 0); err != nil {
		return nil, fmt.Errorf("failed to read header magic: %w", err)
	}

	h.version = types.DetectHeaderVersion(magic)
	if h.version == 0 {
		return nil, fmt.Errorf("not an ENCV container (bad magic)")
	}

	switch h.version {
	case 4:
		if err := h.openV4(); err != nil {
			return nil, err
		}
	case 3:
		if err := h.openV3(); err != nil {
			return nil, err
		}
	case 2:
		if err := h.openV2(); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported container version: %d", h.version)
	}

	h.deriveAttributes()

	return h, nil
}

func (h *containerHandle) openV4() error {
	h.headerSize = types.EnvelopeHeaderSize_v4

	headerReader := io.NewSectionReader(h.source, 0, h.headerSize)
	hdr, err := types.ReadHeaderV4(headerReader)
	if err != nil {
		return fmt.Errorf("failed to parse v4 header: %w", err)
	}
	h.headerV4 = hdr

	footerSize := int64(types.EnvelopeFooterSize_v4)
	srcSize := h.source.Size()
	if srcSize < footerSize {
		return fmt.Errorf("file too small for v4 footer")
	}

	footerReader := io.NewSectionReader(h.source, srcSize-footerSize, footerSize)
	v4Footer, err := types.ReadFooterV4(footerReader)
	if err != nil {
		return fmt.Errorf("failed to parse v4 footer: %w", err)
	}
	h.footerV4 = v4Footer

	if hdr.ManifestLength == 0 || hdr.ManifestOffset == 0 {
		return fmt.Errorf("v4 header has invalid manifest offset/length")
	}

	obfuscatedManifest := make([]byte, hdr.ManifestLength)
	if _, err := h.source.ReadAt(obfuscatedManifest, int64(hdr.ManifestOffset)); err != nil {
		return fmt.Errorf("failed to read v4 manifest data: %w", err)
	}

	plainManifest, err := crypto.DeobfuscateManifest(obfuscatedManifest)
	if err != nil {
		return fmt.Errorf("failed to deobfuscate v4 manifest: %w", err)
	}

	v4Manifest, err := types.DeserializeManifest_v4(plainManifest)
	if err != nil {
		return fmt.Errorf("failed to deserialize v4 manifest: %w", err)
	}
	h.manifestV4 = v4Manifest

	h.manifestV2 = AdaptV4ToV2(v4Manifest, hdr)

	return nil
}

func (h *containerHandle) openV3() error {
	h.headerSize = types.EnvelopeHeaderSize_v3

	headerReader := io.NewSectionReader(h.source, 0, h.headerSize)
	hdr, err := types.ReadHeaderV3(headerReader)
	if err != nil {
		return fmt.Errorf("failed to parse v3 header: %w", err)
	}
	h.headerV3 = hdr

	return h.openV23Common()
}

func (h *containerHandle) openV2() error {
	h.headerSize = types.EnvelopeHeaderSize_v2
	return h.openV23Common()
}

func (h *containerHandle) openV23Common() error {
	footerSize := int64(types.EnvelopeFooterSize_v2)
	srcSize := h.source.Size()
	if srcSize < footerSize {
		return fmt.Errorf("file too small for v2/v3 footer")
	}

	footerData := make([]byte, footerSize)
	if _, err := h.source.ReadAt(footerData, srcSize-footerSize); err != nil {
		return fmt.Errorf("failed to read v2/v3 footer: %w", err)
	}

	ftr, err := envelope.ParseEnvelopeFooterFromBytes(footerData)
	if err != nil {
		return fmt.Errorf("failed to parse v2/v3 footer: %w", err)
	}
	h.footerV2 = ftr

	manifestRemaining := srcSize - int64(ftr.ManifestOffset)
	if manifestRemaining <= 0 {
		return fmt.Errorf("invalid manifest offset in footer")
	}

	blockReader := io.NewSectionReader(h.source, int64(ftr.ManifestOffset), manifestRemaining)
	mf, err := readAndDecryptManifest(blockReader)
	if err != nil {
		return fmt.Errorf("failed to read manifest block: %w", err)
	}
	h.manifestV2 = mf

	return nil
}

func readAndDecryptManifest(r io.Reader) (*types.Manifest_v2, error) {
	blockHeader, err := block.ReadBlockHeader_v2(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read block header: %w", err)
	}

	rawData := make([]byte, blockHeader.Length)
	if _, err := io.ReadFull(r, rawData); err != nil {
		return nil, fmt.Errorf("failed to read block data: %w", err)
	}

	var plainData []byte
	plainData, err = crypto.DecryptSystemPayload(rawData)
	if err != nil {
		var check types.Manifest_v2
		if json.Unmarshal(rawData, &check) == nil {
			out := make([]byte, len(rawData))
			copy(out, rawData)
			plainData = out
		} else {
			return nil, fmt.Errorf("failed to decrypt manifest block (and raw parse failed): %w", err)
		}
	}

	var manifest types.Manifest_v2
	if err := json.Unmarshal(plainData, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}
	return &manifest, nil
}

func (h *containerHandle) deriveAttributes() {
	if h.version == 4 && h.headerV4 != nil {
		h.containerType = h.headerV4.ContainerType
		h.isSeekable = h.headerV4.IsSeekable == 1
		h.containerID = ""
		if h.manifestV4 != nil {
			h.containerID = h.manifestV4.ContainerID
			h.originalDuration = h.manifestV4.OriginalDuration
		}
	} else if h.manifestV2 != nil {
		h.containerType = indexKindToContainerType(h.manifestV2.Kind)
		h.isSeekable = hasSeekableFragment(h.manifestV2)
		h.containerID = ""
		h.originalDuration = 0
	}
}

func (h *containerHandle) Version() int                  { return h.version }
func (h *containerHandle) HeaderSize() int64             { return h.headerSize }
func (h *containerHandle) ContainerType() uint16         { return h.containerType }
func (h *containerHandle) IsSeekable() bool              { return h.isSeekable }
func (h *containerHandle) ContainerID() string           { return h.containerID }
func (h *containerHandle) OriginalDuration() float64     { return h.originalDuration }
func (h *containerHandle) Manifest() *types.Manifest_v2  { return h.manifestV2 }
func (h *containerHandle) ManifestV4() *types.Manifest_v4 { return h.manifestV4 }
func (h *containerHandle) HeaderV2() *types.EnvelopeHeader_v2 { return h.headerV2 }
func (h *containerHandle) HeaderV3() *types.EnvelopeHeaderV3  { return h.headerV3 }
func (h *containerHandle) HeaderV4() *types.EnvelopeHeaderV4  { return h.headerV4 }
func (h *containerHandle) FooterV2() *types.EnvelopeFooter_v2 { return h.footerV2 }
func (h *containerHandle) FooterV4() *types.EnvelopeFooterV4  { return h.footerV4 }
func (h *containerHandle) Source() ContainerSource            { return h.source }

func (h *containerHandle) Close() error {
	if closer, ok := h.source.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}
