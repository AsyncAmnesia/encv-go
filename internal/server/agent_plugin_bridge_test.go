// internal/server/agent_plugin_bridge_test.go
//
// 插件桥接 + session 缓存的单元测试。
//
// 测试策略：
//   - ListPluginTools / pluginNameCN：纯函数测试，无需 mock
//   - executePluginTool 错误路径：传 invalid args → 验证返回 errJSON（不调用真插件）
//   - getOrCreateSession：map 并发安全 + 复用 + LastAccess 更新
//   - chatMsg 序列化：ToolCallID/Name/ToolCalls 字段 JSON 编解码正确
//   - session GC：清理过期 + 保留活跃 + LastAccess 不被错误清理
package server

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

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
	if back.Role != msg.Role || back.Content != msg.Content ||
		back.ToolCallID != msg.ToolCallID || back.Name != msg.Name {
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

	// 模拟 confirm 流程（用新的 ToolCalls 字段，不再 hack Content）
	sess.mu.Lock()
	tool := sess.PendingTools[0]
	allToolCalls := make([]toolCallAccumulator, len(sess.PendingTools))
	copy(allToolCalls, sess.PendingTools)
	assistantMsg := chatMsg{
		Role:      "assistant",
		Content:   "",
		ToolCalls: allToolCalls,
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
	if len(sess.Messages[1].ToolCalls) != 1 {
		t.Fatalf("assistant 消息 ToolCalls 数量 = %d, want 1", len(sess.Messages[1].ToolCalls))
	}
	if sess.Messages[1].ToolCalls[0].ID != "call_1" {
		t.Errorf("assistant ToolCalls[0].ID = %q, want call_1", sess.Messages[1].ToolCalls[0].ID)
	}
	if sess.Messages[1].ToolCalls[0].Function.Name != "video_encrypt" {
		t.Errorf("assistant ToolCalls[0].Function.Name = %q, want video_encrypt", sess.Messages[1].ToolCalls[0].Function.Name)
	}
	if sess.Messages[1].Content != "" {
		t.Errorf("assistant 消息 Content 应为空（被 ToolCalls 替代）: %q", sess.Messages[1].Content)
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

// ─── session GC ──────────────────────────────────────────────

func TestGcIdleSessions_EvictsExpired(t *testing.T) {
	// 创建一个 session 然后手动回拨 LastAccess 到过期
	id := "gc-evict-test"
	sess := getOrCreateSession(id)
	if sess == nil {
		t.Fatal("getOrCreateSession 失败")
	}

	// 回拨到 31 分钟前（超过 sessionIdleTTL=30min）
	sessionMu.Lock()
	sess.LastAccess = time.Now().Add(-31 * time.Minute)
	sessionMu.Unlock()

	// 执行 GC
	evicted := gcIdleSessions()
	if evicted < 1 {
		t.Errorf("应至少清理 1 个 session, got %d", evicted)
	}

	// 验证 session 已消失
	sessionMu.RLock()
	_, exists := sessions[id]
	sessionMu.RUnlock()
	if exists {
		t.Errorf("session %q 应已被 GC 清理", id)
	}
}

func TestGcIdleSessions_PreservesActive(t *testing.T) {
	// 创建活跃 session（LastAccess = now），GC 不应清理
	id := "gc-active-test"
	sess := getOrCreateSession(id)
	if sess == nil {
		t.Fatal("getOrCreateSession 失败")
	}

	// 显式设置 LastAccess 为 1 分钟前（未过期）
	sessionMu.Lock()
	sess.LastAccess = time.Now().Add(-1 * time.Minute)
	sessionMu.Unlock()

	evicted := gcIdleSessions()
	_ = evicted // 可能有别的测试残留的过期 session 被清

	sessionMu.RLock()
	_, exists := sessions[id]
	sessionMu.RUnlock()
	if !exists {
		t.Errorf("活跃 session %q 不应被 GC 清理", id)
	}
}

func TestGetOrCreateSession_UpdatesLastAccess(t *testing.T) {
	// 首次创建
	id := "gc-touch-test"
	s1 := getOrCreateSession(id)
	sessionMu.RLock()
	firstAccess := s1.LastAccess
	sessionMu.RUnlock()

	// 睡 50ms 后再 getOrCreate → LastAccess 必须更新
	time.Sleep(50 * time.Millisecond)
	s2 := getOrCreateSession(id)
	sessionMu.RLock()
	secondAccess := s2.LastAccess
	sessionMu.RUnlock()

	if !secondAccess.After(firstAccess) {
		t.Errorf("LastAccess 未更新: first=%v, second=%v", firstAccess, secondAccess)
	}
}

// ─── chatMsg.ToolCalls 序列化 ────────────────────────────────

func TestChatMsg_AssistantWithToolCalls(t *testing.T) {
	// assistant 消息带 tool_calls 数组 → 序列化必须正确
	tc := toolCallAccumulator{ID: "call_1", Type: "function"}
	tc.Function.Name = "video_encrypt"
	tc.Function.Arguments = `{"input_path":"/a.mp4"}`

	msg := chatMsg{
		Role:      "assistant",
		Content:   "",
		ToolCalls: []toolCallAccumulator{tc},
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal 失败: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, `"role":"assistant"`) {
		t.Errorf("缺 role: %s", got)
	}
	if !strings.Contains(got, `"tool_calls":[{`) {
		t.Errorf("缺 tool_calls 数组: %s", got)
	}
	if !strings.Contains(got, `"id":"call_1"`) {
		t.Errorf("缺 tool_call id: %s", got)
	}
	if !strings.Contains(got, `"name":"video_encrypt"`) {
		t.Errorf("缺 function.name: %s", got)
	}
	if !strings.Contains(got, `"arguments":"{\"input_path\":\"/a.mp4\"}"`) {
		t.Errorf("缺 function.arguments: %s", got)
	}

	// 反序列化恢复
	var back chatMsg
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal 失败: %v", err)
	}
	if len(back.ToolCalls) != 1 || back.ToolCalls[0].ID != "call_1" {
		t.Errorf("反序列化 ToolCalls 异常: %+v", back.ToolCalls)
	}
}

func TestChatMsg_OmitsToolCallsWhenEmpty(t *testing.T) {
	// 普通 user 消息不应有 tool_calls 字段（omitempty 生效）
	msg := chatMsg{Role: "user", Content: "hello"}
	raw, _ := json.Marshal(msg)
	got := string(raw)
	if strings.Contains(got, "tool_calls") {
		t.Errorf("user 消息不应有 tool_calls 字段: %s", got)
	}
	if strings.Contains(got, "tool_call_id") {
		t.Errorf("user 消息不应有 tool_call_id 字段: %s", got)
	}
}

func TestToolCallAccumulator_IndexOmittedWhenZero(t *testing.T) {
	// Index 字段 omitempty：0 值不序列化（OpenAI 协议不需要 index）
	tc := toolCallAccumulator{ID: "call_x", Type: "function"}
	tc.Function.Name = "video_encrypt"
	raw, _ := json.Marshal(tc)
	got := string(raw)
	if strings.Contains(got, `"index"`) {
		t.Errorf("Index=0 不应被序列化: %s", got)
	}
	// 验证必要字段都在
	for _, want := range []string{`"id":"call_x"`, `"type":"function"`, `"name":"video_encrypt"`} {
		if !strings.Contains(got, want) {
			t.Errorf("缺字段 %s: %s", want, got)
		}
	}
}

func TestToolCallAccumulator_IndexSerializedWhenNonZero(t *testing.T) {
	// Index 字段非 0 时正常序列化（流式响应需要）
	tc := toolCallAccumulator{ID: "call_x", Type: "function", Index: 3}
	tc.Function.Name = "video_encrypt"
	raw, _ := json.Marshal(tc)
	got := string(raw)
	if !strings.Contains(got, `"index":3`) {
		t.Errorf("Index=3 应被序列化: %s", got)
	}
}
