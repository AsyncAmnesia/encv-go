// internal/server/mock_generator.go
// 自动化测试用 Mock 数据生成 / 重置（Go 后端）
//
// 提供两个端点：
//   POST /api/mock/generate { root, type }  → SSE 流式进度
//   POST /api/mock/reset    { root }         → JSON { removed }
//
// 用途：自动化测试入口在前端触发时，需要把 mock 文件写入到：
//   - 真机 / dev preview：<servingDir>/01-plain-media/ 等
//   - 自动化测试命名空间：<servingDir>/encv-automation/01-plain-media/ 等
// 前端在浏览器/真机 WebView 没有权限直写这些目录，必须走后端。
//
// 安全：root 必须在白名单前缀内（见 validateMockRoot），否则 403。
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/utils/ffmpeg"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════════
// 🆕 2026-06-11 修复：mp4/mkv 真机错（base64 fallback 是垃圾）
//
// 历史：
//   - 2026-06-10：后端 ffmpeg 优先 + base64 内嵌 fallback（MP4_B64 4.8KB / MKV_B64 171B）
//   - 2026-06-11：用户反馈「真机 APK 上 ffmpeg 不存在，永远走 base64 fallback —— 傻逼，
//     集成的 ffmpeg 是摆设吗？给我用，删掉 base64」
//
// 根因（旧 fallback 缺陷）：
//   - mock_generator.go 直接 exec.Command("ffmpeg", ...) → 真机没有 /usr/bin/ffmpeg → 必 fail
//   - fail → 静默 fallback 到 base64 → MKV_B64 仅 171 字节（只有 EBML header，无视频帧）
//   - 真机生成的 mp4/mkv 不能播放、不能 ffprobe → 自动化测试跑挂
//
// 修复方案（2026-06-11）：
//   1. ffmpeg 调用改走项目集成的 internal/utils/ffmpeg.Runner 抽象层
//      - 沙箱 (!android build tag)：ExecRunner 用 os/exec 调 /usr/bin/ffmpeg
//      - 真机 (android build tag)：NativeRunner 用 cgo dlopen 调 libffmpeg.so (ffmpeg_run)
//      - 真机必须先跑 app/encv-mobile/scripts/build-ffmpeg-android.sh 编 libffmpeg.so 打到 APK jniLibs
//   2. mp4/mkv ffmpeg 调用补 -map 0:a -map 1:v（之前 2 个 input 没指定 stream，sine+color
//      默认选 video from input 0 = sine 失败）
//   3. 删 base64 fallback（mock_media_bytes.go 整个文件 + decodeBase64Media 函数）
//      - 失败就返回 nil，让调用方报错（不静默给垃圾字节）
//   4. 测试在 ffmpeg 不可用时 SKIP（CI 容器可能没 ffmpeg，不算失败）
//
// 验证：
//   - 沙箱：/usr/bin/ffmpeg 6.1 在 → 实际跑 mp4=19801B / mkv=9453B / mp3=33062B / flac=32487B
//   - 真机：build-ffmpeg-android.sh 编 libffmpeg.so → APK jniLibs → dlopen 调 ffmpeg_run
// ════════════════════════════════════════════════════════════════════

// mockRootAllowList 是允许写入的根目录白名单（绝对路径前缀）。
// 2026-06-10 改造：删除 dev 模式相对路径（`__mock_data__`），全部走绝对路径。
//   1. /storage/emulated/0（servingDir 根，给 Files 浏览器用）
//   2. /storage/emulated/0/encv-automation（自动化测试命名空间，withSafetyBoundary 改写后的目标）
//   3. /sdcard/encv-automation（真机 symlink 兼容）
//   4. /data/local/tmp/encv-automation（调试用）
// 其他路径一律 403。
var mockRootAllowList = []string{
	"/storage/emulated/0",
	"/storage/emulated/0/encv-automation",
	"/sdcard/encv-automation",
	"/data/local/tmp/encv-automation",
}

// mockGeneratorRequest 是 POST /api/mock/generate 的请求体
type mockGeneratorRequest struct {
	Root string `json:"root"`
	Type string `json:"type"` // "all" | "plain" | "ae" | "container" | "boundary"
}

// mockGeneratorProgress 是 SSE progress 事件 payload
type mockGeneratorProgress struct {
	RelativePath string `json:"relativePath"`
	Size         int    `json:"size"`
}

// mockGeneratorDone 是 SSE done 事件 payload
type mockGeneratorDone struct {
	Count     int   `json:"count"`
	TotalSize int64 `json:"totalSize"`
}

