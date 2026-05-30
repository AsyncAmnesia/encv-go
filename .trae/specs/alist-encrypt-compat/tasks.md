# Tasks

## 已完成（Phase 1-5 基础骨架）
- [x] Phase 1: Go 后端 FileFeature 接口定义
- [x] Phase 2: 后端 AlistEncrypt Feature 模块实现
- [x] Phase 3: API 层适配与集成测试
- [x] Phase 4: FileFeature 架构 UI 隔离骨架（types + useFileFeatures）
- [x] Phase 5.1-5.2: alist-encrypt Feature 子模块实现（actions/subtitle/badge/password-dialog/useAlistEncrypt）
- [x] Phase 5.3: Files.vue 重构为使用 useFileFeatures（badges/subtitles/actions 集成）
- [x] Phase 5.4: App.vue 注册 createAlistEncryptFeature()

## 待修复（Phase 6: 适配问题修复）

### Task 6.1: 解密操作路由跳转反模式修复
- [ ] 6.1a: actions.ts 的 `createDecryptAction` 使用 `router.push('/tabs/tasks')` 违反 capacitor.md §1.4
- [ ] 6.1b: 改为直接 import 并调用 `useNewTaskModal().openNewTask(path, 'decrypt')`
- [ ] 6.1c: actions.ts 的 `createEncryptAction` 同样问题，改为 openNewTask(path, 'encrypt')

### Task 6.2: 密码弹窗确认后不自动关闭
- [ ] 6.2a: password-dialog.ts confirm handler 中 `return false` 阻止了 alert 自动关闭
- [ ] 6.2b: 改为 `return true` 或在 resolve 后手动 dismiss

### Task 6.3: 字幕查询防抖竞态优化
- [ ] 6.3a: subtitle.ts 的 getAlistSubtitle 在防抖窗口内对同一文件重复调用返回 null
- [ ] 6.3b: 优化为返回已有 Promise 而非 null（避免字幕丢失）

### Task 6.4: 加密文件信息查询增强
- [ ] 6.4a: 加密文件的 FileInfo 页面应显示加密元信息（版本、原始大小等）
- [ ] 6.4b: 确认 FileInfo.vue 是否正确展示 alist-encrypt 文件的解码后名称

### Task 6.5: 预览流程适配
- [ ] 6.5a: 确认加密文件预览走 StreamPreview 还是先解密再预览
- [ ] 6.5b: 预览页面的密码输入流程是否与解密任务复用

## 待验证（Phase 7-8）

### Task 7: 端到端验证
- [ ] 7.1: 加密文件在文件列表显示 AE badge + 解码后文件名 subtitle
- [ ] 7.2: 长按菜单显示「解密」action → 点击打开 NewTaskModal（decrypt 模式）
- [ ] 7.3: 解密任务正常执行 → 完成后在文件列表可见解密产物
- [ ] 7.4: 非加密文件长按菜单显示「加密」action → 点击打开 NewTaskModal（encrypt 模式）
- [ ] 7.5: 加密任务正常执行 → 文件列表出现新的 .enc 文件带 AE badge
- [ ] 7.6: 密码弹窗可正常输入/取消/确认，确认后自动关闭
- [ ] 7.7: vue-tsc 零错误 + vitest 全通过 + vite build 成功

# Task Dependencies
- [Task 6.1] 无依赖，优先级最高（核心功能路径断裂）
- [Task 6.2] 无依赖，影响用户体验
- [Task 6.3] 依赖 6.1 完成后验证
- [Task 6.4] 可与 6.1 并行
- [Task 6.5] 可与 6.1 并行
- [Task 7] 依赖 6.1-6.5 全部完成
