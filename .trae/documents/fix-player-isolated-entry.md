# 播放器彻底隔离 — 半独立应用架构

## 架构总览

```
ENCV Mobile App
├── 主应用 (MainActivity)              播放器应用 (PlayerActivity)
│   ├── index.html                     ├── player.html
│   ├── src/main.ts                    ├── src/player-main.ts
│   ├── App.vue                        ├── PlayerApp.vue (根组件)
│   ├── router/index.ts                └── router/player.ts (独立路由)
│   │   ├── /tabs/files               │     ├── /player          → StandalonePlayer
│   │   ├── /tabs/tasks               │     └── /player/preview  → FilePreview (未来扩展)
│   │   ├── /tabs/settings            │
│   │   └── ...                       │
│   ├── views/Tabs.vue (含TabBar)      └── 无 TabBar，全屏播放
│   └── WebSocket + 权限请求             独立后端交互，无 WS
│                                       共享层：
│                                     ├── composables/* (复用)
│                                     ├── plugins/GoProcess (复用)
│                                     ├── api/encv.ts (复用)
│                                     └── theme/variables.css (复用)
```

两个应用共享：composables、plugins、API、工具函数、CSS 主题。
完全隔离：HTML 入口、Vue 实例、Router、根组件、页面结构。

---

## 文件变更清单

### 新建文件（4 个）

#### 1. `player.html` — 播放器 HTML 入口

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no" />
  <title>ENCV Player</title>
</head>
<body>
  <div id="app"></div>
  <script type="module" src="/src/player-main.ts"></script>
</body>
</html>
```

#### 2. `src/player-main.ts` — 播放器 Vue 入口

```typescript
import { createApp } from 'vue'
import { IonicVue } from '@ionic/vue'
import PlayerApp from '@/PlayerApp.vue'
import playerRouter from '@/router/player'

import '@ionic/vue/css/core.css'
import '@ionic/vue/css/normalize.css'
import './theme/variables.css'

const app = createApp(PlayerApp).use(IonicVue).use(playerRouter)

playerRouter.isReady().then(() => {
  app.mount('#app')
})
```

与 `main.ts` 的差异：
- 根组件是 `PlayerApp.vue` 而非 `App.vue`
- Router 是 `router/player.ts` 而非 `router/index.ts`
- 不引入 structure/typography/padding/flex-utils/display CSS（播放器不需要完整 UI 框架）
- 不连接 WebSocket、不请求权限

#### 3. `PlayerApp.vue` — 播放器根组件

```vue
<template>
  <ion-app>
    <ion-router-outlet />
  </ion-app>
</template>

<script setup lang="ts">
import { IonApp, IonRouterOutlet } from '@ionic/vue'
</script>
```

极简根组件。只有 `<ion-app>` + `<ion-router-outlet>`，无 TabBar、无权限、无 WebSocket。

对比主应用的 `App.vue`（有权限请求、WebSocket、主题初始化等）。

#### 4. `src/router/player.ts` — 播放器独立路由

```typescript
import { createRouter, createWebHistory } from '@ionic/vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/player',
  },
  {
    path: '/player',
    component: () => import('@/views/StandalonePlayer.vue'),
  },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
})

export default router
```

当前只有 `/player` 一个路由。后续可轻松扩展：
- `/player/preview` → 文件预览
- `/player/settings` → 播放器设置
- `/player/history` → 播放历史

### 修改文件（4 个）

#### 5. `vite.config.ts` — 多入口构建

```typescript
export default defineConfig({
  build: {
    rollupOptions: {
      input: {
        main: resolve(__dirname, 'index.html'),
        player: resolve(__dirname, 'player.html'),
      },
    },
  },
})
```

Vite 自动处理 code splitting，共享 chunk 只打包一次。

#### 6. `PlayerActivity.kt` — 加载独立入口

```kotlin
override fun load() {
    super.load()
    try {
        bridge?.webView?.loadUrl("https://localhost/player.html")
        Log.i(TAG, "PlayerActivity loading isolated player app")
    } catch (e: Exception) {
        Log.e(TAG, "Failed to load player page", e)
    }
}
```

`onCreate` 中删除所有导航代码，只保留后端交互和 intent 解析。

#### 7. `StandalonePlayer.vue` — 微调

当前已不依赖 `useRouter()`（检查确认），基本无需改动。如果内部有用到路由导航的地方改为使用当前已有的方式即可。

#### 8. `router/index.ts` — 清理

删除 `/standalone/player` 路由（该路由属于播放器应用，不在主应用中）：

```typescript
// 删除这段：
// {
//   path: '/standalone/player',
//   component: () => import('@/views/StandalonePlayer.vue'),
// },
```

---

## 构建产物

```
dist/
├── index.html              ← 主应用入口 (MainActivity)
├── player.html             ← 播放器入口 (PlayerActivity)
├── assets/
│   ├── index-[hash].js     ← 主应用 chunk
│   ├── player-[hash].js    ← 播放器 chunk
│   └── vendor-[hash].js    ← 共享依赖 (vue, ionic 等，只一份)
└── ...
```

---

## 实施步骤

- [ ] Step 1: 创建 `player.html`
- [ ] Step 2: 创建 `src/player-main.ts`
- [ ] Step 3: 创建 `PlayerApp.vue`
- [ ] Step 4: 创建 `src/router/player.ts`
- [ ] Step 5: 修改 `vite.config.ts` 增加多入口
- [ ] Step 6: 修改 `PlayerActivity.kt` 的 `load()` 加载 `player.html`
- [ ] Step 7: 从 `router/index.ts` 删除 `/standalone/player` 路由
- [ ] Step 8: 检查 `StandalonePlayer.vue` 是否需要调整
- [ ] Step 9: 构建验证（vue-tsc + vite build + go build）
- [ ] Step 10: 本地合并模拟验证
