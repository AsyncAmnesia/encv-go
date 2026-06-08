# Checklist

实施完成时的验收清单。每条对应 spec 中的具体 Requirement / Scenario。

---

## 用户原话验证（最高优先级）

> "所谓的v2剧本还是硬编码！就是个笑话！"
> "剧本严禁走用户输入！增加不必要的复杂度"
> "是预设选项不是直接走完！"

- [ ] ❌ **没有任何剧本**写在 Go 字面量 `var mockScenarios* = []*MockScenario{...}` 中
- [ ] ✅ 所有剧本都在 `internal/server/mock_scenarios/*.yaml`
- [ ] ✅ Go 字面量**仅**作为 fallback 保留，且加 deprecation 注释
- [ ] ❌ **没有任何**模板引擎（`internal/server/mock_scenario_template.go` **不存在**）
- [ ] ❌ **没有任何** `{{ .UserText }}` / `{{ .ToolResult.X }}` 模板引用
- [ ] ❌ **没有任何**剧本接 free-form text 推进
- [ ] ✅ 分支 = `mock_branch_choice` + 预设 `options` 列表（用户只能点 chip）
- [ ] ✅ `branch-pick` API **拒绝**任何 free-form text 字段
- [ ] ✅ text_delta.text 是 YAML 写死的字符串
- [ ] ✅ 演示团队加新剧本 = 写 YAML + 重启（或开热重载），无需 Go 工程师

---

## T1. 剧本 YAML schema 定义

- [ ] `internal/server/mock_scenario_schema.go` 存在
- [ ] `LoadedScenario` / `YAMLStep` / `YAMLEvent` / `YAMLBranchOption` 结构体定义
- [ ] 所有字段带 `yaml:"..."` tag，snake_case
- [ ] `Validate() error` 实现
- [ ] 缺 `id` / 空 `steps` / 空 `events` → 拒绝
- [ ] `mock_branch_choice.options` < 2 → 拒绝
- [ ] `text_delta.text` 含 `{{` → 拒绝（**严禁模板**）
- [ ] `go test -run TestSchema -v` 全过（6+ 用例）

---

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
- [ ] `go test -run TestLoader -v` 全过（12+ 用例）

---

## T3. MockEngine 集成 + 预设分支推进

- [ ] `MockEngine.scenarios` 改为 `map[string]*MockScenario`
- [ ] 移除对 `var mockScenarios` 的直接引用
- [ ] `NewMockEngine(scenarios []*MockScenario)` 构造 map
- [ ] `POST /api/agent/branch-pick` 端点实现
- [ ] 入参只有 `{scenario_id, branch_id, option_id}`，**无** `user_text` 字段
- [ ] 拒绝 free-form text（返回 400）
- [ ] 拒绝未知 option_id（返回 404）
- [ ] 跳到对应 step（按 `option_id` 匹配同名 step）
- [ ] tool_result 真实化走 schema 替换（不靠模板）
- [ ] `go test -run TestMockEngine -v` 全过（6+ 新增用例）
- [ ] 现有 `TestMockEngine*` 测试 0 修改仍通过

---

## T4. 内置剧本迁移

- [ ] `internal/server/mock_scenarios/builtin/` 目录存在
- [ ] 12 个 v1 剧本 YAML 文件存在
- [ ] `internal/server/mock_scenarios/v2/` 目录存在
- [ ] 8 个 v2 剧本 YAML 文件存在
- [ ] v2 tool_result 是占位字符串
- [ ] `agent_mock_scenarios.go` 顶部 deprecation 注释
- [ ] `agent_mock_v2_scenarios.go` 顶部 deprecation 注释
- [ ] Go 字面量保留作为 fallback
- [ ] `TestMigration_*` 通过
- [ ] 启动 log：`Loaded 20 scenarios from YAML (overriding 0 Go-literal fallbacks)`

---

## T5. CLI flag + 服务集成

- [ ] `cmd/encv/main.go` 新增 `-mock-scenarios-dir` flag
- [ ] `ServerOptions.ScenariosDir` 字段
- [ ] `NewServer` 初始化 loader
- [ ] 加载失败 → log.Fatal
- [ ] `go run ./cmd/encv -mock-scenarios-dir=./testdata/yaml` 启动成功
- [ ] `TestMain_FlagParse` / `TestServer_NewServer_*` 通过

---

## T6. 配置 schema 增量

- [ ] `AgentSettings.MockScenariosDir` 字段
- [ ] `schema.json` 新增 1 字段
- [ ] `Settings.vue` 渲染目录输入 + 选择按钮
- [ ] `TestConfig_*` 通过

---

## T7. 端到端集成测试

- [ ] sandbox 目录准备
- [ ] E2E: YAML 剧本端到端（text_delta 是预设字符串）
- [ ] E2E: 预设选项分支推进（POST option_id → 跳 step）
- [ ] E2E: free-form text 被拒绝（带 user_text → 400）
- [ ] E2E: 未知 option_id 被拒绝（404）
- [ ] E2E: 热重载
- [ ] `go test -run TestE2E -v` 全过（5+ 用例）

---

## T8. 文档与示例

- [ ] `mock_scenarios/SCHEMA.md` 存在
- [ ] `mock_scenarios/EXAMPLE_basic.yaml` 存在（5 步最小）
- [ ] `mock_scenarios/EXAMPLE_branch.yaml` 存在（预设选项示例）
- [ ] `agent_mock_scenarios.go` 顶部注释指向 SCHEMA.md
- [ ] 注释强调：**剧本不接 free-form text 输入**

---

## 类型检查

- [ ] `go build ./cmd/encv` 0 错误
- [ ] `vue-tsc --noEmit` 0 错误
- [ ] `pnpm test --run` 0 失败（mobile 前端）

---

## 关键约束再确认

| 约束 | 状态 |
|------|------|
| 剧本严禁走用户输入 | ❌ 严禁 / ✅ 拒绝 free-form text |
| 不引入模板引擎 | ❌ `mock_scenario_template.go` 不存在 |
| 不增加不必要的复杂度 | ✅ 只加 loader + YAML |
| 分支 = 预设选项 chip | ✅ `mock_branch_choice.options` |
| 文本永远是预设字符串 | ✅ text_delta 不允许 `{{` |
| 向后兼容 | ✅ Go 字面量 fallback |
| 加新剧本不需要 Go 工程师 | ✅ 改 YAML 即可 |
