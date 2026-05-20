# ENCV-Mobile 后端 API 实现 Spec

## Why

encv-mobile 前端（Ionic + Vue）已完整实现，包含 Files/Player/Tasks/WebDAV/Settings 五个页面及 WebSocket 实时通讯层。但 Go 后端缺少前端所需的所有 REST API 端点和 WebSocket 端点，导致 App 显示"后端未连接"。需要在 `internal/server` 中实现完整的移动端后端 API。

## What Changes

- 在 `internal/server/server.go` 的路由注册中新增移动端 API 路由
- 新增 `internal/server/mobile_api.go` 文件，实现所有移动端需要的 HTTP API handler：
  - `GET /health` — 健康检查（兼容前端的 `checkServerStatus()`）
  - `GET /api/files?path=xxx` — JSON 格式文件列表（替代当前 HTML 输出）
  - `DELETE /api/files?path=xxx` — 删除文件
  - `GET /stream?path=xxx` — 兼容前端的流式播放接口（现有 `?file=` 的别名）
  - `GET /api/tasks` — 任务列表
  - `POST /api/tasks` — 创建任务
  - `POST /api/tasks/{id}/cancel` — 取消任务
  - `POST /api/tasks/{id}/retry` — 重试任务
  - `POST /api/webdav/test` — 测试 WebDAV 连接
- 新增 `internal/server/mobile_ws.go` 文件，实现 WebSocket 端点：
  - `GET /ws` — WebSocket 连接，支持 ping/pong 心跳和事件推送

## Impact

- Affected specs: 无已有 spec 受影响
- Affected code:
  - `internal/server/server.go` — 新增路由注册
  - `internal/server/mobile_api.go`（新建）— 所有移动端 HTTP API handler
  - `internal/server/mobile_ws.go`（新建）— WebSocket handler

## ADDED Requirements

### Requirement: 健康检查接口

系统 SHALL 提供 `GET /health` 接口，返回 `{"status":"ok"}` JSON 响应，HTTP 200。

#### Scenario: 前端检查服务器状态
- **WHEN** 前端调用 `GET /health`
- **THEN** 返回 `{"status":"ok"}`，HTTP status 200

### Requirement: JSON 文件列表接口

系统 SHALL 提供 `GET /api/files?path=xxx` 接口，返回指定目录的文件列表为 JSON 格式。

#### Scenario: 获取根目录文件列表
- **WHEN** 前端调用 `GET /api/files?path=/`
- **THEN** 返回 `{"files":[{"name":"xxx","path":"/xxx","isDirectory":false,"size":1024,"modified":"2025-01-01T00:00:00Z"}]}`，隐藏以 `.` 开头的文件

#### Scenario: 获取子目录文件列表
- **WHEN** 前端调用 `GET /api/files?path=/subdir`
- **THEN** 返回该子目录的文件 JSON 列表

#### Scenario: 路径遍历攻击防护
- **WHEN** 请求路径包含 `..` 或尝试访问服务目录之外的位置
- **THEN** 返回 HTTP 403 Forbidden

### Requirement: 删除文件接口

系统 SHALL 提供 `DELETE /api/files?path=xxx` 接口，用于删除指定文件。

#### Scenario: 删除成功
- **WHEN** 前端调用 `DELETE /api/files?path=/test.txt` 且文件存在
- **THEN** 文件被删除，返回 HTTP 200

#### Scenario: 文件不存在
- **WHEN** 目标文件不存在
- **THEN** 返回 HTTP 404

### Requirement: 流式播放接口兼容

系统 SHALL 支持 `GET /stream?path=xxx` 查询参数格式，作为现有 `?file=` 格式的别名。

#### Scenario: 通过 path 参数播放
- **WHEN** 前端调用 `GET /stream?path=/video.mp4`
- **THEN** 行为与 `GET /stream?file=/video.mp4` 完全一致

### Requirement: 任务管理接口

系统 SHALL 提供任务 CRUD 接口的内存级实现（后续可对接真实任务系统）。

#### Scenario: 获取任务列表
- **WHEN** 前端调用 `GET /api/tasks`
- **THEN** 返回 `{"tasks":[...]}`，包含 id/type/sourcePath/status/progress/error/createdAt 字段

#### Scenario: 创建任务
- **WHEN** 前端 POST `{"type":"encrypt","sourcePath":"/test.mp4"}` 到 `/api/tasks`
- **THEN** 返回新创建的任务对象，状态为 `queued`

#### Scenario: 取消任务
- **WHEN** 前端调用 `POST /api/tasks/{id}/cancel`
- **THEN** 对应任务状态更新为 `cancelled`

#### Scenario: 重试任务
- **WHEN** 前端调用 `POST /api/tasks/{id}/retry`
- **THEN** 对应任务状态重置为 `queued`

### Requirement: WebDAV 连接测试接口

系统 SHALL 提供 `POST /api/webdav/test` 接口，测试 WebDAV 配置是否可达。

#### Scenario: WebDAV 测试
- **WHEN** 前端 POST WebDAV 配置到 `/api/webdav/test`
- **THEN** 尝试连接并返回成功/失败状态

### Requirement: WebSocket 实时通讯

系统 SHALL 提供 `GET /ws` WebSocket 端点，支持心跳和事件推送。

#### Scenario: WS 连接建立
- **WHEN** 前端连接 `ws://{host}/ws`
- **THEN** 连接建立成功，服务端推送 `server:status` 事件 `{online:true}`

#### Scenario: 心跳机制
- **WHEN** 客户端发送 `{"type":"ping"}`
- **THEN** 服务端回复 `{"type":"pong"}`

#### Scenario: WS 断开
- **WHEN** 客户端断开连接
- **THEN** 服务端清理资源
