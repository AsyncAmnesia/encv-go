# 修复 Lynx 播放器渲染崩溃 + 线程安全问题

## 日志诊断结果

从 logcat.txt 中提取到 **3 类错误**（按严重程度排序）：

---

## 错误 1（致命）：`Attempt to render more than one <page />` — errCode 201

**日志证据**：
```
lynx_console.cc(252): "Error: Attempt to render more than one <page />, which is not supported."
LynxPlayerClient onReceivedError: code=201
LynxTemplateRender onErrorOccurred type -1,errCode:201
```

**根因**：[PlayerControls.tsx](app/encv-mobile/lynx-player/src/components/PlayerControls.tsx) 有 **3 个条件分支**各自返回了 `<page>` 元素：
| 行号 | 状态 | 问题代码 |
|------|------|----------|
| L44 | error | `<page className="ErrorContainer">` |
| L53 | loading | `<page className="CenterArea">` |
| L65 | idle | `<page className="CenterArea">` |

而 [AppComponent.tsx:152](app/encv-mobile/lynx-player/src/components/AppComponent.tsx#L152) 始终返回 `<page>` 作为根元素。

当 playerState 为 error/loading/idle 时 → **同时存在两个 `<page>`** → Lynx 引擎直接崩溃。

**修复方案**：将 PlayerControls.tsx 中所有 `<page>` 替换为 `<view>`。Lynx 要求整个应用只能有 **唯一一个** `<page>` 根节点。

---

## 错误 2（严重）：`setFullscreen failed: Only the original thread...Calling: Lynx_JS`

**日志证据**：
```
MpvPlayerModule setFullscreen failed: Only the original thread that created a view hierarchy
can touch its views. Expected: main Calling: Lynx_JS
```

**根因**：[MpvPlayerModule.kt](app/encv-mobile/android-overlay/app/src/main/java/com/encvgo/app/MpvPlayerModule.kt) 的以下 `@LynxMethod` 方法直接操作了 Android UI（Activity orientation、Window flags、View systemUiVisibility），但这些方法被 Lynx JS 引擎在 **后台线程（Lynx_JS）** 上调用：

| 方法 | 行号 | UI 操作 |
|------|------|---------|
| `setFullscreen()` | L222-251 | requestedOrientation, window.flags, systemUiVisibility |
| `finish()` | L271-280 | activity.finish() |
| `setOrientation()` | L254-268 | requestedOrientation |

**修复方案**：将上述方法中的所有 UI 操作包裹在 `mainHandler.post { }` 或 `activity?.runOnUiThread { }` 中。模块已有 `mainHandler = Handler(Looper.getMainLooper())`（L35），直接复用即可。

---

## 错误 3（非致命，无需修复）

| 日志 | 原因 | 处理 |
|------|------|------|
| `DevToolLifecycle.nativeSyncStateToNative` No implementation found | Release 包不含 devtool .so | 忽略 |
| `LynxEventReporter eventReporter service not found` | 未集成 Lynx 埋点 SDK | 忽略 |
| `js_cache_manager meta.json file_path is empty` | 未配置缓存目录 | 忽略 |
| `timing_map Set duplicated timing_key` | Lynx 内部时序重复 | 忽略 |
| `DestroyLayoutNodeBeforeRemoveFromParent` | 错误后的布局清理 | 随错误1修复消失 |
| `GPUAUX Null anb` | vivo 设备 GPU 特有 | 忽略 |
| `RecyclerView No adapter attached` | 错误后 RecyclerView 状态 | 随错误1修复消失 |

---

## 实施步骤

### Step 1：修复 PlayerControls.tsx 多 page 问题
- 文件：`app/encv-mobile/lynx-player/src/components/PlayerControls.tsx`
- 将 L44、L53、L65 三处 `<page>` 全部替换为 `<view>`
- 对应 CSS class 选择器无需改动（view 也支持 className）
- 重新构建 lynx bundle：`cd app/encv-mobile/lynx-player && npm run build`

### Step 2：修复 MpvPlayerModule.kt 线程安全
- 文件：`app/encv-mobile/android-overlay/app/src/main/java/com/encvgo/app/MpvPlayerModule.kt`
- `setFullscreen()`：将 L226-245 的 UI 操作包裹在 `mainHandler.post { }` 中，callback 在 post 内部调用
- `finish()`：将 L274 `activity?.finish()` 包裹在 `mainHandler.post { }` 中
- `setOrientation()`：将 L258-261 包裹在 `mainHandler.post { }` 中

### Step 3：验证
- 运行 `node --check` 验证 post-cap-sync.mjs 语法（上轮已修复 overlayJniDir 重复变量）
- 本地 Kotlin 类型检查：`cd app/encv-mobile && node scripts/check-kotlin.mjs`
