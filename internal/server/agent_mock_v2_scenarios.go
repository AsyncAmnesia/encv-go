package server

// ────────────────────────────────────────────────────────────────────
// 剧本 v2 场景（agent-tools-scenarios-v2 spec §三.5）— Go 字面量版本
// ────────────────────────────────────────────────────────────────────
//
// ⚠️ DEPRECATED — 剧本外置 spec 已生效。
//
// 8 个 v2 场景，覆盖：
//   - 单轮线性（search_*）: 演示 search_files 各种 query 形态
//   - 多轮向导（edit_metadata_wizard）: 4 轮用户输入
//   - 多轮 + 预览确认（batch_rename_with_preview）: dry_run → 确认 → 执行
//   - 分支选择（branch_encrypt_or_decrypt / branch_video_or_audio）
//   - 真实 shell（command_run_ffprobe）: command_run 受限 shell 输出
//
// 当前状态：保留作为 fallback。推荐迁移路径：
//   1. 把下方 mockScenariosV2 列表的 8 个剧本转写为 YAML
//   2. 放到 internal/server/mock_scenarios/v2/01_xxx.yaml
//   3. 验证通过后即可禁用此 fallback
//   4. 详见 internal/server/mock_scenarios/SCHEMA.md
//
// 所有 v2 场景都把 Rounds/RoundContext/Branches 等字段填齐；
// 引擎通过 scenario.Rounds > 0 自动判定走 v2 路径。
//
// 注：PauseForUser / SetContext / BranchChoice / BranchID 是 MockStep 级字段，
// 写在 Step 字面量上（不是 Events 内的元素）。
// ────────────────────────────────────────────────────────────────────

