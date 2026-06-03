# Checklist

## Phase 0: ComboLite 合规修复（与 UI 无关）

- [ ] IPluginEntryClass 接口契约已确认
- [ ] MpvPluginEntry vs OpenListPluginEntry 差距已分析
- [ ] OpenListPluginEntry.pluginModule = emptyList()
- [ ] OpenListPluginEntry.onLoad 初始化 OpenListBridge
- [ ] OpenListPluginEntry.onUnload shutdown Bridge + Service
- [ ] Content() 用 OpenListEmbedWebView 替代 Compose UI
- [ ] StatusCard / ControlCard / ConfigCard / InfoGrid / formatFileSize 已删除
- [ ] build.gradle.kts 移除 compose plugin / buildFeatures / dependencies
- [ ] plugin-openlist 编译通过
- [ ] combolite-host 编译通过

## Phase 1: 嵌入式 WebView + JS-Native 桥接

- [ ] OpenListEmbedWebView.kt 已创建（@Composable + AndroidView）
- [ ] OpenListPluginJSInterface.kt 已创建（@JavascriptInterface 暴露 start/stop/getStatus/setPassword/readConfig/writeConfig/getVersion）
- [ ] OpenListWebViewClient.kt 已创建
- [ ] plugin-openlist 编译通过

## Phase 2: Monorepo 改造（pnpm workspace）

- [ ] pnpm-workspace.yaml 已创建（app, plugin-openlist/web, packages/*）
- [ ] 项目根 package.json 添加 packageManager
- [ ] packages/components/package.json 已创建（@encvgo/components）
- [ ] packages/components/src/index.ts 已创建
- [ ] packages/components/src/OpenListStatusCard.vue 已移植
- [ ] packages/components/src/OpenListLogList.vue 已移植
- [ ] plugin-openlist/web/package.json 已创建（@encvgo/components: workspace:*）
- [ ] plugin-openlist/web/vite.config.ts / tsconfig.json / index.html 已创建
- [ ] pnpm install 在 packages/components 和 plugin-openlist/web 成功

## Phase 3: 插件 web 项目页面

- [ ] plugin-openlist/web/src/main.ts / App.vue / router/index.ts 已创建
- [ ] OpenListHome.vue 已创建（K-Sillot OpenListScreen 复刻：AppBar + FAB + 状态卡 + 日志流）
- [ ] OpenListConfigEditor.vue 已创建（K-Sillot ConfigEditorPage 复刻：JSON 编辑 + 校验 + 备份）
- [ ] OpenListSettings.vue 已创建（简化版：版本/数据目录）
- [ ] OpenListWebView.vue 已创建（iframe 加载 OpenList SPA，可选）
- [ ] openlist-native.ts 已创建（Window.OpenListNative 类型 + 包装对象）
- [ ] pnpm run build 产出 dist/
- [ ] vue-tsc --noEmit 通过

## Phase 4: 主 app 集成

- [ ] app/src/router/index.ts 添加 /openlist 路由
- [ ] 主 app pnpm install + pnpm run build 通过
- [ ] 主 app vue-tsc --noEmit 通过

## Phase 5: 编译与部署验证

- [ ] plugin-openlist 编译通过
- [ ] combolite-host 编译通过
- [ ] app 编译通过
- [ ] 插件 web dist 产出
- [ ] 主 app dist 产出
- [ ] 沙箱预览启动正常
- [ ] 主 app /openlist 路由可访问
- [ ] 嵌入式 WebView 通过 OpenListNative JSInterface 调 OpenListBridge 成功
- [ ] 共享组件 OpenListStatusCard / OpenListLogList 在主 app 和插件都可用
