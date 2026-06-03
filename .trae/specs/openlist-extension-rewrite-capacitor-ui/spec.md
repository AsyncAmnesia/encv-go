# OpenList 扩展重写 Spec（Capacitor 多例 + ComboLite 合规）

## Why

用户对当前 spec 反馈"还是很乱"，并明确要求"必须实现 Capacitor 多例"。

### 核心问题诊断

"Capacitor 多例" 在此项目语境下的含义：

**本项目已有模式参考**：`MpvEmbedService` 模式——每次调用 `GoProcess.startMpvInPlace({filePath, name})` 都会让 Native 侧在指定 `containerId` 容器中**动态实例化一个内嵌视图**（`MpvEngine`）。**同一个 Capacitor 插件方法可被多次调用、每次产出独立的内嵌视图实例**——这就是"Capacitor 多例"。

```
TS: GoProcess.startMpvInPlace({ filePath: "a.mp4" }) → Native: MpvEmbedService.startEmbed(containerId="mpv-1") → 实例1
TS: GoProcess.startMpvInPlace({ filePath: "b.mp4" }) → Native: MpvEmbedService.startEmbed(containerId="mpv-2") → 实例2
                                                                每次都新实例，containerId 区分
```

### OpenList 现状 vs 需求

| 维度 | 现状 | 需求 |
|------|------|------|
| **UI 承载** | `OpenListPluginEntry.Content()` 内嵌 Compose (~400行) | **Capacitor 插件多例内嵌**（与 MpvEmbed 同模式） |
| **多例支持** | 单一 Content() 只能一个实例 | 多次调用支持同时打开多个独立视图 |
| **ComboLite 合规** | 违反（onLoad 几乎为空、Service 生命周期解耦） | **必须修复** |
| **OpenList Web UI** | 缺失（仅 Plugin APK 的 Compose UI） | 通过 Capacitor 多例内嵌 WebView 加载 `http://127.0.0.1:{port}` |

### 用户已明确否决的方向

- ❌ 把 OpenList 完整 UI 塞到主应用 Ionic Vue（用户："谁让你塞完整ui到主应用的？"）
- ✅ OpenList 扩展的 UI 在**插件自身进程内**实现
- ✅ 用 **Capacitor 多例** 模式承载（与 MpvEmbedService 一致）

---

## Part A: 架构总览（重写后）

### A.1 Capacitor 多例模式（参考 MpvEmbedService）

```
┌─ Host App Process (Capacitor + Ionic Vue) ─────────────────────┐
│                                                                 │
│  Vue 组件 <div id="openlist-container-A">                       │
│       │                                                         │
│       │ await OpenListEmbed.open({                              │
│       │   containerId: 'openlist-container-A',                  │
│       │   port: 5244,                                           │
│       │   path: '/#/login'                                      │
│       │ })                                                      │
│       │   ↓ Capacitor Bridge                                    │
│       └────→ OpenListEmbedPlugin.open()                         │
│                                                                 │
│  Vue 组件 <div id="openlist-container-B">                       │
│       │                                                         │
│       │ await OpenListEmbed.open({ containerId: '...-B', ... }) │
│       └────→ 同一个插件方法，再次调用 → 新实例                    │
│                                                                 │
│  Host App 内：零 OpenList 业务逻辑代码                             │
│    (只定义容器 div + 触发 OpenListEmbed.open)                    │
│                                                                 │
├─ Native Plugin (在 com.encvgo.app 中，与 GoProcess 平级) ───────┤
│                                                                 │
│  OpenListEmbedPlugin (Capacitor Plugin)                         │
│    ├─ @CapacitorPlugin(name = "OpenListEmbed")                  │
│    ├─ @PluginMethod fun open(call) {                            │
│    │     val containerId = call.getString("containerId")        │
│    │     val port = call.getInt("port")                         │
│    │     val path = call.getString("path") ?: "/"               │
│    │     OpenListEmbedService.startEmbed(                       │
│    │         activity, containerId, port, path)                 │
│    │ }                                                          │
│    ├─ @PluginMethod fun close(call) { ... }                     │
│    └─ @PluginMethod fun setBounds(call) { ... }                 │
│                                                                 │
│  OpenListEmbedService (多例管理器)                               │
│    ├─ instances: ConcurrentHashMap<String, OpenListEmbedInstance> │
│    ├─ startEmbed(activity, containerId, port, path): Instance   │
│    ├─ stopEmbed(containerId)                                    │
│    ├─ findViewByContainerId(): WebView?                         │
│    └─ 每个 instance 独立生命周期（独立 WebView + CookieStore）   │
│                                                                 │
├─ ComboLite Plugin APK (plugin-openlist) ────────────────────────┤
│                                                                 │
│  OpenListPluginEntry.Content() → 最小占位（删 Compose UI）       │
│  OpenListBridge (gomobile 桥接, 不变)                            │
│  OpenListService (前台服务, 不变)                                │
│  OpenListStatusProvider (ContentProvider IPC, 不变)             │
│  OpenListConfig (配置持久化, 不变)                               │
│                                                                 │
├─ OpenList Web UI (Vue3 SPA) ───────────────────────────────────┤
│  http://127.0.0.1:{port}/#/                                     │
│  被多例 WebView 加载                                              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### A.2 多例 Container 管理

OpenListEmbedService 用 `containerId` 区分多个实例：

```kotlin
class OpenListEmbedService {
    private val instances = ConcurrentHashMap<String, OpenListEmbedInstance>()

