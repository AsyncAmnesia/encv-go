# 统一 HTTP 框架为 Fiber v3 重构 Spec

## Why

当前项目混用两套 HTTP 框架：GoFrame (ghttp) 用于 admin 服务（登录认证、反向代理、OpenList 代理、文件管理），net/http 标准库用于后端 API（移动端 API、文件服务、WebSocket、WebDAV）。两套框架各自有独立的中间件（CORS、Auth、Logging）、独立的启动逻辑（`StartGfServerWithRetry` vs `StartHttpHandlerWithRetry`）、独立的路由注册方式，导致：
1. 移动端 API（net/http）和 admin API（GoFrame）风格完全不同，维护成本高
2. 两套 CORS、Auth 中间件逻辑重复但实现不同
3. GoFrame 依赖链庞大（引入了 ORM、配置管理、模板引擎等不需要的功能），但实际只用了路由和中间件
4. 新增 API 时需要决定放哪个服务，增加心智负担

统一到 Fiber v3 后，所有 HTTP 处理在一个框架内完成，减少依赖、统一风格、简化部署。

## What Changes

- **BREAKING**：移除 GoFrame (`gogf/gf/v2`) 依赖，admin 服务改用 Fiber v3 实现
- **BREAKING**：后端服务从 `net/http.ServeMux` 迁移到 Fiber v3 路由
- 合并两个独立服务器（backend + admin）为单个 Fiber 应用
- 统一中间件实现（CORS、Auth、Logging）
- 统一启动逻辑（端口递增 + 自检）
- WebSocket 从 `gorilla/websocket` 迁移到 Fiber v3 的 WebSocket 支持（`fiberws`）
- WebDAV 处理器通过 `adaptor` 中间件桥接到 Fiber
- 移除 `internal/admin/` 目录，功能合并到 `internal/server/`
- 移除 `internal/register/server_start.go` 中的 `StartGfServerWithRetry`

## Impact

- Affected specs: refactor-mobile-backend-service
- Affected code:
  - `internal/admin/` — 整个目录重写/合并
  - `internal/server/` — 从 net/http 迁移到 Fiber
  - `internal/middleware/` — 统一到 Fiber 中间件
  - `internal/register/` — 简化启动逻辑
  - `cmd/encv/servers.go` — 启动逻辑调整
  - `go.mod` — 移除 GoFrame，添加 Fiber v3

## 架构分析：当前双服务器结构

### Backend Server (net/http, 端口由 cfg.Server.Port 决定)
| 路由 | Handler | 用途 |
|------|---------|------|
| `/ping` | handlePing | 自检 |
| `/health` | handleHealth | 健康检查 |
| `/stream` | handleStreamRequest | 加密视频流 |
| `/api/config` | handleConfigAPI | 配置读写 |
| `/api/config/schema` | handleConfigSchemaAPI | 配置 schema |
| `/api/files` | handleMobileFiles | 文件列表 |
| `/api/file` | handleReadFileContent | 文件内容 |
| `/api/files/exists` | handleFileExistsAPI | 文件存在检查 |
| `/api/files/search` | handleSearchFilesAPI | 文件搜索 |
| `/api/tasks` | handleMobileTasks | 任务 CRUD |
| `/api/webdav/test` | handleTestWebDAV | WebDAV 测试 |
| `/api/remote/info` | handleRemoteInfo | 远端信息 |
| `/api/remote/openlist` | handleOpenlistSites | OpenList 站点 |
| `/api/permissions` | handlePermissions | 权限检查 |
| `/api/server/shutdown` | handleServerShutdown | 服务器关闭 |
| `/api/index/stats` | handleIndexStats | 索引统计 |
| `/api/index/rebuild` | handleIndexRebuild | 索引重建 |
| `/api/index/clear` | handleIndexClear | 索引清除 |
| `/api/stream/external` | handleStreamExternalFile | 外部文件流 |
| `/api/logs` | handleAPILogs | 日志 API |
| `/ws` | handleWebSocket | WebSocket |
| `/` | handleRequest | 文件服务（目录列表+静态文件） |
| WebDAV 路由 | goWebdav.Handler | WebDAV 服务 |

