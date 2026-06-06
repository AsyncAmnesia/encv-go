package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// ─── readAgentConfig 测试 ──────────────────────────────────────

func TestReadAgentConfig_SystemPrompt(t *testing.T) {
	// 准备临时配置文件（含 system_prompt）
	cfgJSON := `{
		"agent_settings": {
			"openai_api_key": "test-key",
			"openai_base_url": "https://api.example.com",
			"system_prompt": "你是一个有帮助的 AI 助手。"
		}
	}`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.user.json")
	if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0644); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}

	s := &Server{configPath: cfgPath}
	cfg := s.readAgentConfig()

	if cfg.SystemPrompt != "你是一个有帮助的 AI 助手。" {
		t.Errorf("SystemPrompt = %q, want %q", cfg.SystemPrompt, "你是一个有帮助的 AI 助手。")
	}
	if cfg.BaseURL != "https://api.example.com" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "https://api.example.com")
	}
}

func TestReadAgentConfig_EmptySystemPrompt(t *testing.T) {
	// system_prompt 字段缺失或为空
	cfgJSON := `{
		"agent_settings": {
			"openai_api_key": "test-key",
			"openai_base_url": "https://api.example.com"
		}
	}`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.user.json")
	os.WriteFile(cfgPath, []byte(cfgJSON), 0644)

	s := &Server{configPath: cfgPath}
	cfg := s.readAgentConfig()

	if cfg.SystemPrompt != "" {
		t.Errorf("SystemPrompt 应为空字符串，得到 %q", cfg.SystemPrompt)
	}
}

func TestReadAgentConfig_NoAgentSettings(t *testing.T) {
	// 完全没有 agent_settings 节
	cfgJSON := `{"some_other": "data"}`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.user.json")
	os.WriteFile(cfgPath, []byte(cfgJSON), 0644)

	s := &Server{configPath: cfgPath}
	cfg := s.readAgentConfig()

	if cfg.SystemPrompt != "" || cfg.APIKey != "" || cfg.BaseURL != "" {
		t.Errorf("无 agent_settings 时应返回零值，got %+v", cfg)
	}
}

func TestReadAgentConfig_DefaultBaseURL(t *testing.T) {
	// base_url 缺失时应 fallback 到 OpenAI 默认值
	cfgJSON := `{
		"agent_settings": {
			"openai_api_key": "test-key",
			"system_prompt": "hello"
		}
	}`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.user.json")
	os.WriteFile(cfgPath, []byte(cfgJSON), 0644)

	s := &Server{configPath: cfgPath}
	cfg := s.readAgentConfig()

	if cfg.BaseURL != "https://api.openai.com" {
		t.Errorf("默认 BaseURL = %q, want https://api.openai.com", cfg.BaseURL)
	}
}

// ─── System Prompt 注入逻辑测试（通过拦截上游请求验证） ──

// recordedRequest 记录 handleAgentChat 发出的上游 HTTP 请求
type recordedRequest struct {
	URL    string            `json:"-"`
	Header map[string][]string `json:"-"`
	Body   map[string]interface{} `json:"body"`
}

