# Files.vue 三项修复计划

## Issue 1: 插件模式缺少加载状态

### 根因

模板 L93 的条件 `&& !selectedPlugin` 导致插件模式下**整个 loading 模板块被隐藏**：

```
L93:  <template v-if="(loading||isSearching|...) && !selectedPlugin">   ← 插件模式 = false，整块隐藏
...
L123: <template v-else>                                                ← 插件模式走这里
L124:   <div v-if="selectedPlugin">                                     ← 直接渲染列表（可能为空）
         ...无 spinner...
       </div>
```

`openPluginView()` 设置 `loading=true` + `selectedPlugin=plugin` → watch 触发异步 `searchPluginFiles()` → 期间**白屏或显示"无匹配文件"**。

### 修复方案

在插件模式的 `<div v-if="selectedPlugin">` 内部、`<ion-segment>` 之前添加加载状态判断。当 `pluginFiles.length === 0 && loading` 时显示 spinner。

**修改位置**: [Files.vue L124-134](file:///workspace/app/encv-mobile/src/views/Files.vue#L124-L134)

```html
<div v-if="selectedPlugin">
  <ion-toolbar>...</ion-toolbar>
  <!-- 新增：插件模式加载状态 -->
  <div v-if="pluginFiles.length === 0" class="loading-container">
    <ion-spinner name="crescent"></ion-spinner>
    <p>{{ t('files.loading') }}</p>
  </div>

  <template v-else>
    <ion-segment v-model="pluginTab" value="source">...</ion-segment>
    <ion-list :inset="true">
      ...existing list items...
    </ion-list>
  </template>
</div>
```

同理，标签筛选模式 (`handleTagFilter`) 也需要同样的处理——但标签筛选复用 `files.value` + `sortedFiles` 走普通列表路径，而普通列表的 `<template v-else>` 中没有 loading 判断。需要在普通 `<ion-list>` 前也加一个 loading 守卫。

---

## Issue 2: 图片懒加载性能优化

### 当前实现的问题

| # | 问题 | 影响 |
|---|------|------|
| 1 | **无并发限制** | IntersectionObserver 同时触发所有可见图片请求 → N 个并行 HTTP |
| 2 | **Observer 反复创建销毁** | 每次 `setupLazyThumbnails()` 都 `new IntersectionObserver()`，旧实例被 GC 但可能有残留回调 |
| 3 | **无跨导航缓存** | `thumbnailUrls` 是组件内 ref，切换目录/模式后清空 |
| 4 | **无请求去重** | 同一文件被重复 observe 时可能触发重复 URL 设置 |
| 5 | **DOM 全量渲染** | 1000+ 文件时全部创建 DOM 节点（含 img/icon） |

### 优化方案

#### 2a. 单例 Observer + 动态目标管理

将 IntersectionObserver 从函数内部变量提升为模块级单例（或 `onMounted` 创建一次），通过 `observe/unobserve` 动态管理目标。

```typescript
let thumbnailObserver: IntersectionObserver | null = null
const THROTTLE_QUEUE: string[] = []
let THROTTLE_TIMER: ReturnType<typeof setTimeout> | null = null
const MAX_CONCURRENT = 3
let activeRequests = 0

function ensureThumbnailObserver() {
  if (thumbnailObserver) return
  thumbnailObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        const path = entry.target.getAttribute('data-file-path')
        if (path) scheduleThumbnailLoad(path)
        thumbnailObserver!.unobserve(entry.target)
      }
    })
  }, { rootMargin: '150px' })
}

function scheduleThumbnailLoad(path: string) {
  if (thumbnailUrls.value[path]) return
  THROTTLE_QUEUE.push(path)
  processQueue()
}

function processQueue() {
  if (THROTTLE_TIMER) return
  THROTTLE_TIMER = setTimeout(() => {
    const batch = THROTTLE_QUEUE.splice(0, MAX_CONCURRENT)
    for (const path of batch) {
      if (!thumbnailUrls.value[path]) {
        thumbnailUrls.value[path] = getExternalStreamUrl(path)
      }
    }
    THROTTLE_TIMER = null
    if (THROTTLE_QUEUE.length > 0) processQueue()
  }, 50)
}
```

#### 2b. 缓存层

引入模块级缓存 Map，跨导航保留已加载的缩略图 URL：

```typescript
const thumbCache = new Map<string, string>()
const CACHE_MAX = 500

function getCachedThumbUrl(path: string): string | undefined {
  return thumbCache.get(path)
}

function setCachedThumbUrl(path: string, url: string) {
  if (thumbCache.size >= CACHE_MAX) {
    const firstKey = thumbCache.keys().next().value
    if (firstKey) thumbCache.delete(firstKey)
  }
  thumbCache.set(path, url)
}
```

在 `setupLazyThumbnails()` 中优先从缓存读取：
```typescript
if (path && !thumbnailUrls.value[path]) {
  const cached = getCachedThumbUrl(path)
  thumbnailUrls.value[path] = cached || getExternalStreamUrl(path)
  if (!cached) setCachedThumbUrl(path, thumbnailUrls.value[path])
}
```

#### 2c. onThumbError 清理缓存

```typescript
function onThumbError(path: string) {
  delete thumbnailUrls.value[path]
  thumbCache.delete(path)
}
```

#### 2d. 组件卸载时清理 observer

```typescript
onUnmounted(() => {
  if (thumbnailObserver) {
    thumbnailObserver.disconnect()
    thumbnailObserver = null
  }
})
```

### 性能预期

| 场景 | 优化前 | 优化后 |
|------|--------|--------|
| 20 张图片同时可见 | 20 并发 HTTP | 3 并发 + 队列 |
| 1000 文件目录滚动 | 每次重新创建 Observer | 单例复用 |
| 导航回退再进入 | 重新请求全部图片 | 命中缓存 |

---

## Issue 3: 单元测试与 Mock 测试

### 3a. 测试基础设施搭建

当前项目**没有任何测试框架**。需要：

1. 安装依赖：
   ```bash
   cd app/encv-mobile && npm install -D vitest @vue/test-utils jsdom @vitest/coverage-v8
   ```

2. 创建 `vitest.config.ts`:
   ```typescript
   import { defineConfig } from 'vitest/config'
   import vue from '@vitejs/plugin-vue'
   import path from 'path'

   export default defineConfig({
     plugins: [vue()],
     test: {
       environment: 'jsdom',
       globals: true,
       coverage: {
         provider: 'v8',
         reporter: ['text', 'json', 'html'],
       },
     },
     resolve: {
       alias: { '@': path.resolve(__dirname, './src') },
     },
   })
   ```

3. 在 `package.json` scripts 中添加：
   ```json
   "test": "vitest",
   "test:run": "vitest run",
   "test:coverage": "vitest run --coverage"
   ```

### 3b. 测试用例清单

#### B1. 纯逻辑测试（`__tests__/files.logic.test.ts`）

不依赖 Vue 组件，直接测试可导出的纯函数/计算逻辑：

| 测试项 | 覆盖点 |
|--------|--------|
| `isImageFile()` | jpg/png/gif/webp/bmp/svg/heic/heif/avif 返回 true；mp4/pdf/txt 返回 false；目录返回 false |
| `getFileIcon()` | 各 category 映射正确 |
| `getFileIconColor()` | 各 category 颜色映射正确 |
| 排序逻辑 | name↑/name↓/size↑/size↓/time↑/time↓ 六种模式 + 目录置顶 |
| `sortLabel` computed | 显示正确的中文标签 |
| `cycleSort()` | 6 态循环顺序正确 |
| 搜索过滤（插件模式） | `filteredPluginFiles` 对 searchQuery 的本地过滤 |
| imageExts Set | 包含所有预期扩展名 |

**关键：需要将这些函数和逻辑抽取到可单独 import 的 composable 或 util 文件中**，否则无法脱离 SFC 测试。

建议新建 `src/composables/useFileList.ts`，将以下逻辑迁移：
- `isImageFile()`
- `getFileIcon()`, `getFileIconColor()`
- 排序相关：`sortBy`, `sortDesc`, `sortLabel`, `SORT_CYCLE`, `cycleSort()`
- `imageExts`

#### B2. Composable 测试（`__tests__/composables.test.ts`）

测试 `useFileList()` composable 的响应式行为：

| 测试项 | 覆盖点 |
|--------|--------|
| sortedFiles 计算属性 | 不同 sortBy/sortDesc 组合的排序结果 |
| filteredPluginFiles | tab 切换过滤 + 搜索过滤 + 排序组合 |
| displayFiles | searchResults 优先级 |

#### B3. API Mock 测试（`__tests__/api.mock.test.ts`）

使用 vitest `vi.mock()` 模拟 API 层：

| 测试项 | Mock 方式 |
|--------|----------|
| `listFiles()` 成功 | mock fetch → 返回 FileItem[] |
| `listFiles()` 权限拒绝 | mock fetch → 403 → 抛 PermissionDeniedError |
| `listFiles()` 未找到 | mock fetch → 404 → 抛 NotFoundError |
| `searchFiles()` 带缓存 | 验证第二次调用使用缓存 |
| `fetchPlugins()` / `fetchTags()` | mock 返回值 |
| `getExternalStreamUrl()` URL 格式 | DEV vs PROD 模式 |

#### B4. 组件快照测试（可选，`__tests__/Files.snapshot.test.ts`）

使用 `@vue/test-utils` mount Files.vue，验证关键 DOM 结构：
- 加载状态渲染
- 空状态渲染
- 文件列表渲染（含缩略图槽位）
- 抽屉菜单结构

### 3c. 文件结构

```
app/encv-mobile/
├── __tests__/
│   ├── files.logic.test.ts          # 纯逻辑
│   ├── composables.test.ts          # composable 响应式
│   └── api.mock.test.ts             # API mock
├── src/
│   └── composables/
│       └── useFileList.ts           # 新建：抽取的可测试逻辑
├── vitest.config.ts                 # 新建
└── package.json                     # 修改：添加 test scripts + devDeps
```

---

## 实施步骤

### Step 1: 修复插件模式加载状态
- 在插件 div 内添加 `v-if="pluginFiles.length === 0"` 的 loading spinner
- 在普通 ion-list 前添加 loading 守卫（当 `loading && !selectedPlugin` 时——虽然外层 template 已处理，但加防御性检查）
- 验证：vue-tsc + vite build

### Step 2: 懒加载性能优化
- 重构 `setupLazyThumbnails()` 为单例 Observer
- 引入并发队列（MAX_CONCURRENT=3）
- 引入模块级 LRU 缓存（CACHE_MAX=500）
- `onUnmounted` 清理 observer
- `onThumbError` 同步清理缓存
- 验证：vue-tsc + vite build

### Step 3: 测试基础设施 + 用例
- 安装 vitest + @vue/test-utils + jsdom
- 创建 vitest.config.ts
- 抽取 `src/composables/useFileList.ts`
- 编写 B1（纯逻辑）~15 个测试用例
- 编写 B2（composable）~5 个测试用例
- 编写 B3（API mock）~6 个测试用例
- 运行 `npm run test:run` 验证全部通过

### Step 4: 最终构建验证
- `vue-tsc --noEmit && vite build`
- `npm run test:run`
- `npm run test:coverage`（覆盖率报告）
