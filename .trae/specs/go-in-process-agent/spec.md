# Spec: Go Agent 进程内闭环库 + Vue 渲染壳

> **核心思路**：把 Agent 写进 Go 应用进程内，AI 直接调用应用自己的 Go 方法（零跨语言绑定、零伴生进程）。
> **架构简化**：Go Agent Library（注册中心 + 流控）+ Vue 极薄渲染层（只发消息、收 SSE、渲染 Markdown、展示确认 UI）。

---

## Why

当前 ENCV + OpenList 的 AI 集成方式（如果有）是外部 HTTP/MCP 桥接，AI 与应用本体之间有网络边界，无法直接调用 Go 方法、无法利用进程内缓存、无法保证调用安全性。

**新方案核心价值**：

1. **零网络开销**：Agent 跑在应用进程内，工具调用就是 Go 函数调用，零序列化、零 IPC。
2. **绝对安全**：不需要把 Go 方法暴露成 HTTP 端点，能力调用完全在进程内完成。
3. **天然支持断点续传**：Go 进程不挂，内存缓存就在。前端 WebView 刷新、App 重启，/resume 接口瞬间追平进度。
4. **可复用**：agent 库是独立 Go module，可被 OpenList、encv-go、任何 Go 应用 import。
5. **前端解耦**：Vue 不知道后端有什么工具，只根据 `Event` 流渲染 UI。换后端、换工具，前端零改动。

---

## What Changes

### 新增
- `agent/` 顶层 Go module —— 可复用的 Agent 进程内库
  - `types.go` — Event / EventType / ToolCallData / ToolResultData / MessageData
  - `registry.go` — ToolRegistry（注册中心，线程安全）
  - `agent.go` — Agent 核心（Chat / ConfirmTool / Resume + SessionCache）
  - `openai.go` — OpenAI 流式客户端（tool_calls 处理 + 多轮递归）
  - `http.go` — HTTP/SSE handlers（`/api/chat` / `/api/resume` / `/api/confirm`）
  - `cmd/agent-demo/main.go` — 演示程序（注册 list_files + delete_file）
  - `go.mod` / `README.md` — 模块声明 + 使用说明
- `app/encv-mobile/plugin-openlist/web/src/composables/useAgent.ts` — Vue 复合式
- `app/encv-mobile/plugin-openlist/web/src/views/AgentChat.vue` — 渲染壳组件
- `app/encv-mobile/plugin-openlist/web/src/router/index.ts` — 注册 `/agent` 路由

### 修改
- `app/encv-mobile/plugin-openlist/web/package.json` — 新增 `markstream-vue` 依赖
- `app/encv-mobile/plugin-openlist/web/src/router/index.ts` — 新增 AgentChat 路由

### 影响的现有 spec
- `wire-openlist-runtime-and-ui-v2` — OpenList 集成与 UI
- `unify-sandbox-preview-port` — preview-gateway 路由（未来加 `/agent-ui/` upstream）

---

## ADDED Requirements

### Requirement: Go Agent 库核心类型

`agent` 包 SHALL 提供标准化的、与前端解耦的事件流类型。

#### Scenario: Event 类型契约

- **WHEN** Agent 向 SSE channel 推事件
- **THEN** 每个事件都是 `*Event{Type: EventType, Data: string (JSON)}`
- **AND** `EventType` 取值限于：`text_delta` | `tool_call` | `tool_result` | `stream_end`
- **AND** `Data` 字段是 JSON 字符串，前端按 `Type` 自行反序列化

#### Scenario: ToolCallData 字段

- **WHEN** LLM 返回 `tool_calls`
- **THEN** Agent 推 `EventToolCall`，`Data` 包含 `ToolCallData{ID, Name, Args, AutoRun}`
- **AND** `AutoRun = !def.NeedConfirm`（注册时声明的 needConfirm 决定）

#### Scenario: ToolResultData 字段

