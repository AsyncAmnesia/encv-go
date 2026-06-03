# Checklist

## Phase 1: ComboLite 合规性诊断与修复

- [ ] `IPluginEntryClass` 接口契约已确认
- [ ] MpvPluginEntry vs OpenListPluginEntry 差距分析完成
- [ ] PluginLifecycleEngine 调用链路已确认
- [ ] Service ↔ Plugin 生命周期同步方案确定
- [ ] `onLoad()` / `onUnload()` 已修复符合规范
- [ ] Content() 内 Bridge 调用有 try-catch 防御
- [ ] Koin module 注册合规
- [ ] `./gradlew :plugin-openlist:compileDebugKotlin` 通过

## Phase 2: 插件内 UI 重写（Content()）

> 取决于用户选择的方案 A 或 B

### 方案 A (Compose WebView):
- [ ] 现有手写 Compose UI (StatusCard/ControlCard/ConfigCard) 已删除
- [ ] Content() 使用 AndroidView(WebView) 加载 OpenList SPA
- [ ] WebView 错误自愈逻辑实现
- [ ] 端口动态获取
- [ ] build.gradle.kts 仅保留 compose.ui (AndroidView)，移除 material3 等
- [ ] 编译通过

### 方案 B (纯 Compose 原生):
- [ ] Content() 精简为最小控制面板
- [ ] ConfigCard 已删除（配置在 Web UI 操作）
- [ ] StatusCard 精简为 running 状态
- [ ] "打开 Web UI" 按钮存在
- [ ] 编译通过

## Phase 3: 验证

- [ ] plugin-openlist 编译通过
- [ ] combolite-host 编译通过
- [ ] Host App vue-tsc 通过（不受影响）
