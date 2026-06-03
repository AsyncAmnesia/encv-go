# OpenList 扩展重写 Spec（Capacitor 多例 + K-Sillot UI 复刻）

## Why

### 用户对前几版 spec 的反馈

- ❌ "谁让你塞完整ui到主应用的？" → 不要把 OpenList UI 嵌入主应用 Ionic Vue
- ❌ "还是很乱" → 前版 spec 把 UI 写法和架构混在一起
- ❌ "完全没理解我的意思" → 我误解了"Capacitor 多例"的真实含义

### 用户的真实意图（澄清后）

通过 AskUserQuestion 确认：

| 问题 | 答案 |
|------|------|
| "Capacitor 多例" 含义 | **openlist 也看做一个 Capacitor 应用**（不是装多份 OpenList） |
| UI 加载什么内容 | **K-Sillot/OpenList-Mobile UI 复刻**（不加载 OpenList 自带 Web SPA） |
| UI 放在哪里 | **在 plugin-openlist 自己的 Activity/View 中**（不在主 app 进程） |

### 最终需求（正确版本）

> **把 `plugin-openlist` 改造成一个独立的 Capacitor 应用，内部运行 K-Sillot/OpenList-Mobile 风格的 UI（4 Tab：OpenList 控制 / WebView 加载 OpenList SPA / 下载管理 / 设置），而不是用 Compose 重复实现。**

---

## Part A: 架构总览（最终正确版）

### A.1 K-Sillot/OpenList-Mobile 的架构参考

K-Sillot/OpenList-Mobile 是一个 **Flutter 应用**，包含完整 OpenList 管理 UI：

```
┌─ OpenList-Mobile (Flutter) ───────────────────┐
│                                                  │
│  MyHomePage (BottomNavigationBar 4 tab)        │
│  ├─ Tab 0: WebScreen (InAppWebView)            │
│  │   → http://localhost:5244/ 加载 OpenList SPA │
│  │   （主要交互界面：文件管理/存储/用户/ENCV）    │
│  │                                              │
│  ├─ Tab 1: OpenListScreen (原生控制面板)        │
│  │   ├─ AppBar: "OpenList - v0.x.x"            │
│  │   ├─ Actions: 密码 / Config / 快捷方式 / 更多 │
│  │   ├─ FAB: Start/Stop 切换按钮                │
│  │   └─ Body: LogListView (实时日志)            │
│  │                                              │
│  ├─ Tab 2: DownloadManagerPage (下载管理)       │
│  │                                              │
│  └─ Tab 3: SettingsScreen (应用设置)            │
│                                                  │
└──────────────────────────────────────────────────┘
```

**关键观察**：

1. **K-Sillot Mobile 是完整的独立应用**——不是 plugin，不是 SDK
2. **UI 是 Flutter 写的原生界面**，不直接复用 OpenList Web SPA（虽然 SPA 通过 WebView 加载作为 Tab 0）
3. **OpenList Web SPA 通过 `flutter_inappwebview` 加载**到 WebScreen Tab 里

### A.2 我们项目的目标架构

**把 K-Sillot 模式用 Capacitor/Ionic 复刻到 plugin-openlist 内**：

