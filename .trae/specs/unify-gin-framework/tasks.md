# Tasks

- [ ] Task 1: 添加 Gin 依赖，移除 GoFrame 依赖
  - [ ] SubTask 1.1: `go get github.com/gin-gonic/gin` 添加 Gin
  - [ ] SubTask 1.2: `go mod tidy` 清理 GoFrame 依赖（注意：此时先保留 GoFrame，等 admin 迁移完再删）
  - [ ] SubTask 1.3: 验证 `go build ./...` 编译通过

- [ ] Task 2: 创建 Gin 应用入口和统一启动逻辑
  - [ ] SubTask 2.1: 创建 `internal/server/gin_app.go`，定义 `NewGinApp()` 函数，初始化 Gin 引擎、注册全局中间件（CORS、Logger、Config注入）
  - [ ] SubTask 2.2: 重写 `internal/register/server_start.go`，将 `StartHttpHandlerWithRetry` 和 `StartGfServerWithRetry` 合并为 `StartGinWithRetry`，使用 `net/http.Server{Handler: ginEngine}` + 端口递增 + 自检
  - [ ] SubTask 2.3: 修改 `cmd/encv/servers.go`，使用新的统一启动逻辑

- [ ] Task 3: 迁移 Backend API 路由（net/http → Gin）
  - [ ] SubTask 3.1: 在 `gin_app.go` 中注册 `/ping`、`/health` 路由，使用 `gin.WrapF()` 包裹现有 handler（渐进式）
  - [ ] SubTask 3.2: 注册 `/api/config`、`/api/config/schema` 路由
  - [ ] SubTask 3.3: 注册 `/api/files`、`/api/file`、`/api/files/exists`、`/api/files/search` 路由
  - [ ] SubTask 3.4: 注册 `/api/tasks` 路由
  - [ ] SubTask 3.5: 注册 `/api/webdav/test`、`/api/remote/info`、`/api/remote/openlist` 路由
  - [ ] SubTask 3.6: 注册 `/api/permissions`、`/api/server/shutdown` 路由
  - [ ] SubTask 3.7: 注册 `/api/index/stats`、`/api/index/rebuild`、`/api/index/clear` 路由
  - [ ] SubTask 3.8: 注册 `/api/stream/external`、`/api/logs` 路由
  - [ ] SubTask 3.9: 注册 `/ws` WebSocket 路由（`gin.WrapF()` 包裹）
  - [ ] SubTask 3.10: 注册 WebDAV 路由（`gin.WrapH()` 包裹 webdav.Handler）
  - [ ] SubTask 3.11: 注册 `/` NoRoute handler 处理文件服务
  - [ ] 注：初期全部使用 `gin.WrapF/H` 包裹，功能验证通过后再逐步改为 Gin 原生 handler

- [ ] Task 4: 迁移 Admin 功能（GoFrame → Gin）
  - [ ] SubTask 4.1: 迁移 JWT 认证逻辑（`internal/admin/logic/auth/` → `internal/server/auth/`），JWT 核心逻辑（CreateToken、ValidateToken）与框架无关，只需调整 cookie 读写
  - [ ] SubTask 4.2: 迁移登录/登出路由（`/login`、`/logout`），GoFrame `r.Parse(&req)` → Gin `c.ShouldBind()`，模板渲染 → `c.HTML()`
  - [ ] SubTask 4.3: 迁移 Admin API（`/admin/file/analyze`、`/admin/file/rename`），GoFrame controller → Gin handler
  - [ ] SubTask 4.4: 迁移 HTML 注入逻辑（GoFrame `BindHookHandler` → Gin 中间件，在 `c.Next()` 后修改响应）
  - [ ] SubTask 4.5: 迁移反向代理（`/p/*`、`/p-api/*`），保留 `httputil.ReverseProxy`，通过 Gin 路由注册

- [ ] Task 5: 迁移 OpenList 代理（GoFrame → Gin）
  - [ ] SubTask 5.1: 将 `proxy_ghttp.go` 的 `ghttp.Request` 改为 `*gin.Context`，`r.Response.Writer` → `c.Writer`，`r.Request` → `c.Request`
  - [ ] SubTask 5.2: 将 `multi_site_server.go` 的 GoFrame RouterGroup 改为 Gin 路由组
  - [ ] SubTask 5.3: 迁移 Token 管理和站点选择逻辑

