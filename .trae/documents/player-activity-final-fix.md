# PlayerActivity 白屏 + 独立窗口彻底修复 Plan

## 问题 1：白屏根因分析

### 当前 `load()` 执行链路

```
PlayerActivity.onCreate()
  → registerPlugin(GoProcessPlugin)
  → super.onCreate()  [BridgeActivity.onCreate]
      → setContentView(WebView布局)
      → bridgeBuilder.addPlugins(从assets加载的插件)
      → this.load()  [我们的 override]
          → super.load()  [BridgeActivity.load]
              → bridge = bridgeBuilder.create()
              → Bridge 构造函数内:
                  → initWebView()
                  → localServer = new WebViewLocalServer(context, this, injector, authorities, html5mode)
                  → localServer.hostAssets(DEFAULT_WEB_ASSET_DIR)  // host "public" 目录
                  → webView.loadUrl(appUrl)  // 开始异步加载 index.html ← 第一次 loadUrl
          → bridge?.webView?.loadUrl("https://localhost/player.html")  // ← 第二次 loadUrl，取消第一次
```

### 为什么第二次 `loadUrl(player.html)` 可能失败

从 [BridgeWebViewClient.java](https://unpkg.com/@capacitor/android@8.3.4/capacitor/src/main/java/com/getcapacitor/BridgeWebViewClient.java) 看到：

1. **`onPageStarted` 中调用 `bridge.reset()`** — 每次 page load 开始都 reset bridge 状态
2. **`shouldInterceptRequest` 将所有请求交给 `localServer.shouldInterceptRequest`** — LocalServer 从 assets 目录提供文件

**核心风险**：`super.load()` 中的 `webView.loadUrl(appUrl)` 和我们的 `loadUrl(playerUrl)` 之间没有时序保证。WebView 可能在第一次 loadUrl 的回调链中（如 `shouldInterceptRequest` 正在处理 index.html 请求）就被第二次 loadUrl 打断，导致 LocalServer 进入不一致状态。

### 更可靠的方案：不依赖 load() 中的二次 redirect

**方案 A（推荐）：使用 `Handler().post { }` 延迟 redirect**

将 redirect 放到下一个消息循环，确保 `super.load()` 的所有同步操作完成、WebView 的第一次 loadUrl 已经发起：

```kotlin
override fun load() {
    super.load()
    android.os.Handler(android.os.Looper.getMainLooper()).post {
        try {
            val playerUrl = "https://localhost/player.html"
            Log.i(TAG, "load: deferred redirect to $playerUrl")
            bridge?.webView?.loadUrl(playerUrl)
        } catch (e: Exception) {
            Log.e(TAG, "load: deferred redirect failed", e)
        }
    }
}
```

**方案 B（最稳）：覆写 `this.config` 注入自定义 `serverPath`**

在 `super.onCreate()` 之前设置 config，让 bridge 直接加载 player.html 而非 index.html：

```kotlin
override fun onCreate(savedInstanceState: Bundle?) {
    registerPlugin(GoProcessPlugin::class.java)
    config = CapConfig.loadDefault(this)  // 先加载默认配置
    // 通过反射或 Builder 设置 serverPath 指向 player.html
    super.onCreate(savedInstanceState)
}
```
→ 太复杂且脆弱，不推荐。

**最终选择：方案 A + 验证 LocalServer 能否提供 player.html**

---

## 问题 2：独立窗口 — 缺少文档中心模型

### 用户明确指出的问题

当前配置缺少 Android **Document-Centric** 模型的核心要素：

| 当前状态 | 应有状态 |
|---------|---------|
| 无 `FLAG_ACTIVITY_NEW_DOCUMENT` | 必须添加 |
| 无 `documentLaunchMode` | Manifest 必须声明 |
| `launchMode="singleTask"` | 应改为 `"standard"` |
| Intent 无区分 data | 每个"文档"应有唯一标识 |

### 修复详情

#### Step 1：AndroidManifest.xml — PlayerActivity 声明

```xml
<activity
    android:name=".PlayerActivity"
    android:exported="true"
    android:launchMode="standard"
    android:documentLaunchMode="always"
    android:maxRecents="16"
    android:taskAffinity="com.encvgo.app.player.task"
    android:theme="@style/AppTheme.NoActionBar"
    android:configChanges="orientation|keyboardHidden|keyboard|screenSize|locale|smallestScreenSize|screenLayout|uiMode"
    android:label="ENCV Player">
    <!-- intent-filter 保持不变 -->
</activity>
```

变更点：
- `launchMode`: `singleTask` → `standard`（singleTask 会强制合并同 affinity 任务）
- 新增 `android:documentLaunchMode="always"`（**关键**，启用文档中心模型）
- 新增 `android:maxRecents="16"`（限制最近任务卡片数）
- 保留 `taskAffinity`（配合 documentLaunchMode 使用）

#### Step 2：GoProcessPlugin.kt — openInPlayer() Intent Flags

```kotlin
@PluginMethod
fun openInPlayer(call: PluginCall) {
    val path = call.getString("path", "")
    val name = call.getString("name", "")
    val mimeType = call.getString("mimeType", "")
    if (path.isNullOrEmpty()) {
        Log.w(TAG, "openInPlayer rejected: path is empty")
        call.reject("path is required")
        return
    }
    try {
        Log.d(TAG, "openInPlayer: path=$path, name=$name, mimeType=$mimeType")
        val uniqueId = System.currentTimeMillis().toString()  // 每次打开生成唯一 ID
        val intent = Intent(activity, PlayerActivity::class.java).apply {
            addFlags(
                Intent.FLAG_ACTIVITY_NEW_DOCUMENT
                    or Intent.FLAG_ACTIVITY_MULTIPLE_TASK
                    or Intent.FLAG_ACTIVITY_RETAIN_IN_RECENTS
            )
            data = Uri.parse("encvgo://player/$uniqueId")  // 唯一 data 区分不同文档实例
            putExtra("file_path", path)
            putExtra("file_name", name)
            putExtra("file_mime_type", mimeType)
        }
        Log.d(TAG, "openInPlayer: launching with NEW_DOCUMENT+MULTIPLE_TASK+RETAIN_IN_RECENTS, data=${intent.data}")
        activity.startActivity(intent)
        call.resolve()
    } catch (e: Exception) {
        Log.e(TAG, "openInPlayer failed to start PlayerActivity", e)
        call.reject("Failed to open player: ${e.message}")
    }
}
```

变更点：
- 添加 `FLAG_ACTIVITY_NEW_DOCUMENT`（**核心**，触发文档中心模型）
- 保留 `FLAG_ACTIVITY_MULTIPLE_TASK`（允许同时存在多个播放器卡片）
- 添加 `FLAG_ACTIVITY_RETAIN_IN_RECENTS`（关闭后仍保留卡片）
- 设置唯一 `data` URI（系统据此判断是否为不同文档）

#### Step 3：PlayerActivity.kt — onDestroy 清理

关闭播放器时移除任务卡片，避免残留空卡片：

```kotlin
override fun onDestroy() {
    Log.d(TAG, "onDestroy: cleaning up")
    if (backendReceiverRegistered) {
        unregisterReceiver(backendReceiver)
        backendReceiverRegistered = false
    }
    finishAndRemoveTask()  // 移除最近任务中的卡片
    super.onDestroy()
}
```

---

## 修改文件清单

| # | 文件 | 修改内容 |
|---|------|---------|
| 1 | `android-overlay/.../PlayerActivity.kt` | `load()` 改用 Handler.post 延迟 redirect；`onDestroy()` 添加 `finishAndRemoveTask()` |
| 2 | `android-overlay/.../AndroidManifest.xml` | PlayerActivity 添加 `documentLaunchMode="always"`，`launchMode` 改为 `standard`，添加 `maxRecents` |
| 3 | `android-overlay/.../GoProcessPlugin.kt` | `openInPlayer()` 添加 `FLAG_ACTIVITY_NEW_DOCUMENT` + `RETAIN_IN_RECENTS` + 唯一 data URI |

---

## 白屏补充诊断

如果上述修复后仍白屏，需检查以下方向（按优先级排序）：

1. **确认 `dist/player.html` 存在** — Vite 多入口构建必须产出该文件
2. **logcat 过滤 `ENCV-go`** — 查看 `load:` 日志是否打印了 redirect URL
3. **logcat 过滤 `Capacitor` / `LocalServer`** — 查看 shouldInterceptRequest 是否收到 `/player.html` 请求及返回结果
4. **Chrome DevTools 远程调试** — 连接 WebView 检查 Console 错误和网络请求
5. **验证 `player-main.ts` 及其 import 链** — 所有 Vue 组件、router、plugin 是否正确打包
