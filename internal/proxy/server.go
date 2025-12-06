package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/middleware"
	"github.com/Soltus/encv-go/internal/v2/openlist"
	"github.com/Soltus/encv-go/internal/v2/reader"
)

// 【新增】Proxy 结构体
type Proxy struct {
	cfg *config.Config
	// 【新增】工厂缓存
	factoryCache map[string]reader.DecryptReaderFactory
	cacheMutex   sync.RWMutex
}

// 【新增】NewProxy 构造函数
func NewProxy(ctx context.Context) *Proxy {
	cfg := config.FromContext(ctx)
	return &Proxy{
		cfg:          cfg,
		factoryCache: make(map[string]reader.DecryptReaderFactory),
	}
}

// StartServer 启动代理服务器
func (p *Proxy) StartServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleRequest)

	// 【关键】使用中间件链，CORS 在最外层
	// 1. WithConfig 注入配置到 context
	// 2. CorsMiddleware 处理所有跨域问题
	configAwareHandler := middleware.WithConfig(p.cfg, mux)
	finalHandler := middleware.CorsMiddleware(configAwareHandler)

	log.Printf("Starting ENCV proxy server on port %d", p.cfg.Proxy.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", p.cfg.Proxy.Port), finalHandler); err != nil {
		log.Fatalf("Failed to start proxy server: %v", err)
	}
}

// handleRequest 创建并返回 HTTP 处理函数
// 【关键修改】handleRequest 现在是一个方法，并且不再接收 ctx
func (p *Proxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	// 【关键】从请求的 context 中获取配置，因为中间件已经注入了
	cfg := config.FromContext(r.Context())

	path := r.URL.Path
	sign := r.URL.Query().Get("sign")
	isInternalRequest := r.URL.Query().Get("internal_request") == "true"

	if path == "" {
		http.Error(w, "Missing 'path' parameter", http.StatusBadRequest)
		return
	}
	// 签名验证
	if !isInternalRequest && !cfg.Proxy.DisableSignatureVerification {
		if sign == "" {
			http.Error(w, "Missing 'sign' parameter", http.StatusBadRequest)
			return
		}
		if !openlist.OpenListVerifySign(path, sign, cfg) {
			log.Printf("Invalid signature for path: %s", path)
			http.Error(w, "Forbidden: Invalid signature", http.StatusForbidden)
			return
		}
	} else if cfg.Proxy.DisableSignatureVerification {
		log.Printf("-> [Security] Signature verification is disabled, allowing request for: %s", path)
	} else {
		log.Printf("-> [Proxy] Handling internal request, skipping signature check for: %s", path)
	}

	log.Printf("Received valid request for: %s", path)

	// --- 核心逻辑：判断是否是 ENCV 容器文件 ---
	// openlist 依赖扩展名预览，因此直接使用扩展名判断的函数，不必检测 magic header
	if cfg.IsContainerPath(path) {
		log.Printf("-> [Proxy] Detected ENCV container file: %s", path)
		fileInfo, err := openlist.OpenListGetFileURL(path, cfg.Proxy.OpenListHost, cfg.Proxy.Token)
		if err != nil {
			log.Printf("Error getting file URL for %s: %v", path, err)
			http.Error(w, "Failed to locate file", http.StatusInternalServerError)
			return
		}
		// 【修改】使用通用的密码字段
		// 在 handleRequest 函数中
		p.serveEncryptedContainer(w, r, fileInfo.Data.URL, fileInfo.Data.Header, path)
		return
	}

	// --- 如果不是容器文件，则按普通文件处理 ---
	log.Printf("-> [Proxy] Not a container file, handling as standard file: %s", path)
	if strings.HasPrefix(path, "/p/") {
		log.Printf("-> [Proxy] Intercepted internal link: %s", path)
		fileURL := cfg.Proxy.OpenListHost + path + "?" + r.URL.RawQuery
		serveDirectStreamWithFix(w, fileURL, nil)
		return
	} else {
		fileInfo, err := openlist.OpenListGetFileURL(path, cfg.Proxy.OpenListHost, cfg.Proxy.Token)
		if err != nil {
			log.Printf("Error getting file URL for %s: %v", path, err)
			http.Error(w, "Failed to locate file", http.StatusInternalServerError)
			return
		}
		serveDirectStreamWithFix(w, fileInfo.Data.URL, fileInfo.Data.Header)
	}
}

// 通用地处理所有 ENCV 容器
// func (p *Proxy) serveEncryptedContainer(w http.ResponseWriter, r *http.Request, containerURL string, headers map[string]string, originalPath string) {
// 	p.cacheMutex.RLock()
// 	factory, exists := p.factoryCache[containerURL]
// 	p.cacheMutex.RUnlock()

// 	if !exists {
// 		p.cacheMutex.Lock()

// 		// 【关键】使用原始逻辑路径和配置来创建 URLResolver
// 		urlResolver := openlist.NewOpenListURLResolver(p.cfg, originalPath)
// 		if factory, exists = p.factoryCache[containerURL]; !exists {
// 			log.Printf("INFO: [Proxy] Cache miss for %s, creating new factory.", containerURL)
// 			// 【关键】将 resolver 注入到工厂构造函数中
// 			newFactory, err := reader.NewRemoteDecryptReaderFactory(containerURL, p.cfg.Password, headers, urlResolver)
// 			if err != nil {
// 				p.cacheMutex.Unlock()
// 				log.Printf("ERROR: [Proxy] Failed to create remote decrypt reader factory: %v", err)
// 				http.Error(w, "Could not initialize decryption: "+err.Error(), http.StatusInternalServerError)
// 				return
// 			}
// 			p.factoryCache[containerURL] = newFactory
// 			factory = newFactory
// 		}
// 		p.cacheMutex.Unlock()
// 	} else {
// 		log.Printf("INFO: [Proxy] Cache hit for %s.", containerURL)
// 	}

