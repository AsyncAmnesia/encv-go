# CI构建报错修复 + MPV播放器UI修复计划

## 问题概述

### 问题1：CI构建报错
- 需要解压 `job_logs.zip` 查看具体错误日志并修复

### 问题2：MPV控件UI丢失 + external播放器模式理解错误
**关键Bug发现**：Settings.vue 和 Files.vue 之间播放器模式值不匹配！

| 位置 | 视频播放器可选值 | 存储到localStorage的值 |
|------|------------------|----------------------|
| Settings.vue (UI选项) | `artplayer`, `mpv-plugin`, `external` | `mpv-plugin` |
| Files.vue (getPlayMode检查) | 检查 `artplayer`, `mpv`, `external` | **只认`mpv`，不认`mpv-plugin`！** |

**结果**：用户在设置中选择"MPV插件扩展"后，存储值为 `mpv-plugin`，但 `getPlayMode()` 无法匹配，fallback到默认的 `artplayer`。**MPV插件永远无法被选中启动！**

另外，`external` 是用户自行选择的外部播放器模式（通过Intent调用用户设备上安装的其他播放器），不是系统播放器。

---

## 修复步骤

### Step 1: 解压CI构建日志
- 删除已有的旧日志目录（如果有）
- 解压 `/workspace/job_logs.zip` 到临时目录
- 分析具体报错内容（Kotlin编译/前端构建/Gradle/打包等）

### Step 2: 根据日志修复CI构建错误
- 根据解压后的日志定位具体失败步骤和错误信息
- 修复对应代码问题
- 常见可能：Kotlin编译错误、前端build错误、依赖问题、NDK构建问题等

### Step 3: 修复播放器模式值不匹配 Bug（核心！）
**文件**: `/workspace/app/encv-mobile/src/views/Files.vue`

修改 `getPlayMode()` 函数，使其兼容 Settings.vue 存储的值：
```typescript
function getPlayMode(mediaType: 'video' | 'audio'): PlayMode {
  const key = mediaType === 'video' ? 'encv_player_video' : 'encv_player_audio'
  const stored = localStorage.getItem(key)
  // 兼容 Settings.vue 存储的值：视频端用 'mpv-plugin'，音频端用 'mpv'
  if (stored === 'artplayer' || stored === 'mpv' || stored === 'mpv-plugin' || stored === 'external') return stored
  return mediaType === 'video' ? 'artplayer' : 'mpv'
}
```

同时修改 `playMedia()` 函数中的 switch 分支，将 `mpv-plugin` 映射到 mpv 播放逻辑：
```typescript
switch (mode) {
    case 'artplayer':
      router.push({ path: '/player', query: { path: file.path, name: file.name } })
      break
    case 'mpv':
    case 'mpv-plugin':  // ← 新增：兼容 Settings.vue 的值
      if (isNative()) {
        openPlayer(file.path, file.name, mimeType)
      } else {
        router.push({ path: '/player', query: { path: file.path, name: file.name } })
      }
      break
    case 'external':
      if (isNative()) {
        const url = getExternalStreamUrl(file.path)
        openExternal(url, mimeType)
      } else {
        router.push({ path: '/player', query: { path: file.path, name: file.name } })
      }
      break
  }
```

### Step 4: 验证MPV原生控件UI完整性
确认以下文件完整且正常：
- [MpvControls.kt](app/encv-mobile/plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvControls.kt) — 完整的Compose UI控件（播放/暂停/进度条/锁定/全屏/倍速/错误处理等）
- [MpvPlayerScreen.kt](app/encv-mobile/plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvPlayerScreen.kt) — 播放器屏幕主逻辑
- [MpvPlayerActivity.kt](app/encv-mobile/plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvPlayerActivity.kt) — Activity入口
- [MpvEngine.kt](app/encv-mobile/plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvEngine.kt) — MPV引擎封装
- [MpvProgressBar.kt](app/encv-mobile/plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvProgressBar.kt) — 进度条组件
- [MpvPlayerState.kt](app/encv-mobile/plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvPlayerState.kt) — 状态定义

### Step 5: 前端构建验证
运行完整验证命令：
```bash
cd /workspace/app/encv-mobile && npx vue-tsc --noEmit && npm run build
```

### Step 6: 清理
- 删除解压的日志目录
- 删除 `job_logs.zip`
- 删除 `.trae/documents/job_logs.zip`

---

## 文件变更清单
1. **Files.vue** — 修复 `getPlayMode()` + `playMedia()` 的 `mpv-plugin` 值兼容
2. **CI相关文件** — 根据日志修复具体构建错误（待定）
