# 任务创建界面插件化适配计划

## 一、问题诊断

### 当前 Tasks.vue 任务模态框（L135-211）的硬编码问题

| 问题 | 位置 | 详情 |
|------|------|------|
| **密码字段形同虚设** | L292 声明 `newTaskPassword`，但模板 L193-204 是 disabled + "comingSoon" badge | Alist-Encrypt 解密需要密码，但前端无法输入 |
| **容器版本选择硬编码** | L188 `v-if="newTaskType === 'encrypt'"` | Video 插件支持版本选择，Alist-Encrypt 不支持（返回 nil），但前端不知道 |
| **无插件感知** | 整个模态框 | 不知道用户选的文件会由哪个插件处理，无法显示插件特有选项 |
| **`newTaskPassword` 未传入 API** | L537-543 `handleCreateTask` | 即使填写了密码也不会发送到后端 |

### 后端 `/api/plugins` 返回信息不足（mobile_api.go L768-779）

当前返回：`name`, `supportedExtensions`, `supportedMimePrefixes`, `containerExtension`

缺失：
- `settingFields` — 插件声明的配置项（后端 `GetSettingFields()` 已有，未暴露）
- `supportedContainerVersions` / `defaultContainerVersion` — 版本支持能力
- 插件是否支持版本选择（alist_encrypt 返回 nil）

### 核心矛盾

```
视频插件任务需要：容器版本选择（V2/V3/V4）
Alist-Encrypt 任务需要：密码输入（解密时必填）
→ 但前端对两者一视同仁，只有通用的 路径+版本(仅加密)
```

---

## 二、设计原则

> **插件声明式，前端委托渲染，不写胶水代码。**

参照已有的成功模式：
- `PluginSettings.vue` 从 schema.json 动态渲染 → **任务表单也应从插件声明动态渲染**
- `GetSettingFields()` 驱动设置页 → **新增 `GetTaskFields()` 驱动任务页**

---

## 三、实施方案

### Step 1: 后端 — Plugin 接口新增 `GetTaskOptions()` 方法

**文件**: `internal/v2/plugins/registry.go` — Plugin interface

在现有接口中新增方法：

```go
// GetTaskOptions 返回该插件在创建加解密任务时需要的额外选项声明
// 前端根据此声明动态渲染表单字段，无需硬编码插件特定逻辑
func GetTaskOptions() TaskOptions

type TaskOptions struct {
    // SupportVersionSelect 表示是否支持容器版本选择
    // video=true, alist_encrypt=false
    SupportVersionSelect bool
    
    // SupportedVersions 可选的容器版本列表（nil 表示不支持）
    // DefaultVersion 默认版本号
    SupportedVersions []int
    DefaultVersion     int
    
    // ExtraFields 任务创建所需的额外输入字段声明
    // 例如 Alist-Encrypt 声明 password 字段
    ExtraFields []TaskField
}

type TaskField struct {
    Key          string   // 字段名，如 "password"
    Label        string   // 显示标签
    Type         string   // "string" | "password" | "select" | "bool"
    Required     bool     // 是否必填
    DefaultValue string   // 默认值
    Help         string   // 帮助文本
    Options      []string // Type="select" 时的可选项
    Condition    string   // 显示条件: "encrypt" | "decrypt" | ""(始终显示)
}
```

### Step 2: 各插件实现 `GetTaskOptions()`

**文件**: `internal/v2/plugins/video/plugin.go`

```go
func (p *VideoPlugin) GetTaskOptions() plugins.TaskOptions {
    return plugins.TaskOptions{
        SupportVersionSelect: true,
        SupportedVersions:    p.SupportedContainerVersions(),
        DefaultVersion:       p.DefaultContainerVersion(),
        ExtraFields:          []plugins.TaskField{}, // 视频插件无需额外字段
    }
}
```

**文件**: `internal/v2/plugins/alistencrypt/plugin.go`