// validateMockRoot 校验 root 是否在白名单前缀内
func validateMockRoot(root string) error {
	if root == "" {
		return fmt.Errorf("root is empty")
	}
	// 规范化
	clean := filepath.Clean(root)
	if !filepath.IsAbs(clean) {
		// dev 模式：相对路径转绝对
		abs, err := filepath.Abs(clean)
		if err != nil {
			return fmt.Errorf("invalid root path: %w", err)
		}
		clean = abs
	}
	for _, allow := range mockRootAllowList {
		allowClean := filepath.Clean(allow)
		if !filepath.IsAbs(allowClean) {
			if abs, err := filepath.Abs(allowClean); err == nil {
				allowClean = abs
			}
		}
		// 精确匹配或在 allow 前缀下
		if clean == allowClean || strings.HasPrefix(clean, allowClean+string(os.PathSeparator)) {
			return nil
		}
	}
	return fmt.Errorf("root %q is not in allowlist", root)
}

// mockFileSpec 描述一个待生成的文件
type mockFileSpec struct {
	relativePath string
	data         []byte
}

// generateMockSpecs 返回指定 type 的所有文件 specs
// 字节内容是硬编码的最小有效格式（与前端 lib/mockDataGenerator.ts 对齐）
func generateMockSpecs(typeName string) []mockFileSpec {
	plainSpecs := []mockFileSpec{
		{relativePath: "01-plain-media/image/photo.jpg", data: minimalJPEG()},
		{relativePath: "01-plain-media/image/screenshot.png", data: minimalPNG()},
		{relativePath: "01-plain-media/video/sample.mp4", data: minimalMP4()},
		{relativePath: "01-plain-media/video/comedy.mkv", data: minimalMKV()},
		{relativePath: "01-plain-media/audio/music.mp3", data: minimalMP3()},
		{relativePath: "01-plain-media/audio/podcast.flac", data: minimalFLAC()},
		{relativePath: "01-plain-media/document/report.pdf", data: minimalPDF()},
		{relativePath: "01-plain-media/document/notes.txt", data: []byte("ENCV Mock Notes\n中文测试\n日本語テスト\n한국어 테스트\n")},
		{relativePath: "01-plain-media/document/data.csv", data: []byte("id,name,size\n1,photo.jpg,107\n2,sample.mp4,45056\n")},
	}
	aeSpecs := []mockFileSpec{
		{relativePath: "02-alist-encrypt/secret.ae", data: makeAEFile("secret.ae", 4096)},
		{relativePath: "02-alist-encrypt/document.ae", data: makeAEFile("document.ae", 8192)},
		{relativePath: "02-alist-encrypt/hidden-gem.ae", data: makeAEFile("hidden-gem.ae", 16384)},
	}
	containerSpecs := []mockFileSpec{
		{relativePath: "03-encv-containers/container.sccgv", data: makeSCCVFile("container", "sccgv", 8192)},
		{relativePath: "03-encv-containers/archive.scext", data: makeSCCVFile("archive", "scext", 16384)},
		{relativePath: "03-encv-containers/bundle.scepkg", data: makeSCCVFile("bundle", "scepkg", 32768)},
	}
	boundarySpecs := []mockFileSpec{
		{relativePath: "04-boundary-test/zero-byte-file.bin", data: []byte{}},
		{relativePath: "04-boundary-test/single-byte.bin", data: []byte{0x42}},
		{relativePath: "04-boundary-test/exactly-1kb.bin", data: makeBytes(1024, 0x41)},
		{relativePath: "04-boundary-test/large-1mb.dat", data: makeBytes(1024*1024, 0x58)},
		{relativePath: "04-boundary-test/normal.txt", data: []byte("plain text")},
	}

	switch typeName {
	case "plain":
		return plainSpecs
	case "ae":
		return aeSpecs
	case "container":
		return containerSpecs
	case "boundary":
		return boundarySpecs
	case "all", "":
		return append(append(append(plainSpecs, aeSpecs...), containerSpecs...), boundarySpecs...)
	default:
		return nil
	}
}

