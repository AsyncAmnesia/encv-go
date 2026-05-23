# 修复计划：Lynx 播放器 UI 不显示 + 播放不生效

## 根因分析

### 问题1：UI 只能看到几个 emoji 图标

**核心原因：Lynx 的默认布局是 `display: linear`（不是 `display: flex`），且 `flex-direction` 在 Lynx 中不存在于 linear 布局。**

根据 Lynx 官方文档（https://lynxjs.org/zh/guide/ui/layout/）：

1. **Lynx 默认布局是 `display: linear`**，不是 Web 的 `display: block`
2. **`display: flex` 需要显式声明**才能使用弹性布局
3. **Lynx 不支持 `display: block/inline`**
4. 在 `linear` 布局中，方向由 `linear-direction`（不是 `flex-direction`）控制
5. **`flex: 1`** 只在 `display: flex` 下生效，在 `linear` 布局下要用 `linear-weight`

当前代码的问题：
- 所有组件的 CSS 都使用了 `display: flex`、`flex-direction: row/column`、`flex: 1` 等 Web flex 语法
- **虽然 Lynx 支持 `display: flex`**，但代码中大量地方**没有显式声明 `display: flex`**，导致使用默认的 `linear` 布局
- 在 `linear` 布局下，`flex-direction`、`flex: 1`、`flex-wrap`、`gap` 等属性**全部无效**
- 结果：布局完全错乱，所有元素垂直堆叠（linear 默认方向），`flex: 1` 不生效导致元素高度为 0 或塌缩
- 只能看到 emoji 是因为 `<text>` 元素始终可见，但周围的 `<view>` 容器因为没有正确的布局属性而塌缩

**另一个关键问题：`scroll-view` 强制使用 `linear` 布局**
- 官方文档明确说明：`<scroll-view>` 被强制为 linear 布局
- 在 `scroll-view` 内部不能使用 `display: flex`
- 需要使用 `scroll-orientation="vertical"`（已有）来控制滚动方向

### 问题2：播放没有生效

**播放逻辑本身是正确的**（`startPlayback` 调用链：`getBackendStatus` → `startBackend` → `getStreamUrl` → `MpvPlayerModule.play`），但有两个问题：

1. **UI 不显示导致无法触发播放**：PlayerView 的 `onMounted` 中调用 `startPlayback`，但如果路由没有正确导航到 `/player`，`PlayerView` 不会挂载
2. **`getInitialRoute()` 判断逻辑**：当前通过 `globalThis.lynx.__globalProps.filePath` 判断是否跳转到 `/player`，但 `__globalProps` 的数据来源是 `buildInitDataJson()` 传入的 initData，不是 `__globalProps`

**关键 Bug：initData 的传递方式**

在 `PlayerActivityLynx.kt` 中：
```kotlin
lynxView?.renderTemplateUrl("player.lynx.bundle", initData)
```
`initData` 是通过 `renderTemplateUrl` 的第二个参数传入的，这在 Lynx 中作为 **`__globalProps`** 传递。所以 `globalThis.lynx.__globalProps` 应该能获取到数据。

但问题在于：`main.ts` 中 `getInitialRoute()` 在 `app.mount()` **之前**执行，此时 Lynx 运行时可能还没准备好 `__globalProps`。需要验证这个时序问题。

## 修复方案

### 修复1：所有组件 CSS 添加 `display: flex`

在每个需要 flex 布局的组件样式中，显式添加 `display: flex`。这是最直接的修复方式，因为 Lynx 完全支持 `display: flex`，只是默认值不是 flex。

需要修改的文件和关键样式：

#### App.vue
- `.AppRoot`：添加 `display: flex; flex-direction: column;`

#### HomeView.vue
- `.home-page`：添加 `display: flex; flex-direction: column;`
- `.home-header`：已有 `flex-direction: row`，需添加 `display: flex;`
- `.settings-btn`：添加 `display: flex;`
- `.quick-play-banner`：添加 `display: flex;`
- `.quick-play-btn`：添加 `display: flex;`
- `.section-header`：添加 `display: flex;`
- `.empty-section`：添加 `display: flex; flex-direction: column;`
- `.recent-scroll`：添加 `display: flex;`
- `.recent-card`：添加 `display: flex; flex-direction: column;`
- `.recent-card-icon`：添加 `display: flex;`
- `.category-grid`：添加 `display: flex;`
- `.category-card`：添加 `display: flex; flex-direction: column;`

