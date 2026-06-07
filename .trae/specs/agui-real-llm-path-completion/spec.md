# AG-UI 真实 LLM 路径补齐 Spec

## Why

当前 `multi-engine-chat-architecture` spec Phase 4 (Task 4.3) 已实现 **mock 模式**下的 AG-UI 输出适配（`AGUIEventMapper` 完整 7 种事件映射），但**真实 LLM 路径**（`callOpenAIStream` → `streamChat`）**完全没有**接 AG-UI 格式输出。这导致 TDesign 引擎在前端虽然配了 `protocol: 'agui'`，但只要用户没开 mock 模式，AG-UI 解析器收到的是自定义 `text_delta` / `tool_call` / `tool_result` 事件流——**AG-UI 协议解析器无法识别，TDesign 渲染层直接挂掉**。

**实际后果**：
1. TDesign 引擎**只在 mock 模式可见**——演示场景下能用，真机 LLM 场景下就是死的
2. 用户在 Settings 切到 TDesign 引擎 + 真实 API 模式 → 发送消息 → TDesign 不渲染（拿到非 AG-UI 事件）
3. 引擎切换器的"无消息丢失"承诺在 TDesign ↔ Default 切换时实际违反（消息流格式不兼容）
4. spec 里 `Task 4.3` 原文只覆盖 `MockEngine.Run` 的 `aguiMode` 参数，没提 `streamChat` 的 AG-UI 适配，是 spec 设计本身遗漏

**新方案核心价值**：
1. **TDesign 引擎真实可用** — 切到 TDesign + 真实 LLM 也能正常渲染
2. **代码 0 改动即可用** — `AGUIEventMapper` 已经写好，只需在 `streamChat` 调用处判断 `aguiMode` 分支即可
3. **保留自定义 SSE 路径** — DefaultEngine / CopilotKitStyleEngine 不受影响（用原有 `sendAndCache`）
4. **Mock 路径不变** — 现有 mock 模式 AG-UI 输出继续走 `MockEngine.Run` 的 `aguiMode` 路径

---

## What Changes

### 修改

- `internal/server/agent_tool_loop.go` — `streamChat` 函数签名增加 `aguiMode bool` 参数；函数内部把 `s.sendAndCache(...)` / `s.sendSSEEventSafe(...)` 替换为统一 `emitEvent(evType, data)` 闭包；闭包根据 `aguiMode` 决定走 `sendAndCache`（自定义 SSE）还是 `AGUIEventMapper.MapEvent`（AG-UI 标准）
- `internal/server/agent_api.go` — `handleAgentChat` 在真实 LLM 路径把 `aguiMode` 透传给 `callOpenAIStream` / `streamChat`（通过返回值或函数参数）
- `internal/server/agent_api.go` — `handleAgentConfirm` 同样识别 `X-Agent-Protocol: agui` header 并透传 `aguiMode`
- `internal/server/agent_api.go` — `handleAgentResume` 同样识别 header 并透传 `aguiMode`

### 不影响

- Mock 路径的 AG-UI 输出（`MockEngine.Run` 已支持 `aguiMode`，本次不动）
- `AGUIEventMapper` 实现本体（已完整，7 种事件映射到位）
- 前端 `TDesignChatView.vue`（端点已配 `?protocol=agui`，只需后端能正确响应）
- DefaultEngine / CopilotKitStyleEngine（继续走自定义 SSE 格式，0 改动）

---

## ADDED Requirements

### Requirement: streamChat 支持 aguiMode 透传

`streamChat` 函数 SHALL 接受 `aguiMode bool` 参数，控制事件输出格式。

#### Scenario: 函数签名变更

**BEFORE**:
```go
func (s *Server) streamChat(ctx context.Context, c *gin.Context, cfg AgentConfig, model string, temperature float64, messages []chatMsg, sess *agentSession, openAITools []map[string]interface{}, toolMeta map[string]map[string]interface{})
```

**AFTER**:
```go
func (s *Server) streamChat(ctx context.Context, c *gin.Context, cfg AgentConfig, model string, temperature float64, messages []chatMsg, sess *agentSession, openAITools []map[string]interface{}, toolMeta map[string]map[string]interface{}, aguiMode bool)
```

#### Scenario: 闭包 emitEvent 分流

`streamChat` 内部 SHALL 用一个 `emitEvent` 闭包统一所有事件出口：

```go
var emitEvent func(evType string, data map[string]interface{})
if aguiMode {
    aguiMapper := NewAGUIMapper(c.Writer, flusher, sess.SessionID)
    emitEvent = func(evType string, data map[string]interface{}) {
        aguiMapper.MapEvent(MockEvent{Type: evType, Data: data}, 0, 0)
    }
} else {
    emitEvent = s.sendAndCache // 或 s.sendSSEEventSafe 包装
}
```

- 所有原本调用 `s.sendAndCache(sess, w, flusher, evType, data)` 的地方 → 改为 `emitEvent(evType, data)`
- 所有原本调用 `s.sendSSEEventSafe(c.Writer, flusher, evType, data)` 的地方 → 改为 `emitEvent(evType, data)`
- 闭包内部捕获 `sess` / `w` / `flusher` / `aguiMode`（避免到处改函数签名）