### Admin Server (GoFrame ghttp, 端口由 cfg.Admin.Port 决定)
| 路由 | Handler | 用途 |
|------|---------|------|
| `/login` GET/POST | handleLogin | 登录页面 |
| `/logout` ALL | 登出 | 登出 |
| `/admin/*` | GoFrame Bind (hello, file) | Admin API |
| `/admin/file/analyze` POST | file_v1_analyze | 文件分析 |
| `/admin/file/rename` POST | file_v1_rename | 文件重命名 |
| `/openlist/sites` GET | OpenList 站点列表 | OpenList 管理 |
| `/openlist/sites/{siteId}/*` | OpenList 代理 | OpenList 代理 |
| `/p/*` ALL | 反向代理到 Backend | 文件浏览代理 |
| `/p-api/*` ALL | 反向代理到 Backend | API 代理 |

### 中间件对比
| 功能 | Backend (net/http) | Admin (GoFrame) |
|------|-------------------|-----------------|
| CORS | `middleware.CorsMiddleware` | `admin/middleware.CORS` |
| Auth | `middleware.BasicAuth` (WebDAV) | `admin/middleware.AuthMiddleware` (JWT) |
| Logging | `middleware.LoggingMiddleware` | 无 |
| Config注入 | `middleware.WithConfig` | GoFrame 内置配置 |
| Response | 无 | `admin/middleware.Response` |

### GoFrame 依赖范围
- `internal/admin/admin.go` — 主入口
- `internal/admin/middleware/` — CORS、Auth、Response（3 个文件）
- `internal/admin/controller/file/` — 文件分析、重命名（4 个文件）
- `internal/admin/controller/hello/` — Hello API（3 个文件）
- `internal/admin/logic/auth/` — JWT 认证（2 个文件）
- `internal/admin/logic/openlist/` — OpenList 代理（5 个文件）
- `internal/admin/injector/` — HTML 注入（1 个文件）
- `internal/admin/routes/` — 路由常量（1 个文件）
- `internal/register/server_start.go` — `StartGfServerWithRetry`

## ADDED Requirements

### Requirement: 统一 Fiber v3 应用

系统 SHALL 将当前的双服务器架构（GoFrame admin + net/http backend）合并为单个 Fiber v3 应用。

#### Scenario: 单端口服务
- **WHEN** 启动 `encv start` 命令
- **THEN** 系统在单个端口上启动 Fiber 应用，同时提供文件服务、API、Admin 和代理功能
- **AND** 端口由 `cfg.Server.Port` 决定，`cfg.Admin.Port` 配置项不再使用

#### Scenario: 移动端模式
- **WHEN** 环境变量 `ENCV_MOBILE=1`
- **THEN** 系统跳过 admin 相关路由（登录、代理、OpenList），仅注册移动端 API

### Requirement: Fiber v3 路由注册

系统 SHALL 使用 Fiber v3 路由 API 注册所有端点，替代 net/http ServeMux 和 GoFrame RouterGroup。

#### Scenario: API 路由
- **WHEN** 注册 `/api/*` 路由
- **THEN** 使用 `app.Group("/api")` 创建路由组，应用 CORS 和 Config 中间件

#### Scenario: Admin 路由
- **WHEN** 注册 `/admin/*` 路由
- **THEN** 使用 `app.Group("/admin")` 创建路由组，应用 JWT Auth 中间件

#### Scenario: 文件服务路由
- **WHEN** 注册 `/` 兜底路由
- **THEN** 使用 `app.Use("/")` 处理文件浏览和静态文件服务

### Requirement: 统一中间件

系统 SHALL 使用 Fiber v3 中间件机制统一所有 HTTP 中间件。

#### Scenario: CORS 中间件
- **WHEN** 请求到达
- **THEN** 使用 Fiber 内置 CORS 中间件处理跨域，替代两套自定义 CORS 实现

#### Scenario: JWT Auth 中间件
- **WHEN** 访问 admin 路由
- **THEN** 使用 Fiber 中间件检查 JWT token，未认证重定向到 `/login`

#### Scenario: Basic Auth 中间件
- **WHEN** 访问 WebDAV 路由
- **THEN** 使用 Fiber 中间件检查 HTTP Basic Auth

#### Scenario: Logging 中间件
- **WHEN** 请求处理完成
- **THEN** 使用 Fiber 内置 Logger 中间件记录请求信息

