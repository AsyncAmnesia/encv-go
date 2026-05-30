# Checklist

## REQ-1: 统一操作入口（Feature Actions 唯一性）— 上一轮已完成 ✅
- [x] 1.1: actions.ts decrypt handler 使用 useNewTaskModal 替代 router.push ✅
- [x] 1.2: actions.ts encrypt handler 使用 useNewTaskModal 替代 router.push ✅
- [x] 1.3: Files.vue Section 2 内联加解密代码已删除，由 getAllActions() Feature action 提供 ✅
- [x] 1.4: handleEncryptFile/handleDecryptFile 已清理（dead code） ✅

## REQ-2: 密码弹窗正确交互 — 上一轮已完成 ✅
- [x] 2.1: confirm 后弹窗自动关闭（return true） ✅
- [x] 2.2: cancel 后弹窗关闭并 resolve null ✅

## REQ-3: 字幕查询稳定性 — 上一轮已完成 ✅
- [x] 3.1: 防抖窗口内重复调用返回已有 Promise（非 null） ✅

## REQ-4: FileInfo 元信息完整 — 上一轮已完成 ✅
- [x] 4.1: 加密文件展示解码后名称（灰色斜体 + border-top 分隔） ✅
- [x] 4.2: 展示加密状态 badge（warning 色 Yes） ✅

## REQ-5: 编译与测试通过 — 上一轮已完成，本轮回归验证
- [ ] 5.1: vue-tsc 零错误（待 Task 12 回归验证）
- [ ] 5.2: vitest 全通过（待 Task 12 回归验证）
- [ ] 5.3: vite build 成功（待 Task 12 回归验证）

---

## REQ-6: 新建任务必须传递 pluginName 到后端（新增）
- [ ] 6.1: `createTask()` API 签名包含 `pluginName?: string` 参数
- [ ] 6.2: `useNewTaskModal.onSubmit` 将用户选择的插件名传递给 createTask()
- [ ] 6.3: 单插件自动预测场景 pluginName 正确
- [ ] 6.4: 多插件切换选择后最终提交使用正确的 pluginName

## REQ-7: 任务创建防重复提交（新增）
- [ ] 7.1: useNewTaskModal 存在 submitting 锁变量
- [ ] 7.2: 快速双击只触发一次 createTask 调用

## REQ-8: 普通文件长按菜单显示加密操作（新增）
- [ ] 8.1: `getAlistActions(normalFile)` 返回包含 encrypt action
- [ ] 8.2: encrypt action 点击调用 openNewTask(path, 'encrypt')
- [ ] 8.3: `.bin` 加密文件仍返回 decrypt + preview（回归保护）
- [ ] 8.4: Feature isActive 判定允许普通文件走到 getFileActions 的 encrypt 分支

## REQ-9: 插件模式空状态下拉刷新不泄漏主列表（新增）
- [ ] 9.1: handleRefresh 在 selectedPlugin 存在时刷新 pluginFiles
- [ ] 9.2: 插件空状态 + 下拉刷新后不出现主文件列表
- [ ] 9.3: 插件有文件 + 下拉刷新后正确更新插件文件列表

## REQ-10: Mock 测试覆盖新增场景（新增）
- [ ] 10.1: getAlistActions 普通文件/加密文件双分支测试
- [ ] 10.2: useNewTaskModal onSubmit pluginName 传递测试
- [ ] 10.3: useNewTaskModal 双击防重入测试
- [ ] 10.4: Files.vue plugin mode refresh 行为测试
