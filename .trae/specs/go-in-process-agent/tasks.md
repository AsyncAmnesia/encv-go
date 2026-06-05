# Tasks: Go Agent 进程内闭环库 + Vue 渲染壳

> 实施顺序：先 Go 库（自底向上），后 Vue 渲染壳，最后集成 demo + 验证。

---

## Phase 1: Go Agent 库骨架

### Task 1.1: 创建 agent 顶层 Go module

- [ ] SubTask 1.1.1: 在 `/workspace/agent/` 创建 `go.mod`，module path 暂定 `github.com/encv/agent`（待最终命名）
- [ ] SubTask 1.1.2: 初始化 go.sum 空白文件，`go mod tidy` 一次确保格式
- [ ] SubTask 1.1.3: 写 `agent/README.md`（库定位、使用示例、API 一览）

### Task 1.2: 实现核心类型 (types.go)

- [ ] SubTask 1.2.1: 定义 `EventType` string 类型 + 4 个常量（EventTextDelta / EventToolCall / EventToolResult / EventStreamEnd）
- [ ] SubTask 1.2.2: 定义 `Event` 结构（Type + Data string JSON）
- [ ] SubTask 1.2.3: 定义 `ToolCallData` 结构（ID / Name / Args / AutoRun）
- [ ] SubTask 1.2.4: 定义 `ToolResultData` 结构（ID / Name / Result / IsError）
- [ ] SubTask 1.2.5: 定义 `MessageData` 结构（Content / ToolCalls / ToolResults）—— 用于流式累积
- [ ] SubTask 1.2.6: 写 `types_test.go` 覆盖 JSON 序列化/反序列化

### Task 1.3: 实现工具注册中心 (registry.go)

- [ ] SubTask 1.3.1: 定义 `ToolDefinition` 结构（Schema / Handler / NeedConfirm）
- [ ] SubTask 1.3.2: 定义 `ToolRegistry` 结构（`tools map[string]ToolDefinition` + `sync.RWMutex`）
- [ ] SubTask 1.3.3: 实现 `NewRegistry()` 构造函数
- [ ] SubTask 1.3.4: 实现 `Register(name, schema, handler, needConfirm)` 方法
- [ ] SubTask 1.3.5: 实现 `Get(name) (ToolDefinition, bool)` 方法
- [ ] SubTask 1.3.6: 实现 `GetAllSchemas() []any` 方法
- [ ] SubTask 1.3.7: 写 `registry_test.go` 覆盖：注册、并发 Get、Schema 列表

---

## Phase 2: Agent 核心逻辑

### Task 2.1: 实现 SessionCache

- [ ] SubTask 2.1.1: 在 `agent.go` 内定义 `SessionCache` 结构（Events []*Event / IsFinished bool / mu sync.Mutex）
- [ ] SubTask 2.1.2: 实现 `pushAndSend(ch, e)` 辅助方法：先 append 到 Events，再 send 到 channel
- [ ] SubTask 2.1.3: 实现 `markFinished()` 辅助方法

### Task 2.2: 实现 Agent.Chat 流式入口

- [ ] SubTask 2.2.1: 定义 `Agent` 结构（registry / sessions sync.Map / apiKey / openaiClient）
- [ ] SubTask 2.2.2: 实现 `NewAgent(apiKey, registry) *Agent` 构造函数
- [ ] SubTask 2.2.3: 实现 `Chat(sessionID, messages) (<-chan *Event, error)`
  - 创建 SessionCache 存入 sessions
  - 启动 goroutine 调 OpenAI 流
  - 返回 channel
- [ ] SubTask 2.2.4: 实现 `streamOpenAI(sessionID, messages) <-chan *Event`（内部辅助）
  - 调用 OpenAI streaming API（带 tool schemas）
  - 解析 delta → Event
  - 处理 tool_calls：自动执行 / 挂起等待确认
  - 递归调用自身处理多轮 tool 反馈

### Task 2.3: 实现 Agent.ConfirmTool

- [ ] SubTask 2.3.1: 在 Agent 上加 `pendingCalls map[string]pendingCall` 字段（sessionID + toolCallID → def + args）
- [ ] SubTask 2.3.2: 实现 `ConfirmTool(sessionID, toolCallID, approved) (<-chan *Event, error)`
  - 查 pendingCalls
  - approved=true：执行 Handler，推 ToolResult
  - approved=false：推 ToolResult(Result=`{"error":"user_rejected"}`, IsError=true)
  - 追加 tool_result 到 messages，递归 streamOpenAI
  - 返回新 channel

