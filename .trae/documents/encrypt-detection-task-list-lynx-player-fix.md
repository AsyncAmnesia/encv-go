# 修复计划：加密覆盖检测、任务列表增强、Lynx播放器UI

## 问题 1：加密覆盖检测不符合事实

### 根因分析

`PredictEncryptOutputName` 调用 `FindEncryptingPlugin` 获取插件后，直接调用 `plugin.GetChunkNamer()` 获取命名器。但此时插件**未被初始化**（`Initialize` 未调用），导致：

1. **VideoPlugin**：`p.chunkNamer` 在 `Initialize` 中才赋值为 `namer.NewPaddedNamer(p.settings.Ext, ...)`，未初始化时为 nil
2. **AudioPlugin**：`GetChunkNamer()` 始终返回 nil（音频不用分片命名器）

当 `chunkNamer == nil` 时，`PredictEncryptOutputName` 返回 `encryptedBaseName`（如 `test.4pm`），但实际加密输出文件名是：
- 视频：`test.4pm.sccgv`（chunkNamer.GenerateMainChunkName 追加容器扩展名）
- 音频：`test.3pm.sccga`（PostEncryptProcessor 中 `encryptedBaseName + p.settings.Ext`）

**预测名 ≠ 实际名**，导致 `CheckEncryptOutputExists` 检查的文件路径与实际加密输出不一致，覆盖检测永远找不到已存在的加密文件。

### 修复方案

**方案：改用目录扫描 + 模式匹配替代精确预测**

不再尝试精确预测输出文件名（这依赖插件初始化状态），改为在目标目录中扫描是否有匹配的加密容器文件：

1. 修改 `CheckEncryptOutputExists`：
   - 用 `GenerateEncryptedBaseName` 生成加密基础名（如 `test.4pm`）
   - 列出目标目录中的文件
   - 检查是否有文件名以该基础名开头且是已知容器文件（`IsContainer` 检测）
   - 返回匹配的文件路径

2. 删除 `PredictEncryptOutputName` 函数（不再需要）

3. 具体实现：
   ```go
   func (s *MobileService) CheckEncryptOutputExists(sourcePath, targetDir string) (bool, string, error) {
       sourceAbs, _ := utils.SafeURLToAbsPath(s.servingDir, sourcePath)
       baseNamer := namer.NewDefaultBaseNamer()
       encryptedBaseName := baseNamer.GenerateEncryptedBaseName(filepath.Base(sourceAbs))

       // 确定目标目录的 URL 路径和绝对路径
       outputDirURL := targetDir
       if outputDirURL == "" {
           outputDirURL = filepath.Dir(sourcePath)
           if outputDirURL == "" { outputDirURL = "/" }
       }
       outputDirAbs, _ := utils.SafeURLToAbsPath(s.servingDir, outputDirURL)

       // 扫描目标目录
       entries, err := os.ReadDir(outputDirAbs)
       if err != nil {
           if os.IsNotExist(err) { return false, "", nil }
           return false, "", err
       }

       for _, entry := range entries {
           name := entry.Name()
           if strings.HasPrefix(name, encryptedBaseName) && !entry.IsDir() {
               entryAbs := filepath.Join(outputDirAbs, name)
               if plugins.IsContainer(entryAbs) {
                   // 构造 URL 路径返回
                   outputPath := strings.TrimRight(outputDirURL, "/") + "/" + name
                   return true, outputPath, nil
               }
           }
       }
       return false, "", nil
   }
   ```

### 涉及文件

| 文件 | 修改内容 |
|------|---------|
| `internal/service/mobile_service.go` | 重写 `CheckEncryptOutputExists`，改用目录扫描 |
| `internal/v2/plugins/registry.go` | 删除 `PredictEncryptOutputName` 函数 |

---

## 问题 2：任务列表增加创建时间和耗时显示

### 现状分析

- 后端 `MobileTask` 已有 `CreatedAt time.Time`，但无耗时字段
- 前端 `EncvTask` 已有 `createdAt: string`，但 `Tasks.vue` 未显示
- 前端 `Tasks.vue` 的 `onTaskCreated` 中手动设置 `createdAt: new Date().toISOString()`

### 修复方案

1. **后端**：在 `MobileTask` 增加 `CompletedAt` 字段
   ```go
   type MobileTask struct {
       // ...existing fields...
       CreatedAt   time.Time  `json:"createdAt"`
       CompletedAt *time.Time `json:"completedAt,omitempty"`
   }
   ```

