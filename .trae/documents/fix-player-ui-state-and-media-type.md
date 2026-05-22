# 修复播放器 UI 状态卡死 + 区分视频/音频控件

## 问题分析

### 问题 1："正在初始化视频窗口..." 永远覆盖在视频上层

**根因**：JS 端事件处理逻辑缺陷。

时序追踪：
1. JS 调用 `play()` → native `play()` 发现 `surfaceReady=false` → 设置 `pendingUrl` → 派发 `"waiting_surface"` 事件
2. JS 收到 `"waiting_surface"` → `setPlayerState("loading")` + `setErrorMessage("正在初始化视频窗口...")` → **UI 显示 loading 状态**
3. 主线程执行 `init{}` 的 `mainHandler.post` → `attachToLayout()` → SurfaceView 被添加
4. `surfaceCreated()` → `surfaceReady=true` → 派发 `"surface_ready"` → 自动播放 `pendingUrl`
5. JS 收到 `"surface_ready"` → **直接 return，不更新 playerState！** → playerState 仍然是 `"loading"`
6. MPV 播放文件 → 派发 `"playing"` 事件 → JS 收到 → `setPlayerState("playing")`

**问题出在第 5 步**：`surface_ready` 事件处理器直接 return，没有更新 `playerState`。虽然第 6 步的 `"playing"` 事件应该能修复状态，但存在竞态条件：

- 如果 `"playing"` 事件在 `"surface_ready"` 之前到达（MPV 文件加载非常快），JS 状态会正确更新
- 如果 `"playing"` 事件延迟或丢失（Lynx 全局事件是异步的），UI 就卡在 loading 状态

**更深层的问题**：`PlayerControls.tsx` 中 `state === 'loading'` 时显示 loading dots（三个点），**不是**显示 "正在初始化视频窗口..." 文字。但 `errorMessage` 被设置为 "正在初始化视频窗口..."，且 `error` 优先于 `state === 'loading'` 判断：

```tsx
// PlayerControls.tsx 第 49 行
if (error) {
    return (  // ← error 优先级最高！显示错误 UI
        <view className="ErrorContainer">
            <text className="ErrorTitle">⚠ 播放失败</text>
            <text className="ErrorDetail">{error}</text>  // ← "正在初始化视频窗口..."
        </view>
    );
}
```

**真正根因**：`setErrorMessage("正在初始化视频窗口...")` 把 loading 状态当成了 error！`PlayerControls` 中 `error` 检查优先于 `state` 检查，所以一旦 `errorMessage` 被设置，UI 就永远显示错误界面。后续 `"playing"` 事件虽然更新了 `playerState`，但 `errorMessage` 从未被清除。

### 问题 2：没有显示播放控件

当视频在播放时（`state === 'playing'`），`PlayerControls` 应该显示 TopBar + CenterArea + BottomBar。但前提是 `error` 为空。由于问题 1 中 `errorMessage` 从未清除，`error` 始终为 truthy，所以永远不会进入正常播放控件分支。

### 问题 3：需要区分视频/音频显示不同控件

当前 `initData` 包含 `mimeType`，但 JS 端没有利用它来区分媒体类型。需要：
- 视频：显示 SurfaceView + 全屏按钮 + 视频控件
- 音频：隐藏 SurfaceView + 显示专辑封面占位 + 音频控件（无全屏按钮）

## 修复方案

### Step 1：修复 JS 端事件处理逻辑（核心修复）

**文件**：`AppComponent.tsx`

1. `"waiting_surface"` 事件：不再设置 `errorMessage`，只设置 `playerState("loading")`
2. `"surface_ready"` 事件：清除 errorMessage，设置 `playerState("loading")`（等待 playing 事件）
3. `"playing"` 事件：清除 errorMessage（通过 `setErrorMessage("")`）
4. `startPlayback()` 中：`setPlayerState("loading")` 时也清除 `errorMessage`

```tsx
useLynxGlobalEventListener("mpv:state-change", (event: any) => {
    const state = event?.state;
    const error = event?.error;
    if (state) {
        if (state === "surface_ready") {
            setErrorMessage("");
            return;
        }
        if (state === "waiting_surface") {
            setPlayerState("loading");
            // 不设置 errorMessage！loading 不是 error
            return;
        }
        if (state === "mpv_ready") return;
        setPlayerState(state as PlayerState);
    }
    if (error) setErrorMessage(error);
    if (state === "playing" || state === "paused") {
        setErrorMessage("");
        setShowControls(true);
    }
});
```

### Step 2：修复 PlayerControls loading 状态显示

**文件**：`PlayerControls.tsx`

loading 状态应该显示 "正在加载..." 文字提示，而不是只有三个点。同时确保 error 和 loading 是互斥的：

```tsx
if (state === 'loading') {
    return (
        <view className="CenterArea">
            <view className="LoadingDots">
                <view className="Dot DotDim1" />
                <view className="Dot DotDim2" />
                <view className="Dot DotDim3" />
            </view>
            <text className="IdleTitle">正在加载...</text>
        </view>
    );
}
```

### Step 3：区分视频/音频，传递 mediaType 到前端

**文件**：`PlayerActivityLynx.kt`

在 `buildInitDataJson()` 中添加 `mediaType` 字段：