```go
func (p *AlistEncryptPlugin) GetTaskOptions() plugins.TaskOptions {
    return plugins.TaskOptions{
        SupportVersionSelect: false, // 不使用 ENCV 容器版本
        ExtraFields: []plugins.TaskField{
            {
                Key:       "plugin_password",
                Label:     "Encryption Password",
                Type:      "password",
                Required:  false,
                Help:      "Leave empty to use the default password from plugin settings",
                Condition: "decrypt", // 仅解密时可手动指定（加密用默认密码）
            },
        },
    }
}
```

其他插件（text/audio/image/pdf/wps）返回空 `TaskOptions{}`。

### Step 3: 后端 — 增强 `/api/plugins` 端点

**文件**: `internal/server/mobile_api.go` — `handlePluginsGin()`

在返回的 PluginMeta 中增加 `taskOptions` 字段：

```go
func (s *Server) handlePluginsGin(c *gin.Context) {
    var metas []gin.H
    for _, p := range plugins.Plugins {
        opts := p.GetTaskOptions()
        metas = append(metas, gin.H{
            "name":                  p.Name(),
            "supportedExtensions":   p.SupportedExtensions(),
            "supportedMimePrefixes": p.SupportedMimePrefixes(),
            "containerExtension":    p.GetContainerExtension(),
            "taskOptions": gin.H{
                "supportVersionSelect": opts.SupportVersionSelect,
                "supportedVersions":    opts.SupportedVersions,
                "defaultVersion":       opts.DefaultVersion,
                "extraFields":          opts.ExtraFields,
            },
        })
    }
    c.JSON(200, gin.H{"plugins": metas})
}
```

### Step 4: 后端 — 新增 `/api/tasks/predict-plugin` 端点

**目的**: 前端输入源文件路径后，调用此接口得知将由哪个插件处理，从而显示对应的 `taskOptions`。

**文件**: `internal/server/mobile_api.go`

```go
// POST /api/tasks/predict-plugin
// Request: { "sourcePath": "/path/to/file.mp4", "type": "encrypt"|"decrypt" }
// Response: { "pluginName": "video", "taskOptions": {...} }
func (s *Server) handlePredictPluginGin(c *gin.Context) {
    var req struct {
        SourcePath string `json:"sourcePath"`
        Type       string `json:"type"` // "encrypt" | "decrypt"
    }
    // bind JSON ...
    
    var targetPlugin plugins.Plugin
    var err error
    if req.Type == "encrypt" {
        targetPlugin, err = plugins.FindEncryptingPlugin(req.SourcePath)
    } else {
        targetPlugin, err = plugins.FindDecryptingPlugin(req.SourcePath)
    }
    
    if err != nil {
        c.JSON(200, gin.H{"pluginName": nil, "error": err.Error(), "taskOptions": nil})
        return
    }
    
    opts := targetPlugin.GetTaskOptions()
    c.JSON(200, gin.H{
        "pluginName":  targetPlugin.Name(),
        "taskOptions": opts,
    })
}
```

### Step 5: 前端 — 新建 `useTaskForm` composable

**文件**: `app/encv-mobile/src/composables/useTaskForm.ts`（新建）

职责：
1. 管理 `predictPlugin` API 调用（防抖）
2. 根据 `taskOptions.extraFields` 动态生成表单状态（`extraValues` ref）
3. 暴露 `taskOptions`、`extraFields`、`predictedPluginName` 给组件
4. 提供 `getExtraPayload()` 方法收集额外参数供 `createTask()` 使用

核心逻辑：
```typescript
export function useTaskForm() {
  const predictedPlugin = ref<string | null>(null)
  const taskOptions = ref<TaskOptions | null>(null)
  const extraValues = ref<Record<string, string>>({})
  
  async function predictPlugin(sourcePath: string, taskType: TaskType) {
    // POST /api/tasks/predict-plugin
    // 更新 predictedPlugin + taskOptions
    // 初始化 extraValues 默认值
  }
  
  function getExtraPayload(): Record<string, unknown> {
    // 收集 extraValues 中非空字段
  }
  
  return { predictedPlugin, taskOptions, extraValues, predictPlugin, getExtraPayload }
}
```