### Task 2.4: 实现 Agent.Resume

- [ ] SubTask 2.4.1: 实现 `Resume(sessionID, offset) (<-chan *Event, error)`
  - 查 sessions
  - 启动 goroutine：从 cache.Events[offset] 开始依次推 channel
  - 追到 len(Events) 且 !IsFinished → sleep 50ms 重试
  - 追到 IsFinished → 推 EventStreamEnd 后退出

### Task 2.5: OpenAI 客户端集成 (openai.go)

- [ ] SubTask 2.5.1: 引入 `github.com/sashabaranov/go-openai` 依赖
- [ ] SubTask 2.5.2: 实现 `createChatCompletionStream(messages, tools) (stream, error)` 包装
  - 转换 messages map → openai.ChatCompletionMessage
  - 转换 registry schemas → []openai.Tool
- [ ] SubTask 2.5.3: 实现 `parseDelta(delta) (textContent, toolCalls, isFinished, error)` 辅助
- [ ] SubTask 2.5.4: 写 `openai_test.go` 覆盖：mock OpenAI server 返回流，验证 delta 解析

---

## Phase 3: HTTP/SSE Handlers

### Task 3.1: 实现 /api/chat handler

- [ ] SubTask 3.1.1: 定义 `ChatRequest{SessionID string, Messages []map[string]any}`（json tags）
- [ ] SubTask 3.1.2: 实现 `HandleChat(w, r)`：
  - 解析 JSON body
  - 如果 SessionID 空 → 生成 UUID
  - 调 `agent.Chat(sessionID, messages)` 拿 channel
  - 设置 SSE headers
  - 循环 channel → `data: {json}\n\n` 写 ResponseWriter
  - channel 关闭 → 关闭 writer

### Task 3.2: 实现 /api/resume handler

- [ ] SubTask 3.2.1: 定义 `ResumeRequest{SessionID string, Offset int}`
- [ ] SubTask 3.2.2: 实现 `HandleResume(w, r)`：调 `agent.Resume`，同 SSE 写入逻辑

### Task 3.3: 实现 /api/confirm handler

- [ ] SubTask 3.3.1: 定义 `ConfirmRequest{SessionID, ToolCallID string, Approved bool}`
- [ ] SubTask 3.3.2: 实现 `HandleConfirm(w, r)`：调 `agent.ConfirmTool`，同 SSE 写入逻辑

### Task 3.4: SSE 通用工具函数

- [ ] SubTask 3.4.1: 抽 `writeSSE(w, event *Event) error` 工具：序列化为 `data: {json}\n\n` 并 flush
- [ ] SubTask 3.4.2: 写 `http_test.go` 覆盖：mock ResponseWriter 验证 SSE 格式

---

## Phase 4: 演示程序

### Task 4.1: agent-demo 入口

- [ ] SubTask 4.1.1: 在 `/workspace/agent/cmd/agent-demo/main.go` 写演示程序
  - 注册 list_files（auto-run）：返回 mock 文件列表
  - 注册 delete_file（need confirm）：mock 删除逻辑
  - 注册 search_files（auto-run）：mock 搜索
  - mount 3 个 HTTP handler
  - 监听 :5245
- [ ] SubTask 4.1.2: 写 Makefile 或 go run 指令方便启动
- [ ] SubTask 4.1.3: 验证 `curl -N http://localhost:5245/api/chat -d '{...}'` 能流式输出 SSE

---

## Phase 5: Vue 渲染壳

### Task 5.1: useAgent composable

- [ ] SubTask 5.1.1: 在 `app/encv-mobile/plugin-openlist/web/src/composables/useAgent.ts` 创建
- [ ] SubTask 5.1.2: 定义 `reactive messages[]` / `ref status` / `let sessionId` / `let eventOffset`
- [ ] SubTask 5.1.3: 实现 `processSSE(stream: ReadableStream)` 解析器
- [ ] SubTask 5.1.4: 实现 `send(text)` 发起对话 + 持久化
- [ ] SubTask 5.1.5: 实现 `confirmTool(toolCallId, approved)` 确认/拒绝
- [ ] SubTask 5.1.6: 实现 `resume()` mount 时自动续传
- [ ] SubTask 5.1.7: 实现 `saveState()` / `loadState()` localStorage 持久化（key: `agent:session:{sessionId}`）
- [ ] SubTask 5.1.8: 写 unit test（mock fetch + ReadableStream）覆盖事件分发

