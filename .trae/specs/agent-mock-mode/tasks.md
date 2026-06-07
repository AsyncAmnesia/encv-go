# Tasks

> 依赖说明：每个 Task 内部子项按顺序执行；不同 Task 之间若有依赖用「Task N 依赖」标注。

---

- [ ] Task 1: 实现 MockEngine 核心 + 内置 12 个剧本
  - 依赖：无
  - 涉及文件：`internal/server/agent_mock.go`（新建）
  - 步骤：
    - [ ] SubTask 1.1: 定义 `MockScenario` / `MockStep` / `MockEngine` 类型
    - [ ] SubTask 1.2: 实现 `Match(userText string) *MockScenario` —— 精确 > 关键词 > 正则 > fallback
    - [ ] SubTask 1.3: 实现 `Run(ctx, sess, stepCh chan<- agentLoopEvent, speed float64) error` —— 推 SSE 事件
    - [ ] SubTask 1.4: 实现 12 个内置剧本常量（`default_friendly` / `list_files_query` / `encrypt_video` / `read_and_summarize` / `multi_step_search` / `streaming_error` / `truncation_long_text` / `reasoning_chain` / `tool_call_with_args` / `multi_tool_parallel` / `context_exhausted` / `chinese_greeting`）
    - [ ] SubTask 1.5: 实现时间模拟（`DelayMs / speed`）+ 错误注入（`errorType` 字段处理）
  - 验证：编译通过；`go vet` 无警告

- [ ] Task 2: MockEngine 单元测试（30+ 用例）
  - 依赖：Task 1
  - 涉及文件：`internal/server/agent_mock_test.go`（新建）
  - 步骤：
    - [ ] SubTask 2.1: `TestMockEngine_Match` —— 4 种匹配优先级 + 大小写不敏感 + 正则失败容错 + builtin/custom fallback 差异
    - [ ] SubTask 2.2: `TestMockEngine_Run_DefaultFriendly` —— 验证 text_delta 数量
    - [ ] SubTask 2.3: `TestMockEngine_Run_ListFilesQuery` —— 验证 [真机问题] 修复：2 tool_call + 2 tool_result + 完整文本
    - [ ] SubTask 2.4: `TestMockEngine_Run_StreamingError` —— 验证 stream_status(error) 事件
    - [ ] SubTask 2.5: `TestMockEngine_Run_TruncationLongText` —— 验证单 chunk > 2000 字符
    - [ ] SubTask 2.6: `TestMockEngine_Run_ReasoningChain` —— 验证 reasoning_delta + text_delta 交替
    - [ ] SubTask 2.7: `TestMockEngine_Run_MultiToolParallel` —— 验证同 step 多 tool_call
    - [ ] SubTask 2.8: `TestMockEngine_Run_ContextExhausted` —— 验证 finishReason=length
    - [ ] SubTask 2.9: `TestMockEngine_Run_ChineseGreeting` —— 验证单字符 chunk
    - [ ] SubTask 2.10: `TestMockEngine_Run_EncryptVideo` / `ReadAndSummarize` / `MultiStepSearch` / `ToolCallWithArgs`
    - [ ] SubTask 2.11: `TestMockEngine_Run_Errors` —— 5 种错误类型（mid_stream_disconnect / sse_corrupt_chunk / tool_call_missing_field / timeout / upstream_5xx）
    - [ ] SubTask 2.12: `TestMockEngine_Speed` —— 4 种 speed（0 / 0.1 / 1.0 / 10）误差 ±50ms
  - 验证：`go test ./internal/server/... -run TestMockEngine -v` 全部通过

- [ ] Task 3: config 字段扩展
  - 依赖：无
  - 涉及文件：`internal/config/config.go`
  - 步骤：
    - [ ] SubTask 3.1: 在 `Agent` struct 加 `MockMode string` / `MockSpeed float64` / `MockScenarios []MockScenario`
    - [ ] SubTask 3.2: 字段默认值（`MockMode: "off"` / `MockSpeed: 1.0` / `MockScenarios: nil`）
    - [ ] SubTask 3.3: `Agent` struct 加注释（字段含义、JSON tag、schema 提示）
  - 验证：`go build` 通过

- [ ] Task 4: config.schema.json 扩展
  - 依赖：Task 3
  - 涉及文件：`config.schema.json`（或 `internal/config/schema.json`）
  - 步骤：
    - [ ] SubTask 4.1: 在 `agent_settings` 加 `mock_mode`（select: off/builtin/custom）
    - [ ] SubTask 4.2: 加 `mock_speed`（number: 0-10, step 0.1, default 1.0）
    - [ ] SubTask 4.3: 加 `mock_scenarios`（array of object）
  - 验证：Settings 二级页能渲染新字段

