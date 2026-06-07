# Tasks

## Phase 1: streamChat 闭包重构

- [ ] Task 1.1: 修改 `streamChat` 函数签名 + 内部 emitEvent 闭包
  - [ ] 1.1.1 在 `internal/server/agent_tool_loop.go` 给 `streamChat` 函数签名末尾加 `aguiMode bool` 参数
  - [ ] 1.1.2 在 `streamChat` 函数开头构造 `emitEvent` 闭包：`aguiMode=true` 时初始化 `AGUIEventMapper` 并用 `aguiMapper.MapEvent` 包装；`aguiMode=false` 时直接用 `s.sendAndCache` 包装
  - [ ] 1.1.3 把 `streamChat` 函数体内所有 `s.sendAndCache(sess, w, flusher, evType, data)` 调用替换为 `emitEvent(evType, data)`
  - [ ] 1.1.4 把所有 `s.sendSSEEventSafe(c.Writer, flusher, evType, data)` 调用替换为 `emitEvent(evType, data)`（progress / stream_status 事件）
  - [ ] 1.1.5 `tool_call` 累积完成后，最终推送时仍用 `emitEvent`（不绕过闭包）

- [ ] Task 1.2: 单元测试
  - [ ] 1.2.1 `TestStreamChat_AGUIMode_OutputsRUN_STARTED` — aguiMode=true 时第一行是 `event: RUN_STARTED\ndata: {...}\n\n`
  - [ ] 1.2.2 `TestStreamChat_AGUIMode_TextDeltaMapsToTEXT_MESSAGE_CONTENT` — text_delta 映射到 `event: TEXT_MESSAGE_CONTENT`
  - [ ] 1.2.3 `TestStreamChat_AGUIMode_ToolCallMapsToTOOL_CALL_START` — tool_call 映射到 `event: TOOL_CALL_START` + `event: TOOL_CALL_ARGS`
  - [ ] 1.2.4 `TestStreamChat_CustomMode_PreservesDataFormat` — aguiMode=false 时输出 `data: {"type":"text_delta",...}` 格式（不出现 `event:` 行）
  - [ ] 1.2.5 `TestStreamChat_AGUIMode_StreamStatusSkipped` — `stream_status` 事件在 aguiMode=true 时不出现在输出流
  - [ ] 1.2.6 `TestStreamChat_AGUIMode_ToolCallAccumulation` — 多次 tool_call delta 累积后只发一次完整 TOOL_CALL_START/ARGS

## Phase 2: handleAgentChat 真实 LLM 路径透传

- [ ] Task 2.1: `callOpenAIStream` 接受 aguiMode 参数
  - [ ] 2.1.1 在 `internal/server/agent_tool_loop.go` 给 `callOpenAIStream` 函数签名末尾加 `aguiMode bool` 参数
  - [ ] 2.1.2 内部透传给 `streamChat`
  - [ ] 2.1.3 `executeAndRecurse` 内部递归调 `streamChat` 时也透传 `aguiMode`

- [ ] Task 2.2: `handleAgentChat` 真实 LLM 路径调用更新
  - [ ] 2.2.1 `internal/server/agent_api.go` line 900 `callOpenAIStream(...)` 末尾加 `aguiMode` 实参
  - [ ] 2.2.2 mock 短路分支（line 818）`s.mockEngine.Run(...)` 已传 `aguiMode` 变参，保持不变
  - [ ] 2.2.3 `handleAgentChat` 内 `slog.Info("agent: ..."` 关键日志加 `agui_mode` 字段（真实 LLM 路径起始处）

- [ ] Task 2.3: 单元测试
  - [ ] 2.3.1 `TestHandleAgentChat_RealLLM_PassesAGUIModeToStreamChat` — 真实 LLM 路径下 `aguiMode=true` 时 streamChat 输出 AG-UI 格式
  - [ ] 2.3.2 `TestHandleAgentChat_RealLLM_DefaultNoAGUIHeader` — 默认请求走自定义 SSE 格式（回归测试）

## Phase 3: handleAgentConfirm / handleAgentResume 透传

- [ ] Task 3.1: `handleAgentConfirm` 识别 AG-UI 头并透传
  - [ ] 3.1.1 `internal/server/agent_api.go` `handleAgentConfirm` 函数体开头加 `aguiMode := c.GetHeader("X-Agent-Protocol") == "agui" || c.Request.URL.Query().Get("protocol") == "agui"`
  - [ ] 3.1.2 调 `s.streamChat(...)` 末尾加 `aguiMode` 实参

- [ ] Task 3.2: `handleAgentResume` 识别 AG-UI 头并透传
  - [ ] 3.2.1 `internal/server/agent_api.go` `handleAgentResume` 函数体开头加 `aguiMode := ...`（同 3.1.1）
  - [ ] 3.2.2 调 `s.streamChat(...)` 末尾加 `aguiMode` 实参

- [ ] Task 3.3: 单元测试
  - [ ] 3.3.1 `TestHandleAgentConfirm_AGUIHeader_PassesThrough` — confirm 路径下 X-Agent-Protocol: agui 触发 streamChat 的 aguiMode=true
  - [ ] 3.3.2 `TestHandleAgentResume_AGUIHeader_PassesThrough` — resume 路径同上

## Phase 4: 全量回归验证

- [ ] Task 4.1: 编译 + 类型检查
  - [ ] 4.1.1 `go build ./cmd/encv` 0 错误
  - [ ] 4.1.2 `go vet ./internal/server/...` 0 警告

- [ ] Task 4.2: 单测全跑
  - [ ] 4.2.1 `go test ./internal/server/... -run TestStreamChat -v` 全部通过
  - [ ] 4.2.2 `go test ./internal/server/... -run TestHandleAgentChat -v` 全部通过
  - [ ] 4.2.3 `go test ./internal/server/... -run TestHandleAgentConfirm -v` 全部通过
  - [ ] 4.2.4 `go test ./internal/server/... -run TestHandleAgentResume -v` 全部通过
  - [ ] 4.2.5 `go test ./internal/server/...` 全跑 0 回归

- [ ] Task 4.3: 前端 + 端到端验证
  - [ ] 4.3.1 前端 `vue-tsc --noEmit` 0 错误（**前端 0 改动**，仅验证编译）
  - [ ] 4.3.2 TDesign 引擎 + 真实 API 模式：发送消息 → Network 面板验证响应流是 AG-UI 格式（`event: RUN_STARTED` 等）→ TDesign 组件正常渲染
  - [ ] 4.3.3 DefaultEngine + 真实 API 模式：发送消息 → 验证响应流仍是 `data: {"type":"text_delta"...}` 自定义格式 → UI 与改造前像素级一致
  - [ ] 4.3.4 TDesign 引擎 + 触发确认工具（如 `video_encrypt`）→ 点批准 → confirm 后流式响应继续 AG-UI 格式
  - [ ] 4.3.5 TDesign 引擎 + App 重启触发 resume → 验证续传流是 AG-UI 格式

# Task Dependencies
- [Task 1.1] → [Task 1.2]（先重构再测）
- [Task 1.1] → [Task 2.1] → [Task 2.2] → [Task 2.3]（真实 LLM 路径依赖 streamChat 闭包）
- [Task 1.1] → [Task 3.1] → [Task 3.2] → [Task 3.3]（confirm / resume 路径也依赖 streamChat 闭包）
- [Phase 4] 必须在 Phase 1-3 全部完成后执行
