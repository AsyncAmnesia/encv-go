// internal/admin/middleware/cors.go
package middleware

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// CORS 允许所有跨域请求
func CORS(r *ghttp.Request) {
	g.Log().Infof(r.Context(), "[%s] %s -> ", r.Method, r.URL)
	r.Response.CORS(ghttp.CORSOptions{
		AllowOrigin:      "*",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,PATCH",
		AllowHeaders:     "*",
		AllowCredentials: "true",
		ExposeHeaders:    "*",
		MaxAge:           86400,
	})
	r.Middleware.Next()
}

// CORSDefault 使用默认配置允许跨域
func CORSDefault(r *ghttp.Request) {
	r.Response.CORSDefault()
	r.Middleware.Next()
}
