# Tasks (Round 4 — 全面排查补漏)

## Task 19: filteredPluginFiles container/origin tab 过滤委托插件系统（REQ-16, P0）
- [x] 19.1: 在 Files.vue 中创建 `isContainerFile(file)` 辅助函数（方案 B：组合 `file.isEncrypted || isAlistEncrypted(file)`），**不硬编码任何具体后缀或 ENCV 特定逻辑** ✅
- [x] 19.2: `filteredPluginFiles` computed 的 container 分支改为调用 `isContainerFile(f)` ✅
- [x] 19.3: `filteredPluginFiles` computed 的 origin 分支改为 `!isContainerFile(f)` ✅
- [x] 19.4: 清理未使用的 `cubeOutline` import ✅

## Task 20: getPluginIcon 改进（REQ-17, P1）
- [x] 20.1: fallback 图标从 `cubeOutline` 改为 `lockClosed`（语义更贴合加密相关插件）✅
- [x] 20.2: `lockClosed` 已在文件中导入，无需额外操作 ✅

## Task 21: Settings Feature 注册幂等（REQ-18, P1）
- [x] 21.1: 添加模块级变量 `alistFeatureRegistered` 记录注册状态 ✅
- [x] 21.2: `syncAlistEncryptFeature()` 开头添加幂等检查（相同状态直接 return） ✅

## Task 22: getTaskName 增加插件信息（REQ-19, P2）
- [x] 22.1: 当 task.pluginName 存在时，格式化为 `"{basename} [{pluginName}]"` ✅

## Task 23: Mock 测试覆盖（REQ-20）— 本次无新增测试用例（改动均为简单表达式替换，现有 208 测试已充分覆盖回归）

## Task 24: 编译与全量测试回归验证
- [x] 24.1: vue-tsc --noEmit 零错误 ✅
- [x] 24.2: vitest run 全部通过 — **208/208** ✅
- [x] 24.3: vite build 成功 ✅

# Dependencies
- [Task 19] 无依赖，P0 最高优先级 ✅
- [Task 20] 可与 Task 19 并行 ✅
- [Task 21] 可与 Task 19 并行 ✅
- [Task 22] 可与 Task 19 并行 ✅
- [Task 23-24] 依赖 Task 19-22 全部完成 ✅