    data class OpenListEmbedInstance(
        val containerId: String,
        val webView: WebView,
        val port: Int,
        val createdAt: Long
    )

    fun startEmbed(activity, containerId, port, path): OpenListEmbedInstance {
        if (instances.containsKey(containerId)) {
            return instances[containerId]!!  // 已存在则复用
        }
        val wv = WebView(activity).apply {
            webViewClient = WebViewClient()
            loadUrl("http://127.0.0.1:$port$path")
        }
        val inst = OpenListEmbedInstance(containerId, wv, port, System.currentTimeMillis())
        instances[containerId] = inst
        return inst
    }

    fun stopEmbed(containerId) {
        instances.remove(containerId)?.webView?.destroy()
    }
}
```

### A.3 完整调用链

```
1. 用户在主应用点击"打开 OpenList A"按钮
2. Vue: await OpenListEmbed.open({ containerId: 'A', port: 5244 })
3. Native: OpenListEmbedPlugin.open() → OpenListEmbedService.startEmbed('A', 5244)
4. Native: WebView 创建 → loadUrl(http://127.0.0.1:5244/#/login)
5. Native: 通过 ContainerManager 把 WebView 添加到 Capacitor 容器布局
6. 用户可同时点击"打开 OpenList B" → 独立 WebView 实例加载
7. 用户关闭 A → open({ containerId: 'A' }) 不存在实例时再开
8. 用户关闭页面 → OpenListEmbed.close({ containerId: 'A' }) → WebView.destroy()
```

---

## Part B: ComboLite 合规修复（不依赖 UI）

### B.1 修复项

```kotlin
// 修复后 OpenListPluginEntry.kt
class OpenListPluginEntry : IPluginEntryClass {
    override val pluginModule = emptyList<Module>()  // 删除 OpenListBridge 注册

    override fun onLoad(context: PluginContext) {
        // 由 OpenListEmbedService（host 侧）按需启动 OpenList
        // 不再独立启动 Android Service
    }

    override fun onUnload() {
        // 卸载时清理：OpenListEmbedService 会在所有 WebView 关闭后 stop
    }

    @Composable
    override fun Content() {
        // 空/最小占位：所有 UI 由 host 侧 OpenListEmbed 多例承载
        Box {} // 0 行 Compose 业务代码
    }
}
```

### B.2 删除的 Compose UI 代码

- `StatusCard` (90 行)
- `ControlCard` (60 行)
- `ConfigCard` (65 行)
- `InfoGrid` (35 行)
- `formatFileSize` (10 行)
- 删除所有 `androidx.compose.foundation.*`, `material3.*`, `material.icons.*`, `lifecycle.*` import

### B.3 build.gradle.kts 变更

- **删除** `id("org.jetbrains.kotlin.plugin.compose")`
- **删除** `buildFeatures { compose = true }`
- **删除** compose BOM + ui/runtime/material3/icons-extended/lifecycle-runtime-compose
- **保留** combolite-core, openlist-classes.jar, core-ktx, localbroadcastmanager, koin-core

### B.4 删除的运行时逻辑

当前 OpenListPluginEntry 的 `onLoad()` 已几乎为空，运行时由独立 `OpenListService` 启动。但 OpenListService 应改为：
- **不自动启动**（由 host 侧 `OpenListEmbed.open()` 触发）
- **最后一个 WebView 关闭后延迟 stop**（资源回收）

---

## What Changes

### 变更一：新增 Capacitor 插件（host app 模块）

**新增文件**：

```
android/app/src/main/java/com/encvgo/app/openlist/
├── OpenListEmbedPlugin.kt          # Capacitor 插件入口
├── OpenListEmbedService.kt          # 多例管理器
└── OpenListEmbedInstance.kt         # 单例数据类
```

**注册插件**：在 `MainActivity.onCreate` 调用 `registerPlugin(OpenListEmbedPlugin::class.java)`（如果尚未自动注册）。

### 变更二：TypeScript 插件定义

**修改** `src/plugins/GoProcess.ts`：
- 删除 `getOpenListRuntime()` 和 `controlOpenList()`（迁至新插件）
- 改用新的 `OpenListEmbed` 插件

**新增** `src/plugins/OpenListEmbed.ts`：
```typescript
export interface OpenListEmbedPlugin {
  open(options: { containerId: string; port: number; path?: string }): Promise<{ success: boolean }>
  close(options: { containerId: string }): Promise<{ success: boolean }>
  setBounds(options: { containerId: string; x: number; y: number; width: number; height: number }): Promise<void>
  isLoaded(options: { containerId: string }): Promise<{ loaded: boolean }>
  getOpenListRuntime(): Promise<OpenListRuntime>  // 替代 GoProcess 中的同名方法
  controlOpenList(action: 'start' | 'stop' | 'set_admin_password', args: Record<string, any>): Promise<boolean>
  navigate(options: { containerId: string; path: string }): Promise<void>
}
```

### 变更三：Vue 组件（仅作为容器，不含业务逻辑）

**新增** `src/components/OpenListEmbedContainer.vue`：

```vue
<template>
  <div :id="containerId" :style="containerStyle">
    <!-- 占位：Capacitor 会在此 id 注入 WebView -->
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { OpenListEmbed, getOpenListRuntime, controlOpenList } from '@/plugins/OpenListEmbed'

const props = defineProps<{ containerId: string; autoStart?: boolean }>()

onMounted(async () => {
  if (props.autoStart) {
    const rt = await getOpenListRuntime()
    if (!rt.running) await controlOpenList('start')
  }
  // 等待 Vue 渲染出 div，再让 Native 挂 WebView
  await nextTick()
  const runtime = await getOpenListRuntime()
  await OpenListEmbed.open({
    containerId: props.containerId,
    port: runtime.port || 5244,
    path: '/'
  })
})

onUnmounted(async () => {
  await OpenListEmbed.close({ containerId: props.containerId })
})
</script>
```

> 关键：Vue 组件**不重建 OpenList UI**，只是声明一个 `containerId` 容器，Capacitor Native 把 WebView 注入到该 DOM 节点。

### 变更四：Plugin APK（plugin-openlist）瘦身

- 删除 `OpenListPluginEntry` 中所有 Compose UI
- `build.gradle.kts` 移除 compose 依赖
- `OpenListService` 改为按需启动（不再 boot 时自启）
- `OpenListConfig`, `OpenListBridge`, `OpenListStatusProvider`, `OpenListStatusBridge` **不变**

### 变更五：Host App 集成

**Remote.vue / ExtensionsPage.vue**：
- 提供一个简单的入口，**导航到包含 OpenListEmbedContainer 的新页面**
- 例如：Remote tab 增加按钮"打开 OpenList" → 跳转新页面
- 新页面内挂载 `<OpenListEmbedContainer container-id="openlist-A" auto-start />`

**LocalOpenListStatusCard.vue**：
- 改用 `getOpenListRuntime()` 从新 `OpenListEmbed` 插件获取
- 状态展示保持只读摘要

---

## Impact

### 新增文件
- `android/app/src/main/java/com/encvgo/app/openlist/OpenListEmbedPlugin.kt`
- `android/app/src/main/java/com/encvgo/app/openlist/OpenListEmbedService.kt`
- `android/app/src/main/java/com/encvgo/app/openlist/OpenListEmbedInstance.kt`
- `src/plugins/OpenListEmbed.ts`
- `src/plugins/openlist-embed/web.ts` (Web stub)
- `src/components/OpenListEmbedContainer.vue`
- `src/views/OpenListPage.vue` (入口页面)

### 修改文件
- `android/app/src/main/java/com/encvgo/app/MainActivity.kt` (注册插件)
- `android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt` (移除 OpenList 方法)
- `src/plugins/GoProcess.ts` (移除 OpenList 相关导出)
- `src/plugins/web.ts` (移除 OpenList 相关接口)
- `src/components/LocalOpenListStatusCard.vue` (改用新插件)
- `src/views/Remote.vue` (添加 OpenList 入口)
- `plugin-openlist/OpenListPluginEntry.kt` (删除 Compose UI)
- `plugin-openlist/OpenListService.kt` (按需启动)
- `plugin-openlist/build.gradle.kts` (移除 compose)

### 不变文件
- `plugin-openlist/OpenListBridge.kt`
- `plugin-openlist/OpenListStatusProvider.kt`
- `plugin-openlist/OpenListConfig.kt`
- `android/combolite-host/OpenListStatusBridge.kt`

## ADDED Requirements

### Requirement: OpenListEmbed Capacitor 插件（多例）

新增 `OpenListEmbedPlugin` 暴露以下方法给 JS 侧：

- `open({ containerId, port, path? })` → 创建/复用 WebView 并挂载到指定容器
- `close({ containerId })` → 销毁指定 WebView
- `setBounds({ containerId, x, y, width, height })` → 调整 WebView 位置和大小
- `isLoaded({ containerId })` → 查询 WebView 是否已挂载
- `navigate({ containerId, path })` → 在已挂载的 WebView 内导航
- `getOpenListRuntime()` → 替代 `GoProcess.getOpenListRuntime()`，从 host 侧 `OpenListStatusBridge` 读取
- `controlOpenList(action, args)` → 替代 `GoProcess.controlOpenList()`

#### Scenario: 多实例独立打开
- **WHEN** Vue 调用 `OpenListEmbed.open({ containerId: 'A', port: 5244 })` 且 'A' 不存在
- **THEN** Native 创建新 WebView → 挂载到 'A' → 加载 `http://127.0.0.1:5244/`

- **WHEN** Vue 调用 `OpenListEmbed.open({ containerId: 'B', port: 5244 })` 且 'B' 不存在
- **THEN** Native 创建另一个独立 WebView → 挂载到 'B' → 独立加载（与 A 完全隔离）

- **WHEN** 再次调用 `OpenListEmbed.open({ containerId: 'A' })` 已存在
- **THEN** 复用现有实例，不创建新 WebView

#### Scenario: 关闭单实例不影响其他
- **WHEN** Vue 调用 `OpenListEmbed.close({ containerId: 'A' })`
- **THEN** 销毁 'A' 的 WebView，'B' 实例不受影响

#### Scenario: getOpenListRuntime 替代
- **WHEN** Vue 调用 `OpenListEmbed.getOpenListRuntime()`
- **THEN** 返回 `{ isInstalled, running, port, pid, dataSizeBytes, lastError, lastUpdateTs }`

### Requirement: 插件内 UI 完全由 host 多例承载

`plugin-openlist/OpenListPluginEntry.Content()` SHALL 返回最小占位 Composable（`<Box />`），不包含任何 UI 组件。所有 OpenList 用户交互通过 host 侧的 `OpenListEmbed` 插件多例承载。

### Requirement: OpenListEmbedService 多例管理

`OpenListEmbedService` SHALL 用 `ConcurrentHashMap<containerId, OpenListEmbedInstance>` 管理多个独立 WebView 实例。每个实例独立管理 WebView、Cookie 存储、加载状态。

#### Scenario: 同一 containerId 多次 open 复用
- **WHEN** 同一 `containerId` 多次调用 `open()`
- **THEN** 复用第一次创建的 WebView 实例（不创建重复实例）

### Requirement: OpenListService 按需启动

`OpenListService` SHALL 改为按需启动：第一个 `OpenListEmbed.open()` 触发时启动；所有 WebView 关闭后延迟 N 秒（可配，默认 30s）停止。

#### Scenario: 第一个 open 触发启动
- **WHEN** `OpenListEmbed.open()` 第一次被调用且 OpenList 未运行
- **THEN** 调用 `OpenListStatusBridge.control('start')` 启动 OpenList 后端，然后创建 WebView

#### Scenario: 全部 WebView 关闭后延迟停止
- **WHEN** 最后一个 WebView 实例被 close()
- **THEN** 启动 30s 延迟计时器，期间如果再有 open() 则取消停止；否则到时 stop()

## MODIFIED Requirements

### Requirement: GoProcess 移除 OpenList 方法

`GoProcess.ts` 导出 SHALL 移除 `getOpenListRuntime()` 和 `controlOpenList()`，迁移到 `OpenListEmbed` 插件。

### Requirement: LocalOpenListStatusCard 改用新插件

`LocalOpenListStatusCard.vue` SHALL 改用 `OpenListEmbed.getOpenListRuntime()` 替代 `GoProcess.getOpenListRuntime()`。

## REMOVED Requirements

### Removed: OpenListPluginEntry 内 Compose UI

**Reason**: ComboLite 插件应只暴露能力，UI 承载交给 host 应用的多例 Capacitor 插件。Plugin APK 内的 Compose UI 体积大且与 OpenList Web UI 重复。
**Migration**: 删除 StatusCard/ControlCard/ConfigCard/InfoGrid/formatFileSize；OpenList 用户界面完全由 host 侧 `OpenListEmbed.open()` 创建的 WebView 加载 OpenList SPA 提供。

### Removed: GoProcess 中的 OpenList 桥接

**Reason**: OpenList 相关能力应集中在专门的 `OpenListEmbed` 插件，避免 GoProcess 职责过重。
**Migration**: `getOpenListRuntime()` 和 `controlOpenList()` 迁移到 `OpenListEmbed` 插件。

---

## 架构总览（最终）

```
┌─ Host App Process (Capacitor + Ionic Vue) ─────────────────────┐
│                                                                 │
│  OpenListPage.vue (新页面)                                       │
│  └─ <OpenListEmbedContainer container-id="primary" />           │
│        │                                                        │
│        │  await OpenListEmbed.open({ containerId, port })       │
│        ▼                                                        │
│  [Capacitor Bridge]                                             │
│                                                                 │
├─ Capacitor Native Plugins (com.encvgo.app) ────────────────────┤
│  ┌────────────────────────────────────────────┐                │
│  │  GoProcessPlugin (com.encvgo.app)          │                │
│  │  - restart/stop/getStatus                  │                │
│  │  - installPlugin/uninstallPlugin           │                │
│  │  - openPlayer/startMpvInPlace              │                │
│  │  - checkInstalledPlugins/etc.              │                │
│  └────────────────────────────────────────────┘                │
│  ┌────────────────────────────────────────────┐                │
│  │  OpenListEmbedPlugin (新增)                │                │
│  │  - open/close/setBounds/isLoaded/navigate  │                │
│  │  - getOpenListRuntime/controlOpenList      │                │
│  └────────────────────────────────────────────┘                │
│           │                                                      │
│           ▼                                                      │
│  OpenListEmbedService (多例管理器)                               │
│    ├─ instances: Map<containerId, OpenListEmbedInstance>        │
│    └─ 每个 instance 含独立 WebView                                │
│                                                                 │
├─ ComboLite Plugin APK (plugin-openlist) ────────────────────────┤
│  OpenListPluginEntry.Content() → <Box /> (空占位)               │
│  OpenListBridge (gomobile, 不变)                                 │
│  OpenListService (按需启动, 修改)                                 │
│  OpenListStatusProvider (ContentProvider, 不变)                  │
│  OpenListConfig (不变)                                           │
│                                                                 │
├─ OpenList Web UI (Vue3 SPA) ───────────────────────────────────┤
│  http://127.0.0.1:{port}/#/                                     │
│  被 OpenListEmbedService 的多例 WebView 加载                      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```
