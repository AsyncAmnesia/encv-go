# Tasks

## Phase 1: 清理 Mock Handler 代码（核心任务）

- [ ] Task 1: 重写 `mock/handlers.ts` 为最小化实现
  - [ ] SubTask 1.1: 删除 `fileSystemHandler` 函数（94-193 行，10 个文件系统 API）
  - [ ] SubTask 1.2: 删除 `fileContentHandler` 函数（195-267 行，7 个文件内容 API）
  - [ ] SubTask 1.3: 精简 `staticJsonHandler` 函数：仅保留 `/health`, `/api/config`, `/api/plugins` 三个端点
  - [ ] SubTask 1.4: 删除 `taskMockHandler` 函数（358-427 行，4 个任务 API）
  - [ ] SubTask 1.5: 删除 `staticFileHandler`, `debugControlHandler` 及特殊路由（/decrypt, /preview/*, /api/file/info）
  - [ ] SubTask 1.6: 替换为最小化实现（≤50 行，3 个端点 + 501 fallback）

- [ ] Task 2: 更新 mock 模块引用链
  - [ ] SubTask 2.1: 更新 `mock/index.ts`，移除对已删除函数的 import
  - [ ] SubTask 2.2: 审计 `mock/file-system.ts` 的使用者，如仅被 handlers 使用则标记 deprecated
  - [ ] SubTask 2.3: 验证 `vite.config.ts` 无需修改（proxy fallback 已正确）

## Phase 2: 规范化开发流程文档

- [ ] Task 3: 创建/更新开发铁律规则
  - [ ] SubTask 3.1: 在 `.trae/rules/` 下创建或追加开发规范章节
  - [ ] SubTask 3.2: 添加"严禁 mock 大量 handle"铁律（含当前 620 行 vs 目标 50 行对比）
  - [ ] SubTask 3.3: 添加"严禁阻塞式服务启动"铁律（含后台运行示例）
  - [ ] SubTask 3.4: 添加"Go 程序直接运行"规范（go run vs go build 对比）
  - [ ] SubTask 3.5: 添加"端口必须正确"铁律（标准端口表 + 错误端口 2026 修正指引）

- [ ] Task 4: 编写 Capacitor 预览标准化启动文档
  - [ ] SubTask 4.1: 基于 `vite.config.ts` 审计结果编写 3 步启动流程
  - [ ] SubTask 4.2: 明确端口依赖关系（Go 2025 → Vite 5173 → Proxy 转发）
  - [ ] SubTask 4.3: 提供后台运行命令模板（& / nohup / tmux 三种方式）

## Phase 3: 验证与测试

- [ ] Task 5: 验证清理后的代码质量
  - [ ] SubTask 5.1: 统计 `mock/handlers.ts` 行数（目标 < 100 行）
  - [ ] SubTask 5.2: 统计 Mock API 端点数量（目标 ≤ 3 个）
  - [ ] SubTask 5.3: 运行 TypeScript 编译检查（`npx vue-tsc --noEmit`）
  - [ ] SubTask 5.4: 运行现有单元测试（`npm test` 或 `npx vitest run`）

- [ ] Task 6: 验证正确的服务启动流程
  - [ ] SubTask 6.1: 测试 `go run ./cmd/encv/ serve --port 2025` 后台启动成功
  - [ ] SubTask 6.2: 测试 `npx vite --port 5173 --host` 启动并代理到 2025
  - [ ] SubTask 6.3: 验证前端页面能通过真实后端加载 API 数据
  - [ ] SubTask 6.4: 检查无硬编码错误端口号（搜索 2026, 8080, 3000 等非标准端口）

# Task Dependencies

- Task 2 depends on Task 1（引用链更新依赖代码清理完成）✅
- Task 3 depends on Task 1（规则文档需引用实际代码变更）✅
- Task 4 depends on Task 1（启动文档需基于清理后的配置）✅
- Task 5 depends on Task 1, Task 2（验证需在代码清理和引用更新后）✅
- Task 6 depends on Task 5（服务验证需在代码质量检查通过后）✅

# Parallelizable Work

- Task 1 可独立执行（核心代码重构）✅
- Task 3 + Task 4 可并行（文档编写独立于代码验证）⚠️ 建议在 Task 1 后执行以保持一致性
