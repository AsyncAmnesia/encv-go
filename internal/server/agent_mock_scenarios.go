// internal/server/agent_mock_scenarios.go
//
// 12 个内置 MockScenario 定义（v1 剧本，Go 字面量版本）。
//
// ⚠️ DEPRECATED — 剧本外置 spec 已生效。
//
// 当前状态：保留作为 fallback。
//   - 当 agent_settings.mock_scenarios_dir 为空时，loader 注入此 12 个剧本
//   - 当 mock_scenarios_dir 有 YAML 时，YAML 优先，此文件内容被覆盖
//   - 当 mock_scenarios_dir 解析失败时，loader 也注入此 fallback
//
// 推荐迁移路径：
//   1. 把本文件中每个 scenarioDefaultFriendly() 等函数转写为 YAML
//   2. 放到 internal/server/mock_scenarios/builtin/01_xxx.yaml
//   3. 验证通过后（CI 红线测试 + 启动 log）即可禁用此文件
//   4. 详见 internal/server/mock_scenarios/SCHEMA.md
//
// 设计原则（v1 设计，仍适用）：
//   - 每个剧本是独立函数返回 *MockScenario，便于单测直接调用验证事件序列
//   - 所有事件类型与真实 OpenAI 路径输出字节级一致（前端 0 改动）
//   - 延迟/事件顺序严格按 spec.md 编排，覆盖 [真机问题] / [reasoning 排序] / [并行工具] 等代码分支
//
// 参考：
//   - Spec: /workspace/.trae/specs/agent-mock-mode/spec.md §Requirement: 内置剧本清单
//   - 迁移指南: /workspace/.trae/specs/externalize-mock-scenarios/spec.md
package server

import (
	"strings"
)

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
// 用途：触发 list_mounts → list_files → list_files → read_file → markdown
// 总结，验证工具调用结构化渲染 + 真实多步流程 + 真实文件系统数据。
// 触发："有哪些文件" / "有哪些视频" 等。
//
// ⚠️ 重要：所有 args.rel_path 必须是 /storage/emulated/0/ 下真实存在的路径，
// 否则 execute_real=true 的真实 handler 会返 readdir_failed 真错误（与硬编码
// 的假数据混在一起，看起来"剧本又造假了"）。