#### PlayerView.vue
- `.PlayerContainer`：添加 `display: flex;`

#### PlayerControls.vue
- 所有使用 `display: flex` 的类：确认已声明（大部分已有）
- `.VideoOverlay`、`.LockedOverlay`、`.AudioOverlay`：确认已有 `display: flex`
- `.TopGradient`、`.BottomGradient`：使用 `position: absolute`，需要父容器是 `position: relative`，确认 `.VideoOverlay` 等有

#### ProgressBar.vue
- `.ProgressRow`：确认已有 `display: flex; flex-direction: row;`
- `.SliderTrackOuter`：确认已有 `display: flex;`
- `.SliderThumbWrapper`：确认已有 `display: flex;`

#### SettingsView.vue
- `.SettingsPage`：添加 `display: flex; flex-direction: column;`
- `.SettingsHeader`：添加 `display: flex;`
- `.CtrlBtn`：添加 `display: flex;`
- `.SettingsScroll`：添加 `display: flex;`
- `.SettingsSection`：添加 `display: flex; flex-direction: column;`
- `.SettingsItem`：添加 `display: flex;`
- `.SettingsItemLeft`：添加 `display: flex; flex-direction: column;`
- `.ToggleSwitch`：添加 `display: flex;`
- `.SpeedSelector`：添加 `display: flex;`
- `.SpeedOption`：添加 `display: flex;`
- `.SettingsFooter`：添加 `display: flex;`

#### PlaylistView.vue
- `.PlaylistPage`：添加 `display: flex; flex-direction: column;`
- `.PlaylistHeader`：添加 `display: flex;`
- `.CtrlBtn`：添加 `display: flex;`
- `.PlaylistScroll`：添加 `display: flex;`
- `.PlaylistItem`：添加 `display: flex;`
- `.PlaylistItemIndex`：添加 `display: flex;`
- `.PlaylistItemInfo`：添加 `display: flex; flex-direction: column;`

### 修复2：`calc()` 在 HomeView.vue 中的使用

HomeView.vue 中 `.category-card` 使用了 `width: calc(33.33% - 12px)`。虽然 Lynx 支持 `calc()`，但更安全的做法是使用百分比宽度 + gap：
```css
.category-grid {
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
  column-gap: 12px;
  row-gap: 12px;
}
.category-card {
  width: calc((100% - 24px) / 3);
  /* ... */
}
```

### 修复3：播放初始化时序

`getInitialRoute()` 在 `app.mount()` 之前执行时，`__globalProps` 可能尚未就绪。需要将初始路由判断移到组件挂载后。

修改 `main.ts`：
```typescript
const app = createApp(App)
app.use(router)
app.mount()
```

修改 `App.vue`：在 `onMounted` 中判断 `__globalProps` 并导航：
```typescript
onMounted(() => {
  try {
    const lynxObj = (globalThis as any).lynx
    const globalProps = lynxObj?.__globalProps
    if (globalProps?.filePath) {
      router.push({ name: 'player' })
    }
  } catch (_e) {}
})
```

### 修复4：ProgressBar 中 `calc()` 的使用

ProgressBar.vue 中 `.SliderThumbWrapper` 使用了 `left: 'calc(' + ... + '% - 8px)'`。这是内联 style，Lynx 支持 `calc()`，保留即可。

### 修复5：`pointer-events: none` 确认

根据 Lynx 官方文档，`pointer-events` 支持 `auto` 和 `none`。PlayerControls.vue 中的 `pointer-events: none` 可以保留。

## 实施步骤

1. 修复 App.vue — 添加 `display: flex`
2. 修复 HomeView.vue — 添加 `display: flex` + 修复 `calc()` 布局
3. 修复 PlayerView.vue — 添加 `display: flex`
4. 修复 PlayerControls.vue — 确认所有 `display: flex` 声明
5. 修复 ProgressBar.vue — 确认 `display: flex` 声明
6. 修复 SettingsView.vue — 添加 `display: flex`
7. 修复 PlaylistView.vue — 添加 `display: flex`
8. 修复播放初始化时序 — main.ts + App.vue
9. 构建验证
