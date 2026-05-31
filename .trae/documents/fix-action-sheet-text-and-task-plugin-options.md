# 修复长按菜单文本 + 任务创建插件选项 Bug

## Bug 1: 长按菜单显示 `alistEncrypt.encrypt` 而非"加密"

### 根因
`/workspace/app/encv-mobile/src/features/alist-encrypt/actions.ts:61` 使用 `t('alistEncrypt.encrypt')`，
但 i18n 字典中缺少 `alistEncrypt.encrypt` 这个 key。`t()` 函数在找不到 key 时直接返回 key 字符串本身。

### 修复
在 `/workspace/app/encv-mobile/src/composables/useI18n.ts` 中添加缺失的 i18n key：
- 中文: `'alistEncrypt.encrypt': '加密'`
- 英文: `'alistEncrypt.encrypt': 'Encrypt'`

---

## Bug 2: 选择 video 插件无配置项和容器版本选择 / 选择 alist_encrypt 错误使用 v4 容器

### 根因
`/workspace/internal/server/mobile_api.go:845-854` 的 `handlePredictPluginGin` 中，`taskOptions` 直接传递了 Go struct（`pluginInterfaces.TaskOptions`），该 struct **没有 json tag**，导致 JSON 序列化输出 PascalCase 字段名（`SupportVersionSelect`、`SupportedVersions` 等），而前端期望 camelCase（`supportVersionSelect`、`supportedVersions`）。

对比 `handlePluginsGin`（L793-812），那里**手动构建了 gin.H 用 camelCase key**，是正确的。

**结果**：
- 前端读取 `taskOptions.supportVersionSelect` 得到 `undefined`（实际 JSON 中是 `SupportVersionSelect`）
- 前端读取 `taskOptions.supportedVersions` 得到 `undefined` → `versionOptions` 为空 → 容器版本选择器不显示
- 前端读取 `taskOptions.extraFields` 得到 `undefined` → 额外配置项不显示
- `state.version` 硬编码为 `4`，且 `syncState()` 不从 taskOptions.defaultVersion 更新 → alist_encrypt 任务错误发送 version=4

### 修复步骤

#### 2a. 后端：`handlePredictPluginGin` 中手动构建 taskOptions 的 gin.H（与 handlePluginsGin 保持一致）

文件: `/workspace/internal/server/mobile_api.go`

将 `handlePredictPluginGin` 中的 `taskOptions: opts` 改为手动构建 camelCase gin.H，与 `handlePluginsGin` 一致：

```go
// 之前
candidateList = append(candidateList, gin.H{
    "name":        cand.Name,
    "matchType":   cand.MatchType,
    "priority":    cand.Priority,
    "taskOptions": opts,  // ← PascalCase!
})

// 之后
candidateList = append(candidateList, gin.H{
    "name":        cand.Name,
    "matchType":   cand.MatchType,
    "priority":    cand.Priority,
    "taskOptions": gin.H{
        "passwordStrategy":     string(opts.PasswordStrategy),
        "supportVersionSelect": opts.SupportVersionSelect,
        "supportedVersions":    opts.SupportedVersions,
        "defaultVersion":       opts.DefaultVersion,
        "extraFields":          opts.ExtraFields,
    },
})
```

同样修复解密分支（L834-842）中的 taskOptions 序列化。

#### 2b. 后端：给 `pluginInterfaces.TaskOptions` 和 `TaskField` 添加 json tag（根治）

文件: `/workspace/internal/v2/plugins/interfaces/interfaces.go`

```go
type TaskOptions struct {
    PasswordStrategy     PasswordStrategy `json:"passwordStrategy"`
    SupportVersionSelect bool             `json:"supportVersionSelect"`
    SupportedVersions    []int            `json:"supportedVersions"`
    DefaultVersion       int              `json:"defaultVersion"`
    ExtraFields          []TaskField      `json:"extraFields"`
}

type TaskField struct {
    Key          string   `json:"key"`
    Label        string   `json:"label"`
    Type         string   `json:"type"`
    Required     bool     `json:"required"`
    DefaultValue string   `json:"defaultValue"`
    Help         string   `json:"help"`
    Options      []string `json:"options,omitempty"`
    Condition    string   `json:"condition,omitempty"`
}
```

添加 json tag 后，`handlePredictPluginGin` 中直接传递 struct 也能正确序列化为 camelCase。同时 `handlePluginsGin` 中的手动 gin.H 构建可以简化为直接传 struct（但为安全起见，两处都保持一致即可）。

