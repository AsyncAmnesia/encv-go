# 后端未连接排查 & Dev 预览集成后端测试方案

## 一、根因分析：为什么显示"后端未连接"

### 结论：不是代码 bug，而是 **dev 预览环境中没有运行后端进程**

完整调用链路追踪：

```
前端 App.vue.onMounted()
  → useTheme().initTheme()           ✅ 纯客户端，无问题
  → useWebSocket().connect()         → ws://127.0.0.1:2025/ws  ❌ 无进程监听
     ↓ 连接失败
Files/Tabs 页面使用 useServerStatus()
  → checkServerStatus()              → GET http://127.0.0.1:2025/health  ❌ 无进程监听
     ↓ fetch 抛异常
  → isOnline = false                 → 显示"后端未连接"
```

### 排除的伪因

| 假设 | 排除理由 |
|------|---------|
| 端口不匹配？ | 前端默认 `2025`，用户 `config.user.json` 中 `server.port=2025`，一致 |
| 路由没注册？ | 上轮已实现，`go build` 编译通过，路由正确注册在 `Start()` 中 |
| CORS 问题？ | 所有路由被 `CorsMiddleware` 包裹，已覆盖新路由 |
| WS 路径错误？ | 前端 `getWebSocketUrl()` 拼接为 `ws://127.0.0.1:2025/ws`，后端注册了 `/ws` |

### 核心原因

> **Dev Preview 只启动了 Vite（端口 5173），没有任何进程监听 2025 端口。**
> 前端所有 `fetch('http://127.0.0.1:2025/...')` 和 `new WebSocket('ws://127.0.0.1:2025/ws')` 全部 Connection Refused。

---

## 二、Dev 预览能否拉起后端？

### 能力评估

| 能力 | 是否支持 | 说明 |
|------|---------|------|
| 编译 Go 二进制 | ✅ | `go build ./cmd/encv/` 已验证可用 |
| 运行后台进程 | ✅ | `RunCommand` 支持 `blocking: false` + long_running_process |
| 创建测试配置 | ✅ | 可写入沙箱文件系统 |
| 同时运行前后端 | ✅ | 后台跑 Go 服务 + 启动 Vite dev server |
| OpenPreview 激活 | ✅ | 对 Vite 端口生效 |

### 但有一个关键障碍

`encv start` 命令会同时拉起 **两个服务**：
1. **Backend Server**（我们的 HTTP API）— 端口 2025 — 这个我们需要的
2. **Admin Server**（GoFrame ghttp）— 端口 1808 — **依赖 SQLite 数据库**、GoFrame 框架

查看 [servers.go](file:///workspace/cmd/encv/servers.go) 的 start 命令：
```go
s.Start(Version)                              // ← Backend（我们需要这个）
adminServer, _ := admin.SetupAdminServer(...) // ← Admin（依赖重）
register.StartGfServerWithRetry(...)          // ← GoFrame 服务（可能缺 DB）
```

如果直接运行 `encv start`，Admin 服务初始化可能因为缺少数据库而失败，连带整个进程退出。

### 解决方案：创建轻量级 Dev-Only 后端

新建一个最小化的 `cmd/encv-dev/main.go`，**只启动 Backend Server**，跳过 Admin/GoFrame/WebDAV/插件等重量级组件：

```
cmd/encv-dev/
  main.go          — 入口：加载配置 → NewServer → 注册移动端路由 → ListenAndServe
```

这个 dev-server 只做一件事：**在指定端口上提供 REST API + WebSocket**，用于前端开发调试。

---

## 三、实施步骤

### Step 1: 创建 Dev-Only 后端入口

**新建文件**: `cmd/encv-dev/main.go`

功能清单：
- 从 `config.user.json` 加载配置（或使用默认值）
- 创建 `server.NewServer(ctx, configPath)`
- 手动创建 `ServeMux`，只注册移动端需要的路由：
  - `GET  /health`
  - `GET  /api/files?path=`
  - `DELETE /api/files?path=`
  - `GET  /stream?path=`
  - `GET  /api/tasks` / `POST /api/tasks` / ...
  - `POST /api/webdav/test`
  - `GET  /ws`（WebSocket）
  - `GET  /api/config` / `PUT  /api/config`
  - `GET  /api/config/schema`
  - `GET  /ping`（保留兼容）
- 使用 `http.ListenAndServe` 直接启动
- 不启动 Admin、不初始化插件、不加载 WebDAV
- 支持通过 `-port` flag 指定端口（默认 2025）

### Step 2: 创建 Dev 最小化配置

**新建文件**: `config.dev.json`

```json
{
  "password": "",
  "recover": false,
  "output_path": "./output",
  "plugin_settings": {},
  "server": { "port": 2025, "dir": "/" },
  "admin": { "port": 18080, "password": "" },
  "webdav": { "port": 12340, "root": "/webdav/", "dir": "", "username": "", "password": "" },
  "proxy": { "sites": {}, "disable_signature_verification": true },
  "log": { "level": "debug", "file": "", "console": true }
}
```

关键点：
- `server.dir: "/"` — 在沙箱中映射到 filesystem root
- `log.level: "debug"` — 方便调试
- 其他字段填最小值避免 nil panic

### Step 3: 配置 Vite 开发代理（可选增强）

**修改文件**: `app/encv-mobile/vite.config.ts`

添加 `server.proxy`，让开发时 API 请求走代理而非跨域直连：

```ts
server: {
  port: 5173,
  host: '0.0.0.0',
  proxy: {
    '/api': { target: 'http://127.0.0.1:2025', changeOrigin: true },
    '/health': { target: 'http://127.0.0.1:2025', changeOrigin: true },
    '/stream': { target: 'http://127.0.0.1:2025', changeOrigin: true },
    '/ws': { target: 'ws://127.0.0.1:2025', ws: true },
    '/ping': { target: 'http://127.0.0.1:2025', changeOrigin: true },
  }
}
```

这样前端的 `getApiBaseUrl()` 在开发时可返回空字符串（走相对路径代理），生产环境仍用绝对 URL。

### Step 4: 创建一键启动脚本（可选）

创建一个组合命令，按顺序启动：
1. `go run ./cmd/encv-dev/` （后台）
2. 等 1 秒让后端就绪
3. `cd app/encv-mobile && npm run dev` （前台/OpenPreview）

### Step 5: 验证

1. 构建并后台启动 dev-server
2. `curl http://127.0.0.1:2025/health` → `{"status":"ok"}`
3. `curl http://127.0.0.1:2025/api/files?path=/` → JSON 文件列表
4. 启动 Vite dev server + OpenPreview
5. 浏览器确认前端显示"在线"状态
6. 测试 Files 页面浏览文件
7. 测试 Settings 页面配置编辑
