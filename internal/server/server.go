// internal/server/server.go
package server

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/middleware"
	"github.com/Soltus/encv-go/internal/register"
	"github.com/Soltus/encv-go/internal/v2/container/detector"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	"github.com/Soltus/encv-go/internal/v2/service"
	"github.com/Soltus/encv-go/internal/web"
	"github.com/Soltus/encv-go/internal/webdav"
	"github.com/dustin/go-humanize"
	goWebdav "golang.org/x/net/webdav"
)

type Server struct {
	server     *http.Server
	cfg        *config.Config
	servingDir string // 主服务目录的绝对路径
	version    string // 存储应用版本
	instanceID string // 存储本次启动的唯一实例ID
	webdavDir  string // WebDAV 目录的绝对路径
	webdavPath string // WebDAV 的路由前缀
	// 【关键替换】用新的 ReaderService 替代旧的 ContainerManager
	readerService *service.ReaderService
}

func NewServer(ctx context.Context) *Server {
	cfg := config.FromContext(ctx)
	containerManager := service.NewContainerManager()
	return &Server{
		cfg:           cfg,
		readerService: service.NewReaderService(containerManager),
		instanceID:    fmt.Sprintf("%x", time.Now().UnixNano()),
	}
}

func (s *Server) GetInstanceID() string {
	return s.instanceID
}

// Start 启动播放器服务器，如果端口被占用或被其他服务劫持，会自动递增尝试
func (s *Server) Start(version string) (string, error) {
	// 【关键修改】在启动时初始化版本和实例ID
	s.version = version // 从 main 包获取编译时注入的版本

	// 1. 解析并存储主服务目录
	dir := s.cfg.Server.Dir
	var err error

	s.servingDir, err = filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for directory '%s': %w", dir, err)
	}

	// 2. 解析并存储 WebDAV 目录和路径
	if s.cfg.Webdav.Dir != "" {
		s.webdavDir, err = filepath.Abs(s.cfg.Webdav.Dir)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute path for webdav dir '%s': %w", s.cfg.Webdav.Dir, err)
		}
		s.webdavPath = s.cfg.Webdav.Root
		if s.webdavPath == "" {
			s.webdavPath = "/webdav/"
		}
		if !strings.HasPrefix(s.webdavPath, "/") {
			s.webdavPath = "/" + s.webdavPath
		}
		if !strings.HasSuffix(s.webdavPath, "/") {
			s.webdavPath += "/"
		}
		log.Printf("WebDAV enabled. Serving '%s' at path '%s'", s.webdavDir, s.webdavPath)
	}

	log.Printf("Server starting instance %s with version %s", s.instanceID, s.version)
	log.Printf("Main service serving from: %s", s.servingDir)

	// 3. 创建统一的 ServeMux 并注册路由
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", s.handlePing)
	mux.HandleFunc("/stream", s.handleStreamRequest)
	mux.Handle("/_preview/", web.PreviewHandler())

	mux.HandleFunc("/", s.handleRequest)

	// 如果启用了 WebDAV，则注册其处理器
	if s.webdavDir != "" {
		fs := webdav.NewENCVFS(config.NewContext(context.Background(), s.cfg), s.readerService)
		webdavHandler := &goWebdav.Handler{
			FileSystem: fs,
			LockSystem: goWebdav.NewMemLS(),
		}
		// WebDAV 也需要通过配置中间件来处理密码等
		configAwareWebdavHandler := middleware.WithConfig(s.cfg, webdavHandler)
		mux.Handle(s.webdavPath, configAwareWebdavHandler)

	}

	// CorsMiddleware 应该在最外层，最先处理请求
	finalHandler := middleware.CorsMiddleware(middleware.WithConfig(s.cfg, mux))
	return register.StartHttpHandlerWithRetry(finalHandler, s.cfg.Server.Port, s.instanceID, s.version)
}

