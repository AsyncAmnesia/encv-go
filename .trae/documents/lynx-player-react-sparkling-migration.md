# 修复计划：Capacitor 主页面内嵌 MPV 播放器 + ReactLynx 播放器 UI

## 背景

用户要求：主流视频播放器 App（YouTube、Bilibili 等）播放视频时不跳转新 Activity，而是在当前页面内嵌入播放器。当前主应用是 Capacitor（WebView），需要在 Capacitor 页面内嵌入 MPV 播放器。

## 架构方案

**在 MainActivity（Capacitor BridgeActivity）中动态叠加 MPV SurfaceView + LynxView 播放器 UI 控件层**

```
MainActivity (BridgeActivity)
├── WebView（Capacitor 主界面，全屏）
└── PlayerOverlay（FrameLayout，动态添加/移除）
    ├── MpvSurfaceView（视频渲染层）
    └── LynxView（播放器 UI 控件层，透明背景）
```

- 播放时：PlayerOverlay 覆盖在 WebView 上方，MPV 渲染视频，LynxView 渲染 UI 控件
- 不播放时：PlayerOverlay 隐藏/移除，WebView 全屏显示
- Capacitor Plugin（GoProcessPlugin）提供 JS API 控制播放器的显示/隐藏/播放/暂停等

## 实施步骤

### 第一阶段：JS 端 — ReactLynx 播放器 UI

#### 步骤1：重构 lynx-player 项目为 ReactLynx

- 删除所有 Vue 文件和 vue-lynx/vue-router 依赖
- 添加 `@lynx-js/react` 依赖
- 修改 `lynx.config.ts`：
  - 替换 `pluginVueLynx()` 为 `@lynx-js/react-rsbuild-plugin` 的 `pluginReactLynx()`
  - 设置 `defaultDisplayLinear: false`
  - 配置入口：`entry: './src/player/index.tsx'`
  - 输出 bundle 名：`player.lynx.bundle`

#### 步骤2：实现播放器 React 组件

文件结构：
```
src/
  player/
    index.tsx          — 入口，root.render(<PlayerApp />)
    PlayerApp.tsx      — 主组件，状态管理 + MPV 事件监听
    PlayerControls.tsx — 播放控件（播放/暂停/进度条/全屏等）
    ProgressBar.tsx    — 进度条组件
    player.css         — 样式
```

核心逻辑从 Vue 版移植，关键变化：
- `useState`/`useEffect`/`useCallback` 替代 Vue Composition API
- `lynx.__globalProps` 获取 initData
- `GlobalEventEmitter` 监听 mpv 事件
- `NativeModules` 调用 MpvPlayerModule/GoBackendModule
- 事件使用 `bindtap` 而非 `@tap`
- 所有 CSS 显式声明 `display: flex`（`defaultDisplayLinear: false` 后默认就是 flex，但显式声明更安全）

#### 步骤3：构建验证 JS 端

`npx rspeedy build` 成功，生成 `dist/player.lynx.bundle`

### 第二阶段：Android 端 — 在 MainActivity 中嵌入播放器

#### 步骤4：创建 PlayerOverlayManager

文件：`android/app/src/main/java/com/encvgo/app/PlayerOverlayManager.kt`

职责：
- 管理 PlayerOverlay（FrameLayout）的添加/移除
- 创建 MpvSurfaceView 和 LynxView 并添加到 PlayerOverlay
- 提供 `showPlayer(filePath, fileName, mimeType, isExternal)` 和 `hidePlayer()` 方法
- 将 PlayerOverlay 添加到 MainActivity 的 `android.R.id.content` 中

```kotlin
class PlayerOverlayManager(private val activity: Activity) {
    private var playerOverlay: FrameLayout? = null
    private var lynxView: LynxView? = null
    private var mpvSurfaceView: MpvSurfaceView? = null

    fun showPlayer(filePath: String, fileName: String, mimeType: String, isExternal: Boolean) {
        if (playerOverlay != null) return // 已显示
        
        // 1. 创建 PlayerOverlay
        playerOverlay = FrameLayout(activity).apply {
            layoutParams = FrameLayout.LayoutParams(MATCH_PARENT, MATCH_PARENT)
            setBackgroundColor(Color.BLACK)
        }
        
        // 2. 创建 MpvSurfaceView
        mpvSurfaceView = MpvSurfaceView(activity).apply {
            layoutParams = FrameLayout.LayoutParams(MATCH_PARENT, MATCH_PARENT)
        }
        playerOverlay!!.addView(mpvSurfaceView)
        
        // 3. 创建 LynxView（播放器 UI 控件层）
        lynxView = createLynxView(filePath, fileName, mimeType, isExternal)
        playerOverlay!!.addView(lynxView)
        
        // 4. 添加到 MainActivity 的 content view
        val contentView = activity.findViewById<FrameLayout>(android.R.id.content)
        contentView.addView(playerOverlay)
        
        // 5. 初始化 MPV
        MpvPlayerModule.preInit(activity)
        MpvPlayerModule.getInstance()?.attachToLayout(playerOverlay!!)
    }

    fun hidePlayer() {
        MpvPlayerModule.getInstance()?.let {
            it.pause {}
            it.detachFromLayout(playerOverlay!!)
            it.release()
        }
        lynxView?.destroy()
        val contentView = activity.findViewById<FrameLayout>(android.R.id.content)
        playerOverlay?.let { contentView.removeView(it) }
        playerOverlay = null
        lynxView = null
        mpvSurfaceView = null
    }
}
```

