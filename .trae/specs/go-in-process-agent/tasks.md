# Tasks: Go Agent 进程内闭环库 + Vue 渲染壳

> 实施顺序：先 Go 库（自底向上），后 Vue 渲染壳（先原子组件 → 组合组件 → 顶层视图），最后集成 demo + 验证。
> **UI 组件命名/Props 必须与 codex_web 1:1 对齐**，便于后续组件库合并。

---

## Phase 1: Go Agent 库骨架

### Task 1.1: 创建 agent 顶层 Go module

- [ ] SubTask 1.1.1: 在 `/workspace/agent/` 创建 `go.mod`，module path 暂定 `github.com/encv/agent`
- [ ] SubTask 1.1.2: 初始化 go.sum 空白文件，`go mod tidy` 一次确保格式
- [ ] SubTask 1.1.3: 写 `agent/README.md`（库定位、使用示例、API 一览）

### Task 1.2: 实现核心类型 (types.go)

- [ ] SubTask 1.2.1: 定义 `EventType` string + **6 个**常量（`EventTextDelta` / `EventReasoningDelta` / `EventToolCall` / `EventToolStatus` / `EventToolResult` / `EventStreamEnd`）
- [ ] SubTask 1.2.2: 定义 `Event` 结构（Type + Data string JSON）
- [ ] SubTask 1.2.3: 定义 `ToolCallData{ID, Name, Args, AutoRun, Kind}` 结构
- [ ] SubTask 1.2.4: 定义 `ToolResultData{ID, Name, Result, IsError, Status, DurationMs}` 结构
- [ ] SubTask 1.2.5: 定义 `MessageData` 结构（Content / Reasoning / ToolCalls / ToolResults）—— 用于流式累积
- [ ] SubTask 1.2.6: 定义 `Decision` string + **4 个**常量（`DecisionAccept` / `DecisionAcceptForSession` / `DecisionDecline` / `DecisionCancel`）
- [ ] SubTask 1.2.7: 定义 `ToolKind` string + 4 个常量（`KindCommand` / `KindFileChange` / `KindReadOnly` / `KindUnknown`）
- [ ] SubTask 1.2.8: 写 `types_test.go` 覆盖 JSON 序列化/反序列化

### Task 1.3: 实现工具注册中心 (registry.go)

- [ ] SubTask 1.3.1: 定义 `ToolDefinition{Schema, Handler, NeedConfirm, Kind}` 结构
- [ ] SubTask 1.3.2: 定义 `ToolRegistry{tools map, mutex}`
- [ ] SubTask 1.3.3: 实现 `NewRegistry()` 构造函数
- [ ] SubTask 1.3.4: 实现 `Register(name, schema, handler, needConfirm, kind)` 方法
- [ ] SubTask 1.3.5: 实现 `Get(name) (ToolDefinition, bool)` 方法
- [ ] SubTask 1.3.6: 实现 `GetAllSchemas() []any` 方法
- [ ] SubTask 1.3.7: 写 `registry_test.go` 覆盖：注册、并发 Get、Schema 列表、Kind 字段

---

## Phase 2: Agent 核心逻辑

### Task 2.1: 实现 SessionCache

- [ ] SubTask 2.1.1: 在 `agent.go` 内定义 `SessionCache` 结构（Events / IsFinished / mu）
- [ ] SubTask 2.1.2: 实现 `pushAndSend(ch, e)` 辅助方法
- [ ] SubTask 2.1.3: 实现 `markFinished()` 辅助方法

### Task 2.2: 实现 Agent.Chat 流式入口

- [ ] SubTask 2.2.1: 定义 `Agent` 结构（registry / sessions / apiKey / openaiClient / **sessionGrants**）
- [ ] SubTask 2.2.2: 实现 `NewAgent(apiKey, registry) *Agent`
- [ ] SubTask 2.2.3: 实现 `Chat(sessionID, messages) (<-chan, error)`
- [ ] SubTask 2.2.4: 实现 `streamOpenAI(...)` 内部辅助
  - 调用 OpenAI streaming API（带 tool schemas）
  - 解析 delta → Event（含 `reasoning_delta`）
  - 处理 tool_calls：自动执行 / 挂起 / **session 级放行**

### Task 2.3: 实现 Agent.ConfirmTool（4 决策）

