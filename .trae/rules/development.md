# 开发环境铁律（来自实战踩坑）

> **核心原则：开发环境必须与生产环境保持一致的运行路径。**
> **Mock 是技术债务的源头——宁可多花 5 分钟启动真实后端，也不要花 2 天调试 mock 与真实 API 的行为差异。**

---

## 一、严禁 mock 大量 handle（违反 = 严重错误）

### 1.1 数量红线

| Mock 端点数量 | 判定 | 后果 |
|-------------|------|------|
| 2-5 个 | ✅ 允许 | 仅覆盖前端开发阻塞点（如登录态、基础配置） |
| 6-10 个 | ⚠️ 警告 | 必须附迁移计划，限期替换为真实 API |
| **> 10 个** | **❌ 违规** | **立即重构或删除** |

### 1.2 禁止实现的 mock 逻辑

**以下业务逻辑禁止在 mock 中实现**：

| 禁止 mock 的逻辑 | 原因 |
|-----------------|------|
| 文件搜索递归遍历 | 边界条件（符号链接、权限、深层嵌套）与真实文件系统差异巨大 |
| 任务状态机（pending→running→completed） | 异步时序、并发竞争、失败重试无法模拟 |
| 加密/解密流程 | 密码学操作的正确性必须在真实环境验证 |
| 插件安装/卸载生命周期 | ComboLite 的类加载、签名校验、资源合并无法伪造 |
| WebSocket 消息广播 | 连接管理、断线重连、消息顺序保证 |

**❌ 错误（过度 mock）**：
```typescript
// mock/server.ts — 15 个 handler，包含完整业务逻辑
app.get('/api/files/list', (req, res) => {
  const dir = req.query.path as string
  // ❌ 递归遍历模拟 — 与真实 fs 操作行为不一致
  const files = walkDirectory(dir, { recursive: true, followSymlinks: false })
  res.json({ files })
})

app.post('/api/tasks/create', (req, res) => {
  // ❌ 完整状态机模拟 — 无法复现真实的协程调度
  const task = { id: uuid(), status: 'running', progress: 0 }
  setTimeout(() => { task.status = 'completed' }, 3000)
  res.json(task)
})

app.get('/api/plugins/installed', (req, res) => {
  // ❌ 硬编码插件列表 — 新增插件后此处过时
  res.json([{ id: 'mpv-player', version: '1.0.0', enabled: true }])
})
```

**✅ 正确（最小 mock 集合）**：
```typescript
// mock/server.ts — 3 个核心端点，仅解决前端开发阻塞
app.get('/api/config/schema', (_req, res) => {
  // ✅ 静态 JSON 快照 — schema 变更频率极低
  res.json(require('./fixtures/config-schema.json'))
})

app.get('/api/auth/status', (_req, res) => {
  // ✅ 固定已登录状态 — 前端路由守卫需要
  res.json({ authenticated: true, user: { role: 'admin' } })
})

app.post('/api/health', (_req, res) => {
  // ✅ 心跳检测 — 前端断线重连需要
  res.json({ status: 'ok', timestamp: Date.now() })
})
```

### 1.3 推荐替代方案

| 场景 | 替代方案 | 示例 |
|------|---------|------|
| 需要 API 返回数据 | **测试数据 fixture 文件** | `test/fixtures/api-responses/*.json` |
| 需要验证前端渲染 | **真实后端 + 测试数据库** | `go run ./cmd/encv/ serve --config test-config.json` |
| 需要测试异常场景 | **后端注入故障模式** | 环境变量 `ENCV_FAULT_INJECTION=slow_api:500ms` |
| 需要独立前端开发 | **Vite proxy 到真实后端** | `vite.config.ts` proxy 配置（见 §五） |

---

## 二、严禁阻塞式服务启动（违反 = 开发体验灾难）

### 2.1 核心规则

**Go 后端服务必须在后台运行，不得占用当前终端。**

### 2.2 错误示例（阻塞终端）

```bash
# ❌ 直接前台运行 — 终端被占用，无法执行其他命令
$ go run ./cmd/encv/ serve
# → 终端输出日志流，Ctrl+C 才能退出
# → 无法在同一终端启动 Vite / Capacitor / 其他命令
```

### 2.3 正确示例（后台运行）

```bash
# ✅ 方式一：& 后台运行（最简单）
$ go run ./cmd/encv/ serve > /tmp/encv-backend.log 2>&1 &
$ echo $!  # 打印 PID，用于后续监控/终止

# ✅ 方式二：nohup（断开 SSH 后仍运行）
$ nohup go run ./cmd/encv/ serve > /tmp/encv-backend.log 2>&1 &
echo "Backend PID: $!"

# ✅ 方式三：tmux/screen（推荐长期开发会话）
$ tmux new-session -d -s encv 'go run ./cmd/encv/ serve'
# 查看：tmux attach -t encv
# 分离：Ctrl+B 然后 D

# ✅ 方式四：IDE 独立终端（VS Code / GoLand）
# 在 IDE 的 Terminal 面板中新建一个独立标签页运行
# IDE 终端之间互不阻塞
```

### 2.4 后台进程管理命令

```bash
# 查看后端是否在运行
lsof -i :2025 -t

# 查看日志（实时跟踪）
tail -f /tmp/encv-backend.log

# 停止后端
kill $(lsof -i :2025 -t)

# 强制停止（如果 kill 无效）
kill -9 $(lsof -i :2025 -t)
```

---

## 三、Go 程序直接运行规范

### 3.1 核心规则

**开发环境必须使用 `go run` 一键运行，禁止 `go build` + 手动执行两步法。**

### 3.2 对比

| 方式 | 命令 | 适用场景 |
|------|------|---------|
| **✅ go run** | `go run ./cmd/encv/ serve` | **日常开发**（编译+执行一步完成，支持热重载工具） |
| **❌ go build + 执行** | `go build -o encv && ./encv serve` | **仅生产部署 / CI 构建**（需要控制输出路径和交叉编译） |

### 3.3 为什么禁止两步法

1. **额外步骤**：每次代码修改都要手动重新 build → 容易遗漏 → 运行的是旧代码
2. **二进制残留**：项目根目录散落 `encv` 二进制文件 → 可能被 git 意外提交
3. **路径污染**：`./encv` 在 PATH 中优先于系统命令 → 难以排查的"为什么我的修改没生效"
4. **交叉编译陷阱**：`GOOS=android go build` 产生的二进制无法在桌面运行 → 浪费调试时间

### 3.4 正确用法示例

