# Tasks (Round 3 — 深度适配)

## Task 13: isAlistEncrypted 移除 isEncrypted 排除（REQ-11）
- [ ] 13.1: `useAlistEncrypt.ts` L27 — 移除 `|| file.isEncrypted` 条件，只保留 `file.isDirectory` 检查 + 后缀匹配
- [ ] 13.2: 更新测试：`aeFile`（isEncrypted=true 的 .bin 文件）的 isAlistEncrypted 断言需要调整 — 如果 aeFile.name 以 .bin 结尾则仍返回 true

## Task 14: getAlistActions 扩展三分支（REQ-13）
- [ ] 14.1: `actions.ts` — 重构为三分支：
  - **A** `isAlistEncrypted(file)` → decrypt + stream-preview（现有不变）
  - **B** `file.isEncrypted === true` → decrypt action（openNewTask(path, 'decrypt')）← 新增
  - **C** else（非目录）→ encrypt action（现有不变）

## Task 15: Files.vue .bin 点击预览路径（REQ-12）
- [ ] 15.1: `Files.vue` — 在 handleFileClick() 中、`if (file.isEncrypted)` 分支之前，增加 `if (isAlistEncrypted(file))` 分支
- [ ] 15.2: 该分支调用 promptPassword → getStreamUrl → 打开播放器
- [ ] 15.3: 需要确认播放器打开方式（router.push 到 preview 带 stream URL？还是直接用 ArtPlayer？）

## Task 16: 解密任务 doPredict 降级（REQ-14）
- [ ] 16.1: `useNewTaskModal.ts` — present() 后的 doPredict 回调中（约 L128-134），增加判断：
  - `if (state.taskType === 'decrypt' && state.candidates.length === 0 && !state.predictedPlugin)`
  - → 设置 `state.predictedPlugin = 'auto-detect'`

## Task 17: Mock 测试更新（REQ-15）
- [ ] 17.1: 扩展 `__tests__/features.alist-encrypt.test.ts`：
  - isAlistEncrypted 对 isEncrypted=true 非 .bin 文件返回 false
  - getAlistActions 对 isEncrypted=true 文件返回 decrypt action（新增分支 B）
  - getAlistActions 三分支全覆盖

## Task 18: 编译与全量测试回归验证
- [ ] 18.1: vue-tsc --noEmit 零错误
- [ ] 18.2: vitest run 全部通过
- [ ] 18.3: vite build 成功

# Dependencies
- [Task 13] 无依赖，最高优先级（核心重构基础）
- [Task 14] 可与 Task 13 并行（actions.ts 独立修改）
- [Task 15] 依赖 Task 13（handleFileClick 需要 isAlistEncrypted 正确识别 .bin）
- [Task 16] 可与 Task 13 并行（useNewTaskModal 独立修改）
- [Task 17] 依赖 Task 13-16 完成
- [Task 18] 依赖 Task 13-17 全部完成
