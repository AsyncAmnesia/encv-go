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
// 按 path 前缀把请求反向代理到四个上游。
//
// 架构：
//
//	Browser → 16000 (agent-tool-host) → 2025 (encv-go, 本文件编排) → ┬─ /openlist-ui/*   → :5174 (plugin-openlist/web vite, Capacitor UI)
//	                                                                  ├─ /openlist-spa/*  → :3000 (Hi-Sillot-OpenList-Frontend vite dev, OpenList web UI)
//	                                                                  ├─ /api/*           → :5244 (OpenList backend 真实 API)
//	                                                                  └─ /*               → :5173 (encv-mobile vite)
//
// 关键澄清（避免再次混淆）：
//   - **plugin-openlist/web**（:5174, Vue + Ionic + Capacitor, ENCV 自己封装的 Capacitor UI）
//     和 **Hi-Sillot-OpenList-Frontend**（:3000, SolidJS, OpenListTeam 官方 web UI）
//     是**两个完全独立**的前端项目！路径前缀也不同：plugin-openlist 用 `/openlist-ui/`，Hi-Sillot-OpenList-Frontend 用 `/openlist-spa/`。
//   - plugin-openlist 的 OpenListWebView.vue 通过 iframe 加载 `/openlist-spa/#/login`，
//     iframe 内显示的是 OpenListTeam 官方 web UI（Hi-Sillot-OpenList-Frontend 的 dev server）。
//   - :5244 binary 不 serve web UI（dist 留 stub 占位）—— OpenList web UI 完全由 :3000 vite dev 提供。
//
// 三个 vite + 1 个 backend 完全独立、各自 HMR，互不污染。
// 本地直接访问 5173 / 5174 / 3000 也能 dev。沙箱走 16000 时 HMR 仍不通但页面渲染不受影响。
//
// **绝对不做的事（铁律）**：
//   - **不 mock 任何 /api/**——全部 reverse proxy 到 :5244 OpenList backend，由 backend 真实服务。
//   - **不 build production dist**——dev preview 直接用 vite dev（按用户原则"开发预览不是生产预览"）。
//   - 不在 dev_preview_proxy 接管任何 /api/* mock 端点。
//
// 仅在 ENCV_DEV_PREVIEW=1 时启用，避免污染生产路径（生产 APK 内 gomobile 自己服务）。
type DevPreviewProxy struct {
	openlistViteURL          *url.URL // plugin-openlist/web (Capacitor UI) - :5174
	openlistFrontendViteURL  *url.URL // Hi-Sillot-OpenList-Frontend (OpenList web UI) - :3000
	encvMobileViteURL        *url.URL
	openlistBackendURL       *url.URL
}

