// internal/server/agent_mock.go
//
// Mock LLM 引擎 — 不计费的剧本驱动 SSE 输出。
//
// 核心目标：
//   1. **零成本**：本地生成 SSE 事件流，不调真实 OpenAI，节省 API 费用
//   2. **确定性**：相同输入 → 相同输出，可做 e2e 断言
//   3. **场景覆盖**：内置 12 个剧本覆盖所有代码分支（工具/错误/截断/reasoning/并行/中文/无匹配）
//   4. **UI 0 改动**：mock 输出与真实输出走同一 s.sendAndCache 通道，事件结构字节级一致
//
// 触发匹配优先级：精确匹配 > 关键词 > 正则 > fallback
//   - builtin 模式无匹配 → 返回 default_friendly
//   - custom 模式无匹配 → 返回 nil（让调用方 fallback 到真实 API）
//
// 参考：
//   - Spec: /workspace/.trae/specs/agent-mock-mode/spec.md
//   - 接口契约：agentSession / AgentEvent / sendAndCache（见 agent_api.go）
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ════════════════════════════════════════════════════════════════
// 类型定义
// ════════════════════════════════════════════════════════════════

// MockScenario 定义一个剧本化模拟对话。
// 触发匹配规则按以下优先级（第一个命中即返回）：
//   1. ExactMatch（精确匹配）
//   2. Keywords（任一关键词命中，不区分大小写）
//   3. Regex（正则编译后匹配）
//   4. fallback：ID == "default_friendly"
type MockScenario struct {
	ID          string
	Description string
	ExactMatch  string   // 精确匹配字符串
	Keywords    []string // 关键词匹配（任一命中，不区分大小写）
	Regex       string   // 正则匹配
	Steps       []MockStep
	// Presets 是「剧本激活时」向 UI 推送的快捷输入按钮。
	// 每个预设会被前端渲染为 chip；点击后自动填充或直接发送 userText。
	// 高级剧本可同时利用 mid-scenario 的 mock_presets 事件覆盖/扩展
	// 此初始列表（实现「随剧本进度更新」的多轮会话交互）。
	Presets []MockPreset
}

// MockPreset 是单个预设输入按钮。
//
// ID 在剧本内唯一，用于去重 / 追踪点击。
// Label 显示在 chip 上，UserText 是点击后实际发送给 agent 的文本。
// Icon / Tooltip 可选（前端会安全 fallback）。
type MockPreset struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	UserText string `json:"userText"`
	Icon     string `json:"icon,omitempty"`
	Tooltip  string `json:"tooltip,omitempty"`
}

// MockStep 是一组事件 + 触发前延迟。
//
// 推送语义：
//   - 先按 step 顺序遍历 Steps
//   - 每个 step 内：先 sleep(DelayMs / speed) → 再依次推 Events
//   - 推事件过程中检测 ctx.Done() 立即退出
type MockStep struct {
	DelayMs int
	Events  []MockEvent
}

// MockEvent 是单个 SSE 事件。
//
// Type 取值：stream_start / text_delta / reasoning_delta / tool_call /
// tool_status / tool_result / stream_status / stream_end /
// mid_stream_disconnect / sse_corrupt_chunk
type MockEvent struct {
	Type string                 // 事件类型
	Data map[string]interface{} // 事件 data 载荷
}

// MockEngine 持有激活的剧本集合（builtin + custom）。
//
// 线程安全说明：本类型主要在 handleAgentChat 单请求上下文中使用，
// LoadCustom 在配置变更时调用，调用方需自行加锁（典型用法是 Server 启动时一次）。
type MockEngine struct {
	builtinScenarios []*MockScenario
	customScenarios  []*MockScenario

	// realExecutor 是 tool_call 事件 data 中 execute_real=true 时
	// 实际调用以拿真实结果的回调。典型为 (*Server).executeAgentTool。
	// nil 时 execute_real=true 的 tool_call 仍按剧本硬编码 result 推送
	// （保持单测可用 + 容灾）。
	realExecutor func(ctx context.Context, toolName, argsJSON string) (string, error)
}

