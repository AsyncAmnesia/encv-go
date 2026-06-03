# Tasks

## Phase 1: 新增 Capacitor 插件（Host App 模块）

### 1.1 创建 OpenListEmbedPlugin（Kotlin）

- [ ] 1.1.1 新建 `android/app/src/main/java/com/encvgo/app/openlist/OpenListEmbedPlugin.kt`
- [ ] 1.1.2 定义 `@CapacitorPlugin(name = "OpenListEmbed")` 类继承 `Plugin`
- [ ] 1.1.3 实现 `@PluginMethod fun open(call)` → 调 `OpenListEmbedService.startEmbed()`
- [ ] 1.1.4 实现 `@PluginMethod fun close(call)` → 调 `OpenListEmbedService.stopEmbed()`
- [ ] 1.1.5 实现 `@PluginMethod fun setBounds(call)` → 调整 WebView 位置
- [ ] 1.1.6 实现 `@PluginMethod fun isLoaded(call)` → 查询实例
- [ ] 1.1.7 实现 `@PluginMethod fun navigate(call)` → `webView.loadUrl()`
- [ ] 1.1.8 实现 `@PluginMethod fun getOpenListRuntime(call)` → 调 `OpenListStatusBridge.read()`
- [ ] 1.1.9 实现 `@PluginMethod fun controlOpenList(call)` → 调 `OpenListStatusBridge.control()`
- [ ] 1.1.10 在 `MainActivity.onCreate()` 注册插件（如果需要）

### 1.2 创建 OpenListEmbedService（多例管理器）

- [ ] 1.2.1 新建 `android/app/src/main/java/com/encvgo/app/openlist/OpenListEmbedService.kt`
- [ ] 1.2.2 定义 `companion object` 持有 `ConcurrentHashMap<String, OpenListEmbedInstance>`
- [ ] 1.2.3 实现 `startEmbed(activity, containerId, port, path)` → 创建 WebView 并 addView 到主 Activity
- [ ] 1.2.4 实现 `stopEmbed(containerId)` → 销毁 WebView
- [ ] 1.2.5 实现 `setBounds(containerId, x, y, w, h)` → 调整 WebView layoutParams
- [ ] 1.2.6 实现 WebView 错误自愈（loadUrl 失败 → 启动 OpenList → 重试 3 次）
- [ ] 1.2.7 实现按需启动 OpenList（第一个 open() 触发 control('start')）
- [ ] 1.2.8 实现延迟停止（所有 WebView 关闭后 30s 计时器）

### 1.3 创建 OpenListEmbedInstance（数据类）

- [ ] 1.3.1 新建 `android/app/src/main/java/com/encvgo/app/openlist/OpenListEmbedInstance.kt`
- [ ] 1.3.2 定义 `data class(containerId, webView, port, path, createdAt)`
- [ ] 1.3.3 定义 `isLoaded: Boolean` 计算属性

### 1.4 修改 GoProcessPlugin 移除 OpenList 方法

- [ ] 1.4.1 移除 `getOpenListRuntime(call)` 方法
- [ ] 1.4.2 移除 `controlOpenList(call)` 方法
- [ ] 1.4.3 移除 `OpenListStatusBridge` import

### 1.5 编译验证

- [ ] 1.5.1 `./gradlew :app:compileDebugKotlin` 通过
- [ ] 1.5.2 确认 OpenListEmbedPlugin 已注册

## Phase 2: TypeScript 插件定义

### 2.1 新增 OpenListEmbed.ts

- [ ] 2.1.1 新建 `src/plugins/OpenListEmbed.ts`
- [ ] 2.1.2 定义接口：`open/close/setBounds/isLoaded/navigate/getOpenListRuntime/controlOpenList`
- [ ] 2.1.3 使用 `registerPlugin<OpenListEmbedPlugin>('OpenListEmbed', { web: () => import('./openlist-embed/web').then(m => new m.OpenListEmbedWeb()) })`
- [ ] 2.1.4 导出 `OpenListEmbed` 实例

### 2.2 新增 web.ts（Web 端 stub）

- [ ] 2.2.1 新建 `src/plugins/openlist-embed/web.ts`
- [ ] 2.2.2 实现 `OpenListEmbedWeb extends WebPlugin` 提供 web 端 stub

### 2.3 修改 GoProcess.ts 移除 OpenList

- [ ] 2.3.1 移除 `getOpenListRuntime()` 函数导出
- [ ] 2.3.2 移除 `controlOpenList()` 函数导出
- [ ] 2.3.3 移除 `OpenListRuntime` interface
- [ ] 2.3.4 移除 `controlOpenList` 类型定义

### 2.4 修改 web.ts 移除 OpenList

- [ ] 2.4.1 移除 `GoProcessPlugin` interface 中的 `getOpenListRuntime` / `controlOpenList` 方法
- [ ] 2.4.2 移除 `GoProcessWeb` 中相应 stub 实现

### 2.5 TypeScript 编译

