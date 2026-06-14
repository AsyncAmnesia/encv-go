# ENCV 跨进程 IPC 重构 Spec

> **核心原则**：parent ↔ child 协调**只用 HTTP localhost**（行业标准：Android Studio、VS Code、Firebase CLI）。**禁止**共享文件 mtime、**禁止**双向改写配置文件、**禁止**进程间 env var 协商路径。
>
> **完整行业对比 + 实战案例**：[详情文档](./rationale.md)

---

## 一、现状：5 个耦合源 — 到底烂在哪

当前 Kotlin (Android) 与 Go subprocess 的协调是**一坨多源耦合**：

| # | 耦合点 | 写入方 | 读取方 | 问题 |
|---|--------|--------|--------|------|
| 1 | `config.user.json` 的 `mobile.server.dir` 字段 | **Kotlin 写**（降级时）| **Go 读**（mobile overlay） | Kotlin 必须知道 Go 的 config schema，Kotlin 反向修正 Go 的运行时路径 |
| 2 | `ENCV_SERVING_DIR` env var | Kotlin set | Go 读 | Go 内部已能用 `os.Getwd()` 推断，env var 是冗余 |
| 3 | `ENCV_HEARTBEAT_PATH` env var | Kotlin set | Go 读 | 路径本应在 Kotlin 端算好，Go 不需要知道 |
| 4 | `.encv_heartbeat` 文件 mtime | Go 写（每 2s）| Kotlin 1s poll | 共享文件系统依赖，FAT32/exFAT mtime 精度 2s 引发误判 |
| 5 | `/health` HTTP 端点 | Go 暴露 | Kotlin 1s GET | ✅ **这个是对的** |

**实际触发过的 bug**（2026-06-14 复盘）：

