# 修复 DevLogs 独立窗口 + MP4 播放 + DevLogs 日志显示

## 问题 1：DevLogs 独立窗口

### 现状

DevLogs 当前是 Tabs 中的一个 tab（`/tabs/devlogs`），用户切换到其他 tab 时无法同时查看日志。在 Android 上，用户需要频繁在 DevLogs 和其他页面之间切换，体验差。

### 方案

将 DevLogs 从 Tabs 中移出，改为独立路由 `/devlogs`，从 Settings 页面提供入口。这样用户可以在 Android 的多窗口/分屏模式下同时查看日志和操作应用。

具体改动：
1. `router/index.ts`：将 `/devlogs` 从 `/tabs/` children 移到顶层路由
2. `Tabs.vue`：移除 devlogs tab-button
3. `Settings.vue`：添加"开发者日志"入口，点击跳转 `/devlogs`
4. `DevLogs.vue`：添加返回按钮（因为不再是 tab 子页面，没有底部导航栏）

## 问题 2：无法播放普通 MP4 文件

### 根因（从 logcat 确认）

logcat 清楚地显示了问题：

```
[Detector] FAILED: Footer magic number mismatch. Expected: ENVC, Got: ����
Container is invalid and not cached, rebuilding as last resort
ERROR: failed to get a readable container path for '...mp4': container manager failed to rebuild '...mp4': failed to find and parse manifest: failed to read header for version detection: invalid header GetDecryptReader failed
```

**根因**：`/stream` 端点（`handleStreamRequest`）总是调用 `serveEncryptedFile()`，对所有文件都尝试 ENCV 容器检测和解密。当文件不是 ENCV 容器时，`DetectContainer()` 返回错误，然后 `GetReadablePath()` 尝试 rebuild 也失败，最终返回 HTTP 500。

**缺少降级路径**：对于非加密文件（普通 MP4），应该直接用 `http.ServeFile` 提供原始文件，而不是尝试解密。

### 方案

在 `handleStreamRequest` 中，先检测文件是否为 ENCV 容器。如果不是，直接用 `http.ServeFile` 提供原始文件：

```go
func (s *Server) handleStreamRequest(w http.ResponseWriter, r *http.Request) {
    // ... 参数解析 ...

    // 先检测是否为 ENCV 容器
    _, err := detector.DetectContainer(cleanedFilePath)
    if err != nil {
        // 不是 ENCV 容器，直接提供原始文件
        slog.Info("File is not an ENCV container, serving raw file", "path", cleanedFilePath)
        http.ServeFile(w, r, cleanedFilePath)
        return
    }

    // 是 ENCV 容器，走解密流程
    s.serveEncryptedFile(w, r, cleanedFilePath)
}
```

这样：
- 普通 MP4 → `http.ServeFile` 直接提供（支持 Range 请求）
- 加密 MP4 → `serveEncryptedFile` 解密后提供

## 问题 3：DevLogs 没看到更多前后端日志

### 根因

**后端日志未到达前端的原因**：`WSLogHandler` 发送的消息格式是 `{type: "log", level: "info", message: "...", timestamp: "..."}`，但 `useWebSocket.ts` 的 `handleMessage` 将所有 WebSocket 消息解析为 `{type, data}` 格式：

```typescript
const msg: WsMessage = JSON.parse(event.data)  // {type: "log", level: "info", message: "...", timestamp: "..."}
eventBus.emit('ws:message', { type: msg.type, data: msg.data })  // data = undefined!
```

`msg.data` 是 `undefined`（因为 WSLogHandler 的消息没有 `data` 字段），所以 DevLogs 的 `onWsMessage` 收到的是 `{type: "log", data: undefined}`，`data.type` 不是 `"log"`，走了 fallback 路径，把整个对象 JSON.stringify 作为消息。

**前端日志缺失的原因**：`DevLogs.vue` 的 `hijackConsole()` 在 `onMounted` 时劫持 console，在 `onUnmounted` 时恢复。但当用户离开 DevLogs 页面时，`onUnmounted` 触发，console 被恢复，后续的 console.info/warn/error 不会被捕获。只有用户在 DevLogs 页面时才能捕获前端日志。

### 方案

**A. 修复 WSLogHandler 消息格式**

将 `WSLogHandler` 的消息格式改为与 `WSMessage` 一致，使用 `data` 字段：

```go
msg, _ := json.Marshal(map[string]any{
    "type": "log",
    "data": map[string]string{
        "level":     levelStr,
        "message":   r.Message,
        "timestamp": time.Now().Format("15:04:05"),
    },
})
```

这样 `useWebSocket.ts` 解析后 `msg.data` 就是 `{level, message, timestamp}`，DevLogs 的 `onWsMessage` 能正确识别 `data.type === "log"`。

**B. DevLogs 的 onWsMessage 适配**

修改 `onWsMessage` 以正确处理新的消息格式：

```typescript
function onWsMessage(data: any) {
  if (data && data.type === 'log' && data.data) {
    const logData = data.data
    backendLogs.value.push({
      id: ++nextId,
      timestamp: logData.timestamp || new Date().toLocaleTimeString('zh-CN', { hour12: false }),
      level: ['debug', 'info', 'warn', 'error'].includes(logData.level) ? logData.level : 'info',
      message: logData.message || '',
    })
    return
  }
  // fallback
  const msg = typeof data === 'string' ? data : JSON.stringify(data)
  backendLogs.value.push({ id: ++nextId, timestamp: new Date().toLocaleTimeString('zh-CN', { hour12: false }), level: 'info', message: msg })
}
```

**C. 前端日志持久化**

将 `hijackConsole()` 从 DevLogs 组件移到 App.vue，这样无论用户在哪个页面，console 日志都会被捕获。DevLogs 页面只负责显示，不再负责劫持。

在 App.vue 中：
- `onMounted` 时劫持 console，将日志存入全局数组
- `onUnmounted` 时不恢复（App 永远不会卸载）

在 DevLogs.vue 中：
- 移除 `hijackConsole/restoreConsole`
- 从全局数组读取前端日志

## 文件变更清单

| 文件 | 变更 |
|------|------|
| `internal/server/server_handle.go` | `handleStreamRequest` 添加非加密文件降级：`detector.DetectContainer` 失败时 `http.ServeFile` |
| `internal/server/ws_log_handler.go` | 消息格式改为 `{type: "log", data: {level, message, timestamp}}` |
| `app/encv-mobile/src/views/DevLogs.vue` | 适配新消息格式；移除 hijackConsole/restoreConsole；从全局读取前端日志 |
| `app/encv-mobile/src/router/index.ts` | DevLogs 从 tabs children 移到顶层路由 |
| `app/encv-mobile/src/views/Tabs.vue` | 移除 devlogs tab-button |
| `app/encv-mobile/src/views/Settings.vue` | 添加"开发者日志"入口 |
| `app/encv-mobile/src/composables/useFrontendLogs.ts` | 新建：全局前端日志收集器 |
| `app/encv-mobile/src/App.vue` | 使用全局前端日志收集器替代 DevLogs 内的 hijackConsole |
