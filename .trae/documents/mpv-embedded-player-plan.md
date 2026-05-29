# MPV 播放器嵌入式播放方案

## 现状问题

### 当前播放方式对比

| 播放器 | 技术实现 | 用户体验 | 异常处理 |
|--------|---------|---------|---------|
| **Artplayer (内置)** | Vue 组件嵌入 `/player` 路由页面 | ✅ 无跳转，嵌入当前页面 | ✅ 错误直接在页面内展示 |
| **MPV Plugin (外部)** | `startActivity(EncvHostActivity)` → 新 Activity → ProxyManager 代理 | ❌ 页面跳转 + 白屏 | ❌ 白屏无反馈 |
| **External** | `Intent.ACTION_VIEW` 外部应用 | ❌ 跳转出应用 | N/A |

### 用户期望

> "之前就实现过直接嵌入页面播放，用户没有跳转感觉"

MPV 应该像 Artplayer 一样：**点击视频 → 在当前页面/WebView 内显示播放器**，不启动新 Activity。

## 方案对比

### 方案 A: ComposeView 原生嵌入（推荐 ⭐）

**原理**：在 Capacitor BridgeActivity 的 WebView 上层叠加一个原生 ComposeView，渲染 MPV 播放器 UI。

```
BridgeActivity (Capacitor)
├── WebView (Ionic/Vue 文件列表)
│   └── Files.vue
│       └── 点击视频 → 调用 startMpvInPlace()
└── [新增] ComposeView (MPV 播放器) ← 叠加在 WebView 上层
    └── MpvPlayerScreen (Compose UI)
```

**优点**：
- ✅ 完全无跳转，用户体验与 Artplayer 一致
- ✅ 错误可直接回传给前端（通过 JS bridge callback）
- ✅ 不依赖 ProxyManager Activity 代理机制
- ✅ 失败时可立即显示错误 banner（不白屏）

**缺点**：
- 需要修改 EncvApplication 或 GoProcessPlugin 暴露新 API
- 需要处理 ComposeView 与 WebView 的层级关系（触摸事件、返回键）
- MPV 插件的 Compose UI 需要脱离 ComboLite 的 Activity 生命周期独立运行
- 复杂度较高

**改动范围**：
- Kotlin: 新增 `MpvEmbedService` / 在 `GoProcessPlugin` 暴露 `startMpvInPlace()` @PluginMethod
- Kotlin: 创建 ComposeView 并附加到当前 Activity
- 前端: Files.vue 调用新 API，接收回调结果

---

### 方案 B: Fragment 嵌入

**原理**：把 EncvHostActivity 改为 Fragment，在当前页面的容器中加载。

```
Files.vue
├── <div ref="mpvContainer" v-if="showMpvPlayer">
│   └── [FragmentContainerView] ← Android 原生 Fragment 容器
│       └── MpvPlayerFragment (继承 BaseHostActivity 改造)
│           └── ProxyManager 代理 MpvPlayerActivity
```

**优点**：
- ✅ 视觉上嵌入页面
- ✅ 复用现有 ProxyManager 机制

**缺点**：
- ❌ ComboLite 的 `BaseHostActivity` 是 **Activity** 不是 Fragment，改造风险大
- ❌ ProxyManager 设计为 Activity 代理，不支持 Fragment 场景
- ❌ 需要 AndroidManifest 大改（声明 Fragment 容器 Activity）
- ❌ ComboLite 升级可能破坏兼容性

**结论**：不推荐，ComboLite 架构不支持。

---

### 方案 C: 透明 Activity + startActivityForResult（快速修复 ⚡）

**原理**：EncvHostActivity 使用透明主题，视觉上无跳转。失败时 setResult 回传错误。

```
Files.vue (WebView)
    ↓ 点击视频
    ↓ startActivityForResult(EncvHostActivity, REQUEST_MPV)
    ↓
[透明] EncvHostActivity (用户看不到跳转)
    ├── 成功: 显示 MPV 播放器全屏
    └── 失败: setResult(error) + finish()
        ↓
Files.vue.onActivityResult()
    ↓
playError.value = error  ← 显示红色 banner
```

**优点**：
- ✅ 改动最小（只改 PlayerEntry + GoProcessPlugin + Files.vue）
- ✅ 不白屏（失败时有明确错误反馈）
- ✅ 视觉上接近"嵌入"（透明 Activity 无感知）

**缺点**：
- ⚠️ 仍然是 Activity 跳转（只是透明的）
- ⚠️ 用户按返回键会回到 EncvHostActivity 而非 Files.vue
- ⚠️ 生命周期管理复杂（Activity 栈多一层）

**改动范围**：
- Kotlin: PlayerEntry.startMpvPlayer() 改为 `activity.startActivityForResult()`
- Kotlin: EncvHostActivity 失败时 `setResult()` 返回错误
- Kotlin: GoProcessPlugin.openPlayer() 用 `startActivityForResult` 包装
- 前端: Files.vue 用 `@capacitor/android` 的 result 监听

