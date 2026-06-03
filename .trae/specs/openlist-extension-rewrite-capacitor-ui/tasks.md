# Tasks

## Phase 0: ComboLite 合规修复（与 UI 无关）

- [ ] 0.1 查阅 `combolite-core` AAR 中 `IPluginEntryClass` 接口契约
- [ ] 0.2 对比 MpvPluginEntry vs OpenListPluginEntry 差距
- [ ] 0.3 重写 `plugin-openlist/OpenListPluginEntry.kt`：
  - `pluginModule = emptyList()`（与 MpvPluginEntry 一致）
  - `onLoad(context)` 初始化 OpenListBridge
  - `onUnload()` shutdown OpenListBridge + OpenListService
  - `Content()` 用 `OpenListEmbedWebView` Composable 替代 Compose UI
- [ ] 0.4 删除现有 Compose UI（StatusCard / ControlCard / ConfigCard / InfoGrid / formatFileSize）
- [ ] 0.5 瘦身 `plugin-openlist/build.gradle.kts`（移除 compose plugin / buildFeatures / dependencies）
- [ ] 0.6 `./gradlew :plugin-openlist:compileDebugKotlin` 通过
- [ ] 0.7 `./gradlew :combolite-host:compileDebugKotlin` 通过

## Phase 1: 嵌入式 WebView + JS-Native 桥接

### 1.1 新增 OpenListEmbedWebView Composable

- [ ] 1.1.1 新建 `plugin-openlist/src/main/java/.../OpenListEmbedWebView.kt`
- [ ] 1.1.2 定义 `@Composable fun OpenListEmbedWebView(containerId, initialPath)` 
- [ ] 1.1.3 用 `AndroidView(factory = { WebView })` 创建 WebView
- [ ] 1.1.4 启用 JS / DOM storage / file access
- [ ] 1.1.5 注册 `OpenListPluginJSInterface` (addJavascriptInterface)
- [ ] 1.1.6 设置 WebViewClient 处理页面加载回调

### 1.2 新增 OpenListPluginJSInterface（JS-Native 桥接）

- [ ] 1.2.1 新建 `plugin-openlist/src/main/java/.../OpenListPluginJSInterface.kt`
- [ ] 1.2.2 `@JavascriptInterface fun startOpenList(): String` → `OpenListBridge.start()`
- [ ] 1.2.3 `@JavascriptInterface fun stopOpenList(): Boolean` → `OpenListBridge.stop()`
- [ ] 1.2.4 `@JavascriptInterface fun getRuntimeStatus(): String` → `OpenListBridge.snapshot()` 返回 JSON
- [ ] 1.2.5 `@JavascriptInterface fun setAdminPassword(pwd: String): Boolean` → `OpenListBridge.setAdminPwd()`
- [ ] 1.2.6 `@JavascriptInterface fun readConfig(): String` → 读 config.json
- [ ] 1.2.7 `@JavascriptInterface fun writeConfig(content: String): Boolean` → 写 config.json (自动备份)
- [ ] 1.2.8 `@JavascriptInterface fun getVersion(): String` → 读 OpenListConfig.version

### 1.3 新增 OpenListWebViewClient

- [ ] 1.3.1 新建 `plugin-openlist/src/main/java/.../OpenListWebViewClient.kt`
- [ ] 1.3.2 处理 shouldOverrideUrlLoading（OpenList SPA 内部链接）
- [ ] 1.3.3 处理 onPageStarted / onPageFinished 回调

### 1.4 编译验证

- [ ] 1.4.1 `./gradlew :plugin-openlist:compileDebugKotlin` 通过

## Phase 2: Monorepo 改造（pnpm workspace）

### 2.1 项目根 workspace 配置

- [ ] 2.1.1 新建 `pnpm-workspace.yaml`：
  ```yaml
  packages:
    - 'app'
    - 'plugin-openlist/web'
    - 'packages/*'
  ```
- [ ] 2.1.2 修改项目根 `package.json` 添加 `"packageManager": "pnpm@9.x"`
- [ ] 2.1.3 确认项目根 `package.json` 的 workspaces 字段（如果用 npm workspaces 而非 pnpm）

### 2.2 共享组件包 @encvgo/components

- [ ] 2.2.1 新建 `packages/components/package.json`：
  - name: `@encvgo/components`
  - main: `src/index.ts`
  - exports: `./OpenListStatusCard`, `./OpenListLogList`
  - peerDependencies: vue, @ionic/vue, ionicons
- [ ] 2.2.2 新建 `packages/components/tsconfig.json`
- [ ] 2.2.3 新建 `packages/components/src/index.ts`
- [ ] 2.2.4 新建 `packages/components/src/OpenListStatusCard.vue`（从原 LocalOpenListStatusCard.vue 移植）
- [ ] 2.2.5 新建 `packages/components/src/OpenListLogList.vue`（从 OpenListPluginEntry 中日志组件移植）
- [ ] 2.2.6 验证 `pnpm install` 在 packages/components 成功

