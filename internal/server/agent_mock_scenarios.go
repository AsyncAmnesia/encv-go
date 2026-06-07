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
		// 预设：引导用户进入主功能剧本（覆盖常见开场白 → 触发后续子剧本）
		Presets: []MockPreset{
			{ID: "ask_files", Label: "📁 查看文件", UserText: "有哪些视频文件", Tooltip: "触发 list_files_query 剧本"},
			{ID: "ask_summarize", Label: "📄 总结文档", UserText: "总结 docs/readme.md", Tooltip: "触发 read_and_summarize 剧本"},
			{ID: "ask_encrypt", Label: "🔒 加密视频", UserText: "加密这个视频", Tooltip: "触发 encrypt_video 剧本"},
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
		// execute_real=true 标记：list_mounts / list_files 是只读工具，
		// 应在 mock 模式下也实际执行（结果用真实文件系统数据，覆盖硬编码 JSON），
		// 避免「mock 剧本编造文件列表」的真机问题。
		Keywords: []string{"有哪些文件", "有什么文件", "视频", "Movies", "目录", "列一下"},
		Steps: []MockStep{
			// Step 1: 开场白
			{DelayMs: 0, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]interface{}{
					"scenario": "list_files_query",
				}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "好的，我先查看挂载点。\n\n"}},
			}},
			// Step 2: 调 list_mounts（execute_real=true → 实际调用 s.executeAgentTool）
			{DelayMs: 500, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_1",
					"name":         "list_mounts",
					"args":         "{}",
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
					"execute_real": true,
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_1",
					"status": "running",
				}},
			}},
			// Step 3: list_mounts 结果（execute_real 时硬编码 result 会被覆盖，可保留作 fallback）
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
			// Step 5: 调 list_files（execute_real=true）
			{DelayMs: 500, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_2",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"Movies"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
					"execute_real": true,
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
		// 预设：用户看了 /Movies 列表后最可能的 4 个后续动作
		//   - p_qq/p_sub/p_music：进入 Movies 的 3 个子目录，触发 list_files_query 递归
		//     （args.rel_path 不同 → 真实 list_files 返回对应子目录内容）
		//   - p_encrypt：跳到 encrypt_video 剧本（写操作需用户二次确认）
		//   - p_search：跳到 multi_step_search 剧本
		Presets: []MockPreset{
			{ID: "p_qq", Label: "📂 QQ 子目录", UserText: "查看 Movies/QQ 目录", Tooltip: "查看 QQ 目录里的内容"},
			{ID: "p_sub", Label: "📂 Subtitles", UserText: "查看 Movies/Subtitles 目录", Tooltip: "查看字幕目录"},
			{ID: "p_search", Label: "🔍 搜索字幕", UserText: "搜索带字幕的视频", Tooltip: "触发 multi_step_search 剧本"},
			{ID: "p_encrypt", Label: "🔒 加密这个视频", UserText: "加密这个视频", Tooltip: "触发 encrypt_video 剧本（需用户确认）"},
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
		// ⚠️ execute_real=false（不是 true）：
		// video_encrypt 是写操作（会真的加密文件），mock 模式绝对不能让
		// 剧本误删/误改用户文件。硬编码 tool_result 用 success 占位，
		// 真正的加密由用户在确认流（needsConfirm=true）触发。
		Keywords: []string{"加密视频", "加密", "encrypt", "video"},
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
					"execute_real": false, // 写操作：见上方注释，绝不能在 mock 模式下实际执行
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
		// 预设：tool_call 推出去之前用户可以点 chip 切换动作
		//   - p_cancel：放弃加密（最常见）
		//   - p_choose_other：改看其他文件
		//   - p_modify_path：修改输出路径（实际触发 list_files_query 让用户选目录）
		// 注意：tool_call 仍会推送（needsConfirm=true），用户后续在确认卡上
		// 点 Approve/Deny 决定是否真加密。preset 是"前哨"操作。
		Presets: []MockPreset{
			{ID: "p_cancel", Label: "❌ 取消", UserText: "算了，不加密了", Tooltip: "取消加密动作"},
			{ID: "p_modify_path", Label: "📁 改输出到 /Encrypted", UserText: "改加密输出到 /Encrypted 目录", Tooltip: "修改输出路径"},
			{ID: "p_choose_other", Label: "📂 看其他视频", UserText: "有哪些视频文件", Tooltip: "跳到 list_files_query 选别的"},
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
					"execute_real": true, // 只读工具：mock 模式也用真实文件内容
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
		// 预设：用户看完 summary 后的常见后续
		//   - p_full：要求看完整原文（不总结）
		//   - p_translate：翻译成中文
		//   - p_bullets：要 bullet 列表版总结
		//   - p_save：保存总结到文件（跳到需要 write_file 的剧情）
		Presets: []MockPreset{
			{ID: "p_full", Label: "📜 看完整原文", UserText: "显示 readme.md 完整内容", Tooltip: "跳到 read_file 完整版"},
			{ID: "p_bullets", Label: "🔹 bullet 列表版", UserText: "用 bullet 列表总结 readme.md", Tooltip: "切换为列表格式"},
			{ID: "p_translate", Label: "🌐 翻译成中文", UserText: "把 readme.md 翻译成中文", Tooltip: "中文化输出"},
			{ID: "p_save", Label: "💾 保存为摘要", UserText: "把这份总结保存到 notes/summary.md", Tooltip: "保存到文件（需确认）"},
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
			// Round 1: list_files（execute_real=true）
			{DelayMs: 400, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_search_1",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"Movies"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
					"execute_real": true,
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
			// Round 2: 模拟 grep 工具（实际项目中是 list_files + filter，execute_real=true）
			{DelayMs: 300, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_search_2",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"Movies/sub"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
					"execute_real": true,
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
			// ⏸️ Mid-scenario preset update（演示「高级剧本连续会话预设」）：
			// Round 2 完成后发现目标，前端 bar 切换为「深入看 sub / 统计大小 / 加密」
			// 用户点 chip 直接发送 UserText，触发下一个剧本（无 server pause，
			// mock_presets 只是 UI 提示，scenario 仍会继续播到 stream_end）。
			{DelayMs: 100, Events: []MockEvent{
				{Type: "mock_presets", Data: map[string]interface{}{
					"scenario": "multi_step_search",
					"phase":    "after_round_2",
					"presets": []MockPreset{
						{ID: "p_stat", Label: "📊 看文件大小", UserText: "Movies/sub/target.mp4 多大", Tooltip: "触发 stat_file 详细输出"},
						{ID: "p_encrypt_found", Label: "🔒 加密这个文件", UserText: "加密 Movies/sub/target.mp4", Tooltip: "跳到 encrypt_video"},
						{ID: "p_done", Label: "✅ 结束搜索", UserText: "好的，不用再找了", Tooltip: "结束本轮搜索"},
					},
				}},
			}},
			// Round 3: stat_file（execute_real=true — 真实文件 stat）
			{DelayMs: 300, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_search_3",
					"name":         "stat_file",
					"args":         `{"mount_id":"serving","rel_path":"Movies/sub/target.mp4"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
					"execute_real": true,
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
		// 初始预设（剧本刚激活时显示）。mid-scenario 阶段另有 after_round_2 预设
		// （见上方 step 5），前端会按"最新 mock_presets 事件"覆盖。
		Presets: []MockPreset{
			{ID: "p_init_listing", Label: "📂 先列 Movies", UserText: "列出 Movies 目录", Tooltip: "从入口开始"},
			{ID: "p_init_all_video", Label: "🎬 找所有视频", UserText: "找所有视频", Tooltip: "触发完整 3 轮搜索"},
			{ID: "p_init_cancel", Label: "❌ 取消", UserText: "算了不找了", Tooltip: "退出搜索"},
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
		// 预设：用户看到错误状态后可能的恢复动作
		Presets: []MockPreset{
			{ID: "p_retry", Label: "🔁 重试", UserText: "重试一下", Tooltip: "重发相同问题"},
			{ID: "p_simpler", Label: "📝 简化问题", UserText: "列出 /Movies", Tooltip: "换成简单 list_files_query"},
			{ID: "p_view_logs", Label: "📋 查看错误日志", UserText: "显示错误日志", Tooltip: "切到 DevLogs tab"},
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
		// 预设：超长文本后用户常见动作
		Presets: []MockPreset{
			{ID: "p_summarize_long", Label: "🔹 总结上面", UserText: "用 bullet 总结上面这篇", Tooltip: "压缩为 bullet"},
			{ID: "p_key_points", Label: "🎯 关键点", UserText: "提取关键点", Tooltip: "关键点提取"},
			{ID: "p_save_long", Label: "💾 保存全文", UserText: "保存到 docs/long.md", Tooltip: "保存到文件"},
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
		// 预设：推理完成后用户可能追问
		Presets: []MockPreset{
			{ID: "p_why_z", Label: "❓ 为什么选 Z", UserText: "为什么是 Z 不是 X", Tooltip: "追问决策依据"},
			{ID: "p_alternative", Label: "🔄 换思路", UserText: "换个思路分析", Tooltip: "重新推理"},
			{ID: "p_save_reasoning", Label: "💾 保存推理过程", UserText: "把推理过程保存到 analysis.md", Tooltip: "保存到文件"},
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
					"execute_real": true, // 真实 list_files（args 透传给 handler）
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
		// 预设：用户看到带复杂 args 的工具调用后，可调整参数重试
		//   - p_no_filter：去掉 filter 直接列
		//   - p_smaller_size：把 min_size 改大（找大文件）
		//   - p_different_ext：换文件类型
		Presets: []MockPreset{
			{ID: "p_no_filter", Label: "📂 不用 filter", UserText: "列出 Movies/2024 全部文件", Tooltip: "去掉 filter/max_entries"},
			{ID: "p_smaller_size", Label: "💎 找大于 10GB 的", UserText: "Movies/2024 找大于 10GB 的视频", Tooltip: "min_size 调大"},
			{ID: "p_different_ext", Label: "🎞️ 改成 mkv", UserText: "Movies/2024 找 mkv 视频", Tooltip: "改 ext 过滤"},
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
					"execute_real": true,
				}},
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_par_2",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"Music"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
					"execute_real": true,
				}},
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_par_3",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"Documents"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRead",
					"execute_real": true,
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
		// 预设：3 个并行工具调用后用户可能想筛选/汇总
		Presets: []MockPreset{
			{ID: "p_only_movies", Label: "🎬 只看 Movies", UserText: "只列 Movies 目录", Tooltip: "缩减范围"},
			{ID: "p_count_files", Label: "🔢 统计文件数", UserText: "统计每个目录有多少文件", Tooltip: "触发 stat_file 批量"},
			{ID: "p_export", Label: "📤 导出结果", UserText: "把列表导出为 json", Tooltip: "导出结构化结果"},
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
		// 预设：触发 length 截断后用户恢复动作
		Presets: []MockPreset{
			{ID: "p_clear_history", Label: "🧹 清空上下文", UserText: "清空上下文", Tooltip: "清空历史会话重试"},
			{ID: "p_split", Label: "✂️ 分两段问", UserText: "分段总结这段", Tooltip: "分两轮问"},
			{ID: "p_shorter", Label: "📝 简短回答", UserText: "简短回答 100 字内", Tooltip: "限制 max_tokens"},
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
		// 预设：打招呼后用户可能立刻开问
		Presets: []MockPreset{
			{ID: "p_greet_files", Label: "📁 看看文件", UserText: "有哪些视频文件", Tooltip: "跳到 list_files_query"},
			{ID: "p_greet_help", Label: "❓ 你能做什么", UserText: "你能做什么", Tooltip: "跳到 default_friendly"},
			{ID: "p_greet_encrypt", Label: "🔒 加密视频", UserText: "加密这个视频", Tooltip: "跳到 encrypt_video"},
		},
	}
}
