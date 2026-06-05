# Tasks

> Spec: [spec.md](file:///workspace/.trae/specs/unify-sandbox-preview-port/spec.md)
> 目标：preview-gateway 监听 :16666（用户决策 D1："好记"），外网链 `外网 → :16000 (OpenPreview) → :16666 (preview-gateway) → 4 个 upstream`，本地 dev / agent-browser 直接访问 :16666。Vite 监听 :8100（vite.config.ts 已声明），是 preview-gateway 的 fallthrough 上游。

---

- [ ] **Task 1**: 改造 preview-gateway 监听 :16666
  - [ ] SubTask 1.1: 修改 `app/preview-gateway/src/server.ts` 默认监听端口 `:16000` → `:16666`
  - [ ] SubTask 1.2: 修改 fallthrough 目标 `:5173` → `:8100`（vite 新端口）
  - [ ] SubTask 1.3: 修改 WebSocket upgrade 转发目标 `:5173` → `:8100`
  - [ ] SubTask 1.4: 重新 `pnpm build` 验证编译通过
  - [ ] SubTask 1.5: 修改启动日志（"D1: 好记"）

- [ ] **Task 2**: start-preview.sh 显式指定 vite 监听 :8100
  - [ ] SubTask 2.1: 找到 `app/encv-mobile/scripts/start-preview.sh` 中 vite 启动命令
  - [ ] SubTask 2.2: 把 `--port 5174 --strictPort` 等显式参数改为 `--port 8100 --strictPort`
  - [ ] SubTask 2.3: 确认 pm2 重启后 vite 在 :8100 监听
  - [ ] SubTask 2.4: 现有 pm2 进程 :5173 占用需要先 kill（避免端口冲突）

- [ ] **Task 3**: 撤销 vite.config.ts 的所有反向代理胶水（D9 决策）
  - [ ] SubTask 3.1: 撤销 `cors: { origin: '*' }` 硬编码
  - [ ] SubTask 3.2: 删除 `server.proxy: { '/api': 2025, '/openlist/': 2025, '/p': 2025, '/play': 2025 }` 块
  - [ ] SubTask 3.3: 移除 `openlistUiProxy` plugin 引用
  - [ ] SubTask 3.4: 端到端验证：:16666/tabs/openlist 仍能加载 OpenListView

- [ ] **Task 4**: pm2 注册 preview-gateway 进程（端口 :16666）
  - [ ] SubTask 4.1: `ecosystem.config.cjs` 加 `preview-gateway` app（PORT=16666）
  - [ ] SubTask 4.2: `setup-sandbox-env.sh` 加启动步骤（`pnpm install` + `pnpm build` + `pm2 start`）
  - [ ] SubTask 4.3: 启动顺序：preview-gateway 必须在 vite 之前起来（先占 :16666，vite 监听 :8100 无冲突）

- [ ] **Task 5**: 端到端验证（spec.md §八 J1-J14）
  - [ ] SubTask 5.1: J1-J5 本地 :16666 网关 + 4 上游
  - [ ] SubTask 5.2: J6 WebSocket upgrade 转发到 :8100 vite
  - [ ] SubTask 5.3: J7 health 端点
  - [ ] SubTask 5.4: J8-J9 agent-browser 浏览器实测 :16666/tabs/openlist
  - [ ] SubTask 5.5: J10 pm2 重启自愈
  - [ ] SubTask 5.6: J11-J12 vite.config.ts 干净度
  - [ ] SubTask 5.7: J13 vite :8100 + :5173 端口空闲
  - [ ] SubTask 5.8: J14 agent-browser navigate :16666 触发 preview-proxy 自动注册（关键链路验证）

- [ ] **Task 6**: 文档（app/preview-gateway/README.md）
  - [ ] SubTask 6.1: 架构图（4 upstream → :16666 网关 → 浏览器；外网 :16000 → :16666 网关）
  - [ ] SubTask 6.2: 启动步骤（`pnpm install` + `pnpm build` + `pm2 start`）
  - [ ] SubTask 6.3: 故障排查（upstream 502 / ws upgrade 失败 / CORS 异常 / 端口冲突）
  - [ ] SubTask 6.4: 端口决策说明（:16666 vs :16000 vs :5173 vs :8100）

---

# Task Dependencies

- [Task 1] depends on 现有 preview-gateway 项目
- [Task 2] depends on [Task 1]（先改 gateway 端口，vite 才能搬到 :8100）
- [Task 3] depends on [Task 2]（vite 配置改完）
- [Task 4] depends on [Task 1]（pm2 基于新端口的 gateway）
- [Task 5] depends on [Task 3] + [Task 4]
- [Task 6] depends on [Task 5]

# Parallelizable

- [Task 6] 文档可与 [Task 5] 部分并行（架构图和启动步骤不依赖验证结果）
