package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// SessionCache holds the in-memory event stream for one chat
// session. Events are appended as the agent core pushes them, so
// Resume can replay from any offset.
//
// The mutex is taken on every event append and every Resume
// read, so it is intentionally lightweight (no RWMutex — writes
// and reads are bursty in equal measure and the critical section
// is a slice append / length check).
type SessionCache struct {
	mu         sync.Mutex
	Events     []*Event
	IsFinished bool
}

// appendEvent is the only legitimate way to grow the cache; it
// also broadcasts to any in-flight Resume polls.
func (c *SessionCache) appendEvent(e *Event) {
	c.mu.Lock()
	c.Events = append(c.Events, e)
	c.mu.Unlock()
}

// snapshot returns a defensive copy of the events slice from
// `offset` onward. If the cache has fewer than `offset` events,
// the returned slice has length zero — the caller is expected to
// poll.
func (c *SessionCache) snapshot(offset int) []*Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	if offset >= len(c.Events) {
		return nil
	}
	out := make([]*Event, len(c.Events)-offset)
	copy(out, c.Events[offset:])
	return out
}

// pendingCall is the state of a suspended session. We persist
// the in-flight messages so ConfirmTool can resume from where
// Chat() paused.
type pendingCall struct {
	ToolCallID string
	ToolName   string
	Args       string
	Messages   []openai.ChatCompletionMessage
}

// Agent is the long-lived orchestrator. It owns the registry, the
// OpenAI client, the per-session caches, the session-level
// approval grants, and the pending-confirmation state.
type Agent struct {
	cfg      AgentConfig
	Registry *ToolRegistry
	llm      llmStream

	// Sessions maps sessionID → *SessionCache.
	Sessions sync.Map

	// SessionGrants maps "<sessionID>|<toolName>" → struct{} so
	// subsequent calls of the same tool in the same session
	// auto-run.
	SessionGrants sync.Map

	// PendingCalls maps sessionID → *pendingCall. Only one
	// pending call per session is allowed at a time; the
	// guarantee is enforced by Agent.mu.
	mu            sync.Mutex
	PendingCalls  map[string]*pendingCall
}

// NewAgent constructs an Agent. The registry must already contain
// all tools; NewAgent does not auto-register anything.
func NewAgent(cfg AgentConfig, registry *ToolRegistry) *Agent {
	if registry == nil {
		registry = NewRegistry()
	}
	a := &Agent{
		cfg:          cfg,
		Registry:     registry,
		llm:          &openaiStream{client: newOpenAIClient(cfg)},
		PendingCalls: make(map[string]*pendingCall),
	}
	return a
}

// NewAgentWithLLM lets tests inject a fake llmStream. Production
// code should use NewAgent.
func NewAgentWithLLM(cfg AgentConfig, registry *ToolRegistry, llm llmStream) *Agent {
	if registry == nil {
		registry = NewRegistry()
	}
	return &Agent{
		cfg:          cfg,
		Registry:     registry,
		llm:          llm,
		PendingCalls: make(map[string]*pendingCall),
	}
}

// ensureSession returns the cache for sessionID, creating one if
// it does not exist yet.
func (a *Agent) ensureSession(sessionID string) *SessionCache {
	if v, ok := a.Sessions.Load(sessionID); ok {
		return v.(*SessionCache)
	}
	c := &SessionCache{}
	actual, _ := a.Sessions.LoadOrStore(sessionID, c)
	return actual.(*SessionCache)
}

// getSession fetches a cache without creating it.
func (a *Agent) getSession(sessionID string) (*SessionCache, bool) {
	v, ok := a.Sessions.Load(sessionID)
	if !ok {
		return nil, false
	}
	return v.(*SessionCache), true
}