#### 2c. 前端：`syncState()` 从 taskOptions.defaultVersion 更新 state.version

文件: `/workspace/app/encv-mobile/src/composables/useNewTaskModal.ts`

在 `syncState()` 中，当 candidates 有结果时，从 taskOptions.defaultVersion 更新 state.version：

```typescript
function syncState() {
    state.candidates = candidates.value
    state.predictedPlugin = predictedPlugin.value
    state.selectedPluginIndex = selectedPluginIndex.value
    state.versionOptions = versionOptions.value ?? []
    state.extraValues = { ...extraValues.value }
    state.filteredExtraFields = visibleExtraFields.value
    if (candidates.value.length > 0) {
        state.taskOptions = candidates.value[selectedPluginIndex.value]?.taskOptions ?? null
        // 从插件声明同步默认版本
        const defaultVer = candidates.value[selectedPluginIndex.value]?.taskOptions?.defaultVersion
        if (defaultVer && defaultVer > 0) {
            state.version = defaultVer
        }
    }
}
```

#### 2d. 前端：`onSelectPlugin` 切换插件时同步版本和 taskOptions

文件: `/workspace/app/encv-mobile/src/composables/useNewTaskModal.ts`

当用户在多候选中选择不同插件时，需要同步 version 和 taskOptions：

```typescript
onSelectPlugin: (idx: number) => {
    state.selectedPluginIndex = idx
    if (candidates.value.length > 0) {
        state.taskOptions = candidates.value[idx]?.taskOptions ?? null
        const defaultVer = candidates.value[idx]?.taskOptions?.defaultVersion
        if (defaultVer && defaultVer > 0) {
            state.version = defaultVer
        } else if (!candidates.value[idx]?.taskOptions?.supportVersionSelect) {
            state.version = 0  // 插件不使用容器版本
        }
    }
},
```

#### 2e. 前端：createTask 提交时，不使用容器版本的插件不传 version

文件: `/workspace/app/encv-mobile/src/composables/useNewTaskModal.ts`

```typescript
// 之前
state.taskType === 'encrypt' ? state.version : undefined,

// 之后
const shouldSendVersion = state.taskType === 'encrypt' && state.taskOptions?.supportVersionSelect
shouldSendVersion ? state.version : undefined,
```

#### 2f. 前端：createTask 提交时传递 extraFields 和 secondaryPassword

文件: `/workspace/app/encv-mobile/src/composables/useNewTaskModal.ts`

当前 `onSubmit` 没有传递 `extraFields`（如 alist_encrypt 的 plugin_password）和 `secondaryPassword`：

```typescript
// 之前
await createTask(
    state.taskType as TaskType,
    state.sourcePath,
    state.targetPath || undefined,
    undefined,
    state.taskType === 'encrypt' ? state.version : undefined,
    pluginName
)

// 之后
const shouldSendVersion = state.taskType === 'encrypt' && state.taskOptions?.supportVersionSelect
const extraPayload = Object.keys(state.extraValues).length > 0 ? state.extraValues : undefined
await createTask(
    state.taskType as TaskType,
    state.sourcePath,
    state.targetPath || undefined,
    undefined,
    shouldSendVersion ? state.version : undefined,
    pluginName,
    extraPayload,
    state.secondaryPassword || undefined,
)
```

#### 2g. 后端：`handleCreateTaskGin` 接受 pluginName 字段

文件: `/workspace/internal/server/mobile_api.go`

当前 `handleCreateTaskGin` 不接受 `pluginName`，导致后端忽略前端选择的插件，总是自动检测。需要添加 `pluginName` 字段并在 `CreateWithExtras` 中传递。

但这涉及后端任务处理逻辑的较大改动（`TaskManager.processEncrypt` 需要根据 `pluginName` 选择插件而非自动检测），作为本 bug 修复范围，先在前端正确传递参数，后端 `pluginName` 支持作为后续优化。

---

## 修改文件清单

| 文件 | 修改内容 |
|------|---------|
| `app/encv-mobile/src/composables/useI18n.ts` | 添加 `alistEncrypt.encrypt` i18n key |
| `internal/v2/plugins/interfaces/interfaces.go` | TaskOptions + TaskField 添加 json tag |
| `internal/server/mobile_api.go` | handlePredictPluginGin 中 taskOptions 手动构建 camelCase gin.H |
| `app/encv-mobile/src/composables/useNewTaskModal.ts` | syncState 同步 defaultVersion；onSelectPlugin 同步版本；onSubmit 传递 extraFields/secondaryPassword/条件 version |