1. **路径不同步 bug**：[server.go:229](file:///workspace/internal/server/server.go#L229) `os.Setenv("ENCV_HEARTBEAT_PATH", servingDir/.encv_heartbeat)` **无条件**覆盖 Kotlin set 的 env → 两进程读不同文件 → Kotlin 8s 误判 hang → 杀 Go → WS 7s 必死
2. **mtime 精度 bug**：Android 共享存储 (FAT32/exFAT) mtime 精度 2s → 同一秒多次写 mtime 不变 → Kotlin 看到的"陈旧时间"虚高
3. **scoped storage bug**：真机用户拒存储权限 → `/storage/emulated/0` 不可写 → Go 写心跳失败 → Kotlin mtime 永远 0 → 误判 hang
4. **fallback 链乱**：Kotlin 端 4 层 fallback、Go 端 3 层 fallback、config 端 2 层 fallback，任何一处对不上就崩

**根因**：违反了"single source of truth"原则。Go 进程知道的所有事实（端口、servingDir、pid）应该由 Go 自己声明，Kotlin 只消费。

---

## 二、行业标准方案对比

### 2.1 调研的标杆项目

| 项目 | parent ↔ child 协调方式 | 关键文件 |
|------|------------------------|---------|
| **Android Studio** ↔ Gradle Daemon | HTTP localhost + 1s health poll | `~/.gradle/daemon/<version>/registry.bin`（可选，daemon 主动写） |
| **VS Code** ↔ TypeScript Language Server | stdio pipe + LSP 协议（JSON-RPC） | 无文件 |
| **Firebase CLI** ↔ Emulator | HTTP localhost + `--host-addr` flag | `~/.cache/firebase/emulators/.../*.json`（emulator 自己写） |
| **Docker CLI** ↔ dockerd | Unix domain socket + REST API | 无文件 |
| **Flutter CLI** ↔ Dart VM Service | HTTP localhost + WebSocket（observatory 协议） | 无文件 |
| **Termux** ↔ proot-distro | Unix domain socket | 无文件 |
| **OpenAPI / gRPC over localhost** | HTTP/2 + protobuf | 无文件 |

**共同模式**：
- ✅ 单一传输通道（HTTP localhost / Unix socket / stdio pipe）
- ✅ child 端**主动声明**自己的状态（端口、pid、metadata）→ parent **只读**
- ❌ 绝无"parent 写文件让 child 读"或"child 写文件让 parent 读"的反模式
- ❌ 绝无"env var 协商共享路径"的反模式

### 2.2 本项目内已有先例

**[app/preview-gateway/src/children.ts#L46-71](file:///workspace/app/preview-gateway/src/children.ts#L46-L71) 已实现 HTTP readiness 探活**：
- `pingHttp(url, 1500ms)` — 1.5s 超时 GET，返回 2xx/3xx/4xx 算 alive
- `waitForReady()` — 500ms 间隔轮询直到 ready
- 失败即崩 gateway（让 pm2 重启整套）

**Kotlin EncvGoService 应该**复用同款模式**，不是发明新机制。

---

## 三、目标架构：HTTP-only IPC + Go 主动声明

### 3.1 单一传输通道

```
┌─────────────────┐                              ┌─────────────────┐
│   parent        │                              │     child        │
│  (Kotlin / Node) │  HTTP localhost:2025/ws      │  encv-go (Go)    │
│                 │ ──────────────────────────────▶  ① HTTP API     │
│  ┌────────────┐ │  GET /health                  │  ② WebSocket /ws│
│  │ 探活循环    │ │  GET /api/runtime (NEW)      │  ③ 主动声明状态  │
│  │ 1s 间隔    │ │  WS  /ws 推送                 │                 │
│  └────────────┘ │                               │                 │
└─────────────────┘                               └─────────────────┘
        │                                                │
        │  ★ 单向：parent → child 只发 HTTP 请求         │
        │  ★ 单向：child → parent 只通过 HTTP/WS 响应     │
        │  ★ 零文件依赖，零 env var 协商路径               │
```

### 3.2 新增端点：`GET /api/runtime`

Go 服务启动并就绪后，**主动**在内存中持有运行时信息，HTTP 端点返回 JSON：

```go
// internal/server/runtime_api.go (NEW)
type RuntimeInfo struct {
    PID         int    `json:"pid"`           // 进程 pid
    Version     string `json:"version"`       // 构建版本
    ServingDir  string `json:"serving_dir"`   // Go 实际在用的 serving dir
    Port        int    `json:"port"`          // 监听端口
    StartedAt   int64  `json:"started_at"`    // 启动时间（Unix ms）
    Mobile      bool   `json:"mobile"`        // 是否 mobile overlay
    ConfigPath  string `json:"config_path"`   // 加载的 config 路径
    HeartbeatOK bool   `json:"heartbeat_ok"`  // 心跳机制状态
}

func (s *Server) handleRuntimeAPI(w http.ResponseWriter, r *http.Request) {
    s.runtimeInfoMu.RLock()
    defer s.runtimeInfoMu.RUnlock()
    json.NewEncoder(w).Encode(s.runtimeInfo)
}
```

**端点契约**：

| 字段 | 用途 | 谁来用 |
|------|------|--------|
| `pid` | parent 记录用，可选 | 调试 |
| `serving_dir` | **核心**：parent 想知道 Go 在哪写文件 | Kotlin 写入文件时可对齐路径 |
| `port` | 备用 | Kotlin 备用（主要靠 `waitForReady` 探活）|
| `started_at` | 计算进程年龄 | 调试 |
| `mobile` | 区分 mobile / desktop | 调试 |
| `config_path` | 调试 | 调试 |
| `heartbeat_ok` | 心跳机制是否启用 | 调试 |

### 3.3 心跳机制改写：内存中（不再写文件）

**现状**（[worker_client.go#L248-256](file:///workspace/internal/utils/ffmpeg/worker_client.go#L248-L256)）：

```go
func writeHeartbeat() {
    path := os.Getenv("ENCV_HEARTBEAT_PATH")
    if path == "" { return }
    _ = os.WriteFile(path, ...)  // 写文件
}
```

**目标**：心跳移入 `Server.runtimeInfo` 内存字段。

```go
// internal/server/server.go
type Server struct {
    // ...
    runtimeInfo   RuntimeInfo
    runtimeInfoMu sync.RWMutex
    lastHeartbeatMs int64  // atomic.Int64
}

// StartHeartbeatLoop(ctx) 不再写文件，改写内存
func StartHeartbeatLoop(ctx context.Context) {
    ticker := time.NewTicker(2 * time.Second)
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C:
            atomic.StoreInt64(&s.lastHeartbeatMs, time.Now().UnixMilli())
        }
    }
}

// /api/runtime 读取 lastHeartbeatMs
func (s *Server) handleRuntimeAPI(w, r) {
    hb := atomic.LoadInt64(&s.lastHeartbeatMs)
    info := s.runtimeInfo
    info.HeartbeatOK = time.Since(time.UnixMilli(hb)) < 30*time.Second
    json.NewEncoder(w).Encode(info)
}
```

**/health 端点**同时返回心跳状态：

```go
func (s *Server) handleHealth(w, r) {
    hb := atomic.LoadInt64(&s.lastHeartbeatMs)
    ageMs := time.Since(time.UnixMilli(hb)).Milliseconds()
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "status":          "ok",
        "heartbeat_age_ms": ageMs,
        "heartbeat_ok":    ageMs < 30_000,
    })
}
```

### 3.4 Kotlin 端：只调 HTTP，零文件依赖

**`EncvGoService.kt` 重写后**：

```kotlin
// ❌ 删除：resolvedHeartbeatFile() / resolveServingDir() / probeDirWritable() / 
//         readMobileServerDirFromConfig() / updateMobileServerDirInConfig()
// ❌ 删除：ENCV_HEARTBEAT_PATH env
// ❌ 删除：ENCV_SERVING_DIR env
// ❌ 删除：heartbeatFile 字段

// ✅ 保留并简化：startGoProcess
private fun startGoProcess(source: String, command: String?) {
    // ... 启动 binary ...
    goProcess = ProcessBuilder(binary.absolutePath, "start").apply {
        environment()["ENCV_CONFIG_PATH"] = configPath
        environment()["ENCV_MOBILE"] = "1"
        environment()["HOME"] = filesDir.absolutePath
        environment()["ENCV_LIB_DIR"] = applicationInfo.nativeLibraryDir
        environment()["ENCV_FFMPEG_WORKER"] = File(nativeLibraryDir, "libffmpeg-worker.so").absolutePath
        // ❌ 删除 ENCV_HEARTBEAT_PATH、ENCV_SERVING_DIR
        redirectErrorStream(true)
        directory(filesDir)
    }.start()
    
    // 探活：1s 间隔 GET /health，无文件 mtime 依赖
    startHttpLivenessMonitor()
}

// ✅ 新：startHttpLivenessMonitor（仿 preview-gateway/children.ts:waitForReady）
private fun startHttpLivenessMonitor() {
    monitorExecutor.scheduleWithFixedDelay({
        val proc = goProcess ?: return@scheduleWithFixedDelay
        if (!proc.isAlive) { 
            handleExit(proc)
            return@scheduleWithFixedDelay
        }
        if (!processReady.get()) return@scheduleWithFixedDelay
        
        // 单 HTTP GET，1s 超时
        val healthOk = checkHealth(currentPort)  // 已有
        val heartbeatOk = checkHeartbeatOk(currentPort)  // 新增：解析 /health JSON
        
        if (!heartbeatOk && processReady.get()) {
            handleHang("heartbeat_stale_via_http")
        }
    }, 1, 1, TimeUnit.SECONDS)
}

private fun checkHeartbeatOk(port: Int): Boolean {
    return try {
        val conn = URL("http://127.0.0.1:$port/health").openConnection() as HttpURLConnection
        conn.connectTimeout = 1000
        conn.readTimeout = 1000
        val response = conn.inputStream.bufferedReader().readText()
        val json = JSONObject(response)
        json.optBoolean("heartbeat_ok", false)
    } catch (e: Exception) {
        false
    }
}
```

### 3.5 删除的耦合点清单

| 删除项 | 原因 |
|--------|------|
| `ENCV_HEARTBEAT_PATH` env var | Go 不再需要知道 Kotlin 的心跳文件路径 |
| `ENCV_SERVING_DIR` env var | Go 内部 `os.Getwd()` + mobile overlay 足够 |
| `heartbeatFile: File by lazy` 字段 | 改用 HTTP 探活 |
| `resolvedHeartbeatFile()` 方法 | 同上 |
| `resolveServingDir()` 方法 | Kotlin 不再决定 servingDir，Go 启动后通过 `/api/runtime` 告诉 Kotlin |
| `probeDirWritable()` 方法 | 同上 |
| `readMobileServerDirFromConfig()` 方法 | 同上 |
| `updateMobileServerDirInConfig()` 方法 | 同上（**不修改** config.user.json.mobile.server.dir）|
| `ffmpeg.touchHeartbeatFile()` 函数 | 改为写内存 |
| `ffmpeg.writeHeartbeat()` 写文件 | 改为写内存 |
| `StartHeartbeatLoop` 启 goroutine 写文件 | 改为启 goroutine 写内存 |

### 3.6 保留的耦合点

| 保留项 | 原因 |
|--------|------|
| `ENCV_CONFIG_PATH` env | 显式指向 config 文件，Kotlin 拥有 |
| `ENCV_MOBILE=1` env | Go 行为开关 |
| `HOME` env | Go 找 user home |
| `ENCV_LIB_DIR` env | Kotlin 显式知道 native lib 位置 |
| `ENCV_FFMPEG_WORKER` env | 同上 |
| `ENCV_CONFIG_PATH` 文件本身 | 配置单一来源（Kotlin 拥有） |

---

## 四、跨平台 + 沙箱 dev 要求

### 4.1 支持矩阵

| 平台 | child 启动方式 | parent 协调方式 | 状态 |
|------|---------------|----------------|------|
| **Android 真机** (vivo V2362A 等) | `ProcessBuilder(binary, "start")` | Kotlin 1s HTTP `/health` | 必须支持 |
| **Android 沙箱** (preview-gateway) | preview-gateway `child_process.spawn` | gateway 1s HTTP `/health` | 必须支持 |
| **Linux dev** (本地 Go 直接跑) | `go run ./cmd/encv/ serve` | 用户手动 `curl :2025/health` | 必须支持 |
| **macOS dev** | 同上 | 同上 | 必须支持 |
| **Windows dev** | 同上 | 同上 | 必须支持 |
| **CI** | `go test ./internal/...` | 不需要 runtime IPC | N/A |

### 4.2 沙箱 dev 兼容：preview-gateway 不受影响

`app/preview-gateway/src/children.ts` 已经用 HTTP `/health` 探活，重构后**无需任何修改**。Kotlin EncvGoService 与 preview-gateway 行为一致。

### 4.3 Desktop dev 兼容：standalone Go 不变

桌面端 `go run ./cmd/encv/ serve` 启动后，浏览器访问 `http://127.0.0.1:2025/health` 仍返回 200，心跳信息附加在 JSON 里。**API 行为兼容**，只是 `/health` 响应多了字段。

### 4.4 真机权限兼容：scoped storage 不再影响 IPC

之前 Kotlin 必须探测 `/storage/emulated/0` 是否可写（scoped storage 拒权限就挂），重构后 **Kotlin 不再触碰 servingDir**：
- Kotlin 启动 Go 时只设最少的 env
- Go 自己用 `os.Getwd()` = `filesDir`（Kotlin `ProcessBuilder.directory(filesDir)`）+ mobile overlay = `/storage/emulated/0`
- 如果 `/storage/emulated/0` 不可写，Go 自己用 `os.MkdirAll` fallback 到 `filesDir`
- Kotlin 通过 `/api/runtime` 读 Go **实际在用**的 `servingDir`，需要写文件时对齐这个路径

**关键收益**：Kotlin 不再决定 servingDir，scoped storage 权限问题从"Kotlin 必须处理"变成"Go 自己处理"。

---

## 五、变更影响面

### 5.1 Affected Code

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| `internal/server/server.go` | 修改 | 加 `runtimeInfo` 字段、`handleRuntimeAPI` 路由 |
| `internal/server/runtime_api.go` (新) | 新增 | `/api/runtime` 端点实现 |
| `internal/utils/ffmpeg/worker_client.go` | 修改 | `writeHeartbeat` 改写内存；删除 `touchHeartbeatFile` |
| `internal/utils/ffmpeg/heartbeat_test.go` | 修改 | 改为测试内存字段 |
| `app/encv-mobile/android/.../EncvGoService.kt` | 大改 | 删除 6 个方法、2 个字段、2 个 env 变量；新增 `checkHeartbeatOk` |
| `app/encv-mobile/android/.../EncvGoServiceTest.kt` (新) | 新增 | 单元测试 mock `URL.openConnection` |

### 5.2 不需要改

- `app/preview-gateway/src/children.ts` — 已经 HTTP-only，无需改
- 前端 `useRealtimeTransport.ts` — 只调 HTTP API，行为不变
- `config.user.json` — `mobile.server.dir` 保留为 `/storage/emulated/0`（默认值），Kotlin 不再修改

### 5.3 受影响的功能行为

| 行为 | 之前 | 之后 |
|------|------|------|
| Go 启动后 Kotlin 多久能感知 ready | 等 Kotlin mtime poll 第 1 次（最多 1s） | Kotlin HTTP `/health` 1s poll |
| 心跳超时检测 | Kotlin 读文件 mtime | Kotlin 读 HTTP `/health` JSON 字段 |
| 桌面 dev `curl /health` | 返回简单 text | 返回 JSON 多了 `heartbeat_ok` 等字段（**向前兼容**，前端用 `code==200` 判断） |
| 真机 scoped storage 拒权限 | Kotlin 必须 probe + 改 config + 降级 | Go 自己 fallback，Kotlin 不管 |
| 沙箱 dev 启动 | preview-gateway HTTP poll 正常工作 | preview-gateway HTTP poll 正常工作（**完全不变**）|

---

## 六、风险与回滚

| 风险 | 概率 | 影响 | 回滚方案 |
|------|------|------|---------|
| `/health` 端点 1s HTTP 性能开销 | 极低 | 1 RPS，<1ms | 改回 2s 间隔 |
| Kotlin 1s HTTP 探活在弱网下抖动 | 中 | 偶发误判 hang | 增加连续失败 N 次才判 hang（与现有 `restartAttempts > 0` 类似）|
| 真机 WebView 拦截 localhost HTTP | 低 | WebView 默认允许 localhost | 检查 WebView 客户端 allowlist |
| Go 端 `runtimeInfo` 并发读写 | 低 | 有 mutex 保护 | 加 atomic.Int64 |
| `/api/runtime` 信息泄露 servingDir 路径 | 低 | 内部 API，不暴露公网 | 网络隔离（`127.0.0.1` only）|

---

## 七、验收标准

### 7.1 功能验收

- [ ] Kotlin EncvGoService 启动 Go 后 1s 内能感知 ready
- [ ] Kotlin 1s HTTP `/health` poll 工作正常，连续 8s 失败才判 hang
- [ ] 真机 `/storage/emulated/0` 不可写时，Kotlin 不再需要 probe/降级
- [ ] 沙箱 dev preview-gateway 启动流程不变（HTTP `/health` poll 已经 work）
- [ ] Desktop dev `curl :2025/health` 返回 JSON 包含 `heartbeat_ok` 字段

### 7.2 代码验收

- [ ] EncvGoService.kt 净减 ≥ 100 行（删除 6 个方法 + 2 个 env + 2 个字段）
- [ ] ffmpeg/worker_client.go 净减 ≥ 30 行（删除文件 mtime 逻辑）
- [ ] 新增 `internal/server/runtime_api.go` ≤ 50 行
- [ ] Go 单测全过 + Kotlin 编译通过

### 7.3 回归测试

- [ ] 真机 WebSocket 30 分钟长稳（不出现 7s 必死）
- [ ] 沙箱 dev：vite + encv-go 同时启动，gateway HTTP poll 全部 ready
- [ ] 桌面 dev：`go run ./cmd/encv/ serve` + `curl :2025/health` 正常

---

## 八、相关文档

- [rationale.md](./rationale.md) — 行业对比详细分析（Android Studio / VS Code / Firebase CLI / Docker）
- [checklist.md](./checklist.md) — 实施检查清单
- [tasks.md](./tasks.md) — 实施任务分解

---

## 九、引用

- 现状 bug 分析：[`/workspace/.trae/documents/backend-crash-websocket-1006-fix.md`](../documents/backend-crash-websocket-1006-fix.md)
- 行业先例：[`/workspace/app/preview-gateway/src/children.ts`](file:///workspace/app/preview-gateway/src/children.ts)
- 当前耦合代码：[`/workspace/app/encv-mobile/android/app/src/main/java/com/encvgo/app/EncvGoService.kt`](file:///workspace/app/encv-mobile/android/app/src/main/java/com/encvgo/app/EncvGoService.kt#L121-L236)
