# 剧本外置 + 数据驱动 Spec

## Why

`agent-mock-mode` v1 + `agent-tools-scenarios-v2` v2 共 20 个剧本**全部以 Go 字面量**写在 `internal/server/agent_mock_scenarios.go` 和 `agent_mock_v2_scenarios.go`：

```go
// 反模式 v2 现状：
var mockScenariosV2 = []*MockScenario{
    {
        ID: "search_recursive_mp4",
        ...
        Steps: []MockStep{{
            Events: []MockEvent{
                {Type: "tool_result", Data: map[string]any{
                    "result": `{"matches":[{"path":"Movies/2024/big.mp4",...}],"count":1}`,
                    "isError": false,
                    ...
                }},
                {Type: "text_delta", Data: map[string]any{
                    "text": "找到 1 个文件：Movies/2024/big.mp4",
                }},
                {Type: "stream_end", ...},
            }},
        }},
    },
    ...
}
```

**这正是用户原话**："所谓的v2剧本还是硬编码！就是个笑话！"

四大致命问题：

1. **加剧本要重新编译 Go** — 演示 / 销售 / QA 团队加新剧本必须找 Go 工程师，迭代速度被锁死
2. **tool_result 文本与真实文件系统脱钩** — `"ERROR: connection timeout after 30s"` / `"Movies/2024/big.mp4"` 全是脑补的演示数据
3. **v2 spec 自称的"数据真实化"是骗局** — engine 支持 `execute_real` 但 fallback 路径（tool_result 模板）全是假数据
4. **v2 spec 自称的"多轮向导"是骗局** — `edit_metadata_wizard` 4 轮的 user input 全是 `SetContext` 预设，不是真的"接收用户输入再推进"

**新方案核心价值**：
- **剧本即配置** — YAML / JSON 文件 + Go 模板插值，演示团队可独立添加
- **数据真实化真正落地** — `{{ mount.path }}` / `{{ search.matches[0] }}` 引用真实工具结果
- **多轮真正交互** — 剧本中 `user_input: free` 节点在每次运行时等待真实 user_text，不靠 SetContext 假装
- **完全向后兼容** — Go 字面量剧本作为 fallback（YAML 目录空时仍用旧剧本），零回归

---

## What Changes

### 新增

**剧本 schema + 加载器**
- `internal/server/mock_scenario_schema.go` — Go struct（与 YAML tag 双向映射）
- `internal/server/mock_scenario_loader.go` — 扫描目录 + 解析 YAML/JSON + 校验
- `internal/server/mock_scenario_template.go` — 模板插值引擎（Go `text/template` 包装）

**示例剧本目录**
- `internal/server/mock_scenarios/builtin/` — 8 个 v1 剧本迁移到 YAML
- `internal/server/mock_scenarios/v2/` — 8 个 v2 剧本迁移到 YAML，tool_result 用模板

**CLI 启动参数**
- `-mock-scenarios-dir string` — 剧本目录（默认 `internal/server/mock_scenarios/builtin`）
- `-mock-scenarios-reload bool` — 文件系统变更时热重载（默认 false，watcher 模式）

**YAML schema 文档**
- `internal/server/mock_scenarios/SCHEMA.md` — 完整字段说明 + 示例
- `internal/server/mock_scenarios/EXAMPLE_basic.yaml` — 5 步最小剧本示例

### 修改

- `internal/server/server.go` — `NewServer` 接收 `scenariosDir` 参数，初始化 loader
- `internal/server/agent_mock.go` — `MockEngine.Run` / `Resume` 接收已加载的 scenario，模板在事件推流前渲染
- `internal/server/agent_mock_v2.go` — `MockEngineV2.Resume` 接收真实 `userText` 而非 `SetContext` 预设值
- `cmd/encv/main.go` — 新增 `-mock-scenarios-dir` flag
- `internal/config/schema.json` — 新增 `mock_scenarios_dir` 字段

### 不影响