// handleMockGenerateGin 处理 POST /api/mock/generate
// SSE response:
//   - event: progress  data: { "relativePath": "...", "size": N }
//   - event: done      data: { "count": N, "totalSize": M }
func (s *Server) handleMockGenerateGin(c *gin.Context) {
	// 🆕 2026-06-10：显式意图确认（防擅自生成）
	//  - 防止 preflight / 第三方爬虫 / 误调触发数据生成
	//  - 前端 UI 按钮自动带 X-Confirm-Mock-Mutation: yes
	//  - Node CLI 已废弃，不存在自动调用方
	if c.GetHeader("X-Confirm-Mock-Mutation") != "yes" {
		slog.Warn("Mock generate rejected: missing confirm header")
		c.JSON(http.StatusForbidden, gin.H{
			"error": "X-Confirm-Mock-Mutation header required (UI 按钮自动带；防擅自生成)",
		})
		return
	}

	var req mockGeneratorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}

	if err := validateMockRoot(req.Root); err != nil {
		slog.Warn("Mock generate rejected: root not in allowlist", "root", req.Root, "error", err)
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	// 规范化 root
	root := filepath.Clean(req.Root)
	if !filepath.IsAbs(root) {
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
	}

	specs := generateMockSpecs(req.Type)
	if specs == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid type: " + req.Type})
		return
	}

	// SSE writer
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}

	enc := json.NewEncoder(c.Writer)

	count := 0
	var totalSize int64
	for _, sp := range specs {
		fullPath := filepath.Join(root, sp.relativePath)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			emitSseEvent(c.Writer, flusher, "error", fmt.Sprintf(`{"error": "mkdir %s: %s"}`, dir, err.Error()))
			return
		}
		if err := os.WriteFile(fullPath, sp.data, 0644); err != nil {
			emitSseEvent(c.Writer, flusher, "error", fmt.Sprintf(`{"error": "write %s: %s"}`, sp.relativePath, err.Error()))
			return
		}
		count++
		totalSize += int64(len(sp.data))
		_ = enc.Encode(mockGeneratorProgress{RelativePath: sp.relativePath, Size: len(sp.data)})
		// SSE event: progress
		writeSseEvent(c.Writer, flusher, "progress", fmt.Sprintf(`{"relativePath": %q, "size": %d}`, sp.relativePath, len(sp.data)))
	}
	writeSseEvent(c.Writer, flusher, "done", fmt.Sprintf(`{"count": %d, "totalSize": %d}`, count, totalSize))
}

// mockResetRequest 是 POST /api/mock/reset 的请求体
type mockResetRequest struct {
	Root string `json:"root"`
}

// handleMockResetGin 处理 POST /api/mock/reset
// 🆕 2026-06-10 修复：递归删除 mockRoot 下的 4 个子目录全部内容
// 历史 bug：只删 generateMockSpecs 列出的具体文件，但 02-test-output 等其他子目录不删
// 修复：清空 4 个已知子目录（01-plain-media / 02-alist-encrypt / 03-encv-containers / 04-boundary-test）
//       + 02-test-output（自动化测试运行时生成的产物），保留目录结构
func (s *Server) handleMockResetGin(c *gin.Context) {
	var req mockResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	if err := validateMockRoot(req.Root); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	root := filepath.Clean(req.Root)
	if !filepath.IsAbs(root) {
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
	}

	// 已知子目录（保留目录结构，删除其中内容）
	knownSubdirs := []string{
		"01-plain-media",
		"02-alist-encrypt",
		"03-encv-containers",
		"04-boundary-test",
		// 🆕 自动化测试运行产物（buildDynamicWorkflow 用 targetPath 写到这里的子目录）
		"02-test-output",
	}

	removed := 0
	for _, sub := range knownSubdirs {
		dir := filepath.Join(root, sub)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		// 遍历子目录中所有文件并删除
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil // 跳过不可访问的文件
			}
			if d.IsDir() {
				return nil
			}
			if rmErr := os.Remove(path); rmErr == nil {
				removed++
			}
			return nil
		})
		if err != nil {
			slog.Warn("Mock reset: walk failed", "dir", dir, "error", err)
		}
	}

	// 同时尝试删除 generateMockSpecs 中已知的具体文件（防御性，保留对旧版兼容）
	for _, sp := range generateMockSpecs("all") {
		fullPath := filepath.Join(root, sp.relativePath)
		if err := os.Remove(fullPath); err == nil {
			removed++
		}
	}

	c.JSON(http.StatusOK, gin.H{"removed": removed})
}

// writeSseEvent 写一个 SSE 事件（event: <name>\ndata: <payload>\n\n）
func writeSseEvent(w io.Writer, flusher http.Flusher, event, data string) {
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	flusher.Flush()
}

// emitSseEvent 是 writeSseEvent 的语义化别名（用于错误事件）
func emitSseEvent(w io.Writer, flusher http.Flusher, event, data string) {
	writeSseEvent(w, flusher, event, data)
}

