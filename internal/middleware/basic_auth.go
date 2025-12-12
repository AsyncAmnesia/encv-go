package middleware

import (
	"encoding/base64"
	"net/http"
	"strings"
)

// BasicAuth 创建一个用于 HTTP 基础认证的中间件，用于 Webdav
// 如果 username 或 password 为空，则直接通过，不进行认证
func BasicAuth(username, password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// 如果用户名或密码为空，则禁用认证
		if username == "" || password == "" {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 检查 Authorization 头
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// 头不存在，要求认证
				unauthorized(w)
				return
			}

			// 解析 "Basic <credentials>" 格式
			const prefix = "Basic "
			if !strings.HasPrefix(authHeader, prefix) {
				unauthorized(w)
				return
			}

			// 解码 Base64 凭证
			credentials, err := base64.StdEncoding.DecodeString(authHeader[len(prefix):])
			if err != nil {
				unauthorized(w)
				return
			}

			// 凭证格式应为 "username:password"
			parts := strings.SplitN(string(credentials), ":", 2)
			if len(parts) != 2 || parts[0] != username || parts[1] != password {
				unauthorized(w)
				return
			}

			// 认证成功，调用下一个处理器
			next.ServeHTTP(w, r)
		})
	}
}

// unauthorized 向客户端发送认证质询
func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="ENCV WebDAV"`)
	w.WriteHeader(http.StatusUnauthorized)
	// 可选：可以写入一个响应体，但大多数客户端会忽略它
	// w.Write([]byte("Unauthorized"))
}
