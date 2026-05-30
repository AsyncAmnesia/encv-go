# Capacitor / Ionic Vue 架构铁律（来自实战踩坑）

> **核心原则：modalController.create() 是全局 overlay，不依赖任何组件生命周期。**
> **任何跨 tab 的 eventBus 依赖都是定时炸弹。**

---

## 一、Modal 架构（⚠️ 反复踩坑的核心区域）

### 1.1 禁止 inline `<ion-modal :is-open>` 用于跨 tab 操作

**❌ 错误（Ionic Vue 8 已知 bug）**：
```vue
<!-- Tasks.vue — inline modal 绑定在 Tasks 组件 DOM 树内 -->
<ion-modal :is-open="showNewTaskModal">
  <NewTaskModal @close="showNewTaskModal = false" />
</ion-modal>
```

**症状**：
- 在 Files tab 通过 eventBus 发送 `open-new-task` 事件
- Tasks.vue 的 `onMounted` 未执行（tab 未激活）→ 监听器未注册 → **事件丢失**
- 用户必须先手动切到 Tasks tab 再回来，modal 才能工作
- Tab 切换时 overlay 可能闪烁、卡顿、或不渲染

**根因**：inline `<ion-modal :is-open>` 的 overlay 挂载在父组件的 DOM 树内。当父组件所在的 tab 非活跃时，Ionic 的路由 outlet 可能会销毁/隐藏该 DOM 子树，导致：
1. overlay 元素被从 document 中移除或隐藏
2. `is-open` 变更无法触发正确的动画/显示逻辑
3. 即使 `isOpen = true`，用户也看不到任何东西

**✅ 正确（全局 overlay 模式）**：
```typescript
// useNewTaskModal.ts — composable 封装
import { modalController } from '@ionic/vue'

export function useNewTaskModal() {
  async function openNewTask(sourcePath?: string, taskType?: 'encrypt' | 'decrypt') {
    const modal = await modalController.create({
      component: NewTaskModal,
      componentProps: { /* ... */ },
    })
    await modal.present()  // ← 挂载在 <body> 根节点，与 tab 无关
  }
  return { openNewTask }
}
```

### 1.2 `modalController.create()` 的 componentProps 是静态快照

**关键认知**：`componentProps` 在 `create()` 时被**一次性快照**传入子组件。后续对原始对象的修改**不会自动反映**到子组件。

**❌ 错误（扁平 props 快照断裂）**：
```typescript
const modal = await modalController.create({
  component: NewTaskModal,
  componentProps: {
    sourcePath: initialSourcePath,          // ← 值快照！
    candidates: [],                         // ← 空数组快照！
    onUpdateSourcePath: async (v) => {
      // 这里更新的是闭包变量，但 NewTaskModal 收到的 props.candidates 不会变
      await doPredict(v)
      // candidates.value 更新了，但 props 不变！
    },
  },
})
```

**✅ 正确（Reactive State Object 模式）**：
```typescript
const state = reactive<NewTaskState>({
  sourcePath: initialSourcePath || '',
  candidates: [],
  // ... 所有状态字段
})

const modal = await modalController.create({
  component: NewTaskModal,
  componentProps: {
    state,  // ← 传入 reactive 对象引用！子组件通过 computed 读取最新值
    onUpdateSourcePath: async (v) => {
      state.sourcePath = v        // ← 直接修改对象属性
      await doPredict(v)
      syncState()                // ← 同步内部数据到 state 对象
    },
  },
})
```

**子组件读取模式（双源 fallback）**：
```typescript
// NewTaskModal.vue
const props = defineProps<{ state?: NewTaskState; sourcePath?: string }>()

// 优先读 state（modalController 场景），fallback 到扁平 props（测试场景）
const src = computed(() => props.state?.sourcePath ?? props.sourcePath ?? '')
const cands = computed(() => {
  const arr = props.state?.candidates ?? props.candidates
  return Array.isArray(arr) ? arr : []
})
```

### 1.3 Modal 必须秒开——不要在 present() 前阻塞等待异步数据

