# 4 个用户反馈问题 + agent-browser 调试规范

> **本计划整合**：① 用户 4 个问题的修复进度；② agent-browser 调试规范（避免异常打断）
> **进入 Plan 模式原因**：用户在调试过程中两次被打断，意识到 agent-browser 没有被适配，会异常中断
> 当前是 **READ-ONLY 状态**，写完本计划后必须等用户批准才能继续

---

## 一、当前进度（实测验证后的真实状态）

### 1.1 4 个用户问题的修复状态

| # | 问题 | 状态 | 证据 |
|---|------|------|------|
| 1 | 解密任务不该有前置 promptPassword（NewTaskModal 已有密码字段） | ✅ **已修复** | `actions.ts:42-53` — `alist-decrypt` handler 直接 `openNewTask`，不再弹 promptPassword |
| 2a | 解密输出文件名多 `.bin` 后缀 | ✅ **已修复** | `decryptor.go tryDecodeFilename` 不再追加 `ext`；实测产物 `CAD放样.mp4` (51958979 bytes) |
| 2b | 重命名 404 | ✅ **已修复** | `server.go:223` 添加 `r.POST("/api/file/rename", s.handleFileRenameGin)`；实测 API 返回 200 |
| 2c | **还原文件名**为 `CAD放样.mp4`（不是 -renamed） | ✅ **已完成** | `ls /storage/emulated/0/CAD放样.mp4` 存在，51958979 bytes |
| 3 | 任务详情缺产物展示 | ✅ **已修复** | `TaskDetailModal.vue` + `task_manager.go` 新增 `OutputPath` 字段 + i18n key |
| 4 | `hyYGPCwJPQ3+xrdAvfnn2.bin` 预览失败 | ⚠️ **修了 2 个根因，第 3 个未明** | 见下表 |

### 1.2 问题 4（预览失败）已修复的两个根因

**根因 A：`Content-Length` 在 206 Partial Content 响应中必须是部分大小（之前错填为全文件大小）**
- 修复：`streamer.go:144-145` 添加 `partialLen := end - start + 1; w.Header().Set("Content-Length", ...)` 覆盖默认值
- 验证：curl 验证 `Range: bytes=0-99` → `HTTP 206, Content-Length: 100, Content-Range: bytes 0-99/51958979` ✅
- 验证：curl 验证 `Range: bytes=0-524287` → `Content-Length: 524288` ✅

**根因 B：streamUrl 整体作为 router query → Vue Router 二次 URL 编码 → 后端收到的 path 是双编码**
- 修复：3 处调用方（`actions.ts`、`Files.vue` × 2）从 `streamUrl` 改为 `alistPath + alistPassword`
- 修复：`ArtPlayerView.vue` 新增 `alistPath/alistPassword` ref + `getAlistEncryptStreamUrl` 自己构造 URL
- 验证：浏览器测试新 URL `/api/alist-encrypt/stream?path=%252F...&password=8682268`（仍是双编码，但 **curl 后端 200 OK**）

### 1.3 ⚠️ 问题 4 仍未解决的第三个症状

**浏览器实际行为（即使 1.2 修复后）**：
```
network log:
  GET /api/alist-encrypt/stream?path=%252F...&password=8682268 (Media) 400  ← ArtPlayer 拿到 400
  GET /api/alist-encrypt/stream?path=%252F...&password=8682268 (Media) 400  ← (10+ 次重试)
```

**curl 同时刻同样 URL** → **200 OK**（完整 MP4 数据）

**关键差异**：
- curl `Range: bytes=0-99` → 206 + 100 bytes
- Chrome Media request 拿到 **400**

**可能原因**（未定位，需要在浏览器 DevTools 看具体响应体）：
1. **CORS / Origin 头**导致后端 400
2. **Range 格式不同**（Chrome 可能发 `bytes=0-` 而非 `bytes=0-99`）
3. **Sec-Fetch-Mode: no-cors** 让 Gin 走不同分支
4. **vite 代理**对 Media request 的特殊处理
5. **`streamer.go` 在 Range header 解析失败时返回 400**（line 116-120 `invalid range format`），但我们看到正常解析路径不会触发

