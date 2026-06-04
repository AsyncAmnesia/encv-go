package server

import (
	"log/slog"
	"net/http"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/gin-gonic/gin"
)

// OpenlistUIHandler dev 沙箱预览的 OpenList UI 入口（仅 302 跳转）
//
// 路由:
//   GET /api/preview/openlist-ui → 302 重定向到 /openlist-ui/
//
// 设计动机:
//   前端 SPA 永远不应该硬编码 /openlist-ui/ —— 由后端决定最终 URL。
//   这样 dev（vite dev server :3000）和 production（APK 内 gomobile 进程
//   从 filesDir/openlist/dist/ 服务）的入口点对前端完全一致：永远走 /api/preview/openlist-ui。
//
// 与生产的协调:
//   - dev 沙箱：/openlist-ui/* 由 encv-mobile vite 透传到 OpenList-Frontend
//     的 vite dev server（:3000，以 OPENLIST_PREVIEW_BASE="/openlist-ui/" 启动，
//     由 vite-plugin-dynamic-base 改写 index.html 内的 asset 路径）。
//   - 生产：APK 内 gomobile 进程从 filesDir/openlist/dist/ 自己服务。
//   两种模式下，前端只发起 GET /api/preview/openlist-ui，由 encv-go 302。
type OpenlistUIHandler struct {
	previewConfigured bool
}

// NewOpenlistUIHandler 构造 handler。当前 dev 模式下永远启用（只要 dev 模式开启）。
// 保留 cfg 参数是为了未来按 platform / build flavor 控制可用性。
func NewOpenlistUIHandler(_ *config.Config) *OpenlistUIHandler {
	slog.Info("[openlist-ui] dev 预览入口就绪（302 → /openlist-ui/，由 vite 透传到 :3000）")
	return &OpenlistUIHandler{previewConfigured: true}
}

// handlePreviewRedirect 后端驱动跳转入口：返回 302 → /openlist-ui/
// 浏览器自动跟随 302，前端只设置目标 URL（不是最终 URL）。
func (h *OpenlistUIHandler) handlePreviewRedirect(c *gin.Context) {
	if !h.previewConfigured {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "openlist-ui preview disabled",
			"hint":  "preview only available in dev sandbox",
		})
		return
	}
	c.Redirect(http.StatusFound, "/openlist-ui/")
}
