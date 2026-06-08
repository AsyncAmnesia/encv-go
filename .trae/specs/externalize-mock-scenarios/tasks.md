# Tasks

有序、可验证的工作项；每项都对应具体文件 / 函数 / 测试。

## Task Dependencies

- T1 → T2（schema 先于 loader）
- T1 → T3（template 引擎依赖 schema 数据形状）
- T2, T3 → T4（MockEngine 集成需 loader + template）
- T4 → T5（v2 user_text 真实化依赖 MockEngine 改造）
- T5 → T6（迁移工作）
- T2..T6 → T7（CLI 集成）
- T7 → T8（配置 schema 增量）
- T1..T8 → T9（端到端验证）
- T9 → T10（文档）

---

## T1. 剧本 YAML schema 定义

**目标**: Go struct + YAML tag 双向映射，约束字段形状

- [ ] **T1.1** 新建 `internal/server/mock_scenario_schema.go`
  - `LoadedScenario` 结构（id / description / keywords / rounds / branches / presets / steps）
  - `YAMLStep`（round_idx / delay_ms / pause_for_user / set_context / use_context / events）
  - `YAMLEvent`（type / data map[string]any）
  - `YAMLBranch`（id / label / icon / trigger_keywords / trigger_regex / on_match / initial_step_id）
  - `YAMLPreset`（id / label / user_text / icon）
  - 所有字段带 `yaml:"..."` tag，snake_case
  - 校验函数 `Validate() error`
- [ ] **T1.2** 单元测试（5+）
  - `TestSchema_ParseYAML_BasicFields` — 解析标准 5 字段
  - `TestSchema_ParseYAML_AllEventTypes` — 覆盖 12 种 event type
  - `TestSchema_ParseYAML_Branches` — branches 列表
  - `TestSchema_Validate_RejectsMissingID`
  - `TestSchema_Validate_RejectsEmptySteps`

✅ **验收**: `go test ./internal/server/... -run TestSchema -v` 全过

---

## T2. 剧本加载器

**目标**: 扫描目录 + 解析 YAML/JSON + 校验 + 注册

- [ ] **T2.1** 新建 `internal/server/mock_scenario_loader.go`
  - `ScenarioLoader` 结构（dir / logger / mu / scenarios map）
  - `NewScenarioLoader(dir string) *ScenarioLoader`
  - `LoadAll(ctx) error` — 扫描 `*.yaml` + `*.json`，逐个解析
  - 错误聚合（不中断，单文件失败 log error 继续）
  - 重复 id → 第一个赢，第二个 log error
  - `scenariosFromGoFallback()` — 注入 Go 字面量剧本（向后兼容）
- [ ] **T2.2** 热重载 watcher（`Watch(ctx)`）
  - 用 `github.com/fsnotify/fsnotify`
  - 监听 `*.yaml` / `*.json` 变更
  - 触发 reload（新文件 / 修改文件）
  - 失败 log error 但不中断 watcher
  - 活跃 stream 不受影响（旧剧本继续）
- [ ] **T2.3** 单元测试（10+）
  - `TestLoader_LoadYAML_BasicFields`
  - `TestLoader_LoadYAML_AllEventTypes`
  - `TestLoader_LoadYAML_MultipleFiles`
  - `TestLoader_LoadJSON_EquivalentToYAML`
  - `TestLoader_RejectMissingID`
  - `TestLoader_RejectDuplicateID`
  - `TestLoader_RejectEmptySteps`
  - `TestLoader_DirEmpty_UsesGoFallback`
  - `TestLoader_DirNotFound_UsesGoFallback`
  - `TestLoader_HotReload_FileChange` — fsnotify 触发 reload
  - `TestLoader_Priority_YAMLOverridesGo`

✅ **验收**: `go test ./internal/server/... -run TestLoader -v` 全过

---

## T3. 模板插值引擎

**目标**: 用 Go text/template 渲染 string 字段，引用 ToolResult / Context / UserText

