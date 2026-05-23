# Tasks

- [ ] Task 1: 添加 Fiber v3 依赖，移除 GoFrame 依赖
  - [ ] SubTask 1.1: `go get github.com/gofiber/fiber/v3` 添加 Fiber v3
  - [ ] SubTask 1.2: `go get github.com/gofiber/contrib/websocket` 添加 Fiber WebSocket
  - [ ] SubTask 1.3: `go mod tidy` 清理 GoFrame 依赖
  - [ ] SubTask 1.4: 验证 `go build ./...` 编译通过（此时只是添加依赖，不改代码，可能需要临时保留 GoFrame）

- [ ] Task 2: 创建 Fiber 应用入口和统一启动逻辑
  - [ ] SubTask 2.1: 创建 `internal/server/fiber_app.go`，定义 `NewFiberApp()` 函数，初始化 Fiber 应用、注册全局中间件（CORS、Logger、Config注入）
  - [ ] SubTask 2.2: 重写 `internal/register/server_start.go`，将 `StartHttpHandlerWithRetry` 和 `StartGfServerWithRetry` 合并为 `StartFiberWithRetry`，使用 Fiber 的 `app.Listen()` + 端口递增 + 自检
  - [ ] SubTask 2.3: 修改 `cmd/encv/servers.go`，使用新的统一启动逻辑

- [ ] Task 3: 迁移 Backend API 路由（net/http → Fiber）
  - [ ] SubTask 3.1: 迁移 `/ping`、`/health` 路由
  - [ ] SubTask 3.2: 迁移 `/api/config`、`/api/config/schema` 路由
  - [ ] SubTask 3.3: 迁移 `/api/files`、`/api/file`、`/api/files/exists`、`/api/files/search` 路由
  - [ ] SubTask 3.4: 迁移 `/api/tasks`、`/api/tasks/` 路由
  - [ ] SubTask 3.5: 迁移 `/api/webdav/test`、`/api/remote/info`、`/api/remote/openlist` 路由
  - [ ] SubTask 3.6: 迁移 `/api/permissions`、`/api/server/shutdown` 路由
  - [ ] SubTask 3.7: 迁移 `/api/index/stats`、`/api/index/rebuild`、`/api/index/clear` 路由
  - [ ] SubTask 3.8: 迁移 `/api/stream/external`、`/api/logs` 路由
  - [ ] SubTask 3.9: 将 `mobile_api.go` 中的 handler 从 `func(w http.ResponseWriter, r *http.Request)` 签名改为 `func(c fiber.Ctx) error`，使用 Fiber 的 JSON 绑定和响应 API

- [ ] Task 4: 迁移文件服务和目录列表
  - [ ] SubTask 4.1: 将 `handleRequest`、`servePath`、`serveFile` 改为 Fiber handler
  - [ ] SubTask 4.2: 将 `http.ServeFile` 替换为 `c.SendFile`
  - [ ] SubTask 4.3: 将 `listFilesInDir` 的模板渲染改为 Fiber Views 引擎
  - [ ] SubTask 4.4: 迁移加密文件流服务（`serveEncryptedFile`、`handleStreamRequest`）

- [ ] Task 5: 迁移 WebSocket
  - [ ] SubTask 5.1: 将 `mobile_ws.go` 从 gorilla/websocket 迁移到 Fiber WebSocket
  - [ ] SubTask 5.2: 验证 `ws_hub.go` 和 `ws_log_handler.go` 兼容性

- [ ] Task 6: 迁移 WebDAV
  - [ ] SubTask 6.1: 使用 `adaptor.HTTPHandler` 将 webdav.Handler 桥接到 Fiber 路由
  - [ ] SubTask 6.2: 将 BasicAuth 中间件改为 Fiber 中间件

- [ ] Task 7: 迁移 Admin 功能（GoFrame → Fiber）
  - [ ] SubTask 7.1: 迁移 JWT 认证逻辑（`internal/admin/logic/auth/` → `internal/server/middleware/jwt_auth.go`）
  - [ ] SubTask 7.2: 迁移登录/登出路由（`/login`、`/logout`），使用 Fiber 模板渲染
  - [ ] SubTask 7.3: 迁移 Admin API（`/admin/file/analyze`、`/admin/file/rename`），从 GoFrame controller 改为 Fiber handler
  - [ ] SubTask 7.4: 迁移 HTML 注入逻辑（GoFrame `BindHookHandler` → Fiber Hooks/中间件）
  - [ ] SubTask 7.5: 迁移反向代理（`/p/*`、`/p-api/*`），从 httputil.ReverseProxy 改为 Fiber 代理中间件或 adaptor

- [ ] Task 8: 迁移 OpenList 代理（GoFrame → Fiber）
  - [ ] SubTask 8.1: 将 `proxy_ghttp.go` 的 `ghttp.Request` 改为 `fiber.Ctx`
  - [ ] SubTask 8.2: 将 `multi_site_server.go` 的 GoFrame RouterGroup 改为 Fiber 路由组
  - [ ] SubTask 8.3: 迁移 Token 管理和站点选择逻辑

- [ ] Task 9: 统一中间件
  - [ ] SubTask 9.1: 用 Fiber 内置 CORS 中间件替代 `middleware/cors.go` 和 `admin/middleware/cors.go`
  - [ ] SubTask 9.2: 用 Fiber Logger 中间件替代 `middleware/logging.go`
  - [ ] SubTask 9.3: 重写 BasicAuth 中间件为 Fiber 中间件
  - [ ] SubTask 9.4: 重写 JWT Auth 中间件为 Fiber 中间件
  - [ ] SubTask 9.5: 重写 Config 注入中间件为 Fiber 中间件

- [ ] Task 10: 清理旧代码和依赖
  - [ ] SubTask 10.1: 删除 `internal/admin/` 目录
  - [ ] SubTask 10.2: 删除 `internal/middleware/cors.go`（已被 Fiber CORS 替代）
  - [ ] SubTask 10.3: 删除 `internal/middleware/logging.go`（已被 Fiber Logger 替代）
  - [ ] SubTask 10.4: 清理 `go.mod`，移除 `gogf/gf/v2` 和 `gorilla/websocket`
  - [ ] SubTask 10.5: 验证 `go build ./...` 编译通过

- [ ] Task 11: 验证和测试
  - [ ] SubTask 11.1: 启动服务器，验证所有 API 端点正常
  - [ ] SubTask 11.2: 验证移动端 API 功能
  - [ ] SubTask 11.3: 验证 Admin 登录/代理功能
  - [ ] SubTask 11.4: 验证 OpenList 代理功能
  - [ ] SubTask 11.5: 验证 WebDAV 功能
  - [ ] SubTask 11.6: 验证 WebSocket 日志推送

# Task Dependencies

- [Task 1] 是所有后续任务的前置条件
- [Task 2] 依赖 [Task 1]
- [Task 3-8] 依赖 [Task 2]，可以并行开发
- [Task 9] 可以与 [Task 3-8] 并行开发
- [Task 10] 依赖 [Task 3-9] 全部完成
- [Task 11] 依赖 [Task 10]

# 推荐实施策略

由于 Fiber v3 基于 fasthttp 而非 net/http，所有 handler 签名必须改变，无法渐进式迁移。建议：

1. **一次性切换**：在一个分支上完成所有迁移，避免混用两套框架
2. **先迁移 Backend API**（Task 3），因为 handler 数量最多但逻辑最简单
3. **再迁移 Admin 功能**（Task 7-8），因为涉及 GoFrame 特有 API 的替换
4. **最后处理文件服务和 WebDAV**（Task 4-6），因为涉及 HTTP Handler 桥接