### Step 6: 前端 — 重构 Tasks.vue 任务模态框

**文件**: `app/encv-mobile/src/views/Tasks.vue`

改动点：

#### 6a. 引入 `useTaskForm`

```typescript
const {
  predictedPlugin,
  taskOptions,
  extraValues,
  predictPlugin,
  getExtraPayload,
} = useTaskForm()
```

#### 6b. 源路径变化时触发插件预测

在 `validateSourcePath` 成功后（路径存在），自动调用 `predictPlugin(newTaskPath.value, newTaskType.value)`。

#### 6c. 模板动态渲染替换硬编码区域

**替换前**（硬编码）:
```html
<!-- 容器版本选择（仅加密时显示） -->
<ion-item v-if="newTaskType === 'encrypt'">
  <ContainerVersionSelector v-model="newTaskVersion" />
</ion-item>
<!-- 二级密码（占位） -->
<ion-item>
  <ion-input v-model="newTaskSecondaryPassword" ... disabled />
  <ion-badge color="medium" slot="end">Coming Soon</ion-badge>
</ion-item>
```

**替换后**（声明式驱动）:
```html
<!-- 容器版本选择：仅当目标插件支持且为加密模式时显示 -->
<ion-item v-if="taskOptions?.supportVersionSelect && newTaskType === 'encrypt'">
  <ContainerVersionSelector 
    v-model="newTaskVersion" 
    :versions="versionOptions" 
  />
</ion-item>

<!-- 插件声明的额外字段（动态渲染） -->
<template v-for="field in visibleExtraFields" :key="field.key">
  <ion-item>
    <ion-input
      v-model="extraValues[field.key]"
      :label="field.label"
      :type="field.type"
      :placeholder="field.help"
    ></ion-input>
  </ion-item>
</template>

<!-- 预测到的插件名称提示 -->
<ion-note v-if="predictedPlugin" class="plugin-hint">
  {{ t('tasks.willBeHandledBy', { plugin: predictedPlugin }) }}
</ion-note>
```

其中 `visibleExtraFields` 是 computed：
```typescript
const visibleExtraFields = computed(() => {
  if (!taskOptions.value?.extraFields) return []
  return taskOptions.value.extraFields.filter(f => {
    if (!f.condition) return true
    if (f.condition === 'encrypt') return newTaskType.value === 'encrypt'
    if (f.condition === 'decrypt') return newTaskType.value === 'decrypt'
    return true
  })
})

const versionOptions = computed(() => {
  if (!taskOptions.value?.supportedVersions) return undefined
  return taskOptions.value.supportedVersions.map(v => ({
    version: v,
    status: v === taskOptions.value.defaultVersion ? 'recommended' as const :
           v === 2 ? 'deprecated' as const : 'stable' as const,
    label: `V${v}`,
  }))
})
```

#### 6d. 创建任务时携带额外参数

```typescript
async function handleCreateTask() {
  if (!newTaskPath.value) return
  try {
    await createTask(
      newTaskType.value,
      newTaskPath.value,
      newTaskTargetPath.value || undefined,
      extraValues.value.plugin_password || undefined,  // 密码
      taskOptions.value?.supportVersionSelect ? newTaskVersion.value : undefined,
    )
    // ...
  }
}
```

#### 6e. 删除废弃代码

- 删除 `newTaskSecondaryPassword` ref 及相关模板
- 删除 `newTaskPassword` ref（改用 `extraValues.plugin_password`）

### Step 7: 前端 — API 层更新

**文件**: `app/encv-mobile/src/api/encv.ts`

新增类型和函数：

```typescript
export interface TaskField {
  key: string
  label: string
  type: 'string' | 'password' | 'select' | 'bool'
  required: boolean
  defaultValue: string
  help: string
  options?: string[]
  condition?: '' | 'encrypt' | 'decrypt'
}

export interface TaskOptions {
  supportVersionSelect: boolean
  supportedVersions: number[] | null
  defaultVersion: number
  extraFields: TaskField[]
}

export interface PredictPluginResponse {
  pluginName: string | null
  error?: string
  taskOptions: TaskOptions | null
}

export async function predictPlugin(sourcePath: string, type: TaskType): Promise<PredictPluginResponse> {
  // POST /api/tasks/predict-plugin
}
```

