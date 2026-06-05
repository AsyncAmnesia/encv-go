# Checklist: Go Agent 进程内闭环库 + Vue 渲染壳

> 每个 checkpoint 都要勾选才能算 spec 完成。验证方法见每项 `[verify]` 标注。

---

## Go Agent 库

### 核心类型

- [ ] `agent/types.go` 定义 `EventType` string + 4 个常量
- [ ] `agent/types.go` 定义 `Event{Type, Data string}` 结构
- [ ] `agent/types.go` 定义 `ToolCallData{ID, Name, Args, AutoRun}` 结构
- [ ] `agent/types.go` 定义 `ToolResultData{ID, Name, Result, IsError}` 结构
- [ ] `agent/types.go` 定义 `MessageData` 结构（流式累积用）
- [ ] `types_test.go` 覆盖 JSON 序列化/反序列化 round-trip
- [ ] `[verify]` `go test ./agent/... -run TestTypes` 通过

### 工具注册中心

- [ ] `agent/registry.go` 定义 `ToolDefinition{Schema, Handler, NeedConfirm}`
- [ ] `agent/registry.go` 定义 `ToolRegistry{tools map, mutex}`
- [ ] `NewRegistry()` 构造函数
- [ ] `Register(name, schema, handler, needConfirm)` 方法
- [ ] `Get(name) (ToolDefinition, bool)` 方法
- [ ] `GetAllSchemas() []any` 方法
- [ ] `registry_test.go` 覆盖：注册、并发 Get（10 goroutine race-free）、Get 不存在 name
- [ ] `[verify]` `go test ./agent/... -run TestRegistry` 通过 + `go test -race` 无 race warning

### Agent 核心

- [ ] `agent/agent.go` 定义 `SessionCache{Events, IsFinished, mu}`
- [ ] `agent/agent.go` 实现 `pushAndSend(ch, e)` helper
- [ ] `agent/agent.go` 定义 `Agent{registry, sessions, apiKey, openaiClient}`
- [ ] `agent/agent.go` 实现 `NewAgent(apiKey, registry)`
- [ ] `agent/agent.go` 实现 `Chat(sessionID, messages) (<-chan, error)`
- [ ] `agent/agent.go` 实现自动执行路径（needConfirm=false → 立即执行 → 推 ToolResult → 递归）
- [ ] `agent/agent.go` 实现挂起路径（needConfirm=true → 推 EventToolCall + EventStreamEnd → 不执行）
- [ ] `agent/agent.go` 实现 `ConfirmTool(sessionID, toolCallID, approved)` 恢复执行
- [ ] `agent/agent.go` 实现拒绝路径（approved=false → 推 ToolResult error → 递归）
- [ ] `agent/agent.go` 实现 `Resume(sessionID, offset)` 重放
- [ ] `agent/agent.go` 实现 Resume 等待机制（offset == len(Events) && !IsFinished → sleep 50ms）
- [ ] `agent_test.go` 覆盖：Chat 流（mock OpenAI server）、ConfirmTool 恢复、Resume 重放、session 不存在 error
- [ ] `[verify]` `go test ./agent/... -run TestAgent` 通过

### OpenAI 集成

- [ ] `agent/openai.go` 引入 `github.com/sashabaranov/go-openai`
- [ ] `agent/openai.go` 实现 `createChatCompletionStream(messages, tools)`
- [ ] `agent/openai.go` 实现 `parseDelta(delta) (text, toolCalls, isFinished)`
- [ ] `openai_test.go` mock OpenAI HTTP server 返回流
- [ ] `[verify]` `go test ./agent/... -run TestOpenAI` 通过

### HTTP/SSE Handlers

- [ ] `agent/http.go` 定义 `ChatRequest / ResumeRequest / ConfirmRequest`（json tags）
- [ ] `agent/http.go` 实现 `HandleChat(w, r)` —— 调 Chat + 写 SSE
- [ ] `agent/http.go` 实现 `HandleResume(w, r)` —— 调 Resume + 写 SSE
- [ ] `agent/http.go` 实现 `HandleConfirm(w, r)` —— 调 ConfirmTool + 写 SSE
- [ ] `agent/http.go` 实现 `writeSSE(w, event)` 工具（`data: {json}\n\n`）
- [ ] `http_test.go` mock ResponseWriter 验证 SSE 格式合规
- [ ] `[verify]` `go test ./agent/... -run TestHTTP` 通过

### 演示程序

- [ ] `agent/cmd/agent-demo/main.go` 创建
- [ ] demo 注册 list_files (auto-run, mock 返回 ["a.txt", "b.txt"])
- [ ] demo 注册 delete_file (need confirm, mock 打印日志)
- [ ] demo 注册 search_files (auto-run, mock 模糊匹配)
- [ ] demo mount 3 个 handler 到 :5245
- [ ] `[verify]` `go run ./agent/cmd/agent-demo` 启动后 `curl -N http://localhost:5245/api/chat -d '{"messages":[{"role":"user","content":"list files"}]}'` 输出 SSE