- [ ] Task 5: 集成到 handleAgentChat / handleAgentConfirm
  - 依赖：Task 1, Task 3
  - 涉及文件：`internal/server/agent_api.go`
  - 步骤：
    - [ ] SubTask 5.1: `handleAgentChat` 入口处增加 `if cfg.Agent.MockMode != "off"` 分支
    - [ ] SubTask 5.2: 实现 `s.streamMockScenario(c, sess, scenario, ...)` 内部函数
      - 调 `mockEngine.Run` 推事件
      - 设置响应头 `X-Mock-Scenario` / `X-Mock-Mode`
      - 第一个事件 `data` 加 `mock: true` 字段
    - [ ] SubTask 5.3: `handleAgentConfirm` 也走相同 mock 分支
    - [ ] SubTask 5.4: builtin 模式 fallback 失败 → 走真实；custom 模式 fallback 失败 → 也走真实（按 spec 约束）
  - 验证：手动 curl `POST /api/chat` + mock_mode=builtin + "有哪些视频文件" → 收到 `X-Mock-Scenario: list_files_query` 头

- [ ] Task 6: 端到端集成测试
  - 依赖：Task 5
  - 涉及文件：`internal/server/agent_api_test.go`（追加测试）
  - 步骤：
    - [ ] SubTask 6.1: `TestHandleAgentChat_MockBuiltin` —— 验证响应头 + 事件流
    - [ ] SubTask 6.2: `TestHandleAgentChat_MockCustom` —— 验证自定义剧本加载
    - [ ] SubTask 6.3: `TestHandleAgentChat_MockFallback` —— builtin 无匹配走 default_friendly
    - [ ] SubTask 6.4: `TestHandleAgentChat_MockResume` —— 验证 mock 路径也支持断点续传（EventCache 已自动写入）
  - 验证：`go test ./internal/server/... -run TestHandleAgentChat_Mock -v` 全部通过

- [ ] Task 7: useAgent 检测 mock 模式
  - 依赖：Task 5
  - 涉及文件：`app/encv-mobile/src/composables/useAgent.ts`
  - 步骤：
    - [ ] SubTask 7.1: 新增 `isMockMode = ref(false)` + `mockScenario = ref<string>('')`
    - [ ] SubTask 7.2: `processSSE` 解析响应头 `X-Mock-Mode` / `X-Mock-Scenario`
    - [ ] SubTask 7.3: 第一个 SSE 事件 `stream_start` 的 data 含 `mock: true` 时设置 `isMockMode = true`
    - [ ] SubTask 7.4: 导出 `isMockMode` / `mockScenario`（AgentChat.vue 读取显示徽章）
  - 验证：`vue-tsc --noEmit` 0 错误

- [ ] Task 8: AgentChat.vue 显示「模拟模式」徽章
  - 依赖：Task 7
  - 涉及文件：`app/encv-mobile/src/views/AgentChat.vue`
  - 步骤：
    - [ ] SubTask 8.1: 在 header 标题与 ContextIcon 之间新增 `<span class="mockBadge" v-if="isMockMode">{{ t('agent.mockBadge') }}</span>`
    - [ ] SubTask 8.2: 徽章加 `title` 属性 = `{{ t('agent.mockBadgeTooltip', { scenario: mockScenario }) }}`
    - [ ] SubTask 8.3: 徽章样式（沿用 StatusBadge 模式，`tone="idle"` 灰色 + flaskOutline 图标）
  - 验证：vue-tsc 0 错误；vite build 0 错误

- [ ] Task 9: i18n 文案
  - 依赖：无
  - 涉及文件：`app/encv-mobile/src/i18n/agent.ts` / `app/encv-mobile/src/i18n/settings.ts`
  - 步骤：
    - [ ] SubTask 9.1: agent.ts 新增 8 个 key（mockBadge / mockBadgeTooltip / mockMode / mockModeOff / mockModeBuiltin / mockModeCustom / mockSpeed / mockSpeedHelp）
    - [ ] SubTask 9.2: settings.ts 新增 3 个 key（mockBuiltinHint / mockCustomHint / mockScenarios）
    - [ ] SubTask 9.3: 同步 en 翻译
  - 验证：vue-tsc 0 错误

- [ ] Task 10: 编译 + 重启 + 集成验证
  - 依赖：所有上述 Task
  - 步骤：
    - [ ] SubTask 10.1: `cd /workspace && go build ./cmd/encv` 0 错误
    - [ ] SubTask 10.2: `cd /workspace/app/encv-mobile && npx vue-tsc --noEmit` 0 错误
    - [ ] SubTask 10.3: `cd /workspace/app/encv-mobile && npx vite build` 0 错误
    - [ ] SubTask 10.4: `bash app/encv-mobile/scripts/start-preview.sh` 按规范重启
    - [ ] SubTask 10.5: 设置 mock_mode=builtin → 提问"有哪些视频文件" → 验证：
      - 响应头 `X-Mock-Scenario: list_files_query`
      - 工具调用显示为 GroupedOperationMessage（无裸 JSON）
      - 回答完整不截断
      - 顶部「🧪 模拟」徽章显示
    - [ ] SubTask 10.6: 提问"触发超时" → 验证错误 toast
    - [ ] SubTask 10.7: 设置 mock_speed=0.1 → 验证 SSE 明显变慢

# Task Dependencies
- Task 2 依赖 Task 1
- Task 4 依赖 Task 3
- Task 5 依赖 Task 1, Task 3
- Task 6 依赖 Task 5
- Task 7 依赖 Task 5
- Task 8 依赖 Task 7
- Task 9 无依赖
- Task 10 依赖所有 Task
