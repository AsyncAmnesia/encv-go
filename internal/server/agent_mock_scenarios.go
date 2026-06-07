// internal/server/agent_mock_scenarios.go
//
// 12 个内置 MockScenario 定义。
//
// 设计原则：
//   - 每个剧本是独立函数返回 *MockScenario，便于单测直接调用验证事件序列
//   - 所有事件类型与真实 OpenAI 路径输出字节级一致（前端 0 改动）
//   - 延迟/事件顺序严格按 spec.md 编排，覆盖 [真机问题] / [reasoning 排序] / [并行工具] 等代码分支
//
// 参考：
//   - Spec: /workspace/.trae/specs/agent-mock-mode/spec.md §Requirement: 内置剧本清单
package server

import "strings"

// ════════════════════════════════════════════════════════════════
// 1. default_friendly — fallback 默认剧本
// ════════════════════════════════════════════════════════════════
//
// 用途：覆盖纯文本闲聊路径，验证 text_delta + stream_end 基本流程。
// 触发：无匹配时的兜底。

func scenarioDefaultFriendly() *MockScenario {
	return &MockScenario{
		ID:          "default_friendly",
		Description: "fallback 默认剧本：3 段流式文本 + 正常结束",
		Steps: []MockStep{
			{DelayMs: 0, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]interface{}{
					"scenario": "default_friendly",
				}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "你好！"}},
			}},
			{DelayMs: 200, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "我是 ENCV 助手。"}},
			}},
			{DelayMs: 300, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "有什么可以帮你的吗？"}},
			}},
			{DelayMs: 100, Events: []MockEvent{
				{Type: "stream_end", Data: map[string]interface{}{
					"finishReason": "stop",
					"usage":        map[string]int{"totalTokens": 42},
				}},
			}},
		},
	}
}

// ════════════════════════════════════════════════════════════════
// 2. list_files_query — 修复 [真机问题] 的核心场景
// ════════════════════════════════════════════════════════════════
//
// 用途：触发 list_mounts → list_files → 完整文本回复，验证工具调用结构化渲染。
// 触发："有哪些文件" / "有哪些视频" 等。

func scenarioListFilesQuery() *MockScenario {
	mountsResult := `{"count":1,"items":[{"id":"serving","path":"/mnt/serving","name":"serving"}]}`
	filesResult := `{"files":[{"name":"studio_video_1762059800961.mp4","size":554000000,"is_dir":false},{"name":"QQ","is_dir":true},{"name":"Subtitles","is_dir":true},{"name":"qqmusic","is_dir":true}]}`

	return &MockScenario{
		ID:          "list_files_query",
		Description: "list_mounts → list_files + 完整文本（覆盖 [真机问题]）",
		Keywords:    []string{"有哪些文件", "有什么文件", "视频", "Movies", "目录", "列一下"},
		Steps: []MockStep{
			// Step 1: 开场白
			{DelayMs: 0, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]interface{}{
					"scenario": "list_files_query",
				}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "好的，我先查看挂载点。\n\n"}},
			}},
			// Step 2: 调 list_mounts
			{DelayMs: 500, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_1",
					"name":         "list_mounts",
					"args":         "{}",
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_1",
					"status": "running",
				}},
			}},
			// Step 3: list_mounts 结果
			{DelayMs: 300, Events: []MockEvent{
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_1",
					"status": "success",
				}},
				{Type: "tool_result", Data: map[string]interface{}{
					"id":         "call_1",
					"name":       "list_mounts",
					"result":     mountsResult,
					"isError":    false,
					"status":     "success",
					"durationMs": 12,
				}},
			}},
			// Step 4: 中间过渡文本
			{DelayMs: 400, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "已找到挂载点 serving。继续查看 Movies 目录..."}},
			}},
			// Step 5: 调 list_files
			{DelayMs: 500, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_2",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"Movies"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_2",
					"status": "running",
				}},
			}},
			// Step 6: list_files 结果
			{DelayMs: 400, Events: []MockEvent{
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_2",
					"status": "success",
				}},
				{Type: "tool_result", Data: map[string]interface{}{
					"id":         "call_2",
					"name":       "list_files",
					"result":     filesResult,
					"isError":    false,
					"status":     "success",
					"durationMs": 18,
				}},
			}},
			// Step 7: 完整文本回答（4 段拼接为完整回复）
			{DelayMs: 300, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "在 /Movies 目录下发现 1 个视频文件：\n\n"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "- studio_video_1762059800961.mp4 (约 554MB)\n\n"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "其他条目都是子目录，如 QQ、Subtitles、qqmusic...\n"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "如需进一步查看子目录内容，请告诉我。"}},
			}},
			// Step 8: 结束
			{DelayMs: 100, Events: []MockEvent{
				{Type: "stream_end", Data: map[string]interface{}{
					"finishReason": "stop",
					"usage":        map[string]int{"totalTokens": 318},
				}},
			}},
		},
	}
}

