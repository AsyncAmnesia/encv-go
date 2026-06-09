# Tasks (Borrow Nuclear-Boy 2026Q2 — 多阶段借鉴)

> **总览**：12 个独立可交付 Stage，每个 Stage 都有「借鉴模式 + 实施步骤 + 单测 + 验证」闭环。
> **Stage 0 必须最先做**（无它，后续 Stage 借鉴什么模糊）。

---

## Stage 0: 仓库深读 + 借鉴点设计文档

> **目标**：把 nuclear-boy 仓库 12 模块 + HANDOVER 全部吃透，输出一份 ≥300 行设计文档，作为后续 Stage 的"借鉴什么"参考。

- [ ] Task 0.1: 仓库已克隆到 `/tmp/nuclear-boy`（已完成）
- [ ] Task 0.2: 读 HANDOVER.md + HANDOVER2.0.md 完整内容（含工具速查 / 提示词设计 / 关键 Bug 修复）
- [ ] Task 0.3: 深读 agent-core 8 个核心 .kt 文件（AgentEngine / SystemPromptBuilder / ToolRegistry / AgentEvent / ToolCallAccumulator / DeepSeekApiClient / TokenTracker / AppModule）
- [ ] Task 0.4: 深读 memory 3 个文件（MemoryDao / MemoryDatabase / MemoryStore）
- [ ] Task 0.5: 深读 skills 4 个文件（SkillManager / SkillManifest / SkillMarketPlace + SkillManagerTest）
- [ ] Task 0.6: 深读 python-bridge 3 个文件（ChaquopyPythonExecutor / PythonSandbox / PolicyEnforcer）
- [ ] Task 0.7: 深读 tools-docgen 2 个文件（FileOperations / DocumentGenerator）
- [ ] Task 0.8: 深读 ui-chat 4 个文件（ChatScreen / ChatViewModel / TokenHudBar / 状态机）
- [ ] Task 0.9: 读 CLAUDE.md / INFO.md（哲学 + 设计原则）
- [ ] Task 0.10: 输出 `/workspace/.trae/documents/nuclear-boy-borrowing-design.md`（≥300 行）
  - 每个借鉴点列出 3 列映射表（N-B 实现 / encv 现状 / 借鉴方法论）
  - 8 大模块 + 工具调用链 + 错误处理哲学 + 性能调优
- [ ] Task 0.11: 输出 `/workspace/.trae/specs/borrow-nuclear-boy-2026q2/borrowing-points.md`（借鉴点索引，供后续 Stage 引用）

**Stage 0 验收**：
- [ ] 文档 ≥300 行
- [ ] 8 大模块代码深读全部完成
- [ ] 借鉴点索引 ≥ 12 个（与 Stage 1-12 一一对应）

---

## Stage 1: System Prompt 工程化

> **目标**：把 nuclear-boy 800 字精简哲学（正面示例 > 规则 / 避免否定 / 工具描述即文档）落到 encv-go。

- [ ] Task 1.1: 创建 `/workspace/internal/agent/prompt.go`（SystemPromptBuilder）
- [ ] Task 1.2: 实现 Build() 方法遵循 5 大原则（来自 HANDOVER2.0.md §五）
- [ ] Task 1.3: 实现动态内容后置（用户偏好 / 项目上下文 / Skills 列表）
- [ ] Task 1.4: 创建 `/workspace/internal/agent/prompt_test.go` 单测：
  - ❌ 包含 "不要" / "不能" / "禁止" / "不可用" 报错
  - ❌ 提到不存在的工具报错
  - ❌ 单行工具描述无正面示例警告
  - ❌ 总长 > 1500 字警告
  - ✅ 每个工具 1 行格式正确
- [ ] Task 1.5: 验证 `go build ./cmd/encv` 0 错误

**Stage 1 验收**：
- [ ] 5 大原则单测全部通过
- [ ] 实际生成的 prompt 长度 < 1500 字

---

## Stage 2: ToolCallAccumulator 流式累积模式

> **目标**：前端 useAgent 加 ToolCallAccumulator，处理 LLM 流式 tool_call_start / tool_call_delta / tool_call_end 事件。

- [ ] Task 2.1: 创建 `/workspace/app/encv-mobile/src/composables/useToolCallAccumulator.ts`
  - 状态：`pending` / `accumulating` / `complete` / `executed`
  - `clear()` 在 ReAct 循环开始时调用（不在 tool_call_start 时清）
