# Checklist

实施完成时的验收清单。每条对应 spec 中的具体 Requirement / Scenario。

## T1. 剧本 YAML schema 定义

- [ ] `internal/server/mock_scenario_schema.go` 存在
- [ ] `LoadedScenario` / `YAMLStep` / `YAMLEvent` / `YAMLBranch` / `YAMLPreset` 结构体定义
- [ ] 所有字段带 `yaml:"..."` tag，snake_case
- [ ] `Validate() error` 实现，缺 id / 空 steps 拒绝
- [ ] `go test -run TestSchema -v` 全过（5+ 用例）

## T2. 剧本加载器

- [ ] `internal/server/mock_scenario_loader.go` 存在
- [ ] `NewScenarioLoader(dir)` / `LoadAll(ctx)` / `Watch(ctx)` 三个方法
- [ ] YAML + JSON 双格式支持
- [ ] 错误聚合（不中断，单文件失败 log 继续）
- [ ] 重复 id → 第一个赢，第二个 log error
- [ ] 目录为空 → 自动注入 Go 字面量 fallback
- [ ] 目录不存在 → 同上
- [ ] fsnotify 热重载（`-mock-scenarios-reload=true` 时启用）
- [ ] 优先级：YAML > Go 字面量
- [ ] 启动 log 列出加载源 + 覆盖关系
- [ ] `go test -run TestLoader -v` 全过（10+ 用例）

## T3. 模板插值引擎

- [ ] `internal/server/mock_scenario_template.go` 存在
- [ ] `TemplateContext` struct（ToolResult / Context / UserText / Mount / Search）
- [ ] `RenderString(s, ctx)` / `RenderEvent(event, ctx)` 实现
- [ ] 递归遍历 event.data 中 string 值，含 `{{` 才走模板
- [ ] `tojsonFunc` 自定义 func（map/slice → JSON string）
- [ ] 模板执行失败保留占位符原样 + log warn
- [ ] `go test -run TestTemplate -v` 全过（5+ 用例）

## T4. MockEngine 集成

- [ ] `MockEngine.scenarios` 改为 `map[string]*MockScenario`
- [ ] 移除对 `var mockScenarios` 的直接引用
- [ ] `NewMockEngine(scenarios []*MockScenario)` 构造 map
- [ ] 推流前调 `RenderEvent(event, ctx)`
- [ ] `MockEngineV2.Resume` 接收真实 userText
- [ ] `Resume` 把 userText 注入 `TemplateContext.UserText`
- [ ] v2 推流前渲染模板
- [ ] 现有 `TestMockEngine*` 测试 0 修改仍通过

## T5. v2 剧本 user_text 真实化

- [ ] Loader 检测 `set_context` 字段 → log deprecation warn
- [ ] `set_context` 仍兼容（不报错）
- [ ] `edit_metadata_wizard` YAML 用 `{{ .UserText }}` 模板
- [ ] 4 轮剧本用真实 user_text 推进
- [ ] `TestMockEngineV2_EditMetadataWizard_RealUserText` 通过
- [ ] `TestLoader_LogsDeprecation_OnSetContext` 通过

## T6. 内置剧本迁移

- [ ] `internal/server/mock_scenarios/builtin/` 目录存在
- [ ] 12 个 v1 剧本 YAML 文件存在
- [ ] `internal/server/mock_scenarios/v2/` 目录存在
- [ ] 8 个 v2 剧本 YAML 文件存在
- [ ] v2 tool_result 用 `{{ .ToolResult.matches | tojson }}` 模板
- [ ] `agent_mock_scenarios.go` 顶部 deprecation 注释
- [ ] `agent_mock_v2_scenarios.go` 顶部 deprecation 注释
- [ ] Go 字面量保留作为 fallback
- [ ] `TestMigration_AllBuiltinScenarios_Loadable` 通过
- [ ] `TestMigration_AllV2Scenarios_Loadable` 通过
- [ ] `TestMigration_BehaviorEquivalentToGoLiteral` 通过
- [ ] 启动 log：`Loaded 20 scenarios from YAML (overriding 0 Go-literal fallbacks)`

## T7. CLI flag + 服务集成

- [ ] `cmd/encv/main.go` 新增 `-mock-scenarios-dir` flag
- [ ] `ServerOptions.ScenariosDir` 字段
- [ ] `NewServer` 初始化 loader
- [ ] 加载失败 → log.Fatal
- [ ] `go run ./cmd/encv -mock-scenarios-dir=./testdata/yaml` 启动成功
- [ ] `TestMain_FlagParse` / `TestServer_NewServer_*` 通过

## T8. 配置 schema 增量

- [ ] `AgentSettings.MockScenariosDir` 字段
- [ ] `AgentSettings.MockScenariosReload` 字段
- [ ] `schema.json` 新增 2 字段
- [ ] `Settings.vue` 渲染目录输入 + 选择按钮
- [ ] `Settings.vue` 渲染 toggle 开关（dev only）
- [ ] `TestConfig_*` 通过

## T9. 端到端集成测试

- [ ] sandbox 目录准备（mp4/srt/log/json）
- [ ] E2E: YAML 剧本真实 mount 搜索
- [ ] E2E: 4 轮真实 user_text 推进
- [ ] E2E: 热重载新文件立即可用
- [ ] `go test -run TestE2E_YAML -v` 全过（3+ 用例）

## T10. 文档与示例

- [ ] `mock_scenarios/SCHEMA.md` 存在
- [ ] `mock_scenarios/EXAMPLE_basic.yaml` 存在（5 步最小）
- [ ] `mock_scenarios/EXAMPLE_advanced.yaml` 存在（多轮 + 分支 + 模板）
- [ ] `agent_mock_scenarios.go` 顶部注释指向 SCHEMA.md

---

## 用户原话验证

> "所谓的v2剧本还是硬编码！就是个笑话！"

- [ ] ❌ **没有任何剧本**写在 Go 字面量 `var mockScenarios* = []*MockScenario{...}` 中
- [ ] ✅ 所有剧本都在 `internal/server/mock_scenarios/*.yaml`
- [ ] ✅ Go 字面量**仅**作为 fallback 保留，且加 deprecation 注释
- [ ] ✅ tool_result 文本通过 `{{ .ToolResult.X }}` 引用真实工具结果
- [ ] ✅ 多轮剧本通过 `{{ .UserText }}` 接收真实用户输入
- [ ] ✅ 加新剧本不需要重新编译 Go（改 YAML 文件即可）
- [ ] ✅ 演示团队读 SCHEMA.md 即可上手