### Requirement: WebSocket 迁移

系统 SHALL 使用 Fiber v3 兼容的 WebSocket 库替代 gorilla/websocket。

#### Scenario: 日志 WebSocket
- **WHEN** 客户端连接 `/ws`
- **THEN** 升级为 WebSocket 连接，推送日志消息

### Requirement: WebDAV 桥接

系统 SHALL 通过 Fiber adaptor 将 golang.org/x/net/webdav Handler 桥接到 Fiber 路由。

#### Scenario: WebDAV 请求
- **WHEN** 请求匹配 WebDAV 路由前缀
- **THEN** 通过 `adaptor.HTTPHandler` 将请求转发给 webdav.Handler

### Requirement: HTML 模板渲染

系统 SHALL 使用 Fiber v3 的模板引擎支持（html/template）替代手动 template.Execute。

#### Scenario: 目录列表
- **WHEN** 浏览目录
- **THEN** 使用 Fiber 模板引擎渲染目录列表页面

#### Scenario: 登录页面
- **WHEN** 访问 `/login`
- **THEN** 使用 Fiber 模板引擎渲染登录页面

### Requirement: 反向代理

系统 SHALL 使用 Fiber v3 的反向代理中间件替代 GoFrame + httputil.ReverseProxy。

#### Scenario: 文件代理
- **WHEN** 访问 `/p/*` 路由
- **THEN** 使用 Fiber 代理中间件转发到后端文件服务

### Requirement: 启动逻辑统一

系统 SHALL 使用 Fiber v3 的 `app.Listen()` 替代两套启动逻辑，保留端口递增和自检功能。

#### Scenario: 端口递增
- **WHEN** 默认端口被占用
- **THEN** 自动尝试下一个端口

#### Scenario: 自检
- **WHEN** 服务器启动后
- **THEN** 通过 `/ping` 端点验证实例 ID

## MODIFIED Requirements

### Requirement: 服务器配置
原：`cfg.Server.Port` 用于 backend，`cfg.Admin.Port` 用于 admin
改：统一使用 `cfg.Server.Port`，`cfg.Admin.Port` 保留但作为向后兼容的别名（如果设置了则使用该端口，否则使用 Server.Port）

### Requirement: OpenList 代理
原：通过 GoFrame RouterGroup 注册
改：通过 Fiber v3 路由组注册，逻辑不变

## REMOVED Requirements

### Requirement: GoFrame 依赖
**Reason**: 统一到 Fiber v3，GoFrame 不再需要
**Migration**: 移除 `gogf/gf/v2` 依赖，删除 `internal/admin/` 目录

### Requirement: 双服务器架构
**Reason**: 合并为单服务器
**Migration**: 删除 `StartGfServerWithRetry`，简化 `cmd/encv/servers.go`

### Requirement: gorilla/websocket 依赖
**Reason**: Fiber v3 有内置 WebSocket 支持
**Migration**: 使用 `github.com/gofiber/contrib/websocket` 或 Fiber v3 内置 WebSocket

## 风险评估

### 低风险
- API 路由迁移：当前 `mobile_api.go` 中的 handler 都是 `func(w http.ResponseWriter, r *http.Request)` 签名，通过 Fiber adaptor 可逐步迁移
- 中间件统一：Fiber 内置 CORS、Logger、BasicAuth 中间件，功能覆盖完整

### 中风险
- WebSocket 迁移：需要确认 Fiber WebSocket 库与当前 `ws_hub.go` 的兼容性
- HTML 注入（HookAfterServe）：GoFrame 的 `BindHookHandler` 用于在响应后注入工具栏 HTML，Fiber 需要用 Hooks 或中间件实现类似功能
- OpenList 代理：`proxy_ghttp.go` 大量使用 `ghttp.Request`，需要重写为 Fiber Ctx

### 需要注意
- Fiber v3 基于 fasthttp，与 net/http 不兼容，所有 handler 签名需要从 `func(w http.ResponseWriter, r *http.Request)` 改为 `func(c fiber.Ctx) error`
- WebDAV handler 是标准 `http.Handler` 接口，需要通过 `adaptor.HTTPHandler` 桥接
- 文件服务（`http.ServeFile`）需要改用 `c.SendFile`
- 模板渲染需要改用 Fiber 的 Views 引擎
