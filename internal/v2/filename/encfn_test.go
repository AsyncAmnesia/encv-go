package filename

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func defaultCompactCfg() FNConfig {
	return FNConfig{
		Password:   []byte("test-password"),
		Salt:       []byte("encfn-test-salt"),
		Charsets:   []FNCharset{},
		Deconfuse:  true,
		Rounds:     6,
		Structured: false,
	}
}

func defaultStructuredCfg() FNConfig {
	return FNConfig{
		Password:   []byte("test-password"),
		Salt:       []byte("encfn-test-salt"),
		Charsets:   []FNCharset{},
		Deconfuse:  true,
		Rounds:     6,
		Structured: true,
	}
}

func TestEncodeDecodeRoundtrip(t *testing.T) {
	tests := []struct {
		name  string
		input string
		cfg   FNConfig
	}{
		{"short alnum", "video.mp4", defaultCompactCfg()},
		{"long name", "2024年度财务报表_Q3_final_version.pdf", defaultStructuredCfg()},
		{"emoji", "照片🎉2024.jpg", defaultCompactCfg()},
		{"Chinese", "中文文件名测试.txt", defaultCompactCfg()},
		{"rare hanzi", "龘靁齉爨麤.doc", defaultCompactCfg()},
		{"special chars", "file-with_spaces.and-dots.tar.gz", defaultCompactCfg()},
		{"dots only", "...", defaultCompactCfg()},
		{"unicode mixed", "Hello世界🌍龘.txt", defaultCompactCfg()},
		{"single char", "a", defaultCompactCfg()},
		{"numbers", "12345", defaultCompactCfg()},
		{"structured roundtrip", "important_file.docx", defaultStructuredCfg()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := tt.cfg.Encode([]byte(tt.input))
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
			if len(encoded) == 0 {
				t.Fatal("Encode returned empty string")
			}

			decoded, err := tt.cfg.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}
			if string(decoded) != tt.input {
				t.Errorf("roundtrip mismatch:\n  got:  %q\n  want: %q", string(decoded), tt.input)
			}
		})
	}
}

func TestEmptyInput(t *testing.T) {
	cfg := defaultCompactCfg()

	_, err := cfg.Encode([]byte{})
	if err != ErrFNEmptyInput {
		t.Errorf("Encode empty input: got err=%v, want ErrFNEmptyInput", err)
	}

	_, err = cfg.Decode("")
	if err != ErrFNEmptyInput {
		t.Errorf("Decode empty input: got err=%v, want ErrFNEmptyInput", err)
	}
}

func TestWhitespaceInput(t *testing.T) {
	cfg := defaultCompactCfg()

	inputs := []string{" ", "  ", "\t", "\n", " \t "}
	for _, input := range inputs {
		name := strings.Map(func(r rune) rune {
			if r == ' ' { return 'S' }
			if r == '\t' { return 'T' }
			if r == '\n' { return 'N' }
			return r
		}, input)
		t.Run(name, func(t *testing.T) {
			encoded, err := cfg.Encode([]byte(input))
			if err != nil {
				t.Fatalf("Encode failed for %q: %v", input, err)
			}
			decoded, err := cfg.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode failed for %q: %v", input, err)
			}
			if string(decoded) != input {
				t.Errorf("roundtrip mismatch for whitespace %q: got %q", input, string(decoded))
			}
		})
	}
}

func TestLongFilename(t *testing.T) {
	cfg := defaultCompactCfg()

	longInput := strings.Repeat("AB", 250)
	encoded, err := cfg.Encode([]byte(longInput))
	if err != nil {
		t.Fatalf("Encode 500-byte input failed: %v", err)
	}
	if len(encoded) == 0 {
		t.Fatal("encoded output is empty")
	}

	table, _ := BuildCharsetTable(cfg.Charsets, cfg.Deconfuse)
	tableSize := uint64(len(table))

	expectedMinLen := (8*uint64(len(longInput)) + tableSize - 1) / tableSize
	if uint64(len(encoded)) < expectedMinLen {
		t.Errorf("encoded length %d < theoretical minimum %d (tableSize=%d)",
			len(encoded), expectedMinLen, tableSize)
	}

	decoded, err := cfg.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if string(decoded) != longInput {
		t.Errorf("roundtrip mismatch for long input: got len=%d, want len=%d",
			len(string(decoded)), len(longInput))
	}
}

