# Checklist: Go Agent 进程内闭环库 + Vue 渲染壳

> 每个 checkpoint 都要勾选才能算 spec 完成。验证方法见每项 `[verify]` 标注。
> **UI 组件验收**：每个 Vue 组件必须有 props 形状校验、className 与 codex_web 一致、状态文案中文 1:1 对齐。

---

## Go Agent 库

### 核心类型

- [ ] `agent/types.go` 定义 `EventType` string + **6 个**常量（`EventTextDelta`/`EventReasoningDelta`/`EventToolCall`/`EventToolStatus`/`EventToolResult`/`EventStreamEnd`）
- [ ] `agent/types.go` 定义 `Event{Type, Data string}` 结构
- [ ] `agent/types.go` 定义 `ToolCallData{ID, Name, Args, AutoRun, Kind}` 结构
- [ ] `agent/types.go` 定义 `ToolResultData{ID, Name, Result, IsError, Status, DurationMs}` 结构
- [ ] `agent/types.go` 定义 `MessageData` 结构（流式累积用）
- [ ] `agent/types.go` 定义 `Decision` string + **4 个**常量（`DecisionAccept`/`DecisionAcceptForSession`/`DecisionDecline`/`DecisionCancel`）
- [ ] `agent/types.go` 定义 `ToolKind` string + 4 个常量
- [ ] `types_test.go` 覆盖 JSON 序列化/反序列化 round-trip
- [ ] `[verify]` `go test ./agent/... -run TestTypes` 通过

### 工具注册中心

- [ ] `agent/registry.go` 定义 `ToolDefinition{Schema, Handler, NeedConfirm, Kind}`
- [ ] `agent/registry.go` 定义 `ToolRegistry{tools map, mutex}`
- [ ] `NewRegistry()` 构造函数
- [ ] `Register(name, schema, handler, needConfirm, kind)` 方法
- [ ] `Get(name) (ToolDefinition, bool)` 方法
- [ ] `GetAllSchemas() []any` 方法
- [ ] `registry_test.go` 覆盖：注册、并发 Get、Kind 字段
- [ ] `[verify]` `go test -race ./agent/... -run TestRegistry` 通过

### Agent 核心

- [ ] `agent/agent.go` 定义 `SessionCache{Events, IsFinished, mu}`
- [ ] `agent/agent.go` 实现 `pushAndSend(ch, e)` helper
- [ ] `agent/agent.go` 定义 `Agent{registry, sessions, apiKey, openaiClient, sessionGrants, pendingCalls}`
- [ ] `agent/agent.go` 实现 `NewAgent(apiKey, registry)`
- [ ] `agent/agent.go` 实现 `Chat(sessionID, messages) (<-chan, error)`
- [ ] `agent/agent.go` 实现自动执行路径（needConfirm=false → 立即执行 → 推 ToolStatus + ToolResult → 递归）
- [ ] `agent/agent.go` 实现挂起路径（needConfirm=true && !sessionGranted → 推 EventToolCall + EventStreamEnd）
- [ ] `agent/agent.go` 实现 **session 级放行路径**（`(toolName, sessionID)` 在 sessionGrants → 自动通过，不弹 ApprovalCard）
- [ ] `agent/agent.go` 实现 `ConfirmTool(sessionID, toolCallID, decision)` 4 决策
  - [ ] `accept` 路径：执行 → 推 ToolResult → 递归
  - [ ] `accept_for_session` 路径：执行 + 写 sessionGrants → 递归
  - [ ] `decline` 路径：推 ToolResult(cancelled, error) → 递归
  - [ ] `cancel` 路径：推 ToolResult(cancelled) + 推 StreamEnd + **不递归**
- [ ] `agent/agent.go` 实现 `Resume(sessionID, offset)` 重放
- [ ] `agent/agent.go` 实现 Resume 等待机制（offset == len(Events) && !IsFinished → sleep 50ms）
- [ ] `agent_test.go` 覆盖：Chat 流（mock OpenAI server）、**4 决策 ConfirmTool 各路径**、sessionGrants 生效、Resume 重放、session 不存在 error
- [ ] `[verify]` `go test ./agent/... -run TestAgent` 通过

### OpenAI 集成

- [ ] `agent/openai.go` 引入 `github.com/sashabaranov/go-openai`
- [ ] `agent/openai.go` 实现 `createChatCompletionStream(messages, tools)`
- [ ] `agent/openai.go` 实现 `parseDelta(delta) (text, reasoning, toolCalls, isFinished)`
- [ ] `openai_test.go` mock OpenAI HTTP server 返回流
- [ ] `[verify]` `go test ./agent/... -run TestOpenAI` 通过

### HTTP/SSE Handlers