// SetRealExecutor 注入真实工具执行器。生产环境（*Server）启动时调用一次，
// 把 s.executeAgentTool 绑进来；单测环境不调用，全部走硬编码剧本。
func (e *MockEngine) SetRealExecutor(fn func(ctx context.Context, toolName, argsJSON string) (string, error)) {
	e.realExecutor = fn
}

// ════════════════════════════════════════════════════════════════
// 构造函数 + 辅助方法
// ════════════════════════════════════════════════════════════════

// NewMockEngine 返回预加载 12 个内置剧本的引擎。
func NewMockEngine() *MockEngine {
	e := &MockEngine{}
	e.builtinScenarios = []*MockScenario{
		// 按优先级排序：精确 > 关键词 > 正则，但同优先级内顺序无影响（Match 按规则重排）
		scenarioDefaultFriendly(),
		scenarioListFilesQuery(),
		scenarioEncryptVideo(),
		scenarioReadAndSummarize(),
		scenarioMultiStepSearch(),
		scenarioStreamingError(),
		scenarioTruncationLongText(),
		scenarioReasoningChain(),
		scenarioToolCallWithArgs(),
		scenarioMultiToolParallel(),
		scenarioContextExhausted(),
		scenarioChineseGreeting(),
	}
	return e
}

// LoadCustom 替换 custom 剧本集合，并对每个剧本做轻量验证。
// 验证失败的剧本 slog.Warn 后跳过，不影响其他剧本加载。
func (e *MockEngine) LoadCustom(custom []MockScenario) {
	validated := make([]*MockScenario, 0, len(custom))
	seenIDs := make(map[string]bool, len(custom))
	for i := range custom {
		sc := custom[i]
		if sc.ID == "" {
			slog.Warn("mock: skip custom scenario (empty id)", "index", i)
			continue
		}
		if seenIDs[sc.ID] {
			slog.Warn("mock: skip custom scenario (duplicate id)", "id", sc.ID)
			continue
		}
		if sc.ExactMatch == "" && len(sc.Keywords) == 0 && sc.Regex == "" {
			slog.Warn("mock: skip custom scenario (no trigger rule)", "id", sc.ID)
			continue
		}
		// 验证 Regex 可编译（失败不 panic，仅 warn 跳过）
		if sc.Regex != "" {
			if _, err := regexp.Compile(sc.Regex); err != nil {
				slog.Warn("mock: skip custom scenario (regex compile failed)",
					"id", sc.ID, "regex", sc.Regex, "error", err)
				continue
			}
		}
		seenIDs[sc.ID] = true
		validated = append(validated, &sc)
	}
	e.customScenarios = validated
	slog.Info("mock: custom scenarios loaded", "count", len(validated))
}

// AllScenarios 返回 builtin + custom 全部剧本（供 inspection / debug）。
func (e *MockEngine) AllScenarios() []*MockScenario {
	out := make([]*MockScenario, 0, len(e.builtinScenarios)+len(e.customScenarios))
	out = append(out, e.builtinScenarios...)
	out = append(out, e.customScenarios...)
	return out
}

// ════════════════════════════════════════════════════════════════
// 匹配逻辑
// ════════════════════════════════════════════════════════════════

// Match 返回最高优先级的匹配剧本。
//
// mode 取值：
//   - "builtin"：先搜 custom → 失败再搜 builtin → 失败返回 default_friendly
//   - "custom"：仅搜 custom → 失败返回 nil（让调用方 fallback 到真实 API）
//   - 其他值：等同于 "builtin"（保守 fallback）
func (e *MockEngine) Match(userText string, mode string) *MockScenario {
	if userText == "" {
		// 空文本：builtin 模式返回 default，custom 模式返回 nil
		return e.fallbackForMode(mode)
	}

	// custom 模式优先（spec：custom 模式仅匹配 custom 剧本）
	if mode == "custom" {
		if sc := matchInList(userText, e.customScenarios); sc != nil {
			return sc
		}
		return nil
	}

	// builtin 模式：custom 也可命中（允许用户覆盖默认剧本）
	if sc := matchInList(userText, e.customScenarios); sc != nil {
		return sc
	}
	if sc := matchInList(userText, e.builtinScenarios); sc != nil {
		return sc
	}
	return e.defaultFriendly()
}