func TestOddLengthInputRoundtrip(t *testing.T) {
	cfg := defaultCompactCfg()

	oddInputs := []string{
		"a",
		"abc",
		"video.mp4",
		"abcde",
		"\x00\x01\x02", // 3 binary bytes
		"g",             // single byte
	}

	for _, input := range oddInputs {
		t.Run(fmt.Sprintf("len_%d", len(input)), func(t *testing.T) {
			if len(input)%2 == 0 {
				t.Skip("not an odd-length input")
			}
			encoded, err := cfg.Encode([]byte(input))
			if err != nil {
				t.Fatalf("Encode failed for odd-length input %q: %v", input, err)
			}
			decoded, err := cfg.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode failed for odd-length input %q: %v", input, err)
			}
			if string(decoded) != input {
				t.Errorf("roundtrip mismatch: got %q want %q", string(decoded), input)
			}
		})
	}
}

func TestUnicodeFullCoverage(t *testing.T) {
	cfg := defaultCompactCfg()

	tests := []struct {
		name  string
		input string
	}{
		{"emoji basic", "🎉🔐📁💾🔒"},
		{"chinese filename", "中文文件名.txt"},
		{"rare hanzi", "龘靁齉爨麤"},
		{"arabic rtl", "مرحبا"},
		{"hebrew rtl", "שלום"},
		{"control chars", "a\x00b\nc\td"},
		{"mixed script", "Hello世界🌍龘.txt"},
		{"japanese hiragana", "ファイル名"},
		{"korean hangul", "파일이름"},
		{"cyrillic", "файл"},
		{"thai", "ชื่อไฟล์"},
		{"combining chars", "e\u0301"}, // é as combining sequence
		{"precomposed", "\u00E9"},        // é precomposed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := cfg.Encode([]byte(tt.input))
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
			decoded, err := cfg.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}
			if string(decoded) != tt.input {
				t.Errorf("roundtrip mismatch: got %q (len=%d) want %q (len=%d)",
					string(decoded), len(string(decoded)), tt.input, len(tt.input))
			}
		})
	}
}

func TestUTF8ByteSemantics(t *testing.T) {
	cfg := defaultCompactCfg()

	precomposed := "\u00E9" // é = U+00E9, 2 bytes in UTF-8: 0xC3 0xA9
	combining := "e\u0301"  // e + combining acute accent, 3 bytes: 0x65 0xCC 0x81

	encPre, _ := cfg.Encode([]byte(precomposed))
	encComb, _ := cfg.Encode([]byte(combining))

	if encPre == encComb {
		t.Error("precomposed and combining é should produce different outputs (different byte sequences)")
	}

	decPre, _ := cfg.Decode(encPre)
	decComb, _ := cfg.Decode(encComb)

	if string(decPre) != precomposed {
		t.Errorf("precomposed roundtrip failed: got %q want %q", string(decPre), precomposed)
	}
	if string(decComb) != combining {
		t.Errorf("combining roundtrip failed: got %q want %q", string(decComb), combining)
	}
}

func TestPasswordSensitivity(t *testing.T) {
	input := []byte("test.txt")

	cfg1 := defaultCompactCfg()
	cfg1.Password = []byte("password1")

	cfg2 := defaultCompactCfg()
	cfg2.Password = []byte("password2")

	enc1, _ := cfg1.Encode(input)
	enc2, _ := cfg2.Encode(input)

	if enc1 == enc2 {
		t.Error("different passwords should produce different encoded outputs")
	}
}

func TestSaltSensitivity(t *testing.T) {
	input := []byte("test.txt")

	cfg1 := defaultCompactCfg()
	cfg1.Salt = []byte("salt-A")

	cfg2 := defaultCompactCfg()
	cfg2.Salt = []byte("salt-B")

	enc1, _ := cfg1.Encode(input)
	enc2, _ := cfg2.Encode(input)

	if enc1 == enc2 {
		t.Error("different salts should produce different encoded outputs")
	}
}

