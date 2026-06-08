# 剧本外置 Spec（数据驱动版，零模板引擎）

## Why

`agent-mock-mode` v1 + `agent-tools-scenarios-v2` v2 共 20 个剧本**全部以 Go 字面量**写在 `internal/server/agent_mock_scenarios.go` 和 `agent_mock_v2_scenarios.go`。演示团队加新剧本必须找 Go 工程师重编二进制，迭代速度被锁死。

**用户原话**："所谓的v2剧本还是硬编码！就是个笑话！"

**修法**：把剧本从 Go 源码搬到 YAML/JSON 文件，加剧本 = 改配置文件 + 重启（或热重载），不需要重编 Go。

**关键约束**（用户明确要求）：
- ❌ **剧本严禁走用户输入**（不接 free-form text）
- ❌ **不引入模板引擎**（不需要 `{{ .UserText }}` 这种东西）
- ❌ **不增加不必要的复杂度**（loader 一份、YAML 一份、够用就行）
- ✅ **剧本 = 预设场景序列**（像剧本杀 / 互动剧，所有对话、所有选项都是预设的）
- ✅ **分支 = 预设选项 chip**（用户只能点，不能输入）
- ✅ **数据真实化 = tool_result 走真实工具**，文本是固定字符串
- ✅ **完全向后兼容** — Go 字面量剧本作为 fallback（YAML 目录空时仍用旧剧本），零回归

---

## What Changes

### 新增

- `internal/server/mock_scenario_schema.go` — Go struct 与 YAML 字段双向映射
- `internal/server/mock_scenario_loader.go` — 扫描目录 + 解析 + 校验 + 注册
- `internal/server/mock_scenarios/builtin/*.yaml` — 12 个 v1 剧本迁移
- `internal/server/mock_scenarios/v2/*.yaml` — 8 个 v2 剧本迁移
- `internal/server/mock_scenarios/SCHEMA.md` — 完整字段说明
- `internal/server/mock_scenarios/EXAMPLE_basic.yaml` — 5 步最小示例

### 修改

- `internal/server/server.go` — `NewServer` 接收 `scenariosDir` 参数
- `cmd/encv/main.go` — 新增 `-mock-scenarios-dir` flag
- `internal/config/schema.json` — 新增 `mock_scenarios_dir` 字段
- `app/encv-mobile/src/views/Settings.vue` — 渲染新字段
- `agent_mock_scenarios.go` / `agent_mock_v2_scenarios.go` — 加 deprecation 注释

### 不修改

- `execute_real` 机制
- MockEngine 状态机本身
- ToolRegistry
- 真实 LLM 路径
- 前端事件 payload 形状

---

## ADDED Requirements

### Requirement: 剧本 YAML schema

`internal/server/mock_scenario_schema.go` SHALL 定义 Go struct。

#### Scenario: 剧本结构（核心）

