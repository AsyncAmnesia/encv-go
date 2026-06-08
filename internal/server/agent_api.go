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
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/scrypt"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/tools"
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
//     1. 设备变化 / 浏览器沙箱切换 → deviceId 变 → 已存密文永远解不出
//     2. 历史 Node.js agent-stub 用 scryptSync + 不同 salt 加密的密文，
//     即便有正确 deviceId 也解不开（参数不兼容）
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
// ════════════════════════════════════════════════════════════
// 平台级 Tool Use 架构（核心设计决策）
// ════════════════════════════════════════════════════════════
//
// 背景：当前使用的 API 代理（gptgod）会静默丢弃 OpenAI 标准的 `tools` 参数，
// 导致 API 级 Function Calling（tool_calls 字段）永远为空。
//
// 解决方案：在 system prompt 中嵌入完整的工具定义 + 调用协议，
// 让 LLM 以**纯文本 JSON 数组**格式输出工具调用，后端用 extractToolCallsFromText() 解析执行。
//
// 调用循环：
//
//	用户提问 → LLM 输出 [tool_call JSON] → 后端解析+执行 → 注入结果 → LLM 基于结果回答
//
// ⚠️ 修改此常量时必须同步更新 agent_api_test.go 中的相关测试。
const defaultAgentSystemPrompt = `你是 ENCV AI 助手，可以帮助用户浏览文件、管理加密容器和执行操作。

## ═══════════════════════════════════════════════════════
## 工具调用协议（平台级 Tool Use）— 必须严格遵守
## ═══════════════════════════════════════════════════════

### 调用方式

当你需要使用工具时，你的**整个回复必须只包含一个 JSON 数组**，不能有任何其他文字：

[{"name":"工具名","arguments":{"参数1":"值1","参数2":"值2"}}]

一次可以调用多个工具（数组多个元素）。不需要工具时，正常用自然语言回复。

### 完整对话流程示例

用户: 有哪些文件？
助手: [{"name":"list_mounts","arguments":{}}]
系统: [工具结果注入: {"count":2,"items":[{"id":"local","path":"/data"},{"id":"usb","path":"/mnt/usb"}]}]
助手: 当前有 2 个挂载点：
1. local (/data)
2. usb (/mnt/usb)
需要查看哪个目录的内容？

用户: 看 /data 下有什么
助手: [{"name":"list_files","arguments":{"mount_id":"local","rel_path":"/"}}]
系统: [工具结果注入: {"items":[{"name":"doc.pdf","is_dir":false,"size":204800},...]}]
助手: /data 目录下有以下文件：
- doc.pdf (200KB)
- photo.jpg (1.2MB)
- videos/ (目录)

用户: 读一下 doc.pdf 的内容
助手: [{"name":"read_file","arguments":{"mount_id":"local","rel_path":"/doc.pdf"}}]
系统: [工具结果注入: {"content":"%PDF-1.4...","note":"二进制文件，无法显示文本内容"}]
助手: doc.pdf 是一个二进制 PDF 文件，无法直接显示文本内容。如需查看，请用 PDF 解密工具解密后打开。

### 错误恢复示例

用户: 加密这个视频 /data/video.mp4
助手: [{"name":"video_encrypt","arguments":{"input_paths":["/data/video.mp4"],"output_path":"/data/video.enc"}}]
系统: [等待用户确认...]

---

## ═══════════════════════════════════════════════════════
## 可用工具定义（含完整参数 Schema）
## ═══════════════════════════════════════════════════════

### 🔍 文件系统只读工具（自动执行，无需确认）

#### 1. list_mounts — 列出挂载点
参数：无（传空对象 {}）
返回：挂载点列表，每个包含 id 和 path
⚠️ 在回答任何"有哪些文件""什么目录"问题之前，**必须先调用此工具**

#### 2. list_files — 列出目录内容
参数（均为 string 类型，必填）：
- mount_id: 挂载点 ID（从 list_mounts 返回值获取）
- rel_path: 相对路径（根目录用 "/"）
可选参数：
- max_entries: 最大返回条目数（数字字符串，默认 "100"）

#### 3. read_file — 读取文件内容
参数（均为 string 类型，必填）：
- mount_id: 挂载点 ID
- rel_path: 文件相对路径
⚠️ 仅适用于文本文件。二进制文件会返回占位提示。

#### 4. stat_file — 查询文件元信息
参数（均为 string 类型，必填）：
- mount_id: 挂载点 ID
- rel_path: 文件/目录相对路径
返回：大小、修改时间、是否目录、是否容器

#### 5. get_storage_info — 磁盘空间
参数：无（传空对象 {}）
返回：总容量、已用、剩余字节数

### 🔐 加密/解密工具（需要用户确认）

所有加密/解密工具共享相同参数格式：

**加密工具参数（必填）：**
- input_paths: string[] — 要加密的源文件路径数组
- output_path: string — 加密后的输出容器路径

**解密工具参数（必填）：**
- container_path: string — 加密容器文件路径
- output_dir: string — 解密输出目录

可用工具列表：
6. video_encrypt / video_decrypt — 视频（插件名 video）
7. audio_encrypt / audio_decrypt — 音频（插件名 audio）
8. image_encrypt / image_decrypt — 图片（插件名 image）
9. wps_encrypt / wps_decrypt — WPS文档（插件名 wps）
10. pdf_encrypt / pdf_decrypt — PDF文件（插件名 pdf）
11. text_encrypt / text_decrypt — 纯文本（插件名 text）

---

## ═══════════════════════════════════════════════════════
## 强制规则（违反 = 严重错误）
## ═══════════════════════════════════════════════════════

1. **禁止编造文件路径**。未调用 list_mounts/list_files 就不知道有什么文件。如果不知道，明确说"我需要先查看文件列表"，不要猜测。
2. **工具调用的 arguments 必须是有效的 JSON 对象**。不要省略引号、不要用 Python dict 格式。
3. **只读工具会自动执行**，加密/解密工具需要用户确认。你只需输出 JSON，系统会处理其余一切。
4. **绝对不要混合文字和 JSON**。要么纯自然语言，要么纯 JSON 数组。`

// ─── Agent 配置读取 ──────────────────────────────────────────

type agentConfig struct {
	APIKey       string `json:"openai_api_key"`
	BaseURL      string `json:"openai_base_url"`
	SystemPrompt string `json:"system_prompt"`
	OpenAIModel  string `json:"openai_model"`
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
	cfg.OpenAIModel = agent["openai_model"]
	return cfg
}