func TestDeterminism(t *testing.T) {
	cfg := defaultCompactCfg()
	input := []byte("same_input.txt")

	results := make(map[string]int)
	for i := 0; i < 5; i++ {
		enc, err := cfg.Encode(input)
		if err != nil {
			t.Fatalf("Encode iteration %d failed: %v", i, err)
		}
		results[enc]++
	}

	if len(results) != 1 {
		t.Errorf("expected identical output across 5 runs, got %d distinct results", len(results))
	}
}

func TestBuildCharsetTable(t *testing.T) {
	t.Run("alnum deconfuse", func(t *testing.T) {
		table, err := BuildCharsetTable([]FNCharset{}, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedSize := 56
		if len(table) != expectedSize {
			t.Errorf("table size: got %d, want %d (62 alnum - 6 deconfused)", len(table), expectedSize)
		}
		for _, r := range "0Oo1lI" {
			for _, tr := range table {
				if tr == r {
					t.Errorf("deconfused table should not contain %q", r)
				}
			}
		}
	})

	t.Run("alnum no deconfuse", func(t *testing.T) {
		table, err := BuildCharsetTable([]FNCharset{}, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(table) != 62 {
			t.Errorf("full alnum table size: got %d, want 62", len(table))
		}
	})

	t.Run("alnum plus hanzi_rare", func(t *testing.T) {
		tableAlnum, _ := BuildCharsetTable([]FNCharset{}, true)
		tableMixed, err := BuildCharsetTable([]FNCharset{FNHanziRare}, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tableMixed) <= len(tableAlnum) {
			t.Errorf("hanzi_rare table (%d) must be larger than alnum-only (%d)",
				len(tableMixed), len(tableAlnum))
		}
	})

	t.Run("unknown charset", func(t *testing.T) {
		_, err := BuildCharsetTable([]FNCharset{"unknown_charset"}, true)
		if err == nil {
			t.Error("expected error for unknown charset")
		}
	})

	t.Run("empty after deconfuse edge case", func(t *testing.T) {
		_, err := BuildCharsetTable([]FNCharset{}, true)
		if err != nil {
			t.Fatalf("standard alnum deconfuse should not be empty: %v", err)
		}
	})
}

func TestDecodeErrors(t *testing.T) {
	t.Run("structured tamper detection", func(t *testing.T) {
		cfg := defaultStructuredCfg()
		valid, err := cfg.Encode([]byte("test.txt"))
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
		if len(valid) < 3 {
			t.Fatalf("encoded too short: %q", valid)
		}

		tampered := valid[:len(valid)-2] + "XX"
		_, err = cfg.Decode(tampered)
		if err == nil {
			t.Error("tampered structured data should return error")
		}
	})

	t.Run("compact invalid charset char", func(t *testing.T) {
		cfg := defaultCompactCfg()
		invalidEncoded := "!@#$%^&*()__INVALID_CHARS_NOT_IN_TABLE__"
		_, err := cfg.Decode(invalidEncoded)
		if err == nil {
			t.Error("decode with invalid charset characters should return error")
		}
		if err != ErrFNCharsetMismatch {
			t.Errorf("got err=%v, want ErrFNCharsetMismatch", err)
		}
	})

	t.Run("structured invalid format", func(t *testing.T) {
		cfg := defaultStructuredCfg()
		cases := []string{
			"S",
			"S1",
			"X1:",
			"S1:body",
			"S1:body:",
		}
		for _, c := range cases {
			_, err := cfg.Decode(c)
			if err == nil {
				t.Errorf("invalid format %q should return error", c)
			}
		}
	})
}

func TestStructuredVsCompact(t *testing.T) {
	input := []byte("same_data.txt")

	cfgCompact := defaultCompactCfg()
	cfgStructured := defaultStructuredCfg()

	encCompact, _ := cfgCompact.Encode(input)
	encStructured, _ := cfgStructured.Encode(input)

	if encCompact == encStructured {
		t.Error("compact and structured modes should produce different output formats")
	}

	if !strings.HasPrefix(encStructured, "S") {
		prefixLen := 3
		if len(encStructured) < prefixLen {
			prefixLen = len(encStructured)
		}
		t.Errorf("structured output should start with 'S', got prefix: %q", encStructured[:prefixLen])
	}

	decCompact, _ := cfgCompact.Decode(encCompact)
	decStructured, _ := cfgStructured.Decode(encStructured)

	if string(decCompact) != string(input) {
		t.Error("compact roundtrip failed")
	}
	if string(decStructured) != string(input) {
		t.Error("structured roundtrip failed")
	}
}

func TestRoundsVariation(t *testing.T) {
	input := []byte("rounds_test.dat")

	for rounds := 1; rounds <= 12; rounds++ {
		t.Run(fmt.Sprintf("rounds_%d", rounds), func(t *testing.T) {
			cfg := defaultCompactCfg()
			cfg.Rounds = rounds

			encoded, err := cfg.Encode(input)
			if err != nil {
				t.Fatalf("Encode failed at rounds=%d: %v", rounds, err)
			}

			decoded, err := cfg.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode failed at rounds=%d: %v", rounds, err)
			}
			if string(decoded) != string(input) {
				t.Errorf("roundtrip mismatch at rounds=%d", rounds)
			}
		})
	}
}

func TestMultiCharsetRoundtrip(t *testing.T) {
	charsetCombos := [][]FNCharset{
		{FNSymbolsBasic},
		{FNSymbolsExt},
		{FNHanziRare},
		{FNEmoji},
		{FNSymbolsBasic, FNSymbolsExt},
		{FNAlnum, FNHanziRare, FNEmoji},
	}

	input := []byte("multi_charset_test.file")

	for _, combo := range charsetCombos {
		name := ""
		for _, cs := range combo {
			name += string(cs) + "_"
		}
		t.Run(name, func(t *testing.T) {
			cfg := defaultCompactCfg()
			cfg.Charsets = combo

			encoded, err := cfg.Encode(input)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			decoded, err := cfg.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}
			if string(decoded) != string(input) {
				t.Errorf("roundtrip mismatch: got %q want %q", string(decoded), string(input))
			}
		})
	}
}

