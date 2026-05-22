# 修复 Lynx 播放器黑屏问题

## 现象
闪退已修复（AndroidManifest 注册 Activity 生效），但播放器界面全黑，无任何可见 UI 元素。

## 根因分析

### 布局层级结构

```
FrameLayout (rootLayout, 黑色背景, MATCH_PARENT)
├── [index 0] MpvSurfaceView (SurfaceView, MATCH_PARENT) ← 最底层
└── [index 1+] LynxView (MATCH_PARENT) ← 最上层（后添加，z-index 更高）
    └── <page> (❌ 无样式!)
        └── <view className="PlayerContainer"> (flex:1, bg:#000)
            └── PlayerControls (idle → ControlsOverlay 底部控制栏)
```

### 🔴 P0：`<page>` 根元素缺少尺寸样式（最可能的黑屏根因）

**文件**：`lynx-player/src/App.tsx` 第 106-127 行

Lynx 的 `<page>` 类似于 Web 的 `<body>`，是渲染树的根容器。当前 `<page>` 没有设置任何样式属性。在 Lynx 中：

- `<page>` **不会自动填充父容器**，需要显式设置 `flex: 1`
- 子元素 `PlayerContainer` 设置了 `flex: 1`，但它的父容器 `<page>` 高度为 0 或不确定
- 这导致整个 Lynx 渲染内容区域高度为 **0**，所有子元素不可见

**修复**：给 `<page>` 添加 `style={{ flex: 1 }}`

### 🟡 P1：`background: linear-gradient()` 可能不被 Lynx 支持

**文件**：`lynx-player/src/App.css` 第 8 行

```css
.ControlsOverlay {
  background: linear-gradient(transparent, rgba(0, 0, 0, 0.85));
}
```

虽然 Lynx 文档示例中有 `radial-gradient` 用法，但：
- `linear-gradient` 的支持可能不完整或语法不同
- 如果 CSS 解析失败，背景变为完全透明
- 在黑色 PlayerContainer 上，白色文字应该仍可见...**除非 P0 导致容器高度为 0**

**修复**：改为纯色半透明背景 `background-color: rgba(0, 0, 0, 0.85)`，或使用 Lynx 确认支持的 gradient 语法

### 🟡 P2：idle 状态下 UI 可见性差

**文件**：`lynx-player/src/App.tsx` + `PlayerControls.tsx`

当前 idle 状态的渲染路径：
1. `playerState = "idle"` → `isOverlay = true` → `justifyContent = "center"`
2. `showControls = true` → 渲染 PlayerControls
3. PlayerControls 不是 "error" 不是 "loading" → 返回 **ControlsOverlay**（底部控制栏）
4. ControlsOverlay 包含：文件名（可能为空）、全屏按钮、▶ 播放按钮、进度条

问题：idle 状态下显示的是底部对齐的控制栏（`justify-content: flex-end` 是默认），但 `isOverlay=true` 时被覆盖为居中。然而 ControlsOverlay 本身的设计是底部面板，在屏幕中间看起来会很奇怪。

**修复**：idle 状态下也显示 LoadingIndicator 风格的居中UI（大播放按钮 + 文件名）

### 🟢 P3：SurfaceView 层级确认

MpvSurfaceView 通过 `rootLayout.addView(mpvSurfaceView, 0, params)` 插入到 index 0（最底层）。在 FrameLayout 中：
- 后添加的 view（LynxView）在上层
- SurfaceView 不应遮挡 LynxView

但如果 LynxView 内容高度为 0（P0），用户看到的实际上是 FrameLayout 的黑色背景 + 底层的黑色 SurfaceView。

## 修复步骤

### Step 1：给 `<page>` 添加 flex: 1 样式

**文件**：`lynx-player/src/App.tsx`

将：
```tsx
return (
    <page>
      <view className="PlayerContainer" ...>
```

改为：
```tsx
return (
    <page style={{ flex: 1 }}>
      <view className="PlayerContainer" ...>
```

这是最关键的修复，`<page>` 必须有明确的尺寸才能让子元素的 flex 布局生效。

### Step 2：替换 linear-gradient 为纯色背景

**文件**：`lynx-player/src/App.css`

将：
```css
.ControlsOverlay {
  background: linear-gradient(transparent, rgba(0, 0, 0, 0.85));
  ...
}
```

改为：
```css
.ControlsOverlay {
  background-color: rgba(0, 0, 0, 0.85);
  ...
}
```

### Step 3：idle 状态显示居中大播放按钮

**文件**：`lynx-player/src/components/PlayerControls.tsx`

在现有的 `if (playerState === "loading")` 判断之前，添加 idle 状态处理：

```tsx
if (playerState === "idle") {
    return (
      <view className="LoadingIndicator">
        <text className="PlayButton" bindtap={onPlayPause}>▶</text>
        {fileName && <text className="LoadingText">{fileName}</text>}
      </view>
    );
}
```

同时在 App.tsx 中，让 idle 状态下的点击直接触发播放（通过 handleToggleControls 已经可以切换 showControls，但更好的 UX 是点击直接开始播放）。

### Step 4（可选）：添加 LynxView 背景色验证渲染

**文件**：`android-overlay/app/src/main/java/com/encvgo/app/PlayerActivityLynx.kt`

在 `createLynxView()` 中 lynxView 创建后设置背景色用于调试：

```kotlin
lynxView?.setBackgroundColor(android.graphics.Color.parseColor("#FF0010"))
```

如果看到红色/品红色背景说明 LynxView 正确渲染，问题在 CSS/JS 层。如果仍然是黑色说明 LynxView 本身没有正确显示。

此步骤仅用于验证，确认问题后应移除。

## 预期效果

- Step 1 修复后，Lynx 内容区域正确填充整个屏幕，不再高度为 0
- Step 2 修复后，控制面板背景正确显示半透明黑色
- Step 3 修复后，初始状态显示居中的大播放按钮和文件名，用户体验更好
- 用户能看到播放器 UI，点击播放按钮后 mpv 开始播放视频
