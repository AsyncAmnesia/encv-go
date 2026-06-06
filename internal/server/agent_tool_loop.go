// internal/server/agent_tool_loop.go
//
// 工具调用循环 + 确认/拒绝/递归调 OpenAI 的实现。
//
// 状态机：
//   - 客户端 → POST /api/chat {messages, sessionId}
//     → 后端代理到 OpenAI streaming →
//       ① 累积 content → text_delta
//       ② 累积 tool_calls（按 id 聚合 index/function.name/args）
//       ③ 若 finish_reason == "tool_calls"：
//          - 推 tool_call 事件（每个 tool_call 一个事件）
//          - 推 stream_end
//          - 把 messages + pendingToolCalls 缓存到 session 状态
//       ④ 否则 finish_reason == "stop"：
//          - 推 stream_end
//
//   - 客户端 → POST /api/confirm {sessionId, toolCallId, decision}
//     - decision=accept  → 取出 pending tool_call → executePluginTool → 把 result 追加到 messages → 递归 chat
//     - decision=decline → 追加 cancelled tool_result 到 messages → 递归 chat
//     - decision=cancel  → 推 stream_end（不递归）
//
//   - 客户端 → POST /api/resume {sessionId, offset}
//     - 简化版：当前后端无事件缓存（bufio.Scanner 流式）→ 返回 "resume_not_implemented" 错误。
//       完整断点续传需要重构 chat 为事件驱动（带 ID），后续 phase 处理。
//
// 会话存储：sync.Map[sessionId]*agentSession + RWMutex
package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// toolCallAccumulator 累积 OpenAI 流式 tool_calls。
// OpenAI 工具调用在多个 chunk 中分片到达，必须按 (id, index) 聚合。
type toolCallAccumulator struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Index     int    `json:"index"`
	Function  struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// agentSession 保存单次对话的服务端状态
type agentSession struct {
	mu              sync.Mutex
	SessionID       string
	Messages        []chatMsg
	PendingTools    []toolCallAccumulator // 当前 LLM 决策的工具调用（待 confirm）
	LastModel       string
	LastTemperature float64
}

var (
	sessionMu sync.RWMutex
	sessions  = make(map[string]*agentSession)
)

// getOrCreateSession 获取或创建 session
func getOrCreateSession(id string) *agentSession {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if s, ok := sessions[id]; ok {
		return s
	}
	s := &agentSession{SessionID: id}
	sessions[id] = s
	return s
}

// updateSession 原子更新 session 内容
func updateSession(id string, fn func(*agentSession)) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if s, ok := sessions[id]; ok {
		fn(s)
	}
}

// ─── 工具调用执行 + 递归 ───────────────────────────────────────

// executeAndRecurse 执行用户确认的工具调用，追加 tool_result 到 messages，
// 然后递归调 OpenAI 下一轮（带 tool_choice="none" 强制 LLM 继续生成）。
//
// 返回：
//   - tool_result（已执行）
//   - 新的 messages（已追加 tool_result）
func executeAndRecurse(ctx context.Context, sess *agentSession, agentCfg agentConfig, tool toolCallAccumulator) (chatMsg, error) {
	var argsObj map[string]interface{}
	_ = json.Unmarshal([]byte(tool.Function.Arguments), &argsObj)

	var (
		resultStr string
	)

	start := time.Now()
	raw, execErr := executePluginTool(ctx, tool.Function.Name, tool.Function.Arguments)
	_ = time.Since(start).Milliseconds() // 预留 durationMs 字段供后续 metrics
	if execErr != nil {
		resultStr = fmt.Sprintf(`{"error":"internal","message":%q}`, execErr.Error())
	} else {
		resultStr = raw
		// 检测插件执行结果中的 error 字段
		var probe struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal([]byte(raw), &probe)
	}

	// 构造 tool 消息（OpenAI 协议要求 role="tool" + tool_call_id 引用）
	toolMsg := chatMsg{
		Role:       "tool",
		Content:    resultStr,
		ToolCallID: tool.ID,
		Name:       tool.Function.Name,
	}

	return toolMsg, nil
}