#### Scenario: 流式调 LLM 的 tool_call 累积仍走 sendAndCache

- **WHEN** 真实 LLM 流式响应含 `tool_call` delta
- **THEN** `tool_call` 事件**始终**走 `sendAndCache`（自定义 SSE 格式）
- **AND** 仅在 `streamChat` 出口处（最终推送 tool_call 完整数据给客户端时）才判断 `aguiMode` 转换
- **原因**：工具调用的累积逻辑（多次 delta 合并成一个完整 tool_call）发生在 `streamChat` 内部，仅在最终 `emitEvent` 时决定格式

#### Scenario: stream_status / 循环进度事件不映射到 AG-UI

- **WHEN** `streamChat` 推送 `stream_status{status:"thinking"}` 进度事件
- **THEN** 在 `aguiMode=true` 模式下 → **静默跳过**（AGUIEventMapper 的 default case 不处理）
- **AND** 在 `aguiMode=false` 模式下 → 正常推送自定义 SSE 事件
- **依据**：`AGUIEventMapper.MapEvent` 已有 default 跳过逻辑

#### Scenario: 单元测试覆盖

`internal/server/agent_tool_loop_test.go`（新建或补充） SHALL 覆盖：

- [ ] `aguiMode=true` 时 `streamChat` 输出含 `event: RUN_STARTED` / `event: TEXT_MESSAGE_CONTENT` / `event: TOOL_CALL_START` / `event: RUN_FINISHED` SSE 行
- [ ] `aguiMode=true` 时不输出 `data: {...}` 自定义事件（仅 AG-UI 格式）
- [ ] `aguiMode=false` 时输出原有 `data: {...}` 格式，不含 `event:` 行
- [ ] `stream_status` 事件在 `aguiMode=true` 时**不**出现在输出流中
- [ ] `tool_call` 累积逻辑（多次 delta → 一次完整 ARGS）在两种模式下都正确

---

### Requirement: handleAgentChat 真实 LLM 路径透传 aguiMode

`handleAgentChat` SHALL 在真实 LLM 路径（mock 短路之后）将 `aguiMode` 透传到 `callOpenAIStream` / `streamChat`。

#### Scenario: 透传点

**BEFORE**（`agent_api.go:900`）:
```go
streamCh, err := callOpenAIStream(c.Request.Context(), cfg, model, body.Temperature, loopMessages, openAITools)
```

**AFTER**:
```go
streamCh, err := callOpenAIStream(c.Request.Context(), cfg, model, body.Temperature, loopMessages, openAITools, aguiMode)
```

**BEFORE**（`agent_api.go:1579`，`handleAgentConfirm` 调 `streamChat`）:
```go
s.streamChat(c.Request.Context(), c, cfg, sess.LastModel, sess.LastTemperature, finalMessages, sess, openAITools, toolMeta)
```

**AFTER**:
```go
s.streamChat(c.Request.Context(), c, cfg, sess.LastModel, sess.LastTemperature, finalMessages, sess, openAITools, toolMeta, aguiMode)
```

#### Scenario: 真实 LLM 路径 + aguiMode=true 时输出格式

- **WHEN** 用户在 Settings 选 TDesign 引擎 + 真实 API 模式
- **AND** 前端发 `POST /api/chat?protocol=agui`
- **AND** mock 模式关闭（`mockMode == "off"`）
- **THEN** 后端走真实 LLM 路径
- **AND** 响应流是 AG-UI 标准格式（`event: RUN_STARTED` / `event: TEXT_MESSAGE_CONTENT` / ...）
- **AND** TDesign 引擎能正常解析并渲染

#### Scenario: 真实 LLM 路径 + aguiMode=false 时输出格式

- **WHEN** 前端用 DefaultEngine / CopilotKitStyleEngine
- **AND** 请求不含 `X-Agent-Protocol: agui` header 也不含 `?protocol=agui` query
- **THEN** 真实 LLM 路径走原有自定义 SSE 格式
- **AND** 与改造前**字节级一致**（不破坏现有前端）

---

### Requirement: handleAgentConfirm / handleAgentResume 支持 aguiMode

`handleAgentConfirm` 和 `handleAgentResume` SHALL 同样识别 AG-UI 协议请求头 / query 并透传 `aguiMode`。

#### Scenario: confirm 时 TDesign 也能继续走 AG-UI

- **WHEN** TDesign 引擎用户点击 ApprovalCard "批准" 按钮
- **AND** 前端发 `POST /api/confirm` + `X-Agent-Protocol: agui` header
- **THEN** `handleAgentConfirm` 识别 header
- **AND** 把 `aguiMode=true` 透传给 `s.streamChat(...)`
- **AND** confirm 后的流式响应继续是 AG-UI 格式（TDesign 不需要切换协议）

#### Scenario: resume 时 TDesign 也能续传

