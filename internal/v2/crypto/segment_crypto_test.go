package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestEncryptDecryptSegmentRoundTrip(t *testing.T) {
	key := make([]byte, KeySize_v2)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	plainData := []byte("This is a secret segment payload for round-trip testing.")

	result, err := EncryptSegment(plainData, key, 0)
	if err != nil {
		t.Fatalf("EncryptSegment failed: %v", err)
	}

	if result.SegmentID != 0 {
		t.Errorf("Expected SegmentID 0, got %d", result.SegmentID)
	}
	if len(result.Nonce) != IVSize_v2 {
		t.Errorf("Expected nonce length %d, got %d", IVSize_v2, len(result.Nonce))
	}
	if len(result.EncryptedData) != len(plainData) {
		t.Errorf("Expected encrypted data length %d, got %d", len(plainData), len(result.EncryptedData))
	}
	if result.DataCRC32 == 0 {
		t.Error("Expected non-zero CRC32")
	}

	decrypted, err := DecryptSegment(result.EncryptedData, result.Nonce, key)
	if err != nil {
		t.Fatalf("DecryptSegment failed: %v", err)
	}

	if !bytes.Equal(decrypted, plainData) {
		t.Errorf("Decrypted data does not match original.\nExpected: %s\nGot: %s", plainData, decrypted)
	}
}

func TestDifferentSegmentsDifferentCiphertext(t *testing.T) {
	key := make([]byte, KeySize_v2)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	plainData := []byte("Same plaintext for both segments")

	result0, err := EncryptSegment(plainData, key, 0)
	if err != nil {
		t.Fatalf("EncryptSegment 0 failed: %v", err)
	}

	result1, err := EncryptSegment(plainData, key, 1)
	if err != nil {
		t.Fatalf("EncryptSegment 1 failed: %v", err)
	}

	if bytes.Equal(result0.EncryptedData, result1.EncryptedData) {
		t.Error("Different segments with same plaintext and key should produce different ciphertext due to independent nonces")
	}

	if bytes.Equal(result0.Nonce, result1.Nonce) {
		t.Error("Different segments should have different nonces")
	}
}

func TestSegmentIndependence(t *testing.T) {
	key := make([]byte, KeySize_v2)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	plainData0 := []byte("Segment zero payload data")
	plainData1 := []byte("Segment one payload data")

	result0, err := EncryptSegment(plainData0, key, 0)
	if err != nil {
		t.Fatalf("EncryptSegment 0 failed: %v", err)
	}

	result1, err := EncryptSegment(plainData1, key, 1)
	if err != nil {
		t.Fatalf("EncryptSegment 1 failed: %v", err)
	}

	decrypted0, err := DecryptSegment(result0.EncryptedData, result0.Nonce, key)
	if err != nil {
		t.Fatalf("DecryptSegment 0 failed: %v", err)
	}

	decrypted1, err := DecryptSegment(result1.EncryptedData, result1.Nonce, key)
	if err != nil {
		t.Fatalf("DecryptSegment 1 failed: %v", err)
	}

	if !bytes.Equal(decrypted0, plainData0) {
		t.Errorf("Segment 0 decryption failed.\nExpected: %s\nGot: %s", plainData0, decrypted0)
	}

	if !bytes.Equal(decrypted1, plainData1) {
		t.Errorf("Segment 1 decryption failed.\nExpected: %s\nGot: %s", plainData1, decrypted1)
	}

	corruptedData := make([]byte, len(result1.EncryptedData))
	copy(corruptedData, result1.EncryptedData)
	corruptedData[0] ^= 0xFF

	decrypted0Again, err := DecryptSegment(result0.EncryptedData, result0.Nonce, key)
	if err != nil {
		t.Fatalf("DecryptSegment 0 after corruption of segment 1 failed: %v", err)
	}

	if !bytes.Equal(decrypted0Again, plainData0) {
		t.Error("Corrupting segment 1 data should not affect decryption of segment 0")
	}
}

func TestEncryptStreamToSegments(t *testing.T) {
	key := make([]byte, KeySize_v2)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	data := make([]byte, 5000)
	if _, err := rand.Read(data); err != nil {
		t.Fatalf("Failed to generate test data: %v", err)
	}

	segmentSize := int64(2048)
	results, err := EncryptStreamToSegments(bytes.NewReader(data), key, segmentSize)
	if err != nil {
		t.Fatalf("EncryptStreamToSegments failed: %v", err)
	}

	expectedSegments := 3
	if len(results) != expectedSegments {
		t.Fatalf("Expected %d segments, got %d", expectedSegments, len(results))
	}

	if results[0].SegmentID != 0 {
		t.Errorf("Segment 0 ID expected 0, got %d", results[0].SegmentID)
	}
	if results[1].SegmentID != 1 {
		t.Errorf("Segment 1 ID expected 1, got %d", results[1].SegmentID)
	}
	if results[2].SegmentID != 2 {
		t.Errorf("Segment 2 ID expected 2, got %d", results[2].SegmentID)
	}

	if len(results[0].EncryptedData) != 2048 {
		t.Errorf("Segment 0 data length expected 2048, got %d", len(results[0].EncryptedData))
	}
	if len(results[1].EncryptedData) != 2048 {
		t.Errorf("Segment 1 data length expected 2048, got %d", len(results[1].EncryptedData))
	}
	if len(results[2].EncryptedData) != 904 {
		t.Errorf("Segment 2 data length expected 904, got %d", len(results[2].EncryptedData))
	}

	var allDecrypted []byte
	for _, result := range results {
		decrypted, err := DecryptSegment(result.EncryptedData, result.Nonce, key)
		if err != nil {
			t.Fatalf("DecryptSegment %d failed: %v", result.SegmentID, err)
		}
		allDecrypted = append(allDecrypted, decrypted...)
	}

	if !bytes.Equal(allDecrypted, data) {
		t.Errorf("Decrypted stream does not match original data")
	}
}

func TestSegmentIndependentNonces(t *testing.T) {
	key := make([]byte, KeySize_v2)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	data := make([]byte, 3000)
	if _, err := rand.Read(data); err != nil {
		t.Fatalf("Failed to generate test data: %v", err)
	}

	segmentSize := int64(1000)
	results, err := EncryptStreamToSegments(bytes.NewReader(data), key, segmentSize)
	if err != nil {
		t.Fatalf("EncryptStreamToSegments failed: %v", err)
	}

	for i, result := range results {
		if len(result.Nonce) != IVSize_v2 {
			t.Errorf("Segment %d nonce length expected %d, got %d", i, IVSize_v2, len(result.Nonce))
		}
	}

	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if bytes.Equal(results[i].Nonce, results[j].Nonce) {
				t.Errorf("Segment %d and %d have identical nonces, expected independent nonces", i, j)
			}
		}
	}
}
