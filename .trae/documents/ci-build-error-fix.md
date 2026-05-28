# CI 构建报错修复计划

## 问题分析

### 问题 1：CI 编译错误 — `Unresolved reference 'REQUEST_CODE_PLUGIN_PICK'`

**错误日志**（Step 28: Build Debug APK）：
```
e: GoProcessPlugin.kt:33:21 Unresolved reference 'REQUEST_CODE_PLUGIN_PICK'.
e: GoProcessPlugin.kt:33:47 Unresolved reference 'REQUEST_CODE_INSTALL_CONFIRM'.
```

**根因**：在 Kotlin 中，类注解（`@CapacitorPlugin`）不能引用同一类的 `companion object` 中定义的常量。因为注解在类定义之前求值，此时 `companion object` 尚不存在。

**修复**：将 `REQUEST_CODE_PLUGIN_PICK` 和 `REQUEST_CODE_INSTALL_CONFIRM` 从 `companion object` 移到文件顶层（top-level `const val`）。

---

### 问题 2：构建主应用时不应触发插件构建任务

**现象**：`./gradlew assembleDebug` 触发了 `:plugin-mpv-player:assembleDebug` 和 `:convert_plugin-mpv-player_debug` 任务。

**根因**：`app/build.gradle.kts` 中的 `packagePlugins { enabled.set(true) }` 配置让 ComboLite aar2apk 插件在主应用构建时自动执行插件 AAR→APK 转换，并将插件 APK 放入 `debug_plugins/` 目录（最终打包进主 APK 的 assets）。

**为什么这是错误的**：
1. CI 中已有**单独的步骤**（Step 25-26）构建和打包插件 APK
2. 插件 APK **不应打包到主应用中**——ComboLite 的插件是运行时动态加载的（从 `filesDir/plugins/` 安装），不应嵌入主 APK
3. 重复构建浪费时间，且可能导致构建冲突

**修复**：将 `packagePlugins { enabled.set(true) }` 改为 `enabled.set(false)`，禁用主应用构建时的自动插件打包。CI 中通过单独的 Gradle 任务（`convert_plugin-mpv-player_debug`）构建插件 APK。

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

### Step 2：禁用主应用构建时的自动插件打包

修改 `app/build.gradle.kts`：

```kotlin
packagePlugins {
    enabled.set(false)  // 禁用：插件 APK 由 CI 单独构建，不应打包到主 APK
    buildType.set(PackageBuildType.DEBUG)
    pluginsDir.set("debug_plugins")
}
```

### Step 3：验证

- 确认 `GoProcessPlugin.kt` 编译通过
- 确认 `assembleDebug` 不再触发 `:plugin-mpv-player` 任务
- 确认 CI 中单独的插件构建步骤仍正常工作

### Step 4：清理日志文件

删除 `job_logs/` 目录和 `job_logs.zip`
