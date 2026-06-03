# OpenList 扩展重写 Spec（符合 ComboLite 规范 + 独立 UI）

## Why

当前 `plugin-openlist` 实现存在核心问题：**不符合 ComboLite 插件规范，且错误地在插件内嵌入了 Compose UI**。

### 问题一：违反原始 spec 设计决策

原始 spec（[integrate-openlist-as-combolite-plugin](../integrate-openlist-as-combolite-plugin/spec.md) §二）明确规定：

> `plugin-openlist/` 仿 `plugin-mpv-player/` 骨架，但**不分 Compose UI**（OpenList 自带 Web 管理 UI，WebView 直接打开 `http://127.0.0.1:5244/#/`）

但当前 `OpenListPluginEntry.kt` 内嵌了完整 Compose UI（StatusCard / ControlCard / ConfigCard 三个 @Composable 组件，~400 行），违反此设计决策。

### 问题二：OpenList 拥有完整的独立 UI，不应被破坏或替代

OpenList fork 内置了一套完整的 Vue3 SPA 管理界面（`public/dist/`），包含：
- 文件浏览/管理
- 存储驱动配置
- 用户权限管理
- ENCV 解密设置（fork 特有）
- 系统监控

这套 UI 是 OpenList 产品的一部分，**不应被主应用的 Ionic Vue 页面替代或嵌入**。主应用只负责：
1. 安装/卸载/启用/禁用 OpenList 插件（ComboLite 生命周期）
2. 展示 OpenList 运行时状态（running/port/pid）—— 仅摘要级别
3. 提供一个入口打开 OpenList 自有的管理界面

### 问题三："用 Capacitor 实现"的正解

用户问的是：**OpenList 扩展自身的 UI 能否用 Capacitor 技术承载？**

答案是：**可以，且应该这样做**。具体来说：
- OpenList 的 Web UI（Vue3 SPA）已经在 `127.0.0.1:5244` 服务
- 通过 **Capacitor Browser 插件** 或 **Plugin 内嵌 WebView** 来展示这个 UI
- 不需要在 Host App 的 Ionic Vue 里重建 OpenList 的管理功能
- Plugin APK 移除 Compose UI 后更轻量，专注运行时职责

## What Changes

### 一、移除 Plugin Compose UI（瘦身 plugin-openlist）—— 核心改动

从 `OpenListPluginEntry.kt` 删除所有 `@Composable` 组件。Plugin APK 职责回归纯运行时：

```
plugin-openlist/（重写后）
├── libs/
│   ├── openlist.aar              # gomobile bind 产物（不变）
│   └── openlist-classes.jar      # extracted classes（不变）
├── src/main/java/.../openlist/
│   ├── OpenListPluginEntry.kt    # ✅ 保留但 Content() 瘦身为空
│   ├── OpenListService.kt        # ✅ 保留（前台服务）
│   ├── OpenListBridge.kt         # ✅ 保留（gomobile 桥接）
│   ├── OpenListConfig.kt         # ✅ 保留（配置持久化）
│   └── OpenListStatusProvider.kt # ✅ 保留（ContentProvider IPC）
├── build.gradle.kts              # 🔧 移除 compose 相关依赖
```

`build.gradle.kts` 变更：
- **删除** `org.jetbrains.kotlin.plugin.compose` plugin
- **删除** `buildFeatures { compose = true }`
- **删除** 所有 `implementation(libs.compose.*)` 依赖
- **删除** `implementation("androidx.compose.material:material-icons-extended")`
- **删除** `implementation("androidx.lifecycle:lifecycle-runtime-compose")`

`OpenListPluginEntry.Content()` 改为返回最小占位：

```kotlin
@Composable
override fun Content() {
    // OpenList has its own full Web UI at http://127.0.0.1:5244
    // The host app opens it via Capacitor Browser.
    // This Composable is only a fallback when accessed via ComboLite
    // plugin activity proxy (rare).
}
```

### 二、Host App 侧——仅保留轻量状态摘要 + 入口按钮

**不做**：不在 Remote.vue / ExtensionsPage.vue 增加启停控制、配置编辑等管理功能。
**只做**：
1. `LocalOpenListStatusCard.vue` 保持为**只读状态摘要卡片**（PID/端口/运行状态/数据大小）
2. 卡片上增加一个 **"打开 OpenList"** 按钮 → 调用 Capacitor Browser 打开 `http://127.0.0.1:5244/#/login`
3. `ExtensionsPage.vue` 已安装的 OpenList 卡片保持现有行为（install/uninstall/toggle enabled）