```
┌─ plugin-openlist (独立 APK / 独立 Capacitor 应用) ──────────────┐
│                                                                  │
│  进程边界：plugin-openlist.apk 是独立 Android 进程               │
│  入口：OpenListApplication (extends Application) + Capacitor     │
│  入口 Activity：OpenListMainActivity (extends BridgeActivity)    │
│                                                                  │
│  路由：/ (默认 HomePage) / web / openlist / downloads / settings  │
│                                                                  │
│  关键页面：                                                       │
│  ├─ HomePage.vue (ion-tabs 4 tab)                                │
│  │   ├─ Tab 0: WebScreen.vue                                     │
│  │   │   → <iframe> 或 Capacitor Browser 加载 http://127.0.0.1   │
│  │   │                                                              │
│  │   ├─ Tab 1: OpenListScreen.vue (主控面板)                      │
│  │   │   ├─ AppBar: "OpenList - v{version}"                       │
│  │   │   ├─ Toolbar: 密码 / Config / 桌面快捷 / 更多              │
│  │   │   ├─ FAB: Start/Stop 切换                                  │
│  │   │   └─ Body: LogListView (实时日志流)                         │
│  │   │                                                              │
│  │   ├─ Tab 2: DownloadManager.vue                                │
│  │   │                                                              │
│  │   └─ Tab 3: Settings.vue                                       │
│                                                                  │
│  Capacitor 插件（在 plugin-openlist 自己内部）：                   │
│  ├─ OpenListServicePlugin (start/stop/getStatus)                 │
│  │   → 调 OpenListBridge (gomobile bind)                         │
│  ├─ OpenListConfigPlugin (read/write config.json)                │
│  │   → 调 OpenListBridge (gomobile bind)                         │
│  └─ OpenListPasswordPlugin (set admin password)                  │
│       → 调 OpenListBridge (gomobile bind)                         │
│                                                                  │
│  底层运行时：                                                     │
│  ├─ OpenListBridge (gomobile bind 产物)                          │
│  ├─ OpenListService (Android 前台服务)                            │
│  ├─ OpenListConfig (config.json 持久化)                           │
│  └─ OpenListStatusProvider (ContentProvider IPC, host 端读取)    │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘

┌─ 主 ENCV App (encvg-mobile) ────────────────────────────────────┐
│  保持不变：                                                     │
│  ├─ ExtensionsPage 显示 OpenList 扩展已安装                      │
│  └─ 状态摘要卡片（通过 ContentProvider 读 OpenListStatusProvider）│
│                                                                  │
│  ❌ **不**包含 OpenList 管理 UI（不嵌入主应用）                  │
│  ❌ **不**调 OpenList 控制命令（由用户通过 OpenList 自己的 App UI │
│     或 External Intent 启动 OpenList 独立 Activity）             │
└──────────────────────────────────────────────────────────────────┘
```

### A.3 "多例" 的最终含义

> **"多例" 指 plugin-openlist 本身可以独立启动多次（每次都是完整的 Capacitor 应用），而不是把 OpenList 当成宿主 App 的单例。**

具体场景：

1. **作为插件 APK 的一部分**——主 app 通过 `startActivity(Intent)` 调起 OpenList 独立 Activity，呈现完整 UI
2. **作为独立应用**——用户可以从 launcher 启动 plugin-openlist 本身（如果有 launcher intent-filter）
3. **多次启动**——用户可多次 startActivity 打开多个 OpenList 实例（每个独立 Activity/独立 WebView/独立后端数据）

这是 K-Sillot Mobile 的设计：作为独立应用，**不**作为宿主的内嵌 SDK。

---

## Part B: 关键架构决策

### B.1 Capacitor 应用 vs 插件

**问题**：plugin-openlist 当前是 **ComboLite 插件 APK**（被宿主动态加载的 dex/AAR），不是独立应用。

**需求**：要把它改造成可独立启动的 **Capacitor Android 应用 APK**。

**技术方案**：

1. **保留 ComboLite 插件形态**（仍是 `.apk`，被 ENCV host 加载）
2. **额外添加一个独立入口 Activity**（`OpenListMainActivity extends BridgeActivity`）
3. **这个 Activity 启动 Capacitor runtime**，加载 plugin-openlist 自己的 web 资源（`assets/public/`）
4. **web 资源**包含 K-Sillot Mobile UI 的 Ionic 复刻版

### B.2 Capacitor 工程拆分

