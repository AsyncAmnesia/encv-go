# Tasks

- [ ] Task 1: 修复文本预览换行状态一致性（D0）
  - [ ] 1.1 分析 text.html 的 `isWrapping` 初始化逻辑
  - [ ] 1.2 修复文本加载后的 `no-wrap` 类设置逻辑（确保与当前 `isWrapping` 状态一致）
  - [ ] 1.3 验证换行按钮点击后 CSS 类正确切换

- [ ] Task 2: 修复 InstallConfirmActivity 启动失败（D1）
  - [ ] 2.1 移除 GoProcessPlugin.kt 中错误的 `action="com.encvgo.app.INSTALL_RESULT"` 设置
  - [ ] 2.2 添加 `FLAG_ACTIVITY_NEW_TASK` 标志（当 context 不是 Activity 时）
  - [ ] 2.3 验证 Activity 能正常启动并显示

- [ ] Task 3: 新增 SkipDeepCheck 参数（D2.1）
  - [ ] 3.1 在 `interfaces.go` 的 `VerifyOptions` 结构体中添加 `SkipDeepCheck bool` 字段
  - [ ] 3.2 在 `content_verifier.go` 的 `Verify()` 方法中，当 `SkipDeepCheck=true` 时跳过 `runDeepVideoIntegrityCheck`

- [ ] Task 4: PostEncryptProcessor 设置 SkipDeepCheck（D2.2）
  - [ ] 4.1 在 `plugin.go` 的 `verifyContainer()` 方法中，当 `isPostEncryptVerify=true` 时设置 `verifyOpts.SkipDeepCheck=true`
  - [ ] 4.2 确保 `SkipStructCheck=true` 和 `SkipDeepCheck=true` 同时设置

- [ ] Task 5: 验证修复效果
  - [ ] 5.1 运行 Go 测试验证 SkipDeepCheck 生效
  - [ ] 5.2 检查 Kotlin 代码编译通过
  - [ ] 5.3 检查 text.html 逻辑正确

# Task Dependencies
- [Task 1] 可独立执行
- [Task 2] 可独立执行
- [Task 3] → [Task 4]（先定义参数，再使用）
- [Task 5] 依赖 [Task 1-4] 完成