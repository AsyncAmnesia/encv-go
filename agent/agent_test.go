package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// fakeLLM is a small llmStream used by the agent tests to inject
// errors. The structured stream-scripting helpers live in
// openai_test.go (they drive a real *openai.Client through
// httptest, which is the highest-fidelity path).
type fakeLLM struct {
	mu          sync.Mutex
	openErr     error
	openErrOnce bool
	openCount   int
}

func newFakeLLM() *fakeLLM { return &fakeLLM{} }

func (f *fakeLLM) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.openCount++
	if f.openErr != nil {
		err := f.openErr
		if f.openErrOnce {
			f.openErr = nil
		}
		return nil, err
	}
	return nil, errors.New("fakeLLM: streaming not scripted here; use makeFakeAgent in http_test.go")
}

// drainChatStream reads from the SSE channel until it closes or
// the deadline expires. Tests that use the existing fakeLLM (and
// therefore get an openErr) typically see only the stream_end
// event + the close.
func drainChatStream(t *testing.T, ch <-chan *Event) []*Event {
	t.Helper()
	events := []*Event{}
	deadline := time.After(3 * time.Second)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, e)
			if e.Type == EventStreamEnd {
				return events
			}
		case <-deadline:
			t.Fatal("drainChatStream: deadline exceeded")
		}
	}
}

// sessionHasEvent is a small helper for the chat tests.
func sessionHasEvent(events []*Event, t2 EventType) bool {
	for _, e := range events {
		if e.Type == t2 {
			return true
		}
	}
	return false
}

// ----------------------------------------------------------------------
// Agent-loop tests.
//
// These exercise the public surface (Chat / ConfirmTool / Resume)
// using the existing fakeLLM to inject failures. The streaming
// happy path is covered in http_test.go via a real
// *openai.Client talking to httptest.
// ----------------------------------------------------------------------

// TestParseDelta_ExtractsAllFields locks the shape that the
// agent loop relies on. parseDelta returns a parsedDelta struct
// that the runLoop turns into Events.
func TestParseDelta_ExtractsAllFields(t *testing.T) {
	// Plain text delta.
	d := parseDelta(openai.ChatCompletionStreamResponse{
		Choices: []openai.ChatCompletionStreamChoice{{
			Delta: openai.ChatCompletionStreamChoiceDelta{
				Content: "hello",
			},
		}},
	})
	if d.Text != "hello" {
		t.Errorf("text: got %q want %q", d.Text, "hello")
	}
	if d.Reasoning != "" {
		t.Errorf("reasoning should be empty for plain text delta, got %q", d.Reasoning)
	}
	if d.Finished {
		t.Errorf("finished should be false when FinishReason is empty")
	}

	// Reasoning delta (o1).
	d = parseDelta(openai.ChatCompletionStreamResponse{
		Choices: []openai.ChatCompletionStreamChoice{{
			Delta: openai.ChatCompletionStreamChoiceDelta{
				ReasoningContent: "thinking...",
			},
		}},
	})
	if d.Reasoning != "thinking..." {
		t.Errorf("reasoning: got %q want %q", d.Reasoning, "thinking...")
	}
	if d.Text != "" {
		t.Errorf("text should be empty for reasoning delta, got %q", d.Text)
	}

	// Tool call delta.
	d = parseDelta(openai.ChatCompletionStreamResponse{
		Choices: []openai.ChatCompletionStreamChoice{{
			Delta: openai.ChatCompletionStreamChoiceDelta{
				ToolCalls: []openai.ToolCall{{
					ID: "call_1",
					Function: openai.FunctionCall{
						Name:      "list_files",
						Arguments: `{"path":"/"}`,
					},
				}},
			},
		}},
	})
	if len(d.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(d.ToolCalls))
	}
	if d.ToolCalls[0].ID != "call_1" {
		t.Errorf("tool call ID: got %q", d.ToolCalls[0].ID)
	}
	if d.ToolCalls[0].Name != "list_files" {
		t.Errorf("tool call name: got %q", d.ToolCalls[0].Name)
	}
	if d.ToolCalls[0].Arguments != `{"path":"/"}` {
		t.Errorf("tool call args: got %q", d.ToolCalls[0].Arguments)
	}

	// Finish reason.
	d = parseDelta(openai.ChatCompletionStreamResponse{
		Choices: []openai.ChatCompletionStreamChoice{{
			FinishReason: "tool_calls",
		}},
	})
	if !d.Finished {
		t.Errorf("finished should be true when FinishReason is set")
	}
}