```
plugin-openlist/
├── android/                          # 已是独立 Android Library/AAR
│   └── src/main/
│       ├── AndroidManifest.xml       # 🔧 添加独立 launcher Activity
│       ├── java/com/encvgo/plugin/openlist/
│       │   ├── OpenListBridge.kt    # ✅ 不变
│       │   ├── OpenListConfig.kt    # ✅ 不变
│       │   ├── OpenListService.kt   # ✅ 不变
│       │   ├── OpenListStatusProvider.kt # ✅ 不变
│       │   └── (新) OpenListMainActivity.kt # 🔧 新增独立入口
│       └── assets/                   # 🔧 新增 Ionic web 资源
│           ├── public/               # Vite build 产物
│           │   ├── index.html
│           │   └── assets/
│           └── capacitor.config.json
│
└── web/                              # 🔧 新增 Ionic + Vue 项目
    ├── package.json                  # Vue 3 + Ionic Vue
    ├── vite.config.ts
    ├── capacitor.config.ts
    ├── src/
    │   ├── main.ts                   # 入口
    │   ├── App.vue
    │   ├── router/
    │   ├── views/
    │   │   ├── HomePage.vue          # 4 tab 容器
    │   │   ├── WebScreen.vue         # Tab 0: WebView OpenList SPA
    │   │   ├── OpenListScreen.vue    # Tab 1: 主控面板
    │   │   ├── DownloadManager.vue   # Tab 2
    │   │   └── Settings.vue          # Tab 3
    │   ├── components/
    │   │   ├── LogListView.vue
    │   │   ├── PwdEditDialog.vue
    │   │   └── ConfigEditorPage.vue
    │   └── plugins/                  # 调主 app 的 Capacitor 插件
    │       ├── OpenListService.ts    # start/stop
    │       ├── OpenListConfig.ts     # read/write config
    │       └── OpenListPassword.ts   # set password
    └── public/
        └── openlist-spa/             # OpenList 自带 Vue3 SPA（可选）
```

### B.3 Capacitor Plugin 跨进程调用

plugin-openlist 是独立 Capacitor 应用，但**它也需要调自己的 OpenList 后端**（gomobile bind 产物）：

```
plugin-openlist Capacitor App
  (web 进程)                    (native 进程，同 APK)
  OpenListScreen.vue
    └─ await OpenListService.start()      [TypeScript → Native Bridge]
        │
        ▼
  OpenListServicePlugin.kt (Capacitor Plugin in plugin-openlist)
        │
        └─ OpenListBridge.start()          [gomobile bind → JNI → Go]
            │
            └─ OpenListService (前台服务)   [Android Service]
```

**不需要跨 APK 通信**——因为 gomobile 库和 web 资源在同一个 APK 内。

但**主 app 仍然要读 OpenList 状态**——通过 **ContentProvider IPC**（已有 `OpenListStatusBridge`）：

```
主 ENCV app (com.encvgo.app)
  └─ ContentResolver.query(OpenListStatusProvider URI)
       ↓ 跨进程 ContentProvider
  plugin-openlist (com.encvgo.plugin.openlist)
     └─ OpenListStatusProvider.query() → OpenListBridge.snapshot()
```

这是**唯一**的跨 APK 通信通道，**只读**（主 app 只看状态，不控制）。

### B.4 ComboLite 合规问题（与 UI 无关）

用户已确认"违反 combolite 和 ui 无关"——意味着存在**非 UI 的合规问题**。

主要怀疑点（需在实施时验证 `combolite-core` 接口契约）：

1. **`onLoad(context)` 几乎为空**——运行时由独立 `OpenListService` 管理
2. **Service 生命周期与 Plugin 生命周期解耦**——plugin unload 时 Service 可能仍在运行
3. **`pluginModule` 注册了 OpenListBridge 单例**——与参考 MpvPluginEntry（`emptyList()`）不同

**修复方向**（Phase 0 任务）：

- `onLoad()` 初始化 Bridge（参考 MpvPluginEntry 的 Content() 内部初始化）
- `onUnload()` shutdown Bridge + Service
- `pluginModule = emptyList()`（与 MpvPluginEntry 一致）

---

## What Changes

### 变更一：ComboLite 合规修复（Phase 0）

- 修改 `OpenListPluginEntry.kt`：
  - `pluginModule = emptyList()`
  - `onLoad(context)` 初始化 OpenListBridge
  - `onUnload()` shutdown OpenListBridge
  - `Content()` 返回**最小占位**（不实现 UI，所有 UI 移到 Capacitor 应用中）

