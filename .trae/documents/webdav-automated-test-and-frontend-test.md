# WebDAV 自动化测试 + 前端连接测试

## 问题

WebDAV 服务反复出问题，需要：
1. 在开发环境自动化测试 WebDAV 功能，确保基本功能正常
2. 前端添加 WebDAV 连接测试，让用户能快速判断 WebDAV 是否可用

## 方案

### 一、Go 后端自动化测试

编写 `internal/server/webdav_test.go`，启动完整服务器并测试 WebDAV 端点：

**测试用例**：
1. `TestWebDAV_PROPFIND_Root` — PROPFIND `/webdav/` 返回 207 Multi-Status
2. `TestWebDAV_PROPFIND_WithoutSlash` — PROPFIND `/webdav`（无尾部斜杠）返回 207
3. `TestWebDAV_AuthRequired` — 无凭据时返回 401
4. `TestWebDAV_AuthSuccess` — 正确凭据时返回 207
5. `TestWebDAV_AuthWrong` — 错误凭据时返回 401
6. `TestWebDAV_Options` — OPTIONS 请求返回正确头部
7. `TestWebDAV_GetFile` — GET 普通文件返回内容
8. `TestWebDAV_LogMiddleware` — WebDAV 请求被 slog 记录

**实现方式**：
- 创建临时目录作为 serving dir
- 创建 `config.Config` 对象，启用 WebDAV（设置 root、dir、username、password）
- 调用 `server.New()` + `server.Start()` 启动服务器
- 使用 `net/http` 客户端发送 WebDAV 请求
- 测试完成后调用 `server.Stop()`

**关键点**：
- 使用 `httptest.Server` 或直接启动服务器在随机端口
- 需要处理 `encv.Init()` 依赖（加密模块初始化）
- 临时目录中创建测试文件（普通文件 + 目录）

### 二、后端 API：测试本机 WebDAV

添加 `GET /api/webdav/test-local` 端点，测试本机 WebDAV 服务是否可用：

**返回信息**：
```json
{
  "available": true/false,
  "url": "http://127.0.0.1:2025/webdav/",
  "authRequired": true/false,
  "error": "错误信息（如果不可用）",
  "details": {
    "propfindRoot": "ok/fail",
    "authWorks": "ok/fail/na",
    "dirReadable": "ok/fail"
  }
}
```

**实现逻辑**：
1. 检查 `s.webdavFS != nil`（WebDAV 是否启用）
2. 构造本机 URL `http://127.0.0.1:{port}/webdav/`
3. 发送 PROPFIND 请求到 `/webdav/`，检查返回 207
4. 如果配置了凭据，测试认证是否工作
5. 返回详细结果

**修改文件**：
- `internal/server/server.go` — 注册路由
- `internal/server/mobile_api.go` — 实现 handler

### 三、前端 WebDAV 连接测试

在 Settings.vue 的 WebDAV 配置区域添加"测试连接"按钮：

**UI 设计**：
- 在 WebDAV section 的 list-header 旁边添加测试按钮
- 点击后调用 `/api/webdav/test-local`
- 显示测试结果（成功/失败 + 详细信息）
- 测试中显示 loading 状态

**修改文件**：
- `app/encv-mobile/src/api/encv.ts` — 添加 `testLocalWebDAV()` 函数
- `app/encv-mobile/src/views/Settings.vue` — 添加测试按钮和结果显示
- `app/encv-mobile/src/composables/useI18n.ts` — 添加 i18n 键

## 实施步骤

### 步骤 1：编写 Go 后端自动化测试

创建 `internal/server/webdav_test.go`：
1. 创建测试辅助函数 `setupTestServer(t)` — 创建临时目录、配置、启动服务器
2. 创建测试辅助函数 `teardownTestServer(s)` — 停止服务器、清理临时目录
3. 编写 8 个测试用例
4. 运行 `go test ./internal/server/ -run TestWebDAV -v` 验证

### 步骤 2：添加后端 test-local API

1. `internal/server/mobile_api.go` — 添加 `handleTestLocalWebDAVGin` handler
2. `internal/server/server.go` — 注册 `GET /api/webdav/test-local` 路由

### 步骤 3：添加前端 WebDAV 测试按钮

1. `app/encv-mobile/src/api/encv.ts` — 添加 `testLocalWebDAV()` 函数
2. `app/encv-mobile/src/views/Settings.vue` — 在 WebDAV section 添加测试按钮
3. `app/encv-mobile/src/composables/useI18n.ts` — 添加 i18n 键

### 步骤 4：验证

1. `go test ./internal/server/ -run TestWebDAV -v` 通过
2. `go build ./internal/...` 通过
3. `vue-tsc --noEmit && vite build` 通过