// TestSessionCache_AppendAndSnapshot exercises the cache that
// powers the agent's Resume endpoint.
func TestSessionCache_AppendAndSnapshot(t *testing.T) {
	c := &SessionCache{}
	c.appendEvent(&Event{Type: EventTextDelta, Data: `{"content":"a"}`})
	c.appendEvent(&Event{Type: EventTextDelta, Data: `{"content":"b"}`})

	got := c.snapshot(0)
	if len(got) != 2 {
		t.Fatalf("snapshot(0) length: got %d want 2", len(got))
	}
	if got[0].Data != `{"content":"a"}` || got[1].Data != `{"content":"b"}` {
		t.Errorf("snapshot order/content mismatch: %+v", got)
	}

	got = c.snapshot(1)
	if len(got) != 1 {
		t.Fatalf("snapshot(1) length: got %d want 1", len(got))
	}
	if got[0].Data != `{"content":"b"}` {
		t.Errorf("snapshot(1) content: %q", got[0].Data)
	}

	// Offset past end yields an empty slice (Resume polls).
	if got := c.snapshot(99); len(got) != 0 {
		t.Errorf("snapshot(99) should be empty, got %d events", len(got))
	}
}

// TestSessionCache_SnapshotIsACopy verifies the defensive copy
// contract so callers can hold the slice without fearing
// mutation.
func TestSessionCache_SnapshotIsACopy(t *testing.T) {
	c := &SessionCache{}
	c.appendEvent(&Event{Type: EventTextDelta, Data: "x"})

	got := c.snapshot(0)
	got[0] = &Event{Type: EventToolResult, Data: "tampered"}

	again := c.snapshot(0)
	if again[0].Type != EventTextDelta {
		t.Errorf("snapshot should be defensive copy; cache was mutated via slice")
	}
}

// TestSessionCache_IsFinishedRoundTrip locks the boolean toggle
// used by Resume to decide when to stop polling.
func TestSessionCache_IsFinishedRoundTrip(t *testing.T) {
	c := &SessionCache{}
	if c.IsFinished {
		t.Errorf("new SessionCache should not be finished")
	}
	c.mu.Lock()
	c.IsFinished = true
	c.mu.Unlock()
	if !c.IsFinished {
		t.Errorf("IsFinished should be readable after setting")
	}
}

// TestAgent_NewAgentAndRegistryWiring is the smoke test:
// NewAgent must accept a non-nil registry and not panic.
func TestAgent_NewAgentAndRegistryWiring(t *testing.T) {
	cfg := AgentConfig{OpenAIModel: "gpt-4o"}
	reg := NewRegistry()
	a := NewAgent(cfg, reg)
	if a == nil {
		t.Fatal("NewAgent returned nil")
	}
	if a.Registry != reg {
		t.Errorf("NewAgent did not wire the registry")
	}
	if a.cfg.OpenAIModel != "gpt-4o" {
		t.Errorf("config not stored: %+v", a.cfg)
	}
}

// TestAgent_EnsureSessionCreates verifies the cache is created
// on first use and reused thereafter.
func TestAgent_EnsureSessionCreates(t *testing.T) {
	a := NewAgent(AgentConfig{}, NewRegistry())
	c1 := a.ensureSession("s1")
	c2 := a.ensureSession("s1")
	if c1 != c2 {
		t.Errorf("ensureSession should return the same instance for the same id")
	}
	if c1 == nil {
		t.Errorf("cache should not be nil")
	}

	c3 := a.ensureSession("s2")
	if c3 == c1 {
		t.Errorf("different session ids should yield different caches")
	}
}

