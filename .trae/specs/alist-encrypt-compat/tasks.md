# Tasks

## Task 7: createTask API 增加 pluginName 参数（REQ-6 前置）
- [ ] 7.1: `src/api/encv.ts` — `createTask()` 函数签名增加 `pluginName?: string` 参数（放在 `version` 之后或 `extraFields` 之前）
- [ ] 7.2: `createTask()` body 构建中，当 `pluginName` 有值时添加到请求 body
- [ ] 7.3: 确认 `EncvTask` 接口已有 `pluginName` 字段（已有 ✅），无需修改

## Task 8: useNewTaskModal.onSubmit 传递 pluginName + 防重入锁（REQ-6 + REQ-7）
- [ ] 8.1: `useNewTaskModal.ts` — 在 onSubmit handler 中，从 `state.candidates[state.selectedPluginIndex]?.name` 或 `state.predictedPlugin` 获取 pluginName 并传递给 `createTask()`
- [ ] 8.2: `useNewTaskModal.ts` — 添加 `submitting` ref 变量，onSubmit 开始时设为 true 并检查，modal.dismiss() 后重置为 false；提交按钮 disabled 绑定 submitting 状态
- [ ] 8.3: 验证：单插件自动预测场景下 pluginName 正确传递
- [ ] 8.4: 验证：多插件切换选择后最终提交使用正确的 pluginName

## Task 9: actions.ts 增加普通文件的 encrypt action（REQ-8）
- [ ] 9.1: `actions.ts` — 重构 `getAlistActions(file)` 为双分支：
  - **分支 A**（`isAlistEncrypted(file) === true`）：返回 decrypt + stream-preview（现有行为不变）
  - **分支 B**（`isAlistEncrypted(file) === false` 且非目录）：返回 encrypt action（调用 `openNewTask(path, 'encrypt')`）
- [ ] 9.2: 导入 `lockOpen`（lock-closed 图标）用于 encrypt action 的 icon
- [ ] 9.3: `index.ts` — 调整 `isActive` 判定或改为始终返回 true（让 collectActions 能走到 getFileActions），或将 encrypt action 放在 isActive 判断之外
- [ ] 9.4: 验证：`.bin` 文件长按仍显示 decrypt + preview（回归保护）

## Task 10: Files.vue handleRefresh 区分插件模式（REQ-9）
- [ ] 10.1: `Files.vue` — `handleRefresh()` 函数内部分叉：
  - 当 `selectedPlugin.value` 存在时 → 重新加载插件文件列表（重新调用 `searchPluginFiles` 更新 `pluginFiles`）
  - 当 `selectedPlugin.value` 不存在时 → 保持现有逻辑（刷新主 `files` 列表）
- [ ] 10.2: 验证：插件空状态 + 下拉刷新后不出现主文件列表

## Task 11: 补充 Mock 测试（REQ-10）
- [ ] 11.1: 新建或扩展 `__tests__/features.alist-encrypt.test.ts`：
  - 测试 `getAlistActions(normalFile)` 返回包含 `alist-encrypt` action
  - 测试 `getAlistActions(aeFile)` 返回 decrypt + preview（回归保护）
  - 测试 encrypt action 的 handler 调用 `openNewTask(path, 'encrypt')`
- [ ] 11.2: 新建 `__tests__/useNewTaskModal.test.ts`：
  - Mock `createTask` API，验证 onSubmit 传递了正确的 pluginName
  - Mock `modalController.create`，验证快速连续两次 onSubmit 只调用一次 createTask
- [ ] 11.3: 扩展 `__tests__/files.logic.test.ts`（如存在）或新建：
  - Mock 插件模式下的 handleRefresh 行为验证

## Task 12: 编译与全量测试（REQ-5 回归 + 新 REQ）
- [ ] 12.1: vue-tsc --noEmit 零错误
- [ ] 12.2: vitest run 全部通过（含新增测试）
- [ ] 12.3: vite build 成功

# Dependencies
- [Task 7] 无依赖，最高优先级（API 层变更，其他 Task 依赖此签名）
- [Task 8] 依赖 Task 7 完成（需要 createTask 新签名才能传参）
- [Task 9] 可与 Task 7-8 并行（actions.ts 独立于 API 签名变更）
- [Task 10] 可与 Task 7-9 并行（Files.vue 独立修复）
- [Task 11] 依赖 Task 8-10 完成（测试需基于修复后的代码编写断言）
- [Task 12] 依赖 Task 7-11 全部完成
