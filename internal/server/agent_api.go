// internal/server/agent_api.go
package server

import (
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

// deriveKey 使用 passphrase + salt 进行 scrypt 密钥派生。
//
// 关键设计决策（2026-06 修复）：
//   - **完全忽略 deviceId**。早期版本把 deviceId 拼到 salt 后面作为设备绑定，
//     实际产生 2 个致命问题：
//       1. 设备变化 / 浏览器沙箱切换 → deviceId 变 → 已存密文永远解不出
//       2. 历史 Node.js agent-stub 用 scryptSync + 不同 salt 加密的密文，
//          即便有正确 deviceId 也解不开（参数不兼容）
//   - 现在统一只用固定 passphrase + salt 派生，跨设备稳定。
//   - deviceId 仍然作为参数保留（API 兼容 + 给未来可选的"设备绑定"模式留口子），
//     但实际不参与派生。
func deriveKey(deviceId ...string) []byte {
	saltBase := cryptoSalt
	_ = deviceId // 故意忽略，保持签名兼容
	key, err := scrypt.Key(
		[]byte(cryptoPassphrase),
		[]byte(saltBase),
		16384, // N
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

// defaultAgentSystemPrompt 是内置默认 system prompt。
// 当 config.user.json 的 agent_settings.system_prompt 为空或未配置时使用。
//
// 设计原则：
//   - 强制 LLM 先调 list_mounts 发现可用文件系统，禁止凭训练数据编造路径
//   - 明确告知工具能力边界（只读、无写入）
//   - 中文为主（本项目面向国内用户）
const defaultAgentSystemPrompt = `你是 ENCV AI 助手，可以帮助用户浏览文件、管理加密容器和执行操作。

【重要规则 — 违反 = 严重错误】
1. 在回答任何关于"有哪些文件""当前目录有什么"的问题之前，**必须先调用 list_mounts 工具**获取可访问的挂载点列表，再用 list_files 查看具体内容。
2. **绝对禁止编造文件路径或目录结构**。如果未调用工具就不知道有哪些文件，应明确告知用户"我需要先查看文件列表"，而不是猜测 /boot/、/etc/ 等路径。
3. 你只能看到通过 list_mounts 工具返回的挂载点和文件。不要假设任何预置的目录结构。
4. 所有文件操作都是只读的（list_mounts / list_files / read_file / stat_file）。如需修改文件，请告知用户手动操作。

【可用工具】
- list_mounts: 列出当前可访问的文件系统挂载点
- list_files: 列出某个挂载点内的目录内容
- read_file: 读取文本文件内容
- stat_file: 查询文件/目录元信息
- get_storage_info: 查看磁盘空间使用情况
- 加密/解密相关工具（由插件提供）`

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
	r.POST("/api/agent/reset-key", s.handleAgentResetKey)
	r.GET("/api/agent/context-usage", s.handleAgentContextUsage)
	r.GET("/test", s.handleAgentTest)
	r.POST("/test", s.handleAgentTest)
	r.POST("/api/chat", s.handleAgentChat)
	r.POST("/api/confirm", s.handleAgentConfirm)
	r.POST("/api/resume", s.handleAgentResume)

	slog.Info("Agent API routes registered (integrated into encv-go)")
}

// ─── POST /api/agent/reset-key — 清空 config 里的 openai_api_key ──────
//
// 用途：前端在 decrypt-key 持续返回空字符串时（典型场景：deviceId 变了 /
// 历史 Node.js agent-stub 加密的密文与 Go 端 scrypt 不兼容），自动调本端点
// 把 config.user.json 里的 agent_settings.openai_api_key 字段置空，
// 引导用户重新输入一次。
//
// 设计原则：
//   - 不接受任何参数。直接清空，单一职责。
//   - 写盘前加锁防并发。
//   - 写盘后用 slog.Info 留痕（不写 s.cfg 内存——密文不在内存中，运行时
//     每次都从文件 readAgentConfig 读，避免内存/磁盘状态漂移）。
func (s *Server) handleAgentResetKey(c *gin.Context) {
	if s.configPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "config path not available"})
		return
	}

	s.configMu.Lock()
	defer s.configMu.Unlock()

	// 1. 读现有 config
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		slog.Warn("agent: reset-key cannot read config", "path", s.configPath, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot_read_config", "detail": err.Error()})
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		slog.Warn("agent: reset-key cannot parse config", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid_config", "detail": err.Error()})
		return
	}

	// 2. 拿 agent_settings 块
	agentRaw, ok := raw["agent_settings"]
	if !ok {
		// agent_settings 块本就不存在 → 没东西可清，直接成功
		slog.Info("agent: reset-key no-op (agent_settings absent)")
		c.JSON(http.StatusOK, gin.H{"reset": false, "reason": "no agent_settings block"})
		return
	}
	var agent map[string]interface{}
	if err := json.Unmarshal(agentRaw, &agent); err != nil {
		slog.Warn("agent: reset-key cannot parse agent_settings", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid_agent_settings", "detail": err.Error()})
		return
	}

	// 3. 记录原值（仅长度，不打内容，避免日志泄露密文）
	prev, _ := agent["openai_api_key"].(string)
	prevLen := len(prev)

	// 4. 清空
	agent["openai_api_key"] = ""
	newAgent, _ := json.Marshal(agent)
	raw["agent_settings"] = newAgent

	// 5. 写回（保留缩进风格）
	indented, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "marshal_failed", "detail": err.Error()})
		return
	}
	if err := os.WriteFile(s.configPath, append(indented, '\n'), 0644); err != nil {
		slog.Error("agent: reset-key write config failed", "path", s.configPath, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write_failed", "detail": err.Error()})
		return
	}

	slog.Info("agent: reset-key cleared openai_api_key from config", "path", s.configPath, "prev_len", prevLen)
	c.JSON(http.StatusOK, gin.H{
		"reset":   true,
		"prevLen": prevLen,
		"message": "openai_api_key has been cleared. Please re-enter the key in AI Settings.",
	})
}

