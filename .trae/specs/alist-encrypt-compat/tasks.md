# Tasks (Round 4 — 全面排查补漏)

## Task 19: filteredPluginFiles container/origin tab 修复（REQ-16, P0）
- [ ] 19.1: `Files.vue` L1272 — container tab 过滤条件增加 `isAlistEncrypted(f)`
- [ ] 19.2: `Files.vue` L1274 — origin tab 过滤条件增加 `&& !isAlistEncrypted(f)` 排除加密文件

## Task 20: getPluginIcon 改进（REQ-17, P1）
- [ ] 20.1: 检查 PluginMeta 类型定义是否包含 icon 字段
- [ ] 20.2: 如有 icon 字段 → 方案 A（优先使用 PluginMeta.icon）；如无 → 方案 B（改 fallback 图标 + 预留接口）

## Task 21: Settings Feature 注册幂等（REQ-18, P1）
- [ ] 21.1: `Settings.vue` — 在 `syncAlistEncryptFeature()` 内添加已注册状态记录和幂等检查

## Task 22: getTaskName 增加插件信息（REQ-19, P2）
- [ ] 22.1: `Tasks.vue` L264-L268 — 当 task.pluginName 存在时，格式化为 `"{basename} [{pluginName}]"`

## Task 23: Mock 测试覆盖（REQ-20）
- [ ] 23.1: filteredPluginFiles 双 tab 过滤逻辑测试（container 含 / origin 排除 isAlistEncrypted 文件）
- [ ] 23.2: getTaskName 格式化测试

## Task 24: 编译与全量测试回归验证
- [ ] 24.1: vue-tsc --noEmit 零错误
- [ ] 24.2: vitest run 全部通过
- [ ] 24.3: vite build 成功

# Dependencies
- [Task 19] 无依赖，P0 最高优先级
- [Task 20] 可与 Task 19 并行
- [Task 21] 可与 Task 19 并行
- [Task 22] 可与 Task 19 并行
- [Task 23] 依赖 Task 19-22 完成
- [Task 24] 依赖 Task 19-23 全部完成