- **WHEN** 工具执行完成（自动或确认后）
- **THEN** Agent 推 `EventToolResult`，`Data` 包含 `ToolResultData{ID, Name, Result, IsError}`

---

### Requirement: 工具注册中心（ToolRegistry）

`agent` 包 SHALL 提供线程安全的工具注册中心，让应用能注册自己的 Go 原生能力。

#### Scenario: 注册工具

- **WHEN** 应用调用 `registry.Register(name, schema, handler, needConfirm)`
- **THEN** 工具被存入 `tools map[string]ToolDefinition`
- **AND** `ToolDefinition = {Schema, Handler, NeedConfirm}`
- **AND** 注册可并发安全（使用 `sync.RWMutex` 或 `sync.Map`）

#### Scenario: 获取所有 Schema

- **WHEN** Agent 调用 `registry.GetAllSchemas()`
- **THEN** 返回所有注册工具的 `Schema` 切片
- **AND** 该切片按 OpenAI tool_calls 格式传递给 LLM

#### Scenario: 查找工具

- **WHEN** LLM 返回 `tool_calls` 中某个 name
- **THEN** `registry.Get(name)` 返回 `ToolDefinition, bool`
- **AND** `bool == false` 时 Agent 跳过该 tool_call（不报错，继续流）

---

### Requirement: Agent 核心（Chat 流式对话）

`agent` 包 SHALL 提供流式对话能力，支持 tool_calls 自动执行与确认挂起。

#### Scenario: 发起对话

- **WHEN** 应用调用 `agent.Chat(sessionID, messages)`
- **THEN** 返回 `(<-chan *Event, error)`
- **AND** 内部创建 `SessionCache{Events: [], IsFinished: false}` 并存入 `sessions sync.Map`
- **AND** 后台 goroutine 启动：调 OpenAI 流 → 解析 delta → 转 Event → 推 channel + 写 cache

#### Scenario: 自动执行工具（needConfirm=false）

- **WHEN** LLM delta 包含 tool_call 且 `def.NeedConfirm == false`
- **THEN** Agent 推 `EventToolCall`（AutoRun=true）
- **AND** 立即同步执行 `def.Handler(args)`
- **AND** 推 `EventToolResult`
- **AND** 把 tool_result 追加到 messages，递归发起下一轮 LLM 调用

#### Scenario: 需确认工具（needConfirm=true）挂起

- **WHEN** LLM delta 包含 tool_call 且 `def.NeedConfirm == true`
- **THEN** Agent 推 `EventToolCall`（AutoRun=false）
- **AND** **不执行** Handler，**不追加** tool_result 到 messages
- **AND** 推 `EventStreamEnd` 结束当前 stream（挂起）
- **AND** 后续由前端调用 `ConfirmTool` 恢复

#### Scenario: 文本增量

- **WHEN** LLM delta 包含 `Content`
- **THEN** Agent 推 `EventTextDelta`，`Data` 包含 `{content: string}`
- **AND** 多个 text_delta 累积形成完整消息

---

### Requirement: Agent 工具确认（ConfirmTool）

`agent` 包 SHALL 提供确认恢复能力，接收前端 approve/reject 信号后继续推理。

#### Scenario: 确认执行（approved=true）

- **WHEN** 应用调用 `agent.ConfirmTool(sessionID, toolCallID, approved=true)`
- **THEN** 找到挂起的 `ToolDefinition`
- **AND** 同步执行 `def.Handler(args)`
- **AND** 推 `EventToolResult`
- **AND** 追加 tool_result 到 messages，递归发起下一轮 LLM 调用
- **AND** 返回新的 `<-chan *Event`

#### Scenario: 拒绝执行（approved=false）

- **WHEN** 应用调用 `agent.ConfirmTool(sessionID, toolCallID, approved=false)`
- **THEN** **不执行** Handler
- **AND** 推 `EventToolResult`（Result=`{"error":"user_rejected"}`, IsError=true）
- **AND** 追加 tool_result 到 messages（让 LLM 知道用户拒绝）
- **AND** 递归发起下一轮 LLM 调用
- **AND** 返回新的 `<-chan *Event`

