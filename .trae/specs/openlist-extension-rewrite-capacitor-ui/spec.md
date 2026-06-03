# OpenList 扩展重写 Spec（符合 ComboLite 规范 + Capacitor UI）

## Why

当前 `plugin-openlist` 实现存在两个核心问题：

**问题一：不符合 ComboLite 插件规范**

原始 spec（[integrate-openlist-as-combolite-plugin](../integrate-openlist-as-combolite-plugin/spec.md) §二）明确规定：
> `plugin-openlist/` 仿 `plugin-mpv-player/` 骨架，但**不分 Compose UI**（OpenList 自带 Web 管理 UI，WebView 直接打开 `http://127.0.0.1:5244/#/`）

但当前 `OpenListPluginEntry.kt` 内嵌了完整 Compose UI（StatusCard / ControlCard / ConfigCard 三个 @Composable 组件，~400 行），违反了原始设计决策。

**问题二：UI 架构冗余**

| 层 | 当前实现 | 问题 |
|---|---------|------|
| OpenList Web UI | fork 内置 Vue3 SPA（`public/dist/`） | 已有完整管理界面 |
| Plugin Compose UI | `OpenListPluginEntry.Content()` 的 3 个 Card | 与 Web UI 功能重复 |
| Host Ionic Vue | `LocalOpenListStatusCard.vue` + `useOpenListBridge.ts` | 又一套状态展示 |

三层 UI 做同一件事：展示 OpenList 运行状态 + 启停控制 + 配置修改。这导致：
- Plugin AAR 体积因 Compose 依赖膨胀 ~15MB
- 状态同步逻辑在三个层各自维护（Bridge.snapshot / ContentProvider / TS polling）
- 用户需要在不同界面间切换才能完成完整操作

**用户诉求**：去掉插件内的 Compose UI，改用 Capacitor（Ionic Vue）实现 OpenList 扩展的管理界面——与宿主 App 统一技术栈、统一设计语言、零 Compose 依赖。

## What Changes

### 一、移除 Plugin Compose UI（瘦身 plugin-openlist）

从 `OpenListPluginEntry.kt` 删除所有 `@Composable` 组件（StatusCard / ControlCard / ConfigCard），`Content()` 返回空 Composable 或最小占位。Plugin APK 职责回归纯运行时：

```
plugin-openlist/（重写后）
├── libs/
│   ├── openlist.aar              # gomobile bind 产物（不变）
│   └── openlist-classes.jar      # extracted classes（不变）
├── src/main/java/.../openlist/
│   ├── OpenListPluginEntry.kt    # ✅ 保留但 Content() 瘦身
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

### 二、Host 侧 Capacitor UI 替代（新增/改造）

OpenList 的所有用户交互通过宿主 App 的 Ionic Vue 层实现：

#### 2.1 Remote.vue 增强（替代 Plugin Compose 的 StatusCard + ControlCard）

当前 `Remote.vue` 已有 `<LocalOpenListStatusCard>` 基础组件。需增强为完整的 OpenList 管理面板：

| 功能 | 对应原 Compose 组件 | 实现 |
|------|---------------------|------|
| 状态展示（PID/端口/数据大小/心跳） | `StatusCard` | 增强 `LocalOpenListStatusCard.vue` |
| 启停控制 | `ControlCard` | 新增 Start/Stop 按钮 |
| 配置编辑（端口/密码） | `ConfigCard` | 新增配置区域或跳转 Settings |
| 打开 Web UI | 无（原设计用外部浏览器） | "打开管理界面" 按钮 → InAppBrowser |

#### 2.2 ExtensionsPage.vue 增强（安装后的管理入口）

已安装 OpenList 后，扩展卡片增加操作：
- "管理" 按钮 → 导航到 Remote tab 的 OpenList 面板
- 或直接在卡片内展开状态摘要

#### 2.3 新增 `useOpenListManager` composable（统一状态管理）

合并 `useOpenListBridge.ts` 的轮询逻辑 + 控制动作（start/stop/configure），提供：
```typescript
// useOpenListManager.ts
interface OpenListManagerState {
  runtime: OpenListRuntime       // 来自 getOpenListRuntime()
  isControlling: boolean          // 操作中锁
  error: string | null            // 最近错误
}

