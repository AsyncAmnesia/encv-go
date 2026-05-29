# 修复 Logcat 日志集成 — 入口可见性 + 日志为空

## Bug 诊断

### 问题 1：DevTools 入口在 Web 端无效

```
Settings.vue L291: <ion-item @click="goDevTools">  ← 无 v-if 守卫，Web 端也显示
Settings.vue L495: function goDevTools() { if (!isNative()) return }  ← Web 端点后无反应
```

**现象**：Web 端看到入口但点击无反应，用户困惑。

### 问题 2：AppLogger 缓冲区几乎为空（核心问题）

**当前 AppLogger.log() 调用点仅 8 处**，全部在 GoProcessPlugin.kt 的插件管理操作中：
- `pickAndInstallPlugin` ×1
- `togglePluginEnabled` ×2 (成功/失败)
- `uninstallPlugin` ×2 (成功/失败)
- `executeComboLiteInstall` ×3

**关键路径完全缺失 AppLogger 调用**：

| 文件 | Log 调用方式 | 写入 AppLogger？ |
|------|------------|----------------|
| PlayerEntry.kt (32处) | `android.util.Log.i/w/e` | ❌ 全部只写系统 logcat |
| EncvHostActivity.kt (8处) | `android.util.Log.i/e/w` | ❌ 同上 |
| PluginLifecycleEngine.kt (9处) | `android.util.Log.i/w/e` | ❌ 同上 |
| MpvEmbedService.kt (多处) | `android.util.Log.i/w/e` | ❌ 同上 |
| GoProcessPlugin.openPlayer | 无日志！ | — |

**结果**：用户打开"查看日志"→ 只有安装/卸载操作的几条记录 → MPV 播放相关日志完全看不到。

### 问题 3：logcat 命令在 Android 14+ 返回空

```kotlin
// LogExporter.kt L26
Runtime.getRuntime().exec(arrayOf("logcat", "-d", "--pid=$pid", "-t", "5000"))
    .catch (_: Exception) {}  // ← 异常被静默吞掉！
if (!logcatFile.exists() || logcatFile.length() == 0L)
    logcatFile.writeText("(logcat empty)\n")  // ← 用户只看到这一行
```

Android 14+ 普通应用无 `READ_LOGS` 权限 → `logcat -d` 返回空 → catch 吞掉异常 → 写入 "(logcat empty)"。

### 问题 4：LogcatActivity 死代码 + CI Manifest 警告

**CI 警告**：
```
activity#com.hjq.logcat.LogcatActivity was tagged at AndroidManifest.xml:115
to replace another declaration but no other declaration present
```

**根因链**：
```
libs.versions.toml: logcat = "com.github.getActivity:Logcat:13.0"
build.gradle.kts: debugImplementation(libs.logcat)  ← 仅 debug 构建！
AndroidManifest.xml: <activity android:name="com.hjq.logcat.LogcatActivity"
                    tools:node="replace" />  ← 替换库 AAR 中的声明

Release 构建:
  → logcat 库不在 classpath → AAR 无 LogcatActivity 声明
  → tools:replace 找不到目标 → Warning（非 Error）
  → LogcatActivity 未注册 → 即使代码启动它也会 ActivityNotFoundException

Debug 构建:
  → 库存在 → replace 正常 → LogcatActivity 可用
  → 但没有任何代码启动它！→ 死代码
```

**影响**：
- Release 构建中 **LogcatActivity 完全不可用**（未注册）
- Debug 构建中 **LogcatActivity 注册了但从未被调用**
- CI 每次构建都输出 Warning，污染日志

---

## 修复方案

### Task 1: LogBridge — 统一日志桥接（解决 AppLogger 为空）

**核心思路**：新增 `LogBridge` 工具类，同时写入 `android.util.Log` 和 `AppLogger`。所有模块改用 `LogBridge` 替代直接 `Log.x` 调用。

#### SubTask 1.1: 新建 LogBridge.kt

