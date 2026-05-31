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

### 1.2 文件名编码机制（⚠️ 核心认知：均为可选，默认不启用）

#### Video 插件 — 容器级文件名加密（可选）

```
当前行为 (plugin.go:571):
  finalFilename = chunkNamer.GenerateMainChunkName(baseNamer.GenerateEncryptedBaseName(originalFilename))
  → 无条件生成哈希式名称: movie_2025.mp4 → a1b2c3d4.sccgv

期望行为:
  encrypt_filename=false (默认): 保留原始文件名或使用可读命名
  encrypt_filename=true (用户勾选): 使用 GenerateEncryptedBaseName() 编码
```

**关键代码路径**：
- [video.go:472](/workspace/internal/v2/plugins/video/plugin.go#L472) — PreEncryptProcessor 中无条件调用 `GenerateEncryptedBaseName()`
- [video.go:571](/workspace/internal/v2/plugins/video/plugin.go#L571) — PostEncryptProcessor 中再次调用
- [container_namer.go:16](/workspace/internal/v2/namer/container_namer.go#L16) — `GenerateEncryptedBaseName()` 哈希实现

#### AlistEncrypt 插件 — 独立文件名编码（可选）

```
当前行为 (plugin.go:164-175):
  PostEncryptProcessor → RenameToFinalEncrypted(tempPath, originalFilename, outputDir, suffix)
    → 仅改扩展名: secret.txt → secret.bin
    → ❌ 不调用 EncodeName()

  EncodeName()/DecodeName() 仅用于:
    - /api/alist-encode/encode-filename API（前端手动触发）
    - /api/alist-encrypt/decode-filename API（前端显示转换）

期望行为:
  encode_filename=false (默认): 只改扩展名 .xxx → .bin（当前行为）
  encode_filename=true (用户勾选): 调用 EncodeName() 编码文件名 → xYzAbCdE123.bin
```

**关键代码路径**：
- [plugin.go:167](/workspace/internal/v2/plugins/alistencrypt/plugin.go#L167) — `RenameToFinalEncrypted()` 只改扩展名
- [filename.go:229](/workspace/internal/v2/plugins/alistencrypt/filename.go#L229) — `EncodeName()` 编码函数（MixBase64 + CRC6）
- [filename.go:242](/workspace/internal/v2/plugins/alistencrypt/filename.go#L242) — `DecodeName()` 解码函数
- [encryptor.go:67](/workspace/internal/v2/plugins/alistencrypt/encryptor.go#L67) — `RenameToFinalEncrypted()` 实现

#### 差异对比表

| 维度 | Video 插件 | AlistEncrypt 插件 |
|------|-----------|-------------------|
| **编码方式** | SHA 哈希式 (`GenerateEncryptedBaseName`) | MixBase64 变体 + CRC6 校验 (`EncodeName`) |
| **当前默认行为** | ⚠️ 无条件编码（需改为可选） | ✅ 不编码（只改扩展名） |
| **目标默认行为** | 不编码（`encrypt_filename=false`） | 不编码（`encode_filename=false`） |
| **启用后效果** | 文件名变为哈希值 + 容器扩展名 | 文件名变为 Base64 编码 + `.bin` |
| **密钥来源** | 全局密码（PasswordGlobal） | 独立密码 plugin_password |
| **算法参数** | 无额外参数 | 需指定 `enc_type`（aesctr/rc4md5/chacha20） |

### 1.3 当前各插件的 TaskOptions 完整度

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

**❌ 缺失 ExtraFields**：
1. `stream_preset`（`type: select`）— 编码预设选择：balanced/quality/high_quality
2. `encrypt_filename`（`type: bool`）— 是否编码文件名，默认 false

#### AlistEncrypt 插件 — `[internal/v2/plugins/alistencrypt/plugin.go:235-250](/workspace/internal/v2/plugins/alistencrypt/plugin.go#L235-L250)`

```go
func (p *AlistEncryptPlugin) GetTaskOptions() pluginInterfaces.TaskOptions {
    return pluginInterfaces.TaskOptions{
        PasswordStrategy:     pluginInterfaces.PasswordIndependent,
        SupportVersionSelect: false,
        ExtraFields: []pluginInterfaces.TaskField{
            { Key: "plugin_password", Label: "tasks.pluginPassword", Type: "password", ... },
        },
    }
}
```

**❌ 缺失 ExtraFields**：
1. `encode_filename`（`type: bool`）— 是否编码文件名，默认 false
2. `enc_type`（`type: select`）— 文件名编码算法（仅在 `encode_filename=true` 时有意义）：aesctr/rc4md5/chacha20

### 1.4 前端渲染链路（已就绪，无需修改核心逻辑）

```
后端 GetTaskOptions()
  → predict-plugin API 返回 candidates[].taskOptions
    → useTaskForm.ts 存储到 taskOptions computed
      → useNewTaskModal.ts 同步到 state.filteredExtraFields
        → NewTaskModal.vue v-for 渲染 ion-input / ion-select / ion-toggle（L131-141）
```

前端已支持 ExtraFields 动态渲染的 `string`/`password` 类型，**需补充** `select` 和 `bool` 类型的渲染分支。

---

## 二、需要修改的文件清单

### 2.1 后端修改

| # | 文件 | 修改内容 |
|---|------|---------|
| B1 | `internal/v2/plugins/video/plugin.go` — `GetTaskOptions()` L431-438 | 补充 ExtraFields：`stream_preset`(select) + `encrypt_filename`(bool) |
| B2 | `internal/v2/plugins/alistencrypt/plugin.go` — `GetTaskOptions()` L235-250 | 补充 ExtraFields：`encode_filename`(bool) + `enc_type`(select) |
| B3 | `internal/v2/plugins/task_options_test.go` | 更新测试断言覆盖新增 ExtraFields |

### 2.2 前端修改

| # | 文件 | 修改内容 |
|---|------|---------|
| F1 | `src/composables/useI18n.ts` | 补充 i18n key：streamPreset、encryptFilename、encType、encTypeHelp 等 |
| F2 | `src/components/NewTaskModal.vue` | 补充 `type='select'` 的 `<ion-select>` 渲染 + `type='bool'` 的 `<ion-toggle>` 渲染 |

### 2.3 不需要修改的文件

- `src/api/encv.ts` — 类型定义已包含 `options?: string[]` 和 `type: 'select'/'bool'` ✅
- `src/composables/useTaskForm.ts` — ExtraFields 透传逻辑与字段数量无关 ✅
- `src/composables/useNewTaskModal.ts` — 同上 ✅
- `internal/v2/plugins/interfaces/interfaces.go` — `TaskField` 结构体已支持所有类型 ✅
- `internal/server/mobile_api.go` — `taskOptionsToGinH()` 已透传 ExtraFields ✅

---

## 三、详细实现步骤

### Step B1: Video 插件 — 补全 ExtraFields

**文件**: `/workspace/internal/v2/plugins/video/plugin.go` — `GetTaskOptions()` (L431-438)

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
            {
                Key:          "encrypt_filename",
                Label:        "tasks.encryptFilename",
                Type:         "bool",
                Required:     false,
                DefaultValue: "false",
                Help:         "tasks.encryptFilenameVideoHelp",
                Condition:     "encrypt",
            },
        },
    }
}
```

设计决策：
- **`stream_preset`**（select, condition=encrypt）：仅加密时需要，覆盖全局 `default_stream_preset`
- **`encrypt_filename`**（bool, default=false, condition=encrypt）：用户主动勾选才编码文件名，默认保留原始文件名的可读形式

### Step B2: AlistEncrypt 插件 — 补全 ExtraFields

**文件**: `/workspace/internal/v2/plugins/alistencrypt/plugin.go` — `GetTaskOptions()` (L235-250)

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
                Help:      "tasks.alistPasswordHelp",
                Condition: "",
            },
            {
                Key:          "encode_filename",
                Label:        "tasks.encodeFilename",
                Type:         "bool",
                Required:     false,
                DefaultValue: "false",
                Help:         "tasks.encodeFilenameHelp",
                Condition:     "encrypt",
            },
            {
                Key:          "enc_type",
                Label:        "tasks.encType",
                Type:         "select",
                Required:     false,
                DefaultValue: "aesctr",
                Help:         "tasks.encTypeHelp",
                Options:      []string{"aesctr", "rc4md5", "chacha20"},
                Condition:     "encrypt",
            },
        },
    }
}
```

设计决策：
- **`encode_filename`**（bool, default=false, condition=encrypt）：默认不编码文件名（当前行为），勾选后才调用 `EncodeName()`
- **`enc_type`**（select, condition=encrypt）：仅当 `encode_filename=true` 时有意义，选择 MixBase64 编码算法。暴露全部 3 个选项为扩展预留
- **`plugin_password` Help 更新**：改用 `tasks.alistPasswordHelp`，说明密码用途（内容加密始终需要，文件名编码仅在勾选时使用）

### Step B3: 更新后端测试

**文件**: `/workspace/internal/v2/plugins/task_options_test.go`

Video 断言更新（ExtraFields 从 0 → 2）：

```go
require.Len(t, opts.ExtraFields, 2)
assert.Equal(t, "stream_preset", opts.ExtraFields[0].Key)
assert.Equal(t, "select", opts.ExtraFields[0].Type)
assert.Equal(t, "encrypt", opts.ExtraFields[0].Condition)
assert.Equal(t, "encrypt_filename", opts.ExtraFields[1].Key)
assert.Equal(t, "bool", opts.ExtraFields[1].Type)
assert.Equal(t, "false", opts.ExtraFields[1].DefaultValue)
```

AlistEncrypt 断言更新（ExtraFields 从 1 → 3）：

```go
require.Len(t, opts.ExtraFields, 3)
assert.Equal(t, "plugin_password", opts.ExtraFields[0].Key)
assert.Equal(t, "encode_filename", opts.ExtraFields[1].Key)
assert.Equal(t, "bool", opts.ExtraFields[1].Type)
assert.Equal(t, "false", opts.ExtraFields[1].DefaultValue)
assert.Equal(t, "enc_type", opts.ExtraFields[2].Key)
assert.Equal(t, "select", opts.ExtraFields[2].Type)
assert.Equal(t, "encrypt", opts.ExtraFields[2].Condition)
```

### Step F1: 补充 i18n key

**文件**: `/workspace/app/encv-mobile/src/composables/useI18n.ts`

在 tasks 命名空间添加：

```typescript
// Video - 编码预设
'tasks.streamPreset': '编码预设',
'tasks.streamPresetHelp': '选择视频流式编码预设',

// Video - 文件名加密开关
'tasks.encryptFilename': '加密文件名',
'tasks.encryptFilenameVideoHelp': '将原始文件名编码为不可读的哈希值，随 ENCV 容器一起存储',

// AlistEncrypt - 密码
'tasks.alistPasswordHelp': '用于内容加密；勾选编码文件名时同时用于文件名编码',

// AlistEncrypt - 文件名编码开关
'tasks.encodeFilename': '编码文件名',
'tasks.encodeFilenameHelp': '使用 MixBase64 算法对文件名进行编码，隐藏原始文件名',

// AlistEncrypt - 编码算法
'tasks.encType': '编码算法',
'tasks.encTypeHelp': '文件名编码使用的算法类型',
```

### Step F2: NewTaskModal.vue 补充 select + bool 渲染

**文件**: `/workspace/app/encv-mobile/src/components/NewTaskModal.vue`

#### F2.1: import 补充

```typescript
import { IonSelect, IonSelectOption, IonToggle } from '@ionic/vue'
```

#### F2.2: 模板修改（替换 L131-141 区域的 ExtraFields 渲染）

```html
<template v-for="field in extraFlds" :key="field.key">
  <!-- === bool 类型: 开关 === -->
  <ion-item
    v-if="(!field.condition || field.condition === taskType) && field.type === 'bool'"
    lines="none" class="extra-field-item"
  >
    <ion-toggle
      :checked="getExtra(field.key) === 'true' || getExtra(field.key) === true"
      @ionChange="(e: any) => { const v = e.detail.checked ? 'true' : 'false'; emit('updateExtraValue', { key: field.key, value: v }); props.onUpdateExtraValue?.({ key: field.key, value: v }) }"
      :label="t(field.label)"
      justify="space-between"
      class="extra-field-toggle"
    />
    <ion-note v-if="field.help" slot="helper">{{ t(field.help) }}</ion-note>
  </ion-item>

  <!-- === select 类型: 下拉选择 === -->
  <ion-item
    v-else-if="(!field.condition || field.condition === taskType) && field.type === 'select'"
    lines="none" class="extra-field-item"
  >
    <ion-select
      :model-value="getExtra(field.key)"
      @ionChange="(e: any) => { emit('updateExtraValue', { key: field.key, value: e.detail.value }); props.onUpdateExtraValue?.({ key: field.key, value: e.detail.value }) }"
      :label="t(field.label)"
      interface="action-sheet"
      placement="bottom"
      class="extra-field-select"
    >
      <ion-select-option v-for="opt in field.options || []" :key="opt" :value="opt">
        {{ opt }}
      </ion-select-option>
    </ion-select>
    <ion-note v-if="field.help" slot="helper">{{ t(field.help) }}</ion-note>
  </ion-item>

  <!-- === string/password 类型: 文本输入（原有逻辑）=== -->
  <ion-item
    v-else-if="!field.condition || field.condition === taskType"
    lines="none" class="extra-field-item"
  >
    <ion-input
      :type="field.type === 'password' ? 'password' : 'text'"
      :model-value="getExtra(field.key)"
      :placeholder="field.help ? t(field.help) : ''"
      @input="(e: any) => { emit('updateExtraValue', { key: field.key, value: (e.target as HTMLInputElement).value }); props.onUpdateExtraValue?.({ key: field.key, value: (e.target as HTMLInputElement).value }) }"
      class="extra-field-input"
    />
  </ion-item>
</template>
```

---

## 四、验证方案

### 4.1 后端单元测试

```bash
cd /workspace && go test ./internal/v2/plugins/ -run "TestVideoPlugin_GetTaskOptions|TestAlistEncryptPlugin_GetTaskOptions" -v
```

### 4.2 前端单元测试

```bash
cd /workspace/app/encv-mobile && npx vitest run --reporter=verbose
```

### 4.3 API 集成验证

```bash
# Video: 应返回 2 个 extraFields
curl 'http://localhost:2025/api/predict-plugin?path=/storage/emulated/0/test.mp4&taskType=encrypt' \
  | jq '.candidates[] | select(.id=="video") | .taskOptions.extraFields'

# 期望:
# [{key:"stream_preset",type:"select",...}, {key:"encrypt_filename",type:"bool",defaultValue:"false",...}]

# AlistEncrypt: 应返回 3 个 extraFields
curl 'http://localhost:2025/api/predict-plugin?path=/storage/emulated/0/test.bin&taskType=encrypt' \
  | jq '.candidates[] | select(.id=="alist_encrypt") | .taskOptions.extraFields'

# 期望:
# [{key:"plugin_password",type:"password"}, {key:"encode_filename",type:"bool",defaultValue:"false"}, {key:"enc_type",type:"select",...}]
```

### 4.4 UI 手动验证

1. 新建任务 → 输入 `.mp4` 文件 → video 插件：
   - ✅ 「编码预设」下拉框显示（仅 encrypt 模式）
   - ✅ 「加密文件名」开关显示（默认关闭/unchecked）
   - ✅ 切换 decrypt → 两者都消失
2. 新建任务 → 输入任意文件 → alist_encrypt 插件：
   - ✅ 「插件密码」密码框
   - ✅ 「编码文件名」开关（默认关闭）
   - ✅ 「编码算法」下拉框（aesctr/rc4md5/chacha20）
   - ✅ 切换 decrypt → encode_filename 和 enc_type 消失，plugin_password 保留

---

## 五、风险和注意事项

1. **声明侧 vs 执行侧**：本次只补全 `GetTaskOptions()` **声明侧**。执行侧（`PostEncryptProcessor` 中读取 `extraFields["encrypt_filename"]` 并决定是否调用 `GenerateEncryptedBaseName`/`EncodeName`）是后续工作。
2. **`enc_type` 与 `encode_filename` 的依赖关系**：`enc_type` 仅在 `encode_filename=true` 时有实际意义。UI 上两者都显示（condition 都是 encrypt），但执行侧应在 `encode_filename=false` 时忽略 `enc_type`。
3. **bool 值序列化**：ExtraFields 的 value 以 `map[string]string` 传输，bool 用 `"true"`/`"false"` 字符串表示，前端 toggle 的 `@ionChange` 输出需转为字符串。
4. **Video 插件当前无条件编码文件名**是既有行为变更风险点——声明 `encrypt_filename` 默认 false 意味着未来执行侧实现后，默认行为从「编码」变为「不编码」，属于 breaking change，需确认是否接受。
