# 修复 chrome-error 导航错误计划

## 错误现象

```
Not allowed to load local resource: chrome-error://chromewebdata/#/standalone/player
```

## 根因分析

### 调用链路

```
PlayerActivity.onCreate()
  └─ super.onCreate(savedInstanceState)     ← BridgeActivity 开始加载 WebView
      └─ BridgeActivity 内部加载 URL       ← capacitor.config.server.androidScheme='https'
          └─ 此时后端可能还没启动！
              └─ WebView 加载失败 → 显示 chrome-error://chromewebdata/ 错误页
  └─ setupWebViewNavigation()             ← 包装 WebViewClient
  └─ ...（等待 onPageFinished）
      └─ onPageFinished(view, "chrome-error://chromewebdata/")  ← ❌ 在错误页上触发了！
          └─ evaluateJavascript("window.location.hash='#/standalone/player'")
              └─ 浏览器尝试导航到 chrome-error://chromewebdata/#/standalone/player
                  └─ 🔴 Not allowed to load local resource
```

### 核心问题：`onPageFinished` 不区分正常页面和错误页面

当前代码 [PlayerActivity.kt L94-L109](file:///workspace/app/encv-mobile/android-overlay/app/src/main/java/com/encvgo/app/PlayerActivity.kt#L94-L109)：

```kotlin
override fun onPageFinished(view: WebView?, url: String?) {
    originalClient?.onPageFinished(view, url)
    if (!navigatedToPlayer) {        // ❌ 没有检查 url 是否为有效页面！
        navigatedToPlayer = true
        // 直接在可能的错误页上执行 JS...
    }
}
```

**三个时序场景**：

| 场景 | 后端状态 | WebView 行为 | 当前行为 |
|------|---------|-------------|---------|
| A: 后端已运行 | isRunning=true | 正常加载 SPA 页面 ✅ | 正常工作 |
| B: 后端未运行 | isRunning=false | **加载失败 → chrome-error 页** ❌ | 在错误页执行 hash 导航 → 报错 |
| C: 后端正在启动 | isRunning=false | 先显示错误页，需要重试 | 同 B，且不会自动恢复 |

**场景 B/C 是实际发生的情况**：PlayerActivity 启动时检测到后端没运行 → 启动后端服务 → 但 WebView 已经开始加载了（在 `super.onCreate()` 阶段），此时后端还没就绪 → WebView 加载失败 → `chrome-error` 页面 → 我们的 JS 在错误页上执行。

### 为什么 MainActivity 没这个问题？

MainActivity **不修改 hash 导航**。它只做 `notifyFrontend`（派发事件），不改变 URL。WebView 自然加载到首页 `/tabs/files` 就停了，不需要额外导航。

---

## 修复方案

### Fix 1: onPageFinished 增加 URL 校验（必须）

在 `onPageFinished` 中校验 URL，只在有效页面才执行导航：

```kotlin
override fun onPageFinished(view: WebView?, url: String?) {
    originalClient?.onPageFinished(view, url)
    if (!navigatedToPlayer && isValidAppUrl(url)) {
        navigatedToPlayer = true
        navigationHandler?.removeCallbacksAndMessages(null)
        // 执行导航...
    }
}

private fun isValidAppUrl(url: String?): Boolean {
    if (url.isNullOrEmpty()) return false
    // 排除所有错误/内部协议
    return !url.startsWith("chrome-error://")
        && !url.startsWith("about:")
        && !url.startsWith("data:")
        && !url.startsWith("javascript:")
}
```

### Fix 2: 错误页时自动重试加载（必须）

当检测到 WebView 加载的是错误页面时，不能只是跳过——因为如果后端随后启动了，WebView 不会自动重新加载。需要一个重试机制：

方案：**用 `shouldInterceptRequest` 或定时轮询检测错误页 + 后端就绪后 reload**

更简单的方案：**在 `notifyFrontend` 收到后端就绪事件时，如果还没导航成功，触发一次 `webView.reload()` 或直接 `loadUrl`**

```kotlin
// notifyFrontend 中增加逻辑：
if (!navigatedToPlayer) {
    Log.i(TAG, "Backend ready but not navigated yet, reloading WebView")
    bridge?.webView?.reload()
}
// 然后 onPageFinished 会在 reload 完成后再次触发，此时 URL 应该是有效的
```

### Fix 3: 增加重试计数防止死循环（增强健壮性）

```kotlin
private var navigationRetryCount = 0
private val MAX_NAVIGATION_RETRIES = 5

// 在 onPageFinished 中：
if (navigationRetryCount >= MAX_NAVIGATION_RETRIES) {
    Log.e(TAG, "Max navigation retries reached, giving up")
    return
}
```

---

## 实施步骤

### Step 1: 修改 PlayerActivity.kt — URL 校验 + 重试机制

- [ ] 新增 `isValidAppUrl(url: String?): Boolean` 方法，过滤 chrome-error、about:blank 等无效 URL
- [ ] 新增 `navigationRetryCount` 计数器和 `MAX_NAVIGATION_RETRIES` 常量
- [ ] 修改 `onPageFinished`：调用 `isValidAppUrl` 校验，无效则递增 retryCount 并返回（不标记 navigatedToPlayer=true）
- [ ] 修改 `notifyFrontend`：在后端就绪但 `!navigatedToPlayer` 时调用 `bridge?.webView?.reload()` 触发重新加载
- [ ] 修改 `setupNavigationTimeout` 兜底：超时时也先尝试 reload 而不是直接设置 hash

### Step 2: 验证构建

- [ ] `go build ./internal/...` 通过
- [ ] `vue-tsc --noEmit` 通过

### Step 3: 本地合并模拟验证

- [ ] 确认 post-cap-sync 会正确复制 PlayerActivity.kt
- [ ] 确认 AndroidManifest.xml 包含 taskAffinity 和 intent-filter

---

## 修复后的完整流程

```
PlayerActivity.onCreate()
  ├─ registerPlugin → super.onCreate (BridgeActivity 开始异步加载 WebView)
  ├─ setupWebViewNavigation()   ← 包装 WebViewClient，带 URL 校验
  ├─ setupNavigationTimeout()   ← 10s 超时兜底
  ├─ registerBackendReceiver()
  ├─ resolveFileInfo(intent)
  └─ 后端未运行?
      ├─ YES → startBackendService(ACTION_START) → 等待广播
      │         WebView 可能已加载出 chrome-error 页
      │         onPageFinished("chrome-error://...") → isValidAppUrl=false → 跳过 ✅
      │
      └─ NO → notifyFrontend(port, true, ...)
              navigatedToPlayer? 
              ├─ 否 → webView.reload() → 触发重新加载
              │         onPageFinished("https://localhost:port/#") → isValidAppUrl=true → 导航 ✅
              └─ 是 → 正常派发事件

收到 BROADCAST_BACKEND_READY:
  └─ notifyFrontend(port, true, ...)
      └─ !navigatedToPlayer? → webView.reload() → 重新加载 → onPageFinished → 导航 ✅
```