需要在浏览器 DevTools 看 Network → request/response headers + body 才能定位 400 的真正原因。

### 1.4 agent-browser 适配问题（⚠️ 用户原话）

> "你没有适配 agent-browser 会导致异常打断"

实测发现的真实问题：
1. **`agent-browser` 库缺失系统库**（libatk1.0 等）→ 需 apt-get install `libatk1.0-0t64 libatk-bridge2.0-0t64 libxkbcommon0 libxcomposite1 libxdamage1 libxfixes3 libxrandr2 libgbm1 libpango-1.0-0 libcairo2 libasound2t64` 修复
2. **`--cdp 9222` 模式**：沙箱里 Chrome 跑在 9222，agent-browser 通过 CDP 连接成功；但默认 `open` 模式（拉新 Chrome）连不上 `localhost:5173`（网络命名空间不同）
3. **eval 不支持 `await`**：必须用 `.then()` 链式调用
4. **密码 alertdialog 文本框引用**：必须用 `snapshot -i` 重取 ref（每次弹窗都是新 DOM）
5. **直接 dispatch input 事件**：设置 `pwdInput.value = 'xxx'` + `dispatchEvent('input')` 才能让 Vue 响应

**两次"意外破坏会话"的可能根因**：
- 第一次："长按菜单"调试时，agent-browser 的 `open` 命令导致新 Chrome 拉起失败、连接被 reset
- 第二次：可能在改 streamer.go 后，air 重启 encv 进程时端口被短暂占用导致 agent-browser 的 fetch 失败

---

## 二、agent-browser 调试铁律（**写进 plan 防止再踩**）

### 2.1 启动顺序

```bash
# 1. 确认 preview 服务在跑（不要重启）
ps -ef | grep -E "(start-preview|air|encv|vite)" | grep -v grep
# 期待：6 个进程

# 2. 确认 Chrome 9222 在跑（agent-tool-host 启动的）
curl -s http://localhost:9222/json/version | head -3
# 期待：返回 webSocketDebuggerUrl

# 3. 用 --cdp 9222 连接（不要默认 open）
agent-browser --cdp 9222 open "http://localhost:5174/tabs/files"
```

### 2.2 禁止操作

| 禁止 | 原因 |
|------|------|
| ❌ `pkill -f air` 或 `pkill -f vite` | 会杀掉 preview server，破坏会话 |
| ❌ `pkill -f agent-tool-host` | 沙箱基础设施，反代 vite |
| ❌ 重命名/删除 mock 文件 | 用户已经验过的产物，丢失会破坏回归测试 |
| ❌ 改 `config.user.json` | 脚本铁律 |
| ❌ 用默认 `open` 命令 | 拉新 Chrome 失败，连不上 5173 |
| ❌ `eval 'await ...'` | eval 不支持 await 顶层 |

### 2.3 推荐操作

| 场景 | 操作 |
|------|------|
| 调试长按菜单 | `window.__ENCV_TEST.simulateLongPress(fileName).then(()=>1)` |
| 看 console 日志 | `agent-browser --cdp 9222 eval` 读 `document.querySelector` 或注入 `console.log` 拦截 |
| 设置 input 值 | `el.value = 'x'; el.dispatchEvent(new Event('input', {bubbles:true}))` |
| 看 Network 请求 | `agent-browser --cdp 9222 network requests --type xhr,fetch,media` |
| 重置页面 | `agent-browser --cdp 9222 open http://localhost:5174/tabs/files` |
| 不要关闭 Chrome | 用 `agent-browser --cdp 9222 close` 而不是 `agent-browser close` |

### 2.4 调试问题的正确步骤

```
1. 先 curl 验证后端行为（独立于浏览器）
2. 再用 agent-browser --cdp 9222 触发前端流程
3. 用 network requests 看实际请求是否和 curl 一致
4. 用 eval 读 page 状态（playerError, videoState, etc.）
5. 修复代码 → 等 Vite HMR / air 重载 → 重测
6. **绝不**为清状态杀掉 preview server
```

