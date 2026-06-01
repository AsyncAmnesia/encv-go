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

func TestDecodeGinQueryParam_PreciseSingleDecode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Chinese path (Gin-decoded once from double-encoded)",
			input:    "%2F123%E4%BA%91%E7%9B%98%2Fbookmarks.html",
			expected: "/123云盘/bookmarks.html",
		},
		{
			name:     "path with @ symbol",
			input:    "%2F%E8%A7%86%E9%A2%91%40%E5%90%88%E9%9B%86%2Ftest.mp4",
			expected: "/视频@合集/test.mp4",
		},
		{
			name:     "path with # and ?",
			input:    "%2Ffile%231%3Fv2",
			expected: "/file#1?v2",
		},
		{
			name:     "spaces in path (Gin decodes + to space)",
			input:    "%2FMy+Documents%2Ffile.txt",
			expected: "/My Documents/file.txt",
		},
		{
			name:     "emoji path",
			input:    "%2F%F0%9F%98%80%F0%9F%8E%89%2Ftest.mp4",
			expected: "/😀🎉/test.mp4",
		},
		{
			name:     "percent followed by non-hex char (literal % preserved)",
			input:    "%2Freport%25Q1.txt",
			expected: "/report%Q1.txt",
		},
		{
			name:     "percent followed by valid hex ba (literal %ba preserved - KEY FIX)",
			input:    "%2Ftest%25ba.txt",
			expected: "/test%ba.txt",
		},
		{
			name:     "percent 20 as literal filename (space encoding preserved)",
			input:    "%2Ffoo%2520bar.txt",
			expected: "/foo%20bar.txt",
		},
		{
			name:     "percent 41 as literal filename (A encoding preserved)",
			input:    "%2Ffoo%2541bar.txt",
			expected: "/foo%41bar.txt",
		},
		{
			name:     "root path",
			input:    "%2F",
			expected: "/",
		},
		{
			name:     "deep nested path",
			input:    "%2Fa%2Fb%2Fc%2Fd%2Fe%2Ff.txt",
			expected: "/a/b/c/d/e/f.txt",
		},
		{
			name:     "null byte encoded (preserved as literal)",
			input:    "%2F%2500%2Fnull.txt",
			expected: "/%00/null.txt",
		},
		{
			name:     "double percent sign",
			input:    "%2F%25%25double%25.txt",
			expected: "/%%double%.txt",
		},
		{
			name:     "literal %2F in filename (slash encoding preserved)",
			input:    "%2F%252F%2Fslash-encoded.txt",
			expected: "/%2F/slash-encoded.txt",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "already decoded plain text (idempotent)",
			input:    "/DCIM/video.mp4",
			expected: "/DCIM/video.mp4",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DecodeGinQueryParam(tc.input)
			if got != tc.expected {
				t.Errorf("DecodeGinQueryParam(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestDecodeGinQueryParam_FullRoundtrip(t *testing.T) {
	testPaths := []string{
		"/123云盘/bookmarks.html",
		"/视频@合集/test.mp4",
		"/file#1?v2",
		"/My Documents/file.txt",
		"/😀🎉/test.mp4",
		"/report%Q1.txt",
		"/test%ba.txt",
		"/foo%20bar.txt",
		"/foo%41bar.txt",
		"/%00/null.txt",
		"/%%double%.txt",
		"/%2F/slash-encoded.txt",
		"/a/b/c/d/e/f.txt",
	}
	for _, originalPath := range testPaths {
		t.Run(originalPath, func(t *testing.T) {
			doubleEnc := url.QueryEscape(url.QueryEscape(originalPath))
			ginDecoded, _ := url.ParseQuery("p=" + doubleEnc)
			rawQuery := ginDecoded.Get("p")

			decoded := DecodeGinQueryParam(rawQuery)

			if decoded != originalPath {
				t.Errorf("roundtrip mismatch:\n  original:   %q\n  doubleEnc:  %s\n  ginDecoded: %q\n  decoded:    %q",
					originalPath, doubleEnc, rawQuery, decoded)
			}
		})
	}
}

func TestDecodeGinQueryParam_vs_DecodePathParam_DifferenceOnPercentHexHex(t *testing.T) {

	trickyPaths := []struct {
		name     string
		filename string
	}{
		{"%ba (valid hex)", "/test%ba.txt"},
		{"%20 (space)", "/foo%20bar.txt"},
		{"%41 (letter A)", "/foo%41bar.txt"},
		{"%00 (null)", "/%00/null.txt"},
		{"%2F (slash)", "/%2F/slash-encoded.txt"},
	}
	for _, tc := range trickyPaths {
		t.Run(tc.name, func(t *testing.T) {
			doubleEnc := url.QueryEscape(url.QueryEscape(tc.filename))
			ginDecoded, _ := url.ParseQuery("p=" + doubleEnc)
			rawQuery := ginDecoded.Get("p")

			singleDecode := DecodeGinQueryParam(rawQuery)
			doubleDecode := DecodePathParam(rawQuery)

			if singleDecode != tc.filename {
				t.Errorf("DecodeGinQueryParam failed: got %q, want %q", singleDecode, tc.filename)
			}

			if doubleDecode == tc.filename {
				t.Logf("DecodePathParam also matched (coincidence for this input)")
			} else {
				t.Logf("DecodeGinQueryParam=%q (correct) vs DecodePathParam=%q (over-decoded) — this is the bug we fixed",
					singleDecode, doubleDecode)
			}
		})
	}
}

func simulateProxySafeEncode(value string) string {
	return url.QueryEscape(url.QueryEscape(value))
}

func simulateGinQueryDecode(encoded string) string {
	decoded, _ := url.QueryUnescape(encoded)
	return decoded
}

func TestSafeURLToAbsPath_ProxySafeEncodeRoundTrip(t *testing.T) {
	baseDir := "/storage/emulated/0"

	tests := []struct {
		name      string
		original  string
		wantAbs   string
	}{
		{
			name:     "root path",
			original: "/",
			wantAbs:  "/storage/emulated/0",
		},
		{
			name:     "subdirectory",
			original: "/DCIM",
			wantAbs:  "/storage/emulated/0/DCIM",
		},
		{
			name:     "path with @ (WAF trigger char)",
			original: "/视频@合集",
			wantAbs:  "/storage/emulated/0/视频@合集",
		},
		{
			name:     "path with # and ?",
			original: "/file#1?v2",
			wantAbs:  "/storage/emulated/0/file#1?v2",
		},
		{
			name:     "Chinese path",
			original: "/中文目录/文件.txt",
			wantAbs:  "/storage/emulated/0/中文目录/文件.txt",
		},
		{
			name:     "emoji path",
			original: "/😀🎉/test.mp4",
			wantAbs:  "/storage/emulated/0/😀🎉/test.mp4",
		},
		{
			name:     "special chars !@#$%^&*()",
			original: "/04-boundary-test/special-chars-!@#$%^&*()_+.txt",
			wantAbs:  "/storage/emulated/0/04-boundary-test/special-chars-!@#$%^&*()_+.txt",
		},
		{
			name:     "percent literal in filename",
			original: "/reports/report%Q1.txt",
			wantAbs:  "/storage/emulated/0/reports/report%Q1.txt",
		},
		{
			name:     "space in path",
			original: "/My Documents/file.txt",
			wantAbs:  "/storage/emulated/0/My Documents/file.txt",
		},
		{
			name:     "deep nested path",
			original: "/a/b/c/d/e/f.txt",
			wantAbs:  "/storage/emulated/0/a/b/c/d/e/f.txt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doubleEncoded := simulateProxySafeEncode(tc.original)
			ginDecoded := simulateGinQueryDecode(doubleEncoded)

			got, err := SafeURLToAbsPath(baseDir, ginDecoded)
			if err != nil {
				t.Fatalf("SafeURLToAbsPath(%q) error: %v (original=%q, doubleEncoded=%q)", ginDecoded, err, tc.original, doubleEncoded)
			}
			if got != tc.wantAbs {
				t.Errorf("roundtrip mismatch:\n  original:      %q\n  doubleEncoded: %q\n  ginDecoded:    %q\n  got:           %q\n  want:          %q",
					tc.original, doubleEncoded, ginDecoded, got, tc.wantAbs)
			}
		})
	}
}

func TestDecodePathParam_Idempotent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain path unchanged",
			input: "/DCIM/video.mp4",
			want:  "/DCIM/video.mp4",
		},
		{
			name:  "already decoded Chinese",
			input: "/中文目录/文件.txt",
			want:  "/中文目录/文件.txt",
		},
		{
			name:  "single encoded value decoded once",
			input: "%2FDCIM%2Fvideo.mp4",
			want:  "/DCIM/video.mp4",
		},
		{
			name:  "double encoded value decoded fully",
			input: "%252FDCIM%252Fvideo.mp4",
			want:  "/DCIM/video.mp4",
		},
		{
			name:  "percent literal in filename (encoded as %25)",
			input: "/reports/report%25Q1.txt",
			want:  "/reports/report%Q1.txt",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DecodePathParam(tc.input)
			if got != tc.want {
				t.Errorf("DecodePathParam(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNoTripleDecode_AfterRemovingManualCalls(t *testing.T) {
	baseDir := "/storage/emulated/0"

	original := "/reports/report%Q1.txt"
	doubleEncoded := simulateProxySafeEncode(original)
	ginDecoded := simulateGinQueryDecode(doubleEncoded)

	got, err := SafeURLToAbsPath(baseDir, ginDecoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "/storage/emulated/0/reports/report%Q1.txt"
	if got != want {
		t.Errorf("triple decode detected!\n  original:      %q\n  doubleEncoded: %q\n  ginDecoded:    %q\n  got:           %q\n  want:          %q",
			original, doubleEncoded, ginDecoded, got, want)
	}
}

func TestSafeURLPathToRelative_DoubleEncodedFromGinQuery(t *testing.T) {
	tests := []struct {
		name          string
		ginQueryParam string
		expectedRel   string
	}{
		{
			name:          "root path double-encoded - the actual mobile bug",
			ginQueryParam: "%2F",
			expectedRel:   "",
		},
		{
			name:          "subdir double-encoded",
			ginQueryParam: "%2FDCIM%2F",
			expectedRel:   "DCIM",
		},
		{
			name:          "deep path with @ double-encoded (Gin decoded once)",
			ginQueryParam: "%2F04-boundary-test%2Fspecial-chars-!%40%23%24%25%5E%26*()_%2B.txt",
			expectedRel:   "04-boundary-test/special-chars-!@#$%^&*()_+.txt",
		},
		{
			name:          "Chinese path double-encoded via Gin",
			ginQueryParam: "%2F%E4%B8%AD%E6%96%87%E7%9B%AE%E5%BD%95%2F",
			expectedRel:   "中文目录",
		},
		{
			name:          "path with spaces single-encoded by Gin",
			ginQueryParam: "/dir/my%20file.txt",
			expectedRel:   "dir/my file.txt",
		},
		{
			name:          "path with percent literal in filename (triple encoded original)",
			ginQueryParam: "/dir/file%25name.txt",
			expectedRel:   "dir/file%name.txt",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SafeURLPathToRelative(tc.ginQueryParam)
			if err != nil {
				t.Fatalf("unexpected error for Gin query param %q: %v", tc.ginQueryParam, err)
			}
			if got != tc.expectedRel {
				t.Errorf("SafeURLPathToRelative(%q) = %q, want %q", tc.ginQueryParam, got, tc.expectedRel)
			}
		})
	}
}

func TestSafeURLToAbsPath_DoubleEncodedRootBug(t *testing.T) {
	baseDir := "/storage/emulated/0"

	absPath, err := SafeURLToAbsPath(baseDir, "%2F")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if absPath != baseDir {
		t.Errorf("double-encoded root resolved to %q, want %q (baseDir)", absPath, baseDir)
	}

	absPath, err = SafeURLToAbsPath(baseDir, "%2FDCIM%2F")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := baseDir + "/DCIM"
	if absPath != want {
		t.Errorf("double-encoded DCIM resolved to %q, want %q", absPath, want)
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