// fallbackForMode 根据 mode 决定空匹配时的 fallback。
func (e *MockEngine) fallbackForMode(mode string) *MockScenario {
	if mode == "custom" {
		return nil
	}
	return e.defaultFriendly()
}

// defaultFriendly 返回内置 default_friendly 剧本（不存在则 panic — 配置错误）。
func (e *MockEngine) defaultFriendly() *MockScenario {
	for _, sc := range e.builtinScenarios {
		if sc.ID == "default_friendly" {
			return sc
		}
	}
	// 不会发生：NewMockEngine 必定包含 default_friendly
	return scenarioDefaultFriendly()
}

// matchInList 在剧本列表中按优先级匹配（精确 > 关键词 > 正则）。
// 找到第一个命中即返回。Regex 编译失败 slog.Warn 后跳过该剧本。
func matchInList(userText string, scenarios []*MockScenario) *MockScenario {
	for _, sc := range scenarios {
		// 1. 精确匹配
		if sc.ExactMatch != "" && userText == sc.ExactMatch {
			return sc
		}
		// 2. 关键词匹配（不区分大小写）
		if len(sc.Keywords) > 0 {
			lower := strings.ToLower(userText)
			for _, kw := range sc.Keywords {
				if kw == "" {
					continue
				}
				if strings.Contains(lower, strings.ToLower(kw)) {
					return sc
				}
			}
		}
		// 3. 正则匹配
		if sc.Regex != "" {
			re, err := regexp.Compile(sc.Regex)
			if err != nil {
				slog.Warn("mock: scenario regex compile failed, skipping",
					"id", sc.ID, "regex", sc.Regex, "error", err)
				continue
			}
			if re.MatchString(userText) {
				return sc
			}
		}
	}
	return nil
}

// ════════════════════════════════════════════════════════════════
// 剧本执行
// ════════════════════════════════════════════════════════════════

