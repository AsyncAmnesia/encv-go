// internal/v2/crypto/aes_v2_test.go
package crypto

import (
	"bytes"
	"crypto/aes"
	"strings"
	"testing"
)

func TestEncryptDecrypt_v2(t *testing.T) {
	password := "my-super-secret-password"
	salt, err := GenerateSalt_v2(SaltSize_v2)
	if err != nil {
		t.Fatalf("Failed to generate salt: %v", err)
	}

	key := GenerateKey(password, salt, KeySize_v2)
	iv, err := GenerateIV_v2(IVSize_v2)
	if err != nil {
		t.Fatalf("Failed to generate IV: %v", err)
	}

	// 保存原始字符串
	originalText := "This is a secret message that needs to be encrypted."
	plaintext := strings.NewReader(originalText)
	var ciphertext bytes.Buffer

	err = EncryptStream_v2(plaintext, &ciphertext, key, iv)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	var decrypted bytes.Buffer
	ciphertextReader := bytes.NewReader(ciphertext.Bytes())
	err = DecryptStream_v2(ciphertextReader, &decrypted, key, iv)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	// 比较解密后的字符串与原始字符串
	if decrypted.String() != originalText {
		t.Errorf("Decrypted text does not match original. Got %s", decrypted.String())
	}
}

func TestGenerateSalt_v2_Size16(t *testing.T) {
	salt, err := GenerateSalt_v2(16)
	if err != nil {
		t.Fatalf("GenerateSalt_v2(16) returned error: %v", err)
	}
	if len(salt) != 16 {
		t.Errorf("GenerateSalt_v2(16) returned length %d, want 16", len(salt))
	}
}

func TestGenerateSalt_v2_Size32(t *testing.T) {
	salt, err := GenerateSalt_v2(32)
	if err != nil {
		t.Fatalf("GenerateSalt_v2(32) returned error: %v", err)
	}
	if len(salt) != 32 {
		t.Errorf("GenerateSalt_v2(32) returned length %d, want 32", len(salt))
	}
}

func TestGenerateSalt_v2_ZeroSize(t *testing.T) {
	salt, err := GenerateSalt_v2(0)
	if err != nil {
		t.Fatalf("GenerateSalt_v2(0) returned unexpected error: %v", err)
	}
	if len(salt) != 0 {
		t.Errorf("GenerateSalt_v2(0) returned length %d, want 0", len(salt))
	}
}

func TestGenerateSalt_v2_Deterministic(t *testing.T) {
	salt1, err := GenerateSalt_v2(16)
	if err != nil {
		t.Fatalf("first GenerateSalt_v2(16) failed: %v", err)
	}
	salt2, err := GenerateSalt_v2(16)
	if err != nil {
		t.Fatalf("second GenerateSalt_v2(16) failed: %v", err)
	}
	if bytes.Equal(salt1, salt2) {
		t.Error("GenerateSalt_v2 produced identical salts; salts should be random")
	}
}

func TestGenerateIV_v2_CorrectSize(t *testing.T) {
	iv, err := GenerateIV_v2(aes.BlockSize)
	if err != nil {
		t.Fatalf("GenerateIV_v2(%d) returned error: %v", aes.BlockSize, err)
	}
	if len(iv) != aes.BlockSize {
		t.Errorf("GenerateIV_v2(%d) returned length %d, want %d", aes.BlockSize, len(iv), aes.BlockSize)
	}
}

func TestGenerateIV_v2_ZeroSize(t *testing.T) {
	iv, err := GenerateIV_v2(0)
	if err != nil {
		t.Fatalf("GenerateIV_v2(0) returned unexpected error: %v", err)
	}
	if len(iv) != 0 {
		t.Errorf("GenerateIV_v2(0) returned length %d, want 0", len(iv))
	}
}

func TestBase64Encode_v2_Roundtrip(t *testing.T) {
	original := []byte("hello world! \x00\x01\x02\xff\xfe")
	encoded := Base64Encode_v2(original)
	decoded, err := Base64Decode_v2(encoded)
	if err != nil {
		t.Fatalf("Base64Decode_v2 failed: %v", err)
	}
	if !bytes.Equal(decoded, original) {
		t.Errorf("Roundtrip mismatch: got %x, want %x", decoded, original)
	}
}

func TestBase64Encode_v2_Empty(t *testing.T) {
	encoded := Base64Encode_v2([]byte{})
	if encoded != "" {
		t.Errorf("Base64Encode_v2(empty) returned %q, want empty string", encoded)
	}
}

func TestEncryptDecryptBytes_v2_Roundtrip(t *testing.T) {
	password := "test-password-123"
	salt, _ := GenerateSalt_v2(SaltSize_v2)
	key := GenerateKey(password, salt, KeySize_v2)
	iv, _ := GenerateIV_v2(IVSize_v2)

	plaintext := []byte("EncryptBytes_v2 round-trip test data with some binary: \x00\xff\xab\xcd")

	ciphertext, err := EncryptBytes_v2(plaintext, key, iv)
	if err != nil {
		t.Fatalf("EncryptBytes_v2 failed: %v", err)
	}

	decrypted, err := DecryptBytes_v2(ciphertext, key, iv)
	if err != nil {
		t.Fatalf("DecryptBytes_v2 failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Roundtrip mismatch:\n  plaintext : %x\n  decrypted: %x", plaintext, decrypted)
	}
}
