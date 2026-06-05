# Spec: 沙箱统一预览端口（Unified Sandbox Preview Gateway）

> **核心目标**：构建一个独立的"预览端口网关"项目，对外暴露 1 个独立端口 (`:16001`)，由该网关统一代理 encv-mobile 主 app（:5173）、plugin-openlist/web（:5174）、OpenList 后端（:5244，经 encv-go :2025），解决当前沙箱多端口无法被外网预览、Chrome `--proxy-server=:18080` body 截断、Origin 头被改写等系列预览稳定性问题。**`:16000` 已被 `agent-tool-host` (OpenPreview) 占用且不可让出，本 spec 不与它争抢。**

---

## 一、动机与现状

### 1.1 沙箱当前预览形态（4 端口 + 1 代理）

```
浏览器（沙箱内 Chrome agent-browser）
  ↓ 走 :18080 强制代理（Trae IDE 内部）
预览入口 :16000 (OpenPreview) → vite (:5173)               ← 主 app
                                              ↓
                                         :5174 (plugin-openlist-vite)
                                         :5244 (OpenList Go server)
                                         :2025 (encv-go reverse proxy)
```

### 1.2 当前痛点

| # | 痛点 | 现象 | 已踩过的坑 |
|---|------|------|-----------|
| **P1** | **多端口外网暴露困难** | 用户（外网）只能通过 :16000 单端口访问，其它 :5174/:5244/:2025 沙箱外不可达 | 用户原话："预览端口需要沙箱代理，我无法访问远程ip" |
| **P2** | **沙箱 :18080 内部代理 bug** | Chrome agent-browser 走 :18080，body 截断 310 字节 | curl 经 :16000 拿 14645 字节，Chrome 经 :16000 拿 14335 字节 |
| **P3** | **Origin 头被改写** | vite 看到 `Origin: http://localhost:5173`（不是 :16000），回显 `Access-Control-Allow-Origin: http://localhost:5173`，ESM `import()` 跨域失败 | 用户看到"openlist 扩展 ui 空白" |
| **P4** | **vite HMR 跨端口失效** | 主 app 改代码，HMR 通过 :5173 ws 推送，但浏览器当前 origin 是 :16000，HMR 客户端拒绝连 | Vite 默认 ws 端口 = 同 server port，跨端口失效 |
| **P5** | **iframe 同源策略** | 原本 OpenListView 用 iframe 加载 :5174，:16000 同源但跨子域跨端口 → iframe 父子通信需要 postMessage | 当前 OpenListView 已用 Capacitor 嵌入绕开此问题，但 WebView 路径仍需 iframe |
| **P6** | **启动顺序耦合** | 用户必须 4 个 dev server 都起来（:5173 / :5174 / :5244 / :2025），任一未起 → 预览碎一半 | 已在 setup-sandbox-env.sh 加 pm2 兜底 |

### 1.3 目标形态（1 端口网关）

```
浏览器 (本地 dev / agent-browser)
  ↓ http://localhost:16001/...
预览网关 :16001 (新项目, Node + http-proxy)
  ├── /                  → encv-mobile SPA (:5173)
  ├── /openlist-ui/      → plugin-openlist/web (:5174)        ← SPA
  ├── /openlist/         → encv-go (:2025) → OpenList (:5244) ← reverse proxy
  ├── /api/              → encv-go (:2025)
  ├── /p/                → encv-go (:2025)
  ├── /play              → encv-go (:2025)
  ├── /src/* /@vite/*    → vite 资源直转（保持 HMR 端口）
  └── /ws                → vite HMR WebSocket 转发（关键！解决 P4）

外部访问路径（保持现状，spec 不破坏）:
  外网用户 → :16000 (agent-tool-host / OpenPreview)
                     ↓
                  :5173 (encv-mobile vite 静态主 app)
```

