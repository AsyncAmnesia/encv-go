// internal/server/agent_plugin_bridge_test.go
//
// 插件桥接 + session 缓存的单元测试。
//
// 测试策略：
//   - ListPluginTools / pluginNameCN：纯函数测试，无需 mock
//   - executePluginTool 错误路径：传 invalid args → 验证返回 errJSON（不调用真插件）
//   - getOrCreateSession：map 并发安全 + 复用
//   - chatMsg 序列化：ToolCallID/Name 字段 JSON 编解码正确
package server

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"testing"

	encvPlugins "github.com/Soltus/encv-go/pkg/encv/plugins"
)

// ─── ListPluginTools ──────────────────────────────────────────

func TestListPluginTools_Contains12Tools(t *testing.T) {
	tools := ListPluginTools()
	// 排除所有 alist* 开头的 OpenList 工具族，剩下的插件 × 2
	allPlugins := encvPlugins.Plugins()
	expected := 0
	for _, p := range allPlugins {
		if !strings.HasPrefix(p.Name(), "alist") {
			expected += 2
		}
	}
	if len(tools) != expected {
		t.Errorf("ListPluginTools() 长度 = %d, want %d", len(tools), expected)
	}

	// 验证每个工具都有 name/description/parameters/needConfirm
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool["name"].(string))
		if tool["description"] == nil || tool["description"].(string) == "" {
			t.Errorf("tool %v 缺 description", tool["name"])
		}
		if tool["parameters"] == nil {
			t.Errorf("tool %v 缺 parameters", tool["name"])
		}
		if tool["needConfirm"] != true {
			t.Errorf("tool %v needConfirm 应为 true", tool["name"])
		}
	}
	sort.Strings(names)
	t.Logf("工具列表: %v", names)
}

func TestListPluginTools_NoDuplicateNames(t *testing.T) {
	tools := ListPluginTools()
	seen := make(map[string]bool)
	for _, tool := range tools {
		name := tool["name"].(string)
		if seen[name] {
			t.Errorf("工具名重复: %s", name)
		}
		seen[name] = true
	}
}

func TestListPluginTools_SkipsAlistencrypt(t *testing.T) {
	tools := ListPluginTools()
	for _, tool := range tools {
		name := tool["name"].(string)
		// alistencrypt / alist_encrypt / 任何 alist* 都不应出现
		if strings.HasPrefix(name, "alist") {
			t.Errorf("alist* 工具不应出现在工具列表（已砍掉 OpenList）: %s", name)
		}
	}
}

// ─── pluginNameCN ─────────────────────────────────────────────

