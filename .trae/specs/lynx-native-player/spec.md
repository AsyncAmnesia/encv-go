# Lynx + mpv 原生播放器 Spec

## Why

当前 PlayerActivity 基于 Capacitor BridgeActivity（WebView），ArtPlayer 在 Android WebView 中始终显示浏览器原生视频控件而非自定义 UI，CSS `::-webkit-media-controls` 隐藏和 JS `removeAttribute('controls')` 均无效。这是 WebView 的根本限制。引入 Lynx 原生渲染引擎替代 WebView 做 UI 渲染，集成 mpv（内含 ffmpeg）做视频解码播放，从根本上解决 WebView 原生控件问题，同时获得原生级性能和全格式支持。

## What Changes

- 新建 Lynx 播放器子项目 `encv-mobile/lynx-player/`，使用 ReactLynx 编写播放器 UI
- 修改 `PlayerActivity.kt`，从 `BridgeActivity`（Capacitor WebView）改为 `AppCompatActivity`，嵌入 `LynxView` 替代 WebView
- 新建 Native Module `MpvPlayerModule`，封装 mpv-android-lib（`io.github.abdallahmehiz:mpv-android-lib`），暴露播放控制 API 给 Lynx JS 层
- 新建 Native Module `GoBackendModule`，封装 Go 后端交互（获取状态、启动服务、流式 URL），替代 Capacitor GoProcessPlugin
- 保留旧 Capacitor 播放器代码（`StandalonePlayer.vue`、`PlayerApp.vue`、`player-main.ts`、`player.html`）不删除，通过构建变体切换，方便回滚
- 修改 `AndroidManifest.xml` 中 PlayerActivity 配置以适配 LynxView
- 修改 `build.gradle.kts` 添加 Lynx SDK + mpv-android-lib 依赖
- 修改 `post-cap-sync.mjs` 添加 Lynx bundle 复制逻辑

## Impact

- Affected specs: `upgrade-player-to-activity`（PlayerActivity 实现方式从 Capacitor WebView 改为 Lynx + mpv）
- Affected code:
  - `android-overlay/app/src/main/java/com/encvgo/app/PlayerActivity.kt` — 重写为 LynxView 宿主
  - `android-overlay/app/build.gradle.kts` — 添加 Lynx SDK + mpv-android-lib 依赖
  - `android-overlay/app/src/main/AndroidManifest.xml` — PlayerActivity 配置调整
  - `scripts/post-cap-sync.mjs` — 添加 Lynx bundle 复制
  - 新建 `lynx-player/` — Lynx 播放器子项目
  - 新建 Native Module Kotlin 文件 — MpvPlayerModule、GoBackendModule

## ADDED Requirements

### Requirement: Lynx 播放器子项目

系统 SHALL 提供 `encv-mobile/lynx-player/` 子项目，使用 ReactLynx 编写播放器 UI。

#### Scenario: 项目结构
- **WHEN** 开发者查看 `lynx-player/` 目录
- **THEN** 包含以下文件：
  - `package.json` — 依赖 @lynx-js/react、@lynx-js/rspeedy 等
  - `rspeedy.config.ts` — Rspeedy 构建配置
  - `src/App.tsx` — 播放器主界面
  - `src/components/VideoPlayer.tsx` — 视频播放器组件（mpv SurfaceView 容器 + 自定义控件覆盖层）
  - `src/components/PlayerControls.tsx` — 自定义播放控件（橙色主题 #ffad00）
  - `src/components/PlayerSettings.tsx` — 播放器设置面板
  - `src/typing.d.ts` — Native Module 类型声明（MpvPlayerModule、GoBackendModule）

#### Scenario: 构建输出
- **WHEN** 执行 `npm run build`
- **THEN** 生成 `dist/player.lynx.bundle` 文件，可被 Android LynxView 加载

### Requirement: PlayerActivity 改为 LynxView 宿主

系统 SHALL 将 PlayerActivity 从 Capacitor BridgeActivity 改为 AppCompatActivity，嵌入 LynxView 渲染播放器 UI。

#### Scenario: Activity 启动
- **WHEN** PlayerActivity 启动
- **THEN** 创建 LynxView，加载 `player.lynx.bundle`
- **THEN** 通过 LynxView 初始数据传递文件路径、文件名、MIME 类型

#### Scenario: 独立窗口
- **WHEN** PlayerActivity 启动
- **THEN** 仍使用 `FLAG_ACTIVITY_NEW_DOCUMENT` + `documentLaunchMode="always"` + 独立 taskAffinity，保持独立窗口行为

#### Scenario: 后端交互
- **WHEN** Lynx 播放器需要与 Go 后端交互
- **THEN** 通过 GoBackendModule Native Module 调用原生层，原生层复用现有 EncvGoService 逻辑

### Requirement: MpvPlayerModule Native Module

系统 SHALL 提供 MpvPlayerModule，封装 mpv-android-lib 播放能力，暴露给 Lynx JS 层。mpv 内置 ffmpeg，支持几乎所有视频/音频格式。

#### Scenario: 播放视频
- **WHEN** JS 层调用 `NativeModules.MpvPlayerModule.play(url)`
- **THEN** 原生层创建 MPVView 实例，设置播放 URL，开始播放
- **THEN** MPVView 的 SurfaceView 嵌入到 Activity 布局中

#### Scenario: 播放控制
- **WHEN** JS 层调用 `NativeModules.MpvPlayerModule.pause()` / `resume()` / `seekTo(positionMs)`
- **THEN** 原生层控制 mpv 执行对应操作

