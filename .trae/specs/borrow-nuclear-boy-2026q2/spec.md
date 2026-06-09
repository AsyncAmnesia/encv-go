# Nuclear-Boy 分阶段借鉴 Spec（2026Q2）

## Why

`/tmp/nuclear-boy`（[muzapar00/nuclear-boy](https://github.com/muzapar00/nuclear-boy)，v1.0.0 "核聚变 — 记忆觉醒 + 主动智能 + Skill生态"）是一个 Android 端 AI 编程助手，使用 DeepSeek V4 + Chaquopy Python 沙箱 + Skills 生态。虽然栈不同（Kotlin/Compose/原生 Android vs Vue/Ionic/Capacitor + Go/encv-go），但**核心设计哲学和 ReAct agent 引擎模式**高度可借鉴：

- 800 字精简 system prompt（**正面示例 > 规则**、**避免否定表述**）
- ToolCallAccumulator 流式累积 + 完整触发
- 参数别名容错（path/filePath/filename 互通）
- buildHistoryMessages 的 tool_call 去重 + completedCalls 过滤（**解决 400 insufficient tool messages**）
- AppResult<T> + AppError.humanMessage 错误包装
- 技能市场（skill.yaml + main.py 自动注册为工具）
- Python 沙箱执行（虽然 encv-go 用 Go，但 Python 桥的"脚本注入 __main__ + stdout/stderr 临时文件"模式可参考）

借鉴**不是**复制代码（栈完全不同），而是**抽取设计模式**适配到 encv-go 的 Go 后端 + encv-mobile 的 Vue/Ionic 前端。每个阶段独立可交付，**互相不阻塞**（除 stage 0 之外）。

## 借鉴点全景（按 ROI 排序）

| 阶段 | 借鉴点 | ROI | 复杂度 | 关键文件 |
|------|--------|-----|--------|----------|
| Stage 0 | 仓库深读 + 借鉴点设计文档 | ⭐⭐⭐ | 低 | `/tmp/nuclear-boy/HANDOVER2.0.md` 等 |
| Stage 1 | System Prompt 工程化 | ⭐⭐⭐ | 低 | agent-core/SystemPromptBuilder.kt + AgentEngine.kt |
| Stage 2 | ToolCallAccumulator 流式累积模式 | ⭐⭐⭐ | 中 | agent-core/AgentEngine.kt ToolCallAccumulator |
| Stage 3 | buildHistoryMessages tool_call 去重 + completedCalls 过滤 | ⭐⭐⭐ | 中 | agent-core/AgentEngine.kt buildHistoryMessages |
| Stage 4 | 参数别名容错层 | ⭐⭐ | 低 | agent-core/ToolRegistry.kt executeSafe |
| Stage 5 | 工具定义 JSON Schema 优化（description 字段是关键） | ⭐⭐ | 低 | agent-core/ToolRegistry.kt toDeepSeekToolDefinitions |
| Stage 6 | AppResult<T> + AppError.humanMessage 错误模型 | ⭐⭐ | 中 | common/Models.kt AppError |
| Stage 7 | Skills 生态（skill.yaml + main.py → 自动注册工具） | ⭐ | 高 | skills/SkillManager.kt + SkillManifest.kt + SkillMarketPlace.kt |
| Stage 8 | 三层记忆系统（短期/中期/长期） | ⭐ | 高 | memory/MemoryStore.kt + MemoryDao.kt + MemoryDatabase.kt |
| Stage 9 | 文档生成（docx/xlsx/pptx） via Python 沙箱 | ⭐ | 高 | tools-docgen/DocumentGenerator.kt + Python 沙箱 |
| Stage 10 | HUD 栏（模型/缓存命中/费用/Token 进度） UI 借鉴 | ⭐ | 中 | ui-chat/TokenHudBar.kt |
| Stage 11 | 凌晨 22:00-06:00 自动轻声模式（"温柔"哲学） | ⭐ | 低 | common/AppConstants 夜间模式判定 |
| Stage 12 | 错误处理哲学（"搞定了 ✨" + 先共情后方案） | ⭐ | 低 | common/AppError 错误文案 |

每个阶段是独立 spec 子任务，可**按 ROI 优先级**逐个执行，互相不阻塞。

---

## What Changes

### 总览

- **不复制** Nuclear-Boy 任何代码（Kotlin/Compose 栈不适用）
- **抽取模式** — 分析 nuclear-boy 怎么解决某个问题，把方法论落到 encv-go（Go）和 encv-mobile（Vue/Ionic）的对应位置
- 每个 Stage 都有独立 spec.md / tasks.md / checklist.md
- Stage 0 是**所有后续阶段的前置**——没有它，其余阶段的"借鉴什么"模糊

### 与现有 encv 项目的关系

| 现有 spec | 关系 |
|----------|------|
| `agent-tools-scenarios-v2` | **基线** — Stage 4 / 5 适配到现有 tool registry |
| `agui-real-llm-path-completion` | **基线** — Stage 2 适配到 AG-UI SSE 流式 |
| `mobile-agent-polish-2026q2` | **已完** — 不冲突 |
| `agent-mock-mode` | **基线** — Stage 5/6 适配到 mock 剧本 |
| `multi-engine-chat-architecture` | **基线** — Stage 10 借鉴 HUD 时复用 |

### 阶段依赖图

```
Stage 0 (深读)
  ├─→ Stage 1 (System Prompt)
  │     └─→ Stage 2 (ToolCallAccumulator)
  │           └─→ Stage 3 (buildHistoryMessages)
  ├─→ Stage 4 (参数别名) — 可与 Stage 1 并行
  ├─→ Stage 5 (JSON Schema) — 可与 Stage 4 并行
  ├─→ Stage 6 (AppResult) — 独立
  ├─→ Stage 7 (Skills 生态) — 依赖 Stage 0
  ├─→ Stage 8 (记忆) — 依赖 Stage 0
  ├─→ Stage 9 (文档生成) — 依赖 Stage 0 + Stage 8
  ├─→ Stage 10 (HUD) — 依赖 Stage 0
  ├─→ Stage 11 (夜间模式) — 独立
  └─→ Stage 12 (错误哲学) — 依赖 Stage 6
```

---

## ADDED Requirements

### Requirement: Stage 0 — 仓库深读 + 借鉴点设计文档

实施者**必须**先把 nuclear-boy 仓库读透，输出 1 份"借鉴点设计文档"到 `/workspace/.trae/documents/nuclear-boy-borrowing-design.md`，内容**不少于**：

#### Scenario: 文档必须覆盖 8 大模块的代码深读

| 模块 | 文件 | 必须搞懂 |
|------|------|---------|
| agent-core | AgentEngine.kt / SystemPromptBuilder.kt / ToolRegistry.kt | ReAct 循环 / prompt 工程 / 工具注册 + 执行 + 转换 |
| api-deepseek | DeepSeekApiClient.kt | SSE 流式 + 重试 + 错误分类 + sanitizeMessages |
| memory | MemoryDao.kt / MemoryDatabase.kt / MemoryStore.kt | 三层记忆的 schema / API / 触发条件 |
| skills | SkillManager.kt / SkillManifest.kt / SkillMarketPlace.kt | 加载流程 / YAML 解析 / 项目级 vs 全局 / 注册为工具 |
| python-bridge | ChaquopyPythonExecutor.kt + PythonSandbox.kt + PolicyEnforcer.kt | 注入机制 / 沙箱策略 / 工作目录处理 |
| tools-docgen | FileOperations.kt + DocumentGenerator.kt | 文件 CRUD 安全检查 + docx/xlsx/pptx 生成 |
| ui-chat | ChatScreen.kt / ChatViewModel.kt / TokenHudBar.kt | 状态机 / HUD 实时数据 / Markdown 渲染 |
| app | AppModule.kt (Hilt DI 核心) | 工具如何注入 / 单例管理 / 跨模块依赖 |

#### Scenario: 文档必须为每个借鉴点列出 3 列映射表

```markdown
| Nuclear-Boy 实现 | encv-go 现状 | 借鉴方法论 |
|-----------------|--------------|-----------|
| [N-B 代码片段] | [encv-go 现状代码/缺失] | [借鉴什么 / 不借鉴什么 / 怎么落地] |
```

#### Scenario: Stage 0 交付物清单

- `/workspace/.trae/documents/nuclear-boy-borrowing-design.md`（设计文档，≥300 行）
- `/workspace/.trae/specs/borrow-nuclear-boy-2026q2/borrowing-points.md`（借鉴点索引，供后续 Stage 引用）
- Stage 0 不写任何业务代码，**只**分析 + 写文档

---

### Requirement: Stage 1 — System Prompt 工程化

**目标**：把 nuclear-boy 的 800 字精简哲学（**正面示例 > 规则**、**避免否定表述**、**工具描述即文档**）落到 encv-go 的 system prompt 构建器。

#### Scenario: SystemPromptBuilder 重构

- [internal/agent/prompt.go](file:///workspace/internal/agent/prompt.go)（或类似位置）创建 `SystemPromptBuilder`
- 严格遵循 nuclear-boy 教训（来自 `HANDOVER2.0.md §五`）：
  1. **工具描述比 prompt 更重要** —— 工具 description 字段是模型看的主要参考
  2. **绝对不要否定表述**（"不要用 path"会植入错误模式）
  3. **正面示例 > 规则列表**（`read_file(path="x")` 比"read_file 需要 path"有效 10 倍）
  4. **精简至上**（4000 → 800 字后成功率从 50% 飙升到 95%）
  5. **DeepSeek 默认 thinking=enabled** → 必须显式传 `{"thinking": {"type": "disabled"}}`
- 总长 ≤ 1500 字（包含动态部分）
- 动态内容（用户偏好 / 项目上下文 / Skills 列表）放在最末（缓存优化）

#### Scenario: prompt 校验 checklist

构建完成后跑 lint/单测验证：
- ❌ 包含 "不要" / "不能" / "禁止" / "不可用" → 报错
- ❌ 提到任何不存在的工具 → 报错
- ❌ 单行工具描述不包含正面示例 → 警告
- ❌ 总长 > 1500 字 → 警告
- ✅ 每个工具 1 行，格式：`N. 调用 tool_name，参数：key="value"`

#### Scenario: 借鉴 N-B 但不复制

- **不**用 Kotlin 的 DSL
- **不**硬编码 DeepSeek 特定字段（`reasoning_content` 等）—— 走 encv-go 已有 AG-UI 抽象
- **不**破坏 AG-UI 协议（11 种事件类型不变）

---

### Requirement: Stage 2 — ToolCallAccumulator 流式累积模式

**目标**：把 nuclear-boy 的 `ToolCallAccumulator` 流式累积 + 完整触发模式落到 encv-go 的 AG-UI 协议层。

#### Scenario: 借鉴的模式

来自 `HANDOVER2.0.md §三.2.d`：
```
callApiStreaming() → SSE 流式接收
  → ToolCallRequest → accumulator.clear() + feed(id+name+args)
  → ToolCallDelta    → accumulator.feed(args fragments)
  → 当整个 tool call 完整 → executeSafe()
```

#### Scenario: encv-go 现状问题（Stage 0 设计文档要确认）

- 当前 useAgent.ts L2162-2194 send() 用 `fetch() + processLegacySSE` 处理 SSE
- `parseToolResultData` / `parseContentDelta` 已有，但**没有**累积器
- 假设 LLM 流式输出 `tool_call` 事件 + 后续 delta 事件 → 当前可能丢失 args 片段

#### Scenario: 实施内容

- 在 encv-go 后端：如果用 AG-UI 协议（已经有 `tool_call_start` / `tool_call_delta` / `tool_call_end` 等事件），**不改协议**，只需在后端 parser 端确保累积逻辑正确
- 在前端 useAgent.ts：
  - 创建 `useToolCallAccumulator` composable（受 nuclear-boy `ToolCallAccumulator.kt` 启发）
  - 状态：`pending` / `accumulating` / `complete` / `executed`
  - 收到 `tool_call_start` → 初始化 entry
  - 收到 `tool_call_delta` → 累加 args JSON 字符串
  - 收到 `tool_call_end` → 标记 complete + 入栈到执行队列
  - **同一轮多 tool call 时**：`clear()` 在每轮 ReAct 开始，**不在**每个 `tool_call_start` 时清（避免第 2 个 tool call 清掉第 1 个的 args — nuclear-boy 实战踩坑）

#### Scenario: 单测覆盖

- ✅ 单一 tool call 完整累积
- ✅ 同一轮 2-3 个 tool call 不互相覆盖
- ✅ 中断累积（abort）不破坏下一个 tool call
- ✅ args JSON 解析失败的容错（参考 nuclear-bot 的"参数别名容错"—— Stage 4）

---

### Requirement: Stage 3 — buildHistoryMessages tool_call 去重 + completedCalls 过滤

**目标**：把 nuclear-boy `buildHistoryMessages` 逻辑（来自 `HANDOVER2.0.md §三.3`）落到 encv-go 后端 + 前端。

#### Scenario: 借鉴的模式

```
遍历 history.reversed() (从最新到最旧)
  ├─ 跳过 MessageRole.TOOL (旧版格式)
  ├─ 跳过 MessageRole.SYSTEM
  ├─ 预算控制: BUDGET_CONVERSATION_HISTORY = 100,000 tokens
  └─ 遇到 ASSISTANT with toolCalls:
       ├─ 按 toolCallId 去重 (AgentEngine 发射两次 ToolExecution: RUNNING+COMPLETED)
       ├─ 筛选 completedCalls (output != null && toolCallId != null)
       ├─ 生成 tool 消息 (role="tool", toolCallId=..., name=...)
       └─ 生成 assistant 消息 (toolCalls 只包含 completedCalls)
           ⚠️ 如果 completedCalls 为空 → toolCalls=null（防止 API 400 insufficient tool messages）
```

#### Scenario: encv-go 现状

- 后端 agent_api.go / agent_tool_loop.go 当前可能有**重复发送** `tool_status: running` + `tool_status: completed` + `tool_result`
- 前端 useAgent.ts 收到 `tool_result` → 把它 push 到 message.tool_results 数组
- **问题**：如果 LLM 看到 `assistant tool_calls: [a, b]` 但历史里只有 `tool_result: a`，没有 b → 报 400 insufficient tool messages
- **问题**：`tool_status: running` 可能被 LLM 误读为"tool result message"（nuclear-bot 修复前的同类 bug）

#### Scenario: 实施内容

- **后端**：在 `agent_api.go` 的 ReAct 循环里
  - 每轮 LLM 响应解析后，如果 assistant 有 `tool_calls`，**必须**所有 tool 都执行完（成功或失败）才推下一轮请求
  - 已完成的 tool_calls 携带完整 `tool_result` 消息
  - 未完成的（中断 / 30s timeout）→ 从历史里**移除**该 tool_call（避免 400）
- **前端**：
  - useAgent.ts 处理 tool_result 时，按 toolCallId **去重**（同一个 ID 不重复 push）
  - 构建下一轮 messages 时，**只**包含 completedCalls（nuclear-bot 教训）

#### Scenario: 单测覆盖

- ✅ 中断对话后 assistant 残留未完成 tool_call → 下一轮 messages 把它过滤掉
- ✅ 同一 toolCallId 推 2 次（running + completed）→ 历史只保留 1 条
- ✅ 全部 tool_call 完成的轮次 → 历史完整保留

---

### Requirement: Stage 4 — 参数别名容错层

**目标**：nuclear-boy 的 "path/filePath/filename 互通" 模式落到 encv-go tool registry。

#### Scenario: 现状（来自 `HANDOVER2.0.md §四`）

| # | 工具 | 主参数 | 别名 |
|---|------|--------|------|
| 1 | `read_file` | `path` | `filePath`, `filename` |
| 2 | `write_file` | `path` | — |
| 3 | `list_directory` | `path` | — |
| 4 | `search_files` | `query` | — |
| 5 | `run_python` | `script` | — |
| 6 | `web_search` | `query` | — |
| 7 | `web_fetch` | `url` | `link`, `query` |
| 8 | `generate_docx` | `path` | `output_path` |
| 9 | `generate_xlsx` | `path` | `output_path` |
| 10 | `create_project` | `name` | `path`, `projectName` |

#### Scenario: encv-go 实施

- 在 `internal/tools/registry.go` 的 `ToolDef` 加 `ArgAliases map[string][]string` 字段
- 工具 handler 内：`actualPath := args["path"] ?? args["filePath"] ?? args["filename"]`（Go 1.22+ 用 `slices.Contains` 简化）
- 失败时错误信息自动附加"required: path（可写作 filePath/filename）"提示（nuclear-boy 经验）
- 兼容：旧工具无别名也能跑

#### Scenario: 单测

- ✅ `read_file` 收到 `filePath` → 成功
- ✅ `read_file` 收到 `filename` → 成功
- ✅ `read_file` 收到 3 个全有 → 优先级 `path > filePath > filename`
- ✅ `read_file` 三个全无 → 错误消息含"required: path（可写作 filePath/filename）"

---

### Requirement: Stage 5 — 工具 JSON Schema description 优化

**目标**：借鉴 nuclear-boy "工具描述 (JSON Schema) 比系统提示词更重要"。

#### Scenario: 工具描述模板

每个工具的 `description` 字段必须包含 4 要素：
1. **使用场景**（什么时候用）
2. **参数示例**（正面示例，1-2 个）
3. **格式要求**（特殊字符 / 编码要求）
4. **关联工具**（用 A 之前需要 B）

#### Scenario: 模板示例

```go
// Bad:
// Description: "Read a file"

// Good (nuclear-boy 风格):
Description: `读取项目内的文本文件。返回文件内容（前 100KB）。

使用场景：需要查看现有文件内容、参考模板、检查配置。
参数示例：read_file(path="src/main.go")
关联：先用 list_directory 找到 path。

限制：单文件 > 100KB 会截断，需用 read_file_range。
```

#### Scenario: 实施

- 现有 10 个 encv 工具（read_file / list_files / edit_metadata / batch_rename / delete_file / command_run / etc.）的 description 字段全部按 4 要素重写
- 单测：description 包含"使用场景"/"参数示例"/"关联工具"关键词

---

### Requirement: Stage 6 — AppResult<T> + AppError.humanMessage 错误模型

**目标**：把 nuclear-boy 的 `AppResult<T>` + `AppError.humanMessage` 模式落到 encv-go。

#### Scenario: Go 版 AppResult

```go
// internal/common/result.go
type Result[T any] struct {
    Value T
    Err   *AppError
}

type AppError struct {
    Code        string  // e.g. "ENOENT", "PERMISSION_DENIED"
    HumanMessage string  // 给用户看的本地化消息
    Technical   string  // 给开发看的技术细节
    Cause       error   // 原始 error
    Recoverable bool
}
```

#### Scenario: 实施

- 创建 `internal/common/result.go` 定义 `Result[T]` 和 `AppError`
- 工具 handler 返回 `(ToolResult, *AppError)`（已在 Stage 1+2 的 mobile-agent-polish-2026q2 实施过，这里扩展 HumanMessage 字段）
- 前端 useAgent.ts 解析 `errorCode` → 查 i18n 表 → 显示本地化消息

#### Scenario: 借鉴哲学

来自 `CLAUDE.md`：
> **错误处理** — 所有可失败操作返回 `AppResult<T>`，用户可见错误使用 `AppError.humanMessage`，UI 层错误用友好语气（不是技术术语）

---

### Requirement: Stage 7 — Skills 生态（skill.yaml + main.py → 自动注册为工具）

**目标**：借鉴 nuclear-boy `SkillManager.kt` + `SkillManifest.kt` + `SkillMarketPlace.kt` 的"写一个 skill.yaml + main.py 就成为 AI 工具"模式。

#### Scenario: nuclear-boy 现有 skills

来自 `INFO.md`：
- 全局 Skills + 项目级 Skills
- 预置 3 个 Skills：**skill-creator** / **file-organizer** / **code-formatter**
- 编写 `skill.yaml` + `main.py` 即可扩展能力
- 自动注册为 AI 工具，支持参数验证

#### Scenario: encv-go 实施（按能力递减）

##### 7.1: Skill YAML schema

```yaml
# skill.yaml
id: file-organizer
name: 文件整理
version: 1.0.0
scope: global  # or project
description: 自动整理项目文件，按类型/日期归类
parameters:
  - name: directory
    type: string
    required: true
    description: 要整理的目录
  - name: strategy
    type: string
    enum: [by_type, by_date]
    default: by_type
runtime: python  # or "go" (直接调 .go 文件)
entry: main.py
```

##### 7.2: SkillManager（Go）

- `internal/skills/manager.go` 加载 `.skills/*.yaml`（全局 + `<project>/.skills/*.yaml`）
- 解析 → 注册到 ToolRegistry
- LLM 看到的工具 = skill id

##### 7.3: Skill 执行

- `runtime: python` → 调 encv-go 的 `python-bridge`（Chaquopy 不适用，但 encv-go 可以用 `os/exec` + 临时 .py 脚本模式 + `__main__` 注入，借鉴 nuclear-boy 沙箱设计）
- `runtime: go` → 编译时内置或 plugin 加载

##### 7.4: 预置 3 个 skills

移植 nuclear-boy 预置：
- `skill-creator` — 创建新 skill 的 skill
- `file-organizer` — 文件按类型/日期整理
- `code-formatter` — 代码格式化（go fmt / prettier / black）

#### Scenario: 借鉴但不复制

- **不**用 Chaquopy（encv-go 走 Go 主程 + Python 子进程）
- **不**做"skill 市场"在线浏览（先做本地 install/load）
- **保留** skill.yaml schema 兼容 nuclear-boy 格式（方便迁移用户写过的 skill）

---

### Requirement: Stage 8 — 三层记忆系统

**目标**：借鉴 nuclear-boy `memory/` 模块的 Room 三层记忆。

#### Scenario: 三层定义

| 层 | 存储 | 容量 | 触发 | 保留期 |
|----|------|------|------|--------|
| 短期 (working) | 内存 | 最近 20 轮对话 | 自动 | 当前 session |
| 中期 (episodic) | SQLite | 项目级事件 | 项目操作时 | 90 天 |
| 长期 (semantic) | SQLite | 用户偏好/项目元信息 | LLM 摘要 | 永久 |

#### Scenario: encv-go 现状

- 当前 useAgent.ts 用 in-memory `messages.value[]` 数组
- 无项目级事件
- 无用户偏好持久化（除 localStorage 几个简单 key）

#### Scenario: 实施

- `internal/memory/short.go` — 短期（in-memory 20 轮）
- `internal/memory/medium.go` — 中期（SQLite `events` 表，schema 与 nuclear-boy 兼容）
- `internal/memory/long.go` — 长期（SQLite `preferences` + `project_meta` 表）
- 触发条件：每完成一轮 ReAct → 写中期；每 10 轮或退出 session → 调 LLM 摘要 → 写长期

---

### Requirement: Stage 9 — 文档生成（docx/xlsx/pptx）

**目标**：借鉴 nuclear-boy `tools-docgen/DocumentGenerator.kt`（通过 Python 沙箱生成文档）。

#### Scenario: encv-go 实施

- `internal/tools/docgen.go` 定义 `generate_docx` / `generate_xlsx` / `generate_pptx` 三个工具
- 实施方式：与 Stage 7 的 `runtime: python` 共用 python-bridge
- Python 脚本内 import `python-docx` / `openpyxl` / `python-pptx`（nuclear-boy 预装列表）
- 安全：脚本注入 `__name__ = '__main__'`（nuclear-bot 教训）+ 工作目录限制在 project dir

#### Scenario: 借鉴

- Python 库列表（python-docx / openpyxl / Pillow / chardet / python-pptx / requests / beautifulsoup4）= 完全照搬
- DocumentGenerator 模板（title / content / 表格）= 借鉴但简化

---

### Requirement: Stage 10 — HUD 栏（模型/缓存命中/费用/Token 进度）

**目标**：借鉴 nuclear-boy `ui-chat/TokenHudBar.kt` 的实时可观测 HUD。

#### Scenario: HUD 元素

| 元素 | 数据源 | 实时性 |
|------|--------|--------|
| 模型名 | `getAgentBase()` 返回 | 切换时 |
| Token 速度 | 上一轮 input + output / 耗时 | 完成后 |
| 缓存命中率 | DeepSeek 响应头 `x-ds-cache-hit` | 完成后 |
| 费用 | input/output 单价 × token | 完成后 |
| 上下文占用 | messages token 估算 / 1M 窗口 | 每轮 |
| 黄/红预警 | 占用 > 70% 黄 / > 90% 红 | 每轮 |

#### Scenario: encv-mobile 实施

- 已有 [useTokenTracker.ts](file:///workspace/app/encv-mobile/src/composables/useTokenTracker.ts)？需 Stage 0 确认
- 新建 [components/agent/TokenHudBar.vue](file:///workspace/app/encv-mobile/src/components/agent/TokenHudBar.vue)（**注**：encv-mobile 当前用 `OperationCard.vue` / 没有 `TokenHudBar.vue`）
- 数据从 AG-UI 协议的 `usage` / `cost` 事件读

---

### Requirement: Stage 11 — 凌晨 22:00-06:00 自动轻声模式

**目标**：借鉴 nuclear-boy "凌晨自动轻声模式"。

#### Scenario: 行为变化

| 时间段 | 文案风格 | 通知音量 |
|--------|---------|---------|
| 06:00-22:00 | "搞定了 ✨" / "已完成 ✅" | 正常 |
| 22:00-06:00 | "已就绪 🌙" / "完成 💤" | 静音（仅振动） |

#### Scenario: 实施

- [composables/useNotificationTone.ts](file:///workspace/app/encv-mobile/src/composables/useNotificationTone.ts) 新建
- 检测 `new Date().getHours()` → 返回 tone 配置
- i18n 加 `agent.tone.night.greeting` / `agent.tone.day.greeting` 等 key

---

### Requirement: Stage 12 — 错误处理哲学（"搞定了 ✨" + 先共情后方案）

**目标**：借鉴 nuclear-boy 错误文案哲学（来自 `CLAUDE.md` "人性化原则"）。

#### Scenario: 文案模板

| 状态 | nuclear-boy 文案 | encv-mobile 文案（中文） |
|------|------------------|------------------------|
| 成功 | "搞定了 ✨" | "搞定 ✨" |
| 成功（夜间） | "已就绪 🌙" | "已就绪 🌙" |
| 文件不存在 | "找不到这个文件，要不要先列一下目录？" | "找不到这个文件，要先列目录吗？" |
| 权限不足 | "没权限操作这个文件，试试 sudo？" | "没权限，需要更高权限吗？" |
| 网络错 | "网络有点问题，等会儿再试" | "网络不太通畅，要不要稍后再试？" |

#### Scenario: 实施

- 在 Stage 6 的 `AppError.HumanMessage` 字段上分层
- 默认 fallback 用 nuclear-boy 风格
- 高级用户可在 Settings 切换"技术化错误"模式

---

## MODIFIED Requirements

无（全部为新功能，不修改现有 spec；Stage 1-6 适配到现有 AG-UI / tool registry 不破坏向后兼容）

---

## REMOVED Requirements

无

---

## 约束与限制

1. **不复制 Nuclear-Boy 任何代码**（栈不同，复制无意义）
2. **每个 Stage 独立可交付**——Stage 7 失败不影响 Stage 1-6
3. **Stage 0 必须最先做**——没有它，后续 Stage 没有"借鉴什么"的清晰定义
4. **工具描述优化（Stage 5）必须先于 prompt 优化（Stage 1）**——nuclear-boy 教训："工具描述比 prompt 更重要"
5. **AppResult 错误模型（Stage 6）必须先于 Skills 生态（Stage 7）**——Skills 失败要返 Result 而不是 panic
6. **CJK + en 双语**——所有新增文案必须 i18n 齐全
7. **不破坏向后兼容**——现有 tool / AG-UI 协议 / mock 剧本继续工作
8. **借鉴哲学优先**——不复制具体实现；如 nuclear-boy 用了 Chaquopy 而 encv-go 用 os/exec，这是合理替换

---

## 验证步骤

每个 Stage 独立验证（不强制一次性跑全部）：

| Stage | 验证方法 |
|-------|---------|
| 0 | 文档 300+ 行 + 8 模块覆盖检查 |
| 1 | prompt lint pass（无否定表述 / ≤1500 字 / 每工具 1 行） |
| 2 | ToolCallAccumulator 单测覆盖（单 tool / 多 tool / 中断 / 解析失败） |
| 3 | buildHistoryMessages 单测覆盖（中断 / 重复 ID / completed 过滤） |
| 4 | 参数别名 4 个用例 |
| 5 | description 包含 4 要素单测 |
| 6 | AppResult / AppError 类型 + HumanMessage 单测 |
| 7 | skill.yaml 解析 + 3 个预置 skill 注册 |
| 8 | 短期 20 轮 FIFO + 中期 SQLite 持久化 + 长期 LLM 摘要 |
| 9 | generate_docx / xlsx / pptx 端到端 |
| 10 | HUD 实时数据 + 6 元素全部显示 |
| 11 | 22:00 切换文案风格 + 通知静音 |
| 12 | 错误文案 A/B 对照（nuclear-boy 风格 vs 旧技术化） |

---

## 关键文件 / 函数

| Stage | 文件 |
|-------|------|
| 0 | `/workspace/.trae/documents/nuclear-boy-borrowing-design.md`（新建） |
| 1 | `/workspace/internal/agent/prompt.go`（新建） + `/workspace/internal/agent/prompt_test.go` |
| 2 | `/workspace/app/encv-mobile/src/composables/useToolCallAccumulator.ts`（新建） |
| 3 | `/workspace/internal/server/agent_api.go`（修改 buildHistoryMessages） + `/workspace/app/encv-mobile/src/composables/useAgent.ts`（去重） |
| 4 | `/workspace/internal/tools/registry.go`（加 ArgAliases） + 各工具 handler |
| 5 | `/workspace/internal/tools/*.go` 工具 description 字段重写 |
| 6 | `/workspace/internal/common/result.go`（新建） + `AppError` 字段 |
| 7 | `/workspace/internal/skills/manager.go` + `manifest.go` + 3 个预置 skill |
| 8 | `/workspace/internal/memory/{short,medium,long}.go` + SQLite schema |
| 9 | `/workspace/internal/tools/docgen.go`（新建） |
| 10 | `/workspace/app/encv-mobile/src/components/agent/TokenHudBar.vue`（新建） |
| 11 | `/workspace/app/encv-mobile/src/composables/useNotificationTone.ts`（新建） |
| 12 | `/workspace/app/encv-mobile/src/i18n/agent.ts` 文案 + `AppError.HumanMessage` 映射 |

---

## 与现有 encv 项目的关系（汇总）

| 现有 spec | 关联 Stage |
|----------|----------|
| `agent-tools-scenarios-v2` | Stage 4 / 5 / 6 适配到现有 tool registry |
| `agui-real-llm-path-completion` | Stage 2 / 3 适配到 AG-UI 11 种事件 |
| `mobile-agent-polish-2026q2`（已完） | 不冲突；Stage 12 错误文案可参考其 i18n 体系 |
| `agent-mock-mode` | Stage 5 / 6 适配到 mock 剧本 |
| `multi-engine-chat-architecture` | Stage 10 借鉴 HUD 时复用 engine 抽象 |
| `mobile-overlay-mechanism`（preview-management.md） | 不冲突 |
