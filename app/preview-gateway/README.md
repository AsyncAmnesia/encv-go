# preview-gateway

> **沙箱统一预览网关**：单端口 `:16666` 接管 encv-mobile / plugin-openlist-web / encv-go 跨上游转发，让 vite 退回纯净 SPA dev server。

---

## 一、它在生态里的位置

```
外网用户 / 本地 dev / agent-browser
  ↓
:16666  (preview-gateway, 唯一对外入口)
  ├── /             → :8100  encv-mobile Vite (纯净 SPA)
  ├── /openlist-ui/ → :5174  plugin-openlist-web Vite
  ├── /openlist/    → :2025  encv-go (Go) → :5244  OpenList fork
  ├── /api          → :2025  encv-go
  ├── /p/           → :2025  encv-go
  ├── /play         → :2025  encv-go
  └── /__gateway/health   ← 自带健康检查

升级链路（vite HMR）:
  :16666 ws upgrade → :8100 ws (主 app HMR)
  :16666 ws upgrade → :5174 ws (plugin HMR)

外网入口（Trae IDE 集成）:
  :16000 (OpenPreview / agent-tool-host)
    ↓ 首次用 OpenPreview(preview_url="http://localhost:16666/") 注册后
  :16666 (preview-gateway)
```

**核心收益**：
- 外网 / 本地 / agent-browser 全部走单端口 `:16666`
- vite :8100 是纯净 SPA dev server，零反向代理胶水
- 网关层 `changeOrigin: false` 透传 Origin 头，Vite 默认 `cors: true` reflect Origin 天然通过 CORS

---

## 二、为什么独立项目

之前 vite 内部承担了 4 个上游的反向代理职责（`/api` → :2025、`/openlist/` → :2025、`/openlist-ui/*` 静态重写 + 代理），单点职责过重，配置胶水累积。本 spec 把这部分职责拆出到独立 Node 项目：

| 关注点 | 之前（vite 承担） | 现在（preview-gateway 承担） |
|--------|------------------|----------------------------|
| 跨上游路由 | `vite.config.ts` 的 `server.proxy` 块 | `server.ts` 的 `UPSTREAMS` 列表 |
| `/openlist-ui/*` 静态重写 | `openlistUiProxy` plugin 60 行 | `:5174` plugin-openlist-web 自己用 `VITE_BASE=/openlist-ui/` 处理 |
| CORS `'*'` 硬编码 | vite config | 不需要（Vite 默认 reflect Origin） |
| HMR WebSocket | vite 自处理 | 网关 `upgrade` 事件 → `proxy.ws()` |

---

## 三、路由表

| 路径 | 目标 | 用途 |
|------|------|------|
| `/` | `http://127.0.0.1:8100` | encv-mobile Vite SPA（默认 fallthrough） |
| `/openlist-ui` | `http://127.0.0.1:5174` | plugin-openlist-web Vite（OpenList 管理 UI） |
| `/openlist/` | `http://127.0.0.1:2025` | encv-go → OpenList 运行时数据路径 |
| `/api` | `http://127.0.0.1:2025` | encv-go API（service-guard / config / 上传等） |
| `/p/` | `http://127.0.0.1:2025` | encv-go 公开路径 |
| `/play` | `http://127.0.0.1:2025` | encv-go 媒体播放路径 |
| `/__gateway/health` | 内置 | 并发探测所有 upstream 返回 200 / 503 |
| `/__gateway` | 内置 | 静态 banner 端点 |

匹配顺序：先匹配具体路径，否则 fallthrough 到 `/`（encv-mobile）。

---

## 四、端口决策 `:16666`

| 候选 | 状态 | 原因 |
|------|------|------|
| `:16000` | ❌ 占用 | `agent-tool-host` (pid 821) 监听；与 OpenPreview 工具冲突 |
| `:5173` | ❌ 已废弃 | vite 老端口；语义冲突（vite 应在 :8100） |
| `:8100` | ❌ 占用 | encv-mobile Vite；不能让网关抢占 |
| `:16666` | ✅ 选用 | 用户决策"好记"；独立、易记、不与现有冲突 |

---

## 五、配置

通过环境变量：

| 变量 | 默认 | 说明 |
|------|------|------|
| `PORT` | `16666` | 监听端口 |
| `HOST` | `0.0.0.0` | 监听地址 |

---

## 六、启动 / 停止

通过 pm2 统一管理（见 `/workspace/ecosystem.config.cjs`）：

```bash
# 启动所有（含 preview-gateway）
pm2 start /workspace/ecosystem.config.cjs

# 单独管理 preview-gateway
pm2 status preview-gateway
pm2 logs preview-gateway
pm2 restart preview-gateway
pm2 stop preview-gateway
```

通过 `scripts/previews.sh` 包装：

