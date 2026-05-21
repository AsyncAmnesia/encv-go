# 播放器修复计划：ArtPlayer 原生控件 + 全屏退出竖屏 + 设置重构

## 问题分析

### 问题 1：ArtPlayer 显示浏览器原生控件
**现象**：logcat 显示 ArtPlayer 初始化成功，但视频仍然显示浏览器原生控件而非 ArtPlayer 自定义 UI。

**根因**：Android WebView 对 `<video>` 元素有特殊行为。即使 JS 设置 `video.controls = false`，WebView 仍可能渲染原生控件。ArtPlayer 5.4.0 通过 `moreVideoAttr: { controls: false }` 设置 `video.controls = false`，但这在 WebView 中不够可靠。

**修复方案**：
1. 在 StandalonePlayer.vue 的 `<style>` 中添加 CSS 强制隐藏 WebKit 原生控件伪元素：
   ```css
   video::-webkit-media-controls { display: none !important; }
   video::-webkit-media-controls-enclosure { display: none !important; }
   video::-webkit-media-controls-panel { display: none !important; }
   ```
2. 在 `initArtPlayer()` 创建实例后，显式调用 `art.video.removeAttribute('controls')` + `art.video.controls = false` 作为双重保障

### 问题 2：退出全屏后未恢复竖屏
**现象**：全屏时锁定横屏，退出全屏后 `handleFullscreenExit()` 调用 `setScreenOrientation({ orientation: 'unlocked' })`，但屏幕仍保持横屏。

**根因**：`unlocked` 对应 `SCREEN_ORIENTATION_UNSPECIFIED`，这不会强制回到竖屏，而是允许当前方向继续。在 PlayerActivity 中，退出全屏后应该明确恢复为竖屏。

**修复方案**：
- `handleFullscreenExit()` 中将 `orientation` 从 `'unlocked'` 改为 `'portrait'`
- GoProcessPlugin.kt 中 `setScreenOrientation` 添加 `'portrait'` 对应 `SCREEN_ORIENTATION_SENSOR_PORTRAIT`（已有，无需改 Kotlin）
- 前端只需改一个字符串

### 问题 3：设置界面插件设置统一到单独的二级界面
**现象**：当前 PlayerSettings.vue 是一个扁平列表，所有设置平铺。用户要求将插件相关设置统一到单独的二级界面。

**当前 PlayerSettings 结构**：
- 播放：自动播放、画中画、后台播放
- 外观：深色模式
- 高级：硬件解码、清除播放缓存

**修复方案**：
- 将"高级"分组改为"插件"分组，点击进入独立二级页面 PlayerPluginSettings.vue
- PlayerPluginSettings.vue 包含：硬件解码、清除播放缓存，以及未来可能添加的插件相关设置
- PlayerSettings.vue 中"插件"分组显示为一个带箭头的导航项，点击后 `currentView` 切换到 `'plugin-settings'`
- PlayerApp.vue 中添加 `PlayerPluginSettings` 组件的条件渲染

---

## 修复步骤

### 步骤 1：修复 ArtPlayer 原生控件问题
**文件**：`src/views/StandalonePlayer.vue`

1. 在 `<style scoped>` 中添加 WebKit 原生控件隐藏 CSS（注意 `scoped` 下需要 `:deep()` 穿透）
2. 在 `initArtPlayer()` 的 `new Artplayer(...)` 之后添加：
   ```typescript
   nextTick(() => {
     if (art?.video) {
       art.video.removeAttribute('controls')
       art.video.controls = false
     }
   })
   ```

### 步骤 2：修复退出全屏后恢复竖屏
**文件**：`src/views/StandalonePlayer.vue`

- `handleFullscreenExit()` 中 `orientation` 从 `'unlocked'` 改为 `'portrait'`

### 步骤 3：创建 PlayerPluginSettings.vue
**文件**：`src/views/PlayerPluginSettings.vue`（新建）

- 从 PlayerSettings.vue 提取"高级"分组内容
- 添加返回按钮，emit('close') 回到设置主页
- 包含：硬件解码开关、清除播放缓存

### 步骤 4：修改 PlayerSettings.vue
**文件**：`src/views/PlayerSettings.vue`

- 删除"高级"分组
- 添加"插件"导航项（带箭头），点击 emit('open-plugin-settings')
- 添加 defineEmits 中的 'open-plugin-settings'

### 步骤 5：修改 PlayerApp.vue
**文件**：`src/PlayerApp.vue`

- 导入 PlayerPluginSettings
- currentView 类型扩展为 `'player' | 'settings' | 'plugin-settings'`
- 添加 PlayerPluginSettings 条件渲染

### 步骤 6：本地构建验证
- 执行 `npm run build` 确认零错误