// ─── GET /api/models — 从供应商获取模型列表 ─────────────────

// buildChatCompletionsURL 拼接 OpenAI 兼容 chat completions 端点 URL。
//
// 关键修复：base URL 已经包含 /v1 后缀时不能重复拼接。
// 之前无条件 + "/v1/chat/completions" 导致 base_url="https://api.openai.com/v1" 时
// 实际请求 URL 变成 "https://api.openai.com/v1/v1/chat/completions" → 上游 404 / EOF。
//
// 规则：
//   - 去掉尾部 /
//   - 去掉已存在的 /v1 后缀（不区分大小写，兼容 https://api.openai.com/V1 等）
//   - 拼接标准 /v1/chat/completions
//
// 用例：
//   "https://api.openai.com/v1"   → "https://api.openai.com/v1/chat/completions"
//   "https://api.openai.com/v1/"  → "https://api.openai.com/v1/chat/completions"
//   "https://api.openai.com"      → "https://api.openai.com/v1/chat/completions"
//   "https://api.openai.com/V1"   → "https://api.openai.com/v1/chat/completions"
//   "https://proxy.example.com/openai/v1" → "https://proxy.example.com/openai/v1/chat/completions"
func buildChatCompletionsURL(baseURL string) string {
	base := strings.TrimRight(baseURL, "/")
	// 去掉已存在的 /v1 后缀（不区分大小写，避免 https://api.openai.com/V1 的边界情况）
	if strings.HasSuffix(strings.ToLower(base), "/v1") {
		base = strings.TrimRight(base[:len(base)-3], "/")
	}
	return base + "/v1/chat/completions"
}

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
	//     配置为空时使用内置默认 prompt（强制 list_mounts + 禁止编造路径）
	finalMessages := body.Messages
	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = defaultAgentSystemPrompt
	}
	finalMessages = make([]chatMsg, 0, len(body.Messages)+1)
	finalMessages = append(finalMessages, chatMsg{Role: "system", Content: systemPrompt})
	finalMessages = append(finalMessages, body.Messages...)

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

	// ════════════════════════════════════════════════════════════
	// 阶段 1: Agent Tool Loop（参照 OpenAI Agents SDK 模式）
	// ════════════════════════════════════════════════════════════
	//
	// 核心循环：非流式调 LLM → 如果返回 tool_calls → 自动执行只读工具
	//           → 注入结果 → 继续循环 → 直到 LLM 返回纯文本
	//
	// 客户端无感知：工具执行完全在服务端完成，客户端只收到最终流式文本。
	// 只有需要用户确认的工具（加密/解密等）才会中断循环并推 approval 事件。
	// ════════════════════════════════════════════════════════════
	const maxAgentLoopRounds = 5
	var (
		loopMessages     = finalMessages // 循环内的 messages（包含 system + 历史对话）
		pendingTools     []toolCallAccumulator
		finalAssistantText string         // LLM 最终文本回复
		autoToolExecuted bool           // 是否有工具被执行过
	)

	for round := 0; round < maxAgentLoopRounds; round++ {
		slog.Info("agent: loop round",
			"round", round+1,
			"max_rounds", maxAgentLoopRounds,
			"messages_count", len(loopMessages),
			"tools_count", len(openAITools))

		resp, err := callOpenAIChatOnce(c.Request.Context(), cfg, model, body.Temperature, loopMessages, openAITools)
		if err != nil {
			slog.Warn("agent: loop request failed", "round", round+1, "error", err)
			// 循环中失败时降级：直接告诉客户端错误
			c.JSON(http.StatusBadGateway, gin.H{"error": "llm_request_failed", "message": err.Error()})
			return
		}

		if resp.Error != nil {
			slog.Warn("agent: loop API error", "round", round+1, "error", resp.Error.Message)
			c.JSON(http.StatusBadRequest, gin.H{"error": "upstream_error", "message": resp.Error.Message})
			return
		}

		if len(resp.Choices) == 0 {
			slog.Warn("agent: loop empty choices", "round", round+1)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "empty_response"})
			return
		}

		choice := resp.Choices[0]
		msg := choice.Message

		// ── 分支 A: LLM 返回了 tool_calls ──
		if len(msg.ToolCalls) > 0 {
			slog.Info("agent: loop tool_calls received",
				"round", round+1,
				"tool_count", len(msg.ToolCalls),
				"finish_reason", choice.FinishReason)

			// 把 assistant 的 tool_calls 消息追加到历史
			loopMessages = append(loopMessages, chatMsg{
				Role:      "assistant",
				Content:   msg.Content,
				ToolCalls: msg.ToolCalls,
			})

			// 逐个处理工具调用
			allAutoExecuted := true
			for _, tc := range msg.ToolCalls {
				// 查工具 meta 判断是否需要确认
				needConfirm := true
				if meta, ok := toolMeta[tc.Function.Name]; ok {
					if v, ok := meta["needConfirm"].(bool); ok {
						needConfirm = v
					}
				}

				if needConfirm {
					// 需要用户确认 → 存入 pending，退出循环
					slog.Info("agent: loop tool needs confirm",
						"name", tc.Function.Name,
						"round", round+1)
					pendingTools = append(pendingTools, tc)
					allAutoExecuted = false
				} else {
					// 只读工具 → 自动执行
					slog.Info("agent: loop auto-executing tool",
						"name", tc.Function.Name,
						"round", round+1)
					start := time.Now()
					result, execErr := s.executeAgentTool(
						c.Request.Context(),
						tc.Function.Name,
						tc.Function.Arguments,
					)
					slog.Info("agent: loop tool executed",
						"name", tc.Function.Name,
						"duration_ms", time.Since(start).Milliseconds(),
						"has_error", execErr != nil)

					if execErr != nil {
						result = fmt.Sprintf(`{"error":"tool_execution_failed","detail":%q}`, execErr.Error())
					}

					// 注入 tool 结果（OpenAI 协议要求 role="tool" + tool_call_id）
					loopMessages = append(loopMessages, chatMsg{
						Role:       "tool",
						Content:    result,
						ToolCallID: tc.ID,
						Name:       tc.Function.Name,
					})
					autoToolExecuted = true
				}
			}

			if !allAutoExecuted {
				// 有工具需要用户确认 → 退出循环，进入阶段 2 推送 approval
				slog.Info("agent: loop exiting — tools need user confirmation",
					"pending_count", len(pendingTools))
				break
			}

			// 所有工具都自动执行了 → 继续下一轮循环（LLM 会基于工具结果继续生成）
			// 追加一个提示消息引导 LLM 输出最终回复
			loopMessages = append(loopMessages, chatMsg{
				Role:    "user",
				Content: "[工具执行结果已注入。请基于以上结果回答用户的原始问题。]",
			})
			continue
		}

		// ── 分支 B: LLM 返回纯文本（无 tool_calls）──
		slog.Info("agent: loop got text response",
			"round", round+1,
			"finish_reason", choice.FinishReason,
			"text_len", len(msg.Content))

		finalAssistantText = msg.Content
		// 把最终 assistant 回复也追加到 session messages
		loopMessages = append(loopMessages, chatMsg{Role: "assistant", Content: msg.Content})
		break
	}

	// 更新 session 的 messages（用于后续 resume/confirm）
	sess.mu.Lock()
	sess.Messages = append([]chatMsg{}, body.Messages...) // 用户原始消息
	// 把循环中产生的所有 assistant/tool 消息也存一份（简化：只存最后几轮的关键内容）
	if len(pendingTools) > 0 {
		sess.PendingTools = pendingTools
	}
	sess.mu.Unlock()

	// ════════════════════════════════════════════════════════════
	// 阶段 2: 流式输出给客户端
	// ════════════════════════════════════════════════════════════
	s.setSSEHeaders(c.Writer)
	defer func() {
		sess.mu.Lock()
		sess.InProgress = false
		sess.mu.Unlock()
	}()

	// 2a: 有待确认工具 → 推送 tool_call 事件 + stream_end
	if len(pendingTools) > 0 {
		for _, tc := range pendingTools {
			s.emitToolCallEvent(sess, c.Writer, flusher, tc, toolMeta)
		}
		s.sendAndCache(sess, c.Writer, flusher, "stream_end", "")
		slog.Info("agent: chat completed (pending approval)",
			"pending_tools", len(pendingTools),
			"auto_executed", autoToolExecuted)
		return
	}

	// 2b: 有最终文本 → 模拟流式输出（从非流式响应转为 SSE chunks）
	if finalAssistantText != "" {
		// 按 chunk 分割模拟打字效果（类似 codex_web 的体验）
		chunkSize := 16
		runes := []rune(finalAssistantText)
		totalChunks := (len(runes) + chunkSize - 1) / chunkSize
		for i := 0; i < len(runes); i += chunkSize {
			end := i + chunkSize
			if end > len(runes) {
				end = len(runes)
			}
			s.sendAndCache(sess, c.Writer, flusher, "text_delta", string(runes[i:end]))
			// 每 10 个 chunk flush 一次，模拟自然打字节奏
			if (i/chunkSize)%10 == 0 {
				time.Sleep(15 * time.Millisecond)
			}
		}
		s.sendAndCache(sess, c.Writer, flusher, "stream_end", "")
		slog.Info("agent: chat completed (text streamed)",
			"chars", len(finalAssistantText),
			"chunks", totalChunks,
			"loop_rounds_executed", autoToolExecuted)
		return
	}

	// 2c: 兜底——不应到达这里
	s.sendSSEEventSafe(c.Writer, flusher, "stream_end", "")
	slog.Warn("agent: chat completed with no output (unexpected)")
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
	// 注入 system prompt（与 handleAgentChat 一致，空配置时用默认）
	finalMessages := sess.Messages
	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = defaultAgentSystemPrompt
	}
	finalMessages = make([]chatMsg, 0, len(sess.Messages)+1)
	finalMessages = append(finalMessages, chatMsg{Role: "system", Content: systemPrompt})
	finalMessages = append(finalMessages, sess.Messages...)
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
