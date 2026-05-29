# 修复 Logcat 日志集成 — 入口可见性 + 日志为空 + LogcatActivity 正确启动

## Bug 诊断

### 问题 1：DevTools 入口在 Web 端无效

Settings.vue DevTools 项无 `v-if="isNative()"` 守卫，Web 端点了无反应。

### 问题 2：AppLogger 缓冲区几乎为空（核心问题）

`AppLogger.log()` 仅在 GoProcessPlugin 的 8 处调用。PlayerEntry(32处)、EncvHostActivity(8处)、PluginLifecycleEngine(9处) 全部只用 `android.util.Log` 不写 AppLogger。

### 问题 3：logcat 命令在 Android 14+ 返回空

Android 14+ 无 READ_LOGS 权限 → `logcat -d --pid=$pid` 返回空 → 异常被静默 catch。

### 问题 4（最关键）：LogcatActivity 从未被启动！

**当前代码现状**：

| 组件 | 状态 | 说明 |
|------|------|------|
| `com.github.getActivity:Logcat:13.0` | `debugImplementation` | 仅 debug 构建 |
| AndroidManifest.xml LogcatActivity 声明 | 有，但用 `tools:node="replace"` | release 构建找不到目标 → CI Warning |
| `GoProcessPlugin.openLogViewer()` | ❌ 不启动 LogcatActivity | 只写文本文件 + ACTION_VIEW |
| 任何代码启动 LogcatActivity | ❌ **完全没有** | 全局搜索确认 |

**用户期望 vs 实际行为**：

```
用户期望:
  点"查看日志" → LogcatActivity 启动 → 实时浏览系统 logcat ✅
  
实际行为:
  点"查看日志" → openLogViewer() 写文本文件 → ACTION_VIEW 打开文本文件
  → 文件内容 "(no app log entries)" → 啥也看不到 ❌
```

---

## 修复方案

### Task 1: LogBridge — 统一日志桥接（解决 AppLogger 为空）

新建 `LogBridge.kt`，同时写入 `android.util.Log` 和 `AppLogger`。所有模块改用 `LogBridge` 替代直接 `Log.x` 调用。

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
        if (tr != null) android.util.Log.e(tag, msg, tr) else android.util.Log.e(tag, msg)
        AppLogger.log("E", tag, if (tr != null) "$msg\n${tr.stackTraceToString()}" else msg)
    }
    fun d(tag: String, msg: String) {
        android.util.Log.d(tag, msg)
        AppLogger.log("D", tag, msg)
    }

    fun i(msg: String) { i(TAG_DEFAULT, msg) }
    fun w(msg: String) { w(TAG_DEFAULT, msg) }
    fun e(msg: String) { e(TAG_DEFAULT, msg) }
}
```

#### SubTask 1.2-1.6: 替换所有模块 Log 调用

| 文件 | 改动 |
|------|------|
| PlayerEntry.kt | `import android.util.Log` → `import com.encvgo.app.LogBridge`; 所有 `Log.x` → `LogBridge.x` (32 处) |
| EncvHostActivity.kt | 同上 (8 处) |
| PluginLifecycleEngine.kt | 同上 (9 处) |
| MpvEmbedService.kt | 同上 |
| GoProcessPlugin.openPlayer | 补充缺失日志 |

### Task 2: 正确集成 LogcatActivity（核心修复）

#### SubTask 2.1: 将 logcat 库改为全构建类型可用

```toml
# libs.versions.toml — 保持不变
logcat = "com.github.getActivity:Logcat:13.0"

# build.gradle.kts
# Before: debugImplementation(libs.logcat)  ← 仅 debug
# After:  implementation(libs.logcat)         ← 所有构建类型
```

#### SubTask 2.2: 修复 AndroidManifest.xml LogcatActivity 声明

```xml
<!-- Before: tools:node="replace" 在 release 找不到目标 -->
<activity
    android:name="com.hjq.logcat.LogcatActivity"
    tools:node="replace" />

<!-- After: 标准声明，让库 AAR 的 merge 策略处理 -->
<activity
    android:name="com.hjq.logcat.LogcatActivity"
    android:exported="false"
    android:theme="@style/Theme.AppCompat.Light.NoActionBar" />
