# 修复三个运行时缺陷 Spec

## Why

用户反馈三个运行时问题：

1. **文本预览滚动 bug**：默认换行模式下无法滚动内容，切换到不换行再切回即可滚动（CSS 初始化状态问题）
2. **安装确认界面不显示**：用户从未看到 InstallConfirmActivity，只看到 "Installation timeout"（120s 超时）
3. **加密视频两个错误**：
   - v4 容器：`stsz box missing`（与之前相同的验证错误）
   - v3 容器：ffprobe JSON 解析失败 `invalid character '[' after array element`

## What Changes

### 变更 1：文本预览滚动修复

**根因分析**：
- [FilePreview.vue L333-337](app/encv-mobile/src/views/FilePreview.vue#L333-L337)：`.text-preview` 容器设置了 `overflow: auto`
- [FilePreview.vue L340-345](app/encv-mobile/src/views/FilePreview.vue#L340-L345)：`.preview-iframe` 设置了 `width: 100%; height: 100%; flex: 1; border: none`
- text.html 内部 `#textContent` 也设置了 `overflow-y: auto; overflow-x: auto; height: 100vh`
- **嵌套 overflow 冲突**：父容器 `.text-preview` 的 `overflow: auto` 在初始渲染时劫持了滚动事件，导致 iframe 内部内容无法滚动。切换换行模式触发了 DOM reflow，重置了滚动状态，因此恢复滚动

**修复**：移除 `.text-preview` 的 `overflow: auto`，iframe 自身处理内部滚动

### 变更 2：安装确认界面不显示

**根因分析**：
- GoProcessPlugin 在 `PluginManager.isInitialized` 为 true 时启动 InstallConfirmActivity
- 但如果设备上 ComboLite 未正确初始化（isInitialized=false），代码 fallback 到系统安装器路径
- 系统安装器路径调用 `context.startActivity(intent)` 后立即 `call.resolve()` —— 但系统安装是异步的，用户操作完成后 call 已经 resolve 了
- **更关键的问题**：ExtensionsPage.vue L182 有 **120 秒超时** `Promise.race([pickAndInstallPlugin(), setTimeout(120000)])`
- 如果 `pickAndInstallPlugin()` 走了系统安装器路径，它在文件选择后很快返回 resolve（因为系统安装器 intent 发出后就 resolve 了），所以不应该超时
- **真正的超时原因**：走 ComboLite 路径时，`startActivityResult` 启动 InstallConfirmActivity 后函数 return（不 resolve/reject call），等待 onActivityResult 回调。但如果 Activity 崩溃或 onActivityResult 永远不被触发，call 就永远 pending → 120s 后超时

**可能原因**：
a) InstallConfirmActivity 因 Compose Theme 问题崩溃（MaterialTheme 需要在 setContent 中包裹）
b) `activity` 属性在 Capacitor 插件中为 null，导致 `activity.startActivityForResult()` 静默失败
c) APK 图标提取在主线程执行导致 ANR

**修复**：
- 在 `startActivityResult` 前检查 `activity != null`，null 时 fallback 到直接安装
- InstallConfirmActivity 中增加 try-catch 包裹 Compose 内容
- 减少超时时间或添加进度反馈

### 变更 3：加密视频错误

#### 3a: v4 stsz box missing

**根因**：
- [plugin.go L475-476](internal/v2/plugins/video/plugin.go#L475-L476)：当 dataReader 是 `*os.File` 时，`p.encryptedSourcePath = file.Name()` = inputPath
- [plugin.go L739](internal/v2/plugins/video/plugin.go#L739)：`sourcePath != p.inputPath` 判断为 false → SkipStructCheck=false
- [content_verifier.go L186-187](internal/v2/plugins/video/content_verifier.go#L186-L187)：stsz 检查对解密后文件执行 → 失败

**核心矛盾**：v4 容器加密后，输出文件的 MP4 结构可能与原始文件不同（容器格式转换、moov atom 重排等），导致原始文件的 stsz 信息不能用于验证解密后文件。

**修复**：对 v4 容器类型强制启用 SkipStructCheck，或在 verifyContainer 中检测容器版本并调整策略。

#### 3b: v3 ffprobe JSON 解析失败

**根因**：
- 错误信息 hex dump 显示 ffprobe 输出以 `{"frames": [` 开头
- 但 [metadata_extractor.go L180](internal/v2/plugins/video/metadata_extractor.go#L180) 使用的是 `-show_format -show_streams` 参数，应产生 `{"streams": [...], "format": {...}}` 格式
- `FFProbeRawMetadata` struct 期望的也是 streams/format 格式
- 输出不匹配说明 **实际执行的 ffprobe 命令与代码不一致**，或者 **FFmpeg 8.0 的 ffprobe 行为变化**

**修复**：检查 `ffmpeg.Probe()` 函数的实际实现，确认参数传递是否正确；对解析失败的情况增加容错（尝试兼容 frames 格式 或降级处理）。

## Impact

- Affected code:
  - `app/encv-mobile/src/views/FilePreview.vue` — CSS 修改
  - `app/encv-mobile/android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt` — 安装流程健壮性
  - `app/encv-mobile/android/app/src/main/java/com/encvgo/app/InstallConfirmActivity.kt` — 崩溃防护
  - `internal/v2/plugins/video/plugin.go` — SkipStructCheck 策略
  - `internal/v2/plugins/video/metadata_extractor.go` — ffprobe 解析容错
