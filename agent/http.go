package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// ChatRequest is the JSON payload accepted by /api/chat.
//
// Messages are accepted as []map[string]any to keep the wire
// format decoupled from the openai library's exact Go type. The
// handler converts each entry into an openai.ChatCompletionMessage
// before calling Agent.Chat. A future migration to another LLM
// SDK is then a local change to this file.
type ChatRequest struct {
	SessionID string           `json:"session_id"`
	Messages  []map[string]any `json:"messages"`
	// Mode selects the send mode for the message:
	//   - "" or "start": regular new turn (default behaviour,
	//     equivalent to a fresh Chat call).
	//   - "steer":       the message is meant to steer the
	//     current in-flight turn; the agent uses the supplied
	//     messages as the new running state. Steer preserves
	//     the existing confirm flow (if a tool call is
	//     produced the ApprovalCard is still shown).
	//   - "queue":       the message is held until the current
	//     turn fully ends. The agent stores it in
	//     agent.PendingMessages[sessionID] and a
	//     HookTurnEnd listener starts a new Chat after the
	//     current runLoop returns. Queue returns immediately
	//     (the HTTP response is closed) because the events
	//     will surface through the existing SSE / Resume
	//     connection once the queued Chat runs.
	Mode string `json:"mode,omitempty"`
	// SelectedSkills is the optional list of skill names the
	// front-end wants to activate for this session. When
	// non-empty, the default session_start hook registered
	// by NewAgent will append the corresponding skill
	// prompts to the session's per-session system prompt
	// override so the LLM sees them on the first turn.
	SelectedSkills []string `json:"selected_skills,omitempty"`
}

// ResumeRequest is the JSON payload accepted by /api/resume.
// Offset is the index into the cached event list; the handler
// returns events from that offset onward.
type ResumeRequest struct {
	SessionID string `json:"session_id"`
	Offset    int    `json:"offset"`
}

// ConfirmRequest is the JSON payload accepted by /api/confirm.
// Decision is one of the four Decision constants.
type ConfirmRequest struct {
	SessionID  string `json:"session_id"`
	ToolCallID string `json:"tool_call_id"`
	Decision   string `json:"decision"`
}