---

## Vue 渲染壳

### useAgent composable

- [ ] `useAgent.ts` 创建
- [ ] `reactive messages[]` + `ref status` + `sessionId/offset` 内部变量
- [ ] `processSSE(stream)` 解析器（TextDecoder + 行 split + JSON.parse）
- [ ] `send(text)` 推 user 消息 + 空 assistant + fetch /api/chat
- [ ] `confirmTool(toolCallId, approved)` 调 /api/confirm
- [ ] `resume()` mount 时从 localStorage 读 + fetch /api/resume
- [ ] `saveState/loadState` localStorage 持久化（key: `agent:session:{sessionId}`）
- [ ] 事件类型分发：text_delta → append content；tool_call → push tool_calls；tool_result → push tool_results；stream_end → status='idle'
- [ ] `useAgent.test.ts` 覆盖事件分发
- [ ] `[verify]` `pnpm test -- useAgent` 通过

### AgentChat.vue 渲染壳

- [ ] `AgentChat.vue` 创建
- [ ] 引入 `markstream-vue` + `markstream-vue/dist/style.css`
- [ ] 消息列表渲染（v-for messages）
- [ ] user 气泡：右对齐 + 蓝色 + 圆角
- [ ] assistant 气泡：左对齐 + 白底 + shadow
- [ ] assistant 内容：MarkStream 组件（streaming prop）
- [ ] 工具调用展示：
  - [ ] 已执行（tool_results 包含）→ ✅ 绿色 + 工具名
  - [ ] 需确认（needsConfirm && !executed）→ ⚠️ 黄色卡片（name + args JSON + 确认/取消按钮）
  - [ ] 等待中 → ⏳ 灰色 pulse + 工具名
- [ ] 输入框：idle 显示 ➤，streaming 显示 ⏹（停止 = abort controller）
- [ ] 自动滚动到底部（nextTick + scrollTop = scrollHeight）
- [ ] i18n key 占位（common.send / modals.confirm / modals.reject）
- [ ] `[verify]` 浏览器打开 /agent，输入消息看到流式渲染

### 路由挂载

- [ ] `router/index.ts` 加 `/agent` 路由 → AgentChat
- [ ] OpenListHome 或 Tabs 加 "💬 AI 助手" 入口按钮
- [ ] `[verify]` 浏览器点击入口跳到 /agent 路由

### markstream-vue 集成

- [ ] `plugin-openlist/web/package.json` 加 `markstream-vue: ^x.x.x` 依赖
- [ ] `pnpm install` 成功
- [ ] 最小 demo：`<MarkStream :source="text" :streaming="true" />` 能渲染代码块/表格
- [ ] `[verify]` `pnpm dev` 起后访问测试页，code block 渐进渲染

---

## 端到端验证

### 沙箱 dev 联调

- [ ] `ecosystem.config.cjs` 加 `agent-demo` app（编译 + 跑 :5245）
- [ ] `pm2 save` 持久化
- [ ] `curl -N http://localhost:5245/api/chat` SSE 正常流
- [ ] 浏览器 `/openlist-ui/#/agent` 输入 "list files" → UI 展示 ✅ list_files + 文件列表
- [ ] 浏览器输入 "delete foo.txt" → ⚠️ 确认卡片出现 → 点击确认 → UI 展示 ✅ delete_file
- [ ] 浏览器输入 "delete foo.txt" → 拒绝 → UI 展示 ToolResult error（user_rejected）
- [ ] 流式过程中刷新页面 → 几秒内 resume 追平进度，UI 完整复现
- [ ] `[verify]` 0 个 console error、0 个 SSE 解析失败

### 单元测试

- [ ] `go test ./agent/...` 全绿
- [ ] `go test -race ./agent/...` 无 race warning
- [ ] `pnpm test` vitest 全绿
- [ ] 覆盖率：Go ≥ 70%、TypeScript ≥ 70%
- [ ] `[verify]` CI 测试 pipeline 绿

### 文档同步

- [ ] `unify-sandbox-preview-port/spec.md` 加 D16 章节（agent-demo upstream）
- [ ] `unify-sandbox-preview-port/tasks.md` 加对应 task
- [ ] `unify-sandbox-preview-port/checklist.md` 加检查点（agent-demo 路由 + 联调）
- [ ] `agent/README.md` 含使用示例 + API 一览
- [ ] `[verify]` 文档 review 通过

---

## 非功能性需求

### 性能

- [ ] Chat 首次 Event 延迟 < 500ms（OpenAI TTFB 范围内）
- [ ] SSE 写入不阻塞（每事件写完即 flush）
- [ ] Resume 追平进度延迟 < 1s（50ms polling）

### 安全

- [ ] Tool Handler 返回 error 不暴露内部堆栈（只 `error.Error()` 字符串）
- [ ] HTTP handler 设 `Content-Type: text/event-stream` 严格，无任何 HTML 注入
- [ ] SessionID 由 server 生成（UUIDv4），不接受客户端传入作为唯一信任

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
