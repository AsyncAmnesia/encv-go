package detector

import (
	"bytes"
	"encoding/binary"
	"fmt"

	containerhandle "github.com/Soltus/encv-go/internal/v2/container/handle"
	"github.com/Soltus/encv-go/internal/v2/container/envelope"
	"github.com/Soltus/encv-go/internal/v2/types"
)

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

func DetectContainer(filePath string) (*types.ContainerDescriptor, error) {
	src, err := containerhandle.NewFileSource(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	h, err := containerhandle.Open(src)
	if err != nil {
		return nil, fmt.Errorf("detection failed: %w", err)
	}
	defer h.Close()

	return &types.ContainerDescriptor{
		FilePath:   filePath,
		IsSeekable: h.IsSeekable(),
	}, nil
}

func DetectContainerType(path string) (uint16, error) {
	src, err := containerhandle.NewFileSource(path)
	if err != nil {
		return types.ContainerTypeUnknown, err
	}
	defer src.Close()

	h, err := containerhandle.Open(src)
	if err != nil {
		return types.ContainerTypeUnknown, err
	}
	defer h.Close()

	return h.ContainerType(), nil
}

func DetectIsSeekable(path string) (bool, error) {
	src, err := containerhandle.NewFileSource(path)
	if err != nil {
		return false, err
	}
	defer src.Close()

	h, err := containerhandle.Open(src)
	if err != nil {
		return false, err
	}
	defer h.Close()

	return h.IsSeekable(), nil
}

func DetectV4Header(path string) (*types.EnvelopeHeaderV4, error) {
	src, err := containerhandle.NewFileSource(path)
	if err != nil {
		return nil, err
	}
	defer src.Close()

	h, err := containerhandle.Open(src)
	if err != nil {
		return nil, err
	}
	defer h.Close()

	if h.Version() != 4 {
		return nil, fmt.Errorf("file is not a v4 container (detected version: %d)", h.Version())
	}
	return h.HeaderV4(), nil
}

func DetectIndexKind(filePath string) (types.IndexKind, error) {
	src, err := containerhandle.NewFileSource(filePath)
	if err != nil {
		return "", fmt.Errorf("invalid container (cannot open file): %w", err)
	}
	defer src.Close()

	h, err := containerhandle.Open(src)
	if err != nil {
		return "", fmt.Errorf("invalid container (cannot detect header): %w", err)
	}
	defer h.Close()

	mf := h.Manifest()
	if mf != nil && mf.Kind != "" {
		return mf.Kind, nil
	}
	return "", fmt.Errorf("could not determine index kind from manifest")
}
