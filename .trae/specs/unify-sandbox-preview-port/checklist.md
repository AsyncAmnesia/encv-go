# Checklist

> Spec: [spec.md](file:///workspace/.trae/specs/unify-sandbox-preview-port/spec.md)
> Tasks: [tasks.md](file:///workspace/.trae/specs/unify-sandbox-preview-port/tasks.md)
>
> **端口注意**：所有 curl 验证命令使用 `:5173` (preview-gateway 本地端口) 和 `:16000` (外网端口)。**不使用 :16001** (spec 已废弃)。

按 spec.md §八 判据 J1-J14 逐项验证。

---

## 网关基础（J1）

- [ ] preview-gateway 项目独立 `package.json` 包含 `http-proxy` + TypeScript
- [ ] `pnpm install` 在 app/preview-gateway 干净通过
- [ ] `pnpm build` 产出 `dist/server.js`
- [ ] `node dist/server.js` 启动监听 **:5173** 无 panic
- [ ] 启动日志包含"replaces vite old port" + 路由表 + 4 个 upstream URL
- [ ] `curl -sI http://localhost:5173/` 返回 200 + HTML（fallthrough 到 vite :5175）

## HTTP 路由（J2-J5）

- [ ] :5173/ 转发到 :5175（encv-mobile SPA），返回 200 + HTML
- [ ] :5173/src/main.ts 转发到 :5175，返回 200 + JavaScript
- [ ] :5173/openlist-ui/ 转发到 :5174（plugin-openlist-web），返回 200 + HTML
- [ ] :5173/openlist-ui/src/main.ts 转发到 :5174，返回 200 + JavaScript
- [ ] :5173/api/public/settings 转发到 :2025（encv-go），返回 200
- [ ] :5173/p/... 转发到 :2025，返回 200
- [ ] :5173/play/... 转发到 :2025，返回 200
- [ ] :5173/openlist/sites/local/api/public/settings 转发到 :2025 → :5244，返回 200
- [ ] **外网 :16000/** 走 agent-tool-host → :5173 网关 → :5175 vite，返回 200 + HTML（J2）
- [ ] **外网 :16000/openlist-ui/** 走网关 → :5174，返回 200 + HTML（J3）
- [ ] **外网 :16000/api/public/settings** 走网关 → :2025，返回 200（J4）
- [ ] **外网 :16000/openlist/sites/local/api/public/settings** 走网关 → :2025 → :5244，返回 200（J5）
- [ ] 任意 upstream 不可达时返回 502 + JSON `{ error, upstream, target, path, hint }`

## WebSocket 转发（J6）

- [ ] `Upgrade: websocket` 头请求 :5173/?token=... 被转发到 ws://:5175/?token=...，返回 101
- [ ] `Upgrade: websocket` 头请求 :5173/openlist-ui/?token=... 被转发到 ws://:5174/?token=...，返回 101
- [ ] vite HMR 在浏览器 :16000/:5173 下能正常推送（Vite 客户端 ws 握手成功）

## /__gateway/health（J7）

- [ ] GET :5173/__gateway/health 返回 200 + JSON
- [ ] JSON 含 4 个 upstream（encv-mobile :5175 / plugin-openlist-web :5174 / encv-go :2025 / openlist :5244）
- [ ] 每个 upstream 含 url / alive / latency_ms 字段
- [ ] 3s timeout 不挂起（upstream 不可达时 alive=false，3s 内返回）

## pm2 集成

- [ ] `ecosystem.config.cjs` 含 preview-gateway app（PORT=5173）
- [ ] `pm2 start ecosystem.config.cjs` 成功拉起 5+ 个进程（preview-gateway / start-preview / plugin-openlist-vite / encv-go / openlist）
- [ ] `pm2 kill && pm2 start` 重启后 preview-gateway 自动恢复（J10）
- [ ] `setup-sandbox-env.sh` 含 preview-gateway 启动步骤
- [ ] 启动顺序：preview-gateway 先于 vite 起来（避免 :5173 端口冲突）

## vite 端口搬到 :5175（J13）

- [ ] `app/encv-mobile/scripts/start-preview.sh` 中 vite 启动命令改 `--port 5175 --strictPort`
- [ ] `app/encv-mobile/package.json` 中 dev 脚本如有硬编码 :5173 一并改
- [ ] `curl -sI http://localhost:5175/` 返回 200 + vite HTML
- [ ] `curl -sI http://localhost:5173/` 拿到的是 preview-gateway 自己的响应（不是 vite）
- [ ] pm2 重启后 `ss -tlnp | grep :5175` 看到 vite 监听
- [ ] `ss -tlnp | grep :5173` 看到 preview-gateway 监听（不是 vite）

## vite.config.ts 撤销所有反向代理胶水（D9 决策，J11-J12）

- [ ] vite.config.ts 不含 `cors: { origin: '*' }` 硬编码
- [ ] vite.config.ts 不含 `server.proxy: { ... }` 块（J11）
- [ ] vite.config.ts 不含 `openlistUiProxy` plugin 引用（J12）
- [ ] vite 默认 `cors: true`（回显 Origin）
- [ ] :5173 访问时 Origin 头 = `http://localhost:5173`
- [ ] vite 响应 `Access-Control-Allow-Origin: http://localhost:5173`（匹配）
- [ ] ESM `import()` 在 :5173 origin 下成功

## agent-tool-host 配置未改（J14）

- [ ] `curl -sI http://<sandbox>:16000/` 拿到 200 + 转发到网关后的内容
- [ ] 沙箱外用户通过 :16000 仍可访问主 app + openlist-ui + api 等全部上游
- [ ] agent-tool-host 进程未重启、未重新配置

## 端到端浏览器实测（J8-J9）

- [ ] agent-browser open :16000/tabs/openlist 看到 OpenListView 组件 mount
- [ ] snapshot 显示 "OpenList 管理" 标题 + 状态卡 + 提示信息
- [ ] 控制台无 "Failed to fetch" 错误
- [ ] :16000/openlist-ui/ 浏览器直接访问能进入 OpenList Web SPA
- [ ] ENCV 加密视频在 :16000/openlist-ui/ 内能预览（需 .sccgv 测试文件，可选）

## 文档（J6-Task 6）

- [ ] app/preview-gateway/README.md 含架构图（4 upstream → :5173 网关 → 浏览器；外网 :16000 → :5173 网关）
- [ ] README 含启动步骤
- [ ] README 含故障排查指南（502 / ws 失败 / CORS 异常 / vite 端口冲突）
- [ ] README 含端口决策说明（接管 :5173 的理由 / agent-tool-host 零改动）
- [ ] README 与 spec.md D1-D10 决策一致