```bash
# ✅ 启动后端服务
go run ./cmd/encv/ serve

# ✅ 启动 CLI 工具（如一次性任务）
go run ./cmd/encv/ encrypt --input file.sccgv --output file.sccga

# ✅ 带环境变量
ENCV_CONFIG_PATH=./dev-config.json go run ./cmd/encv/ serve

# ✅ 带 build tags（移动端 stub 编译验证）
GOOS=android go run ./cmd/encv/ serve  # 验证 android tag 分支编译通过
```

---

## 四、端口必须正确（违反 = 服务全部失联）

### 4.1 端口分配表

| 服务 | 端口 | 用途 | 配置位置 |
|------|------|------|---------|
| **Go Backend API** | **2025** | HTTP REST API + WebSocket | Go 代码 `Serve()` 或 `--port` 标志 |
| **Vite Dev Server** | **5173** | 前端 HMR + 开发服务器 | `vite.config.ts` (默认) |
| **Proxy Target** | **127.0.0.1:2025** | Vite dev proxy 转发目标 | `vite.config.ts` proxy 配置 |

### 4.2 禁止行为

- ❌ 硬编码其他端口号（如 8080、3000、4000 等 Web 框架默认端口）
- ❌ 使用 `:0` 随机端口（导致 Vite proxy 无法配置固定 target）
- ❌ 在 `config.user.json` 中修改默认端口（见 project_rules.md 配置模板保护规则）
- ❌ 前端 API base URL 写死非标准端口

### 4.3 端口冲突检测

```bash
# 一键检查所有关键端口是否被占用
check_ports() {
  for port in 2025 5173; do
    if lsof -i :$port -t >/dev/null 2>&1; then
      echo "⚠️  Port $port is in use by PID $(lsof -i :$port -t)"
      lsof -i :$port -t | xargs ps -p -o pid,command= 2>/dev/null
    else
      echo "✅ Port $port is free"
    fi
  done
}
check_ports

# 快速查找占用进程
lsof -i :2025 -i :5173

# 杀掉冲突进程（谨慎使用）
kill $(lsof -i :2025 -t) 2>/dev/null
kill $(lsof -i :5173 -t) 2>/dev/null
```

### 4.4 Vite Proxy 配置（正确示例）

```typescript
// vite.config.ts
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,           // ✅ 标准 Vite 端口
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:2025',  // ✅ Go Backend 标准端口
        changeOrigin: true,
        ws: true,            // ✅ WebSocket 代理
      },
      '/ws': {
        target: 'ws://127.0.0.1:2025',  // ✅ WebSocket 直连
        ws: true,
      },
    },
  },
})
```

---

## 五、Capacitor 预览标准化流程

### 5.1 完整启动序列

```
Step 1 ──→ Step 2 ──→ Step 3（可选）
Backend     Frontend    Capacitor
:2025       :5173       sync/preview
```

#### Step 1：启动 Go 后端（后台，端口 2025）

```bash
# 进入项目根目录
cd /workspace

# 后台启动 Go 后端
go run ./cmd/encv/ serve > /tmp/encv-backend.log 2>&1 &

# 验证启动成功
sleep 2
curl -s http://127.0.0.1:2025/api/health | jq .
# 预期输出: {"status":"ok","timestamp":...}
```

#### Step 2：启动 Vite 前端（端口 5173）

```bash
# 同一目录下新开终端（或使用 tmux/tab）
npm run dev
# 或
npx vite

# 预期输出:
#   VITE v5.x.x  ready in xxx ms
#   ➜  Local:   http://localhost:5173/
#   ➜  Network: use --host to expose
```

#### Step 3（可选）：Capacitor 同步 / 预览

```bash
# 同步 web 资源到 native 项目
npx cap sync

# Android 预览（需 Android Studio / SDK）
npx cap open android

# iOS 预览（需 Xcode，macOS only）
npx cap open ios

# 浏览器预览（不需要 native 工具链）
npx cap serve
# 访问 http://localhost:3333 （Capacitor 内置预览服务器）
```

### 5.2 一键启动脚本（参考）

```bash
#!/bin/bash
# start-dev.sh — 开发环境一键启动

set -e

echo "=== ENCV Development Environment ==="

# Step 1: Backend
echo "[1/2] Starting Go backend on :2025 ..."
if lsof -i :2025 -t >/dev/null 2>&1; then
  echo "  ⚠️  Port 2025 already in use, skipping"
else
  go run ./cmd/encv/ serve > /tmp/encv-backend.log 2>&1 &
  BACKEND_PID=$!
  echo "  ✅ Backend started (PID: $BACKEND_PID)"
fi

# Wait for backend ready
for i in $(seq 1 10); do
  if curl -s http://127.0.0.1:2025/api/health >/dev/null 2>&1; then
    echo "  ✅ Backend health check passed"
    break
  fi
  sleep 0.5
done

# Step 2: Frontend
echo "[2/2] Starting Vite frontend on :5173 ..."
npm run dev &
FRONTEND_PID=$!
echo "  ✅ Frontend starting (PID: $FRONTEND_PID)"

echo ""
echo "=== Ready ==="
echo "  Frontend: http://localhost:5173"
echo "  Backend:  http://localhost:2025"
echo "  Logs:     tail -f /tmp/encv-backend.log"
echo "  Stop:     kill $(lsof -i :2025 -t) $(lsof -i :5173 -t)"
```

### 5.2.1 Capacitor 预览专用一键启动（`scripts/start-preview.sh`）

**适用场景**：浏览器沙箱预览、Capacitor 开发模式。脚本整合 mock 数据生成 + 后端 air 监视 + Vite 前端为一条命令。

**铁律**：
- servingDir 永远为 `/storage/emulated/0` 绝对路径（设计预期，脚本自建真实目录）
- **严禁任何符号链接**（mock-data 真实目录就在 `/storage/emulated/0`）
- **严禁修改 `config.user.json`**（保持 `mobile.server.dir` 为绝对 Android 路径）
- **后端必须用 air 监视重载**（禁止 `go build` / `go run`）
- **严禁误杀 agent-tool-host**（沙箱基础设施在 16000 端口，反向代理到 Vite）
- 脚本保持前台运行（便于 OpenPreview 激活），脚本退出时优雅停止所有子进程

**前置条件**：
- `air` 在 PATH 中（mise 安装的 Go 1.25.1 自带：`/root/.local/share/mise/installs/go/1.25.1/bin/air`）
- 前端依赖已安装：`cd app/encv-mobile && npm install`（脚本会自动检测并安装）

**使用方式**：
```bash
cd /workspace
bash app/encv-mobile/scripts/start-preview.sh
```