- `execute_real` 机制（继续走 ToolRegistry）
- MockEngine / MockEngineV2 状态机本身（不重构）
- 真实 LLM 路径
- 前端（事件 payload 形状完全不变）

---

## ADDED Requirements

### Requirement: 剧本 YAML schema

`internal/server/mock_scenario_schema.go` SHALL 定义 Go struct，与 YAML 字段一一对应。

#### Scenario: Scenario 结构

```yaml
id: search_recursive_mp4              # 必填，全局唯一
description: 搜索 100MB 以上的 mp4    # 可选
keywords: [search_recursive_mp4]      # 触发关键词（与原 Keywords 字段对应）
rounds: 1                              # 1=线性，N=多轮
branches:                              # 可选，分支选择（v2）
  - id: encrypt
    label: 加密
    icon: 🔒
    trigger_keywords: [加密]
    on_match: scenario_encrypt_flow
presets:                               # 可选，引导 chip（与原 Presets 字段对应）
  - id: start
    label: 开始
    user_text: 选 a
steps:                                 # 必填，步骤序列
  - round_idx: 0
    delay_ms: 10
    pause_for_user: false              # v2 多轮暂停点
    set_context:                       # v2 设置 round context
      selected_file: Movies/a.mp4
    use_context: []                    # v2 从 context 读取的字段
    events:
      - type: stream_start
        data:
          scenario: search_recursive_mp4
      - type: text_delta
        data:
          text: "正在搜索大于 100MB 的 MP4 文件…"
```

#### Scenario: 模板插值

`text` / `result` / 任何 string 字段 SHALL 支持 `{{ }}` 模板：

| 模板语法 | 含义 | 示例 |
|---------|------|------|
| `{{ .ToolResult.matches[0].path }}` | 引用前序 step 的 tool_result | `{{ .ToolResult.matches[0].path }}` → "Movies/big.mp4" |
| `{{ .Context.selected_file }}` | 引用 RoundContext 变量 | `{{ .Context.selected_file }}` → "Movies/a.mp4" |
| `{{ .UserText }}` | 当前 round 用户的输入 | `{{ .UserText }}` → "title" |
| `{{ .Mount.path }}` | 当前 mount 路径（execute_real 时填） | `{{ .Mount.path }}` → "/mnt/sandbox" |
| `{{ .Search.count }}` | 预定义变量：搜索总数 | `{{ .Search.count }}` → 1 |

#### Scenario: 校验规则

- 缺 `id` → 拒绝加载
- `id` 重复 → 拒绝加载（log 错误但继续加载其他）
- `steps` 为空 → 拒绝加载
- `rounds > 0` 但无 step 标 `pause_for_user: true` → 警告（多轮却不会暂停 = 配置错误）
- 模板语法错误 → 拒绝加载

---

### Requirement: 剧本加载器

`internal/server/mock_scenario_loader.go` SHALL 在启动时扫描目录，加载所有 YAML/JSON。

#### Scenario: 启动加载流程

```
NewServer(opts):
  - loader := NewScenarioLoader(opts.ScenariosDir)
  - scenarios, err := loader.LoadAll()
  - 失败 → log.Fatal（启动失败）
  - 成功 → 注册到 MockEngine.scenarios map[id]*LoadedScenario
  - 同时保留 Go 字面量 fallback（mockScenarios + mockScenariosV2）
```

#### Scenario: 优先级

- 同 `id` 时：YAML 剧本 > Go 字面量剧本（YAML 覆盖）
- 启动 log：`Loaded 16 scenarios from YAML (overriding 2 Go-literal fallbacks)`

#### Scenario: 目录为空

- YAML 目录不存在 / 无 `*.yaml` / `*.json` → 全部用 Go 字面量
- 启动 log：`No YAML scenarios found, using 20 Go-literal fallbacks`

#### Scenario: 热重载（可选）

- `-mock-scenarios-reload=true` 时启动 fsnotify watcher
- 检测到 `*.yaml` 变更 → 重新解析（不重启进程）
- 变更期间活跃的 stream 走旧剧本（不中断）
- 新 stream 用新剧本
- 错误 log：`Failed to reload scenario file: ...`

