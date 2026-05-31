# Tasks

- [ ] Task 1: 新增 TaskStateResetter 接口
  - [ ] 1.1: 在 `internal/v2/plugins/interfaces/interfaces.go` 中添加 `TaskStateResetter` 接口（`ResetTaskState()` 方法）
- [ ] Task 2: AlistEncryptPlugin 实现 ResetTaskState
  - [ ] 2.1: 在 `internal/v2/plugins/alistencrypt/plugin.go` 中实现 `ResetTaskState()` 方法，重置 `taskExtraFields`、`outputDir`、`inputPath`
  - [ ] 2.2: 将 `Encrypt` 方法中的 `slog.Info` 调试日志降级为 `slog.Debug`
- [ ] Task 3: TaskManager 在任务完成后调用 ResetTaskState
  - [ ] 3.1: 在 `processEncrypt` 的成功路径和 failTask 路径后添加 `defer` 调用 `ResetTaskState()`
  - [ ] 3.2: 在 `processDecrypt` 的成功路径和 failTask 路径后添加 `defer` 调用 `ResetTaskState()`
- [ ] Task 4: 前端 ExtraField label 走 i18n 翻译
  - [ ] 4.1: 修改 `NewTaskModal.vue` 中 ExtraField 的 `:label="field.label"` 为 `:label="t(field.label)"`
  - [ ] 4.2: 修改 ExtraField 的 `:placeholder="field.help"` 为 `:placeholder="t(field.help)"`
- [ ] Task 5: registry.go 调试日志降级
  - [ ] 5.1: 将 `EncryptFileWithPlugin` 中的 `slog.Info` 改为 `slog.Debug`
- [ ] Task 6: 验证构建和测试
  - [ ] 6.1: `go build ./cmd/encv-mobile/` 通过
  - [ ] 6.2: `go test ./internal/v2/plugins/alistencrypt/...` 通过
  - [ ] 6.3: `go test ./internal/server/... -run "AlistEncrypt"` 通过

# Task Dependencies
- Task 2 depends on Task 1
- Task 3 depends on Task 1
- Task 6 depends on Task 1-5