### 2.3 插件 web 项目

- [ ] 2.3.1 新建 `plugin-openlist/web/package.json`：
  - name: `@encvgo/plugin-openlist-web`
  - dependencies: `@encvgo/components: workspace:*`, vue, @ionic/vue, ionicons, vue-router, vite, @vitejs/plugin-vue
- [ ] 2.3.2 新建 `plugin-openlist/web/vite.config.ts`
- [ ] 2.3.3 新建 `plugin-openlist/web/tsconfig.json`
- [ ] 2.3.4 新建 `plugin-openlist/web/index.html`
- [ ] 2.3.5 验证 `pnpm install` 在 plugin-openlist/web 成功

## Phase 3: 插件 web 项目页面

### 3.1 入口与路由

- [ ] 3.1.1 新建 `plugin-openlist/web/src/main.ts`
- [ ] 3.1.2 新建 `plugin-openlist/web/src/App.vue`（ion-app 根）
- [ ] 3.1.3 新建 `plugin-openlist/web/src/router/index.ts`
  - 路由：/ → OpenListHome, /config → OpenListConfigEditor, /settings → OpenListSettings

### 3.2 页面

- [ ] 3.2.1 新建 `plugin-openlist/web/src/views/OpenListHome.vue`（K-Sillot OpenListScreen 复刻）
  - AppBar: 标题 + 工具按钮（密码/Config/快捷方式）
  - Body: OpenListStatusCard + OpenListLogList
  - FAB: Start/Stop 切换
  - onMounted: refreshStatus + setInterval 3s
  - toggleService: 调 OpenListNative.start/stop
- [ ] 3.2.2 新建 `plugin-openlist/web/src/views/OpenListConfigEditor.vue`（K-Sillot ConfigEditorPage 复刻）
  - onMounted: `OpenListNative.readConfig()` → 填充 textarea
  - JSON 校验（debounce 300ms）
  - 保存按钮：备份 + writeConfig
  - 三选项 dialog: 取消/仅保存/保存并重启
- [ ] 3.2.3 新建 `plugin-openlist/web/src/views/OpenListSettings.vue`（简化版）
  - 显示 OpenList 版本、数据目录
  - 关于按钮
- [ ] 3.2.4 新建 `plugin-openlist/web/src/views/OpenListWebView.vue`（可选，iframe 加载 OpenList SPA）

### 3.3 插件定义

- [ ] 3.3.1 新建 `plugin-openlist/web/src/plugins/openlist-native.ts`
  - 类型声明 `Window.OpenListNative`
  - 导出 `OpenListNative` 对象包装 window.OpenListNative
  - 方法: start / stop / getStatus / setPassword / readConfig / writeConfig / getVersion

### 3.4 构建配置

- [ ] 3.4.1 `plugin-openlist/web/vite.config.ts` 配置：
  - build.outDir: `dist`
  - build.assetsDir: `assets`
  - 别名 `@` → `src`
- [ ] 3.4.2 `cd plugin-openlist/web && pnpm install`
- [ ] 3.4.3 `pnpm run build` 产出 `dist/`

### 3.5 TypeScript 编译

- [ ] 3.5.1 `cd plugin-openlist/web && npx vue-tsc --noEmit` 通过 (0 errors)

## Phase 4: 主 app 集成（最小改动）

### 4.1 主 app 路由

- [ ] 4.1.1 修改 `app/src/router/index.ts`：
  - 添加 `/openlist` 路由 → `OpenListHome` (从 `@encvgo/plugin-openlist-web/views/OpenListHome.vue` 导入)
- [ ] 4.1.2 验证主 app `pnpm install` + `pnpm run build` 通过

### 4.2 TypeScript 编译

- [ ] 4.2.1 主 app `npx vue-tsc --noEmit` 通过 (0 errors)

## Phase 5: 编译与部署验证

- [ ] 5.1 `./gradlew :plugin-openlist:compileDebugKotlin` 通过
- [ ] 5.2 `./gradlew :combolite-host:compileDebugKotlin` 通过
- [ ] 5.3 `./gradlew :app:compileDebugKotlin` 通过
- [ ] 5.4 插件 web `pnpm run build` 产出 dist/
- [ ] 5.5 主 app `pnpm run build` 产出 dist/
- [ ] 5.6 沙箱预览 `scripts/start-preview.sh` 启动正常
- [ ] 5.7 主 app 路由 `/openlist` 可访问，渲染 OpenListHome
- [ ] 5.8 验证嵌入式 WebView 通过 OpenListNative JSInterface 调 OpenListBridge 成功
- [ ] 5.9 验证 OpenListStatusCard / OpenListLogList 共享组件在主 app 和插件都可用

## Phase 6: plugin-openlist/web 前端开发预览（沙箱浏览器版）