---

### Requirement: 模板插值引擎

`internal/server/mock_scenario_template.go` SHALL 用 Go `text/template` 实现字符串渲染。

#### Scenario: 渲染时机

- MockEngine 准备推 `text_delta` / `tool_result` 事件时
- 检查事件的 `data` 字段，若 string 值含 `{{` → 走模板渲染
- 模板执行上下文：`ToolResult`（最近一次 tool_result 的 result 反序列化）/ `Context`（RoundContext）/ `UserText`（本轮 user_text）/ `Mount`（mount 解析后的真实路径）

#### Scenario: tool_result 模板

```yaml
- type: tool_result
  data:
    id: call_srm_1
    name: search_files
    result: |
      {
        "matches": {{ .ToolResult.matches | tojson }},
        "count": {{ .Search.count }}
      }
```

- `tojson` 是自定义 func，把 map/slice 序列化为 JSON
- 工具执行完毕后，模板在结果插入到 event.data 之前渲染

#### Scenario: 错误处理

- 模板执行失败（引用不存在的字段） → 不崩溃，**保留原始字符串** + log warn
  - 例：`{{ .Context.nonexistent }}` → 渲染为 `{{ .Context.nonexistent }}`（占位符原样），devlog 标 warn
- 目的是 demo 永远不会因为模板错误而黑屏

---

### Requirement: 真实 user_text（多轮真正交互）

v2 剧本 SHALL **不再**用 `SetContext` 假装用户输入。

#### Scenario: 真实 user_text 流

```
Round 0: 剧本推 "请选择文件"
          (用户真实输入 "选 a" → 后端 Receive 真实 user_text)
Round 1: 剧本推 "你想改哪个字段" (引用 .Context.selected_file = {{ .UserText }})
          (用户真实输入 "title")
Round 2: ...
```

- **REMOVED**: `SetContext` 字段在 v2 YAML 剧本中**标记为 deprecated**（loader 检测到时 log warn）
- **NEW**: Round 0 后的 step 模板可引用 `{{ .UserText }}` 拿到**真实**用户输入
- 向后兼容：v1 剧本（无 rounds）走 `SetContext` 路径不变

#### Scenario: Branch 选择同此

- 剧本推 `mock_branch_choice` 后，**真实等待** `pickMockBranch(branchId)` 调用
- 不再用 `TriggerKeywords` + fake 文本匹配

---

### Requirement: 内置剧本迁移

`internal/server/mock_scenarios/builtin/*.yaml` SHALL 包含所有 12 个 v1 剧本的 YAML 版本。

#### Scenario: 迁移文件清单

| 原 Go 文件 | 新 YAML 文件 |
|----------|------------|
| `agent_mock_scenarios.go::scenarioDefaultFriendly` | `builtin/01_default_friendly.yaml` |
| `...::scenarioListMounts` | `builtin/02_list_mounts.yaml` |
| ... | ... |
| `agent_mock_v2_scenarios.go::searchRec*` | `v2/01_search_recursive_mp4.yaml` |
| ... | ... |

#### Scenario: 迁移原则

- 文本 / 关键词 / 事件流原样保留
- 唯一变化：`"{{ .Context.xxx }}"` 占位符替换原 Go 字符串拼接
- 测试断言保持兼容：原有的 `TestMockEngine_BranchChoice_PausesUntilUserPicks` 等不修改

---

### Requirement: 配置 schema 增量

`internal/config/schema.json` SHALL 新增：

```json
{
  "agent_settings": {
    "properties": {
      "mock_scenarios_dir": {
        "type": "string",
        "default": "internal/server/mock_scenarios/builtin",
        "description": "agent.mockScenariosDir"
      },
      "mock_scenarios_reload": {
        "type": "bool",
        "default": false,
        "description": "agent.mockScenariosReload"
      }
    }
  }
}
```

#### Scenario: Settings.vue 渲染

