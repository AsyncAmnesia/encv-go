# Checklist

## Phase 1: ComboLite 合规性诊断与修复（非 UI）

- [ ] `IPluginEntryClass` 接口契约已确认（onLoad/onUnload/Content/pluginModule 的要求）
- [ ] MpvPluginEntry vs OpenListPluginEntry 差距分析完成
- [ ] PluginLifecycleEngine 调用链路已确认
- [ ] Service ↔ Plugin 生命周期同步方案确定
- [ ] `onLoad()` / `onUnload()` 已修复符合规范
- [ ] Content() 内 Bridge 调用有 try-catch 防御
- [ ] Koin module 注册合规
- [ ] `./gradlew :plugin-openlist:compileDebugKotlin` 通过

## Phase 2: Plugin APK Content() 瘦身

- [ ] OpenListPluginEntry.kt 不包含 StatusCard/ControlCard/ConfigCard/InfoGrid/formatFileSize
- [ ] 无 Compose Material3 import
- [ ] Content() 返回空/最小占位
- [ ] build.gradle.kts 无 compose plugin / buildFeatures / compose dependencies
- [ ] 产物 AAR 不含 `androidx.compose.*` 类
- [ ] combolite-host 编译通过

## Phase 3: Host App — InAppBrowser 入口

- [ ] LocalOpenListStatusCard 包含"打开管理界面"按钮
- [ ] 按钮调用 Browser/InAppBrowser 打开 `http://127.0.0.1:{port}/#/`
- [ ] 按钮仅在 running=true 时 enabled
- [ ] Capacitor browser/inappbrowser 依赖已在 package.json
- [ ] vue-tsc --noEmit 通过 (0 errors)