```kotlin
package com.encvgo.app

object LogBridge {
    private const val TAG_DEFAULT = "ENCV-go"

    fun i(tag: String, msg: String) {
        android.util.Log.i(tag, msg)
        AppLogger.log("I", tag, msg)
    }
    fun w(tag: String, msg: String) {
        android.util.Log.w(tag, msg)
        AppLogger.log("W", tag, msg)
    }
    fun e(tag: String, msg: String, tr: Throwable? = null) {
        if (tr != null) android.util.Log.e(tag, msg, tr)
        else android.util.Log.e(tag, msg)
        AppLogger.log("E", tag, if (tr != null) "$msg\n${tr.stackTraceToString()}" else msg)
    }
    fun d(tag: String, msg: String) {
        android.util.Log.d(tag, msg)
        AppLogger.log("D", tag, msg)
    }

    // 兼容现有代码的便捷方法（tag 从调用者传入）
    fun i(msg: String) { i(TAG_DEFAULT, msg) }
    fun w(msg: String) { w(TAG_DEFAULT, msg) }
    fun e(msg: String) { e(TAG_DEFAULT, msg) }
}
```

#### SubTask 1.2: PlayerEntry.kt — Log 替换为 LogBridge（32 处）

```kotlin
// Before:
import android.util.Log
Log.i(TAG, "[ModeC-Activity] startMpvViaActivity...")

// After:
import com.encvgo.app.LogBridge
LogBridge.i(TAG, "[ModeC-Activity] startMpvViaActivity...")
```

#### SubTask 1.3: EncvHostActivity.kt — 同上（8 处）

#### SubTask 1.4: PluginLifecycleEngine.kt — 同上（9 处）

#### SubTask 1.5: MpvEmbedService.kt — 同上

#### SubTask 1.6: GoProcessPlugin.openPlayer — 补充缺失日志

```kotlin
@PluginMethod
fun openPlayer(call: PluginCall) {
    AppLogger.log("I", TAG, "openPlayer: mode=${call.getString("mode")} filePath=${call.getString("filePath")}")
    // ... existing code ...
}
```

### Task 2: LogExporter 改进 — logcat 降级策略（解决 Android 14+ 空）

```kotlin
val logcatFile = File(logDir, "logcat_${timestamp}.txt")
try {
    val pid = android.os.Process.myPid()
    val process = Runtime.getRuntime().exec(arrayOf(
        "logcat", "-d", "--pid=$pid", "-t", "5000", "-v", "threadtime"
    ))
    val exitCode = process.waitFor()
    process.inputStream.bufferedReader().use { reader ->
        logcatFile.outputStream().bufferedWriter().use { writer ->
            var line: String?
            var lineCount = 0
            while (reader.readLine().also { line = it } != null) {
                writer.write(line); writer.newLine()
                lineCount++
            }
            if (lineCount == 0 && exitCode != 0) {
                writer.write("(logcat returned empty, exitCode=$exitCode — likely no READ_LOGS permission on Android 14+)\n")
            }
        }
    }
} catch (e: Exception) {
    logcatFile.writeText("(logcat exec failed: ${e.javaClass.simpleName}: ${e.message})\n")
    AppLogger.log("W", "LogExporter", "logcat exec failed: ${e.message}")
}
```

关键改进：
1. `process.waitFor()` 获取退出码
2. 记录行数，为空时写入原因说明
3. 异常不再静默吞掉，写入文件 + AppLogger

### Task 3: DevTools 入口可见性修复

#### SubTask 3.1: Settings.vue — 添加 isNative 守卫

```vue
<!-- Before -->
<ion-item button @click="goDevTools" detail>

<!-- After -->
<ion-item v-if="isNative()" button @click="goDevTools" detail>
```

#### SubTask 3.2: DevToolsDetail.vue — openLogViewer 增强提示

当 AppLogger 缓冲区为空时，给用户明确提示：

```typescript
async function handleOpenLogViewer() {
  if (!isNative()) return
  try {
    await openLogViewer()
    showToast({ message: t('devtools.openLogHint'), duration: 2000, color: 'medium' })
  } catch {
    showToast({ message: t('devtools.openLogFailed'), duration: 2000, color: 'danger' })
  }
}
```

新增 i18n key: `devtools.openLogHint: '应用内日志已打开。如需完整 logcat 请使用「导出日志」功能。'`

### Task 4: openLogViewer 增强 — 显示更多信息

当前 `openLogViewer()` 只写 AppLogger 内容到文件。增强为包含时间戳和来源统计：

```kotlin
fun openViewer(context: Context): Boolean {
    return try {
        val logFile = File(context.cacheDir, "encv_logs_export/app_log_latest.txt")
        logFile.parentFile?.mkdirs()
        val logs = AppLogger.getLogs()
        val header = buildString {
            appendLine("=== ENCV App Log Viewer ===")
            appendLine("Timestamp: ${SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.US).format(Date())")
            appendLine("PID: ${android.os.Process.myPid()}")
            appendLine("Total entries: ${if (logs.isEmpty()) 0 else logs.lines().size}")
            appendLine("==========================\n")
        }
        logFile.writeText(header + (logs.ifEmpty { "(no app log entries yet — perform actions like plugin install/play video to generate logs)\n" }))
        // ... existing intent code ...
    }
}
```

