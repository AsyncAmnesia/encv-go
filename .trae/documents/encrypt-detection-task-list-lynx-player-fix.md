# 修复计划：加密覆盖检测、任务列表增强、Lynx 播放器 UI

## 问题1：加密覆盖没有正确识别

### 根因分析

`PredictEncryptOutputName` 调用 `FindEncryptingPlugin` 获取插件后，直接调用 `plugin.GetChunkNamer()`。
但此时插件**未被 Initialize**，导致：
- VideoPlugin 的 `chunkNamer` 为 nil（在 `Initialize` 中才赋值）
- AudioPlugin 的 `GetChunkNamer()` 始终返回 nil（设计如此，音频无分片）

结果：`PredictEncryptOutputName` 返回不完整的名称（缺少分片后缀），导致 `CheckEncryptOutputExists` 检测不到已存在的加密文件。

### 修复方案

**核心思路：让预测逻辑走与实际加密相同的初始化路径，确保命名器可用。**

#### 步骤1：修改 `PredictEncryptOutputName` 增加 `cfg` 参数

文件：`/workspace/internal/v2/plugins/registry.go`

- 函数签名改为 `PredictEncryptOutputName(inputPath string, cfg *config.Config) (string, error)`
- 在预测前调用 `plugin.Initialize(ctx)` 初始化插件（与实际加密流程一致）
- 初始化后 `GetChunkNamer()` 才能返回正确的命名器
- 对于无分片命名器的插件（如 AudioPlugin），追加 `GetContainerExtension()` 作为扩展名

```go
func PredictEncryptOutputName(inputPath string, cfg *config.Config) (string, error) {
    plugin, err := FindEncryptingPlugin(inputPath)
    if err != nil {
        return "", err
    }
    ctx := config.NewContext(context.Background(), cfg)
    if err := plugin.Initialize(ctx); err != nil {
        return "", fmt.Errorf("failed to initialize plugin for prediction: %w", err)
    }
    baseNamer := namer.NewDefaultBaseNamer()
    originalFilename := filepath.Base(inputPath)
    encryptedBaseName := baseNamer.GenerateEncryptedBaseName(originalFilename)
    chunkNamer := plugin.GetChunkNamer()
    if chunkNamer != nil {
        return chunkNamer.GenerateMainChunkName(encryptedBaseName), nil
    }
    ext := plugin.GetContainerExtension()
    if ext != "" {
        return encryptedBaseName + ext, nil
    }
    return encryptedBaseName, nil
}
```

#### 步骤2：修改 `CheckEncryptOutputExists` 传入 `s.cfg`

文件：`/workspace/internal/service/mobile_service.go`

- `MobileService` 已有 `cfg *config.Config` 字段（第68行）
- 将 `plugins.PredictEncryptOutputName(sourceAbs)` 改为 `plugins.PredictEncryptOutputName(sourceAbs, s.cfg)`

#### 步骤3：编译验证

---

## 问题2：任务列表增加创建时间和耗时显示

### 后端修改

#### 步骤4：`MobileTask` 增加 `CompletedAt` 字段

文件：`/workspace/internal/service/task_manager.go`

```go
type MobileTask struct {
    // ... 现有字段 ...
    CreatedAt   time.Time  `json:"createdAt"`
    CompletedAt *time.Time `json:"completedAt,omitempty"` // 新增
    cancelFn    context.CancelFunc
}
```

使用 `*time.Time` 指针类型，nil 表示未完成，非 nil 表示完成时间。

#### 步骤5：在任务完成/失败/取消时设置 `CompletedAt`

在以下位置添加 `now := time.Now(); task.CompletedAt = &now`：

1. `processEncrypt` 中任务完成时（第417-425行区域）
2. `processDecrypt` 中任务完成时（第595-603行区域）
3. `failTask` 中（第619-635行区域）
4. `Cancel` 中取消时（第177-201行区域）
5. `loadTasks` 中恢复中断任务时，对 `failed` 状态的任务设置 `CompletedAt`（可用 `time.Now()`）

### 前端修改

#### 步骤6：`EncvTask` 接口增加 `completedAt`

文件：`/workspace/app/encv-mobile/src/api/encv.ts`

```typescript
export interface EncvTask {
  // ... 现有字段 ...
  createdAt: string
  completedAt?: string  // 新增
}
```

#### 步骤7：`Tasks.vue` 显示创建时间和耗时