```

> 注：如果 AAR 内已有该 Activity 声明且用了 `tools:merge`，则此处可能需要 `tools:replace` 或直接删除（让 AAR 自带声明生效）。需根据实际 AAR 内容调整。

#### SubTask 2.3: 新增 `launchLogcatActivity()` 方法

```kotlin
// GoProcessPlugin.kt — 新增方法
@PluginMethod
fun launchLogcatActivity(call: PluginCall) {
    try {
        val intent = Intent().apply {
            setClassName(context.packageName, "com.hjq.logcat.LogcatActivity")
            if (context !is android.app.Activity) addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        context.startActivity(intent)
        call.resolve(JSObject().apply { put("success", true); put("launched", true) })
        AppLogger.log("I", TAG, "launchLogcatActivity: started")
    } catch (e: Exception) {
        AppLogger.log("E", TAG, "launchLogcatActivity failed: ${e.message}")
        call.resolve(JSObject().apply {
            put("success", false)
            put("error", "无法启动 LogcatActivity")
            put("errorDetail", e.message ?: "Unknown")
        })
    }
}
```

#### SubTask 2.4: GoProcess.ts 新增前端函数

```typescript
export async function launchLogcatActivity(): Promise<{ success: boolean; error?: string }> {
  try {
    await GoProcess.launchLogcatActivity()
    return { success: true }
  } catch (e) {
    return { success: false, error: String(e) }
  }
}
```

#### SubTask 2.5: web.ts 类型定义和 stub

```typescript
// web.ts interface
launchLogcatActivity(): Promise<{ success: boolean; error?: string }>

// web.ts stub
async launchLogcatActivity(): Promise<{ success: boolean; error?: string }> {
  return { success: false, error: 'Native only' }
}
```

#### SubTask 2.6: DevToolsDetail.vue "查看日志"按钮改为启动 LogcatActivity

```vue
<!-- Before: openLogViewer (打开文本文件，没用) -->
<ion-button @click="handleOpenLogViewer" ... >{{ t('devtools.openLogViewer') }}</ion-button>

<!-- After: launchLogcatActivity (启动 Logcat 库的可视化查看器) -->
<ion-button @click="handleLaunchLogcat" ... >{{ t('devtools.viewLogcat') }}</ion-button>
```

```typescript
async function handleLaunchLogcat() {
  if (!isNative()) return
  const result = await launchLogcatActivity()
  if (!result.success) showToast({ message: result.error || '启动失败', color: 'danger' })
}
```

新增 i18n key: `devtools.viewLogcat: '查看 Logcat' / 'View Logcat'`

保留原有 `openLogViewer` 作为备选入口（查看应用内缓冲日志），但改为次要位置或合并到同一区域。

### Task 3: LogExporter 改进 — logcat 降级策略

同原计划：process.waitFor() + 行数记录 + 异常不再静默吞掉。

### Task 4: DevTools 入口可见性修复

Settings.vue DevTools 入口加 `v-if="isNative()"`。

---

## 改动文件清单

| 文件 | Task | 改动 |
|------|------|------|
| `LogBridge.kt` (**新建**) | T1 | 统一日志桥接（Log + AppLogger 双写） |
| [PlayerEntry.kt](app/encv-mobile/android/app/src/main/java/com/encvgo/app/PlayerEntry.kt) | T1.2 | `Log` → `LogBridge`（32 处） |
| [EncvHostActivity.kt](app/encv-mobile/android/app/src/main/java/com/encvgo/app/EncvHostActivity.kt) | T1.3 | `Log` → `LogBridge`（8 处） |
| [PluginLifecycleEngine.kt](...) | T1.4 | `Log` → `LogBridge`（9 处） |
| [MpvEmbedService.kt](...) | T1.5 | `Log` → `LogBridge` |
| [GoProcessPlugin.kt](...) | T1.6+T2.3 | openPlayer 补充日志 + 新增 `launchLogcatActivity()` |
| [build.gradle.kts](...) | T2.1 | `debugImplementation` → `implementation` (libs.logcat) |
| [AndroidManifest.xml](...) | T2.2 | 修复 LogcatActivity 声明 |
| [GoProcess.ts](...) | T2.4 | 新增 `launchLogcatActivity()` |
| [web.ts](...) | T2.5 | 类型 + stub |
| [DevToolsDetail.vue](...) | T2.6 | 主按钮改为启动 LogcatActivity |
| [useI18n.ts](...) | T2.6 | 新增 `devtools.viewLogcat` |
| [Settings.vue](...) | T3 | DevTools 入口加 `v-if="isNative()"` |
| [LogExporter.kt](...) | T3 | logcat 降级策略 |

## 验证清单

- [ ] Settings Native 端显示 DevTools 入口，Web 端不显示
- [ ] 点"查看 Logcat" → **LogcatActivity 启动** → 能看到实时系统日志（含 MPV 相关）
- [ ] 用 MPV 播放视频 → LogcatActivity 中能看到 `[ModeC-Activity]` / `EncvHostActivity` 日志
- [ ] **CI 构建 Manifest 不再输出 LogcatActivity Warning**
- [ ] vue-tsc + vite build 通过
