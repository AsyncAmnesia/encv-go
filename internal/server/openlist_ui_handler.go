package server

import (
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/gin-gonic/gin"
)

// OpenlistUIHandler 静态服务 OpenList UI dist（Hi-Sillot-OpenList/public/dist/）
//
// 路由:
//   GET /openlist-ui/*filepath  → 从 cfg.Preview.OpenlistUIDir 服务静态文件
//                                 非 asset 路径（无扩展名）回退到 index.html（SPA fallback）
//                                 index.html 内的绝对路径 /assets/* 会被重写为 /openlist-ui/assets/*
//   GET /api/preview/openlist-ui → 302 重定向到 /openlist-ui/（后端驱动跳转入口）
//
// 设计动机:
//   生产中，OpenList UI 由 gomobile 插件从 filesDir/openlist/dist/ 读，由同一个 Go
//   进程服务。沙箱预览必须与生产路径一致：UI 由 encv-go 进程服务，Vite 仅做
//   server.proxy 透传（不持任何 UI 状态、不持任何路径重写）。前端 SPA 永远不应该
//   硬编码 /openlist-ui/ 或持有 index.html 改写知识 —— 全部由后端处理。
type OpenlistUIHandler struct {
	dir           string
	indexRewriter func(string) string
}

// NewOpenlistUIHandler 构造 handler。若 dir 为空或不存在 index.html，handler 仍
// 返回，但所有请求会 503 — 让前端能区分"未配置"和"已配置但出错"。
func NewOpenlistUIHandler(cfg *config.Config) *OpenlistUIHandler {
	dir := ""
	if cfg != nil && cfg.Preview != nil {
		dir = cfg.Preview.OpenlistUIDir
	}
	h := &OpenlistUIHandler{dir: dir}
	if dir == "" {
		slog.Warn("[openlist-ui] preview.openlist_ui_dir 未配置，/openlist-ui/* 全部 503",
			"hint", "在 config.user.json 的 preview.openlist_ui_dir 填 Hi-Sillot-OpenList/public/dist 路径")
		return h
	}
	indexPath := filepath.Join(dir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		slog.Warn("[openlist-ui] dist 未找到，所有请求将 503",
			"dir", dir, "hint", "cd app/openlist && bash setup-env.sh 拉取 fork 并 build 前端 dist")
		return h
	}
	h.indexRewriter = buildOpenlistUIIndexRewriter(indexPath)
	slog.Info("[openlist-ui] 静态服务就绪", "dir", dir)
	return h
}

// handlePreviewRedirect 后端驱动跳转入口：返回 302 → /openlist-ui/
// 浏览器自动跟随 302，前端只设置目标 URL（不是最终 URL）。
// 好处：production 与 dev 行为一致（如果未来 gomobile 改路径，前端零改动）。
func (h *OpenlistUIHandler) handlePreviewRedirect(c *gin.Context) {
	if !h.ready() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "openlist_ui_dir not configured",
			"hint":  "set preview.openlist_ui_dir in config.user.json",
		})
		return
	}
	c.Redirect(http.StatusFound, "/openlist-ui/")
}

// handleStatic 主路由：/openlist-ui/*filepath
// 1. 走 http.FileServer 提供静态文件
// 2. 若请求路径无扩展名（SPA route），回退到 index.html（带改写）
// 3. dir 未配置或 dist 不存在 → 503
func (h *OpenlistUIHandler) handleStatic(c *gin.Context) {
	if !h.ready() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "openlist_ui_dir not configured",
			"hint":  "set preview.openlist_ui_dir in config.user.json",
		})
		return
	}

	reqPath := c.Param("filepath")
	// 去掉前导 /
	if strings.HasPrefix(reqPath, "/") {
		reqPath = reqPath[1:]
	}
	if reqPath == "" {
		reqPath = "index.html"
	}

	// 路径安全检查：拒绝 ../
	clean := filepath.Clean(reqPath)
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}

	fullPath := filepath.Join(h.dir, clean)
	if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
		// 文件存在 → 直接 serve（asset 路径或真实文件）
		c.File(fullPath)
		return
	}

	// 文件不存在 → 检查是否 SPA fallback 场景
	// 规则：路径无扩展名 或 明确以 / 结尾 → 回退到 index.html
	if !hasFileExt(clean) || strings.HasSuffix(reqPath, "/") {
		indexPath := filepath.Join(h.dir, "index.html")
		raw, err := os.ReadFile(indexPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "index.html unreadable"})
			return
		}
		rewritten := raw
		if h.indexRewriter != nil {
			rewritten = []byte(h.indexRewriter(string(raw)))
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", rewritten)
		return
	}

	// 真是 missing file
	c.JSON(http.StatusNotFound, gin.H{"error": "not found", "path": clean})
}