// resolveActiveModel 解析当前激活模型：优先从 agent_settings 读（用户在 AI 设置中选的），
// 否则默认 gpt-4o。这才是 context-usage 在"无活动 session"时该展示的模型，
// 否则前端会显示 8192 兜底默认值，误导用户。
func (s *Server) resolveActiveModel(deviceId string) string {
	cfg := s.readAgentConfig(deviceId)
	if cfg.OpenAIModel != "" {
		return cfg.OpenAIModel
	}
	return "gpt-4o"
}

// getAgentConfig 读取 config 文件中的 agent_settings 段，并解析为类型化的 *config.Agent。
// 用于访问 MockMode / MockSpeed / MockScenarios 等运行时字段。
// 任何错误（路径为空 / 读不到文件 / JSON 解析失败 / agent_settings 缺失）都返回 DefaultAgentConfig()。
func (s *Server) getAgentConfig() *config.Agent {
	if s.configPath == "" {
		return config.DefaultAgentConfig()
	}
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		slog.Debug("agent: getAgentConfig read file failed", "path", s.configPath, "error", err)
		return config.DefaultAgentConfig()
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		slog.Debug("agent: getAgentConfig parse top-level json failed", "error", err)
		return config.DefaultAgentConfig()
	}
	agentRaw, ok := raw["agent_settings"]
	if !ok {
		return config.DefaultAgentConfig()
	}
	var agentCfg config.Agent
	if err := json.Unmarshal(agentRaw, &agentCfg); err != nil {
		slog.Debug("agent: getAgentConfig parse agent_settings failed", "error", err)
		return config.DefaultAgentConfig()
	}
	// 防御性修正：用户配置文件中可能缺失 mock_speed 字段（JSON 零值为 0）
	// 导致 MockEngine.Run() 中 sleepDelay 把所有 Step 延迟归零，SSE 事件
	// 毫秒级全部推送完毕，前端无法看到逐步流式效果。
	if agentCfg.MockSpeed <= 0 {
		agentCfg.MockSpeed = 1.0
	}
	return &agentCfg
}

// lastUserTextFromLoopMessages 提取 messages 列表中最后一条 role=="user" 消息的文本内容。
// 用于 Mock 模式下的剧本触发匹配（关键词/正则/精确匹配都需要纯文本）。
//
// chatMsg.Content 当前始终是 string 类型；未来若扩展为 []ContentPart 等结构，
// 在此处补充分支即可（参考 OpenAI multimodal 多模态 content array 格式）。
func lastUserTextFromLoopMessages(msgs []chatMsg) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i].Content
		}
	}
	return ""
}

// classifyAgentToolError 从 executeAgentTool 返回的 error 中提取 (code, message)。
//
//   - 如果是 *tools.ToolError → 用其 Code + Message
//   - 否则用 errors.As 透传尝试提取
//   - 兜底：code = "EXEC_FAILED"，message = err.Error()
//
// 给 SSE tool_result 事件的 errorCode / errorMessage 字段用。
func classifyAgentToolError(err error) (code, message string) {
	if err == nil {
		return "", ""
	}
	if te := tools.AsToolError(err); te != nil {
		if te.Code != "" {
			code = te.Code
		} else {
			code = tools.CodeUnknown
		}
		if te.Message != "" {
			message = te.Message
		} else {
			message = err.Error()
		}
		return code, message
	}
	// 兜底：非 ToolError 类型 → 给通用码
	return tools.CodeExecFailed, err.Error()
}

// truncateForLog 把字符串截断到 max 字符，超出部分用 "..." 表示。
// 用于日志中预览用户输入，避免长消息刷屏。
func truncateForLog(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// ─── 路由注册 ────────────────────────────────────────────────

func (s *Server) registerAgentRoutes(r *gin.Engine) {
	r.GET("/api/models", s.handleAgentModels)
	r.POST("/api/encrypt-key", s.handleAgentEncryptKey)
	r.POST("/api/decrypt-key", s.handleAgentDecryptKey)
	r.POST("/api/agent/reset-key", s.handleAgentResetKey)
	r.GET("/api/agent/context-usage", s.handleAgentContextUsage)
	r.GET("/api/agent/mock/presets", s.handleAgentMockPresets)
	r.GET("/test", s.handleAgentTest)
	r.POST("/test", s.handleAgentTest)
	r.POST("/api/chat", s.handleAgentChat)
	r.POST("/api/confirm", s.handleAgentConfirm)
	r.POST("/api/agent/branch-pick", s.handleAgentBranchPick) // 剧本外置 spec：预设选项 chip
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
//
//	"https://api.openai.com/v1"   → "https://api.openai.com/v1/chat/completions"
//	"https://api.openai.com/v1/"  → "https://api.openai.com/v1/chat/completions"
//	"https://api.openai.com"      → "https://api.openai.com/v1/chat/completions"
//	"https://api.openai.com/V1"   → "https://api.openai.com/v1/chat/completions"
//	"https://proxy.example.com/openai/v1" → "https://proxy.example.com/openai/v1/chat/completions"
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
			"models":       []interface{}{},
			"defaultModel": cfg.OpenAIModel,
			"error":        "no_api_key",
			"note":         note,
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
			"error": err.Error(),
			"note":  "无法连接到供应商 API",
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
			"error": fmt.Sprintf("供应商返回非 JSON 响应 (%s)", ct),
			"note":  "无法从供应商获取模型列表",
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
			"error": err.Error(),
			"note":  "解析供应商响应失败",
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
		"models":       sorted,
		"defaultModel": cfg.OpenAIModel,
	})
	slog.Info("agent: models fetched", "count", len(sorted), "base_url", cfg.BaseURL)
}

// ─── POST /api/encrypt-key — 加密 API Key ────────────────────