- [ ] 2.5.1 `npx vue-tsc --noEmit` 通过 (0 errors)
- [ ] 2.5.2 修复所有引用旧 `GoProcess.getOpenListRuntime` 的地方改用 `OpenListEmbed.getOpenListRuntime()`

## Phase 3: Vue 组件（仅作为容器）

### 3.1 新增 OpenListEmbedContainer.vue

- [ ] 3.1.1 新建 `src/components/OpenListEmbedContainer.vue`
- [ ] 3.1.2 props: `{ containerId: string, autoStart?: boolean }`
- [ ] 3.1.3 template: `<div :id="containerId" :style="containerStyle" />`
- [ ] 3.1.4 onMounted: 等待 nextTick → `OpenListEmbed.open()`
- [ ] 3.1.5 onUnmounted: `OpenListEmbed.close()`
- [ ] 3.1.6 `vue-tsc --noEmit` 通过

### 3.2 新增 OpenListPage.vue 入口

- [ ] 3.2.1 新建 `src/views/OpenListPage.vue`
- [ ] 3.2.2 包含 `<OpenListEmbedContainer container-id="primary" auto-start />`
- [ ] 3.2.3 包含 Ionic 全屏布局（ion-header + ion-content + ion-footer）
- [ ] 3.2.4 可选：浮动按钮"打开新窗口" → 调 `OpenListEmbed.open({ containerId: 'secondary', port: 5244 })`（多例演示）

### 3.3 修改 router 添加路由

- [ ] 3.3.1 在 `src/router/index.ts` 添加 `/openlist` → `OpenListPage.vue`

## Phase 4: 修改现有页面（最小改动）

### 4.1 修改 LocalOpenListStatusCard.vue

- [ ] 4.1.1 改用 `OpenListEmbed.getOpenListRuntime()` 替代 `GoProcess.getOpenListRuntime()`

### 4.2 修改 Remote.vue 添加入口

- [ ] 4.2.1 添加"打开 OpenList"按钮（ion-button）
- [ ] 4.2.2 点击 → `router.push('/openlist')`
- [ ] 4.2.3 可选：在卡片里显示 Embedded 状态指示

### 4.3 修改 ExtensionsPage.vue

- [ ] 4.3.1 已安装的 OpenList 卡片增加"打开管理"按钮（替代或补充 enable toggle）
- [ ] 4.3.2 点击 → `router.push('/openlist')`

## Phase 5: Plugin APK 瘦身（plugin-openlist）

### 5.1 重写 OpenListPluginEntry.kt

- [ ] 5.1.1 删除 StatusCard/ControlCard/ConfigCard/InfoGrid/formatFileSize 函数
- [ ] 5.1.2 删除所有 compose material3/icons/foundation/lifecycle import
- [ ] 5.1.3 `pluginModule = emptyList()` 替代注册 OpenListBridge
- [ ] 5.1.4 `onLoad()` 简化为空（OpenList 启动由 host 侧 OpenListEmbedService 触发）
- [ ] 5.1.5 `onUnload()` 简化为空
- [ ] 5.1.6 `Content()` 返回 `Box {}` 最小占位

### 5.2 瘦身 build.gradle.kts

- [ ] 5.2.1 删除 `id("org.jetbrains.kotlin.plugin.compose")`
- [ ] 5.2.2 删除 `buildFeatures { compose = true }`
- [ ] 5.2.3 删除 compose BOM + ui/runtime/material3/icons-extended/lifecycle-runtime-compose

### 5.3 修改 OpenListService.kt

- [ ] 5.3.1 移除 `onCreate()` 中的自动启动逻辑
- [ ] 5.3.2 改为按需启动（接收 `OpenListStatusBridge.control('start')` 触发）
- [ ] 5.3.3 移除 boot-time self-start

### 5.4 编译验证

- [ ] 5.4.1 `./gradlew :plugin-openlist:compileDebugKotlin` 通过
- [ ] 5.4.2 `./gradlew :combolite-host:compileDebugKotlin` 通过
- [ ] 5.4.3 确认 plugin-openlist AAR 不含 compose 类

## Phase 6: 端到端验证

- [ ] 6.1 全模块编译通过
- [ ] 6.2 `vue-tsc --noEmit` 0 errors
- [ ] 6.3 沙箱预览启动正常
- [ ] 6.4 路径 `/openlist` 可访问，挂载的 OpenListEmbedContainer 渲染 div
- [ ] 6.5 Native 侧 WebView 正确加载 OpenList Web UI
- [ ] 6.6 多次进入 `/openlist` 验证多例（每个 containerId 独立）
- [ ] 6.7 关闭页面 → WebView 销毁 → OpenListService 30s 延迟后停止

## Task Dependencies

- Phase 1 (Native Plugin) → Phase 2 (TS Plugin) → Phase 3 (Vue) → Phase 4 (页面修改) → Phase 5 (Plugin 瘦身) → Phase 6 (验证)
- Phase 5 (Plugin 瘦身) 可与 Phase 1-4 并行（仅 plugin-openlist 内修改）
