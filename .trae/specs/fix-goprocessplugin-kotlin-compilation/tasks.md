# Tasks

- [ ] Task 1: 修复 GoProcessPlugin.kt 编译错误
  - [ ] SubTask 1.1: 添加缺失的 import：`android.content.BroadcastReceiver` 和 `android.content.Context`
  - [ ] SubTask 1.2: 将第 409 行的 `private companion object` 中的 `REQUEST_CODE_PLUGIN_PICK` 移入第 29 行的 `companion object`，删除 `private companion object`
  - [ ] SubTask 1.3: 修复第 589 行和第 598 行的 `String?` nullable 安全调用问题（`path.isEmpty()` → `path?.isEmpty() == true`，`path.removePrefix("/")` → `path.removePrefix("/")` 保留，因为 `path` 在此处已确认非空）

# Task Dependencies

无依赖，所有子任务可顺序执行