**脚本行为**：
1. Step 0: 杀掉残留的 air / encv / vite 进程（精确按进程名匹配 + `lsof -ti :2025` 按端口兜底，绝不杀 agent-tool-host）
2. Step 1: 确保 `node_modules` 就绪（缺失则自动 `pnpm install --no-frozen-lockline`，支持 workspace 成员）
3. Step 2: 运行 `npx tsx scripts/generate-mock-files.ts` 生成 mock 数据到 `/storage/emulated/0/01-plain-media/...`
4. Step 3: 启动 `ENCV_DEV_PREVIEW=1 air` 监视 `./cmd/encv/`（air 自动重建并重启后端）+ **verify `/api/service-guard` 返回 `servingDir=/storage/emulated/0`，否则 exit 1**
5. Step 4: 启动 Vite（`--host 0.0.0.0 --port 5174 --strictPort`）
6. Step 5: 启动 OpenList 前端 dev server (`:3000`, `OPENLIST_PREVIEW_BASE="/openlist-ui/"`) — 沙箱预览 OpenList UI 用
7. Step 6/7: 状态报告 + 保持前台 / 退出 detach

**激活外部访问**：
- 脚本返回 `command_id` 后，**必须调用 `OpenPreview(command_id="<id>", preview_url="http://localhost:5174/")`**
- **预览 URL 用 Vite 实际端口（5174）**，不是 5173（5173 在沙箱中无服务监听，agent-tool-host 实际占 16000）

**沙箱端口身份**（铁律）：

| 端口 | 进程 | 身份 |
|------|------|------|
| 16000 | agent-tool-host | 公网反向代理入口（`PREVIEW_PROXY_PUBLIC_PORT`） |
| 5174/5175/... | Vite | 实际 dev server（端口漂移，agent-tool-host 反代到此） |
| 2025 | encv（air 监视） | Go Backend |

**禁止做法**：
- ❌ `ln -sfn` 任何路径"桥接" mock-data（mock-data 实际位置就是 `/storage/emulated/0`）
- ❌ 修改 `config.user.json` 的 `mobile.server.dir`
- ❌ 使用 `go build` / `go run`（必须用 air 监视）
- ❌ `lsof -i :5173 | xargs kill`（会误杀 agent-tool-host 沙箱基础设施）
- ❌ 跳过 mock 生成直接启动（会触发 service guard）

### 5.3 常见问题排查

| 症状 | 可能原因 | 排查命令 |
|------|---------|---------|
| 前端 API 请求 404/502 | 后端未启动或端口错误 | `curl http://127.0.0.1:2025/api/health` |
| 前端 API 请求跨域 CORS 错误 | Vite proxy 未配置或 target 端口错误 | 检查 `vite.config.ts` proxy.target |
| WebSocket 连接失败 | proxy 未开启 `ws: true` 或后端未监听 `/ws` | `curl -i -N -H "Connection: Upgrade" http://127.0.0.1:2025/ws` |
| Capacitor 预览空白页 | 未执行 `cap sync` 或 web 资源过期 | `npx cap sync && npx cap serve` |
| 端口被占用 | 上次开发会话未清理 | `lsof -i :2025 -i :5173` + `kill` |
| 修改 Go 代码后前端无变化 | 后端是旧进程（go run 不会自动重启） | 重启后端：先 kill 旧进程再 `go run` |
| Vite HMR 不生效 | 文件保存事件未触发（某些远程文件系统） | 触发一次 touch 或重启 Vite |
| **service-guard BLOCKED：`server.dir missing "01-plain-media"`，列出 `.md` 文件** | **mobile overlay 未生效**：`server.dir` 留在默认的 `/` → 解析为 `/workspace` → 看到 workspace 根目录文件。常见原因：手工 `tmp/encv start` 没设 `ENCV_DEV_PREVIEW=1`，或 start-preview.sh Step 0 没杀掉残留进程 | `curl -s http://127.0.0.1:2025/api/service-guard` 看 `servingDir` 字段；应该是 `/storage/emulated/0` 而不是 `/` 或 `/workspace` |

### 5.3.1 ⚠️ service-guard 失败根因清单（2026-06-04 实战踩坑）

> **`/api/service-guard` 报告 `server.dir missing "01-plain-media"` 是 mobile overlay 没生效的标志**。
> 任何路径下看到此错误，都按"mobile overlay 未触发"处理，不要去改 `config.user.json`。

| 触发场景 | 根因 | 修复 |
|----------|------|------|
| 手工启动 `tmp/encv start` / `tmp/encv` | 缺 `ENCV_DEV_PREVIEW=1` 环境变量，config overlay 不被加载 | 用 `start-preview.sh` 启动；或手工起时 `export ENCV_DEV_PREVIEW=1` |
| `start-preview.sh` 启动后服务 guard 失败 | Step 0 漏杀旧 `tmp/encv` 进程（`pkill -f '^./tmp/encv'` 不匹配 `/workspace/tmp/encv start`） | 2026-06-04 修复：Step 0 改用 `lsof -ti :2025` 按端口兜底杀进程；Step 3 启动 air 后必须 verify `/api/service-guard` 的 `servingDir == /storage/emulated/0`，否则 `exit 1` |
| mock-data 不在 `/storage/emulated/0/01-plain-media` | 跳过了 `npx tsx scripts/generate-mock-files.ts` 步骤 | 重跑 start-preview.sh（Step 2 自动生成） |
| 改了 `config.user.json` 的 `server.dir` 或 `mobile.server.dir` | 违反 start-preview.sh §5.2.1 铁律 | `git checkout config.user.json` 还原 |

### 5.4 开发环境健康检查

```bash
#!/bin/bash
# check-dev.sh — 快速诊断开发环境状态

echo "=== Development Environment Health Check ==="
echo ""

# Backend
if curl -sf http://127.0.0.1:2025/api/health >/dev/null 2>&1; then
  echo "✅ Backend (:2025) — healthy"
else
  echo "❌ Backend (:2025) — not responding"
fi

# Frontend
if curl -sf http://127.0.0.1:5173 >/dev/null 2>&1; then
  echo "✅ Frontend (:5173) — running"
else
  echo "⚠️  Frontend (:5173) — not detected (may be on different port)"
fi

# Proxy connectivity
if curl -sf http://127.0.0.1:5173/api/health >/dev/null 2>&1; then
  echo "✅ Vite Proxy → Backend — connected"
else
  echo "⚠️  Vite Proxy → Backend — cannot verify (frontend may not be running)"
fi

echo ""
echo "All ports:"
lsof -i :2025 -i :5173 2>/dev/null || echo "  (no processes found)"
```