// ════════════════════════════════════════════════════════════════════
// 最小有效字节模板（ffmpeg 优先，无 base64 fallback）
// ════════════════════════════════════════════════════════════════════
//
// 2026-06-11 改造：删 base64 fallback（真机上就是 171 字节假 MKV，垃圾）
//
// 优先级：
//   1. 走 internal/utils/ffmpeg.Runner 抽象调 ffmpeg
//      - 沙箱：ExecRunner (os/exec /usr/bin/ffmpeg)
//      - 真机：NativeRunner (cgo dlopen libffmpeg.so ffmpeg_run)
//   2. ffmpeg 不可用或失败 → 返回 nil（**严禁 base64 fallback**）
//      上层 handleMockGenerateGin 写 0 字节文件，调用方会看到错误
//
// mp4/mkv 必须显式 -map 0:a -map 1:v（sine+color 2 个 input 不指定 stream 会失败）
//
// 单次 ffmpeg 调用 ~100-300ms，4 个文件串行最多 ~1.2s。可接受。
// ════════════════════════════════════════════════════════════════════

// ffmpegGenerate 调用 ffmpeg 生成指定格式的媒体字节
//
// 2026-06-11 改造：改走 internal/utils/ffpeg.Runner 抽象层
//   - 沙箱：ExecRunner (os/exec 调 /usr/bin/ffmpeg)
//   - 真机：NativeRunner (cgo dlopen 调 libffmpeg.so ffmpeg_run)
//
// 失败返回 nil（**严禁 base64 fallback** —— 真机就是 171 字节假 MKV）
// 调用方 minimalMP4/MKV/MP3/FLAC 会把 nil 透传给上层 generateMockSpecs，
// 上层 handleMockGenerateGin 写到磁盘时直接 0 字节，调用方会看到错误
func ffmpegGenerate(args []string, ext string) []byte {
	// 0. 先 ffmpeg.Available() 检查（Runner 内部已缓存）
	ffmpegOk, ffprobeOk, errMsg := ffmpeg.Available()
	if !ffmpegOk && !ffprobeOk {
		slog.Warn("[mock] ffmpeg runner not available, returning nil (no base64 fallback)", "ext", ext, "errMsg", errMsg)
		return nil
	}

	// 1. 写 temp file（ffmpeg 输出到 file，stdout 不行）
	tmpFile, err := os.CreateTemp("", "encv-mock-*"+"."+ext)
	if err != nil {
		slog.Warn("[mock] create temp file failed", "ext", ext, "err", err)
		return nil
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	ffmpegArgs := append(append([]string{}, args...), "-y", "-loglevel", "error", tmpPath)

	// 2. 跨平台调 ffmpeg（沙箱 exec / 真机 dlopen）
	ctx := context.Background()
	_, stderr, exitCode, err := ffmpeg.RunWithOutput(ctx, ffmpegArgs...)
	if err != nil || exitCode != 0 {
		slog.Warn("[mock] ffmpeg generate failed (no base64 fallback)", "ext", ext, "exitCode", exitCode, "err", err, "stderr", stderr)
		return nil
	}

	// 3. 读回 temp file
	data, err := os.ReadFile(tmpPath)
	if err != nil || len(data) == 0 {
		slog.Warn("[mock] read temp file failed", "ext", ext, "err", err, "size", len(data))
		return nil
	}
	slog.Info("[mock] ffmpeg generated media", "ext", ext, "size", len(data))
	return data
}

func minimalJPEG() []byte {
	// 来自 scripts/generate-mock-files.ts 1:1 对应
	return []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9,
	}
}

func minimalPNG() []byte {
	// 8-byte PNG signature + 简化的 IHDR/IDAT/IEND chunk
	sig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	// IHDR: 1x1 RGB
	ihdrData := []byte{0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02, 0x00, 0x00, 0x00}
	ihdr := makePngChunk("IHDR", ihdrData)
	// IDAT: 1x1 RGB filter+RGB (filter=0, R=128, G=128, B=128)
	idatData := []byte{0x00, 0x80, 0x80, 0x80}
	idat := makePngChunk("IDAT", idatData)
	// IEND
	iend := makePngChunk("IEND", nil)
	return append(append(append(sig, ihdr...), idat...), iend...)
}

func makePngChunk(typ string, data []byte) []byte {
	out := []byte{
		byte(len(data) >> 24), byte(len(data) >> 16), byte(len(data) >> 8), byte(len(data)),
	}
	out = append(out, []byte(typ)...)
	out = append(out, data...)
	// CRC 占位（PNG 解码器对 CRC 校验在严格模式下会失败，但对我们的 mock 需求可接受）
	crc := pngCrc32(append([]byte(typ), data...))
	out = append(out, byte(crc>>24), byte(crc>>16), byte(crc>>8), byte(crc))
	return out
}

