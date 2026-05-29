# 修复：MPV 播放 "Unable to get stream URL"

## 根因

`PlayerEntry.getBackendBaseUrl()` 硬编码 `"http://127.0.0.1:8899"`，实际后端端口是 `2025`。

`resolveStreamUrl` 构造的 HTTP 流 URL 指向 8899 → HEAD 请求失败 → 返回空字符串 → "Unable to get stream URL"。

## 修复

### Step 1：修复 `PlayerEntry.getBackendBaseUrl()` 端口

**文件**：`android/app/src/main/java/com/encvgo/app/PlayerEntry.kt`

```kotlin
private fun getBackendBaseUrl(context: Context): String {
    val port = EncvGoService.lastKnownPort
    return if (port > 0) "http://127.0.0.1:$port" else "http://127.0.0.1:2025"
}
```

### Step 2：改进 `resolveStreamUrl` 逻辑 + 诊断日志

**文件**：`plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvPlayerScreen.kt`

- 本地绝对路径且文件存在 → 直接播放（快速路径）
- 否则走 HTTP 流（和 ArtPlayer 一视同仁，加密文件由后端 `/stream` 端点处理）
- HEAD 超时 3s，添加诊断日志

### Step 3：`startPlayback` 添加诊断日志

### Step 4：验证构建
