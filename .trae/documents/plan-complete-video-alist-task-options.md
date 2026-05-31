# 计划：按规范补全 video 插件和 alist_encrypt 加密任务配置项

## 一、现状分析

### 1.1 TaskOptions 声明机制

每个插件通过 `GetTaskOptions() -> TaskOptions` 向前端声明任务创建表单字段：

| 字段 | 含义 |
|------|------|
| `PasswordStrategy` | 密码策略：global / independent / none |
| `SupportVersionSelect` | 是否支持容器版本选择 |
| `ExtraFields` | 插件自定义额外输入字段（声明式，前端自动渲染） |

前端 `NewTaskModal.vue` 根据 `ExtraFields[]` 动态渲染表单。

### 1.2 两个插件当前的真实文件名处理

#### Video 插件 — 容器命名（非加密）

```
原始: movie_2025.mp4
  ↓ GenerateEncryptedBaseName() [container_namer.go:16]
中间: movie_2025.4pm          ← 只是反转扩展名！
  ↓ GenerateMainChunkName()
输出: movie_2025.4pm.sccgv     ← 拼接容器扩展名
```

[container_namer.go:16-27](/workspace/internal/v2/namer/container_namer.go#L16-L27) 的实际代码：

```go
func (n *DefaultBaseNamer) GenerateEncryptedBaseName(originalFilename string) string {
    base := filepath.Base(originalFilename)
    ext := filepath.Ext(base)
    cleanBaseName := strings.TrimSuffix(base, ext)
    if ext == "" { return cleanBaseName }
    reversedExt := generateReversedExt(ext)   // ← "mp4" → "4pm"
    return fmt.Sprintf("%s.%s", cleanBaseName, reversedExt)
}
```

函数名里的 `Encrypted` 指 ENCV **容器**命名空间，不是密码学操作。

#### AlistEncrypt 插件 — 只换扩展名

```
原始: secret.txt
  ↓ RenameToFinalEncrypted() [encryptor.go:67]
输出: secret.bin              ← strings.TrimSuffix + 拼接 suffix
```

[encryptor.go:67-77](/workspace/internal/v2/plugins/alistencrypt/encryptor.go#L67-L77) 的实际代码：

```go
func RenameToFinalEncrypted(tempPath string, originalFilename string, outputDir string, suffix string) (string, error) {
    baseName := strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))
    finalName := baseName + suffix       // "secret" + ".bin"
    finalPath := filepath.Join(outputDir, finalName)
    os.Rename(tempPath, finalPath)
    return finalPath, nil
}
```

**不调用** `EncodeName()`。`EncodeName()/DecodeName()` ([filename.go:229](/workspace/internal/v2/plugins/alistencrypt/filename.go#L229)) 仅被以下 API 端点使用：
- `/api/alist-encode/encode-filename` — 前端手动触发编码
- `/api/alist-encrypt/decode-filename` — 前端显示转换

### 1.3 当前 TaskOptions 缺失项

#### Video — `[plugin.go:431-438](/workspace/internal/v2/plugins/video/plugin.go#L431-L438)`

```go
// 当前：ExtraFields 为空
func (p *VideoPlugin) GetTaskOptions() pluginInterfaces.TaskOptions {
    return pluginInterfaces.TaskOptions{
        PasswordStrategy:     pluginInterfaces.PasswordGlobal,
        SupportVersionSelect: true,
        SupportedVersions:    p.SupportedContainerVersions(),
        DefaultVersion:       p.DefaultContainerVersion(),
        // ❌ 无 ExtraFields
    }
}
```

**缺失**：
- `stream_preset`（select）— 编码预设：balanced/quality/high_quality，对应 `VideoPluginConfig.DefaultStreamPreset`

#### AlistEncrypt — `[plugin.go:235-250](/workspace/internal/v2/plugins/alistencrypt/plugin.go#L235-L250)`

```go
// 当前：只有 plugin_password 一个字段
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

**缺失**：
- `enc_type`（select）— 文件名编码算法选项：aesctr/rc4md5/chacha20，对应 `AlistEncryptPluginConfig.EncType`

---

## 二、修改清单

| # | 文件 | 改动 |
|---|------|------|
| B1 | `internal/v2/plugins/video/plugin.go` GetTaskOptions() | 补充 `stream_preset` ExtraField |
| B2 | `internal/v2/plugins/alistencrypt/plugin.go` GetTaskOptions() | 补充 `enc_type` ExtraField |
| B3 | `internal/v2/plugins/task_options_test.go` | 更新断言 |
| F1 | `src/composables/useI18n.ts` | 补充 i18n key |
| F2 | `src/components/NewTaskModal.vue` | 补充 `<ion-select>` 渲染分支 |

---

## 三、实现步骤

### Step B1: Video — 补 stream_preset

```go
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
```

- condition=`encrypt`：仅加密时需选预设，解密时自动检测容器格式
- DefaultValue=`balanced`：与 `VideoPluginConfig.DefaultStreamPreset` 一致

### Step B2: AlistEncrypt — 补 enc_type

```go
ExtraFields: []pluginInterfaces.TaskField{
    { Key: "plugin_password", Label: "tasks.pluginPassword", Type: "password", Required: false, Help: "tasks.pluginPasswordHelp", Condition: "" },
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
```

- 无 condition：加解密都可能需要知道算法类型（用于 API 层 decode-filename 等）
- DefaultValue=`aesctr`：与 `AlistEncryptPluginConfig.EncType` 默认值一致

### Step B3: 测试更新

```go
// Video: ExtraFields 从 0 → 1
require.Len(t, opts.ExtraFields, 1)
assert.Equal(t, "stream_preset", opts.ExtraFields[0].Key)
assert.Equal(t, "select", opts.ExtraFields[0].Type)
assert.Contains(t, opts.ExtraFields[0].Options, "balanced")
assert.Equal(t, "encrypt", opts.ExtraFields[0].Condition)

// AlistEncrypt: ExtraFields 从 1 → 2
require.Len(t, opts.ExtraFields, 2)
assert.Equal(t, "plugin_password", opts.ExtraFields[0].Key)
assert.Equal(t, "enc_type", opts.ExtraFields[1].Key)
assert.Equal(t, "select", opts.ExtraFields[1].Type)
```

### Step F1: i18n key

```typescript
'tasks.streamPreset': '编码预设',
'tasks.streamPresetHelp': '选择视频流式编码预设',
'tasks.encType': '编码算法',
'tasks.encTypeHelp': '选择编码算法类型',
```

### Step F2: NewTaskModal.vue — 补 select 渲染

import 补充：`IonSelect`, `IonSelectOption`

模板中 ExtraFields 区域按 type 分支：

```html
<template v-for="field in extraFlds" :key="field.key">
  <!-- select -->
  <ion-item v-if="(!field.condition || field.condition === taskType) && field.type === 'select'" lines="none">
    <ion-select :model-value="getExtra(field.key)"
      @ionChange="(e:any)=>{emit('updateExtraValue',{key:field.key,value:e.detail.value});props.onUpdateExtraValue?.({key:field.key,value:e.detail.value})}"
      :label="t(field.label)" interface="action-sheet" placement="bottom">
      <ion-select-option v-for="opt in field.options||[]" :key="opt" :value="opt">{{ opt }}</ion-select-option>
    </ion-select>
    <ion-note v-if="field.help" slot="helper">{{ t(field.help) }}</ion-note>
  </ion-item>

  <!-- string/password (原有) -->
  <ion-item v-else-if="!field.condition || field.condition === taskType" lines="none">
    <ion-input :type="field.type==='password'?'password':'text'" ... />
  </ion-item>
</template>
```

---

## 四、验证

```bash
# 后端测试
cd /workspace && go test ./internal/v2/plugins/ -run "TestVideoPlugin_GetTaskOptions|TestAlistEncryptPlugin_GetTaskOptions" -v

# 前端测试
cd /workspace/app/encv-mobile && npx vitest run --reporter=verbose

# API 验证
curl 'http://localhost:2025/api/predict-plugin?path=/storage/emulated/0/test.mp4&taskType=encrypt' \
  | jq '.candidates[]|select(.id=="video")|.taskOptions.extraFields'
# → [{key:"stream_preset",type:"select",options:["balanced","quality","high_quality"],condition:"encrypt"}]

curl 'http://localhost:2025/api/predict-plugin?path=/storage/emulated/0/test.bin&taskType=encrypt' \
  | jq '.candidates[]|select(.id=="alist_encrypt")|.taskOptions.extraFields'
# → [{key:"plugin_password",type:"password"},{key:"enc_type",type:"select",options:["aesctr","rc4md5","chacha20"]}]
```