---

## 六、WAF/代理截断路径参数（⚠️ 实战踩坑！）

> **核心原则：经过 WAF/反向代理的请求中，`@` 字符会被当作 URL authority 分隔符截断。**
> **所有路径参数必须使用双重编码（double encoding）穿越代理层。**

### 6.1 症状

```
用户操作：点击文件 `special-chars-!@#$%^&*()_+.txt` 预览
预期行为：前端 fetch /decrypt?file=... → 后端返回文件内容
实际行为：HTTP 404 — file not found

关键矛盾：
  curl 同样请求 → 200 OK ✅
  浏览器同样请求 → 404 ❌
```

### 6.2 根因链路

```
前端: encodeURIComponent("special-chars-!@#$%^&*()_+.txt")
  → "special-chars-!%40%23%24%25%5E%26*()_%2B.txt"
    ↓ 发送到 Vite dev server

Vite proxy 转发到后端（正常）
  ↓

用户浏览器环境: Android WebView / com.xunlei.browser（迅雷浏览器）
  带有大量 WAF/proxy header:
    x-alb-waf-requestid, x-clb-cluster, x-envoy-external-address, ...
  ↓

WAF/中间代理处理 query string:
  发现 %40 → 解码为 @ → 当作 URL authority 分隔符
  → 截断 @ 之后的所有字符
  → 实际到达后端的 filePath = "special-chars-!" （不完整！）
    ↓

后端: 在 mock_data 目录查找 "special-chars-!.txt" → 404 Not Found
```

**证据**（mock 层 404 响应体中的 debug 信息）：

```json
{
  "error": "file not found",
  "debug": {
    "receivedFilePath": "/04-boundary-test/special-chars-!@",
    "resolvedAbsPath": "/workspace/app/encv-mobile/__mock_data__/04-boundary-test/special-chars-!@",
    "siblings": ["special-chars-!@#$%^&*()_+.txt", ...]
  }
}
```

`siblings` 列表中存在完整文件名，但 `receivedFilePath` 在 `@` 处被截断。

### 6.3 修复方案：双重编码（Double Encoding）

**原理**：编码两次，WAF 只解码外层一次，内层 `%40` 安全通过。

```
原始路径: special-chars-!@#$%^&*()_+.txt
  ↓ 第 1 次 encodeURIComponent
单层编码: special-chars-!%40%23%24%25%5E%26*()_%2B.txt
  ↓ 第 2 次 encodeURIComponent（proxySafeEncode）
双重编码: special-chars-!%2540%2523%2524%25255E%2526*()_%252B.txt
  ↓ WAF/代理解码外层
WAF 输出: special-chars-!%40%23%24%25%5E%26*()_%2B.txt  (@ 仍是 %40！)
  ↓ 后端 decodeURIComponent（第二次解码）
最终结果: special-chars-!@#$%^&*()_+.txt  ✅ 完整恢复
```

### 6.4 实现细节

#### 前端（TypeScript）— `proxySafeEncode()`

```typescript
// src/api/encv.ts
export function proxySafeEncode(value: string): string {
  return encodeURIComponent(encodeURIComponent(value))
}
```

**应用范围**（19 处替换）：所有将路径放入 query parameter 的 API 调用。

| 文件 | 替换数 | 涉及端点 |
|------|--------|---------|
| [api/encv.ts](app/encv-mobile/src/api/encv.ts) | 14 | listFiles, stream, plugin-stream, deleteFile, readFileContent, checkFileExists, getFileStreamUrl, getFilePreviewUrl, getExternalStreamUrl, listFilesByTag, getAlistEncryptStreamUrl |
| [views/FilePreview.vue](app/encv-mobile/src/views/FilePreview.vue) | 2 | decrypt, api/file/info |
| [views/FileInfo.vue](app/encv-mobile/src/views/FileInfo.vue) | 1 | api/file/info |

#### Go 后端 — `DecodePathParam()`

```go
// internal/utils/path.go
func DecodePathParam(raw string) string {
    s, err := url.QueryUnescape(raw)
    if err != nil { return raw }
    s2, err := url.QueryUnescape(s)
    if err != nil { return s }
    return s2
}
```

**应用范围**（4 处）：

| 文件 | 函数 | 端点 |
|------|------|------|
| [server_handle.go](internal/server/server_handle.go) | handleStreamRequest | /stream?path= /stream?file= |
| [openlist_handlers.go](internal/server/openlist_handlers.go) | handler | /openlist/sites/:siteId/decrypt?file= |
| [openlist_middleware.go](internal/server/openlist_middleware.go) | OpenlistSiteMiddleware | /openlist/sites/:siteId/decrypt?file= |

#### Mock 层同步更新

```typescript
// app/encv-mobile/mock/handlers.ts
let filePath = url.searchParams.get('file') || url.searchParams.get('path') || ''
try { filePath = decodeURIComponent(filePath) } catch {}
```

### 6.5 为什么只对 path 参数双重编码

| 参数类型 | 是否需要双重编码 | 原因 |
|---------|-----------------|------|
| **path / file**（文件路径） | **✅ 必须** | 用户可控，可能包含 `@#` 等特殊字符 |
| password（加密密码） | 可选 | 通常不含特殊字符，但建议保持一致 |
| tag（标签名） | 可选 | 通常为 ASCII 字母数字 |
| extensions（扩展名列表） | 不需要 | 固定格式 `.ext1,.ext2` |

### 6.6 测试覆盖

| 测试文件 | 用例数 | 覆盖场景 |
|---------|--------|---------|
| [path_test.go](internal/utils/path_test.go) | 24 (15+8+1) | DecodePathParam 双重解码 + RoundTrip 编码往返验证 |
| [proxy-safe-encode.test.ts](app/encv-mobile/__tests__/proxy-safe-encode.test.ts) | 8 | proxySafeEncode 双重编码 + Unicode + 特殊字符 + 空值 |

### 6.7 排查此类问题的诊断方法

当出现「curl 正常但浏览器失败」的矛盾时：

1. **Mock 层拦截**：在 Vite middleware 中直接处理请求，打印 `req.url` 和解析后的参数
2. **响应体带 debug 信息**：404 时返回 `{ error, debug: { receivedFilePath, resolvedAbsPath, siblings } }`
3. **前端错误详情按钮**：FilePreview.vue 的 Show Details 展开响应体 JSON
4. **对比法**：同一 URL 用 curl 和浏览器分别测试，对比差异

### 6.8 已知受影响的特殊字符

以下字符在 URL query string 中有特殊含义，必须被正确编码：