// setupChatRouter 创建 gin router 并拦截上游 HTTP 请求用于验证
// 返回 router 和 cleanup 函数（必须在测试结束时调用）
func setupChatRouter(s *Server, recorder *recordedRequest) (*gin.Engine, func()) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	origTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		recorder.URL = req.URL.String()
		recorder.Header = req.Header.Clone()
		json.NewDecoder(req.Body).Decode(&recorder.Body)
		body := `data: {"type":"message_delta","content":"ok"}
data: [DONE]
`
		return &http.Response{
			StatusCode: 200,
			Header:     map[string][]string{"Content-Type": {"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})

	s.registerAgentRoutes(r)

	cleanup := func() { http.DefaultTransport = origTransport }
	return r, cleanup
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestHandleAgentChat_SystemPromptInjected(t *testing.T) {
	// 配置含 system_prompt → 应注入到转发请求的 messages[0]
	cfgJSON := `{
		"agent_settings": {
			"openai_api_key": "sk-test",
			"openai_base_url": "https://api.test.com",
			"system_prompt": "你是文件管理助手。"
		}
	}`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.user.json")
	os.WriteFile(cfgPath, []byte(cfgJSON), 0644)

	s := &Server{configPath: cfgPath}
	var req recordedRequest
	r, cleanup := setupChatRouter(s, &req)
	defer cleanup()

	body := map[string]interface{}{
		"sessionId": "sess-1",
		"model":     "gpt-4o-mini",
		"messages": []map[string]string{
			{"role": "user", "content": "你好"},
		},
	}
	w := httptest.NewRecorder()
	reqJSON, _ := json.Marshal(body)
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/chat", strings.NewReader(string(reqJSON))))

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, body = %s", w.Code, w.Body.String())
	}

	// 验证转发给 OpenAI 的 messages
	messages, ok := req.Body["messages"].([]interface{})
	if !ok {
		t.Fatalf("messages 字段缺失或类型错误，body = %+v", req.Body)
	}
	if len(messages) < 2 {
		t.Fatalf("messages 长度 = %d，期望 >= 2（system + user），messages = %+v", len(messages), messages)
	}

	// messages[0] 必须是 system prompt
	sysMsg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatalf("messages[0] 不是 map，%T", messages[0])
	}
	if sysMsg["role"] != "system" {
		t.Errorf("messages[0].role = %q, want \"system\"", sysMsg["role"])
	}
	if sysMsg["content"] != "你是文件管理助手。" {
		t.Errorf("messages[0].content = %q, want \"你是文件管理助手。\"", sysMsg["content"])
	}

	// messages[1] 必须保持原始 user 消息
	userMsg := messages[1].(map[string]interface{})
	if userMsg["role"] != "user" || userMsg["content"] != "你好" {
		t.Errorf("messages[1] = %+v, want role=user content=你好", userMsg)
	}
}

func TestHandleAgentChat_NoSystemPrompt_WhenEmpty(t *testing.T) {
	// 配置无 system_prompt → 不注入，原样转发
	cfgJSON := `{
		"agent_settings": {
			"openai_api_key": "sk-test",
			"openai_base_url": "https://api.test.com"
		}
	}`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.user.json")
	os.WriteFile(cfgPath, []byte(cfgJSON), 0644)

	s := &Server{configPath: cfgPath}
	var req recordedRequest
	r, cleanup := setupChatRouter(s, &req)
	defer cleanup()

	body := map[string]interface{}{
		"sessionId": "sess-2",
		"model":     "gpt-4o",
		"messages": []map[string]string{
			{"role": "user", "content": "hello"},
			{"role": "assistant", "content": "hi there"},
		},
	}
	w := httptest.NewRecorder()
	reqJSON, _ := json.Marshal(body)
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/chat", strings.NewReader(string(reqJSON))))

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, body = %s", w.Code, w.Body.String())
	}

	messages, _ := req.Body["messages"].([]interface{})
	if len(messages) != 2 {
		t.Fatalf("无 system_prompt 时 messages 长度应为 2，得到 %d", len(messages))
	}
	// 第一条必须是原始 user 消志（不是 system）
	first := messages[0].(map[string]interface{})
	if first["role"] == "system" {
		t.Error("system_prompt 为空时不应该注入 system 消息")
	}
	if first["role"] != "user" {
		t.Errorf("messages[0].role = %q, want \"user\"", first["role"])
	}
}

func TestHandleAgentChat_MultiMessagesWithSystemPrompt(t *testing.T) {
	// 多轮对话 + system_prompt → system 在最前，其余顺序不变
	cfgJSON := `{
		"agent_settings": {
			"openai_api_key": "sk-test",
			"openai_base_url": "https://api.test.com",
			"system_prompt": "只回复 JSON。"
		}
	}`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.user.json")
	os.WriteFile(cfgPath, []byte(cfgJSON), 0644)

	s := &Server{configPath: cfgPath}
	var req recordedRequest
	r, cleanup := setupChatRouter(s, &req)
	defer cleanup()

	body := map[string]interface{}{
		"sessionId": "sess-3",
		"messages": []map[string]string{
			{"role": "user", "content": "第一轮"},
			{"role": "assistant", "content": "回复1"},
			{"role": "user", "content": "第二轮"},
		},
	}
	w := httptest.NewRecorder()
	reqJSON, _ := json.Marshal(body)
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/chat", strings.NewReader(string(reqJSON))))

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, body = %s", w.Code, w.Body.String())
	}

	messages, _ := req.Body["messages"].([]interface{})
	if len(messages) != 4 {
		t.Fatalf("期望 4 条消息（1 system + 3 原始），得到 %d", len(messages))
	}

	roles := make([]string, len(messages))
	for i, m := range messages {
		roles[i] = m.(map[string]interface{})["role"].(string)
	}
	expected := []string{"system", "user", "assistant", "user"}
	for i, got := range roles {
		if got != expected[i] {
			t.Errorf("messages[%d].role = %q, want %q", i, got, expected[i])
		}
	}

	// 验证 system 内容
	sysContent := messages[0].(map[string]interface{})["content"].(string)
	if sysContent != "只回复 JSON。" {
		t.Errorf("system content = %q", sysContent)
	}
}

func TestHandleAgentChat_NoAPIKey_Returns503(t *testing.T) {
	// 无 API Key → 503 错误
	cfgJSON := `{
		"agent_settings": {
			"openai_api_key": "",
			"system_prompt": "test"
		}
	}`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.user.json")
	os.WriteFile(cfgPath, []byte(cfgJSON), 0644)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	s := &Server{configPath: cfgPath}
	s.registerAgentRoutes(r)

	body := `{"messages":[{"role":"user","content":"hi"}]}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/chat", strings.NewReader(body)))

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("无 API Key 期望 503，得到 %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAgentChat_EmptyMessages_Returns400(t *testing.T) {
	cfgJSON := `{
		"agent_settings": {
			"openai_api_key": "sk-test",
			"system_prompt": "test"
		}
	}`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.user.json")
	os.WriteFile(cfgPath, []byte(cfgJSON), 0644)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	s := &Server{configPath: cfgPath}
	s.registerAgentRoutes(r)

	body := `{"messages":[]}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/chat", strings.NewReader(body)))

	if w.Code != http.StatusBadRequest {
		t.Errorf("空消息期望 400，得到 %d: %s", w.Code, w.Body.String())
	}
}
