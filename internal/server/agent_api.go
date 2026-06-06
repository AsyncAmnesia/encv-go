// internal/server/agent_api.go
package server

import (
	"bufio"
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
	"strconv"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/scrypt"
)

// ─── API Key 加密/解密（防止 config.user.json 明文暴露） ──────
// 使用 AES-256-CBC + scrypt 密钥派生（与 Node.js agent-stub 兼容）
// 存储格式: enc:<base64(iv:ciphertext_with_pkcs7_padding)>

const (
	cryptoPassphrase = "encv-agent-key-v1"
	cryptoSalt       = "encv-mobile-salt-2024"
)

// deriveKey 使用 passphrase + salt + 可选的设备指纹进行 scrypt 密钥派生
// 设备指纹确保同一密文在不同设备上无法解密（即使知道 passphrase）
func deriveKey(deviceId ...string) []byte {
	// 基础 salt
	saltBase := cryptoSalt
	// 如果提供了设备 ID，追加到 salt 中
	if len(deviceId) > 0 && deviceId[0] != "" {
		saltBase = saltBase + ":" + deviceId[0]
	}
	key, err := scrypt.Key(
		[]byte(cryptoPassphrase),
		[]byte(saltBase),
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

// EncryptApiKey 加密明文 API Key（可选设备指纹加盐）
func EncryptApiKey(plaintext string, deviceId ...string) string {
	if plaintext == "" {
		return ""
	}
	if strings.HasPrefix(plaintext, "enc:") {
		return plaintext // 已加密不重复加密
	}
	key := deriveKey(deviceId...)
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

// DecryptApiKey 解密存储的 API Key（兼容两种格式，可选设备指纹加盐需与加密时一致）
// 格式 A（Node.js agent-stub）: enc:<base64(iv)>:<base64(ciphertext)>
// 格式 B（Go EncryptApiKey）:  enc:<base64(iv||ciphertext)>  （IV 和密文拼接后整体 base64）
func DecryptApiKey(stored string, deviceId ...string) string {
	if stored == "" {
		return ""
	}
	if !strings.HasPrefix(stored, "enc:") {
		return stored // 未加密的旧格式兼容
	}
	raw := stored[4:]

	// ── 尝试格式 A：冒号分隔（Node.js 兼容）────────
	if strings.Contains(raw, ":") {
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) == 2 {
			if result := tryDecryptParts(parts[0], parts[1], deviceId...); result != "" {
				return result
			}
			// 格式 A 解密失败，不返回错误——可能实际是格式 B 中包含冒号的 base64
			// 继续尝试格式 B
		}
	}

	// ── 尝试格式 B：Go 单段 base64（iv || ciphertext 拼接）──
	combined, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(combined) < aes.BlockSize+1 {
		slog.Warn("agent: decrypt format-B base64 failed", "error", err, "len", len(raw))
		return ""
	}
	iv := combined[:aes.BlockSize]
	ct := combined[aes.BlockSize:]
	result := tryDecrypt(iv, ct, deviceId...)
	if result != "" {
		return result
	}

	slog.Warn("agent: all decrypt formats exhausted")
	return ""
}

func tryDecryptParts(ivB64, ctB64 string, deviceId ...string) string {
	iv, err := base64.StdEncoding.DecodeString(ivB64)
	if err != nil || len(iv) != aes.BlockSize {
		return ""
	}
	ct, err := base64.StdEncoding.DecodeString(ctB64)
	if err != nil || len(ct) == 0 {
		return ""
	}
	return tryDecrypt(iv, ct, deviceId...)
}

func tryDecrypt(iv, ct []byte, deviceId ...string) string {
	key := deriveKey(deviceId...)
	block, err := aes.NewCipher(key)
	if err != nil {
		return ""
	}
	stream := cipher.NewCBCDecrypter(block, iv)
	stream.CryptBlocks(ct, ct)
	pt := pkcs7Unpad(ct)
	if pt == nil {
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
	APIKey       string `json:"openai_api_key"`
	BaseURL      string `json:"openai_base_url"`
	SystemPrompt string `json:"system_prompt"`
}

func (s *Server) readAgentConfig(deviceId ...string) agentConfig {
	var cfg agentConfig
	if s.configPath == "" {
		slog.Warn("agent: configPath is empty")
		return cfg
	}
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
		return cfg // agent_settings 不存在不是错误，返回空配置
	}
	var agent map[string]string
	if err := json.Unmarshal(agentRaw, &agent); err != nil {
		slog.Warn("agent: invalid agent_settings json", "error", err)
		return cfg
	}
	cfg.APIKey = DecryptApiKey(agent["openai_api_key"], deviceId...)
	cfg.BaseURL = agent["openai_base_url"]
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com"
	}
	cfg.SystemPrompt = agent["system_prompt"]
	return cfg
}

// ─── 路由注册 ────────────────────────────────────────────────

func (s *Server) registerAgentRoutes(r *gin.Engine) {
	r.GET("/api/models", s.handleAgentModels)
	r.POST("/api/encrypt-key", s.handleAgentEncryptKey)
	r.POST("/api/decrypt-key", s.handleAgentDecryptKey)
	r.GET("/api/agent/context-usage", s.handleAgentContextUsage)
	r.GET("/test", s.handleAgentTest)
	r.POST("/test", s.handleAgentTest)
	r.POST("/api/chat", s.handleAgentChat)
	r.POST("/api/confirm", s.handleAgentConfirm)
	r.POST("/api/resume", s.handleAgentResume)

	slog.Info("Agent API routes registered (integrated into encv-go)")
}

// ─── GET /api/models — 从供应商获取模型列表 ─────────────────

func (s *Server) handleAgentModels(c *gin.Context) {
	// deviceId 必须从 query 取（GET 无 body）
	// 不传 deviceId 会用错的 key 派生，永远解不出设备绑定的密文
	deviceId := c.Query("deviceId")
	cfg := s.readAgentConfig(deviceId)

	if cfg.APIKey == "" {
		// 区分两种失败原因：
		// - 真的没配置（用户从未填过） → note 给空
		// - 配了但 deviceId 不对 → note 给"设备不匹配"
		note := "未配置 OpenAI API Key，请在 AI 设置中填写"
		if deviceId == "" {
			note = "未配置 OpenAI API Key 或缺少 deviceId 参数，请在 AI 设置中填写"
		} else {
			note = "未配置 OpenAI API Key 或 deviceId 不匹配当前设备"
		}
		c.JSON(http.StatusOK, gin.H{
			"models":      []interface{}{},
			"defaultModel": "",
			"error":       "no_api_key",
			"note":        note,
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
		Key       string `json:"key"`
		DeviceId  string `json:"deviceId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	encrypted := EncryptApiKey(body.Key, body.DeviceId)
	c.JSON(http.StatusOK, gin.H{"encrypted": encrypted})
}

// ─── POST /api/decrypt-key — 解密 API Key（前端编辑时回填） ───

func (s *Server) handleAgentDecryptKey(c *gin.Context) {
	var body struct {
		Encrypted string `json:"encrypted"`
		DeviceId  string `json:"deviceId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	decrypted := DecryptApiKey(body.Encrypted, body.DeviceId)
	c.JSON(http.StatusOK, gin.H{"decrypted": decrypted})
}

// ─── GET/POST /test — 测试连接 ───────────────────────────────

func (s *Server) handleAgentTest(c *gin.Context) {
	// deviceId 从 query/header 取（GET 无 body，POST 允许 header 走）
	deviceId := c.Query("deviceId")
	if deviceId == "" {
		deviceId = c.GetHeader("X-Device-Id")
	}
	cfg := s.readAgentConfig(deviceId)

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

// ─── POST /api/chat — SSE 对话（代理到 OpenAI 兼容 API） ──────

func (s *Server) handleAgentChat(c *gin.Context) {
	// ① 解析请求体（必须在 WriteHeader 之前）
	var body struct {
		SessionId   string    `json:"sessionId"`
		Model       string    `json:"model"`
		Temperature float64   `json:"temperature"`
		Messages    []chatMsg `json:"messages"`
		DeviceId    string    `json:"deviceId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json", "detail": err.Error()})
		return
	}

	// ①½ 启动后台 session GC（幂等）
	startSessionGC()

	// ② 读取 agent 配置（API Key / Base URL / System Prompt）
	cfg := s.readAgentConfig(body.DeviceId)
	if cfg.APIKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no_api_key", "message": "未配置 API Key，请在 AI 设置中填写"})
		return
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com"
	}
	model := body.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	// ③ 防御：空消息
	if len(body.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty_messages"})
		return
	}

	// ③½ 注入系统提示词（从配置读取，前端无需关心）
	finalMessages := body.Messages
	if cfg.SystemPrompt != "" {
		finalMessages = make([]chatMsg, 0, len(body.Messages)+1)
		finalMessages = append(finalMessages, chatMsg{Role: "system", Content: cfg.SystemPrompt})
		finalMessages = append(finalMessages, body.Messages...)
	}

	// ③¾ 缓存 session：messages（不含 system）+ model + temperature
	//     —— confirm 时按 sessionId 取回继续对话
	sessID := body.SessionId
	if sessID == "" {
		sessID = "default"
	}
	sess := getOrCreateSession(sessID)
	sess.mu.Lock()
	sess.Messages = append([]chatMsg{}, body.Messages...) // 存用户原始 messages（不含 system）
	sess.LastModel = model
	sess.LastTemperature = body.Temperature
	sess.PendingTools = nil // 新一轮开始，清空旧的 pending
	sess.InProgress = true  // 标记流式生成中
	sess.mu.Unlock()

	// ④ Flusher 检测
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sse_not_supported"})
		return
	}

	// ⑤ 构建 OpenAI 兼容请求
	//
	// 关键：把 agent 工具列表（plugin 加密解密 + fs 只读）发给 LLM。
	// 之前这里没 "tools" 字段，agent 实际根本无法调任何工具——这是让 LLM "perceive
	// the mounted file system" 的真正入口。
	agentTools := s.ListAgentTools()
	openAITools := agentToolsToOpenAITools(agentTools)
	toolMeta := make(map[string]map[string]interface{}, len(agentTools))
	for _, t := range agentTools {
		if n, ok := t["name"].(string); ok {
			toolMeta[n] = t
		}
	}

	reqBody := map[string]interface{}{
		"model":       model,
		"messages":    finalMessages,
		"temperature": body.Temperature,
		"stream":      true,
	}
	if len(openAITools) > 0 {
		reqBody["tools"] = openAITools
		reqBody["tool_choice"] = "auto"
	}
	reqJSON, _ := json.Marshal(reqBody)

	reqURL := strings.TrimRight(cfg.BaseURL, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, reqURL, bytes.NewReader(reqJSON))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "build_request_failed", "message": err.Error()})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	// ⑥ 发起上游请求
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		slog.Warn("agent: chat upstream request failed", "url", reqURL, "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream_error", "message": "无法连接到 AI 服务: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// ⑦ 上游错误处理
	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(resp.Body)
		slog.Warn("agent: chat upstream error", "status", resp.StatusCode, "body", string(errBody))
		// 尝试解析 OpenAI 错误格式
		var openaiErr struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			} `json:"error"`
		}
		json.Unmarshal(errBody, &openaiErr)
		msg := openaiErr.Error.Message
		if msg == "" {
			msg = fmt.Sprintf("AI 服务返回 HTTP %d", resp.StatusCode)
		}
		c.JSON(resp.StatusCode, gin.H{"error": "upstream_error", "message": msg})
		return
	}

	// ⑧ 设置 SSE 响应头并开始流式转发
	s.setSSEHeaders(c.Writer)
	// chat 流结束（或 panic）时清 InProgress
	defer func() {
		sess.mu.Lock()
		sess.InProgress = false
		sess.mu.Unlock()
	}()

	// ⑨ 逐行读取上游 SSE 并转换格式转发给客户端
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 大 buffer 防止长行截断
	hasContent := false

	// 累积 tool_calls（OpenAI 流式分片到达，必须按 index 聚合）
	tcAccum := make(map[int]*toolCallAccumulator)
	var pendingTools []toolCallAccumulator
	streamEnded := false

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		dataStr := strings.TrimPrefix(line, "data: ")
		if dataStr == "[DONE]" {
			// 推 finish 阶段的 tool_call 事件（如果有累积的）
			if len(pendingTools) == 0 {
				for _, tc := range tcAccum {
					pendingTools = append(pendingTools, *tc)
				}
			}
			for _, tc := range pendingTools {
				s.emitToolCallEvent(sess, c.Writer, flusher, tc, toolMeta)
			}
			// 缓存 pending tools 到 session（供 confirm 取用）
			if len(pendingTools) > 0 {
				sess.mu.Lock()
				sess.PendingTools = pendingTools
				sess.mu.Unlock()
			}
			s.sendAndCache(sess, c.Writer, flusher, "stream_end", "")
			streamEnded = true
			break
		}

		// 解析 OpenAI chunk 格式
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string                   `json:"content"`
					Role             string                   `json:"role"`
					ToolCalls        []openaiToolCallChunk    `json:"tool_calls"`
					ReasoningContent string                   `json:"reasoning_content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
			slog.Debug("agent: skip unparseable chunk", "error", err)
			continue
		}

		for _, choice := range chunk.Choices {
			delta := choice.Delta

			// 文本内容 → text_delta
			if delta.Content != "" {
				hasContent = true
				s.sendAndCache(sess, c.Writer, flusher, "text_delta", delta.Content)
			}

			// 推理内容 → reasoning_delta
			if delta.ReasoningContent != "" {
				hasContent = true
				s.sendAndCache(sess, c.Writer, flusher, "reasoning_delta", delta.ReasoningContent)
			}

			// 工具调用 → 按 index 累积
			if len(delta.ToolCalls) > 0 {
				for _, tc := range delta.ToolCalls {
					cur, ok := tcAccum[tc.Index]
					if !ok {
						cur = &toolCallAccumulator{Index: tc.Index, ID: tc.ID, Type: tc.Type}
						tcAccum[tc.Index] = cur
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
				}
			}

			// 结束标记 —— 在 finish_reason 出现时推 tool_call 事件
			if choice.FinishReason != "" {
				// 收集所有累积的 tool_calls
				if len(pendingTools) == 0 {
					for _, tc := range tcAccum {
						pendingTools = append(pendingTools, *tc)
					}
				}
				for _, tc := range pendingTools {
					s.emitToolCallEvent(sess, c.Writer, flusher, tc, toolMeta)
				}
				// 缓存 pending tools
				if len(pendingTools) > 0 {
					sess.mu.Lock()
					sess.PendingTools = pendingTools
					sess.mu.Unlock()
				}
				s.sendAndCache(sess, c.Writer, flusher, "stream_end", "")
				streamEnded = true
			}
		}
	}

	// 兜底：扫描器跑完但没收到 [DONE] / finish_reason
	if !streamEnded {
		if len(pendingTools) == 0 {
			for _, tc := range tcAccum {
				pendingTools = append(pendingTools, *tc)
			}
		}
		for _, tc := range pendingTools {
		s.emitToolCallEvent(sess, c.Writer, flusher, tc, toolMeta)
	}
	if len(pendingTools) > 0 {
		sess.mu.Lock()
		sess.PendingTools = pendingTools
		sess.mu.Unlock()
	}
	s.sendAndCache(sess, c.Writer, flusher, "stream_end", "")
}

	// 如果上游没有发送任何内容（非流式响应等），兜底读取完整 body
	if !hasContent {
		slog.Info("agent: no streaming content from upstream, checking for non-stream response")
		// 已经被 scanner 消费了，无法重读。这里只是安全兜底。
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("agent: sse scanner error", "error", err)
	}

	slog.Info("agent: chat completed", "model", model, "has_content", hasContent, "pending_tools", len(pendingTools))
}

// openaiToolCallChunk 是 OpenAI 流式 tool_calls 单个分片的结构
type openaiToolCallChunk struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// emitToolCallEvent 推送一个完整的 tool_call 事件给前端
//
// toolMeta 是 agent 内部 tool 元数据（name → {needConfirm, kind, ...}），
// 用于正确标记每个 tool 的 needConfirm（fs 工具永远 false，plugin 工具永远 true）
// 和 kind（fs=fileRead, plugin=fileChange）。
func (s *Server) emitToolCallEvent(sess *agentSession, w http.ResponseWriter, flusher http.Flusher, tc toolCallAccumulator, toolMeta map[string]map[string]interface{}) {
	needConfirm := true
	kind := "fileChange"
	if meta, ok := toolMeta[tc.Function.Name]; ok {
		if v, ok := meta["needConfirm"].(bool); ok {
			needConfirm = v
		}
		if v, ok := meta["kind"].(string); ok {
			kind = v
		}
	}
	// F 阶段：检查 session 授权表 → 已授权工具自动放行（auto_run=true）
	autoRun := false
	if needConfirm && sess != nil {
		sess.mu.Lock()
		autoRun = sess.GrantedTools[tc.Function.Name]
		sess.mu.Unlock()
	}

	payload := map[string]interface{}{
		"id":           tc.ID,
		"name":         tc.Function.Name,
		"args":         tc.Function.Arguments,
		"auto_run":     autoRun, // F 阶段：true=前端不弹 ApprovalCard 直接放行
		"needsConfirm": needConfirm && !autoRun,
		"kind":         kind,
	}
	s.sendAndCache(sess, w, flusher, "tool_call", payload)
}

// ─── POST /api/confirm — SSE 工具确认 ───────────────────────

// confirmRequest 是 /api/confirm 的请求体
type confirmRequest struct {
	SessionId  string `json:"sessionId"`
	ToolCallId string `json:"toolCallId"`
	Decision   string `json:"decision"` // accept | decline | cancel | accept_for_session
	DeviceId   string `json:"deviceId"` // 设备指纹（用于 API Key 解密 + 系统提示词）
}

func (s *Server) handleAgentConfirm(c *gin.Context) {
	// ① 解析请求体
	var body confirmRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json", "detail": err.Error()})
		return
	}

	// ② 决策白名单校验
	switch body.Decision {
	case "accept", "decline", "cancel", "accept_for_session":
		// pass
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_decision", "message": "decision 必须是 accept / decline / cancel / accept_for_session"})
		return
	}

	// ③ session 必须存在
	sessID := body.SessionId
	if sessID == "" {
		sessID = "default"
	}
	sessionMu.RLock()
	sess, ok := sessions[sessID]
	sessionMu.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session_not_found", "message": "未找到会话，请重新发起对话"})
		return
	}

	// ④ Flusher 检测
	flusher, okFlusher := c.Writer.(http.Flusher)
	if !okFlusher {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sse_not_supported"})
		return
	}
	s.setSSEHeaders(c.Writer)

	// ⑤ cancel：立即终止，不执行、不递归
	if body.Decision == "cancel" {
		s.sendAndCache(sess, c.Writer, flusher, "stream_end", "")
		return
	}

	// ⑥ accept / decline：必须找到对应的 tool_call
	sess.mu.Lock()
	var tool *toolCallAccumulator
	for i := range sess.PendingTools {
		if sess.PendingTools[i].ID == body.ToolCallId {
			tool = &sess.PendingTools[i]
			break
		}
	}
	if tool == nil {
		sess.mu.Unlock()
		slog.Warn("agent: confirm tool not found", "session", sessID, "toolCallId", body.ToolCallId)
		s.sendAndCache(sess, c.Writer, flusher, "stream_error", "tool_call_not_found: "+body.ToolCallId)
		s.sendAndCache(sess, c.Writer, flusher, "stream_end", "")
		return
	}

	// ⑦ 构造 assistant 消息（带 tool_calls 数组）
	//     OpenAI 协议要求 assistant 消息带 tool_calls 数组，否则 LLM 不知道上轮调用了哪些 tool。
	//     这里把所有 pending tool_calls 都放进去（即使有些用户没确认，也保留引用给 LLM），
	//     对应的 tool 消息只针对用户 confirm 的那一个。
	//     注：toolCallAccumulator.Index 在序列化时已用 omitempty 跳过（OpenAI 协议不需要）。
	allToolCalls := make([]toolCallAccumulator, len(sess.PendingTools))
	copy(allToolCalls, sess.PendingTools)
	assistantMsg := chatMsg{
		Role:      "assistant",
		Content:   "", // assistant 决策后 content 通常为空（被 tool_calls 替代）
		ToolCalls: allToolCalls,
	}

	// ⑧ accept / accept_for_session → 真实执行；decline → 注入 cancelled 假结果
	var toolMsg chatMsg
	if body.Decision == "accept" || body.Decision == "accept_for_session" {
		// 读取 agent 配置（用 deviceId 派生 API Key 解密 + 系统提示词）
		cfg := s.readAgentConfig(body.DeviceId)
		if cfg.BaseURL == "" {
			cfg.BaseURL = "https://api.openai.com"
		}
		toolMsg, _ = executeAndRecurse(c.Request.Context(), s, sess, cfg, *tool)
		// 推 tool_status: completed 给前端
		statusMsg := "completed"
		if body.Decision == "accept_for_session" {
			// 同时记录到 session 授权表
			sess.GrantedTools[tool.Function.Name] = true
			statusMsg = "completed_granted" // 前端可显示不同文案
		}
		s.sendAndCache(sess, c.Writer, flusher, "tool_status", map[string]interface{}{
			"id":     tool.ID,
			"status": statusMsg,
			"result": toolMsg.Content,
		})
	} else {
		// decline: 构造 cancelled 假结果
		toolMsg = chatMsg{
			Role:       "tool",
			Content:    `{"cancelled": true, "reason": "user_declined"}`,
			ToolCallID: tool.ID,
			Name:       tool.Function.Name,
		}
		s.sendAndCache(sess, c.Writer, flusher, "tool_status", map[string]interface{}{
			"id":     tool.ID,
			"status": "cancelled",
			"result": "user_declined",
		})
	}

	// ⑨ 把 assistant + tool 消息追加到 session.messages
	sess.Messages = append(sess.Messages, assistantMsg, toolMsg)
	// 清空 pending tools
	sess.PendingTools = nil
	// 标记递归 chat 进行中（让 resume 能感知）
	sess.InProgress = true
	sess.mu.Unlock()

	// ⑨½ 递归 chat 结束/panic 时清 InProgress
	defer func() {
		sess.mu.Lock()
		sess.InProgress = false
		sess.mu.Unlock()
	}()

	// ⑩ 递归下一轮 chat（流式）
	cfg := s.readAgentConfig(body.DeviceId)
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com"
	}
	// 注入 system prompt（与 handleAgentChat 一致）
	finalMessages := sess.Messages
	if cfg.SystemPrompt != "" {
		finalMessages = make([]chatMsg, 0, len(sess.Messages)+1)
		finalMessages = append(finalMessages, chatMsg{Role: "system", Content: cfg.SystemPrompt})
		finalMessages = append(finalMessages, sess.Messages...)
	}
	// 递归时也要把工具列表塞回去——LLM 可能在后续轮次继续调工具
	agentTools := s.ListAgentTools()
	openAITools := agentToolsToOpenAITools(agentTools)
	toolMeta := make(map[string]map[string]interface{}, len(agentTools))
	for _, t := range agentTools {
		if n, ok := t["name"].(string); ok {
			toolMeta[n] = t
		}
	}
	s.streamChat(c.Request.Context(), c, cfg, sess.LastModel, sess.LastTemperature, finalMessages, sess, openAITools, toolMeta)
}

// ─── POST /api/resume — SSE 断点续传（基于事件缓存重放） ──

func (s *Server) handleAgentResume(c *gin.Context) {
	// ① 解析请求体（lastEventID 从 body 或 SSE standard header Last-Event-ID 取）
	var body struct {
		SessionId   string `json:"sessionId"`
		LastEventID int64  `json:"lastEventId"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.LastEventID == 0 {
		// SSE 协议标准 header：Last-Event-ID（前端用 EventSource 自带支持）
		if hdr := c.GetHeader("Last-Event-ID"); hdr != "" {
			if n, err := strconv.ParseInt(hdr, 10, 64); err == nil {
				body.LastEventID = n
			}
		}
	}

	// ② session 必须存在
	sessID := body.SessionId
	if sessID == "" {
		sessID = "default"
	}
	sessionMu.RLock()
	sess, ok := sessions[sessID]
	sessionMu.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session_not_found", "message": "未找到会话"})
		return
	}

	// ③ Flusher 检测
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sse_not_supported"})
		return
	}
	s.setSSEHeaders(c.Writer)

	// ④ 取 EventCache 副本（释放锁后遍历，避免长时间持锁）
	sess.mu.Lock()
	eventCache := make([]AgentEvent, len(sess.EventCache))
	copy(eventCache, sess.EventCache)
	inProgress := sess.InProgress
	sess.mu.Unlock()

	// ⑤ 找到 lastEventID 之后第一个事件的索引
	startIdx := 0
	for i, e := range eventCache {
		if e.ID > body.LastEventID {
			startIdx = i
			break
		}
		// 如果遍历完没找到（即 lastEventID >= 最后一个事件 ID），保持 startIdx = 0
		// 但这会导致全量重放——更合理的处理：如果 lastEventID == 最后事件 ID，startIdx = len
		if i == len(eventCache)-1 && e.ID <= body.LastEventID {
			startIdx = len(eventCache)
		}
	}

	// ⑥ 如果 lastEventID 已经到最后（startIdx == len），且 in_progress=false（已结束）→ 推 stream_end
	if startIdx >= len(eventCache) {
		if !inProgress {
			s.sendSSEEventSafe(c.Writer, flusher, "stream_end", "")
			return
		}
		// 仍 inProgress 但没有新事件 → 推 status: synced
		s.sendSSEEventSafe(c.Writer, flusher, "stream_status", map[string]interface{}{
			"status":     "synced",
			"inProgress": true,
			"maxEventId": maxEventID(eventCache),
		})
		return
	}

	// ⑦ 重放 startIdx 之后的所有事件
	slog.Info("agent: resume replay events", "session", sessID, "lastEventID", body.LastEventID, "count", len(eventCache)-startIdx)
	for _, e := range eventCache[startIdx:] {
		// SSE 标准：在 data 之前带 `id: <eventID>` 字段
		raw, _ := json.Marshal(e.Data)
		fmt.Fprintf(c.Writer, "id: %d\ndata: {\"type\": \"%s\", \"data\": %s}\n\n", e.ID, e.Type, raw)
	}
	flusher.Flush()

	// ⑧ 如果仍 inProgress，前端稍后再次 resume；如果已完成 → 推 stream_end
	if inProgress {
		s.sendSSEEventSafe(c.Writer, flusher, "stream_status", map[string]interface{}{
			"status":     "more_pending",
			"inProgress": true,
			"maxEventId": maxEventID(eventCache),
		})
	} else {
		// 检查最后一个事件是否是 stream_end
		last := eventCache[len(eventCache)-1]
		if last.Type != "stream_end" {
			s.sendSSEEventSafe(c.Writer, flusher, "stream_end", "")
		}
	}
}

// maxEventID 返回 EventCache 中最大的事件 ID（按 ID 找真正的 max，不依赖顺序）
func maxEventID(events []AgentEvent) int64 {
	if len(events) == 0 {
		return 0
	}
	max := events[0].ID
	for _, e := range events[1:] {
		if e.ID > max {
			max = e.ID
		}
	}
	return max
}

// ─── SSE 辅助函数 ────────────────────────────────────────────

func (s *Server) setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	// 注释：初始 SSE comment 用于确认连接建立
	// 如果写入失败（client 已断开），调用方 Safe 函数会检测到
	_, _ = w.Write([]byte(": agent ok\n\n"))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) sendSSEEvent(w http.ResponseWriter, eventType string, data interface{}) {
	raw, _ := json.Marshal(data)
	fmt.Fprintf(w, "data: {\"type\": \"%s\", \"data\": %s}\n\n", eventType, raw)
	w.(http.Flusher).Flush()
}

// sendSSEEventSafe — 带 client disconnect 检测的安全版本
func (s *Server) sendSSEEventSafe(w http.ResponseWriter, flusher http.Flusher, eventType string, data interface{}) {
	raw, _ := json.Marshal(data) // json.Marshal 不追加 \n（与 json.Encoder.Encode 不同！）
	n, err := fmt.Fprintf(w, "data: {\"type\": \"%s\", \"data\": %s}\n\n", eventType, raw)
	if err != nil || n < 0 {
		slog.Warn("agent: sse write failed (client disconnected?)", "error", err)
		return
	}
	flusher.Flush()
}

// sendAndCache 发送 SSE 事件并同时缓存到 session.EventCache（供 /api/resume 重放）。
// 如果 sess == nil（例如 session 不存在），降级为只发送不缓存。
func (s *Server) sendAndCache(sess *agentSession, w http.ResponseWriter, flusher http.Flusher, eventType string, data interface{}) {
	if sess != nil {
		sess.mu.Lock()
		sess.eventIDCounter++
		eventID := sess.eventIDCounter
		sess.EventCache = append(sess.EventCache, AgentEvent{ID: eventID, Type: eventType, Data: data})
		sess.mu.Unlock()
	}
	s.sendSSEEventSafe(w, flusher, eventType, data)
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

// streamTextSafe — 安全版本：检测断连后立即停止
func (s *Server) streamTextSafe(w http.ResponseWriter, flusher http.Flusher, text string, chunkSize int, delayMs time.Duration) {
	runes := []rune(text)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		s.sendSSEEventSafe(w, flusher, "text_delta", string(runes[i:end]))
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

// chatMsg 是 chat 请求中的消息格式。
// ToolCallID 和 Name 仅在 role=="tool" 时使用（OpenAI tool message 协议要求）。
// ToolCalls 仅在 role=="assistant" 时使用（携带 LLM 决策的工具调用清单，
// 供 confirm 后的下一轮 chat 引用，否则 LLM 不知道上轮调用了哪些 tool）。
type chatMsg struct {
	Role       string                 `json:"role"`
	Content    string                 `json:"content"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
	Name       string                 `json:"name,omitempty"`
	ToolCalls  []toolCallAccumulator  `json:"tool_calls,omitempty"`
}