// ════════════════════════════════════════════════════════════════
// 3. encrypt_video — 单 tool_call（kind=fileChange）
// ════════════════════════════════════════════════════════════════
//
// 用途：验证 plugin 工具（需用户确认）的 tool_call 渲染路径。
// 触发："加密视频" 等。

func scenarioEncryptVideo() *MockScenario {
	return &MockScenario{
		ID:          "encrypt_video",
		Description: "1 tool_call (kind=fileChange) — 验证 plugin 工具确认流程",
		Keywords:    []string{"加密视频", "加密", "encrypt", "video"},
		Steps: []MockStep{
			{DelayMs: 0, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]interface{}{"scenario": "encrypt_video"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "好的，我来加密这个视频文件。"}},
			}},
			{DelayMs: 400, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_enc_1",
					"name":         "video_encrypt",
					"args":         `{"input_paths":["/Movies/sample.mp4"],"output_path":"/Movies/sample.enc"}`,
					"auto_run":     false,
					"needsConfirm": true,
					"kind":         "fileChange",
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_enc_1",
					"status": "running",
				}},
			}},
			{DelayMs: 100, Events: []MockEvent{
				{Type: "stream_end", Data: map[string]interface{}{
					"finishReason": "tool_calls",
					"usage":        map[string]int{"totalTokens": 89},
				}},
			}},
		},
	}
}

// ════════════════════════════════════════════════════════════════
// 4. read_and_summarize — read_file 工具调用
// ════════════════════════════════════════════════════════════════
//
// 用途：验证只读文件工具 + 总结文本输出。
// 触发："总结" + 文件名 / "读" + "总结"。

func scenarioReadAndSummarize() *MockScenario {
	return &MockScenario{
		ID:          "read_and_summarize",
		Description: "read_file tool_call — 验证只读工具 + 总结",
		Keywords:    []string{"总结", "summarize", "摘要", "归纳"},
		Steps: []MockStep{
			{DelayMs: 0, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]interface{}{"scenario": "read_and_summarize"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "我先读取这个文件的内容。"}},
			}},
			{DelayMs: 400, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_read_1",
					"name":         "read_file",
					"args":         `{"mount_id":"serving","rel_path":"docs/readme.md"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_read_1",
					"status": "running",
				}},
			}},
			{DelayMs: 300, Events: []MockEvent{
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_read_1",
					"status": "success",
				}},
				{Type: "tool_result", Data: map[string]interface{}{
					"id":         "call_read_1",
					"name":       "read_file",
					"result":     `{"content":"# Project README\nThis is a sample README...","note":""}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 24,
				}},
			}},
			{DelayMs: 200, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "文件内容已读取。"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "核心要点如下：\n- 项目名称：encv\n- 用途：容器加密\n- 主要功能：..."}},
			}},
			{DelayMs: 100, Events: []MockEvent{
				{Type: "stream_end", Data: map[string]interface{}{
					"finishReason": "stop",
					"usage":        map[string]int{"totalTokens": 156},
				}},
			}},
		},
	}
}

// ════════════════════════════════════════════════════════════════
// 5. multi_step_search — 3 轮工具调用
// ════════════════════════════════════════════════════════════════
//
// 用途：验证 Agent Loop 多次工具调用 + 文本综合输出。
// 触发："搜索" + "视频" / "查找" + "视频"。