```yaml
id: search_recursive_mp4        # 必填，全局唯一
description: 搜索 100MB+ 的 mp4  # 可选
keywords:                       # 触发关键词（与原 Keywords 字段一致）
  - search_recursive_mp4
  - 找视频

steps:                           # 必填，步骤序列（从头执行到尾）
  - id: announce                 # 步骤 ID（仅用于日志/调试，不参与逻辑）
    events:                      # 该步骤推流的事件序列
      - type: stream_start
        data:
          scenario: search_recursive_mp4
      - type: text_delta
        data:
          text: "正在搜索 100MB 以上的 MP4..."
      - type: tool_call
        data:
          id: call_srm_1
          name: search_files
          args: { path: "/mnt/sandbox", ext: ".mp4", min_size: 104857600 }
      - type: tool_result
        data:
          id: call_srm_1
          name: search_files
          isError: false
          result: '{"matches":[],"count":0}'    # 占位：execute_real 时由工具真实结果替换
      - type: text_delta
        data:
          text: "搜索完成，0 个匹配。请选择操作："
      - type: mock_branch_choice # 推 mock_branch_choice 事件，前端渲染选项 chip
        data:
          branch_id: post_search  # 前端选完后 POST /api/agent/branch-pick
          options:                # 预设选项（chip 列表），用户只能点这些
            - id: relax
              label: 放宽条件（不限大小）
              keywords: [放宽]
              icon: 🎚️
            - id: change_ext
              label: 改其他格式
              keywords: [改格式]
              icon: 📁
            - id: cancel
              label: 取消
              keywords: [取消]
              icon: ❌
      - type: stream_end
        data:
          scenario: search_recursive_mp4

  - id: relax                    # 用户选了 "relax" 之后跳到的步骤
    events:
      - type: stream_start
        data:
          scenario: search_recursive_mp4
      - type: text_delta
        data:
          text: "好的，已放宽到不限大小，正在重新搜索..."
      - type: tool_call
        data:
          ...
      - type: stream_end
        data:
          scenario: search_recursive_mp4

  - id: change_ext
    events: [...]
  - id: cancel
    events: [...]

  # 错误分支：tool_result.isError=true 时走这条
  - id: error_path
    when_tool_error: true         # 触发条件（这是剧本游戏，不是 if-else 语言）
    events: [...]
```

#### Scenario: 字段约束

- 缺 `id` → 拒绝加载
- `id` 重复 → 第一个赢，第二个 log error 跳过
- `steps` 为空 → 拒绝加载
- `events` 为空 → 拒绝加载
- `mock_branch_choice` 的 `options` 至少 2 个、不能为空

#### Scenario: 关键事件类型

| 事件 | 用途 |
|------|------|
| `stream_start` / `stream_end` | 标记一轮流的开头/结尾 |
| `text_delta` | 推一段固定文本（**不允许模板**——只允许纯字符串） |
| `tool_call` | 声明一个工具调用，args 是真实工具入参 |
| `tool_result` | 工具返回结果（**execute_real 时由真实结果替换 result 字段**） |
| `mock_branch_choice` | 推分支选项，前端渲染为 chip |

#### Scenario: tool_result 真实化（不靠模板，靠"替换"）

- YAML 里 `tool_result.result` 是个**占位字符串**（多数情况是 `"{}"` 或示例 JSON）
- 当 MockEngine 检测到对应 `tool_call.name` 走 `execute_real` 路径时：
  - **用真实工具结果替换 `result` 字段**
  - `text_delta.text` 仍是 YAML 里写死的固定字符串（**不需要模板去引用工具结果**）
- 当 MockEngine 走 `mock_only` 路径时：
  - `result` 保持 YAML 里的占位字符串
  - 前端可能 fallback 显示一些默认 UI

**这才是"数据真实化"的正确做法**：不靠模板字符串拼接，而是靠 schema 层面的"占位 → 真实结果"映射。文本永远是预设的（"搜索完成"），数据永远是真实的（tool_result 来自工具）。

---

### Requirement: 剧本加载器

`internal/server/mock_scenario_loader.go` SHALL 在启动时扫描目录。

#### Scenario: 启动加载

```
NewServer(opts):
  loader := NewScenarioLoader(opts.ScenariosDir)
  scenarios, err := loader.LoadAll()
  失败 → log.Fatal
  成功 → 注册到 MockEngine.scenarios map[id]*MockScenario
  同时注入 Go 字面量 fallback（mockScenarios + mockScenariosV2）
```

#### Scenario: 优先级

- 同 `id` 时：YAML > Go 字面量
- 启动 log：`Loaded 20 scenarios from YAML (overriding 0 Go-literal fallbacks)`

#### Scenario: 目录为空

- YAML 目录不存在 / 无 `*.yaml` / `*.json` → 全部用 Go 字面量
- 启动 log：`No YAML scenarios found, using 20 Go-literal fallbacks`

#### Scenario: 热重载（可选）

- `-mock-scenarios-reload=true` 启动 fsnotify watcher
- 检测到 `*.yaml` 变更 → 重新解析（不重启进程）
- 活跃 stream 走旧剧本（不中断）
- 新 stream 用新剧本

