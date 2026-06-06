// internal/server/agent_api.go
package server

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/scrypt"

	"github.com/gin-gonic/gin"
)

// ─── API Key 加密/解密（防止 config.user.json 明文暴露） ──────
// 使用 AES-256-CBC + scrypt 密钥派生（与 Node.js agent-stub 兼容）
// 存储格式: enc:<base64(iv:ciphertext_with_pkcs7_padding)>

const (
	cryptoPassphrase = "encv-agent-key-v1"
	cryptoSalt       = "encv-mobile-salt-2024"
)

func deriveKey() []byte {
	// 与 Node.js scryptSync(passphrase, salt, 32) 默认参数完全一致：
	//   N=16384(2^14), r=8, p=1, keylen=32
	key, err := scrypt.Key(
		[]byte(cryptoPassphrase),
		[]byte(cryptoSalt),
		16384, // N (与 Node.js 默认值一致)
		8,     // r
		1,     // p
		32,    // keylen (AES-256)
	)
	if err != nil {
		slog.Warn("agent: scrypt key derivation failed", "error", err)
		return make([]byte, 32)
	}
	return key
}

// EncryptApiKey 加密明文 API Key（与 Node.js createCipheriv 'aes-256-cbc' 兼容）
func EncryptApiKey(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	if strings.HasPrefix(plaintext, "enc:") {
		return plaintext // 已加密不重复加密
	}
	key := deriveKey()
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		slog.Warn("agent: failed to generate IV", "error", err)
		return ""
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		slog.Warn("agent: failed to create cipher", "error", err)
		return ""
	}

	// PKCS7 padding
	padded := pkcs7Pad([]byte(plaintext), aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	stream := cipher.NewCBCEncrypter(block, iv)
	stream.CryptBlocks(ciphertext, padded)

	result := append(iv, ciphertext...)
	return "enc:" + base64.StdEncoding.EncodeToString(result)
}

// DecryptApiKey 解密存储的 API Key（与 Node.js 兼容）
// 存储格式: enc:<base64(iv)>:<base64(ciphertext_with_pkcs7)>
func DecryptApiKey(stored string) string {
	if stored == "" {
		return ""
	}
	if !strings.HasPrefix(stored, "enc:") {
		return stored // 未加密的旧格式兼容
	}
	raw := stored[4:]
	// 格式: <base64_iv>:<base64_ciphertext> — 先按 ':' 分割再分别 base64 解码
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		slog.Warn("agent: invalid enc format, expected iv:ciphertext")
		return ""
	}

	iv, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil || len(iv) != aes.BlockSize {
		slog.Warn("agent: decrypt iv base64 failed", "error", err, "len", len(iv))
		return ""
	}

	ct, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil || len(ct) == 0 || len(ct)%aes.BlockSize != 0 {
		slog.Warn("agent: decrypt ct base64 failed", "error", err, "len", len(ct))
		return ""
	}

	key := deriveKey()

	block, err := aes.NewCipher(key)
	if err != nil {
		slog.Warn("agent: decrypt cipher failed", "error", err)
		return ""
	}
	stream := cipher.NewCBCDecrypter(block, iv)
	stream.CryptBlocks(ct, ct)

	// 移除 PKCS7 padding
	pt := pkcs7Unpad(ct)
	if pt == nil {
		slog.Warn("agent: invalid pkcs7 padding")
		return ""
	}
	return string(pt)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	result := make([]byte, len(data)+padding)
	copy(result, data)
	for i := len(data); i < len(result); i++ {
		result[i] = byte(padding)
	}
	return result
}

func pkcs7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	padding := int(data[len(data)-1])
	if padding < 1 || padding > aes.BlockSize || padding > len(data) {
		return nil
	}
	for i := len(data) - padding; i < len(data); i++ {
		if int(data[i]) != padding {
			return nil // 无效的 padding
		}
	}
	return data[:len(data)-padding]
}

// ─── Agent 配置读取 ──────────────────────────────────────────

type agentConfig struct {
	APIKey  string `json:"openai_api_key"`
	BaseURL string `json:"openai_base_url"`
}

func (s *Server) readAgentConfig() agentConfig {
	var cfg agentConfig
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		slog.Warn("agent: cannot read config file", "path", s.configPath, "error", err)
		return cfg
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		slog.Warn("agent: invalid config json", "error", err)
		return cfg
	}
	agentRaw, ok := raw["agent_settings"]
	if !ok {
		return cfg
	}
	var agent map[string]string
	if err := json.Unmarshal(agentRaw, &agent); err != nil {
		return cfg
	}
	cfg.APIKey = DecryptApiKey(agent["openai_api_key"])
	cfg.BaseURL = agent["openai_base_url"]
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com"
	}
	return cfg
}