func TestBinaryData(t *testing.T) {
	cfg := defaultCompactCfg()

	binaryInputs := [][]byte{
		{0x00},
		{0xFF},
		{0x00, 0xFF},
		{0x01, 0x02, 0x03},
		{0x01, 0x02, 0x03, 0x04, 0x05},
		make([]byte, 100),
		{0xDE, 0xAD, 0xBE, 0xEF},
	}

	for i, input := range binaryInputs {
		t.Run(fmt.Sprintf("binary_%d_len_%d", i, len(input)), func(t *testing.T) {
			encoded, err := cfg.Encode(input)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
			decoded, err := cfg.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}
			if len(decoded) != len(input) {
				t.Errorf("length mismatch: got %d want %d", len(decoded), len(input))
			}
			for j := range input {
				if decoded[j] != input[j] {
					t.Errorf("byte[%d] mismatch: got 0x%02X want 0x%02X", j, decoded[j], input[j])
					break
				}
			}
		})
	}
}

func TestSingleByteValues(t *testing.T) {
	cfg := defaultCompactCfg()

	for b := 0; b < 256; b++ {
		input := []byte{byte(b)}
		encoded, err := cfg.Encode(input)
		if err != nil {
			t.Fatalf("Encode byte 0x%02X failed: %v", b, err)
		}
		decoded, err := cfg.Decode(encoded)
		if err != nil {
			t.Fatalf("Decode byte 0x%02X failed: %v", b, err)
		}
		if len(decoded) != 1 || decoded[0] != byte(b) {
			t.Errorf("roundtrip mismatch for byte 0x%02X: got %v", b, decoded)
		}
	}
}

func TestConfigValidationDefaults(t *testing.T) {
	cfg := FNConfig{
		Password:   []byte("pass"),
		Rounds:     0,
		Charsets:   nil,
		Structured: false,
	}
	cfg.Validate()

	if cfg.Rounds < 1 {
		t.Error("Validate should set default Rounds >= 1")
	}
	if len(cfg.Charsets) == 0 {
		t.Error("Validate should set default Charsets")
	}
}

