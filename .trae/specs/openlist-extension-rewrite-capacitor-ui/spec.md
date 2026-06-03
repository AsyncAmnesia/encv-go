# OpenList 扩展重写 Spec（ComboLite 合规 + 插件内 UI 技术选型）

## Why

当前 `plugin-openlist` 存在**两个独立问题**：

### 问题 A：不符合 ComboLite 插件规范（非 UI 问题）

用户明确指出"违反 combolite 和 ui 无关"。需要排查并修复 Plugin APK 在 ComboLite 框架层面的合规性问题。

### 问题 B：插件内 UI 实现方式需决策

用户问"openlist 扩展 UI 能否用 Capacitor 或 Compose 实现"。通过克隆分析 [K-Sillot/OpenList-Mobile](https://github.com/K-Sillot/OpenList-Mobile)（参考实现），已获得架构参考。

**核心原则：OpenList 扩展的 UI 属于插件自身，在 `Content()` 内渲染，不嵌入主应用。**

---

## Part A: K-Sillot/OpenList-Mobile 架构分析（UI 参考）

### A.1 技术栈概览

| 维度 | K-Sillot/OpenList-Mobile | 我们的项目 |
|------|--------------------------|-----------|
| **框架** | Flutter (Dart) | Host: Capacitor+Ionic Vue / Plugin: Android原生 |
| **OpenList 集成** | gomobile bind (`openlist-lib/`) | gomobile bind (`libs/openlist.aar`) |
| **Web UI 展示** | `flutter_inappwebview` (InAppWebView) | 待决定 |
| **服务管理** | OpenListService (Android Service) | OpenListService (Android Service) |

### A.2 K-Sillot 的做法（关键参考）

K-Sillot/OpenList-Mobile 是一个**独立的 Flutter 应用**，它完整地包含了 OpenList 的所有 UI：

```
┌─ OpenList-Mobile App (Flutter) ──────────────┐
│                                                │
│  Tab 0: WebScreen                             │
│  ┌────────────────────────────────────────┐   │
│  │  InAppWebView                          │   │
│  │  → http://localhost:{port}             │   │
│  │  （OpenList 自有 Vue3 SPA 全部功能）     │   │
│  └────────────────────────────────────────┘   │
│                                                │
│  Tab 1: OpenListScreen (Flutter 原生控件)      │
│    ├─ FAB: Start/Stop 切换                    │
│    ├─ LogListView (实时日志)                   │
│    ├─ 密码设置 (PwdEditDialog)                │
│    ├─ Config 编辑器 (ConfigEditorPage)        │
│    └─ 版本号 / 桌面快捷方式                   │
│                                                │
│  Tab 2: DownloadManager                       │
│  Tab 3: Settings                              │
└────────────────────────────────────────────────┘
```

**核心模式**：
- **InAppWebView** 嵌入 OpenList 自有 Web UI（文件管理/存储驱动/用户权限/ENCV设置全部在这里操作）
- **原生控件**只做启停控制 + 日志查看 + 密码/配置快捷编辑
- Web UI 是主要交互界面，原生控件是辅助工具栏

### A.3 WebScreen 关键行为（InAppWebView）

```dart
// URL 动态化 — 从 Native 获取端口
Android().getOpenListHttpPort()
    .then((port) => {_url = "http://localhost:$port"});

// 错误自愈 — 加载失败自动启动服务并重试 3 次
onReceivedError: (controller, request, error) async {
  if (!await Android().isRunning()) {
    await Android().startService();
    for (int i = 0; i < 3; i++) {
      await Future.delayed(Duration(milliseconds: 500));
      if (await Android().isRunning()) { _webViewController?.reload(); break; }
    }
  }
}

// 下载拦截 — 文件下载走内置管理器
onDownloadStartRequest: (controller, url) async {
  DownloadManager.downloadFileInBackground(url.url, url.suggestedFilename);
}
```

### A.4 对我们项目的启示

K-Sillot 证明了 **"WebView 嵌入 OpenList Web UI + 原生辅助控件"** 是可行的产品形态。

我们的 `plugin-openlist` 作为 ComboLite 插件，其 `Content()` 可以采用类似模式——在插件内部用 **Compose WebView** 或其他方式展示 OpenList 的管理界面。

---

## Part B: ComboLite 规范合规性分析（非 UI）

### B.1 参考实现对比

| 维度 | MpvPluginEntry（合规参考） | OpenListPluginEntry（当前） |
|------|--------------------------|--------------------------|
| `pluginModule` | `emptyList()` | `listOf(module { single{OpenListBridge} })` |
| `onLoad(context)` | 空 | log + defer to Service |
| `onUnload()` | 空 | log |
| `Content()` | 返回核心功能 UI（播放器） | 返回管理面板 UI (~400行) |
| **运行时初始化** | Content() 内延迟初始化 | 由独立 OpenListService 管理 |
| **生命周期绑定** | 跟随 ComboLite 插件生命周期 | 独立 Android Service |

### B.2 可能的合规问题（待实施时验证）

> 具体合规项需对照 `combolite-core` AAR 接口定义逐一确认。

1. **运行时初始化时机**：`onLoad()` 是否应完成 Bridge 初始化？当前几乎为空。
2. **Service 生命周期与 Plugin 生命周期的解耦**：`onUnload()` 时 Service 可能仍在运行。
3. **Koin module 注册规范**：MpvPluginEntry 不注册模块，OpenListPluginEntry 注册了 Bridge 单例。
4. **Crash 隔离**：Content() 内 Bridge 调用无 try-catch，崩溃可能传播到 ComboLite 框架。

---

## What Changes

### 变更一：修复 ComboLite 合规性问题

具体修复项待实施时对照 `combolite-core` 接口逐一验证和修复：
1. 对齐 `onLoad()` / `onUnload()` 行为
2. Service 生命周期与 Plugin 生命周期同步
3. Content() 内防御性编程
4. 验证 Koin module 注册规范

### 变更二：插件内 UI 重写（Content()）

**当前**：~400 行 Compose Material3 手写 UI（StatusCard + ControlCard + ConfigCard），功能与 OpenList Web UI 重复。

**目标方案（二选一）**：

#### 方案 A：Compose WebView（推荐，对齐 K-Sillot 模式）

在 `Content()` 内使用 **AndroidX WebView**（非 Compose Material3 手写组件）加载 OpenList 自有 Web UI：

```kotlin
@Composable
override fun Content() {
    // 使用 AndroidView 包裹 WebView，加载 http://127.0.0.1:{port}
    // 类似 K-Sillot 的 InAppWebView 模式
    AndroidView(factory = { context ->
        WebView(context).apply {
            webViewClient = WebViewClient()
            loadUrl("http://127.0.0.1:${OpenListBridge.snapshot()["port"]}")
        }
    })
}
```

**优点**：
- 与 K-Sillot 成功实践一致
- 自动获得 OpenList 所有 Web UI 功能（无需手写）
- OpenList Web UI 升级时插件自动受益
- Compose 依赖仅保留 `androidx.compose.ui`（AndroidView）+ WebView

**缺点**：
- 仍需 compose runtime（但只需最小子集，不需要 material3）
- WebView 在独立进程中可能有限制

#### 方案 B：纯 Compose 重写（保留现有模式但修正）

保留手写 Compose UI 方向，但：
1. 修正为符合 ComboLite 规范的实现
2. 只保留**必要**的控制功能（start/stop + 状态摘要），不重复 Web UI
3. 增加"打开 Web UI"按钮跳转外部浏览器

**优点**：完全原生体验，不依赖 WebView
**缺点**：需要维护两套 UI（Compose + Web UI）；功能覆盖不如 Web UI 完整

> **待用户确认选择方案 A 或 B**。

### 变更三：Host App 侧不做改动

Host App（Ionic Vue）的 OpenList 相关代码保持不变：
- `LocalOpenListStatusCard.vue` — 保持只读状态摘要
- `Remote.vue` — 不增加管理功能
- `ExtensionsPage.vue` — 不改变交互逻辑
- `useOpenListBridge.ts` — 保持轮询逻辑

---

## Impact

- Affected code（仅在 `plugin-openlist/` 内）：
  - **修改**: `OpenListPluginEntry.kt`（Content() 重写 + 合规修复）
  - **修改**: `build.gradle.kts`（依赖调整）
  - **不变**: `OpenListBridge.kt`, `OpenListStatusProvider.kt`, `OpenListService.kt`, `OpenListConfig.kt`
- **不影响** Host App 任何 Vue/TS 文件

## ADDED Requirements

### Requirement: ComboLite 合规修复

`plugin-openlist` SHALL 通过 ComboLite 框架的所有合规性检查。

### Requirement: 插件内 UI 在 Content() 中渲染

OpenList 扩展的所有用户交互界面 SHALL 在 `OpenListPluginEntry.Content()` 中渲染，属于插件进程内部。Host App 不包含任何 OpenList 管理功能代码。

#### Scenario: 用户打开 OpenList 插件页面
- **WHEN** ComboLite 框架调用 `Content()` 渲染 OpenList 插件页面
- **THEN** 显示插件的完整管理界面（方案 A: WebView 加载 OpenList SPA / 方案 B: Compose 原生控件）

## MODIFIED Requirements

（无）

## REMOVED Requirements

（无）

---

## 架构总览（目标状态）

```
【最终架构】
┌─ Host App Process (Capacitor / Ionic Vue) ────┐
│                                                │
│  Remote.vue                                    │
│  └─ LocalOpenListStatusCard (只读摘要，不变)    │
│                                                │
│  ExtensionsPage.vue (install/uninstall，不变)   │
│                                                │
├─ Plugin APK Process (Independent) ─────────────┤  ← 所有 UI 在这里
│                                                │
│  OpenListPluginEntry.Content()                 │
│    ├─ 方案 A: AndroidView(WebView)             │  ← 加载 http://127.0.0.1:{port}
│    │   → OpenList 自有 Vue3 SPA 完整功能        │
│    └─ 或方案 B: Compose 原生控件               │  ← start/stop/status
│                                                │
│  OpenListBridge (gomobile 桥接)    （不变）      │
│  OpenListService (前台服务)         （不变）      │
│  OpenListStatusProvider (IPC)       （不变）      │
│  OpenListConfig (配置持久化)         （不变）      │
│                                                │
├─ OpenList Web UI (Vue3 SPA) ──────────────────┤
│  http://127.0.0.1:{port}/#/                     │  ← 由插件内 WebView 加载
│                                                │
└────────────────────────────────────────────────┘
```
