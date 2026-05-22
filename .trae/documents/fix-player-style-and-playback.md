# 修复播放器样式异常 + 播放无响应

## 问题诊断

### 问题 A：播放控件样式异常（可交互但显示不对）

从 [App.css](app/encv-mobile/lynx-player/src/App.css) 分析，以下 CSS 属性在 **Lynx 中不支持或行为异常**：

| 行号 | 属性 | Lynx 支持情况 | 视觉影响 |
|------|------|--------------|---------|
| L51-53 | `border-radius: 36px` (PlayButtonCircle) | ⚠️ 部分支持，大值可能异常 | 播放按钮非圆形 |
| L108 | `overflow: hidden` (SliderTrackOuter) | ❌ 不支持 | 进度条溢出 |
| L129-133 | `position: absolute` (SliderThumb) | ❌ 不支持 | 滑块 thumb 完全错位 |
| L140 | `gap: 8px` (LoadingDots) | ⚠️ Lynx 3.7 可能不支持 | 加载点间距失效 |
| L31-32 | `text-overflow: ellipsis` (TitleText) | ⚠️ 需配合固定宽度 | 标题文字可能不截断 |

**最大问题**：SliderThumb 使用 `position: absolute` — 这是 Lynx 明确不支持的属性，整个进度条组件视觉异常。

### 问题 B：点击播放无反应

完整链路追踪（[AppComponent.tsx](app/encv-mobile/lynx-player/src/components/AppComponent.tsx) L74-111）：

```
用户点 ▶ → handlePlayPause() → playerState=="idle"
  → startPlayback(initData)
    → Step 1: GoBackendModule.getBackendStatus()
    → Step 2: (如需) GoBackendModule.startBackend()
    → Step 3: GoBackendModule.getStreamUrl(filePath, isExternal)
    → Step 4: MpvPlayerModule.play(url)
```

**可能的断点**：

| 断点 | 风险 | 原因 |
|------|------|------|
| `initData` 为空 | 高 | [PlayerActivityLynx.resolveFileInfo()](app/encv-mobile/android-overlay/app/src/main/java/com/encvgo/app/PlayerActivityLynx.kt#L102) 未解析到文件路径时 filePath="" |
| 后端未就绪 | 中 | `getStreamUrl()` 在 port≤0 时返回错误字符串而非 URL |
| MPV surface 未就绪 | 高 | [MpvPlayerModule.play()](app/encv-mobile/android-overlay/app/src/main/java/com/encvgo/app/MpvPlayerModule.kt#L158) 中 `surfaceReady=false` 时只排队不播放；但 surfaceCreated 回调可能因 SurfaceView z-order 问题未被触发 |
| 错误被静默吞掉 | 高 | 上轮修复移除了 Toast 报错，错误仅走 LogBridge → logcat → DevLogs，用户在前端看不到任何反馈 |

**关键发现**：`startPlayback` 的 catch 块调用 `setPlayerState("error")` + `setErrorMessage(...)`，但 [PlayerControls.tsx](app/encv-mobile/lynx-player/src/components/PlayerControls.tsx) 的 error 分支只显示文本，**没有自动重试机制**。且用户说"没有 toast 了"说明错误确实在发生但不可见。

---

## 修复方案

### Step 1：重写 App.css — 移除所有 Lynx 不支持的 CSS 属性

**文件**：`app/encv-mobile/lynx-player/src/App.css`

具体改动：
1. **删除 SliderThumb 整个规则**（L124-134）— `position: absolute` 不可替代，改为不用 thumb 的进度条设计
2. **删除 `overflow: hidden`**（L109）— Lynx 不支持
3. **删除 `gap: 8px`**（L140）— 用 margin 替代
4. **`border-radius`** 保留但降低到合理值（Lynx 对超大 border-radius 渲染可能有问题）
5. 确保所有颜色使用标准格式（`#rrggbb` 或 `rgb()` 而非 `rgba()` 以防兼容问题）

重写后的核心布局策略：
- 进度条改用**纯宽度百分比**的嵌套 view 结构（外层背景 + 内层填充），不依赖 overflow/absolute
- 加载动画用 margin 替代 gap
- 保持三段式布局不变（TopBar / CenterArea / BottomBar）

### Step 2：增强前端错误可见性 + 播放状态反馈

**文件**：`app/encv-mobile/lynx-player/src/components/AppComponent.tsx`

改动：
1. `startPlayback` catch 块中除了 setPlayerState("error") 外，额外通过 `lynxLog.error()` 确保日志输出
2. 在 error 状态的 UI 中增加 **"点击重试"** 按钮（复用 onPlayPause）
3. 确保 idle 状态下 `initData` 为空时有明确提示（而非静默）

**文件**：`app/encv-mobile/lynx-player/src/components/PlayerControls.tsx`

改动：
1. error 状态容器内添加重试按钮：
   ```jsx
   <view className="PlayButtonCircle" bindtap={onPlayPause}>
     <text className="PlayIconLarge">🔄</text>
   </view>
   <text className="ErrorDetail">{error}</text>
   ```
2. idle 状态下如果 fileName 为空，显示 "等待文件..." 而非空白

### Step 3：MPV 播放链路防御性修复

**文件**：`app/encv-mobile/android-overlay/app/src/main/java/com/encvgo/app/MpvPlayerModule.kt`

改动：
1. `play()` 方法中增加：当 surfaceReady=false 且有 pendingUrl 时，**主动触发一次 Lynx 全局事件**通知前端正在等待 surface
2. `surfaceCreated()` 回调中确认 attachSurface 成功后，额外 dispatchStateChange("surface_ready") 让前端知道可以重试

### Step 4：重新构建 lynx bundle 并验证

```bash
cd app/encv-mobile/lynx-player && npm run build
```

验证清单：
- `node --check scripts/post-cap-sync.mjs` 通过
- PlayerControls.tsx 中无 `<page>` 标签（上轮已修复）
- App.css 中无 `position: absolute`、`overflow: hidden`、`gap:` 属性
