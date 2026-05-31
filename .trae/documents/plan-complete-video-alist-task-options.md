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

### 1.2 文件名加密机制差异（⚠️ 核心认知）

两个插件都支持加密文件名，但使用完全不同的加密系统：

#### Video 插件 — 容器级文件名加密

```
原始文件名: movie_2025.mp4
    ↓ baseNamer.GenerateEncryptedBaseName("movie_2025.mp4")
加密后基础名: a1b2c3d4
    ↓ 拼接容器扩展名
最终文件名: a1b2c3d4.sccgv

加密原理:
  - 原始文件名存储在 VideoIndex.OriginalFilename 中（KVI 元数据的一部分）
  - 整个 KVI（含 OriginalFilename）+ 加密数据块一起被 AES 加密
  - 密钥来源：全局密码（PasswordStrategy = global）
  - 解密时从 KVI 中恢复原始文件名
  - 文件名加密是 ENCV 容器格式的固有行为，不可关闭
```

**关键代码路径**：
- [container_namer.go:16](/workspace/internal/v2/namer/container_namer.go#L16) — `GenerateEncryptedBaseName()` 生成哈希式名称
- [video.go:276](/workspace/internal/v2/plugins/video/plugin.go#L276) — `OriginalFilename` 写入 VideoIndex
- [video.go:298](/workspace/internal/v2/plugins/video/plugin.go#L298) — KVI 被整体加密

#### AlistEncrypt 插件 — 独立文件名编码

```
原始文件名: secret_data.txt
    ↓ EncodeName("secret_data.txt", password, "aesctr")
编码后文件名: (Base64变体+CRC6校验).bin
    ↓ 存储到磁盘
磁盘文件名: xYzAbCdE123.bin

加密原理:
  - 使用 MixBase64（自定义 Base64 变体）+ CRC6 校验位
  - 密钥来源：独立密码 plugin_password（PasswordStrategy = independent）
  - 通过 PBKDF2(密码, salt=算法名, 1000轮) 派生 MixBase64 字母表
  - 编码后的名字直接作为磁盘文件名（非隐藏元数据）
  - 文件名编码是 Alist 格式的核心行为，不可关闭
```

**关键代码路径**：
- [filename.go:229](/workspace/internal/v2/plugins/alistencrypt/filename.go#L229) — `EncodeName()` 编码函数
- [filename.go:242](/workspace/internal/v2/plugins/alistencrypt/filename.go#L242) — `DecodeName()` 解码函数
- [filename.go:225](/workspace/internal/v2/plugins/alistencrypt/filename.go#L225) — `GetPasswdOutward()` PBKDF2 密钥派生

#### 差异对比表

| 维度 | Video 插件 | AlistEncrypt 插件 |
|------|-----------|-------------------|
| **加密方式** | 随 ENCV 容器 KVI 一起 AES 加密 | 独立 MixBase64 + CRC6 编码 |
| **密钥来源** | 全局密码（global） | 独立密码（plugin_password） |
| **存储位置** | KVI 元数据内部（解密后恢复） | 直接作为磁盘文件名 |
| **可配置性** | 固有行为，无选项 | 固有行为，但 enc_type 可选 |
| **密码策略** | `PasswordGlobal` | `PasswordIndependent` |

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

**❌ 缺失项**：
1. **`ExtraFields` 为空（nil）** — `VideoPluginConfig` 有 `default_stream_preset` 字段（balanced/quality/high_quality），用户无法在创建任务时逐任务选择编码预设
2. 无文件名相关选项（文件名加密是固有行为，但可在 Help 中说明）

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

**❌ 缺失项**：
1. **缺少 `enc_type` ExtraField** — 后端配置有 `enc_type`（aesctr/rc4md5/chacha20），但任务创建时不可选
2. Schema 中有 `algorithm` 字段但 Go 结构体用的是 `enc_type`，命名不一致（需统一为 `enc_type`）

### 1.4 前端渲染链路（已就绪，无需修改核心逻辑）

```
后端 GetTaskOptions()
  → predict-plugin API 返回 candidates[].taskOptions
    → useTaskForm.ts 存储到 taskOptions computed
      → useNewTaskModal.ts 同步到 state.filteredExtraFields
        → NewTaskModal.vue v-for 渲染 ion-input / ion-select（L131-141）
```

前端已完整支持 ExtraFields 的动态渲染，包括：
- `type: 'password'` → 密码输入框 ✅
- `type: 'string'` → 文本输入框 ✅
- `type: 'select'` + `options` → 下拉选择（**需补充 `<ion-select>` 渲染分支**）
- `condition` → 按 encrypt/decrypt 条件显示 ✅
- i18n `t(label)` / `t(help)` 翻译 ✅

---

## 二、需要修改的文件清单

### 2.1 后端修改

| # | 文件路径 | 修改内容 |
|---|---------|---------|
| B1 | `internal/v2/plugins/video/plugin.go` — `GetTaskOptions()` L431-438 | 补充 `ExtraFields`：添加 `stream_preset`（编码预设选择）+ `filename_note`（文件名加密说明） |
| B2 | `internal/v2/plugins/alistencrypt/plugin.go` — `GetTaskOptions()` L235-250 | 补充 `ExtraFields`：追加 `enc_type`（加密算法选择）+ 更新 `plugin_password` 的 Help 说明文件名编码 |
| B3 | `internal/v2/plugins/task_options_test.go` | 更新测试断言以覆盖新增的 ExtraFields |

### 2.2 前端修改

| # | 文件路径 | 修改内容 |
|---|---------|---------|
| F1 | `src/composables/useI18n.ts` | 补充新增的 i18n translation key（streamPreset、encType、filenameNote 等） |
| F2 | `src/components/NewTaskModal.vue` | 补充 `type='select'` 的 `<ion-select>` 渲染分支；import IonSelect/IonSelectOption |

### 2.3 不需要修改的文件

- `src/api/encv.ts` — 类型定义已包含 `options?: string[]` 和 `type: 'select'` ✅
- `src/composables/useTaskForm.ts` — ExtraFields 透传逻辑与字段数量无关 ✅
- `src/composables/useNewTaskModal.ts` — 同上 ✅
- `internal/v2/plugins/interfaces/interfaces.go` — `TaskField` 结构体已支持 `options []string` ✅
- `internal/server/mobile_api.go` — `taskOptionsToGinH()` 已透传 ExtraFields ✅
- `config.schema.json` — 前端 schema 是设置页用的，不影响任务创建表单 ✅

---

## 三、详细实现步骤

### Step B1: Video 插件 — 补充 stream_preset + filename_note ExtraFields

**文件**: `/workspace/internal/v2/plugins/video/plugin.go` 的 `GetTaskOptions()` 方法（L431-438）

将当前实现改为：

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
- **`stream_preset`**（`type: select`, `condition: encrypt`）：
  - 流式预设仅在加密时需要选择，解密时自动检测容器格式
  - 用户从预设列表中选择，覆盖 `VideoPluginConfig.DefaultStreamPreset` 全局默认值
  - 选项与后端 `StreamPreset` 类型一致：`balanced` / `quality` / `high_quality`
  - Key 名 `stream_preset` 与后端配置字段对应，TaskManager 执行时可读取此值
- **文件名加密不需要单独字段**：Video 的文件名加密是 ENCV 容器的固有行为（原始文件名存入 KVI 并随容器整体 AES 加密），用户无法也不需要控制。在 `stream_preset.Help` 和全局密码提示中已隐含说明。

### Step B2: AlistEncrypt 插件 — 补充 enc_type ExtraField + 更新 Help 文案

**文件**: `/workspace/internal/v2/plugins/alistencrypt/plugin.go` 的 `GetTaskOptions()` 方法（L235-250）

将当前实现改为：

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
                Key:          "enc_type",
                Label:        "tasks.encType",
                Type:         "select",
                Required:     false,
                DefaultValue: "aesctr",
                Help:         "tasks.encTypeHelp",
                Options:      []string{"aesctr", "rc4md5", "chacha20"},
                Condition:     "",
            },
        },
    }
}
```

设计决策：
- **`plugin_password` Help 更新**：从 `"tasks.pluginPasswordHelp"` 改为 `"tasks.alistPasswordHelp"`，文案应体现「此密码同时用于文件名编码」这一 Alist 特性
- **`enc_type`**（`type: select`, 无 condition）：
  - 加密和解码都需要知道算法类型（解码时用相同算法反向操作）
  - 暴露全部 3 个选项（`aesctr`/`rc4md5`/`chacha20`），而非仅 `aesctr`
  - `DefaultValue: "aesctr"` 与 `AlistEncryptPluginConfig.EncType` 默认值一致
  - Options 列表来自 `supportedEncTypes` 常量或硬编码已知值
- **文件名编码不需要单独字段**：AlistEncrypt 的文件名编码是其核心固有行为（`EncodeName()` 自动执行），使用 `plugin_password` + `enc_type` 作为参数。在 `enc_type.Help` 和 `plugin_password.Help` 中说明即可。

### Step B3: 更新后端测试

**文件**: `/workspace/internal/v2/plugins/task_options_test.go`

更新 `TestVideoPlugin_GetTaskOptions` 断言，增加 ExtraFields 验证：

```go
func TestVideoPlugin_GetTaskOptions(t *testing.T) {
    // ...existing setup code...
    assert.Equal(t, pluginInterfaces.PasswordGlobal, opts.PasswordStrategy)
    assert.True(t, opts.SupportVersionSelect)
    assert.NotEmpty(t, opts.SupportedVersions)

    // 新增: 验证 ExtraFields
    require.Len(t, opts.ExtraFields, 1, "video plugin should have 1 extra field")
    assert.Equal(t, "stream_preset", opts.ExtraFields[0].Key)
    assert.Equal(t, "select", opts.ExtraFields[0].Type)
    assert.False(t, opts.ExtraFields[0].Required)
    assert.Equal(t, "balanced", opts.ExtraFields[0].DefaultValue)
    assert.Contains(t, opts.ExtraFields[0].Options, "balanced")
    assert.Contains(t, opts.ExtraFields[0].Options, "high_quality")
    assert.Equal(t, "encrypt", opts.ExtraFields[0].Condition)
}
```

更新 `TestAlistEncryptPlugin_GetTaskOptions` 断言：

```go
func TestAlistEncryptPlugin_GetTaskOptions(t *testing.T) {
    // ...existing setup code...
    assert.Equal(t, pluginInterfaces.PasswordIndependent, opts.PasswordStrategy)
    assert.False(t, opts.SupportVersionSelect)

    // 更新: 验证 ExtraFields 从 1 个增加到 2 个
    require.Len(t, opts.ExtraFields, 2, "alist_encrypt should have 2 extra fields")

    // 第一个字段: plugin_password
    assert.Equal(t, "plugin_password", opts.ExtraFields[0].Key)
    assert.Equal(t, "password", opts.ExtraFields[0].Type)

    // 第二个字段: enc_type (新增)
    assert.Equal(t, "enc_type", opts.ExtraFields[1].Key)
    assert.Equal(t, "select", opts.ExtraFields[1].Type)
    assert.Equal(t, "aesctr", opts.ExtraFields[1].DefaultValue)
    assert.Contains(t, opts.ExtraFields[1].Options, "aesctr")
    assert.Contains(t, opts.ExtraFields[1].Options, "chacha20")
}
```

### Step F1: 补充前端 i18n key

**文件**: `/workspace/app/encv-mobile/src/composables/useI18n.ts`

在 tasks 命名空间下添加以下 key：

```typescript
// Video 插件 - 编码预设
'tasks.streamPreset': '编码预设',
'tasks.streamPresetHelp': '选择视频流式编码预设。文件名将随容器一起加密存储',

// AlistEncrypt 插件 - 独立密码和算法
'tasks.alistPasswordHelp': '此密码同时用于内容加密和文件名编码',
'tasks.encType': '编码算法',
'tasks.encTypeHelp': '选择文件名编码算法。不同算法产生不同的编码结果',
```

注意：
- `tasks.pluginPassword` 保持不变（label 复用已有 key）
- `tasks.pluginPasswordHelp` 替换为新的 `tasks.alistPasswordHelp`（更具体地描述 Alist 的双用途密码特性）
- Help 文案中明确提及「文件名」二字，让用户理解两套系统的差异

### Step F2: NewTaskModal.vue 补充 ion-select 渲染

**文件**: `/workspace/app/encv-mobile/src/components/NewTaskModal.vue`

#### F2.1: import 补充

在 `<script setup>` 的 import 区域增加：

```typescript
import { IonSelect, IonSelectOption } from '@ionic/vue'
```

并在 components 注册中添加（如使用 options API 则在 components 对象中添加）。

#### F2.2: 模板修改（L131-141 区域）

将当前的单一 `ion-input` 渲染替换为按 type 分支的条件渲染：

```html
<!-- ExtraFields 动态渲染 -->
<template v-for="field in extraFlds" :key="field.key">
  <!-- === select 类型: 下拉选择 === -->
  <ion-item
    v-if="(!field.condition || field.condition === taskType) && field.type === 'select'"
    lines="none" class="extra-field-item"
  >
    <ion-select
      :model-value="getExtra(field.key)"
      @ionChange="onExtraSelectChange(field, $event)"
      :label="t(field.label)"
      interface="action-sheet"
      placement="bottom"
      class="extra-field-select"
    >
      <ion-select-option
        v-for="opt in field.options || []"
        :key="opt"
        :value="opt"
      >
        {{ opt }}
      </ion-select-option>
    </ion-select>
    <ion-note v-if="field.help" slot="helper">{{ t(field.help) }}</ion-note>
  </ion-item>

  <!-- === string/password 类型: 文本/密码输入（原有逻辑）=== -->
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

#### F2.3: script 补充 onExtraSelectChange handler

```typescript
function onExtraSelectChange(field: any, event: CustomEvent) {
  const value = event.detail.value
  emit('updateExtraValue', { key: field.key, value })
  props.onUpdateExtraValue?.({ key: field.key, value })
}
```

---

## 四、验证方案

### 4.1 后端单元测试

```bash
cd /workspace && go test ./internal/v2/plugins/ -run "TestVideoPlugin_GetTaskOptions|TestAlistEncryptPlugin_GetTaskOptions" -v
```

预期：所有新断言通过（ExtraFields 数量、Key/Type/Options/Condition/DefaultValue 全部匹配）。

### 4.2 前端单元测试

```bash
cd /workspace/app/encv-mobile && npx vitest run --reporter=verbose
```

### 4.3 集成验证（手动检查 predict-plugin API 响应）

启动前后端后调用：

```bash
# Video 插件预测
curl 'http://localhost:2025/api/predict-plugin?path=/storage/emulated/0/test.mp4&taskType=encrypt' | jq '.candidates[] | select(.id=="video") | .taskOptions'

# 期望输出包含:
#   extraFields: [{ key:"stream_preset", type:"select", options:["balanced","quality","high_quality"], condition:"encrypt" }]

# AlistEncrypt 插件预测
curl 'http://localhost:2025/api/predict-plugin?path=/storage/emulated/0/test.bin&taskType=encrypt' | jq '.candidates[] | select(.id=="alist_encrypt") | .taskOptions'

# 期望输出包含:
#   extraFields: [
#     { key:"plugin_password", type:"password" },
#     { key:"enc_type", type:"select", options:["aesctr","rc4md5","chacha20"] }
#   ]
```

### 4.4 UI 手动验证

1. 打开新建任务弹窗
2. 输入 `.mp4` 视频文件路径 → 预测到 video 插件：
   - ✅ 看到「编码预设」下拉框（仅 encrypt 模式显示）
   - ✅ 选项：balanced / quality / high_quality
   - ✅ 默认选中 balanced
   - ✅ 切换到 decrypt → 编码预设消失
3. 输入任意文件路径 → 预测到 alist_encrypt 插件：
   - ✅ 看到「插件密码」密码框
   - ✅ 看到「编码算法」下拉框（选项：aesctr / rc4md5 / chacha20）
   - ✅ 默认选中 aesctr
   - ✅ 密码策略提示：「此插件使用独立密码」
   - ✅ Help 文案提及文件名编码

---

## 五、风险和注意事项

1. **`stream_preset` 是 UI 声明侧补全**：后端 `TaskManager` 在执行加密任务时需读取 `extraFields["stream_preset"]` 并注入 `VideoPlugin` 实例。本次只补全**声明侧**（`GetTaskOptions`），执行侧透传如未实现需后续跟进。
2. **`enc_type` 三选一**：暴露 `aesctr`/`rc4md5`/`chacha20` 三个选项。虽然当前默认只有 aesctr 经过充分测试，但 select 类型可防止非法输入并为扩展预留。
3. **i18n key 统一使用 `tasks.` 前缀**：所有 label/help 都走 i18n，不硬编码中文。
4. **文件名加密差异已在 Help 文案中体现**：Video 的 Help 说「文件名将随容器一起加密存储」，AlistEncrypt 的 Help 说「密码同时用于内容加密和文件名编码」，用户能直观感知两套系统的区别。
5. **`condition: "encrypt"` 仅用于 `stream_preset`**：因为解密时编码预设无意义（容器格式自描述）。`enc_type` 和 `plugin_password` 无 condition（加解密都需要）。