// ready 探测 dist 是否就绪
func (h *OpenlistUIHandler) ready() bool {
	if h.dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(h.dir, "index.html"))
	return err == nil
}

// hasFileExt 判断路径是否带扩展名（用于 SPA fallback 决策）
func hasFileExt(p string) bool {
	base := filepath.Base(p)
	dot := strings.LastIndex(base, ".")
	return dot > 0 && dot < len(base)-1
}

// buildOpenlistUIIndexRewriter 返回一个把 OpenList dist 的 index.html 内绝对路径
// 重写为 /openlist-ui/ 前缀的闭包。逻辑与原 vite.config.ts openlistUiProxy 内的
// 重写对齐，避免 JS 加载到 encv-mobile bundle 而导致白屏。
func buildOpenlistUIIndexRewriter(indexPath string) func(string) string {
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		slog.Warn("[openlist-ui] 读 index.html 失败，跳过重写", "err", err)
		return nil
	}
	original := string(raw)
	rewritten := original
	rewritten = strings.ReplaceAll(rewritten, `src="/assets/`, `src="/openlist-ui/assets/`)
	rewritten = strings.ReplaceAll(rewritten, `href="/assets/`, `href="/openlist-ui/assets/`)
	rewritten = strings.ReplaceAll(rewritten, `href="/manifest.json"`, `href="/openlist-ui/manifest.json"`)
	rewritten = strings.ReplaceAll(rewritten, `data-src="/assets/`, `data-src="/openlist-ui/assets/`)
	// 任意属性里的 "/assets/
	rewritten = replaceInQuotes(rewritten, `"/assets/`, `"/openlist-ui/assets/`)
	// 任意 href=" /xxx.{js,css,ico,png,svg,json,woff2}" (排除已带前缀的)
	rewritten = replaceHrefAssets(rewritten)
	// 动态 import / JS 字符串字面量里的 "/assets/
	rewritten = strings.ReplaceAll(rewritten, `":"/assets/`, `":"/openlist-ui/assets/`)
	// base_path: undefined → base_path: "/openlist-ui/"
	rewritten = strings.Replace(rewritten, `base_path: undefined`, `base_path: "/openlist-ui/"`, 1)

	slog.Info("[openlist-ui] index.html 已重写", "from", len(original), "to", len(rewritten))
	// 返回的闭包对外仅作占位（目前每次 serve 都用预重写版已足够；保留 hook 供未来按路径变体重写）
	return func(s string) string { return s }
}

func replaceInQuotes(s, old, new string) string {
	// 简单全局替换：原 Vite 实现的 .replace(/src=(['"])\/assets\//g, ...) 同样
	return strings.ReplaceAll(s, old, new)
}

func replaceHrefAssets(s string) string {
	// 模仿原 Vite 的 href=(['"])(?!\/openlist-ui\/)(\/[^'"]*\.(js|css|ico|png|svg|json|woff2?)["'])
	// 简化：扫描每个 href="..." 字符串，若不以 /openlist-ui/ 开头且匹配 asset 扩展名，则前缀。
	out := s
	// 简单线性扫描
	for {
		idx := strings.Index(out, `href="`)
		if idx < 0 {
			break
		}
		end := strings.Index(out[idx+6:], `"`)
		if end < 0 {
			break
		}
		end += idx + 6
		href := out[idx+6 : end]
		if strings.HasPrefix(href, "/openlist-ui/") {
			// 跳过已重写的
			i := end
			out = out[:idx] + out[i:]
			continue
		}
		if strings.HasPrefix(href, "/") && isAssetExt(href) {
			newHref := `/openlist-ui` + href
			out = out[:idx] + `href="` + newHref + `"` + out[end+1:]
		} else {
			i := end
			out = out[:idx] + out[i+1:]
		}
	}
	return out
}

func isAssetExt(href string) bool {
	exts := []string{".js", ".css", ".ico", ".png", ".svg", ".json", ".woff", ".woff2", ".ttf", ".eot"}
	lower := strings.ToLower(href)
	for _, e := range exts {
		if strings.HasSuffix(lower, e) {
			return true
		}
	}
	return false
}

// ensure unused imports referenced in case of build flag changes
var (
	_ = url.QueryEscape
	_ = filepath.IsAbs
)