func scenarioMultiStepSearch() *MockScenario {
	return &MockScenario{
		ID:          "multi_step_search",
		Description: "3 轮 tool_call — list_files → grep → read_file 验证递归",
		Keywords:    []string{"搜索", "查找", "search", "找一下", "找视频"},
		Steps: []MockStep{
			{DelayMs: 0, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]interface{}{"scenario": "multi_step_search"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "我来搜索视频文件。"}},
			}},
			// Round 1: list_files
			{DelayMs: 400, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_search_1",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"Movies"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_search_1",
					"status": "running",
				}},
			}},
			{DelayMs: 300, Events: []MockEvent{
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_search_1",
					"status": "success",
				}},
				{Type: "tool_result", Data: map[string]interface{}{
					"id":         "call_search_1",
					"name":       "list_files",
					"result":     `{"files":[{"name":"video1.mp4","is_dir":false},{"name":"sub","is_dir":true}]}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 15,
				}},
			}},
			// Round 2: 模拟 grep 工具（实际项目中是 list_files + filter）
			{DelayMs: 300, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_search_2",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"Movies/sub"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_search_2",
					"status": "running",
				}},
			}},
			{DelayMs: 300, Events: []MockEvent{
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_search_2",
					"status": "success",
				}},
				{Type: "tool_result", Data: map[string]interface{}{
					"id":         "call_search_2",
					"name":       "list_files",
					"result":     `{"files":[{"name":"target.mp4","size":1024000000,"is_dir":false}]}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 12,
				}},
			}},
			// Round 3: read_file
			{DelayMs: 300, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_search_3",
					"name":         "stat_file",
					"args":         `{"mount_id":"serving","rel_path":"Movies/sub/target.mp4"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_search_3",
					"status": "running",
				}},
			}},
			{DelayMs: 300, Events: []MockEvent{
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_search_3",
					"status": "success",
				}},
				{Type: "tool_result", Data: map[string]interface{}{
					"id":         "call_search_3",
					"name":       "stat_file",
					"result":     `{"name":"target.mp4","size":1024000000,"is_dir":false,"mtime":"2024-01-15T10:30:00Z"}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 8,
				}},
			}},
			{DelayMs: 200, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "搜索完成：找到 1 个目标视频文件。\n"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "路径：/Movies/sub/target.mp4（1GB）"}},
			}},
			{DelayMs: 100, Events: []MockEvent{
				{Type: "stream_end", Data: map[string]interface{}{
					"finishReason": "stop",
					"usage":        map[string]int{"totalTokens": 423},
				}},
			}},
		},
	}
}

// ════════════════════════════════════════════════════════════════
// 6. streaming_error — 错误分支覆盖
// ════════════════════════════════════════════════════════════════
//
// 用途：验证前端 stream_status 错误处理（toast / status=error）。
// 触发："触发错误" / "测试错误"。

func scenarioStreamingError() *MockScenario {
	return &MockScenario{
		ID:          "streaming_error",
		Description: "stream_status(error) + auto stream_end — 验证错误处理路径",
		Keywords:    []string{"触发错误", "测试错误", "出错", "上游超时", "error"},
		Steps: []MockStep{
			{DelayMs: 0, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]interface{}{"scenario": "streaming_error"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "正在处理..."}},
			}},
			{DelayMs: 800, Events: []MockEvent{
				{Type: "stream_status", Data: map[string]interface{}{
					"type":    "error",
					"code":    "upstream_timeout",
					"message": "上游 LLM 服务超时（模拟）",
				}},
			}},
		},
	}
}

// ════════════════════════════════════════════════════════════════
// 7. truncation_long_text — 单 text_delta > 2000 字符
// ════════════════════════════════════════════════════════════════
//
// 用途：验证超长单 chunk 的 SSE 传输 + 前端排序重建。
// 触发："写一篇长文" / "长文本"。

