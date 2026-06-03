# Tasks

## Phase 0: ComboLite 合规修复（与 UI 无关）

- [ ] 0.1 阅读 `combolite-core` AAR 的 `IPluginEntryClass` 接口定义，确认 onLoad/onUnload/Content/pluginModule 契约
- [ ] 0.2 对比 MpvPluginEntry vs OpenListPluginEntry 差距
- [ ] 0.3 重写 `plugin-openlist/OpenListPluginEntry.kt`：
  - `pluginModule = emptyList()`（与 MpvPluginEntry 一致）
  - `onLoad(context)` 初始化 OpenListBridge
  - `onUnload()` shutdown OpenListBridge + OpenListService
  - `Content()` 返回最小占位 Box
- [ ] 0.4 删除现有 Compose UI（StatusCard/ControlCard/ConfigCard/InfoGrid/formatFileSize）
- [ ] 0.5 瘦身 `plugin-openlist/build.gradle.kts`（移除 compose plugin/buildFeatures/dependencies）
- [ ] 0.6 `./gradlew :plugin-openlist:compileDebugKotlin` 通过
- [ ] 0.7 `./gradlew :combolite-host:compileDebugKotlin` 通过

## Phase 1: plugin-openlist 改造为独立 Capacitor 应用

### 1.1 新增 OpenListMainActivity

- [ ] 1.1.1 新建 `plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListMainActivity.kt`
- [ ] 1.1.2 extends `com.getcapacitor.BridgeActivity`
- [ ] 1.1.3 覆盖 `onCreate()` 调用 `super.onCreate()` 让 Capacitor 加载资源
- [ ] 1.1.4 添加到 `AndroidManifest.xml`（独立 launcher Activity + intent-filter）

### 1.2 新增 Capacitor 插件（plugin-openlist 内部）

- [ ] 1.2.1 新建 `plugin-openlist/src/main/java/com/encvgo/plugin/openlist/capacitor/OpenListServicePlugin.kt`
  - `@CapacitorPlugin(name = "OpenListService")` extends `Plugin`
  - `@PluginMethod fun start(call)` → `OpenListBridge.start()`
  - `@PluginMethod fun stop(call)` → `OpenListBridge.stop()`
  - `@PluginMethod fun getStatus(call)` → `OpenListBridge.snapshot()`
  - `@PluginMethod fun getVersion(call)` → `OpenListConfig.version`
  - `@PluginMethod fun getPort(call)` → `OpenListService.lastPort`
  - `@PluginMethod fun getIsRunning(call)` → `OpenListService.isRunning`

- [ ] 1.2.2 新建 `plugin-openlist/src/main/java/com/encvgo/plugin/openlist/capacitor/OpenListConfigPlugin.kt`
  - `@CapacitorPlugin(name = "OpenListConfig")` extends `Plugin`
  - `@PluginMethod fun read(call)` → 读 config.json
  - `@PluginMethod fun write(call)` → 写 config.json（自动备份）
  - `@PluginMethod fun getDataDir(call)` → 读 dataDir

- [ ] 1.2.3 新建 `plugin-openlist/src/main/java/com/encvgo/plugin/openlist/capacitor/OpenListPasswordPlugin.kt`
  - `@CapacitorPlugin(name = "OpenListPassword")` extends `Plugin`
  - `@PluginMethod fun setPassword(call)` → `OpenListBridge.setAdminPwd(pwd)`

- [ ] 1.2.4 在 `OpenListMainActivity` 注册这 3 个插件：`registerPlugin(OpenListServicePlugin::class.java)` etc.

### 1.3 资源准备

- [ ] 1.3.1 新建 `plugin-openlist/src/main/assets/capacitor.config.json`
  - `appId: "com.encvgo.plugin.openlist"`
  - `appName: "OpenList"`
  - `webDir: "public"`
  - `bundledWebRuntime: false`

- [ ] 1.3.2 新建 `plugin-openlist/src/main/assets/capacitor.plugins.json`
  - 注册 3 个 Capacitor 插件

- [ ] 1.3.3 新建 `plugin-openlist/src/main/assets/public/index.html`（占位，Phase 2 替换）
  - `<html><body>OpenList - 等待 Phase 2 web 资源</body></html>`

### 1.4 编译验证