function useOpenListManager() {
  const { runtime, start, stop, configure, refresh }
  // start() → controlOpenList('start')
  // stop() → controlOpenList('stop')
  // configure({ port, password }) → controlOpenList('set_admin_password') + ...
}
```

### 三、ContentProvider IPC 保留不动

当前的跨进程通信架构完全正确，不做改动：

```
Host App (Ionic Vue Process)
  → GoProcess Plugin (Capacitor)
    → OpenListStatusBridge.read()     [ContentResolver.query]
      → Plugin APK (Independent Process)
        → OpenListStatusProvider.query() [MatrixCursor]
          → OpenListBridge.snapshot()   [synchronized Map]
```

这是唯一正确的跨进程状态读取方式，已在 `wire-openlist-runtime-and-ui-v2` spec 中验证过。

### 四、OpenList Web UI 访问方式

| 场景 | 方式 | 说明 |
|------|------|------|
| **开发预览** | Vite proxy `/openlist-ui/` → `127.0.0.1:5244` | 已实现（见 `openlist-frontend-extraction` spec） |
| **生产 APK** | Capacitor Browser 插件 | `Browser.open({ url: 'http://127.0.0.1:5244/#/login' })` |
| **嵌入式 WebView** | 未来可选 | 在 Host Activity 内嵌 WebView 加载 OpenList SPA（本次不实现） |

## Impact

- Affected specs: `integrate-openlist-as-combolite-plugin`, `wire-openlist-runtime-and-ui-v2`, `openlist-frontend-extraction-and-sandbox-preview`
- Affected code:
  - **修改**: `plugin-openlist/OpenListPluginEntry.kt`（删除 Compose UI）
  - **修改**: `plugin-openlist/build.gradle.kts`（移除 compose 依赖）
  - **修改**: `src/views/Remote.vue`（增强为完整管理面板）
  - **修改**: `src/components/LocalOpenListStatusCard.vue`（增加启停控制）
  - **新增**: `src/composables/useOpenListManager.ts`（统一管理 composable）
  - **修改**: `src/views/ExtensionsPage.vue`（已安装状态的快捷操作）
  - **不变**: `OpenListBridge.kt`, `OpenListStatusProvider.kt`, `OpenListService.kt`, `OpenListConfig.kt`
  - **不变**: `OpenListStatusBridge.kt`（host 侧 ContentProvider 桥接）
  - **不变**: `GoProcess.ts` 的 `getOpenListRuntime()` / `controlOpenList()`

## ADDED Requirements

### Requirement: Plugin APK 不包含 Compose UI

`plugin-openlist` 的 `OpenListPluginEntry.Content()` SHALL 返回最小 Composable（空 Box 或纯文本标签），不包含任何状态卡片、控制按钮或配置表单。所有用户交互通过 Host App 的 Ionic Vue 层实现。

#### Scenario: Plugin Content() 瘦身
- **WHEN** ComboLite 框架调用 `OpenListPluginEntry.Content()` 渲染插件页面
- **THEN** 显示一个简洁的"请使用主应用 OpenList 管理面板"提示信息，不包含 Compose Material3 组件依赖

#### Scenario: build.gradle.kts 移除 compose
- **WHEN** `./gradlew :plugin-openlist:assembleDebug` 执行
- **THEN** 编译成功且产出的 AAR 不包含任何 `androidx.compose.*` 类

### Requirement: Host Ionic Vue 提供 OpenList 完整管理面板

`src/views/Remote.vue` 或独立的 `OpenListManagePage.vue` SHALL 包含以下全部功能（替代被移除的 Plugin Compose UI）：

1. **状态卡**：running/port/pid/dataSizeBytes/lastError/lastUpdateTs（已有基础，需增强）
2. **控制区**：Start / Stop 按钮（需新增，调 `controlOpenList()`）
3. **配置区**：端口编辑框 / 管理员密码设置（需新增）
4. **Web UI 入口**："打开管理界面"按钮 → Capacitor Browser 打开 `http://127.0.0.1:5244/#/login`（需新增）

#### Scenario: 从 Ionic Vue 启动 OpenList
- **WHEN** 用户点击"启动"按钮
- **THEN** 调用 `controlOpenList('start')` → ContentProvider.insert → OpenListBridge.start() → 5s 内状态变为 running=true

#### Scenario: 从 Ionic Vue 停止 OpenList
- **WHEN** 用户点击"停止"按钮
- **THEN** 调用 `controlOpenList('stop')` → ContentProvider.insert → OpenListBridge.shutdown() → 状态变为 running=false