2. **后端**：在任务完成/失败/取消时设置 `CompletedAt`
   - `processEncrypt` 完成时：`now := time.Now(); task.CompletedAt = &now`
   - `processDecrypt` 完成时：同上
   - `failTask` 时：同上
   - `Cancel` 时：同上

3. **前端**：在 `EncvTask` 接口增加 `completedAt` 字段
   ```typescript
   export interface EncvTask {
     // ...existing fields...
     createdAt: string
     completedAt?: string
   }
   ```

4. **前端**：在 `Tasks.vue` 的任务项中显示创建时间和耗时
   - 创建时间：格式化为 `HH:mm` 或 `MM/DD HH:mm`
   - 耗时：如果有 `completedAt`，计算 `completedAt - createdAt`；如果任务正在运行，计算 `now - createdAt`
   - 显示位置：在任务名称下方或状态标签旁

5. **前端耗时计算工具函数**：
   ```typescript
   function formatDuration(start: string, end?: string): string {
     const startTime = new Date(start).getTime()
     const endTime = end ? new Date(end).getTime() : Date.now()
     const diffSec = Math.floor((endTime - startTime) / 1000)
     if (diffSec < 60) return `${diffSec}s`
     if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m${diffSec % 60}s`
     return `${Math.floor(diffSec / 3600)}h${Math.floor((diffSec % 3600) / 60)}m`
   }
   ```

6. **持久化兼容**：`loadTasks` 中旧任务没有 `CompletedAt`，JSON 反序列化时自动为零值（nil），不影响

### 涉及文件

| 文件 | 修改内容 |
|------|---------|
| `internal/service/task_manager.go` | 增加 `CompletedAt` 字段，完成/失败/取消时赋值 |
| `app/encv-mobile/src/api/encv.ts` | `EncvTask` 接口增加 `completedAt` |
| `app/encv-mobile/src/views/Tasks.vue` | 显示创建时间和耗时 |

---

## 问题 3：Lynx 播放器 UI 不显示

### 根因分析

**核心问题：Lynx CSS 兼容性 + vue-router 兼容性**

Lynx 不是浏览器，它使用原生渲染引擎，只支持 CSS 子集。当前代码使用了大量 Lynx 不支持的 CSS 属性，导致组件渲染失败或尺寸为 0。

#### 不兼容的 CSS 属性清单

| 属性 | 使用位置 | Lynx 支持情况 |
|------|---------|--------------|
| `background-image: linear-gradient(...)` | PlayerControls.vue, HomeView.vue | ❌ 不支持 |
| `calc()` | HomeView.vue, ProgressBar.vue | ❌ 不支持 |
| `transform: translateY(-50%)` | ProgressBar.vue | ❌ 不支持 |
| `pointer-events: none` | PlayerControls.vue | ❌ 不支持 |
| `border-style: solid` (简写) | PlayerControls.vue, ProgressBar.vue | ⚠️ 需拆分为独立属性 |
| `text-overflow: ellipsis` | HomeView.vue | ⚠️ 需用 `lines` 属性替代 |
| `text-transform: uppercase` | SettingsView.vue | ❌ 不支持 |
| `letter-spacing` | SettingsView.vue | ⚠️ 部分支持 |
| `display: flex` | 所有组件 | ✅ 默认即 flex，无需显式声明 |
| `position: absolute` | PlayerControls.vue, ProgressBar.vue | ✅ 支持但需注意约束 |

#### vue-router 兼容性问题

`vue-lynx` v0.3.1 是早期版本，`vue-router` 的 `createMemoryHistory` 可能不被完全支持。`RouterView` 可能无法正确渲染子组件，导致整个页面空白。

#### initData 传递问题

`renderTemplateUrl("player.lynx.bundle", initData)` 的第二个参数映射到 `lynx.__globalProps`，但在 `vue-lynx` 中，这个值的读取时机可能在路由初始化之后，导致 `getInitialRoute()` 无法获取到 `filePath`。

### 修复方案

#### 第一步：移除 vue-router，改用直接组件渲染

参考 React 版本修复经验（PLAYER_ACTIVITY_FIX.md），移除路由依赖，改为条件渲染：

```vue
<!-- App.vue -->
<script setup lang="ts">
import { computed } from 'vue'
import HomeView from './views/HomeView.vue'
import PlayerView from './views/PlayerView.vue'

const initData = computed(() => {
  try {
    const lynxObj = (globalThis as any).lynx
    return lynxObj?.__globalProps || {}
  } catch { return {} }
})

const hasFilePath = computed(() => !!initData.value.filePath)
</script>

