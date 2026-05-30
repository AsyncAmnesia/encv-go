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

## REQ-5: 编译与测试通过 — 回归验证通过
- [x] 5.1: vue-tsc 零错误 ✅
- [x] 5.2: vitest 全通过 (206/206) ✅
- [x] 5.3: vite build 成功 ✅

---

## REQ-6: 新建任务必须传递 pluginName 到后端（新增）
- [x] 6.1: `createTask()` API 签名包含 `pluginName?: string` 参数 ✅
- [x] 6.2: `useNewTaskModal.onSubmit` 将用户选择的插件名传递给 createTask() ✅
- [x] 6.3: 单插件自动预测场景 pluginName 正确 ✅
- [x] 6.4: 多插件切换选择后最终提交使用正确的 pluginName ✅

## REQ-7: 任务创建防重复提交（新增）
- [x] 7.1: useNewTaskModal 存在 submitting 锁变量 ✅
- [x] 7.2: 快速双击只触发一次 createTask 调用 ✅

## REQ-8: 普通文件长按菜单显示加密操作（插件系统架构内解决）（新增）
- [x] 8.1: alist-encrypt Feature `isActive` 改为 `!file.isDirectory`（对所有非目录文件激活） ✅
- [x] 8.2: `getAlistActions(normalFile)` 返回包含 encrypt action（通过 isActive gatekeeper ✅） ✅
- [x] 8.3: encrypt action 点击调用 openNewTask(path, 'encrypt') ✅
- [x] 8.4: `.bin` 加密文件仍返回 decrypt + preview（回归保护） ✅
- [x] 8.5: 目录文件 isActive 返回 false，不显示任何 alist-encrypt action ✅

## REQ-9: 插件模式空状态下拉刷新不泄漏主列表（新增）
- [x] 9.1: handleRefresh 在 selectedPlugin 存在时刷新 pluginFiles ✅
- [x] 9.2: 插件空状态 + 下拉刷新后不出现主文件列表 ✅
- [x] 9.3: 插件有文件 + 下拉刷新后正确更新插件文件列表 ✅

## REQ-10: Mock 测试覆盖新增场景（新增）
- [x] 10.1: isActive: 非目录返回 true / 目录返回 false ✅
- [x] 10.2: getAlistActions 普通文件/加密文件双分支测试 ✅
- [x] 10.3: useNewTaskModal onSubmit pluginName 传递测试 ✅
- [x] 10.4: useNewTaskModal 双击防重入测试 ✅
- [x] 10.5: Files.vue plugin mode refresh 行为验证 ✅
