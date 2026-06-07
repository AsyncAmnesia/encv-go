// internal/server/agent_agui_adapter.go
//
// AG-UI 协议适配层 —— 将内部自定义 SSE 事件映射为标准 AG-UI 格式。
//
// AG-UI (Agent User Interface) 是一种标准化的 agent 通信协议，
// 定义了 RUN_STARTED / TEXT_MESSAGE_CONTENT / TOOL_CALL_START / TOOL_CALL_ARGS /
// TOOL_CALL_END / TOOL_CALL_RESULT / RUN_FINISHED 等事件类型。
//
// 本文件提供：
//   - AGUIEventMapper：将 MockEvent / 内部事件转换为 AG-UI 标准格式
//   - NewAGUIMapper 工厂函数
//
// 触发方式：
//   - Header: X-Agent-Protocol: agui
//   - Query:  ?protocol=agui
//
// SPEC: /workspace/.trae/specs/multi-engine-chat-architecture/ Phase 4
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// AGUIEventMapper 将内部事件格式转换为 AG-UI 标准协议事件。
//
// 使用方式：
//
//	mapper := NewAGUIMapper(w, flusher, sessionID)
//	mapper.MapEvent(mockEvent, stepIdx, evIdx)
type AGUIEventMapper struct {
	w    http.ResponseWriter
	f    http.Flusher
	sess string
}

// NewAGUIMapper 创建一个新的 AG-UI 事件映射器。
//
// 参数：
//   - w: HTTP ResponseWriter（用于写入 SSE 数据）
//   - f: HTTP Flusher（用于立即刷新缓冲区）
//   - sess: 会话 ID（用作 runId / messageId 前缀）
func NewAGUIMapper(w http.ResponseWriter, f http.Flusher, sess string) *AGUIEventMapper {
	return &AGUIEventMapper{w: w, f: f, sess: sess}
}

// MapEvent 根据内部事件类型输出对应的 AG-UI 事件。
//
// 事件映射表：
//
//	| 内部事件类型          | AG-UI 事件类型          |
//	|---------------------|------------------------|
//	| stream_start         | RUN_STARTED            |
//	| text_delta           | TEXT_MESSAGE_CONTENT   |
//	| text_delta_templated | TEXT_MESSAGE_CONTENT   |
//	| tool_call            | TOOL_CALL_START + ARGS |
//	| tool_status (success)| TOOL_CALL_END          |
//	| tool_result          | TOOL_CALL_RESULT       |
//	| stream_end           | RUN_FINISHED           |
func (m *AGUIEventMapper) MapEvent(ev MockEvent, _stepIdx, _evIdx int) {
	switch ev.Type {
	case "stream_start":
		m.sendAGUI("RUN_STARTED", map[string]interface{}{
			"runId": m.sess,
		})

	case "text_delta", "text_delta_templated":
		text, _ := ev.Data["text"].(string)
		if text != "" {
			m.sendAGUI("TEXT_MESSAGE_CONTENT", map[string]interface{}{
				"messageId": fmt.Sprintf("msg_%s", m.sess),
				"delta":     text,
			})
		}

	case "tool_call":
		id, _ := ev.Data["id"].(string)
		name, _ := ev.Data["name"].(string)
		args, _ := ev.Data["args"].(string)

		if id == "" || name == "" {
			return // 必填字段缺失，跳过
		}

		// AG-UI 分两个事件推送：先 START 再 ARGS
		m.sendAGUI("TOOL_CALL_START", map[string]interface{}{
			"toolCallId":   id,
			"toolCallName": name,
		})

		if args != "" {
			m.sendAGUI("TOOL_CALL_ARGS", map[string]interface{}{
				"toolCallId": id,
				"delta":      args,
			})
		}

	case "tool_status":
		// running → 隐含在 TOOL_CALL_START 中（不需要单独事件）
		// success → TOOL_CALL_END
		if status, ok := ev.Data["status"].(string); ok && status == "success" {
			id, _ := ev.Data["id"].(string)
			if id != "" {
				m.sendAGUI("TOOL_CALL_END", map[string]interface{}{
					"toolCallId": id,
				})
			}
		}
		// failed / cancelled 状态也发送 END（带 error 字段）
		if status, ok := ev.Data["status"].(string); ok && (status == "failed" || status == "cancelled") {
			id, _ := ev.Data["id"].(string)
			if id != "" {
				m.sendAGUI("TOOL_CALL_END", map[string]interface{}{
					"toolCallId": id,
					"error":      status,
				})
			}
		}

	case "tool_result":
		id, _ := ev.Data["id"].(string)
		result, _ := ev.Data["result"].(string)
		if id != "" {
			m.sendAGUI("TOOL_CALL_RESULT", map[string]interface{}{
				"toolCallId": id,
				"content":   result,
			})
		}

	case "stream_end":
		m.sendAGUI("RUN_FINISHED", map[string]interface{}{
			"runId": m.sess,
		})

	default:
		// 其他事件类型（stream_status, reasoning_delta 等）不映射到 AG-UI，
		// 静默跳过。未来可扩展支持更多类型。
	}
}

// sendAGUI 发送一个 AG-UI 格式的 SSE 事件。
//
// 格式: event: <eventType>\ndata: <jsonPayload>\n\n
func (m *AGUIEventMapper) sendAGUI(eventType string, payload map[string]interface{}) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return // 序列化失败，静默跳过
	}
	// AG-UI SSE 格式：event 行 + data 行
	fmt.Fprintf(m.w, "event: %s\ndata: %s\n\n", eventType, raw)
	m.f.Flush()
}