### 变更二：plugin-openlist 改造为独立 Capacitor 应用（Phase 1）

- 新增 `OpenListMainActivity.kt`（extends BridgeActivity）
- 新增 `AndroidManifest.xml` 的 Activity 注册（独立 launcher）
- 新增 `assets/public/`（web 资源）
- 新增 `capacitor.config.json`（独立配置）
- 新增 `assets/capacitor.plugins.json`（插件注册）

### 变更三：新增 Capacitor 插件（Phase 1）

在 plugin-openlist 内部新增 3 个 Capacitor 插件供自己的 web 端调用：

- `OpenListServicePlugin.kt`：
  - `start()` → `OpenListBridge.start()`
  - `stop()` → `OpenListBridge.stop()`
  - `getStatus()` → `OpenListBridge.snapshot()`
  - `getVersion()` → 读 OpenListConfig.version
  - `isRunning()` → `OpenListService.isRunning`

- `OpenListConfigPlugin.kt`：
  - `read()` → 读 config.json
  - `write(content)` → 写 config.json
  - `getDataDir()` → 读 OpenListConfig.dataDir

- `OpenListPasswordPlugin.kt`：
  - `setPassword(pwd)` → `OpenListBridge.setAdminPwd(pwd)`

### 变更四：新增 Capacitor Web 项目（Phase 2）

`plugin-openlist/web/` 新增独立 Vite + Vue3 + Ionic Vue 项目，**复刻 K-Sillot Mobile UI**：

#### 4.1 页面清单（与 K-Sillot Mobile 一一对应）

| K-Sillot Mobile | 我们 plugin-openlist/web | 说明 |
|----------------|--------------------------|------|
| `lib/main.dart` | `src/main.ts` + `App.vue` | 入口 + IonTabs |
| `lib/pages/web/web.dart` | `src/views/WebScreen.vue` | Tab 0: WebView |
| `lib/pages/openlist/openlist.dart` | `src/views/OpenListScreen.vue` | Tab 1: 主控面板 |
| `lib/pages/openlist/log_list_view.dart` | `src/components/LogListView.vue` | 日志流组件 |
| `lib/pages/openlist/pwd_edit_dialog.dart` | `src/components/PwdEditDialog.vue` | 密码设置弹窗 |
| `lib/pages/openlist/config_editor_page.dart` | `src/components/ConfigEditorPage.vue` | config.json 编辑器 |
| `lib/pages/openlist/about_dialog.dart` | `src/components/AboutDialog.vue` | 关于弹窗 |
| `lib/pages/download_manager_page.dart` | `src/views/DownloadManager.vue` | Tab 2 |
| `lib/pages/settings/settings.dart` | `src/views/Settings.vue` | Tab 3 |

#### 4.2 K-Sillot 关键 UI 行为完整复刻

**OpenListScreen.vue**（主控面板）：

```vue
<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>OpenList - {{ openlistVersion }}</ion-title>
        <ion-buttons slot="end">
          <ion-button @click="showPwdEditDialog">
            <ion-icon :icon="keyOutline" />
          </ion-button>
          <ion-button @click="openConfigEditor">
            <ion-icon :icon="codeSlashOutline" />
          </ion-button>
          <ion-button @click="addShortcut">
            <ion-icon :icon="homeOutline" />
          </ion-button>
          <ion-button id="more-menu">
            <ion-icon :icon="ellipsisVertical" />
          </ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>
    <ion-content>
      <LogListView :logs="logs" />
    </ion-content>
    <ion-fab vertical="bottom" horizontal="end" slot="fixed">
      <ion-fab-button @click="toggleService">
        <ion-icon :icon="isRunning ? powerOutline : playOutline" />
      </ion-fab-button>
    </ion-fab>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { OpenListService } from '@/plugins/OpenListService'

const openlistVersion = ref('')
const isRunning = ref(false)
const logs = ref<Log[]>([])

onMounted(async () => {
  openlistVersion.value = await OpenListService.getVersion()
  // 监听状态变化（通过 Capacitor Events）
  // 监听日志（通过 OpenListBridge.setLogListener）
})

async function toggleService() {
  if (isRunning.value) {
    await OpenListService.stop()
  } else {
    logs.value = []
    await OpenListService.start()
  }
}
</script>
```

