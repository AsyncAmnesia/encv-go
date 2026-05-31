# 铁律规则更新 Spec

## Why
当前项目中存在错误的 handle mock 模式和阻塞式服务启动方式，导致开发体验差、端口冲突、以及潜在的会话阻塞问题。需要建立明确的铁律来禁止这些反模式。

## What Changes
- **新增铁律 1：严禁 mock 大量 handle** — 禁止在测试和开发中创建大量虚假的 container handle mock 对象
- **新增铁律 2：严禁启动永远阻塞会话的服务** — 禁止启动不会自动退出的长期运行服务
- **移除错误的 handle mock 代码** — 清理现有的错误 mock 实现
- **规范化 Capacitor 前后端预览启动方式** — 使用正确的方式同时启动前端和后端
- **明确 Go 程序运行方式** — 使用 `go run` 直接运行，而非先编译再执行
- **强制端口配置正确性** — 确保所有服务使用正确的端口号

## Impact
- Affected specs: 开发流程、测试规范、CI/CD 配置
- Affected code: mock 目录、启动脚本、预览配置文件

---

## ADDED Requirements

### Requirement: 禁止 Mock 大量 Handle

系统 SHALL NOT 在测试代码或开发工具中创建大量虚假的 ContainerHandle mock 对象。

#### Scenario: 错误的 Mock 模式
- **WHEN** 开发者需要测试 container 功能时
- **THEN** 不应创建数十个虚假的 handle 对象来模拟各种场景
- **INSTEAD** 应该使用真实的测试数据文件或精简的集成测试

#### Scenario: 正确的测试方式
- **WHEN** 需要 container handle 进行测试
- **THEN** 使用 `BytesSource` 包装真实测试数据，或使用少量必要的 test fixture
- **AND** Mock 数量应控制在最小必要范围内（通常 < 5 个）

### Requirement: 禁止阻塞式服务启动

系统 SHALL NOT 启动任何会无限期阻塞当前终端会话的服务进程。

#### Scenario: 错误的服务启动方式
- **WHEN** 需要启动后端 API 服务器进行开发预览
- **THEN** 不应在前台直接运行 `./encv-server` 或 `go run cmd/server/main.go` 等阻塞命令
- **BECAUSE** 这会导致终端会话被占用，无法执行其他操作

#### Scenario: 正确的服务启动方式
- **WHEN** 需要启动后端服务
- **THEN** 使用后台运行模式：
  - Shell: `./encv-server &` 或 `nohup ./encv-server > server.log 2>&1 &`
  - 或使用 `tmux`/`screen` 等终端复用工具
  - 或在 IDE 的独立 Terminal 中运行
- **AND** 确保服务有明确的日志输出位置
- **AND** 提供便捷的停止脚本或命令

### Requirement: 规范化 Capacitor 预览启动流程

系统 SHALL 提供标准化的 Capacitor 前后端预览启动方式。

#### Scenario: 完整的开发预览启动
- **WHEN** 开发者需要启动完整的 Capacitor 应用预览（前端 + 后端）
- **THEN** 执行以下步骤：

  1. **启动后端 Go 服务**（后台运行）：
     ```bash
     cd /workspace
     go run ./cmd/encv/ serve --port 2025 &
     ```

  2. **启动前端 Vite 开发服务器**（新终端）：
     ```bash
     cd /workspace/app/encv-mobile
     npx vite --port 5173 --host
     ```

  3. **启动 Capacitor 同步预览**（第三个终端，可选）：
     ```bash
     cd /workspace/app/encv-mobile
     npx cap sync
     npx cap open android  # 或 ios
     ```

- **AND** 后端端口必须为 `2025`
- **AND** 前端 Vite 端口必须为 `5173`
- **AND** Vite proxy 配置必须正确转发 `/api/*` 到 `http://localhost:2025`

### Requirement: Go 程序直接运行

Go 服务程序 SHALL 使用 `go run` 直接运行，而非先编译为可执行文件再执行。

#### Scenario: 开发环境运行
- **WHEN** 在开发环境中运行 Go 服务
- **THEN** 使用 `go run ./cmd/encv/...` 直接运行
- **NOT** 使用 `go build -o encv ./cmd/encv/ && ./encv` 两步法
- **BECAUSE** `go run` 更快、更简洁，且自动处理编译缓存

#### Scenario: 生产环境部署
- **WHEN** 部署到生产环境
- **THEN** 可以使用编译后的二进制文件以提升启动速度
- **BUT** 开发环境始终优先使用 `go run`

### Requirement: 强制端口正确性

所有服务的端口号 MUST 与配置文件保持一致，且不得硬编码。

#### Scenario: 端口配置一致性
- **WHEN** 启动任何服务（后端 API、前端 Vite、WebSocket 等）
- **THEN** 端口号必须来源于统一的配置文件或环境变量
- **AND** 当前标准端口分配：

  | 服务 | 端口 | 用途 |
  |------|------|------|
  | Go Backend API | `2025` | REST API + WebSocket |
  | Vite Dev Server | `5173` | 前端热重载 |
  | Capacitor Preview | `8100` (Android) / `8080` (iOS) | 原生预览 |

- **AND** 如果端口冲突，必须在配置文件中修改，而非在启动命令中临时指定

#### Scenario: 端口冲突检测
- **WHEN** 启动服务时检测到端口已被占用
- **THEN** 输出清晰的错误信息，包含：
  - 被占用的端口号
  - 占用该端口的进程信息（PID）
  - 建议的解决方案（终止旧进程或更换端口）

---

## MODIFIED Requirements

### Requirement: 移除错误的 Handle Mock 代码

**原因**: 现有的 mock/handlers.ts 和相关文件中存在大量不必要的 handle mock，违反了"最小化 mock"原则。

**迁移方案**:
1. 删除 `mock/handlers.ts` 中超过 5 个以上的批量 mock 生成逻辑
2. 保留 2-3 个核心场景的真实数据 mock（用于基础功能验证）
3. 其他测试改用集成测试或真实测试文件
4. 更新 `mock/index.ts` 移除对已删除 mock 的引用

---

## REMOVED Requirements

无（这是新增规则，不涉及移除现有需求）

---

## Implementation Notes

### 文件变更清单

1. **新建/更新规则文档**:
   - `/workspace/.trae/rules/development.md` — 新增开发流程铁律
   - 或在现有 rules 文件中追加新章节

2. **清理 mock 代码**:
   - 审计 `mock/` 目录下的所有文件
   - 删除批量的 handle mock 生成器
   - 保留最小必要的测试 fixture

3. **标准化启动脚本**（可选）:
   - 创建 `scripts/dev-start.sh` — 一键启动前后端
   - 创建 `scripts/dev-stop.sh` — 一键停止所有服务

4. **验证端口配置**:
   - 检查 `vite.config.ts` 的 proxy 设置
   - 检查 Go 服务的默认端口配置
   - 确保文档中的端口号与代码一致

### 验证标准

- [ ] Mock 目录中 handle 对象数量 < 10 个
- [ ] 所有服务均可通过后台方式启动
- [ ] `go run ./cmd/encv/ serve` 能正常启动并监听 2025 端口
- [ ] `npx vite --port 5173` 能正常启动并代理 API 到 2025
- [ ] 无硬编码的端口号散落在源码中
