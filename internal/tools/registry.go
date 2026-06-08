// Package tools 提供 AI agent 可调用的工具集。
//
// 设计目标：
//   - **统一调度**：所有 v1 + v2 工具通过同一个 ToolRegistry 派发
//   - **Mock 与真实路径共用**：MockEngine.execute_real 走 Registry 拿 handler，
//     真实 OpenAI 路径也走同一份代码，行为一致
//   - **权限标签**：每个 ToolDef 携带 RequiresConfirm / ReadOnly / Kind，
//     上层（MockEngine / UI）据此决定是否弹窗、是否自动执行
//   - **解耦注入**：handler 接收 ToolDeps 依赖（mountManager / config / logger），
//     不直接依赖 Server 具体实现，便于单测
//
// 参考：
//   - Spec: /workspace/.trae/specs/agent-tools-scenarios-v2/spec.md
//   - Requirement: 工具注册中心 ToolRegistry
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// ToolKind 工具种类分类（用于前端做不同 UI 渲染 / Mock 引擎做不同处理）。
type ToolKind string

const (
	KindFileRead   ToolKind = "fileRead"   // 只读文件操作
	KindFileChange ToolKind = "fileChange" // 写文件操作
	KindMetadata   ToolKind = "metadata"   // 元数据查询
	KindCommand    ToolKind = "command"    // 受限 shell
)

// ToolDeps 是所有工具 handler 共用的依赖注入。
// 由 Server 启动时构造一次，注入 GlobalRegistry 时绑定。
type ToolDeps struct {
	// ResolveMount 把 mount_id 解析为物理绝对路径。
	// 返回 ("", false) 时表示 mount 不存在或不可用。
	ResolveMount func(mountID string) (string, bool)
	// SandboxCheck 检查某个绝对路径是否在允许的沙箱内。
	// 用于 command_run 等需要"路径白名单"的工具。
	SandboxCheck func(absPath string) bool
	// Config 注入，方便 handler 读 ToolWhitelist / SandboxPaths 等。
	Config any // 实际为 *config.Agent，但为了避免循环依赖用 any
}

// ToolResult 是 handler 的统一返回结构。
// JSON 序列化为 SSE tool_result 事件 data 字段。
type ToolResult struct {
	// Result 是 JSON 字符串（与原 executeFSTool / executePluginTool 保持一致）。
	// 失败时可以是空或含 error 字段的 JSON。
	Result string
	// IsError 标记执行失败（前端据此显示红色错误态）。
	IsError bool
	// Status 是语义状态："success" / "failed" / "cancelled"。
	Status string
	// DurationMs 实际执行耗时（毫秒）。
	DurationMs int64
}

// ToolHandler 是所有工具的统一处理函数签名。
//   - ctx 用于取消 / 超时控制
//   - argsJSON 是 LLM 传入的参数字符串（与原 executeFSTool 兼容）
//   - deps 是依赖注入（mount 解析 / config 读取等）
// 返回 ToolResult，错误在 Result JSON 中体现（IsError 字段）。
type ToolHandler func(ctx context.Context, argsJSON string, deps *ToolDeps) (ToolResult, error)

// ToolDef 描述一个工具的元信息 + handler。
type ToolDef struct {
	// Name 工具 ID（"search_files" / "read_file" 等，全局唯一）。
	Name string
	// Description 给人 / 给 LLM 看的功能描述。
	Description string
	// ArgsSchema 是 JSON Schema 字符串，描述参数结构。
	ArgsSchema string
	// Handler 实际处理函数。
	Handler ToolHandler
	// RequiresConfirm 是否需要用户 4-决策确认才能执行。
	RequiresConfirm bool
	// ReadOnly 是否只读。ReadOnly=true 的工具不应修改文件系统。
	ReadOnly bool
	// Kind 工具种类。
	Kind ToolKind
}

// ToolRegistry 是线程安全的工具注册中心。
// MockEngine 和真实 LLM 路径共用同一份。
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]*ToolDef
}

// NewRegistry 返回空注册表。
func NewRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]*ToolDef)}
}

// Register 注册一个工具。已存在同名工具 → 报错（防止误覆盖）。
func (r *ToolRegistry) Register(def *ToolDef) error {
	if def == nil {
		return fmt.Errorf("tools: nil def")
	}
	if def.Name == "" {
		return fmt.Errorf("tools: empty name")
	}
	if def.Handler == nil {
		return fmt.Errorf("tools: %s has nil handler", def.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[def.Name]; exists {
		return fmt.Errorf("tools: duplicate registration for %q", def.Name)
	}
	r.tools[def.Name] = def
	return nil
}

// Get 按 name 查工具。返回 (def, true) / (nil, false)。
func (r *ToolRegistry) Get(name string) (*ToolDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.tools[name]
	return def, ok
}

// Has 判断 name 是否已注册（只读快速判断，避免 Get 的二次取值）。
// 与 Get(name).ok 等价但更明确语义。
func (r *ToolRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}

// List 返回所有工具，按 name 排序（确定性输出便于测试 + 前端展示）。
func (r *ToolRegistry) List() []*ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ToolDef, 0, len(r.tools))
	for _, def := range r.tools {
		out = append(out, def)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Names 返回所有工具的 name（按字母序），用于诊断 / 调试。
func (r *ToolRegistry) Names() []string {
	defs := r.List()
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}

// Dispatch 通过 registry 派发到对应 handler（统一入口）。
// 不在 registry 中的 tool 返回 "unknown tool" 错误。
//
// 用于替换原 s.executeAgentTool 中的 if-else 硬编码分支。
func (r *ToolRegistry) Dispatch(ctx context.Context, name, argsJSON string, deps *ToolDeps) (ToolResult, error) {
	def, ok := r.Get(name)
	if !ok {
		return ToolResult{
			Result:  fmt.Sprintf(`{"error":"unknown tool: %s"}`, name),
			IsError: true,
			Status:  "failed",
		}, fmt.Errorf("unknown tool: %s", name)
	}
	return def.Handler(ctx, argsJSON, deps)
}

// ─── 全局注册表 ─────────────────────────────────────────────────

// GlobalRegistry 是进程级单例。Server 启动时一次性 Register 所有内置工具。
// 单测环境可以构造独立的 *ToolRegistry 避免污染全局。
var GlobalRegistry = NewRegistry()

// MustRegister 在 GlobalRegistry 上注册，失败时 panic。
// 仅在 Server 启动（已知工具集）时使用。
func MustRegister(def *ToolDef) {
	if err := GlobalRegistry.Register(def); err != nil {
		panic(fmt.Sprintf("tools: must register %s: %v", def.Name, err))
	}
}

// ResetGlobal 清空 GlobalRegistry（仅用于单测）。
func ResetGlobal() {
	GlobalRegistry = NewRegistry()
}

// ArgsSchemaFromStruct 工具函数：把 Go 结构体反射为 JSON Schema（简化版）。
// 实际生产中通常直接写 schema 字符串更可控。
func ArgsSchemaFromStruct(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