- [ ] **T3.1** 新建 `internal/server/mock_scenario_template.go`
  - `TemplateContext` struct（ToolResult / Context / UserText / Mount / Search）
  - `RenderString(s string, ctx *TemplateContext) (string, error)` — 失败保留原样
  - `RenderEvent(event YAMLEvent, ctx *TemplateContext) (YAMLEvent, error)`
  - 递归遍历 event.data 中 string 值，含 `{{` 才走模板
  - `tojsonFunc` 自定义 func（map/slice → JSON string）
- [ ] **T3.2** 单元测试（5+）
  - `TestTemplate_RenderToolResult_AfterSearch`
  - `TestTemplate_RenderUserText` — `{{ .UserText }}` 拿到真实输入
  - `TestTemplate_RenderContext` — `{{ .Context.selected_file }}`
  - `TestTemplate_MissingField_KeepsPlaceholder` — 不崩溃
  - `TestTemplate_RealTojson_Func`

✅ **验收**: `go test ./internal/server/... -run TestTemplate -v` 全过

---

## T4. MockEngine 集成

**目标**: MockEngine 接收已加载剧本，事件推流前模板渲染

- [ ] **T4.1** 修改 `internal/server/agent_mock.go`
  - `MockEngine.scenarios` 改为 `map[string]*MockScenario`
  - 删除 `var mockScenarios = []*MockScenario{...}` 引用
  - `NewMockEngine(scenarios []*MockScenario)` → 构造 map
  - 推流前调 `RenderEvent(event, ctx)`（ctx 含当前 round state）
- [ ] **T4.2** 修改 `internal/server/agent_mock_v2.go`
  - `Resume(ctx, userText, ...)` 接收真实 userText
  - 把 userText 注入 `TemplateContext.UserText`
  - 推流前渲染模板
- [ ] **T4.3** 单元测试
  - `TestMockEngine_UsesLoadedScenarios` — 验证 loader 注入的剧本被使用
  - `TestMockEngine_RendersTemplate_BeforePush` — 验证 text_delta 模板已渲染
  - `TestMockEngineV2_Resume_RealUserText` — 真实 userText 进 Context

✅ **验收**: 现有 `TestMockEngine*` 全部不修改仍通过

---

## T5. v2 剧本 user_text 真实化

**目标**: SetContext deprecate，多轮靠真实 userText + 模板推进

- [ ] **T5.1** 修改 loader 校验
  - 检测 YAML 中 `set_context` 字段 → log warn：`SetContext is deprecated, use {{ .UserText }} template`
  - 仍兼容（旧 v2 剧本迁移期间不报错）
- [ ] **T5.2** 迁移 `edit_metadata_wizard` 等 4 轮剧本
  - YAML 中 `set_context: {selected_file: Movies/a.mp4}` 改为模板：
    ```yaml
    - type: text_delta
      data:
        text: "好的，你想编辑 {{ .UserText }} 吗？"
    ```
  - 真实 `Resume(ctx, "Movies/a.mp4", ...)` 注入 UserText
- [ ] **T5.3** 单元测试
  - `TestMockEngineV2_EditMetadataWizard_RealUserText` — 4 轮 user_text 真实推进
  - `TestLoader_LogsDeprecation_OnSetContext`

✅ **验收**: 多轮场景不再有 SetContext 假数据

---

## T6. 内置剧本迁移

**目标**: 20 个剧本全部迁移到 YAML

- [ ] **T6.1** 新建 `internal/server/mock_scenarios/builtin/` 目录
  - 12 个 v1 剧本迁移到 YAML
  - 文件名：`01_default_friendly.yaml` ... `12_*.yaml`
- [ ] **T6.2** 新建 `internal/server/mock_scenarios/v2/` 目录
  - 8 个 v2 剧本迁移到 YAML
  - tool_result 用 `{{ .ToolResult.matches | tojson }}` 模板
- [ ] **T6.3** Go 字面量剧本降级
  - `agent_mock_scenarios.go` 加 deprecation 注释
  - `agent_mock_v2_scenarios.go` 加 deprecation 注释
  - 保留作为 fallback（YAML 目录为空时使用）
- [ ] **T6.4** 单元测试
  - `TestMigration_AllBuiltinScenarios_Loadable` — 12 个 YAML 全部解析通过
  - `TestMigration_AllV2Scenarios_Loadable` — 8 个 v2 YAML 全部解析通过
  - `TestMigration_BehaviorEquivalentToGoLiteral` — 同一 id 行为等价