- [ ] Task 2.2: 处理 tool_call_start → 初始化 entry
- [ ] Task 2.3: 处理 tool_call_delta → 累加 args JSON 字符串
- [ ] Task 2.4: 处理 tool_call_end → 标记 complete + 入栈到执行队列
- [ ] Task 2.5: 集成到 useAgent.ts send() / confirmTool() 流程
- [ ] Task 2.6: 单测覆盖：
  - ✅ 单一 tool call 完整累积
  - ✅ 同一轮 2-3 个 tool call 不互相覆盖
  - ✅ 中断累积（abort）不破坏下一个 tool call
  - ✅ args JSON 解析失败容错（→ Stage 4 兜底）
- [ ] Task 2.7: 验证 `npx vue-tsc --noEmit` 0 错误

**Stage 2 验收**：
- [ ] 4 个单测场景通过
- [ ] 与 useAgent.send() 集成无破坏

---

## Stage 3: buildHistoryMessages tool_call 去重 + completedCalls 过滤

> **目标**：解决 400 insufficient tool messages（nuclear-boy 实战踩坑）。

- [ ] Task 3.1: 后端 `/workspace/internal/server/agent_api.go` buildHistoryMessages 实现：
  - 跳过 MessageRole.SYSTEM
  - 预算控制 100,000 tokens
  - 按 toolCallId 去重
  - completedCalls 过滤（output != null && toolCallId != null）
  - completedCalls 为空 → toolCalls=null
- [ ] Task 3.2: 前端 useAgent.ts 处理 tool_result 按 toolCallId 去重
- [ ] Task 3.3: 单测：
  - ✅ 中断对话后残留未完成 tool_call → 下一轮过滤掉
  - ✅ 同一 toolCallId 推 2 次（running + completed）→ 历史只保留 1 条
  - ✅ 全部 tool_call 完成的轮次 → 完整保留
- [ ] Task 3.4: 验证 `go build` + `vue-tsc` 双 0 错误

**Stage 3 验收**：
- [ ] 3 个单测场景通过
- [ ] 集成测试：中断对话后 LLM 不再 400

---

## Stage 4: 参数别名容错层

> **目标**：ToolDef 加 ArgAliases 字段，handler 内自动 fallback。

- [ ] Task 4.1: `/workspace/internal/tools/registry.go` ToolDef 加 `ArgAliases map[string][]string`
- [ ] Task 4.2: 实现 `resolveArg(args, primaryKey, aliases []string) any` helper
- [ ] Task 4.3: 现有 10 个工具 description 标注别名（如 read_file 注明 `path | filePath | filename`）
- [ ] Task 4.4: 错误信息自动附加"required: path（可写作 filePath/filename）"
- [ ] Task 4.5: 单测：
  - ✅ read_file 收到 filePath → 成功
  - ✅ read_file 收到 filename → 成功
  - ✅ read_file 三个全有 → 优先级 path > filePath > filename
  - ✅ read_file 三个全无 → 错误消息含别名提示
- [ ] Task 4.6: 验证 `go build` 0 错误

**Stage 4 验收**：
- [ ] 4 个单测通过
- [ ] 现有 10 工具无破坏

---

## Stage 5: 工具 JSON Schema description 优化

> **目标**：每个工具 description 包含 4 要素（场景/示例/格式/关联）。

- [ ] Task 5.1: 创建 `internal/tools/description_lint.go` 校验工具
- [ ] Task 5.2: 现有 10 个工具 description 按 4 要素重写
- [ ] Task 5.3: 单测：
  - ✅ description 包含"使用场景" / "参数示例" / "关联工具" 关键词
  - ❌ 缺要素 → 警告
- [ ] Task 5.4: 验证 `go build` 0 错误

**Stage 5 验收**：
- [ ] 10 工具全部按 4 要素重写
- [ ] lint pass

---

## Stage 6: AppResult<T> + AppError.humanMessage 错误模型

> **目标**：Go 版 Result + AppError 类型，前端友好化错误。

- [ ] Task 6.1: 创建 `/workspace/internal/common/result.go`
  - `Result[T any] struct { Value T; Err *AppError }`
  - `AppError struct { Code, HumanMessage, Technical string; Cause error; Recoverable bool }`
