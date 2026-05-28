# CI 构建报错修复计划

## 问题分析

### 问题 1：CI 编译错误 — `Unresolved reference 'REQUEST_CODE_PLUGIN_PICK'`

**错误日志**（Step 28: Build Debug APK）：
```
e: GoProcessPlugin.kt:33:21 Unresolved reference 'REQUEST_CODE_PLUGIN_PICK'.
e: GoProcessPlugin.kt:33:47 Unresolved reference 'REQUEST_CODE_INSTALL_CONFIRM'.
```

**根因**：在 Kotlin 中，类注解（`@CapacitorPlugin`）不能引用同一类的 `companion object` 中定义的常量。因为注解在类定义之前求值，此时 `companion object` 尚不存在。

**当前代码**（`GoProcessPlugin.kt` L31-40）：
```kotlin
@CapacitorPlugin(
    name = "GoProcess",
    requestCodes = [REQUEST_CODE_PLUGIN_PICK, REQUEST_CODE_INSTALL_CONFIRM]  // ← 编译错误
)
class GoProcessPlugin : Plugin() {
    companion object {
        const val REQUEST_CODE_PLUGIN_PICK = 9001       // ← 定义在 companion object 中
        const val REQUEST_CODE_INSTALL_CONFIRM = 9002
    }
}
```

**修复**：将常量提升为顶层常量（top-level `const val`），放在类外部，或直接在注解中使用字面量。

**推荐方案**：将常量移到类外部作为顶层常量（Kotlin 惯用写法）：
```kotlin
private const val REQUEST_CODE_PLUGIN_PICK = 9001
private const val REQUEST_CODE_INSTALL_CONFIRM = 9002

@CapacitorPlugin(
    name = "GoProcess",
    requestCodes = [REQUEST_CODE_PLUGIN_PICK, REQUEST_CODE_INSTALL_CONFIRM]
)
class GoProcessPlugin : Plugin() {
    companion object {
        // 其他 companion 常量...
    }
}
```

### 问题 2：为什么构建主应用会出现插件的任务？

**现象**：`./gradlew assembleDebug` 触发了 `:plugin-mpv-player:assembleDebug` 和 `:convert_plugin-mpv-player_debug` 任务。

**根因**：这是 **ComboLite aar2apk 插件的正常行为**，不是 bug。

**依赖链**：
1. `settings.gradle.kts` 中 `include(":plugin-mpv-player")` — 将插件模块纳入 Gradle 项目
2. `build.gradle.kts`（根项目）中配置了 `aar2apk { modules { module(":plugin-mpv-player") } }` — 注册插件模块
3. `app/build.gradle.kts` 中配置了 `packagePlugins { enabled.set(true) }` — 启用自动打包
4. ComboLite 的 aar2apk Gradle 插件在主应用构建时自动执行：
   - 先构建插件模块的 AAR（`:plugin-mpv-player:assembleDebug`）
   - 再将 AAR 转换为 APK（`:convert_plugin-mpv-player_debug`）
   - 最终 APK 放入 `debug_plugins/` 目录供运行时加载

**结论**：这是 ComboLite 插件系统的设计行为，不需要修复。插件 APK 是主应用运行时加载插件所必需的。

---

## 实施步骤

### Step 1：修复 GoProcessPlugin.kt 编译错误

将 `REQUEST_CODE_PLUGIN_PICK` 和 `REQUEST_CODE_INSTALL_CONFIRM` 从 `companion object` 移到文件顶层：

```kotlin
// 文件顶层（类外部）
private const val REQUEST_CODE_PLUGIN_PICK = 9001
private const val REQUEST_CODE_INSTALL_CONFIRM = 9002

@CapacitorPlugin(
    name = "GoProcess",
    requestCodes = [REQUEST_CODE_PLUGIN_PICK, REQUEST_CODE_INSTALL_CONFIRM]
)
class GoProcessPlugin : Plugin() {
    companion object {
        private const val TAG = "ENCV-go"
        // ... 其他 companion 常量不变
    }
    // ... 类体中引用 REQUEST_CODE_* 的地方不需要改，Kotlin 顶层常量在同一文件内可见
}
```

### Step 2：验证

- 确认 `GoProcessPlugin.kt` 中所有引用 `REQUEST_CODE_PLUGIN_PICK` 和 `REQUEST_CODE_INSTALL_CONFIRM` 的地方仍然有效（顶层 `const val` 在同一文件内对所有代码可见）
- CI 构建应通过

### Step 3：清理日志文件

删除 `job_logs/` 目录和 `job_logs.zip`
