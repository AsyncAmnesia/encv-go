# 修复：mpvModule=null + 引入 lynx-ui 组件库

## 问题分析

### 错误现象
前端显示"正在初始化视频窗口..."，后端日志：
```
ERROR [PlayerActivityLynx] createLynxView: cannot attachToLayout,
  rootLayout=FrameLayout{...}, mpvModule=null
```

### 根因

**Native Module 生命周期时序问题**：`createLynxView()` 在 `viewBuilder.build()` 之后立即调用 `MpvPlayerModule.getInstance()`，但 Lynx Native Module 是**懒创建**的——只有当 JS 首次调用 `NativeModules.MpvPlayerModule.xxx()` 时才实例化。`renderTemplateUrl()` 在 getInstance 之后才调用，所以此时模块必然为 null。

```
createLynxView() {
  viewBuilder.registerModule("MpvPlayerModule", class)  // 注册类，不创建实例
  lynxView = viewBuilder.build()
  mpvModule = MpvPlayerModule.getInstance()  // ← null！还没创建
  mpvModule.attachToLayout(rootLayout)       // ← 跳过
  lynxView.renderTemplateUrl(...)            // ← 这之后 JS 才触发模块创建
}
```

---

## 修复方案

### Part A：修复 mpvModule=null（Native 端）

#### Step 1：MpvPlayerModule.init{} 中自动 attachToLayout

当 Lynx 框架创建 MpvPlayerModule 实例时，在 `init {}` 块中自动通过 Activity 引用找到 rootLayout 并 attach：

```kotlin
init {
    _instance = this
    val act = activity
    if (act is PlayerActivityLynx) {
        val root = act.findViewById<FrameLayout>(R.id.lynx_player_root)
        if (root != null) {
            attachToLayout(root)
        }
    }
}
```

#### Step 2：PlayerActivityLynx — 移除同步 attach，改为回调兜底

移除 `createLynxView()` 中的同步 `MpvPlayerModule.getInstance()` + `attachToLayout` 调用。改为在 `onLoadSuccess()` 回调中检查并兜底 attach。

### Part B：引入 lynx-ui 组件库（前端优化）

`@lynx-js/lynx-ui` 是 Lynx 官方 headless UI 组件库，已开源发布到 npm（v3.133.0），兼容 Lynx Engine >= 3.2。

#### 可用于播放器的 lynx-ui 组件

| 组件 | 用途 | 替代当前代码 |
|------|------|-------------|
| `SliderRoot` + `SliderTrack` + `SliderIndicator` + `SliderThumb` | 播放进度条 | 当前自定义 `<view>` 拖拽实现 |
| `Button` | 播放/暂停/全屏按钮 | 当前 `<view bindtap>` 实现 |
| `Switch` | 设置开关 | 当前自定义 toggle |

#### Step 3：安装 lynx-ui

```bash
cd lynx-player && pnpm add @lynx-js/lynx-ui
```

需要同步更新 `lynx.config.ts`：
```typescript
plugins: [
  pluginReactLynx({
    enableNewGesture: true,  // lynx-ui 必需
  }),
]
```

#### Step 4：用 lynx-ui Slider 替换自定义进度条

当前 `PlayerControls.tsx` 中的进度条是手写的 `<view>` + touch 事件处理。替换为 lynx-ui Slider：

```tsx
import { SliderRoot, SliderTrack, SliderIndicator, SliderThumb } from '@lynx-js/lynx-ui'

<SliderRoot
  defaultValue={0}
  value={progress}
  onValueChange={(val) => seekTo(val)}
  onValueCommit={(val) => commitSeek(val)}
>
  <SliderTrack className='progress-track'>
    <SliderIndicator className='progress-indicator' />
    <SliderThumb className='progress-thumb' />
  </SliderTrack>
</SliderRoot>
```

优势：
- **Main Thread Script**：Slider 的拖拽逻辑运行在主线程，零延迟响应
- **跨平台一致性**：自动处理 Android/iOS 触摸差异
- **无障碍支持**：内置 accessibility 语义
- **代码精简**：移除自定义 touch 事件处理代码

#### Step 5：用 lynx-ui Button 替换控制按钮

```tsx
import { Button } from '@lynx-js/lynx-ui'

<Button onClick={togglePlay} className='control-btn'>
  {({ active }) => (
    <view className={clsx('btn-inner', { active })}>
      <text>{isPlaying ? '⏸' : '▶'}</text>
    </view>
  )}
</Button>
```

优势：
- 内置 `ui-active` 状态 CSS 类，按下反馈零延迟
- 内置 `ui-disabled` 状态支持
- Render props 模式，灵活自定义渲染

---

## 修改文件

1. `/workspace/app/encv-mobile/android/app/src/main/java/com/encvgo/app/MpvPlayerModule.kt` — init 中自动 attachToLayout
2. `/workspace/app/encv-mobile/android/app/src/main/java/com/encvgo/app/PlayerActivityLynx.kt` — 移除同步 attach，改为回调兜底
3. `/workspace/app/encv-mobile/lynx-player/package.json` — 添加 `@lynx-js/lynx-ui` 依赖
4. `/workspace/app/encv-mobile/lynx-player/lynx.config.ts` — 启用 `enableNewGesture`
5. `/workspace/app/encv-mobile/lynx-player/src/components/PlayerControls.tsx` — 用 Slider + Button 替换自定义控件
6. `/workspace/app/encv-mobile/lynx-player/src/components/AppComponent.tsx` — 适配新组件接口
7. `/workspace/app/encv-mobile/lynx-player/src/App.css` — 更新样式

## 风险评估

- **lynx-ui 兼容性**：需要 Lynx Engine >= 3.2，项目当前使用的 Lynx SDK 版本需确认
- **Slider 组件**：是 2026 年 4 月新增的组件（PR #109），较新但已有完整文档和示例
- **bundle 体积**：lynx-ui 支持 tree-shaking，只引入 Slider + Button 不会显著增加体积
