// internal/server/mock_generator.go
// 自动化测试用 Mock 数据生成 / 重置（Go 后端）
//
// 提供两个端点：
//   POST /api/mock/generate { root, type }  → SSE 流式进度
//   POST /api/mock/reset    { root }         → JSON { removed }
//
// 用途：自动化测试入口在前端触发时，需要把 mock 文件写入到：
//   - dev 模式：<project>/__mock_data__/01-plain-media 等
//   - 真机：    /storage/emulated/0/encv-automation/01-plain-media 等
// 前端在浏览器/真机 WebView 没有权限直写这些目录，必须走后端。
//
// 安全：root 必须在白名单前缀内（见 validateMockRoot），否则 403。
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// mockRootAllowList 是允许写入的根目录白名单（绝对路径前缀）。
// dev 模式：项目根 + "__mock_data__/"
// 真机：/storage/emulated/0/encv-automation/
// 其他路径一律 403。
var mockRootAllowList = []string{
	"__mock_data__",                                // dev: 相对项目根（运行时被转为绝对路径）
	"/storage/emulated/0/encv-automation",         // 真机
	"/sdcard/encv-automation",                     // 真机 symlink 兼容
	"/data/local/tmp/encv-automation",             // 调试用
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
// 删除 root 下所有 generateMockSpecs 产生的文件（不递归删 root 本体）
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

	// 删所有已知 specs（type=all）
	specs := generateMockSpecs("all")
	removed := 0
	for _, sp := range specs {
		fullPath := filepath.Join(root, sp.relativePath)
		if err := os.Remove(fullPath); err == nil {
			removed++
		} else if !os.IsNotExist(err) {
			slog.Warn("Mock reset: failed to remove", "path", fullPath, "error", err)
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

// ==================== 最小有效字节模板 ====================

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
	// ftyp box
	ftyp := []byte{
		0x00, 0x00, 0x00, 0x14, 'f', 't', 'y', 'p',
		'i', 's', 'o', 'm', 0x00, 0x00, 0x02, 0x00,
		'i', 's', 'o', 'm',
	}
	// minimal moov + mdat (各 8 bytes 头部)
	moov := []byte{0x00, 0x00, 0x00, 0x08, 'm', 'o', 'o', 'v'}
	mdat := []byte{0x00, 0x00, 0x00, 0x08, 'm', 'd', 'a', 't'}
	return append(append(ftyp, moov...), mdat...)
}

func minimalMKV() []byte {
	// EBML header + 0-size segment
	return []byte{
		0x1A, 0x45, 0xDF, 0xA3, // EBML magic
		0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // EBML size 0
		0x18, 0x53, 0x80, 0x67, // Segment
		0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // size 0
	}
}

func minimalMP3() []byte {
	// ID3v2 header + 1 silent MPEG frame
	return []byte{
		'I', 'D', '3', 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0A,
		0xFF, 0xFB, 0x90, 0x00, // MPEG audio frame header
	}
}

func minimalFLAC() []byte {
	// fLaC signature + minimal STREAMINFO
	return []byte{
		'f', 'L', 'a', 'C',
		0x00, 0x22, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
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
