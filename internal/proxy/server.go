package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/middleware"
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/container/manifest"
	"github.com/Soltus/encv-go/internal/v2/openlist"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	"github.com/Soltus/encv-go/internal/v2/reader"
	"github.com/Soltus/encv-go/internal/web"
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
	mux.Handle("/_preview/", http.StripPrefix("/_preview/", web.PreviewHandler()))

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

// isEncvContainerFromBytes 从字节数组中判断是否为 ENCV 容器
func isEncvContainerFromBytes(data []byte) (bool, error) {
	if len(data) < 32 {
		return false, nil // 文件太小，不可能是 ENCV 容器
	}
	// 读取文件末尾的 32 字节
	footerData := data[len(data)-32:]
	// 尝试解析 Footer，如果能成功，就是 ENCV 容器
	_, err := manifest.ParseFooterFromBytes(footerData)
	return err == nil, nil
}

// handleRequest 创建并返回 HTTP 处理函数
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
	// --- 情况 1: 请求解密 ---
	if path == "/decrypt" {
		durl := r.URL.Query().Get("file")
		if durl == "" {
			http.Error(w, "Bad Request: 'file' query parameter is missing", http.StatusBadRequest)
			return
		}
		log.Printf("-> [Proxy] Received decrypt request for durl: %s", durl)

		u, err := url.Parse(durl)
		if err != nil {
			http.Error(w, "Bad Request: invalid durl format", http.StatusBadRequest)
			return
		}
		filePath := u.Path
		if after, ok := strings.CutPrefix(filePath, "/d/"); ok {
			filePath = after
		}
		log.Printf("-> [Proxy] Parsed logical file path from durl: %s", filePath)

		fileInfo, err := openlist.OpenListGetFileURL(filePath, p.cfg.Proxy.OpenListHost, p.cfg.Proxy.Token)
		if err != nil {
			log.Printf("Error getting stream URL for path %s: %v", filePath, err)
			http.Error(w, "Failed to locate file", http.StatusInternalServerError)
			return
		}
		streamURL := fileInfo.Data.URL
		log.Printf("-> [Proxy] Successfully translated durl to stream URL: %s", streamURL)

		// 【关键新增】在解密前，先验证 streamURL 指向的是否为有效的 ENCV 文件
		log.Printf("-> [Proxy] Validating stream URL before decryption...")
		// 只下载文件的最后 32 字节用于验证
		resp, err := utils.GetRemoteStreamWithRange(streamURL, nil, -32, -1)
		if err != nil {
			log.Printf("ERROR: [Proxy] Failed to validate stream URL %s: %v", streamURL, err)
			http.Error(w, "Upstream server is unreachable or invalid", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		footerBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("ERROR: [Proxy] Failed to read validation data from %s: %v", streamURL, err)
			http.Error(w, "Failed to validate upstream file", http.StatusInternalServerError)
			return
		}

		isValid, err := isEncvContainerFromBytes(footerBytes)
		if err != nil {
			log.Printf("ERROR: [Proxy] Validation check failed for %s: %v", streamURL, err)
			http.Error(w, "Failed to validate upstream file", http.StatusInternalServerError)
			return
		}

		if !isValid {
			log.Printf("WARN: [Proxy] Validation failed! Stream URL %s did not return an ENCV container. It might be an HTML error page.", streamURL)
			http.Error(w, "Upstream server returned an invalid file for decryption. The link might be expired or the file not found.", http.StatusBadGateway)
			return
		}

		log.Printf("-> [Proxy] Validation successful. Proceeding with decryption.")
		// 3. 验证通过，复用现有的、成功的解密服务逻辑
		p.serveEncryptedContainer(w, r, streamURL, nil, filePath)
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
	if plugins.IsContainer(path) {
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