func scenarioListFilesQuery() *MockScenario {
	return &MockScenario{
		ID:          "list_files_query",
		Description: "5 步真实流程：mounts → files → files → read_file → markdown 总结",
		// execute_real=true 标记：list_mounts / list_files / read_file 是只读工具，
		// 在 mock 模式下也应实际执行（结果用真实文件系统数据），避免"剧本编造"。
		Keywords: []string{
			// 中文
			"有哪些文件", "有什么文件", "视频", "目录", "列一下", "01-plain-media", "看看文件", "看下文件",
			// 英文（小写，Match 函数统一转小写后 Contains 匹配）
			"video", "movie", "files", "list files", "list",
		},
		Steps: []MockStep{
			// Step 1: stream_start + 开场白（markdown: 标题）—— 立即展示，用户已等待
			{DelayMs: 0, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]interface{}{
					"scenario": "list_files_query",
				}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "## 查看你的媒体库\n\n"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "先列出**挂载点**，再递归到 `01-plain-media` 看视频文件。\n\n"}},
			}},

			// Step 2: list_mounts（execute_real=true → 真实读 mounts 配置）
			{DelayMs: 1200, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_mount",
					"name":         "list_mounts",
					"args":         "{}",
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "readOnly",
					"execute_real": true,
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_mount",
					"status": "running",
				}},
			}},

			// Step 3: list_mounts 结果（被真实结果覆盖，hardcode 仅作 fallback）
			{DelayMs: 1000, Events: []MockEvent{
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_mount",
					"status": "success",
				}},
				{Type: "tool_result", Data: map[string]interface{}{
					"id":         "call_mount",
					"name":       "list_mounts",
					"result":     `{"count":1,"items":[{"id":"serving","path":"/storage/emulated/0","name":"serving"}]}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 5,
				}},
			}},

			// Step 4: 过渡文本 + list_files(/01-plain-media) 调真实
			{DelayMs: 800, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "已找到挂载点 `serving` → `/storage/emulated/0`。继续递归到 `01-plain-media`...\n\n"}},
			}},
			{DelayMs: 1000, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_files1",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"/01-plain-media"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "readOnly",
					"execute_real": true,
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_files1",
					"status": "running",
				}},
			}},
			{DelayMs: 900, Events: []MockEvent{
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_files1",
					"status": "success",
				}},
				{Type: "tool_result", Data: map[string]interface{}{
					"id":         "call_files1",
					"name":       "list_files",
					"result":     `{"files":[{"name":"audio","is_dir":true},{"name":"document","is_dir":true},{"name":"image","is_dir":true},{"name":"video","is_dir":true}]}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 8,
				}},
			}},

			// Step 5: 进入 video 子目录
			{DelayMs: 800, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "`01-plain-media` 下有 4 个子目录（`audio` / `document` / `image` / `video`）。点开 `video` 看一下...\n\n"}},
			}},
			{DelayMs: 1000, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_files2",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"/01-plain-media/video"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "readOnly",
					"execute_real": true,
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_files2",
					"status": "running",
				}},
			}},
			{DelayMs: 900, Events: []MockEvent{
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_files2",
					"status": "success",
				}},
				{Type: "tool_result", Data: map[string]interface{}{
					"id":         "call_files2",
					"name":       "list_files",
					"result":     `{"files":[{"name":"comedy.mkv","size":136,"is_dir":false},{"name":"sample.mp4","size":22261,"is_dir":false}]}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 6,
				}},
			}},

			// Step 6: 读 video 里的 sample.mp4 文件元数据
			{DelayMs: 1000, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_read",
					"name":         "read_file",
					"args":         `{"mount_id":"serving","rel_path":"/01-plain-media/document/notes.txt","max_bytes":500}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "readOnly",
					"execute_real": true,
				}},
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_read",
					"status": "running",
				}},
			}},
			{DelayMs: 900, Events: []MockEvent{
				{Type: "tool_status", Data: map[string]interface{}{
					"id":     "call_read",
					"status": "success",
				}},
				{Type: "tool_result", Data: map[string]interface{}{
					"id":         "call_read",
					"name":       "read_file",
					"result":     `{"content":"# Mock 媒体库说明\n\n这是 encv-go 沙箱 mock 模式测试用的文件。\n存放在 /storage/emulated/0/01-plain-media/ 目录下。\n\n- video/  : sample.mp4 (22KB), comedy.mkv (136B)\n- document/: notes.txt (1187B)\n- audio/  : music.mp3 (45KB)\n- image/   : screenshot.png\n","mimeType":"text/plain","size":1187}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 4,
				}},
			}},

			// Step 7: 动态总结（从 tool_result 真实数据取值——文件名/大小全部来自 handler 返回值）
			// 使用 text_delta_templated 事件类型，{%id:field%} 占位符在 Run 时替换为真实数据
			{DelayMs: 1500, Events: []MockEvent{
				{Type: "text_delta_templated", Data: map[string]interface{}{"text": "### 你的媒体库概览\n\n"}},
			{Type: "text_delta_templated", Data: map[string]interface{}{"text":
				"已完成对 `01-plain-media` 的递归扫描。\n\n"+
				"**视频文件**：{%call_files2:files%}\n\n"+
				"**子目录**：{%call_files1:items%}\n\n"+
				"**挂载点**：{%call_mount:mounts%}\n\n",
			}},
			{Type: "text_delta_templated", Data: map[string]interface{}{
				"text":
					"**读取文件**：{%call_read:error%}\n\n"+
					"**下一步建议**（点击 chip 直接执行）：\n\n"+
					"1. 加密视频文件\n2. 查看文档完整内容\n3. 切换到搜索剧本\n",
			}},
			}},

			// Step 8: 结束
			{DelayMs: 500, Events: []MockEvent{
				{Type: "stream_end", Data: map[string]interface{}{
					"finishReason": "stop",
					"usage":        map[string]int{"totalTokens": 618},
				}},
			}},
		},
		// 预设：用户看了媒体库列表后最可能的 3 个后续动作
		//   - p_qq/p_sub：进入更深的子目录（实际触发 list_files_query 递归）
		//   - p_search：跳到 multi_step_search 剧本
		//   - p_encrypt：跳到 encrypt_video 剧本（写操作）
		Presets: []MockPreset{
			{ID: "p_video", Label: "🎬 视频文件", UserText: "查看 video 子目录", Tooltip: "进入 01-plain-media/video 看视频文件列表", Icon: "🎬"},
			{ID: "p_doc", Label: "📄 文档", UserText: "查看 document 子目录", Tooltip: "进入 01-plain-media/document 看文档列表", Icon: "📄"},
			{ID: "p_search", Label: "🔍 搜索字幕", UserText: "搜索带字幕的视频", Tooltip: "触发 multi_step_search 剧本", Icon: "🔍"},
			{ID: "p_encrypt", Label: "🔒 加密 sample.mp4", UserText: "加密 sample.mp4", Tooltip: "触发 encrypt_video 剧本（需用户确认）", Icon: "🔒"},
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
					"args":         `{"input_paths":["/01-plain-media/video/sample.mp4"],"output_path":"/01-plain-media/video/sample.enc"}`,
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
					"kind":         "readOnly",
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
					"args":         `{"mount_id":"serving","rel_path":"01-plain-media"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "readOnly",
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
					"result":     `{"files":[{"name":"audio","is_dir":true},{"name":"document","is_dir":true},{"name":"image","is_dir":true},{"name":"video","is_dir":true}]}`,
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
					"args":         `{"mount_id":"serving","rel_path":"01-plain-media/video"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "readOnly",
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
					"result":     `{"files":[{"name":"sample.mp4","size":22261,"is_dir":false},{"name":"comedy.mkv","size":136,"is_dir":false}]}`,
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
						{ID: "p_stat", Label: "📊 看文件大小", UserText: "01-plain-media/video/sample.mp4 多大", Tooltip: "触发 stat_file 详细输出"},
						{ID: "p_encrypt_found", Label: "🔒 加密这个文件", UserText: "加密 01-plain-media/video/sample.mp4", Tooltip: "跳到 encrypt_video"},
						{ID: "p_done", Label: "✅ 结束搜索", UserText: "好的，不用再找了", Tooltip: "结束本轮搜索"},
					},
				}},
			}},
			// Round 3: stat_file（execute_real=true — 真实文件 stat）
			{DelayMs: 300, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_search_3",
					"name":         "stat_file",
					"args":         `{"mount_id":"serving","rel_path":"01-plain-media/video/sample.mp4"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "readOnly",
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
					"result":     `{"name":"sample.mp4","size":22261,"is_dir":false,"mtime":"2024-01-15T10:30:00Z"}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 8,
				}},
			}},
			{DelayMs: 200, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "搜索完成，结果详见上方工具返回。\n"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "文件信息详见上方工具结果。"}},
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
			{ID: "p_init_listing", Label: "📂 先列目录", UserText: "列出 01-plain-media 目录", Tooltip: "从入口开始"},
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
			{ID: "p_simpler", Label: "📝 简化问题", UserText: "列出 01-plain-media", Tooltip: "换成简单 list_files_query"},
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
					"args":         `{"mount_id":"serving","rel_path":"01-plain-media/video","max_entries":"50","filter":{"ext":[".mp4",".mkv"],"min_size":1024}}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "readOnly",
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
			{ID: "p_no_filter", Label: "📂 不用 filter", UserText: "列出 01-plain-media/video 全部文件", Tooltip: "去掉 filter/max_entries"},
			{ID: "p_smaller_size", Label: "💎 找大于 1MB 的", UserText: "01-plain-media/video 找大于 1MB 的视频", Tooltip: "min_size 调大"},
			{ID: "p_different_ext", Label: "🎞️ 改成 mkv", UserText: "01-plain-media/video 找 mkv 视频", Tooltip: "改 ext 过滤"},
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
					"args":         `{"mount_id":"serving","rel_path":"01-plain-media/video"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "readOnly",
					"execute_real": true,
				}},
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_par_2",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"01-plain-media/audio"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "readOnly",
					"execute_real": true,
				}},
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "call_par_3",
					"name":         "list_files",
					"args":         `{"mount_id":"serving","rel_path":"01-plain-media/document"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "readOnly",
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
			{ID: "p_only_movies", Label: "🎬 只看视频", UserText: "只列 video 目录", Tooltip: "缩减范围"},
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

// ════════════════════════════════════════════════════════════════
// 13. complex_workflow — 超长输出繁杂剧本（流式渲染+折叠态+Footer 综合测试）
// ════════════════════════════════════════════════════════════════
//
// 用途：验证真正逐步流式渲染、OperationCard 折叠态、独立 Footer、
//       长文本滚动性能。包含 6 个 tool_call（混合 readOnly/fileChange/command）、
//       10+ 段 text_delta、总时长 ~20s @ 1x 速。
//
// 触发："帮我全面分析" / "详细分析" / "完整报告" / "comprehensive" / "繁杂"

func scenarioComplexWorkflow() *MockScenario {
	return &MockScenario{
		ID:          "complex_workflow",
		Description: "超长多步工作流：6 tool_call + 10 text_delta — 验证流式渲染/折叠/Footer/滚动",
		Keywords:    []string{"帮我全面分析", "详细分析", "完整报告", "comprehensive", "繁杂", "深度诊断", "全量扫描"},
		Steps: []MockStep{
			// ── Phase 1：开场 + 第一个工具调用 ──
			{DelayMs: 0, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]interface{}{"scenario": "complex_workflow"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "## 系统全面诊断报告\n\n"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "正在对当前环境进行**多维度深度扫描**，请稍候...\n\n"}},
			}},
			{DelayMs: 1200, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id":           "cw_mount", "name": "list_mounts", "args": "{}",
					"auto_run": true, "needsConfirm": false, "kind": "readOnly", "execute_real": true,
				}},
				{Type: "tool_status", Data: map[string]interface{}{"id": "cw_mount", "status": "running"}},
			}},
			{DelayMs: 1000, Events: []MockEvent{
				{Type: "tool_status", Data: map[string]interface{}{"id": "cw_mount", "status": "success"}},
				{Type: "tool_result", Data: map[string]interface{}{
					"id": "cw_mount", "name": "list_mounts",
					"result": `{"count":2,"items":[{"id":"serving","path":"/storage/emulated/0","name":"serving"},{"id":"sdcard","path":"/storage/emulated/0/Android/data","name":"sdcard"}]}`,
					"isError": false, "status": "success", "durationMs": 8,
				}},
			}},

			// ── Phase 2：递归文件扫描（3 层）──
			{DelayMs: 800, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "### 挂载点状态\n\n检测到 **2 个挂载点**，开始递归扫描...\n\n"}},
			}},
			{DelayMs: 1000, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id": "cw_root", "name": "list_files",
					"args": `{"mount_id":"serving","rel_path":"/01-plain-media"}`,
					"auto_run": true, "needsConfirm": false, "kind": "readOnly", "execute_real": true,
				}},
				{Type: "tool_status", Data: map[string]interface{}{"id": "cw_root", "status": "running"}},
			}},
			{DelayMs: 900, Events: []MockEvent{
				{Type: "tool_status", Data: map[string]interface{}{"id": "cw_root", "status": "success"}},
				{Type: "tool_result", Data: map[string]interface{}{
					"id": "cw_root", "name": "list_files",
					"result":     `{"count":4,"files":[{"name":"video","is_dir":true},{"name":"document","is_dir":true},{"name":"audio","is_dir":true},{"name":"image","is_dir":true}]}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 12,
				}},
			}},
			{DelayMs: 800, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id": "cw_video", "name": "list_files",
					"args": `{"mount_id":"serving","rel_path":"/01-plain-media/video"}`,
					"auto_run": true, "needsConfirm": false, "kind": "readOnly", "execute_real": true,
				}},
				{Type: "tool_status", Data: map[string]interface{}{"id": "cw_video", "status": "running"}},
			}},
			{DelayMs: 900, Events: []MockEvent{
				{Type: "tool_status", Data: map[string]interface{}{"id": "cw_video", "status": "success"}},
				{Type: "tool_result", Data: map[string]interface{}{
					"id": "cw_video", "name": "list_files",
					"result": `{"files":[{"name":"sample.mp4","size":22261,"is_dir":false},{"name":"comedy.mkv","size":136,"is_dir":false}]}`,
					"isError": false, "status": "success", "durationMs": 15,
				}},
			}},

			// ── Phase 3：读取文档文件 ──
			{DelayMs: 800, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "### 文件系统详情\n\n"}},
				{Type: "text_delta_templated", Data: map[string]interface{}{
				"text":
					"根目录下发现 **4** 个子目录。\n\n"+
					"**视频目录内容**：{%cw_video:files%}\n\n"+
					"正在读取文档文件...\n\n",
			}},
			}},
			{DelayMs: 1000, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id": "cw_read_cfg", "name": "read_file",
					"args": `{"mount_id":"serving","rel_path":"/01-plain-media/document/notes.txt","max_bytes":2000}`,
					"auto_run": true, "needsConfirm": false, "kind": "readOnly", "execute_real": true,
				}},
				{Type: "tool_status", Data: map[string]interface{}{"id": "cw_read_cfg", "status": "running"}},
			}},
			{DelayMs: 800, Events: []MockEvent{
				{Type: "tool_status", Data: map[string]interface{}{"id": "cw_read_cfg", "status": "success"}},
				{Type: "tool_result", Data: map[string]interface{}{
					"id": "cw_read_cfg", "name": "read_file",
					"result": `{"content":"# Mock 媒体库说明\n\n这是 encv-go 沙箱 mock 模式测试用的文件。\n- video/: sample.mp4, comedy.mkv\n- document/: notes.txt\n- audio/: music.mp3\n","mimeType":"text/plain","size":1187}`,
					"isError": false, "status": "success", "durationMs": 5,
				}},
			}},

			// ── Phase 4：命令执行（磁盘空间检查）──
			{DelayMs: 800, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "### 系统资源\n\n正在检查磁盘空间和内存使用情况...\n\n"}},
			}},
			{DelayMs: 1000, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id": "cw_df", "name": "shell_command",
					"args": `{"command":"df -h /storage/emulated/0 && free -m"}`,
					"auto_run": true, "needsConfirm": false, "kind": "command", "execute_real": false,
				}},
				{Type: "tool_status", Data: map[string]interface{}{"id": "cw_df", "status": "running"}},
			}},
			{DelayMs: 1200, Events: []MockEvent{
				{Type: "tool_status", Data: map[string]interface{}{"id": "cw_df", "status": "success"}},
				{Type: "tool_result", Data: map[string]interface{}{
					"id": "cw_df", "name": "shell_command",
					"result": `{"output":"Filesystem      Size  Used Avail Use%/mnt\n/dev/block/bootdevice  118G   42G   76G   36%\n\n              total        used        free        shared    buff/cache   available\nMem:          8192        3521        2145         512         2526        4321\nSwap:         4096         128         3968","exit_code":0}`,
					"isError": false, "status": "success", "durationMs": 320,
				}},
			}},

			// ── Phase 5：写操作（需确认）──
			{DelayMs: 800, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]interface{}{"text": "### 建议操作\n\n基于以上扫描结果，建议执行以下优化操作：\n\n"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "1. **清理缓存**：释放约 2.5GB 磁盘空间\n"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "2. **生成索引**：为 video 目录建立文件索引\n"}},
				{Type: "text_delta", Data: map[string]interface{}{"text": "3. **备份配置**：导出当前配置快照\n\n"}},
			}},
			{DelayMs: 600, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]interface{}{
					"id": "cw_backup", "name": "write_file",
					"args": `{"mount_id":"serving","rel_path":"/01-plain-media/backup-index-20250607.json","content":"{\"scan_time\":\"2025-06-07T12:00:00Z\",\"files_found\":4,\"total_size_bytes\":68781}"}`,
					"auto_run": false, "needsConfirm": true, "kind": "fileChange", "execute_real": false,
				}},
				{Type: "tool_status", Data: map[string]interface{}{"id": "cw_backup", "status": "running"}},
			}},

			// ── Phase 6：动态总结 + 结束 ──
			{DelayMs: 1500, Events: []MockEvent{
				{Type: "text_delta_templated", Data: map[string]interface{}{
					"text": "## 诊断总结\n\n",
				}},
				{Type: "text_delta_templated", Data: map[string]interface{}{
				"text":
					"| 维度 | 结果 |\n|------|------|\n"+
					"| 挂载点 | {%cw_root:count} 个子目录 |\n"+
					"| 视频文件 | {%cw_video:files%} |\n"+
					"| 磁盘使用 | 42G / 118G (36%%) |\n"+
					"| 内存使用 | 3.5G / 8G (43%%) |\n"+
					"| 待确认操作 | 备份索引文件（需用户批准）|\n\n"+
					"**下一步建议**（点击 chip 直接执行）：\n\n"+
					"1. 加密所有视频文件\n2. 清理临时缓存\n3. 生成完整文件索引\n4. 导出 PDF 报告\n",
			}},
			}},
			{DelayMs: 500, Events: []MockEvent{
				{Type: "stream_end", Data: map[string]interface{}{
					"finishReason": "stop",
					"usage":        map[string]int{"totalTokens": 1247},
				}},
			}},
		},
		Presets: []MockPreset{
			{ID: "p_cw_encrypt_all", Label: "🔒 全部加密", UserText: "加密所有视频文件", Tooltip: "触发 encrypt_video 剧本批量模式"},
			{ID: "p_cw_clean", Label: "🧹 清理缓存", UserText: "清理临时缓存释放空间", Tooltip: "执行 shell_command 清理"},
			{ID: "p_cw_index", Label: "📋 生成索引", UserText: "生成文件索引", Tooltip: "遍历所有目录生成 JSON 索引"},
			{ID: "p_cw_pdf", Label: "📄 导出PDF", UserText: "导出诊断报告为PDF", Tooltip: "将本次结果格式化为 PDF 文档"},
		},
	}
}

// ════════════════════════════════════════════════════════════════
//  v2 场景（agent-tools-scenarios-v2 spec §三.5）
// ════════════════════════════════════════════════════════════════
//
// 与 v1 区别：
//   - 8 个场景全部使用新工具名（search_files / get_metadata / command_run / edit_metadata / batch_rename）
//   - 部分场景声明 Rounds > 1，触发 v2 多轮状态机
//   - 2 个场景声明 Branches，触发 v2 分支选择
//   - 全部 tool_call 标记 execute_real=true，调用真实工具实现
// ────────────────────────────────────────────────────────────────

// allMockScenarios 合并 v1 + v2 场景，供 MockEngine.Match 使用。
// v1 场景通过 scenarioXxx() 函数构造（不存为变量，因为 v1 场景按需懒初始化）。
// v2 场景是预先构造的 *MockScenario 列表。
func allMockScenarios() []*MockScenario {
	out := make([]*MockScenario, 0, 20)
	// v1
	out = append(out,
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
		scenarioComplexWorkflow(),
	)
	// v2
	out = append(out, mockScenariosV2...)
	return out
}
