package server

// 暂时注释，需要修改，勿删

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/middleware"
	"github.com/Soltus/encv-go/internal/webdav"
	goWebdav "golang.org/x/net/webdav"
)

// StartWebdav 启动一个专用的 WebDAV 服务器。
// 它会返回服务器的监听地址，以便主程序打印信息。
func StartWebdav(ctx context.Context) (string, string, error) {
	cfg := config.FromContext(ctx)
	// 1. 检查 WebDAV 配置是否有效
	if cfg.Webdav.Dir == "" {
		return "", "", fmt.Errorf("webdav 'dir' (serving directory) is not configured")
	}
	if cfg.Webdav.Port == 0 {
		return "", "", fmt.Errorf("webdav 'port' is not configured")
	}

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

	// 创建 WebDAV 文件系统实例和核心处理器
	fs := webdav.NewENCVFS(ctx)
	webdavCoreHandler := &goWebdav.Handler{
		FileSystem: fs,
		LockSystem: goWebdav.NewMemLS(),
	}

	// 【关键修改】使用我们的中间件来包装 webdavCoreHandler
	// 我们从启动时的 ctx 中提取 cfg，然后交给中间件处理
	configAwareWebdavHandler := middleware.WithConfig(cfg, webdavCoreHandler)

	mux := http.NewServeMux()
	// 【修改】注册被中间件包装过的 handler
	mux.Handle(webdavPath, configAwareWebdavHandler)

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