func (s *Server) Stop() error {
	s.readerService.Cleanup()
	if s.server != nil {
		log.Println("-> Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// handleRequest 是主路由 / 处理器
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("[DEBUG] handleRequest -> (r.URL.Path: %s", r.URL.Path)
	// 1. 从 URL 路径中移除开头的斜杠，得到相对于 servingDir 的路径
	// 例如，如果 URL 是 "/video.sccgv"，relativePath 就是 "video.sccgv"
	relativePath := strings.TrimPrefix(r.URL.Path, "/")

	// 2. 【安全】清理路径，防止 `..` 等路径遍历攻击
	//    filepath.Clean 会处理 `..` 和多余的 `.`
	cleanRelativePath := filepath.Clean(relativePath)

	// 3. 【安全】检查路径是否在尝试跳出目录
	//    如果 Clean 后的结果以 ".." 开头，说明是恶意请求
	if strings.HasPrefix(cleanRelativePath, "..") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 4. 【核心】构建服务器上的完整文件路径
	//    将清理后的相对路径与 servingDir 拼接
	// absPath := filepath.Join(s.servingDir, cleanRelativePath)
	// log.Printf("[DEBUG] handleRequest -> containerPath: %s", absPath)

	// // 5. 【可选但推荐】检查文件是否存在，如果不存在则返回 404
	// //    这可以避免后续的解密逻辑处理不存在的文件
	// if _, err := os.Stat(absPath); os.IsNotExist(err) {
	// 	http.NotFound(w, r)
	// 	return
	// }

	// 4. 将清理后的相对路径传递给 servePath
	s.servePath(w, r, cleanRelativePath)
}

// 能处理文件和目录
func (s *Server) servePath(w http.ResponseWriter, r *http.Request, relativePath string) {
	// 构建完整的文件系统路径
	fullPath := filepath.Join(s.servingDir, relativePath)

	// 使用 filepath.Rel 进行健壮的安全检查
	relPath, err := filepath.Rel(s.servingDir, fullPath)
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
		s.listFilesInDir(w, r, fullPath, urlPath)
		return
	}

	// 如果是文件，则继续处理
	s.serveFile(w, r, fullPath)
}

// serveFile 处理对单个文件的请求
func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, fullPath string) {
	fileName := filepath.Base(fullPath)

	// 判断是否是 ENCV 容器文件
	_, err := detector.DetectContainer(fullPath)
	if err == nil {
		// 如果 err 为 nil，说明文件是有效的 ENCV 容器
		log.Printf("-> [File] Serving ENCV container: %s", fileName)
		// 【关键修改】调用我们新的、统一的处理函数
		// false 表示这不是一个 /stream 端点，而是浏览器直接访问
		s.serveEncryptedFile(w, r, fullPath, false)
		return
	}

	// 如果不是容器文件，作为普通文件（如字幕）提供服务
	log.Printf("-> [File] Serving standard file: %s", fileName)
	http.ServeFile(w, r, fullPath)
}

// listFilesInDir 在指定目录生成一个文件列表页面
// urlPath 是当前目录对应的 URL 路径，用于生成正确的导航链接
func (s *Server) listFilesInDir(w http.ResponseWriter, r *http.Request, dirPath, urlPath string) {
	// 【核心】从 Header 中获取代理前缀
	forwardedPrefix := r.Header.Get("X-Forwarded-Prefix")
	if forwardedPrefix == "" {
		forwardedPrefix = "" // 如果没有代理，前缀为空
	}

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
		HumanSize   string // 【修改】使用 humanize 格式化后的大小
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

		// 【关键修改】在生成文件路径时，加上代理前缀
		files = append(files, FileInfo{
			Name:        entry.Name(),
			Path:        forwardedPrefix + urlPath + entry.Name(),
			IsDir:       entry.IsDir(),
			IsContainer: !entry.IsDir() && plugins.IsContainer(entry.Name()),
			HumanSize:   humanize.Bytes(uint64(info.Size())),
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

	// 【关键修改】在计算父目录链接时，加上代理前缀
	// 【关键修改】使用 path.Dir 处理 URL 路径，确保使用 '/'
	cleanedUrlPath := path.Clean(urlPath)
	parentPath := forwardedPrefix + path.Dir(cleanedUrlPath)
	if parentPath == forwardedPrefix+"." {
		parentPath = forwardedPrefix + "/"
	}

	// 为模板准备数据
	data := struct {
		CurrentPath string
		ParentPath  string
		NotRoot     bool
		RootPath    string // 【新增】用于面包屑的根路径
		Ancestors   []struct{ Name, Path string }
		Files       []FileInfo
	}{
		CurrentPath: urlPath, // CurrentPath 用于显示，不需要前缀
		ParentPath:  parentPath,
		NotRoot:     urlPath != "/",
		RootPath:    forwardedPrefix + "/", // 【新增】设置根路径
		Files:       files,
	}

	// 【关键修改】在生成面包屑导航的路径时，加上代理前缀
	parts := strings.Split(strings.Trim(urlPath, "/"), "/")
	for i := 0; i < len(parts); i++ {
		// 跳过空的部分，比如当 urlPath 是 "/" 时
		if parts[i] == "" {
			continue
		}
		// 【正确】使用 strings.Join 来构建路径片段
		ancestorPath := "/" + strings.Join(parts[:i+1], "/") + "/"
		data.Ancestors = append(data.Ancestors, struct{ Name, Path string }{
			Name: parts[i],
			Path: forwardedPrefix + ancestorPath, // 加上前缀
		})
	}

	t, _ := template.New("list").Parse(tmpl_dynamic_files)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		log.Printf("Error executing template: %v", err)
	}
}