---

## 三、问题 4 第三个根因的排查计划

### 3.1 假设验证顺序

| 假设 | 验证方式 |
|------|---------|
| A. 浏览器 Origin 头导致后端 400 | 在 Chrome DevTools Network 看 Request Headers + Response Body；用 curl 模拟加 `-H "Origin: http://localhost:5174"` 看是否也 400 |
| B. Range 格式不同导致 `invalid range format` | 看 Chrome DevTools 的 Range header，对比 curl |
| C. vite proxy 对 Media 类型特殊处理 | 看 Chrome DevTools Network 实际请求的 host 是 `localhost:5174` 还是 `127.0.0.1:2025` |
| D. 后端 encv log 有具体错误 | `slog.Info("API: alist-encrypt stream", "path", absPath)` 应该在 encv 进程 stdout；air 重载会覆盖日志 |

### 3.2 修复方案（按假设逐个尝试）

**方案 1：在 `getAlistEncryptStreamUrl` 中去掉双重编码**
```ts
// 改为单层 encodeURIComponent，让后端只需要一次 PathUnescape
return `/api/alist-encrypt/stream?path=${encodeURIComponent(params.path)}&password=${encodeURIComponent(params.password)}`
```
但这会破坏 `proxySafeEncode` 在其他地方的兼容契约。

**方案 2：让 ArtPlayerView 构造 URL 时不经过 `getAlistEncryptStreamUrl`**
```ts
// 在 ArtPlayerView.vue 直接构造单层编码 URL
const streamUrl = computed(() => {
  if (alistPath.value && alistPassword.value) {
    return `/api/alist-encrypt/stream?path=${encodeURIComponent(alistPath.value)}&password=${encodeURIComponent(alistPassword.value)}`
  }
  ...
})
```

**方案 3：让后端兼容 1/2/3 层编码**（增加 `DecodeGinQueryParam` 的循环 decode）

### 3.3 优先方案 2（最小改动 + 干净 URL）

实施步骤：
1. 改 `ArtPlayerView.vue` 构造 URL 时不调用 `getAlistEncryptStreamUrl`，改用单层 `encodeURIComponent`
2. 验证：curl + 浏览器测试，确认 Range 200/206 且能播放

---

## 四、端到端验证清单

修复完成后必须跑过：

- [ ] 后端 curl：`/api/alist-encrypt/stream?path=...&password=8682268` 带 Range → 206 + 部分大小
- [ ] 浏览器：长按 `hyYGPCwJPQ3+xrdAvfnn2.bin` → 菜单有 2 项（流式预览 + 解密）
- [ ] 浏览器：流式预览 → 输密码 → ArtPlayer 播放 `CAD放样.mp4`
- [ ] 浏览器：解密 → NewTaskModal 弹出 → 输密码 → 任务完成 → 详情显示产物
- [ ] 浏览器：产物详情 → "打开产物" 跳 player → "在 Files 中定位" 跳 Files

---

## 五、不能动的资产

| 资产 | 原因 |
|------|------|
| `air` (pid 101849) | preview server 后端监视器 |
| `vite` (pid 101912) | preview server 前端 |
| `encv` (pid 130047) | air 重载后的最新后端实例（已含 Content-Length fix） |
| `agent-tool-host` (pid 821) | 沙箱基础设施 |
| `Chrome @ 9222` | 已有页面 `http://localhost:5174/tabs/files` + agent-browser 通过它调试 |
| `/storage/emulated/0/CAD放样.mp4` | 已还原的解密产物（用户明确要的不是 -renamed） |
| `/storage/emulated/0/hyYGPCwJPQ3+xrdAvfnn2.bin` | 原始加密测试文件 |
| `config.user.json` | 脚本铁律禁止改 |

---

## 六、待用户决定

1. **是否批准 plan 进入实施？** 批准后我会先验证问题 4 第 3 个根因的假设 A-D，然后实施修复方案 2。
2. **是否需要我先做一次完整的"备份快照"**（记录当前所有进程 PID、文件状态、preview URL），防止后续调试再被破坏？