---

### Requirement: 剧本分支（预设选项，不是 free-form 输入）

**核心约束（用户明确要求）**：
- ❌ **剧本严禁走用户输入**（不接 free-form text，不接真实 user_text 推进）
- ✅ **分支 = 预设选项 chip**（用户在 YAML 写好的 options 列表里点一个）
- ✅ **step → 选 option → 跳到对应 step**（确定性，无意外）
- ✅ **文本永远是预设字符串**（不用 `{{ }}` 模板）

#### Scenario: 分支推进流程

```
剧本执行到 step "post_search"（含 mock_branch_choice）
   ↓
推 mock_branch_choice 事件，前端展示 3 个 chip
   ↓
用户点击 "放宽" chip
   ↓
前端 POST /api/agent/branch-pick {scenario: "search_recursive_mp4", branch_id: "post_search", option: "relax"}
   ↓
后端 MockEngine 收到 pick，根据 "relax" 跳到 step "relax"
   ↓
继续推 step "relax" 的 events
```

- **没有任何 free-form text 输入**
- 用户**只能**在 `mock_branch_choice.options` 列表里选
- YAML 里写死有哪些选项，每个选项对应哪个 step
- 这就是剧本游戏的本质：**剧本 = 预定义剧情树，用户在岔路口点预设选项**

#### Scenario: 关键词触发（同 v1/v2 现状）

- 剧本匹配通过 `keywords` 字段（前缀 / 完全匹配）
- YAML `keywords` 数组保留与 Go 字面量完全一致的行为

---

### Requirement: 单元测试

#### Scenario: 加载器测试

- [ ] `TestLoader_LoadYAML_BasicFields`
- [ ] `TestLoader_LoadYAML_AllEventTypes`
- [ ] `TestLoader_LoadYAML_MultipleFiles`
- [ ] `TestLoader_LoadJSON_EquivalentToYAML`
- [ ] `TestLoader_RejectMissingID`
- [ ] `TestLoader_RejectDuplicateID`
- [ ] `TestLoader_RejectEmptySteps`
- [ ] `TestLoader_RejectEmptyEvents`
- [ ] `TestLoader_DirEmpty_UsesGoFallback`
- [ ] `TestLoader_DirNotFound_UsesGoFallback`
- [ ] `TestLoader_HotReload_FileChange`
- [ ] `TestLoader_Priority_YAMLOverridesGo`

#### Scenario: 分支测试

- [ ] `TestScenario_Branch_OptionsArePrescript` — 选项必须 ≥2 个
- [ ] `TestScenario_Branch_PickAdvancesToCorrectStep`
- [ ] `TestScenario_Branch_FreeFormUserInputIsRejected` — 后端拒绝接 free-form text 推进剧本

#### Scenario: 端到端测试

- [ ] `TestE2E_YAMLScenario_RunEndToEnd` — 触发 YAML 剧本 → 验证 step 序列
- [ ] `TestE2E_BranchPick_AdvancesToCorrectStep`
- [ ] `TestE2E_HotReload_NewScenarioAdded`
- [ ] `TestE2E_FreeFormUserInput_NotConsumedByScenario` — 验证剧本不响应 free text

---

## MODIFIED Requirements

### Requirement: MockEngine 接收已加载剧本

**BEFORE**: `var mockScenarios = []*MockScenario{...}`（Go 字面量全局变量）
**AFTER**: `MockEngine.scenarios map[string]*MockScenario`（loader 注入）

#### Scenario: 向后兼容

- 当 `scenariosDir` 不存在或为空时，loader 把 `mockScenarios` + `mockScenariosV2` 注入到 `MockEngine.scenarios`
- 所有现有测试（无 ScenariosDir 字段）继续工作

#### Scenario: 优先级

- YAML 同 id 剧本 > Go 字面量剧本
- 加载 log：`Mock scenario 'X' overridden by YAML`

---

### Requirement: Go 字面量剧本降级为 fallback