func (s *Server) handleAgentEncryptKey(c *gin.Context) {
	var body struct {
		Key      string `json:"key"`
		DeviceId string `json:"deviceId"`
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

// chatRequest 是 /api/chat 的请求体（named struct 以便跨函数复用 + 测试 mock）。
//
// 字段：
//   - SessionId / Model / Temperature / Messages / DeviceId  v1 字段
//   - Mode       "start" / "steer" / "queue" / "mock_resume"
//     前端 useAgent.send() 第二个参数透传（Task 11）
//   - Scenario   mock_resume 时携带的当前激活剧本 ID
//     （MockEngineV2 据此找到正确的状态机继续推事件）
type chatRequest struct {
	SessionId   string    `json:"sessionId"`
	Model       string    `json:"model"`
	Temperature float64   `json:"temperature"`
	Messages    []chatMsg `json:"messages"`
	DeviceId    string    `json:"deviceId"`
	Mode        string    `json:"mode,omitempty"`     // start / steer / queue / mock_resume
	Scenario    string    `json:"scenario,omitempty"` // mock_resume 时携带的剧本 ID
}

func (s *Server) handleAgentChat(c *gin.Context) {
	// ① 解析请求体（必须在 WriteHeader 之前）
	var body chatRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json", "detail": err.Error()})
		return
	}

	// ①½ 启动后台 session GC（幂等）
	startSessionGC()

	// A2UI 协议版本识别（预留，本轮不处理）
	if a2v := c.GetHeader("X-A2UI-Version"); a2v != "" {
		slog.Info("agent: A2UI protocol requested", "version", a2v, "session", body.SessionId)
		// 未来：根据 version 选择不同的 Surface 渲染策略
	}

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
		// 跟随用户在 AI 设置中选的激活模型，而不是写死 gpt-4o-mini
		if cfg.OpenAIModel != "" {
			model = cfg.OpenAIModel
		} else {
			model = "gpt-4o"
		}
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

	// ════════════════════════════════════════════════════════════
	// ③⁴ Mock 模式短路（核心测试 / CI / 离线开发路径）
	// ════════════════════════════════════════════════════════════
	//
	// 触发条件：config.user.json 的 agent_settings.mock_mode 设为 "builtin" 或 "custom"。
	//   - builtin 模式无匹配 → Match 内部 fallback 到 default_friendly（不会落到这里）
	//   - custom 模式无匹配 → Match 返回 nil → 继续走真实 OpenAI
	// 短路后完全不调用 OpenAI/gptgod，0 token 消耗。
	//
	// 必须放在 session 缓存之后（需要 sess 写入 EventCache），
	// 必须放在 callOpenAIStream 之前（避免无谓的 API 请求）。
	agentCfg := s.getAgentConfig()
	mockMode := strings.ToLower(strings.TrimSpace(agentCfg.MockMode))

	// ════════════════════════════════════════════════════════════
	// AG-UI 协议模式检测（Phase 4）
	// ════════════════════════════════════════════════════════════
	// 当请求携带 X-Agent-Protocol: agui header 或 ?protocol=agui query 时，
	// 后端使用 AGUIEventMapper 输出标准 AG-UI 格式事件，而非自定义 SSE 格式。
	// 前端 TDesignEngine 通过此协议与后端通信。
	aguiMode := c.GetHeader("X-Agent-Protocol") == "agui" || c.Request.URL.Query().Get("protocol") == "agui"

	// ════════════════════════════════════════════════════════════
	// ③⁴ ⅰ mock_resume 路径（Task 11 / T15 unblock）
	// ════════════════════════════════════════════════════════════
	//
	// 当 body.Mode == "mock_resume" 且 body.Scenario 非空时，前端要求在已
	// 暂停的多轮 / 分支剧本上继续推事件。
	//   - 这里跳过 Match 关键词匹配（避免"开始"等 userText 重新匹配到首轮剧本）
	//   - 用 body.Scenario 在 mockScenariosV2 中查找剧本定义
	//   - 用 per-session v2 engine map 取出 / 创建一个 stateful 引擎
	//   - 调 engine.Resume 让 userText 进入 roundCtx 并推下一轮
	//
	// mock_resume 模式在 mock_mode = "off" 时也允许走（保留路径给未来扩展），
	// 但实际只有 mock_mode != "off" 才有可恢复的 v2 引擎。
	if strings.EqualFold(body.Mode, "mock_resume") && body.Scenario != "" {
		if handled, err := s.handleMockResume(c, body, mockMode, aguiMode); handled {
			if err != nil {
				slog.Warn("agent: mock_resume failed", "scenario", body.Scenario, "error", err)
			}
			return
		}
		// handled=false 表示找不到 v2 剧本 → 落到下方 default 流程（让前端知道是 404）
		c.JSON(http.StatusNotFound, gin.H{
			"error":    "mock_resume_scenario_not_found",
			"scenario": body.Scenario,
			"hint":     "v2 剧本必须存在于 mockScenariosV2 且 mode 为 builtin/custom",
		})
		return
	}

	if mockMode != "" && mockMode != "off" {
		userText := lastUserTextFromLoopMessages(body.Messages)
		scenario := s.mockEngine.Match(userText, mockMode)
		// v2 场景在 mockEngine.builtinScenarios 之外（保持 v1 builtin=12 不变），
		// 这里手动检查并补充
		if scenario == nil {
			for _, sc := range mockScenariosV2 {
				if sc.Rounds > 0 || len(sc.Branches) > 0 {
					if matchScenarioSimple(userText, sc) {
						scenario = sc
						break
					}
				}
			}
		}
		if scenario != nil {
			c.Header("X-Mock-Scenario", scenario.ID)
			c.Header("X-Mock-Mode", mockMode)
			if aguiMode {
				c.Header("X-Agent-Protocol", "agui")
			}

			flusher, _ := c.Writer.(http.Flusher)
			s.setSSEHeaders(c.Writer)
			slog.Info("agent: mock mode short-circuit",
				"mode", mockMode,
				"scenario", scenario.ID,
				"user_text", truncateForLog(userText, 100),
				"speed", agentCfg.MockSpeed,
				"agui_mode", aguiMode)
			// v2 场景（带 Rounds/Branches）走 MockEngineV2 路径
			if scenario.Rounds > 0 || scenario.TotalRounds > 0 || len(scenario.Branches) > 0 {
				v2 := NewMockEngineV2()
				if err := v2.Run(c.Request.Context(), s, sess, c.Writer, flusher, scenario,
					agentCfg.MockSpeed, aguiMode); err != nil {
					slog.Warn("agent: mock v2 engine run failed", "scenario", scenario.ID, "error", err)
				}
			} else {
				if err := s.mockEngine.Run(c.Request.Context(), s, sess, c.Writer, flusher, scenario,
					agentCfg.MockSpeed, true /* mockFlag */, aguiMode); err != nil {
					slog.Warn("agent: mock engine run failed", "scenario", scenario.ID, "error", err)
				}
			}
			sess.mu.Lock()
			sess.InProgress = false
			sess.mu.Unlock()
			return
		}
		// builtin 模式无匹配 → Match 内部已 fallback 到 default_friendly，不会到这里
		// custom 模式无匹配 → Match 返回 nil，落到这里 → 继续走真实 OpenAI
		if mockMode == "custom" {
			slog.Info("agent: custom mock no match, falling through to real API",
				"user_text", truncateForLog(userText, 200))
		}
	}

	// ④ Flusher 检测
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sse_not_supported"})
		return
	}

	// ④½ 提前设置 SSE headers —— 让客户端立即建立连接，
	//     Agent Loop 期间可通过同一连接推送进度事件（thinking / tool_executed）
	s.setSSEHeaders(c.Writer)
	// 初始注释确认 SSE 连接已建立
	c.Writer.Write([]byte(": agent loop starting\n\n"))
	flusher.Flush()

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
		loopMessages       = finalMessages // 循环内的 messages（包含 system + 历史对话）
		pendingTools       []toolCallAccumulator
		finalAssistantText string // LLM 最终文本回复
		autoToolExecuted   bool   // 是否有工具被执行过
		textSeq            int    // 全局 seq 计数器（跨 round 递增，供 text_delta 使用）
		reasoningSeq       int    // 全局 seq 计数器（跨 round 递增，供 reasoning_delta 使用）
	)

	for round := 0; round < maxAgentLoopRounds; round++ {
		slog.Info("agent: loop round",
			"round", round+1,
			"max_rounds", maxAgentLoopRounds,
			"messages_count", len(loopMessages),
			"tools_count", len(openAITools))

		// 推送循环进度事件（客户端可显示"正在思考..."或轮次指示器）
		s.sendSSEEventSafe(c.Writer, flusher, "stream_status", map[string]interface{}{
			"status":  "thinking",
			"round":   round + 1,
			"message": fmt.Sprintf("正在调用 LLM (第 %d/%d 轮)...", round+1, maxAgentLoopRounds),
		})

		// ═════════════════════════════════════════════════════
		// 流式调用 LLM —— 文本实时转发给客户端，tool_calls 累积后处理
		// ═════════════════════════════════════════════════════
		streamCh, err := callOpenAIStream(c.Request.Context(), cfg, model, body.Temperature, loopMessages, openAITools, aguiMode)
		if err != nil {
			slog.Warn("agent: loop stream failed", "round", round+1, "error", err)
			s.sendSSEEventSafe(c.Writer, flusher, "stream_error", map[string]interface{}{
				"code":    "llm_request_failed",
				"message": err.Error(),
				"round":   round + 1,
			})
			s.sendSSEEventSafe(c.Writer, flusher, "stream_end", "")
			return
		}

		// 读取流式事件：累积 tool_calls / 智能缓冲文本
		var (
			roundTextContent string
			roundToolCalls   []toolCallAccumulator
			tcAccumulator    = make(map[int]*toolCallAccumulator)
			finishReason     string
			gotToolCalls     bool

			// ═══ 平台级 Tool Use 智能缓冲 ═══
			// 问题：如果 LLM 输出工具调用 JSON（如 [{"name":"list_mounts",...}]），
			//       之前的代码会通过 text_delta 实时把原始 JSON 推送给用户。
			//       用户看到的是裸 JSON 而非工具执行结果。
			//
			// 解决：前 bufSizeLimit 字符进入缓冲区，检测是否像工具调用 JSON：
			//   - 以 [ 或 { 开头 + 包含 "name" 字段 → 进入"疑似工具调用"模式
			//     → 继续缓冲所有后续文本，不转发给客户端
			//     → 流结束后用 extractToolCallsFromText 解析
			//     → 解析成功 → 执行工具，JSON 永远不暴露给用户
			//     → 解析失败 → 补发所有缓冲的文本（降级为普通文本）
			//   - 不像工具调用 → 立即转发已缓冲的部分 + 切回实时模式
			textBuf           []string // 缓冲的 text_delta chunks
			bufMode           = true   // 是否在缓冲模式（前 N 字符）
			suspectedToolCall = false  // 是否检测到可能是工具调用
		)
		const bufSizeLimit = 60 // 缓冲阈值（字符数）

		// containsEmbeddedToolCallPattern 在任意位置扫描工具调用 JSON 特征。
		//
		// 参考 LobeChat 的协议级分离思路：LobeChat 通过 chunkType='text'/'tools_calling'
		// 在协议层面分离文本和工具调用。由于 gptgod 代理不发送标准 tool_call_chunk 事件，
		// 我们需要在文本层做更精确的启发式检测来模拟同样的效果。
		//
		// 此函数用于：
		//   1) looksLikeToolCheck 策略 2 — 嵌入式检测（中文正文后接 JSON）
		//   2) 实时模式的二次检测 — bufMode 已释放后发现后续 chunk 出现工具调用特征
		containsEmbeddedToolCallPattern := func(s string) bool {
			if len(s) < 20 {
				return false
			}
			return strings.Contains(s, `[{"name"`) ||
				strings.Contains(s, `{"name":"`) ||
				strings.Contains(s, `"function":`) ||
				strings.Contains(s, `"arguments":`)
		}

		// splitTextIntoChunks 将文本按字符数分割为等长大块。
		// 用于 Branch B 成功解析工具调用后，将 remainingText 分块作为 text_delta 补发给前端，
		// 模拟 LobeChat stream_chunk chunkType='text' 的增量推送效果。
		splitTextIntoChunks := func(text string, chunkSize int) []string {
			runes := []rune(text)
			if len(runes) <= chunkSize {
				return []string{text}
			}
			var chunks []string
			for i := 0; i < len(runes); i += chunkSize {
				end := i + chunkSize
				if end > len(runes) {
					end = len(runes)
				}
				chunks = append(chunks, string(runes[i:end]))
			}
			return chunks
		}

		// truncateStr 截断字符串到指定长度（用于日志预览）
		truncateStr := func(s string, maxLen int) string {
			if len(s) <= maxLen {
				return s
			}
			return s[:maxLen] + "..."
		}

		// looksLikeToolCall 检查累积文本是否看起来像工具调用 JSON
		looksLikeToolCall := func(s string) bool {
			trimmed := strings.TrimSpace(s)
			if len(trimmed) < 3 {
				return false
			}
			// 策略 1：文本以 [ 或 { 开头（原有逻辑 — 处理以 JSON 开头的回复）
			if (trimmed[0] == '[' || trimmed[0] == '{') &&
				strings.Contains(trimmed, `"name"`) {
				return true
			}
			// ★ 策略 2：文本任意位置嵌入工具调用 JSON 特征（新增）
			//    处理「中文正文 + 后接工具调用 JSON」的嵌入式场景
			return containsEmbeddedToolCallPattern(trimmed)
		}

		// flushBuffer 把缓冲的文本一次性转发给客户端（当确定不是工具调用时调用）
		// 架构升级：text_delta / reasoning_delta 事件从裸字符串改为 {seq, text} 结构，
		// 前端按 seq 排序渲染，解决 SSE chunk 乱序/丢包问题。
		flushBuffer := func() {
			for _, chunk := range textBuf {
				textSeq++
				s.sendAndCache(sess, c.Writer, flusher, "text_delta",
					map[string]interface{}{"seq": textSeq, "text": chunk})
			}
			textBuf = nil
		}

		for ev := range streamCh {
			switch ev.Type {
			case "text_delta":
				if textChunk, ok := ev.Data.(string); ok && textChunk != "" {
					roundTextContent += textChunk

					if suspectedToolCall {
						// 已确认为疑似工具调用 → 继续缓冲，不转发
						textBuf = append(textBuf, textChunk)
					} else if bufMode && len(roundTextContent) < bufSizeLimit {
						// 缓冲阶段：积累足够样本再判断
						textBuf = append(textBuf, textChunk)
						// 积累到一定量后判断
						if len(roundTextContent) >= bufSizeLimit || looksLikeToolCall(roundTextContent) {
							if looksLikeToolCall(roundTextContent) {
								suspectedToolCall = true
								slog.Info("agent: detected suspected tool call JSON, buffering",
									"prefix_len", len(roundTextContent),
									"preview", roundTextContent[:min(80, len(roundTextContent))])
							} else {
								// 不像工具调用 → 立即释放缓冲区，切回实时模式
								flushBuffer()
								bufMode = false
							}
						}
					} else {
						// 正常实时模式或缓冲已释放
						// ★ 二次检测（参考 LobeChat chunkType 分发思路）：
						//   如果累积文本中出现嵌入式工具调用特征，立即重新进入缓冲模式，
						//   防止工具调用 JSON 作为普通 text_delta 泄漏给前端。
						//   注意：只在 roundTextContent 上做检测（O(n) 字符串搜索），
						//   不对每个 chunk 做（避免性能问题）。
						if !suspectedToolCall && containsEmbeddedToolCallPattern(roundTextContent) {
							slog.Info("agent: mid-stream embedded tool call detected, re-buffering",
								"text_len", len(roundTextContent),
								"chunk_preview", truncateStr(textChunk, 40))
							suspectedToolCall = true
							bufMode = true
							textBuf = append(textBuf, textChunk)
						} else {
							textSeq++
							s.sendAndCache(sess, c.Writer, flusher, "text_delta",
								map[string]interface{}{"seq": textSeq, "text": textChunk})
						}
					}
				}
			case "reasoning_delta":
				if textChunk, ok := ev.Data.(string); ok && textChunk != "" {
					reasoningSeq++
					s.sendAndCache(sess, c.Writer, flusher, "reasoning_delta",
						map[string]interface{}{"seq": reasoningSeq, "text": textChunk})
				}
			case "tool_call_chunk":
				gotToolCalls = true
				tc := ev.Data.(toolCallAccumulator)
				cur, ok := tcAccumulator[tc.Index]
				if !ok {
					cur = &toolCallAccumulator{Index: tc.Index, Type: "function"}
					tcAccumulator[tc.Index] = cur
				}
				if tc.ID != "" {
					cur.ID = tc.ID
				}
				if tc.Type != "" {
					cur.Type = tc.Type
				}
				cur.Function.Name += tc.Function.Name
				cur.Function.Arguments += tc.Function.Arguments
			case "finish_reason":
				if s, ok := ev.Data.(string); ok {
					finishReason = s
				}
			case "stream_end":
				// 正常结束
			}
		}

		// 检测输出被 token 限制截断
		if finishReason == "length" {
			slog.Warn("agent: LLM response truncated (finish_reason=length)",
				"round", round+1,
				"text_len", len(roundTextContent),
				"text_tail", roundTextContent[max(0, len(roundTextContent)-100):])
		}

		// 收集所有累积的完整 tool_calls
		if gotToolCalls {
			for _, tc := range tcAccumulator {
				if tc.Function.Name != "" {
					roundToolCalls = append(roundToolCalls, *tc)
				}
			}
		}

		// ── 分支 A: LLM 返回了 tool_calls ──
		if gotToolCalls && len(roundToolCalls) > 0 {
			slog.Info("agent: loop tool_calls received (stream)",
				"round", round+1,
				"tool_count", len(roundToolCalls),
				"finish_reason", finishReason)

			// ★ 如果有缓冲区残留的前置文本（工具调用之前的正常正文），立即发送。
			// 这些文本在 tool_call_chunk 事件到达之前就已经产生，是安全的自然语言内容。
			// 参考 LobeChat stream_start 重置 accumulatedContent 的思路：
			// 在进入工具调用处理之前，先确保前置文本已正确投递给客户端。
			if len(textBuf) > 0 {
				flushBuffer()
				slog.Info("agent: Branch A flushed pre-tool-call buffered text",
					"buf_chunks", len(textBuf))
				textBuf = nil
			}

			// 把 assistant 的 tool_calls 消息追加到历史
			loopMessages = append(loopMessages, chatMsg{
				Role:      "assistant",
				Content:   roundTextContent,
				ToolCalls: roundToolCalls,
			})

			allAutoExecuted := true
			for _, tc := range roundToolCalls {
				needConfirm := true
				if meta, ok := toolMeta[tc.Function.Name]; ok {
					if v, ok := meta["needConfirm"].(bool); ok {
						needConfirm = v
					}
				}

				// ★ 无论是否需要确认，都向前端推送 tool_call 事件
				// 这样前端才能渲染 GroupedOperationMessage 等结构化组件
				s.emitToolCallEvent(sess, c.Writer, flusher, tc, toolMeta)

				if needConfirm {
					pendingTools = append(pendingTools, tc)
					allAutoExecuted = false
				} else {
					start := time.Now()
					result, execErr := s.executeAgentTool(
						c.Request.Context(), tc.Function.Name, tc.Function.Arguments)
					slog.Info("agent: loop tool executed",
						"name", tc.Function.Name,
						"duration_ms", time.Since(start).Milliseconds(),
						"has_error", execErr != nil)
					s.sendSSEEventSafe(c.Writer, flusher, "stream_status", map[string]interface{}{
						"status":      "tool_executed",
						"tool_name":   tc.Function.Name,
						"round":       round + 1,
						"duration_ms": time.Since(start).Milliseconds(),
					})
					// 推送 tool_status 事件，让前端 GroupedOperationMessage 更新状态徽章
					// 关键改动：异常时也推 tool_status { status: "error" } 而非 success
					// （参考 .trae/specs/mobile-agent-polish-2026q2/spec.md §tool_status 同步）
					statusVal := "success"
					if execErr != nil {
						statusVal = "error"
					}
					s.sendAndCache(sess, c.Writer, flusher, "tool_status", map[string]interface{}{
						"id":     tc.ID,
						"status": statusVal,
					})
					// 推 tool_result 事件（带 isError / errorCode / errorMessage），
					// 让前端 useAgent 在收到此事件时把 tool_call.status 置为 error。
					// （参考 spec §tool_result 事件带 isError 字段）
					if execErr != nil {
						errCode, errMsg := classifyAgentToolError(execErr)
						result = fmt.Sprintf(`{"error":"tool_execution_failed","code":%q,"message":%q,"detail":%q}`,
							errCode, errMsg, execErr.Error())
						s.sendAndCache(sess, c.Writer, flusher, "tool_result", map[string]interface{}{
							"id":           tc.ID,
							"name":         tc.Function.Name,
							"result":       result,
							"isError":      true,
							"status":       "failed",
							"errorCode":    errCode,
							"errorMessage": errMsg,
						})
					} else {
						s.sendAndCache(sess, c.Writer, flusher, "tool_result", map[string]interface{}{
							"id":      tc.ID,
							"name":    tc.Function.Name,
							"result":  result,
							"isError": false,
							"status":  "success",
						})
					}
					loopMessages = append(loopMessages, chatMsg{
						Role: "tool", Content: result,
						ToolCallID: tc.ID, Name: tc.Function.Name,
					})
					autoToolExecuted = true
				}
			}

			if !allAutoExecuted {
				slog.Info("agent: loop exiting — tools need user confirmation",
					"pending_count", len(pendingTools))
				break
			}

			loopMessages = append(loopMessages, chatMsg{
				Role:    "user",
				Content: "[工具执行结果已注入。请基于以上结果回答用户的原始问题。]",
			})
			continue
		}

		// ── 分支 B: LLM 返回了纯文本（无 API 级 tool_calls）──
		//     注意：如果 suspectedToolCall=true，文本可能还在缓冲区中未转发！
		//     需要在 extractToolCallsFromText 结果出来后决定：丢弃（是工具调用）或补发（普通文本）

		// DEBUG: 记录 LLM 实际返回内容（截断到 500 字符），用于诊断工具调用为何不触发
		textPreview := roundTextContent
		if len(textPreview) > 500 {
			textPreview = textPreview[:500] + "...(truncated)"
		}
		slog.Info("agent: loop got text response",
			"round", round+1,
			"finish_reason", finishReason,
			"text_len", len(roundTextContent),
			"text_preview", textPreview,
			"suspected_tool_call", suspectedToolCall,
			"buf_mode", bufMode,
			"buf_len", len(textBuf))
		finalAssistantText = roundTextContent

		// 平台级 Tool Use：尝试从文本中解析工具调用 JSON（应对 API 代理丢弃 tools 参数的情况）
		parsedCalls, remainingText := extractToolCallsFromText(finalAssistantText)
		if len(parsedCalls) > 0 {
			// ★★ 工具调用成功解析 ★★
			// 如果文本在缓冲区中 → 丢弃缓冲区，用户永远看不到原始 JSON
			if suspectedToolCall || bufMode {
				slog.Info("agent: discarding buffered tool call JSON — user will not see raw JSON",
					"buf_size", len(textBuf))
				textBuf = nil // 丢弃缓冲区
				suspectedToolCall = false
				bufMode = false
			}

			slog.Info("agent: loop parsed tool calls from text (platform-level Tool Use) ★★ 工具调用成功解析 ★★",
				"round", round+1,
				"parsed_count", len(parsedCalls),
				"remaining_len", len(remainingText),
				"tool_names", func() (names []string) {
					ns := make([]string, len(parsedCalls))
					for i, c := range parsedCalls {
						ns[i] = c.Name
					}
					return ns
				}())

			accums := parsedToolCallsToAccumulator(parsedCalls)
			loopMessages = append(loopMessages, chatMsg{
				Role: "assistant", Content: finalAssistantText,
			})

			allAutoExecuted := true
			for _, tc := range accums {
				needConfirm := true
				if meta, ok := toolMeta[tc.Function.Name]; ok {
					if v, ok := meta["needConfirm"].(bool); ok {
						needConfirm = v
					}
				}

				// ★ 向前端推送 tool_call 事件（与 API 级 tool_calls 路径一致）
				s.emitToolCallEvent(sess, c.Writer, flusher, tc, toolMeta)

				if needConfirm {
					pendingTools = append(pendingTools, tc)
					allAutoExecuted = false
				} else {
					start := time.Now()
					result, execErr := s.executeAgentTool(
						c.Request.Context(), tc.Function.Name, tc.Function.Arguments)
					slog.Info("agent: loop parsed tool executed",
						"name", tc.Function.Name,
						"duration_ms", time.Since(start).Milliseconds(),
						"has_error", execErr != nil)
					s.sendSSEEventSafe(c.Writer, flusher, "stream_status", map[string]interface{}{
						"status":      "tool_executed",
						"tool_name":   tc.Function.Name,
						"round":       round + 1,
						"duration_ms": time.Since(start).Milliseconds(),
					})
					// 推送 tool_status 事件（平台级 Tool Use 路径）
					// 关键改动：异常时推 tool_status { status: "error" } 而非 success
					// （参考 .trae/specs/mobile-agent-polish-2026q2/spec.md §tool_status 同步）
					parsedStatusVal := "success"
					if execErr != nil {
						parsedStatusVal = "error"
					}
					s.sendAndCache(sess, c.Writer, flusher, "tool_status", map[string]interface{}{
						"id":     tc.ID,
						"status": parsedStatusVal,
					})
					// 推 tool_result 事件（带 isError / errorCode / errorMessage），
					// 让前端 useAgent 在收到此事件时把 tool_call.status 置为 error。
					// （参考 spec §tool_result 事件带 isError 字段）
					if execErr != nil {
						errCode, errMsg := classifyAgentToolError(execErr)
						result = fmt.Sprintf(`{"error":"tool_execution_failed","code":%q,"message":%q,"detail":%q}`,
							errCode, errMsg, execErr.Error())
						s.sendAndCache(sess, c.Writer, flusher, "tool_result", map[string]interface{}{
							"id":           tc.ID,
							"name":         tc.Function.Name,
							"result":       result,
							"isError":      true,
							"status":       "failed",
							"errorCode":    errCode,
							"errorMessage": errMsg,
						})
					} else {
						s.sendAndCache(sess, c.Writer, flusher, "tool_result", map[string]interface{}{
							"id":      tc.ID,
							"name":    tc.Function.Name,
							"result":  result,
							"isError": false,
							"status":  "success",
						})
					}
					loopMessages = append(loopMessages, chatMsg{
						Role: "tool", Content: result,
						ToolCallID: tc.ID, Name: tc.Function.Name,
					})
					autoToolExecuted = true
				}
			}
			if !allAutoExecuted {
				break
			}
			loopMessages = append(loopMessages, chatMsg{
				Role:    "user",
				Content: "[工具执行结果已注入。请基于以上结果回答用户的原始问题。]",
			})

			// ★ 补发 remainingText（参考 LobeChat stream_chunk chunkType='text' 的增量模式）
			// extractToolCallsFromText 返回的 remainingText 是剥离工具调用 JSON 后的
			// 自然语言部分。LobeChat 不需要这个步骤因为它的协议天然分离（chunkType 区分），
			// 但我们的架构决定了必须手动补发，否则 JSON 之后的正文会丢失 → 回答截断。
			if remainingText != "" {
				chunks := splitTextIntoChunks(remainingText, 100)
				for _, ch := range chunks {
					textSeq++
					s.sendAndCache(sess, c.Writer, flusher, "text_delta",
						map[string]interface{}{"seq": textSeq, "text": ch})
				}
				slog.Info("agent: Branch B remaining text sent to client",
					"remaining_len", len(remainingText), "chunks", len(chunks))
			}

			continue
		}
		// 分支 B 也没有解析到工具调用 → LLM 输出了纯文本回复（非工具调用）
		// 如果之前在缓冲模式 → 需要补发缓冲的文本给客户端
		if suspectedToolCall || bufMode {
			slog.Info("agent: flushing buffered text — was suspected tool call but parsing failed",
				"buf_size", len(textBuf))
			flushBuffer()
			suspectedToolCall = false
			bufMode = false
		}

		slog.Info("agent: loop no tool calls found — LLM returned plain text response",
			"round", round+1,
			"auto_tool_executed", autoToolExecuted,
			"text_len", len(finalAssistantText))
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
	// （SSE headers 已在阶段 1 之前设置，无需重复）
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

	// 2b: 有最终文本 → 文本已在 Agent Loop 中通过 streaming 实时发送
	//     这里只需确保 stream_end 被发出（作为安全兜底）
	if finalAssistantText != "" {
		s.sendAndCache(sess, c.Writer, flusher, "stream_end", "")
		slog.Info("agent: chat completed (text streamed in real-time)",
			"chars", len(finalAssistantText),
			"loop_rounds_executed", autoToolExecuted)
		return
	}

	// 2c: 兜底——LLM 未返回任何文本（finalAssistantText 为空）
	//     发送一个 text_delta 提示事件，避免前端显示"服务端返回空回复"
	textSeq++ // fallback 也递增 seq，保持全局唯一
	s.sendSSEEventSafe(c.Writer, flusher, "text_delta",
		map[string]interface{}{"seq": textSeq, "text": "（AI 助手未生成有效回复，可能需要换个问题或检查 API Key 配置）"})
	s.sendAndCache(sess, c.Writer, flusher, "stream_end", "")
	slog.Warn("agent: chat completed with no output (empty finalAssistantText)",
		"rounds", autoToolExecuted)
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

// handleAgentConfirm 处理用户对工具调用的 accept/decline/cancel 决策。
//
// 注意：Mock 模式（agent_settings.mock_mode = "builtin" | "custom"）**不**影响本端点。
// 原因：confirm 只是执行已生成的 tool_call / 递归下一轮 chat，不直接驱动 LLM。
// Mock 短路只发生在 /api/chat 的初始入口；confirm 路径直接走真实 OpenAI 即可。
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

	// ════════════════════════════════════════════════════════════
	// AG-UI 协议模式检测（Phase 2 真实 LLM 路径透传）
	// ════════════════════════════════════════════════════════════
	// 当请求携带 X-Agent-Protocol: agui header 或 ?protocol=agui query 时，
	// 递归 streamChat 输出标准 AG-UI 格式。
	aguiMode := c.GetHeader("X-Agent-Protocol") == "agui" || c.Request.URL.Query().Get("protocol") == "agui"

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
	// 拷贝 tool 引用到本地（在 lock 内完成，避免 unlock 后 dangling）
	toolCopy := *tool
	sess.mu.Unlock()

	// ⑧ accept / accept_for_session → 真实执行；decline → 注入 cancelled 假结果
	var toolMsg chatMsg
	if body.Decision == "accept" || body.Decision == "accept_for_session" {
		// 读取 agent 配置（用 deviceId 派生 API Key 解密 + 系统提示词）
		cfg := s.readAgentConfig(body.DeviceId)
		if cfg.BaseURL == "" {
			cfg.BaseURL = "https://api.openai.com"
		}
		toolMsg, _ = executeAndRecurse(c.Request.Context(), s, sess, cfg, toolCopy)
		// 推 tool_status: completed 给前端
		statusMsg := "completed"
		if body.Decision == "accept_for_session" {
			// 同时记录到 session 授权表
			sess.mu.Lock()
			sess.GrantedTools[toolCopy.Function.Name] = true
			sess.mu.Unlock()
			statusMsg = "completed_granted" // 前端可显示不同文案
		}
		s.sendAndCache(sess, c.Writer, flusher, "tool_status", map[string]interface{}{
			"id":     toolCopy.ID,
			"status": statusMsg,
			"result": toolMsg.Content,
		})
	} else {
		// decline: 构造 cancelled 假结果
		toolMsg = chatMsg{
			Role:       "tool",
			Content:    `{"cancelled": true, "reason": "user_declined"}`,
			ToolCallID: toolCopy.ID,
			Name:       toolCopy.Function.Name,
		}
		s.sendAndCache(sess, c.Writer, flusher, "tool_status", map[string]interface{}{
			"id":     toolCopy.ID,
			"status": "cancelled",
			"result": "user_declined",
		})
	}

	// ⑨ 把 assistant + tool 消息追加到 session.messages
	sess.mu.Lock()
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
	s.streamChat(c.Request.Context(), c, cfg, sess.LastModel, sess.LastTemperature, finalMessages, sess, openAITools, toolMeta, aguiMode)
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

	// ════════════════════════════════════════════════════════════
	// AG-UI 协议模式检测（Phase 2 真实 LLM 路径透传）
	// ════════════════════════════════════════════════════════════
	// resume 路径目前只重放 EventCache，不直接调 streamChat。
	// 但保留 aguiMode 检测以保持接口一致，并供未来重放逻辑参考。
	_ = c.GetHeader("X-Agent-Protocol") == "agui" || c.Request.URL.Query().Get("protocol") == "agui"
	// 注：resume 不走 streamChat，故暂不实际透传 aguiMode。

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
	seq := 0
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		seq++
		s.sendSSEEvent(w, "text_delta", map[string]interface{}{"seq": seq, "text": string(runes[i:end])})
		time.Sleep(delayMs)
	}
}

