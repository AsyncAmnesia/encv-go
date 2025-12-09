package middleware

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// Response 是一个统一处理响应的中间件
// 它会捕获控制器返回的错误，并将其格式化为标准的 JSON 响应
func Response(r *ghttp.Request) {
	// 执行下一个中间件或控制器
	r.Middleware.Next()

	// 获取控制器返回的错误和响应数据
	err := r.GetError()
	res := r.GetHandlerResponse()

	// 如果没有错误，正常返回业务数据
	if err == nil {
		r.Response.WriteJsonExit(ghttp.DefaultHandlerResponse{
			Code:    gcode.CodeOK.Code(),
			Message: gcode.CodeOK.Message(),
			Data:    res,
		})
		return
	}

	// 如果有错误，格式化错误信息并返回
	g.Log().Errorf(r.Context(), "API Error: %+v", err) // 记录详细的错误堆栈到日志

	r.Response.WriteJsonExit(ghttp.DefaultHandlerResponse{
		Code:    gerror.Code(err).Code(),
		Message: err.Error(), // err.Error() 会包含完整的错误链信息
		Data:    nil,
	})
}
