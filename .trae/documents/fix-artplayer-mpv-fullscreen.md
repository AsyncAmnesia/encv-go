# 修复 ArtPlayer 功能 + 全屏处理 + MPV 错误处理 + 外部打开

## 问题 1: ArtPlayer 高级控件缺失（核心问题）

### 根因
ArtPlayer 的很多功能默认是 **关闭** 的。当前配置只启用了：
```js
{ autoplay, autoSize, autoMini, mutex, playsInline, theme, volume, fullscreen, miniProgressBar }
```

ArtPlayer 默认关闭的功能（`false`）：
- `setting` — **设置面板**（包含字幕轨选择、播放速度、宽高比等入口）
- `playbackRate` — 播放速度选择
- `aspectRatio` — 宽高比选择
- `flip` — 翻转
- `lock` — 移动端锁定
- `autoOrientation` — 自动旋转
- `autoPlayback` — 记忆播放
- `subtitleOffset` — 字幕偏移
- `fastForward` — 移动端快进手势

用户反馈"弹幕开关/字幕轨选择等高级控件都没有显示"，因为 `setting: false` 导致设置面板不渲染。

### 修复方案
启用 ArtPlayer 移动端完整功能集：
```js
{
  setting: true,        // 设置面板（字幕/速度/宽高比/翻转）
  playbackRate: true,   // 播放速度
  aspectRatio: true,    // 宽高比
  flip: true,           // 翻转
  lock: true,           // 移动端锁定
  autoOrientation: true,// 自动旋转
  autoPlayback: true,   // 记忆播放
  subtitleOffset: true, // 字幕偏移
  fastForward: true,    // 快进手势
  hotkey: true,         // 快捷键
  gesture: true,        // 手势控制
}
```

同时给 `.video-player` 容器添加 `position: relative; overflow: hidden;` 确保定位正确。

## 问题 2: 全屏进入/退出需要 Capacitor 插件

### 根因
当前全屏处理使用 `GoProcess.setScreenOrientation()`，退出全屏后没有正确恢复屏幕方向，
且没有处理状态栏显示/隐藏。

### 修复方案
1. 安装 `@capacitor/status-bar` 和 `@capacitor/screen-orientation` 官方插件
2. ArtPlayerView.vue 全屏事件：
   - 进入全屏：`StatusBar.hide()` + `ScreenOrientation.lock(LANDSCAPE)`
   - 退出全屏：`StatusBar.show()` + `ScreenOrientation.lock(PORTRAIT)`
3. `onBeforeUnmount` 恢复状态栏和屏幕方向
4. MPV 播放器全屏也同步处理

## 问题 3: 设置中添加屏幕方向选项

### 修复方案
Settings.vue 播放器区域添加屏幕方向选择：
- 自动（跟随传感器）
- 竖屏锁定
- 横屏锁定

## 问题 4: MPV 播放错误"Unknown"且播放地址为空

### 根因
1. 错误信息太简略："播放地址为空"没有上下文
2. MPV 错误事件只传递原始 error 字符串，没有分类
3. 错误没有推送到 LogBridgeModule → DevLogs
4. PlayerControls 错误显示没有错误类型标签

### 修复方案
1. **PlayerApp.tsx**：所有错误路径推送 `lynxLog.error()` 到 LogBridgeModule
2. 错误信息包含 initData 完整 JSON、resolvedStreamUrl、fileName
3. MPV 错误分类：网络错误/文件错误/解码错误/未知错误
4. **PlayerControls.tsx**：错误显示增加错误类型标签

## 问题 5: 外部打开路径无效（Go 后端流式播放）

### 根因
外部打开必须通过 Go 后端 `/api/stream/external` 流式播放（才能解密加密容器视频）。

用户实际错误：`http://127.0.0.1:2025/api/stream/external?path=%2F123云盘%2Fe844e9ffb6650d294870a6d5ea963ec6mp4.mp4`
后端返回：`stat /123云盘/e844e9ffb6650d294870a6d5ea963ec6mp4.mp4: no such file or directory`

问题出在 `PlayerActivityLynx.resolveFileInfo()`：
- 对于 `file://` URI，`uri.path` 返回 `/123云盘/...` 而不是 `/storage/emulated/0/123云盘/...`
- Android 的 `file://` URI 格式可能是 `file:///123云盘/...`（缺少 `/storage/emulated/0` 前缀）
- 对于 `content://` URI，`MediaStore.MediaColumns.DATA` 可能返回不完整路径
- 路径缺少 Android 存储前缀

### 修复方案
1. **PlayerActivityLynx.resolveFileInfo()**：
   - `file://` URI：检查路径是否以 `/storage/` 或 `/data/` 开头，否则尝试拼接 `/storage/emulated/0/` 前缀
   - `content://` URI：如果 `MediaStore` 查询返回的路径不存在，优先 `copyContentToCache`（缓存文件路径一定是有效的绝对路径）
   - 添加路径有效性检查（`File(path).exists()`），无效路径回退到 `copyContentToCache`
2. **`Uri.encode(intentFilePath, "/")`**：保留 `/` 不编码
3. **后端**：`StreamExternalFile` 改进错误信息，返回实际查找的绝对路径
4. 等待后端端口就绪后再构建 streamUrl

## 实施步骤

### Step 1: 安装 Capacitor 插件
```bash
npm install @capacitor/status-bar @capacitor/screen-orientation
npx cap sync android
```

### Step 2: ArtPlayerView.vue — 启用完整功能 + 全屏处理
1. 启用 setting/playbackRate/aspectRatio/flip/lock/autoOrientation/autoPlayback/subtitleOffset/fastForward
2. `.video-player` 添加 `position: relative; overflow: hidden;`
3. 全屏处理改用 `@capacitor/status-bar` + `@capacitor/screen-orientation`
4. `onBeforeUnmount` 恢复状态栏和方向

### Step 3: PlayerApp.tsx — 增强 MPV 错误处理
1. 所有错误路径推送 `lynxLog.error()` 到 LogBridgeModule
2. 错误信息包含 initData 完整内容
3. MPV 错误分类映射
4. `startPlayback` 错误信息包含 resolvedStreamUrl 和 fileName

### Step 4: PlayerControls.tsx — 增强错误显示
1. 错误类型标签
2. 分层显示：错误类型 → 文件名 → 详细信息 → streamUrl

### Step 5: PlayerActivityLynx.kt — 修复外部打开路径
1. `file://` URI 路径补全（添加 `/storage/emulated/0/` 前缀）
2. 路径有效性检查，无效回退 `copyContentToCache`
3. `Uri.encode(path, "/")` 保留 `/`
4. 等待后端端口就绪
5. 详细日志

### Step 6: 后端 StreamExternalFile 改进错误信息
1. 错误返回包含实际查找的绝对路径

### Step 7: Settings.vue — 添加屏幕方向选项
1. 播放器区域添加屏幕方向选择
2. 选项：自动/竖屏锁定/横屏锁定

### Step 8: App.vue — 初始化屏幕方向
1. 根据设置初始化屏幕方向

### Step 9: 构建验证
```bash
vue-tsc --noEmit && vite build
```
