# Tasks

- [ ] Task 1: 初始化 Lynx 播放器子项目
  - [ ] SubTask 1.1: 创建 `encv-mobile/lynx-player/` 目录，初始化 package.json，安装 @lynx-js/react、@lynx-js/rspeedy 等依赖
  - [ ] SubTask 1.2: 配置 rspeedy.config.ts，设置构建输出为 `dist/player.lynx.bundle`
  - [ ] SubTask 1.3: 创建 `src/typing.d.ts`，声明 MpvPlayerModule 和 GoBackendModule 接口
  - [ ] SubTask 1.4: 创建 `src/App.tsx` 播放器主界面骨架

- [ ] Task 2: 实现 MpvPlayerModule Native Module（Android）
  - [ ] SubTask 2.1: 创建 `MpvPlayerModule.kt`，封装 MPVView 实例管理（play/pause/seekTo/release）
  - [ ] SubTask 2.2: 实现 MPVView SurfaceView 嵌入机制，将视频输出嵌入 Activity 布局
  - [ ] SubTask 2.3: 实现播放状态回调（mpv-event: idle/paused/playing/end-file/error）
  - [ ] SubTask 2.4: 实现全屏切换（setFullscreen）和屏幕方向控制（setOrientation）
  - [ ] SubTask 2.5: 实现 Range 请求流式播放支持（mpv 默认支持 HTTP Range）
  - [ ] SubTask 2.6: 实现硬件/软件解码切换（mpv hwdec 属性）

- [ ] Task 3: 实现 GoBackendModule Native Module（Android）
  - [ ] SubTask 3.1: 创建 `GoBackendModule.kt`，封装 EncvGoService 交互（getBackendStatus、startBackend）
  - [ ] SubTask 3.2: 实现后端就绪广播监听，通过 Lynx 事件通知 JS 层
  - [ ] SubTask 3.3: 实现 getStreamUrl 方法，返回正确的流式 URL

- [ ] Task 4: 重写 PlayerActivity 为 LynxView 宿主
  - [ ] SubTask 4.1: 修改 PlayerActivity 继承 AppCompatActivity，创建 LynxView 替代 WebView
  - [ ] SubTask 4.2: 实现 USE_LYNX_PLAYER 构建变体，支持 Lynx/Capacitor 双模式切换
  - [ ] SubTask 4.3: 保留旧 Capacitor PlayerActivity 代码为 `PlayerActivityCapacitor.kt`
  - [ ] SubTask 4.4: 实现 LynxView 初始数据传递（文件路径、文件名、MIME 类型）
  - [ ] SubTask 4.5: 注册 MpvPlayerModule 和 GoBackendModule 到 LynxEnv
  - [ ] SubTask 4.6: 实现 MPVView 与 LynxView 的布局协调（FrameLayout 叠加）

- [ ] Task 5: 实现 Lynx 播放器 UI
  - [ ] SubTask 5.1: 实现 VideoPlayer 组件（MPVView SurfaceView 容器 + 控件覆盖层）
  - [ ] SubTask 5.2: 实现 PlayerControls 组件（播放/暂停、进度条、时间、音量、全屏，橙色主题 #ffad00）
  - [ ] SubTask 5.3: 实现 PlayerSettings 组件（自动播放、硬件解码、深色模式）
  - [ ] SubTask 5.4: 实现暗黑模式支持（CSS 变量 + 系统主题检测）
  - [ ] SubTask 5.5: 实现音频播放界面（专辑封面占位 + 控件）
  - [ ] SubTask 5.6: 实现后端连接状态 UI（加载动画、错误提示、重试按钮）

- [ ] Task 6: 修改 Android 构建配置
  - [ ] SubTask 6.1: 修改 build.gradle.kts，添加 Lynx SDK 依赖（org.lynxsdk.lynx:lynx、lynx-service-image 等）
  - [ ] SubTask 6.2: 修改 build.gradle.kts，添加 mpv-android-lib 依赖（io.github.abdallahmehiz:mpv-android-lib:0.1.12）
  - [ ] SubTask 6.3: 添加 USE_LYNX_PLAYER 构建变体配置
  - [ ] SubTask 6.4: 修改 AndroidManifest.xml PlayerActivity 配置
  - [ ] SubTask 6.5: 修改 post-cap-sync.mjs，添加 Lynx bundle 复制逻辑
  - [ ] SubTask 6.6: 添加 Lynx 混淆规则

- [ ] Task 7: 集成测试与验证
  - [ ] SubTask 7.1: 本地构建验证（npm run build + gradle assembleDebug）
  - [ ] SubTask 7.2: 验证第三方打开视频文件播放正常
  - [ ] SubTask 7.3: 验证应用内打开视频播放正常
  - [ ] SubTask 7.4: 验证全屏/退出全屏/屏幕旋转
  - [ ] SubTask 7.5: 验证加密 .encv 文件播放
  - [ ] SubTask 7.6: 验证回滚到 Capacitor 模式正常

# Task Dependencies

- [Task 2] depends on [Task 4]（MpvPlayerModule 需要在 PlayerActivity 中注册和布局协调）
- [Task 3] depends on [Task 4]（GoBackendModule 需要在 PlayerActivity 中注册）
- [Task 5] depends on [Task 1]（UI 代码依赖子项目初始化）+ [Task 2] + [Task 3]（UI 调用 Native Module）
- [Task 6] depends on [Task 4]（构建配置依赖 PlayerActivity 实现）
- [Task 7] depends on [Task 1-6] 全部完成
- [Task 1] 和 [Task 4] 可并行启动
