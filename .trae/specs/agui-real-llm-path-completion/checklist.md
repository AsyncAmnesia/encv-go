# Checklist

## Phase 1: streamChat 闭包重构

- [ ] `streamChat` 函数签名末尾增加 `aguiMode bool` 参数
- [ ] `streamChat` 函数体内构造 `emitEvent` 闭包统一事件出口
- [ ] `emitEvent` 在 `aguiMode=true` 时调用 `AGUIEventMapper.MapEvent`
- [ ] `emitEvent` 在 `aguiMode=false` 时调用 `s.sendAndCache`
- [ ] 所有 `s.sendAndCache(...)` 调用点已替换为 `emitEvent(...)`
- [ ] 所有 `s.sendSSEEventSafe(...)` 调用点已替换为 `emitEvent(...)`
- [ ] `tool_call` 累积逻辑（多次 delta → 一次完整 ARGS）仍正确
- [ ] 单元测试 `TestStreamChat_AGUIMode_OutputsRUN_STARTED` 通过
- [ ] 单元测试 `TestStreamChat_AGUIMode_TextDeltaMapsToTEXT_MESSAGE_CONTENT` 通过
- [ ] 单元测试 `TestStreamChat_AGUIMode_ToolCallMapsToTOOL_CALL_START` 通过
- [ ] 单元测试 `TestStreamChat_CustomMode_PreservesDataFormat` 通过（回归保护）
- [ ] 单元测试 `TestStreamChat_AGUIMode_StreamStatusSkipped` 通过
- [ ] 单元测试 `TestStreamChat_AGUIMode_ToolCallAccumulation` 通过

## Phase 2: handleAgentChat 真实 LLM 路径透传

- [ ] `callOpenAIStream` 函数签名末尾增加 `aguiMode bool` 参数
- [ ] `callOpenAIStream` 内部透传 `aguiMode` 给 `streamChat`
- [ ] `executeAndRecurse` 内部递归调 `streamChat` 时透传 `aguiMode`
- [ ] `handleAgentChat` line 900 `callOpenAIStream(...)` 末尾加 `aguiMode` 实参
- [ ] mock 短路分支 line 818 `s.mockEngine.Run(...)` 的 `aguiMode` 变参保持不变
- [ ] 关键 slog.Info 日志加 `agui_mode` 字段
- [ ] 单元测试 `TestHandleAgentChat_RealLLM_PassesAGUIModeToStreamChat` 通过
- [ ] 单元测试 `TestHandleAgentChat_RealLLM_DefaultNoAGUIHeader` 通过

## Phase 3: handleAgentConfirm / handleAgentResume 透传

- [ ] `handleAgentConfirm` 函数体开头识别 `X-Agent-Protocol: agui` header 和 `?protocol=agui` query
- [ ] `handleAgentConfirm` 调 `s.streamChat(...)` 末尾加 `aguiMode` 实参
- [ ] `handleAgentResume` 函数体开头识别 header/query
- [ ] `handleAgentResume` 调 `s.streamChat(...)` 末尾加 `aguiMode` 实参
- [ ] 单元测试 `TestHandleAgentConfirm_AGUIHeader_PassesThrough` 通过
- [ ] 单元测试 `TestHandleAgentResume_AGUIHeader_PassesThrough` 通过

## Phase 4: 全量回归验证

- [ ] `go build ./cmd/encv` 0 错误
- [ ] `go vet ./internal/server/...` 0 警告
- [ ] `go test ./internal/server/... -run TestStreamChat -v` 全部通过
- [ ] `go test ./internal/server/... -run TestHandleAgentChat -v` 全部通过
- [ ] `go test ./internal/server/... -run TestHandleAgentConfirm -v` 全部通过
- [ ] `go test ./internal/server/... -run TestHandleAgentResume -v` 全部通过
- [ ] `go test ./internal/server/...` 全跑 0 回归
- [ ] 前端 `vue-tsc --noEmit` 0 错误（前端 0 改动）
- [ ] TDesign 引擎 + 真实 API 模式：响应流是 AG-UI 格式（`event: RUN_STARTED` 等）TDesign 组件正常渲染
- [ ] DefaultEngine + 真实 API 模式：响应流仍是 `data: {"type":"text_delta"...}` 自定义格式，UI 与改造前像素级一致
- [ ] TDesign 引擎 + 触发确认工具 → 点批准 → confirm 后流式响应继续 AG-UI 格式
- [ ] TDesign 引擎 + App 重启触发 resume → 续传流是 AG-UI 格式