### Task 5: LogcatActivity 死代码清理 + Manifest 警告消除

#### SubTask 5.1: 移除 AndroidManifest.xml 中 LogcatActivity 声明

该 Activity 从未被任何代码启动（全局搜索确认），且仅 debug 构建有库依赖。移除死声明：

```xml
<!-- 删除这段 -->
<activity
    android:name="com.hjq.logcat.LogcatActivity"
    android:configChanges="orientation|screenSize|keyboardHidden"
    android:launchMode="singleInstance"
    android:screenOrientation="portrait"
    android:theme="@style/Theme.AppCompat.Light.NoActionBar"
    tools:node="replace" />
```

#### SubTask 5.2: 评估 logcat 库是否需要保留

`com.github.getActivity:Logcat:13.0` 是一个 **logcat 可视化查看器 Activity**，提供 UI 界面浏览 logcat 输出。但：
- 当前项目从未启动 `LogcatActivity`
- 项目有自己的日志导出机制（`LogExporter.export()` → zip 分享）
- 如果未来需要内嵌 logcat 查看器，可以通过 Intent 启动库的 Activity（需重新加回 Manifest）

**建议**：暂时保留 `debugImplementation(libs.logcat)` 依赖（不占用 release 包体积），但移除 Manifest 中的死声明。等真正需要时再加回 Activity 声明 + 启动逻辑。

---

## 改动文件清单

| 文件 | Task | 改动 |
|------|------|------|
| `LogBridge.kt` (**新建**) | T1 | 统一日志桥接（Log + AppLogger 双写） |
| [PlayerEntry.kt](app/encv-mobile/android/app/src/main/java/com/encvgo/app/PlayerEntry.kt) | T1.2 | `Log` → `LogBridge`（32 处） |
| [EncvHostActivity.kt](app/encv-mobile/android/app/src/main/java/com/encvgo/app/EncvHostActivity.kt) | T1.3 | `Log` → `LogBridge`（8 处） |
| [PluginLifecycleEngine.kt](app/encv-mobile/android/combolite-host/src/main/java/com/encvgo/combolite/engine/PluginLifecycleEngine.kt) | T1.4 | `Log` → `LogBridge`（9 处） |
| [MpvEmbedService.kt](app/encv-mobile/android/app/src/main/java/com/encvgo/app/MpvEmbedService.kt) | T1.5 | `Log` → `LogBridge` |
| [GoProcessPlugin.kt](app/encv-mobile/android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt) | T1.6 | openPlayer 补充日志 |
| [LogExporter.kt](app/encv-mobile/android/app/src/main/java/com/encvgo/app/LogExporter.kt) | T2 | logcat 降级策略 + 错误信息保留 |
| [Settings.vue](app/encv-mobile/src/views/Settings.vue) | T3.1 | DevTools 入口加 `v-if="isNative()"` |
| [DevToolsDetail.vue](app/encv-mobile/src/views/DevToolsDetail.vue) | T3.2-4 | openLogViewer 提示 + i18n |
| [useI18n.ts](app/encv-mobile/src/composables/useI18n.ts) | T3.2 | 新增 `devtools.openLogHint` |
| [AndroidManifest.xml](app/encv-mobile/android/app/src/main/AndroidManifest.xml) | T5.1 | 移除 LogcatActivity 死声明 |

## 铁律合规检查

| 铁律 | 合规方式 |
|------|---------|
| **饱和调试** | 所有 Log 调用通过 LogBridge 双写 → app_log 缓冲区 + 系统 logcat 都有数据 |
| **§1.4 应用内日志缓冲** | LogBridge 是 §1.4 的标准化实现，替代散落的 Log.x 调用 |

## 验证清单

- [ ] Settings 页面 Web 端不显示 DevTools 入口，Native 端正常显示
- [ ] 安装插件 → 打开日志查看器 → 能看到安装流程日志
- [ ] 用 MPV 播放视频 → 打开日志查看器 → 能看到 `[ModeC-Activity]` / `EncvHostActivity` 日志
- [ ] 导出日志 zip → logcat 文件非空 或 明确标注 "no READ_LOGS permission" 原因
- [ ] **CI 构建 Manifest 不再输出 LogcatActivity Warning**
- [ ] vue-tsc + vite build 通过
