# Checklist

> Spec: [spec.md](file:///workspace/.trae/specs/unify-sandbox-preview-port/spec.md)
> Tasks: [tasks.md](file:///workspace/.trae/specs/unify-sandbox-preview-port/tasks.md)
>
> **端口注意**：所有 curl 验证命令使用 `:16001`（preview-gateway）；**不使用 `:16000`**（被 agent-tool-host 占用）。

按 spec.md §八 判据 J1-J12 逐项验证。

---

## 网关基础

- [ ] preview-gateway 项目独立 `package.json` 包含 `http-proxy` + `ws` + TypeScript
- [ ] `pnpm install` 在 app/preview-gateway 干净通过
- [ ] `pnpm build` 产出 `dist/server.js`
- [ ] `node dist/server.js` 启动监听 :16001 无 panic
- [ ] 启动日志包含监听端口 + 路由表 + 4 个 upstream URL

## HTTP 路由

- [ ] :16001/ 转发到 :5173（encv-mobile SPA），返回 200 + HTML
- [ ] :16001/src/main.ts 转发到 :5173，返回 200 + JavaScript
- [ ] :16001/openlist-ui/ 转发到 :5174（plugin-openlist-web），返回 200 + HTML
- [ ] :16001/openlist-ui/src/main.ts 转发到 :5174，返回 200 + JavaScript
- [ ] :16001/api/public/settings 转发到 :2025（encv-go），返回 200
- [ ] :16001/p/... 转发到 :2025，返回 200
- [ ] :16001/play/... 转发到 :2025，返回 200
- [ ] :16001/openlist/sites/local/api/public/settings 转发到 :2025 → :5244，返回 200
- [ ] 任意 upstream 不可达时返回 502 + JSON `{ error, upstream, target, path, hint }`

## WebSocket 转发

- [ ] `Upgrade: websocket` 头请求 :16001/?token=... 被转发到 ws://:5173/?token=...，返回 101
- [ ] `Upgrade: websocket` 头请求 :16001/openlist-ui/?token=... 被转发到 ws://:5174/?token=...，返回 101
- [ ] vite HMR 在浏览器 :16001 下能正常推送（Vite 客户端 ws 握手成功）

## /__gateway/health

- [ ] GET :16001/__gateway/health 返回 200 + JSON
- [ ] JSON 含 4 个 upstream（encv-mobile / plugin-openlist-web / encv-go / openlist）
- [ ] 每个 upstream 含 url / alive / latency_ms 字段
- [ ] 3s timeout 不挂起（upstream 不可达时 alive=false，3s 内返回）

## pm2 集成

- [ ] `ecosystem.config.cjs` 含 preview-gateway app（PORT=16001）
- [ ] `pm2 start ecosystem.config.cjs` 成功拉起 5 个进程（preview-gateway / start-preview / plugin-openlist-vite / encv-go / openlist）
- [ ] `pm2 kill && pm2 start` 重启后 preview-gateway 自动恢复
- [ ] `setup-sandbox-env.sh` 含 preview-gateway 启动步骤

## vite.config.ts 撤销所有反向代理胶水（D9 决策）

- [ ] vite.config.ts 不含 `cors: { origin: '*' }` 硬编码（J6/J9）
- [ ] vite.config.ts 不含 `server.proxy: { ... }` 块（J11）
- [ ] vite.config.ts 不含 `openlistUiProxy` plugin 引用（J12）
- [ ] vite 默认 `cors: true`（回显 Origin）
- [ ] :16001 访问时 Origin 头 = `http://localhost:16001`
- [ ] vite 响应 `Access-Control-Allow-Origin: http://localhost:16001`（匹配）
- [ ] ESM `import()` 在 :16001 origin 下成功

## 端到端浏览器实测

- [ ] agent-browser open :16001/tabs/openlist 看到 OpenListView 组件 mount
- [ ] snapshot 显示 "OpenList 管理" 标题 + 状态卡 + 提示信息
- [ ] 控制台无 "Failed to fetch" 错误
- [ ] :16001/openlist-ui/ 浏览器直接访问能进入 OpenList Web SPA
- [ ] ENCV 加密视频在 :16001/openlist-ui/ 内能预览（需 .sccgv 测试文件，可选）

## 端口隔离验证

- [ ] :16000 仍由 agent-tool-host 监听（curl :16000/ 拿到 vite HTML，title "ENCV-go"）
- [ ] :16000 路径下 `/openlist-ui/` 返回 404 或 502（agent-tool-host 不转发到 preview-gateway）
- [ ] :16001 路径下所有路由可访问
- [ ] 沙箱外用户通过 :16000 仍可访问主 app（外网访问路径未破坏）

## 文档

- [ ] app/preview-gateway/README.md 含架构图（4 upstream → :16001 网关 → 浏览器；外网 :16000 → :5173 主 app）
- [ ] README 含启动步骤
- [ ] README 含故障排查指南（502 / ws 失败 / CORS 异常）
- [ ] README 含 :16000 vs :16001 端口选择说明
- [ ] README 含沙箱限制说明（:16000 被 agent-tool-host 占用）
- [ ] README 与 spec.md D1-D9 决策一致