**WebScreen.vue**（Tab 0）：

```vue
<template>
  <ion-page>
    <ion-content>
      <iframe
        v-if="openlistUrl"
        :src="openlistUrl"
        :style="{ width: '100%', height: '100%', border: 'none' }"
        @error="handleError"
      />
      <div v-else class="empty-state">
        <ion-spinner />
        <p>OpenList 未启动，正在尝试启动...</p>
      </div>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { OpenListService } from '@/plugins/OpenListService'

const openlistUrl = ref('')

onMounted(async () => {
  const port = await OpenListService.getPort()
  if (port > 0) {
    openlistUrl.value = `http://127.0.0.1:${port}/#/login`
  } else {
    // 启动服务并轮询端口
    await OpenListService.start()
    setTimeout(async () => {
      const p = await OpenListService.getPort()
      openlistUrl.value = `http://127.0.0.1:${p}/#/login`
    }, 2000)
  }
})
</script>
```

#### 4.3 Capacitor Web 端 ↔ Native 桥接

`src/plugins/OpenListService.ts`：

```typescript
import { registerPlugin } from '@capacitor/core'

export interface OpenListServicePlugin {
  start(): Promise<{ success: boolean; port: number }>
  stop(): Promise<{ success: boolean }>
  getStatus(): Promise<{ running: boolean; port: number; pid: number; dataSizeBytes: number; lastError: string; lastUpdateTs: number }>
  getVersion(): Promise<string>
  getPort(): Promise<number>
}