- [ ] `agent/http.go` 定义 `ChatRequest / ResumeRequest / ConfirmRequest`（json tags）
- [ ] `agent/http.go` 实现 `HandleChat(w, r)` —— 调 Chat + 写 SSE
- [ ] `agent/http.go` 实现 `HandleResume(w, r)` —— 调 Resume + 写 SSE
- [ ] `agent/http.go` 实现 `HandleConfirm(w, r)` —— 解析 4 决策 → 调 ConfirmTool + 写 SSE
- [ ] `agent/http.go` 实现 `writeSSE(w, event)` 工具（`data: {json}\n\n`）
- [ ] `http_test.go` mock ResponseWriter 验证 SSE 格式合规 + 4 决策都通过
- [ ] `[verify]` `go test ./agent/... -run TestHTTP` 通过

### 演示程序

- [ ] `agent/cmd/agent-demo/main.go` 创建
- [ ] demo 注册 list_files (auto-run, KindReadOnly, mock 返回 ["a.txt", "b.txt"])
- [ ] demo 注册 delete_file (need confirm, KindFileChange, mock 打印日志)
- [ ] demo 注册 exec_command (need confirm, KindCommand, mock echo)
- [ ] demo mount 3 个 handler 到 :5245
- [ ] `[verify]` `go run ./agent/cmd/agent-demo` 启动后 `curl -N http://localhost:5245/api/chat -d '{"messages":[{"role":"user","content":"list files"}]}'` 输出 SSE

---

## Vue 渲染壳（原子 + 复合 + 顶层）

### useAgent composable

- [ ] `useAgent.ts` 创建
- [ ] `reactive messages[]` + `ref status` + `sessionId/offset` 内部变量
- [ ] `processSSE(stream)` 解析器（支持 **6 种** event type：text_delta/reasoning_delta/tool_call/tool_status/tool_result/stream_end）
- [ ] `send(text)` 推 user 消息 + 空 assistant + fetch /api/chat
- [ ] `confirmTool(toolCallId, decision: 'accept'|'accept_for_session'|'decline'|'cancel')` 调 /api/confirm
- [ ] `resume()` mount 时从 localStorage 读 + fetch /api/resume
- [ ] `saveState/loadState` localStorage 持久化（key: `agent:session:{sessionId}`）
- [ ] 事件类型分发 6 种全实现
- [ ] `useAgent.test.ts` 覆盖事件分发 + 4-决策 confirmTool
- [ ] `[verify]` `pnpm test -- useAgent` 通过

### 原子组件 — 与 codex_web 1:1 对齐

- [ ] **`StatusBadge.vue`** props `{label, tone: 'ready'|'warn'|'idle'}`，class `statusBadge` + `statusBadge_{tone}` —— 颜色对齐 codex_web tokens (`--color-success` / `--color-warning` / `--color-border-subtle`)
- [ ] **`MessageAuthor.vue`** props `{icon, label, meta}`，class `messageAuthor` / `avatar` / `authorName` / `authorMeta`
- [ ] **`BlockHeader.vue`** props `{icon, title, status, statusTone, copyText, expanded, onToggleExpanded}`，class `blockHeader` / `blockTitle` / `blockActions` —— 包含 CopyButton + ExpandButton
- [ ] `tokens.css` 写全：颜色/字体/spacing/radius/z-index —— 数值与 codex_web 1:1（`#15803d success`、`#b45309 warning`、`#991b1b danger`、`--font-sans: ui-sans-serif, system-ui...`、`--font-mono: ui-monospace...`、`--chat-column-max: 58rem` 等）
- [ ] `agent.css` 1:1 移植 codex_web App.module.css 的 messageAuthor / blockHeader / statusBadge / collapsedMessageToggle / userMessageBubble / approvalCard / approvalHeader / approvalBody / approvalFiles / approvalDiff / approvalActions 等 section
- [ ] `[verify]` 浏览器打开 /agent，对照 codex_web UI 截图（若有）逐项核对

### 复合组件

- [ ] **`CollapsedMessageToggle.vue`** props `{icon, label, meta, expanded, active, onToggle}`，class `collapsedMessageToggle` + `collapsedMessageToggleActive` —— 活跃时浅灰脉冲 CSS 动画
- [ ] **`ApprovalCard.vue`** —— 4 决策按钮
  - [ ] `approvalHeader` 显示 `icon`（按 Kind 选 TerminalSquare/FileCode2/ShieldCheck）+ `title` + `reason`
  - [ ] `approvalBody` 显示 `command` / `cwd` / `changedFiles` / `permissions` 摘要
  - [ ] `approvalFiles` 显示 `changedFiles` 前 6 个路径 chip（条件渲染）
  - [ ] `approvalDiff` 可折叠 + CopyButton + ExpandButton（fileChange 时）
  - [ ] `approvalActions` 4 按钮顺序固定：批准 / 本轮批准（条件显示）/ 拒绝 / 拒绝并停止
  - [ ] 点击任一按钮：该按钮显示「处理中」并禁用其他按钮
