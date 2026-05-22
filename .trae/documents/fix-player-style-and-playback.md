# 修复播放器样式异常 + 播放无响应（修订版）

## 前提：需要完整日志

当前 logcat.txt 只有 **error 级别**，而所有自定义模块日志（MpvPlayerModule、PlayerActivityLynx、GoBackendModule、LogRelay）都走 **info/warning** 级别 → **零条可见日志**。

**需要用户补充**：重新触发播放操作后抓取 **全级别** logcat：
```bash
adb logcat -d > logcat_full.txt
# 或至少过滤自定义 tag：
adb logcat -d -s MpvPlayer PlayerActivityLynx GoBackend LogRelay ENCVGO lynx:V *:S
```

---

## 已确认的问题（基于 error 级别日志）

### 问题 1：`DestroyLayoutNodeBeforeRemoveFromParent` 反复触发（样式异常根因）

```
19:19:36.537  layout_context: DestroyLayoutNodeBeforeRemoveFromParent tag:raw-text/tag:text/tag:view/tag:view
19:19:39.310  layout_context: DestroyLayoutNodeBeforeRemoveFromParent tag:raw-text/tag:text    ← ~3s 后再次
19:19:40.442  layout_context: DestroyLayoutNodeBeforeRemoveFromParent tag:raw-text/tag:text    ← ~1s 后再次
```

**每 1-3 秒销毁重建所有节点**。最可能的原因是 **App.css 中使用了 Lynx 不支持的 CSS 属性**，导致 FlexLayout 引擎反复尝试布局 → 失败 → 销毁重建 → 再尝试。

**不需要隔离 SurfaceView**。mpv-android 官方同样将 SurfaceView 放在 rootLayout 中正常工作。修复 CSS 即可消除抖动。

### 问题 2：播放无响应 — 缺乏前端反馈

点击 ▶ 后无任何视觉变化。可能原因（需完整日志确认）：
- initData 为空（filePath 未传递）
- MPV surface 未就绪但前端不知道
- 某步 NativeModules 调用抛异常被 catch 吞掉

---

## 修复方案（4 步）

### Step 1：重写 App.css — 彻底移除不支持的 CSS 属性

**文件**：`app/encv-mobile/lynx-player/src/App.css`

| 删除的不支持属性 | 行号 | 替代方案 |
|------------------|------|---------|
| `position: absolute` | L129-133 (SliderThumb) | 删除 thumb 元素，进度条改为纯宽度填充条 |
| `overflow: hidden` | L109 (SliderTrackOuter) | 删除，外层容器固定高度即可 |
| `gap: 8px` | L140 (LoadingDots) | 改为 `margin: 0 6px` |
| `border-radius: 36px` | L53 (PlayButtonCircle) | 改为 `border-radius: 16px` |
| `text-overflow: ellipsis` | L31-32 (TitleText) | 改用 Lynx 原生 `text-maxline: 1` |

**进度条新设计**（不依赖 absolute/overflow）：
```
SliderTrackOuter (固定高度, 背景灰)
  └── SliderTrackFill (width 百分比, 背景蓝)
```
不再使用 SliderThumb（absolute 定位的滑块圆点），改用纯色填充条显示进度。

### Step 2：增强错误可见性 + 重试机制

**文件**：`app/encv-mobile/lynx-player/src/components/AppComponent.tsx`
- `startPlayback()` 开头加 initData 空值保护，空值时 setPlayerState("error") + 明确错误信息
- 每个 NativeModules 调用前后加 lynxLog.info 打点

**文件**：`app/encv-mobile/lynx-player/src/components/PlayerControls.tsx`
- error 状态增加 🔄 重试按钮（复用 onPlayPause）
- idle 状态 fileName 为空时显示 "等待文件信息..."

### Step 3：MPV 模块增加状态通知

**文件**：`app/encv-mobile/android-overlay/app/src/main/java/com/encvgo/app/MpvPlayerModule.kt`
- `surfaceCreated()` 成功后 dispatchStateChange("surface_ready")
- `play()` 在 surfaceReady=false 时 dispatchStateChange("waiting_surface")
- 这些事件让前端能显示对应状态（如"正在初始化视频..."）

### Step 4：构建验证

```bash
cd app/encv-mobile/lynx-player && npm run build
# 验证 CSS 无不支持的属性
grep -nE 'position:|overflow:|gap:|text-overflow' src/App.css || echo "✅无不支持的CSS"
node --check ../scripts/post-cap-sync.mjs
```

---

## 不做的事

- ❌ 不隔离 SurfaceView 到独立 FrameLayout（mpv-android 官方就是这么放的，CSS 修好后不会抖动）
- ❌ 不修改 PlayerActivityLynx 的布局结构
- ❌ 不新增任何 Android 原生 View 层
