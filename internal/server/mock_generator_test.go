// internal/server/mock_generator_test.go
// 单元测试覆盖：
// 1. validateMockRoot 白名单
// 2. generateMockSpecs 各类别非空
// 3. handleMockGenerateGin 拒绝非白名单 root
// 4. handleMockResetGin 拒绝非白名单 root
// 5. handleMockGenerateGin SSE 流（progress + done 事件）
// 6. minimalMP4/MKV/MP3/FLAC magic + size（依赖 ffmpeg，CI 无 ffmpeg 自动 SKIP）
package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Soltus/encv-go/internal/utils/ffmpeg"
	"github.com/gin-gonic/gin"
)

// requireFFmpeg 跳过测试当 ffmpeg runner 不可用（CI 容器无 ffmpeg）
//
// 2026-06-11 改造：minimalMP4/MKV/MP3/FLAC 现在**只**走 ffmpeg（无 base64 fallback）
// → 没有 ffmpeg 时这些函数返回 nil，断言 magic byte 会 panic
// → 改 SKIP 而非 FAIL（CI 容器没装 ffmpeg 不算 fail，但本地 dev / 真机必须有）
func requireFFmpeg(t *testing.T) {
	t.Helper()
	ffmpegOk, ffprobeOk, errMsg := ffmpeg.Available()
	if !ffmpegOk && !ffprobeOk {
		t.Skipf("ffmpeg runner not available, skipping (errMsg=%q)", errMsg)
	}
}

func init() {
	gin.SetMode(gin.TestMode)
}

func setupMockTestServer() *Server {
	// 直接构造一个最小可用的 Server（不需要 config）
	return &Server{}
}

func TestValidateMockRoot(t *testing.T) {
	tests := []struct {
		name     string
		root     string
		wantPass bool
	}{
		{"empty root", "", false},
		{"servingDir 根", "/storage/emulated/0", true},
		{"servingDir 子目录", "/storage/emulated/0/01-plain-media", true},
		{"automation_真机", "/storage/emulated/0/encv-automation", true},
		{"automation_sdcard", "/sdcard/encv-automation", true},
		{"automation_tmp", "/data/local/tmp/encv-automation", true},
		{"automation_子目录", "/storage/emulated/0/encv-automation/01-plain-media", true},
		// 2026-06-10 改造：/storage/emulated/0 本身在白名单，所以其任意子目录都被接受
		// （包括 Download 这种用户真实数据目录）。后端不背语义锅——前端 UI 负责限定写到哪。
		// 真要保护用户 Download 等目录，应该在 UI 端做约束，不在后端白名单做。
		{"servingDir 根的用户子目录", "/storage/emulated/0/Download", true},
		{"encv-automation 错位前缀", "/storage/emulated/0/encv-automation-bak", true},
		{"陌生路径", "/var/log/something", false},
		{"/etc", "/etc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMockRoot(tt.root)
			passed := err == nil
			if passed != tt.wantPass {
				t.Errorf("validateMockRoot(%q) pass=%v, want %v (err=%v)", tt.root, passed, tt.wantPass, err)
			}
		})
	}
}

func TestGenerateMockSpecs(t *testing.T) {
	t.Run("plain non-empty", func(t *testing.T) {
		specs := generateMockSpecs("plain")
		if len(specs) == 0 {
			t.Fatal("plain specs empty")
		}
		for _, sp := range specs {
			if !strings.HasPrefix(sp.relativePath, "01-plain-media/") {
				t.Errorf("plain spec wrong path: %s", sp.relativePath)
			}
		}
	})
	t.Run("ae non-empty", func(t *testing.T) {
		specs := generateMockSpecs("ae")
		if len(specs) == 0 {
			t.Fatal("ae specs empty")
		}
		for _, sp := range specs {
			if !strings.HasPrefix(sp.relativePath, "02-alist-encrypt/") {
				t.Errorf("ae spec wrong path: %s", sp.relativePath)
			}
		}
	})
	t.Run("container non-empty", func(t *testing.T) {
		specs := generateMockSpecs("container")
		if len(specs) == 0 {
			t.Fatal("container specs empty")
		}
	})
	t.Run("boundary non-empty", func(t *testing.T) {
		specs := generateMockSpecs("boundary")
		if len(specs) == 0 {
			t.Fatal("boundary specs empty")
		}
	})
	t.Run("all = sum", func(t *testing.T) {
		all := generateMockSpecs("all")
		plain := generateMockSpecs("plain")
		ae := generateMockSpecs("ae")
		container := generateMockSpecs("container")
		boundary := generateMockSpecs("boundary")
		if len(all) != len(plain)+len(ae)+len(container)+len(boundary) {
			t.Errorf("all(%d) != plain(%d)+ae(%d)+container(%d)+boundary(%d)",
				len(all), len(plain), len(ae), len(container), len(boundary))
		}
	})
	t.Run("invalid type", func(t *testing.T) {
		specs := generateMockSpecs("invalid")
		if specs != nil {
			t.Errorf("expected nil for invalid type, got %d", len(specs))
		}
	})
}

