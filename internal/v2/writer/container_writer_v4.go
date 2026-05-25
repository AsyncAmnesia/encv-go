package writer

import (
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"io"
	"os"

	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/types"
)

type V4WriteParams struct {
	OutputPath     string
	IsMain         bool
	ContainerType  uint16
	IsSeekable     bool
	IDType         types.IDType
	IDData         []byte
	Manifest       *types.Manifest_v4
	SegmentResults []*crypto.SegmentEncryptionResult
	DisasterZones  []types.DisasterZone
}

func WriteV4Container(params *V4WriteParams) error {
	header, err := types.CreateHeaderV4(params.IsMain, params.ContainerType, params.IsSeekable, params.IDType, params.IDData)
	if err != nil {
		return fmt.Errorf("failed to create v4 header: %w", err)
	}

	f, err := os.Create(params.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	if err := types.WriteHeaderV4(f, header); err != nil {
		return fmt.Errorf("failed to write placeholder header: %w", err)
	}

	dataStartOffset := int64(types.EnvelopeHeaderSize_v4)
	globalHasher := crc32.NewIEEE()

	for i, segResult := range params.SegmentResults {
		segHeader := &types.SegmentHeader{
			SegmentID:  segResult.SegmentID,
			DataLength: uint64(len(segResult.EncryptedData)),
			NonceSize:  uint16(len(segResult.Nonce)),
			DataCRC32:  segResult.DataCRC32,
		}

		segHeaderBytes, err := segHeader.MarshalBinary()
		if err != nil {
			return fmt.Errorf("failed to marshal segment header %d: %w", i, err)
		}

		if _, err := f.Write(segHeaderBytes); err != nil {
			return fmt.Errorf("failed to write segment header %d: %w", i, err)
		}
		globalHasher.Write(segHeaderBytes)

		if _, err := f.Write(segResult.Nonce); err != nil {
			return fmt.Errorf("failed to write nonce for segment %d: %w", i, err)
		}
		globalHasher.Write(segResult.Nonce)

		if _, err := f.Write(segResult.EncryptedData); err != nil {
			return fmt.Errorf("failed to write encrypted data for segment %d: %w", i, err)
		}
		globalHasher.Write(segResult.EncryptedData)

		segOffset, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return fmt.Errorf("failed to get current offset after segment %d: %w", i, err)
		}

		segTotalSize := uint64(types.SegmentHeaderSize) + uint64(len(segResult.Nonce)) + uint64(len(segResult.EncryptedData))

		if i < len(params.Manifest.Segments) {
			params.Manifest.Segments[i].Offset = uint64(segOffset) - segTotalSize
			params.Manifest.Segments[i].Size = segTotalSize
			params.Manifest.Segments[i].Nonce = base64.StdEncoding.EncodeToString(segResult.Nonce)
		}
	}

	for _, dz := range params.DisasterZones {
		srcFile, err := os.Open(params.OutputPath)
		if err != nil {
			return fmt.Errorf("failed to open source for disaster zone %s: %w", dz.Name, err)
		}

		dzData := make([]byte, dz.Size)
		if _, err := srcFile.Seek(dz.Offset, io.SeekStart); err != nil {
			srcFile.Close()
			return fmt.Errorf("failed to seek to disaster zone %s: %w", dz.Name, err)
		}
		if _, err := io.ReadFull(srcFile, dzData); err != nil {
			srcFile.Close()
			return fmt.Errorf("failed to read disaster zone %s: %w", dz.Name, err)
		}
		srcFile.Close()

		if _, err := f.Write(dzData); err != nil {
			return fmt.Errorf("failed to write disaster zone %s: %w", dz.Name, err)
		}
		globalHasher.Write(dzData)
	}

	manifestOffset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get manifest offset: %w", err)
	}

	manifestJSON, err := params.Manifest.SerializeToJSON_v4()
	if err != nil {
		return fmt.Errorf("failed to serialize manifest: %w", err)
	}

	obfuscatedManifest, err := crypto.ObfuscateManifest(manifestJSON)
	if err != nil {
		return fmt.Errorf("failed to obfuscate manifest: %w", err)
	}

	if _, err := f.Write(obfuscatedManifest); err != nil {
		return fmt.Errorf("failed to write obfuscated manifest: %w", err)
	}
	globalHasher.Write(obfuscatedManifest)

	manifestLength := uint64(len(obfuscatedManifest))

	allDataBuf := make([]byte, manifestOffset-dataStartOffset)
	if _, err := f.Seek(dataStartOffset, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to data start for crc: %w", err)
	}
	if _, err := io.ReadFull(f, allDataBuf); err != nil {
		return fmt.Errorf("failed to read data for global crc: %w", err)
	}
	globalCRC32 := crc32.ChecksumIEEE(allDataBuf)

	header.ManifestOffset = uint64(manifestOffset)
	header.ManifestLength = manifestLength

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to header: %w", err)
	}
	if err := types.WriteHeaderV4(f, header); err != nil {
		return fmt.Errorf("failed to rewrite header with manifest info: %w", err)
	}

	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("failed to seek to end for footer: %w", err)
	}

	footer := &types.EnvelopeFooterV4{
		Magic:       types.MagicFooter_v2,
		GlobalCRC32: globalCRC32,
	}
	if err := types.WriteFooterV4(f, footer); err != nil {
		return fmt.Errorf("failed to write footer: %w", err)
	}

	return nil
}

func WriteV4EmptyContainer(params *V4WriteParams) error {
	params.SegmentResults = nil
	params.DisasterZones = nil
	return WriteV4Container(params)
}
