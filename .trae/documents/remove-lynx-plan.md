# 彻底移除 Lynx

## 当前状态分析

Lynx 在项目中有 **两套独立使用**：

### A. Lynx 前端播放器 UI（`lynx-player/` 目录）
- 基于 `@lynx-js/react` 构建的播放器界面 bundle（`player.lynx.bundle`）
- 通过 `rspeedy build` 编译输出到 Android assets
- 组件：PlayerApp、PlayerControls、ProgressBar

### B. Lynx Android SDK 集成（`android-overlay/` 目录中的 Kotlin 代码）
| 文件 | 角色 | 状态 |
|------|------|------|
| `EncvApplication.kt` | 初始化 Lynx SDK (Fresco/LynxEnv/LynxServiceCenter) | **核心入口** |
| `PlayerActivityLynx.kt` | 全屏 Lynx 播放器 Activity | 已被替代 |
| `PlayerOverlayManager.kt` | 在 MPV Surface 上叠加 LynxView 作为播放器 UI | 已被替代 |
| `PlayerTemplateProvider.kt` | 从 assets 加载 player.lynx.bundle | 仅被 OverlayManager 使用 |
| `PlayerBridgeModule.kt` | LynxModule 桥接 (playFile/isMpvAvailable) | 被 PlayerEntry 替代 |
| `MpvPlayerModule.kt` | LynxModule 控制 MPV | **文件自身标注 "MIGRATED to :plugin-mpv-player"** |
| `GoBackendModule.kt` | LynxModule Go 后端通信 | 仅被 Lynx UI 调用 |
| `LogBridgeModule.kt` | LynxModule 日志桥接 | 仅被 Lynx UI 调用 |
| `AndroidManifest.xml` | 注册 PlayerActivityLynx | 可移除 |
| `res/layout/lynx_player_activity.xml` | Lynx 播放器布局 | 可移除 |
| `proguard-rules.pro` | `-keep class com.lynx.**` | 可移除 |

### 关键发现：Lynx 已是死代码

当前播放路径（[PlayerEntry.kt](file:///workspace/app/encv-mobile/android-overlay/app/src/main/java/com/encvgo/app/PlayerEntry.kt)）：
```
PlayerEntry.play()
  → MPV 插件可用? → MpvPlayerActivity (ComboLite 插件, 独立进程)
  → 否则 → PlayerActivityCapacitor (WebView + ArtPlayer, Capacitor BridgeActivity)
```

**两条路径都不经过 Lynx。** Lynx 相关代码（PlayerActivityLynx、PlayerOverlayManager、所有 LynxModule）已无任何调用者。

---

## 移除计划

### 第 1 步：删除 `lynx-player/` 整个目录

```bash
rm -rf app/encv-mobile/lynx-player/
```

包含：
- `package.json` (@lynx-js/react 依赖)
- `lynx.config.ts`
- `src/player/` (PlayerApp.tsx, PlayerControls.tsx, ProgressBar.tsx, index.tsx, player.css)
- `src/typing.d.ts`

### 第 2 步：删除 android-overlay 中的 Lynx 文件

删除以下文件：
- `android-overlay/app/src/main/java/com/encvgo/app/PlayerActivityLynx.kt`
- `android-overlay/app/src/main/java/com/encvgo/app/PlayerOverlayManager.kt`
- `android-overlay/app/src/main/java/com/encvgo/app/PlayerTemplateProvider.kt`
- `android-overlay/app/src/main/java/com/encvgo/app/PlayerBridgeModule.kt`
- `android-overlay/app/src/main/java/com/encvgo/app/MpvPlayerModule.kt`
- `android-overlay/app/src/main/java/com/encvgo/app/GoBackendModule.kt`
- `android-overlay/app/src/main/java/com/encvgo/app/LogBridgeModule.kt`
- `android-overlay/app/src/main/res/layout/lynx_player_activity.xml`

### 第 3 步：修改 `EncvApplication.kt`

移除所有 Lynx 初始化代码：
- 删除 import: `com.lynx.service.devtool.LynxDevToolService`, `com.lynx.service.http.LynxHttpService`, `com.lynx.service.image.LynxImageService`, `com.lynx.service.log.LynxLogService`, `com.lynx.tasm.LynxEnv`, `com.lynx.tasm.service.LynxServiceCenter`, Fresco 相关
- 删除 `ensureLynxInitialized()` 方法及 `lynxInitialized` 标志
- 删除 `initLynxService()`, `initLynxEnv()` 方法
- 删除 `onCreate()` 中的 `ensureLynxInitialized(this)` 调用
- 删除 Fresco 依赖（如果仅被 Lynx 使用）

简化后 `EncvApplication.kt` 应该是一个极简的 Application 子类（可能只保留空壳或完全删除自定义逻辑）。

### 第 4 步：清理 `AndroidManifest.xml`

从 [AndroidManifest.xml](file:///workspace/app/encv-mobile/android-overlay/app/src/main/AndroidManifest.xml) 中移除：
```xml
<activity
    android:name=".PlayerActivityLynx"
    ... />
```

### 第 5 步：清理 `proguard-rules.pro`

移除：
```
# Keep Lynx SDK classes
-keep class com.lynx.** { *; }
-keep class org.lynxsdk.** { *; }
```

### 第 6 步：清理 Gradle 依赖

从 [libs.versions.toml](file:///workspace/app/encv-mobile/android/gradle/libs.versions.toml) 移除 Lynx 相关依赖声明。

从 [app/build.gradle.kts](file:///workspace/app/encv-mobile/android/app/build.gradle.kts) 移除 Lynx 依赖引用。

### 第 7 步：清理 CI workflow ([android.yml](file:///workspace/.github/workflows/android.yml))

移除：
1. **Cache npm dependencies (lynx-player)** 步骤（整个 cache block）
2. **Build Lynx player bundle** 步骤 (`cd app/encv-mobile/lynx-player && npm install && npm run build`)
3. **Verify APK contents** 中的 **Lynx bundle** 检查段
4. **Verify APK contents** 中的 **Lynx bundle in APK** 检查行

### 第 8 步：更新项目规则 ([project_rules.md](file:///workspace/.trae/rules/project_rules.md))

移除两个规则段落：
- **Lynx NativeModules 访问规则（重要！）**（第 39-50 行）
- **Lynx 全局事件监听规则（重要！）**（第 52-58 行）

### 第 9 步：检查是否有其他残留引用

全局搜索确认无遗漏的 `lynx` / `Lynx` / `LYNX` 引用。

---

## 不改动的部分

| 项目 | 原因 |
|------|------|
| `plugin-mpv-player/` | 独立插件模块，不依赖 Lynx |
| `PlayerEntry.kt` | 已走 Plugin/Capacitor 路径，不涉及 Lynx |
| `PlayerActivityCapacitor.kt` | WebView 播放器 fallback，不涉及 Lynx |
| `MainActivity.kt` | 主 Activity，不直接引用 Lynx |
| `capacitor.config.ts` | 无 Lynx 配置 |
| 主应用 `package.json` | 无 Lynx 依赖（只有 artplayer） |

## 预期收益

| 维度 | 变化 |
|------|------|
| APK 体积 | 减少 ~30MB（liblynx.so + liblynx_debug.so + libv8_libfull.cr.so + liblynx_*.so 共 ~35MB） |
| 构建时间 | 减少 lynx bundle 构建 (~20s) + npm install (~10s) |
| 代码复杂度 | 删除 ~8 个 Kotlin 文件 + ~6 个 TS/TSX 文件 |
| 内存占用 | 运行时不加载 Lynx/V8/Fresco |
