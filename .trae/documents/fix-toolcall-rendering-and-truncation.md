# 真机测试修复计划：回答不完整 + 工具调用未渲染为结构化组件

## 问题现象（真机）

用户提问"有哪些视频文件"，AI 回复：
```
回答"有哪些视频文件"，必须先调用 list_mounts 查看挂载点，然后再查看目录内容。

[{"name":"list_mounts","arguments":{}}] studio_video_1762059800961.mp4 (约 554MB)

其他条目都是子目录...在 /Movies 目录下的可用视频文件有：
```
两个问题：
1. **工具调用以裸 JSON 文本显示**，而非 GroupedOperationMessage 结构化组件
2. **回答截断**——句子未完就结束了

---

## 根因分析

### 问题 1：工具调用 JSON 泄漏到文本输出

**完整事件链路**：

```
LLM 输出 → gptgod 代理 → 后端 SSE 缓冲 → 前端 text_delta → 消息 content → MarkdownStream 渲染
```

后端 [agent_api.go:837-892](internal/server/agent_api.go#L837-L892) 的流处理循环有一个「智能缓冲」机制：

1. **前 60 字符进入缓冲区**（`bufMode=true`），用来检测是否像工具调用 JSON
2. `looksLikeToolCheck()` 检测逻辑（[L812-823](internal/server/agent_api.go#L812-L823)）：
   - 仅检查文本是否 **以 `[` 或 `{` 开头且包含 `"name"`**
3. 如果前 60 字符是正常中文文本（如 `"回答\"有哪些视频文件\"，必须先调用..."`），检测返回 **false**
4. **`flushBuffer()` 被调用，`bufMode=false`，后续所有文本实时转发给客户端**
5. 后续到来的工具调用 JSON `[{"name":"list_mounts","arguments":{}}]` 作为普通 **text_delta** 直接发送到前端！
6. 因为 gptgod 代理可能不发送标准的 `tool_call_chunk` 事件 → `gotToolCalls=false`
7. 流结束后进入 **Branch B**（[L1006-1085](internal/server/agent_api.go#L1006-L1085)）
8. `extractToolCallsFromText()` 从 `roundTextContent` 中成功解析出工具调用 ✅
9. 后端通过 `emitToolCallEvent()` 发送了正确的 `tool_call` SSE 事件 ✅
10. **但裸 JSON 已经作为 text_delta 发给前端了** → 同时存在于消息的 `content` 字段中
11. `renderTurnItems.ts` 渲染时：`assistantText`（含裸 JSON）+ `operationGroup`（结构化组件）同时出现

**关键缺陷**：
- `looksLikeToolCheck()` 只看文本开头，无法检测「**中文正文 + 后接工具调用 JSON**」的模式
- Branch B 成功解析后只丢弃了缓冲区（`textBuf = nil`），但没有处理已经 flush 到客户端的文本
- 没有将 `remainingText`（剥离 JSON 后的自然语言部分）作为修正 text_delta 补发给前端

### 问题 2：回答截断

可能原因（按概率排序）：

**原因 A（最可能）：Branch B 成功路径缺少 remainingText 补发**
- 在 Branch B 的成功路径中（[L1007-1085](internal/server/agent_api.go#L1007-L1085)），`emitToolCallEvent` 发送了 tool_call 事件
- 但 **没有发送 `remainingText`（JSON 之后的部分）作为 text_delta**
- 如果模型的自然语言回复在 JSON 之后（如分析结果），这部分内容丢失

**原因 B：finish_reason=length**
- LLM 达到输出 token 上限，后端仅 warn 不中断（[L895-900](internal/server/agent_api.go#L895-L900)）
- 但如果后续 round 继续时又触发同样问题，累积丢失

**原因 C：第二轮 loop 的响应也被错误缓冲**
- 工具执行后 loop continue，新一轮 LLM 回复
- 如果这轮回复也触发了缓冲区误判，内容再次丢失

---

## 修复方案

### 修复 1：增强工具调用检测 + 补发 remainingText

**文件**: `/workspace/internal/server/agent_api.go`

#### 1a. 增强 looksLikeToolCheck —— 支持嵌入式检测

当前逻辑只检查文本开头。改为同时在累积文本中搜索 `[{"name":` 模式：

```go
// 新增：全量扫描（不仅检查开头）
func containsToolCallPattern(s string) bool {
    return strings.Contains(s, `[{"name"`) ||
           strings.Contains(s, `{"name"`)  // 单对象形式
}

// 修改 looksLikeToolCall：结合开头检测 + 嵌入式检测
looksLikeToolCall := func(s string) bool {
    trimmed := strings.TrimSpace(s)
    if len(trimmed) < 3 { return false }
    // 原有：开头检测
    if (trimmed[0] == '[' || trimmed[0] == '{') &&
        strings.Contains(trimmed, `"name"`) {
        return true
    }
    // ★ 新增：嵌入式检测（中文正文后接 JSON）
    return containsToolCallPattern(trimmed)
}
```

#### 1b. 实时流中发现工具调用模式时立即切换到缓冲模式

在 text_delta 处理的 `else` 分支（正常实时模式，[L862-867](internal/server/agent_api.go#L862-L867)）增加二次检测：

```go
} else {
    // 正常实时模式或缓冲已释放 → 直接转发
    // ★ 二次检测：如果当前 chunk 让整体文本出现工具调用特征，立即重新进入缓冲
    if !suspectedToolCall && containsToolCallPattern(roundTextContent) {
        slog.Info("agent: mid-stream tool call detected, re-entering buffer mode",
            "text_len", len(roundTextContent))
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

#### 1c. Branch B 成功解析后补发 remainingText

在 [L1007-1085](internal/server/agent_api.go#L1007-L1085) Branch B 成功路径末尾、`continue` 之前：

```go
// ★ 补发剩余文本（剥离 JSON 后的自然语言部分）
if remainingText != "" {
    // 将 remainingText 分块发送为 text_delta（与正常流一致）
    for i, ch := range splitIntoChunks(remainingText, 80) {
        textSeq++
        s.sendAndCache(sess, c.Writer, flusher, "text_delta",
            map[string]interface{}{"seq": textSeq, "text": ch})
    }
    slog.Info("agent: Branch B remaining text补发完成",
        "remaining_len", len(remainingText), "chunks", len(splitIntoChunks(remainingText, 80)))
}
```

需要新增辅助函数 `splitIntoChunks(text string, size int) []string`。

#### 1d. Branch A 也需要补发前置文本（如有缓冲遗漏）

在 [L911-983](internal/server/agent_api.go#L911-L983) Branch A 入口处，先 flush 可能残留的缓冲区：

```go
if gotToolCalls && len(roundToolCalls) > 0 {
    // ★ Branch A：如果有缓冲区残留的前置文本（工具调用之前的正文），立即发送
    if len(textBuf) > 0 {
        // 过滤掉可能是工具调用 JSON 的部分，只保留真正的正文
        // 简单策略：如果 roundTextContent 包含工具调用特征，则不 flush（已在 Branch B 处理）
        // 这里是 Branch A，说明 gotToolCalls=true 来自 tool_call_chunk 事件
        // textBuf 中的文本是工具调用之前的正常正文，应该安全地发送
        flushBuffer()
        slog.Info("agent: Branch A flushing pre-tool-call text",
            "buf_chunks", len(textBuf))
    }
    // ... 后续不变
```

### 修复 2：前端安全网 —— 渲染层过滤 content 中的工具调用 JSON

**即使后端修复了，前端也应做防御性处理**：如果 content 中包含工具调用 JSON 格式文本，渲染前应剔除（避免在 GroupedOperationMessage 旁边重复显示裸 JSON）。

**文件**: `/workspace/app/encv-mobile/src/composables/renderTurnItems.ts`

在 assistant 消息的 contentText 推送前（约 [L356](file:///workspace/app/encv-mobile/src/composables/renderTurnItems.ts#L356)），增加过滤：

```typescript
// 如果本条消息有 tool_calls（说明工具调用已被正确解析为结构化数据），
// 则从 content 中剥离工具调用 JSON 避免重复显示
let displayText = contentText
if (msg.tool_calls.length > 0 && contentText) {
    displayText = stripToolCallJSON(contentText)
}
// ... 使用 displayText 替代 contentText
```

新增函数 `stripToolCallJSON(text: string): string`：
- 用正则匹配 `[{"name":"...",...}]` 或 `{"name":"...",...}` 模式
- 从文本中移除这些 JSON 块（可能前后有换行）
- 返回清理后的文本

---

## 改动文件清单

| 文件 | 改动类型 | 内容 |
|------|---------|------|
| `internal/server/agent_api.go` | 修改 | 1a 增强 looksLikeToolCheck + 1b 实时二次检测 + 1c Branch B 补发 remainingText + 1d Branch A flush 残留 |
| `app/encv-mobile/src/composables/renderTurnItems.ts` | 修改 | 前端安全网：stripToolCallJSON 过滤 content 中的裸 JSON |

## 验证步骤

1. `cd /workspace && go build ./cmd/encv` 编译通过
2. `cd /workspace/app/encv-mobile && npx vue-tsc --noEmit` 0 错误
3. 按 start-preview.sh 规范重启服务
4. 真机测试：提问触发工具调用的问题（如"有哪些视频文件"）
5. 验证：
   - 工具调用显示为 GroupedOperationMessage 结构化卡片（无裸 JSON）
   - 回答完整不被截断
   - 正常纯文本回答不受影响（无多余空白/延迟）