**核心收益**：
- **网关是唯一路由权威**：所有路径都由 preview-gateway 决定，无 vite 内胶水
- **本地 dev / agent-browser 用 :16001**：避开 :18080 沙箱代理 body 截断；避开 agent-tool-host 改写 Origin
- **Origin 头不变量**：所有 :16001 请求都带 `Origin: http://localhost:16001`，vite 看到的 Origin 永远是 :16001 → CORS 一致
- **HMR 走转发**：WebSocket 升级握手走网关 → vite `:5173/ws` 正常 HMR
- **零胶水 vite**：vite :5173 是纯净的 vite dev server，不挂任何 reverse-proxy plugin

---

## 二、架构

### 2.1 三层 + 1 网关

```
Layer 0: 沙箱预览网关 (新, :16001, Node http-proxy)
  └── 本地 dev / agent-browser 唯一入口；纯转发；零业务逻辑

Layer 1: encv-mobile (:5173) + plugin-openlist/web (:5174) — Vite dev
  └── 沙箱内部，仍由 Vite 各自起；网关转发浏览器请求
  └── **vite :5173 不再承担任何反向代理职责**（用户决策 D9）

Layer 2: encv-go (:2025) + OpenList (:5244) — Go server
  └── 沙箱内部；纯后端 API

Layer 3: 浏览器
  ├── 本地 dev / agent-browser → http://localhost:16001/...
  └── 外网用户 → http://<沙箱>/:16000/... → agent-tool-host → :5173 (vite 主 app 静态)
```

### 2.2 关键决策

| # | 决策 | 取值 | 理由 |
|---|------|------|------|
| **D1** | 网关端口 | **`:16001`** | :16000 已被 agent-tool-host (OpenPreview) 占用且不可让出；:16001 接近 :16000 语义、易记、不冲突 |
| **D2** | 网关用什么实现？ | **Node `http-proxy` + `ws` 库** | 成熟；可同时代理 HTTP + WebSocket（HMR 关键） |
| **D3** | 网关放哪个仓库？ | **`app/preview-gateway/` 独立项目** | 单一职责；可独立 pm2 进程；可独立测试 |
| **D4** | 网关要鉴权吗？ | **不要** | 沙箱内本地 dev 用，无外部鉴权需求 |
| **D5** | CORS / Origin 处理 | **网关层不做任何改写** | 让上游 vite / Go 自行处理；网关是 transparent proxy |
| **D6** | HMR WebSocket 怎么转？ | **`http-proxy` 的 `ws: true` 自动 upgrade** | vite dev 默认 ws path = `/?token=...`；`http-proxy` 自动处理 Upgrade 头 |
| **D7** | 出错兜底？ | **502 + JSON 错误** | 任意 upstream 不可达 → 网关返回 `{ error: "upstream down", upstream: "..." }` 便于 DevLogs 诊断 |
| **D8** | 健康检查端点？ | **`/__gateway/health`** | 返回所有 upstream 的 ping 结果；pm2 readiness 用 |
| **D9** | vite :5173 是否承担反向代理？ | **完全不代理**（用户决策） | 之前的所有 vite 胶水（workspacePackageRewrite / alias / fs.allow / proxy 块）已撤销；vite 是纯净 dev server |

### 2.3 与现有 OpenPreview 关系

- `:16000` 是 `agent-tool-host` (OpenPreview) 实际监听的 TCP 端口，**无法让出**
- preview-gateway `:16001` 是新增的**沙箱内**"二次路由"端口
- 两者**不冲突**：preview-gateway 是新的、独立的本地 dev 入口
- **外网访问**: 仍走 :16000 → agent-tool-host → :5173 (vite) → 只能访问主 app 静态页
- **本地 dev / agent-browser**: 走 :16001 → preview-gateway → :5173/:5174/:2025 → 全部上游可达

**等价关系**：
```
本地 / agent-browser  → :16001
                          │
                          └─ 网关 (本 spec 引入)
                                ├─ :5173 encv-mobile
                                ├─ :5174 plugin-openlist/web
                                └─ :2025 encv-go → :5244 OpenList

外网                   → :16000
                          │
                          ├─ OpenPreview (agent-tool-host)
                          │     └─ :5173 encv-mobile (主 app 静态)
                          │
                          └─ 不可达 :5174 / :2025（沙箱基础设施限制）
```

