package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"hash/crc32"
	"io"
)

type SegmentEncryptionResult struct {
	SegmentID     uint32
	Nonce         []byte
	EncryptedData []byte
	DataCRC32     uint32
}

func EncryptSegment(plainData []byte, key []byte, segmentID uint32) (*SegmentEncryptionResult, error) {
	nonce, err := GenerateIV_v2(IVSize_v2)
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce for segment %d: %w", segmentID, err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block for segment %d: %w", segmentID, err)
	}

	stream := cipher.NewCTR(block, nonce)
	encryptedData := make([]byte, len(plainData))
	stream.XORKeyStream(encryptedData, plainData)

	dataCRC32 := crc32.ChecksumIEEE(encryptedData)

	return &SegmentEncryptionResult{
		SegmentID:     segmentID,
		Nonce:         nonce,
		EncryptedData: encryptedData,
		DataCRC32:     dataCRC32,
	}, nil
}

func DecryptSegment(encryptedData []byte, nonce []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %w", err)
	}

	if len(nonce) != aes.BlockSize {
		return nil, ErrInvalidIVLength_v2
	}

	stream := cipher.NewCTR(block, nonce)
	plaintext := make([]byte, len(encryptedData))
	stream.XORKeyStream(plaintext, encryptedData)

	return plaintext, nil
}

func EncryptStreamToSegments(src io.Reader, key []byte, segmentSize int64) ([]*SegmentEncryptionResult, error) {
	var results []*SegmentEncryptionResult
	buf := make([]byte, segmentSize)
	segmentID := uint32(0)

	for {
		n, err := io.ReadFull(src, buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			if err == io.ErrUnexpectedEOF {
				if n == 0 {
					break
				}
				result, encErr := EncryptSegment(buf[:n], key, segmentID)
				if encErr != nil {
					return nil, fmt.Errorf("failed to encrypt segment %d: %w", segmentID, encErr)
				}
				results = append(results, result)
				break
			}
			return nil, fmt.Errorf("failed to read segment %d: %w", segmentID, err)
		}

		result, encErr := EncryptSegment(buf[:n], key, segmentID)
		if encErr != nil {
			return nil, fmt.Errorf("failed to encrypt segment %d: %w", segmentID, encErr)
		}
		results = append(results, result)
		segmentID++
	}

	return results, nil
}
