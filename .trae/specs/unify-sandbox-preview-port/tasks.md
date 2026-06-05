# Tasks

> Spec: [spec.md](file:///workspace/.trae/specs/unify-sandbox-preview-port/spec.md)
> 目标：preview-gateway **接管 :5173** 顶替 vite，统一对外（外网 :16000 + 本地 :5173）路由 4 个 upstream。Vite 搬离 :5173 到 :5175。
> **关键反转**：D1 决策从"用 :16001 新端口"改为"接管 :5173 老端口"，让 agent-tool-host 现有链路 :16000 → :5173 直接命中网关。

---

- [ ] **Task 1**: 重写 preview-gateway 项目以适配 :5173 端口
  - [ ] SubTask 1.1: 修改 `app/preview-gateway/src/server.ts` 默认监听端口 `:16001` → `:5173`
  - [ ] SubTask 1.2: 修改 fallthrough 目标 `:5173` → `:5175`（vite 新端口）
  - [ ] SubTask 1.3: 修改 WebSocket upgrade 转发目标 `:5173` → `:5175`
  - [ ] SubTask 1.4: 重新 `pnpm build` 验证编译通过
  - [ ] SubTask 1.5: 修改启动日志输出（"replaces vite old port"）

- [ ] **Task 2**: vite 端口从 :5173 搬到 :5175（D10 决策）
  - [ ] SubTask 2.1: 找到 vite 启动命令（`app/encv-mobile/scripts/start-preview.sh` 或 `package.json` 脚本）
  - [ ] SubTask 2.2: 修改启动参数 `--port 5173 --strictPort` → `--port 5175 --strictPort`
  - [ ] SubTask 2.3: 确认 pm2 重启后 vite 在 :5175 监听（pm2 kill + start 后 `ss -tlnp | grep 5175`）
  - [ ] SubTask 2.4: 检查 .air.conf / package.json / Dockerfile 是否有硬编码 :5173 一并改

- [ ] **Task 3**: pm2 注册 preview-gateway 进程（端口 :5173）
  - [ ] SubTask 3.1: `ecosystem.config.cjs` 加 `preview-gateway` app（cwd: `app/preview-gateway`、script: `dist/server.js`、port: 5173）
  - [ ] SubTask 3.2: `setup-sandbox-env.sh` 加启动步骤（`pnpm install` + `pnpm build` + `pm2 start`）
  - [ ] SubTask 3.3: 启动顺序：preview-gateway 先于 vite 起来（避免 :5173 端口冲突）

- [ ] **Task 4**: 撤销 vite.config.ts 的所有反向代理胶水（D9 决策）
  - [ ] SubTask 4.1: 撤销 `cors: { origin: '*' }` 硬编码
  - [ ] SubTask 4.2: 删除 `server.proxy: { '/api': 2025, '/openlist/': 2025, '/p': 2025, '/play': 2025 }` 块
  - [ ] SubTask 4.3: 移除 `openlistUiProxy` plugin 引用
  - [ ] SubTask 4.4: 端到端验证：:5173/tabs/openlist 仍能加载 OpenListView（vite 转发改由 :5173 网关接管）

- [ ] **Task 5**: 端到端验证（spec.md §八 J1-J14）
  - [ ] SubTask 5.1: J1-J2 本地 :5173 网关基础 + 主 app
  - [ ] SubTask 5.2: J3-J5 外网 :16000 路径下 4 个上游可达
  - [ ] SubTask 5.3: J6 WebSocket upgrade 转发到 :5175 vite
  - [ ] SubTask 5.4: J7 health 端点
  - [ ] SubTask 5.5: J8-J9 agent-browser 浏览器实测 :16000/tabs/openlist
  - [ ] SubTask 5.6: J10 pm2 重启自愈
  - [ ] SubTask 5.7: J11-J12 vite.config.ts 干净度
  - [ ] SubTask 5.8: J13-J14 vite :5175 + agent-tool-host 配置未改

- [ ] **Task 6**: 文档（app/preview-gateway/README.md）
  - [ ] SubTask 6.1: 架构图（4 upstream → :5173 网关 → 浏览器；外网 :16000 → :5173 网关）
  - [ ] SubTask 6.2: 启动步骤（`pnpm install` + `pnpm build` + `pm2 start`）
  - [ ] SubTask 6.3: 故障排查（upstream 502、ws upgrade 失败、CORS 异常、vite 端口冲突）
  - [ ] SubTask 6.4: 端口决策说明（接管 :5173 的理由 / agent-tool-host 零改动）

---

# Task Dependencies

- [Task 1] depends on 现有 preview-gateway 项目（已存在但端口是 :16001）
- [Task 2] depends on [Task 1]（先改 gateway 端口才能让 vite 搬离 :5173）
- [Task 3] depends on [Task 1]（pm2 启动基于新端口的 gateway）
- [Task 4] depends on [Task 3]（先有网关才能撤 vite 反向代理）
- [Task 5] depends on [Task 4]
- [Task 6] depends on [Task 5]（文档基于验证结果）

# Parallelizable

- [Task 6] 文档可与 [Task 5] 部分并行（架构图和启动步骤不依赖验证结果）
