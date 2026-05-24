# 修复 ArtPlayer 样式 + 全屏处理 + MPV 错误处理 + 外部打开

## 问题 1: ArtPlayer 样式缺失

### 根因
ArtPlayer 5.x 的 CSS 是通过 JS 动态注入到 `document.head` 的（`setStyleText("artplayer-style", style)`）。
在 Capacitor WebView 中，这个机制本身应该能工作。但用户报告"artplayer的控制台日志有，但外观样式都没有"。

**最可能的原因**：ArtPlayer 的 CSS 虽然注入到了 `<head>`，但可能被 Ionic 的全局 CSS 覆盖或干扰。
特别是 Ionic 的 `normalize.css` 可能重置了 ArtPlayer 使用的某些样式。

**另一个可能**：ArtPlayer 的 `container` 设置为 `artContainer.value`，但容器没有设置 `position: relative`，
导致 ArtPlayer 的绝对定位控件溢出或不可见。

### 修复方案
1. 给 `.video-player` 容器添加 `position: relative; overflow: hidden;` 确保定位正确
2. 在 `initArtPlayer` 中添加 CSS 注入检查日志，确认 `artplayer-style` 是否存在于 `<head>`
3. 如果 CSS 未注入，手动调用 `Artplayer.STYLE` 注入

## 问题 2: 全屏进入/退出需要 Capacitor 插件

### 根因
当前全屏处理使用 `GoProcess.setScreenOrientation()`，这是自定义的 Capacitor 插件方法。
退出全屏后没有正确恢复屏幕方向，且没有处理状态栏显示/隐藏。

### 修复方案
1. 安装 `@capacitor/status-bar` 和 `@capacitor/screen-orientation` 官方插件
2. 在 `ArtPlayerView.vue` 的全屏事件中：
   - 进入全屏：`StatusBar.hide()` + `ScreenOrientation.lock(ScreenOrientationOrientation.LANDSCAPE)`
   - 退出全屏：`StatusBar.show()` + `ScreenOrientation.lock(ScreenOrientationOrientation.PORTRAIT)`
3. 在 `onBeforeUnmount` 中恢复状态栏和屏幕方向
4. 同步更新 MPV 播放器（PlayerApp.tsx）的全屏处理逻辑

## 问题 3: 设置中添加屏幕方向选项

### 修复方案
在 Settings.vue 的"播放器"区域添加屏幕方向选择：
- 自动（跟随传感器）
- 竖屏锁定
- 横屏锁定

存储到 `localStorage` 的 `encv_screen_orientation` 键。
在 App.vue 中根据设置初始化屏幕方向。

## 问题 4: MPV 播放错误"Unknown"且播放地址为空

### 根因
用户说"菜的抠脚，既没有识别错误类型也没有原始错误信息"。

查看 PlayerApp.tsx 的错误处理：
- `startPlayback` 中 `setErrorMessage('播放地址为空')` — 太简略，没有上下文
- `mpv:state-change` 事件中 `if (error) setErrorMessage(error)` — 但 MPV 的错误可能是数字代码
- `PlayerControls.tsx` 错误显示只有 `⚠ 播放失败` + `fileName` + `error` + `streamUrl`
- 没有 LogBridgeModule 推送错误到 DevLogs

**关键问题**：当 `initStreamUrl` 为空且 `initFilePath` 也为空时，错误信息只是"播放地址为空"，
没有说明 initData 的完整内容，无法诊断问题来源。

### 修复方案
1. **PlayerApp.tsx**：增强错误信息，包含 initData 完整内容、错误类型分类
2. **PlayerApp.tsx**：所有错误路径都通过 `lynxLog.error()` 推送到 LogBridgeModule（→ DevLogs）
3. **PlayerControls.tsx**：错误显示增加错误类型标签（如"网络错误"、"文件不存在"、"解码失败"等）
4. **PlayerApp.tsx**：MPV 错误事件中解析错误代码，映射为可读的错误类型

## 问题 5: 外部打开链接是内部路径拼接的无效 URL

### 根因
`PlayerActivityLynx.buildInitDataJson()` 中：
- 外部文件：`"http://127.0.0.1:$port/api/stream/external?path=$encodedPath"` — 这是正确的
- 内部文件：`"http://127.0.0.1:$port/stream?path=$encodedPath"` — 但 `intentFilePath` 对于外部文件是绝对路径如 `/storage/emulated/0/Movies/test.mp4`

**但更关键的问题**：`EncvGoService.lastKnownPort` 可能为 0（后端还没启动），
此时 `streamUrl` 为空字符串，导致"播放地址为空"错误。

另外，`android.net.Uri.encode(intentFilePath)` 会编码整个路径包括 `/`，
导致后端收到的 path 是 `%2Fstorage%2Femulated%2F0%2F...`，
后端 `handleStreamExternalFileGin` 需要正确解码。

### 修复方案
1. **PlayerActivityLynx**：如果 `lastKnownPort <= 0`，等待后端启动后再构建 initData
2. **PlayerActivityLynx**：对 `intentFilePath` 只编码值部分，不编码 `/`（使用 `Uri.encode(path, "/")`）
3. **PlayerActivityLynx**：添加日志记录完整的 initData JSON 和端口状态

## 实施步骤

### Step 1: 安装 Capacitor 插件
```bash
npm install @capacitor/status-bar @capacitor/screen-orientation
npx cap sync android
```

### Step 2: ArtPlayerView.vue — 修复样式 + 全屏
1. `.video-player` 添加 `position: relative; overflow: hidden;`
2. `initArtPlayer` 添加 CSS 注入检查
3. 全屏处理改用 `@capacitor/status-bar` + `@capacitor/screen-orientation`
4. `onBeforeUnmount` 恢复状态栏和方向

### Step 3: PlayerApp.tsx — 增强 MPV 错误处理
1. 所有错误路径推送 `lynxLog.error()` 到 LogBridgeModule
2. 错误信息包含 initData 完整内容
3. MPV 错误事件解析错误代码，映射为可读类型
4. `startPlayback` 错误信息包含 `resolvedStreamUrl` 和 `fileName`

### Step 4: PlayerControls.tsx — 增强错误显示
1. 错误类型标签（网络错误/文件错误/解码错误/未知错误）
2. 分层显示：错误类型 → 文件名 → 详细信息 → streamUrl

### Step 5: PlayerActivityLynx.kt — 修复外部打开
1. 等待后端端口就绪后再构建 initData
2. `Uri.encode(path, "/")` 保留 `/` 不编码
3. 添加详细日志

### Step 6: Settings.vue — 添加屏幕方向选项
1. 在"播放器"区域添加屏幕方向选择
2. 选项：自动/竖屏锁定/横屏锁定
3. 存储到 localStorage

### Step 7: App.vue — 初始化屏幕方向
1. 根据设置初始化屏幕方向

### Step 8: 构建验证
```bash
vue-tsc --noEmit && vite build
```
