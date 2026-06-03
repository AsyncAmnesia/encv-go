# Tasks

## Phase 1: ComboLite 合规性诊断与修复（非 UI）

### 1.1 诊断：对照 combolite-core 接口验证合规性

- [ ] 1.1.1 查阅 `combolite-core` AAR 中的 `IPluginEntryClass` 接口定义，确认方法契约
- [ ] 1.1.2 对比 MpvPluginEntry 与 OpenListPluginEntry 的每个方法实现差异
- [ ] 1.1.3 检查 PluginLifecycleEngine 如何调用这些方法
- [ ] 1.1.4 确认 Service 生命周期与 Plugin load/unload 是否需要同步
- [ ] 1.1.5 输出合规性差距报告

### 1.2 修复 OpenListPluginEntry.kt（合规）

- [ ] 1.2.1 根据 1.1 诊断结果修复 `onLoad()` / `onUnload()` 行为
- [ ] 1.2.2 如需同步生命周期：`onLoad()` 检查/启动 Service；`onUnload()` shutdown Service
- [ ] 1.2.3 Content() 内所有 Bridge 调用加 try-catch 防御
- [ ] 1.2.4 验证/调整 Koin module 注册
- [ ] 1.2.5 `./gradlew :plugin-openlist:compileDebugKotlin` 通过

## Phase 2: 插件内 UI 重写（Content()）

> **需要用户确认方案 A（Compose WebView）或 方案 B（纯 Compose 原生控件）后再实施**

### 方案 A: Compose WebView（推荐）

- [ ] 2A.1 删除现有 StatusCard/ControlCard/ConfigCard 等 ~400 行手写 Compose UI
- [ ] 2A.2 Content() 改为 `AndroidView { WebView }` 加载 `http://127.0.0.1:{port}`
- [ ] 2A.3 实现 WebView 错误自愈（加载失败 → 启动 OpenList → 重试）
- [ ] 2A.4 动态获取端口（从 Bridge snapshot 或 Config）
- [ ] 2A.5 build.gradle.kts 移除 material3/icons-extended/lifecycle-runtime-compose，保留 compose.ui (AndroidView)
- [ ] 2A.6 编译通过 + WebView 在插件进程中正确加载 OpenList SPA

### 方案 B: 纯 Compose 原生控件

- [ ] 2B.1 精简 Content() 为最小控制面板（仅 start/stop + 状态摘要 + "打开 Web UI" 按钮）
- [ ] 2B.2 删除 ConfigCard（配置操作在 Web UI 中完成）
- [ ] 2B.3 删除 StatusCard 中的详细字段（PID/数据大小等），只保留 running 状态
- [ ] 2B.4 "打开 Web UI"按钮调用外部 Intent 打开浏览器
- [ ] 2B.5 build.gradle.kts 可保留现有 compose 依赖或按需精简
- [ ] 2B.6 编译通过

## Phase 3: 验证

- [ ] 3.1 `./gradlew :plugin-openlist:compileDebugKotlin` 通过
- [ ] 3.2 `./gradlew :combolite-host:compileDebugKotlin` 通过
- [ ] 3.3 Host App TypeScript 编译不受影响（`vue-tsc --noEmit` 通过）

## Task Dependencies

- Phase 1（合规修复）可独立执行
- Phase 2（UI 重写）依赖用户方案选择
- Phase 3 在 Phase 1 + 2 完成后执行