<template>
  <view class="AppRoot">
    <PlayerView v-if="hasFilePath" />
    <HomeView v-else />
  </view>
</template>
```

- 删除 `router.ts`
- 修改 `main.ts`：移除 `app.use(router)`
- 修改 `PlayerView.vue`：移除 `useRouter`，返回首页改为 emit 事件或直接调用 NativeModules
- 修改 `HomeView.vue`：移除 `useRouter`，导航改为直接切换组件状态

#### 第二步：修复所有不兼容的 CSS

1. **替换 `linear-gradient`**：
   - 用纯色半透明背景替代渐变
   - `.TopGradient { background-color: rgba(0, 0, 0, 0.4); }` 替代 `linear-gradient(to bottom, rgba(0,0,0,0.6), rgba(0,0,0,0))`

2. **替换 `calc()`**：
   - 用固定百分比或 flex 布局替代
   - `width: calc(33.33% - 12px)` → 用 `flex: 1` + `margin` 实现

3. **替换 `transform: translateY(-50%)`**：
   - 用 `margin-top: -8px` 或 flex `align-items: center` 替代

4. **移除 `pointer-events: none`**：
   - Lynx 中无此属性，直接删除（渐变遮罩层本身不需要交互）

5. **修复 `border` 简写**：
   - `border-style: solid; border-width: 3px; border-color: xxx` → Lynx 需要分别设置 `border-top-width`, `border-right-width` 等，或使用 `border-width` 统一设置

6. **替换 `text-overflow: ellipsis`**：
   - Lynx 的 `<text>` 组件使用 `lines` 属性控制行数，超出自动截断
   - 删除 `text-overflow: ellipsis`，保留 `lines: 2`

7. **移除 `text-transform: uppercase`**：
   - 在 JS 中手动转换为大写

8. **移除 `display: flex`**：
   - Lynx 默认就是 flex 布局，无需显式声明

#### 第三步：简化组件结构

1. **App.vue**：条件渲染 PlayerView 或 HomeView
2. **PlayerView.vue**：移除 router 依赖，直接从 globalProps 读取数据
3. **HomeView.vue**：移除 router 依赖，简化为静态展示
4. **SettingsView.vue / PlaylistView.vue**：改为内联渲染（v-if 切换），不用路由导航

#### 第四步：验证 Lynx bundle 构建

1. 确认 `rspeedy build` 输出的 `player.lynx.bundle` 文件在 Android assets 目录中
2. 确认 `PlayerTemplateProvider` 能正确加载 bundle
3. 添加 Lynx 错误日志捕获（`onReceivedError`, `onReceivedJSError`）

#### 第五步：编写修复文档

将 Lynx 播放器 UI 修复经验固定为文档，更新 `PLAYER_ACTIVITY_FIX.md`，增加：
- Lynx CSS 兼容性清单
- vue-lynx 与 vue-router 的兼容性问题
- 正确的 Lynx Vue 组件架构（无路由、条件渲染）
- 调试方法（logcat 过滤、Lynx DevTool）

### 涉及文件

| 文件 | 修改内容 |
|------|---------|
| `app/encv-mobile/lynx-player/src/App.vue` | 移除 RouterView，改为条件渲染 |
| `app/encv-mobile/lynx-player/src/main.ts` | 移除 router 注册 |
| `app/encv-mobile/lynx-player/src/router.ts` | 删除此文件 |
| `app/encv-mobile/lynx-player/src/views/PlayerView.vue` | 移除 router，修复 CSS |
| `app/encv-mobile/lynx-player/src/views/HomeView.vue` | 移除 router，修复 CSS |
| `app/encv-mobile/lynx-player/src/views/SettingsView.vue` | 改为内联渲染，修复 CSS |
| `app/encv-mobile/lynx-player/src/views/PlaylistView.vue` | 改为内联渲染，修复 CSS |
| `app/encv-mobile/lynx-player/src/components/PlayerControls.vue` | 修复 CSS 兼容性 |
| `app/encv-mobile/lynx-player/src/components/ProgressBar.vue` | 修复 CSS 兼容性 |
| `app/encv-mobile/PLAYER_ACTIVITY_FIX.md` | 增加 Lynx 修复经验文档 |

---

## 实施顺序

1. **问题 1（加密覆盖检测）**— 影响数据安全，优先修复
2. **问题 2（任务列表增强）**— 功能增强，简单直接
3. **问题 3（Lynx 播放器 UI）**— 最复杂，需要逐步验证
4. **文档更新**— 最后完成