- [ ] Task 6: 统一中间件
  - [ ] SubTask 6.1: 用 Gin CORS 中间件替代 `middleware/cors.go` 和 `admin/middleware/cors.go`
  - [ ] SubTask 6.2: 用 Gin Logger 中间件替代 `middleware/logging.go`
  - [ ] SubTask 6.3: 重写 BasicAuth 中间件为 Gin 中间件（或使用 `gin.BasicAuth()`）
  - [ ] SubTask 6.4: 重写 JWT Auth 中间件为 Gin 中间件
  - [ ] SubTask 6.5: 重写 Config 注入中间件为 Gin 中间件

- [ ] Task 7: 清理旧代码和依赖
  - [ ] SubTask 7.1: 删除 `internal/admin/` 目录
  - [ ] SubTask 7.2: 删除 `internal/middleware/cors.go`（已被 Gin CORS 替代）
  - [ ] SubTask 7.3: 删除 `internal/middleware/logging.go`（已被 Gin Logger 替代）
  - [ ] SubTask 7.4: 清理 `go.mod`，移除 `gogf/gf/v2`
  - [ ] SubTask 7.5: 验证 `go build ./...` 编译通过

- [ ] Task 8: 逐步将 `gin.WrapF/H` 包裹的 handler 改为 Gin 原生 handler
  - [ ] SubTask 8.1: 将 `mobile_api.go` 中的 handler 从 `func(w, r)` 改为 `func(c *gin.Context)`，使用 `c.JSON()`、`c.ShouldBindJSON()` 等 Gin API
  - [ ] SubTask 8.2: 将文件服务 handler 改为 Gin 原生
  - [ ] SubTask 8.3: 将配置 API handler 改为 Gin 原生
  - [ ] 注：此任务为优化项，可在功能验证通过后分批进行

- [ ] Task 9: 移动端兼容性验证
  - [ ] SubTask 9.1: 验证所有 23 个 HTTP API 端点响应格式不变（对照 encv.ts 中的前端函数逐一测试）
  - [ ] SubTask 9.2: 验证 CORS 配置正确（移动端 WebView 跨域请求正常）
  - [ ] SubTask 9.3: 验证 WebSocket 连接正常（`ws://127.0.0.1:2025/ws`，心跳 ping/pong，日志推送）
  - [ ] SubTask 9.4: 验证视频流端点正常（`/stream?path=` 和 `/api/stream/external?path=`，MPV 可正常播放）
  - [ ] SubTask 9.5: 验证 Capacitor 插件调用不受影响（GoProcessPlugin 通过 JNI 调用，不经过 HTTP）
  - [ ] SubTask 9.6: 验证 `ENCV_MOBILE=1` 模式下跳过 admin 路由
  - [ ] SubTask 9.7: 验证单端口模式（移动端不再需要区分 backend/admin 端口）

- [ ] Task 10: 桌面端验证
  - [ ] SubTask 10.1: 验证 Admin 登录/登出功能
  - [ ] SubTask 10.2: 验证反向代理（`/p/*`、`/p-api/*`）功能
  - [ ] SubTask 10.3: 验证 OpenList 代理功能
  - [ ] SubTask 10.4: 验证 WebDAV 功能
  - [ ] SubTask 10.5: 验证文件浏览和目录列表页面

# Task Dependencies

- [Task 1] 是所有后续任务的前置条件
- [Task 2] 依赖 [Task 1]
- [Task 3] 依赖 [Task 2]
- [Task 4] 依赖 [Task 2]，可与 [Task 3] 并行
- [Task 5] 依赖 [Task 4]
- [Task 6] 可与 [Task 3-5] 并行开发
- [Task 7] 依赖 [Task 3-6] 全部完成
- [Task 8] 依赖 [Task 7]，是优化项
- [Task 9] 依赖 [Task 7]
- [Task 10] 依赖 [Task 7]，可与 [Task 9] 并行

# 渐进式迁移策略

相比 Fiber 方案的核心优势：**Gin 完全兼容 net/http，可以渐进式迁移**。

1. **Phase 1（功能对齐）**：Task 1-5，使用 `gin.WrapF/H` 包裹所有现有 handler，确保功能不变
2. **Phase 2（清理）**：Task 6-7，统一中间件，删除旧代码和 GoFrame 依赖
3. **Phase 3（优化）**：Task 8，逐步将 handler 改为 Gin 原生风格

Phase 1 完成后即可发布，Phase 3 可在后续迭代中逐步完成。
