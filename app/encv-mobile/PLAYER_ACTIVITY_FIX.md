# Lynx 播放器 Activity 修复经验

## 核心问题：Lynx Vue 路由配置

### 问题描述

Lynx 播放器 Activity 启动后不显示任何 UI，首页和播放页均为空白。

### 根因

Lynx 运行在非浏览器环境（无 `window.location`、无 History API），vue-router 必须使用 `createMemoryHistory()`。

两个关键 Bug：

1. **`createMemoryHistory()` 参数误用**：其参数是 `base`（URL 前缀），不是初始路由。错误地将 `/player` 作为 base 传入，导致所有路由 URL 被加上 `/player` 前缀，路由匹配完全失败。
2. **缺少手动初始导航**：Memory 模式不会自动触发初始导航，`RouterView` 始终为空，必须在 `app.use(router)` 之后手动 `router.push()` 到初始路由。

### 正确写法

```typescript
// router.ts
import { createRouter, createMemoryHistory } from 'vue-router'

const router = createRouter({
  history: createMemoryHistory(), // 无参数！base 默认为 '/'
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/player', name: 'player', component: PlayerView },
  ],
})

export default router
```

```typescript
// main.ts
import { createApp } from 'vue-lynx'
import App from './App.vue'
import router from './router'

function getInitialRoute(): string {
  try {
    const lynxObj = (globalThis as any).lynx
    const globalProps = lynxObj?.__globalProps
    if (globalProps?.filePath) {
      return '/player'
    }
  } catch (_e) {
    // ignore
  }
  return '/'
}

const app = createApp(App)
app.use(router)
router.push(getInitialRoute()) // 必须手动 push！
app.mount()
```

### 参考文档

- Lynx Vue Router 官方文档：https://vue.lynxjs.org/guide/routing
- vue-router Memory 模式文档：https://router.vuejs.org/guide/essentials/history-mode.html#memory-mode

> "Memory 模式不会假定自己处于浏览器环境，因此不会与 URL 交互也不会自动触发初始导航。需要你在调用 `app.use(router)` 之后手动 push 到初始导航。"

## Lynx CSS 兼容性

以下 CSS 属性在 Lynx 中均支持，无需替换：

| CSS 属性 | 支持状态 | 官方文档 |
|----------|---------|---------|
| `linear-gradient` | ✅ 支持 | https://lynxjs.org/api/css/properties/background-image.html |
| `transform` | ✅ 支持 | https://lynxjs.org/api/css/properties/transform |
| `pointer-events` | ✅ 支持（`auto`/`none`） | https://lynxjs.org/api/css/properties/pointer-events |
| `calc()` | ✅ 支持 | 在 bottom、inset-inline-start 等属性文档中可见 |
| `position: absolute` | ✅ 支持 | — |
| `flex` 布局 | ✅ 支持 | — |

## 调试方法

### 1. LynxViewClient 错误捕获

`PlayerActivityLynx.kt` 中通过 `LynxViewClient` 回调捕获错误：

- `onReceivedError`：通用错误
- `onReceivedJSError`：JavaScript 运行时错误
- `onReceivedJavaError`：Java 层错误
- `onReceivedNativeError`：Native 层错误

### 2. JS 端日志

通过 `LogBridgeModule` 将 JS 日志转发到 Android LogRelay：

```typescript
globalThis.NativeModules.LogBridgeModule.log('error', message, () => {})
```

### 3. 常见排查步骤

1. 检查 `onLoadFailed` 是否被调用 — 模板加载失败
2. 检查 `onReceivedJSError` — JS 运行时错误
3. 检查 `onRuntimeReady` 和 `onLoadSuccess` 是否正常触发
4. 确认 `renderTemplateUrl` 的 initData 参数正确传递了 `filePath` 等信息

## 经验教训

1. **查阅官方文档优先**：Lynx Vue 官方文档明确说明了 router 的用法，不应凭猜测杜撰不支持的属性
2. **`createMemoryHistory()` 参数是 base 不是路由**：这是 vue-router API 设计，不是 Lynx 特有问题
3. **Memory 模式必须手动 push 初始路由**：这是 vue-router 官方文档明确说明的行为
4. **CSS 兼容性需要查文档确认**：Lynx 的 CSS 支持度很高（97%），不要轻易假设某个 CSS 属性不支持