### 三、OpenList Web UI 访问路径（Capacitor 承载）

| 场景 | 方式 | 说明 |
|------|------|------|
| **开发预览** | Vite proxy `/openlist-ui/` → `127.0.0.1:5244` | 已实现（见 `openlist-frontend-extraction` spec） |
| **生产 APK — 主应用内** | `@capacitor/browser` 的 `Browser.open()` | 在系统浏览器或自定义 tab 中打开 |
| **生产 APK — 插件内嵌（未来可选）** | Plugin Activity 内嵌 WebView 加载 `http://127.0.0.1:5244` | 本次不实现，留扩展点 |

### 四、ContentProvider IPC 保留不动

当前的跨进程通信架构完全正确，不做改动：

```
Host App (Ionic Vue Process)
  → GoProcess Plugin (Capacitor)
    → OpenListStatusBridge.read()     [ContentResolver.query]
      → Plugin APK (Independent Process)
        → OpenListStatusProvider.query() [MatrixCursor]
          → OpenListBridge.snapshot()   [synchronized Map]
```

## Impact

- Affected specs: `integrate-openlist-as-combolite-plugin`, `wire-openlist-runtime-and-ui-v2`
- Affected code:
  - **修改**: `plugin-openlist/OpenListPluginEntry.kt`（删除全部 Compose UI，~400 行 → ~30 行）
  - **修改**: `plugin-openlist/build.gradle.kts`（移除 compose 依赖，~15MB 瘦身）
  - **微调**: `src/components/LocalOpenListStatusCard.vue`（确保"打开 OpenList"按钮存在）
  - **不变**: `OpenListBridge.kt`, `OpenListStatusProvider.kt`, `OpenListService.kt`, `OpenListConfig.kt`
  - **不变**: `OpenListStatusBridge.kt`（host 侧 ContentProvider 桥接）
  - **不变**: `GoProcess.ts` 的 `getOpenListRuntime()` / `controlOpenList()`
  - **不变**: `useOpenListBridge.ts`（轮询逻辑保留）
  - **不变**: `ExtensionsPage.vue`（不增加管理功能）
  - **不变**: `Remote.vue`（不改造为管理面板）

## ADDED Requirements

### Requirement: Plugin APK 不包含任何 UI 层

`plugin-openlist` SHALL 是纯运行时插件，不含 Compose UI、不含 WebView、不含任何用户界面组件。`OpenListPluginEntry.Content()` 返回空 Composable。

#### Scenario: Plugin Content() 为空
- **WHEN** ComboLite 框架调用 `OpenListPluginEntry.Content()` 渲染插件页面
- **THEN** 返回空内容或纯文本提示"请通过主应用'打开 OpenList'按钮访问管理界面"

#### Scenario: build.gradle.kts 无 compose
- **WHEN** `./gradlew :plugin-openlist:assembleDebug` 执行
- **THEN** 编译成功且产出的 AAR 不包含任何 `androidx.compose.*` 类

### Requirement: OpenList 管理界面通过 Capacitor Browser 访问

Host App 提供"打开 OpenList"入口按钮，使用 `@capacitor/browser` 插件打开 OpenList 自有的 Web UI。

#### Scenario: 从 StatusCard 打开 OpenList
- **WHEN** 用户在 LocalOpenListStatusCard 点击"打开 OpenList"按钮且 running=true
- **THEN** `Browser.open({ url: 'http://127.0.0.1:5244/#/login', presentationStyle: 'popover' })` 打开 OpenList 管理 SPA

#### Scenario: OpenList 未运行时禁止打开
- **WHEN** OpenList running=false
- **THEN** "打开 OpenList"按钮 disabled 或隐藏，提示"请先启动 OpenList"

### Requirement: Host App 只展示状态摘要

Host App 的 OpenList 相关 UI SHALL 仅包含：
1. 安装/卸载/启用/禁用控制（ExtensionsPage 已有）
2. 运行时状态摘要（LocalOpenListStatusCard 已有：PID/端口/running/数据大小）
3. 打开 Web UI 入口按钮（需确认已有）