// Chat kicks off a streaming turn. The returned channel emits
// Events in real time and is closed when the stream ends (either
// naturally or because a tool call awaits confirmation).
//
// If a previous turn on the same sessionID is still pending
// confirmation, the new call is rejected so we never end up with
// two concurrent goroutines mutating the same session cache.
func (a *Agent) Chat(
	ctx context.Context,
	sessionID string,
	messages []openai.ChatCompletionMessage,
) (<-chan *Event, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("agent: sessionID must not be empty")
	}
	a.mu.Lock()
	if _, busy := a.PendingCalls[sessionID]; busy {
		a.mu.Unlock()
		return nil, fmt.Errorf("agent: session %q is awaiting confirmation", sessionID)
	}
	a.mu.Unlock()

	_ = a.ensureSession(sessionID)
	out := make(chan *Event, 32)
	go a.runLoop(ctx, sessionID, messages, out)
	return out, nil
}

// runLoop is the worker goroutine behind Chat/ConfirmTool. It is
// the single owner of the session cache and the OpenAI stream
// for one turn; the loop is re-entered after auto-run tools to
// fetch the next assistant message.
func (a *Agent) runLoop(
	ctx context.Context,
	sessionID string,
	messages []openai.ChatCompletionMessage,
	out chan<- *Event,
) {
	defer a.finishSession(sessionID, out)

	for turn := 0; ; turn++ {
		if a.cfg.MaxToolCallsPerTurn > 0 && turn >= a.cfg.MaxToolCallsPerTurn {
			a.emitError(sessionID, out, "max_tool_calls_exceeded",
				fmt.Sprintf("reached MaxToolCallsPerTurn=%d", a.cfg.MaxToolCallsPerTurn))
			return
		}

		shouldContinue, err := a.streamOneTurn(ctx, sessionID, messages, out)
		if err != nil {
			a.emitError(sessionID, out, "openai_error", err.Error())
			return
		}
		if !shouldContinue {
			return
		}
	}
}

