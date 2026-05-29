# 修复：加密视频回滚 ArtPlayer + MPV 加载状态转圈圈

## 问题分析

### 问题 1：加密视频回滚到 ArtPlayer

**根因链**：

1. `VIDEO_DEFAULT = PLAY_MODE.ARTPLAYER`（[player.ts:12](src/constants/player.ts)）
2. 用户未显式切换播放器偏好时，`getPlayMode('video')` 返回 `'artplayer'`
3. `playMedia()` 走 `PLAY_MODE.ARTPLAYER` 分支 → `router.push('/player')` → ArtPlayer WebView
4. ArtPlayer 虽然也用 HTTP stream（`/stream?path=...`），但加密文件需要 MPV 的原生解密流支持

**修复**：在 `playMedia()` 中，当 `file.isEncrypted` 为 true 时，强制使用 MPV 模式，忽略用户偏好。加密文件必须走 MPV 原生播放器。

### 问题 2：加载状态一直转圈圈

**根因链**：

1. `MpvPlayerActivity.onCreate()` 中 `engine.initialize()` 在 L47 执行，内部调用 `notifyState(State.MpvReady)`
2. `engine.stateListener` 在 L83 才设置（在 `initialize()` 之后）
3. `MpvReady` 事件丢失 → `attachSurfaceView()` 永远不执行 → `MpvSurfaceView` 永远不创建
4. `MpvPlayerScreen.LaunchedEffect` 调用 `startPlayback()` → `engine.play(url)` → `surfaceReady=false` → `pendingUrl` 被设置但永远不被消费
5. `MpvPlayerScreen` 没有订阅 `engine.stateListener`，engine 的异步状态变化（Playing/Paused/Error）无法传递到 Compose UI
6. 结果：Compose UI 停留在 `PlayerState.Loading`，永远转圈圈

**修复**：
- 在 `MpvPlayerActivity.onCreate()` 中，`setContent` 后直接调用 `attachSurfaceView()`，不依赖 `stateListener` 回调
- 在 `MpvPlayerScreen` 和 `MpvAudioPlayerScreen` 中添加 `DisposableEffect(engine)` 订阅 `engine.stateListener`

---

## 实施步骤

### Step 1：Files.vue 第 317 行加密徽章修复

文件：`src/views/Files.vue` 第 317 行

```diff
- <ion-badge v-if="file.isEncrypted || getFileCategory(file.name, file.isEncrypted) === 'encrypted'" color="warning" slot="end">
+ <ion-badge v-if="file.isEncrypted || getFileCategory(file.name, file.isEncrypted).startsWith('encrypted')" color="warning" slot="end">
```

### Step 2：playMedia 强制加密文件使用 MPV

文件：`src/views/Files.vue` 的 `playMedia` 函数

在 `getPlayMode` 之后添加加密文件检查，强制覆盖为 MPV：

```typescript
async function playMedia(file: FileItem, category: string) {
  const isVideo = category === 'video'
  const mediaType = isVideo ? 'video' : 'audio'
  const mimeType = isVideo ? 'video/*' : 'audio/*'
  let mode = getPlayMode(mediaType)

  // 加密文件必须走 MPV（HTTP stream 解密流）
  if (file.isEncrypted && !mode.startsWith('mpv-')) {
    mode = PLAY_MODE.MPV_ACTIVITY
  }

  // ... 后续逻辑不变
}
```

### Step 3：MpvPlayerActivity — Surface 直接挂载

文件：`plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvPlayerActivity.kt`

核心改动：
1. 删除 `stateListener` 回调方式挂载 Surface 的代码（L80-95）
2. 在 `setContent` 之后直接调用 `attachSurfaceView()`

```kotlin
override fun onCreate(savedInstanceState: Bundle?) {
    super.onCreate(savedInstanceState)
    val host = proxyActivity ?: return
    val hostIntent = host.intent ?: return
    // ... 读取参数 ...

    engine = createMpvEngine(host)
    engine.initialize()

    host.setContent {
        EncvMpVPlayerTheme {
            Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
                if (audioMode) {
                    MpvAudioPlayerScreen(...)
                } else {
                    MpvPlayerScreen(...)
                }
            }
        }
    }

    // 直接挂载 Surface，不依赖 stateListener 回调
    if (!audioMode) {
        val decorView = host.window?.decorView as? ViewGroup
        if (decorView != null) {
            val contentRoot = decorView.findViewById<ViewGroup>(android.R.id.content)
            if (contentRoot != null) {
                engine.attachSurfaceView(contentRoot)
            }
        }
    }
}
```

### Step 4：MpvPlayerScreen — 订阅 engine.stateListener

文件：`plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvPlayerScreen.kt`

在现有 `DisposableEffect(Unit)` 之前添加：

```kotlin
DisposableEffect(engine) {
    val listener: (MpvEngine.State) -> Unit = { state ->
        when (state) {
            is MpvEngine.State.Playing -> playerState = PlayerState.Playing
            is MpvEngine.State.Paused -> playerState = PlayerState.Paused
            is MpvEngine.State.AudioOnly -> playerState = PlayerState.AudioOnly
            is MpvEngine.State.Ended -> playerState = PlayerState.Ended
            is MpvEngine.State.Error -> playerState = PlayerState.Error(classifyError(state.message), state.message)
            is MpvEngine.State.SurfaceReady -> { /* Surface 就绪，等待播放 */ }
            is MpvEngine.State.WaitingSurface -> { /* 等待 Surface，已设置 pendingUrl */ }
            is MpvEngine.State.MpvReady -> { /* MPV 引擎就绪 */ }
        }
    }
    engine.stateListener = listener
    onDispose {
        engine.stateListener = null
    }
}
```

同时修改 `startPlayback` 函数：删除 `onStateChange(PlayerState.Loading)` 的第二次调用（L298），因为状态现在由 `stateListener` 驱动。

### Step 5：MpvAudioPlayerScreen — 订阅 engine.stateListener

文件：`plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvAudioPlayerScreen.kt`

同样添加 `DisposableEffect(engine)` 订阅 `engine.stateListener`，映射到 Compose `PlayerState`。

同时修改 `LaunchedEffect(filePath)` 中的播放逻辑：不再手动设置 `playerState = PlayerState.AudioOnly`，而是由 `stateListener` 驱动状态变化。

### Step 6：验证构建

```bash
cd /workspace/app/encv-mobile && ./gradlew :plugin-mpv-player:compileDebugKotlin
```
