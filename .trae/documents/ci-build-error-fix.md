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

### 问题 3：installPlugin() 的系统安装兜底逻辑是错误的

**当前代码**（`GoProcessPlugin.kt` L386-420）：
```kotlin
@PluginMethod
fun installPlugin(call: PluginCall) {
    val apkPath = call.getString("apkPath") ?: ...
    val apkFile = File(apkPath)
    if (PluginManager.isInitialized) {
        startInstallConfirm(call, apkPath, apkFile.name)  // ✅ 正确：走 ComboLite 安装
        return
    }
    // ❌ 错误：系统安装兜底！
    val uri = FileProvider.getUriForFile(context, ..., apkFile)
    val intent = Intent(Intent.ACTION_INSTALL_PACKAGE).apply { ... }
    context.startActivity(intent)
    call.resolve(JSObject().put("success", true).put("method", "intent"))
}
```

**为什么这是错误的**：
1. ComboLite 插件 APK **不是普通 Android 应用**，不能用系统安装器（`ACTION_INSTALL_PACKAGE`）安装
2. 系统安装器会尝试将插件 APK 作为独立应用安装到系统，这会失败（插件 APK 没有 launcher Activity 等）
3. 即使系统安装器"成功"了，安装的也不是 ComboLite 插件，而是一个无法运行的独立应用
4. `PluginManager.isInitialized` 为 `false` 说明 ComboLite 框架未初始化，此时根本不应该尝试安装插件

**同样的问题**也存在于 `checkInstalledPlugins()` 的 `fallbackCheckInstalled()` 方法（L464-480），它在 PluginManager 未初始化时通过文件系统扫描查找插件 APK，但这只是检查文件是否存在，不代表插件已正确安装。

**修复**：
1. `installPlugin()`：当 `PluginManager.isInitialized` 为 `false` 时，直接返回错误，不使用系统安装兜底
2. `checkInstalledPlugins()`：当 `PluginManager.isInitialized` 为 `false` 时，返回空结果，不使用文件扫描兜底

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

### Step 2：移除 installPlugin() 的系统安装兜底逻辑

修改 `installPlugin()` 方法，当 PluginManager 未初始化时直接返回错误：

```kotlin
@PluginMethod
fun installPlugin(call: PluginCall) {
    val apkPath = call.getString("apkPath") ?: run {
        call.reject("apkPath is required")
        return
    }
    try {
        val apkFile = File(apkPath)
        if (!apkFile.exists()) {
            call.reject("APK file not found: $apkPath")
            return
        }
        if (!PluginManager.isInitialized) {
            call.reject("PluginManager not initialized, cannot install plugin")
            return
        }
        startInstallConfirm(call, apkPath, apkFile.name)
    } catch (e: Exception) {
        Log.e(TAG, "installPlugin failed", e)
        call.reject("Failed to install plugin: ${e.message}")
    }
}
```

### Step 3：移除 checkInstalledPlugins() 的文件扫描兜底

修改 `checkInstalledPlugins()` 方法，当 PluginManager 未初始化时返回空结果：

```kotlin
@PluginMethod
fun checkInstalledPlugins(call: PluginCall) {
    Log.d(TAG, "checkInstalledPlugins() called")
    val result = JSObject()
    try {
        if (PluginManager.isInitialized) {
            val plugins = PluginManager.getAllInstallPlugins()
            for (plugin in plugins) {
                result.put(plugin.id, true)
            }
            Log.i(TAG, "checkInstalledPlugins via ComboLite: $result")
            call.resolve(result)
            return
        }
        Log.w(TAG, "checkInstalledPlugins: PluginManager not initialized, returning empty")
        call.resolve(result)
    } catch (e: Exception) {
        Log.e(TAG, "checkInstalledPlugins failed", e)
        call.reject("Failed to check installed plugins: ${e.message}")
    }
}
```

同时删除 `fallbackCheckInstalled()` 方法。

### Step 4：禁用主应用构建时的自动插件打包

修改 `app/build.gradle.kts`：

```kotlin
packagePlugins {
    enabled.set(false)  // 禁用：插件 APK 由 CI 单独构建，不应打包到主 APK
    buildType.set(PackageBuildType.DEBUG)
    pluginsDir.set("debug_plugins")
}
```

### Step 5：验证

- 确认 `GoProcessPlugin.kt` 编译通过
- 确认 `assembleDebug` 不再触发 `:plugin-mpv-player` 任务
- 确认 CI 中单独的插件构建步骤仍正常工作
- 确认 `installPlugin()` 在 PluginManager 未初始化时返回错误而非走系统安装
- 确认 `checkInstalledPlugins()` 在 PluginManager 未初始化时返回空结果

### Step 6：清理日志文件

删除 `job_logs/` 目录和 `job_logs.zip`