func scenarioTruncationLongText() *MockScenario {
	// 生成 3000 字符的 lorem ipsum 风格文本
	var b strings.Builder
	b.WriteString("# 关于 ENCV 容器加密的完整技术报告\n\n")
	b.WriteString("## 概述\n\n")
	b.WriteString("ENCV（Encrypted Container for Video）是一个端到端加密的容器格式，")
	b.WriteString("专为音视频等大文件设计。本报告将详细介绍其设计原理、实现细节、性能特征，")
	b.WriteString("以及与同类方案的对比分析。\n\n")
	b.WriteString("## 一、设计目标\n\n")
	b.WriteString("ENCV 的设计目标可以归纳为以下几点：\n\n")
	b.WriteString("1. **强加密**：使用 AES-256-GCM 提供认证加密\n")
	b.WriteString("2. **流式处理**：支持边读边解密，无需加载整个文件\n")
	b.WriteString("3. **可恢复**：单个分片损坏不影响其他分片\n")
	b.WriteString("4. **跨平台**：桌面 / 移动 / 嵌入式均可运行\n")
	b.WriteString("5. **元数据保护**：文件名、大小、修改时间均加密\n\n")
	// 重复填充以达到 3000 字符
	for b.Len() < 3000 {
		b.WriteString("\n\n本节讨论容器内部的字节对齐策略和块大小选择。")
		b.WriteString("经过多次实验，我们最终选择了 64KB 的固定块大小，")
		b.WriteString("这一大小在 4K 随机读取和顺序读取之间取得了良好的平衡。")
	}
	return &MockScenario{
		ID:          "truncation_long_text",
		Description: "单 text_delta > 2000 字符 — 验证超长 chunk 传输",
		Keywords:    []string{"写一篇长文", "长文本", "写长", "long text", "详细解释"},
		Steps: []MockStep{
			{DelayMs: 0, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]interface{}{"scenario": "truncation_long_text"}},
			}},
			{DelayMs: 100, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": b.String()}},
			}},
			{DelayMs: 100, Events: []MockEvent{
				{Type: "stream_end", Data: map[string]interface{}{
					"finishReason": "stop",
					"usage":        map[string]int{"totalTokens": 1500},
				}},
			}},
		},
	}
}

// ════════════════════════════════════════════════════════════════
// 8. reasoning_chain — reasoning + text 交替
// ════════════════════════════════════════════════════════════════
//
// 用途：验证 reasoning_delta 字段的前端 Map 排序逻辑。
// 触发："推理" / "思考" / "分析"。

func scenarioReasoningChain() *MockScenario {
	return &MockScenario{
		ID:          "reasoning_chain",
		Description: "reasoning_delta + text_delta 交替 — 验证 reasoning 字段排序",
		Keywords:    []string{"推理", "思考", "分析", "reasoning", "step by step"},
		Steps: []MockStep{
			{DelayMs: 0, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]interface{}{"scenario": "reasoning_chain"}},
			}},
			{DelayMs: 200, Events: []MockEvent{
				{Type: "reasoning_delta", Data: map[string]interface{}{"text": "让我先分析问题..."}},
			}},
			{DelayMs: 300, Events: []MockEvent{
				{Type: "reasoning_delta", Data: map[string]interface{}{"text": "需要考虑 X、Y、Z 三个因素。"}},
			}},
			{DelayMs: 300, Events: []MockEvent{
				{Type: "reasoning_delta", Data: map[string]interface{}{"text": "X 因素涉及...\nY 因素涉及...\nZ 因素涉及..."}},
			}},
			{DelayMs: 200, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "根据以上分析，"}},
			}},
			{DelayMs: 200, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "我的答案是..."}},
			}},
			{DelayMs: 200, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "综合三个因素，最佳方案是 Z。"}},
			}},
			{DelayMs: 100, Events: []MockEvent{
				{Type: "stream_end", Data: map[string]interface{}{
					"finishReason": "stop",
					"usage":        map[string]int{"totalTokens": 234},
				}},
			}},
		},
	}
}

// ════════════════════════════════════════════════════════════════
// 9. tool_call_with_args — 完整 args JSON
// ════════════════════════════════════════════════════════════════
//
// 用途：验证 tool_call 的 args JSON 字段不截断 + 复杂参数透传。
// 触发："调用工具" / "tool call"。

func scenarioToolCallWithArgs() *MockScenario {
	return &MockScenario{
		ID:          "tool_call_with_args",
		Description: "1 tool_call with full args JSON — 验证 args 字段不截断",
		Keywords:    []string{"调用工具", "tool call", "test tool", "工具调用"},
		Steps: []MockStep{
			{DelayMs: 0, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]interface{}{"scenario": "tool_call_with_args"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "我调用一个带复杂参数的工具："}},
			}},
			{DelayMs: 300, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_args_1",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"Movies/2024","max_entries":"50","filter":{"ext":[".mp4",".mkv"],"min_size":1048576}}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_args_1",
					"status": "running",
				}},
			}},
			{DelayMs: 100, Events: []MockEvent{
				{Type: "stream_end", Data: map[string]interface{}{
					"finishReason": "tool_calls",
					"usage":        map[string]int{"totalTokens": 78},
				}},
			}},
		},
	}
}