// callOpenAIChat 同步调一次 OpenAI Chat API（非流式）。用于工具结果注入后的递归。
// 返回 LLM 完整回复（一段 assistant 文本）或 tool_calls。
type openaiChatResponse struct {
	Choices []struct {
		Message struct {
			Role      string                 `json:"role"`
			Content   string                 `json:"content"`
			ToolCalls []toolCallAccumulator  `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// callOpenAIChatOnce 同步调一次 OpenAI（非流式）。返回 LLM 响应。
func callOpenAIChatOnce(ctx context.Context, cfg agentConfig, model string, temperature float64, messages []chatMsg) (*openaiChatResponse, error) {
	reqBody := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": temperature,
		"stream":      false,
	}
	reqJSON, _ := json.Marshal(reqBody)

	reqURL := strings.TrimRight(cfg.BaseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(reqJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return &openaiChatResponse{Error: &struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		}{Message: string(body)}}, nil
	}

	body, _ := io.ReadAll(resp.Body)
	var out openaiChatResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode openai response: %w", err)
	}
	return &out, nil
}

// callOpenAIStream 调一次 OpenAI 流式，返回事件 channel。
// 用于在 confirm 后递归流式输出 LLM 继续生成的回复。
func callOpenAIStream(ctx context.Context, cfg agentConfig, model string, temperature float64, messages []chatMsg) (<-chan openaiStreamEvent, error) {
	reqBody := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": temperature,
		"stream":      true,
	}
	reqJSON, _ := json.Marshal(reqBody)

	reqURL := strings.TrimRight(cfg.BaseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(reqJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("upstream HTTP %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan openaiStreamEvent, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			dataStr := strings.TrimPrefix(line, "data: ")
			if dataStr == "[DONE]" {
				ch <- openaiStreamEvent{Type: "stream_end"}
				return
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content           string                `json:"content"`
						ToolCalls         []toolCallAccumulator `json:"tool_calls"`
						ReasoningContent  string                `json:"reasoning_content"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
				continue
			}
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					ch <- openaiStreamEvent{Type: "text_delta", Data: choice.Delta.Content}
				}
				if choice.Delta.ReasoningContent != "" {
					ch <- openaiStreamEvent{Type: "reasoning_delta", Data: choice.Delta.ReasoningContent}
				}
				if len(choice.Delta.ToolCalls) > 0 {
					for _, tc := range choice.Delta.ToolCalls {
						ch <- openaiStreamEvent{Type: "tool_call_chunk", Data: tc}
					}
				}
				if choice.FinishReason != "" {
					ch <- openaiStreamEvent{Type: "finish_reason", Data: choice.FinishReason}
				}
			}
		}
	}()

	return ch, nil
}

type openaiStreamEvent struct {
	Type string
	Data interface{}
}

// streamChat 写 SSE 事件到 client —— 用于 /api/confirm 递归时
func (s *Server) streamChat(ctx context.Context, c *gin.Context, cfg agentConfig, model string, temperature float64, messages []chatMsg, sess *agentSession) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return
	}
	s.setSSEHeaders(c.Writer)

	ch, err := callOpenAIStream(ctx, cfg, model, temperature, messages)
	if err != nil {
		slog.Warn("agent: stream error", "error", err)
		s.sendSSEEventSafe(c.Writer, flusher, "stream_error", err.Error())
		s.sendSSEEventSafe(c.Writer, flusher, "stream_end", "")
		return
	}

	// 累积 tool_calls（如果有的话）
	var pendingTools []toolCallAccumulator
	accumulator := make(map[int]*toolCallAccumulator)

	for ev := range ch {
		switch ev.Type {
		case "text_delta":
			s.sendSSEEventSafe(c.Writer, flusher, "text_delta", ev.Data)
		case "reasoning_delta":
			s.sendSSEEventSafe(c.Writer, flusher, "reasoning_delta", ev.Data)
		case "tool_call_chunk":
			tc := ev.Data.(toolCallAccumulator)
			cur, ok := accumulator[tc.Index]
			if !ok {
				cur = &toolCallAccumulator{Index: tc.Index, ID: tc.ID, Type: tc.Type}
				accumulator[tc.Index] = cur
			}
			if tc.ID != "" {
				cur.ID = tc.ID
			}
			if tc.Type != "" {
				cur.Type = tc.Type
			}
			if tc.Function.Name != "" {
				cur.Function.Name += tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				cur.Function.Arguments += tc.Function.Arguments
			}
		case "finish_reason":
			// 收集所有累积的 tool_calls
			for _, tc := range accumulator {
				pendingTools = append(pendingTools, *tc)
			}
			if len(pendingTools) > 0 {
				// 推 tool_call 事件
				for _, tc := range pendingTools {
					payload := map[string]interface{}{
						"id":           tc.ID,
						"name":         tc.Function.Name,
						"args":         tc.Function.Arguments,
						"auto_run":     false,
						"needsConfirm": true,
						"kind":         "fileChange",
					}
					s.sendSSEEventSafe(c.Writer, flusher, "tool_call", payload)
				}
				// 缓存到 session
				sess.mu.Lock()
				sess.PendingTools = pendingTools
				sess.mu.Unlock()
			}
		case "stream_end":
			s.sendSSEEventSafe(c.Writer, flusher, "stream_end", "")
		}
	}
}
