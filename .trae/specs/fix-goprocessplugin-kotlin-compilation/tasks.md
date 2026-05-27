# Tasks

- [ ] Task 1: 修复 GoProcessPlugin.kt 编译错误
  - [ ] SubTask 1.1: 添加缺失的 import——在 `import android.content.IntentFilter` 之后添加 `import android.content.BroadcastReceiver` 和 `import android.content.Context`，遵循项目 import 排序范式（android.* 在前）
  - [ ] SubTask 1.2: 合并两个 companion object——将第 409 行 `private companion object` 中的 `REQUEST_CODE_PLUGIN_PICK = 9001` 移入第 29 行的 `companion object`（在 `TAG` 之后），然后删除整个 `private companion object` 块（第 409-411 行）
  - [ ] SubTask 1.3: 修复 nullable 安全调用——第 589 行 `path.isEmpty()` 改为 `path.isEmpty()`（此处 `path` 是 `call.getString("path", "")` 的返回值，类型为 `String` 非 `String?`，但 CI 报错说 `String?`，需检查 `getString` 的返回类型；如果确实是 `String?`，则改为 `path.isNullOrEmpty()`）；第 598 行 `path.removePrefix("/")` 同理

# Task Dependencies

无依赖，所有子任务可顺序执行

# 备注

- `call.getString("path", "")` 在 Capacitor 的 `PluginCall` 中返回 `String?`（即使有默认值），所以 `path` 是 `String?` 类型
- 第 589 行应改为 `path.isNullOrEmpty()`
- 第 598 行 `path.removePrefix("/")` 应改为 `path!!.removePrefix("/")`（因为前面已确认 `path` 非空）或使用 `path?.removePrefix("/") ?: ""`