const OpenListService = registerPlugin<OpenListServicePlugin>('OpenListService')
export { OpenListService }
```

### 变更五：主 ENCV app 不变（最小改动）

主 app 的 OpenList 相关代码**完全不变**：

- `LocalOpenListStatusCard.vue` — 保持只读状态摘要（通过 ContentProvider）
- `Remote.vue` — 不增加 OpenList UI
- `ExtensionsPage.vue` — OpenList 卡片增加一个 "启动 OpenList 独立界面" 按钮，**调主 app 的 Intent** 启动 plugin-openlist 的 OpenListMainActivity
- `GoProcess.ts` / `web.ts` — **不**移除 OpenList 相关方法（仍保留作 IPC 桥接）

新增一个 `openOpenListApp()` 函数：调主 app Capacitor 插件 `startOpenListApp()` → `context.startActivity(Intent(...).setComponent(ComponentName("com.encvgo.plugin.openlist", "com.encvgo.plugin.openlist.OpenListMainActivity")))`

---

## Impact

### Phase 0: ComboLite 合规

- **修改**: `plugin-openlist/OpenListPluginEntry.kt`

### Phase 1: plugin-openlist 改造

- **新增**: `plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListMainActivity.kt`
- **新增**: `plugin-openlist/src/main/java/com/encvgo/plugin/openlist/capacitor/OpenListServicePlugin.kt`
- **新增**: `plugin-openlist/src/main/java/com/encvgo/plugin/openlist/capacitor/OpenListConfigPlugin.kt`
- **新增**: `plugin-openlist/src/main/java/com/encvgo/plugin/openlist/capacitor/OpenListPasswordPlugin.kt`
- **修改**: `plugin-openlist/src/main/AndroidManifest.xml`（添加独立 launcher Activity）
- **新增**: `plugin-openlist/src/main/assets/capacitor.config.json`
- **新增**: `plugin-openlist/src/main/assets/capacitor.plugins.json`

### Phase 2: Capacitor Web 项目

- **新增**: `plugin-openlist/web/`（独立 npm 项目）
  - `package.json`（Vue 3, Ionic Vue, @capacitor/core, @capacitor/android, vue-router, vite）
  - `vite.config.ts`
  - `capacitor.config.ts`
  - `src/main.ts`
  - `src/App.vue`（IonTabs 4 tab）
  - `src/views/HomePage.vue` / `WebScreen.vue` / `OpenListScreen.vue` / `DownloadManager.vue` / `Settings.vue`
  - `src/components/LogListView.vue` / `PwdEditDialog.vue` / `ConfigEditorPage.vue` / `AboutDialog.vue`
  - `src/plugins/OpenListService.ts` / `OpenListConfig.ts` / `OpenListPassword.ts`
  - `src/router/index.ts`

### Phase 3: 主 app 集成

- **新增**: 主 app Capacitor 插件 `StartOpenListAppPlugin.kt` + TS
- **修改**: `src/components/LocalOpenListStatusCard.vue`（加一个"启动独立界面"按钮）
- **修改**: `src/views/ExtensionsPage.vue`（已安装 OpenList 卡片增加"启动"按钮）

### 不变文件

- `plugin-openlist/OpenListBridge.kt`
- `plugin-openlist/OpenListService.kt`（保留前台服务，Capacitor 插件可触发启停）
- `plugin-openlist/OpenListConfig.kt`
- `plugin-openlist/OpenListStatusProvider.kt`（主 app 仍通过它读状态）
- `android/combolite-host/OpenListStatusBridge.kt`
- `src/plugins/GoProcess.ts` / `web.ts`（保持 OpenList 桥接方法）

---

## ADDED Requirements

### Requirement: plugin-openlist 是独立 Capacitor 应用

`plugin-openlist` SHALL 包含一个独立的 Capacitor Android 应用入口 `OpenListMainActivity`，可从 launcher 或外部 Intent 启动。入口后呈现 K-Sillot Mobile 风格的 4 tab UI。

#### Scenario: 独立启动
- **WHEN** 用户从 launcher 点击 "OpenList" 图标（如果有 launcher intent-filter）
- **THEN** 启动 `OpenListMainActivity` → Capacitor 加载 web 资源 → 显示 4 tab HomePage

#### Scenario: 从主 app 调起
- **WHEN** 主 app 调用 `StartOpenListAppPlugin.start()`
- **THEN** 主 app 通过 Intent 启动 plugin-openlist 的 `OpenListMainActivity`

### Requirement: K-Sillot Mobile UI 完整复刻

`plugin-openlist/web/` 项目的 UI SHALL 复刻 K-Sillot/OpenList-Mobile 的 4 tab 布局和功能：

1. **Tab 0: WebScreen** — iframe 加载 `http://127.0.0.1:{port}/#/login`（OpenList 自带 SPA）
2. **Tab 1: OpenListScreen** — 主控面板（版本号 + 启停 FAB + 日志流 + 工具栏）
3. **Tab 2: DownloadManager** — 下载管理（可简化）
4. **Tab 3: Settings** — 应用设置（可简化）

#### Scenario: 启停服务
- **WHEN** 用户在 OpenListScreen 点击 FAB
- **THEN** 若 running=false → `OpenListService.start()` → FAB 图标变 power → 日志开始流入
- **THEN** 若 running=true → `OpenListService.stop()` → FAB 图标变 play → 日志停止

#### Scenario: 加载 OpenList Web UI
- **WHEN** 用户切换到 WebScreen tab
- **THEN** 检查 OpenList port → 若 0 则自动 start 并轮询 → iframe 加载 `http://127.0.0.1:{port}/#/login`

#### Scenario: 设置管理员密码
- **WHEN** 用户点击密码图标 → 输入密码 → 确认
- **THEN** `OpenListPassword.setPassword(pwd)` → 弹 snackbar 提示"已设置"

#### Scenario: 编辑 config.json
- **WHEN** 用户点击 config 图标
- **THEN** 跳转 ConfigEditorPage → 显示当前 config.json → 支持编辑 + 保存（自动备份 + JSON 校验）

### Requirement: Capacitor 插件（plugin-openlist 内部）

`plugin-openlist` 内部 SHALL 实现 3 个 Capacitor 插件供自己的 web 端调用：

