package agent

import "sync"

// ToolDefinition is the registration record of a single tool inside a
// [ToolRegistry].
//
// Schema is an OpenAI-style function-calling schema describing the
// tool's parameters; the agent forwards it verbatim when assembling the
// LLM request. Schema is typed as any so callers can use whichever
// schema representation they prefer (struct, map[string]any,
// *jsonschema.Schema, etc.).
//
// Handler executes the tool. It receives args as the raw JSON string
// the LLM produced (which avoids re-marshaling) and must return a JSON
// string that will be wrapped in [ToolResultData.Result].
//
// NeedConfirm is the source of truth for the AutoRun flag on
// [ToolCallData]: when true, the agent pauses for a [Decision] before
// invoking Handler.
//
// Kind classifies the tool for the front-end's approval card icon.
type ToolDefinition struct {
	Schema      any
	Handler     func(args string) (string, error)
	NeedConfirm bool
	Kind        ToolKind
}

// ToolRegistry is a thread-safe collection of tool definitions.
//
// A single registry is shared by the agent core and any HTTP handler
// that exposes tool metadata (e.g. an OpenAI-compatible /v1/tools
// endpoint), so reads must scale. Writes (Register) are expected to
// happen at startup, so a sync.RWMutex is the right primitive: many
// concurrent Get/GetAllSchemas callers, but only one or two writers.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]ToolDefinition
}

// NewRegistry returns an empty, ready-to-use registry.
func NewRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]ToolDefinition),
	}
}

// Register stores a tool under the given name. If a tool with the same
// name already exists, it is overwritten. Register is safe to call
// concurrently with Get / GetAllSchemas.
func (r *ToolRegistry) Register(
	name string,
	schema any,
	handler func(args string) (string, error),
	needConfirm bool,
	kind ToolKind,
) {
	def := ToolDefinition{
		Schema:      schema,
		Handler:     handler,
		NeedConfirm: needConfirm,
		Kind:        kind,
	}
	r.mu.Lock()
	r.tools[name] = def
	r.mu.Unlock()
}

// Get fetches a tool by name. The boolean return mirrors map lookups
// and lets callers branch without sentinel values.
func (r *ToolRegistry) Get(name string) (ToolDefinition, bool) {
	r.mu.RLock()
	def, ok := r.tools[name]
	r.mu.RUnlock()
	return def, ok
}

// GetAllSchemas returns every registered tool's Schema in an
// unspecified order. The slice is freshly allocated, so callers may
// mutate it without affecting the registry. This is the canonical
// shape the agent forwards to the LLM as the "tools" field.
func (r *ToolRegistry) GetAllSchemas() []any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]any, 0, len(r.tools))
	for _, def := range r.tools {
		out = append(out, def.Schema)
	}
	return out
}

// Names returns the registered tool names in unspecified
// order. The slice is freshly allocated and may be mutated by
// the caller. Used by the demo's enabled_tools filter and by
// debug endpoints that need to enumerate the registry.
func (r *ToolRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.tools))
	for name := range r.tools {
		out = append(out, name)
	}
	return out
}

// Unregister removes a tool by name. It is a no-op if the
// tool does not exist. The primary use case is the demo's
// enabled_tools filter; production code should not strip
// tools at runtime.
func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	delete(r.tools, name)
	r.mu.Unlock()
}