**BEFORE**: `mockScenariosV2` 是主源，编译进二进制
**AFTER**: `mockScenariosV2` 是 fallback，YAML 目录为空时才用

#### Scenario: 不删除

- **保留** `agent_mock_scenarios.go` + `agent_mock_v2_scenarios.go` 作为 fallback
- 注释更新：`// 当 ScenariosDir 为空时使用此 fallback；推荐迁移到 YAML`
- 单元测试 / 快速启动 / 旧 demo 数据继续工作

---

## REMOVED Requirements

**用户原话**："剧本严禁走用户输入！增加不必要的复杂度"

- ❌ **删除** T3 模板插值引擎 — `internal/server/mock_scenario_template.go` 不再存在
- ❌ **删除** `{{ .UserText }}` / `{{ .ToolResult.matches[0] }}` 之类模板引用
- ❌ **删除** `RenderString` / `RenderEvent` 之类模板 API
- ❌ **删除** T5 v2 user_text 真实化（旧的 `edit_metadata_wizard` SetContext 也走 fallback）
- ❌ **删除** `tojsonFunc` 自定义 func
- ❌ **删除** T1.2 / T3.2 中所有模板相关测试

**保留**：
- ✅ YAML 外置（核心价值）
- ✅ loader 加载 + 校验
- ✅ 预设选项 chip（mock_branch_choice）
- ✅ 关键词触发
- ✅ tool_result 真实化（通过 schema 替换，不通过模板）
- ✅ Go 字面量 fallback
- ✅ 热重载（可选）

---

## 约束与限制（更严了）

1. **完全向后兼容** — Go 字面量剧本 + `execute_real` + MockPreset + ToolRegistry 必须继续工作
2. **加载失败不启动** — YAML 解析错误 → log.Fatal（不静默回退）
3. **剧本不接 free-form text** — 后端 API `branch-pick` 只接受 `option` ID，不接受 text
4. **文本永远是预设字符串** — text_delta.text 字段不允许模板语法（拒绝 `{{` 字符）
5. **YAML 字段命名** — snake_case
6. **演示团队可独立操作** — 加新剧本 = 写 YAML + 重启（或开热重载）

---

## 关键文件 / 函数

| 文件 | 作用 |
|------|------|
| `internal/server/mock_scenario_schema.go` | `LoadedScenario` / `YAMLStep` / `YAMLEvent` struct + Validate() |
| `internal/server/mock_scenario_loader.go` | `NewScenarioLoader(dir)` / `LoadAll(ctx)` / `Watch(ctx)` |
| `internal/server/agent_mock.go` | `MockEngine.scenarios` 改为 map（loader 注入） |
| `internal/server/agent_mock_scenarios.go` | 加 deprecation 注释（保留 fallback） |
| `internal/server/agent_mock_v2_scenarios.go` | 加 deprecation 注释（保留 fallback） |
| `internal/server/mock_scenarios/builtin/*.yaml` | 12 个 v1 剧本迁移 |
| `internal/server/mock_scenarios/v2/*.yaml` | 8 个 v2 剧本迁移 |
| `internal/server/mock_scenarios/SCHEMA.md` | 完整 schema 文档 |
| `internal/server/mock_scenarios/EXAMPLE_basic.yaml` | 5 步最小剧本示例 |
| `cmd/encv/main.go` | 新增 `-mock-scenarios-dir` flag |
| `internal/config/schema.json` | 新增 `mock_scenarios_dir` 字段 |

---

## 与现有 spec 的关系

| 现有 spec | 关系 |
|----------|------|
| `agent-mock-mode` | **修改点** — Go 字面量剧本降级为 fallback，YAML 为主源 |
| `agent-tools-scenarios-v2` | **修改点** — v2 剧本迁移到 YAML，**不动** SetContext / branch 机制（保留作为 YAML 数据） |
| `multi-engine-chat-architecture` | 无关 |
| `go-in-process-agent` | 无关 |
| `mock-router-refactor` | 无关（前端 mock 路由） |
