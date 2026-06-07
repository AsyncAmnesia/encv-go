# Checklist — Agent Mock 模式

> 验证标准：每项必须为 ✅ 才能视为完成。

---

## Mock 引擎核心

- [ ] MockEngine 核心类型定义完整（MockEngine / MockScenario / MockStep / Match / Run）
- [ ] 12 个内置剧本全部实现（default_friendly / list_files_query / encrypt_video / read_and_summarize / multi_step_search / streaming_error / truncation_long_text / reasoning_chain / tool_call_with_args / multi_tool_parallel / context_exhausted / chinese_greeting）
- [ ] Match 优先级正确：精确 > 关键词 > 正则 > fallback
- [ ] 关键词匹配不区分大小写
- [ ] 正则编译失败不 panic
- [ ] builtin 模式无匹配 → 返回 default_friendly
- [ ] custom 模式无匹配 → 返回 nil（走真实 API）

## 时间模拟

- [ ] MockSpeed=1.0 误差 ≤ ±50ms
- [ ] MockSpeed=0.1 实际 delay = DelayMs / 0.1
- [ ] MockSpeed=10 实际 delay = DelayMs / 10
- [ ] MockSpeed=0 零延迟同步推完

## 错误模拟

- [ ] mid_stream_disconnect 推 2 text_delta 后 SSE close
- [ ] sse_corrupt_chunk 推损坏 JSON 后前端不 panic
- [ ] tool_call_missing_field 后端 slog.Warn 跳过
- [ ] timeout 30s 无 stream_end 前端 abort
- [ ] upstream_5xx 立即推 stream_status(5xx)

## 关键剧本（[真机问题] 验证）

- [ ] list_files_query 触发 2 个 tool_call + 2 个 tool_result
- [ ] list_files_query 工具调用渲染为 GroupedOperationMessage（不是裸 JSON）
- [ ] list_files_query 回答完整不截断（text_delta 总和 = 剧本预期）
- [ ] multi_step_search 验证 3 轮 tool_call 递归逻辑

## 集成到 handleAgentChat

- [ ] cfg.Agent.MockMode != "off" 时走 mock 路径
- [ ] 响应头含 X-Mock-Scenario
- [ ] 响应头含 X-Mock-Mode
- [ ] 第一个 SSE 事件含 mock: true 字段
- [ ] 真实 OpenAI 路径代码完全不变
- [ ] builtin fallback 失败 → 走真实
- [ ] custom fallback 失败 → 走真实

## Config 字段

- [ ] agent_settings.mock_mode（select: off/builtin/custom）
- [ ] agent_settings.mock_speed（number: 0-10, step 0.1）
- [ ] agent_settings.mock_scenarios（array of object）
- [ ] Settings 二级页能渲染新字段（沿用 ConfigFieldItem）
- [ ] 保存走 useConfig.saveConfig()

## 前端

- [ ] useAgent 暴露 isMockMode / mockScenario
- [ ] processSSE 解析响应头 + 第一个事件 mock: true
- [ ] AgentChat.vue 顶部显示「🧪 模拟」徽章
- [ ] 徽章 tooltip 显示 scenario ID
- [ ] 徽章样式沿用 StatusBadge idle tone + flaskOutline

## i18n

- [ ] agent.mockBadge / mockBadgeTooltip zh + en
- [ ] agent.mockMode / mockModeOff / mockModeBuiltin / mockModeCustom zh + en
- [ ] agent.mockSpeed / mockSpeedHelp zh + en
- [ ] settings.mockBuiltinHint / mockCustomHint / mockScenarios zh + en

## 编译 / 测试

- [ ] go build ./cmd/encv 0 错误
- [ ] go test ./internal/server/... 全部通过
- [ ] vue-tsc --noEmit 0 错误
- [ ] vite build 0 错误
- [ ] start-preview.sh 启动后 /health 返回 200

## 端到端验证

- [ ] 设置 mock_mode=builtin → 提问"有哪些视频文件" → 工具调用 = 结构化卡片
- [ ] 提问"触发超时" → 错误 toast 显示
- [ ] 设置 mock_speed=0.1 → SSE 明显变慢
- [ ] 自定义剧本（custom 模式）→ 用户输入命中关键词 → 触发自定义剧本
