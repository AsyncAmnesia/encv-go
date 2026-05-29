# 修复：MPV 播放 "Unable to get stream URL"

## 根因

`PlayerEntry.getBackendBaseUrl()` 硬编码 `"http://127.0.0.1:8899"`，实际后端端口是 `2025`。

## 修复

### Step 1：修复 `PlayerEntry.getBackendBaseUrl()` 端口

**文件**：`android/app/src/main/java/com/encvgo/app/PlayerEntry.kt`

动态读取 `EncvGoService.lastKnownPort`，回退 `2025`。

### Step 2：重写 `resolveStreamUrl` — 只构造 HTTP 流 URL

**文件**：`plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvPlayerScreen.kt`

和前端 `getFileStreamUrl` 一视同仁，只做一件事：`$backendUrl/stream?path=$encodedPath`。删除所有本地路径识别逻辑。

### Step 3：`startPlayback` 添加诊断日志

### Step 4：验证构建
