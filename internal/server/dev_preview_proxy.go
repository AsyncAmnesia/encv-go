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
// 按 path 前缀把请求反向代理到两个独立的 vite dev server。
//
// 架构（2026-06-04 踩坑后定型）：
//
//	Browser → 16000 (agent-tool-host) → 2025 (encv-go, 本文件编排) → ┬─ /openlist-ui/*  → :3000 (openlist vite，独立)
//	                                                                  ├─ /api/public/*   → 本文件 mock
//	                                                                  └─ /*              → :5173 (encv-mobile vite，独立)
//
// 两个 vite 完全独立、各自 HMR，互不污染。本地直接访问 5173 / 3000 也能 dev。
// 沙箱走 16000 时 HMR 仍不通（16000 不支持 WebSocket 升级），但页面渲染不受影响。
//
// 仅在 ENCV_DEV_PREVIEW=1 时启用，避免污染生产路径（生产 APK 内 gomobile 自己服务）。
type DevPreviewProxy struct {
	openlistViteURL  *url.URL
	encvMobileViteURL *url.URL
}

// NewDevPreviewProxy 构造 dev preview proxy。从环境变量读上游地址，缺省 :3000 / :5173。
func NewDevPreviewProxy() (*DevPreviewProxy, error) {
	openlistURL := os.Getenv("ENCV_DEV_OPENLIST_VITE_URL")
	if openlistURL == "" {
		openlistURL = "http://127.0.0.1:3000"
	}
	mobileURL := os.Getenv("ENCV_DEV_MOBILE_VITE_URL")
	if mobileURL == "" {
		mobileURL = "http://127.0.0.1:5173"
	}

	openlistParsed, err := url.Parse(openlistURL)
	if err != nil {
		return nil, err
	}
	mobileParsed, err := url.Parse(mobileURL)
	if err != nil {
		return nil, err
	}

	slog.Info("[dev-preview-proxy] enabled",
		"openlistVite", openlistURL,
		"encvMobileVite", mobileURL,
		"trigger", "ENCV_DEV_PREVIEW=1",
	)

	return &DevPreviewProxy{
		openlistViteURL:   openlistParsed,
		encvMobileViteURL: mobileParsed,
	}, nil
}

// RegisterExplicit 注册显式路由：必须先于 encv-go 原 NoRoute 调用。
//   - /api/public/*  → 本地 mock（OpenList 公共 API）
//   - /openlist-ui/* → :3000 openlist vite
func (p *DevPreviewProxy) RegisterExplicit(r *gin.Engine) {
	// OpenList 公共 API mock — 必须用 Any 覆盖所有方法
	openlistPublic := r.Group("/api/public")
	openlistPublic.Any("/*subpath", p.handleOpenlistPublicAPI)
	openlistPublic.Any("", p.handleOpenlistPublicAPI)

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
// 把 /、/tabs/*、/assets/*、/play 等 encv-mobile SPA 路径反代到 :5173。
func (p *DevPreviewProxy) RegisterNoRoute(r *gin.Engine) {
	r.NoRoute(func(c *gin.Context) {
		// /api/*、/admin/*、/webdav/*、/openlist/* 已被显式路由注册，
		// 不会进 NoRoute。NoRoute 这里只会接到 encv-mobile SPA 自己的路径。
		p.proxyTo(c, p.encvMobileViteURL)
	})
}

// handleOpenlistPublicAPI OpenList 公共 API mock
// 让 OpenList UI（Solid app）能 boot，不依赖 OpenList backend 二进制。
// 真实后端启动后改成代理 :5244 即可。
func (p *DevPreviewProxy) handleOpenlistPublicAPI(c *gin.Context) {
	slog.Info("[dev-preview-proxy] /api/public/* hit", "path", c.Request.URL.Path, "remote", c.ClientIP())
	c.Header("Content-Type", "application/json; charset=utf-8")

	// 拼接 subpath（gin's /*subpath 包含前导 /，先 trim）
	sub := strings.TrimPrefix(c.Param("subpath"), "/")
	full := "/api/public/" + sub

	var data any
	switch {
	case sub == "settings" || sub == "settings/":
		data = gin.H{
			"version":        "dev",
			"site_title":     "OpenList",
			"favicon":        "https://res.oplist.org/logo/logo.svg",
			"main_color":     "",
			"icon_color":     "",
			"default_page":   "home",
			"school_titles":  []string{},
			"home_readme":    "",
			"home_icon":      "",
			"home_pinned":    []string{},
			"search_enabled": true,
		}
	case sub == "archive_extensions" || sub == "archive_extensions/":
		data = []string{}
	default:
		// 其它 /api/public/* 端点（Solid app 后续可能加）→ 返回空 data 200，让 UI 继续
		data = nil
		slog.Debug("[dev-preview-proxy] mock /api/public/* unimplemented subpath", "path", full)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    data,
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