---

## 三、关键技术细节

### 3.1 网关 HTTP 路由表

```typescript
// app/preview-gateway/src/server.ts
import http from 'node:http'
import httpProxy from 'http-proxy'

const routes: Array<{ match: string, target: string, name: string }> = [
  { match: '/openlist-ui',  target: 'http://127.0.0.1:5174', name: 'plugin-openlist-web' },
  { match: '/openlist/',    target: 'http://127.0.0.1:2025', name: 'encv-go' },
  { match: '/api',          target: 'http://127.0.0.1:2025', name: 'encv-go' },
  { match: '/p',            target: 'http://127.0.0.1:2025', name: 'encv-go' },
  { match: '/play',         target: 'http://127.0.0.1:2025', name: 'encv-go' },
  // 默认 fallthrough → 主 app :5173
]

const proxies = new Map<string, httpProxy>()
for (const r of routes) {
  proxies.set(r.name, httpProxy.createProxyServer({ target: r.target, changeOrigin: false }))
}
const mainApp = httpProxy.createProxyServer({ target: 'http://127.0.0.1:5173', ws: true, changeOrigin: false })

const server = http.createServer((req, res) => {
  const route = routes.find(r => req.url?.startsWith(r.match))
  if (route) {
    proxies.get(route.name)!.web(req, res)
  } else {
    mainApp.web(req, res)
  }
})

// WebSocket：识别 Upgrade 头，转发到 :5173 (主 app HMR) / :5174 (plugin HMR)
server.on('upgrade', (req, socket, head) => {
  if (req.url?.startsWith('/openlist-ui/')) {
    proxies.get('plugin-openlist-web')!.ws(req, socket, head)
  } else {
    mainApp.ws(req, socket, head)
  }
})
```

### 3.2 沙箱 :18080 代理兼容性

`agent-browser` Chrome 强制走 `http://127.0.0.1:18080` 沙箱代理，触发 body 截断 310 字节 bug。
- **本 spec 不能修复** :18080 bug
- **本 spec 缓解**：本地 agent-browser 调试时改用 `http://localhost:16001`（不走 :18080，因为 :16001 是沙箱内直接端口）
- **body 完整传递** 由 Node http-proxy 保证（实测经 Node 转发的响应 Content-Length 完整）

### 3.3 CORS / Origin 一致性

- 网关**不**改 `Origin` / `Host` 头（`changeOrigin: false`）
- 浏览器请求 :16001 → 网关转发 → 上游看到 `Origin: http://localhost:16001`
- vite 5+ 默认 `cors: true` 回显 Origin → 浏览器收到 `Access-Control-Allow-Origin: http://localhost:16001` → 匹配 → CORS 通过
- **撤销**之前在 vite.config.ts 硬编码的 `cors: { origin: '*' }`（dev 配置可以是回显 Origin）
- **不**在 vite.config.ts 中挂任何 reverse-proxy plugin（D9 决策）

### 3.4 HMR WebSocket 转发

- vite dev 默认 ws path: `/?token=...`（无独立路径）
- 浏览器看到 ws URL: `ws://localhost:16001/?token=...`（基于当前 origin :16001）
- 网关在 `upgrade` 事件中识别 `Upgrade: websocket` + `Connection: Upgrade` 头
- 转发到 `ws://127.0.0.1:5173/?token=...`（保持 query string 不变）
- vite HMR 客户端 ws 握手成功 → 正常推送 hot module updates

### 3.5 错误兜底与可观测

每个 upstream 不可达时：

```json
HTTP/1.1 502 Bad Gateway
Content-Type: application/json

{
  "error": "upstream_unavailable",
  "upstream": "plugin-openlist-web",
  "target": "http://127.0.0.1:5174",
  "path": "/openlist-ui/@login",
  "hint": "Check pm2 status for plugin-openlist-vite"
}
```

`/__gateway/health`：