Host App SHALL NOT 包含：启停控制按钮、端口/密码配置表单、存储驱动管理等 OpenList 内置功能。

## MODIFIED Requirements

### Requirement: OpenListPluginEntry.onLoad（简化版）

`onLoad(context)` SHALL 仅注册 Koin module 和初始化 Bridge（不启动 Service）。`Content()` 返回瘦身后不再引用任何 UI 组件。

### Requirement: LocalOpenListStatusCard 保持只读

`LocalOpenListStatusCard.vue` SHALL 保持为只读状态展示组件，不增加 start/stop/configure 交互功能。唯一新增的是"打开 OpenList"入口按钮。

## REMOVED Requirements

### Removed: Plugin Compose UI（StatusCard / ControlCard / ConfigCard）

**Reason**:
1. 违反原始 spec 设计决策（"不分 Compose UI"）
2. 与 OpenList 自有完整 Web UI 功能重复
3. 增加 plugin AAR 体积 ~15MB（compose 依赖）
4. 用户明确表示 OpenList 有独立 UI 不能嵌入主应用

**Migration**: 删除所有 Compose UI 代码。OpenList 的管理功能完全由其自有 Web UI（`127.0.0.1:5244`）通过 Capacitor Browser 承载。

### Removed: Host App 内建 OpenList 管理面板

**Reason**: 用户明确反馈"openlist扩展有一整套独立的ui，不能嵌入主应用破坏"。在主应用的 Ionic Vue 中重建 OpenList 管理功能会破坏 OpenList 产品的独立性。

**Migration**: Host App 只做状态摘要 + 入口按钮。所有管理操作在 OpenList 自有 Web UI 中完成。

---

## 架构对比

```
【重写前 — 错误：Plugin 内嵌 Compose UI】
┌─────────────────────────────────┐
│  Plugin APK (独立进程)           │
│  ├─ Compose: StatusCard         │ ❌ 违反 spec
│  ├─ Compose: ControlCard        │ ❌ 与 Web UI 重复
│  ├─ Compose: ConfigCard         │ ❌ 不应存在
│  ├─ OpenListBridge (运行时)      │ ✅
│  └─ OpenListStatusProvider      │ ✅
├─────────────────────────────────┤
│  Host App (Ionic Vue)            │
│  ├─ LocalOpenListStatusCard     │ ✅ 只读摘要
│  └─ ExtensionsPage              │ ✅ 生命周期管理
├─────────────────────────────────┤
│  OpenList Web UI (Vue3 SPA)      │ ⚠️ 存在但无直接入口
│  └─ http://127.0.0.1:5244       │
└─────────────────────────────────┘

【重写后 — 正确：独立 UI + Capacitor 承载】
┌─────────────────────────────────┐
│  Plugin APK (独立进程)           │
│  ├─ Content(): 空               │ ✅ 瘦身
│  ├─ OpenListBridge (运行时)      │ ✅ 不变
│  ├─ OpenListService (前台服务)    │ ✅ 不变
│  └─ OpenListStatusProvider      │ ✅ 不变
├─────────────────────────────────┤
│  Host App (Ionic Vue)            │
│  ├─ LocalOpenListStatusCard     │ ✅ 只读 + "打开"按钮
│  │   └─ [打开 OpenList]          │ → Browser.open(5244)
│  └─ ExtensionsPage              │ ✅ install/uninstall/toggle
├─────────────────────────────────┤
│  OpenList Web UI (Vue3 SPA)      │ ✅ 唯一管理界面
│  └─ http://127.0.0.1:5244       │ ← Capacitor Browser 承载
└─────────────────────────────────┘
```

## 设计原则

| 原则 | 说明 |
|------|------|
| **UI 独立性** | OpenList 的管理 UI 是其产品边界，不由 Host App 重建或嵌入 |
| **Plugin 瘦身** | Plugin APK 只负责运行时（Bridge + Service + Provider），零 UI 依赖 |
| **Capacitor 作为载体** | Host App 用 Capacitor Browser 插件打开 OpenList Web UI，而非用 Ionic Vue 重建 |
| **IPC 只读状态** | Host App 通过 ContentProvider 读运行时状态（摘要级），不写入控制命令（除 install/uninstall/toggle 外） |
| **最小改动** | 不修改 Remote.vue 为管理面板、不新建 useOpenListManager、不改 ExtensionsPage 的交互逻辑 |
