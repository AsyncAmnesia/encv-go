# WebSocket 替代轮询 & 本地预览方案

## 问题分析

### 问题 1：轮询替代方案

当前 `useServerStatus.ts` 使用 `setInterval` 每 10 秒轮询 `/health` 端点，存在以下问题：
- **资源浪费**：无论服务器状态是否变化，都持续发请求
- **延迟高**：最长 10 秒才能感知状态变化
- **对 Tasks 页面尤其不友好**：任务进度需要实时更新，轮询体验差

**方案对比：**

| 方案 | 实时性 | 复杂度 | 兼容性 | 适合场景 |
|------|--------|--------|--------|---------|
| 轮询（当前） | 低（10s延迟） | 最低 | 最好 | 仅状态检测 |
| WebSocket | 高（即时） | 中 | 好（需服务端支持） | 双向实时通信 |
| SSE（Server-Sent Events） | 高（服务端→客户端） | 低 | 好 | 服务端推送场景 |
| 混合方案：SSE + 按需轮询 | 高 | 中 | 最好 | 最佳实践 |

**推荐方案：混合策略**
- **服务器状态**：保留轻量轮询，但改为"指数退避"策略（在线时 30s 一次，离线时逐步退避到 60s），因为服务器上下线是低频事件
- **任务进度**：改用 SSE（`EventSource`），Go 后端推送任务进度变更，前端实时接收
- **WebSocket 作为未来升级路径**：当需要双向通信（如远程控制）时再引入

**为什么 SSE 而非 WebSocket？**
1. ENCV 场景主要是服务端→客户端推送（任务进度、文件变更通知），不需要双向通信
2. SSE 基于 HTTP，无需升级协议，Go 后端实现简单（`text/event-stream`）
3. SSE 自动重连，浏览器原生支持
4. 前端 `EventSource` API 极简，无需额外依赖

### 问题 2：本地预览

当前环境是远程沙箱，可以通过 `vite dev` 启动开发服务器，配合 `OpenPreview` 工具暴露端口给外部访问。

**可行性**：✅ 完全可行
- Vite dev server 监听 `0.0.0.0:5173`
- 通过 `OpenPreview` 激活预览
- 可实时查看所有页面效果（暗黑模式、文件浏览、播放器等）
- 无需 Go 后端也能预览 UI（API 调用会失败但 UI 正常渲染）

## 实现计划

### 第一步：重构 useServerStatus — 指数退避轮询

**修改文件：** `src/composables/useServerStatus.ts`

**实现细节：**
1. 移除固定间隔轮询
2. 实现"指数退避"策略：
   - 在线时：30 秒检查一次
   - 离线时：首次 5s → 10s → 20s → 30s → 最大 60s
   - 在线恢复后重置为 30s
3. 使用 `setTimeout` 递归替代 `setInterval`，每次检查完再安排下一次
4. 保持 `isOnline` ref 和 `checkStatus()` 接口不变

### 第二步：新增 SSE 实时事件系统

**新建文件：** `src/composables/useEventSource.ts`

**实现细节：**
1. 封装 `EventSource` 连接管理：
   - `connect()` — 建立 SSE 连接到 `{baseUrl}/api/events`
   - `disconnect()` — 关闭连接
   - 自动重连（利用 EventSource 内置重连 + 自定义退避）
   - 连接状态 `connectionState` ref：`connecting` | `connected` | `disconnected`
2. 事件分发：
   - 监听 SSE 事件类型：`task:update`、`file:change`、`server:status`
   - 使用 Vue 的 `provide/inject` 或自定义事件总线分发到各组件
3. 类型安全的事件接口

### 第三步：新增事件总线

**新建文件：** `src/composables/useEventBus.ts`

**实现细节：**
1. 轻量级 TypeScript 事件总线（基于 Vue 的 `mitt` 模式或手写）
2. 类型定义：
   ```typescript
   interface EncvEvents {
     'task:update': EncvTask
     'file:change': { path: string; action: string }
     'server:status': { online: boolean }
   }
   ```
3. `on(event, handler)` / `off(event, handler)` / `emit(event, data)`

### 第四步：Tasks 页面接入实时更新

**修改文件：** `src/views/Tasks.vue`

**实现细节：**
1. 监听 `task:update` 事件，实时更新任务列表中对应任务的状态和进度
2. 保留下拉刷新作为兜底
3. 运行中的任务不再需要手动刷新

### 第五步：Files 页面接入文件变更通知

**修改文件：** `src/views/Files.vue`

**实现细节：**
1. 监听 `file:change` 事件，当当前目录下文件变化时自动刷新列表
2. 保留下拉刷新作为兜底

### 第六步：useServerStatus 接入 SSE 状态推送

**修改文件：** `src/composables/useServerStatus.ts`

**实现细节：**
1. 当 SSE 连接成功时，`isOnline` 自动设为 true
2. 监听 `server:status` 事件更新在线状态
3. SSE 连接失败时回退到退避轮询

### 第七步：API 层增加 SSE 端点配置

**修改文件：** `src/api/encv.ts`

**实现细节：**
1. 新增 `getEventSourceUrl()` 函数，返回 SSE 端点 URL
2. SSE URL 使用与 API 相同的 base URL

### 第八步：启动本地预览

**操作步骤：**
1. 修改 `vite.config.ts`，添加 `host: '0.0.0.0'` 使 dev server 监听所有接口
2. 运行 `npm run dev` 启动 Vite 开发服务器
3. 使用 `OpenPreview` 工具激活预览
4. 在浏览器中查看所有页面效果

### 第九步：构建验证

1. 运行 `npm run build` 确保 TypeScript 无错误
2. 确认 dist 产物正常

## 文件变更总览

| 操作 | 文件路径 | 说明 |
|------|---------|------|
| 修改 | `src/composables/useServerStatus.ts` | 指数退避轮询 + SSE 状态 |
| 新建 | `src/composables/useEventSource.ts` | SSE 连接管理 |
| 新建 | `src/composables/useEventBus.ts` | 类型安全事件总线 |
| 修改 | `src/views/Tasks.vue` | 接入 task:update 实时更新 |
| 修改 | `src/views/Files.vue` | 接入 file:change 实时刷新 |
| 修改 | `src/api/encv.ts` | 新增 SSE URL 函数 |
| 修改 | `vite.config.ts` | 添加 host: '0.0.0.0' |
| 修改 | `src/App.vue` | 初始化 SSE 连接 |

## 执行顺序

1. 新建事件总线 `useEventBus.ts`
2. 新建 SSE 管理 `useEventSource.ts`
3. 扩展 API 层 `encv.ts`
4. 重构 `useServerStatus.ts`（退避轮询 + SSE）
5. 修改 `App.vue` 初始化 SSE
6. 修改 `Tasks.vue` 接入实时更新
7. 修改 `Files.vue` 接入文件变更通知
8. 修改 `vite.config.ts` 添加 host
9. 启动 dev server + OpenPreview 预览
10. 构建验证