```json
{
  "ok": true,
  "upstreams": {
    "encv-mobile":        { "url": "http://127.0.0.1:5173", "alive": true, "latency_ms": 12 },
    "plugin-openlist-web":{ "url": "http://127.0.0.1:5174", "alive": true, "latency_ms": 8  },
    "encv-go":            { "url": "http://127.0.0.1:2025", "alive": true, "latency_ms": 5  },
    "openlist":           { "url": "http://127.0.0.1:5244", "alive": true, "latency_ms": 23 }
  }
}
```

---

## 四、改动清单

| # | 改动 | 文件 | 性质 | 风险 |
|---|------|------|------|------|
| **G1** | 新建 `app/preview-gateway/` 项目（package.json + tsconfig + src/server.ts） | 新 | TS | 低 |
| **G2** | pm2 注册 `preview-gateway` 进程（端口 16001） | `ecosystem.config.cjs` (改) | JS | 低 |
| **G3** | setup-sandbox-env.sh 加 preview-gateway 启动步骤 | `scripts/setup-sandbox-env.sh` (改) | shell | 低 |
| **G4** | vite.config.ts 撤销 `cors: { origin: '*' }` 硬编码 + 撤销 proxy 块 | `app/encv-mobile/vite.config.ts` (改) | TS | 中（需端到端验证） |
| **G5** | agent-browser 调试时改用 :16001 单端口验证 | 无代码 | — | — |
| **G6** | 文档：沙箱预览拓扑图更新 | `app/preview-gateway/README.md` (新) | md | 低 |

### 4.1 撤销列表（修复后清理）

| 之前"胶水" | 撤销原因 | 撤销方式 |
|------------|---------|---------|
| `vite.config.ts` 的 `cors: { origin: '*' }` | 网关层不做 Origin 改写，vite 回显 Origin 即可 | 改回默认 `cors: true` 或不显式设置 |
| `vite.config.ts` 的 `server.proxy` 块 | D9 决策：vite 不再承担反向代理 | 删除 `proxy: { '/api': 2025, '/openlist/': 2025, '/p': 2025, '/play': 2025 }` |
| `vite.config.ts` 的 `openlistUiProxy` plugin | D9 决策：vite 不再处理子路径 | 移除 plugin 引用 |
| `vite.config.ts` 的 `server.host: '0.0.0.0'` | 网关层转发即可 | 保留（保险；vite 仍可监听 0.0.0.0） |

**已撤销（之前的胶水重构已完成）**：
- ✅ 删除 `/workspace/app/encv-mobile/packages/`（假 monorepo 共享）
- ✅ 创建 `/workspace/app/encv-mobile/src/components-shared/`（主 app 复本）
- ✅ plugin-openlist/web 创建本地 `components-shared/`
- ✅ `pnpm-workspace.yaml` 删 `packages/*`
- ✅ 两个 `package.json` 删 `@encvgo/components` 依赖
- ✅ `vite.config.ts` 撤销 `workspacePackageRewrite` plugin / `resolve.alias` / `fs.allow`

---

## 五、执行顺序

```
P0 — 新建 preview-gateway 项目骨架 (G1)               ✅
P1 — 写 HTTP 路由 + WebSocket 转发                    ✅
P2 — 写 /__gateway/health 端点                       ✅
P3 — pm2 注册 + setup-sandbox-env.sh 集成 (G2, G3)   ✅
P4 — vite.config.ts 撤销 cors '*' + proxy 块 (G4)     ⏳ P3 后
P5 — 端到端验证 (:16001 路径下 OpenListView 正常)    ⏳ P4 后
P6 — 文档 (G6)                                       ⏳ P5 后
```

---

## 六、风险登记