文件：`/workspace/app/encv-mobile/src/views/Tasks.vue`

- 在每个任务项中增加一行显示：
  - 创建时间：格式化为 `HH:mm` 显示
  - 耗时：如果有 `completedAt`，计算 `completedAt - createdAt` 的差值并格式化（如 `2m30s`）
  - 运行中任务：显示已运行时间（从 `createdAt` 到现在的差值）
- 在 `onTaskCreated` 和 `onTaskCompleted` 事件处理中传递 `completedAt` 字段

---

## 问题3：Lynx 播放器不显示任何 UI

### 根因分析

**经过查阅 Lynx Vue 官方文档（https://vue.lynxjs.org/guide/routing）确认：**

1. Lynx Vue **完全支持** vue-router，官方文档有专门的 Routing 章节
2. 必须使用 `createMemoryHistory()` 而非 `createWebHistory()`
3. `createMemoryHistory()` 的参数是 `base`（URL 前缀），**不是**初始路由
4. Memory 模式**不会自动触发初始导航**，需要在 `app.use(router)` 之后手动 `router.push()` 到初始路由

**当前代码的两个 Bug：**

1. `createMemoryHistory(getInitialRoute())` — 把初始路由（如 `/player`）当作 `base` 传入，导致所有路由 URL 被错误加上前缀（如 `/player/`、`/player/player`），路由匹配完全失败
2. 缺少 `router.push(getInitialRoute())` — 即使修复了参数问题，Memory 模式不会自动导航到任何路由，`RouterView` 始终为空

**CSS 兼容性确认（查阅 Lynx 官方文档）：**

- `linear-gradient` ✅ 支持（background-image 文档明确列出）
- `transform` ✅ 支持（有独立的 transform 属性文档）
- `pointer-events` ✅ 支持（有独立的 pointer-events 属性文档，支持 `auto` 和 `none`）
- `calc()` ✅ 支持（在 bottom、inset-inline-start 等属性文档的语法示例中可见 `calc(1px + 1px)`）

**结论：播放器无 UI 的唯一原因是路由配置错误，CSS 全部兼容无需修改。**

### 修复方案

#### 步骤8：修复 `router.ts` — `createMemoryHistory` 无参数

文件：`/workspace/app/encv-mobile/lynx-player/src/router.ts`

```typescript
const router = createRouter({
  history: createMemoryHistory(),  // 移除 getInitialRoute() 参数
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/player', name: 'player', component: PlayerView },
    { path: '/playlist', name: 'playlist', component: PlaylistView },
    { path: '/settings', name: 'settings', component: SettingsView },
  ],
})
```

#### 步骤9：修复 `main.ts` — 手动 push 到初始路由

文件：`/workspace/app/encv-mobile/lynx-player/src/main.ts`

```typescript
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
router.push(getInitialRoute())  // 手动触发初始导航
app.mount()
```

注意：`getInitialRoute()` 需要从 `router.ts` 移到 `main.ts`（或提取为共享模块），因为 `router.ts` 不再需要它。

#### 步骤10：`PlayerActivityLynx.kt` 添加错误日志增强

文件：`/workspace/app/encv-mobile/android/app/src/main/java/com/encvgo/app/PlayerActivityLynx.kt`

当前已有较完善的错误日志（`onReceivedError`、`onReceivedJSError`、`onReceivedJavaError`、`onReceivedNativeError`），无需额外修改。

---

## 问题4：文档更新

#### 步骤11：创建 `PLAYER_ACTIVITY_FIX.md`

文件：`/workspace/app/encv-mobile/PLAYER_ACTIVITY_FIX.md`

记录 Lynx 播放器修复经验，核心要点：
- Lynx Vue 支持 vue-router，必须使用 `createMemoryHistory()`
- `createMemoryHistory()` 的参数是 `base`（URL 前缀），不是初始路由
- Memory 模式不会自动触发初始导航，必须手动 `router.push()`
- Lynx CSS 兼容性：`linear-gradient`、`transform`、`pointer-events`、`calc()` 均支持
- 调试方法：通过 `LynxViewClient` 的 `onReceivedError`/`onReceivedJSError` 捕获错误

---

## 实施顺序

1. 问题1：步骤1 → 步骤2 → 步骤3（编译验证）
2. 问题2：步骤4 → 步骤5 → 步骤6 → 步骤7
3. 问题3：步骤8 → 步骤9 → 步骤10
4. 文档：步骤11
