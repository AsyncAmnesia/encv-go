# Tasks (Round 3 — 深度适配)

## Task 13: isAlistEncrypted 消除硬编码（REQ-11）
- [ ] 13.1: `useAlistEncrypt.ts` — 重构 `isAlistEncrypted(file)` 函数：
  - **优先级 1**：`file.isEncrypted === true` → 返回 true（通用容器加密标记）
  - **优先级 2**：检查文件名是否以已注册插件的 `containerExtension` 结尾（需要从 PluginMeta 列表获取后缀集合；如果无法获取则用配置项 suffix fallback）
  - **目录** → 始终返回 false
- [ ] 13.2: 需要确认 PluginMeta/containerExtension 数据来源（Files.vue 的 pluginList？还是 API 接口？）并确定如何传入 isAlistEncrypted
- [ ] 13.3: 更新现有测试中 `isAlistEncrypted(aeFile)` 的断言（aeFile.isEncrypted 现在应导致返回 true）

## Task 14: .bin 文件预览走流式解密路径（REQ-12）
- [ ] 14.1: `Files.vue` — 在 `handleFileClick()` 中，在现有 `if (file.isEncrypted)` 分支**之前或并行**，增加 `if (isAlistEncrypted(file))` 分支
- [ ] 14.2: 该分支调用 alist-encrypt 的流式预览逻辑：promptPassword → getStreamUrl → router.push 到 player
- [ ] 14.3: 确保 ENCV 容器文件（isEncrypted=true）的预览路径不受影响

## Task 15: 解密任务 doPredict 降级处理（REQ-14）
- [ ] 15.1: `useNewTaskModal.ts` — 在 doPredict 回调（present() 之后，约 L128-134）中增加判断：
  - 如果 taskType === 'decrypt' 且 cands 为空且 predictedPlugin 为 null
  - 则设置 state.predictedPlugin = 'auto-detect'（或类似标记），让 NewTaskModal 不再显示"分析中"
- [ ] 15.2: 或者在 NewTaskModal.vue 的 isPredicting computed 中对 decrypt 类型做特殊处理

## Task 16: Mock 测试更新（REQ-15）
- [ ] 16.1: 扩展 `__tests__/features.alist-encrypt.test.ts`：
  - 测试 `isAlistEncrypted(isEncryptedTrueFile)` 返回 true（REQ-11 场景 11.3）
  - 测试 `isAlistEncrypted(otherSuffixFile)` 返回 true（REQ-11 场景 11.2，如 .enc 后缀）
  - 测试 `getAlistActions(otherSuffixFile)` 返回 decrypt action（REQ-13 自动解决验证）

## Task 17: 编译与全量测试回归验证
- [ ] 17.1: vue-tsc --noEmit 零错误
- [ ] 17.2: vitest run 全部通过（含更新后的测试断言）
- [ ] 17.3: vite build 成功

# Dependencies
- [Task 13] 无依赖，最高优先级（核心重构，其他 Task 依赖此修复）
- [Task 14] 依赖 Task 13 完成（handleFileClick 需要 isAlistEncrypted 正确识别 .bin）
- [Task 15] 可与 Task 13 并行（useNewTaskModal 独立修改）
- [Task 16] 依赖 Task 13-15 完成
- [Task 17] 依赖 Task 13-16 全部完成