func TestPluginNameCN(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"video", "视频"},
		{"audio", "音频"},
		{"image", "图片"},
		{"wps", "WPS 文档"},
		{"pdf", "PDF"},
		{"text", "文本"},
		{"unknown_plugin", "unknown_plugin"}, // 未知名 fallback
	}
	for _, tt := range tests {
		if got := pluginNameCN(tt.name); got != tt.want {
			t.Errorf("pluginNameCN(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// ─── executePluginTool 错误路径 ───────────────────────────────

func TestExecutePluginTool_UnknownTool(t *testing.T) {
	_, err := executePluginTool(context.Background(), "nonexistent_tool", "{}")
	if err == nil {
		t.Fatal("executePluginTool(未知工具) 应返回 error")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("error 应包含 'unknown tool', got: %v", err)
	}
}

func TestExecutePluginTool_InvalidArgsJSON(t *testing.T) {
	// 找到第一个有效的 encrypt 工具
	tools := ListPluginTools()
	if len(tools) == 0 {
		t.Skip("无可用插件工具")
	}
	var encryptTool string
	for _, tool := range tools {
		name := tool["name"].(string)
		if strings.HasSuffix(name, "_encrypt") {
			encryptTool = name
			break
		}
	}
	if encryptTool == "" {
		t.Skip("未找到 _encrypt 工具")
	}

	// 传非法 JSON → 应返回 errJSON(invalid_args) 而非 panic
	raw, err := executePluginTool(context.Background(), encryptTool, "not-valid-json{")
	if err != nil {
		t.Fatalf("executePluginTool 错误路径不应返回 error，应返回 errJSON: %v", err)
	}
	if !strings.Contains(raw, `"error":"invalid_args"`) {
		t.Errorf("result 应包含 invalid_args, got: %s", raw)
	}
}

func TestExecutePluginTool_MissingArgs(t *testing.T) {
	tools := ListPluginTools()
	if len(tools) == 0 {
		t.Skip("无可用插件工具")
	}
	var encryptTool string
	for _, tool := range tools {
		name := tool["name"].(string)
		if strings.HasSuffix(name, "_encrypt") {
			encryptTool = name
			break
		}
	}
	if encryptTool == "" {
		t.Skip("未找到 _encrypt 工具")
	}

	// 传空 args → 应返回 errJSON(missing_args)
	raw, _ := executePluginTool(context.Background(), encryptTool, `{}`)
	if !strings.Contains(raw, `"error":"missing_args"`) {
		t.Errorf("result 应包含 missing_args, got: %s", raw)
	}
}

// ─── getOrCreateSession ───────────────────────────────────────

func TestGetOrCreateSession_NewAndReuse(t *testing.T) {
	id := "test-session-1"
	s1 := getOrCreateSession(id)
	if s1 == nil {
		t.Fatal("getOrCreateSession 不应返回 nil")
	}
	if s1.SessionID != id {
		t.Errorf("SessionID = %q, want %q", s1.SessionID, id)
	}
	s2 := getOrCreateSession(id)
	if s1 != s2 {
		t.Errorf("同 ID 应返回同一 session 实例")
	}
}

func TestGetOrCreateSession_DifferentIDs(t *testing.T) {
	s1 := getOrCreateSession("session-a")
	s2 := getOrCreateSession("session-b")
	if s1 == s2 {
		t.Errorf("不同 ID 应返回不同 session 实例")
	}
}

func TestGetOrCreateSession_Concurrent(t *testing.T) {
	// 并发创建同 ID → 必须返回同一实例
	const id = "concurrent-session"
	const n = 50
	var wg sync.WaitGroup
	results := make([]*agentSession, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx] = getOrCreateSession(id)
		}(i)
	}
	wg.Wait()
	for i := 1; i < n; i++ {
		if results[0] != results[i] {
			t.Errorf("并发创建返回了不同实例: results[0]=%p, results[%d]=%p",
				results[0], i, results[i])
		}
	}
}

// ─── chatMsg 序列化（OpenAI 工具调用协议） ────────────────────

func TestChatMsg_ToolMessageSerialization(t *testing.T) {
	// 验证 ToolCallID 和 Name 字段能正确 JSON 序列化（OpenAI tool message 协议要求）
	msg := chatMsg{
		Role:       "tool",
		Content:    `{"ok": true}`,
		ToolCallID: "call_abc123",
		Name:       "video_encrypt",
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal 失败: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, `"role":"tool"`) {
		t.Errorf("序列化缺 role: %s", got)
	}
	if !strings.Contains(got, `"tool_call_id":"call_abc123"`) {
		t.Errorf("序列化缺 tool_call_id: %s", got)
	}
	if !strings.Contains(got, `"name":"video_encrypt"`) {
		t.Errorf("序列化缺 name: %s", got)
	}
	if !strings.Contains(got, `"content":"{\"ok\": true}"`) {
		t.Errorf("序列化缺 content: %s", got)
	}

	// 反序列化也必须能恢复
	var back chatMsg
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal 失败: %v", err)
	}
	if back != msg {
		t.Errorf("反序列化结果不一致:\n got: %+v\nwant: %+v", back, msg)
	}
}

func TestChatMsg_UserMessageOmitsToolFields(t *testing.T) {
	// user 消息不应有 tool_call_id / name → omitempty 生效
	msg := chatMsg{Role: "user", Content: "hello"}
	raw, _ := json.Marshal(msg)
	got := string(raw)
	if strings.Contains(got, "tool_call_id") {
		t.Errorf("user 消息不应有 tool_call_id: %s", got)
	}
	if strings.Contains(got, `"name"`) {
		t.Errorf("user 消息不应有 name 字段: %s", got)
	}
}

// ─── Session 状态机 ──────────────────────────────────────────

func TestAgentSession_StoreMessagesAndPendingTools(t *testing.T) {
	id := "session-messages"
	sess := getOrCreateSession(id)

	// 模拟 handleAgentChat 缓存 messages
	userMsg := chatMsg{Role: "user", Content: "加密 /tmp/test.mp4"}
	sess.mu.Lock()
	sess.Messages = []chatMsg{userMsg}
	sess.LastModel = "gpt-4o-mini"
	sess.LastTemperature = 0.7
	sess.PendingTools = nil
	sess.mu.Unlock()

	// 模拟 LLM 返回 tool_call
	tc := toolCallAccumulator{ID: "call_xyz", Index: 0, Type: "function"}
	tc.Function.Name = "video_encrypt"
	tc.Function.Arguments = `{"input_path":"/tmp/test.mp4","output_dir":"/tmp/out"}`
	sess.mu.Lock()
	sess.PendingTools = []toolCallAccumulator{tc}
	sess.mu.Unlock()

	// 模拟 handleAgentConfirm 读取
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if len(sess.Messages) != 1 || sess.Messages[0].Content != "加密 /tmp/test.mp4" {
		t.Errorf("Messages 状态异常: %+v", sess.Messages)
	}
	if len(sess.PendingTools) != 1 {
		t.Fatalf("PendingTools 数量 = %d, want 1", len(sess.PendingTools))
	}
	got := sess.PendingTools[0]
	if got.ID != "call_xyz" || got.Function.Name != "video_encrypt" {
		t.Errorf("PendingTools 内容异常: %+v", got)
	}
	if got.Function.Arguments == "" {
		t.Error("PendingTools.Function.Arguments 不应为空")
	}
	if sess.LastModel != "gpt-4o-mini" {
		t.Errorf("LastModel = %q, want gpt-4o-mini", sess.LastModel)
	}
}