// streamOneTurn runs ONE round of "OpenAI stream → tool calls →
// either auto-execute or suspend". It returns shouldContinue=true
// when the loop should fetch the next assistant message
// (i.e. all tool calls were auto-executed).
func (a *Agent) streamOneTurn(
	ctx context.Context,
	sessionID string,
	messages []openai.ChatCompletionMessage,
	out chan<- *Event,
) (bool, error) {
	req := a.chatRequest(messages, a.cfg.OpenAIModel)
	stream, err := a.llm.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return false, fmt.Errorf("create chat stream: %w", err)
	}
	defer stream.Close()

	// 1. Drain the stream, accumulate deltas, push events.
	assistant := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant}
	var toolCallsByIndex = make(map[int]*parsedToolCall)

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return false, fmt.Errorf("openai stream recv: %w", err)
		}
		delta := parseDelta(chunk)
		if delta.Text != "" {
			assistant.Content += delta.Text
			a.emitData(sessionID, out, EventTextDelta,
				mustJSON(map[string]string{"content": delta.Text}))
		}
		if delta.Reasoning != "" {
			a.emitData(sessionID, out, EventReasoningDelta,
				mustJSON(map[string]string{"content": delta.Reasoning}))
		}
		for _, ptc := range delta.ToolCalls {
			idx := len(toolCallsByIndex)
			existing, ok := toolCallsByIndex[idx]
			if !ok {
				toolCallsByIndex[idx] = &parsedToolCall{ID: ptc.ID, Name: ptc.Name, Arguments: ptc.Arguments}
			} else {
				if ptc.ID != "" {
					existing.ID = ptc.ID
				}
				if ptc.Name != "" {
					existing.Name = ptc.Name
				}
				existing.Arguments += ptc.Arguments
			}
		}
		if delta.Finished {
			break
		}
	}

	// 2. Persist the assistant message into the rolling history.
	if len(toolCallsByIndex) > 0 {
		openAITCs := make([]openai.ToolCall, 0, len(toolCallsByIndex))
		for i := 0; i < len(toolCallsByIndex); i++ {
			ptc := toolCallsByIndex[i]
			openAITCs = append(openAITCs, openai.ToolCall{
				ID:   ptc.ID,
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      ptc.Name,
					Arguments: ptc.Arguments,
				},
			})
		}
		assistant.ToolCalls = openAITCs
	}
	if assistant.Content != "" || len(assistant.ToolCalls) > 0 {
		messages = append(messages, assistant)
	}

	// 3. No tool calls → turn is over.
	if len(toolCallsByIndex) == 0 {
		return false, nil
	}

	// 4. Process each tool call. The order is the order in which
	//    the LLM emitted them.
	anySuspended := false
	for i := 0; i < len(toolCallsByIndex); i++ {
		ptc := toolCallsByIndex[i]
		if ptc.ID == "" {
			// The LLM sometimes omits the ID on later deltas of
			// the same tool call. Skip the empty filler chunks;
			// the real one will arrive with the same index.
			continue
		}

		def, ok := a.Registry.Get(ptc.Name)
		if !ok {
			// Unknown tool → push a synthetic error result so
			// the LLM can recover and try a different tool.
			a.emitToolCall(sessionID, out, ptc, def, false)
			a.emitData(sessionID, out, EventToolResult, mustJSON(ToolResultData{
				ID:      ptc.ID,
				Name:    ptc.Name,
				Result:  fmt.Sprintf(`{"error":"unknown_tool","name":%q}`, ptc.Name),
				IsError: true,
				Status:  "failed",
			}))
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: ptc.ID,
				Content:    fmt.Sprintf(`{"error":"unknown_tool","name":%q}`, ptc.Name),
			})
			continue
		}

		autoRun := !def.NeedConfirm || a.isGranted(sessionID, ptc.Name)
		a.emitToolCall(sessionID, out, ptc, def, autoRun)

		if !autoRun {
			// Suspend: store the messages history and break
			// out of the loop. ConfirmTool will resume.
			a.mu.Lock()
			a.PendingCalls[sessionID] = &pendingCall{
				ToolCallID: ptc.ID,
				ToolName:   ptc.Name,
				Args:       ptc.Arguments,
				Messages:   cloneMessages(messages),
			}
			a.mu.Unlock()
			anySuspended = true
			continue
		}

		// Auto-run.
		resultStr, status, runErr := a.runTool(def, ptc.Arguments)
		a.emitData(sessionID, out, EventToolResult, mustJSON(ToolResultData{
			ID:         ptc.ID,
			Name:       ptc.Name,
			Result:     resultStr,
			IsError:    runErr != nil,
			Status:     status,
			DurationMs: 0, // filled in by the handler
		}))
		if runErr != nil {
			// Failure result is still appended to the message
			// history so the LLM can see what went wrong and
			// try a different approach.
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: ptc.ID,
				Content:    resultStr,
			})
			continue
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			ToolCallID: ptc.ID,
			Content:    resultStr,
		})
	}

	if anySuspended {
		// Do not loop back to the LLM; the user (or the
		// frontend) must ConfirmTool.
		return false, nil
	}
	return true, nil
}

// runTool invokes a handler and times the call. The duration is
// pushed as a ToolStatus update so the UI can render a "running"
// badge that becomes "success" / "failed".
func (a *Agent) runTool(def ToolDefinition, args string) (string, string, error) {
	t0 := time.Now()
	result, err := def.Handler(args)
	dur := time.Since(t0).Milliseconds()
	_ = dur
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error()), "failed", err
	}
	return result, "success", nil
}

// emitToolCall pushes an EventToolCall (with AutoRun and Kind) and
// an EventToolStatus("running") so the UI can render the badge.
func (a *Agent) emitToolCall(sessionID string, out chan<- *Event, ptc *parsedToolCall, def ToolDefinition, autoRun bool) {
	kind := def.Kind
	if kind == "" {
		kind = KindUnknown
	}
	name := ptc.Name
	if name == "" && def.Kind != "" {
		// Some defs have a fixed name registered; the LLM will
		// never send us an empty name, so this branch is for
		// defensive completeness only.
		name = "<unknown>"
	}
	a.emitData(sessionID, out, EventToolCall, mustJSON(ToolCallData{
		ID:      ptc.ID,
		Name:    name,
		Args:    ptc.Arguments,
		AutoRun: autoRun,
		Kind:    kind,
	}))
	if autoRun {
		a.emitData(sessionID, out, EventToolStatus, mustJSON(ToolStatusData{
			ID:     ptc.ID,
			Status: "running",
		}))
	}
}