- `OpenListServicePlugin`（start / stop / getStatus / getVersion / getPort）
- `OpenListConfigPlugin`（read / write / getDataDir）
- `OpenListPasswordPlugin`（setPassword）

#### Scenario: 启动服务
- **WHEN** web 端调 `OpenListService.start()`
- **THEN** 调 `OpenListBridge.start()` → 启动前台服务 → 返回 `{ success: true, port: 5244 }`

### Requirement: 主 app "启动 OpenList 独立界面" 入口

主 app 的 `LocalOpenListStatusCard.vue` 和 `ExtensionsPage.vue` 已安装 OpenList 卡片 SHALL 增加一个"启动 OpenList 独立界面"按钮，点击后调主 app Capacitor 插件 `StartOpenListAppPlugin` 启动 `OpenListMainActivity`。

#### Scenario: 从主 app 启动
- **WHEN** 用户在主 app 点击"启动 OpenList 独立界面"按钮
- **THEN** 主 app 调 `StartOpenListAppPlugin.start()` → Intent 启动 plugin-openlist 的 OpenListMainActivity

## MODIFIED Requirements

### Requirement: OpenListPluginEntry onLoad/onUnload 合规

`OpenListPluginEntry` SHALL 在 `onLoad(context)` 中初始化 OpenListBridge（参考 MpvPluginEntry 的 Content() 内初始化模式），`onUnload()` 中 shutdown。`pluginModule = emptyList()`。

## REMOVED Requirements

### Removed: plugin-openlist 内的 Compose Material3 UI

**Reason**: 
1. 违反原始 spec（"不分 Compose UI"）
2. K-Sillot Mobile 风格 UI 在 Capacitor 中实现（Web 技术）
3. 减小 APK 体积

**Migration**: 
- `StatusCard`, `ControlCard`, `ConfigCard` 删除
- 主控面板功能迁移到 `plugin-openlist/web/src/views/OpenListScreen.vue`
- 状态卡 → 仍由主 app `LocalOpenListStatusCard.vue` 显示（通过 ContentProvider IPC 读取）

### Removed: GoProcess 中的 OpenList 桥接（保留）

GoProcess 中的 `getOpenListRuntime` / `controlOpenList` **保留**（主 app 仍需要读 OpenList 状态）。但用户不再通过主 app 控制 OpenList——控制改在 OpenList 自己的 Capacitor 应用内。

---

## 架构总览（最终）

```
┌──────────────────────────────────────────────────────────────┐
│  plugin-openlist.apk                                         │
│  (独立 Capacitor Android 应用)                                │
│                                                              │
│  OpenListMainActivity (BridgeActivity)                       │
│    └─ Capacitor 加载 assets/public/                          │
│        └─ Vue 3 + Ionic Vue (Web 资源)                       │
│            └─ IonTabs (4 tab)                                │
│                ├─ WebScreen.vue → iframe(127.0.0.1:port)     │
│                ├─ OpenListScreen.vue (主控)                  │
│                ├─ DownloadManager.vue                        │
│                └─ Settings.vue                               │
│            └─ 调 Capacitor 插件                              │
│                ├─ OpenListServicePlugin ──┐                  │
│                ├─ OpenListConfigPlugin ───┤                  │
│                └─ OpenListPasswordPlugin ─┤                  │
│                                            │                  │
│  OpenListBridge (gomobile bind) ←────────────┘               │
│  OpenListService (Android 前台服务)                          │
│  OpenListConfig (config.json 持久化)                          │
│  OpenListStatusProvider (ContentProvider) ──┐                │
│                                              │                │
└──────────────────────────────────────────────│────────────────┘
                                               │ 跨进程 ContentProvider
┌──────────────────────────────────────────────│────────────────┐
│  com.encvgo.app (主 ENCV app)                │                │
│                                              ▼                │
│  OpenListStatusBridge.read() ◄─── content:// URI             │
│  LocalOpenListStatusCard.vue (只读状态摘要)                  │
│  ExtensionsPage.vue (install/uninstall + 启动独立界面)       │
│  StartOpenListAppPlugin → Intent 启动 OpenListMainActivity    │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```