#### 步骤5：修改 GoProcessPlugin 添加播放器控制 API

文件：`android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt`

添加 Capacitor Plugin 方法：
- `openPlayer(call)` — 显示播放器覆盖层，传入 filePath 等参数
- `closePlayer(call)` — 隐藏播放器覆盖层
- `playerControl(call)` — 播放/暂停/seek 等控制（透传到 MpvPlayerModule）

```kotlin
@CapacitorPlugin(name = "GoProcess")
class GoProcessPlugin : Plugin() {
    private var playerOverlayManager: PlayerOverlayManager? = null
    
    @PluginMethod
    fun openPlayer(call: PluginCall) {
        val filePath = call.getString("filePath") ?: ""
        val fileName = call.getString("fileName") ?: ""
        val mimeType = call.getString("mimeType") ?: ""
        val isExternal = call.getBoolean("isExternal", false)
        
        activity.runOnUiThread {
            if (playerOverlayManager == null) {
                playerOverlayManager = PlayerOverlayManager(activity as MainActivity)
            }
            playerOverlayManager!!.showPlayer(filePath, fileName, mimeType, isExternal)
        }
        call.resolve()
    }
    
    @PluginMethod
    fun closePlayer(call: PluginCall) {
        activity.runOnUiThread {
            playerOverlayManager?.hidePlayer()
        }
        call.resolve()
    }
}
```

#### 步骤6：前端 Capacitor 调用

在 Ionic/Vue 前端中：
```typescript
import { GoProcess } from '@/plugins/GoProcess'

// 打开播放器（在当前页面内嵌入，不跳转 Activity）
GoProcess.openPlayer({
  filePath: '/path/to/video.mp4',
  fileName: 'video.mp4',
  mimeType: 'video/mp4',
  isExternal: false,
})

// 关闭播放器
GoProcess.closePlayer()
```

#### 步骤7：修改 PlayerActivity 路由

当前 `PlayerActivity` 会跳转到 `PlayerActivityLynx`。修改为：
- 如果从 Capacitor 主界面调用，不跳转 Activity，而是通过 `PlayerOverlayManager` 在当前页面嵌入
- 如果从外部 Intent（如文件管理器打开），仍然跳转到 `PlayerActivityLynx`（独立 Activity）

### 第三阶段：Sparkling 导航（后续扩展）

#### 步骤8：集成 Sparkling SDK（可选，后续扩展）

当前播放器不需要 Sparkling 导航（因为在同一 Activity 内）。但未来如果需要独立的设置页面、播放列表页面等，可以通过 Sparkling 的 `navigate()` 打开新容器。

### 第四阶段：验证

#### 步骤9：构建验证

- JS 端：`npx rspeedy build` 成功
- Android 端：`./gradlew assembleDebug` 成功
- 运行时验证：从 Capacitor 主页面点击文件 → 播放器覆盖层显示 → MPV 播放视频 → UI 控件正常 → 关闭播放器返回主页面

## 关键注意事项

1. **MpvSurfaceView 层级**：SurfaceView 是特殊 View，必须放在 LynxView 下方（先添加到 FrameLayout），否则会遮挡 UI 控件
2. **LynxView 透明背景**：`lynxView.setBackgroundColor(0)` 确保透明，让 MPV 视频可见
3. **生命周期管理**：PlayerOverlayManager 需要正确处理 Activity 的 onPause/onResume/onDestroy
4. **`defaultDisplayLinear: false`**：确保 Lynx 默认使用 flex 布局
5. **ReactLynx 事件**：使用 `bindtap` 而非 `onClick`
6. **NativeModules**：ReactLynx 中通过 `globalThis.NativeModules` 访问
7. **initData 传递**：通过 `lynxView.renderTemplateUrl("player.lynx.bundle", initDataJson)` 传递，JS 端通过 `lynx.__globalProps` 读取