// HandleChat is the HTTP entry point for /api/chat. It parses
// the request, calls Agent.Chat, and streams the resulting
// events as Server-Sent Events (SSE).
//
// SSE format:
//   data: {"type":"text_delta","data":"..."}\n\n
//   data: {"type":"stream_end","data":""}\n\n
//
// The response Content-Type is "text/event-stream".
func (a *Agent) HandleChat(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.SessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}
	msgs, err := convertMessages(req.Messages)
	if err != nil {
		http.Error(w, "invalid messages: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Register the per-session skill selection BEFORE Chat
	// so the session_start hook (which runs inside the
	// Chat's runLoop goroutine) can read it from
	// a.selectedSkillsFor. The happens-before guarantee of
	// sync.Map.Store makes the SetSelectedSkills call
	// visible to the runLoop goroutine without any
	// additional synchronisation.
	a.SetSelectedSkills(req.SessionID, req.SelectedSkills)

	ch, err := a.ChatMode(r.Context(), req.SessionID, msgs, req.Mode)
	if err != nil {
		http.Error(w, "chat error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Queue mode stores the message and returns a channel
	// that is already closed — the actual Chat happens
	// later, on the next HookTurnEnd. The HTTP response is
	// therefore 202 Accepted with an empty body so the
	// front-end knows the message was buffered rather than
	// streamed.
	if req.Mode == "queue" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	streamSSE(w, r, ch)
}

// HandleResume is the HTTP entry point for /api/resume. The
// front-end calls it whenever the SSE connection drops and the
// user wants to continue receiving the rest of an in-flight
// stream. Resume replays cached events from the supplied offset
// and waits for the session to finish if more events are
// pending.
func (a *Agent) HandleResume(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var req ResumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.SessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}
	ch, err := a.Resume(r.Context(), req.SessionID, req.Offset)
	if err != nil {
		http.Error(w, "resume error: "+err.Error(), http.StatusNotFound)
		return
	}
	streamSSE(w, r, ch)
}

// HandleConfirm is the HTTP entry point for /api/confirm. The
// front-end dispatches the user's decision (one of accept /
// accept_for_session / decline / cancel) here, and the agent
// resumes the stream accordingly.
func (a *Agent) HandleConfirm(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var req ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.SessionID == "" || req.ToolCallID == "" || req.Decision == "" {
		http.Error(w, "session_id, tool_call_id, and decision are all required", http.StatusBadRequest)
		return
	}
	decision := Decision(req.Decision)
	if !isValidDecision(decision) {
		http.Error(w, "invalid decision: "+req.Decision, http.StatusBadRequest)
		return
	}
	ch, err := a.ConfirmTool(r.Context(), req.SessionID, req.ToolCallID, decision)
	if err != nil {
		http.Error(w, "confirm error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	streamSSE(w, r, ch)
}

// requirePOST is a small helper that enforces the POST method
// and writes a 405 if anything else is used. Returning a bool
// lets the caller early-return without further boilerplate.
func requirePOST(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// streamSSE is the shared SSE writer used by HandleChat,
// HandleResume, and HandleConfirm. It also installs a short
// keep-alive ticker so intermediate proxies do not close idle
// connections.
func streamSSE(w http.ResponseWriter, r *http.Request, ch <-chan *Event) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported by this server", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			_, _ = fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if err := writeSSE(w, ev); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// writeSSE serialises one Event to the SSE wire format. The
// public API is a single function (rather than inline format
// strings sprinkled through the handlers) so the test suite
// can assert on its output without exercising the whole
// streaming pipeline.
func writeSSE(w http.ResponseWriter, ev *Event) error {
	if ev == nil {
		return nil
	}
	payload := map[string]any{
		"type": ev.Type,
		"data": ev.Data,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", raw)
	return err
}

// convertMessages walks the wire-shape []map[string]any and
// emits a slice of openai.ChatCompletionMessage. We accept any
// map shape (including additional fields) but only honour a
// documented subset:
//
//	role:    "user" | "assistant" | "system" | "tool"
//	content: string | []map[string]any
//	name:    string  (for tool messages)
//	tool_call_id: string  (for tool messages)
//	tool_calls: []map[string]any  (assistant → tool invocation)
//
// Unknown fields are silently dropped, which keeps the
// handler compatible with future LLM SDKs that add new
// message fields.
func convertMessages(in []map[string]any) ([]openai.ChatCompletionMessage, error) {
	out := make([]openai.ChatCompletionMessage, 0, len(in))
	for i, m := range in {
		role, _ := m["role"].(string)
		if role == "" {
			return nil, fmt.Errorf("messages[%d]: missing role", i)
		}
		msg := openai.ChatCompletionMessage{Role: role}

		// Content can be a plain string or a structured
		// list (OpenAI's vision / multimodal format). We
		// stringify the structured form back into a JSON
		// blob — the openai library's parser tolerates that.
		if v, ok := m["content"]; ok {
			switch t := v.(type) {
			case string:
				msg.Content = t
			default:
				b, err := json.Marshal(t)
				if err != nil {
					return nil, fmt.Errorf("messages[%d].content: marshal: %w", i, err)
				}
				msg.Content = string(b)
			}
		}
		if name, ok := m["name"].(string); ok {
			msg.Name = name
		}
		if toolID, ok := m["tool_call_id"].(string); ok {
			msg.ToolCallID = toolID
		}

		// tool_calls is an assistant-only field. We convert
		// each entry into openai.ToolCall.
		if tcs, ok := m["tool_calls"].([]any); ok {
			calls := make([]openai.ToolCall, 0, len(tcs))
			for j, raw := range tcs {
				tc, ok := raw.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("messages[%d].tool_calls[%d]: not an object", i, j)
				}
				call := openai.ToolCall{Type: openai.ToolTypeFunction}
				if id, ok := tc["id"].(string); ok {
					call.ID = id
				}
				if fn, ok := tc["function"].(map[string]any); ok {
					fc := openai.FunctionCall{}
					if name, ok := fn["name"].(string); ok {
						fc.Name = name
					}
					if args, ok := fn["arguments"].(string); ok {
						fc.Arguments = args
					}
					call.Function = fc
				}
				calls = append(calls, call)
			}
			msg.ToolCalls = calls
		}

		out = append(out, msg)
	}
	return out, nil
}

// HandleHealth is a minimal liveness probe. It is intentionally
// separate from the SSE endpoints so an external watcher can
// poll it cheaply.
//
// The response carries the agent's process-wide serverInstanceId
// (see Agent.ServerInstanceId) so the front-end can detect
// server restarts and discard any stale SSE sequence tracking
// state from the previous instance.
func (a *Agent) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":                true,
		"ts":                time.Now().UnixMilli(),
		"openai_ok":         a.cfg.OpenAIAPIKey != "",
		"serverInstanceId": a.serverInstanceId,
	})
}

// SetRouteTimeout is a convenience wrapper that wraps each
// handler in an http.TimeoutHandler. Callers can use it from
// cmd/agent-demo/main.go if they want an extra layer of
// protection against long-running handlers, but the default
// behaviour is no timeout (the agent stream is intentionally
// long-lived).
func (a *Agent) SetRouteTimeout(h http.HandlerFunc, d time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), d)
		defer cancel()
		h(w, r.WithContext(ctx))
	}
}

// WriteJSONError is a small helper used by error paths in the
// HTTP handlers. It always emits a consistent JSON shape so
// the front-end can rely on `{"error": "..."}`.
func WriteJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": strings.TrimSpace(msg),
	})
}
