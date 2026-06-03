# OpenList 扩展重写 Spec（ComboLite 合规修复 + UI 技术选型）

## Why

当前 `plugin-openlist` 存在**两个独立问题**：

### 问题 A：不符合 ComboLite 插件规范（非 UI 问题）

用户明确指出"违反 combolite 和 ui 无关"。需要排查并修复 Plugin APK 在 ComboLite 框架层面的合规性问题。参考实现为 [plugin-mpv-player](../plugin-mpv-player/)。

### 问题 B：UI 技术选型未定

用户问"openlist 扩展 UI 能否用 Capacitor 或 Compose 实现"。通过克隆分析 [K-Sillot/OpenList-Mobile](https://github.com/K-Sillot/OpenList-Mobile)（参考实现），已获得明确的架构参考。

---

## Part A: K-Sillot/OpenList-Mobile 架构分析（UI 参考）

### A.1 技术栈概览

| 维度 | K-Sillot/OpenList-Mobile | 我们的项目 |
|------|--------------------------|-----------|
| **框架** | Flutter (Dart) | Capacitor + Ionic Vue (TypeScript) |
| **Android 壳** | 标准 Flutter Activity | Capacitor Android Runtime |
| **OpenList 集成** | gomobile bind (`openlist-lib/`) | gomobile bind (`plugin-openlist/libs/openlist.aar`) |
| **Web UI 展示** | `flutter_inappwebview` (InAppWebView) | 待决定 |
| **服务管理** | OpenListService (Android Service) | OpenListService (Android Service) |
| **状态通信** | MethodChannel (Flutter↔Native) + LocalBroadcast | ContentProvider IPC |

### A.2 K-Sillot 的 4 Tab 布局

```
┌─────────────────────────────────────────┐
│  MyHomePage (Flutter Scaffold)           │
│  ┌───────────────────────────────────┐  │
│  │                                   │  │
│  │  Tab 0: WebScreen (InAppWebView)  │  │ ← 内嵌浏览器加载 http://localhost:{port}
│  │    → OpenList 自有 Vue3 SPA       │  │   （文件管理/存储驱动/用户权限）
│  │                                   │  │
│  ├───────────────────────────────────┤  │
│  │  Tab 1: OpenListScreen (原生控件)  │  │ ← Flutter 原生控制面板
│  │    ├─ AppBar: 版本号              │  │
│  │    ├─ FAB: Start/Stop 切换        │  │
│  │    └─ Body: LogListView (实时日志) │  │
│  │    ├─ 密码设置 (PwdEditDialog)     │  │
│  │    ├─ Config 编辑器               │  │
│  │    └─ 桌面快捷方式                 │  │
│  │                                   │  │
│  ├───────────────────────────────────┤  │
│  │  Tab 2: DownloadManagerPage       │  │
│  │  Tab 3: SettingsScreen            │  │
│  └───────────────────────────────────┘  │
│  ═════════ BottomNavigationBar ════════ │
└─────────────────────────────────────────┘
```

### A.3 关键 UI 组件分析

#### WebScreen — 内嵌 WebView 加载 OpenList Web UI

```dart
// lib/pages/web/web.dart (核心代码)
class WebScreenState extends State<WebScreen> {
  String _url = "http://localhost:5244";  // 默认端口

  @override
  void initState() {
    // 动态获取实际端口
    Android().getOpenListHttpPort()
        .then((port) => {_url = "http://localhost:$port"});
  }

  Widget build(BuildContext context) {
    return InAppWebView(
      initialUrlRequest: URLRequest(url: WebUri(_url)),
      onReceivedError: (controller, request, error) async {
        // 收到错误时自动启动 OpenList 并重试
        if (!await Android().isRunning()) {
          await Android().startService();
          for (int i = 0; i < 3; i++) {
            await Future.delayed(Duration(milliseconds: 500));
            if (await Android().isRunning()) { _webViewController?.reload(); break; }
          }
        }
      },
      onDownloadStartRequest: (controller, url) async {
        // 下载拦截 → 内置下载管理器或外部应用
      },
    );
  }
}
```

**关键行为**：
1. URL 动态化——从 Native 获取实际端口，不硬编码
2. 错误自愈——WebView 加载失败时自动启动服务并重试 3 次
3. 下载拦截——文件下载走内置 DownloadManager 或跳转外部 App
4. URL scheme 过滤——非 http/https/file 等安全 scheme 弹窗确认后跳转

#### OpenListScreen — 原生控制面板

```dart
// lib/pages/openlist/openlist.dart (核心代码)
class OpenListScreen extends StatelessWidget {
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text("OpenList - ${version}"),
        actions: [
          IconButton(icon: Icons.password, onPressed: () {
            showDialog(builder: (_) => PwdEditDialog(onConfirm: (pwd) =>
              Android().setAdminPwd(pwd)));  // → MethodChannel → Native
          }),
          IconButton(icon: Icons.edit_note, onPressed: () {
            Get.to(() => ConfigEditorPage());  // config.json 编辑器
          }),
          IconButton(icon: Icons.add_home, onPressed: () {
            Android().addShortcut();  // 桌面快捷方式
          }),
        ],
      ),
      floatingActionButton: SwitchFloatingButton(
        isSwitch: isRunning,
        onSwitchChange: (s) => s ? startService() : stopService(),
      ),
      body: LogListView(logs: logs),  // 实时日志流
    );
  }
}
```

**功能清单**：
| 功能 | 实现方式 | 对应 Native 方法 |
|------|---------|----------------|
| 启停切换 | FAB SwitchFloatingButton | `ServiceManager.startService()/stopService()` |
| 版本显示 | AppBar title | `Android().getOpenListVersion()` |
| 密码设置 | PwdEditDialog | `Android().setAdminPwd(pwd)` |
| 配置编辑 | ConfigEditorPage（独立页面） | 读/写 `config.json` 文件 |
| 日志查看 | LogListView | Event receiver (`onServerLog`) |
| 桌面快捷方式 | IconButton | `Android().addShortcut()` |

#### ConfigEditorPage — config.json 编辑器

完整功能的 JSON 编辑器：
- 从 Native 获取 dataDir 路径 → 读取 `config.json`
- 实时 JSON 校验（300ms debounce）+ 行号错误定位
- 保存前自动备份 → 失败时回滚
- 三选项对话框：取消 / 仅保存 / 保存并重启服务
- 语法高亮预览模式（flutter_highlight + monokai/github theme）

### A.4 服务通信架构

```
┌─ Flutter Layer (Dart)
│   ServiceManager.instance.startService()
│       ↓ MethodChannel
│   Android().startService() / .stopService() / .isRunning()
│       ↓ MethodChannel
│
├─ Android Native Layer (Kotlin)
│   MainActivity.serviceBridge (ServiceBridge)
│       ↓ startService() → startForegroundService(Intent)
│   OpenListService (Foreground Service)
│       ↓ OpenList.init() → OpenList.startup()
│       ↓ OpenList.shutdown()
│
├─ Event 回调 (Native → Flutter)
│   OpenList.Listener (onShutdown)
│   LocalBroadcastManager (ACTION_STATUS_CHANGED)
│       ↓ MainActivity.serviceBridge?.notifyServiceStatusChanged(isRunning)
│           ↓ MethodChannel callback
│   MyEventReceiver (onServiceStatusChanged, onServerLog)
│
└─ gomobile bind Layer
    openlistlib.Openlistlib (AAR)
        ↓ JNI
    Go runtime (libgojni.so)
```

### A.5 对我们项目的启示

| K-Sillot 模式 | 我们的对应方案 |
|--------------|--------------|
| **InAppWebView** 嵌入 OpenList Web UI | **Capacitor InAppBrowser** 或 `<iframe>` (dev) |
| **FAB 启停** + **LogListView** | Ionic Vue 控制面板（StatusCard + ControlCard） |
| **PwdEditDialog** | Ionic Alert / Modal 输入密码 |
| **ConfigEditorPage** | 可选：Ionic 页面编辑 config.json 或直接打开 Web UI 的设置页 |
| **MethodChannel 通信** | 已有 ContentProvider IPC（`OpenListStatusBridge`）+ GoProcess Plugin |
| **Event 回调** | 已有轮询机制（`getOpenListRuntime()` 3s interval） |

---

## Part B: ComboLite 规范合规性分析

### B.1 参考实现对比（MpvPluginEntry vs OpenListPluginEntry）

| 维度 | MpvPluginEntry（合规参考） | OpenListPluginEntry（当前） | 差距？ |
|------|--------------------------|--------------------------|--------|
| `pluginModule` | `emptyList()` | `listOf(module { single{OpenListBridge} })` | ? 需确认 |
| `onLoad(context)` | 空（无操作） | log + defer to Service | ? 需确认 |
| `onUnload()` | 空（无操作） | log | ? 需确认 |
| `Content()` | 返回核心功能 UI（播放器） | 返回管理面板 UI（状态+控制+配置） | ⚠️ 但用户说这不是问题 |
| **运行时初始化** | Content() 内 `remember { MpvEngine(context) }` | **由 OpenListService 独立管理** | **可能违规** |
| **生命周期绑定** | 跟随 ComboLite 插件生命周期 | **独立 Android Service** | **可能违规** |
| **Crash 隔离** | 依赖 ComboLite 框架 | 自行 try-catch | 需检查 |

### B.2 可能的合规问题（待实施时逐一验证）

> **注意**：以下是基于代码审查的推测，具体需在实施中对照 `combolite-core` AAR 的接口定义验证。

1. **运行时初始化时机**：MpvPluginEntry 在 `Content()` 内延迟初始化引擎；OpenListPluginEntry 的 `onLoad()` 几乎为空，实际初始化委托给独立的 `OpenListService`。如果 ComboLite 要求 `onLoad()` 完成所有初始化，则当前实现不合规。

2. **插件生命周期与 Service 生命周期的解耦**：OpenList 使用独立 Foreground Service，其生命周期不受 ComboLite `load/unload` 控制。这可能导致：
   - ComboLite unload 插件后 OpenList 仍在后台运行
   - ComboLite load 插件时 OpenList 已经在运行（端口冲突）

3. **Koin module 注册内容**：MpvPluginEntry 不注册任何模块；OpenListPluginEntry 注册了 `OpenListBridge` 单例。需确认这是否符合 ComboLite 的 DI 规范。

4. **错误上报机制**：ComboLite 可能有 `PluginCrashHandler` 或类似机制用于捕获插件崩溃。当前 OpenListPluginEntry 的 Compose UI 直接调用 `OpenListBridge.snapshot()` 无 try-catch，可能导致渲染线程崩溃传播到 ComboLite 框架。

---

## What Changes

### 变更一：修复 ComboLite 合规性问题（Part A 核心）

具体修复项待实施时对照 `combolite-core` 接口逐一验证和修复。初步方向：

1. **对齐 `onLoad()` / `onUnload()` 行为**：确保与 MpvPluginEntry 一致或符合 ComboLite 文档要求
2. **Service 生命周期与 Plugin 生命周期同步**：`onUnload()` 时必须 shutdown service；`onLoad()` 时检查并恢复
3. **Content() 内的防御性编程**：所有 Bridge 调用加 try-catch，防止崩溃传播到 ComboLite 框架
4. **验证 Koin module 注册规范**

### 变更二：UI 技术方案（Part B 核心）

基于 K-Sillot/OpenList-Mobile 的成功实践，采用 **双面板方案**：

```
┌─ Host App (Capacitor / Ionic Vue) ─────────────┐
│                                                  │
│  Panel A: OpenList Web UI（内嵌 WebView）         │
│  ┌──────────────────────────────────────────┐   │
│  │  Capacitor InAppBrowser / IonRouterOutlet │   │
│  │  → http://127.0.0.1:{port}/#/login      │   │
│  │  （OpenList 自有 Vue3 SPA 完整功能）       │   │
│  │  - 文件浏览/管理                          │   │
│  │  - 存储驱动配置                            │   │
│  │  - 用户权限管理                            │   │
│  │  - ENCV 解密设置                           │   │
│  └──────────────────────────────────────────┘   │
│                                                  │
│  Panel B: 原生控制条（Ionic Vue 组件）           │
│  ┌──────────────────────────────────────────┐   │
│  │  [● 运行中]  PID:12345  Port:5244         │   │
│  │  [▶ 启动]  [⏹ 停止]  [🔑 密码]  [⚙ 配置] │   │
│  │  [🌐 打开管理界面]                         │   │
│  └──────────────────────────────────────────┘   │
│                                                  │
└──────────────────────────────────────────────────┘
```

**技术选择**：

| 场景 | 方案 | 说明 |
|------|------|------|
| **开发预览** | Vite proxy `/openlist-ui/` → `127.0.0.1:5244` | 已有实现 |
| **生产 APK — 方案首选** | `@capacitor/inappbrowser` 打开 OpenList SPA | 与 K-Sillot 的 InAppWebView 对等 |
| **生产 APK — 方案备选** | Plugin APK 内嵌 Compose WebView | 如果 InAppBrowser 体验不佳 |

**Panel B（原生控制条）放在哪里**：
- **方案 B1**：增强 `LocalOpenListStatusCard.vue`（Remote.vue 内），增加启停/密码/配置按钮
- **方案 B2**：新建独立的 `OpenListManagePage.vue` 作为路由页面，包含完整控制面板 + WebView 入口

> **推荐 B1**（最小改动）：保持 StatusCard 只读摘要不变，仅增加"打开管理界面"按钮（调 InAppBrowser）。启停/配置等操作直接在 OpenList Web UI 中完成（K-Sillot 也是这样——他的原生面板只有 start/stop + logs + password，其他都在 Web UI 里操作）。

### 变更三：Plugin APK Content() 瘦身

无论 UI 最终用什么技术实现，`OpenListPluginEntry.Content()` 都应瘦身：

- 当前：~400 行 Compose Material3 UI（StatusCard + ControlCard + ConfigCard）
- 目标：空 Composable 或最小占位（因为 UI 已移至 Host App 侧）
- 效果：移除 compose 依赖，APK 瘦身 ~15MB

---

## Impact

- Affected code:
  - **修改**: `plugin-openlist/OpenListPluginEntry.kt`（Content() 瘦身 + 合规修复）
  - **修改**: `plugin-openlist/build.gradle.kts`（移除 compose 依赖）
  - **微调**: `src/components/LocalOpenListStatusCard.vue`（增加"打开管理界面"按钮）
  - **可选新增**: `src/composables/useOpenListManager.ts`（如需原生控制条）
  - **不变**: `OpenListBridge.kt`, `OpenListStatusProvider.kt`, `OpenListService.kt`, `OpenListConfig.kt`
  - **不变**: `OpenListStatusBridge.kt`, `GoProcess.ts`

## ADDED Requirements

### Requirement: ComboLite 合规修复

`plugin-openlist` SHALL 通过 ComboLite 框架的所有合规性检查。具体包括但不限于：

- `onLoad()` / `onUnload()` 行为符合 `IPluginEntryClass` 接口契约
- 插件运行时生命周期与 ComboLite 插件生命周期正确同步
- 所有 Bridge 调用在 Composable 内有防御性异常处理
- Koin module 注册符合 ComboLite DI 规范

> **实施说明**：具体合规项需在编码阶段对照 `combolite-core` AAR 的接口定义逐一验证。

### Requirement: OpenList Web UI 通过 Capacitor InAppBrowser 访问

Host App SHALL 提供"打开 OpenList 管理界面"入口，使用 Capacitor InAppBrowser 插件打开 OpenList 自有的 Vue3 SPA。

- **WHEN** 用户点击"打开管理界面"且 OpenList running=true
- **THEN** `InAppBrowser.open({ url: 'http://127.0.0.1:{port}/#/' })` 打开 OpenList 管理 SPA
- **WHEN** OpenList running=false
- **THEN** 按钮 disabled 或隐藏

### Requirement: Plugin Content() 瘦身

`OpenListPluginEntry.Content()` SHALL 返回空或最小占位 Composable，不含任何管理 UI 组件。

## MODIFIED Requirements

### Requirement: LocalOpenListStatusCard 增强

保留只读状态摘要展示，新增"打开管理界面"入口按钮。

## REMOVED Requirements

### Removed: Plugin Compose Management UI

**Reason**: 功能重复（OpenList 自有 Web UI 已覆盖）；违反原始 spec 设计决策（"不分 Compose UI"）；用户明确表示 OpenList 有独立 UI。
**Migration**: 删除 StatusCard/ControlCard/ConfigCard。管理功能通过 InAppBrowser 打开 OpenList Web UI 完成。

---

## 架构总览（目标状态）

```
【最终架构】
┌─ Host App Process (Capacitor / Ionic Vue) ────┐
│                                                │
│  Remote.vue                                    │
│  └─ LocalOpenListStatusCard                   │
│       ├─ PID / Port / Running / DataSize       │  ← 只读摘要（已有）
│       └─ [🌐 打开管理界面] 按钮                  │  ← 新增：InAppBrowser.open(5244)
│                                                │
│  ExtensionsPage.vue                            │
│  └─ OpenList 卡片                              │  ← install/uninstall/toggle（不变）
│                                                │
├─ Plugin APK Process (Independent) ─────────────┤
│                                                │
│  OpenListPluginEntry.Content()                 │  ← 空/最小占位（瘦身）
│  OpenListBridge (gomobile 桥接)                │  ← 不变
│  OpenListService (前台服务)                     │  ← 不变
│  OpenListStatusProvider (ContentProvider IPC)  │  ← 不变
│  OpenListConfig (配置持久化)                    │  ← 不变
│                                                │
├─ OpenList Web UI (Vue3 SPA) ──────────────────┤
│  http://127.0.0.1:{port}/#/                    │  ← InAppBrowser 承载
│  （完整管理功能：文件/存储/用户/ENCV设置）        │
│                                                │
└────────────────────────────────────────────────┘
```
