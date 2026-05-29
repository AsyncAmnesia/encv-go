# MPV 播放器状态显示方案（遵循铁律）

## 铁律约束

1. **严禁自动 fallback**：MPV 不可用时不能自动切换到 Artplayer
2. **严禁 Toast 提示**：不能用 Toast 临时提示

## 正确做法

在设置页面播放器选项旁显示插件状态标签，让用户明确知道当前状态并主动选择。

## 修复方案

### 核心思路

- **设置页面状态显示**：MPV 播放器选项旁显示状态标签（未安装/已禁用/已加载）
- **选项禁用**：MPV 未就绪时禁用该选项，防止用户误选
- **日志输出**：关键路径必须有 Log.i 输出，便于调试

### 修复层级

| 层级 | 文件 | 修复内容 |
|------|------|---------|
| 前端 UI | Settings.vue | 显示 MPV 插件状态 + 禁用未就绪选项 |
| Kotlin 面 | EncvComboLiteHost.kt | 完善状态查询 API |
| Kotlin 引擎 | PluginLifecycleEngine.kt | `ensurePluginLoaded()` 返回布尔值 + 日志 |

### 详细修复步骤

#### Step 1: EncvComboLiteHost.kt — 完善状态查询

```kotlin
// 新增：获取完整状态（供前端显示）
fun getPluginFullState(pluginId: String): PluginFullState {
    if (!PluginLifecycleEngine.isInitialized()) {
        return PluginFullState(id = pluginId, status = "framework_not_ready")
    }
    val state = getPluginInfo(pluginId)
    if (state == null) {
        return PluginFullState(id = pluginId, status = "not_installed")
    }
    if (!state.enabled) {
        return PluginFullState(id = pluginId, status = "disabled", name = state.name)
    }
    val loaded = PluginLifecycleEngine.isPluginLoaded(pluginId)
    return PluginFullState(
        id = pluginId,
        status = if (loaded) "ready" else "not_loaded",
        name = state.name,
        version = state.versionName
    )
}

// 修复：检查完整状态（installed + enabled + loaded）
fun isPluginAvailable(pluginId: String): Boolean {
    if (!PluginLifecycleEngine.isInitialized()) return false
    val state = getPluginInfo(pluginId)
    return state != null && state.installed && state.enabled && PluginLifecycleEngine.isPluginLoaded(pluginId)
}
```

#### Step 2: PluginLifecycleEngine.kt — ensurePluginLoaded 返回布尔值 + 日志

```kotlin
// 修复：返回布尔值 + 日志
fun ensurePluginLoaded(pluginId: String): Boolean {
    if (!PluginManager.isInitialized) {
        Log.w(TAG, "ensurePluginLoaded($pluginId): PluginManager not initialized")
        return false
    }
    return try {
        if (PluginManager.getPluginInfo(pluginId) != null) {
            Log.i(TAG, "ensurePluginLoaded($pluginId): already loaded")
            true
        } else {
            Log.i(TAG, "ensurePluginLoaded($pluginId): loading...")
            val success = runBlocking { launchPlugin(pluginId) }
            Log.i(TAG, "ensurePluginLoaded($pluginId): load result=$success")
            success
        }
    } catch (e: Exception) {
        Log.e(TAG, "ensurePluginLoaded($pluginId): failed", e)
        false
    }
}

// 新增：检查是否已加载
fun isPluginLoaded(pluginId: String): Boolean {
    if (!PluginManager.isInitialized) return false
    return PluginManager.getPluginInfo(pluginId) != null
}
```

#### Step 3: GoProcessPlugin.kt — 新增状态查询 API

```kotlin
@PluginMethod
fun getPluginFullState(call: PluginCall) {
    val pluginId = call.getString("pluginId") ?: run { call.reject("pluginId required"); return }
    val state = EncvComboLiteHost.getPluginFullState(pluginId)
    call.resolve(JSObject().apply {
        put("id", state.id)
        put("status", state.status)
        put("name", state.name ?: "")
        put("version", state.version ?: "")
    })
}
```

#### Step 4: GoProcess.ts — 前端 API

