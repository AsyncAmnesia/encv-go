# Lynx React → Vue Lynx 迁移分析

## 背景

当前 Lynx 播放器使用 `@lynx-js/react`（React），主应用使用 Vue + Ionic。用户提出：既然 Lynx 没有路由库，是否应该迁移到 `vue-lynx` 以获得 Vue Router 支持，同时与主应用技术栈统一。

## vue-lynx 路由方案分析

### 核心机制

vue-lynx 官方支持 Vue Router，使用 `createMemoryHistory()` 代替 `createWebHistory()`（因为 Lynx 无 `window.location` / History API）。

```ts
// src/router.ts
import { createRouter, createMemoryHistory } from 'vue-router'
import Home from './views/Home.vue'
import Player from './views/Player.vue'

const router = createRouter({
  history: createMemoryHistory(),
  routes: [
    { path: '/', name: 'home', component: Home },
    { path: '/player', name: 'player', component: Player },
    { path: '/playlist', name: 'playlist', component: Playlist },
    { path: '/settings', name: 'settings', component: Settings },
  ],
})
```

```ts
// src/index.ts
import { createApp } from 'vue-lynx'
import router from './router'
import App from './App.vue'

const app = createApp(App)
app.use(router)
app.mount()
```

```vue
<!-- src/App.vue -->
<script setup lang="ts">
import { RouterView } from 'vue-router'
</script>
<template>
  <view>
    <RouterView />
  </view>
</template>
```

### 导航链接

Lynx 没有 `<a>` 标签，所以 `RouterLink` 的默认渲染不适用。需使用 `custom` + `v-slot` API：

```vue
<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>
<template>
  <RouterLink to="/player" custom v-slot="{ isActive, navigate }">
    <view :class="{ active: isActive }" @tap="navigate">
      <text>播放器</text>
    </view>
  </RouterLink>
</template>
```

### 生态支持

vue-lynx 官方文档列出完整生态：
- ✅ **Vue Router** — `createMemoryHistory()` 完整支持
- ✅ **Pinia** — 状态管理
- ✅ **Vue Query** — 数据获取
- ✅ **Tailwind CSS** — 样式
- ✅ **TypeScript** — 类型支持
- ✅ **Testing Library** — 测试

## 对比分析

### 当前方案（Lynx React + 手动状态路由）

| 维度 | 评价 |
|------|------|
| **路由** | 手动 `useState<AppView>` 切换，无路由守卫/动画/历史 |
| **状态管理** | React `useState/useCallback`，无全局 store |
| **代码量** | AppComponent.tsx 170 行路由逻辑 + 5 个 TSX 页面 |
| **与主应用一致性** | ❌ 主应用 Vue，播放器 React |
| **UI 组件** | `@lynx-js/lynx-ui`（React 版） |
| **调试** | 需要单独的 React 开发工具 |
| **维护成本** | 两套框架知识 |

### 迁移方案（vue-lynx + Vue Router）

| 维度 | 评价 |
|------|------|
| **路由** | ✅ Vue Router `createMemoryHistory()`，完整路由守卫/动画/历史 |
| **状态管理** | ✅ Pinia，与主应用共享模式 |
| **代码量** | SFC 更简洁，`<style scoped>` 替代 App.css |
| **与主应用一致性** | ✅ 统一 Vue 生态 |
| **UI 组件** | 需要确认 vue-lynx 是否有对应 UI 库 |
| **调试** | Vue DevTools |
| **维护成本** | 单一框架 |

### 关键风险

| 风险 | 严重度 | 说明 |
|------|--------|------|
| **vue-lynx Pre-Alpha** | 🔴 高 | npm 周下载量仅 75，版本 0.3.1，官方标注 "Expect bugs and enjoy!" |
| **@lynx-js/lynx-ui 不兼容** | 🔴 高 | 当前使用的 SliderRoot/SliderTrack/Button 等是 React 组件，vue-lynx 无对应 UI 库 |
| **rspeedy 插件不兼容** | 🟡 中 | 需从 `@lynx-js/react-rsbuild-plugin` 换为 `vue-lynx/plugin` |
| **NativeModules 调用** | 🟢 低 | `NativeModules.MpvPlayerModule` 等是 Lynx 原生桥接，与框架无关 |
| **双线程架构差异** | 🟡 中 | vue-lynx 有 `runOnMainThread`/`useMainThreadRef` 等 API，需学习 |
| **CI 构建变更** | 🟡 中 | rspeedy 配置、依赖安装流程需调整 |
| **迁移工作量** | 🟡 中 | 6 个 TSX → 6 个 SFC，约 800 行代码重写 |

## 决策分析

### 不迁移的理由（推荐）

1. **vue-lynx 太不成熟**：Pre-Alpha，周下载 75，可能存在未发现的 bug，不适合生产环境
2. **lynx-ui 不可用**：当前播放器重度使用 `@lynx-js/lynx-ui`（SliderRoot/Button 等），vue-lynx 没有对应 UI 库，需要手动实现所有 UI 原语
3. **当前手动路由已够用**：播放器只有 4 个页面（首页/播放/列表/设置），状态切换足够，不需要路由守卫或复杂导航
4. **风险收益不成比例**：迁移工作量大、风险高，收益仅为"技术栈统一"和"Vue Router"

### 迁移的理由

1. **技术栈统一**：主应用和播放器都用 Vue，降低维护成本
2. **Vue Router**：`createMemoryHistory()` 提供完整路由能力（历史、守卫、动画）
3. **Pinia 共享**：如果后续需要主应用和播放器共享状态
4. **SFC 开发体验**：`<style scoped>` 比 App.css 更模块化

## 结论

**推荐不迁移**，理由：

1. vue-lynx 处于 Pre-Alpha 阶段，不适合生产项目
2. `@lynx-js/lynx-ui` 是 React 专属，vue-lynx 无替代品
3. 当前 4 页面的手动路由完全够用
4. 可以在 vue-lynx 成熟后（达到 Beta/Stable）再考虑迁移

### 替代方案：优化当前 React 手动路由

在当前 React 架构下改进路由体验：
- 封装 `useRouter` hook，提供 `push/pop/replace/back` 等方法
- 添加页面切换动画（Lynx `animation` API）
- 维护导航历史栈（支持返回）
