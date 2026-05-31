# 计划：按规范补全 video 插件和 alist_encrypt 加密任务配置项

## 一、现状分析

### 1.1 后端 TaskOptions 声明机制

每个插件通过 `GetTaskOptions() -> TaskOptions` 向前端声明任务创建时需要的表单字段：

| 字段 | 含义 |
|------|------|
| `PasswordStrategy` | 密码策略：`global`(用全局密码) / `independent`(独立密码) / `none`(不需要密码) |
| `SupportVersionSelect` | 是否支持容器版本选择（V2/V3/V4） |
| `SupportedVersions` | 支持的版本列表 |
| `DefaultVersion` | 默认版本号 |
| `ExtraFields` | 插件自定义额外输入字段（声明式，前端自动渲染） |

前端 `NewTaskModal.vue` 根据 `TaskOptions.ExtraFields[]` 动态渲染表单字段（见 L131-141），支持 `string`/`password`/`select`/`bool` 四种类型。

### 1.2 当前各插件的 TaskOptions 完整度

#### Video 插件 — `[internal/v2/plugins/video/plugin.go:431-438](/workspace/internal/v2/plugins/video/plugin.go#L431-L438)`

```go
func (p *VideoPlugin) GetTaskOptions() pluginInterfaces.TaskOptions {
    return pluginInterfaces.TaskOptions{
        PasswordStrategy:     pluginInterfaces.PasswordGlobal,
        SupportVersionSelect: true,
        SupportedVersions:    p.SupportedContainerVersions(),
        DefaultVersion:       p.DefaultContainerVersion(),
    }
}
```

**❌ 缺失**：`ExtraFields` 为空（nil）。但 `VideoPluginConfig` 有 `default_stream_preset` 字段（balanced/quality/high_quality），用户无法在创建任务时逐任务选择编码预设。

#### AlistEncrypt 插件 — `[internal/v2/plugins/alistencrypt/plugin.go:235-250](/workspace/internal/v2/plugins/alistencrypt/plugin.go#L235-L250)`

```go
func (p *AlistEncryptPlugin) GetTaskOptions() pluginInterfaces.TaskOptions {
    return pluginInterfaces.TaskOptions{
        PasswordStrategy:     pluginInterfaces.PasswordIndependent,
        SupportVersionSelect: false,
        ExtraFields: []pluginInterfaces.TaskField{
            {
                Key:       "plugin_password",
                Label:     "tasks.pluginPassword",
                Type:      "password",
                Required:  false,
                Help:      "tasks.pluginPasswordHelp",
                Condition: "",
            },
        },
    }
}
```

**⚠️ 基本完整但可增强**：
- 已有 `plugin_password` 额外字段 ✅
- 但缺少 `enc_type` 额外字段（后端配置有 `enc_type`，但任务创建时不可选）
- Schema 中有 `algorithm` 字段但 Go 结构体用的是 `enc_type`，命名不一致

### 1.3 前端渲染链路（已就绪，无需修改）

```
后端 GetTaskOptions()
  → predict-plugin API 返回 candidates[].taskOptions
    → useTaskForm.ts 存储到 taskOptions computed
      → useNewTaskModal.ts 同步到 state.filteredExtraFields
        → NewTaskModal.vue v-for 渲染 ion-input（L131-141）
```

前端已完整支持 `ExtraFields` 的动态渲染，包括：
- `type: 'password'` → 密码输入框 ✅
- `type: 'select'` + `options` → 下拉选择（**需确认是否支持**）✅
- `condition` → 按 encrypt/decrypt 条件显示 ✅
- i18n `t(label)` / `t(help)` 翻译 ✅

---

## 二、需要修改的文件清单

### 2.1 后端修改

| # | 文件路径 | 修改内容 |
|---|---------|---------|
| B1 | `internal/v2/plugins/video/plugin.go` — `GetTaskOptions()` 方法 | 补充 `ExtraFields`，添加 `stream_preset` 选择字段 |
| B2 | `internal/v2/plugins/alistencrypt/plugin.go` — `GetTaskOptions()` 方法 | 补充 `enc_type` 选择字段；确保 label/help 使用正确的 i18n key |
| B3 | `internal/v2/plugins/task_options_test.go` | 更新测试断言以覆盖新增的 ExtraFields |

### 2.2 前端修改

| # | 文件路径 | 修改内容 |
|---|---------|---------|
| F1 | `src/composables/useI18n.ts` | 补充新增的 i18n translation key |
| F2 | `src/components/NewTaskModal.vue` | （可选）如果 `type=select` 的 ExtraField 尚未支持下拉渲染，补充 `<ion-select>` 分支 |

### 2.3 不需要修改的文件