- [ ] **`GroupedOperationMessage.vue`** —— 累积 command/fileChange/toolOutput 渲染单一摘要
- [ ] **`FileChangeSummaryMessage.vue`** —— 文件变更特化（默认折叠「已编辑 N 个文件」）
- [ ] **`WebSearchSummaryMessage.vue`** —— v1 可先放空实现，v2 填
- [ ] 4 决策按钮文案来自 i18n：`modals.approve` / `modals.approveForSession` / `modals.decline` / `modals.cancel`
- [ ] `[verify]` 浏览器触发 delete_file → 4 按钮 ApprovalCard 正确出现

### 用户消息 / Markdown 渲染

- [ ] **`UserMessageBubble.vue`** —— 右对齐 + 蓝色 + 圆角
- [ ] 长消息自动折叠（>560 字符 或 >9 行）+ 「显示更多」/「收起」toggle
- [ ] 纯文本渲染，不解析 Markdown
- [ ] **`MarkdownStream.vue`** —— 封装 markstream-vue 的 `<MarkStream>` + `dist/style.css`
- [ ] `streaming=true` 启用代码块/表格渐进渲染
- [ ] i18n key 完整：`modals.approve` / `modals.approveForSession` / `modals.decline` / `modals.cancel` / `agent.thinking` / `agent.running` / `agent.completed` / `agent.failed` / `agent.cancelled` / `agent.collapse` / `agent.expand`
- [ ] `[verify]` 浏览器输入 600+ 字符 → 看到「显示更多」折叠

### 虚拟化与顶层视图

- [ ] **`MessageVirtualList.vue`** —— 封装 `vue-virtual-scroller` 的 `<RecycleScroller>`（itemSize=112, minItemSize=80, buffer=600）
- [ ] 阈值判断：`messages.length > 120` 用虚拟列表，否则普通 v-for
- [ ] **`renderTurnItems.ts`** 组合式 —— 实现与 codex_web `renderTurnItems()` 等价的逻辑
  - [ ] 累积 `operationGroup`（command/fileChange/toolOutput）
  - [ ] 累积 `webSearchGroup`
  - [ ] flush 时根据 group 类型渲染不同组件
- [ ] **`AgentChat.vue`** —— 顶层视图
  - [ ] 调用 `renderTurnItems(messages, status)`
  - [ ] 输入框 + 发送/停止按钮
  - [ ] 自动滚动到底部（`scrollToIndex(messages.length - 1, {align: 'end'})`）
  - [ ] 仅当 `nearBottom === true` 时跟随滚动
- [ ] `[verify]` 注入 130 条消息 → DevTools 检查虚拟列表生效（DOM 中只有 ~20 个 message 节点）

### 路由挂载

- [ ] `router/index.ts` 加 `/agent` 路由 → AgentChat
- [ ] OpenListHome 或 Tabs 加 "💬 AI 助手" 入口按钮
- [ ] `[verify]` 浏览器点击入口跳到 /agent 路由

### markstream-vue + vue-virtual-scroller 依赖

- [ ] `plugin-openlist/web/package.json` 加 `markstream-vue: ^x.x.x` + `vue-virtual-scroller: ^x.x.x`
- [ ] `pnpm install` 成功
- [ ] `[verify]` `pnpm dev` 起后访问测试页，code block 渐进渲染

---

## 端到端验证

### 沙箱 dev 联调

- [ ] `ecosystem.config.cjs` 加 `agent-demo` app（编译 + 跑 :5245）
- [ ] `pm2 save` 持久化
- [ ] `curl -N http://localhost:5245/api/chat` SSE 正常流
- [ ] 浏览器 `/openlist-ui/#/agent` 输入 "list files" → UI 展示 ✅ list_files + 文件列表
- [ ] 浏览器输入 "delete foo.txt" → ApprovalCard 4 按钮出现 → 点击「批准」→ ✅ delete_file
- [ ] 浏览器输入 "delete foo.txt" → ApprovalCard 4 按钮出现 → 点击「本轮批准」→ ✅ 再次同类调用自动执行（无 ApprovalCard）
- [ ] 浏览器输入 "delete foo.txt" → 点击「拒绝」→ ToolResult error（user_rejected），LLM 继续生成
- [ ] 浏览器输入 "delete foo.txt" → 点击「拒绝并停止」→ 立即收到 stream_end，本轮结束
- [ ] 流式过程中刷新页面 → 几秒内 resume 追平进度，UI 完整复现
- [ ] 注入 130 条消息 → DevTools 验证 `<MessageVirtualList>` 触发（DOM 节点数稳定）
- [ ] 0 个 console error、0 个 SSE 解析失败
- [ ] `[verify]` 0 个 console error、0 个 SSE 解析失败