同步更新 `fetchPlugins()` 返回类型中的 PluginMeta 增加 `taskOptions`。

### Step 8: 前端 — i18n 新增翻译

**文件**: `app/encv-mobile/src/composables/useI18n.ts`

新增 key:
- `tasks.willBeHandledBy`: "此文件将由 {plugin} 插件处理"
- `tasks.pluginPassword`: "加密密码" / "Encryption Password"
- `tasks.pluginPasswordHelp`: "留空则使用插件设置的默认密码"

### Step 9: 测试

#### 后端测试

**文件**: `internal/v2/plugins/registry_test.go` 或新建 `task_options_test.go`

```go
func TestVideoPlugin_GetTaskOptions(t *testing.T) {
    p := &video.VideoPlugin{}
    opts := p.GetTaskOptions()
    assert.True(t, opts.SupportVersionSelect)
    assert.NotEmpty(t, opts.SupportedVersions)
    assert.Empty(t, opts.ExtraFields)
}

func TestAlistEncryptPlugin_GetTaskOptions(t *testing.T) {
    p := &alistencrypt.AlistEncryptPlugin{}
    opts := p.GetTaskOptions()
    assert.False(t, opts.SupportVersionSelect)
    assert.Len(t, opts.ExtraFields, 1)
    assert.Equal(t, "password", opts.ExtraFields[0].Key)
    assert.Equal(t, "decrypt", opts.ExtraFields[0].Condition)
}
```

#### 前端测试

**文件**: `app/encv-mobile/src/__tests__/useTaskForm.test.ts`（新建）

- mock `predictPlugin` API
- 测试视频插件预测结果包含版本选择、无额外字段
- 测试 Alist-Encrypt 预测结果无版本选择、有密码字段（仅 decrypt 条件显示）
- 测试 `getExtraPayload()` 正确收集已填写的额外字段
- 测试未知文件类型预测失败时的降级行为

---

## 四、文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/v2/plugins/registry.go` | 修改 | 新增 `TaskOptions`/`TaskField` 类型 + `GetTaskOptions()` 接口方法 |
| `internal/v2/plugins/video/plugin.go` | 修改 | 实现 `GetTaskOptions()` |
| `internal/v2/plugins/alistencrypt/plugin.go` | 修改 | 实现 `GetTaskOptions()` |
| `internal/server/mobile_api.go` | 修改 | 增强 `handlePluginsGin` + 新增 `handlePredictPluginGin` |
| `internal/server/server.go` | 修改 | 注册新路由 `POST /api/tasks/predict-plugin` |
| `app/encv-mobile/src/api/encv.ts` | 修改 | 新增 `TaskOptions`/`TaskField` 类型 + `predictPlugin()` 函数 |
| `app/encv-mobile/src/composables/useTaskForm.ts` | **新建** | 任务表单 composable |
| `app/encv-mobile/src/views/Tasks.vue` | 修改 | 模态框改为声明式渲染 |
| `app/encv-mobile/src/composables/useI18n.ts` | 修改 | 新增翻译 key |
| `internal/server/mobile_api_test.go` 或新建 | **新建** | 后端 TaskOptions 测试 |
| `app/encv-mobile/src/__tests__/useTaskForm.test.ts` | **新建** | 前端 composable 测试 |

---

## 五、不做的事情（边界）

- **不改任务执行流程** — 只改创建界面的 UI 适配，后端 `Create()` 方法签名不变
- **不改 schema.json 驱动的 PluginSettings** — 那是全局配置页，与任务创建是不同场景
- **不给其他 5 个插件（text/audio/image/pdf/wps）添加特殊字段** — 它们返回空 `ExtraFields` 即可
- **不在前端实现 MIME 检测** — 插件预测完全委托给后端 `/api/tasks/predict-plugin`（符合防御性编程铁律：不硬编码动态数据）