**❌ 错误（阻塞式打开）**：
```typescript
async function openNewTask(sourcePath?: string) {
  const state = reactive({...})

  if (sourcePath) {
    await doPredict(sourcePath, 'encrypt')  // ← 500ms 防抖 + API 调用！
    syncState()                              // ← 用户要等 ~1s 才看到 modal
  }

  const modal = await modalController.create({ component: NewTaskModal, componentProps: { state } })
  await modal.present()  // ← 才到这里才展示
}
```

**✅ 正确（先打开后填充）**：
```typescript
async function openNewTask(sourcePath?: string) {
  const state = reactive<NewTaskState>({..., candidates: [], predictedPlugin: null})

  // ① 先创建并立即展示 modal（空状态）
  const modal = await modalController.create({ component: NewTaskModal, componentProps: { state } })
  await modal.present()

  // ② 后台预测插件，完成后 reactive state 自动驱动 UI 更新
  if (sourcePath) {
    const norm = normalize(sourcePath)
    if (norm) {
      await doPredict(norm, state.taskType as 'encrypt' | 'decrypt')
      syncState()  // ← 数据到达后 UI 自动刷新
    }
  }
}
```

**用户体验对比**：

| | 阻塞式 | 秒开式 |
|---|--------|--------|
| 点击→看到 modal | ~800-1500ms | <100ms |
| 插件数据到达 | 同时出现 | modal 打开后 600ms 内渐进加载 |
| 用户感知 | "卡了" / "没反应" | "秒开 + 数据加载中" |

### 1.4 modalController.create() 不需要 router.push 切 tab

**❌ 错误（幽灵路由跳转）**：
```typescript
function handleEncryptFile(file: FileItem) {
  eventBus.emit('open-new-task', { sourcePath: path, taskType: 'encrypt' })
  router.push({ path: '/tabs/tasks' })  // ← 完全多余！modal 是全局 overlay
}
```

**后果**：用户在 Files tab 长按加密 → 路由跳到 Tasks tab → 上下文丢失、位置重置、体验割裂。

**✅ 正确**：
```typescript
// Files.vue — 直接调用 composable，不走 eventBus 中转
const { openNewTask } = useNewTaskModal()

function handleEncryptFile(file: FileItem) {
  openNewTask(resolveFileItem(file), 'encrypt')  // ← 全局 overlay，当前 tab 不变
}
```

---

## 二、eventBus 跨组件通信铁律

### 2.1 禁止跨 tab 的 eventBus 依赖

**致命反模式**：
```
Files.vue ──emit('open-new-task')──> 🌀 [Tasks.vue 未挂载 = 无监听器]
                                        ↓
用户首次打开 App → 在 Files tab → Tasks.vue onMounted 未执行
→ eventBus.on('open-new-task', handler) 从未注册 → 事件永久丢失
```

**正确架构**：

| 通信场景 | 推荐方式 | 原因 |
|---------|---------|------|
| 同组件内（parent ↔ child） | props / emits / v-model | Vue 原生响应式 |
| 跨 tab 的操作触发 | **直接 import composable 调用** | 不依赖目标组件生命周期 |
| 任务创建后通知列表刷新 | eventBus（自消费） | Tasks.vue 自己监听自己需要的事件 |
| WebSocket 消息分发 | eventBus（自消费） | DevLogs.vue 自己监听 |

### 2.2 eventBus 安全使用清单

每次使用 eventBus 前必须确认：

- [ ] **发射者和监听者在同一组件？** ✅ 安全
- [ ] **监听者的 onMounted 一定会在 emit 之前执行？** ⚠️ 危险——tab 切换顺序不可控
- [ ] **onUnmounted 会正确注销？** ✅ 必须配对
- [ ] **事件名是否全局唯一？** ✅ 使用命名空间 `domain:action`

**本项目安全事件清单**：

| 事件 | 发射者 | 监听者 | 跨 tab? | 安全 |
|------|--------|--------|---------|------|
| ~~`open-new-task`~~ | ~~Files.vue~~ | ~~Tasks.vue~~ | ~~是~~ | ~~**已消除**~~ |
| `task:update` | useNewTaskModal.onSubmit → WS | Tasks.vue | 否（同组件自消费） | ✅ |
| `task:progress` | TaskManager.worker → WS | Tasks.vue | 否 | ✅ |
| `task:created` | useNewTaskModal.onSubmit → WS | Tasks.vue | 否 | ✅ |
| `task:completed` | TaskManager.process* → WS | Tasks.vue | 否 | ✅ |
| `task:refresh` | useNewTaskModal.onSubmit | Tasks.vue | 否 | ✅ |
| `file:change` | TaskManager.process* → WS | Files.vue | 否（自消费） | ✅ |
| `ws:message` | WebSocket client | DevLogs.vue | 否（自消费） | ✅ |
| `server:status` | WebSocket client | DevLogs.vue | 否（自消费） | ✅ |

