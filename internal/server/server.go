package server

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/utils"
)

// Player ... (保持不变) ...
type Player struct {
	dir      string
	password string
	server   *http.Server
}

// NewPlayer ... (保持不变) ...
func NewPlayer(dir, password string) *Player {
	return &Player{dir: dir, password: password}
}

// Start ... (保持不变) ...
func (p *Player) Start(port int) (string, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleRequest)

	addr := fmt.Sprintf(":%d", port)
	p.server = &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	go func() {
		log.Printf("-> Server is now listening on %s\n", addr)
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("-> Server error: %v\n", err)
		}
	}()

	return addr, nil
}

// Stop ... (保持不变) ...
func (p *Player) Stop() {
	if p.server != nil {
		p.server.Close()
	}
}

// 主路由处理器
func (p *Player) handleRequest(w http.ResponseWriter, r *http.Request) {
	// 从 URL 路径中提取相对于服务器根目录的路径
	// 例如："/output/321.sccgv" -> "output/321.sccgv"
	// 例如："/" -> ""
	relativePath := strings.TrimPrefix(r.URL.Path, "/")

	// 如果请求的是根目录，则列出服务器根目录的文件
	if relativePath == "" {
		p.listFilesInDir(w, r, p.dir, "/")
		return
	}

	// 否则，作为文件或目录请求处理
	p.servePath(w, r, relativePath)
}

// 新的服务函数，能处理文件和目录
func (p *Player) servePath(w http.ResponseWriter, r *http.Request, relativePath string) {
	// 构建完整的文件系统路径
	fullPath := filepath.Join(p.dir, relativePath)

	// 安全检查：确保请求的路径在服务目录内
	if !strings.HasPrefix(filepath.Clean(fullPath)+string(os.PathSeparator), filepath.Clean(p.dir)+string(os.PathSeparator)) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 检查路径信息
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Could not access path", http.StatusInternalServerError)
		}
		return
	}

	// 如果是目录，则列出该目录的内容
	if info.IsDir() {
		// 为目录 URL 添加末尾斜杠，以便正确生成相对链接
		urlPath := "/" + strings.TrimSuffix(relativePath, "/") + "/"
		p.listFilesInDir(w, r, fullPath, urlPath)
		return
	}

	// 如果是文件，则继续处理
	p.serveFile(w, r, fullPath)
}

// serveFile 处理对单个文件的请求
func (p *Player) serveFile(w http.ResponseWriter, r *http.Request, fullPath string) {
	fileName := filepath.Base(fullPath)

	// 判断是否是 ENCV 容器文件
	if config.IsContainerFile(fileName) {
		log.Printf("-> [File] Serving ENCV container: %s", fileName)
		// p.serveEncryptedContainer(w, r, fullPath)
		// 【关键修改】调用封装后的函数，不再关心内部是分片还是单文件
		p.serveEncryptedContent(w, r, fullPath)
		return
	}

	// 如果不是容器文件，作为普通文件（如字幕）提供服务
	log.Printf("-> [File] Serving standard file: %s", fileName)
	http.ServeFile(w, r, fullPath)
}