- `mock_scenarios_dir` → 文本输入 + 「选择目录」按钮（`@capacitor/filesystem`）
- `mock_scenarios_reload` → toggle 开关（仅 dev 显示）

---

### Requirement: 单元测试

#### Scenario: 加载器测试（10+ 用例）

- [ ] `TestScenarioLoader_LoadYAML_BasicFields` — 解析标准 YAML
- [ ] `TestScenarioLoader_LoadYAML_AllEventTypes` — 覆盖所有 event type
- [ ] `TestScenarioLoader_LoadYAML_MultipleFiles` — 加载目录下多个文件
- [ ] `TestScenarioLoader_LoadJSON_EquivalentToYAML` — JSON 与 YAML 结果一致
- [ ] `TestScenarioLoader_RejectMissingID`
- [ ] `TestScenarioLoader_RejectDuplicateID` — 第一个赢，第二个 log 错误跳过
- [ ] `TestScenarioLoader_RejectEmptySteps`
- [ ] `TestScenarioLoader_RejectTemplateSyntaxError`
- [ ] `TestScenarioLoader_DirEmpty_UsesGoFallback`
- [ ] `TestScenarioLoader_DirNotFound_UsesGoFallback`
- [ ] `TestScenarioLoader_HotReload_FileChange` — fsnotify 触发 reload

#### Scenario: 模板插值测试（5+ 用例）

- [ ] `TestTemplate_RenderToolResult_AfterSearch` — `{{ .ToolResult.matches[0].path }}` 替换为真实结果
- [ ] `TestTemplate_RenderUserText` — `{{ .UserText }}` 拿到真实用户输入
- [ ] `TestTemplate_RenderContext` — `{{ .Context.selected_file }}` 读 RoundContext
- [ ] `TestTemplate_MissingField_KeepsPlaceholder` — 不崩溃，保留 `{{ .X }}` 字符串
- [ ] `TestTemplate_RealTojson_Func`

#### Scenario: 端到端测试（3+ 用例）

- [ ] `TestE2E_YAMLScenario_RunEndToEnd` — 启动服务 → 触发 YAML 剧本 → 验证 tool_result 是真实 mount 数据
- [ ] `TestE2E_MultiRound_RealUserText` — 4 轮剧本用真实 user_text 推进，不用 SetContext
- [ ] `TestE2E_HotReload_NewScenarioAdded` — fs 检测到新文件 → 下次请求用新剧本

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
- 加载 log：`Mock scenario 'search_recursive_mp4' overridden by YAML (path=...)`

---

### Requirement: v2 剧本 user_text 走真实输入

**BEFORE**: `edit_metadata_wizard` 用 `SetContext: {selected_file: "Movies/a.mp4"}` 假装用户选了 a
**AFTER**: Round 1 接收真实 user_text = "选 a"，模板 `{{ .UserText }}` 引用之

#### Scenario: 旧 SetContext 处理

- YAML loader 检测到 `set_context` 字段 → log warn：`SetContext is deprecated, use {{ .UserText }} template instead`
- 仍兼容（旧 v2 剧本迁移期间不报错）
- 1 个大版本后彻底移除

---

### Requirement: Go 字面量剧本降级为 fallback

**BEFORE**: `mockScenariosV2` 是主源，编译进二进制
**AFTER**: `mockScenariosV2` 是 fallback，YAML 目录为空时才用

#### Scenario: 不删除

- **REMOVED-NOT**: 保留 `agent_mock_scenarios.go` + `agent_mock_v2_scenarios.go` 作为 fallback
- 保留原因：单元测试 / 快速启动 / 旧 demo 数据
- 注释更新：`// 当 ScenariosDir 为空时使用此 fallback；推荐迁移到 YAML`

---

## REMOVED Requirements

无（仅增量修改，不删除任何现有能力）

---

## 约束与限制