✅ **验收**: 启动 log 显示 `Loaded 20 scenarios from YAML (overriding 0 Go-literal fallbacks)`

---

## T7. CLI flag + 服务集成

**目标**: `cmd/encv/main.go` 支持 `-mock-scenarios-dir`

- [ ] **T7.1** 修改 `cmd/encv/main.go`
  - 新增 `flag.String("mock-scenarios-dir", "", "YAML scenarios directory (empty = Go literal fallback)")`
  - 传给 `server.NewServer(opts)`
- [ ] **T7.2** 修改 `internal/server/server.go`
  - `ServerOptions` 加 `ScenariosDir string` 字段
  - `NewServer` 初始化 loader
  - 失败 → log.Fatal（启动失败）
- [ ] **T7.3** 单元测试
  - `TestMain_FlagParse` — 验证 flag 解析
  - `TestServer_NewServer_NoDir_UsesFallback`
  - `TestServer_NewServer_WithDir_LoadsYAML`

✅ **验收**: `go run ./cmd/encv -mock-scenarios-dir=./testdata/yaml` 启动成功

---

## T8. 配置 schema 增量

**目标**: config.json 暴露 2 个新字段

- [ ] **T8.1** 修改 `internal/config/config.go`
  - `AgentSettings` 加 `MockScenariosDir string` / `MockScenariosReload bool`
- [ ] **T8.2** 修改 `internal/config/schema.json`
  - 加 2 个新字段 + 默认值 + description
- [ ] **T8.3** 修改 `app/encv-mobile/src/views/Settings.vue` 渲染
  - `mock_scenarios_dir` → 文本输入 + 「选择目录」按钮
  - `mock_scenarios_reload` → toggle 开关（仅 dev 显示）
- [ ] **T8.4** 单元测试
  - `TestConfig_DefaultMockScenariosDir`
  - `TestConfig_ParseMockScenariosReload`

✅ **验收**: 启动时加载 config.json 2 个字段均有值，Settings.vue 渲染正常

---

## T9. 端到端集成测试

**目标**: 真实 mount + 真实剧本 + 真实 user_text + 热重载

- [ ] **T9.1** 准备 sandbox 目录（复用 T15 已有 sandbox）
- [ ] **T9.2** E2E: YAML 剧本端到端
  - 启动服务，YAML 目录 = `./testdata/yaml_scenarios`
  - 提问"找视频" → 验证 tool_result 引用真实 mount 路径
  - 验证 text_delta 模板渲染正确
- [ ] **T9.3** E2E: 多轮真实 user_text
  - 启动 `edit_metadata_wizard` 剧本
  - 4 轮 `Resume("选 a" / "title" / "My New Title" / "yes")`
  - 验证 RoundContext.UserText 被模板正确引用
- [ ] **T9.4** E2E: 热重载
  - 启动服务，`-mock-scenarios-reload=true`
  - 写入新 YAML 文件
  - 下次请求用新剧本（旧 stream 不中断）

✅ **验收**: `go test ./internal/server/... -run TestE2E_YAML -v` 3+ 全过

---

## T10. 文档与示例

**目标**: 演示团队可独立添加剧本

- [ ] **T10.1** 新建 `internal/server/mock_scenarios/SCHEMA.md`
  - 完整字段说明
  - 模板语法参考
  - 最佳实践
- [ ] **T10.2** 新建 `internal/server/mock_scenarios/EXAMPLE_basic.yaml`
  - 5 步最小剧本示例
  - 含 1 个 tool_call + 1 个 user input + 1 个 stream_end
- [ ] **T10.3** 新建 `internal/server/mock_scenarios/EXAMPLE_advanced.yaml`
  - 多轮 + 分支 + 模板
  - 真实 search_files 工具调用 + 真实 mount 路径引用
- [ ] **T10.4** 更新 `internal/server/agent_mock_scenarios.go` 顶部注释
  - 加迁移指南
  - 指向 `mock_scenarios/SCHEMA.md`

✅ **验收**: 演示团队读 SCHEMA.md + 改 EXAMPLE 即能加新剧本，无需 Go 工程师