| 字符 | URL 含义 | 单层编码 | 双重编码后 WAF 解码结果 |
|------|---------|----------|---------------------|
| `@` | **authority 分隔符** ⚠️ | `%40` | `%40`（安全） |
| `#` | fragment 分隔符 | `%23` | `%23`（安全） |
| `$` | 无特殊含义 | `%24` | `%24`（安全） |
| `%` | 编码前缀 | `%25` | `%25`（安全） |
| `^` | 无特殊含义 | `%5E` | `%5E`（安全） |
| `&` | query 分隔符 | `%26` | `%26`（安全） |
| `+` | 空格替代 | `%2B` | `%2B`（安全） |
| `()` | 无特殊含义 | 不编码 | `()`（安全） |
| `!` | 无特殊含义 | 不编码 | `!`（安全） |

**其中 `@` 是唯一确认会被迅雷浏览器/WAF 截断的字符。** 其他字符虽然理论上也可能被某些代理误处理，但双重编码方案统一保护了所有特殊字符。

## 七、Hi-Sillot-OpenList-Frontend fork 适配：solid-icons 命名兼容（⚠️ 实战踩坑！）

### 7.1 症状

打开 `/openlist-ui/` 时，#root 一直空、只有注入的「返回 ENCV」按钮可见。debug pane 抓到：

```
[err] SyntaxError: The requested module '.../solid-icons@1.2.0_.../solid-icons/tb/index.js' does not provide an export named 'TbCheck' @ .../FolderTree.tsx:7:15
```

页面整体 JS module graph 解析失败 → Solid App 永远不 mount。

### 7.2 根因

| 维度 | 说明 |
|------|------|
| **fork 的源码约定** | `solid-icons` 1.8+：`import { TbCheck, TbX, TbFile } from "solid-icons/tb"`（无前缀 = 填充变体） |
| **本工作区实际安装** | pnpm 锁定 `solid-icons@1.2.0`，**只有** `TbFillXxx` / `TbOutlineXxx` 前缀变体 |
| **类型签名** | fork 的 `node_modules/.../tb/index.d.ts` 不会缺（npm 把上游 d.ts 一并装下来），所以 TS 编译过 |
| **运行时** | 浏览器 ESM 解析真实 JS module 时发现导出列表对不上 → throw SyntaxError → 整个 module graph 中断 |

### 7.3 命名映射

| fork 写的（1.8+） | 1.2.0 实际提供 |
|------------------|---------------|
| `TbCheck` | `TbFillCircleCheck` / `TbOutlineCheck` |
| `TbX` | `TbOutlineX` |
| `TbFile` | `TbFillFile` / `TbOutlineFile` |
| `TbFolder` | `TbFillFolder` / `TbOutlineFolder` |
| `TbArchive` / `TbRefresh` / `TbCopy` / `TbLink` / `TbSelector` / `TbPlus` / `TbCheckbox` / `TbExternalLink` / `TbFileArrowRight` | `TbOutline${Xxx}` 全部存在 |

> 其他包（`bi` / `ai` / `io` / `ri` / `cg` / `fa` / `fi` / `bs` / `im` / `si`）在 1.2.0 里都齐全，不受影响。

### 7.4 修复方案：vite plugin 通用 import 重写

**文件**：`app/openlist/Hi-Sillot-OpenList-Frontend/vite-plugins/solid-icons-compat.ts`

```ts
const TB_IMPORT_RE =
  /import\s*\{\s*([^}]+?)\s*\}\s*from\s*(["'])solid-icons\/tb\2/g

// 裸 Tb* → 改写为 TbOutlineXxx as TbXxx
// 已带 Fill/Outline 前缀 → 保持
```

**接入位置**：`vite.config.ts` 必须在 `solidPlugin()` **之前**，`enforce: "pre"`：

```ts
plugins: [
  solidIconsCompat(),  // ← 必须最先
  solidPlugin(),
  ...
]
```

### 7.5 验证

```bash
# 1. 重启 openlist vite
pkill -f 'Hi-Sillot-OpenList-Frontend.*vite'
cd app/openlist/Hi-Sillot-OpenList-Frontend
setsid nohup env OPENLIST_PREVIEW_BASE="/openlist-ui/" OPENLIST_NO_HMR=1 \
  pnpm dev --host 127.0.0.1 --port 3000 --strictPort \
  </dev/null >/tmp/encv-openlist.log 2>&1 &

# 2. 验证重写生效
curl -s 'http://127.0.0.1:3000/openlist-ui/src/components/FolderTree.tsx' \
  | grep 'solid-icons/tb'
# 期望：import { TbOutlineX as TbX, TbOutlineCheck as TbCheck } from "..."

# 3. 浏览器打开 /openlist-ui/，debug pane 应该消失、#root 应该 mount
```

### 7.6 兼容性

- 如果未来 `solid-icons` 升到 ≥ 1.8，本插件变成 no-op（重写后的名字在 1.8+ 也都存在）
- TS 类型可能仍报缺导出，但 fork 是 .tsx + vite-plugin-solid，类型不参与运行
- 不影响 production build：plugin 走 vite.transform，prod 也生效，但 prod 永远命中"已是正确前缀"分支

## 八、vite HMR WebSocket 噪音过滤（16000 沙箱预览专用）

### 8.1 症状

encv-mobile 预览控制台持续刷：
```
[vite] failed to connect to websocket (Error: WebSocket closed without opened.)
    at Object.connect (/@vite/client:892:13)
```

不影响应用运行（HMR 是开发辅助，非核心功能），但会污染 DevLogs。

### 8.2 根因

`16000` 沙箱入口（agent-tool-host）不支持 WebSocket Upgrade 协议。`@vite/client` 启动时尝试连 `ws://.../{__hmr__token__}`，连接被代理中断，浏览器每秒重试一次。

### 8.3 修复

`app/encv-mobile/src/composables/useFrontendLogs.ts` 的 `hijackConsole()` 增加 `isHmrWsNoise` 过滤：

```ts
console.error = (...args) => {
  saved.error(...args)  // 原生 console.error 仍输出到 DevTools
  if (isHmrWsNoise(args)) {
    addLog('debug', ['[HMR WS sandbox noise] ' + args[0]])
    return
  }
  addLog('error', args)
}
```

`isHmrWsNoise` 匹配 `failed to connect to websocket` 和 `WebSocket closed without opened`。命中后**降级为 debug 级别**记录，不丢信息、不污染 error 流。

### 8.4 为什么不做"完全关 HMR"