// 	// 1. 创建一个从头开始的 DecryptReader
// 	decryptReader, err := factory.NewDecryptReader(*p.cfg)
// 	if err != nil {
// 		log.Printf("ERROR: [Proxy] Failed to create decrypt reader: %v", err)
// 		http.Error(w, "Could not initialize decryption: "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	defer decryptReader.Close()

// 	// 2. 解析 HTTP Range 请求头
// 	totalSize := factory.GetOriginalSize()
// 	rangeHeader := r.Header.Get("Range")
// 	rangeStart, rangeEnd, contentLength := parseRangeHeader(rangeHeader, totalSize)

// 	// 【关键修复】尝试进行快速跳转
// 	if rangeStart > 0 {
// 		// 定义一个本地接口来检测 SeekTo 方法
// 		type seeker interface {
// 			SeekTo(offset int64) error
// 		}
// 		if s, ok := decryptReader.(seeker); ok {
// 			log.Printf("INFO: [Proxy] Fast-seeking to %d", rangeStart)
// 			// 【关键修复】必须检查 SeekTo 的返回值
// 			if err := s.SeekTo(rangeStart); err != nil {
// 				if err == io.EOF {
// 					// 如果 SeekTo 返回 EOF，说明请求的起始位置超出了所有数据片段的范围
// 					log.Printf("WARN: [Proxy] Client requested range starting at %d, which is beyond all data fragments.", rangeStart)
// 					w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", totalSize))
// 					http.Error(w, "Requested Range Not Satisfiable", http.StatusRequestedRangeNotSatisfiable)
// 					return
// 				}
// 				log.Printf("ERROR: [Proxy] Fast seek failed: %v", err)
// 				http.Error(w, "Failed to seek to requested position", http.StatusInternalServerError)
// 				return
// 			}
// 		} else {
// 			log.Printf("WARN: [Proxy] DecryptReader does not support fast seeking. This will be slow and may fail.")
// 			// 这里保留旧的逻辑作为最后的回退，尽管它有缺陷
// 			_, err := io.CopyN(io.Discard, decryptReader, rangeStart)
// 			if err != nil {
// 				log.Printf("ERROR: [Proxy] Failed to seek within decrypt stream: %v", err)
// 				http.Error(w, "Failed to seek to requested position", http.StatusRequestedRangeNotSatisfiable)
// 				return
// 			}
// 		}
// 	} else {
// 		log.Printf("INFO: [Proxy] No Range header, serving from beginning.")
// 	}

// 	// 3. 设置正确的 HTTP 响应头
// 	w.Header().Set("Content-Type", factory.GetIndex().GetMimeType())
// 	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", factory.GetIndex().GetOriginalFilename()))
// 	w.Header().Set("Accept-Ranges", "bytes")

// 	if rangeHeader != "" {
// 		// 如果有 Range 请求，返回 206 Partial Content
// 		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rangeStart, rangeEnd, totalSize))
// 		w.WriteHeader(http.StatusPartialContent)
// 	} else {
// 		// 否则返回 200 OK
// 		w.WriteHeader(http.StatusOK)
// 	}

// 	// 4. 流式传输剩余部分给客户端
// 	// 如果是 Range 请求，只传输请求的长度；否则传输全部
// 	if rangeHeader != "" {
// 		var written int64
// 		written, err = io.CopyN(w, decryptReader, contentLength)
// 		// 【关键修复】处理因空洞导致的提前 EOF
// 		if err == io.EOF && written == 0 {
// 			log.Printf("WARN: [Proxy] Range request %d-%d/%d, but no data available (EOF).", rangeStart, rangeEnd, totalSize)
// 			// 由于响应头已发送，无法返回 416，只能记录日志。
// 			// 播放器会因为收到 0 字节而关闭连接。
// 		}
// 	} else {
// 		_, err = io.Copy(w, decryptReader)
// 	}

// 	if err != nil {
// 		if !utils.IsConnectionClosedError(err) && err != io.EOF {
// 			log.Printf("Error streaming decrypted content to client: %v", err)
// 		}
// 		return
// 	}

// 	log.Printf("INFO: [Proxy] Successfully served remote container: %s", containerURL)
// }

// 可以修复 CORS，直接从 URL 下载文件并流式传输给客户端
func serveDirectStreamWithFix(w http.ResponseWriter, fileURL string, headers map[string]string) {
	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		log.Printf("Error creating request to download file: %v", err)
		http.Error(w, "Failed to download file", http.StatusInternalServerError)
		return
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error downloading file from %s: %v", fileURL, err)
		http.Error(w, "Failed to download file", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		w.Header()[key] = values
	}

	w.WriteHeader(resp.StatusCode)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error streaming file to client: %v", err)
	}
}

// parseRangeHeader 是一个辅助函数，用于解析 Range 请求头
func parseRangeHeader(rangeHeader string, totalSize int64) (start, end, length int64) {
	if rangeHeader == "" {
		return 0, totalSize - 1, totalSize
	}
	// 简单解析 "bytes=start-end"
	// 注意：end 可能是 '*'，表示到文件末尾
	if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end); err != nil {
		// 如果解析失败，比如 "bytes=500-"，则 end 默认为文件末尾
		if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &start); err == nil {
			end = totalSize - 1
		} else {
			// 完全无效，从头开始
			return 0, totalSize - 1, totalSize
		}
	}

	// 校验范围
	if start < 0 || end >= totalSize || start > end {
		return 0, totalSize - 1, totalSize
	}

	length = end - start + 1
	return start, end, length
}
