package server

import (
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// DevPreviewProxy 沙箱预览编排：encv-go 作为 16000 preview proxy 的唯一 front door，
// 按 path 前缀把请求反向代理到三个上游。
//
// 架构：
//
//	Browser → 16000 (agent-tool-host) → 2025 (encv-go, 本文件编排) → ┬─ /openlist-ui/*  → :3000 (openlist vite)
//	                                                                  ├─ /api/*          → :5244 (OpenList backend 真实 API)
//	                                                                  └─ /*              → :5173 (encv-mobile vite)
//
// 两个 vite 完全独立、各自 HMR，互不污染。本地直接访问 5173 / 3000 也能 dev。
// 沙箱走 16000 时 HMR 仍不通（16000 不支持 WebSocket 升级），但页面渲染不受影响。
//
// **绝对不做的事（铁律）**：
//   - **不 mock 任何 /api/**（如 /me、/fs/*、/admin/*、/p/*、/public/* 等所有私有 + 公共 API）。
//     全部 reverse proxy 到 :5244 OpenList backend，由 backend 真实服务。
//     dev preview 下让用户用真 admin/admin 登录、看到真文件列表、跑通完整流程。
//   - 不在 dev_preview_proxy 接管任何 /api/* mock 端点。
//
// 仅在 ENCV_DEV_PREVIEW=1 时启用，避免污染生产路径（生产 APK 内 gomobile 自己服务）。
type DevPreviewProxy struct {
	openlistViteURL    *url.URL
	encvMobileViteURL  *url.URL
	openlistBackendURL *url.URL
}

// NewDevPreviewProxy 构造 dev preview proxy。从环境变量读上游地址，缺省 :3000 / :5173 / :5244。
func NewDevPreviewProxy() (*DevPreviewProxy, error) {
	openlistURL := os.Getenv("ENCV_DEV_OPENLIST_VITE_URL")
	if openlistURL == "" {
		openlistURL = "http://127.0.0.1:3000"
	}
	mobileURL := os.Getenv("ENCV_DEV_MOBILE_VITE_URL")
	if mobileURL == "" {
		mobileURL = "http://127.0.0.1:5173"
	}
	backendURL := os.Getenv("ENCV_DEV_OPENLIST_BACKEND_URL")
	if backendURL == "" {
		backendURL = "http://127.0.0.1:5244"
	}

	openlistParsed, err := url.Parse(openlistURL)
	if err != nil {
		return nil, err
	}
	mobileParsed, err := url.Parse(mobileURL)
	if err != nil {
		return nil, err
	}
	backendParsed, err := url.Parse(backendURL)
	if err != nil {
		return nil, err
	}

	slog.Info("[dev-preview-proxy] enabled",
		"openlistVite", openlistURL,
		"encvMobileVite", mobileURL,
		"openlistBackend", backendURL,
		"trigger", "ENCV_DEV_PREVIEW=1",
	)

	return &DevPreviewProxy{
		openlistViteURL:    openlistParsed,
		encvMobileViteURL:  mobileParsed,
		openlistBackendURL: backendParsed,
	}, nil
}

// RegisterExplicit 注册显式路由：必须先于 encv-go 原 NoRoute 调用。
//   - /openlist-ui/* → :3000 openlist vite
//
// 不在 Go 层 mock 任何 /api/*。所有 /api/*（含 /api/public/*）由 NoRoute 反代到 :5244 真 backend。
func (p *DevPreviewProxy) RegisterExplicit(r *gin.Engine) {
	// 显式起 /openlist-ui/* 与 /openlist-ui 两条
	openlistGroup := r.Group("/openlist-ui")
	openlistGroup.Any("/*subpath", func(c *gin.Context) {
		p.proxyTo(c, p.openlistViteURL)
	})
	openlistGroup.Any("", func(c *gin.Context) {
		p.proxyTo(c, p.openlistViteURL)
	})
}

// RegisterNoRoute 注册 NoRoute 兜底反代：必须后于 encv-go 原 NoRoute 调用以覆盖之。
// 把非 encv-go 自身已注册路径的请求分发到三个上游：
//   - /api/*  → :5244 OpenList backend（reverse proxy，不是 mock）
//   - 其他   → :5173 encv-mobile vite
//
// 说明：
//   - /openlist-ui/* 已被显式路由注册（RegisterExplicit），不会进 NoRoute。
//   - /api/config、/api/files/*、/api/tasks/* 等 encv-go 自己的 API 也已被注册，NoRoute 接不到。
//   - NoRoute 这里只会接到：/api/me、/api/fs/*、/api/admin/*、/api/auth/*、/api/public/* 等 OpenList API，
//     以及 encv-mobile SPA 自己的路径（/、/tabs/*、/assets/*、/play）。
//   - 关键：这是 **reverse proxy**（routing），不是 mock——后端真服务 :5244。
func (p *DevPreviewProxy) RegisterNoRoute(r *gin.Engine) {
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// /api/* 转发到 OpenList backend（：5244）
		// 这不是 mock，是真实 reverse proxy，让所有 /api/*（含 /api/public/*）由 backend 服务
		if strings.HasPrefix(path, "/api/") || path == "/api" {
			p.proxyTo(c, p.openlistBackendURL)
			return
		}

		// 其他全部 fallback 到 encv-mobile vite（：5173）
		p.proxyTo(c, p.encvMobileViteURL)
	})
}

// proxyTo 把当前请求反向代理到指定上游 URL。
// 支持 WebSocket 升级（HMR 走这条路时也能用，本地直连 :5173 / :3000 即可）。
func (p *DevPreviewProxy) proxyTo(c *gin.Context, target *url.URL) {
	rp := httputil.NewSingleHostReverseProxy(target)

	// Director：保留 path 但替换 host/scheme
	origDirector := rp.Director
	rp.Director = func(req *http.Request) {
		origDirector(req)
		// 强制 Host header 跟上游一致（vite 在 Host 检查时严格）
		req.Host = target.Host
		req.Header.Set("X-Forwarded-Host", c.Request.Host)
		req.Header.Set("X-Forwarded-Proto", schemeOf(c.Request))
	}

	// WebSocket 升级支持（HMR 通过 gin 也能透传）
	rp.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		// 关键：允许 Upgrade 协议不被破坏
		DisableCompression: true,
	}

	// 注入 client 真实 IP
	c.Request.Header.Set("X-Real-IP", c.ClientIP())

	rp.ServeHTTP(c.Writer, c.Request)
}

func schemeOf(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	if strings.HasPrefix(r.Host, "localhost") {
		return "http"
	}
	return "http"
}
