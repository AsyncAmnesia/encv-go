package openlist

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/container/manifest"
	"github.com/Soltus/encv-go/internal/v2/handler"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	"github.com/Soltus/encv-go/internal/v2/provider"
	"github.com/Soltus/encv-go/internal/v2/reader"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// ProxyGhttp GoFrame 版本的 Proxy
type ProxyGhttp struct {
	cfg            *config.Config
	factoryCache   map[string]reader.DecryptReaderFactory
	contentHandler *handler.ContentHandler
}

// NewProxyGhttp 创建 GoFrame 版本的 Proxy
func NewProxyGhttp(ctx context.Context) *ProxyGhttp {
	cfg := config.FromContext(ctx)
	contentHandler := handler.NewContentHandler()
	return &ProxyGhttp{
		cfg:            cfg,
		factoryCache:   make(map[string]reader.DecryptReaderFactory),
		contentHandler: contentHandler,
	}
}

// HandleRequest 处理请求（GoFrame 版本）
func (p *ProxyGhttp) HandleRequest(r *ghttp.Request) {
	// 从上下文获取站点配置（如果是多站点模式）
	siteHost := r.GetCtxVar("siteHost").String()
	siteToken := r.GetCtxVar("siteToken").String()
	siteId := r.GetCtxVar("siteId").String()

	originalPath := r.URL.Path
	re := regexp.MustCompile(`^/openlist/sites/` + regexp.QuoteMeta(siteId) + `/+`)
	path := re.ReplaceAllString(originalPath, "/")

	// 如果替换后是空字符串，说明是根目录
	if path == "" {
		path = "/"
	}

	// 调试日志
	g.Log().Infof(r.Context(), "[Proxy] %s -> %s", originalPath, path)

	sign := r.URL.Query().Get("sign")
	isInternalRequest := r.GetCtxVar("internal_request").String() == "true"

	if path == "" {
		r.Response.WriteStatus(http.StatusBadRequest)
		r.Response.Write("Missing 'path' parameter")
		return
	}

	// 处理解密请求
	if path == "/decrypt" {
		p.handleDecrypt(r, siteHost, siteToken)
		return
	}

	isDirectory := strings.HasSuffix(path, "/")

	// 签名验证（目录路径不需要签名）
	if !isInternalRequest && !p.cfg.Proxy.DisableSignatureVerification && !isDirectory {
		if sign == "" {
			r.Response.WriteStatus(http.StatusBadRequest)
			r.Response.Write("Missing 'sign' parameter")
			return
		}
		if !OpenListVerifySign(path, sign, siteToken) {
			g.Log().Errorf(r.Context(), "Invalid signature for path: %s", path)
			r.Response.WriteStatus(http.StatusForbidden)
			r.Response.Write("Forbidden: Invalid signature")
			return
		}
	} else if isDirectory {
		g.Log().Infof(r.Context(), "[Proxy] Directory request, skipping signature verification: %s", path)
	} else if p.cfg.Proxy.DisableSignatureVerification {
		g.Log().Infof(r.Context(), "[Security] Signature verification is disabled, allowing request for: %s", path)
	} else {
		g.Log().Infof(r.Context(), "[Proxy] Handling internal request, skipping signature check for: %s", path)
	}

	g.Log().Infof(r.Context(), "Received valid request for: %s", path)

	// 【新增】处理目录请求 - 返回目录列表或重定向到索引文件
	if isDirectory {
		p.handleDirectoryRequest(r, path, siteHost, siteToken)
		return
	}

	// 判断是否是 ENCV 容器文件
	// openlist 依赖扩展名预览，因此直接使用扩展名判断的函数，不必检测 magic header
	if plugins.IsContainer(path) {
		g.Log().Infof(r.Context(), "[Proxy] Detected ENCV container file: %s", path)
		p.serveEncryptedContainer(r, path, siteHost, siteToken)
		return
	}

	// 处理普通文件
	g.Log().Infof(r.Context(), "[Proxy] Not a container file, handling as standard file: %s", path)
	p.serveStandardFile(r, path, siteHost, siteToken)
}

