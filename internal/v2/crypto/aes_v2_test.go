// internal/v2/crypto/aes_v2_test.go
package crypto

import (
	"bytes"
	"strings"
	"testing"
)

func TestEncryptDecrypt_v2(t *testing.T) {
	password := "my-super-secret-password"
	salt, err := GenerateSalt_v2(SaltSize_v2)
	if err != nil {
		t.Fatalf("Failed to generate salt: %v", err)
	}

	key := GenerateKey_v2(password, salt, KeySize_v2)
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
