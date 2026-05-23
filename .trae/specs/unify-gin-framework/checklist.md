- [ ] GoFrame (`gogf/gf/v2`) 依赖已从 go.mod 中移除
- [ ] `internal/admin/` 目录已删除，功能已合并到 `internal/server/`
- [ ] 所有路由使用 Gin 路由注册（无 net/http ServeMux 或 GoFrame RouterGroup）
- [ ] 单个 Gin 应用同时提供文件服务、API、Admin 和代理功能
- [ ] CORS 中间件使用 Gin 中间件实现，无自定义 CORS 代码
- [ ] JWT Auth 中间件使用 Gin 中间件签名
- [ ] BasicAuth 中间件使用 Gin 中间件签名
- [ ] Logging 中间件使用 Gin Logger
- [ ] WebSocket 保留 gorilla/websocket，通过 Gin 路由处理
- [ ] WebDAV 通过 `gin.WrapH()` 桥接到 Gin 路由
- [ ] HTML 模板渲染使用 `c.HTML()`
- [ ] 反向代理保留 `httputil.ReverseProxy`，通过 Gin 路由注册
- [ ] 启动逻辑统一为 `StartGinWithRetry`，无 `StartGfServerWithRetry`
- [ ] `cmd/encv/servers.go` 使用统一启动逻辑
- [ ] `go build ./...` 编译通过

### 移动端兼容性验证
- [ ] 所有 23 个 HTTP API 端点响应格式不变（对照 encv.ts 逐一测试）
- [ ] CORS 配置正确（移动端 WebView 跨域请求正常）
- [ ] WebSocket 连接正常（心跳 ping/pong，日志推送）
- [ ] 视频流端点正常（`/stream` 和 `/api/stream/external`）
- [ ] Capacitor 插件调用不受影响（JNI 桥接不经过 HTTP）
- [ ] `ENCV_MOBILE=1` 模式下跳过 admin 路由
- [ ] 单端口模式正常（移动端不再区分 backend/admin 端口）

### 桌面端验证
- [ ] Admin 登录/登出功能正常
- [ ] 反向代理（`/p/*`、`/p-api/*`）功能正常
- [ ] OpenList 代理功能正常
- [ ] WebDAV 功能正常
- [ ] 文件浏览和目录列表页面正常