// pngCrc32 是 PNG 用的 CRC-32（与 zip 相同算法）
func pngCrc32(buf []byte) uint32 {
	table := pngCrcTable()
	crc := uint32(0xFFFFFFFF)
	for _, b := range buf {
		crc = table[(crc^uint32(b))&0xFF] ^ (crc >> 8)
	}
	return crc ^ 0xFFFFFFFF
}

var pngCrcCache [256]uint32

func pngCrcTable() [256]uint32 {
	for i := range pngCrcCache {
		c := uint32(i)
		for j := 0; j < 8; j++ {
			if c&1 != 0 {
				c = 0xEDB88320 ^ (c >> 1)
			} else {
				c = c >> 1
			}
		}
		pngCrcCache[i] = c
	}
	return pngCrcCache
}

func minimalMP4() []byte {
	// 2026-06-11 修复：加 -map 0:a -map 1:v（sine + color 2 input 不指定 stream 会失败）
	// 沙箱实测：ffmpeg 6.1 → test_mp4_simple.mp4 = 19801 字节 (h264+aac, 2.0s)
	if data := ffmpegGenerate([]string{
		"-f", "lavfi",
		"-i", "sine=frequency=440:duration=2",
		"-f", "lavfi",
		"-i", "color=c=0x3B82F6:s=320x240:d=2:r=15",
		"-map", "0:a",
		"-map", "1:v",
		"-c:v", "libx264", "-preset", "ultrafast", "-tune", "stillimage", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "64k",
		"-shortest",
	}, "mp4"); data != nil {
		return data
	}
	// 严禁 base64 fallback —— 返回 nil 让上层报错
	return nil
}

func minimalMKV() []byte {
	// 2026-06-11 修复：加 -map 0:a -map 1:v
	// 沙箱实测：ffmpeg 6.1 → test_mkv_simple.mkv = 9453 字节 (vorbis+h264, 2.0s)
	if data := ffmpegGenerate([]string{
		"-f", "lavfi",
		"-i", "sine=frequency=660:duration=2",
		"-f", "lavfi",
		"-i", "color=c=0x10B981:s=160x120:d=2:r=10",
		"-map", "0:a",
		"-map", "1:v",
		"-c:v", "libx264", "-preset", "ultrafast", "-tune", "stillimage", "-pix_fmt", "yuv420p",
		"-c:a", "libvorbis",
		"-shortest",
	}, "mkv"); data != nil {
		return data
	}
	// 严禁 base64 fallback
	return nil
}

func minimalMP3() []byte {
	// 2026-06-11 修复：删 base64 fallback
	// 沙箱实测：ffmpeg 6.1 → test_mp3.mp3 = 33062 字节 (libmp3lame, 2.0s)
	if data := ffmpegGenerate([]string{
		"-f", "lavfi",
		"-i", "sine=frequency=440:duration=2",
		"-c:a", "libmp3lame", "-b:a", "128k",
	}, "mp3"); data != nil {
		return data
	}
	// 严禁 base64 fallback
	return nil
}

func minimalFLAC() []byte {
	// 2026-06-11 修复：删 base64 fallback
	// 沙箱实测：ffmpeg 6.1 → test_flac.flac = 32487 字节 (flac, 2.0s)
	if data := ffmpegGenerate([]string{
		"-f", "lavfi",
		"-i", "sine=frequency=440:duration=2",
		"-c:a", "flac", "-sample_fmt", "s16",
	}, "flac"); data != nil {
		return data
	}
	// 严禁 base64 fallback
	return nil
}

func minimalPDF() []byte {
	return []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n%%EOF\n")
}

func makeAEFile(name string, targetSize int) []byte {
	// AENC magic + name + padding
	header := []byte{'A', 'E', 'N', 'C', 0x01, 0x00, byte(len(name))}
	header = append(header, []byte(name)...)
	header = append(header, 0x00)
	out := make([]byte, targetSize)
	copy(out, header)
	return out
}

func makeSCCVFile(name, ext string, targetSize int) []byte {
	manifest := fmt.Sprintf(`{"version":"4.0","originalName":%q,"originalExt":%q,"algorithm":"aes-256-gcm","createdAt":"2026-01-01T00:00:00Z","entries":[{"type":"file","name":%q,"size":%d}]}`, name, ext, name, targetSize-256)
	header := []byte{'S', 'C', 'C', 'V', 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20}
	header = append(header, []byte(manifest)...)
	out := make([]byte, targetSize)
	copy(out, header)
	return out
}

func makeBytes(n int, fill byte) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = fill
	}
	return out
}