// ─── 路由注册 ────────────────────────────────────────────────

func (s *Server) registerAgentRoutes(r *gin.Engine) {
	r.GET("/api/models", s.handleAgentModels)
	r.POST("/api/encrypt-key", s.handleAgentEncryptKey)
	r.GET("/test", s.handleAgentTest)
	r.POST("/test", s.handleAgentTest)
	r.POST("/api/chat", s.handleAgentChat)
	r.POST("/api/confirm", s.handleAgentConfirm)
	r.POST("/api/resume", s.handleAgentResume)

	slog.Info("Agent API routes registered (integrated into encv-go)")
}

// ─── GET /api/models — 从供应商获取模型列表 ─────────────────

func (s *Server) handleAgentModels(c *gin.Context) {
	cfg := s.readAgentConfig()

	if cfg.APIKey == "" {
		c.JSON(http.StatusOK, gin.H{
			"models":      []interface{}{},
			"defaultModel": "",
			"error":       "no_api_key",
			"note":        "未配置 OpenAI API Key，请在 AI 设置中填写",
		})
		return
	}

	client := &http.Client{Timeout: 15 * time.Second}
	reqURL := cfg.BaseURL + "/v1/models"
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, reqURL, nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"models": []interface{}{}, "defaultModel": "", "error": err.Error(), "note": "无法构建请求"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("agent: models fetch failed", "error", err)
		c.JSON(http.StatusOK, gin.H{
			"models": []interface{}{}, "defaultModel": "",
			"error":  err.Error(),
			"note":   "无法连接到供应商 API",
		})
		return
	}
	defer resp.Body.Close()

	// 防御：检查 Content-Type，避免 HTML/WAF 页面导致 JSON 解析崩溃
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(ct), "application/json") {
		bodyPreview := make([]byte, 200)
		io.ReadFull(resp.Body, bodyPreview)
		slog.Warn("agent: models response non-JSON", "content_type", ct, "body_preview", string(bodyPreview))
		c.JSON(http.StatusOK, gin.H{
			"models": []interface{}{}, "defaultModel": "",
			"error":  fmt.Sprintf("供应商返回非 JSON 响应 (%s)", ct),
			"note":   "无法从供应商获取模型列表",
		})
		return
	}

	var data struct {
		Data []struct {
			ID      string `json:"id"`
			Created int64  `json:"created"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		slog.Warn("agent: models json decode failed", "error", err)
		c.JSON(http.StatusOK, gin.H{
			"models": []interface{}{}, "defaultModel": "",
			"error":  err.Error(),
			"note":   "解析供应商响应失败",
		})
		return
	}

	var sorted []modelEntry
	for _, m := range data.Data {
		sorted = append(sorted, modelEntry{
			ID: m.ID, Name: m.ID, Provider: detectProvider(m.ID), Created: m.Created,
		})
	}
	sortModels(sorted)

	c.JSON(http.StatusOK, gin.H{
		"models":      sorted,
		"defaultModel": "",
	})
	slog.Info("agent: models fetched", "count", len(sorted), "base_url", cfg.BaseURL)
}

// ─── POST /api/encrypt-key — 加密 API Key ────────────────────

func (s *Server) handleAgentEncryptKey(c *gin.Context) {
	var body struct {
		Key string `json:"key"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	encrypted := EncryptApiKey(body.Key)
	c.JSON(http.StatusOK, gin.H{"encrypted": encrypted})
}

// ─── GET/POST /test — 测试连接 ───────────────────────────────

func (s *Server) handleAgentTest(c *gin.Context) {
	cfg := s.readAgentConfig()

	result := gin.H{
		"openai": "ok",
		"model":  "connected",
		"note":   "agent integrated into encv-go",
	}

	if cfg.APIKey != "" {
		result["model"] = cfg.BaseURL
	} else {
		result["openai"] = "no_key"
		result["note"] = "未配置 API Key"
	}

	c.JSON(http.StatusOK, result)
}

// ─── POST /api/chat — SSE 对话（stub：echo 模式） ─────────────

func (s *Server) handleAgentChat(c *gin.Context) {
	s.setSSEHeaders(c.Writer)

	var body struct {
		SessionId   string    `json:"sessionId"`
		Model       string    `json:"model"`
		Temperature float64   `json:"temperature"`
		Messages    []chatMsg `json:"messages"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		s.sendSSEEvent(c.Writer, "error", gin.H{"message": "invalid_json"})
		return
	}

	userInput := ""
	for i := len(body.Messages) - 1; i >= 0; i-- {
		if body.Messages[i].Role == "user" {
			userInput = body.Messages[i].Content
			break
		}
	}
	if userInput == "" {
		userInput = "(empty)"
	}

	reply := fmt.Sprintf("（encv-go agent）收到消息: %s\n模型: %s | 温度: %.1f", userInput, body.Model, body.Temperature)
	s.streamText(c.Writer, reply, 4, 40)
	s.sendSSEEvent(c.Writer, "stream_end", "")
}

// ─── POST /api/confirm — SSE 工具确认（stub） ────────────────

func (s *Server) handleAgentConfirm(c *gin.Context) {
	s.setSSEHeaders(c.Writer)
	reply := fmt.Sprintf("（encv-go agent stub）已收到决策")
	s.streamText(c.Writer, reply, 3, 40)
	s.sendSSEEvent(c.Writer, "stream_end", "")
}

// ─── POST /api/resume — SSE 断点续传（stub） ─────────────────

func (s *Server) handleAgentResume(c *gin.Context) {
	s.setSSEHeaders(c.Writer)
	reply := "（encv-go agent stub）已恢复 session。真实 agent 将继续流式输出。"
	s.streamText(c.Writer, reply, 3, 50)
	s.sendSSEEvent(c.Writer, "stream_end", "")
}

// ─── SSE 辅助函数 ────────────────────────────────────────────

func (s *Server) setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(": agent ok\n\n"))
	w.(http.Flusher).Flush()
}

func (s *Server) sendSSEEvent(w http.ResponseWriter, eventType string, data interface{}) {
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(data)
	fmt.Fprintf(w, "data: {\"type\": \"%s\", \"data\": %s}\n\n", eventType, buf.String())
	w.(http.Flusher).Flush()
}

func (s *Server) streamText(w http.ResponseWriter, text string, chunkSize int, delayMs time.Duration) {
	runes := []rune(text)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		s.sendSSEEvent(w, "text_delta", string(runes[i:end]))
		time.Sleep(delayMs)
	}
}

// ─── Provider 检测 & 排序 ───────────────────────────────────

type modelEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Created  int64  `json:"created"`
}

func detectProvider(modelId string) string {
	lower := strings.ToLower(modelId)
	switch {
	case strings.HasPrefix(lower, "gpt") || strings.HasPrefix(lower, "o1") ||
		strings.HasPrefix(lower, "o3") || strings.HasPrefix(lower, "o4"):
		return "openai"
	case strings.HasPrefix(lower, "claude"):
		return "anthropic"
	case strings.Contains(lower, "deepseek"):
		return "deepseek"
	case strings.Contains(lower, "qwen"):
		return "qwen"
	case strings.Contains(lower, "gemini"):
		return "google"
	case strings.Contains(lower, "llama") || strings.Contains(lower, "meta"):
		return "meta"
	default:
		return "unknown"
	}
}

func modelSortKey(id string) int {
	lower := strings.ToLower(id)
	switch {
	case strings.HasPrefix(lower, "gpt-4o"):
		return 0
	case strings.HasPrefix(lower, "gpt-4."):
		return 1
	case lower == "gpt-4":
		return 2
	case strings.HasPrefix(lower, "gpt-4"):
		return 3
	case strings.HasPrefix(lower, "o3-mini"):
		return 4
	case strings.HasPrefix(lower, "o3"):
		return 5
	case strings.HasPrefix(lower, "o4"):
		return 6
	case strings.HasPrefix(lower, "gpt-3.5"):
		return 7
	case strings.HasPrefix(lower, "claude-sonnet"):
		return 8
	case strings.HasPrefix(lower, "claude-haiku"):
		return 9
	case strings.HasPrefix(lower, "claude-opus"):
		return 10
	case strings.HasPrefix(lower, "claude"):
		return 11
	case strings.Contains(lower, "deepseek"):
		return 12
	case strings.Contains(lower, "qwen"):
		return 13
	default:
		return 99
	}
}

func sortModels(models []modelEntry) {
	for i := 0; i < len(models)-1; i++ {
		for j := i + 1; j < len(models); j++ {
			pa, pb := modelSortKey(models[i].ID), modelSortKey(models[j].ID)
			if pa > pb || (pa == pb && strings.ToLower(models[i].ID) > strings.ToLower(models[j].ID)) {
				models[i], models[j] = models[j], models[i]
			}
		}
	}
}

// chatMsg 是 chat 请求中的消息格式
type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