---

## 三、Tab 切换稳定性

### 3.1 Ionic Tabs 路由机制

```
/tabs/
├── home       → HomePage.vue      (lazy loaded)
├── files      → Files.vue         (lazy loaded)
├── tasks      → Tasks.vue         (lazy loaded)
├── remote     → Remote.vue        (lazy loaded)
├── settings   → Settings.vue      (lazy loaded)
└── devlogs    → DevLogs.vue       (lazy loaded)
```

**关键行为**：
- `<ion-router-outlet>` 使用 `keep-alive` 缓存已访问过的 tab 组件
- 但 `onMounted` / `onUnmounted` 只在首次进入和销毁时触发一次
- **切换 tab 不会重新触发 mounted/unmounted**
- `onIonViewWillEnter` / `onIonViewDidLeave` 是 Ionic 生命周期钩子，每次切 tab 都触发

### 3.2 确保稳定的最佳实践

1. **数据获取放在 `onIonViewWillEnter`**（而非 `onMounted`），确保每次切回都刷新
2. **禁止在 `onMounted` 中注册跨 tab 事件的监听器**（见 §2.1）
3. **composable 调用是安全的**——它在模块级别初始化，不依赖组件生命周期
4. **modalController.create() 是最稳定的跨 tab 操作方式**——overlay 挂载在 `<body>` 上

### 3.3 Tab 切换不影响的功能

| 功能 | 是否受 tab 影响 | 原因 |
|------|----------------|------|
| `modalController.create()` | ❌ 不影响 | overlay 在 `<body>` 根节点 |
| `alertController.create()` | ❌ 不影响 | 同上 |
| WebSocket 连接 | ❌ 不影响 | 在 App 层级管理 |
| eventBus 自消费事件 | ❌ 不影响 | 同组件内收发 |
| **composable 直接调用** | ❌ 不影响 | 模块级别初始化 |
| inline ion-modal | ⚠️ **受影响** | DOM 树绑定在 tab 组件内 |
| 跨 tab eventBus | ⚠️ **受影响** | 目标组件可能未挂载 |

---

## 四、防抖/异步时序陷阱

### 4.1 doPredict 防抖 + await 的组合爆炸

**useTaskForm.ts 的 doPredict 内部有 500ms setTimeout 防抖**。如果调用方用 `await doPredict()`：

```
t=0ms:   doPredict() → 设置 500ms timer
t=0ms:   await ??? → 需要 timer resolve + API 返回
t=500ms: timer 触发 → 开始 API 调用
t=500ms+X: API 返回 → resolve()
```

**总延迟 = 500ms(防抖) + API耗时(~100-300ms) = 600-800ms**

如果调用方还加了 `await setTimeout(600)` 固定等待（旧代码）：
- API 快时：浪费 100ms
- API 慢时：**syncState 在 API 返回前执行 → 拿到空数据**

### 4.2 正确做法

**调用方**：`await doPredict()` 即可（它返回 Promise，内部自行处理防抖+API）

**展示方**：先 present modal，再 await doPredict（见 §1.3）

---

## 五、调试检查清单

当 modal 不显示 / 数据不更新 / tab 切换异常时，按此顺序排查：

1. **modal 是否用了 `modalController.create()`？** 如果是 inline `:is-open`，改用 create
2. **是否在 present() 前有长时间 await？** 改为 present 后再异步加载数据
3. **componentProps 是否用了 reactive state object？** 如果是扁平 props，检查数据流
4. **是否有不必要的 router.push？** modalController.create 不需要切路由
5. **eventBus 是否跨 tab？** 改为直接 import composable 调用
6. **doPredict 是否被正确 await？** 检查 Promise 链完整性
7. **syncState() 是否在 API 返回后调用？** 检查时序