- [ ] SubTask 2.3.1: 在 Agent 上加 `pendingCalls map[string]pendingCall` 字段
- [ ] SubTask 2.3.2: 实现 `ConfirmTool(sessionID, toolCallID, decision) (<-chan, error)`
  - `accept` → 执行 + 推 ToolResult + 递归
  - `accept_for_session` → 执行 + **写 sessionGrants** + 递归
  - `decline` → 推 ToolResult(cancelled) + 递归
  - `cancel` → 推 ToolResult(cancelled) + 推 StreamEnd + **不递归**
- [ ] SubTask 2.3.3: 写 `confirm_test.go` 覆盖 4 条决策路径

### Task 2.4: 实现 Agent.Resume

- [ ] SubTask 2.4.1: 实现 `Resume(sessionID, offset) (<-chan, error)`
  - 50ms polling 等流
  - 追到 IsFinished 推 EventStreamEnd

### Task 2.5: OpenAI 客户端集成 (openai.go)

- [ ] SubTask 2.5.1: 引入 `github.com/sashabaranov/go-openai`
- [ ] SubTask 2.5.2: 实现 `createChatCompletionStream(messages, tools)`
- [ ] SubTask 2.5.3: 实现 `parseDelta(delta) (text, reasoning, toolCalls, isFinished)`
- [ ] SubTask 2.5.4: 写 `openai_test.go` mock server

---

## Phase 3: HTTP/SSE Handlers

### Task 3.1: 实现 /api/chat handler

- [ ] SubTask 3.1.1: 定义 `ChatRequest{SessionID, Messages}`
- [ ] SubTask 3.1.2: 实现 `HandleChat(w, r)`

### Task 3.2: 实现 /api/resume handler

- [ ] SubTask 3.2.1: 定义 `ResumeRequest{SessionID, Offset}`
- [ ] SubTask 3.2.2: 实现 `HandleResume(w, r)`

### Task 3.3: 实现 /api/confirm handler（4 决策）

- [ ] SubTask 3.3.1: 定义 `ConfirmRequest{SessionID, ToolCallID, Decision string}`（接受 `accept`/`accept_for_session`/`decline`/`cancel`）
- [ ] SubTask 3.3.2: 实现 `HandleConfirm(w, r)`：解析 decision → 调 ConfirmTool

### Task 3.4: SSE 通用工具函数

- [ ] SubTask 3.4.1: 抽 `writeSSE(w, event)` 工具
- [ ] SubTask 3.4.2: 写 `http_test.go` mock ResponseWriter

---

## Phase 4: 演示程序

### Task 4.1: agent-demo 入口

- [ ] SubTask 4.1.1: 在 `cmd/agent-demo/main.go` 写演示程序
  - 注册 list_files (auto-run, KindReadOnly)
  - 注册 delete_file (need confirm, KindFileChange)
  - 注册 exec_command (need confirm, KindCommand)
  - mount 3 个 HTTP handler
  - 监听 :5245
- [ ] SubTask 4.1.2: 写 Makefile 或 go run 指令
- [ ] SubTask 4.1.3: 验证 `curl -N http://localhost:5245/api/chat -d '{...}'` 流式输出 SSE

---

## Phase 5: Vue 渲染壳（对齐 codex_web 组件）

### Task 5.1: useAgent composable

- [ ] SubTask 5.1.1: 在 `composables/useAgent.ts` 创建
- [ ] SubTask 5.1.2: 定义 `reactive messages[]` / `ref status` / `let sessionId` / `let eventOffset`
- [ ] SubTask 5.1.3: 实现 `processSSE(stream)` 解析器（支持 6 种 event type）
- [ ] SubTask 5.1.4: 实现 `send(text)` 发起对话
- [ ] SubTask 5.1.5: 实现 `confirmTool(toolCallId, decision: Decision)` 4 决策
- [ ] SubTask 5.1.6: 实现 `resume()` mount 时自动续传
- [ ] SubTask 5.1.7: 实现 `saveState/loadState` localStorage
- [ ] SubTask 5.1.8: 写 unit test

### Task 5.2: 原子组件（MessageAuthor / BlockHeader / StatusBadge）