---

### Requirement: 断点续传（Resume）

`agent` 包 SHALL 提供从 offset 重放事件流的能力（内存缓存，不依赖外部存储）。

#### Scenario: 续传未结束的会话

- **WHEN** 应用调用 `agent.Resume(sessionID, offset)`
- **THEN** 找到 `SessionCache`
- **AND** 启动 goroutine：从 `cache.Events[offset]` 开始，依次 push 到 channel
- **AND** 如果追到 `cache.IsFinished == true`，推 `EventStreamEnd` 后退出
- **AND** 如果还有事件在生成中（offset == len(Events)），阻塞等待，每 50ms 重试

#### Scenario: 会话不存在

- **WHEN** `sessionID` 不在 `sessions` map 中
- **THEN** 返回 `error`，前端应重置并发起新对话

---

### Requirement: HTTP/SSE 端点

`agent` 包 SHALL 提供标准化的 HTTP handlers，让任何 `net/http` 应用能挂载。

#### Scenario: POST /api/chat

- **WHEN** 前端 POST `/api/chat`，body=`{messages: [...], sessionId?: string}`
- **AND** 没有 sessionId → 生成新 UUID
- **THEN** Handler 调用 `agent.Chat(sessionId, messages)`
- **AND** 设置 `Content-Type: text/event-stream`、`Cache-Control: no-cache`、`Connection: keep-alive`
- **AND** 遍历 channel，每个 Event 序列化为 `data: {json}\n\n` 写入 ResponseWriter
- **AND** channel 关闭时关闭 ResponseWriter

#### Scenario: POST /api/resume

- **WHEN** 前端 POST `/api/resume`，body=`{sessionId, offset}`
- **THEN** Handler 调用 `agent.Resume(sessionId, offset)`
- **AND** 同 /api/chat 的 SSE 写入逻辑

#### Scenario: POST /api/confirm

- **WHEN** 前端 POST `/api/confirm`，body=`{sessionId, toolCallId, approved}`
- **THEN** Handler 调用 `agent.ConfirmTool(sessionId, toolCallId, approved)`
- **AND** 同 /api/chat 的 SSE 写入逻辑

---

### Requirement: Vue 复合式（useAgent）

`useAgent.ts` SHALL 提供 reactive 状态 + SSE 解析 + 断点续传。

#### Scenario: send() 发起对话

- **WHEN** 用户在输入框敲回车
- **THEN** `send(text)` 推 user 消息 + 空 assistant 消息到 `messages[]`
- **AND** 生成新 `sessionId = crypto.randomUUID()`
- **AND** 持久化 `{sessionId, eventOffset: 0, messages}` 到 localStorage
- **AND** `status = 'streaming'`
- **AND** fetch POST `/api/chat`，processSSE 处理响应流

#### Scenario: processSSE 解析事件

- **WHEN** SSE 流到达
- **THEN** 用 `ReadableStream.getReader()` + `TextDecoder` 按行解析
- **AND** 每行 `data: {json}` → `JSON.parse` → `Event` 对象
- **AND** `eventOffset++` 每次推后
- **AND** 按 type 分发：text_delta → append to last assistant；tool_call → push to tool_calls；tool_result → push to tool_results；stream_end → status='idle'

#### Scenario: confirmTool 确认

- **WHEN** 用户点击确认/拒绝按钮
- **THEN** `confirmTool(toolCallId, approved)` 立即调用
- **AND** `status = 'streaming'`
- **AND** fetch POST `/api/confirm`，processSSE 继续

#### Scenario: 启动时自动续传

- **WHEN** 组件 mount
- **THEN** `resume()` 从 localStorage 读 `{sessionId, eventOffset, messages}`
- **AND** 如果上次 status === 'streaming'，fetch POST `/api/resume`
- **AND** 追平进度