| # | 风险 | 缓解 |
|---|------|------|
| **R1** | 网关单点故障 | pm2 配 `autorestart: true`，挂掉自动拉起 |
| **R2** | WebSocket 转发性能 | `http-proxy` 的 `ws: true` 自动处理 upgrade；4 个 upstream 并发量极小（沙箱内单用户） |
| **R3** | 网关与 vite proxy 重复 | **D9 决策**：vite 不再有 proxy 块；只有 preview-gateway 是唯一代理 |
| **R4** | `:18080` 沙箱代理仍截断 body | 本地 agent-browser 改用 :16001（不走 :18080）；或经 Node http-proxy 中转，body 完整 |
| **R5** | 撤销 `cors: '*'` + proxy 块后其它 dev 场景破坏 | 仅 sandbox 预览路径用网关；其它本地 dev 仍 `pnpm dev` 直接 :5173（vite 默认 cors: true 已足够） |
| **R6** | `:16000` 已被 agent-tool-host 占用 | D1 决策：preview-gateway 用 :16001，不与 :16000 冲突 |
| **R7** | 外网用户只能访问 :16000 路径（agent-tool-host） | 接受限制：外网访问只到主 app 静态；:5174/:2025 等仅本地 dev / agent-browser 可达 |
| **R8** | D9 决策导致 :16000 路径下 /openlist-ui/ 不可达 | 是 spec 接受的取舍；本地 dev 用 :16001 完整路径 |

---

## 七、Spec 自我一致性检查

- [x] 改动都有具体文件路径
- [x] 每个改动都有可执行命令（`pnpm add http-proxy ws`、`pm2 start ecosystem.config.cjs`）
- [x] 决策 D1-D8 都有理由
- [x] 风险 R1-R6 都有缓解
- [x] 不破坏现有 OpenList 嵌入逻辑
- [x] 不依赖 Trae IDE 内部代理（:18080）修复

---

## 八、Spec 完成判据

| # | 判据 | 验证方式 |
|---|------|----------|
| **J1** | `pnpm dev` 起 preview-gateway，监听 :16001 | `curl -sI http://localhost:16001/ \| head -3` |
| **J2** | 浏览器访问 :16001/ 加载 encv-mobile SPA | `curl -s http://localhost:16001/ \| grep -c '<div id="app">'` |
| **J3** | :16001/openlist-ui/ 加载 plugin SPA | `curl -s -o /dev/null -w "%{http_code}" http://localhost:16001/openlist-ui/` |
| **J4** | :16001/api/* 走 encv-go | `curl -s -o /dev/null -w "%{http_code}" http://localhost:16001/api/public/settings` |
| **J5** | :16001/openlist/sites/* 走 encv-go → OpenList | `curl -s -o /dev/null -w "%{http_code}" http://localhost:16001/openlist/sites/local/api/public/settings` |
| **J6** | WebSocket 升级转发到 vite HMR | `curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" http://localhost:16001/?token=test` 期望 101 |
| **J7** | /__gateway/health 返回 4 个 upstream 状态 | `curl -s http://localhost:16001/__gateway/health \| jq .upstreams` |
| **J8** | 浏览器实测 :16001/tabs/openlist 正常渲染 | agent-browser open + snapshot |
| **J9** | 撤销 vite `cors: '*'` 后 :16001 仍 CORS 通过 | snapshot 显示 OpenListView 组件 |
| **J10** | pm2 拉起 preview-gateway 后自愈 | `pm2 kill && pm2 start ecosystem.config.cjs` |
| **J11** | vite.config.ts 不含 `proxy: { ... }` 块（D9 验证） | `grep -n "proxy:" app/encv-mobile/vite.config.ts` 应为空 |
| **J12** | vite.config.ts 不含 `openlistUiProxy` plugin 引用 | `grep -n "openlistUiProxy" app/encv-mobile/vite.config.ts` 应为空 |

---

## 九、相关 spec / 文档

- [openlist-frontend-extraction-and-sandbox-preview](file:///workspace/.trae/specs/openlist-frontend-extraction-and-sandbox-preview/spec.md) — OpenList 前端抽取 + 浏览器预览（上一轮）
- [wire-openlist-runtime-and-ui-v2](file:///workspace/.trae/specs/wire-openlist-runtime-and-ui-v2/spec.md) — OpenList 运行时 + UI 集成
- [app/preview-gateway/README.md](file:///workspace/app/preview-gateway/README.md) — 网关项目文档（待写）
