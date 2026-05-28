# Tasks

- [ ] Task 1: 修复文本预览 iframe 高度（F0）
  - [ ] 1.1 修改 `internal/openlist/web/static/preview/text.html` — `#textContent` height 从 `100vh` 改为 `100%`
  - [ ] 1.2 确保 `app/encv-mobile/src/views/FilePreview.vue` 的 `.text-preview` 有明确高度约束（检查 ion-content 的 flex 布局）

- [ ] Task 2: 修复 InstallConfirmActivity 启动（F1）
  - [ ] 2.1 修改 GoProcessPlugin.kt L422-428：移除错误的 `action` 设置，添加 `FLAG_ACTIVITY_NEW_TASK` 检查
  - [ ] 2.2 修改 GoProcessPlugin.kt L556-562：同上修复
  - [ ] 2.3 添加 `import android.app.Activity`（如果缺失）

- [ ] Task 3: 新增 SkipDeepCheck 参数（F2.1）
  - [ ] 3.1 修改 `internal/v2/plugins/interfaces/interfaces.go` — VerifyOptions 新增 `SkipDeepCheck bool` 字段
  - [ ] 3.2 修改 `internal/v2/plugins/video/content_verifier.go` — Verify() 方法中条件跳过 `runDeepVideoIntegrityCheck`

- [ ] Task 4: verifyContainer 设置 SkipDeepCheck（F2.2）
  - [ ] 4.1 修改 `internal/v2/plugins/video/plugin.go` — verifyContainer() 中设置 `verifyOpts.SkipDeepCheck = true`

- [ ] Task 5: 验证修复
  - [ ] 5.1 Go 编译通过：`go build ./cmd/encv/`
  - [ ] 5.2 Kotlin 编译通过（检查 Gradle）
  - [ ] 5.3 运行 encryption_roundtrip_e2e_test.go 验证 SkipDeepCheck 生效

# Task Dependencies
- [Task 1] 可独立执行
- [Task 2] 可独立执行
- [Task 3] → [Task 4]（先定义参数，再使用）
- [Task 5] 依赖 [Task 1-4] 完成