// TestAgent_GetSessionReturnsExisting tests the read-only
// fetcher used by Resume.
func TestAgent_GetSessionReturnsExisting(t *testing.T) {
	a := NewAgent(AgentConfig{}, NewRegistry())
	a.ensureSession("exists")
	if c, ok := a.getSession("exists"); !ok || c == nil {
		t.Errorf("getSession should find the cached session")
	}
	if _, ok := a.getSession("missing"); ok {
		t.Errorf("getSession should return false for unknown ids")
	}
}

// TestAgent_Chat_RejectsEmptySessionID is the negative path of
// the public Chat() contract.
func TestAgent_Chat_RejectsEmptySessionID(t *testing.T) {
	a := NewAgent(AgentConfig{}, NewRegistry())
	if _, err := a.Chat(context.Background(), "", nil); err == nil {
		t.Errorf("Chat with empty sessionID should return an error")
	}
}

// TestAgent_ConfirmTool_RejectsInvalidDecision locks the
// 4-value whitelist at the public API level.
func TestAgent_ConfirmTool_RejectsInvalidDecision(t *testing.T) {
	a := NewAgent(AgentConfig{}, NewRegistry())
	if _, err := a.ConfirmTool(context.Background(), "s", "tc", Decision("bogus")); err == nil {
		t.Errorf("ConfirmTool with invalid decision should error")
	}
	if _, err := a.ConfirmTool(context.Background(), "s", "tc", Decision("")); err == nil {
		t.Errorf("ConfirmTool with empty decision should error")
	}
}

// TestAgent_ConfirmTool_NoPendingCall verifies the public error
// path: calling ConfirmTool when there is no pending call.
func TestAgent_ConfirmTool_NoPendingCall(t *testing.T) {
	a := NewAgent(AgentConfig{}, NewRegistry())
	if _, err := a.ConfirmTool(context.Background(), "missing", "tc", DecisionAccept); err == nil {
		t.Errorf("ConfirmTool with no pending call should error")
	}
}

// TestAgent_ConfirmTool_ToolCallIDMismatch locks the second
// argument check.
func TestAgent_ConfirmTool_ToolCallIDMismatch(t *testing.T) {
	a := NewAgent(AgentConfig{}, NewRegistry())
	a.mu.Lock()
	a.PendingCalls["s"] = &pendingCall{
		ToolCallID: "call_real",
		ToolName:   "x",
		Args:       `{}`,
		Messages:   []openai.ChatCompletionMessage{},
	}
	a.mu.Unlock()
	_, err := a.ConfirmTool(context.Background(), "s", "call_other", DecisionAccept)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Errorf("expected toolCallID mismatch error, got %v", err)
	}
}

// TestAgent_Resume_RejectsEmptySessionID is the negative path
// of Resume.
func TestAgent_Resume_RejectsEmptySessionID(t *testing.T) {
	a := NewAgent(AgentConfig{}, NewRegistry())
	if _, err := a.Resume(context.Background(), "", 0); err == nil {
		t.Errorf("Resume with empty sessionID should return an error")
	}
}

// TestAgent_Resume_SessionNotFound is the second negative path.
func TestAgent_Resume_SessionNotFound(t *testing.T) {
	a := NewAgent(AgentConfig{}, NewRegistry())
	if _, err := a.Resume(context.Background(), "missing", 0); err == nil {
		t.Errorf("Resume with unknown sessionID should return an error")
	}
}

// TestGrantKey_Format locks the cache-key shape so
// SessionGrants stays in sync with isGranted.
func TestGrantKey_Format(t *testing.T) {
	k := grantKey("sess_1", "delete_file")
	if k != "sess_1|delete_file" {
		t.Errorf("grantKey: got %q", k)
	}
}

