# 修复：播放页面 native crash（进程闪退）

## 问题分析

### 错误日志

logcat 中没有 Java 层 FATAL Exception，但有：
1. `libcrashpad_handler_trampoline.so` 触发 — native crash handler 启动
2. 进程重启（PID 20115 → PID 22406）— 旧进程被杀
3. Lynx `DestroyLayoutNodeBeforeRemoveFromParent` 错误 — 模板渲染中断

这是 **native crash**，进程直接被内核杀死，没有 Java 堆栈。

### 根因分析

**`MPVLib.create()` 在 JS 线程执行，但 mpv GPU 初始化需要 EGL 上下文，必须在主线程。**

调用链：
```
Lynx JS 线程: MpvPlayerModule.init{} 
  → mainHandler.post { attachToLayout(root) }  ← 切到主线程 ✓
    → ensureMpvInitialized() 
      → MPVLib.create(act)  ← 在主线程 ✓
      → MPVLib.init()       ← 在主线程 ✓
    → new MpvSurfaceView()  ← 在主线程 ✓
    → rootLayout.addView()  ← 在主线程 ✓
```

但 `tryAttachMpvModule()` 在 `onRuntimeReady()`/`onLoadSuccess()` 中调用：
```
Lynx 回调线程: onRuntimeReady() / onLoadSuccess()
  → tryAttachMpvModule()
    → mpvModule.attachToLayout(root)  ← 不在主线程！
      → ensureMpvInitialized()
        → MPVLib.create(act)  ← 不在主线程！native crash！
```

**`tryAttachMpvModule()` 没有用 `mainHandler.post` 包裹**，而 `attachToLayout` 内部调用了 `ensureMpvInitialized()` → `MPVLib.create()`，mpv 的 GPU 渲染初始化需要 EGL 上下文，只能在主线程执行。

### 次要问题：双重初始化风险

`init{}` 中的 `mainHandler.post` 和 `tryAttachMpvModule()` 可能同时尝试 `attachToLayout`，导致：
- `MPVLib.create()` 被调用两次
- 两个 `MpvSurfaceView` 被添加到同一个 rootLayout

---

## 修复方案

### Step 1：tryAttachMpvModule() 用 runOnUiThread 包裹

```kotlin
private fun tryAttachMpvModule() {
    val mpvModule = MpvPlayerModule.getInstance()
    if (mpvModule == null) {
        LogRelay.get().relay(TAG, "info", "tryAttachMpvModule: mpvModule not yet created")
        return
    }
    if (mpvModule.isAttached()) {
        LogRelay.get().relay(TAG, "info", "tryAttachMpvModule: already attached")
        return
    }
    val root = findViewById<android.widget.FrameLayout>(R.id.lynx_player_root)
    if (root == null) {
        LogRelay.get().relay(TAG, "warn", "tryAttachMpvModule: lynx_player_root not found")
        return
    }
    runOnUiThread {
        if (!mpvModule.isAttached()) {
            mpvModule.attachToLayout(root)
        }
    }
}
```

### Step 2：attachToLayout 添加防重入检查

```kotlin
fun attachToLayout(rootLayout: ViewGroup) {
    if (mpvSurfaceView != null && mpvSurfaceView?.parent != null) {
        LogRelay.get().relay(TAG, "info", "attachToLayout: already attached, skipping")
        return
    }
    // ... 原有逻辑
}
```

### Step 3：ensureMpvInitialized 添加线程检查

```kotlin
private fun ensureMpvInitialized() {
    if (mpvInitialized) return
    if (Looper.myLooper() != Looper.getMainLooper()) {
        LogRelay.get().relay(TAG, "error", "ensureMpvInitialized: not on main thread! thread=${Thread.currentThread().name}")
        return
    }
    // ... 原有逻辑
}
```

---

## 修改文件

1. `/workspace/app/encv-mobile/android/app/src/main/java/com/encvgo/app/PlayerActivityLynx.kt`
   - `tryAttachMpvModule()` 改为 `runOnUiThread` 包裹

2. `/workspace/app/encv-mobile/android/app/src/main/java/com/encvgo/app/MpvPlayerModule.kt`
   - `attachToLayout()` 添加防重入检查
   - `ensureMpvInitialized()` 添加主线程检查