// NewDevPreviewProxy 构造 dev preview proxy。从环境变量读上游地址，缺省 :5174 / :3000 / :5173 / :5244。
// 重要：plugin-openlist 与 Hi-Sillot-OpenList-Frontend 是两个独立项目，分别跑在不同端口。
func NewDevPreviewProxy() (*DevPreviewProxy, error) {
	// plugin-openlist/web：ENCV 的 Capacitor UI（Vue + Ionic）
	openlistURL := os.Getenv("ENCV_DEV_PLUGIN_OPENLIST_VITE_URL")
	if openlistURL == "" {
		openlistURL = "http://127.0.0.1:5174"
	}
	// Hi-Sillot-OpenList-Frontend：OpenListTeam 官方 web UI（SolidJS）—— plugin-openlist iframe 内加载
	openlistFrontendURL := os.Getenv("ENCV_DEV_OPENLIST_FRONTEND_VITE_URL")
	if openlistFrontendURL == "" {
		openlistFrontendURL = "http://127.0.0.1:3000"
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
	openlistFrontendParsed, err := url.Parse(openlistFrontendURL)
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
		"pluginOpenlistVite", openlistURL,
		"openlistFrontendVite", openlistFrontendURL,
		"encvMobileVite", mobileURL,
		"openlistBackend", backendURL,
		"trigger", "ENCV_DEV_PREVIEW=1",
	)

	return &DevPreviewProxy{
		openlistViteURL:         openlistParsed,
		openlistFrontendViteURL: openlistFrontendParsed,
		encvMobileViteURL:       mobileParsed,
		openlistBackendURL:      backendParsed,
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
// 把非 encv-go 自身已注册路径的请求分发到四个上游：
//   - /api/*                   → :5244 OpenList backend（reverse proxy，不是 mock）
//   - /openlist-spa/api/*      → :5244 OpenList backend（iframe 内 axios 用 /openlist-spa/ 前缀调用）
//   - /openlist-spa/*          → :3000 Hi-Sillot-OpenList-Frontend vite dev（OpenListTeam 官方 web UI）
//   - /__openlist-health       → :5174 plugin-openlist vite（vite 内部 health middleware）
//   - 其他                     → :5173 encv-mobile vite
//
// 说明：
//   - /openlist-ui/* 已被显式路由注册（RegisterExplicit），不会进 NoRoute。
//   - /api/config、/api/files/*、/api/tasks/* 等 encv-go 自己的 API 也已被注册，NoRoute 接不到。
//   - 关键：这是 **reverse proxy**（routing），不是 mock——后端真服务 :5244。
//
// plugin-openlist 的 vite (5174) 用 /__openlist-health 探测 backend 状态。
// plugin-openlist 的 OpenListWebView iframe 用 /openlist-spa/* 加载 Hi-Sillot-OpenList-Frontend (OpenList web UI)。
// 关键陷阱：iframe 内 axios 调用 `/api/...` 时，浏览器解析为 `/openlist-spa/api/...`（iframe base）。
// vite 的 proxy 规则只匹配 `/api` 不匹配 `/openlist-spa/api`，会 fall through 到 SPA index.html。
// **必须**在 dev_preview_proxy 层先把 `/openlist-spa/api/*` 摘出来反代到 :5244，
// 这样 iframe 内的 /api 调用就能直达 backend，vite 完全不需要感知前缀。
func (p *DevPreviewProxy) RegisterNoRoute(r *gin.Engine) {
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// /api/* 转发到 OpenList backend（:5244）
		// 这不是 mock，是真实 reverse proxy，让所有 /api/*（含 /api/public/*）由 backend 服务
		if strings.HasPrefix(path, "/api/") || path == "/api" {
			p.proxyTo(c, p.openlistBackendURL)
			return
		}

		// /openlist-spa/api/* 转发到 OpenList backend
		// iframe 内 axios 调 /api/... → 浏览器解析为 /openlist-spa/api/...（iframe base）
		// 必须**先于** /openlist-spa/* 规则匹配，否则会 fall through 到 vite SPA
		if strings.HasPrefix(path, "/openlist-spa/api/") || path == "/openlist-spa/api" {
			// strip /openlist-spa 前缀（backend 期望 /api/...）
			c.Request.URL.Path = strings.TrimPrefix(path, "/openlist-spa")
			p.proxyTo(c, p.openlistBackendURL)
			return
		}

		// /openlist-spa/* 是 Hi-Sillot-OpenList-Frontend (OpenList web UI) 的 dev server 路径
		// plugin-openlist 的 iframe 通过 /openlist-spa/#/login 加载 OpenList web UI
		//
		// 关键（2026-06-04）：**必须 strip /openlist-spa 前缀**再转发到 vite 3000。
		//   - vite 3000 base = "/"，index.html 模板里 <script src="/src/main.tsx"> 是绝对路径
		//   - 不 strip：vite 3000 收到 /openlist-spa/src/main.tsx → 找 root 下的 src/main.tsx 找不到 → 404
		//   - strip：vite 3000 收到 /src/main.tsx → 正常找到
		//   - 同时 dev_preview_proxy 的 /openlist-spa/api/* 规则仍然先匹配（见前一段代码），
		//     把 /openlist-spa/api/* 转发到 :5244 backend
		if strings.HasPrefix(path, "/openlist-spa/") || path == "/openlist-spa" {
			c.Request.URL.Path = strings.TrimPrefix(path, "/openlist-spa")
			if c.Request.URL.Path == "" {
				c.Request.URL.Path = "/"
			}
			p.proxyTo(c, p.openlistFrontendViteURL)
			return
		}

		// /__openlist-health 是 plugin-openlist vite (5174) 内部自定义 Node middleware
		// 用来探测 :5244 backend 是否可达
		if strings.HasPrefix(path, "/__openlist-") {
			p.proxyTo(c, p.openlistViteURL)
			return
		}

		// 其他全部 fallback 到 encv-mobile vite（:5173）
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
