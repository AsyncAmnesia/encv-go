package types

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
)

const (
	EnvelopeHeaderSize_v4 = 2048
	EnvelopeFooterSize_v4 = 12
)

const (
	ContainerTypeUnknown  uint16 = 0
	ContainerTypeVideo    uint16 = 1
	ContainerTypeAudio    uint16 = 2
	ContainerTypeImage    uint16 = 3
	ContainerTypeDocument uint16 = 4
	ContainerTypeText     uint16 = 5
)

type EnvelopeHeaderV4 struct {
	Magic          [4]byte
	Version        uint16
	Flags          uint16
	ContainerType  uint16
	IsSeekable     uint8
	Reserved1      uint8
	IDType         uint32
	IDLength       uint32
	Reserved2      [8]byte
	SpecialID      [SpecialIDMaxLen]byte
	ManifestOffset uint64
	ManifestLength uint64
	HeaderCRC32    uint32
}

type EnvelopeFooterV4 struct {
	Magic        [4]byte
	GlobalCRC32  uint32
	Reserved     [4]byte
}

func WriteHeaderV4(w io.Writer, h *EnvelopeHeaderV4) error {
	h.HeaderCRC32 = 0

	buf := make([]byte, EnvelopeHeaderSize_v4)

	copy(buf[0:4], h.Magic[:])
	binary.LittleEndian.PutUint16(buf[4:6], h.Version)
	binary.LittleEndian.PutUint16(buf[6:8], h.Flags)
	binary.LittleEndian.PutUint16(buf[8:10], h.ContainerType)
	buf[10] = h.IsSeekable
	buf[11] = h.Reserved1
	binary.LittleEndian.PutUint32(buf[12:16], h.IDType)
	binary.LittleEndian.PutUint32(buf[16:20], h.IDLength)
	copy(buf[20:28], h.Reserved2[:])
	copy(buf[28:28+SpecialIDMaxLen], h.SpecialID[:])
	binary.LittleEndian.PutUint64(buf[2028:2036], h.ManifestOffset)
	binary.LittleEndian.PutUint64(buf[2036:2044], h.ManifestLength)

	crc := crc32.ChecksumIEEE(buf[:EnvelopeHeaderSize_v4-4])
	binary.LittleEndian.PutUint32(buf[EnvelopeHeaderSize_v4-4:EnvelopeHeaderSize_v4], crc)
	h.HeaderCRC32 = crc

	_, err := w.Write(buf)
	return err
}

func ReadHeaderV4(r io.Reader) (*EnvelopeHeaderV4, error) {
	buf := make([]byte, EnvelopeHeaderSize_v4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("failed to read header bytes: %w", err)
	}

	storedCRC := binary.LittleEndian.Uint32(buf[EnvelopeHeaderSize_v4-4 : EnvelopeHeaderSize_v4])
	calculatedCRC := crc32.ChecksumIEEE(buf[:EnvelopeHeaderSize_v4-4])
	if storedCRC != calculatedCRC {
		return nil, fmt.Errorf("header CRC32 mismatch: stored=%08x, calculated=%08x", storedCRC, calculatedCRC)
	}

	header := &EnvelopeHeaderV4{}
	if err := binary.Read(bytes.NewReader(buf), binary.LittleEndian, header); err != nil {
		return nil, fmt.Errorf("failed to unmarshal header: %w", err)
	}

	if header.Magic != MagicHeader_v2 {
		return nil, ErrInvalidMagic_v2
	}

	return header, nil
}

func WriteFooterV4(w io.Writer, f *EnvelopeFooterV4) error {
	buf := make([]byte, EnvelopeFooterSize_v4)

	copy(buf[0:4], f.Magic[:])
	binary.LittleEndian.PutUint32(buf[4:8], f.GlobalCRC32)
	copy(buf[8:12], f.Reserved[:])

	_, err := w.Write(buf)
	return err
}

func ReadFooterV4(r io.Reader) (*EnvelopeFooterV4, error) {
	buf := make([]byte, EnvelopeFooterSize_v4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("failed to read footer bytes: %w", err)
	}

	footer := &EnvelopeFooterV4{}
	if err := binary.Read(bytes.NewReader(buf), binary.LittleEndian, footer); err != nil {
		return nil, fmt.Errorf("failed to unmarshal footer: %w", err)
	}

	if footer.Magic != MagicFooter_v2 {
		return nil, ErrInvalidMagic_v2
	}

	return footer, nil
}

func CreateHeaderV4(isMain bool, containerType uint16, isSeekable bool, idType IDType, idData []byte) (*EnvelopeHeaderV4, error) {
	if idData == nil {
		if idType == IDType_Raw {
			var err error
			idData, err = GeneratePlaceholderIDV3()
			if err != nil {
				return nil, fmt.Errorf("failed to generate placeholder ID: %w", err)
			}
		} else {
			return nil, fmt.Errorf("idData is required for IDType %d", idType)
		}
	}

	if len(idData) > SpecialIDMaxLen {
		return nil, fmt.Errorf("special ID data exceeds max length %d", SpecialIDMaxLen)
	}

	flags := uint16(0)
	if isMain {
		flags |= FlagIsMainContainer
	} else {
		flags |= FlagIsPhysicalChunk
	}

	seekable := uint8(0)
	if isSeekable {
		seekable = 0x01
	}

	header := &EnvelopeHeaderV4{
		Magic:         MagicHeader_v2,
		Version:       0x04,
		Flags:         flags,
		ContainerType: containerType,
		IsSeekable:    seekable,
		IDType:        uint32(idType),
	}
	copy(header.SpecialID[:], idData)
	header.IDLength = uint32(len(idData))

	return header, nil
}