// handleDecrypt 处理解密请求
func (p *ProxyGhttp) handleDecrypt(r *ghttp.Request, siteHost, siteToken string) {
	durl := r.URL.Query().Get("file")
	if durl == "" {
		r.Response.WriteStatus(http.StatusBadRequest)
		r.Response.Write("Bad Request: 'file' query parameter is missing")
		return
	}
	g.Log().Infof(r.Context(), "[Proxy] Received decrypt request for durl: %s", durl)

	// 解析 URL 获取文件路径
	u, err := url.Parse(durl)
	if err != nil {
		r.Response.WriteStatus(http.StatusBadRequest)
		r.Response.Write("Bad Request: invalid durl format")
		return
	}

	filePath := u.Path
	if after, ok := strings.CutPrefix(filePath, "/d/"); ok {
		filePath = after
	}
	g.Log().Infof(r.Context(), "[Proxy] Parsed logical file path from durl: %s", filePath)

	// 获取文件信息
	fileInfo, err := OpenListGetFileURL(filePath, siteHost, siteToken)
	if err != nil {
		g.Log().Errorf(r.Context(), "Error getting stream URL for path %s: %v", filePath, err)
		r.Response.WriteStatus(http.StatusInternalServerError)
		r.Response.Write("Failed to locate file")
		return
	}
	streamURL := fileInfo.Data.URL
	g.Log().Infof(r.Context(), "[Proxy] Successfully translated durl to stream URL: %s", streamURL)

	// 验证文件是否为有效的 ENCV 容器
	g.Log().Infof(r.Context(), "[Proxy] Validating stream URL before decryption...")
	resp, err := utils.GetRemoteStreamWithRange(streamURL, nil, -32, -1)
	if err != nil {
		g.Log().Errorf(r.Context(), "ERROR: [Proxy] Failed to validate stream URL %s: %v", streamURL, err)
		r.Response.WriteStatus(http.StatusBadGateway)
		r.Response.Write("Upstream server is unreachable or invalid")
		return
	}
	defer resp.Body.Close()

	footerBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		g.Log().Errorf(r.Context(), "ERROR: [Proxy] Failed to read validation data from %s: %v", streamURL, err)
		r.Response.WriteStatus(http.StatusInternalServerError)
		r.Response.Write("Failed to validate upstream file")
		return
	}

	isValid, err := isEncvContainerFromBytes(footerBytes)
	if err != nil {
		g.Log().Errorf(r.Context(), "ERROR: [Proxy] Validation check failed for %s: %v", streamURL, err)
		r.Response.WriteStatus(http.StatusInternalServerError)
		r.Response.Write("Failed to validate upstream file")
		return
	}

	if !isValid {
		g.Log().Warningf(r.Context(), "WARN: [Proxy] Validation failed! Stream URL %s did not return an ENCV container.", streamURL)
		r.Response.WriteStatus(http.StatusBadGateway)
		r.Response.Write("Upstream server returned an invalid file for decryption.")
		return
	}

	g.Log().Infof(r.Context(), "[Proxy] Validation successful. Proceeding with decryption.")
	p.serveEncryptedContainerWithURL(r, streamURL, fileInfo.Data.Header, siteHost, siteToken, filePath)
}

