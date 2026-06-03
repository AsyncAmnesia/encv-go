# Tasks

## Phase 1: ComboLite 合规性诊断与修复（非 UI）

### 1.1 诊断：对照 combolite-core 接口验证合规性

- [ ] 1.1.1 反编译或查阅 `combolite-core` AAR 中的 `IPluginEntryClass` 接口定义，确认 `onLoad(context: PluginContext)`, `onUnload()`, `Content(): @Composable`, `pluginModule: List<Module>` 的契约要求
- [ ] 1.1.2 对比 MpvPluginEntry 与 OpenListPluginEntry 的每个方法实现差异
- [ ] 1.1.3 检查 PluginLifecycleEngine 如何调用这些方法（load → onLoad? unload → onUnload? Content 何时渲染？）
- [ ] 1.1.4 确认 Service 生命周期与 Plugin load/unload 是否需要同步
- [ ] 1.1.5 输出合规性差距报告（具体到每个方法的修复项）

### 1.2 修复 OpenListPluginEntry.kt（合规）

- [ ] 1.2.1 根据 1.1 诊断结果修复 `onLoad()` / `onUnload()` 行为
- [ ] 1.2.2 如需同步生命周期：`onLoad()` 中检查/启动 Service；`onUnload()` 中 shutdown Service
- [ ] 1.2.3 Content() 内所有 Bridge 调用加 try-catch 防御
- [ ] 1.2.4 验证 Koin module 注册符合规范（如需调整）
- [ ] 1.2.5 `./gradlew :plugin-openlist:compileDebugKotlin` 通过

## Phase 2: Plugin APK Content() 瘦身（移除 Compose UI）

> **注意**：此阶段与 Phase 1 可并行（如果合规修复不涉及 Content() 内部实现）

### 2.1 重写 OpenListPluginEntry.kt Content()

- [ ] 2.1.1 删除所有 `@Composable` 私有函数：`StatusCard`, `ControlCard`, `ConfigCard`, `InfoGrid`, `formatFileSize`
- [ ] 2.1.2 删除所有 Compose Material3 import（`androidx.compose.foundation.*`, `androidx.compose.material3.*`, `androidx.compose.material.icons.*`, `androidx.lifecycle.*` compose 相关）
- [ ] 2.1.3 `Content()` 改为返回空 `Box {}` 或最小占位文本
- [ ] 2.1.4 保留 `IPluginEntryClass`, `PluginContext`, `@Composable` 的最小 import 集合

### 2.2 瘦身 build.gradle.kts

- [ ] 2.2.1 删除 `id("org.jetbrains.kotlin.plugin.compose")`
- [ ] 2.2.2 删除 `buildFeatures { compose = true }`
- [ ] 2.2.3 删除 compose BOM + 所有 compose dependencies（ui/runtime/material3/icons-extended/lifecycle-runtime-compose）
- [ ] 2.2.4 保留非 compose 依赖不变
- [ ] 2.2.5 `./gradlew :plugin-openlist:compileDebugKotlin` 通过

### 2.3 编译验证

- [ ] 2.3.1 确认产物 AAR 不含 `androidx.compose.*` 类
- [ ] 2.3.2 `./gradlew :combolite-host:compileDebugKotlin` 通过

## Phase 3: Host App — "打开管理界面" 入口（Capacitor InAppBrowser）

### 3.1 检查并增强 LocalOpenListStatusCard.vue

- [ ] 3.1.1 读取现有组件，确认当前状态展示逻辑
- [ ] 3.1.2 新增"打开 OpenList 管理界面"按钮（ion-button）
- [ ] 3.1.3 按钮点击调用 Capacitor Browser/InAppBrowser 打开 `http://127.0.0.1:{port}/#/login`
- [ ] 3.1.4 仅在 `runtime.running === true` 时按钮 enabled
- [ ] 3.1.5 确认 `@capacitor/browser` 或 `@capacitor/inappbrowser` 在 package.json 中

### 3.2 TypeScript 编译验证

- [ ] 3.2.1 `npx vue-tsc --noEmit` 通过 (0 errors)

## Task Dependencies

- Phase 1 和 Phase 2 可**并行执行**（合规修复 vs UI 瘩身互不影响）
- Phase 3 在 Phase 2 完成后执行（确保 Content() 瘦身后再处理 Host 侧入口）
