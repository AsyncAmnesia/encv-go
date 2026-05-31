# Tasks

## Phase 1: 规则文档更新

- [ ] Task 1: 创建/更新开发铁律规则文档
  - [ ] SubTask 1.1: 在 `/workspace/.trae/rules/` 下创建或更新开发规范文件
  - [ ] SubTask 1.2: 添加"严禁 mock 大量 handle"铁律（含正确/错误示例）
  - [ ] SubTask 1.3: 添加"严禁阻塞式服务启动"铁律（含后台运行示例）
  - [ ] SubTask 1.4: 添加"Go 程序直接运行"规范（go run vs go build）
  - [ ] SubTask 1.5: 添加"端口必须正确"铁律（标准端口表 + 冲突检测）

- [ ] Task 2: 更新 Capacitor 预览启动文档
  - [ ] SubTask 2.1: 编写标准化的前后端启动步骤（后端 + Vite + Capacitor）
  - [ ] SubTask 2.2: 明确端口配置要求（2025 / 5173 / 8100）
  - [ ] SubTask 2.3: 提供一键启动/停止脚本模板（可选）

## Phase 2: 清理错误的 Mock 代码

- [ ] Task 3: 审计现有 mock 目录
  - [ ] SubTask 3.1: 列出 `mock/` 目录下所有文件及其 handle mock 数量
  - [ ] SubTask 3.2: 识别批量 mock 生成逻辑（超过 5 个的批量创建）
  - [ ] SubTask 3.3: 标记可删除的 mock 代码段

- [ ] Task 4: 移除多余的 handle mock
  - [ ] SubTask 4.1: 删除 `mock/handlers.ts`（或类似文件）中的批量 mock 生成器
  - [ ] SubTask 4.2: 保留 2-3 个核心场景的真实数据 mock
  - [ ] SubTask 4.3: 更新 `mock/index.ts` 及相关引用，移除对已删除 mock 的依赖
  - [ ] SubTask 4.4: 确保删除后测试仍能通过（或标记为待修复的集成测试）

## Phase 3: 验证端口和服务配置

- [ ] Task 5: 检查并修正端口配置
  - [ ] SubTask 5.1: 审计 Go 后端默认端口（应为 2025）
  - [ ] SubTask 5.2: 审计 Vite proxy 配置（应转发到 localhost:2025）
  - [ ] SubTask 5.3: 检查是否有硬编码端口号散落在源码中
  - [ ] SubTask 5.4: 统一端口配置到单一配置文件或环境变量

- [ ] Task 6: 验证正确的启动方式
  - [ ] SubTask 6.1: 测试 `go run ./cmd/encv/ serve --port 2025` 能正常启动
  - [ ] SubTask 6.2: 测试 `npx vite --port 5173` 能正常启动并代理 API
  - [ ] SubTask 6.3: 确认服务可在后台运行不阻塞终端
  - [ ] SubTask 6.4: 编写端口冲突检测逻辑或文档说明

## Phase 4: 文档和脚本（可选增强）

- [ ] Task 7: 创建便捷开发脚本（可选）
  - [ ] SubTask 7.1: 创建 `scripts/dev-start.sh` 一键启动前后端
  - [ ] SubTask 7.2: 创建 `scripts/dev-stop.sh` 一键停止所有相关进程
  - [ ] SubTask 7.3: 脚本中包含端口冲突检查和错误提示

# Task Dependencies

- Task 2 depends on Task 1（启动文档依赖规则定义）✅
- Task 4 depends on Task 3（清理代码依赖审计结果）✅
- Task 5 depends on Task 1（端口配置依赖规则定义）✅
- Task 6 depends on Task 5（验证启动方式依赖端口配置正确）✅
- Task 7 depends on Task 1, Task 5, Task 6（脚本依赖所有配置就绪）✅

# Parallelizable Work

- Task 1 + Task 3 可并行（规则编写和代码审计独立）✅
- Task 2 + Task 4 可并行（文档更新和代码清理独立，但需保持一致）⚠️ 建议顺序执行
