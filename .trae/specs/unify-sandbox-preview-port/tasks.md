# Tasks

> Spec: [spec.md](file:///workspace/.trae/specs/unify-sandbox-preview-port/spec.md)
> 目标：构建 preview-gateway Node 项目，对外 :16001 单端口（独立于 :16000 OpenPreview）统一代理 4 个 upstream。
> **注意**：`:16000` 已被 `agent-tool-host` (OpenPreview) 占用且不可让出，本任务不与它争抢。

---

- [x] **Task 1**: 创建 preview-gateway 项目骨架（独立 npm 项目）
  - [x] SubTask 1.1: `app/preview-gateway/` 目录初始化
  - [x] SubTask 1.2: `package.json`（name: `preview-gateway`、type: module、dependencies: `http-proxy` + `ws`、scripts: `dev`/`start`）
  - [x] SubTask 1.3: `tsconfig.json`（Node 22 + ESM + 严格模式）
  - [x] SubTask 1.4: `.gitignore`（`node_modules/`、`*.log`、`dist/`）

- [x] **Task 2**: 实现 HTTP 路由分发（src/server.ts）
  - [x] SubTask 2.1: 路由表 routes[] 数组（5 个上游）
  - [x] SubTask 2.2: HTTP request handler：先匹配 routes[]，否则 fallthrough 到主 app :5173
  - [x] SubTask 2.3: error handler：502 + JSON `{ error, upstream, target, path, hint }`
  - [x] SubTask 2.4: 启动日志：监听端口 + 路由表 + 4 个 upstream URL
  - [x] SubTask 2.5: **端口从 :16000 改为 :16001**（D1 决策）

- [x] **Task 3**: 实现 WebSocket 转发（HMR 关键）
  - [x] SubTask 3.1: 监听 `server.on('upgrade')` 事件
  - [x] SubTask 3.2: 根据 URL 前缀分发到 :5173 / :5174
  - [x] SubTask 3.3: ws 升级失败兜底（500 + close）
  - [x] SubTask 3.4: 改用 `http-proxy` 的 `ws: true` 模式（D6 决策）

- [x] **Task 4**: 实现 /__gateway/health 端点
  - [x] SubTask 4.1: 并发 ping 4 个 upstream（GET / HEAD / ping）
  - [x] SubTask 4.2: 返回 `{ ok, upstreams: { name: { url, alive, latency_ms } } }`
  - [x] SubTask 4.3: 3s timeout 防止 health 检查挂起

- [x] **Task 5**: pm2 注册 preview-gateway 进程（端口 :16001）
  - [x] SubTask 5.1: `ecosystem.config.cjs` 加 `preview-gateway` app（cwd: `app/preview-gateway`、script: `dist/server.js`、port: 16001）
  - [x] SubTask 5.2: `setup-sandbox-env.sh` 加启动步骤（`pnpm install` + `pnpm build` + `pm2 start`）

- [ ] **Task 6**: 撤销 vite.config.ts 的所有反向代理胶水（D9 决策）
  - [ ] SubTask 6.1: 撤销 `cors: { origin: '*' }` 硬编码
  - [ ] SubTask 6.2: 删除 `server.proxy: { '/api': 2025, '/openlist/': 2025, '/p': 2025, '/play': 2025 }` 块
  - [ ] SubTask 6.3: 移除 `openlistUiProxy` plugin 引用
  - [ ] SubTask 6.4: 端到端验证：:16001/tabs/openlist 仍能加载 OpenListView（vite 转发改由 :16001 网关接管）

- [ ] **Task 7**: 端到端验证（spec.md §八 J1-J12）
  - [ ] SubTask 7.1: J1-J5 curl 自动化测试（端口 :16001）
  - [ ] SubTask 7.2: J6 WebSocket upgrade 测试
  - [ ] SubTask 7.3: J7 health 端点测试
  - [ ] SubTask 7.4: J8-J9 agent-browser 浏览器实测（:16001/tabs/openlist）
  - [ ] SubTask 7.5: J10 pm2 重启自愈测试
  - [ ] SubTask 7.6: J11-J12 vite.config.ts 干净度检查（无 proxy 块 + 无 openlistUiProxy）

- [ ] **Task 8**: 文档（app/preview-gateway/README.md）
  - [ ] SubTask 8.1: 架构图（4 upstream → :16001 网关 → 浏览器；外网 :16000 → :5173 主 app）
  - [ ] SubTask 8.2: 启动步骤（`pnpm install` + `pnpm build` + `pm2 start`）
  - [ ] SubTask 8.3: 故障排查（upstream 502、ws upgrade 失败、CORS 异常、:16000 vs :16001 选择）
  - [ ] SubTask 8.4: 沙箱限制说明（:16000 被 agent-tool-host 占用，D9 决策依据）

---

# Task Dependencies

- [Task 2] depends on [Task 1]
- [Task 3] depends on [Task 2]
- [Task 4] depends on [Task 2]
- [Task 5] depends on [Task 4]（pm2 启动前需要 health 端点可用）
- [Task 6] depends on [Task 5]（先有网关才能撤 vite 反向代理）
- [Task 7] depends on [Task 6]
- [Task 8] depends on [Task 7]（文档基于验证结果）

# Parallelizable

- [Task 3] 和 [Task 4] 互相独立（都依赖 Task 2），可并行
- [Task 8] 文档可与 [Task 7] 部分并行（架构图和启动步骤不依赖验证结果）
