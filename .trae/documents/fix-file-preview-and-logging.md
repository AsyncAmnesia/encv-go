# 实现文件预览 + 增加前后端日志

## 问题 1：预览文件没有实现

### 现状分析

当前 `FilePreview.vue` 只实现了**文本文件预览**（通过 `/api/file` 读取文件内容，显示在 `<pre>` 标签中）。

缺失的预览类型：
1. **图片预览**（jpg/png/gif/webp/svg）— 后端已有 `/stream?path=...` 端点，可直接用作 `<img src>`
2. **PDF 预览** — 可用 `<iframe>` 或 `<object>` 加载 `/stream?path=...`
3. **大文件/二进制文件提示** — 当前 `ReadFileContent` 限制 2MB，超过限制时显示"文件过大"提示，提供流式查看选项
4. **路由问题** — `Files.vue` 中 `handleFileClick` 只把 `image` 和 `document` 类型路由到 `/tabs/preview`，但 `FilePreview.vue` 没有根据文件类型做区分展示

### 修复方案

**A. `FilePreview.vue` 改为根据文件类型分模式预览**

```
文件类型 → 预览方式:
  image/*  → <img :src="streamUrl"> 全屏展示
  pdf      → <iframe :src="streamUrl"> 嵌入展示
  text/*   → 现有文本预览（<pre> 标签）
  其他     → 显示文件信息 + "不支持预览"提示
  二进制/过大 → 显示文件信息 + "文件过大无法预览"提示
```

**B. `Files.vue` 的 `handleFileClick` 路由调整**

当前逻辑：
- video/audio/encrypted → Player
- 其他 → Preview

修改为：
- video/audio/encrypted → Player
- image/pdf/document/other → Preview

这样所有非媒体文件都走 Preview 页面，由 Preview 页面内部根据类型决定展示方式。

### 实施步骤

1. **修改 `FilePreview.vue`**：
   - 从 `route.query` 获取 `path` 和 `name`
   - 用 `getFileCategory()` 判断文件类型
   - 图片：直接用 `getFileStreamUrl(path)` 作为 `<img src>`
   - PDF：用 `<iframe :src="streamUrl">`
   - 文本：保持现有 `readFileContent` 逻辑
   - 其他/二进制/过大：显示文件元信息 + 提示

2. **修改 `Files.vue` 的 `handleFileClick`**：
   - image 类型也路由到 Preview（而非当前被忽略）
   - 简化逻辑：非 video/audio/encrypted 都走 Preview

## 问题 2：前后端日志过少

### 现状分析

**后端日志问题：**
- `mobile_api.go`：所有 handler 几乎没有日志（没有请求开始/完成日志）
- `mobile_service.go`：只有 Error 级别日志，缺少 Info/Warn
- `server.go`：只有启动时两条 Info 日志
- 没有 HTTP 请求日志中间件（请求方法、路径、耗时、状态码）
- `ws_hub.go`：只有连接/断开日志

**前端日志问题：**
- `api/encv.ts`：所有 API 调用没有 `console.info/warn/error`
- `Files.vue`：没有操作日志
- `FilePreview.vue`：没有加载日志
- `App.vue`：权限申请没有日志
- `useWebSocket.ts`：有少量 `console.warn/error`，但缺少 `console.info`
- `DevLogs.vue`：后端日志只通过 WebSocket 接收，但后端并没有通过 WebSocket 推送 slog 日志

**关键缺失：后端 slog 日志没有桥接到 WebSocket**

后端使用 `slog` 输出日志到 stderr（被 Android logcat 捕获），但前端 DevLogs 页面的"后端日志"标签页只能收到 WebSocket 消息。需要将 slog 输出同时推送到 WebSocket，前端才能看到后端日志。

### 修复方案

**A. 后端：添加 HTTP 请求日志中间件**

创建 `internal/middleware/logging.go`，记录每个请求的方法、路径、状态码、耗时：

```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}
        next.ServeHTTP(wrapped, r)
        slog.Info("HTTP request",
            "method", r.Method,
            "path", r.URL.Path,
            "status", wrapped.statusCode,
            "duration", time.Since(start).String(),
        )
    })
}
```