func TestEvenLengthRoundtrip(t *testing.T) {
	cfg := defaultCompactCfg()
	evenInputs := []string{
		"test.txt",
		"ab",
		"abcd",
		"abcdef",
		"abcdefgh",
		"abcdefghij",
	}
	for _, input := range evenInputs {
		t.Run(fmt.Sprintf("len_%d", len(input)), func(t *testing.T) {
			if len(input)%2 != 0 {
				t.Skip("not even length")
			}
			enc, err := cfg.Encode([]byte(input))
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			dec, err := cfg.Decode(enc)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if string(dec) != input {
				t.Errorf("got %q want %q", string(dec), input)
			}
		})
	}
}

func TestFeistelDirectRoundtrip(t *testing.T) {
	password := []byte("test-password")
	salt := []byte("encfn-test-salt")
	rounds := 6

	keys := DeriveKeys(password, salt, rounds)
	sbox := GenerateSBox(keys.SboxSeed)

	tests := []string{"test.txt", "ab", "abcd", "abcdefgh", "12345678"}
	for _, input := range tests {
		t.Run(fmt.Sprintf("len_%d", len(input)), func(t *testing.T) {
			plaintext := []byte(input)

			encrypted := FeistelEncrypt(plaintext, sbox, keys.RoundKeys)
			decrypted := FeistelDecrypt(encrypted, sbox, keys.RoundKeys)

			if string(decrypted) != string(plaintext) {
				t.Errorf("Feistel roundtrip failed:\n  plaintext:  %x\n  encrypted:  %x\n  decrypted:  %x", plaintext, encrypted, decrypted)
			}
		})
	}
}

func TestFeistelDeterminism(t *testing.T) {
	password := []byte("test-password")
	salt := []byte("encfn-test-salt")
	rounds := 6
	input := []byte("same_input.txt")

	var results [][]byte
	for i := 0; i < 5; i++ {
		keys := DeriveKeys(password, salt, rounds)
		sbox := GenerateSBox(keys.SboxSeed)
		enc := FeistelEncrypt(input, sbox, keys.RoundKeys)
		results = append(results, enc)
	}

	for i := 1; i < len(results); i++ {
		if string(results[i]) != string(results[0]) {
			t.Errorf("non-deterministic: run 0=%x run %d=%x", results[0], i, results[i])
		}
	}
}

func TestSHA256Determinism(t *testing.T) {
	data := []byte("hello")
	var first [32]byte
	for i := 0; i < 5; i++ {
		h := sha256.Sum256(data)
		if i == 0 {
			first = h
		} else if h != first {
			t.Errorf("SHA256 non-deterministic at run %d: %x vs %x", i, first, h)
		}
	}
}

func TestDeriveKeysDeterminism(t *testing.T) {
	password := []byte("test-password")
	salt := []byte("encfn-test-salt")
	rounds := 6

	var firstSeed []byte
	for i := 0; i < 5; i++ {
		keys := DeriveKeys(password, salt, rounds)
		if i == 0 {
			firstSeed = keys.SboxSeed
		} else if string(keys.SboxSeed) != string(firstSeed) {
			t.Errorf("SboxSeed non-deterministic: run 0=%x run %d=%x", firstSeed, i, keys.SboxSeed)
		}
	}
}

func TestSBoxDeterminism(t *testing.T) {
	seed := []byte("fixed-seed-for-test")
	var firstForward [256]byte
	for i := 0; i < 5; i++ {
		sbox := GenerateSBox(seed)
		if i == 0 {
			firstForward = sbox.Forward
		} else if sbox.Forward != firstForward {
			t.Errorf("SBox non-deterministic at run %d", i)
		}
	}
}

