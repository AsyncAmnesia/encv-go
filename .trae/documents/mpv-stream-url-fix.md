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

### 加密视频的播放逻辑

**当前流程**：加密文件（`isEncrypted=true`）在 `Files.vue` 中 `getFileCategory` 返回 `'encrypted'`，不进入 `playMedia()`，而是跳转到预览页面。预览页面检测到加密视频类型后，跳转到 ArtPlayer（WebView 播放器），使用 `getFileStreamUrl(path)` 即 `http://127.0.0.1:2025/stream?path=...`。

**后端 `/stream` 端点**：对加密文件自动执行解密并流式传输。本地路径方式无法处理加密文件。

**结论**：MPV 插件当前只处理非加密视频/音频，必须通过 HTTP 流播放。本地路径回退方案**不能盲目使用**——对于加密文件（如果未来 MPV 也需要支持），必须走 HTTP 流。

---

## 修复步骤

### Step 1：修复 `PlayerEntry.getBackendBaseUrl()` 端口（核心修复）

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

### Step 2：改进 `resolveStreamUrl` 逻辑

**文件**：`plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvPlayerScreen.kt`

改进点：
1. **本地绝对路径优先**：如果 `filePath` 是本地绝对路径且文件存在，直接返回（非加密文件的快速路径）
2. **HTTP 流必须可用**：对于后端 API 路径（如 `/media/video.mp4`），必须通过 HTTP 流播放（后端处理加密解密等逻辑）
3. **HEAD 请求超时缩短**：3 秒足够，5 秒太长
4. **诊断日志**：记录关键决策点

```kotlin
internal suspend fun resolveStreamUrl(filePath: String, isExternal: Boolean, backendUrl: String): String {
    android.util.Log.d("MpvPlayer", "resolveStreamUrl: filePath=$filePath isExternal=$isExternal backendUrl=$backendUrl")

    // 1. 本地绝对路径且文件存在 → 直接播放（非加密文件快速路径）
    if (filePath.startsWith("/") && java.io.File(filePath).exists()) {
        android.util.Log.i("MpvPlayer", "resolveStreamUrl: local file exists → $filePath")
        return filePath
    }

    // 2. HTTP 流（必须通过后端，处理加密文件、权限等）
    if (backendUrl.isNotEmpty()) {
        try {
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

            if (responseCode in 200..299) {
                android.util.Log.i("MpvPlayer", "resolveStreamUrl: HTTP stream OK → $url")
                return url
            }
            android.util.Log.w("MpvPlayer", "resolveStreamUrl: HTTP stream returned $responseCode for $url")
        } catch (e: Exception) {
            android.util.Log.w("MpvPlayer", "resolveStreamUrl: HTTP stream failed: ${e.message}")
        }
    } else {
        android.util.Log.w("MpvPlayer", "resolveStreamUrl: backendUrl is empty, cannot construct HTTP stream URL")
    }

    // 3. 所有方式都失败
    return ""
}
```

**关键设计决策**：
- **不添加本地路径映射回退**（如 `/storage/emulated/0$filePath`）—— 因为 `filePath` 是后端 API 路径，不是本地相对路径。后端可能配置了不同的根目录，而且加密文件必须走 HTTP 流。盲目映射本地路径会绕过后端的解密逻辑。
- **如果后端未启动**：`backendUrl` 为空或 HEAD 请求失败 → 返回空字符串 → 显示错误。用户需要确保后端已启动。

### Step 3：同步修复 `MpvAudioPlayerScreen` 中的相同问题

`MpvAudioPlayerScreen` 也使用 `resolveStreamUrl`（`internal suspend fun`，同一模块），Step 2 的修改自动生效。

但 `MpvAudioPlayerScreen` 中的 `backendUrl` 获取方式需要确认一致：
```kotlin
val backendUrl = (context as? android.app.Activity)?.intent?.getStringExtra("backend_url") ?: ""
```
与 `MpvPlayerScreen` 相同，无需额外修改。

### Step 4：添加诊断日志到 `startPlayback`

在 `startPlayback` 函数中添加关键参数日志，便于未来调试：

```kotlin
internal suspend fun startPlayback(...) {
    android.util.Log.i("MpvPlayer", "startPlayback: filePath=$filePath isExternal=$isExternal backendUrl=$backendUrl")
    // ... 现有逻辑
}
```

---

## 影响范围

| 修改 | 文件 | 影响 |
|------|------|------|
| 端口修复 | `PlayerEntry.kt` | 所有 MPV 模式的 backend_url |
| resolveStreamUrl 改进 | `MpvPlayerScreen.kt` | 视频播放 URL 解析 |
| 自动生效 | `MpvAudioPlayerScreen.kt` | 音频播放 URL 解析（同模块 internal fun） |

## 风险评估

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| 后端端口不是 2025 也不是 lastKnownPort | 低 | HTTP 流失败 | lastKnownPort 动态读取 + 诊断日志 |
| 后端未启动时无法播放 | 中 | 用户看到错误 | 这是正确行为——非加密文件可通过本地路径播放，加密文件必须后端在线 |
| 本地文件存在但无法播放（权限问题） | 低 | 播放失败 | MPV 有文件权限处理 |
