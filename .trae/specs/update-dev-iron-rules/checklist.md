# Checklist

## 规则文档完整性

- [ ] 开发铁律规则文档已创建/更新（`.trae/rules/development.md` 或追加到现有 rules），包含：
  - [ ] 严禁 mock 大量 handle 的规定（>10 个 API 端点视为违规）
  - [ ] 严禁阻塞式服务启动的规定（必须后台运行）
  - [ ] Go 程序使用 `go run` 直接运行的规范
  - [ ] 端口必须正确的标准和端口分配表（2025/5173）
  - [ ] Capacitor 前后端预览的标准启动流程（3 步：后端→前端→Capacitor）

## Mock 代码清理验证

### handlers.ts 最小化验证

- [ ] `mock/handlers.ts` 总行数 **< 100 行**（目标：≤ 50 行）
- [ ] Mock API 端点数量 **≤ 3 个**（仅 health, config, plugins）
- [ ] 已删除以下 handler 函数：
  - [ ] `fileSystemHandler` (94-193 行) — 10 个文件系统 API
  - [ ] `fileContentHandler` (195-267 行) — 7 个文件内容 API
  - [ ] `taskMockHandler` (358-427 行) — 4 个任务管理 API
  - [ ] `staticFileHandler` (429-447 行) — 静态文件服务
  - [ ] `debugControlHandler` (449-481 行) — 调试控制接口
