package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/types"
)

// Player 封装了流媒体服务器
type Player struct {
	dir      string
	password string
	server   *http.Server
}

// NewPlayer 创建一个新的 Player 实例
func NewPlayer(dir, password string) *Player {
	return &Player{dir: dir, password: password}
}

// Start 启动服务器，返回监听的地址
// 这个方法现在是非阻塞的。
func (p *Player) Start(port int) (string, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleRequest)

	addr := fmt.Sprintf(":%d", port)
	p.server = &http.Server{Addr: addr, Handler: mux}

	// 在一个新的 goroutine 中启动服务器，这样它就不会阻塞当前函数
	go func() {
		// 这个日志现在会在服务器真正开始监听时打印
		log.Printf("-> Server is now listening on %s\n", addr)
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// 如果服务器出错，我们在这里打印错误
			log.Printf("-> Server error: %v\n", err)
		}
	}()

	// 立即返回地址，不等待
	return addr, nil
}

// Stop 停止服务器
func (p *Player) Stop() {
	if p.server != nil {
		p.server.Close()
	}
}

func (p *Player) handleRequest(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Serve subtitle/danmaku tracks
	if strings.HasPrefix(path, "/subtitle/") {
		p.serveTrack(w, r, path[len("/subtitle/"):])
		return
	}

	// Serve video playlist or segments
	if strings.HasPrefix(path, "/video/") {
		p.serveVideo(w, r, path[len("/video/"):])
		return
	}

	http.NotFound(w, r)
}

func (p *Player) serveTrack(w http.ResponseWriter, r *http.Request, filename string) {
	trackPath := filepath.Join(p.dir, filename)
	if _, err := os.Stat(trackPath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}
	log.Printf("-> [Track] Serving track: %s\n", filename)
	contentType := "text/plain"
	if strings.HasSuffix(filename, ".vtt") {
		contentType = "text/vtt"
	}
	w.Header().Set("Content-Type", contentType)
	http.ServeFile(w, r, trackPath)
}

func (p *Player) serveVideo(w http.ResponseWriter, r *http.Request, reqPath string) {
	// --- 关键新增：规范化请求名称 ---
	// 用户可能请求 "sample.vkm" 或 "sample.vkm.enc"，我们将其统一处理
	baseName := reqPath
	if strings.HasSuffix(baseName, ".enc") {
		baseName = baseName[:len(baseName)-4]
		log.Printf("-> [Request] User requested .enc file, normalizing to base name: %s", baseName)
	}

	// 检查是否是播放列表请求（以 / 结尾）
	// 这个检查基于原始的 reqPath，以区分 /video/sample/ 和 /video/sample
	if strings.HasSuffix(reqPath, "/") {
		// 从 baseName 中移除尾部斜杠，得到逻辑名称
		logicalName := strings.TrimSuffix(baseName, "/")
		p.servePlaylist(w, r, logicalName)
		return
	}

	// 否则，它是对单个视频文件的直接请求
	p.serveSegment(w, r, baseName)
}

func (p *Player) serveSegment(w http.ResponseWriter, r *http.Request, name string) {
	encPath := filepath.Join(p.dir, name+".enc")
	kviPath := filepath.Join(p.dir, name+".kvi")

	index, err := p.loadIndex(kviPath)
	if err != nil {
		http.Error(w, "Index file not found", http.StatusNotFound)
		return
	}

	// --- 关键修正：解码 Base64 盐值 ---
	salt, err := crypto.Base64Decode(index.Encryption.SaltBase64)
	if err != nil {
		http.Error(w, "Invalid salt in index file", http.StatusInternalServerError)
		return
	}
	key := crypto.GenerateKey(p.password, salt)

	iv, err := crypto.Base64Decode(index.Encryption.IVBase64)
	if err != nil {
		http.Error(w, "Invalid IV in index file", http.StatusInternalServerError)
		return
	}

	// --- 改进：从索引文件中设置正确的 Content-Type ---
	w.Header().Set("Content-Type", "video/"+index.Format)

	// 注意：这是一个简化的实现，它会读取整个文件到内存中解密。
	// 对于大文件，这非常消耗内存。生产环境应使用流式解密。
	encFile, err := os.Open(encPath)
	if err != nil {
		http.Error(w, "Encrypted file not found", http.StatusNotFound)
		return
	}
	defer encFile.Close()

	if err := crypto.DecryptStream(encFile, w, key, iv); err != nil {
		// 如果写入已经开始，此时返回错误可能为时已晚，但这是最好的做法
		log.Printf("Error decrypting stream: %v\n", err)
	}
}

func (p *Player) servePlaylist(w http.ResponseWriter, r *http.Request, logicalName string) {
	parts, err := p.findParts(logicalName)
	if err != nil || len(parts) == 0 {
		http.NotFound(w, r)
		return
	}

	// Load index from the first part to get subtitle info
	firstPartBase := strings.TrimSuffix(parts[0], ".enc")
	index, _ := p.loadIndex(filepath.Join(p.dir, firstPartBase+".kvi"))

	var subtitleTags strings.Builder
	if index != nil {
		for _, sub := range index.Subtitles {
			subtitleTags.WriteString(fmt.Sprintf(`#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="%s",LANGUAGE="%s",URI="/subtitle/%s"`+"\n", sub.Title, sub.Language, sub.Filename))
		}
	}

	var playlist strings.Builder
	playlist.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
	playlist.WriteString(subtitleTags.String())
	playlist.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-ALLOW-CACHE:YES\n")

	for _, part := range parts {
		baseName := strings.TrimSuffix(part, ".enc")
		playlist.WriteString(fmt.Sprintf("#EXTINF:-1,%s\n/video/%s\n", baseName, baseName))
	}
	playlist.WriteString("#EXT-X-ENDLIST\n")

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Write([]byte(playlist.String()))
}

func (p *Player) loadIndex(indexPath string) (*types.VideoIndex, error) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}
	var index types.VideoIndex
	return &index, json.Unmarshal(data, &index)
}

func (p *Player) findParts(logicalName string) ([]string, error) {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		return nil, err
	}

	var parts []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".enc") {
			continue
		}
		if strings.HasPrefix(e.Name(), logicalName) {
			parts = append(parts, e.Name())
		}
	}
	return parts, nil
}