#### Scenario: 打开 OpenList Web UI
- **WHEN** 用户点击"打开管理界面"
- **THEN** Capacitor Browser 插件打开 `http://127.0.0.1:5244/#/login`（仅在 running=true 时可用）

### Requirement: useOpenListManager 统一 Composable

新建 `src/composables/useOpenListManager.ts` SHALL 封装所有 OpenList 状态读写操作：

- `runtime`: reactive ref of `OpenListRuntime`（3s 自动刷新）
- `start()`: async → `controlOpenList('start')` + 刷新
- `stop()`: async → `controlOpenList('stop')` + 刷新
- `setAdminPassword(pwd)`: async → `controlOpenList('set_admin_password', { password: pwd })`
- `updatePort(port)`: 通过 API 或 Provider 更新端口
- `isControlling`: 操作中防重复提交锁

#### Scenario: composable 自动轮询
- **WHEN** 组件调用 `const { runtime, start, stop } = useOpenListManager()`
- **THEN** 每 3 秒自动调用 `getOpenListRuntime()` 并更新 `runtime` ref；组件卸载时自动清理定时器

### Requirement: ExtensionsPage 已安装状态快捷操作

`ExtensionsPage.vue` 中已安装的 OpenList 卡片 SHALL 增加：
- 状态指示器（运行中绿色圆点 / 已停止灰色）
- "管理"按钮 → push 到 OpenList 管理面板

#### Scenario: 已安装 OpenList 显示管理入口
- **WHEN** OpenList 插件 installed=true 且 enabled=true
- **THEN** 卡片显示运行状态 + "管理"按钮（替代当前的 enable/disable 切换按钮）

## MODIFIED Requirements

### Requirement: OpenListPluginEntry.onLoad（简化版）

`onLoad(context)` SHALL 仅注册 Koin module 和初始化 Bridge（不启动 Service）。`Content()` 返回瘦身后不再引用 Compose Material3 组件。

### Requirement: LocalOpenListStatusCard 增强

现有 `LocalOpenListStatusCard.vue` SHALL 从"只读状态展示"升级为"可交互管理面板"，集成 `useOpenListManager` 的 start/stop/configure 方法。

## REMOVED Requirements

### Removed: Plugin Compose UI（StatusCard / ControlCard / ConfigCard）

**Reason**: 与 OpenList Web UI 功能重复；违反原始 spec 设计决策（"不分 Compose UI"）；增加 plugin AAR 体积和维护成本；用户明确要求用 Capacitor UI 替代。
**Migration**: 所有功能迁移到 Host App Ionic Vue 层（`Remote.vue` + `useOpenListManager.ts`）。Plugin `Content()` 返回最小占位。

---

## 架构对比

```
【重写前 — 三层 UI 冗余】
┌─────────────────────────────────┐
│  Plugin APK (独立进程)           │
│  ├─ Compose: StatusCard         │ ← 删除
│  ├─ Compose: ControlCard        │ ← 删除
│  └─ Compose: ConfigCard         │ ← 删除
├─────────────────────────────────┤
│  Host App (Ionic Vue)            │
│  ├─ LocalOpenListStatusCard.vue │ ← 保留但增强
│  └─ useOpenListBridge.ts        │ ← 改造为 useOpenListManager
├─────────────────────────────────┤
│  OpenList Web UI (Vue3 SPA)      │
│  └─ http://127.0.0.1:5244       │ ← 保留（InAppBrowser 打开）
└─────────────────────────────────┘

【重写后 — 单一 Capacitor UI】
┌─────────────────────────────────┐
│  Plugin APK (独立进程)           │
│  ├─ OpenListBridge (运行时)      │ ← 保留不变
│  ├─ OpenListStatusProvider (IPC)│ ← 保留不变
│  └─ Content(): "See host app"    │ ← 瘦身
├─────────────────────────────────┤
│  Host App (Ionic Vue)            │
│  ├─ Remote.vue (完整管理面板)    │ ← 新增：状态+控制+配置+WebUI入口
│  ├─ useOpenListManager.ts        │ ← 新增：统一 composable
│  └─ ExtensionsPage.vue (快捷入口)│ ← 增强
├─────────────────────────────────┤
│  OpenList Web UI (Vue3 SPA)      │
│  └─ Browser.open(5244)           │ ← Capacitor Browser 调起
└─────────────────────────────────┘
```