- [ ] 1.4.1 `./gradlew :plugin-openlist:assembleDebug` 通过
- [ ] 1.4.2 产物 APK 包含 assets/public/index.html
- [ ] 1.4.3 AndroidManifest.xml 中 OpenListMainActivity 已注册

## Phase 2: Capacitor Web 项目（K-Sillot Mobile 风格 UI 复刻）

### 2.1 项目脚手架

- [ ] 2.1.1 新建 `plugin-openlist/web/` 目录
- [ ] 2.1.2 新建 `plugin-openlist/web/package.json`：
  - `vue@^3.4`
  - `@ionic/vue@^8`
  - `vue-router@^4`
  - `vite@^5`
  - `@vitejs/plugin-vue`
  - `@capacitor/core`, `@capacitor/android`, `@capacitor/cli`
  - `ionicons`

- [ ] 2.1.3 新建 `plugin-openlist/web/vite.config.ts`
- [ ] 2.1.4 新建 `plugin-openlist/web/capacitor.config.ts`
- [ ] 2.1.5 新建 `plugin-openlist/web/tsconfig.json`
- [ ] 2.1.6 新建 `plugin-openlist/web/index.html`

### 2.2 Capacitor 插件定义（Web 端）

- [ ] 2.2.1 新建 `plugin-openlist/web/src/plugins/OpenListService.ts`
  - interface: `start/stop/getStatus/getVersion/getPort/getIsRunning`
  - `registerPlugin<OpenListServicePlugin>('OpenListService')`

- [ ] 2.2.2 新建 `plugin-openlist/web/src/plugins/OpenListConfig.ts`
  - interface: `read/write/getDataDir`

- [ ] 2.2.3 新建 `plugin-openlist/web/src/plugins/OpenListPassword.ts`
  - interface: `setPassword`

- [ ] 2.2.4 新建 `plugin-openlist/web/src/plugins/openlist-service/web.ts` 等 web stub（开发预览用）

### 2.3 入口与路由

- [ ] 2.3.1 新建 `plugin-openlist/web/src/main.ts`
  - 引入 IonicVue
  - 引入 router
  - mount('#app')

- [ ] 2.3.2 新建 `plugin-openlist/web/src/router/index.ts`
  - `/` → HomePage（默认 tab=openlist）
  - `/web` → WebScreen
  - `/openlist` → OpenListScreen
  - `/downloads` → DownloadManager
  - `/settings` → Settings

- [ ] 2.3.3 新建 `plugin-openlist/web/src/App.vue`
  - `<ion-app><ion-router-outlet /></ion-app>`

### 2.4 页面（K-Sillot Mobile 复刻）

- [ ] 2.4.1 新建 `plugin-openlist/web/src/views/HomePage.vue`（IonTabs 容器）
  - 4 tab：Web / OpenList / Downloads / Settings
  - tab="openlist" 为默认

- [ ] 2.4.2 新建 `plugin-openlist/web/src/views/WebScreen.vue`（Tab 0，复刻 lib/pages/web/web.dart）
  - iframe 加载 `http://127.0.0.1:{port}/#/login`
  - 加载失败时尝试启动 OpenList 并重试
  - 下载拦截走 Capacitor Browser

- [ ] 2.4.3 新建 `plugin-openlist/web/src/views/OpenListScreen.vue`（Tab 1，复刻 lib/pages/openlist/openlist.dart）
  - AppBar: "OpenList - v{version}"
  - Actions: 密码 / Config / 桌面快捷 / 更多
  - FAB: Start/Stop 切换
  - Body: LogListView

- [ ] 2.4.4 新建 `plugin-openlist/web/src/views/DownloadManager.vue`（Tab 2，占位）
  - 简化的下载管理页面

- [ ] 2.4.5 新建 `plugin-openlist/web/src/views/Settings.vue`（Tab 3，占位）
  - 简化的设置页面

### 2.5 组件

- [ ] 2.5.1 新建 `plugin-openlist/web/src/components/LogListView.vue`（复刻 lib/pages/openlist/log_list_view.dart）
  - 实时日志流（ion-list + 滚动到底部）
  - 日志级别颜色（info/warn/error）