#### Scenario: 持久化

- **WHEN** `processSSE` 推完一个事件
- **THEN** `saveState()` 写 localStorage
- **AND** key: `agent:session:{sessionId}`

---

### Requirement: Vue 渲染壳（AgentChat.vue）

`AgentChat.vue` SHALL 提供聊天 UI，只渲染不知道后端细节。

#### Scenario: 消息气泡

- **WHEN** `messages` 包含 user 消息
- **THEN** 右对齐蓝色气泡
- **WHEN** `messages` 包含 assistant 消息
- **THEN** 左对齐白底气泡

#### Scenario: 流式 Markdown 渲染

- **WHEN** assistant 消息有 `content` 且 `status === 'streaming'`
- **THEN** 用 `MarkStream` 组件渲染（流式 + 复杂 Markdown 块完美呈现）
- **AND** `streaming=true` 触发代码块、表格渐进渲染

#### Scenario: 工具调用展示

- **WHEN** assistant 消息有 `tool_calls`
- **THEN** 每条 tool_call 按状态显示：
  - `tool_results.some(r => r.id === tc.id)` → ✅ 绿色 + 工具名
  - `tc.needsConfirm && !executed` → ⚠️ 黄色确认卡片（工具名 + args + 确认/取消按钮）
  - 等待中 → ⏳ 灰色 pulse + 工具名

#### Scenario: 输入框

- **WHEN** `status === 'idle'`
- **THEN** 显示 ➤ 发送按钮，可输入
- **WHEN** `status === 'streaming'`
- **THEN** 输入框 disabled，显示 ⏹ 停止按钮（停止 = 关闭 SSE 连接）

---

### Requirement: 应用集成示例（OpenList）

OpenList（Hi-Sillot fork）SHOULD 能用 ≤ 20 行代码集成 agent 库。

#### Scenario: 最小集成示例

```go
registry := agent.NewRegistry()
registry.Register("list_files", schema, func(args string) (string, error) {
    files, _ := openlist.ListFiles(parsePath(args))
    return json.Marshal(files)
}, false)  // 自动执行
registry.Register("delete_file", schema, func(args string) (string, error) {
    openlist.DeleteFile(parsePath(args))
    return `{"success":true}`, nil
}, true)  // 需确认

ag := agent.NewAgent(os.Getenv("OPENAI_API_KEY"), registry)

http.HandleFunc("/api/chat", ag.HandleChat)
http.HandleFunc("/api/resume", ag.HandleResume)
http.HandleFunc("/api/confirm", ag.HandleConfirm)
http.ListenAndServe(":5244", nil)
```

#### Scenario: 实际 OpenList fork 集成

- **WHEN** 提交 PR 到 `Hi-Sillot/OpenList` 把 agent 包 vendor 进去
- **THEN** PR review 焦点：go.mod 依赖方向、import path、OpenList 现有 LLM 入口是否替换

---

## MODIFIED Requirements

无（这是全新模块，不修改现有能力）

---

## REMOVED Requirements

无（保留现有 OpenList 真实后端 WebView 集成作为 fallback）

---

## 约束与限制

1. **OpenAI 依赖**：库使用 `github.com/sashabaranov/go-openai` 或类似。需要在 agent 库的 go.mod 明确。
2. **OpenList fork 集成需外部 PR**：本仓库不直接持有 OpenList 源码，agent 库的集成需要在 Hi-Sillot fork 仓库提 PR。
3. **session 内存缓存**：当前版本 session 存在 Go 进程内存，进程重启即丢。生产环境建议加 Redis / SQLite 持久化（v2 任务）。
4. **单个 session 只能串行**：ConfirmTool 调用时假设 session 处于挂起态，并发调用需加锁。
5. **沙箱 dev 阶段**：agent 库 demo 程序（`cmd/agent-demo/main.go`）独立运行在 :5245（不冲突 :5244 OpenList 真实后端），便于前端联调。
