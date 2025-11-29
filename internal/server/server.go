package server

import (
	"context"
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
	"github.com/Soltus/encv-go/internal/middleware"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

type Player struct {
	server *http.Server
	cfg    *config.Config
	// 新增：用于存储处理后的绝对路径
	servingDir string
}

func NewPlayer(ctx context.Context) *Player {
	cfg := config.FromContext(ctx)
	return &Player{
		cfg: cfg,
	}
}

func (p *Player) Start(port int) (string, error) {
	dir := p.cfg.Server.Dir
	var err error

	// 如果用户输入的是 "/"，将其转换为当前工作目录
	if dir == "/" {
		dir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
	}

	// 将任何相对路径（如 "./output"）都转换为绝对路径
	// filepath.Abs 会自动处理 "." 和 "./"
	p.servingDir, err = filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for directory '%s': %w", dir, err)
	}
	log.Printf("Player will serve from resolved directory: %s", p.servingDir)

	// 创建一个基础的 ServeMux
	mux := http.NewServeMux()
	// 注册我们真正的处理函数
	mux.HandleFunc("/", p.handleRequest)

	// 使用我们的中间件来包装 mux
	// 中间件负责将 p.cfg 注入到所有请求的 context 中
	configAwareHandler := middleware.WithConfig(p.cfg, mux)

	// 启动服务器时，使用被中间件包装过的 handler
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting player server on %s", addr)
	go func() {
		if err := http.ListenAndServe(addr, configAwareHandler); err != nil {
			log.Fatalf("Player server failed: %v", err)
		}
	}()

	return addr, nil
}

func (p *Player) Stop() {
	if p.server != nil {
		p.server.Close()
	}
}

// 主路由处理器
func (p *Player) handleRequest(w http.ResponseWriter, r *http.Request) {
	// 1. 从 URL 路径中移除开头的斜杠，得到相对路径
	relativePath := strings.TrimPrefix(r.URL.Path, "/")

	// 2. (可选但推荐) 进一步清理路径，防止 `..` 等路径遍历攻击
	//    filepath.Clean 会处理 `..` 和多余的 `.`
	cleanRelativePath := filepath.Clean(relativePath)

	// 3. (重要) 如果 Clean 后的结果是 ".."，说明是恶意请求
	if cleanRelativePath == ".." {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 4. 将清理后的相对路径传递给 servePath
	p.servePath(w, r, cleanRelativePath)
}

// 能处理文件和目录
func (p *Player) servePath(w http.ResponseWriter, r *http.Request, relativePath string) {
	// 构建完整的文件系统路径
	fullPath := filepath.Join(p.servingDir, relativePath)

	// 使用 filepath.Rel 进行健壮的安全检查
	relPath, err := filepath.Rel(p.servingDir, fullPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
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
	// 这个 context 应该已经被中间件注入了配置
	ctx := r.Context()
	cfg := config.FromContext(ctx)
	// 判断是否是 ENCV 容器文件
	if cfg.IsContainerFile(fileName) {
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
	// 这个 context 应该已经被中间件注入了配置
	ctx := r.Context()
	cfg := config.FromContext(ctx)
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
			IsContainer: !entry.IsDir() && cfg.IsContainerFile(entry.Name()),
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
	// 这个 context 应该已经被中间件注入了配置
	ctx := r.Context()
	cfg := config.FromContext(ctx)
	// 1. 打开文件并读取头部用于类型检测
	file, err := os.Open(anyChunkPath)
	if err != nil {
		log.Printf("-> [File] Failed to open file: %v", err)
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// 2. 动态确定最大魔法数字长度
	magicMap, err := container.GetContainerMagicMap(ctx)
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
	detectedExt, err := container.DetectContainerType(ctx, magicHeader)
	if err != nil {
		log.Printf("-> [File] Failed to detect container type: %v", err)
		http.Error(w, "Unknown container format", http.StatusBadRequest)
		return
	}
	log.Printf("-> [File] Detected container extension: %s", detectedExt)

	// 4. 【关键修复】根据类型选择解包方式
	var packedData *container.PackedData
	switch detectedExt {
	case cfg.BinExtGroup.Video:
		// 视频是分片容器，需要特殊处理
		// 【修改】从 container 包获取魔法数字
		mainMagic := magicMap[detectedExt]
		subMagicMap, err := container.GetSubChunkMagicMap(ctx)
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

	case cfg.BinExtGroup.Image, cfg.BinExtGroup.Audio, cfg.BinExtGroup.Text:
		// 单文件容器，可以直接解包
		// 将文件指针重置到开头，因为 DetectContainerType 已经读取了一部分
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			log.Printf("-> [File] Failed to seek file to start: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		magicMap, err := container.GetContainerMagicMap(ctx)
		packedData, err = container.Unpack(file, magicMap[detectedExt])
		if err != nil {
			log.Printf("-> [File] Failed '%s'", err)
			http.Error(w, "Invalid or incomplete", http.StatusBadRequest)
			return
		}

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
	key, err := crypto.GenerateKey([]byte(p.cfg.Password), salt, crypto.KeySize) // 只传入 KeySize
	if err != nil {
		log.Printf("-> [File] Failed to GenerateKey: %v", err)
		http.Error(w, "Invalid GenerateKey in index file", http.StatusInternalServerError)
		return
	}

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