- `src/api/encv.ts` — 类型定义已包含 `options?: string[]` 和 `type: 'select'` ✅
- `src/composables/useTaskForm.ts` — ExtraFields 透传逻辑与字段数量无关 ✅
- `src/composables/useNewTaskModal.ts` — 同上 ✅
- `internal/v2/plugins/interfaces/interfaces.go` — `TaskField` 结构体已支持 `options []string` ✅
- `internal/server/mobile_api.go` — `taskOptionsToGinH()` 已透传 ExtraFields ✅
- `config.schema.json` — 前端 schema 是设置页用的，不影响任务创建表单 ✅

---

## 三、详细实现步骤

### Step B1: Video 插件 — 补充 stream_preset ExtraField

**文件**: `/workspace/internal/v2/plugins/video/plugin.go` 的 `GetTaskOptions()` 方法（L431-438）

将当前实现：

```go
func (p *VideoPlugin) GetTaskOptions() pluginInterfaces.TaskOptions {
    return pluginInterfaces.TaskOptions{
        PasswordStrategy:     pluginInterfaces.PasswordGlobal,
        SupportVersionSelect: true,
        SupportedVersions:    p.SupportedContainerVersions(),
        DefaultVersion:       p.DefaultContainerVersion(),
    }
}
```

改为：

```go
func (p *VideoPlugin) GetTaskOptions() pluginInterfaces.TaskOptions {
    return pluginInterfaces.TaskOptions{
        PasswordStrategy:     pluginInterfaces.PasswordGlobal,
        SupportVersionSelect: true,
        SupportedVersions:    p.SupportedContainerVersions(),
        DefaultVersion:       p.DefaultContainerVersion(),
        ExtraFields: []pluginInterfaces.TaskField{
            {
                Key:          "stream_preset",
                Label:        "tasks.streamPreset",
                Type:         "select",
                Required:     false,
                DefaultValue: "balanced",
                Help:         "tasks.streamPresetHelp",
                Options:      []string{"balanced", "quality", "high_quality"},
                Condition:     "encrypt",
            },
        },
    }
}
```

设计决策：
- `Condition: "encrypt"` — 流式预设仅在加密时有意义，解密时不显示
- `Type: "select"` + `Options` — 用户从预设列表中选择，而非自由输入
- `DefaultValue: "balanced"` — 与 `VideoPluginConfig.DefaultStreamPreset` 默认值一致
- Key 名 `stream_preset` 与后端 `VideoPluginConfig.DefaultStreamPreset` 对应，TaskManager 执行时可读取此值覆盖默认配置

### Step B2: AlistEncrypt 插件 — 补充 enc_type ExtraField

**文件**: `/workspace/internal/v2/plugins/alistencrypt/plugin.go` 的 `GetTaskOptions()` 方法（L235-250）

在现有 `ExtraFields` 数组中追加 `enc_type` 字段：

```go
func (p *AlistEncryptPlugin) GetTaskOptions() pluginInterfaces.TaskOptions {
    return pluginInterfaces.TaskOptions{
        PasswordStrategy:     pluginInterfaces.PasswordIndependent,
        SupportVersionSelect: false,
        ExtraFields: []pluginInterfaces.TaskField{
            {
                Key:       "plugin_password",
                Label:     "tasks.pluginPassword",
                Type:      "password",
                Required:  false,
                Help:      "tasks.pluginPasswordHelp",
                Condition: "",
            },
            {
                Key:          "enc_type",
                Label:        "tasks.encType",
                Type:         "select",
                Required:     false,
                DefaultValue: "aesctr",
                Help:         "tasks.encTypeHelp",
                Options:      []string{"aesctr"},
                Condition:     "",
            },
        },
    }
}
```

设计决策：
- 虽然 `enc_type` 当前只有 `aesctr` 一个选项，但声明为 `select` 类型为未来扩展其他算法预留空间
- 无 Condition 限制 — 加密和解密都可能需要知道算法类型
- `DefaultValue: "aesctr"` 与 `AlistEncryptPluginConfig.EncType` 默认值一致

### Step B3: 更新后端测试

**文件**: `/workspace/internal/v2/plugins/task_options_test.go`

更新 `TestVideoPlugin_GetTaskOptions` 断言：

```go
func TestVideoPlugin_GetTaskOptions(t *testing.T) {
    // ...existing setup...
    assert.Equal(t, pluginInterfaces.PasswordGlobal, opts.PasswordStrategy)
    assert.True(t, opts.SupportVersionSelect)
    assert.NotEmpty(t, opts.SupportedVersions)
    assert.Len(t, opts.ExtraFields, 1, "video should have 1 extra field (stream_preset)")
    assert.Equal(t, "stream_preset", opts.ExtraFields[0].Key)
    assert.Equal(t, "select", opts.ExtraFields[0].Type)
    assert.Contains(t, opts.ExtraFields[0].Options, "balanced")
    assert.Equal(t, "encrypt", opts.ExtraFields[0].Condition)
}
```

