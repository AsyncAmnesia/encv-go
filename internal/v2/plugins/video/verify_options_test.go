package video

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Soltus/encv-go/internal/v2/plugins/interfaces"
)

func TestVerify_SkipSizeCheck_Mode(t *testing.T) {
	verifier := newVerifier()

	t.Run("different_sizes_no_size_mismatch_error", func(t *testing.T) {
		dir := t.TempDir()

		origPath := filepath.Join(dir, "original.bin")
		origData := make([]byte, 8192)
		for i := range origData {
			origData[i] = byte(i % 256)
		}
		if err := os.WriteFile(origPath, origData, 0644); err != nil {
			t.Fatalf("failed to write original: %v", err)
		}

		decPath := filepath.Join(dir, "decrypted.bin")
		decData := make([]byte, 4096)
		for i := range decData {
			decData[i] = byte((i + 1) % 256)
		}
		if err := os.WriteFile(decPath, decData, 0644); err != nil {
			t.Fatalf("failed to write decrypted: %v", err)
		}

		opts := &interfaces.VerifyOptions{SkipSizeCheck: true}
		err := verifier.Verify(origPath, decPath, opts)

		if err != nil && strings.Contains(err.Error(), "size mismatch") {
			t.Fatalf("SkipSizeCheck=true should not return size mismatch error, got: %v", err)
		}
		if err == nil {
			t.Log("Verify passed (acceptable: re-encode mode allows content difference)")
		} else {
			t.Logf("Verify returned non-size error (expected for different content): %v", err)
		}
	})
}

func TestVerify_DefaultMode_SizeMismatch(t *testing.T) {
	verifier := newVerifier()

	t.Run("different_sizes_returns_error", func(t *testing.T) {
		dir := t.TempDir()

		origPath := filepath.Join(dir, "original.bin")
		origData := make([]byte, 8192)
		if err := os.WriteFile(origPath, origData, 0644); err != nil {
			t.Fatalf("failed to write original: %v", err)
		}

		decPath := filepath.Join(dir, "decrypted.bin")
		decData := make([]byte, 4096)
		if err := os.WriteFile(decPath, decData, 0644); err != nil {
			t.Fatalf("failed to write decrypted: %v", err)
		}

		err := verifier.Verify(origPath, decPath)

		if err == nil {
			t.Fatal("expected size mismatch error in default mode, got nil")
		}
		if !strings.Contains(err.Error(), "size mismatch") {
			t.Fatalf("expected 'size mismatch' error, got: %v", err)
		}
	})

	t.Run("same_sizes_no_error_from_size_check", func(t *testing.T) {
		dir := t.TempDir()
		data := make([]byte, 1024)

		origPath := filepath.Join(dir, "original.bin")
		if err := os.WriteFile(origPath, data, 0644); err != nil {
			t.Fatalf("failed to write original: %v", err)
		}

		decPath := filepath.Join(dir, "decrypted.bin")
		if err := os.WriteFile(decPath, data, 0644); err != nil {
			t.Fatalf("failed to write decrypted: %v", err)
		}

		err := verifier.Verify(origPath, decPath)

		if err != nil && strings.Contains(err.Error(), "size mismatch") {
			t.Fatalf("identical files should not produce size mismatch, got: %v", err)
		}
	})
}

func TestVerify_SkipSizeCheck_StillChecksStructure(t *testing.T) {
	verifier := newVerifier()

	t.Run("corrupted_data_still_detected_with_skip", func(t *testing.T) {
		dir := t.TempDir()

		data := make([]byte, 4096)

		origPath := filepath.Join(dir, "original.bin")
		if err := os.WriteFile(origPath, data, 0644); err != nil {
			t.Fatalf("failed to write original: %v", err)
		}

		decPath := filepath.Join(dir, "decrypted.bin")
		decData := make([]byte, 4096)
		copy(decData, data)
		decData[0] = 0xFF
		decData[1] = 0x00
		if err := os.WriteFile(decPath, decData, 0644); err != nil {
			t.Fatalf("failed to write decrypted: %v", err)
		}

		opts := &interfaces.VerifyOptions{SkipSizeCheck: true}
		err := verifier.Verify(origPath, decPath, opts)

		if err == nil {
			t.Fatal("expected verification error for corrupted data even with SkipSizeCheck=true")
		}
		t.Logf("correctly detected corruption: %v", err)
	})

	t.Run("nil_options_backward_compatible", func(t *testing.T) {
		dir := t.TempDir()

		largeData := make([]byte, 8192)
		smallData := make([]byte, 4096)

		origPath := filepath.Join(dir, "original.bin")
		if err := os.WriteFile(origPath, largeData, 0644); err != nil {
			t.Fatalf("failed to write original: %v", err)
		}

		decPath := filepath.Join(dir, "decrypted.bin")
		if err := os.WriteFile(decPath, smallData, 0644); err != nil {
			t.Fatalf("failed to write decrypted: %v", err)
		}

		err := verifier.Verify(origPath, decPath)

		if err == nil {
			t.Fatal("expected size mismatch error with no options (backward compat)")
		}
		if !strings.Contains(err.Error(), "size mismatch") {
			t.Fatalf("expected 'size mismatch', got: %v", err)
		}
	})

	t.Run("empty_opts_slice_backward_compatible", func(t *testing.T) {
		dir := t.TempDir()

		largeData := make([]byte, 8192)
		smallData := make([]byte, 4096)

		origPath := filepath.Join(dir, "original.bin")
		if err := os.WriteFile(origPath, largeData, 0644); err != nil {
			t.Fatalf("failed to write original: %v", err)
		}

		decPath := filepath.Join(dir, "decrypted.bin")
		if err := os.WriteFile(decPath, smallData, 0644); err != nil {
			t.Fatalf("failed to write decrypted: %v", err)
		}

		err := verifier.Verify(origPath, decPath, nil)

		if err == nil {
			t.Fatal("expected size mismatch error with nil options")
		}
		if !strings.Contains(err.Error(), "size mismatch") {
			t.Fatalf("expected 'size mismatch', got: %v", err)
		}
	})
}
