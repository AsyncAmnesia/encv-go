# MPV 播放器 ComboLite 插件化 — 收尾实施计划

## 概述

基于已完成的 Phase 1-4 基础设施，本计划覆盖 4 项收尾工作：
1. CI 工作流适配（MPV 插件构建集成到 Android CI）
2. 主应用首页增加插件管理入口 + 二级页面
3. 设置页播放方式适配（MPV 插件选项）
4. 剩余 TODO 实现（MpvEngine 接入、流地址解析、全屏、安全区域）

---

## Task 1: CI 工作流适配

### 现状分析

当前 [android.yml](file:///workspace/.github/workflows/android.yml) 关键步骤：
- L116-117: `setup-mpv-libs.sh` — 下载 mpv .so 到 **旧路径** `android-overlay/app/src/main/jniLibs/`
- L119-120: `build-ffmpeg-android.sh` — 编译 Go 后端 FFmpeg
- L182-187: 复制 libencv-go.so 到 `android/app/src/main/jniLibs/arm64-v8a/`
- L189-218: **Verify native libraries** — 检查 `libmpv.so`, `libplayer.so`, FFmpeg 系列 .so, `player.lynx.bundle`
- L246: `./gradlew assembleDebug`
- L272: **APK 验证** — 检查 APK 中包含 mpv/ffmpeg .so

### 需要修改的内容

#### 1.1 修改 Verify native libraries 步骤

**文件**: `.github/workflows/android.yml` (L189-218)

当前逻辑检查 `android/app/src/main/jniLibs/arm64-v8a/` 中的 `.so` 文件。改造后：

```yaml
# === Verify native libraries ===
- name: Verify native libraries
  run: |
    echo "=== Host jniLibs contents (Go backend only) ==="
    HOST_LIBS="app/encv-mobile/android/app/src/main/jniLibs/arm64-v8a"
    echo "=== Checking Go backend libraries ==="
    for lib in libencv-go.so libffmpeg.so libffprobe.so; do
      if [ -f "$HOST_LIBS/$lib" ]; then
        echo "✅ $lib found ($(ls -lh "$HOST_LIBS/$lib" | awk '{print $5}'))"
      else
        echo "⚠️ $lib not found (may be optional)"
      fi
    done

    echo "=== Plugin jniLibs contents (MPV player) ==="
    PLUGIN_LIBS="app/encv-mobile/plugin-mpv-player/src/main/jniLibs/arm64-v8a"
    if [ -d "$PLUGIN_LIBS" ]; then
      echo "Plugin libs:"
      ls -lh "$PLUGIN_LIBS/"*.so 2>/dev/null || echo "(no .so files)"
      echo "=== Checking MPV plugin libraries ==="
      for lib in libmpv.so libplayer.so; do
        if [ -f "$PLUGIN_LIBS/$lib" ]; then
          echo "✅ $lib found ($(ls -lh "$PLUGIN_LIBS/$lib" | awk '{print $5}'))"
        else
          echo "❌ $lib MISSING in plugin module!"
        fi
      done
      # Check FFmpeg .so files that MPV depends on
      for lib in libavcodec.so libavformat.so libavutil.so libswresample.so libswscale.so; do
        if [ -f "$PLUGIN_LIBS/$lib" ]; then
          echo "✅ $lib found ($(ls -lh "$PLUGIN_LIBS/$lib" | awk '{print $5}'))"
        else
          echo "❌ $lib MISSING in plugin module!"
        fi
      done
    else
      echo "⚠️ Plugin jniLibs directory not found (MPV plugin may not be built yet)"
    fi
```

关键变化：
- **Host APK 不再包含** `libmpv.so`, `libplayer.so`, FFmpeg 系列 .so（它们在插件模块中）
- Host APK 只需包含 `libencv-go.so`（Go 后端）+ `libffmpeg.so`/`libffprobe.so`（Go 后端 FFmpeg）
- 新增对插件模块 jniLibs 目录的检查

#### 1.2 修改 APK 验证步骤

**文件**: `.github/workflows/android.yml` (L249-277)

```
# 当前：检查 APK 包含 libmpv/libplayer 等
# 改为：Host APK 不应包含 libmpv/libplayer，但应包含 libencv-go
echo "=== Go binary in APK (jniLibs) ==="
unzip -l "$APK_PATH" | grep -E "libencv-go|lib/arm64" || echo "❌ libencv-go.so NOT in APK!"
echo "=== Host APK should NOT contain MPV .so ==="
if unzip -l "$APK_PATH" | grep -q "libmpv\.so\|libplayer\.so"; then
  echo "⚠️ WARNING: Host APK still contains MPV .so (should be in plugin APK only)"
else
  echo "✅ Host APK correctly excludes MPV .so"
fi
echo "=== Go backend FFmpeg in APK ==="
unzip -l "$APK_PATH" | grep -E "libffmpeg\.so|libffprobe\.so" || echo "⚠️ Go backend FFmpeg .so not in APK"
```

#### 1.3 可选：添加插件 APK 构建步骤

在 Host APK 构建成功后，可选地添加一步验证插件模块编译：

```yaml
# After assembleDebug succeeds:
- name: Build MPV plugin module (verification)
  run: |
    cd app/encv-mobile/android
    ./gradlew :plugin-mpv-player:compileDebugKotlin --stacktrace 2>&1 || echo "⚠️ Plugin module compile skipped (may need NDK)"
  continue-on-error: true
```

> **注意**: 完整的插件 APK 打包（aar2apk）需要 ComboLite Gradle 插件完整配置，包括签名等。CI 初期可以先验证编译通过，后续再启用完整打包。

---

## Task 2: 主应用首页插件管理入口 + 二级页面

### 现状分析

**首页** ([HomePage.vue](file:///workspace/app/encv-mobile/src/views/HomePage.vue)):
- 2×2 grid 卡片布局：Player / Files / Tasks / Remote
- Player 卡片横跨两列（grid-column: 1/-1），是主要入口
- 无插件管理入口

**路由** ([router/index.ts](file:///workspace/app/encv-mobile/src/router/index.ts)):
- 已有 `/tabs/settings/plugins` 路由指向 `PluginSettings.vue`
- 但 PluginSettings.vue 目前只显示 `plugin_settings` 配置字段（Go 后端的插件配置），不显示 ComboLite 插件状态

**Tab 栏** ([Tabs.vue](file:///workspace/app/encv-mobile/src/views/Tabs.vue)):
- 6 个 tab：Home / Files / Tasks / Remote / Settings / DevLogs
- 无独立 Plugins tab（合理，插件管理放在 Settings 下）

### 方案：在 HomePage 新增"插件管理"卡片 + 改造 PluginSettings.vue

#### 2.1 修改 HomePage.vue — 新增插件管理卡片

**文件**: `/workspace/app/encv-mobile/src/views/HomePage.vue`

在现有 4 个卡片后新增第 5 个卡片：

```html
<div class="home-card" @click="handleOpenPlugins">
  <ion-icon :icon="puzzleOutline" class="card-icon plugins-icon"></ion-icon>
  <div class="card-info">
    <h3>{{ t('home.plugins') }}</h3>
    <p>{{ t('home.pluginsDesc') }}</p>
  </div>
</div>
```

script 增加：
```typescript
import { puzzleOutline } from 'ionicons/icons'

function handleOpenPlugins() {
  router.push('/tabs/settings/plugins')
}
```

style 增加 plugins-icon 颜色（紫色系，区别于其他卡片）。

#### 2.2 改造 PluginSettings.vue — 增加 ComboLite 插件管理 UI

**文件**: `/workspace/app/encv-mobile/src/views/PluginSettings.vue`

当前页面只显示 Go 后端 `plugin_settings` 配置字段。需要在 `<ion-list v-if="isNative()">` (L153) 的空列表内填充 ComboLite 插件管理功能：

```html
<ion-list v-if="isNative()">
  <ion-list-header>
    <ion-label>{{ t('plugins.comboLitePlugins') }}</ion-label>
  </ion-list-header>

  <!-- MPV Player Plugin Card -->
  <ion-card>
    <ion-card-header>
      <ion-card-title>
        <ion-icon :icon="filmOutline" slot="start"></ion-icon>
        {{ t('plugins.mpvPlayer') }}
      </ion-card-title>
      <ion-badge v-if="mpvInstalled" color="success" slot="end">{{ t('plugins.installed') }}</ion-badge>
      <ion-badge v-else color="medium" slot="end">{{ t('plugins.notInstalled') }}</ion-badge>
    </ion-card-header>
    <ion-card-content>
      <p>{{ t('plugins.mpvPlayerDesc') }}</p>
      <ion-item v-if="mpvInstalled" lines="none">
        <ion-icon :icon="informationCircle" slot="start" color="primary"></ion-icon>
        <ion-label>
          {{ t('plugins.mpvSizeHint', { size: mpvSizeDisplay }) }}
        </ion-label>
      </ion-item>
    </ion-card-content>
    <ion-card-footer>
      <ion-button v-if="mpvInstalled && !mpvEnabled" fill="outline" size="small" @click="enableMpvPlugin">
        {{ t('plugins.enable') }}
      </ion-button>
      <ion-button v-if="mpvInstalled && mpvEnabled" fill="clear" color="danger" size="small" @click="disableMpvPlugin">
        {{ t('plugins.disable') }}
      </ion-button>
      <ion-button v-if="!mpvInstalled" fill="outline" size="small" disabled>
        {{ t('plugins.downloadPending') }}
      </ion-button>
    </ion-card-footer>
  </ion-card>

  <!-- Future: more plugin cards here -->
</ion-list>
```

script 增加插件状态管理逻辑：
```typescript
import { filmOutline, informationCircle } from 'ionicons/icons'

const mpvInstalled = ref(false)
const mpvEnabled = ref(false)
const mpvSizeDisplay = ref('-- MB')

async function loadPluginStatus() {
  // 通过 PlayerBridgeModule.isMpvAvailable() 或直接调 NativeModules
  // 查询 ComboLite PluginManager 获取插件状态
}

async function enableMpvPlugin() { /* ... */ }
async function disableMpvPlugin() { /* ... */ }

onMounted(async () => {
  // ...existing code...
  if (isNative()) {
    await loadPluginStatus()
  }
})
```

#### 2.3 注册 PlayerBridgeModule 到 LynxViewBuilder

**需要找到 LynxViewBuilder 注册位置**（搜索 `registerModule` 在 Kotlin 代码中的调用点），添加：
```kotlin
viewBuilder.registerModule(PlayerBridgeModule::class.java)
```

#### 2.4 更新 typing.d.ts

**文件**: `app/encv-mobile/lynx-player/src/typing.d.ts` 或 `app/encv-mobile/src/typing.d.ts`

```typescript
declare let NativeModules: {
  // ...existing modules...
  PlayerBridgeModule: {
    playFile: (filePath: string, fileName: string, mimeType: string) => Promise<boolean>
    playFileExternal: (filePath: string, fileName: string, mimeType: string) => Promise<boolean>
    isMpvAvailable: () => Promise<boolean>
  }
}
```

#### 2.5 添加 i18n 键值

**文件**: `app/encv-mobile/src/composables/useI18n.ts`

中文：
```
'home.plugins': '插件管理',
'home.pluginsDesc': '管理和配置播放器等功能插件',
'plugins.comboLitePlugins': 'ComboLite 插件',
'plugins.mpvPlayer': 'MPV 播放器',
'plugins.mpvPlayerDesc': '高性能原生视频/音频播放器，支持 MKV/FLV/ASS 字幕等格式',
'plugins.installed': '已安装',
'plugins.notInstalled': '未安装',
'plugins.enable': '启用',
'plugins.disable': '禁用',
'plugins.downloadPending': '等待下载',
'plugins.mpvSizeHint': '约 {size}，安装后将增加 APK 体积',
```

英文对应翻译。

---

## Task 3: 设置页播放方式适配

### 现状分析

[Settings.vue](file:///workspace/app/encv-mobile/src/views/Settings.vue) 已有播放方式设置（L33-81）：

**视频播放** (`videoPlayerMode`, localStorage key `encv_player_video`):
- `artplayer` — 内置 Artplayer（默认）
- `mpv` — 内置 MPV
- `external` — 外部打开

**音频播放** (`audioPlayerMode`, localStorage key `encv_player_audio`):
- `mpv` — 内置 MPV（默认）
- `external` — 外部打开

### 问题

当前选 `mpv` 时走的是旧的 `PlayerActivityLynx`（LynxView + MpvPlayerModule）。改造后需要：
1. 选 `mpv` → 走 `PlayerEntry.play()` → 自动检测 ComboLite 插件
2. 如果插件未安装 → 降级提示或自动切换到 artplayer

### 方案

#### 3.1 修改 Settings.vue 播放选项文案和逻辑

**文件**: `/workspace/app/encv-mobile/src/views/Settings.vue`

**视频播放选项调整**：
```html
<ion-select-option value="artplayer">{{ t('settings.builtInArtplayer') }}</ion-select-option>
<ion-select-option value="mpv-plugin">{{ t('settings.builtInMpvPlugin') }}</ion-select-option>
<ion-select-option value="external">{{ t('settings.openExternal') }}</ion-select-option>
```

注意：将 `value="mpv"` 改为 `value="mpv-plugin"` 以区分新旧路径。

**localStorage key 保持不变**（向后兼容），但默认值可改为 `'artplayer'`（因为 MPV 作为插件可能未安装）。

#### 3.2 PlayerEntry 适配 videoPlayerMode

**文件**: `/workspace/app/encv-mobile/android/app/src/main/java/com/encvgo/app/PlayerEntry.kt`

`play()` 方法需要读取用户选择的播放模式：

```kotlin
fun play(context: Context, filePath: String, fileName: String, mimeType: String, isExternal: Boolean = false) {
    val prefs = context.getSharedPreferences("encv_player_prefs", Context.MODE_PRIVATE)
    val videoMode = prefs.getString("video_player", "artplayer") ?: "artplayer"

    when (videoMode) {
        "mpv-plugin" -> {
            val pluginManager = PluginManager.getInstance(context)
            val mpvPlugin = pluginManager.getInstalledPlugin("mpv-player")
            if (mpvPlugin != null && mpvPlugin.enabled) {
                startMpvPlayer(context, filePath, fileName, mimeType, isExternal, pluginManager, mpvPlugin)
            } else {
                showMpvNotAvailableToast(context)
                startArtPlayer(context, filePath, fileName)
            }
        }
        "external" -> openExternal(context, filePath)
        else -> startArtPlayer(context, filePath, fileName) // default: artplayer
    }
}
```

#### 3.3 Lynx 前端播放入口适配

当用户选择文件点击播放时，前端通过 `NativeModules.PlayerBridgeModule.playFile()` 调用。PlayerBridgeModule 内部委托给 `PlayerEntry.play()`，PlayerEntry 根据用户设置自动路由。

**无需修改前端播放逻辑**，只需确保 PlayerBridgeModule 正确注册。

---

## Task 4: 完成剩余 TODO 工作

### 4.1 TODO #1: MpvPlayerActivity.createMpvEngine()

**文件**: [MpvPlayerActivity.kt:54-56](file:///workspace/app/encv-mobile/plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvPlayerActivity.kt#L54-L56)

**当前**:
```kotlin
private fun createMpvEngine(): MpvEngine {
    TODO("Phase 3.1: Create and return concrete MpvEngine implementation wrapping MPVLib")
}
```

**改为**:
```kotlin
private fun createMpvEngine(): MpvEngine {
    return MpvEngine(this).also { engine ->
        engine.eventListener = { event ->
            when (event) {
                is MpvEngine.Event.Pause -> { }
                is MpvEngine.Event.Unpause -> { }
                is MpvEngine.Event.EndFile -> { finish() }
                is MpvEngine.Event.Shutdown -> { finish() }
                else -> { }
            }
        }
        engine.stateListener = { state ->
            when (state) {
                is MpvEngine.State.MpvReady -> {
                    engine.attachSurfaceView()
                }
                is MpvEngine.State.Error -> { }
                else -> { }
            }
        }
    }
}
```

同时需要在 Activity 中处理 SurfaceView。MpvEngine 内部已有 `attachSurfaceView()` 方法创建 SurfaceView，Compose 层需要通过 `AndroidView` 将其嵌入：

在 `MpvPlayerScreen.kt` 中增加 SurfaceView 占位参数：
```kotlin
@Composable
fun MpvPlayerScreen(
    // ...existing params...
    onSurfaceViewReady: (android.view.SurfaceView) -> Unit = {},  // NEW
)
```

在 `MpvControls.kt` 的 VideoPlaybackLayout 中：
```kotlin
AndroidView(
    factory = { context ->
        android.view.SurfaceView(context).also { sv ->
            onSurfaceViewReady(sv)
        }
    },
    modifier = Modifier.fillMaxSize()
)
```

### 4.2 TODO #2: resolveStreamUrl()

**文件**: [MpvPlayerScreen.kt:244-246](file:///workspace/app/encv-mobile/plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvPlayerScreen.kt#L244-L246)

**当前**: 返回空字符串

**改为**: 通过 HTTP 请求 Go 后端获取流地址

```kotlin
private suspend fun resolveStreamUrl(filePath: String, isExternal: Boolean): String {
    return try {
        // 从 Intent 或配置获取 Go 后端地址
        val backendUrl = getBackendBaseUrl()
        if (backendUrl.isEmpty()) {
            if (isExternal && filePath.startsWith("/")) {
                return filePath  // 直接使用外部文件路径
            }
            return ""
        }

        val encodedPath = java.net.URLEncoder.encode(filePath, "UTF-8")
        val url = if (isExternal) {
            "$backendUrl/api/stream/external?path=$encodedPath"
        } else {
            "$backendUrl/stream?path=$encodedPath"
        }

        // HEAD 请求验证 URL 可达性（轻量）
        java.net.URL(url).openConnection().let { conn ->
            conn.requestMethod = "HEAD"
            conn.connectTimeout = 5000
            conn.readTimeout = 5000
            conn.responseCode
            url  // 返回 URL 给 MPV 加载
        }
    } catch (e: Exception) {
        ""
    }
}

private fun getBackendBaseUrl(): String {
    // 方案 A: 从 Intent extra 获取（宿主传入）
    // 方案 B: 从 SharedPreferences 读取（与前端共享配置）
    // 方案 C: 默认 localhost:port
    return intent.getStringExtra("backend_url") ?: ""
}
```

**配套修改**: `PlayerEntry.startMpvPlayer()` 需要在 Bundle 中额外传入 `backend_url`：
```kotlin
bundle.putString("backend_url", getBackendUrlFromConfig(context))
```

### 4.3 TODO #3: 全屏切换

**文件**: [MpvPlayerScreen.kt:181-183](file:///workspace/app/encv-mobile/plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvPlayerScreen.kt#L181-L183)

**当前**: TODO 注释

**改为**:
```kotlin
onToggleFullscreen = {
    isFullscreen = !isFullscreen
    val activity = context as? android.app.Activity ?: return@MpvControls
    if (isFullscreen) {
        activity.requestedOrientation = android.content.pm.ActivityInfo.SCREEN_ORIENTATION_LANDSCAPE
        hideSystemUi(activity)
    } else {
        activity.requestedOrientation = android.content.pm.ActivityInfo.SCREEN_ORIENTATION_PORTRAIT
        showSystemUi(activity)
    }
    showControls = true
},
```

辅助函数：
```kotlin
private fun hideSystemUi(activity: android.app.Activity) {
    android.view.WindowCompat.setDecorFitsSystemWindows(activity.window, false)
    android.view.WindowInsetsControllerCompat(activity.window, activity.window.decorView).let { controller ->
        controller.hide(android.view.WindowInsetsCompat.Type.systemBars())
        controller.systemBarsBehavior = android.view.WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
    }
}

private fun showSystemUi(activity: android.app.Activity) {
    android.view.WindowCompat.setDecorFitsSystemWindows(activity.window, true)
    activity.window.decorView.apply {
        systemUiVisibility = systemUiVisibility and
            (android.view.View.SYSTEM_UI_FLAG_FULLSCREEN or
             android.view.View.SYSTEM_UI_FLAG_HIDE_NAVIGATION).inv()
    }
}
```

### 4.4 TODO #4: WindowInsetsSafeTop/Bottom

**文件**: [MpvPlayerScreen.kt:249-252](file:///workspace/app/encv-mobile/plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvPlayerScreen.kt#L249-L252)

**当前**: 返回硬编码 0

**改为** 使用 Compose `WindowInsets`:

```kotlin
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.systemBars

// 删除 WindowInsetsSafeTop()/WindowInsetsSafeBottom() 函数
// 在 MpvControls 的外层 Box 上使用:
Box(
    modifier = Modifier
        .fillMaxSize()
        .windowInsetsPadding(WindowInsets.systemBars)
        // ...
)
```

或者如果需要精确的 px 值给原生 View 定位：
```kotlin
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalConfiguration

val windowInsets = WindowInsets.systemBars
val density = LocalDensity.current
val topInset = with(density) { windowInsets.getTop(density) }
val bottomInset = with(density) { windowInsets.getBottom(density) }
```

---

## 任务依赖关系

```
Task 1 (CI) ──────────────────────────────┐
                                       │ (并行)
Task 2 (首页+插件页) ───────────────────┤
                                       │
Task 3 (设置页适配) ──────────────────┤
                                       │
Task 4 (剩余TODO) ─────────────────────┘
  ├── 4.1 createMpvEngine()     ← 独立
  ├── 4.2 resolveStreamUrl()    ← 独立（但需 Task 3 的 backend_url 传递链路配合）
  ├── 4.3 全屏切换             ← 独立
  └── 4.4 WindowInsets         ← 独立
```

Task 1-3 可以完全并行实施。Task 4 的 4 个子任务也可以并行。
