# 修复计划：Lynx 播放器 UI 不显示 + 播放不生效

## 根因分析

### 问题1：UI 只能看到几个 emoji 图标

**核心原因：Lynx 默认布局是 `display: linear`，所有组件 CSS 都没有显式声明 `display: flex`。**

根据 Lynx 官方文档（https://lynxjs.org/api/css/properties/display.html）：

> Linear layout is the default Layout model developed by Lynx.
> You can set `defaultDisplayLinear: false` in the project configuration to change the default layout to flexible box layout.

在 `display: linear` 布局下：
- `flex-direction`、`flex: 1`、`flex-wrap`、`gap` 等属性**全部无效**
- 子元素默认按 `linear-direction: column` 垂直排列
- 没有 `flex-grow`/`flex-shrink` 行为，元素不会自动填充剩余空间
- 结果：所有 `<view>` 容器塌缩，`flex: 1` 不生效，布局完全错乱
- 只有 `<text>` 元素（emoji）可见，因为 text 始终按内容大小渲染

**两种修复方案：**

**方案A（推荐）：全局设置 `defaultDisplayLinear: false`**

在 `lynx.config.ts` 的 `pluginVueLynx` 选项中设置 `defaultDisplayLinear: false`，将默认布局改为 `flex`。这样所有现有 CSS 中的 `flex-direction`、`flex: 1` 等属性都能正常工作，无需逐个添加 `display: flex`。

```typescript
// lynx.config.ts
export default defineConfig({
  plugins: [
    pluginVueLynx({
      defaultDisplayLinear: false,
    }),
  ],
  // ...
})
```

**方案B：逐个添加 `display: flex`**

在每个需要 flex 布局的组件样式中显式添加 `display: flex`。工作量大，容易遗漏。

**选择方案A**，因为：
1. 项目代码全部按 Web flex 布局编写，全局切换最合适
2. 一行配置修改，无需改动任何组件 CSS
3. `<scroll-view>` 强制使用 linear 布局不受影响（文档明确说明）

### 问题2：播放没有生效

**播放逻辑本身正确**，但有两个问题：

1. **路由初始导航**：`createMemoryHistory()` 的初始位置是 `/`（HomeView），如果从 Android 端打开播放器（传入 `filePath`），需要手动 `router.push('/player')`。当前 `main.ts` 中已有此逻辑，但 `getInitialRoute()` 在 `app.mount()` 之前执行，`__globalProps` 可能尚未就绪。

2. **initData 获取方式**：VueLynx 官方文档没有提到 `__globalProps`，ReactLynx 使用 `useInitData()`。VueLynx 可能使用类似机制。需要确认 VueLynx 如何获取 initData。

**修复方案：**

将初始路由判断从 `main.ts` 移到 `App.vue` 的 `onMounted` 中，确保 Lynx 运行时已完全初始化：

```typescript
// main.ts
const app = createApp(App)
app.use(router)
app.mount()
```

```typescript
// App.vue
onMounted(() => {
  try {
    const lynxObj = (globalThis as any).lynx
    const globalProps = lynxObj?.__globalProps
    if (globalProps?.filePath && router.currentRoute.value.path !== '/player') {
      router.push({ name: 'player' })
    }
  } catch (_e) {}
})
```

## 实施步骤

1. **修改 `lynx.config.ts`**：添加 `defaultDisplayLinear: false`
2. **修改 `main.ts`**：移除 `getInitialRoute()` 和 `router.push()`
3. **修改 `App.vue`**：在 `onMounted` 中判断 initData 并导航到 `/player`
4. **构建验证**

## 风险评估

- `defaultDisplayLinear: false` 只影响未显式声明 `display` 的元素，已声明 `display: flex/linear/grid` 的元素不受影响
- `<scroll-view>` 强制使用 linear 布局，不受此配置影响
- 这是 Lynx 官方支持的配置选项，不是 hack