// ════════════════════════════════════════════════════════════════
// 10. multi_tool_parallel — 同 step 内 3 个 tool_call
// ════════════════════════════════════════════════════════════════
//
// 用途：验证前端 GroupedOperationMessage 对多 tool_call 的渲染。
// 触发："批量" / "并行" / "同时"。

func scenarioMultiToolParallel() *MockScenario {
	return &MockScenario{
		ID:          "multi_tool_parallel",
		Description: "同 step 内 3 tool_call — 验证并行工具调用渲染",
		Keywords:    []string{"批量", "并行", "同时", "parallel", "batch"},
		Steps: []MockStep{
			{DelayMs: 0, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]interface{}{"scenario": "multi_tool_parallel"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "我并行查询多个目录："}},
			}},
			{DelayMs: 400, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_par_1",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"Movies"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
				}},
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_par_2",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"Music"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
				}},
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_par_3",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"Documents"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_par_1",
					"status": "running",
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_par_2",
					"status": "running",
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_par_3",
					"status": "running",
				}},
			}},
			{DelayMs: 100, Events: []MockEvent{
				{Type: "stream_end", Data: map[string]interface{}{
					"finishReason": "tool_calls",
					"usage":        map[string]int{"totalTokens": 145},
				}},
			}},
		},
	}
}

// ════════════════════════════════════════════════════════════════
// 11. context_exhausted — finishReason=length
// ════════════════════════════════════════════════════════════════
//
// 用途：验证 LLM 触发 token 上限截断时的前端黄色警告。
// 触发："上下文爆了" / "超长" / "exhausted"。

func scenarioContextExhausted() *MockScenario {
	return &MockScenario{
		ID:          "context_exhausted",
		Description: "finishReason=length — 验证截断警告",
		Keywords:    []string{"上下文爆了", "超长", "exhausted", "token limit", "太长"},
		Steps: []MockStep{
			{DelayMs: 0, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]interface{}{"scenario": "context_exhausted"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "（长文本达到模型上限）"}},
			}},
			{DelayMs: 100, Events: []MockEvent{
				{Type: "stream_end", Data: map[string]interface{}{
					"finishReason": "length",
					"usage":        map[string]int{
						"totalTokens": 131072,
						"maxTokens":   128000,
					},
				}},
			}},
		},
	}
}

// ════════════════════════════════════════════════════════════════
// 12. chinese_greeting — 单字符 chunk 流式
// ════════════════════════════════════════════════════════════════
//
// 用途：验证中文分词 + 字符级流式不丢字。
// 触发："你好" / "hello" / "hi"。

func scenarioChineseGreeting() *MockScenario {
	return &MockScenario{
		ID:          "chinese_greeting",
		Description: "单字符 chunk 流式 — 验证中文不丢字",
		Keywords:    []string{"你好", "hello", "hi", "在吗", "哈喽"},
		Steps: []MockStep{
			{DelayMs: 0, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]interface{}{"scenario": "chinese_greeting"}},
			}},
			{DelayMs: 50, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "你"}},
			}},
			{DelayMs: 50, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "好"}},
			}},
			{DelayMs: 50, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "！"}},
			}},
			{DelayMs: 50, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "我"}},
			}},
			{DelayMs: 50, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "是"}},
			}},
			{DelayMs: 50, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "EN"}},
			}},
			{DelayMs: 50, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "CV"}},
			}},
			{DelayMs: 50, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "助"}},
			}},
			{DelayMs: 50, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "手"}},
			}},
			{DelayMs: 50, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "。"}},
			}},
			{DelayMs: 100, Events: []MockEvent{
				{Type: "stream_end", Data: map[string]interface{}{
					"finishReason": "stop",
					"usage":        map[string]int{"totalTokens": 16},
				}},
			}},
		},
	}
}