- **WHEN** TDesign 引擎 App 重启 / WebView 刷新
- **AND** 前端发 `POST /api/resume` + `X-Agent-Protocol: agui` header
- **THEN** `handleAgentResume` 识别 header
- **AND** 续传的事件流是 AG-UI 格式
- **AND** 实际：本轮 resume 路径直接调 `streamChat`（line 1579），透传 `aguiMode` 即可

---

### Requirement: 关键文件 / 函数

- `internal/server/agent_tool_loop.go` — `streamChat` 函数签名 + 内部 `emitEvent` 闭包
- `internal/server/agent_api.go` — `handleAgentChat` / `handleAgentConfirm` / `handleAgentResume` 三处 `aguiMode` 透传
- `internal/server/agent_tool_loop_test.go` — 5 个新单元测试（aguiMode=true/false × 多个事件类型）

---

## MODIFIED Requirements

### Requirement: streamChat 函数签名

**BEFORE**: `streamChat` 不感知 AG-UI 协议
**AFTER**: `streamChat` 接受 `aguiMode bool` 参数，内部 `emitEvent` 闭包统一出口

#### Scenario: 兼容性

- `aguiMode=false` 行为与改造前**完全一致**（字节级 SSE 输出）
- 所有调用方（`handleAgentChat` / `handleAgentConfirm`）必须更新传参

### Requirement: handleAgentChat 真实 LLM 路径

**BEFORE**: 真实 LLM 路径忽略 `aguiMode` 变量
**AFTER**: 真实 LLM 路径把 `aguiMode` 透传给 `callOpenAIStream` / `streamChat`

#### Scenario: 透传链

- `handleAgentChat` line 798 检测 header/query → 获得 `aguiMode bool`
- 真实 LLM 路径 line 900 `callOpenAIStream(..., aguiMode)` 透传
- 真实 LLM 路径内部 `s.streamChat(..., aguiMode)` 透传
- `streamChat` 内部 `emitEvent` 闭包用 `aguiMode` 决定走哪个 mapper

### Requirement: handleAgentConfirm / handleAgentResume

**BEFORE**: 不识别 `X-Agent-Protocol` header
**AFTER**: 识别 header 并透传给 `streamChat`

---

## REMOVED Requirements

无（仅新增 aguiMode 透传能力，不删除任何现有能力）

---

## 约束与限制

1. **`AGUIEventMapper` 不动** — 已实现完整的 7 种事件映射，本 spec 不重写
2. **Mock 路径不动** — 现有 `MockEngine.Run` 的 `aguiMode` 变参保持不变
3. **真实 LLM 路径的工具执行** — `streamChat` 内部 `executeAndRecurse` 调用 `executeAgentTool`（plugin / fs 工具）继续走原有逻辑，AG-UI 仅影响**输出格式**不影响**内部工具派发**
4. **tool_call 累积仍走 sendAndCache** — 仅在最终推送完整 tool_call 时才用 `emitEvent`，避免累积中段污染 AG-UI 流
5. **stream_status 不映射** — AG-UI 标准协议没有"循环进度"概念，AGUIEventMapper 已默认跳过

---

## 与现有 spec 的关系

| 现有 spec | 影响 |
|----------|------|
| `multi-engine-chat-architecture` Phase 4 | **本 spec 是其补充** — 补齐 Task 4.3 遗漏的真实 LLM 路径 |
| `agent-mock-mode` | 不受影响（mock 路径 AG-UI 输出已独立实现） |
| `go-in-process-agent` | 不受影响（前端 `useAgent` 走 DefaultEngine，不需 AG-UI） |

---

## 验证步骤

1. **类型检查** — `go build ./cmd/encv` 0 错误
2. **单元测试** — `go test ./internal/server/... -run TestStreamChat -v` 全部通过
3. **真实 LLM + AG-UI 集成** —
   - 启动服务，关闭 mock 模式
   - Settings 选 TDesign 引擎
   - 浏览器开发者工具 → Network → 找到 `?protocol=agui` 请求
   - 验证响应流是 `event: RUN_STARTED\ndata: {"runId":"..."}\n\n` 格式
4. **真实 LLM + 自定义 SSE 回归** —
   - DefaultEngine 发送消息
   - 验证响应流仍是原有 `data: {"type":"text_delta",...}` 格式
   - UI 渲染与改造前像素级一致
5. **TDesign 端到端** — 切到 TDesign + 真实 API → 发送消息 → 验证消息气泡出现 + 工具调用以 TDesign 组件渲染
6. **Confirm 路径** — TDesign 引擎触发需确认工具 → 点批准 → 验证 confirm 后流式响应继续 AG-UI 格式

---

## 端点契约总结

| 端点 | 触发 AG-UI 的方式 | 输出格式（aguiMode=true） | 输出格式（aguiMode=false） |
|------|----------------|------------------------|-----------------------|
| `POST /api/chat` | `X-Agent-Protocol: agui` header 或 `?protocol=agui` query | AG-UI 标准 | 自定义 SSE（默认） |
| `POST /api/confirm` | 同上 | AG-UI 标准 | 自定义 SSE |
| `POST /api/resume` | 同上 | AG-UI 标准 | 自定义 SSE |
| `GET /api/agent/context-usage` | 不变 | 不变 | 不变 |