### 单元测试

- [ ] `go test ./agent/...` 全绿
- [ ] `go test -race ./agent/...` 无 race warning
- [ ] `pnpm test` vitest 全绿（含 useAgent 4-决策、ApprovalCard 4 按钮、UserMessageBubble 折叠、renderTurnItems 分组）
- [ ] 覆盖率：Go ≥ 70%、TypeScript ≥ 70%
- [ ] `[verify]` CI 测试 pipeline 绿

### 文档同步

- [ ] `unify-sandbox-preview-port/spec.md` 加 D16 章节（agent-demo upstream）
- [ ] `unify-sandbox-preview-port/tasks.md` 加对应 task
- [ ] `unify-sandbox-preview-port/checklist.md` 加检查点（agent-demo 路由 + 联调）
- [ ] `agent/README.md` 含使用示例 + API 一览 + **Decision 4 选 1 表格**
- [ ] `plugin-openlist/web/src/components/agent/README.md` —— 记录与 codex_web 1:1 对应的组件、props、CSS class
- [ ] `[verify]` 文档 review 通过

---

## 非功能性需求

### 性能

- [ ] Chat 首次 Event 延迟 < 500ms（OpenAI TTFB 范围内）
- [ ] SSE 写入不阻塞（每事件写完即 flush）
- [ ] Resume 追平进度延迟 < 1s（50ms polling）
- [ ] 消息列表 1000+ 时虚拟列表仍流畅（虚拟化生效）

### 安全

- [ ] Tool Handler 返回 error 不暴露内部堆栈（只 `error.Error()` 字符串）
- [ ] HTTP handler 设 `Content-Type: text/event-stream` 严格，无任何 HTML 注入
- [ ] SessionID 由 server 生成（UUIDv4），不接受客户端传入作为唯一信任
- [ ] 4 决策的 `decision` 字段在 handler 中做白名单校验（拒绝非 4 值）

### 兼容性

- [ ] Go ≥ 1.21（用 `slices` / `maps` 标准库）
- [ ] Vue 3 + TypeScript 5+（项目现状）
- [ ] 不依赖任何 CGO（gomobile 兼容预留）

---

## 已知限制

- [ ] Session 内存缓存，进程重启即丢（v2 加 Redis/SQLite）
- [ ] 单 session 串行，并发 ConfirmTool 需 v2 加锁
- [ ] OpenAI 单一 provider（v2 加 Anthropic / Gemini 适配）
- [ ] OpenList fork 集成需外部 PR（vendor agent 库）
- [ ] WebSearchSummaryMessage v1 仅占位（v2 实装）

---

## 与 codex_web 1:1 验收清单（强制）

实施完成后必须逐项核对：

- [ ] `MessageAuthor.vue` 的 props 形状 `{icon, label, meta}` 与 codex_web 一致
- [ ] `BlockHeader.vue` 的 props 形状 `{icon, title, status, statusTone, copyText, expanded, onToggleExpanded}` 与 codex_web 一致
- [ ] `StatusBadge.vue` 的 tone 取值 `ready` / `warn` / `idle` 与 codex_web 一致
- [ ] `CollapsedMessageToggle.vue` 的 props 形状 `{icon, label, meta, expanded, active, onToggle}` 与 codex_web 一致
- [ ] `ApprovalCard.vue` 的 4 决策按钮顺序（批准 / 本轮批准 / 拒绝 / 拒绝并停止）与 codex_web 一致
- [ ] `GroupedOperationMessage.vue` 的累积逻辑（command/fileChange/toolOutput）与 codex_web `renderTurnItems` 一致
- [ ] `FileChangeSummaryMessage.vue` 的特化分组逻辑（全是 fileChange → 用此组件）与 codex_web 一致
- [ ] 消息虚拟化阈值 `messages.length > 120` 与 codex_web `MESSAGE_VIRTUALIZATION_THRESHOLD` 一致
- [ ] 用户消息折叠阈值 560 字符 / 9 行 与 codex_web 一致
- [ ] 状态文案（中文）「正在思考」/「正在运行」/「已完成」/「失败」/「已取消」与 codex_web 一致
- [ ] CSS class 命名（`messageAuthor` / `blockHeader` / `statusBadge` / `collapsedMessageToggle` / `userMessageBubble` / `approvalCard` / `approvalHeader` / `approvalBody` / `approvalActions` 等）与 codex_web 一致
- [ ] tokens.css 颜色/字体/spacing/radius 数值与 codex_web 一致
- [ ] i18n key 结构（`modals.approve` / `modals.approveForSession` / `modals.decline` / `modals.cancel`）与 codex_web 一致
