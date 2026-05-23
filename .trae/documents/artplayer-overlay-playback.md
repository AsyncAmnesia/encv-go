# Artplayer 覆盖播放方案

## 背景

用户要求：
1. 删除 PlayerApp.vue 及引用的 player views（独立 Capacitor 播放器模式）
2. 音频播放选择：内置 MPV、外部打开
3. 视频播放选择：内置 MPV、内置 Artplayer、外部打开

## 架构设计

### 三种播放方式

| 方式 | 实现 | 适用 |
|------|------|------|
| 内置 Artplayer | 主 WebView 内路由到 `/player` 页面，使用 Artplayer（HTML5）播放 | 视频 |
| 内置 MPV | PlayerOverlayManager 在 MainActivity 上叠加 MpvSurfaceView + LynxView | 视频 + 音频 |
| 外部打开 | 启动 PlayerActivity（系统 Intent） | 视频 + 音频 |

### 播放选择流程

用户点击媒体文件 → 弹出 ActionSheet 选择播放方式 → 根据选择执行对应逻辑

- **视频文件**：显示三个选项（内置 Artplayer / 内置 MPV / 外部打开）
- **音频文件**：显示两个选项（内置 MPV / 外部打开）
- **加密文件**：同视频选项（解密后播放）

### 记住用户选择

用户选择后可勾选"记住选择"，后续同类型文件直接使用上次选择的方式。
存储在 localStorage，key: `player:preferred_video` / `player:preferred_audio`。

## 实施步骤

### 步骤 1：创建 Artplayer 播放器视图

新建 `/workspace/app/encv-mobile/src/views/ArtPlayerView.vue`

基于现有 `StandalonePlayer.vue` 的 Artplayer 逻辑，但改为：
- 作为主应用路由页面（不是独立 Activity）
- 从路由 query 获取文件信息（`path`、`name`）
- 使用 `getFileStreamUrl()` / `getExternalStreamUrl()` 获取流 URL
- 支持全屏切换（通过 GoProcess.setScreenOrientation）
- 顶部工具栏：返回按钮 + 文件名 + 播放方式切换按钮
- 视频区域使用 Artplayer
- 底部显示文件信息（分辨率、时长等）

### 步骤 2：修改路由配置

修改 `/workspace/app/encv-mobile/src/router/index.ts`：
- 保留 `/player` 路由，改为指向 `ArtPlayerView.vue`
- 删除对 `StandalonePlayer.vue` 的引用

### 步骤 3：创建播放选择逻辑

修改 `/workspace/app/encv-mobile/src/views/Files.vue`：

3.1 新增 `showPlayOptions()` 函数：
- 视频文件：弹出 ActionSheet，选项为"内置 Artplayer"/"内置 MPV"/"外部打开"
- 音频文件：弹出 ActionSheet，选项为"内置 MPV"/"外部打开"
- 如果用户已记住选择，直接使用上次选择

3.2 修改 `handleFileClick()`：
- 媒体文件不再直接播放，而是调用 `showPlayOptions()`

3.3 新增 `playWithArtplayer()` 函数：
- 路由到 `/player?path=xxx&name=xxx`

3.4 修改 `openPlayer()` 调用为 `playWithMPV()`：
- 调用 `GoProcess.openPlayer()`（PlayerOverlayManager）

3.5 新增 `playExternal()` 函数：
- 调用 `GoProcess.openInPlayer()`（PlayerActivity 跳转）

### 步骤 4：修改 HomePage.vue

播放器入口按钮改为路由到 `/player`（Artplayer 页面），不再调用 `openPlayer()`。

### 步骤 5：添加 i18n 键

在 `useI18n.ts` 中新增播放选择相关的翻译键：
- `player.playWith` / `player.builtInArtplayer` / `player.builtInMpv` / `player.openExternal`
- `player.rememberChoice`

### 步骤 6：删除旧文件

删除以下文件：
- `/workspace/app/encv-mobile/src/PlayerApp.vue`
- `/workspace/app/encv-mobile/src/views/StandalonePlayer.vue`
- `/workspace/app/encv-mobile/src/views/PlayerSettings.vue`
- `/workspace/app/encv-mobile/src/player-main.ts`
- `/workspace/app/encv-mobile/src/router/player.ts`
- `/workspace/app/encv-mobile/player.html`

### 步骤 7：修改 Vite 配置

修改 `/workspace/app/encv-mobile/vite.config.ts`：
- 移除 `player.html` 的多入口构建配置（`rollupOptions.input` 中的 player 入口）

### 步骤 8：清理 Android 端

8.1 保留 `PlayerActivityCapacitor.kt`（外部 Intent 打开仍需要），但标记为不再主动使用
8.2 保留 `PlayerOverlayManager.kt`（内置 MPV 仍需要）
8.3 保留 `PlayerActivityLynx.kt`（外部 Intent 打开仍需要）

### 步骤 9：清理 Lynx 播放器 JS 端

`/workspace/app/encv-mobile/lynx-player/` 目录保留（内置 MPV 仍需要 LynxView 作为 UI 控件层），不做删除。

### 步骤 10：构建验证

- `npx rspeedy build`（Lynx 播放器 bundle）
- `npx vue-tsc --noEmit && npx vite build`（前端构建）

## 文件变更清单

| 操作 | 文件 | 说明 |
|------|------|------|
| 新建 | `src/views/ArtPlayerView.vue` | Artplayer 播放器视图 |
| 修改 | `src/router/index.ts` | `/player` 路由指向 ArtPlayerView |
| 修改 | `src/views/Files.vue` | 播放选择逻辑 |
| 修改 | `src/views/HomePage.vue` | 播放器入口 |
| 修改 | `src/composables/useI18n.ts` | 新增播放选择 i18n 键 |
| 修改 | `src/plugins/GoProcess.ts` | 重命名/新增播放方式函数 |
| 修改 | `src/plugins/web.ts` | 接口同步 |
| 修改 | `vite.config.ts` | 移除 player.html 多入口 |
| 删除 | `src/PlayerApp.vue` | 独立播放器入口 |
| 删除 | `src/views/StandalonePlayer.vue` | 独立播放器视图 |
| 删除 | `src/views/PlayerSettings.vue` | 播放器设置 |
| 删除 | `src/player-main.ts` | 独立播放器 JS 入口 |
| 删除 | `src/router/player.ts` | 独立播放器路由 |
| 删除 | `player.html` | 独立播放器 HTML |

## 注意事项

1. **Artplayer 在 Capacitor WebView 中的兼容性**：Capacitor WebView 基于 Chrome/WebView，Artplayer 的 HTML5 播放完全兼容
2. **流 URL**：Artplayer 通过 Go 后端的 `/stream` API 获取视频流，需要后端已启动
3. **全屏**：Artplayer 自带全屏功能，配合 `GoProcess.setScreenOrientation()` 锁定屏幕方向
4. **MPV 覆盖层与 Artplayer 互斥**：同一时间只能使用一种内置播放方式
5. **记住选择**：使用 localStorage 存储偏好，用户可在设置中清除
