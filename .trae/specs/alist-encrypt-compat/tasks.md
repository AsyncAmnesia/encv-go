# Tasks (Round 3 — 深度适配)

## Task 13: isAlistEncrypted 移除 isEncrypted 排除（REQ-11）
- [x] 13.1: `useAlistEncrypt.ts` L27 — 移除 `|| file.isEncrypted` 条件，只保留 `file.isDirectory` 检查 + **配置后缀**匹配
- [x] 13.2: 更新测试：测试用例中后缀值通过 mock `getFieldValue` 注入，不得硬编码任何具体后缀字符串

## Task 14: getAlistActions 扩展三分支（REQ-13）
- [x] 14.1: `actions.ts` — 重构为三分支：
  - **A** `isAlistEncrypted(file)` → decrypt + stream-preview（覆盖 alist-encrypt 加密文件，无论后缀为何值）
  - **B** `file.isEncrypted === true` → decrypt action（openNewTask(path, 'decrypt')）← 新增 ENCV 容器模式
  - **C** else（非目录）→ encrypt action（普通文件加密）

## Task 15: Files.vue alist-encrypt 加密文件点击预览路径（REQ-12）
- [x] 15.1: `Files.vue` — 在 handleFileClick() 中、`if (file.isEncrypted)` 分支之前，增加 `if (isAlistEncrypted(file))` 分支
- [x] 15.2: 该分支调用 promptPassword → getStreamUrl → 打开播放器

## Task 16: 解密任务 doPredict 降级（REQ-14）
- [x] 16.1: `useNewTaskModal.ts` — present() 后的 doPredict 回调中（两处），增加判断：
  - `if (state.taskType === 'decrypt' && state.candidates.length === 0 && !state.predictedPlugin)`
  - → 设置 `state.predictedPlugin = 'auto-detect'`

## Task 17: Mock 测试更新（REQ-15）
- [x] 17.1: 扩展 `__tests__/features.alist-encrypt.test.ts`：
  - isAlistEncrypted 对 isEncrypted=true 匹配后缀文件返回 true（行为变更验证）
  - getAlistActions 对 isEncrypted=true 不匹配后缀文件返回 decrypt action（新增分支 B 验证）
  - getAlistActions 三分支全覆盖（使用动态后缀常量 TEST_SUFFIX）

## Task 18: 编译与全量测试回归验证
- [x] 18.1: vue-tsc --noEmit 零错误 ✅
- [x] 18.2: vitest run 全部通过 — **208/208** ✅（+2 新测试）
- [x] 18.3: vite build 成功 ✅

# Dependencies
- [Task 13] 无依赖，最高优先级（核心重构基础） ✅
- [Task 14] 可与 Task 13 并行（actions.ts 独立修改） ✅
- [Task 15] 依赖 Task 13（handleFileClick 需要 isAlistEncrypted 正确识别加密文件） ✅
- [Task 16] 可与 Task 13 并行（useNewTaskModal 独立修改） ✅
- [Task 17] 依赖 Task 13-16 完成 ✅
- [Task 18] 依赖 Task 13-17 全部完成 ✅