**B. 后端：添加 slog → WebSocket 桥接**

创建自定义 `slog.Handler`，在输出日志到 stderr 的同时，将日志推送到 WSHub：

```go
type WSLogHandler struct {
    inner  slog.Handler
    hub    *service.WSHub
    level  slog.Level
}
```

这样所有 `slog.Info/Warn/Error` 调用都会自动推送到前端 DevLogs 页面。

**C. 后端：给 mobile_api.go 各 handler 添加日志**

- `handleListFilesAPI`：Info 级别记录请求路径和返回文件数
- `handleDeleteFileAPI`：Warn 级别记录删除操作
- `handleReadFileContent`：Info 级别记录读取的文件路径
- `handleCreateTask`：Info 级别记录任务创建
- `handlePermissions`：Info 级别记录权限检查结果

**D. 后端：给 mobile_service.go 添加更多日志**

- `ListFiles`：Info 级别记录查询路径和结果数量（当前是 Debug）
- `ReadFileContent`：Info 级别记录文件路径和大小
- `DeleteFile`：Warn 级别记录删除路径
- `CheckStoragePermission`：Info 级别记录检查结果

**E. 前端：给 API 调用添加日志**

在 `api/encv.ts` 中，给关键 API 函数添加 `console.info/warn/error`：
- `listFiles`：info 记录请求路径和返回数量，error 记录失败
- `readFileContent`：info 记录请求路径
- `deleteFile`：warn 记录删除操作
- `checkServerStatus`：info 记录结果

**F. 前端：给组件添加操作日志**

- `Files.vue`：info 记录加载文件、导航、权限状态变化
- `FilePreview.vue`：info 记录文件加载、类型判断
- `App.vue`：info 记录权限申请结果

**G. 前端：DevLogs 后端标签页解析 slog 格式**

当前 `onWsMessage` 把所有 WebSocket 消息都标记为 `info` 级别。需要解析后端推送的日志消息，正确设置 level：

```typescript
function onWsMessage(data: any) {
  if (data.type === 'log') {
    backendLogs.value.push({
      id: ++nextId,
      timestamp: data.timestamp || new Date().toLocaleTimeString(...),
      level: data.level || 'info',
      message: data.message || JSON.stringify(data),
    })
  }
}
```

### 实施步骤

1. **创建 `internal/middleware/logging.go`**：HTTP 请求日志中间件
2. **创建 `internal/server/ws_log_handler.go`**：slog → WebSocket 桥接 handler
3. **修改 `server.go`**：注册日志中间件 + 初始化 WSLogHandler
4. **修改 `mobile_api.go`**：各 handler 添加 Info/Warn/Error 日志
5. **修改 `mobile_service.go`**：各方法添加 Info/Warn 日志
6. **修改 `api/encv.ts`**：各 API 函数添加 console.info/warn/error
7. **修改 `Files.vue`**：添加操作日志
8. **修改 `FilePreview.vue`**：添加操作日志 + 实现多类型预览
9. **修改 `App.vue`**：添加权限申请日志
10. **修改 `DevLogs.vue`**：解析后端日志消息的 level

## 文件变更清单

| 文件 | 变更 |
|------|------|
| `internal/middleware/logging.go` | 新建：HTTP 请求日志中间件 |
| `internal/server/ws_log_handler.go` | 新建：slog → WebSocket 桥接 |
| `internal/server/server.go` | 注册日志中间件 + 初始化 WSLogHandler |
| `internal/server/mobile_api.go` | 各 handler 添加日志 |
| `internal/service/mobile_service.go` | 各方法添加日志 |
| `app/encv-mobile/src/api/encv.ts` | API 调用添加 console 日志 |
| `app/encv-mobile/src/views/Files.vue` | 添加操作日志 + handleFileClick 路由调整 |
| `app/encv-mobile/src/views/FilePreview.vue` | 实现图片/PDF/文本/其他多类型预览 + 日志 |
| `app/encv-mobile/src/views/App.vue` | 权限申请日志 |
| `app/encv-mobile/src/views/DevLogs.vue` | 解析后端日志 level |