| 做法 | 优劣 |
|------|------|
| 完全关 HMR（`hmr: false`） | 噪音彻底消失，但本地直连 `localhost:5173` 也失去 HMR |
| **只过滤日志（推荐）** | **DevLogs 干净 + 本地直连 HMR 仍可用** |

沙箱预览是只读验证场景，HMR 不可用是已知限制；本地开发仍依赖 HMR。

## 九、16000 agent-tool-host 代理路径白名单（⚠️ 实战踩坑！）

### 9.1 症状

OpenList UI 页面 JS 加载、mount 都正常，但 axios 调 `/api/public/settings` 返回 `Network Error`。GIN 日志里 0 条 `/api/*` 记录。看似后端不通，但本地 curl `http://127.0.0.1:2025/api/public/settings` 返回 200。

### 9.2 根因

`16000 agent-tool-host` 的 `preview_proxy::proxy` **只转发特定路径前缀**（基于 OpenPreview 注册的端口 + frontend base path）。从 `/var/log/tool/agent-tool-host.stdout.log` 看到的实际转发列表只有：

| 路径模式 | 转发到 | 用途 |
|---------|--------|------|
| `/` | 2025 | 根 fallback |
| `/openlist-ui/...` | 2025 | openlist vite 的所有资产 + 页面 |
| `/ws` | 2025 | WebSocket（HMR / encv-go WS hub） |
| `/?_port=...` | （自身） | 内部健康检查 |
| `/?token=...` | 2025 | preview URL 带 token 鉴权 |

**其他路径（包括 `/api/*`）被 16000 静默丢弃**——不返回 4xx、不写日志、TCP 直接关闭。浏览器 axios 等不到响应 → `Network Error`。

### 9.3 验证

```bash
# 16000 实际代理了哪些路径
grep "preview_proxy::proxy" /var/log/tool/agent-tool-host.stdout.log \
  | sed 's/.*Proxying //; s/ to port.*//' | sort -u

# 用户 axios 的请求是否进了 encv-go
grep "api/public" /tmp/encv-air.log | grep -v 127.0.0.1
# 期望：空（说明没到 encv-go，被 16000 截了）
```

### 9.4 修复方案：让 axios baseURL 走通前缀

让 axios 的 baseURL 包含 16000 已知的白名单前缀（`/openlist-ui`），让请求被 16000 转发，再由 openlist vite 内部 proxy 回 encv-go 的 mock：

**Step 1**：`vite-plugins/encv-openlist-config.ts` 把 `api` 从 `""` 改为 `"/openlist-ui"`：

```ts
window.OPENLIST_CONFIG = Object.assign({}, window.OPENLIST_CONFIG, {
  // 沙箱预览下必须用 /openlist-ui 前缀，否则 16000 代理会丢请求
  api: "/openlist-ui",
  base_path: "/openlist-ui/",
});
```

**Step 2**：`vite.config.ts` 的 proxy 必须用 RegExp（vite 字符串 key 是 prefix match，`"/api"` 不会匹配 `/openlist-ui/api/...`）：

```ts
proxy: {
  "^/openlist-ui/api": {
    target: "http://127.0.0.1:2025",  // encv-go dev_preview_proxy
    changeOrigin: true,
    // 把 /openlist-ui/api/* 重写为 /api/*，避免 dev_preview_proxy 的
    // /openlist-ui/* 路由把请求回环到本 vite (无限代理循环)
    rewrite: (path) => path.replace(/^\/openlist-ui\/api/, "/api"),
  },
}
```

### 9.5 完整请求链

```
Browser axios
  GET /openlist-ui/api/public/settings
  ↓
16000 agent-tool-host（白名单：/openlist-ui/* 转发）
  ↓
encv-go dev_preview_proxy（/openlist-ui/* → :3000 openlist vite）
  ↓
openlist vite（proxy "^/openlist-ui/api" → encv-go，rewrite 为 /api/...）
  ↓
encv-go /api/public/* mock handler
  ↓ 返回 JSON {"code":200,"data":{...}}
Browser axios 收到响应 ✅
```

### 9.6 反模式（已验证失败）

| 配置 | 失败现象 |
|------|---------|
| `api: ""` + openlist vite proxy `/api` | axios 调 `/api/...` → 16000 丢 |
| `api: ""` + 改 openlist vite proxy 为 `/openlist-ui/api` | proxy 字符串 key 不匹配（不会触发） |
| dev_preview_proxy 加 `/openlist-ui/api/*` 路由 | 与 `/openlist-ui/*` 路由冲突，gin 后注册覆盖前注册 |
| **正确做法** | `api: "/openlist-ui"` + openlist vite proxy RegExp `^/openlist-ui/api` + rewrite |

### 9.7 与 dev_preview_proxy 的职责边界

| 组件 | 职责 |
|------|------|
| **dev_preview_proxy** | 路由分发：`/openlist-ui/*` → openlist vite，`/api/*` → :5244 OpenList backend，其它 → encv-mobile vite |
| **openlist vite proxy** | 跨 backend 桥接：`/api` → :5244 OpenList binary（生产 + dev preview 一致） |
| **不要在两者间加新层** | 任何"再封装一层"都会导致路由冲突或循环代理 |

---

## 十、dev preview 零 mock：全 reverse proxy 到 :5244 OpenList backend

> **铁律（2026-06 更新）**：
> **dev preview 不 mock 任何 `/api/*` 端点。**
> **所有 OpenList API（含 `/me`、`/fs/*`、`/admin/*`、`/auth/*`、`/public/*`）全部 reverse proxy 到 :5244 真 OpenList backend。**
> **这是 routing 不是 mock——后端真服务，开发体验与生产 100% 一致。**

### 10.1 为什么不能再 mock

**症状**：
- mock 2 个端点时工作
- 之后 dev preview 看起来能跑，但 OpenList frontend 一进入真实业务流程就报错
- 用户说"mock 与真实 API 行为差异巨大"——列举了：`/fs/list` 排序、permission 校验、2FA 流程、token 失效时机

**根因**：
- mock 的字段不够全（settings 只有 7 个字段，真 backend 有 50+）
- mock 的 status code 假设与真 backend 不一致
- mock 的时序与真 backend 不同（mock 立即返，真 backend 有 DB 查询延迟）
- mock 不能复现真 backend 的 bug 修复（真 backend 改了，mock 不知道）

**修复**：删除所有 mock，让 NoRoute 把 `/api/*` 反代到 :5244 真 backend。

### 10.2 当前 dev_preview_proxy 架构（最终方案）