- [ ] Task 6.2: 工具 handler 返回值扩展（与 mobile-agent-polish-2026q2 Task 1 兼容）
- [ ] Task 6.3: 前端 i18n 加 error.humanMessage 映射（ENOENT → "找不到这个文件" 等）
- [ ] Task 6.4: 单测：Result 包装 / AppError.HumanMessage 字段 / i18n 映射
- [ ] Task 6.5: 验证 `go build` + `vue-tsc` 双 0 错误

**Stage 6 验收**：
- [ ] 4 个单测场景通过
- [ ] 前端至少 5 个常见 error code 有中文本地化

---

## Stage 7: Skills 生态（skill.yaml + main.py → 自动注册）

> **目标**：写一个 skill.yaml + main.py 就成为 AI 工具的能力。

- [ ] Task 7.1: 定义 skill.yaml schema（id / name / version / scope / description / parameters / runtime / entry）
- [ ] Task 7.2: 创建 `/workspace/internal/skills/manager.go`（加载 + 解析 + 注册到 ToolRegistry）
- [ ] Task 7.3: 创建 `/workspace/internal/skills/manifest.go`（YAML 解析）
- [ ] Task 7.4: 创建 3 个预置 skill（移植 nuclear-bot）：
  - `skill-creator` — 创建新 skill
  - `file-organizer` — 文件整理
  - `code-formatter` — 代码格式化
- [ ] Task 7.5: 实现 `runtime: python`（调 encv-go 的 python-bridge，注入 __main__ 模式）
- [ ] Task 7.6: 单测：
  - ✅ YAML 解析正确
  - ✅ 3 个预置 skill 自动注册为 ToolRegistry 项
  - ✅ 全局 vs 项目级 skill 加载优先级
- [ ] Task 7.7: 验证 `go build` 0 错误

**Stage 7 验收**：
- [ ] 3 个预置 skill 实际可被 LLM 调用
- [ ] 写一个新 skill.yaml → 重启后自动注册

---

## Stage 8: 三层记忆系统

> **目标**：短期（内存 20 轮）+ 中期（SQLite 项目事件）+ 长期（LLM 摘要）。

- [ ] Task 8.1: `/workspace/internal/memory/short.go` 内存 20 轮 FIFO
- [ ] Task 8.2: `/workspace/internal/memory/medium.go` SQLite `events` 表
- [ ] Task 8.3: `/workspace/internal/memory/long.go` SQLite `preferences` + `project_meta` 表
- [ ] Task 8.4: 触发条件：每完成一轮 ReAct → 写中期；每 10 轮 / 退出 session → 调 LLM 摘要 → 写长期
- [ ] Task 8.5: 单测：
  - ✅ 短期 21 轮后第 1 轮被淘汰
  - ✅ 中期 SQLite 持久化（重启可读）
  - ✅ 长期 LLM 摘要正确（基于 5 轮对话）
- [ ] Task 8.6: 验证 `go build` 0 错误

**Stage 8 验收**：
- [ ] 3 层记忆功能可用
- [ ] 重启后中期/长期数据可恢复

---

## Stage 9: 文档生成（docx/xlsx/pptx）

> **目标**：通过 Python 沙箱生成 office 文档。

- [ ] Task 9.1: `/workspace/internal/tools/docgen.go` 定义 `generate_docx` / `generate_xlsx` / `generate_pptx` 三个工具
- [ ] Task 9.2: 与 Stage 7 共用 python-bridge
- [ ] Task 9.3: Python 脚本 import `python-docx` / `openpyxl` / `python-pptx`（借鉴 nuclear-bot 预装列表）
- [ ] Task 9.4: 沙箱安全：脚本注入 `__name__ = '__main__'` + 工作目录限制
- [ ] Task 9.5: 单测：3 个工具端到端（生成文件 → 读回 → 验证内容）
- [ ] Task 9.6: 验证 `go build` 0 错误

**Stage 9 验收**：
- [ ] generate_docx 生成的 .docx 可用 Word 打开
- [ ] generate_xlsx 生成的 .xlsx 可用 Excel 打开

---

## Stage 10: HUD 栏（模型/缓存命中/费用/Token 进度）

> **目标**：实时可观测 HUD 组件。

