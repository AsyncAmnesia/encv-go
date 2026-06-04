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
// 按 path 前缀把请求反向代理到两个独立的 vite dev server，并对 OpenList API 做 mock。
//
// 架构（2026-06-04 踩坑后定型）：
//
//	Browser → 16000 (agent-tool-host) → 2025 (encv-go, 本文件编排) → ┬─ /openlist-ui/*  → :3000 (openlist vite，独立)
//	                                                                  ├─ /api/public/*   → 本文件 mock
//	                                                                  ├─ /api/* (其它)    → 本文件 mock
//	                                                                  └─ /*              → :5173 (encv-mobile vite，独立)
//
// 两个 vite 完全独立、各自 HMR，互不污染。本地直接访问 5173 / 3000 也能 dev。
// 沙箱走 16000 时 HMR 仍不通（16000 不支持 WebSocket 升级），但页面渲染不受影响。
//
// 关键铁律（踩坑 §九）：
//   - **所有 /api/* 必须在本文件显式截获**，绝不能落到 NoRoute。
//     NoRoute → encv-mobile vite (5173) → 它的 /api proxy → encv-go NoRoute → ...
//     无限代理循环，最终 vite proxy 放弃返回 502 给浏览器。
//   - **/openlist-ui/api/* 必须被 openlist vite proxy 改写为 /api/***（去掉前缀），
//     否则 dev_preview_proxy 的 /openlist-ui/* 路由会把请求回环到本 vite (无限代理循环)。
//
// 仅在 ENCV_DEV_PREVIEW=1 时启用，避免污染生产路径（生产 APK 内 gomobile 自己服务）。
type DevPreviewProxy struct {
	openlistViteURL   *url.URL
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
//   - /api/*         → handleAnyOpenlistAPI（**单一 wildcard**，按 subpath 内部分发 mock）
//   - /openlist-ui/* → :3000 openlist vite
//
// **铁律**：不能用两个 group（`/api/public/*` 和 `/api/*`），
// gin radix tree 会在 startup panic：
//   `catch-all wildcard '*subpath' in new path '/api/*subpath' conflicts with
//    existing path segment 'p' in existing prefix 'api/p'`
// 修复：把 /api/public 和 /me、/fs/* 等统一进一个 handler 内部 if 分支。
func (p *DevPreviewProxy) RegisterExplicit(r *gin.Engine) {
	// OpenList 全部 API mock（公共 + 私有）— 单一 wildcard 入口。
	api := r.Group("/api")
	api.Any("/*subpath", p.handleAnyOpenlistAPI)
	api.Any("", p.handleAnyOpenlistAPI)

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
		// /api/*、/openlist-ui/* 已被显式路由注册，不会进 NoRoute。
		// NoRoute 这里只会接到 encv-mobile SPA 自己的路径。
		p.proxyTo(c, p.encvMobileViteURL)
	})
}

// handleAnyOpenlistAPI OpenList 全部 API（公共 + 私有）单一入口。
//
// gin radix tree 不允许 `/api/public/*` 和 `/api/*` 同时存在（启动 panic），
// 也不允许 `/api/preview/openlist-ui` 与 `/api/*` 同时存在（同样 panic），
// 所以这里把 /api/preview/openlist-ui、/api/public/*、/me、/fs/*、/admin/* 等
// 所有 OpenList 端点的 mock 都集中在这一个 handler 内，按 subpath 分发。
//
// 沙箱预览下没有真实 OpenList backend（:5244）：
//   - /api/preview/openlist-ui (GET) → 302 redirect to /openlist-ui/
//   - /api/public/settings           → 完整 settings JSON（含 search_enabled、site_title 等）
//   - /api/public/archive_extensions → []string{}
//   - /api/me                        → admin user（让 MustUser / UserOrGuest 通过）
//   - /api/admin/login (POST)        → 401
//   - /api/fs/list (GET)             → 空 content 列表
//   - /api/fs/get (GET)              → 空对象
//   - /api/fs/put, remove, mkdir, rename, move, copy → 200 + 空（沙箱禁用写）
//   - 其它 /api/*                    → 200 + data: nil（**避免 NoRoute 循环代理**）
func (p *DevPreviewProxy) handleAnyOpenlistAPI(c *gin.Context) {
	slog.Info("[dev-preview-proxy] /api/* hit", "path", c.Request.URL.Path, "method", c.Request.Method, "remote", c.ClientIP())
	c.Header("Content-Type", "application/json; charset=utf-8")

	sub := strings.TrimPrefix(c.Param("subpath"), "/")
	method := c.Request.Method

	// ========== /api/preview/openlist-ui → 302 redirect ==========
	// 见 RegisterExplicit 注册顺序铁律：dev mode 下这条路由由本 catch-all 接管，
	// 生产模式下走 openlistUI.handlePreviewRedirect。
	if sub == "preview/openlist-ui" && method == http.MethodGet {
		c.Redirect(http.StatusFound, "/openlist-ui/")
		return
	}

	// ========== /api/public/* 公共 API ==========
	if sub == "public/settings" || sub == "public/settings/" {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data": gin.H{
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
			},
		})
		return
	}
	if sub == "public/archive_extensions" || sub == "public/archive_extensions/" {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data":    []string{},
		})
		return
	}
	// 其它 /api/public/* 端点
	if strings.HasPrefix(sub, "public/") {
		slog.Debug("[dev-preview-proxy] mock /api/public/* unimplemented", "sub", sub)
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data":    nil,
		})
		return
	}

	// ========== /api/me 当前用户 ==========
	if sub == "me" && method == http.MethodGet {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data": gin.H{
				"id":         1,
				"username":   "admin",
				"password":   "",
				"base_path":  "/",
				"role":       0, // 0 = admin
				"disabled":   false,
				"permission": 255,
				"sso_id":     "",
				"otp":        false,
			},
		})
		return
	}

	// ========== /api/admin/login (POST) ==========
	if strings.HasPrefix(sub, "admin/login") && method == http.MethodPost {
		c.JSON(http.StatusOK, gin.H{
			"code":    401,
			"message": "sandbox mode: login disabled",
			"data":    nil,
		})
		return
	}

	// ========== /api/fs/* 文件操作 ==========
	if strings.HasPrefix(sub, "fs/list") && method == http.MethodGet {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data": gin.H{
				"content":  []any{},
				"total":    0,
				"readme":   "",
				"header":   "",
				"write":    false,
				"provider": "sandbox",
			},
		})
		return
	}
	if strings.HasPrefix(sub, "fs/get") && method == http.MethodGet {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "success",
			"data":    gin.H{},
		})
		return
	}
	if strings.HasPrefix(sub, "fs/put") {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "sandbox: upload disabled",
			"data":    gin.H{"task": gin.H{}},
		})
		return
	}
	if strings.HasPrefix(sub, "fs/remove") ||
		strings.HasPrefix(sub, "fs/mkdir") ||
		strings.HasPrefix(sub, "fs/rename") ||
		strings.HasPrefix(sub, "fs/move") ||
		strings.HasPrefix(sub, "fs/copy") {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "sandbox: write disabled",
			"data":    nil,
		})
		return
	}

	// ========== 其它 /api/* 端点 ==========
	slog.Debug("[dev-preview-proxy] mock /api/* unimplemented", "sub", sub, "method", method)
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    nil,
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