- [ ] 2.5.2 新建 `plugin-openlist/web/src/components/PwdEditDialog.vue`（复刻 lib/pages/openlist/pwd_edit_dialog.dart）
  - ion-alert 输入密码
  - 确认后调 `OpenListPassword.setPassword(pwd)`

- [ ] 2.5.3 新建 `plugin-openlist/web/src/components/ConfigEditorPage.vue`（复刻 lib/pages/openlist/config_editor_page.dart）
  - 读 config.json
  - JSON 编辑器（textarea）
  - 实时 JSON 校验（debounce 300ms）
  - 保存前自动备份
  - 三选项对话框：取消 / 仅保存 / 保存并重启

- [ ] 2.5.4 新建 `plugin-openlist/web/src/components/AboutDialog.vue`（复刻 lib/pages/openlist/about_dialog.dart）
  - 显示 OpenList 版本、ENCV 适配信息

### 2.6 构建集成

- [ ] 2.6.1 `cd plugin-openlist/web && npm install`
- [ ] 2.6.2 `npm run build` 产出 `dist/`
- [ ] 2.6.3 `npx cap sync android` 同步 web 资源到 `android/src/main/assets/public/`
- [ ] 2.6.4 `./gradlew :plugin-openlist:assembleDebug` 重新打包 APK
- [ ] 2.6.5 确认 APK assets/public/ 包含 web bundle

### 2.7 TypeScript 编译

- [ ] 2.7.1 `cd plugin-openlist/web && npx vue-tsc --noEmit` 通过

## Phase 3: 主 app 集成（最小改动）

### 3.1 新增主 app Capacitor 插件

- [ ] 3.1.1 新建 `android/app/src/main/java/com/encvgo/app/StartOpenListAppPlugin.kt`
  - `@CapacitorPlugin(name = "StartOpenListApp")` extends `Plugin`
  - `@PluginMethod fun start(call)` → `context.startActivity(Intent().setComponent(ComponentName("com.encvgo.plugin.openlist", "com.encvgo.plugin.openlist.OpenListMainActivity")))`
  - 异常处理：插件未安装时返回 `{ success: false, error: "OpenList plugin not installed" }`

- [ ] 3.1.2 在主 `MainActivity.onCreate()` 注册：`registerPlugin(StartOpenListAppPlugin::class.java)`

### 3.2 TypeScript 定义

- [ ] 3.2.1 新建 `src/plugins/StartOpenListApp.ts`
  - interface: `start(): Promise<{ success: boolean; error?: string }>`
  - `registerPlugin<StartOpenListAppPlugin>('StartOpenListApp')`

- [ ] 3.2.2 新建 web stub `src/plugins/start-openlist-app/web.ts`

### 3.3 主 app Vue 修改

- [ ] 3.3.1 修改 `src/components/LocalOpenListStatusCard.vue`：
  - 增加"启动 OpenList 独立界面"按钮（ion-button）
  - 点击调 `StartOpenListApp.start()`

- [ ] 3.3.2 修改 `src/views/ExtensionsPage.vue`：
  - 已安装的 OpenList 卡片增加"启动"按钮
  - 点击调 `StartOpenListApp.start()`

### 3.4 TypeScript 编译

- [ ] 3.4.1 `npx vue-tsc --noEmit` 通过 (0 errors)

## Phase 4: 端到端验证

- [ ] 4.1 全模块编译通过
- [ ] 4.2 plugin-openlist APK 独立启动验证（adb am start）
- [ ] 4.3 Capacitor 加载 web 资源成功
- [ ] 4.4 OpenListScreen FAB 启动 OpenList → WebScreen iframe 加载成功
- [ ] 4.5 密码设置 + Config 编辑 + 日志流验证
- [ ] 4.6 主 app 调 `StartOpenListApp.start()` → OpenListMainActivity 启动
- [ ] 4.7 主 app LocalOpenListStatusCard 通过 ContentProvider 读到 OpenList 状态

## Task Dependencies

- Phase 0 → Phase 1 → Phase 2 → Phase 3 → Phase 4
- Phase 1.2 依赖 Phase 0（OpenListBridge 初始化顺序调整）
- Phase 2 依赖 Phase 1（Capacitor 插件 Native 实现先完成）
- Phase 3 依赖 Phase 1（OpenListMainActivity 必须存在才能从主 app 启动）
