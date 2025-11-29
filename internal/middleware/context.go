package middleware

import (
	"net/http"

	"github.com/Soltus/encv-go/internal/config"
)

// WithConfig 是一个 HTTP 中间件，用于将配置对象注入到每个请求的 context 中。
// 它接收一个配置对象和一个下一个处理器，并返回一个新的处理器。
func WithConfig(cfg *config.Config, next http.Handler) http.Handler {
	// 返回一个 http.HandlerFunc，它实现了 http.Handler 接口
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 从请求的原始 context 创建一个新的 context，并注入配置
		ctx := config.NewContext(r.Context(), cfg)

		// 用这个新的 context 创建一个新的 *http.Request
		// 然后调用下一个处理器
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