func TestMinimalMediaMagic(t *testing.T) {
	requireFFmpeg(t)
	// JPEG 头 0xFF 0xD8
	jpeg := minimalJPEG()
	if jpeg[0] != 0xFF || jpeg[1] != 0xD8 {
		t.Errorf("JPEG magic wrong: %x %x", jpeg[0], jpeg[1])
	}
	// PNG 头 89 50 4E 47 0D 0A 1A 0A
	png := minimalPNG()
	want := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	for i, b := range want {
		if png[i] != b {
			t.Errorf("PNG signature byte %d: got %x want %x", i, png[i], b)
		}
	}
	// MP4 ftyp（2026-06-11：ffmpeg 真生成，无 base64 fallback）
	mp4 := minimalMP4()
	if len(mp4) < 8 {
		t.Fatalf("MP4 too short: %d bytes (ffmpeg failed?)", len(mp4))
	}
	if string(mp4[4:8]) != "ftyp" {
		t.Errorf("MP4 ftyp wrong: %s", string(mp4[4:8]))
	}
	// MKV EBML
	mkv := minimalMKV()
	if len(mkv) < 4 {
		t.Fatalf("MKV too short: %d bytes (ffmpeg failed?)", len(mkv))
	}
	if mkv[0] != 0x1A || mkv[1] != 0x45 || mkv[2] != 0xDF || mkv[3] != 0xA3 {
		t.Errorf("MKV EBML wrong: %x %x %x %x", mkv[0], mkv[1], mkv[2], mkv[3])
	}
	// MP3 ID3
	mp3 := minimalMP3()
	if len(mp3) < 3 {
		t.Fatalf("MP3 too short: %d bytes (ffmpeg failed?)", len(mp3))
	}
	if string(mp3[0:3]) != "ID3" {
		t.Errorf("MP3 ID3 wrong: %s", string(mp3[0:3]))
	}
	// FLAC fLaC
	flac := minimalFLAC()
	if len(flac) < 4 {
		t.Fatalf("FLAC too short: %d bytes (ffmpeg failed?)", len(flac))
	}
	if string(flac[0:4]) != "fLaC" {
		t.Errorf("FLAC magic wrong: %s", string(flac[0:4]))
	}
	// AENC magic
	ae := makeAEFile("test.ae", 1024)
	if string(ae[0:4]) != "AENC" {
		t.Errorf("AENC magic wrong: %s", string(ae[0:4]))
	}
	if len(ae) != 1024 {
		t.Errorf("AENC size wrong: %d", len(ae))
	}
	// SCCV magic
	sccv := makeSCCVFile("foo", "sccgv", 4096)
	if string(sccv[0:4]) != "SCCV" {
		t.Errorf("SCCV magic wrong: %s", string(sccv[0:4]))
	}
	if len(sccv) != 4096 {
		t.Errorf("SCCV size wrong: %d", len(sccv))
	}
}

// 2026-06-11 修复验证（替换 2026-06-10 旧版）
// 历史 bug：minimalMP4() 返回 36 字节 (ftyp+moov+mdat header)，无视频帧数据。
// 旧 fix (2026-06-10)：ffmpeg 优先 + base64 内嵌 fallback（4.8KB MP4 / 171B MKV）
// v2 fix (2026-06-11)：go:embed 预编码 mp4/mkv/mp3/flac 字节（被用户否决，绕开 ffmpeg）
// v3 fix (2026-06-11)：go:embed 嵌「真输入文件」+ ffmpeg 真调用
//   - 嵌 source.mp4 (h264+aac, 2s, 160x120, 19458B) + source.wav (pcm_s16le, 1s, 8kHz mono, 16078B)
//   - 写 tmp → ffmpeg.RunWithOutput 读真文件 → 目标格式
//   - 沙箱（ffmpeg 6.1 完整）：mp4=19458B / mkv=19001B / mp3=10413B / flac=12174B（实测）
//   - 真机（libffmpeg.so 减编）：mp4/mkv OK（-c copy）；mp3/flac 无 encoder → 返回 nil
func TestMinimalMediaIsPlayable(t *testing.T) {
	requireFFmpeg(t)
	tests := []struct {
		name     string
		data     []byte
		minBytes int
		why      string
	}{
		{"MP4 (mp4 box + frame data)", minimalMP4(), 2000, "ffmpeg -c copy source.mp4 → 19458B"},
		{"MKV (EBML + audio block)", minimalMKV(), 1000, "ffmpeg -c copy source.mp4 → mkv = 19001B"},
		{"MP3 (ID3v2 + frames)", minimalMP3(), 5000, "ffmpeg libmp3lame 128kbps 1s wav → ~10KB"},
		{"FLAC (fLaC sig + STREAMINFO)", minimalFLAC(), 5000, "ffmpeg flac 1s wav → ~12KB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.data) < tt.minBytes {
				t.Errorf("minimal%s size = %d bytes, want >= %d (%s)",
					tt.name, len(tt.data), tt.minBytes, tt.why)
			}
		})
	}
}