```bash
bash /workspace/scripts/previews.sh start    # 启动全部
bash /workspace/scripts/previews.sh status  # 查看状态
bash /workspace/scripts/previews.sh logs    # 看日志
bash /workspace/scripts/previews.sh restart # 重启全部
```

---

## 七、外网访问（OpenPreview）

**首次**外网访问必须用 OpenPreview 工具显式注册 `:16666`：

```python
OpenPreview(
    command_id="<某个运行中的 command_id>",
    preview_url="http://localhost:16666/"
)
```

注册成功时 `agent-tool-host` 日志：

```
[PreviewManager] Port registered: 16666 (..., old_default: 5173, new_default: 16666, total_ports: 2)
[open_preview] OpenPreview registered successfully port=16666
```

之后 `curl http://localhost:16000/` 才会被转发到 `:16666` → vite。

**为什么不能自动注册**：实测发现 agent-tool-host 的 preview-proxy 内部 `:80` register 端点 `requires_auth=true`，普通 HTTP 请求 401 拒绝。只有 `OpenPreview` 工具（IDE 内部命令）能完成 register。

---

## 八、健康检查

```bash
curl -s http://localhost:16666/__gateway/health | jq .
```

返回示例：

```json
{
  "ok": true,
  "upstreams": {
    "encv-mobile":         { "url": "http://127.0.0.1:8100", "alive": true,  "latency_ms": 6 },
    "plugin-openlist-web": { "url": "http://127.0.0.1:5174", "alive": true,  "latency_ms": 11 },
    "encv-go":             { "url": "http://127.0.0.1:2025", "alive": true,  "latency_ms": 2 }
  }
}
```

`ok: false` 时检查对应 upstream 是否在跑（`ss -tlnp | grep :<port>`）。

---

## 九、故障排查

### Q1: `curl :16666/` 返回 502

上游不可达。看 `pm2 logs preview-gateway` 找到具体 upstream（`encv-mobile` / `plugin-openlist-web` / `encv-go`），然后用 `pm2 status` / `ss -tlnp` 确认对应端口在跑。

### Q2: 浏览器 ESM import() 报 "Failed to fetch dynamically imported module"

CORS 不通过。检查：

1. `curl -I http://localhost:16666/` 看响应头是否有 `Access-Control-Allow-Origin: http://localhost:16666`（vite reflect Origin）
2. 如果没有，确认你访问的是 `:16666` 而不是 `:16000`（未注册时 :16000 走 :5173 默认 upstream）
3. 如果有但浏览器仍报，检查 `:16666` 是否被中间代理（如 `agent-tool-host`）改写了 Origin

### Q3: Vite HMR 不工作

确认 ws upgrade 路径：

```bash
node -e '
const http = require("http");
const req = http.request({
  hostname: "localhost", port: 16666, path: "/?token=test",
  headers: { Connection: "Upgrade", Upgrade: "websocket",
             "Sec-WebSocket-Key": "dGhlIHNhbXBsZSBub25jZQ==",
             "Sec-WebSocket-Version": "13",
             "Sec-WebSocket-Protocol": "vite-hmr" }
});
req.on("upgrade", (res) => { console.log("✅ 101 UPGRADE"); process.exit(0); });
req.on("error", (e) => { console.log("✗", e.message); process.exit(1); });
req.end();
'
```

期望 `✅ 101 UPGRADE`。

### Q4: /openlist-ui/ 看到空白页

确认 plugin-openlist-web :5174 启动时设了 `VITE_BASE=/openlist-ui/`：

```bash
# dev-openlist-web.sh 应在 vite 启动时设此环境变量
grep VITE_BASE /workspace/app/encv-mobile/scripts/dev-openlist-web.sh
```

否则 :5174 会按相对 base 加载，引用 `/assets/...` 解析到 :8100 主 app。

### Q5: 撤销 `cors: '*'` 后 :16666 仍 CORS 通过？

Vite 8 默认 `cors: true` 会 reflect Origin。preview-gateway `changeOrigin: false` 透传 Origin 头，Vite 看到 `Origin: http://localhost:16666`，回 `Access-Control-Allow-Origin: http://localhost:16666`，浏览器 CORS 匹配，天然通过。

---

## 十、相关 spec

- 主 spec: [`.trae/specs/unify-sandbox-preview-port/spec.md`](file:///workspace/.trae/specs/unify-sandbox-preview-port/spec.md) — 完整决策、风险、J1-J14 验证
- 上游 spec: [`.trae/specs/openlist-frontend-extraction-and-sandbox-preview/`](file:///workspace/.trae/specs/openlist-frontend-extraction-and-sandbox-preview/spec.md) — OpenList 前端抽取
- 运行时 spec: [`.trae/specs/wire-openlist-runtime-and-ui-v2/`](file:///workspace/.trae/specs/wire-openlist-runtime-and-ui-v2/spec.md) — OpenList 运行时 + UI 集成