> **与既有 `scripts/dev-openlist.sh` 的关键区别**：
> - `dev-openlist.sh` 预览的是 **OpenList 原生 SPA**（Hi-Sillot-OpenList/public/dist 里的 Vue3 SPA，通过 Vite middleware 反代到 OpenList(5244)）
> - 本 Phase 预览的是 **plugin-openlist/web 的 Capacitor 多例 UI**（plugin 自己的 Vue3 + Ionic Vue 8 管理面板），不依赖 OpenList(5244)，由 `window.OpenListNative` 桥接 Android 端的 OpenListBridge
>
> 沙箱浏览器模式下 `window.OpenListNative` 不存在，所有 JS-Native 调用走 `safe(fallback, fn)` 安全 fallback → 显示「未安装/已停止」默认态。这是预期的「UI 视觉预览」目标。

- [ ] 6.1 新建 `scripts/dev-openlist-web.sh`（沿用 `start-preview.sh` 的铁律风格）
  - 默认端口 5174（plugin-openlist/web vite.config 已设）
  - 端口被占时回退到 5175
  - 启动前清理残留 vite 进程
  - 信号陷阱：Ctrl+C 时优雅 kill 子进程
  - 前台运行（保持 OpenPreview 可激活）
  - 状态报告 + OpenPreview 提示
- [ ] 6.2 `bash scripts/dev-openlist-web.sh` 启动成功
- [ ] 6.3 `curl -s http://localhost:5174/` 返回 200 + HTML
- [ ] 6.4 浏览器访问 OpenListHome（路由 `/home`）看到：
  - AppBar 标题「OpenList - v0.0.0」
  - 4 个工具按钮（密码/Config/Settings/WebView）
  - OpenListStatusCard（默认态：已停止）
  - OpenListLogList（空）
  - FAB 启动按钮（点击走 fallback 不报错）
- [ ] 6.5 访问 `/config` 看到 JSON 编辑器
- [ ] 6.6 访问 `/settings` 看到版本/数据目录占位
- [ ] 6.7 访问 `/webview` 看到「需 Android WebView 容器」提示
- [ ] 6.8 验证 HMR：修改 `OpenListStatusCard.vue` 后浏览器自动刷新

## Phase 7: /webview 嵌入 OpenList 原生 SPA（iframe + Vite proxy）

> **核心目标**：在 Capacitor 多例 UI 的 `/webview` 路由内显示 OpenList 自己的 Vue3 SPA（Hi-Sillot-OpenList-Frontend），让用户能在同一页面里既管理 OpenList 启停，又浏览 OpenList 文件管理。
>
> 沙箱浏览器模式下走 Vite proxy (`/openlist-spa/*` → `http://127.0.0.1:5244/*`)；真机 WebView 内 OpenList 与 Capacitor 同设备 → iframe 直连 5244。
>
> 仍需配合 `bash scripts/dev-openlist.sh` 启动 OpenList(5244) 后端。后端未启动时显示降级 UI「OpenList 后端未运行」+ 启动命令提示。

- [ ] 7.1 修改 `plugin-openlist/web/vite.config.ts`：
  - 添加 `server.proxy['/openlist-spa']` → `http://127.0.0.1:5244`
  - `changeOrigin: true`、`rewrite: path => path.replace(/^\\/openlist-spa/, '')`
- [ ] 7.2 重写 `plugin-openlist/web/src/views/OpenListWebView.vue`：
  - 始终渲染 iframe，`:src` 根据 `import.meta.env.DEV` 切换（dev 走代理，prod 直连）
  - 沙箱模式下 `onMounted` 主动探测 `/openlist-spa/` HEAD 请求（2s 超时）
  - 探测失败 → `showFallback=true` → 显示「OpenList 后端未运行」+ `bash scripts/dev-openlist.sh` 提示 + 重试按钮
  - 真机模式 → 直接假设可达（不探测），始终显示 iframe
  - 顶部 toolbar 增加「外部打开」按钮（仅真机模式）
- [ ] 7.3 重启 dev server：`pkill vite; bash scripts/dev-openlist-web.sh`
- [ ] 7.4 验证 `/openlist-spa/` 代理生效（curl 502 表示代理工作但后端未启）
- [ ] 7.5 验证 `/webview` 页面在浏览器中显示降级 UI（带 `bash scripts/dev-openlist.sh` 命令）
- [ ] 7.6 验证用户运行 `bash scripts/dev-openlist.sh` 后 iframe 自动加载 OpenList SPA（hash 路由 `#/login`）
- [ ] 7.7 验证 `vue-tsc --noEmit` 通过

## Task Dependencies

- Phase 0 → Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5 → Phase 6 → Phase 7
- Phase 1 (Kotlin) 独立于 Phase 2-3 (Web)
- Phase 2 (monorepo) 必须在 Phase 3 之前（共享包要先建好）
- Phase 4 依赖 Phase 3（web 页面要先有，主 app 才能 import）
- Phase 6 依赖 Phase 2-3（依赖 monorepo + 页面已建）
- Phase 7 依赖 Phase 6（Vite dev server 必须已配好）