- [ ] SubTask 5.2.1: **`StatusBadge.vue`** — props `{label, tone: 'ready'|'warn'|'idle'}`，对应 CSS `.statusBadge` `.statusBadge_ready/warn/idle`
- [ ] SubTask 5.2.2: **`MessageAuthor.vue`** — props `{icon, label, meta}`，class `messageAuthor` `avatar` `authorName` `authorMeta`
- [ ] SubTask 5.2.3: **`BlockHeader.vue`** — props `{icon, title, status, statusTone, copyText, expanded, onToggleExpanded}`，class `blockHeader` `blockTitle` `blockActions`
- [ ] SubTask 5.2.4: 写 `tokens.css`（颜色/字体/spacing/radius）—— 复用 codex_web token 数值
- [ ] SubTask 5.2.5: 写 `agent.css`（messageAuthor / blockHeader / statusBadge）—— 1:1 移植 codex_web App.module.css 对应部分

### Task 5.3: 复合组件（ApprovalCard / CollapsedMessageToggle / GroupedOperationMessage）

- [ ] SubTask 5.3.1: **`CollapsedMessageToggle.vue`** — props `{icon, label, meta, expanded, active, onToggle}`，class `collapsedMessageToggle` + `collapsedMessageToggleActive`
- [ ] SubTask 5.3.2: **`ApprovalCard.vue`** — 4 决策按钮 + `approvalHeader` + `approvalBody` + `approvalFiles` + `approvalDiff` + `approvalActions`
  - 按钮顺序固定：批准 / 本轮批准（条件显示） / 拒绝 / 拒绝并停止
  - 点击 → 按钮显示「处理中」并禁用其他按钮
- [ ] SubTask 5.3.3: **`GroupedOperationMessage.vue`** — 累积 command/fileChange/toolOutput，渲染摘要
- [ ] SubTask 5.3.4: **`FileChangeSummaryMessage.vue`** — 文件变更特化分组
- [ ] SubTask 5.3.5: **`WebSearchSummaryMessage.vue`** — Web 搜索特化分组（v1 可先放空实现）

### Task 5.4: 用户消息 / Markdown 渲染

- [ ] SubTask 5.4.1: **`UserMessageBubble.vue`** — 右对齐蓝色气泡 + 长文本自动折叠（>560 字符 或 >9 行）
- [ ] SubTask 5.4.2: **`MarkdownStream.vue`** — 封装 markstream-vue 的 `<MarkStream>` + `dist/style.css`
- [ ] SubTask 5.4.3: 添加 i18n key（`modals.approve`/`modals.approveForSession`/`modals.decline`/`modals.cancel`/`agent.thinking`/`agent.running`/`agent.completed`/`agent.failed`/`agent.cancelled`/`agent.collapse`/`agent.expand`）

### Task 5.5: 虚拟化与顶层视图

- [ ] SubTask 5.5.1: **`MessageVirtualList.vue`** — 封装 `vue-virtual-scroller` 的 `<RecycleScroller>`（itemSize=112, minItemSize=80, buffer=600）
- [ ] SubTask 5.5.2: 阈值判断：`messages.length > 120` 用虚拟列表，否则普通 v-for
- [ ] SubTask 5.5.3: **`renderTurnItems.ts`** 组合式 — 实现与 codex_web `renderTurnItems()` 等价的逻辑
  - 累积 `operationGroup`（command/fileChange/toolOutput）
  - 累积 `webSearchGroup`
  - flush 时根据 group 类型渲染不同组件
- [ ] SubTask 5.5.4: **`AgentChat.vue`** — 顶层视图
  - 调用 `renderTurnItems(messages, status)`
  - 输入框 + 发送/停止按钮
  - 自动滚动到底部（`scrollToIndex(messages.length - 1, {align: 'end'})`）

### Task 5.6: 路由与依赖

- [ ] SubTask 5.6.1: `plugin-openlist/web/package.json` 加 `markstream-vue` + `vue-virtual-scroller` 依赖
- [ ] SubTask 5.6.2: `pnpm install` 验证
- [ ] SubTask 5.6.3: `router/index.ts` 加 `/agent` 路由
- [ ] SubTask 5.6.4: 在 OpenListHome 或 Tabs 加 "💬 AI 助手" 入口按钮

---

## Phase 6: 端到端验证

### Task 6.1: 沙箱 dev 联调

