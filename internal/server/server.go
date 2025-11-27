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
	"github.com/Soltus/encv-go/internal/container/chunked"
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// Player ... (保持不变) ...
type Player struct {
	dir             string
	contentPassword string // 【修改】从 password 改为 contentPassword
	server          *http.Server
}

func NewPlayer(dir, password string) *Player {
	return &Player{dir: dir, contentPassword: password} // 【修改】使用新字段名
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

	t, _ := template.New("list").Parse(tmpl_dynamic_files)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		log.Printf("Error executing template: %v", err)
	}
}

// 统一处理所有加密内容的请求（无论是分片还是单文件）
func (p *Player) serveEncryptedContent(w http.ResponseWriter, r *http.Request, anyChunkPath string) {
	// 1. 打开文件并读取头部用于类型检测
	file, err := os.Open(anyChunkPath)
	if err != nil {
		log.Printf("-> [File] Failed to open file: %v", err)
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// 2. 动态确定最大魔法数字长度
	magicMap := container.GetContainerMagicMap()
	maxMagicLen := 0
	for _, magic := range magicMap {
		if len(magic) > maxMagicLen {
			maxMagicLen = len(magic)
		}
	}

	magicHeader := make([]byte, maxMagicLen)
	bytesRead, err := io.ReadFull(file, magicHeader)
	if err != nil && err != io.ErrUnexpectedEOF {
		log.Printf("-> [File] Failed to read container magic header: %v", err)
		http.Error(w, "Invalid container file", http.StatusBadRequest)
		return
	}
	magicHeader = magicHeader[:bytesRead]

	// 3. 检测容器类型
	detectedExt, err := container.DetectContainerType(magicHeader)
	if err != nil {
		log.Printf("-> [File] Failed to detect container type: %v", err)
		http.Error(w, "Unknown container format", http.StatusBadRequest)
		return
	}
	log.Printf("-> [File] Detected container extension: %s", detectedExt)

	// 4. 【关键修复】根据类型选择解包方式
	var packedData *container.PackedData
	switch detectedExt {
	case config.GlobalConfig.BinExtGroup.Video:
		// 视频是分片容器，需要特殊处理
		// 【修改】从 container 包获取魔法数字
		mainMagic := magicMap[detectedExt]
		subMagicMap := container.GetSubChunkMagicMap()
		subMagic := subMagicMap[detectedExt]

		// 【关键修复】使用新的 chunked.NewReader，它会自动查找主分片
		chunkedReader, err := chunked.LocalReader(anyChunkPath, mainMagic, subMagic)
		if err != nil {
			log.Printf("-> [File] Failed to create chunked reader from '%s': %v", anyChunkPath, err)
			http.Error(w, "Invalid or incomplete chunk set", http.StatusBadRequest)
			return
		}
		// 将 chunkedReader 包装成 PackedData
		packedData = &container.PackedData{
			KVIData:    chunkedReader.KVIData,
			DataStream: chunkedReader,
		}

	case config.GlobalConfig.BinExtGroup.Image, config.GlobalConfig.BinExtGroup.Audio, config.GlobalConfig.BinExtGroup.Text:
		// 单文件容器，可以直接解包
		// 将文件指针重置到开头，因为 DetectContainerType 已经读取了一部分
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			log.Printf("-> [File] Failed to seek file to start: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		magicMap := container.GetContainerMagicMap()
		packedData, err = container.Unpack(file, magicMap[detectedExt])

	default:
		log.Printf("-> [File] Unsupported container type: %s", detectedExt)
		http.Error(w, "Unsupported container type", http.StatusNotImplemented)
		return
	}

	if err != nil {
		log.Printf("-> [File] Failed to unpack container: %v", err)
		http.Error(w, "Invalid container file", http.StatusBadRequest)
		return
	}
	defer packedData.DataStream.Close() // 【修改】使用 DataStream

	// 5. 【关键修改】使用新的统一解析函数
	index, err := utils.UnmarshalKVI(packedData.KVIData)
	if err != nil {
		log.Printf("-> [File] Failed to parse KVI: %v", err)
		http.Error(w, "Failed to parse container metadata", http.StatusInternalServerError)
		return
	}

	// 6. 准备解密密钥
	salt, err := crypto.Base64Decode(index.GetEncryptionInfo().SaltBase64) // 【修改】使用接口方法
	if err != nil {
		log.Printf("-> [File] Failed to decode salt: %v", err)
		http.Error(w, "Invalid salt in index file", http.StatusInternalServerError)
		return
	}
	key := crypto.GenerateKey(p.contentPassword, salt) // 【修改】使用新字段名

	// 7. 【关键修改】根据类型设置响应头
	var contentType string
	switch idx := index.(type) {
	case *types.VideoIndex:
		contentType = utils.GetContentType(idx.Format)
	case *types.ImageIndex:
		contentType = idx.MimeType
	case *types.TextIndex:
		contentType = idx.MimeType
	default:
		contentType = "application/octet-stream" // 默认类型
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", index.GetOriginalFilename())) // inline 告诉浏览器“尝试显示”而不是“必须下载”
	w.Header().Set("Accept-Ranges", "bytes")

	// 8. 流式解密并写入响应体
	// 【修改】使用 DataStream
	decryptedReader, err := crypto.GetDecryptReader(packedData.DataStream, key)
	if err != nil {
		log.Printf("-> [File] Failed to create decrypt reader: %v", err)
		http.Error(w, "Failed to initialize decryption", http.StatusInternalServerError)
		return
	}

	if _, err := io.Copy(w, decryptedReader); err != nil {
		if !utils.IsConnectionClosedError(err) {
			log.Printf("-> [File] Error streaming decrypted content: %v", err)
		}
	}
}