// listFilesInDir 在指定目录生成一个文件列表页面
// urlPath 是当前目录对应的 URL 路径，用于生成正确的导航链接
func (p *Player) listFilesInDir(w http.ResponseWriter, r *http.Request, dirPath, urlPath string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		http.Error(w, "Could not read directory", http.StatusInternalServerError)
		return
	}

	type FileInfo struct {
		Name        string
		Path        string
		IsDir       bool
		IsContainer bool
		Size        int64
		ModTime     time.Time
	}

	var files []FileInfo
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, FileInfo{
			Name:        entry.Name(),
			Path:        urlPath + entry.Name(),
			IsDir:       entry.IsDir(),
			IsContainer: !entry.IsDir() && config.IsContainerFile(entry.Name()),
			Size:        info.Size(),
			ModTime:     info.ModTime(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		// 目录排在文件前面
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	// 计算父目录链接
	parentPath := filepath.Dir(urlPath)
	if parentPath == "." {
		parentPath = "/"
	}

	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>ENCV File Server - {{.CurrentPath}}</title>
    <style>
        body { font-family: sans-serif; background-color: #f4f4f9; color: #333; margin: 2em; }
        h1 { color: #444; }
        .breadcrumb { margin-bottom: 1em; }
        .breadcrumb a { color: #007BFF; text-decoration: none; }
        .breadcrumb a:hover { text-decoration: underline; }
        table { width: 100%; border-collapse: collapse; margin-top: 1em; }
        th, td { padding: 12px; border: 1px solid #ddd; text-align: left; }
        th { background-color: #4CAF50; color: white; }
        tr:nth-child(even) { background-color: #f2f2f2; }
        a { text-decoration: none; color: #007BFF; }
        a:hover { text-decoration: underline; }
        .dir-tag { color: #007BFF; font-weight: bold; }
        .container-tag { color: #d9534f; font-weight: bold; }
    </style>
</head>
<body>
    <h1>ENCV File Server</h1>
    <div class="breadcrumb">
        <a href="/">Root</a> / {{range .Ancestors}}<a href="{{.Path}}">{{.Name}}</a> / {{end}}
    </div>
    <table>
        <thead>
            <tr>
                <th>Name</th>
                <th>Type</th>
                <th>Size</th>
                <th>Modified</th>
            </tr>
        </thead>
        <tbody>
            {{if .NotRoot}}<tr><td><a href="{{.ParentPath}}">..</a></td><td>Directory</td><td>-</td><td>-</td></tr>{{end}}
            {{range .Files}}
            <tr>
                <td><a href="{{.Path}}">{{.Name}}</a></td>
                <td>
                    {{if .IsDir}}<span class="dir-tag">Directory</span>
                    {{else if .IsContainer}}<span class="container-tag">ENCV Container</span>
                    {{else}}File
                    {{end}}
                </td>
                <td>{{if .IsDir}}-{{else}}{{.Size}}{{end}}</td>
                <td>{{.ModTime.Format "2006-01-02 15:04:05"}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
</body>
</html>`

	// 为模板准备数据
	data := struct {
		CurrentPath string
		ParentPath  string
		NotRoot     bool
		Ancestors   []struct{ Name, Path string }
		Files       []FileInfo
	}{
		CurrentPath: urlPath,
		ParentPath:  parentPath,
		NotRoot:     urlPath != "/",
		Files:       files,
	}
	// 生成面包屑导航
	parts := strings.Split(strings.Trim(urlPath, "/"), "/")
	for i := 0; i < len(parts)-1; i++ {
		data.Ancestors = append(data.Ancestors, struct{ Name, Path string }{
			Name: parts[i],
			Path: "/" + strings.Join(parts[:i+1], "/") + "/",
		})
	}

	t, _ := template.New("list").Parse(tmpl)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		log.Printf("Error executing template: %v", err)
	}
}

// 统一处理所有加密内容的请求（无论是分片还是单文件）
func (p *Player) serveEncryptedContent(w http.ResponseWriter, r *http.Request, anyChunkPath string) {
	// 1. 【核心修复】根据任意一个分片，找到主分片
	mainChunkPath, err := container.FindMainChunk(anyChunkPath)
	if err != nil {
		log.Printf("-> [File] Failed to find main chunk for '%s': %v", anyChunkPath, err)
		http.Error(w, "Invalid or incomplete chunk set", http.StatusBadRequest)
		return
	}
	log.Printf("-> [File] Found main chunk: %s", filepath.Base(mainChunkPath))

	// 2. 【核心修复】使用新的分片解包函数
	packedData, err := container.UnpackChunked(mainChunkPath)
	if err != nil {
		log.Printf("-> [File] Failed to unpack main chunk: %v", err)
		http.Error(w, "Invalid container file", http.StatusBadRequest)
		return
	}

	// 【防御性修复】检查 packedData 是否为 nil，以防止 panic
	if packedData == nil {
		// 这种情况理论上不应该发生，但为了健壮性，我们处理它
		log.Printf("-> [File] CRITICAL: UnpackChunked returned nil data without error. This is a bug.")
		http.Error(w, "Internal server error during unpacking", http.StatusInternalServerError)
		return
	}
	defer packedData.VideoStream.Close()

	// 3. 解析 KVI 数据
	index, err := crypto.UnmarshalKVI(packedData.KVIData)
	if err != nil {
		log.Printf("-> [File] Failed to parse KVI: %v", err)
		http.Error(w, "Failed to parse container metadata", http.StatusInternalServerError)
		return
	}

	// 4. 准备解密密钥
	salt, err := crypto.Base64Decode(index.Encryption.SaltBase64)
	if err != nil {
		log.Printf("-> [File] Failed to decode salt: %v", err)
		http.Error(w, "Failed to decode container metadata", http.StatusInternalServerError)
		return
	}
	key := crypto.GenerateKey(p.password, salt)

	// 5. 设置响应头
	w.Header().Set("Content-Type", utils.GetContentType(index.Format))
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", index.OriginalFilename))
	w.Header().Set("Accept-Ranges", "bytes")

	// 6. 流式解密并写入响应体
	decryptedReader, err := crypto.GetDecryptReader(packedData.VideoStream, key)
	if err != nil {
		log.Printf("-> [File] Failed to create decrypt reader: %v", err)
		http.Error(w, "Failed to initialize decryption", http.StatusInternalServerError)
		return
	}

	if _, err := io.Copy(w, decryptedReader); err != nil {
		if !isConnectionClosedError(err) {
			log.Printf("-> [File] Error streaming decrypted content: %v", err)
		}
	}
}

// isConnectionClosedError 判断错误是否由客户端断开连接引起
func isConnectionClosedError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "connection reset by peer") || strings.Contains(errStr, "broken pipe")
}