```typescript
export interface PluginFullState {
    id: string
    status: 'ready' | 'not_installed' | 'disabled' | 'not_loaded' | 'framework_not_ready' | 'error'
    name: string
    version: string
}

export async function getPluginFullState(pluginId: string): Promise<PluginFullState> {
    try {
        const result = await GoProcess.getPluginFullState({ pluginId })
        return result
    } catch (e) {
        console.error('[GoProcess] getPluginFullState failed:', e)
        return { id: pluginId, status: 'error', name: '', version: '' }
    }
}
```

#### Step 5: Settings.vue — 显示 MPV 插件状态 + 禁用未就绪选项

```vue
<ion-item>
    <ion-select
        :value="videoPlayerMode"
        @ionChange="handleVideoPlayerChange"
        :label="t('settings.videoPlayer')"
        label-placement="stacked"
        interface="action-sheet"
        mode="ios"
    >
        <ion-select-option value="artplayer">{{ t('settings.builtInArtplayer') }}</ion-select-option>
        <ion-select-option 
            value="mpv-plugin" 
            :disabled="mpvPluginStatus !== 'ready'"
        >
            {{ t('settings.mpvPluginExtension') }}
            <span v-if="mpvPluginStatus === 'not_installed'" style="color: var(--ion-color-warning)">
                (未安装)
            </span>
            <span v-if="mpvPluginStatus === 'disabled'" style="color: var(--ion-color-warning)">
                (已禁用)
            </span>
            <span v-if="mpvPluginStatus === 'not_loaded'" style="color: var(--ion-color-medium)">
                (未加载)
            </span>
            <span v-if="mpvPluginStatus === 'ready'" style="color: var(--ion-color-success)">
                ✓
            </span>
        </ion-select-option>
        <ion-select-option value="external">{{ t('settings.openExternal') }}</ion-select-option>
    </ion-select>
</ion-item>
```

```typescript
const mpvPluginStatus = ref<string>('unknown')

onMounted(async () => {
    if (isNative()) {
        const state = await getPluginFullState('com.encvgo.plugin.mpv')
        mpvPluginStatus.value = state.status
        console.info('[Settings] MPV plugin status:', state.status)
    }
})
```

## 任务清单

- [ ] Task 1: EncvComboLiteHost.kt — 完善状态查询 API
  - [ ] SubTask 1.1: 新增 `getPluginFullState()` 方法
  - [ ] SubTask 1.2: 新增 `PluginFullState` 数据类
  - [ ] SubTask 1.3: 修复 `isPluginAvailable()` 检查完整状态
- [ ] Task 2: PluginLifecycleEngine.kt — ensurePluginLoaded 返回布尔值 + 日志
  - [ ] SubTask 2.1: `ensurePluginLoaded()` 返回 Boolean
  - [ ] SubTask 2.2: 添加 Log.i 日志输出
  - [ ] SubTask 2.3: 新增 `isPluginLoaded()` 方法
- [ ] Task 3: GoProcessPlugin.kt — 新增状态查询 API
  - [ ] SubTask 3.1: 新增 `getPluginFullState()` @PluginMethod
- [ ] Task 4: GoProcess.ts — 前端 API 封装
  - [ ] SubTask 4.1: 新增 `PluginFullState` 类型定义
  - [ ] SubTask 4.2: 新增 `getPluginFullState()` 函数
- [ ] Task 5: Settings.vue — 显示 MPV 插件状态 + 禁用未就绪选项
  - [ ] SubTask 5.1: 新增 `mpvPluginStatus` ref
  - [ ] SubTask 5.2: onMounted 时查询插件状态
  - [ ] SubTask 5.3: MPV 选项显示状态标签
  - [ ] SubTask 5.4: MPV 未就绪时禁用选项

## 验证标准

1. **设置页面**：MPV 播放器选项旁显示状态（未安装/已禁用/已加载）
2. **选项禁用**：MPV 未就绪时选项禁用，用户无法选择
3. **状态准确**：状态与实际插件状态一致
4. **日志输出**：关键路径有 Log.i 输出，便于调试
5. **无白屏**：用户选择 MPV 时若未就绪，选项已禁用不会触发

## 风险评估

- **低风险**：仅添加状态显示和选项禁用，不改变播放逻辑
- **向后兼容**：Artplayer 模式完全不受影响
- **性能影响**：状态查询仅在设置页面加载时执行一次