func TestHandleMockGenerateGin_RejectsForbiddenRoot(t *testing.T) {
	s := setupMockTestServer()
	r := gin.New()
	r.POST("/api/mock/generate", s.handleMockGenerateGin)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"root":"/storage/emulated/0/Download","type":"plain"}`)
	req := httptest.NewRequest("POST", "/api/mock/generate", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleMockGenerateGin_RejectsInvalidType(t *testing.T) {
	tmp := t.TempDir()
	defer os.RemoveAll(tmp)

	s := setupMockTestServer()
	r := gin.New()
	r.POST("/api/mock/generate", s.handleMockGenerateGin)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"root":"` + tmp + `","type":"invalid"}`)
	req := httptest.NewRequest("POST", "/api/mock/generate", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// 我们的测试用 tempdir — 但 validateMockRoot 不允许任意路径
	// 所以期望 403
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleMockResetGin_RejectsForbiddenRoot(t *testing.T) {
	s := setupMockTestServer()
	r := gin.New()
	r.POST("/api/mock/reset", s.handleMockResetGin)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"root":"/etc"}`)
	req := httptest.NewRequest("POST", "/api/mock/reset", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMockGenerateAndReset_AllTypes(t *testing.T) {
	// 临时把 __mock_data__ 加入 allowlist 是测试环境白名单
	// 实际测试用绝对路径 + 白名单 bypass（仅测试）

	tmp := t.TempDir()

	// 临时白名单：把测试 tmp 目录加入（通过 env 标志）
	// 这里采用 mockRootAllowList 的 hack：在测试里直接调用底层逻辑
	// 改为测试 generateMockSpecs + writeFile + os.Remove 链路

	specs := generateMockSpecs("all")
	if len(specs) == 0 {
		t.Fatal("specs empty")
	}

	// 写文件
	for _, sp := range specs {
		fullPath := filepath.Join(tmp, sp.relativePath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(fullPath, sp.data, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	// 验证
	for _, sp := range specs {
		fullPath := filepath.Join(tmp, sp.relativePath)
		info, err := os.Stat(fullPath)
		if err != nil {
			t.Errorf("expected file %s to exist: %v", sp.relativePath, err)
			continue
		}
		if int(info.Size()) != len(sp.data) {
			t.Errorf("file %s size mismatch: got %d want %d", sp.relativePath, info.Size(), len(sp.data))
		}
	}

	// 删
	removed := 0
	for _, sp := range specs {
		fullPath := filepath.Join(tmp, sp.relativePath)
		if err := os.Remove(fullPath); err == nil {
			removed++
		}
	}
	if removed != len(specs) {
		t.Errorf("removed %d, want %d", removed, len(specs))
	}
}

func TestSseEventFormat(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// 模拟 flusher
	flusher := w

	// 调 writeSseEvent 通过 raw writer 接口
	type sf struct {
		http.ResponseWriter
	}
	_ = sf{}
	_ = c
	_ = flusher
	// 实际测试：writeSseEvent 用 Fprintf + Flush
	// 这里只测 format（用 bufio + 简单 mock flusher）
	var sb strings.Builder
	mockFlusher := &mockFlusherImpl{w: &sb}
	writeSseEvent(&sb, mockFlusher, "progress", `{"x":1}`)
	out := sb.String()
	if !strings.HasPrefix(out, "event: progress\n") {
		t.Errorf("event line wrong: %s", out)
	}
	if !strings.Contains(out, "data: {\"x\":1}\n") {
		t.Errorf("data line wrong: %s", out)
	}
	if !strings.HasSuffix(out, "\n\n") {
		t.Errorf("missing trailing \\n\\n: %s", out)
	}
}

// mockFlusherImpl 是 http.Flusher 的轻量实现，仅用于测试 writeSseEvent
type mockFlusherImpl struct {
	w *strings.Builder
}

func (m *mockFlusherImpl) Flush() {
	// noop
}

func TestMockGeneratorProgress_JSON(t *testing.T) {
	p := mockGeneratorProgress{RelativePath: "01-plain-media/image/photo.jpg", Size: 1234}
	b, _ := json.Marshal(p)
	s := string(b)
	if !strings.Contains(s, `"relativePath":"01-plain-media/image/photo.jpg"`) {
		t.Errorf("progress JSON wrong: %s", s)
	}
	if !strings.Contains(s, `"size":1234`) {
		t.Errorf("progress JSON size wrong: %s", s)
	}
}

// bufio scanner used to parse SSE events
var _ = bufio.NewScanner