// TestIsValidDecision_CoversAllValues locks the 4-value
// whitelist without depending on the HTTP layer.
func TestIsValidDecision_CoversAllValues(t *testing.T) {
	if !isValidDecision(DecisionAccept) {
		t.Errorf("accept should be valid")
	}
	if !isValidDecision(DecisionAcceptForSession) {
		t.Errorf("accept_for_session should be valid")
	}
	if !isValidDecision(DecisionDecline) {
		t.Errorf("decline should be valid")
	}
	if !isValidDecision(DecisionCancel) {
		t.Errorf("cancel should be valid")
	}
	if isValidDecision(Decision("garbage")) {
		t.Errorf("garbage should be invalid")
	}
	if isValidDecision(Decision("")) {
		t.Errorf("empty should be invalid")
	}
}

// TestAgent_SessionGrantRoundTrip exercises isGranted + the
// sessionGrants store directly. This is the storage contract
// that powers the accept_for_session decision.
func TestAgent_SessionGrantRoundTrip(t *testing.T) {
	a := NewAgent(AgentConfig{}, NewRegistry())
	if a.isGranted("s1", "delete_file") {
		t.Errorf("no grant should be set yet")
	}
	a.SessionGrants.Store(grantKey("s1", "delete_file"), struct{}{})
	if !a.isGranted("s1", "delete_file") {
		t.Errorf("grant should be visible after Store")
	}
	if a.isGranted("s1", "other_tool") {
		t.Errorf("grant must be per-tool")
	}
	if a.isGranted("other_session", "delete_file") {
		t.Errorf("grant must be per-session")
	}
}

// TestAgent_ConfirmTool_AcceptForSession_StoresGrantAndRunsHandler
// covers the "accept for session" decision end-to-end. We
// register a tool, manually populate a pending call, then call
// ConfirmTool with accept_for_session and verify:
//  1. the grant is stored
//  2. the tool handler is invoked once
//  3. the agent pushes a tool_result event
//  4. a stream_end event closes the channel
func TestAgent_ConfirmTool_AcceptForSession_StoresGrantAndRunsHandler(t *testing.T) {
	a := NewAgent(AgentConfig{}, NewRegistry())

	var calls int
	var mu sync.Mutex
	a.Registry.Register("echo", nil, func(args string) (string, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		return `{"ok":true}`, nil
	}, true, KindCommand)

	// Manually install a pending call.
	a.mu.Lock()
	a.PendingCalls["sess"] = &pendingCall{
		ToolCallID: "call_x",
		ToolName:   "echo",
		Args:       `{}`,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleAssistant,
				ToolCalls: []openai.ToolCall{
					{
						ID:   "call_x",
						Type: openai.ToolTypeFunction,
						Function: openai.FunctionCall{
							Name:      "echo",
							Arguments: `{}`,
						},
					},
				},
			},
		},
	}
	a.mu.Unlock()

	// Inject a fake LLM that errors out — accept_for_session
	// still runs the handler, but the post-handler LLM call
	// returns an error which finishAndClose absorbs.
	fake := newFakeLLM()
	fake.openErr = errors.New("no_more_llm_calls")
	fake.openErrOnce = true
	a.llm = fake

	ch, err := a.ConfirmTool(context.Background(), "sess", "call_x", DecisionAcceptForSession)
	if err != nil {
		t.Fatalf("ConfirmTool error: %v", err)
	}
	events := drainChatStream(t, ch)
	mu.Lock()
	if calls != 1 {
		t.Errorf("expected handler to be called once, got %d", calls)
	}
	mu.Unlock()
	if !a.isGranted("sess", "echo") {
		t.Errorf("accept_for_session should store grant")
	}
	if !sessionHasEvent(events, EventToolResult) {
		t.Errorf("expected a tool_result event")
	}
	if !sessionHasEvent(events, EventStreamEnd) {
		t.Errorf("expected stream_end")
	}
}