// streamTextSafe — 安全版本：检测断连后立即停止
func (s *Server) streamTextSafe(w http.ResponseWriter, flusher http.Flusher, text string, chunkSize int, delayMs time.Duration) {
	runes := []rune(text)
	seq := 0
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		seq++
		s.sendSSEEventSafe(w, flusher, "text_delta", map[string]interface{}{"seq": seq, "text": string(runes[i:end])})
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
	Role       string                `json:"role"`
	Content    string                `json:"content"`
	ToolCallID string                `json:"tool_call_id,omitempty"`
	Name       string                `json:"name,omitempty"`
	ToolCalls  []toolCallAccumulator `json:"tool_calls,omitempty"`
}

// handleMockResume 处理前端 send() 第二个参数 {mode: "mock_resume", scenario: ...}。
//
// 流程：
//  1. 用 body.Scenario 在 mockScenariosV2 中查找剧本
//     - 找不到 → 返回 (false, nil) → 调用方回 404
//  2. mock 模式必须启用（mockMode != "off"），否则剧本无法恢复
//     - 关闭时仍允许走（向后兼容日志），但会发警告
//  3. 取出 userText（最后一条 user 消息）
//  4. 准备 SSE writer / flusher / session
//  5. 通过 mockV2SessionEngines 取出 / 创建 stateful MockEngineV2
//  6. 调 Resume(userText) 推下一轮事件
//
// 返回 (handled, err)：
//   - handled=true  → 调方无需进一步处理（已发 SSE / 已回 404）
//   - err != nil     → Resume 内部错误（slog 已记）
func (s *Server) handleMockResume(
	c *gin.Context,
	body chatRequest,
	mockMode string,
	aguiMode bool,
) (bool, error) {
	scenario := lookupMockScenarioV2(body.Scenario)
	if scenario == nil {
		return false, nil
	}
	if mockMode == "off" || mockMode == "" {
		slog.Warn("agent: mock_resume called with mock_mode=off — round state may be lost",
			"scenario", body.Scenario, "session", body.SessionId)
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sse_not_supported"})
		return true, fmt.Errorf("sse not supported")
	}

	// 取出 / 创建 session（沿用现有 session 体系，断点续传 EventCache）
	sessID := body.SessionId
	if sessID == "" {
		sessID = "default"
	}
	sess := getOrCreateSession(sessID)
	sess.mu.Lock()
	sess.InProgress = true
	sess.mu.Unlock()

	// 取出 userText（从最后一条 user 消息）
	userText := lastUserTextFromLoopMessages(body.Messages)

	// 设 SSE header
	c.Header("X-Mock-Scenario", scenario.ID)
	c.Header("X-Mock-Mode", mockMode)
	if aguiMode {
		c.Header("X-Agent-Protocol", "agui")
	}
	s.setSSEHeaders(c.Writer)
	flusher.Flush()

	// 取出 / 创建 stateful v2 引擎
	eng := getOrCreateV2Engine(sessID, scenario)

	slog.Info("agent: mock_resume dispatch",
		"mode", body.Mode,
		"scenario", scenario.ID,
		"session", sessID,
		"user_text", truncateForLog(userText, 100),
		"current_round", eng.CurrentRound(),
		"branch_id", eng.CurrentBranchID())

	// Resume 推进下一轮。Resume 内部：
	//   - 把 userText 写进 roundCtx["user_text"] 和 roundCtx["round_N_user_text"]
	//   - 推 mock_round_state{phase:resumed}
	//   - 推进 round 计数
	//   - 推下一轮 steps
	//   - 若该 step 是 BranchChoice → 推 mock_branch_choice 并等待 PickBranch
	//   - 若该 step 是 PauseForUser → 推 mock_round_state{awaiting_user_input} 并等待 Resume
	//   - 若是最后一轮 → 推 stream_end
	if err := eng.Resume(c.Request.Context(), s, sess, c.Writer, flusher, userText); err != nil {
		sess.mu.Lock()
		sess.InProgress = false
		sess.mu.Unlock()
		return true, fmt.Errorf("Resume: %w", err)
	}
	sess.mu.Lock()
	sess.InProgress = false
	sess.mu.Unlock()
	return true, nil
}