// Run 执行剧本：按 step 顺序遍历，每个 step 先 sleep(DelayMs/speed) 再依次推事件。
//
// 参数：
//   - ctx：取消信号（客户端断开 / 请求超时）
//   - s：Server 实例（用于调 sendAndCache）
//   - sess：当前 session（用于 EventCache 断点续传）
//   - w / flusher：SSE writer
//   - scenario：要执行的剧本
//   - speed：速率倍率（1.0=正常，0.1=10x慢，10=10x快，0=零延迟）
//   - mockFlag：是否在第一个 stream_start 事件注入 mock: true 字段
//
// 返回：
//   - 客户端断开 / ctx 取消：返回 ctx.Err()
//   - mid_stream_disconnect 错误类型：推 2 个 text_delta 后返回 nil（关闭 SSE）
//   - 其他错误：返回对应错误
func (e *MockEngine) Run(ctx context.Context, s *Server, sess *agentSession, w http.ResponseWriter, flusher http.Flusher, scenario *MockScenario, speed float64, mockFlag bool) error {
	if scenario == nil {
		return nil
	}

	// 速率归一化：避免 speed 为 0 或负数时除零
	effectiveSpeed := speed
	if effectiveSpeed <= 0 {
		effectiveSpeed = 0 // 表示零延迟（见 sleepDelay 逻辑）
	}

	textSeq := 0
	reasoningSeq := 0
	streamStartEmitted := false

	// pendingRealCalls 记录剧本中声明 execute_real=true 的 tool_call。
	// 键为 call ID，值为 (toolName, argsJSON)。
	// 在后续 step 遇到匹配的 tool_result 时，realExecutor 实际执行，
	// 真实返回值覆盖剧本的硬编码 result。
	pendingRealCalls := make(map[string]pendingRealCall)

	for stepIdx, step := range scenario.Steps {
		// 1. 推 step 前的延迟
		if err := sleepDelay(ctx, step.DelayMs, effectiveSpeed); err != nil {
			slog.Info("mock: ctx cancelled during step delay",
				"scenario", scenario.ID, "step", stepIdx, "error", err)
			return err
		}

		// 2. 推 step 内每个事件
		for evIdx, ev := range step.Events {
			// 中途 ctx 取消检查
			select {
			case <-ctx.Done():
				slog.Info("mock: ctx cancelled mid-step",
					"scenario", scenario.ID, "step", stepIdx, "ev", evIdx)
				return ctx.Err()
			default:
			}

			switch ev.Type {
			case "mid_stream_disconnect":
				// 推 2 个 text_delta 后关闭 SSE（模拟客户端断连）
				textSeq++
				s.sendAndCache(sess, w, flusher, "text_delta",
					map[string]interface{}{"seq": textSeq, "text": "我先说一半..."})
				textSeq++
				s.sendAndCache(sess, w, flusher, "text_delta",
					map[string]interface{}{"seq": textSeq, "text": "嗯"})
				slog.Info("mock: mid_stream_disconnect triggered",
					"scenario", scenario.ID)
				return nil // SSE 由调用方关闭

			case "sse_corrupt_chunk":
				// 推一段非 JSON 的 SSE 数据（验证前端 JSON 解析容错）
				sendCorruptChunk(w)
				slog.Info("mock: sse_corrupt_chunk injected", "scenario", scenario.ID)
				continue

			case "tool_call":
				// 验证必填字段
				name, _ := ev.Data["name"].(string)
				id, _ := ev.Data["id"].(string)
				if name == "" || id == "" {
					slog.Warn("mock: skip tool_call event (missing name or id)",
						"scenario", scenario.ID, "step", stepIdx, "ev", evIdx)
					continue
				}
				// execute_real 字段：true 时把 (id → name+args) 登记到 pendingRealCalls，
				// 后续匹配 tool_result 时实际调 realExecutor 拿真实结果。
				// 缺省 false（保持向后兼容：单测/无 executor 时按硬编码剧本走）。
				if execReal, _ := ev.Data["execute_real"].(bool); execReal {
					args, _ := ev.Data["args"].(string)
					pendingRealCalls[id] = pendingRealCall{name: name, args: args}
					slog.Debug("mock: tool_call marked execute_real=true, will call real handler at result",
						"scenario", scenario.ID, "id", id, "name", name)
				}
				s.sendAndCache(sess, w, flusher, "tool_call", ev.Data)

			case "stream_start":
				// 首个 stream_start 注入 mock: true 字段（前端用来显示「模拟模式」徽章）
				if mockFlag && !streamStartEmitted {
					merged := make(map[string]interface{}, len(ev.Data)+1)
					for k, v := range ev.Data {
						merged[k] = v
					}
					merged["mock"] = true
					merged["scenario"] = scenario.ID
					s.sendAndCache(sess, w, flusher, "stream_start", merged)
					streamStartEmitted = true

					// 紧随 stream_start 推送初始预设按钮（如果有）。
					// 前端 MockPresetBar 监听到 mock_presets 事件即渲染 chip。
					// mid-scenario 期间的预设更新通过单独的 mock_presets 事件
					// 在 steps 内推（见下方 switch case）。
					if len(scenario.Presets) > 0 {
						s.sendAndCache(sess, w, flusher, "mock_presets", map[string]interface{}{
							"scenario": scenario.ID,
							"phase":    "initial",
							"presets":  scenario.Presets,
						})
					}
				} else {
					s.sendAndCache(sess, w, flusher, "stream_start", ev.Data)
				}

			case "mock_presets":
				// mid-scenario 预设更新（高级剧本多轮会话）：
				// 剧本可在任意 step 推一个 mock_presets 事件，data.presets 覆盖
				// 当前激活的预设列表。前端基于此实时刷新 chip 按钮。
				// data.phase 字段由剧本作者标记当前阶段（initial/middle/followup），
				// 仅做调试 / 埋点用，前端不强依赖。
				s.sendAndCache(sess, w, flusher, "mock_presets", ev.Data)

			case "text_delta":
				textSeq++
				text, _ := ev.Data["text"].(string)
				s.sendAndCache(sess, w, flusher, "text_delta",
					map[string]interface{}{"seq": textSeq, "text": text})

			case "reasoning_delta":
				reasoningSeq++
				text, _ := ev.Data["text"].(string)
				s.sendAndCache(sess, w, flusher, "reasoning_delta",
					map[string]interface{}{"seq": reasoningSeq, "text": text})

			case "stream_status":
				s.sendAndCache(sess, w, flusher, "stream_status", ev.Data)
				// 错误状态：自动追加 stream_end(finishReason=error) 终止流
				if isErrorStatus(ev.Data) {
					s.sendAndCache(sess, w, flusher, "stream_end",
						map[string]interface{}{"finishReason": "error"})
					s.endMockPresets(sess, w, flusher, scenario.ID, "stream_status_error")
					slog.Info("mock: stream_status=error injected auto stream_end",
						"scenario", scenario.ID)
					return nil
				}

			case "tool_result":
				// 若对应的 tool_call 标记了 execute_real=true 且 realExecutor 已注入，
				// 实际调用以拿真实结果，覆盖剧本的硬编码 result。
				// 否则按剧本硬编码 data 原样推送。
				id, _ := ev.Data["id"].(string)
				name, _ := ev.Data["name"].(string)
				if pending, ok := pendingRealCalls[id]; ok {
					delete(pendingRealCalls, id)
					if e.realExecutor != nil {
						// 先检查 ctx 取消（避免 realExecutor 长耗时后做无用功）
						select {
						case <-ctx.Done():
							slog.Info("mock: ctx cancelled before real tool exec",
								"scenario", scenario.ID, "id", id, "name", pending.name)
							return ctx.Err()
						default:
						}
						e.executeRealAndEmit(ctx, s, sess, w, flusher, pending, id, name)
						continue
					}
					// realExecutor == nil：单测/容灾路径，硬编码剧本 result
					slog.Debug("mock: execute_real=true but realExecutor nil, falling back to hardcoded result",
						"scenario", scenario.ID, "id", id)
				}
				s.sendAndCache(sess, w, flusher, "tool_result", ev.Data)

			case "stream_end":
				// 推送 stream_end 后立刻推 mock_presets_clear，让前端清空 bar。
				// 这样用户看到 assistant 完整回复后，preset 按钮自动消失，
				// 输入框恢复"自由输入"状态。下次新场景激活时再发新预设。
				s.sendAndCache(sess, w, flusher, "stream_end", ev.Data)
				s.endMockPresets(sess, w, flusher, scenario.ID, "stream_end")

			default:
				// 其他类型：原样推送
				s.sendAndCache(sess, w, flusher, ev.Type, ev.Data)
			}
		}
	}

	// 剧本结束时尚未消费的 pending real calls（剧本只发了 tool_call
	// 没发对应 tool_result）→ 静默丢弃。Run 局部 map 随函数返回 GC。
	if len(pendingRealCalls) > 0 {
		slog.Debug("mock: scenario ended with unmatched real-exec tool_calls (will be GC'd)",
			"scenario", scenario.ID, "count", len(pendingRealCalls))
	}

	// 防御性兜底：若剧本没显式发 stream_end 事件（异常剧本），
	// 此处也推一次 clear，防止 chip 永久挂在 UI 上。
	// 幂等：stream_end case 已发过的 clear 会带相同 scenario ID，重复清空是 OK 的。
	s.endMockPresets(sess, w, flusher, scenario.ID, "scenario_done")

	return nil
}