更新 `TestAlistEncryptPlugin_GetTaskOptions` 断言：

```go
func TestAlistEncryptPlugin_GetTaskOptions(t *testing.T) {
    // ...existing setup...
    assert.Equal(t, pluginInterfaces.PasswordIndependent, opts.PasswordStrategy)
    assert.False(t, opts.SupportVersionSelect)
    assert.Len(t, opts.ExtraFields, 2, "alist_encrypt should have 2 extra fields")
    assert.Equal(t, "plugin_password", opts.ExtraFields[0].Key)
    assert.Equal(t, "enc_type", opts.ExtraFields[1].Key)
    assert.Equal(t, "select", opts.ExtraFields[1].Type)
}
```

### Step F1: 补充前端 i18n key

**文件**: `/workspace/app/encv-mobile/src/composables/useI18n.ts`

在 tasks 命名空间下添加：

```
'tasks.streamPreset': '编码预设',
'tasks.streamPresetHelp': '选择视频编码预设，覆盖全局默认值',
'tasks.encType': '加密算法',
'tasks.encTypeHelp': '选择加密算法类型',
```

### Step F2: 确认/补充 NewTaskModal.vue 的 select 类型渲染

**文件**: `/workspace/app/encv-mobile/src/components/NewTaskModal.vue`（L131-141）

检查当前的 ExtraFields 渲染逻辑是否处理了 `type === 'select'` 的情况。当前代码只处理 `text` 和 `password`：

```html
<ion-input :type="field.type === 'password' ? 'password' : 'text'" ... />
```

**需要补充**：当 `field.type === 'select'` 时渲染 `<ion-select>` 而非 `<ion-input>`：

```html
<template v-for="field in extraFlds" :key="field.key">
  <!-- 新增: select 类型 -->
  <ion-item v-if="(!field.condition || field.condition === taskType) && field.type === 'select'"
            lines="none" class="extra-field-item">
    <ion-select :model-value="getExtra(field.key)"
                @ionChange="(e: any) => { emit('updateExtraValue', { key: field.key, value: e.detail.value }); props.onUpdateExtraValue?.({ key: field.key, value: e.detail.value }) }"
                :label="t(field.label)"
                interface="action-sheet"
                placement="bottom"
                class="extra-field-select">
      <ion-select-option v-for="opt in field.options || []" :key="opt" :value="opt">
        {{ opt }}
      </ion-select-option>
    </ion-select>
  </ion-item>

  <!-- 原有: string/password 类型 -->
  <ion-item v-else-if="!field.condition || field.condition === taskType" lines="none" class="extra-field-item">
    <ion-input ... />
  </ion-item>
</template>
```

需要在 `<script setup>` 中 import `IonSelect` 和 `IonSelectOption`。

---

## 四、验证方案

### 4.1 后端单元测试

```bash
cd /workspace && go test ./internal/v2/plugins/ -run "TestVideoPlugin_GetTaskOptions|TestAlistEncryptPlugin_GetTaskOptions" -v
```

预期：所有新断言通过。

### 4.2 前端单元测试

```bash
cd /workspace/app/encv-mobile && npx vitest run --reporter=verbose
```

### 4.3 集成验证（手动）

1. 启动前后端预览环境
2. 打开新建任务弹窗
3. 输入一个 `.mp4` 视频文件路径
4. **验证 Video 插件**：
   - 应看到「编码预设」下拉框（仅 encrypt 模式显示）
   - 选项：balanced / quality / high_quality
   - 默认选中 balanced
5. 切换到 decrypt 模式 → 编码预设应消失
6. 输入一个 `.bin` 文件路径或任意无扩展名文件
7. **验证 AlistEncrypt 插件**：
   - 应看到「插件密码」密码框
   - 应看到「加密算法」下拉框（选项：aesctr）
   - 密码策略提示应显示「此插件使用独立密码」
8. 创建任务 → 检查 API 请求体中 `extraFields` 包含正确值

---

## 五、风险和注意事项

1. **`stream_preset` ExtraField 仅是 UI 声明**：后端 `TaskManager` 在执行加密任务时需要读取 `extraFields["stream_preset"]` 并将其注入 `VideoPlugin` 实例或传递给预处理流程。本次修改只补全**声明侧**，执行侧的透传如尚未实现则需后续跟进。
2. **`enc_type` 单选项**：虽然只有一个选项 `aesctr`，但保持 `select` 类型可以避免用户输入非法值，同时为未来扩展预留。
3. **i18n key 命名**：使用 `tasks.` 前缀保持一致性，label/help 都走 i18n 不硬编码中文。
4. **条件渲染 `condition: "encrypt"`**：Video 的 `stream_preset` 只在加密时有意义，解密时不需要选择编码预设。