// ─── GET /api/agent/mock/presets — 全局剧本选择器预设 ─────────────
//
// 用途：用户**首次**进入 AgentChat 时（还没发过任何消息），前端主动拉
// 一次本端点拿到"剧本选择器"预设（12 个内置剧本入口），覆盖在输入框
// 上方。后续用户点 chip 触发对应剧本；流结束后 chip 保留（不再自动 clear）。
//
// 行为契约：
//   - mock 模式开启（builtin 或 custom）→ 返回所有 builtin scenarios 的入口
//   - mock 模式关闭（off） → 返回空 presets
//   - 自定义剧本（custom 模式） → builtin + custom 一起返回
//
// 与 /api/chat 流内 mock_presets 事件的差异：
//   - 流内 mock_presets：剧本运行中推，覆盖式更新 chip（mid-scenario 切换分支）
//   - 本端点：仅用于"首次进入" + "用户主动 refresh" 场景
type scenarioPickerEntry struct {
	ID          string `json:"id"`
	ScenarioID  string `json:"scenarioId"`
	Label       string `json:"label"`
	UserText    string `json:"userText"`
	Icon        string `json:"icon,omitempty"`
	Tooltip     string `json:"tooltip,omitempty"`
	Description string `json:"description,omitempty"`
}

func (s *Server) handleAgentMockPresets(c *gin.Context) {
	cfg := s.getAgentConfig()
	mode := cfg.MockMode

	// mock 模式关闭时返回空（前端 v-if 自然不渲染）
	if mode == "off" || mode == "" {
		c.JSON(http.StatusOK, gin.H{
			"scenario": "",
			"phase":    "off",
			"presets":  []scenarioPickerEntry{},
			"mockMode": mode,
		})
		return
	}

	// 遍历所有内置 + 自定义剧本，每个转成一个 picker entry
	allScenarios := s.mockEngine.AllScenarios()
	entries := make([]scenarioPickerEntry, 0, len(allScenarios))

	for _, sc := range allScenarios {
		// 跳过"无 Presets 字段"的剧本（理论上 12 个 builtin 都有）
		if len(sc.Presets) == 0 {
			continue
		}
		// picker 入口：取剧本第一个 Preset 的 userText 作为触发关键词
		firstPreset := sc.Presets[0]
		entries = append(entries, scenarioPickerEntry{
			ID:          "pick_" + sc.ID,
			ScenarioID:  sc.ID,
			Label:       "🎬 " + sc.ID,
			UserText:    firstPreset.UserText,
			Icon:        "🎬",
			Tooltip:     sc.Description,
			Description: sc.Description,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"scenario": "scenario_picker",
		"phase":    "picker",
		"presets":  entries,
		"mockMode": mode,
	})
}
