# 修复 CI 构建失败：PlayerOverlayManager 类缺失

## 问题

CI 构建日志显示 **Kotlin 编译错误**：

```
e: GoProcessPlugin.kt:194:13 Unresolved reference 'PlayerOverlayManager'
e: GoProcessPlugin.kt:208:13 Unresolved reference 'PlayerOverlayManager'
e: MainActivity.kt:64:9   Unresolved reference 'PlayerOverlayManager'
e: MainActivity.kt:73:13 Unresolved reference 'PlayerOverlayManager'
e: MainActivity.kt:74:13 Unresolved reference 'PlayerOverlayManager'

FAILURE: Build failed with an exception
Execution failed for task ':app:compileReleaseKotlin'.
```

**FFmpeg 构建已通过**（`-Wl,-u` + version script 修复生效）：
```
✅ ffprobe_run    ✅ ffprobe_reset    📊 2 text symbols, 4.1M
✅ ffmpeg_run     ✅ ffmpeg_reset     📊 2 text symbols, 4.4M
```

## 根因

`PlayerOverlayManager` 类被 5 处引用但**从未创建**。它是一个 singleton 管理器，负责在 `MainActivity` 上以 overlay 方式嵌入播放器（而非启动新 Activity）。

### 调用点分析

| 文件 | 行为 | 调用 | 用途 |
|------|------|------|------|
| [GoProcessPlugin.kt:194](file:///workspace/app/encv-mobile/android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt#L194) | `openPlayer()` | `showOverlay(activity, filePath, name, mimeType, isExternal)` | 在主 Activity 上内嵌显示播放器 |
| [GoProcessPlugin.kt:208](file:///workspace/app/encv-mobile/android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt#L208) | `closePlayer()` | `hideOverlay()` | 关闭播放器 overlay |
| [MainActivity.kt:64](file:///workspace/app/encv-mobile/android/app/src/main/java/com/encvgo/app/MainActivity.kt#L64) | `onDestroy()` | `hideOverlay()` | Activity 销毁时清理 |
| [MainActivity.kt:73](file:///workspace/app/encv-mobile/android/app/src/main/java/com/encvgo/app/MainActivity.kt#L73) | `onBackPressed()` | `isOverlayShowing()` | 拦截返回键 |
| [MainActivity.kt:74](file:///workspace/app/encv-go/mobile/android/app/src/main/java/com/encvgo/app/MainActivity.kt#L74) | `onBackPressed()` | `hideOverlay()` | 返回键关闭 overlay |

### Player 系统架构（已有）

```
PlayerEntry (入口)
├── mpv-plugin → MpvPlayerActivity (独立 Activity)
├── external → 系统外部播放器
└── artplayer → PlayerActivityCapacitor (独立 Activity)

PlayerOverlayManager (新增，内嵌模式)
└── 在 MainActivity 上以 Fragment/View 方式内嵌 ArtPlayer WebView
```

## 修复方案

创建 `PlayerOverlayManager.kt`，实现 singleton 管理 overlay 的显示/隐藏。

### 新建文件

**路径**：`app/encv-mobile/android/app/src/main/java/com/encvgo/app/PlayerOverlayManager.kt`

**设计要点**：

1. **Singleton 模式** — `getInstance()` 
2. **showOverlay()** — 使用 `PlayerEntry.play()` 的逻辑但在当前 Activity 上以 Fragment/Dialog 方式嵌入，或简化为启动 `PlayerActivityCapacitor` 并用 `startActivityForResult` 管理
3. **hideOverlay()** — 关闭 overlay
4. **isOverlayShowing()** — 返回当前状态
5. **与 MainActivity 生命周期联动** — 已有 `onDestroy`/`onBackPressed` 钩子

**最小可行实现**：由于已有完整的 `PlayerEntry` → `PlayerActivityCapacitor` 路径，`PlayerOverlayManager` 可以封装为一个轻量级管理器——内部使用 Fragment 容器加载 player WebView，或退化为 `startActivityForResult` 模式。

### 实现策略（推荐）

考虑到项目已有 `PlayerActivityCapacitor`（BridgeActivity + WebView），最简方案是：

```kotlin
class PlayerOverlayManager private constructor() {
    var isShowing = false
        private set
    
    fun showOverlay(activity: Activity, filePath: String, name: String, mimeType: String, isExternal: Boolean) {
        // 通过 PlayerEntry 启动内嵌播放器
        // 使用 Fragment 或 DialogFragment 在 activity 上叠加
        isShowing = true
    }
    
    fun hideOverlay() {
        // 移除 overlay
        isShowing = false
    }
    
    fun isOverlayShowing(): Boolean = isShowing
    
    companion object {
        @Volatile private var instance: PlayerOverlayManager? = null
        fun getInstance(): PlayerOverlayManager =
            instance ?: synchronized(this) { instance ?: PlayerOverlayManager().also { instance = it } }
    }
}
```

具体实现细节需参考项目中 Fragment/WebView 复用的现有模式。

## 修改清单

| 操作 | 文件 |
|------|------|
| **新建** | `android/app/src/main/java/com/encvgo/app/PlayerOverlayManager.kt` |

仅新建 1 个文件，不修改任何现有文件。

## 清理

完成后删除：
- `/workspace/job_logs/` 目录
- `/workspace/job_logs.zip`
- `/workspace/.trae/documents/job_logs.zip`