// mockScenariosV2 是 8 个 v2 场景（v1 场景继续保留在 mockScenarios）。
// 顺序对应 spec §三.5 表格。
var mockScenariosV2 = []*MockScenario{
	// ─── 1. 递归 + glob + size > 100MB ─────────────────────────
	{
		ID:          "search_recursive_mp4",
		Description: "搜索 100MB 以上的 mp4（递归 + glob）",
		Keywords:    []string{"search_recursive_mp4"},
		Rounds:      1,
		Steps: []MockStep{
			{RoundIdx: 0, DelayMs: 10, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]any{
					"scenario": "search_recursive_mp4",
				}},
				{Type: "text_delta", Data: map[string]any{
					"text": "正在搜索大于 100MB 的 MP4 文件…",
				}},
			}},
			{RoundIdx: 0, DelayMs: 30, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]any{
					"id":           "call_srm_1",
					"name":         "search_files",
					"args":         `{"expression":{"and":[{"type":"name_glob","value":"*.mp4"},{"type":"size_gt","value":104857600}]},"recursive":true,"max_results":20}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileSearch",
					"execute_real": true,
				}},
			}},
			{RoundIdx: 0, DelayMs: 50, Events: []MockEvent{
				{Type: "tool_result", Data: map[string]any{
					"id":         "call_srm_1",
					"name":       "search_files",
					"result":     `{"matches":[{"path":"Movies/2024/big.mp4","size":2147483648,"mtime":"2024-08-12T10:00:00Z"}],"count":1}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 120,
				}},
				{Type: "text_delta", Data: map[string]any{
					"text": "找到 1 个文件：Movies/2024/big.mp4",
				}},
				{Type: "stream_end", Data: map[string]any{
					"finishReason": "stop",
				}},
			}},
		},
	},

	// ─── 2. 复合 AND：size_gt + mtime_after + ext_eq ─────────────
	{
		ID:          "search_logical_query",
		Description: "复合 AND 查询（size + mtime + ext）",
		Keywords:    []string{"search_logical_query"},
		Rounds:      1,
		Steps: []MockStep{
			{RoundIdx: 0, DelayMs: 10, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]any{
					"scenario": "search_logical_query",
				}},
				{Type: "text_delta", Data: map[string]any{
					"text": "将查询条件组合为 AND 谓词…",
				}},
			}},
			{RoundIdx: 0, DelayMs: 30, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]any{
					"id":           "call_slq_1",
					"name":         "search_files",
					"args":         `{"expression":{"and":[{"type":"size_gt","value":1048576},{"type":"mtime_after","value":"2024-01-01T00:00:00Z"},{"type":"ext_eq","value":".log"}]}}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileSearch",
					"execute_real": true,
				}},
			}},
			{RoundIdx: 0, DelayMs: 50, Events: []MockEvent{
				{Type: "tool_result", Data: map[string]any{
					"id":         "call_slq_1",
					"name":       "search_files",
					"result":     `{"matches":[{"path":"logs/app.log","size":5242880,"mtime":"2024-09-01T08:00:00Z"}],"count":1}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 85,
				}},
				{Type: "text_delta", Data: map[string]any{
					"text": "匹配 1 条：logs/app.log",
				}},
				{Type: "stream_end", Data: map[string]any{
					"finishReason": "stop",
				}},
			}},
		},
	},

	// ─── 3. content_regex：匹配文件内容 ────────────────────────
	{
		ID:          "search_content_regex",
		Description: "用正则搜索文件内容（ERROR.*timeout）",
		Keywords:    []string{"search_content_regex"},
		Rounds:      1,
		Steps: []MockStep{
			{RoundIdx: 0, DelayMs: 10, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]any{
					"scenario": "search_content_regex",
				}},
				{Type: "text_delta", Data: map[string]any{
					"text": "将扫描所有 .log 文件，匹配 ERROR.*timeout 模式…",
				}},
			}},
			{RoundIdx: 0, DelayMs: 30, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]any{
					"id":           "call_scr_1",
					"name":         "search_files",
					"args":         `{"expression":{"and":[{"type":"content_regex","value":"ERROR.*timeout"},{"type":"ext_eq","value":".log"}]},"max_results":50}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileSearch",
					"execute_real": true,
				}},
			}},
			{RoundIdx: 0, DelayMs: 50, Events: []MockEvent{
				{Type: "tool_result", Data: map[string]any{
					"id":         "call_scr_1",
					"name":       "search_files",
					"result":     `{"matches":[{"path":"logs/err.log","size":2048,"matched_line":"ERROR: connection timeout after 30s"}],"count":1}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 200,
				}},
				{Type: "text_delta", Data: map[string]any{
					"text": "命中 1 处：logs/err.log: ERROR: connection timeout after 30s",
				}},
				{Type: "stream_end", Data: map[string]any{
					"finishReason": "stop",
				}},
			}},
		},
	},

	// ─── 4. 4 轮编辑元数据向导 ───────────────────────────────
	//   Round 0: 选文件
	//   Round 1: 选字段
	//   Round 2: 输入新值
	//   Round 3: 确认
	{
		ID:          "edit_metadata_wizard",
		Description: "4 轮元数据编辑向导（选文件→选字段→输入值→确认）",
		Keywords:    []string{"edit_metadata_wizard"},
		Rounds:      4,
		Presets: []MockPreset{
			{ID: "wiz_start", Label: "开始编辑元数据", UserText: "开始"},
		},
		Steps: []MockStep{
			// Round 0: 选文件
			{RoundIdx: 0, DelayMs: 10, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]any{
					"text": "请选择要编辑的文件：",
				}},
			}},
			{RoundIdx: 0, DelayMs: 10, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]any{
					"text": "1) Movies/a.mp4\n2) Movies/b.mp4",
				}},
				{Type: "mock_presets", Data: map[string]any{
					"scenario": "edit_metadata_wizard",
					"phase":    "select_file",
					"presets": []MockPreset{
						{ID: "sel_a", Label: "Movies/a.mp4", UserText: "选 a"},
						{ID: "sel_b", Label: "Movies/b.mp4", UserText: "选 b"},
					},
				}},
			}, PauseForUser: true, SetContext: map[string]any{
				"selected_file": "Movies/a.mp4",
			}},

			// Round 1: 选字段
			{RoundIdx: 1, DelayMs: 10, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]any{
					"text": "你想编辑哪个字段？",
				}},
			}},
			{RoundIdx: 1, DelayMs: 10, Events: []MockEvent{
				{Type: "mock_presets", Data: map[string]any{
					"scenario": "edit_metadata_wizard",
					"phase":    "select_field",
					"presets": []MockPreset{
						{ID: "f_title", Label: "title", UserText: "title"},
						{ID: "f_year", Label: "year", UserText: "year"},
						{ID: "f_genre", Label: "genre", UserText: "genre"},
					},
				}},
			}, PauseForUser: true, SetContext: map[string]any{
				"selected_field": "title",
			}},

			// Round 2: 输入新值
			{RoundIdx: 2, DelayMs: 10, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]any{
					"text": "请输入新值：",
				}},
			}, PauseForUser: true, SetContext: map[string]any{
				"new_value": "My New Title",
			}},

			// Round 3: 确认 + 执行
			{RoundIdx: 3, DelayMs: 10, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]any{
					"text": "将编辑 Movies/a.mp4 的 title 字段，值为「My New Title」。确认执行？",
				}},
			}},
			{RoundIdx: 3, DelayMs: 30, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]any{
					"id":           "call_emw_1",
					"name":         "edit_metadata",
					"args":         `{"path":"Movies/a.mp4","field":"title","value":"My New Title"}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "metadataEdit",
					"execute_real": true,
				}},
			}},
			{RoundIdx: 3, DelayMs: 50, Events: []MockEvent{
				{Type: "tool_result", Data: map[string]any{
					"id":         "call_emw_1",
					"name":       "edit_metadata",
					"result":     `{"ok":true,"path":"Movies/a.mp4","field":"title","old":"Old Title","new":"My New Title"}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 30,
				}},
				{Type: "text_delta", Data: map[string]any{
					"text": "✓ 已更新 title 字段。",
				}},
				{Type: "stream_end", Data: map[string]any{
					"finishReason": "stop",
				}},
			}},
		},
	},

	// ─── 5. dry_run → 确认 → 执行（batch_rename_with_preview） ─────
	{
		ID:          "batch_rename_with_preview",
		Description: "批量重命名（dry_run 预览 → 确认 → 真实执行）",
		Keywords:    []string{"batch_rename_with_preview"},
		Rounds:      3,
		Steps: []MockStep{
			// Round 0: dry_run 预览
			{RoundIdx: 0, DelayMs: 10, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]any{
					"scenario": "batch_rename_with_preview",
				}},
				{Type: "text_delta", Data: map[string]any{
					"text": "正在预览批量重命名（dry_run=true）…",
				}},
			}},
			{RoundIdx: 0, DelayMs: 30, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]any{
					"id":           "call_br_1",
					"name":         "batch_rename",
					"args":         `{"pattern":"S01E{{n}}","source_glob":"*.mkv","dry_run":true}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRename",
					"execute_real": true,
				}},
			}},
			{RoundIdx: 0, DelayMs: 50, Events: []MockEvent{
				{Type: "tool_result", Data: map[string]any{
					"id":         "call_br_1",
					"name":       "batch_rename",
					"result":     `{"dry_run":true,"preview":[{"from":"ep1.mkv","to":"S01E01.mkv"},{"from":"ep2.mkv","to":"S01E02.mkv"}],"count":2}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 45,
				}},
				{Type: "text_delta", Data: map[string]any{
					"text": "预览：2 个文件将重命名（ep1.mkv → S01E01.mkv 等）",
				}},
				{Type: "mock_presets", Data: map[string]any{
					"scenario": "batch_rename_with_preview",
					"phase":    "confirm",
					"presets": []MockPreset{
						{ID: "ok", Label: "✓ 确认执行", UserText: "确认"},
						{ID: "no", Label: "✗ 取消", UserText: "取消"},
					},
				}},
			}, PauseForUser: true, SetContext: map[string]any{
				"preview_count": 2,
			}},

			// Round 1: 真实执行
			{RoundIdx: 1, DelayMs: 10, Events: []MockEvent{
				{Type: "text_delta", Data: map[string]any{
					"text": "执行批量重命名…",
				}},
			}},
			{RoundIdx: 1, DelayMs: 30, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]any{
					"id":           "call_br_2",
					"name":         "batch_rename",
					"args":         `{"pattern":"S01E{{n}}","source_glob":"*.mkv","dry_run":false}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "fileRename",
					"execute_real": true,
				}},
			}},
			{RoundIdx: 1, DelayMs: 50, Events: []MockEvent{
				{Type: "tool_result", Data: map[string]any{
					"id":         "call_br_2",
					"name":       "batch_rename",
					"result":     `{"dry_run":false,"renamed":[{"from":"ep1.mkv","to":"S01E01.mkv"}],"count":2,"errors":0}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 80,
				}},
				{Type: "text_delta", Data: map[string]any{
					"text": "✓ 已重命名 2 个文件。",
				}},
				{Type: "stream_end", Data: map[string]any{
					"finishReason": "stop",
				}},
			}},
		},
	},

	// ─── 6. 分支选择：3 选 1（encrypt / decrypt / cancel） ─────
	{
		ID:          "branch_encrypt_or_decrypt",
		Description: "分支选择：加密 / 解密 / 取消",
		Keywords:    []string{"branch_encrypt_or_decrypt"},
		Rounds:      1,
		Branches: []Branch{
			{
				ID:              "encrypt",
				Label:           "🔒 加密",
				Description:     "对所选文件执行 AES-256-GCM 加密",
				Icon:            "🔒",
				TriggerKeywords: []string{"加密", "encrypt"},
				TriggerRegex:    "(?i)^enc(rypt)?$",
			},
			{
				ID:              "decrypt",
				Label:           "🔓 解密",
				Description:     "解密已加密文件",
				Icon:            "🔓",
				TriggerKeywords: []string{"解密", "decrypt"},
				TriggerRegex:    "(?i)^dec(rypt)?$",
			},
			{
				ID:              "cancel",
				Label:           "✗ 取消",
				Description:     "什么也不做",
				Icon:            "✗",
				TriggerKeywords: []string{"取消", "cancel", "no"},
				TriggerRegex:    "(?i)^(cancel|no|quit)$",
			},
		},
		Steps: []MockStep{
			{RoundIdx: 0, DelayMs: 10, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]any{
					"scenario": "branch_encrypt_or_decrypt",
				}},
				{Type: "text_delta", Data: map[string]any{
					"text": "请选择操作类型：",
				}},
			}, BranchChoice: true, BranchID: "choose_action"},
		},
	},

	// ─── 7. 多分支：视频 / 音频 / 其他 ───────────────────────
	//   - 视频 → ffprobe 提取元数据
	//   - 音频 → ffprobe 提取元数据
	//   - 其他 → get_metadata 读 sidecar
	{
		ID:          "branch_video_or_audio",
		Description: "按文件类型分支：视频 / 音频 / 其他",
		Keywords:    []string{"branch_video_or_audio"},
		Rounds:      1,
		Branches: []Branch{
			{
				ID:              "video",
				Label:           "🎬 视频",
				Description:     "用 ffprobe 提取 codec/duration",
				Icon:            "🎬",
				TriggerKeywords: []string{"视频", "video", "movie", "mp4", "mkv"},
				TriggerRegex:    "(?i)\\.(mp4|mkv|avi|mov)$",
			},
			{
				ID:              "audio",
				Label:           "🎵 音频",
				Description:     "用 ffprobe 提取 bitrate/samplerate",
				Icon:            "🎵",
				TriggerKeywords: []string{"音频", "audio", "mp3", "flac", "wav"},
				TriggerRegex:    "(?i)\\.(mp3|flac|wav|aac)$",
			},
			{
				ID:              "other",
				Label:           "📄 其他",
				Description:     "读 sidecar 元数据",
				Icon:            "📄",
				TriggerKeywords: []string{"其他", "other", "sidecar"},
			},
		},
		Steps: []MockStep{
			{RoundIdx: 0, DelayMs: 10, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]any{
					"scenario": "branch_video_or_audio",
				}},
				{Type: "text_delta", Data: map[string]any{
					"text": "请选择文件类型：",
				}},
			}, BranchChoice: true, BranchID: "choose_type"},
		},
	},

	// ─── 8. 受限 shell：ffprobe ─────────────────────────────
	//   演示 command_run 工具，受限 shell（仅允许 ffprobe / ffmpeg）真实输出。
	{
		ID:          "command_run_ffprobe",
		Description: "通过受限 shell 执行 ffprobe 提取元数据",
		Keywords:    []string{"command_run_ffprobe"},
		Rounds:      1,
		Steps: []MockStep{
			{RoundIdx: 0, DelayMs: 10, Events: []MockEvent{
				{Type: "stream_start", Data: map[string]any{
					"scenario": "command_run_ffprobe",
				}},
				{Type: "text_delta", Data: map[string]any{
					"text": "执行 ffprobe 提取元数据…",
				}},
			}},
			{RoundIdx: 0, DelayMs: 30, Events: []MockEvent{
				{Type: "tool_call", Data: map[string]any{
					"id":           "call_crf_1",
					"name":         "command_run",
					"args":         `{"cmd":"ffprobe","args":["-v","quiet","-print_format","json","-show_format","-show_streams","Movies/a.mp4"]}`,
					"auto_run":     true,
					"needsConfirm": false,
					"kind":         "shell",
					"execute_real": true,
				}},
			}},
			{RoundIdx: 0, DelayMs: 50, Events: []MockEvent{
				{Type: "tool_result", Data: map[string]any{
					"id":         "call_crf_1",
					"name":       "command_run",
					"result":     `{"stdout":"{\"streams\":[{\"codec_name\":\"h264\",\"duration\":\"120.0\"}],\"format\":{\"duration\":\"120.0\",\"bit_rate\":\"5000000\"}}","stderr":"","exitCode":0}`,
					"isError":    false,
					"status":     "success",
					"durationMs": 250,
				}},
				{Type: "text_delta", Data: map[string]any{
					"text": "✓ ffprobe 完成：codec=h264, duration=120s, bitrate=5Mbps",
				}},
				{Type: "stream_end", Data: map[string]any{
					"finishReason": "stop",
				}},
			}},
		},
	},
}