// endMockPresets 推一个 mock_presets_clear 事件，前端 MockPresetBar 收到后清空 chip。
// reason 仅做调试 / 埋点用，前端不强依赖。
func (s *Server) endMockPresets(sess *agentSession, w http.ResponseWriter, flusher http.Flusher, scenarioID, reason string) {
	s.sendAndCache(sess, w, flusher, "mock_presets_clear", map[string]interface{}{
		"scenario": scenarioID,
		"reason":   reason,
	})
}

// pendingRealCall 是剧本中标记 execute_real=true 的 tool_call 的最小信息。
// 后续匹配的 tool_result 事件触发实际 realExecutor 调用时使用。
type pendingRealCall struct {
	name string
	args string
}

// executeRealAndEmit 实际调用 realExecutor 拿真实结果，推送 tool_result 事件。
//
// 事件 data 字段：
//   - id, name：取自剧本 tool_call（保证 ID 一致性，前端可关联 tool_call ↔ tool_result）
//   - result：realExecutor 返回的 JSON 字符串
//   - isError / status：基于 realExecutor 的 error 返回值
//   - durationMs：真实执行耗时
//
// realExecutor 内部会自行处理 ctx 取消（executeFSTool/executePluginTool 都用 ctx 传下去）。
// 本函数不返回错误，slog 记录所有失败路径。
func (e *MockEngine) executeRealAndEmit(
	ctx context.Context,
	s *Server,
	sess *agentSession,
	w http.ResponseWriter,
	flusher http.Flusher,
	pending pendingRealCall,
	id string,
	name string,
) {
	t0 := time.Now()
	out, err := e.realExecutor(ctx, pending.name, pending.args)
	dur := time.Since(t0).Milliseconds()

	if err != nil {
		// 真实工具执行报错 → 推送带 isError=true 的 tool_result
		resultStr := out
		if resultStr == "" {
			resultStr = fmt.Sprintf(`{"error":%q}`, err.Error())
		}
		slog.Warn("mock: real tool exec failed, emit tool_result with isError=true",
			"id", id, "name", pending.name, "args", pending.args, "dur_ms", dur, "error", err)
		s.sendAndCache(sess, w, flusher, "tool_result", map[string]interface{}{
			"id":         id,
			"name":       name,
			"result":     resultStr,
			"isError":    true,
			"status":     "failed",
			"durationMs": dur,
		})
		return
	}

	slog.Info("mock: real tool exec succeeded",
		"id", id, "name", pending.name, "args", pending.args, "dur_ms", dur)
	s.sendAndCache(sess, w, flusher, "tool_result", map[string]interface{}{
		"id":         id,
		"name":       name,
		"result":     out,
		"isError":    false,
		"status":     "success",
		"durationMs": dur,
	})
}

// sleepDelay 按 (DelayMs / speed) 等待，遇 ctx 取消立即返回。
// speed == 0 时不等待（零延迟）。
func sleepDelay(ctx context.Context, delayMs int, speed float64) error {
	if delayMs <= 0 || speed == 0 {
		// 检查 ctx 即使在零延迟场景（避免被 hang 住）
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	d := time.Duration(float64(delayMs)/speed) * time.Millisecond
	if d <= 0 {
		d = time.Millisecond // 最小 1ms
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// isErrorStatus 判断 stream_status 事件 data 是否表示错误。
func isErrorStatus(data map[string]interface{}) bool {
	if data == nil {
		return false
	}
	if t, ok := data["type"].(string); ok && t == "error" {
		return true
	}
	if s, ok := data["status"].(string); ok && s == "error" {
		return true
	}
	return false
}

// sendCorruptChunk 直接向 writer 写入一段非 JSON 的 SSE 数据。
// 不走 sendAndCache 是因为它会触发 JSON 编码。
func sendCorruptChunk(w http.ResponseWriter) {
	// 写入 "data: NOT-JSON\n\n" —— 前端 useAgent 解析器会 catch 错误并丢弃
	_, _ = w.Write([]byte("data: NOT-JSON\n\n"))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
