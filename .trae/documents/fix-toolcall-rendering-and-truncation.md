# 真机测试修复计划：回答不完整 + 工具调用未渲染为结构化组件

## 一、问题现象

真机提问"有哪些视频文件"，AI 回复：
```
回答"有哪些视频文件"，必须先调用 list_mounts 查看挂载点，然后再查看目录内容。

[{"name":"list_mounts","arguments":{}}] studio_video_1762059800961.mp4 (约 554MB)

其他条目都是子目录...在 /Movies 目录下的可用视频文件有：
```

| # | 症状 | 严重度 |
|---|------|--------|
| 1 | 工具调用以裸 `[{"name":"list_mounts",...}]` JSON 文本显示，而非 GroupedOperationMessage 结构化卡片 | P0 |
| 2 | 回答在句子中间截断（"在 /Movies 目录下的可用视频文件有：" 之后无内容） | P0 |

---

## 二、根因分析（完整事件链路）

### 2.1 架构背景

后端 SSE 流处理循环位于 [agent_api.go:837-892](internal/server/agent_api.go#L837-L892)，采用「智能缓冲」架构：

```
text_delta 到达
    │
    ├─ suspectedToolCall=true ──→ 缓冲（不转发）──→ 流结束后 Branch B 解析
    │
    ├─ bufMode=true 且 <60字符 → 继续缓冲积累样本
    │                              │
    │                              └─ ≥60字符 → looksLikeToolCheck()
    │                                      ├── true → suspectedToolCall=true（继续缓冲）
    │                                      └── false → flushBuffer(), bufMode=false
    │
    └─ bufMode=false ───────────→ 实时转发 text_delta 给前端 ★ 泄漏点
```

### 2.2 问题 1 根因：嵌入式工具调用 JSON 绕过缓冲

**时序还原**：

```
t=0ms   text_delta: "回答\"有哪些视频文件\"，必须先调用 list_mounts 查看挂载点..."
        → bufMode=true, 缓冲中（<60字符）

t=50ms  text_delta: "，然后再查看目录内容。\n\n"
        → roundTextContent 长度 > 60, 触发 looksLikeToolCall()
        → 检测：文本以"回"开头，不以 [ 或 { 开头 → 返回 false
        → flushBuffer()！bufMode=false ★ 缓冲释放

t=100ms text_delta: "[{\"name\":\"list_mounts\",\"arguments\":{}}]"
        → bufMode=false → 直接作为 text_delta 转发给前端 ★★ JSON 泄漏到客户端

t=150ms text_delta: "\n\nstudio_video_1762059800961.mp4 (约 554MB)...\n在 /Movies 目录下的可用视频文件有："
        → bufMode=false → 直接转发

t=200ms stream_end / finish_reason
        → gotToolCalls=false（gptgod 未发 tool_call_chunk 事件）
        → 进入 Branch B [L1006]
        → extractToolCallsFromText(roundTextContent)
          → Strategy 6 嵌入式数组检测成功 ✅
          → 返回 (toolCalls=[list_mounts], remainingText="...在 /Movies 目录下的可用视频文件有：")
        → emitToolCallEvent() 发送 tool_call SSE 事件 ✅
        → 但 remainingText 未发送 ❌
        → 已泄漏的 JSON 也无法撤回 ❌
```

**三个缺陷点**：

| 缺陷 | 位置 | 影响 |
|------|------|------|
| `looksLikeToolCheck()` 只检测文本开头 | [agent_api.go:812](internal/server/agent_api.go#L812) | 嵌入式 JSON（正文后接 JSON）不被识别 |
| `bufMode=false` 后无二次检测 | [agent_api.go:862](internal/server/agent_api.go#L862) | 后续 chunk 中出现 JSON 时继续实时转发 |
| Branch B 不补发 remainingText | [agent_api.go:1007](internal/server/agent_api.go#L1007) | JSON 之后的自然语言文本丢失 → 截断 |

### 2.3 问题 2 根因：remainingText 未补发

`extractToolCallsFromText()` 的 Strategy 6（[agent_tool_loop.go:587-596](internal/server/agent_tool_loop.go#L587-L596)）正确返回了 `remainingText`，但 Branch B 的成功路径只做了：
1. 丢弃缓冲区 (`textBuf = nil`)
2. emitToolCallEvent
3. 执行工具
4. continue 进入下一轮 loop

**从未将 remainingText 作为 text_delta 发送给前端**。导致 JSON 之后的分析性文字全部丢失。

---

## 三、修复方案

### 3.1 后端修复 A：增强缓冲检测 —— 支持嵌入式 + 二次捕获

**文件**: `/workspace/internal/server/agent_api.go`
**影响范围**: 流处理循环内部（[L805-L892](internal/server/agent_api.go#L805-L892)）

#### 3.1.1 新增嵌入式检测函数

在流处理循环之前（约 L805 附近），新增辅助函数：

```go
// containsEmbeddedToolCallPattern 在任意位置扫描工具调用 JSON 特征。
// 用于检测「中文正文 + 后接工具调用 JSON」的嵌入式场景。
func containsEmbeddedToolCallPattern(s string) bool {
	// 快速排除：太短不可能包含完整特征
	if len(s) < 20 {
		return false
	}
	// 检测 OpenAI function calling 格式
	return strings.Contains(s, `[{"name"`) ||
		strings.Contains(s, `{"name":"`) ||
		strings.Contains(s, `"function":`) ||
		strings.Contains(s, `"arguments":`)
}
```

#### 3.1.2 改造 looksLikeToolCheck

将原有的内联闭包改为同时支持开头检测和嵌入式检测：

```go
// 原：只检查开头
looksLikeToolCall := func(s string) bool { ... }

// 新：结合开头 + 嵌入式
looksLikeToolCall := func(s string) bool {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) < 3 { return false }
	// 策略 1：文本以 [ 或 { 开头（原有逻辑）
	if (trimmed[0] == '[' || trimmed[0] == '{') &&
		strings.Contains(trimmed, `"name"`) {
		return true
	}
	// ★ 策略 2：文本任意位置嵌入工具调用 JSON 特征
	return containsEmbeddedToolCallPattern(trimmed)
}
```

#### 3.1.3 实时模式增加二次检测

修改 `else` 分支（正常实时模式，当前 [L862-867](internal/server/agent_api.go#L862-L867)），当已释放缓冲后发现工具调用特征时重新进入缓冲：

```go
} else {
	// 正常实时模式或缓冲已释放
	// ★ 二次检测：如果累积文本中出现嵌入式工具调用特征，
	//   立即重新进入缓冲模式防止 JSON 泄漏
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
```

> **注意**：二次检测只在 `roundTextContent` 上做（O(n) 字符串搜索），不对每个 chunk 做（避免性能问题）。由于 `containsEmbeddedToolCallPattern` 只搜固定子串，Go 的 `strings.Contains` 是高度优化的，对典型回复长度（<10K 字符）无性能影响。

#### 3.1.4 辅助函数 truncateStr

新增（如果项目中不存在类似函数）：

```go
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen { return s }
	return s[:maxLen] + "..."
}
```

### 3.2 后端修复 B：Branch B 成功路径补发 remainingText

**文件**: `/workspace/internal/server/agent_api.go`
**位置**: Branch B 成功路径末尾、`continue` 之前（约 [L1082](internal/server/agent_api.go#L1082) 之后）

在 Branch B 成功执行工具后、continue 进入下一轮 loop 之前，插入：

```go
// ★ 补发剩余文本：extractToolCallsFromText 返回的 remainingText
// 是剥离工具调用 JSON 后的自然语言部分，需要发送给前端显示
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
```

#### 3.2.1 新增 splitTextIntoChunks 辅助函数

在 agent_api.go 顶部 var 区域附近添加：

```go
// splitTextIntoChunks 将文本按字符数分割为等长大块（用于流式补发）
func splitTextIntoChunks(text string, chunkSize int) []string {
	runes := []rune(text)
	if len(runes) <= chunkSize {
		return []string{text}
	}
	var chunks []string
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) { end = len(runes) }
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}
```

### 3.3 后端修复 C：Branch A 处理前置文本残留

**文件**: `/workspace/internal/server/agent_api.go`
**位置**: Branch A 入口处（[L912](internal/server/agent_api.go#L912) 之后）

当 API 级别的 `tool_call_chunk` 事件到达时（gotToolCalls=true），可能已有部分前置文本被缓冲。需要在进入 Branch A 时安全释放：

```go
// ── 分支 A: LLM 返回了 tool_calls ──
if gotToolCalls && len(roundToolCalls) > 0 {
	// ★ 如果有缓冲区残留的前置文本（工具调用之前的正常正文），立即发送
	// 这些文本是安全的——它们在 tool_call_chunk 事件到达之前就已经产生
	if len(textBuf) > 0 {
		flushBuffer() // 会递增 textSeq 并逐块发送
		slog.Info("agent: Branch A flushed pre-tool-call buffered text",
			"buf_chunks", len(textBuf))
		textBuf = nil
	}

	// ... 后续代码不变（loopMessages、emitToolCallEvent 等）
```

### 3.4 前端修复 D：渲染层安全网

**目的**: 即使后端仍有极端情况遗漏，前端渲染时应自动过滤 content 中的工具调用 JSON。

**文件**: `/workspace/app/encv-mobile/src/composables/renderTurnItems.ts`
**位置**: assistant 消息的 content 处理段（[L355-363](file:///workspace/app/encv-mobile/src/composables/renderTurnItems.ts#L355-L363)）

#### 3.4.1 新增 stripToolCallJSON 函数

在 renderTurnItems.ts 文件顶部（import 之后）添加：

```typescript
/**
 * 从消息文本中剥离工具调用 JSON 片段。
 *
 * 当后端的流式缓冲未能完全拦截工具调用 JSON（如嵌入式场景），
 * 裸 JSON 可能混入消息 content 字段。此函数在渲染前清理这些残留，
 * 避免在 GroupedOperationMessage 旁边重复显示原始 JSON。
 *
 * 匹配模式：
 *   [{"name":"xxx","arguments":{...}}]     — OpenAI 格式数组
 *   {"name":"xxx","arguments":{...}}         — 单对象
 *   包含在 ```json ... ``` 代码块中的上述格式
 */
export function stripToolCallJSON(text: string): string {
  if (!text) return text

  let cleaned = text

  // 模式 1: [...{"name":"...",...}] — 数组形式（最常见）
  cleaned = cleaned.replace(
    /\[\s*\{\s*"name"\s*:\s*"[^"]*"(?:\s*,\s*"[^"]*"\s*:\s*(?:\{[^}]*\}|"[^"]*"))*\s*\}\s*\]/g,
    '',
  )

  // 模式 2: {"name":"...",...} — 单对象形式（独立行）
  cleaned = cleaned.replace(
    /^\s*\{\s*"name"\s*:\s*"[^"]*"(?:\s*,\s*"[^"]*"\s*:\s*(?:\{[^}]*\}|"[^"]*"))*\s*\}\s*/gm,
    '',
  )

  // 清理产生的多余空行（连续 2+ 个换行合并为最多 1 个）
  cleaned = cleaned.replace(/\n{3,}/g, '\n\n')

  return cleaned.trim()
}
```

#### 3.4.2 修改 assistant content 渲染逻辑

在 [L355-363](file:///workspace/app/encv-mobile/src/composables/renderTurnItems.ts#L355-L363) 处：

```typescript
// ── 原代码 ──
const contentText = contentToText(msg.content, 'assistant')
if (contentText && contentText.trim().length > 0) {
  out.push({
    type: 'assistantText',
    messageId: `a-${idx}`,
    text: contentText,
    streaming,
  })
}

// ── 新代码 ──
const rawContentText = contentToText(msg.content, 'assistant')
// 安全网：如果本消息已被解析出 tool_calls（说明后端成功识别了工具调用），
// 则从显示文本中剥离可能的工具调用 JSON 残留
const displayText =
  (msg.tool_calls?.length ?? 0) > 0
    ? stripToolCallJSON(rawContentText)
    : rawContentText
if (displayText && displayText.trim().length > 0) {
  out.push({
    type: 'assistantText',
    messageId: `a-${idx}`,
    text: displayText,
    streaming,
  })
}
```

---

## 四、改动清单

| # | 文件 | 改动类型 | 改动内容摘要 |
|---|------|---------|-------------|
| 1 | `internal/server/agent_api.go` | 修改 | 新增 `containsEmbeddedToolCallPattern()` + `splitTextIntoChunks()` + `truncateStr()` 辅助函数 |
| 2 | `internal/server/agent_api.go` | 修改 | 改造 `looksLikeToolCall` 闭包：增加策略 2（嵌入式检测） |
| 3 | `internal/server/agent_api.go` | 修改 | 实时模式 else 分支增加二次检测 + 重入缓冲 |
| 4 | `internal/server/agent_api.go` | 修改 | Branch B 成功路径补发 remainingText（分块 text_delta） |
| 5 | `internal/server/agent_api.go` | 修改 | Branch A 入口处 flush 前置文本残留 |
| 6 | `app/encv-mobile/src/composables/renderTurnItems.ts` | 修改 | 新增 `stripToolCallJSON()` 导出函数 |
| 7 | `app/encv-mobile/src/composables/renderTurnItems.ts` | 修改 | assistant content 渲染前调用 `stripToolCallJSON` 过滤 |

---

## 五、边界情况与防护

| 场景 | 预期行为 | 保证机制 |
|------|---------|---------|
| 纯文本回复（无工具调用） | 正常实时流式输出，零延迟 | `containsEmbeddedToolCallPattern` 对普通文本返回 false，不影响正常路径 |
| 回复以工具调用 JSON 开头 | 被 60 字符窗口内的原有检测拦截 | 策略 1（开头检测）保持不变 |
| 正文 + 多个工具调用 JSON | 全部被拦截，仅 remainingText 显示 | `extractToolCallsFromText` Strategy 6 提取所有匹配数组 |
| gptgod 代理正确发送 tool_call_chunk | 走 Branch A，不受缓冲逻辑影响 | Branch A 有独立的 flush 残留处理 |
| remainingText 本身也包含工具调用（模型异常输出） | 仅发送一次 remainingText，不会无限循环 | Branch B 的 continue 进入新 round，新 round 有独立的缓冲检测 |
| 前端 content 为空但 tool_calls 存在 | `stripToolCallJSON("")` 返回空字符串，不 crash | 函数入口有空值守卫 |
| JSON 残留在 ```json 代码块中 | 正则覆盖代码块内外两种格式 | `stripToolCallJSON` 同时匹配裸 JSON 和代码块内 JSON |

---

## 六、验证步骤

1. **编译验证**
   ```bash
   cd /workspace && go build ./cmd/encv       # Go 0 错误
   cd /workspace/app/encv-mobile && npx vue-tsc --noEmit  # TS 0 错误
   cd /workspace/app/encv-mobile && npx vite build        # Vite 0 错误
   ```

2. **按规范重启**（禁止手动 go build / nohup）
   ```bash
   bash app/encv-mobile/scripts/start-preview.sh
   ```

3. **功能验证（真机）**

   | 测试用例 | 预期结果 |
   |----------|---------|
   | 提问"有哪些视频文件" | 工具调用显示为 GroupedOperationMessage 卡片，无裸 JSON；回答完整不截断 |
   | 提问普通问题（不触发工具） | 正常流式输出，无延迟感 |
   | 提问触发多轮工具调用的问题 | 每轮工具调用都显示为结构化组件；最终回答完整 |
   | 切换不同模型（4o / 4o-mini） | 模型切换生效；各模型下的工具调用均正确渲染 |