### Task 5.2: AgentChat.vue 渲染壳

- [ ] SubTask 5.2.1: 在 `app/encv-mobile/plugin-openlist/web/src/views/AgentChat.vue` 创建
- [ ] SubTask 5.2.2: 引入 `markstream-vue`（先加 package.json 依赖）
- [ ] SubTask 5.2.3: 写消息列表渲染（user 气泡右蓝色 / assistant 气泡左白底）
- [ ] SubTask 5.2.4: 写 MarkStream 集成（streaming prop + content source）
- [ ] SubTask 5.2.5: 写工具调用展示（✅ / ⚠️ 确认卡片 / ⏳ 等待）
- [ ] SubTask 5.2.6: 写确认卡片（tool name + args JSON + 确认/拒绝按钮）
- [ ] SubTask 5.2.7: 写输入框（idle/streaming 双态）
- [ ] SubTask 5.2.8: 写自动滚动到底部（nextTick + scrollTop）
- [ ] SubTask 5.2.9: 写 i18n key 占位（common / modals 词条）

### Task 5.3: 路由挂载

- [ ] SubTask 5.3.1: 修改 `app/encv-mobile/plugin-openlist/web/src/router/index.ts` 加 `/agent` 路由 → AgentChat
- [ ] SubTask 5.3.2: 在 OpenListHome 或 Tabs 加入口按钮（"💬 AI 助手"）
- [ ] SubTask 5.3.3: agent-demo 通过 preview-gateway 反代（`/agent-ui/` upstream）—— v2 任务

### Task 5.4: markstream-vue 集成

- [ ] SubTask 5.4.1: 在 `plugin-openlist/web/package.json` 加 `markstream-vue` 依赖
- [ ] SubTask 5.4.2: 跑 `pnpm install` 验证能装上
- [ ] SubTask 5.4.3: 写最小 demo 验证 `<MarkStream :source="text" :streaming="true" />` 能渲染

---

## Phase 6: 端到端验证

### Task 6.1: 沙箱 dev 联调

- [ ] SubTask 6.1.1: `pm2` 加 `agent-demo` app（`cmd/agent-demo/main.go` 编译 + 跑在 :5245）
- [ ] SubTask 6.1.2: `pm2 save` 持久化
- [ ] SubTask 6.1.3: 启动 `agent-demo` 后，`curl -N http://localhost:5245/api/chat` 验证 SSE 正常
- [ ] SubTask 6.1.4: 浏览器打开 `/openlist-ui/#/agent`，输入 "list files" → 验证 list_files 被自动执行 + UI 显示结果
- [ ] SubTask 6.1.5: 输入 "delete foo.txt" → 验证 ⚠️ 确认卡片出现，点击确认 → 验证 delete 执行
- [ ] SubTask 6.1.6: 中途刷新页面 → 验证 resume 追平进度

### Task 6.2: 单元测试

- [ ] SubTask 6.2.1: `go test ./agent/...` 全绿
- [ ] SubTask 6.2.2: `pnpm test` 跑 vitest，useAgent 测试通过
- [ ] SubTask 6.2.3: 覆盖率 ≥ 70%

### Task 6.3: 文档同步

- [ ] SubTask 6.3.1: 在 `unify-sandbox-preview-port/spec.md` 加 D16 章节：agent-demo upstream 路由
- [ ] SubTask 6.3.2: 在 `unify-sandbox-preview-port/tasks.md` 加对应 task
- [ ] SubTask 6.3.3: 在 `unify-sandbox-preview-port/checklist.md` 加检查点

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
| 5.2 (AgentChat) | 5.1, 5.4 |
| 5.3 (路由) | 5.2 |
| 5.4 (markstream-vue) | — |
| 6.1 (联调) | 4.1, 5.3 |
| 6.2 (单测) | 全部 |
| 6.3 (文档) | 6.1 |

---

# 估算

- Phase 1: 0.5 天（types + registry）
- Phase 2: 2 天（Agent 核心，含 OpenAI 集成 + 递归 + 并发）
- Phase 3: 0.5 天（HTTP/SSE）
- Phase 4: 0.5 天（demo）
- Phase 5: 1 天（Vue 渲染壳 + markstream）
- Phase 6: 1 天（联调 + 测试 + 文档）

**总计：~5.5 天**
