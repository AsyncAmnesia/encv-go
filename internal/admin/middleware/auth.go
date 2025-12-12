// internal/admin/middleware/auth.go
package middleware

import (
	"log"
	"strings"

	"github.com/Soltus/encv-go/internal/admin/logic/auth"
	"github.com/Soltus/encv-go/internal/admin/routes"
	"github.com/gogf/gf/v2/net/ghttp"
)

const LoginHeader = "X-ENCV-User-Authenticated"

// AuthMiddleware JWT认证中间件
func AuthMiddleware(jwtManager *auth.JWTManager) func(r *ghttp.Request) {
	return func(r *ghttp.Request) {
		// 检查是否是登录页面，避免重定向循环
		if strings.HasSuffix(r.URL.Path, routes.Login) {
			r.Middleware.Next()
			return
		}
		log.Printf("[AuthMiddleware] Processing path: %s", r.URL.Path)

		// 从Cookie获取token
		token := auth.GetTokenFromCookie(r.Request)

		// 【关键修改1】无论token是否存在或有效，都尝试将其放入Authorization Header
		// 这使得后端服务（或ModifyResponse）可以自主进行验证
		if token != "" {
			r.Request.Header.Set("Authorization", "Bearer "+token)
		}

		// 【关键修改2】后续的验证逻辑仅用于决定是否允许本次请求通过代理
		// 这与后端服务的验证是解耦的
		if token == "" {
			// 没有token，重定向到登录页
			redirectToLogin(r)
			return
		}

		// 验证token，仅用于代理自身的访问控制
		_, err := jwtManager.ValidateToken(token)
		if err != nil {
			// token无效，清除并重定向到登录页
			auth.ClearAuthCookie(r.Response.Writer)
			redirectToLogin(r)
			return
		}

		// token有效，将claims存入上下文（供GoFrame内部使用，如Admin API）
		claims, _ := jwtManager.ValidateToken(token) // 再次验证以获取claims，或者优化一下
		r.SetCtxVar("auth_claims", claims)
		r.SetCtxVar("is_authenticated", true)

		// 执行后续处理
		r.Middleware.Next()
	}
}

// func AuthMiddleware(jwtManager *auth.JWTManager) func(r *ghttp.Request) {
// 	return func(r *ghttp.Request) {
// 		// 检查是否是登录页面，避免重定向循环
// 		if strings.HasSuffix(r.URL.Path, routes.Login) {
// 			r.Middleware.Next()
// 			return
// 		}
// 		log.Printf("[AuthMiddleware] Processing path: %s", r.URL.Path)

// 		// 从Cookie获取token
// 		token := auth.GetTokenFromCookie(r.Request)
// 		if token == "" {
// 			// 没有token，重定向到登录页
// 			redirectToLogin(r)
// 			return
// 		}

// 		// 验证token
// 		claims, err := jwtManager.ValidateToken(token)
// 		if err != nil {
// 			// token无效，清除并重定向到登录页
// 			auth.ClearAuthCookie(r.Response.Writer)
// 			redirectToLogin(r)
// 			return
// 		}

// 		// token有效，将claims存入上下文
// 		r.SetCtxVar("auth_claims", claims)
// 		r.SetCtxVar("is_authenticated", true)
// 		// 【关键】设置请求头，让代理能获取到认证状态
// 		r.Request.Header.Set(LoginHeader, "true")

// 		// 执行后续处理
// 		r.Middleware.Next()
// 	}
// }

// // 代理使用
// func IsLoggedIn(r *http.Response) bool {
// 	return r.Request.Header.Get(LoginHeader) == "true"
// }

// // goframe 使用
// func IsLoggedInGF(r *ghttp.Request) bool {
// 	return r.GetCtxVar("is_authenticated").Bool()
// }

// redirectToLogin 重定向到登录页
func redirectToLogin(r *ghttp.Request) {
	// 保存当前URL用于登录后重定向
	currentURL := r.URL.String()
	if currentURL != "" && !strings.Contains(currentURL, routes.Login) {
		auth.SetRedirectCookie(r.Response.Writer, currentURL)
	}

	// 重定向到登录页
	r.Response.RedirectTo(routes.Login)
}