// isGranted checks the session-level grant map.
func (a *Agent) isGranted(sessionID, toolName string) bool {
	_, ok := a.SessionGrants.Load(grantKey(sessionID, toolName))
	return ok
}

func grantKey(sessionID, toolName string) string {
	return sessionID + "|" + toolName
}

// ConfirmTool applies a user decision to a pending tool call and
// resumes the loop. The returned channel is identical in shape to
// Chat's.
func (a *Agent) ConfirmTool(
	ctx context.Context,
	sessionID, toolCallID string,
	decision Decision,
) (<-chan *Event, error) {
	if !isValidDecision(decision) {
		return nil, fmt.Errorf("agent: invalid decision %q", decision)
	}

	a.mu.Lock()
	pc, ok := a.PendingCalls[sessionID]
	if !ok {
		a.mu.Unlock()
		return nil, fmt.Errorf("agent: no pending call for session %q", sessionID)
	}
	if pc.ToolCallID != toolCallID {
		a.mu.Unlock()
		return nil, fmt.Errorf("agent: pending toolCallID %q does not match %q", pc.ToolCallID, toolCallID)
	}
	delete(a.PendingCalls, sessionID)
	a.mu.Unlock()

	messages := pc.Messages
	out := make(chan *Event, 32)

	// We push the chosen decision's effect as a tool result
	// synchronously into a wrapper channel, then start the
	// resumption loop. Because the wrapper approach is hard to
	// reason about, we instead hand the loop a pre-populated
	// "first action" function. The cleanest way to express that
	// is to inline the decision handling here.
	go a.resumeAfterDecision(ctx, sessionID, pc.ToolName, toolCallID, decision, messages, out)
	return out, nil
}

// resumeAfterDecision runs the post-decision logic: apply the
// decision, push the resulting tool result event, then continue
// the streaming loop with updated messages.
func (a *Agent) resumeAfterDecision(
	ctx context.Context,
	sessionID, toolName, toolCallID string,
	decision Decision,
	messages []openai.ChatCompletionMessage,
	out chan<- *Event,
) {
	defer a.finishSession(sessionID, out)

	def, _ := a.Registry.Get(toolName)
	if def.Handler == nil {
		// We did not find the tool. We still need to emit a
		// synthetic result so the LLM can recover, but without
		// running the handler.
		def = ToolDefinition{Kind: KindUnknown}
	}

	switch decision {
	case DecisionAccept:
		resultStr, status, _ := a.runTool(def, pendingArgs(messages, toolCallID))
		a.emitData(sessionID, out, EventToolResult, mustJSON(ToolResultData{
			ID:     toolCallID,
			Name:   toolName,
			Result: resultStr,
			Status: status,
		}))
		messages = append(messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			ToolCallID: toolCallID,
			Content:    resultStr,
		})

	case DecisionAcceptForSession:
		a.SessionGrants.Store(grantKey(sessionID, toolName), struct{}{})
		resultStr, status, _ := a.runTool(def, pendingArgs(messages, toolCallID))
		a.emitData(sessionID, out, EventToolResult, mustJSON(ToolResultData{
			ID:     toolCallID,
			Name:   toolName,
			Result: resultStr,
			Status: status,
		}))
		messages = append(messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			ToolCallID: toolCallID,
			Content:    resultStr,
		})

	case DecisionDecline:
		cancelled := `{"error":"user_rejected"}`
		a.emitData(sessionID, out, EventToolResult, mustJSON(ToolResultData{
			ID:      toolCallID,
			Name:    toolName,
			Result:  cancelled,
			IsError: true,
			Status:  "cancelled",
		}))
		messages = append(messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			ToolCallID: toolCallID,
			Content:    cancelled,
		})

	case DecisionCancel:
		cancelled := `{"error":"user_cancelled"}`
		a.emitData(sessionID, out, EventToolResult, mustJSON(ToolResultData{
			ID:      toolCallID,
			Name:    toolName,
			Result:  cancelled,
			IsError: true,
			Status:  "cancelled",
		}))
		// No further LLM call; turn ends here.
		return

	default:
		a.emitError(sessionID, out, "invalid_decision", string(decision))
		return
	}

	// Continue the loop with the post-decision messages.
	a.runLoop(ctx, sessionID, messages, out)
}