- [ ] Task 10.1: 创建 `/workspace/app/encv-mobile/src/components/agent/TokenHudBar.vue`
- [ ] Task 10.2: 6 元素（模型 / Token 速度 / 缓存命中 / 费用 / 上下文占用 / 预警色）
- [ ] Task 10.3: 数据从 AG-UI 协议的 `usage` / `cost` 事件读
- [ ] Task 10.4: 黄/红预警（>70% 黄 / >90% 红）
- [ ] Task 10.5: 集成到 AgentChat.vue
- [ ] Task 10.6: 单测：
  - ✅ 6 元素全部显示
  - ✅ 占用 > 70% 变黄
  - ✅ 占用 > 90% 变红
- [ ] Task 10.7: 验证 `vue-tsc` 0 错误

**Stage 10 验收**：
- [ ] HUD 6 元素实时数据
- [ ] 视觉预警工作

---

## Stage 11: 凌晨 22:00-06:00 自动轻声模式

> **目标**：夜间自动切换文案风格 + 通知静音。

- [ ] Task 11.1: 创建 `/workspace/app/encv-mobile/src/composables/useNotificationTone.ts`
- [ ] Task 11.2: 检测 `new Date().getHours()` → tone 配置
- [ ] Task 11.3: i18n 加 `agent.tone.night.greeting` / `agent.tone.day.greeting` 等 key
- [ ] Task 11.4: 通知音量：夜间仅振动
- [ ] Task 11.5: 单测：
  - ✅ 06:00 / 22:00 边界值正确
  - ✅ tone 配置切换
- [ ] Task 11.6: 验证 `vue-tsc` 0 错误

**Stage 11 验收**：
- [ ] 22:00 后所有成功文案走"已就绪 🌙"风格
- [ ] 通知音量切换工作

---

## Stage 12: 错误处理哲学（"搞定了 ✨" + 先共情后方案）

> **目标**：错误文案风格 nuclear-boy 化。

- [ ] Task 12.1: Stage 6 的 `AppError.HumanMessage` 字段应用模板：
  - 成功 → "搞定 ✨"
  - 文件不存在 → "找不到这个文件，要先列目录吗？"
  - 权限不足 → "没权限，需要更高权限吗？"
  - 网络错 → "网络不太通畅，要不要稍后再试？"
- [ ] Task 12.2: i18n 中英双语齐全
- [ ] Task 12.3: Settings 切换"技术化错误"模式（高级用户）
- [ ] Task 12.4: 单测：
  - ✅ 5 个常见错误码有友好文案
  - ✅ Settings 切换工作
- [ ] Task 12.5: 验证 `vue-tsc` 0 错误

**Stage 12 验收**：
- [ ] 至少 5 个错误码有友好文案
- [ ] Settings 切换生效

---

# Task Dependencies

- [Stage 0] 必须在所有 Stage 之前
- [Stage 5] 必须在 [Stage 1] 之前（工具描述比 prompt 更重要）
- [Stage 6] 必须在 [Stage 7] 之前（Skills 失败要返 Result）
- [Stage 2] → [Stage 3]（累积模式是 buildHistoryMessages 输入的前提）
- [Stage 4] 独立（与 Stage 2/3 并行）
- [Stage 7] → [Stage 9]（文档生成用 python-bridge，与 Skills 共用）
- [Stage 8] 独立（与 Stage 7 并行）
- [Stage 10] 独立
- [Stage 11] 独立
- [Stage 12] 依赖 [Stage 6]（AppError 字段）

# Priority Order (按 ROI)

1. Stage 0（前置，无 ROI 估算）
2. **Stage 1** ⭐⭐⭐（高 ROI：直接提升 LLM 成功率）
3. **Stage 2** ⭐⭐⭐（高 ROI：解决 400 insufficient tool messages）
4. **Stage 3** ⭐⭐⭐（高 ROI：同 Stage 2）
5. **Stage 4** ⭐⭐（中 ROI：容错）
6. **Stage 5** ⭐⭐（中 ROI：同 Stage 1 但偏描述层）
7. **Stage 6** ⭐⭐（中 ROI：错误模型基础）
8. **Stage 7** ⭐（高复杂度，建议做）
9. **Stage 8** ⭐（高复杂度，可后置）
10. **Stage 9** ⭐（高复杂度，可后置）
11. **Stage 10** ⭐（UI 层）
12. **Stage 11** ⭐（小优化）
13. **Stage 12** ⭐（文案层）