func TestAgentSession_ConfirmAppendsAssistantAndToolMessages(t *testing.T) {
	// 模拟 confirm accept 后的 messages 追加
	id := "session-confirm"
	sess := getOrCreateSession(id)

	// 初始：用户消息
	sess.mu.Lock()
	sess.Messages = []chatMsg{{Role: "user", Content: "加密"}}
	sess.PendingTools = []toolCallAccumulator{{ID: "call_1", Index: 0, Type: "function"}}
	sess.PendingTools[0].Function.Name = "video_encrypt"
	sess.PendingTools[0].Function.Arguments = `{"input_path":"/a.mp4"}`
	sess.mu.Unlock()

	// 模拟 confirm 流程
	sess.mu.Lock()
	tool := sess.PendingTools[0]
	assistantMsg := chatMsg{
		Role:    "assistant",
		Content: "[tool_call:" + tool.ID + ":" + tool.Function.Name + ":" + tool.Function.Arguments + "]",
	}
	toolMsg := chatMsg{
		Role:       "tool",
		Content:    `{"output":"/tmp/a.encv"}`,
		ToolCallID: tool.ID,
		Name:       tool.Function.Name,
	}
	sess.Messages = append(sess.Messages, assistantMsg, toolMsg)
	sess.PendingTools = nil
	sess.mu.Unlock()

	// 验证
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if len(sess.Messages) != 3 {
		t.Fatalf("Messages 数量 = %d, want 3", len(sess.Messages))
	}
	if sess.Messages[1].Role != "assistant" {
		t.Errorf("第二条消息 role = %q, want assistant", sess.Messages[1].Role)
	}
	if sess.Messages[2].Role != "tool" {
		t.Errorf("第三条消息 role = %q, want tool", sess.Messages[2].Role)
	}
	if sess.Messages[2].ToolCallID != "call_1" {
		t.Errorf("tool_msg ToolCallID = %q, want call_1", sess.Messages[2].ToolCallID)
	}
	if sess.Messages[2].Name != "video_encrypt" {
		t.Errorf("tool_msg Name = %q, want video_encrypt", sess.Messages[2].Name)
	}
	if len(sess.PendingTools) != 0 {
		t.Errorf("PendingTools 应已清空, got %d", len(sess.PendingTools))
	}
}

// ─── toolCallAccumulator JSON 编解码 ─────────────────────────

func TestToolCallAccumulator_RoundTrip(t *testing.T) {
	orig := toolCallAccumulator{
		ID:    "call_999",
		Type:  "function",
		Index: 0,
	}
	orig.Function.Name = "video_encrypt"
	orig.Function.Arguments = `{"input_path":"/x.mp4","output_dir":"/out"}`

	raw, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal 失败: %v", err)
	}
	var back toolCallAccumulator
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal 失败: %v", err)
	}
	if back.ID != orig.ID || back.Type != orig.Type || back.Index != orig.Index {
		t.Errorf("顶层字段不一致: got %+v, want %+v", back, orig)
	}
	if back.Function.Name != orig.Function.Name {
		t.Errorf("Function.Name 不一致: got %q, want %q", back.Function.Name, orig.Function.Name)
	}
	if back.Function.Arguments != orig.Function.Arguments {
		t.Errorf("Function.Arguments 不一致: got %q, want %q", back.Function.Arguments, orig.Function.Arguments)
	}
}

// ─── okJSON / errJSON 辅助函数 ────────────────────────────────

func TestOkJSON_ValidJSON(t *testing.T) {
	got := okJSON(map[string]interface{}{"a": 1, "b": "x"})
	if !strings.Contains(got, `"a":1`) || !strings.Contains(got, `"b":"x"`) {
		t.Errorf("okJSON 输出异常: %s", got)
	}
}

func TestErrJSON_HasErrorAndMessage(t *testing.T) {
	got := errJSON("test_code", "test message")
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("errJSON 输出非合法 JSON: %s", got)
	}
	if m["error"] != "test_code" {
		t.Errorf("error 字段 = %v, want test_code", m["error"])
	}
	if m["message"] != "test message" {
		t.Errorf("message 字段 = %v, want test message", m["message"])
	}
}