// 处理目录请求
func (p *ProxyGhttp) handleDirectoryRequest(r *ghttp.Request, path, siteHost, siteToken string) {
	if siteHost == "" || siteToken == "" {
		g.Log().Errorf(r.Context(), "Site host or token is missing for path %s", path)
	}
	indexFiles := []string{"index.html", "README.md"}

	for _, indexFile := range indexFiles {
		indexPath := path + indexFile
		if plugins.IsContainer(indexPath) {
			g.Log().Infof(r.Context(), "[Proxy] Found index container: %s", indexPath)
			p.serveEncryptedContainer(r, indexPath, siteHost, siteToken)
			return
		}

		// 检查普通文件

		fileInfo, err := OpenListGetFileURL(indexPath, siteHost, siteToken)
		if err == nil && fileInfo.Data.URL != "" {
			g.Log().Infof(r.Context(), "[Proxy] Found index file: %s", indexPath)
			p.serveDirectStream(r, fileInfo.Data.URL, fileInfo.Data.Header)
			return
		}
	}

	// 如果没有找到索引文件，返回简单的目录列表
	g.Log().Infof(r.Context(), "[Proxy] No index file found for directory: %s", path)
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Directory listing for %s</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
            padding: 20px; 
            background-color: var(--hope-ui-background, #ffffff);
            color: var(--hope-ui-text, #1a1a1a);
            transition: background-color 0.3s, color 0.3s;
        }
        body.dark {
            background-color: var(--hope-ui-background, #1a1a1a);
            color: var(--hope-ui-text, #e0e0e0);
        }
        h1 { color: var(--hope-ui-primary, #0066cc); margin-bottom: 20px; }
        .file-list { list-style: none; padding: 0; }
        .file-item { 
            margin: 8px 0; 
            padding: 12px; 
            background: var(--hope-ui-surface, #f5f5f5); 
            border-radius: 6px;
            border: 1px solid var(--hope-ui-border, #e0e0e0);
        }
        .file-link { 
            color: var(--hope-ui-primary, #0066cc); 
            text-decoration: none; 
            font-weight: 500;
        }
        .file-link:hover { text-decoration: underline; }
        .toggle-theme {
            position: fixed;
            top: 20px;
            right: 20px;
            background: var(--hope-ui-surface, #f5f5f5);
            border: 1px solid var(--hope-ui-border, #e0e0e0);
            padding: 8px 12px;
            border-radius: 6px;
            cursor: pointer;
            font-size: 14px;
        }
    </style>
</head>
<body>
    <button class="toggle-theme" onclick="document.body.classList.toggle('dark')">🌙/☀️</button>
    <h1>Directory listing for %s</h1>
    <p>No index file found. This directory may contain encrypted files that require direct access.</p>
    <ul class="file-list">
        <li class="file-item"><a href="../" class="file-link">.. (Parent Directory)</a></li>
    </ul>
    <script>
        // 检测系统主题偏好
        if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
            document.body.classList.add('dark');
        }
    </script>
</body>
</html>`, path, path)

	r.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	r.Response.Write(html)
}

// serveEncryptedContainer 服务加密容器
func (p *ProxyGhttp) serveEncryptedContainer(r *ghttp.Request, path string, siteHost, siteToken string) {
	if siteHost == "" || siteToken == "" {
		g.Log().Errorf(r.Context(), "Site host or token is missing for path %s", path)
	}

	fileInfo, err := OpenListGetFileURL(path, siteHost, siteToken)
	if err != nil {
		g.Log().Errorf(r.Context(), "Error getting file URL for %s: %v", path, err)
		r.Response.WriteStatus(http.StatusInternalServerError)
		r.Response.Write("Failed to locate file")
		return
	}
	p.serveEncryptedContainerWithURL(r, fileInfo.Data.URL, fileInfo.Data.Header, siteHost, siteToken, path)
}

// serveEncryptedContainerWithURL 使用 URL 服务加密容器
func (p *ProxyGhttp) serveEncryptedContainerWithURL(r *ghttp.Request, containerURL string, headers map[string][]string, siteHost, siteToken, originalPath string) {
	// 创建 URLResolver
	urlResolver := NewOpenListURLResolver(siteHost, siteToken, originalPath)

	// 创建远程工厂
	factory, err := reader.NewRemoteDecryptReaderFactory(containerURL, p.cfg.Password, headers, urlResolver)
	if err != nil {
		r.Response.WriteStatus(http.StatusInternalServerError)
		r.Response.Write(fmt.Sprintf("failed to create remote decrypt reader factory: %v", err))
		return
	}

	// 创建解密器
	decryptReader, err := factory.NewDecryptReader(*p.cfg)
	if err != nil {
		factory.Close()
		r.Response.WriteStatus(http.StatusInternalServerError)
		r.Response.Write(fmt.Sprintf("failed to create remote decrypt reader: %v", err))
		return
	}

	// 创建 provider
	prov, err := provider.NewRemoteFileProvider(factory, decryptReader)
	if err != nil {
		// 如果 provider 创建失败，factory 和 reader 的生命周期由 proxy 管理
		decryptReader.Close()
		factory.Close()
		r.Response.WriteStatus(http.StatusInternalServerError)
		r.Response.Write(err.Error())
		return
	}
	defer prov.Close()

	// 使用内容处理器服务文件
	p.contentHandler.ServeFile(r.Response.Writer, r.Request, prov)
}

// serveStandardFile 服务普通文件
func (p *ProxyGhttp) serveStandardFile(r *ghttp.Request, path string, siteHost, siteToken string) {
	if siteHost == "" || siteToken == "" {
		g.Log().Errorf(r.Context(), "Site host or token is missing for path %s", path)
	}

	var fileURL string
	var headers map[string][]string

	if strings.HasPrefix(path, "/p/") {
		g.Log().Infof(r.Context(), "[Proxy] Intercepted internal link: %s", path)
		fileURL = siteHost + path + "?" + r.URL.RawQuery
	} else {
		fileInfo, err := OpenListGetFileURL(path, siteHost, siteToken)
		if err != nil {
			g.Log().Errorf(r.Context(), "Error getting file URL for %s: %v", path, err)
			r.Response.WriteStatus(http.StatusInternalServerError)
			r.Response.Write("Failed to locate file")
			return
		}
		fileURL = fileInfo.Data.URL
		headers = fileInfo.Data.Header
	}

	p.serveDirectStream(r, fileURL, headers)
}

// serveDirectStream 直接流式传输文件
func (p *ProxyGhttp) serveDirectStream(r *ghttp.Request, fileURL string, headers map[string][]string) {
	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		g.Log().Errorf(r.Context(), "Error creating request to download file: %v", err)
		r.Response.WriteStatus(http.StatusInternalServerError)
		r.Response.Write("Failed to download file")
		return
	}

	for key, values := range headers {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		g.Log().Errorf(r.Context(), "Error downloading file from %s: %v", fileURL, err)
		r.Response.WriteStatus(http.StatusInternalServerError)
		r.Response.Write("Failed to download file")
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for key, values := range resp.Header {
		r.Response.Header()[key] = values
	}

	// 确保 CORS 头存在
	r.Response.Header().Set("Access-Control-Allow-Origin", "*")
	r.Response.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
	r.Response.Header().Set("Access-Control-Allow-Headers", "*")
	r.Response.Header().Set("Access-Control-Allow-Credentials", "true")

	r.Response.WriteStatus(resp.StatusCode)

	_, err = io.Copy(r.Response.Writer, resp.Body)
	if err != nil {
		g.Log().Errorf(r.Context(), "Error streaming file to client: %v", err)
	}
}

// isEncvContainerFromBytes 从字节数组判断是否为 ENCV 容器
func isEncvContainerFromBytes(data []byte) (bool, error) {
	if len(data) < 32 {
		return false, nil
	}
	footerData := data[len(data)-32:]
	_, err := manifest.ParseFooterFromBytes(footerData)
	return err == nil, nil
}
