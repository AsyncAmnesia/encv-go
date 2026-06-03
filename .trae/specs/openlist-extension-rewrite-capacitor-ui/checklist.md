# Checklist

## Phase 0: ComboLite 合规修复

- [ ] IPluginEntryClass 接口契约已确认
- [ ] MpvPluginEntry vs OpenListPluginEntry 差距已分析
- [ ] OpenListPluginEntry.pluginModule = emptyList()
- [ ] OpenListPluginEntry.onLoad 初始化 OpenListBridge
- [ ] OpenListPluginEntry.onUnload shutdown Bridge + Service
- [ ] Content() 返回最小占位 Box
- [ ] StatusCard/ControlCard/ConfigCard/InfoGrid/formatFileSize 已删除
- [ ] build.gradle.kts 移除 compose plugin/buildFeatures/dependencies
- [ ] plugin-openlist 编译通过
- [ ] combolite-host 编译通过
- [ ] 产物 AAR 不含 androidx.compose.* 类

## Phase 1: plugin-openlist 改造为独立 Capacitor 应用

- [ ] OpenListMainActivity.kt 已创建（extends BridgeActivity）
- [ ] OpenListServicePlugin.kt 已创建（start/stop/getStatus/getVersion/getPort/getIsRunning）
- [ ] OpenListConfigPlugin.kt 已创建（read/write/getDataDir）
- [ ] OpenListPasswordPlugin.kt 已创建（setPassword）
- [ ] OpenListMainActivity 注册 3 个插件
- [ ] assets/capacitor.config.json 已创建
- [ ] assets/capacitor.plugins.json 已创建
- [ ] assets/public/index.html 占位已创建
- [ ] AndroidManifest.xml 注册独立 launcher Activity
- [ ] plugin-openlist assembleDebug 通过
- [ ] 产物 APK 包含 assets/public/index.html

## Phase 2: Capacitor Web 项目（K-Sillot Mobile 复刻）

- [ ] web/package.json 已创建（vue/ionic/capacitor 依赖）
- [ ] web/vite.config.ts / tsconfig.json / index.html 已创建
- [ ] web/src/plugins/OpenListService.ts + web stub 已创建
- [ ] web/src/plugins/OpenListConfig.ts + web stub 已创建
- [ ] web/src/plugins/OpenListPassword.ts + web stub 已创建
- [ ] web/src/main.ts + App.vue + router 已创建
- [ ] web/src/views/HomePage.vue（4 tab 容器）已创建
- [ ] web/src/views/WebScreen.vue（Tab 0 复刻）已创建
- [ ] web/src/views/OpenListScreen.vue（Tab 1 复刻）已创建
- [ ] web/src/views/DownloadManager.vue 已创建
- [ ] web/src/views/Settings.vue 已创建
- [ ] web/src/components/LogListView.vue 已创建
- [ ] web/src/components/PwdEditDialog.vue 已创建
- [ ] web/src/components/ConfigEditorPage.vue 已创建
- [ ] web/src/components/AboutDialog.vue 已创建
- [ ] npm install + npm run build 通过
- [ ] npx cap sync android 同步成功
- [ ] plugin-openlist assembleDebug 重新打包通过
- [ ] APK assets/public/ 包含 web bundle
- [ ] web vue-tsc --noEmit 通过

## Phase 3: 主 app 集成

- [ ] StartOpenListAppPlugin.kt 已创建
- [ ] MainActivity 注册 StartOpenListAppPlugin
- [ ] src/plugins/StartOpenListApp.ts + web stub 已创建
- [ ] LocalOpenListStatusCard.vue 增加"启动独立界面"按钮
- [ ] ExtensionsPage.vue 已安装 OpenList 卡片增加"启动"按钮
- [ ] 主 app vue-tsc --noEmit 通过

## Phase 4: 端到端验证

- [ ] 全模块编译通过
- [ ] plugin-openlist APK 独立启动（adb am start）
- [ ] Capacitor 加载 web 资源成功
- [ ] OpenListScreen FAB 启动 OpenList → WebScreen iframe 加载成功
- [ ] 密码设置 + Config 编辑 + 日志流验证
- [ ] 主 app 调 StartOpenListApp.start() → OpenListMainActivity 启动
- [ ] 主 app LocalOpenListStatusCard 通过 ContentProvider 读 OpenList 状态
