# 移动端三问题修复计划

## 问题概览

| # | 问题 | 现象 | 根因 |
|---|------|------|------|
| 1 | 插件初始化错误未广播到 devlogs | 加密报错 "could not get settings for plugin text" 但 DevLogs 后端 tab 看不到 | `slog.Error` 虽然走了 `WSLogHandler`，但可能被级别过滤或广播时机问题 |
| 2 | 预览正常但加密报错 | 预览 ENCV 容器文件正常显示，但加密时报插件初始化失败 | `processEncrypt` 中 `plugin.Initialize(cfgCtx)` 重新初始化时找不到 plugin settings |
| 3 | 未知类型卡加载中 | 非 ENCV 容器、非文本文件（如图片）打开预览时一直转圈 | `FilePreview.vue` 的 loading 状态在某些分支未被正确关闭 |

---

## 问题 2 深入分析（核心 bug）

### 预览为什么正常？

`GetFileInfo()` ([mobile_service.go:246](file:///workspace/internal/service/mobile_service.go#L246)) **不调用** `plugin.Initialize()`：
- 用 `detector.DetectContainer(absPath)` 读文件字节检测容器 → 不需要 plugin settings
- 用 `reader.OpenV4Container(absPath, password)` 打开容器 → 只需要密码

### 加密为什么失败？

`TaskManager.processEncrypt()` ([task_manager.go:358](file:////workspace/internal/service/task_manager.go#L358)) 流程：

```
processEncrypt()
  → cfgCtx = getConfigForTask(task, ctx)    // 创建新 context，放入 tm.cfg
  → plugin.FindEncryptingPlugin(absPath)     // 找到 TextPlugin 实例
  → plugin.Initialize(cfgCtx)               // ❌ 重新初始化！
```

`TextPlugin.Initialize()` ([text/plugin.go:90](file:///workspace/internal/v2/plugins/text/plugin.go#L90))：

```go
func (p *TextPlugin) Initialize(ctx context.Context) error {
    if ctx == p.ctx {
        return nil  // 启动时已初始化过，p.ctx = rootCtx，如果相同则跳过
    }
    // ctx != p.ctx（因为 processEncrypt 创建了新 context）
    // ↓ 尝试重新初始化
    p.cfg = config.FromContext(ctx)
    settings, err := config.GetPluginSettingsFor[TextPluginConfig](p.cfg, p.Name())
    // ❌ p.cfg.PluginSettings["text"] 不存在 → 报错
}
```

### 为什么 `PluginSettings["text"]` 会不存在？

调用链：
1. `encv.Init(rootCtx)` → `BuildFullPluginSettings(cfg.PluginSettings)` → 填充所有插件设置 → 存回 `cfg.PluginSettings`
2. `InitializePlugins(rootCtx)` → `p.Initialize(rootCtx)` → 成功（此时有 settings）
3. `NewServer(rootCtx)` → `config.FromContext(rootCtx)` → 同一个 `*Config` 指针 → 传给 `NewTaskManager`
4. 任务执行 → `getConfigForTask` → `config.NewContext(ctx, tm.cfg)` → `tm.cfg` 就是步骤 1-3 的同一个对象

**理论上** `tm.cfg.PluginSettings["text"]` 应该存在。但如果 `encv.Init` 的返回值被忽略且内部出错（如 `BuildFullPluginSettings` 失败），则 `cfg.PluginSettings` 可能不完整。

**关键代码** [servers.go:20](file:///workspace/cmd/encv/servers.go#L20)：`encv.Init(rootCtx)` 返回值被**完全忽略**。

---

## 修复方案

### Fix 1: 插件错误广播到 DevLogs

**文件**: [task_manager.go:653-675](file:///workspace/internal/service/task_manager.go#L653-L675)

当前 `failTask` 已有 `slog.Error` + `broadcaster.Broadcast`，但需要确认：
1. `WSLogHandler` 的 `wsMinLevel` 是否包含 `LevelError`
2. 广播消息格式是否与前端 DevLogs 期望的一致

**修复**：在 `failTask` 中额外通过 `Broadcaster` 直接发送结构化的 log 类型消息（与 WSLogHandler 格式一致），确保 DevLogs 能收到。

同时检查 [server.go:139](file:///workspace/internal/server/server.go#L139) 处 `wsMinLevel` 的值。

### Fix 2: 加密流程插件初始化失败（核心修复）

**根因**: `processEncrypt` / `processDecrypt` 中重复调用 `plugin.Initialize(cfgCtx)` 时，新的 context 导致缓存失效，重新读取 settings 失败。

**方案 A（推荐）**: 在 `getConfigForTask` 中确保 PluginSettings 完整

**文件**: [task_manager.go:349-356](file:////workspace/internal/service/task_manager.go#L349-L356)

```go
func (tm *TaskManager) getConfigForTask(task *MobileTask, ctx context.Context) context.Context {
    if task.Password != "" {
        cfgCopy := *tm.cfg
        cfgCopy.Password = task.Password
        return config.NewContext(ctx, &cfgCopy)
    }
    // 确保 PluginSettings 已填充（防止 encv.Init 失败时丢失）
    if len(tm.cfg.PluginSettings) == 0 {
        fullSettings, _ := plugins.BuildFullPluginSettings(nil)
        tm.cfg.PluginSettings = fullSettings
    }
    return config.NewContext(ctx, tm.cfg)
}
```

**方案 B（更彻底）**: 避免重复 Initialize — 如果插件已初始化过，跳过

**文件**: [task_manager.go:408-417](file:////workspace/internal/service/task_manager.go#L408-L417) 和 [task_manager.go:597-606](file:////workspace/internal/service/task_manager.go#L597-L606)

```go
// processEncrypt 中：移除重复的 plugin.Initialize 调用
// 因为 encv.Init → InitializePlugins 已经初始化过所有插件了
// plugin.Initialize(cfgCtx)  // ← 删除这行
```

**推荐方案 A + B 组合**: 既保底填充 settings，又避免不必要的重复初始化。

### Fix 3: 未知类型卡加载中

**文件**: [FilePreview.vue:203-295](file:///workspace/app/encv-mobile/src/views/FilePreview.vue#L203-L295)

分析 `loadFile()` 的 loading 状态管理：

```typescript
// 当前代码流程：
async function loadFile() {
  loading.value = true          // ✅ 开始加载
  try {
    if (isEncrypted === 'true') {
      // 分支 A: 加密文件
      const info = await fetchFileInfo(...)
      if (info.is_encv_container && info.container) {
        previewType.value = ...   // 根据 container_type 设置
      } else {
        previewType.value = 'unsupported'  // ← 不是 ENCV 容器
      }
    } finally { loading.value = false }  // ✅ finally 关闭 loading
  }

  // 分支 B: 非加密文件
  previewType.value = await determinePreviewType(...)
  } catch (e) { ... }
  finally { loading.value = false }       // ✅ finally 关闭 loading
}
```

**看起来** finally 应该能关闭 loading。但需要检查以下边界情况：

1. **`determinePreviewType()` 内部调用 `fetchTextPreviewExts()`** ([encv.ts:352](file:///workspace/app/encv-mobile/src/api/encv.ts#L352)) — 如果此 API 请求挂起（服务器不可达、超时），整个 `loadFile()` 会卡住
2. **`fetchFileInfo` API 返回异常状态** — 如果 `/api/file/info` 对某些文件类型返回非预期响应

**修复**:
1. 给 `fetchTextPreviewExts()` 加超时保护（已在 api 层面加了 AbortController？需要确认）
2. 在 `determinePreviewType()` 中加 try/catch，失败时 fallback 到 `'unsupported'`
3. 确认所有路径都有 `finally { loading.value = false }`

---

## 实施步骤

1. **Fix 2 先行**（最关键的加密失败问题）
   - 修改 `task_manager.go` 的 `getConfigForTask` 保底填充 PluginSettings
   - 同时考虑移除 `processEncrypt` / `processDecrypt` 中的冗余 `plugin.Initialize` 调用
   - 编译验证

2. **Fix 1**（DevLogs 广播）
   - 检查 `wsMinLevel` 配置
   - 在 `failTask` 中增加显式的 log broadcast
   - 编译验证

3. **Fix 3**（未知类型加载）
   - 检查 `fetchTextPreviewExts` 是否有超时
   - 在 `determinePreviewType` 加 fallback
   - `vue-tsc --noEmit && vite build` 验证

4. **端到端测试**
   - 启动移动端服务
   - 测试加密文本文件 → 验证不再报 plugin settings 错误
   - 测试预览 ENCV 容器 → 验证仍然正常
   - 测试打开未知类型文件 → 验证不再卡加载
   - 检查 DevLogs → 验证错误信息可见
