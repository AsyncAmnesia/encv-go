package middleware

import "net/http"

// CorsMiddleware 创建一个处理CORS的中间件
func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Access-Control-Allow-Origin", "*")                                              // 【关键】设置允许的源，跨域请求
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")               // 【关键】设置允许的请求方法
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With") // 【关键】设置允许的请求头
		w.Header().Set("Cache-Control", "no-cache")                                                     // 禁用缓存方便调试

		// 【关键】处理预检请求
		// 如果是 OPTIONS 请求，直接返回 200 OK，不需要调用下一个处理器
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 对于其他请求，继续传递给下一个处理器
		next.ServeHTTP(w, r)
	})
}