func TestEncodeDecodeRoundtripBothModes(t *testing.T) {
	tests := []struct {
		name string
		input string
	}{
		{"short ascii", "video.mp4"},
		{"long ascii + underscores", "2024年度财务报表_Q3_final_version.pdf"},
		{"with emoji", "照片🎉2024.jpg"},
		{"chinese", "中文文件名测试.txt"},
		{"rare hanzi", "龘靁齉爨麤毊.docx"},
		{"empty string", ""},
		{"whitespace only", "   "},
		{"special chars", "file-with_spaces.and-dots.tar.gz"},
		{"single char", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("compact", func(t *testing.T) {
				cfg := defaultCompactCfg()
				if tt.input == "" {
					_, err := cfg.Encode([]byte(tt.input))
					if err != ErrFNEmptyInput {
						t.Errorf("empty input: got err=%v, want ErrFNEmptyInput", err)
					}
					return
				}
				encoded, err := cfg.Encode([]byte(tt.input))
				if err != nil {
					t.Fatalf("Encode failed: %v", err)
				}
				decoded, err := cfg.Decode(encoded)
				if err != nil {
					t.Fatalf("Decode failed: %v", err)
				}
				if string(decoded) != tt.input {
					t.Errorf("roundtrip mismatch: got %q want %q", string(decoded), tt.input)
				}
			})

			t.Run("structured", func(t *testing.T) {
				cfg := defaultStructuredCfg()
				if tt.input == "" {
					_, err := cfg.Encode([]byte(tt.input))
					if err != ErrFNEmptyInput {
						t.Errorf("empty input: got err=%v, want ErrFNEmptyInput", err)
					}
					return
				}
				encoded, err := cfg.Encode([]byte(tt.input))
				if err != nil {
					t.Fatalf("Encode failed: %v", err)
				}
				decoded, err := cfg.Decode(encoded)
				if err != nil {
					t.Fatalf("Decode failed: %v", err)
				}
				if string(decoded) != tt.input {
					t.Errorf("roundtrip mismatch: got %q want %q", string(decoded), tt.input)
				}
			})
		})
	}
}

func TestLongRandomInputRoundtrip(t *testing.T) {
	cfgCompact := defaultCompactCfg()
	cfgStructured := defaultStructuredCfg()

	longInput := make([]byte, 500)
	for i := range longInput {
		longInput[i] = byte(i*7 + 13*i*i%251)
	}

	for name, cfg := range map[string]FNConfig{"compact": cfgCompact, "structured": cfgStructured} {
		t.Run(name, func(t *testing.T) {
			encoded, err := cfg.Encode(longInput)
			if err != nil {
				t.Fatalf("Encode 500-byte input failed: %v", err)
			}
			if len(encoded) == 0 {
				t.Fatal("encoded output is empty")
			}
			decoded, err := cfg.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}
			if len(decoded) != len(longInput) {
				t.Errorf("length mismatch: got %d want %d", len(decoded), len(longInput))
			}
			for i := range longInput {
				if decoded[i] != longInput[i] {
					t.Errorf("byte[%d] mismatch: got 0x%02X want 0x%02X", i, decoded[i], longInput[i])
					break
				}
			}
		})
	}
}

