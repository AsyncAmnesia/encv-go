# ENCV Mobile PlayerActivity 修复总结与待办

## ✅ 已解决：白屏问题

### 根因
Ionic 的 `<ion-router-outlet>` + Vue Router 在 PlayerActivity（同一进程中的第二个 BridgeActivity 实例）中无法正常工作。两个 Ionic App 实例共享全局状态导致路由系统冲突，`<ion-router-outlet>` 不渲染任何路由组件。

### 修复
[PlayerApp.vue](src/PlayerApp.vue) 移除 `<ion-router-outlet>`，改为直接用 `<Suspense>` 渲染 `StandalonePlayer` 组件，绕过 Ionic Router。

### 验证日志（logcat 确认）
```
[PLAYER-INIT] App mounted successfully           ✅ Vue挂载
[StandalonePlayer] <script setup> evaluating...  ✅ 组件加载
[StandalonePlayer] onMounted fired               ✅ 生命周期
[StandalonePlayer] ArtPlayer initialized          ✅ ArtPlayer 5.4.0
```

---

## 待修复问题

### 问题 1：应用内打开视频无法播放（路径转换）

**现象**：从 ENC 应用内 Files 页面点击视频 → PlayerActivity 打开 → ArtPlayer 报错循环重试

**根因链路**：
```
Files.vue → openInPlayer("/123云盘/xxx.mp4", ...)
  → PlayerActivity.intentFilePath = "/123云盘/xxx.mp4"
  → StandalonePlayer.streamUrl = getExternalStreamUrl("/123云盘/xxx.mp4")
  → GET /api/stream/external?path=%2F123%E4%BA%91%E7%9B%98...
  → 后端 os.Stat("/123云盘/xxx.mp4") → 文件不存在 → 404
  → net::ERR_BLOCKED_BY_ORB (-1) → ArtPlayer playback error (循环)
```

**对比**：第三方应用打开时，content URI 解析出的路径是 `/storage/emulated/0/123云盘/xxx.mp4`（完整绝对路径）→ 正常播放

**原因**：后端文件列表 API 返回的路径是相对于 serve root（`/storage/emulated/0`）的路径，如 `/123云盘/xxx.mp4`。这个路径在 Android 文件系统中不存在，真实路径是 `/storage/emulated/0/123云盘/xxx.mp4`。

**修复**：在 `StandalonePlayer.vue` 的 `startPlayback()` 中，对 `filePath.value` 做路径补全：
- 如果路径以 `/` 开头但不是以 `/storage/` 开头 → 视为相对 serve root 的路径
- 在原生平台上自动补全为 `/storage/emulated/0{原路径}`

### 问题 2：设置按钮无响应

**现象**：播放器界面右上角设置图标点击无反应

**根因**：`goSettings()` 方法使用 `router.push('/player/settings')` 跳转，但我们已移除 `<ion-router-outlet>`，Vue Router 不再工作。

**修复**：将设置页面改为内联渲染（类似主应用的 modal/popup 模式），或使用动态组件切换：

```vue
<!-- PlayerApp.vue 替代方案 -->
<template>
  <ion-app>
    <Suspense>
      <template #default>
        <StandalonePlayer v-if="!showSettings" @settings="showSettings = true" />
        <PlayerSettings v-else @close="showSettings = false" />
      </template>
    </Suspense>
  </ion-app>
</template>
```

同时在 `StandalonePlayer.vue` 中将 `goSettings()` 改为 emit 事件。

### 问题 3：ArtPlayer 全屏旋转

**需求**：全屏后根据视频宽高比智能旋转屏幕（横屏/竖屏）

**修复方向**：
- 监听 ArtPlayer 的 `fullscreen:change` 事件
- 进入全屏时获取视频宽高比 → 决定横屏还是竖屏
- 调用 Capacitor 的屏幕方向 API 或原生插件锁定方向
- 退出全屏时恢复原始方向

---

## 架构总结

### 文件清单

| 文件 | 用途 |
|------|------|
| `player.html` | 播放器 HTML 入口 |
| `src/player-main.ts` | 播放器 Vue 应用入口 |
| `src/PlayerApp.vue` | 播放器根组件（直接渲染 StandalonePlayer） |
| `src/views/StandalonePlayer.vue` | 播放器主组件（ArtPlayer + 控制逻辑） |
| `src/views/PlayerSettings.vue` | 播放器独立设置页 |
| `src/router/player.ts` | 播放器路由（当前未使用，保留备用） |
| `android-overlay/.../PlayerActivity.kt` | Android Activity（独立窗口 + 后端交互） |
| `android-overlay/.../GoProcessPlugin.kt` | Capacitor 插件（openInPlayer/isStandaloneMode） |
| `android-overlay/.../AndroidManifest.xml` | Document-Centric 声明 |

### 关键架构决策

1. **不覆写 `load()`** — 让 Capacitor 正常完成初始化，在 `onCreate().super.onCreate()` 之后导航
2. **不用 Ionic Router** — 直接组件渲染避免第二实例冲突
3. **Document-Centric 模型** — `FLAG_ACTIVITY_NEW_DOCUMENT` + `documentLaunchMode="always"` 实现独立窗口
4. **独立后端交互** — PlayerActivity 自己管理 EncvGoService 生命周期和广播接收