```
Browser → 16000 (agent-tool-host 路径白名单) → 2025 (encv-go 编排) → ┬─ /openlist-ui/*  → :3000 (openlist vite)
                                                                      ├─ /api/*          → :5244 (OpenList backend 真实 API)
                                                                      └─ /*              → :5173 (encv-mobile vite)
```

**dev_preview_proxy.go 三个职责**：
| 方法 | 路由 | 目标 |
|------|------|------|
| `RegisterExplicit` | `/openlist-ui/*` | :3000 openlist vite（vite proxy `/api` → :5244） |
| `RegisterNoRoute` (1) | `/api/*` | :5244 OpenList backend（reverse proxy，不是 mock） |
| `RegisterNoRoute` (2) | 其他 | :5173 encv-mobile vite |

### 10.3 OpenList backend :5244 启动

**编译**：
```bash
cd /workspace/app/openlist/Hi-Sillot-OpenList
go build -tags=jsoniter -o /tmp/openlist .
```

**首次启动**（自动生成 admin 密码 + sqlite db）：
```bash
mkdir -p /tmp/openlist-data
cat > /tmp/openlist-data/config.json <<'EOF'
{
  "database": {"type":"sqlite3","db_file":"/tmp/openlist-data/data.db","table_prefix":"x_"},
  "scheme": {"address":"0.0.0.0","http_port":5244,"https_port":-1},
  "temp_dir":"/tmp/openlist-data/temp",
  "dist_dir":"/workspace/app/openlist/Hi-Sillot-OpenList/public/dist",
  "log": {"enable":true,"name":"/tmp/openlist-data/log/log.log","max_size":50,"max_backups":30,"max_age":28,"compress":false}
}
EOF
# stub index.html 让 go:embed 找到 dist
echo '<!DOCTYPE html><title>OpenList Backend Stub</title>' > /workspace/app/openlist/Hi-Sillot-OpenList/public/dist/index.html

/tmp/openlist server --data /tmp/openlist-data --dev --log-std
```

**关键陷阱**：
- `--data` 标志**不**影响 sqlite db 文件位置（db 位置由 config.json 的 `database.db_file` 决定，或默认 `data/data.db`）
- **必须** 设 `dist_dir`（或放 stub index.html 到 `public/dist`），否则启动 panic: `index.html not exist`
- 默认 admin 密码：启动日志 `Successfully created the admin user and the initial password is: <password>`

**修改 admin 密码为 `admin`（dev 友好）**：
```bash
python3 <<'EOF'
import hashlib, secrets, sqlite3
STATIC = 'https://github.com/alist-org/alist'
new_salt = secrets.token_hex(8)
inner = hashlib.sha256(f'admin-{STATIC}'.encode()).hexdigest()
new_hash = hashlib.sha256(f'{inner}-{new_salt}'.encode()).hexdigest()
db = sqlite3.connect('/tmp/openlist-data/data.db')
db.execute("UPDATE x_users SET pwd_hash=?, salt=? WHERE username='admin'", (new_hash, new_salt))
db.commit()
EOF
# 重启 backend
```

**密码算法参考**（OpenList v4 / alist）：
- `StaticHash(pw) = sha256(pw + "-" + "https://github.com/alist-org/alist")`
- `PwdHash = sha256(StaticHash(pw) + "-" + salt)`

### 10.4 完整请求链路（dev preview 沙箱）

```
Browser
  axios GET /api/auth/login
  ↓
16000 agent-tool-host
  ↓ (白名单转发 /api/*)
2025 encv-go dev_preview_proxy
  ↓ NoRoute 匹配 /api/*
5244 OpenList backend
  ↓ 返回 {code:200, data:{token:"eyJ..."}}
Browser 收到 JWT
  ↓
Browser
  axios GET /api/me + Authorization: <jwt>
  ↓
16000 → 2025 → NoRoute → 5244
  ↓
5244 ParseToken(validTokenCache 有) → 返 admin user
  ✅
```

**关键**：
- OpenList frontend 用 `Authorization: <jwt>`（**无 Bearer 前缀**），不是 `Bearer <jwt>`——backend 期望纯 token
- `validTokenCache` 是**内存 cache**——backend 重启会清空，需重新登录

### 10.5 验证命令（端到端）

```bash
# 1. 登录拿 token
T=$(curl -s -X POST http://127.0.0.1:2025/api/auth/login \
       -H 'Content-Type: application/json' \
       -d '{"username":"admin","password":"admin"}' | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])')

# 2. /api/me（验证 token）
curl -s http://127.0.0.1:2025/api/me -H "Authorization: $T"

# 3. /api/fs/list（验证真实文件）
curl -s "http://127.0.0.1:2025/api/fs/list?path=/" -H "Authorization: $T" | head -c 500

# 4. /api/config（验证 encv-go 自己的 API 不被劫持）
curl -s http://127.0.0.1:2025/api/config | head -c 200
```

### 10.6 反模式（再次强调）

| 反模式 | 后果 |
|--------|------|
| dev_preview_proxy 加 `r.GET("/api/me", mockHandler)` 等具体 mock 端点 | gin 路由污染；mock 与真 backend 数据漂移 |
| dev_preview_proxy 加 `r.GET("/api/*subpath", mock)` catch-all | 与 `/api/config` 等具体路径 gin radix tree 冲突 panic |
| dev_preview_proxy NoRoute 转发 `/api/*` 到 encv-mobile vite (5173) | encv-mobile vite proxy `/api` → encv-go (2025) → 回环 502 |
| **正确做法** | NoRoute 分发：`/api/*` → :5244 backend，其他 → :5173 vite |

---

## 十一、plugin-openlist 与 Hi-Sillot-OpenList-Frontend 是两个完全独立的前端（⚠️ 命名踩坑！）

> **核心原则：**
> **"openlist 前端"在 ENCV 语境下专指 `plugin-openlist/web`（Vue + Ionic + Capacitor），跑在 :5174。**
> **Hi-Sillot-OpenList-Frontend（SolidJS, OpenListTeam 官方 web UI）跑在 :3000，是 OpenList 二进制自带的 dist 来源，与 ENCV 沙箱 dev preview 无关。**

### 11.1 二者对比

