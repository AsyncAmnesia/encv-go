package utils

import (
	"net/url"
	"testing"
)

func TestSafeURLPathToRelative_NormalPaths(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/", ""},
		{"/file.txt", "file.txt"},
		{"/dir/file.txt", "dir/file.txt"},
		{"/a/b/c", "a/b/c"},
		{"//double//slash//path", "double/slash/path"},
		{"/./dir/./file.txt", "dir/file.txt"},
		{"/dir/../root.txt", "root.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := SafeURLPathToRelative(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestDecodePathParam_DoubleEncoding(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "double-encoded @ symbol (root cause of truncation bug)",
			input:    "%2540",
			expected: "@",
		},
		{
			name:     "double-encoded full path with @",
			input:    "%2F04-boundary-test%2Fspecial-chars-!%2540%2523%2524%2525%255E%2526*()_%252B.txt",
			expected: "/04-boundary-test/special-chars-!@#$%^&*()_+.txt",
		},
		{
			name:     "single-encoded @ (no double encoding)",
			input:    "%40",
			expected: "@",
		},
		{
			name:     "plain text passthrough",
			input:    "/01-plain-media/document/notes.txt",
			expected: "/01-plain-media/document/notes.txt",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "double-encoded unicode filename",
			input:    "%25E4%25B8%25AD%25E6%2596%2587",
			expected: "\u4e2d\u6587",
		},
		{
			name:     "emoji in double encoding",
			input:    "%25F0%259F%2598%2580",
			expected: "\U0001f600",
		},
		{
			name:     "malformed percent sequence returns raw",
			input:    "%ZZ",
			expected: "%ZZ",
		},
		{
			name:     "incomplete percent sequence returns raw",
			input:    "%2",
			expected: "%2",
		},
		{
			name:     "path with spaces (single encoded)",
			input:    "%20dir%20%2Ffile.txt",
			expected: " dir /file.txt",
		},
		{
			name:     "path with spaces (double encoded)",
			input:    "%2520dir%2520%252Ffile.txt",
			expected: " dir /file.txt",
		},
		{
			name:     "triple encoded should still work (decode twice only)",
			input:    "%252540",
			expected: "%40",
		},
		{
			name:     "mixed special chars - hash, caret, ampersand, asterisk, plus",
			input:    "%2523%2524%2525%255E%2526*%252B",
			expected: "#$%^&*+",
		},
		{
			name:     "parentheses are safe (not reserved by WAF)",
			input:    "()",
			expected: "()",
		},
		{
			name:     "exclamation mark is safe",
			input:    "!",
			expected: "!",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DecodePathParam(tc.input)
			if got != tc.expected {
				t.Errorf("DecodePathParam(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestDecodePathParam_RoundTripWithProxySafeEncode(t *testing.T) {
	testPaths := []string{
		"/04-boundary-test/special-chars-!@#$%^&*()_+.txt",
		"/01-plain-media/document/notes.txt",
		"/03-encv-containers/container.sccgv",
		"/02-alist-encrypt/secret.ae",
		"中文文件名.txt",
		"emoji-test-😀🎉.txt",
		"spaces   in   name.txt",
		".hidden-file",
	}
	for _, originalPath := range testPaths {
		t.Run(originalPath, func(t *testing.T) {
			singleEnc := url.QueryEscape(originalPath)
			doubleEnc := url.QueryEscape(singleEnc)
			decoded := DecodePathParam(doubleEnc)
			if decoded != originalPath {
				t.Errorf("round-trip failed:\n  original: %q\n  single:  %q\n  double:  %q\n  decoded: %q",
					originalPath, singleEnc, doubleEnc, decoded)
			}
		})
	}
}

func TestSafeURLPathToRelative_SpecialCharsInFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "percent sign in filename",
			input:    "/dir/special-chars-!@#$%^&*()_+.txt",
			expected: "dir/special-chars-!@#$%^&*()_+.txt",
		},
		{
			name:     "spaces in filename",
			input:    "/dir/spaces   in   name.txt",
			expected: "dir/spaces   in   name.txt",
		},
		{
			name:     "trailing space in filename",
			input:    "/dir/trailing-space.txt ",
			expected: "dir/trailing-space.txt ",
		},
		{
			name:     "mixed case extension",
			input:    "/dir/MiXeD-CaSe-FiLe.TxT",
			expected: "dir/MiXeD-CaSe-FiLe.TxT",
		},
		{
			name:     "brackets and parens",
			input:    "/dir/file [v2] (copy).txt",
			expected: "dir/file [v2] (copy).txt",
		},
		{
			name:     "hash and ampersand",
			input:    "/dir/foo#bar&baz.txt",
			expected: "dir/foo#bar&baz.txt",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SafeURLPathToRelative(tc.input)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestSafeURLPathToRelative_UnicodeFilenames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Chinese Japanese Korean Arabic Hebrew Thai Greek mixed",
			input:    "/dir/long-unicode-filename-中文-日本語-한국어-العربية-עברית-ไทย-ελληνικά.txt",
			expected: "dir/long-unicode-filename-中文-日本語-한국어-العربية-עברית-ไทย-ελληνικά.txt",
		},
		{
			name:     "emoji filename",
			input:    "/dir/emoji-test-😀🎉🚀🔥.txt",
			expected: "dir/emoji-test-😀🎉🚀🔥.txt",
		},
		{
			name:     "Hebrew with RTL mark",
			input:    "/dir/\u200f\u05d0\u05d1\u05d2-rtl-filename.txt",
			expected: "dir/\u200f\u05d0\u05d1\u05d2-rtl-filename.txt",
		},
		{
			name:     "simple Chinese",
			input:    "/文档/报告.pdf",
			expected: "文档/报告.pdf",
		},
		{
			name:     "Russian Cyrillic",
			input:    "/документы/файл.txt",
			expected: "документы/файл.txt",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SafeURLPathToRelative(tc.input)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestSafeURLPathToRelative_ControlCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "SOH STX ETX control chars (valid after query decode)",
			input:    "/dir/control-chars-\x01\x02\x03.txt",
			expected: "dir/control-chars-\x01\x02\x03.txt",
			wantErr:  false,
		},
		{
			name:    "null byte injection - rejected",
			input:   "/dir/evil\x00file.txt",
			wantErr: true,
		},
		{
			name:     "tab character in filename",
			input:    "/dir/tab\there.txt",
			expected: "dir/tab\there.txt",
			wantErr:  false,
		},
		{
			name:     "newline in filename",
			input:    "/dir/line\nbreak.txt",
			expected: "dir/line\nbreak.txt",
			wantErr:  false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SafeURLPathToRelative(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestSafeURLPathToRelative_EmptyAndEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{name: "empty string", input: "", expected: "", wantErr: false},
		{name: "just slash", input: "/", expected: "", wantErr: false},
		{name: "dot normalizes to root", input: "/.", expected: "", wantErr: false},
		{name: "dotdot normalizes to root", input: "/..", expected: "", wantErr: false},
		{name: "only dots", input: "/.../...", expected: ".../...", wantErr: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SafeURLPathToRelative(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestSafeURLToAbsPath_Integration(t *testing.T) {
	baseDir := "/storage/emulated/0"
	tests := []struct {
		name        string
		urlPath     string
		expectedAbs string
		wantErr     bool
	}{
		{
			name:        "normal file",
			urlPath:     "/01-plain-media/document/notes.txt",
			expectedAbs: "/storage/emulated/0/01-plain-media/document/notes.txt",
		},
		{
			name:        "special chars with percent",
			urlPath:     "/04-boundary-test/special-chars-!@#$%^&*()_+.txt",
			expectedAbs: "/storage/emulated/0/04-boundary-test/special-chars-!@#$%^&*()_+.txt",
		},
		{
			name:        "control chars",
			urlPath:     "/04-boundary-test/control-chars-\x01\x02\x03.txt",
			expectedAbs: "/storage/emulated/0/04-boundary-test/control-chars-\x01\x02\x03.txt",
		},
		{
			name:        "emoji file",
			urlPath:     "/04-boundary-test/emoji-test-😀🎉🚀🔥.txt",
			expectedAbs: "/storage/emulated/0/04-boundary-test/emoji-test-😀🎉🚀🔥.txt",
		},
		{
			name:        "multi-language long filename",
			urlPath:     "/04-boundary-test/long-unicode-filename-中文-日本語-한국어-العربية-עברית-ไทย-ελληνικά.txt",
			expectedAbs: "/storage/emulated/0/04-boundary-test/long-unicode-filename-中文-日本語-한국어-العربية-עברית-ไทย-ελληνικά.txt",
		},
		{
			name:    "null byte rejected",
			urlPath: "/evil\x00path/file.txt",
			wantErr: true,
		},
		{
			name:    "path traversal via .. normalized by path.Clean",
			urlPath: "/../../etc/passwd",
			expectedAbs: "/storage/emulated/0/etc/passwd",
		},
		{
			name:        "traversal after normalization stays inside baseDir",
			urlPath:     "/dir/../../../etc/passwd",
			expectedAbs: "/storage/emulated/0/etc/passwd",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SafeURLToAbsPath(baseDir, tc.urlPath)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got abs path %q", tc.urlPath, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.urlPath, err)
			}
			if got != tc.expectedAbs {
				t.Errorf("abs path mismatch:\n  got:  %q\n  want: %q", got, tc.expectedAbs)
			}
		})
	}
}