- [ ] `staticJsonHandler` 已精简：
  - [ ] 保留: `/health` (健康检查)
  - [ ] 保留: `/api/config` (返回空对象 `{}`)
  - [ ] 保留: `/api/plugins` (从 JSON 文件读取)
  - [ ] 已删除其余 17+ 个端点（container-versions, schema, ffmpeg-status, webdav/* 等）
- [ ] `dispatchRequest` 主函数中的特殊路由已删除：
  - [ ] `/decrypt` 处理逻辑
  - [ ] `/api/file/info` 处理逻辑（含 container 类型推断）
  - [ ] `/preview/*` PDF 预览 HTML 生成

### index.ts 更新验证

- [ ] `mock/index.ts` 不再引用已删除的 handler 函数
- [ ] `MOCK_API_PREFIXES` 数组已更新（仅保留必要的拦截路径）
- [ ] `shouldMockIntercept` 逻辑已调整（减少拦截范围）
- [ ] Mock 插件仍能正常加载（不报 import 错误）

### file-system.ts 处理验证

- [ ] `MOCK_PLUGINS` 数据已提取到独立 JSON 文件（如 `__mock_data__/plugins.json`）
- [ ] 或：如果最小化 handler 不需要此数据，标记 `file-system.ts` 为 deprecated
- [ ] 不再使用的工具函数（setMockFiles, addMockFile, removeMockFile, resetMockFiles）已删除或标记
- [ ] 无编译错误或 TypeScript 类型错误

## 端口配置一致性验证

- [ ] Go 后端端口配置 = **2025**
  - [ ] `config.user.json` 中 `server.port` = 2025
  - [ ] `internal/server/server.go` 使用该配置
- [ ] Vite 前端端口 = **5173**
  - [ ] `app/encv-mobile/vite.config.ts` 中 `server.port` = 5173
- [ ] Proxy 目标地址 = **127.0.0.1:2025**
  - [ ] `vite.config.ts` 的 proxy 配置正确
- [ ] **关键修复**:
  - [ ] `mock/handlers.ts` 中的 `{ port: 2026 }` 已修正为 `{ port: 2025 }`
  - [ ] Android assets 中的 `config.user.json` 端口 = 2025（已确认 ✅）
- [ ] 无硬编码的错误端口号散落在源码中

## 服务启动方式验证

### Go 后端服务

- [ ] 可通过以下命令成功启动：
  ```bash
  cd /workspace
  go run ./cmd/encv/ serve --port 2025 > /tmp/go-backend.log 2>&1 &
  ```
- [ ] 服务监听在 2025 端口（可通过 `curl http://localhost:2025/health` 验证）
- [ ] 日志输出到指定文件（`/tmp/go-backend.log`）
- [ ] 可通过 `kill $(lsof -t -i :2025)` 停止服务

### Vite 前端服务

- [ ] 可通过以下命令成功启动：
  ```bash
  cd /workspace/app/encv-mobile
  npx vite --port 5173 --host
  ```
- [ ] 服务监听在 5173 端口
- [ ] 控制台输出显示 Local 和 Network 地址
- [ ] Proxy 配置生效（浏览器访问 http://localhost:5173/api/config 能转发到 Go 后端）

### 后台运行模式验证

- [ ] Shell 后台模式（`&`）工作正常
- [ ] 或 tmux/screen 终端复用模式可用（推荐用于长期开发）
- [ ] 或 IDE 多终端模式可用（VS Code / WebStorm）

## Capacitor 预览流程验证

- [ ] 文档描述了完整的 3 步启动流程：
  1. 启动 Go 后端（后台，端口 2025）
  2. 启动 Vite 前端（端口 5173）
  3. （可选）启动 Capacitor 同步/预览
- [ ] 流程中明确标注了每个命令的工作目录
- [ ] 流程中包含了常见问题排查指引：
  - [ ] 端口冲突检测命令（`lsof -i :2025`）
  - [ ] 进程终止命令（`kill <PID>`）
  - [ ] 日志查看位置（`/tmp/go-backend.log` 或 tmux attach）

## 集成功能验证（端到端）

- [ ] 前端页面能通过真实后端加载数据（非 mock 数据）
  - [ ] 访问 http://localhost:5173 能看到真实 UI
  - [ ] API 请求返回真实数据（非 mock 固定值）
- [ ] 核心功能走真实后端：
  - [ ] 文件列表浏览（`GET /api/files`）
  - [ ] 文件信息获取（`GET /api/file/info`）
  - [ ] 配置读写（`GET/PUT /api/config`）
  - [ ] 插件列表（`GET /api/plugins`）
- [ ] WebSocket 连接正常（通过 Vite proxy 转发到 `ws://127.0.0.1:2025/ws`）
- [ ] 无控制台错误或警告（除预期的 501 Not Implemented for non-mocked endpoints）

## 代码质量保障

- [ ] Git 提交信息清晰说明本次重构的范围和原因
  - [ ] 推荐格式: `refactor(mock): minimize handlers from 40+ to 3 endpoints per iron-rule`
- [ ] 无未使用的 import 或死代码残留
- [ ] TypeScript 编译无错误（`npx vue-tsc --noEmit` 通过）
- [ ] Vite 构建无错误（`npx vite build` 通过）
- [ ] 如有测试文件，相关测试通过（`npm test` 或 `vitest run`）

## 可选增强项（非必须，但推荐）

- [ ] （可选）提供了便捷的开发启动脚本 `scripts/dev-start.sh`：
  ```bash
  #!/bin/bash
  # 一键启动前后端开发环境
  cd /workspace
  go run ./cmd/encv/ serve --port 2025 > /tmp/go-backend.log 2>&1 &
  GO_PID=$!
  echo "✅ Go backend started (PID: $GO_PID) on port 2025"
  sleep 2
  cd app/encv-mobile
  npx vite --port 5173 --host
  ```
- [ ] （可选）提供了停止脚本 `scripts/dev-stop.sh`：
  ```bash
  #!/bin/bash
  # 停止所有开发服务
  kill $(lsof -t -i :2025) 2>/dev/null && echo "🛑 Stopped Go backend"
  kill $(lsof -t -i :5173) 2>/dev/null && echo "🛑 Stopped Vite dev server"
  ```
- [ ] （可选）脚本包含端口占用检测和友好错误提示

---

## 快速验证命令清单

```bash
# 1. 检查 handlers.ts 行数
wc -l app/encv-mobile/mock/handlers.ts  # 应 < 100

# 2. 检查 mock 端点数量
grep -c "case '/" app/encv-mobile/mock/handlers.ts  # 应 ≤ 3

# 3. 启动 Go 后端
cd /workspace && go run ./cmd/encv/ serve --port 2025 > /tmp/go-backend.log 2>&1 &

# 4. 验证 Go 后端响应
sleep 2 && curl http://localhost:2025/health  # 应返回 {"status":"ok"}

# 5. 启动 Vite 前端（新终端）
cd app/encv-mobile && npx vite --port 5173 --host

# 6. 验证 Proxy 转发（浏览器或 curl）
curl http://localhost:5173/health  # 应返回 {"status":"ok"}（从 Go 后端代理）

# 7. 停止服务
kill $(lsof -t -i :2025) $(lsof -t -i :5173)
```