1. **完全向后兼容** — Go 字面量剧本 + `execute_real` + MockPreset + ToolRegistry 必须继续工作
2. **加载失败不启动** — YAML 解析错误 → log.Fatal（而不是降级到 Go fallback，避免静默回退）
3. **模板错误不黑屏** — 渲染失败保留占位符原样 + log warn
4. **优先级透明** — 启动 log 列出所有加载源 + 覆盖关系
5. **YAML 字段命名** — snake_case（与 Go tag 一致），不用 camelCase
6. **演示团队可独立操作** — 加新剧本不需要 Go 工程师，但**可以**找 Go 工程师加新工具
7. **多轮 user_text 真实** — 不用 SetContext 假装，必须真实接收

---

## 与现有 spec 的关系

| 现有 spec | 关系 |
|----------|------|
| `agent-mock-mode` | **修改点** — Go 字面量剧本降级为 fallback，YAML 为主源 |
| `agent-tools-scenarios-v2` | **修改点** — v2 剧本迁移到 YAML，SetContext 改 {{ .UserText }} 模板 |
| `multi-engine-chat-architecture` | 无关 |
| `go-in-process-agent` | 无关 |
| `mock-router-refactor` | 无关（前端 mock 路由） |

---

## 验证步骤

1. **加载器单元测试** — `go test ./internal/server/... -run TestScenarioLoader -v` 10+ 全过
2. **模板测试** — `go test ./internal/server/... -run TestTemplate -v` 5+ 全过
3. **迁移完整性** — `git diff --stat internal/server/agent_mock*scenarios.go` 应显示保留但有 deprecation 注释
4. **E2E** — `go test ./internal/server/... -run TestE2E_YAML -v` 3+ 全过
5. **真机 / 集成验证** —
   - 启动服务 `-mock-scenarios-dir=./testdata/yaml_scenarios`
   - 提问"找视频" → 验证 tool_result 引用真实 mount 路径（不是硬编码的 Movies/2024/big.mp4）
   - 提问"改标题" → 4 轮真实 user_text 推进（每轮 user_text 真实进入 Context）
   - 编辑 `testdata/yaml_scenarios/v2/my_test.yaml` 加新剧本 → 下次请求可用，无需重启
6. **类型检查** — `go build ./cmd/encv` 0 错误
7. **前端无改动** — `vue-tsc --noEmit` 0 错误

---

## 关键文件 / 函数

| 文件 | 关键类型/函数 |
|------|--------------|
| `internal/server/mock_scenario_schema.go` | `LoadedScenario` / `YAMLStep` / `YAMLEvent` / `YAMLBranch` / `YAMLPreset` |
| `internal/server/mock_scenario_loader.go` | `NewScenarioLoader(dir)` / `LoadAll()` / `Watch(ctx)` / `scenariosFromYAML` |
| `internal/server/mock_scenario_template.go` | `RenderEvent(event, ctx)` / `RenderString(s, ctx)` / `tojsonFunc` |
| `internal/server/agent_mock.go` | `MockEngine.scenarios` 改为 map，移除 var 引用 |
| `internal/server/agent_mock_v2.go` | `Resume(ctx, userText, ...)` 接收真实 userText |
| `internal/server/agent_mock_scenarios.go` | 加 deprecation 注释（保留 fallback） |
| `internal/server/agent_mock_v2_scenarios.go` | 加 deprecation 注释（保留 fallback） |
| `internal/server/mock_scenarios/builtin/*.yaml` | 12 个 v1 剧本迁移 |
| `internal/server/mock_scenarios/v2/*.yaml` | 8 个 v2 剧本迁移 |
| `internal/server/mock_scenarios/SCHEMA.md` | 完整 schema 文档 |
| `internal/server/mock_scenarios/EXAMPLE_basic.yaml` | 5 步最小示例 |
| `cmd/encv/main.go` | 新增 `-mock-scenarios-dir` flag |
| `internal/config/schema.json` | 新增 `mock_scenarios_dir` / `mock_scenarios_reload` |
| `internal/config/config.go` | `AgentSettings.MockScenariosDir` / `MockScenariosReload` |
| `app/encv-mobile/src/views/Settings.vue` | 渲染新字段（dev only） |