func TestBuildCharsetTableExtended(t *testing.T) {
	t.Run("symbols_extended+emoji with deconfuse", func(t *testing.T) {
		table, err := BuildCharsetTable([]FNCharset{FNSymbolsExt, FNEmoji}, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(table) <= 62 {
			t.Errorf("table with symbols_extended+emoji should be > 62, got %d", len(table))
		}
		hasSymbol := false
		hasEmoji := false
		for _, r := range table {
			if strings.ContainsRune(string(SymbolsExtChars), r) {
				hasSymbol = true
			}
			if strings.ContainsRune(string(EmojiChars), r) {
				hasEmoji = true
			}
		}
		if !hasSymbol {
			t.Error("table should contain at least one symbol_extended character")
		}
		if !hasEmoji {
			t.Error("table should contain at least one emoji character")
		}
		for _, confusable := range "0Oo1lI" {
			for _, tr := range table {
				if tr == confusable {
					t.Errorf("deconfused table should not contain %q", confusable)
				}
			}
		}
	})

	t.Run("hanzi_rare deconfuse size > 1000", func(t *testing.T) {
		table, err := BuildCharsetTable([]FNCharset{FNHanziRare}, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(table) <= 1000 {
			t.Errorf("hanzi_rare table should have > 1000 chars, got %d", len(table))
		}
	})
}

func TestBoundaryCases(t *testing.T) {
	cfgCompact := defaultCompactCfg()
	cfgStructured := defaultStructuredCfg()

	boundaryTests := []struct {
		name  string
		input []byte
	}{
		{"single null byte", []byte{0x00}},
		{"64 zero bytes", bytes.Repeat([]byte{0x00}, 64)},
		{"64 0xFF bytes", bytes.Repeat([]byte{0xFF}, 64)},
		{"4-byte emoji UTF8", []byte{0xF0, 0x9F, 0x8E, 0x99}},
		{"mixed UTF8 sequences", []byte{
			0x41,                                           // 'A' (1 byte)
			0xC3, 0xA9,                                     // é (2 bytes)
			0xE4, 0xB8, 0xAD,                               // 文 (3 bytes)
			0xF0, 0x9F, 0x98, 0x81,                         // 😁 (4 bytes)
			0x00,                                           // null
			0xFF,                                           // max byte
		}},
	}

	for _, bt := range boundaryTests {
		for modeName, cfg := range map[string]FNConfig{"compact": cfgCompact, "structured": cfgStructured} {
			t.Run(bt.name+"/"+modeName, func(t *testing.T) {
				encoded, err := cfg.Encode(bt.input)
				if err != nil {
					t.Fatalf("Encode failed: %v", err)
				}
				decoded, err := cfg.Decode(encoded)
				if err != nil {
					t.Fatalf("Decode failed: %v", err)
				}
				if len(decoded) != len(bt.input) {
					t.Errorf("length mismatch: got %d want %d", len(decoded), len(bt.input))
					return
				}
				for i := range bt.input {
					if decoded[i] != bt.input[i] {
						t.Errorf("byte[%d] mismatch: got 0x%02X want 0x%02X", i, decoded[i], bt.input[i])
						return
					}
				}
			})
		}
	}
}

func TestStructuredTamperChecksumMismatch(t *testing.T) {
	cfg := defaultStructuredCfg()
	input := []byte("checksum_test.txt")

	valid, err := cfg.Encode(input)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Run("tamper last char body", func(t *testing.T) {
		if len(valid) < 5 {
			t.Skip("encoded too short")
		}
		runes := []rune(valid)
		lastBodyIdx := len(runes) - 3
		if lastBodyIdx < 2 {
			t.Skip("too short to tamper body")
		}
		runes[lastBodyIdx] = 'Z'
		tampered := string(runes)
		_, err := cfg.Decode(tampered)
		if err == nil {
			t.Error("tampered body should produce checksum mismatch error")
		}
		if err != ErrFNChecksumMismatch && err != ErrFNInvalidFormat && err != ErrFNCharsetMismatch {
			t.Logf("got error type: %T: %v", err, err)
		}
	})

	t.Run("tamper crc portion", func(t *testing.T) {
		tampered := valid[:len(valid)-1] + "X"
		_, err := cfg.Decode(tampered)
		if err == nil {
			t.Error("tampered CRC should produce error")
		}
	})

	t.Run("wrong prefix", func(t *testing.T) {
		_, err := cfg.Decode("X" + valid[1:])
		if err == nil {
			t.Error("wrong prefix S->X should produce ErrFNInvalidFormat")
		}
		if err != ErrFNInvalidFormat {
			t.Logf("got err=%v (want ErrFNInvalidFormat)", err)
		}
	})
}

func TestDecodeEmptyStringBehavior(t *testing.T) {
	cfg := defaultCompactCfg()
	_, err := cfg.Decode("")
	if err != ErrFNEmptyInput {
		t.Errorf("Decode empty string: got err=%v (%T), want ErrFNEmptyInput", err, err)
	}

	cfgStruct := defaultStructuredCfg()
	_, err = cfgStruct.Decode("")
	if err != ErrFNEmptyInput {
		t.Errorf("Decode empty string structured: got err=%v (%T), want ErrFNEmptyInput", err, err)
	}
}