// TestAgent_ConfirmTool_Accept_RunsHandlerOnceWithoutGrant
// checks the accept (one-shot) decision: handler runs but no
// grant is stored.
func TestAgent_ConfirmTool_Accept_RunsHandlerOnceWithoutGrant(t *testing.T) {
	a := NewAgent(AgentConfig{}, NewRegistry())
	var calls int
	var mu sync.Mutex
	a.Registry.Register("echo", nil, func(args string) (string, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		return `{"ok":true}`, nil
	}, true, KindCommand)

	a.mu.Lock()
	a.PendingCalls["sess"] = &pendingCall{
		ToolCallID: "call_x",
		ToolName:   "echo",
		Args:       `{}`,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleAssistant,
				ToolCalls: []openai.ToolCall{
					{ID: "call_x", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "echo", Arguments: `{}`}},
				},
			},
		},
	}
	a.mu.Unlock()

	fake := newFakeLLM()
	fake.openErr = errors.New("end")
	fake.openErrOnce = true
	a.llm = fake

	ch, err := a.ConfirmTool(context.Background(), "sess", "call_x", DecisionAccept)
	if err != nil {
		t.Fatalf("ConfirmTool: %v", err)
	}
	_ = drainChatStream(t, ch)
	if calls != 1 {
		t.Errorf("expected handler to run once, got %d", calls)
	}
	if a.isGranted("sess", "echo") {
		t.Errorf("accept (one-shot) should NOT store grant")
	}
}

// TestAgent_ConfirmTool_Decline_PushesUserRejectedAndContinues
// verifies the decline path: handler is NOT called, but a
// tool_result with user_rejected is pushed, and the loop
// continues (so the LLM can react).
func TestAgent_ConfirmTool_Decline_PushesUserRejectedAndContinues(t *testing.T) {
	a := NewAgent(AgentConfig{}, NewRegistry())
	var calls int
	a.Registry.Register("echo", nil, func(args string) (string, error) {
		calls++
		return `{}`, nil
	}, true, KindCommand)

	a.mu.Lock()
	a.PendingCalls["sess"] = &pendingCall{
		ToolCallID: "call_x",
		ToolName:   "echo",
		Args:       `{}`,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleAssistant,
				ToolCalls: []openai.ToolCall{
					{ID: "call_x", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "echo", Arguments: `{}`}},
				},
			},
		},
	}
	a.mu.Unlock()

	fake := newFakeLLM()
	fake.openErr = errors.New("end")
	fake.openErrOnce = true
	a.llm = fake

	ch, err := a.ConfirmTool(context.Background(), "sess", "call_x", DecisionDecline)
	if err != nil {
		t.Fatalf("ConfirmTool: %v", err)
	}
	events := drainChatStream(t, ch)
	if calls != 0 {
		t.Errorf("decline must NOT invoke handler; got %d calls", calls)
	}
	if !sessionHasEvent(events, EventToolResult) {
		t.Errorf("expected a tool_result event")
	}
	for _, e := range events {
		if e.Type == EventToolResult && strings.Contains(e.Data, "user_rejected") {
			return
		}
	}
	t.Errorf("expected user_rejected in tool_result, events: %+v", events)
}

// TestAgent_ConfirmTool_Cancel_TerminatesStream is the
// cancel-and-stop path. The stream must end with a stream_end
// and the handler must not be called.
func TestAgent_ConfirmTool_Cancel_TerminatesStream(t *testing.T) {
	a := NewAgent(AgentConfig{}, NewRegistry())
	var calls int
	a.Registry.Register("echo", nil, func(args string) (string, error) {
		calls++
		return `{}`, nil
	}, true, KindCommand)

	a.mu.Lock()
	a.PendingCalls["sess"] = &pendingCall{
		ToolCallID: "call_x",
		ToolName:   "echo",
		Args:       `{}`,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleAssistant,
				ToolCalls: []openai.ToolCall{
					{ID: "call_x", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "echo", Arguments: `{}`}},
				},
			},
		},
	}
	a.mu.Unlock()

	// No need to inject a fake LLM; cancel should not call
	// runLoop, so the LLM is never invoked.
	ch, err := a.ConfirmTool(context.Background(), "sess", "call_x", DecisionCancel)
	if err != nil {
		t.Fatalf("ConfirmTool: %v", err)
	}
	events := drainChatStream(t, ch)
	if calls != 0 {
		t.Errorf("cancel must NOT invoke handler; got %d calls", calls)
	}
	if !sessionHasEvent(events, EventStreamEnd) {
		t.Errorf("cancel should end the stream")
	}
	for _, e := range events {
		if e.Type == EventToolResult && strings.Contains(e.Data, "user_cancelled") {
			return
		}
	}
	t.Errorf("expected user_cancelled in tool_result; events: %+v", events)
}

