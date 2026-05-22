# 修复 Lynx 播放器黑屏 — 视口尺寸为 0

## 根因确认

### logcat 关键证据

```
[Layout] UpdateViewport :size: 0.0, 0.0; mode: 0, 0   ← 🔴 视口 = 0×0！
```

### 完整成功链路（但无可见内容）

| 时间 | 事件 | 状态 |
|------|------|------|
| 10:58:51.805 | `onPageStart: url=player.lynx.bundle` | ✅ |
| 10:58:51.810 | `loadTemplate: loaded 84909 bytes` | ✅ (84K bundle 正确) |
| 10:58:51.822 | `LynxTemplateRender onFirstScreen` | ✅ |
| 10:58:51.858 | `onLoadSuccess` | ✅ |
| 10:58:51.875 | `onRuntimeReady` | ✅ |
| 10:58:51.876 | `onFirstScreen` | ✅ |

**所有回调全部成功，没有任何 JS 错误、加载失败或异常！**

### 为什么视口是 0×0

当前 `createLynxView()` 中的执行顺序：

```kotlin
rootLayout?.addView(lynxView, lynxParams)     // ← addView
// ❌ 立即调用 renderTemplateUrl，此时 LynxView 还没有完成 measure/layout
lynxView?.renderTemplateUrl("player.lynx.bundle", initData)
```

问题：`addView()` 之后 Android 还没有对 LynxView 进行 measure 和 layout，LynxView 的宽高仍然是 0×0。此时 `renderTemplateUrl` 触发 Lynx 内部布局计算，读取到的视口尺寸就是 0×0，导致所有元素的 flex 布局基于 0×0 计算，结果所有元素尺寸都是 0。

## 修复步骤

### Step 1：延迟 renderTemplateUrl 到 LynxView 有有效尺寸后

将 `renderTemplateUrl` 从 `addView` 之后立即执行改为通过 `ViewTreeObserver.OnGlobalLayoutListener` 或 `post {}` 延迟执行：

```kotlin
rootLayout?.addView(lynxView, lynxParams)
Log.d(TAG, "createLynxView: LynxView added to rootLayout")

val initData = buildInitDataJson()
Log.d(TAG, "createLynxView: initData=$initData")

lynxView?.viewTreeObserver?.addOnGlobalLayoutListener(object : ViewTreeObserver.OnGlobalLayoutListener {
    override fun onGlobalLayout() {
        lynxView?.viewTreeObserver?.removeOnGlobalLayoutListener(this)
        val w = lynxView?.width ?: 0
        val h = lynxView?.height ?: 0
        Log.d(TAG, "createLynxView: LynxView size now ${w}x${h}, rendering template")
        if (w > 0 && h > 0) {
            lynxView?.renderTemplateUrl("player.lynx.bundle", initData)
            Log.d(TAG, "createLynxView: renderTemplateUrl called")
        } else {
            Log.w(TAG, "createLynxView: LynxView still 0x0, using post fallback")
            lynxView?.post {
                Log.d(TAG, "createLynxView: post-render size=${lynxView?.width}x${lynxView?.height}")
                lynxView?.renderTemplateUrl("player.lynx.bundle", initData)
            }
        }
    }
})
```

**核心原理**：等待 Android 完成 LynxView 的 measure/layout 后再渲染模板，确保 Lynx 能读到正确的视口尺寸。

### Step 2：给 `<page>` 根元素添加显式尺寸作为兜底

虽然 Step 1 应该能解决主要问题，但为了防御性编程，保留 `<page style={{ flex: 1 }}>` 并同时添加 width/height：

**文件**：`lynx-player/src/App.tsx`

```tsx
<page style={{ flex: 1, width: "100%", height: "100%" }}>
```

### Step 3：给 PlayerContainer 添加调试背景色验证

临时给 `.PlayerContainer` 添加一个非黑色背景来验证 CSS 是否生效：

**文件**：`lynx-player/src/App.css`

```css
.PlayerContainer {
  flex: 1;
  background-color: #001030;  /* 深蓝色而非纯黑，便于区分 */
  justify-content: flex-end;
  align-items: center;
}
```

如果看到深蓝色背景说明 CSS 生效了，只是子元素有问题。如果仍然全黑说明 `<page>` 层面的问题还没解决。

## 预期效果

- Step 1 修复后，Lynx 的 `[Layout] UpdateViewport` 日志应显示正确的屏幕尺寸（如 1260×2800）
- 所有基于 flex 的元素能正确计算尺寸
- 用户能看到播放器 UI（居中的播放按钮 + 文件名）
- Step 3 的深蓝色背景可辅助验证渲染是否正常（后续改回黑色）

## 不需要修改的部分

- Native Modules 注册 ✅ 无需修改
- TemplateProvider ✅ 工作正常
- Bundle 加载 ✅ 84909 bytes 成功
- Lynx SDK 初始化 ✅ 正常
- LynxViewClient 回调 ✅ 全部成功（保持用于监控）