#### Scenario: 播放状态回调
- **WHEN** mpv 播放状态变化（播放中、暂停、结束、错误）
- **THEN** 通过 Lynx 事件机制通知 JS 层

#### Scenario: 全屏切换
- **WHEN** JS 层调用 `NativeModules.MpvPlayerModule.setFullscreen(true)`
- **THEN** Activity 进入全屏模式，MPVView 扩展为全屏布局
- **WHEN** JS 层调用 `NativeModules.MpvPlayerModule.setFullscreen(false)`
- **THEN** 退出全屏，恢复竖屏方向

#### Scenario: 屏幕方向
- **WHEN** JS 层调用 `NativeModules.MpvPlayerModule.setOrientation('landscape')`
- **THEN** Activity 锁定为横屏
- **WHEN** JS 层调用 `NativeModules.MpvPlayerModule.setOrientation('portrait')`
- **THEN** Activity 恢复为竖屏

#### Scenario: Range 请求流式播放
- **WHEN** 播放 URL 为 `http://127.0.0.1:{port}/api/stream/external?path=...`
- **THEN** mpv 通过 HTTP Range 请求流式播放，支持 seek

#### Scenario: 加密文件播放
- **WHEN** 播放 .encv 加密文件
- **THEN** 通过后端 `/api/stream/external` 端点解密流式传输，mpv 正常播放

#### Scenario: 字幕支持
- **WHEN** 视频包含内嵌字幕或外挂字幕
- **THEN** mpv 通过 libass 渲染字幕，支持 SRT/ASS/SSA 等格式

#### Scenario: 硬件解码
- **WHEN** 设备支持对应编码的硬件解码
- **THEN** mpv 自动使用硬件解码（MediaCodec）
- **WHEN** 硬件解码失败
- **THEN** mpv 自动回退到 ffmpeg 软件解码

### Requirement: GoBackendModule Native Module

系统 SHALL 提供 GoBackendModule，封装 Go 后端交互能力，替代 Capacitor GoProcessPlugin。

#### Scenario: 获取后端状态
- **WHEN** JS 层调用 `NativeModules.GoBackendModule.getBackendStatus(callback)`
- **THEN** 原生层检查 EncvGoService 状态，回调返回 `{ running: boolean, port: number }`

#### Scenario: 启动后端
- **WHEN** JS 层调用 `NativeModules.GoBackendModule.startBackend()`
- **THEN** 原生层启动 EncvGoService，注册 BroadcastReceiver 等待就绪广播

#### Scenario: 后端就绪通知
- **WHEN** EncvGoService 就绪
- **THEN** 通过 Lynx 事件机制通知 JS 层，传递端口号

#### Scenario: 获取流式 URL
- **WHEN** JS 层调用 `NativeModules.GoBackendModule.getStreamUrl(path, isExternal)`
- **THEN** 返回 `http://127.0.0.1:{port}/stream?path=...` 或 `http://127.0.0.1:{port}/api/stream/external?path=...`

### Requirement: 旧 Capacitor 播放器保留与回滚

系统 SHALL 保留旧 Capacitor 播放器代码，支持通过构建变体切换。

#### Scenario: 代码保留
- **WHEN** 查看代码仓库
- **THEN** `StandalonePlayer.vue`、`PlayerApp.vue`、`player-main.ts`、`player.html`、旧 `PlayerActivity.kt`（Capacitor 版本）均保留

#### Scenario: 构建变体切换
- **WHEN** Gradle 配置 `buildConfigField "BOOLEAN", "USE_LYNX_PLAYER", "true"`
- **THEN** PlayerActivity 使用 LynxView + mpv 实现
- **WHEN** Gradle 配置 `buildConfigField "BOOLEAN", "USE_LYNX_PLAYER", "false"`
- **THEN** PlayerActivity 使用旧 Capacitor BridgeActivity 实现

### Requirement: Lynx 播放器 UI

系统 SHALL 提供 Lynx 播放器 UI，包含视频播放、自定义控件、设置面板。

#### Scenario: 播放器界面
- **WHEN** Lynx 播放器加载
- **THEN** 显示视频播放区域（MPVView SurfaceView）+ 自定义控件覆盖层
- **THEN** 控件包含：播放/暂停、进度条、时间显示、音量、全屏按钮
- **THEN** 控件使用橙色主题（#ffad00），与主应用一致

#### Scenario: 音频播放
- **WHEN** 文件为音频类型
- **THEN** 显示音频可视化界面（专辑封面占位 + 控件），mpv 仅播放音频

#### Scenario: 设置面板
- **WHEN** 用户点击设置图标
- **THEN** 显示设置面板，包含：自动播放、硬件解码、深色模式开关
- **THEN** 设置通过 GoBackendModule 存储到后端配置

#### Scenario: 暗黑模式
- **WHEN** 系统或用户设置为深色模式
- **THEN** 播放器 UI 切换为深色主题

## MODIFIED Requirements

### Requirement: PlayerActivity 实现

PlayerActivity SHALL 支持两种实现模式：Lynx + mpv 原生渲染（默认）和 Capacitor WebView（回滚），通过 Gradle 构建变体 `USE_LYNX_PLAYER` 切换。

### Requirement: AndroidManifest.xml PlayerActivity

PlayerActivity 的 `configChanges` SHALL 包含 `orientation|screenSize|screenLayout|smallestScreenSize|uiMode` 以适配 LynxView 全屏切换和 mpv SurfaceView。

## REMOVED Requirements

无移除的需求。旧 Capacitor 播放器代码保留但不再作为默认实现。