```kotlin
private fun buildInitDataJson(): String {
    val mediaType = when {
        intentFileMimeType.startsWith("audio/") -> "audio"
        intentFileMimeType.startsWith("video/") -> "video"
        intentFileMimeType.isEmpty() -> {
            val ext = intentFileName.substringAfterLast('.', "").lowercase()
            when (ext) {
                "mp3", "flac", "wav", "ogg", "aac", "m4a", "wma", "opus" -> "audio"
                else -> "video"
            }
        }
        else -> "video"
    }
    val data = JSONObject().apply {
        put("filePath", intentFilePath)
        put("fileName", intentFileName)
        put("mimeType", intentFileMimeType)
        put("isExternal", isExternalFile)
        put("mediaType", mediaType)
    }
    return data.toString()
}
```

**文件**：`MpvPlayerModule.kt`

添加 `@LynxMethod getMediaType()` 让 JS 可以查询当前媒体类型：

```kotlin
@LynxMethod
fun getMediaType(callback: Callback) {
    try {
        if (!mpvInitialized) { callback.invoke("video"); return }
        val videoWidth = MPVLib.getPropertyLong("width") ?: 0
        val videoHeight = MPVLib.getPropertyLong("height") ?: 0
        callback.invoke(if (videoWidth > 0 && videoHeight > 0) "video" else "audio")
    } catch (e: Exception) {
        callback.invoke("video")
    }
}
```

### Step 4：前端根据 mediaType 显示不同控件

**文件**：`AppComponent.tsx`

```tsx
interface InitData {
    filePath: string;
    fileName: string;
    mimeType: string;
    isExternal: boolean;
    mediaType: "video" | "audio";
}

const [mediaType, setMediaType] = useState<"video" | "audio">("video");

useEffect(() => {
    if (initData) {
        setFileName(initData.fileName || "Unknown");
        setMediaType(initData.mediaType || "video");
    }
}, [initData]);
```

**文件**：`PlayerControls.tsx`

拆分为 `VideoControls` 和 `AudioControls`：

- **VideoControls**：TopBar（返回+标题+全屏）+ CenterArea（播放/暂停）+ BottomBar（时间+进度条+时间）
- **AudioControls**：居中专辑封面占位 + 文件名 + BottomBar（时间+进度条+时间）+ 播放/暂停按钮，无全屏按钮

```tsx
export function PlayerControls({ ..., mediaType }: PlayerControlsProps) {
    // error, loading, idle 状态共用
    if (mediaType === 'audio') {
        return <AudioControls ... />;
    }
    return <VideoControls ... />;
}
```

AudioControls 布局：
```
┌─────────────────────┐
│      ✕  文件名       │  ← TopBar（无全屏按钮）
│                     │
│    ┌───────────┐    │
│    │  🎵 封面   │    │  ← 居中，大圆角矩形，渐变背景
│    └───────────┘    │
│                     │
│  0:00 ━━━━━━ 3:45   │  ← 进度条
│       ⏮  ▶  ⏭      │  ← 播放控制
└─────────────────────┘
```

VideoControls 布局（现有布局，不变）：
```
┌─────────────────────┐
│  ✕  文件名      ⤢   │  ← TopBar
│                     │
│       ⏸             │  ← CenterArea
│                     │
│  0:00 ━━━━━━ 3:45   │  ← BottomBar
└─────────────────────┘
```

### Step 5：音频模式隐藏 SurfaceView

**文件**：`MpvPlayerModule.kt`

当媒体类型为音频时，SurfaceView 不需要显示视频，但仍需要 mpv 引擎播放音频。在 `attachToLayout()` 中根据 mediaType 决定 SurfaceView 可见性：

更好的方案：在 `play()` 方法中，播放完成后检查 mpv 的 `width`/`height` 属性，如果为 0（纯音频），则隐藏 SurfaceView 并派发 `media_type` 事件。

```kotlin
// 在 eventObserver 的 MPV_EVENT_FILE_LOADED 中
MpvEvent.MPV_EVENT_FILE_LOADED -> {
    val videoWidth = MPVLib.getPropertyLong("width") ?: 0
    val videoHeight = MPVLib.getPropertyLong("height") ?: 0
    val isAudioOnly = videoWidth == 0 || videoHeight == 0
    if (isAudioOnly) {
        mpvSurfaceView?.visibility = View.GONE
        dispatchStateChange("audio_only")
    } else {
        mpvSurfaceView?.visibility = View.VISIBLE
        dispatchStateChange("playing")
    }
}
```

JS 端收到 `"audio_only"` 后设置 `mediaType = "audio"`。

### Step 6：CSS 样式更新

**文件**：`App.css`

添加音频模式相关样式：

```css
.AudioCoverContainer {
    flex: 1;
    justify-content: center;
    align-items: center;
    width: 100%;
}

.AudioCover {
    width: 200px;
    height: 200px;
    border-radius: 16px;
    background-color: rgba(74, 144, 217, 0.15);
    justify-content: center;
    align-items: center;
}

.AudioCoverIcon {
    font-size: 64px;
    color: rgba(255, 255, 255, 0.6);
}

.AudioPlayControls {
    flex-direction: row;
    justify-content: center;
    align-items: center;
    padding: 16px 0;
}
```

## 修改文件清单

| 文件 | 修改内容 |
|------|---------|
| `AppComponent.tsx` | 修复事件处理逻辑 + 添加 mediaType 状态 |
| `PlayerControls.tsx` | 拆分 VideoControls/AudioControls + 修复 loading 状态 |
| `App.css` | 添加音频模式样式 |
| `PlayerActivityLynx.kt` | buildInitDataJson 添加 mediaType |
| `MpvPlayerModule.kt` | 添加 audio_only 事件 + 隐藏 SurfaceView |

## 优先级

1. **Step 1-2**（核心修复）：修复 loading/error 状态逻辑，解决 UI 卡死
2. **Step 3-6**（功能增强）：区分视频/音频控件