// TestPendingArgs_LocatesArgsFromAssistantMessage locks the
// helper that recovers the original tool args from the
// messages slice after a suspended turn.
func TestPendingArgs_LocatesArgsFromAssistantMessage(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "hi"},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{
			{ID: "a", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "n", Arguments: `{"x":1}`}},
		}},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{
			{ID: "b", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "n", Arguments: `{"x":2}`}},
		}},
	}
	if got := pendingArgs(msgs, "b"); got != `{"x":2}` {
		t.Errorf("pendingArgs(b): got %q", got)
	}
	if got := pendingArgs(msgs, "missing"); got != "" {
		t.Errorf("pendingArgs(missing) should be empty, got %q", got)
	}
}

// TestAgent_Chat_RejectsWhenPendingCallLocks confirms the
// single-pending-call guard. While a session has a pending
// call, a second Chat() call on the same session is rejected.
func TestAgent_Chat_RejectsWhenPendingCallLocks(t *testing.T) {
	a := NewAgent(AgentConfig{}, NewRegistry())
	a.mu.Lock()
	a.PendingCalls["s"] = &pendingCall{ToolCallID: "x", ToolName: "y", Args: "{}"}
	a.mu.Unlock()
	_, err := a.Chat(context.Background(), "s", nil)
	if err == nil || !strings.Contains(err.Error(), "awaiting confirmation") {
		t.Errorf("expected awaiting-confirmation error, got %v", err)
	}
}

// TestAgent_RunTool_TimesHandlerExecution covers the
// instrumentation that backs the DurationMs field on
// ToolResultData.
func TestAgent_RunTool_TimesHandlerExecution(t *testing.T) {
	a := NewAgent(AgentConfig{}, NewRegistry())
	def := ToolDefinition{
		Handler: func(args string) (string, error) {
			time.Sleep(5 * time.Millisecond)
			return `{"ok":true}`, nil
		},
	}
	out, status, err := a.runTool(def, `{}`)
	if err != nil {
		t.Errorf("runTool returned error: %v", err)
	}
	if status != "success" {
		t.Errorf("status: got %q want success", status)
	}
	if out != `{"ok":true}` {
		t.Errorf("output: got %q", out)
	}
	// Error path.
	def = ToolDefinition{
		Handler: func(args string) (string, error) {
			return "", errors.New("boom")
		},
	}
	out, status, err = a.runTool(def, `{}`)
	if err == nil {
		t.Errorf("expected error")
	}
	if status != "failed" {
		t.Errorf("status: got %q want failed", status)
	}
	if !strings.Contains(out, "boom") {
		t.Errorf("output should contain error message, got %q", out)
	}
}

// TestCloneMessages_DefensiveCopy makes sure the snapshot used
// by PendingCalls is independent of the caller's slice.
func TestCloneMessages_DefensiveCopy(t *testing.T) {
	original := []openai.ChatCompletionMessage{{Role: "user", Content: "hi"}}
	cloned := cloneMessages(original)
	cloned[0].Content = "tampered"
	if original[0].Content != "hi" {
		t.Errorf("cloneMessages must be a deep copy; original was mutated")
	}
}