| 维度 | `app/encv-mobile/plugin-openlist/web/` ✅ ENCV 用这个 | `app/openlist/Hi-Sillot-OpenList-Frontend/` ❌ 别用 |
|------|---|---|
| **框架** | Vue 3 + Ionic 8 + Capacitor | SolidJS + Vite |
| **Vite 端口** | **5174** | 3000 |
| **Vite base** | dev: `/openlist-ui/` (env `VITE_BASE`); prod: `./` | dev: `process.env.OPENLIST_PREVIEW_BASE`; prod: `/__dynamic_base__/` |
| **Vite proxy** | `/openlist-spa/*` → `http://127.0.0.1:5244/*` | `/api` → `http://localhost:5244` |
| **健康检查** | `/__openlist-health`（Node 直连 5244） | 无 |
| **角色** | ENCV plugin-openlist 的 Capacitor UI（OpenListHome / OpenListWebView / OpenListConfigEditor / OpenListSettings） | OpenList **自带的 web UI**，编译后是 binary go:embed 的 dist 源（**不是** ENCV 沙箱 dev 入口） |
| **沙箱 dev 路径** | 浏览器 → 16000 → 2025 → **:5174** | 浏览器 → 16000 → 2025 → :3000（**错配**） |
| **生产路径** | Android WebView `file:///android_asset/openlist/index.html` | OpenList binary 内 `public/dist/`（gomobile 用） |

### 11.2 命名冲突的根因

两个项目都叫"openlist 前端"：
- `app/openlist/Hi-Sillot-OpenList-Frontend/` —— OpenListTeam 官方 web UI（SolidJS）
- `app/encv-mobile/plugin-openlist/web/` —— ENCV 自己封装的 Capacitor UI（Vue）

混淆的代价：
- 把 `/openlist-ui/*` 路由到 :3000 → 用户看到 OpenListTeam 官方 web UI，**不是** ENCV 期望的 Capacitor UI
- "Failed fetching settings: home.Network Error"——这是 Hi-Sillot-OpenList-Frontend (SolidJS) 的 i18n key `home.fetching_settings_failed` 报的，**不是** plugin-openlist
- plugin-openlist/web 用的是 `/openlist-spa/*`（不是 `/api/*`），vite.config proxy 路径也不一样
- plugin-openlist 用 `import.meta.env.DEV` 判断沙箱/真机，Hi-Sillot-OpenList-Frontend 用 `import.meta.env.VITE_LITE` 等

### 11.3 plugin-openlist/web Vite 配置要点

**`plugin-openlist/web/vite.config.ts` base 配置**：
```ts
const isSandboxDev = !!process.env.VITE_BASE

export default defineConfig({
  // 沙箱 dev：绝对 base（让 dev_preview_proxy 在 :2025 路由 /openlist-ui/*）
  // 生产：相对 './'（Android WebView file:// 协议加载）
  base: process.env.VITE_BASE || './',
  ...
  server: {
    port: 5174,
    proxy: {
      '/openlist-spa': {
        target: 'http://127.0.0.1:5244',
        rewrite: (path) => path.replace(/^\/openlist-spa/, ''),
      },
    },
  },
})
```

**关键 base 行为**：
- 沙箱 dev 设 `VITE_BASE=/openlist-ui/` → HTML 内 `<base href="/openlist-ui/">`，vite 处理 `/openlist-ui/*` prefix
- 生产不设 → HTML 内 `<base href="./">`，Android file:// 协议加载相对资源
- 同一份 config 同时支持两种模式，不引入分支 vite config

### 11.4 dev_preview_proxy 路由表（最终版）

```
Browser
  ↓
16000 agent-tool-host
  ↓ (白名单转发)
2025 encv-go dev_preview_proxy
  ├─ /openlist-ui/*    → :5174 (plugin-openlist vite, Vue + Ionic, base=/openlist-ui/)
  ├─ /openlist-spa/*   → :5174 (plugin-openlist vite 内部 proxy → :5244)
  ├─ /__openlist-health → :5174 (plugin-openlist vite 自定义 Node middleware)
  ├─ /api/*            → :5244 (OpenList backend 真实 API, reverse proxy 不是 mock)
  └─ /*                → :5173 (encv-mobile vite, Capacitor app 主前端)
```

**`/openlist-spa/*` 和 `/__openlist-health` 必须路由到 :5174**——不是 `/api/*` 也不是其他。plugin-openlist 内部 axios 用 `/openlist-spa/...` 当 baseURL（vite proxy rewrite 后打到 :5244 的 `/api/...`），健康检查用 `/__openlist-health`（Node 直连 :5244）。

### 11.5 验证命令

```bash
# 1. plugin-openlist vite 起来（沙箱 dev 模式必须设 VITE_BASE）
cd /workspace/app/encv-mobile/plugin-openlist/web
VITE_BASE=/openlist-ui/ pnpm dev --host 127.0.0.1 --port 5174 --strictPort
# 期望: "VITE v8.0.16 ready" + "Local: http://127.0.0.1:5174/openlist-ui/"

# 2. 端到端
curl -s http://127.0.0.1:5174/openlist-ui/                      # 200 HTML base=/openlist-ui/
curl -s http://127.0.0.1:5174/__openlist-health                  # {"alive":true,...}
curl -s http://127.0.0.1:5174/openlist-spa/api/public/settings   # 真 backend settings
curl -s http://127.0.0.1:16000/openlist-ui/                      # 走沙箱链 16000→2025→5174
curl -s http://127.0.0.1:16000/__openlist-health                 # 同上
curl -s http://127.0.0.1:16000/openlist-spa/api/public/settings  # 同上
```

### 11.6 错误诊断速查

| 现象 | 错配的路由 | 正确路由 |
|------|----------|---------|
| "Failed fetching settings: home.Network Error" | `/openlist-ui/*` → :3000 (Hi-Sillot SolidJS) | → :5174 (plugin-openlist Vue+Ionic) |
| `/openlist-spa/*` 返 encv-mobile HTML | NoRoute 兜底到 :5173 encv-mobile vite | → :5174 plugin-openlist vite |
| `/__openlist-health` 返 encv-mobile HTML | 同上 | → :5174 plugin-openlist vite |
| `import.meta.env.DEV` 判断错误 | plugin-openlist 在 prod build 时 | vite 正常处理（dev=true 仅当 vite dev mode） |
| backend :5244 启动 panic "index.html not exist" | binary go:embed 嵌入空 dist | config.json 设 `"dist_dir": "<absolute path>/public/dist"` |

### 11.7 ENCV 沙箱 dev preview 不要同时启这两个

- **必须启**：plugin-openlist/web (:5174)
- **不要启**：Hi-Sillot-OpenList-Frontend (:3000)

`dev-openlist.sh`（启动 OpenList binary + 它的 dist）里会触碰 Hi-Sillot-OpenList-Frontend 的 `dist/`，但那是作为 binary 启动的**前置**（生产 dist 源），不是 ENCV 沙箱 dev 入口。

`start-preview.sh` 现在正确只起 :5174，不再起 :3000。
