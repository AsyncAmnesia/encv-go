package server

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/webdav"
	goWebdav "golang.org/x/net/webdav"
)

// StartWebdav 启动一个专用的 WebDAV 服务器。
// 它会返回服务器的监听地址，以便主程序打印信息。
func StartWebdav(cfg *types.UserConfig) (string, string, error) {
	// 1. 检查 WebDAV 配置是否有效
	if cfg.Webdav.Dir == "" {
		return "", "", fmt.Errorf("webdav 'dir' (serving directory) is not configured")
	}
	if cfg.Webdav.Port == 0 {
		return "", "", fmt.Errorf("webdav 'port' is not configured")
	}

	mux := http.NewServeMux()

	// 2. 确定路由前缀
	webdavPath := cfg.Webdav.Root
	if webdavPath == "" {
		webdavPath = "/webdav/" // 如果未配置，则使用默认值
	}
	// 确保 WebdavPath 以 / 开头和结尾
	if !strings.HasPrefix(webdavPath, "/") {
		webdavPath = "/" + webdavPath
	}
	if !strings.HasSuffix(webdavPath, "/") {
		webdavPath += "/"
	}

	// 3. 创建并挂载 WebDAV 处理器
	log.Printf("-> Enabling WebDAV server")
	log.Printf("   Serving directory: %s", cfg.Webdav.Dir)
	log.Printf("   WebDAV path: %s", webdavPath)

	// 【关键修改】将 webdavPath 传递给 FileSystem
	webdavFS := webdav.NewENCVFS(cfg.Webdav.Dir, cfg.Password, webdavPath)
	webdavHandler := &goWebdav.Handler{
		FileSystem: webdavFS,
		LockSystem: goWebdav.NewMemLS(),
	}

	// 使用 http.StripPrefix 包装器来移除 URL 前缀，以便 WebDAV handler 能正确处理相对路径
	// handlerToRegister := http.StripPrefix(strings.TrimSuffix(webdavPath, "/"), webdavHandler)
	// mux.Handle(webdavPath, handlerToRegister)
	// 【关键修改】直接挂载，不使用 StripPrefix
	// mux.Handle(webdavPath, webdavHandler)

	// 【关键】用一个详细的日志包装器来捕获所有请求
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("========================================")
		log.Printf(">>> WEBDAV REQUEST DETECTED <<<")
		log.Printf("Method: %s", r.Method)
		log.Printf("URL: %s", r.URL.String())
		log.Printf("Proto: %s", r.Proto)
		log.Printf("Host: %s", r.Host)
		log.Printf("RemoteAddr: %s", r.RemoteAddr)
		log.Printf("--- Request Headers ---")
		for name, headers := range r.Header {
			for _, h := range headers {
				log.Printf("   %v: %v", name, h)
			}
		}

		// 捕获请求体
		if r.Body != nil {
			// 读取请求体内容
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("Error reading request body: %v", err)
			} else {
				log.Printf("--- Request Body ---")
				log.Printf("%s", string(bodyBytes))
				// 【关键修正】使用 io.NopCloser 恢复 r.Body
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}
		log.Printf("========================================")

		// 调用原始处理器
		webdavHandler.ServeHTTP(w, r)
	})

	mux.Handle(webdavPath, wrappedHandler)

	// 4. 创建并启动 HTTP 服务器
	addr := fmt.Sprintf(":%d", cfg.Webdav.Port)
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	go func() {
		log.Printf("-> WebDAV server is now listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("WebDAV server failed: %v", err)
		}
	}()

	return addr, webdavPath, nil
}