---

### 方案 D: WebView 内 iframe 嵌入

**原理**：MPV 插件提供 Web 接口（类似 Artplayer），在 WebView 内用 iframe 加载。

**缺点**：
- ❌ MPV 是原生 Compose UI，不是 Web 组件
- ❌ 无法提供 Web 接口（除非重写整个播放器）
- ❌ 性能差（视频解码不能在 WebView 内完成）

**结论**：不可行。

## 推荐：方案 A (ComposeView 嵌入) 作为目标，方案 C (透明 Activity) 作为过渡

### 分阶段实施

#### Phase 1（本次）：方案 C — 透明 Activity + 错误回传

**目标**：解决白屏问题，确保异常时有明确错误展示。

**步骤**：
1. EncvHostActivity 设置透明主题 (`android:theme="@style/Theme.AppCompat.Translucent.NoTitleBar"`)
2. PlayerEntry.startMpvPlayer() 改用 `startActivityForResult()`
3. EncvHostActivity 失败时 `setResult(RESULT_OK, Intent with error)`
4. GoProcessPlugin.openPlayer() 用 `registerActivityResultHandler()` 接收结果
5. GoProcess.ts openPlayer() 返回 Promise<PlayResult>（包含 Activity 结果）
6. Files.vue playMedia() 处理失败结果，显示错误 banner

**验收标准**：
- MPV 未安装/未加载/ProxyManager 失败 → Files.vue 顶部显示红色错误 banner
- 不再出现白屏
- 日志输出完整（EncvHostActivity onCreate/onResume/onDestroy）

#### Phase 2（后续）：方案 A — ComposeView 原生嵌入

**目标**：彻底消除 Activity 跳转，实现真正的嵌入式播放。

**前提**：
- Phase 1 已验证 EncvHostActivity 内部的所有失败场景可被捕获
- ComboLite API 确认支持非 Activity 方式使用插件 UI

**步骤**：
1. 新建 `MpvEmbedService.kt` — 管理 ComposeView 生命周期
2. GoProcessPlugin 新增 `startMpvInPlace()` @PluginMethod
3. 在当前 BridgeActivity 上创建 ComposeView 并渲染 MpvPlayerScreen
4. 处理触摸事件穿透（ComposeView ↔ WebView）
5. 处理返回键（关闭 MPV 播放器回到文件列表）
6. 前端调用新 API，传入 container 元素位置

## Phase 1 详细实施计划

### Task 1: EncvHostActivity 透明主题 + setResult

**文件**: `android/app/src/main/java/com/encvgo/app/EncvHostActivity.kt`

```kotlin
// 当前已有饱和调试日志，需补充：
// 1. 透明主题（AndroidManifest 中设置或代码中动态设置）
// 2. ProxyManager 启动失败时 finishWithResult
// 3. onResume 中检测到白屏时 finishWithResult
```

### Task 2: PlayerEntry.startMpvPlayer() 改用 startActivityForResult

**文件**: `android/app/src/main/java/com/encvgo/app/PlayerEntry.kt`

```kotlin
// 问题：context.startActivity() 无法接收结果
// 解决：需要 Activity context 才能调用 startActivityForResult
// 方案：PlayerEntry.play() 接收 Activity 参数，或使用 GlobalScope + ResultReceiver
```

### Task 3: GoProcessPlugin.openPlayer() 用 registerActivityResultHandler

**文件**: `android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt`

```kotlin
// Capacitor 提供 registerActivityResultHandler() API
// 可以在不改变前端调用方式的情况下接收 Activity 结果
```

### Task 4: Files.vue 处理 MPV 播放结果

**文件**: `src/views/Files.vue`

```typescript
// openPlayer() 返回 PlayResult
// 如果 success=false 且来自 ActivityResult（而非预检），显示错误 banner
// 区分"预检失败"和"运行时失败"
```

### Task 5: 验证

- [ ] MPV 未安装 → 显示 "MPV 未安装" banner
- [ ] MPV 已安装但未加载 → 尝试加载 → 成功则播放，失败则显示 banner
- [ ] MPV 加载成功但 ProxyManager 失败 → 显示 "播放器内部错误" banner
- [ ] MPV 正常播放 → 全屏显示（透明 Activity 无感知）
- [ ] Logcat 过滤 EncvHostActivity 输出完整诊断信息

## 铁律合规

1. **严禁 Toast** → 所有错误通过 banner 显示 ✓
2. **严禁 fallback** → 失败后不自动切换到 Artplayer ✓
3. **严禁白屏** → 所有失败路径都有明确的 UI 反馈 ✓（Phase 1 目标）
4. **饱和调试** → EncvHostActivity + PlayerEntry 完整日志输出 ✓