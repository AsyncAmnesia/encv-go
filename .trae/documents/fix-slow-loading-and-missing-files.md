# 修复首屏加载慢 + 只显示目录不显示文件

## 问题 1：首屏加载过慢

### 根因分析

当前首屏加载流程存在串行等待问题：

1. `App.vue` 的 `onMounted` 调用 `connect()`（WebSocket 连接），但此时后端可能还在启动
2. `Files.vue` 的 `onMounted` 调用 `loadFiles()`
3. `loadFiles()` 先调用 `checkServerStatus()`（HTTP `/health`），再调用 `listFiles()`（HTTP `/api/files`）——**两个串行请求**
4. 如果后端还没就绪，`checkServerStatus()` 返回 `online: false`，页面显示"服务器离线"，用户必须手动点重试
5. **没有自动等待后端就绪的机制**——原生平台上后端启动需要数秒，前端不会自动重试

### 修复方案

**A. 前端：添加后端就绪自动等待机制**

在 `Files.vue` 的 `loadFiles()` 中，如果后端未就绪，自动重试而非直接显示离线状态：

```typescript
async function loadFiles() {
  loading.value = true
  const maxRetries = isNative() ? 15 : 3  // 原生平台后端启动需要更长时间
  const retryDelay = 1000

  for (let i = 0; i < maxRetries; i++) {
    const result = await checkServerStatus()
    serverOnline.value = result.online
    if (result.online) {
      try {
        files.value = await listFiles(currentPath.value)
      } catch (error) { ... }
      loading.value = false
      return
    }
    // 等待后端启动
    await new Promise(r => setTimeout(r, retryDelay))
  }
  // 所有重试都失败
  loading.value = false
}
```

**B. 前端：合并 `/health` 和 `/api/files` 为一次请求**

当前 `loadFiles()` 先请求 `/health`，再请求 `/api/files`，这是两次串行 HTTP 请求。可以：
- 方案 1：直接请求 `/api/files`，如果成功则说明后端在线，失败则说明离线（**推荐**，减少一次往返）
- 方案 2：并行请求 `/health` 和 `/api/files`

方案 1 更简洁：`/api/files` 成功本身就证明了后端在线。

**C. 前端：监听 `encv:backend-ready` 事件自动加载**

`useServerStatus.ts` 已经监听了 `encv:backend-ready` 事件并设置了 `isOnline`。`Files.vue` 应该监听这个事件，在后端就绪时自动加载文件列表，而不是只依赖 `loadFiles()` 的轮询。

### 实施步骤

1. 修改 `Files.vue` 的 `loadFiles()`：移除先 `checkServerStatus` 再 `listFiles` 的串行逻辑，改为直接 `listFiles`，失败时自动重试
2. 在 `Files.vue` 中监听 `encv:backend-ready` / `server:status` 事件，后端就绪时自动加载
3. 移除 `checkServerStatus` 的单独调用（`loadFiles` 和 `handleRefresh` 中）

## 问题 2：只显示目录没有显示文件

### 根因分析

后端 `ListFiles()` 使用 `os.ReadDir()` 列出 `/storage/emulated/0` 的内容，代码本身不过滤文件。

**最可能的原因**：`entry.Info()` 调用失败。

在 Android 上，`/storage/emulated/0` 是通过 FUSE/sdcardfs 挂载的虚拟文件系统。`os.ReadDir()` 返回的 `DirEntry` 调用 `Info()` 时需要额外 `stat()` 系统调用，对于某些文件（特别是受作用域存储保护的媒体文件），`stat()` 可能返回权限错误，导致 `entry.Info()` 失败。

当前代码在 `entry.Info()` 失败时直接 `continue`，跳过该条目——**文件被静默跳过，只留下目录**（目录的 `stat()` 通常不会失败）。

此外，`entry.IsDir()` 不需要调用 `Info()`，它直接从 `ReadDir` 结果获取。所以目录不会受影响，但文件会因为 `Info()` 失败而被跳过。

### 修复方案

**A. 后端：`entry.Info()` 失败时仍返回条目（使用降级信息）**

```go
for _, entry := range entries {
    if strings.HasPrefix(entry.Name(), ".") {
        continue
    }

    info, err := entry.Info()
    if err != nil {
        // Info() 失败时仍返回条目，使用降级信息
        slog.Warn("Failed to get file info, using fallback", "name", entry.Name(), "error", err)
        files = append(files, FileInfo{
            Name:        entry.Name(),
            Path:        filePath,
            IsDirectory: entry.IsDir(),
            Size:        0,
            Modified:    "",
        })
        continue
    }

    files = append(files, FileInfo{
        Name:        entry.Name(),
        Path:        filePath,
        IsDirectory: entry.IsDir(),
        Size:        info.Size(),
        Modified:    info.ModTime().Format(time.RFC3339),
    })
}
```

关键改动：`entry.Info()` 失败不再跳过条目，而是用 `Size: 0, Modified: ""` 降级返回。`entry.IsDir()` 和 `entry.Name()` 不依赖 `Info()`，所以即使 `Info()` 失败也能正确判断。

**B. 后端：添加 `listFiles` API 的日志**

在 `ListFiles` 中添加日志，记录返回的文件数量和跳过的条目数，便于诊断。

### 实施步骤

1. 修改 `mobile_service.go` 的 `ListFiles()`：`entry.Info()` 失败时降级返回而非跳过
2. 添加诊断日志

## 文件变更清单

| 文件 | 变更 |
|------|------|
| `app/encv-mobile/src/views/Files.vue` | 重构 `loadFiles()` 为直接请求+自动重试；监听后端就绪事件自动加载 |
| `internal/service/mobile_service.go` | `entry.Info()` 失败时降级返回；添加诊断日志 |