- [ ] SubTask 6.1.1: `ecosystem.config.cjs` 加 `agent-demo` app（编译 + 跑 :5245）
- [ ] SubTask 6.1.2: `pm2 save` 持久化
- [ ] SubTask 6.1.3: `curl -N http://localhost:5245/api/chat` SSE 正常流
- [ ] SubTask 6.1.4: 浏览器 `/openlist-ui/#/agent` 输入 "list files" → UI 展示 ✅ list_files + 文件列表
- [ ] SubTask 6.1.5: 浏览器输入 "delete foo.txt" → **ApprovalCard 4 按钮** 出现 → 点击「批准」→ UI 展示 ✅ delete_file
- [ ] SubTask 6.1.6: 同上但点击「本轮批准」→ 验证 sessionGrants 生效（第二次同类调用直接执行）
- [ ] SubTask 6.1.7: 同上但点击「拒绝」→ UI 展示 ToolResult error（user_rejected），LLM 继续
- [ ] SubTask 6.1.8: 同上但点击「拒绝并停止」→ UI 立即收到 stream_end，本轮结束
- [ ] SubTask 6.1.9: 流式过程中刷新页面 → 几秒内 resume 追平进度
- [ ] SubTask 6.1.10: 注入 130 条消息 → 验证 `<MessageVirtualList>` 触发

### Task 6.2: 单元测试

- [ ] SubTask 6.2.1: `go test ./agent/...` 全绿
- [ ] SubTask 6.2.2: `go test -race ./agent/...` 无 race warning
- [ ] SubTask 6.2.3: `pnpm test` vitest 全绿（含 useAgent + 各 Vue 组件）
- [ ] SubTask 6.2.4: 覆盖率：Go ≥ 70%、TypeScript ≥ 70%

### Task 6.3: 文档同步

- [ ] SubTask 6.3.1: 在 `unify-sandbox-preview-port/spec.md` 加 D16 章节（agent-demo upstream）
- [ ] SubTask 6.3.2: 在 `unify-sandbox-preview-port/tasks.md` 加对应 task
- [ ] SubTask 6.3.3: 在 `unify-sandbox-preview-port/checklist.md` 加检查点
- [ ] SubTask 6.3.4: `agent/README.md` 含使用示例 + API 一览
- [ ] SubTask 6.3.5: 写 `plugin-openlist/web/src/components/agent/README.md` —— 记录与 codex_web 1:1 对应的组件、props、CSS class，便于跨项目复用

---

# Task Dependencies

| Task | Depends on |
|------|-----------|
| 1.1 (Go module) | — |
| 1.2 (types) | 1.1 |
| 1.3 (registry) | 1.2 |
| 2.1 (SessionCache) | 1.2 |
| 2.2 (Chat) | 1.3, 2.1, 2.5 |
| 2.3 (ConfirmTool) | 2.2 |
| 2.4 (Resume) | 2.1 |
| 2.5 (OpenAI 集成) | 1.3 |
| 3.1 (/chat) | 2.2 |
| 3.2 (/resume) | 2.4 |
| 3.3 (/confirm) | 2.3 |
| 3.4 (SSE 工具) | 1.2 |
| 4.1 (demo) | 3.1, 3.2, 3.3 |
| 5.1 (useAgent) | — (前端独立) |
| 5.2 (原子组件) | — |
| 5.3 (复合组件) | 5.2 |
| 5.4 (User/Markdown) | 5.2 |
| 5.5 (虚拟化/顶层) | 5.2, 5.3, 5.4, 5.1 |
| 5.6 (路由/依赖) | 5.5 |
| 6.1 (联调) | 4.1, 5.6 |
| 6.2 (单测) | 全部 |
| 6.3 (文档) | 6.1 |

# 可并行任务

- **Phase 1** (1.1/1.2/1.3) 与 **Phase 5.1** (useAgent) 可完全并行
- **Phase 5.2 / 5.3 / 5.4** 三个组件子阶段可内部并行（不同文件）
- **Phase 2**（Agent 核心）与 **Phase 5.2/5.3**（UI 原子/复合组件）可并行

---

# 估算

- Phase 1: 0.5 天（types + registry）
- Phase 2: 2.5 天（Agent 核心 + 4-决策 + session grants + OpenAI 集成）
- Phase 3: 0.5 天（HTTP/SSE）
- Phase 4: 0.5 天（demo）
- Phase 5: 2 天（Vue 渲染壳：原子组件 0.5d + 复合组件 0.5d + User/Markdown 0.5d + 虚拟化/顶层 0.5d）
- Phase 6: 1 天（联调 + 测试 + 文档）

**总计：~7 天**（比原估算 +1.5 天，因新增 4-决策、sessionGrants、虚拟化、更多 Vue 组件）