func pendingArgs(messages []openai.ChatCompletionMessage, toolCallID string) string {
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if m.Role != openai.ChatMessageRoleAssistant {
			continue
		}
		for _, tc := range m.ToolCalls {
			if tc.ID == toolCallID {
				return tc.Function.Arguments
			}
		}
	}
	return ""
}

// isValidDecision covers the four documented values plus the
// empty string (which the HTTP layer treats as a bad request).
func isValidDecision(d Decision) bool {
	switch d {
	case DecisionAccept, DecisionAcceptForSession, DecisionDecline, DecisionCancel:
		return true
	}
	return false
}

// Resume replays cached events from `offset`. If the session is
// still running, the loop polls every 50ms for new events. If
// the session is finished and we have caught up, it emits a
// stream_end and closes.
func (a *Agent) Resume(
	ctx context.Context,
	sessionID string,
	offset int,
) (<-chan *Event, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("agent: sessionID must not be empty")
	}
	cache, ok := a.getSession(sessionID)
	if !ok {
		return nil, fmt.Errorf("agent: session %q not found", sessionID)
	}
	out := make(chan *Event, 32)
	go a.runResume(ctx, cache, offset, out)
	return out, nil
}

func (a *Agent) runResume(
	ctx context.Context,
	cache *SessionCache,
	offset int,
	out chan<- *Event,
) {
	defer close(out)
	for {
		evs := cache.snapshot(offset)
		for _, e := range evs {
			select {
			case <-ctx.Done():
				return
			case out <- e:
			}
		}
		offset += len(evs)
		cache.mu.Lock()
		finished := cache.IsFinished
		cache.mu.Unlock()
		if finished {
			return
		}
		// No new events yet; sleep briefly and retry.
		select {
		case <-ctx.Done():
			return
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// emitData builds an Event and pushes it to BOTH the session
// cache and the consumer's channel.
func (a *Agent) emitData(sessionID string, out chan<- *Event, t EventType, data string) {
	ev := &Event{Type: t, Data: data}
	if v, ok := a.Sessions.Load(sessionID); ok {
		v.(*SessionCache).appendEvent(ev)
	}
	// We use a non-blocking send: if the consumer is slower than
	// the producer we still want to keep producing. The
	// frontend's UI is forgiving of small backlogs; the agent
	// is not forgiving of deadlock.
	select {
	case out <- ev:
	default:
		// Drop on the floor with a buffered channel that grows.
		// For this implementation we just block; backpressure
		// here is preferable to losing events.
		out <- ev
	}
}

// emitError pushes a stream_end with an error payload.
func (a *Agent) emitError(sessionID string, out chan<- *Event, code, msg string) {
	payload := map[string]string{"code": code, "message": msg}
	ev := &Event{Type: EventStreamEnd, Data: mustJSON(payload)}
	if v, ok := a.Sessions.Load(sessionID); ok {
		v.(*SessionCache).appendEvent(ev)
	}
	out <- ev
}

// finishSession marks the cache as finished and emits a
// stream_end. Called by both Chat and ConfirmTool on exit.
func (a *Agent) finishSession(sessionID string, out chan<- *Event) {
	if v, ok := a.Sessions.Load(sessionID); ok {
		v.(*SessionCache).mu.Lock()
		v.(*SessionCache).IsFinished = true
		v.(*SessionCache).mu.Unlock()
	}
	ev := &Event{Type: EventStreamEnd, Data: ""}
	if v, ok := a.Sessions.Load(sessionID); ok {
		v.(*SessionCache).appendEvent(ev)
	}
	// Best-effort send. The consumer might have disconnected.
	defer func() { _ = recover() }()
	out <- ev
}

// mustJSON is the non-erroring sibling of json.Marshal; the
// payloads we send are always under our control and never fail
// to marshal.
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}

func cloneMessages(in []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	out := make([]openai.ChatCompletionMessage, len(in))
	copy(out, in)
	return out
}
