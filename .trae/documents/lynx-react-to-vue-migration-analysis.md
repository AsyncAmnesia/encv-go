# Lynx 播放器 React → Vue Lynx 迁移计划

## 决策

用户确认：lynx-ui 本身质量一般，Lynx 生态小需要自己写组件，使用 vue-lynx 重构。

## 迁移步骤

### 步骤 1：替换项目依赖和构建配置

**文件变更：**
- `package.json`：移除 `@lynx-js/react`、`@lynx-js/lynx-ui`、`react`、`@lynx-js/react-rsbuild-plugin`、`@types/react`；添加 `vue-lynx`、`vue-router`、`vue-lynx/plugin`
- `lynx.config.ts`：移除 `reactPlugin()`，添加 `pluginVueLynx()`
- `tsconfig.json`：调整 JSX 配置（移除 jsx: "react-jsx"，vue-lynx 用 SFC 不需要）

### 步骤 2：创建 Vue Router 配置

**新建文件：**
- `src/router.ts`：`createMemoryHistory()` + 4 条路由（home/player/playlist/settings）

### 步骤 3：创建 App.vue 根组件

**新建文件：**
- `src/App.vue`：`<RouterView />` 容器，替代原 AppComponent.tsx 的路由逻辑
- `src/main.ts`：`createApp(App).use(router).mount()`，替代原 App.tsx

### 步骤 4：迁移页面组件（TSX → SFC）

| 原 React 组件 | 新 Vue SFC | 说明 |
|---|---|---|
| `HomePage.tsx` | `views/HomeView.vue` | 首页，Composition API |
| `PlayerPage.tsx` | `views/PlayerView.vue` | 播放页，MPV 交互逻辑 |
| `PlaylistPage.tsx` | `views/PlaylistView.vue` | 播放列表 |
| `SettingsPage.tsx` | `views/SettingsView.vue` | 设置页 |

### 步骤 5：迁移 PlayerControls 组件

**原组件：** `PlayerControls.tsx`（使用 `@lynx-js/lynx-ui` 的 SliderRoot/Button）
**新组件：** `components/PlayerControls.vue`

需要自实现的 UI 原语（替代 lynx-ui）：
- **Slider**：用 `<view>` + `@panstart/@panmove/@panend` 手势实现进度条
- **Button**：用 `<view>` + `@tap` 实现（Lynx 原生支持手势事件）

### 步骤 6：迁移 CSS

从 `App.css` 拆分到各 SFC 的 `<style scoped>` 中，共享样式提取到 `src/styles/` 目录。

### 步骤 7：NativeModules 适配

`NativeModules.MpvPlayerModule`、`NativeModules.GoBackendModule`、`NativeModules.LogBridge` 是 Lynx 原生桥接，与框架无关，直接在 Vue 组件中调用即可。

需确认 vue-lynx 的 `useLynxGlobalEventListener` 等价 API。根据文档，vue-lynx 使用 `onGlobalEvent` 或 `useLynxGlobalEventListener`（需验证）。

### 步骤 8：清理旧文件

删除所有 `.tsx` 文件和 `App.css`。

### 步骤 9：更新 Android 端构建配置

确认 `PlayerActivityLynx.kt` 中加载的 bundle 路径是否需要调整（vue-lynx 编译产物可能不同）。

### 步骤 10：更新 preview.html

保持 HTML 预览页面用于本地开发调试。

## 文件清单

### 删除
- `src/App.tsx`
- `src/App.css`
- `src/typing.d.ts`
- `src/components/AppComponent.tsx`
- `src/components/HomePage.tsx`
- `src/components/PlayerPage.tsx`
- `src/components/PlayerControls.tsx`
- `src/components/PlaylistPage.tsx`
- `src/components/SettingsPage.tsx`

### 新建
- `src/main.ts`
- `src/App.vue`
- `src/router.ts`
- `src/views/HomeView.vue`
- `src/views/PlayerView.vue`
- `src/views/PlaylistView.vue`
- `src/views/SettingsView.vue`
- `src/components/PlayerControls.vue`
- `src/components/ProgressBar.vue`（自实现 Slider）
- `src/styles/variables.css`（共享 CSS 变量）

### 修改
- `package.json`
- `lynx.config.ts`
- `tsconfig.json`
- `preview.html`

## 风险与缓解

| 风险 | 缓解措施 |
|---|---|
| vue-lynx Pre-Alpha 有 bug | 先搭建最小 Hello World 验证 rspeedy + vue-lynx 能正常编译运行 |
| Slider 手势实现复杂 | 先用简化版（tap 定位），后续迭代加拖拽 |
| NativeModules 调用方式不同 | 验证 vue-lynx 中 `globalThis.NativeModules` 是否可用 |
| CI 构建失败 | 先在本地验证 `rspeedy build` 成功再推送 |
