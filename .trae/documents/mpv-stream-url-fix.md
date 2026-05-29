# 修复：MPV 播放 "Unable to get stream URL"

## 根因分析

### 错误信息
```
播放失败 / Playback Failed
未知错误 / Unknown Error
Unable to get stream URL
```

### 根因：`PlayerEntry.getBackendBaseUrl()` 硬编码端口 8899，实际后端端口是 2025

**完整链路：**

```
前端 openPlayer(file.path="/media/video.mp4", ...)
  → GoProcessPlugin.openPlayer() → PlayerEntry.buildMpvIntent()
    → getBackendBaseUrl() 返回 "http://127.0.0.1:8899"  ← 硬编码！
    → Intent extra: backend_url = "http://127.0.0.1:8899"
  → MpvPlayerActivity.onCreate()
    → hostIntent.getStringExtra("backend_url") = "http://127.0.0.1:8899"
  → MpvPlayerScreen → startPlayback() → resolveStreamUrl()
    → backendUrl = "http://127.0.0.1:8899"
    → 构造 URL: "http://127.0.0.1:8899/stream?path=%2Fmedia%2Fvideo.mp4"
    → HEAD 请求 → 连接失败（8899 端口无服务）
    → 返回空字符串 → "Unable to get stream URL"
```

**证据：**

| 组件 | 端口 | 来源 |
|------|------|------|
| Go 后端 `EncvGoService.DEFAULT_PORT` | **2025** | `EncvGoService.kt:31` |
| 前端 `DEFAULT_API_BASE_URL` | **2025** | `src/api/encv.ts:2` |
| `PlayerEntry.getBackendBaseUrl()` | **8899** ❌ | `PlayerEntry.kt:272` |

### 次要问题：`resolveStreamUrl` 逻辑不够健壮

1. **HEAD 请求失败时静默返回空字符串** — 无法区分"后端未启动"和"文件不存在"
2. **`isExternal` 始终为 `false`** — `GoProcessPlugin.openPlayer()` 硬编码 `isExternal = false`，导致即使文件是外部文件也走 HTTP 流路径
3. **没有本地路径回退** — 当后端未启动时，如果 `filePath` 对应的本地文件存在，应该直接播放本地文件

---

## 修复步骤

### Step 1：修复 `PlayerEntry.getBackendBaseUrl()` 端口

**文件**：`android/app/src/main/java/com/encvgo/app/PlayerEntry.kt`

将硬编码的 `8899` 改为动态读取 `EncvGoService.lastKnownPort`，回退到默认端口 `2025`：

```kotlin
private fun getBackendBaseUrl(context: Context): String {
    val port = EncvGoService.lastKnownPort
    return if (port > 0) {
        "http://127.0.0.1:$port"
    } else {
        "http://127.0.0.1:2025"
    }
}
```

### Step 2：修复 `resolveStreamUrl` 添加本地路径回退

**文件**：`plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvPlayerScreen.kt`

当 HTTP 流不可用时，尝试将后端 API 路径映射为本地文件路径直接播放：

```kotlin
internal suspend fun resolveStreamUrl(filePath: String, isExternal: Boolean, backendUrl: String): String {
    return try {
        // 1. 如果是本地绝对路径且文件存在，直接返回
        if (filePath.startsWith("/") && java.io.File(filePath).exists()) {
            return filePath
        }

        // 2. 如果 backendUrl 可用，构造 HTTP 流 URL
        if (backendUrl.isNotEmpty()) {
            val encodedPath = java.net.URLEncoder.encode(filePath, "UTF-8")
            val url = if (isExternal) {
                "$backendUrl/api/stream/external?path=$encodedPath"
            } else {
                "$backendUrl/stream?path=$encodedPath"
            }

            val conn = java.net.URL(url).openConnection() as java.net.HttpURLConnection
            conn.requestMethod = "HEAD"
            conn.connectTimeout = 3000
            conn.readTimeout = 3000
            val responseCode = conn.responseCode
            conn.disconnect()

            if (responseCode in 200..299) return url
        }

        // 3. HTTP 流不可用时，尝试映射为本地路径
        //    后端 API 路径如 "/media/video.mp4" → 本地路径 "/storage/emulated/0/media/video.mp4"
        if (filePath.startsWith("/")) {
            val localPath = "/storage/emulated/0$filePath"
            if (java.io.File(localPath).exists()) {
                return localPath
            }
        }

        ""
    } catch (e: Exception) {
        android.util.Log.w("MpvPlayer", "resolveStreamUrl failed: ${e.message}")
        // 异常时也尝试本地路径回退
        if (filePath.startsWith("/")) {
            val localPath = "/storage/emulated/0$filePath"
            if (java.io.File(localPath).exists()) {
                return localPath
            }
        }
        ""
    }
}
```

### Step 3：同步修复 `MpvAudioPlayerScreen` 中的相同问题

**文件**：`plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvAudioPlayerScreen.kt`

`MpvAudioPlayerScreen` 也使用 `resolveStreamUrl`，但 `backendUrl` 的获取方式相同：
```kotlin
val backendUrl = (context as? android.app.Activity)?.intent?.getStringExtra("backend_url") ?: ""
```

由于 `resolveStreamUrl` 是 `internal suspend fun`（同一模块），Step 2 的修改会自动生效。

### Step 4：添加诊断日志

在 `resolveStreamUrl` 关键路径添加日志，便于未来调试：
- 记录输入参数 `filePath`, `isExternal`, `backendUrl`
- 记录 HTTP 流 URL 和 HEAD 请求结果
- 记录本地路径回退尝试

---

## 影响范围

| 修改 | 文件 | 影响 |
|------|------|------|
| 端口修复 | `PlayerEntry.kt` | 所有 MPV 模式的 HTTP 流 URL |
| resolveStreamUrl 改进 | `MpvPlayerScreen.kt` | 视频播放 URL 解析 |
| 自动生效 | `MpvAudioPlayerScreen.kt` | 音频播放 URL 解析（同模块 internal fun） |

## 风险评估

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| 后端端口不是 2025 也不是 lastKnownPort | 低 | HTTP 流失败 | lastKnownPort 动态读取 + 本地路径回退 |
| 本地路径映射不正确 | 中 | 文件不存在 | File.exists() 检查 + 诊断日志 |
| 后端配置了非默认根目录 | 低 | 本地路径回退失败 | HTTP 流仍可用（端口修复后） |
