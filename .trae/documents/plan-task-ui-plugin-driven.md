# 任务创建界面插件化适配计划（修订版）

## 一、问题诊断（含用户新增反馈）

### 1.1 当前 Tasks.vue 任务模态框的硬编码问题

| 问题 | 位置 | 详情 |
|------|------|------|
| **二级密码字段命名误导** | 原 L292 `newTaskPassword` | 变量名暗示"密码"，实际设计意图是**二级/L2 覆盖密码**，与插件独立密码(L1)/全局密码(L0)不是同一层级 |
| **容器版本选择硬编码** | 原 L188 `v-if="newTaskType === 'encrypt'"` | Video 插件支持版本选择，Alist-Encrypt 不支持，但前端不知道 |
| **无插件感知** | 整个模态框 | 不知道用户选的文件会由哪个插件处理 |

### 1.2 密码层级与当前实现的矛盾（⚠️ 核心问题）

**三层密码层级定义**：

```
L0: 全局密码 (config.password)           → 系统级默认，所有 PasswordGlobal 插件使用
L1: 插件独立密码 (plugin.default_password) → 与全局密码同级，PasswordIndependent 插件使用
    └─ L1a: 插件设置中的默认值（配置页填写）
    └─ L1b: 任务创建时指定的覆盖值（extraFields.plugin_password）
L2: 二级/任务密码 (per-task override)      → 对 L0 或 L1 的临时覆盖，所有插件通用
```

**当前代码的致命缺陷**：

#### 缺陷 A：Alist-Encrypt 声明 Independent 但仍 fallback 全局密码

[alistencrypt/plugin.go L104-106](internal/v2/plugins/alistencrypt/plugin.go#L104-L106) 的 `Initialize()`：
```go
if p.settings.DefaultPassword == "" && p.cfg.Password != "" {
    p.settings.DefaultPassword = p.cfg.Password  // ← 自动复制全局密码到插件！
}
```

[alistencrypt/plugin.go L251-258](internal/v2/plugins/alistencrypt/plugin.go#L251-L258) 的 `resolvePassword()`：
```go
func (p *AlistEncryptPlugin) resolvePassword() string {
    if p.settings.DefaultPassword != "" { return p.settings.DefaultPassword }  // L1a
    if p.cfg.Password != "" { return p.cfg.Password }                        // ← 仍 fallback 到 L0！
    return ""
}
```

**矛盾**：插件声明了 `PasswordIndependent`（不用全局密码），但代码在两个地方都偷偷用了全局密码。

#### 缺陷 B：前端把 L1 和 L2 合并为一个字段发送

[Tasks.vue L596](app/encv-mobile/src/views/Tasks.vue#L596)：
```typescript
(extra.plugin_password || extra.secondary_password) as string | undefined
```

`plugin_password`(L1b) 和 `secondary_password`(L2) 被 `||` 合并成单一 `password` 参数发给后端。两者语义完全不同：
- `plugin_password` = 插件独立密码的任务级指定（仅 PasswordIndependent 插件）
- `secondary_password` = 对默认密码（无论 L0 还是 L1）的临时覆盖（所有插件）

#### 缺陷 C：后端任务创建只接受单一 password 字段

[mobile_api.go L200-206](internal/server/mobile_api.go#L200-L206)：
```go
var req struct {
    Type       string `json:"type"`
    SourcePath string `json:"sourcePath"`
    TargetPath string `json:"targetPath,omitempty"`
    Password   string `json:"password,omitempty"`     // ← 只有一个 password！
    Version    int    `json:"version,omitempty"`
    // ❌ 缺少 ExtraFields map
}
```

无法传递 `plugin_password` 等插件声明的额外字段。

#### 缺陷 D：任务执行时密码覆盖机制对 Independent 插件错误

[task_manager.go L374-381](internal/service/task_manager.go#L374-L381) 的 `getConfigForTask()`：
```go
func (tm *TaskManager) getConfigForTask(task *MobileTask, ctx context.Context) context.Context {
    if task.Password != "" {
        cfgCopy := *tm.cfg
        cfgCopy.Password = task.Password  // ← 无条件覆盖全局密码
        return config.NewContext(ctx, &cfgCopy)
    }
    return config.NewContext(ctx, tm.cfg)
}
```

对于 PasswordIndependent 插件，用户传入的是 L1b(plugin_password)，但这里把它写入了 `cfg.Password`(L0 位置)，导致 `resolvePassword()` 中 `p.cfg.Password` 非空，独立密码被污染。

### 1.3 后端 `/api/plugins` 返回信息不足

当前返回缺少密码策略声明、版本支持能力等。（Step 3 已实现）

### 1.4 核心矛盾总结

```
视频插件任务需要：容器版本选择 + 使用全局密码(L0) + 可选L2覆盖
Alist-Encrypt 任务需要：独立密码输入(L1b) + 不使用全局密码 + 可选L2覆盖
→ 但前端对两者一视同仁，后端只有单字段 password 通道
→ 且 Alist-Encrypt 声明了 Independent 却仍在代码中 fallback 到全局密码
```

---

## 二、设计原则

> **插件声明式，前端委托渲染，后端管道完整。**

1. **插件声明驱动 UI**：`GetTaskOptions()` 声明密码策略 + 额外字段，前端动态渲染
2. **密码层级严格分离**：L0/L1/L2 在整个管道中（前端→API→执行）保持独立，不合并
3. **PasswordIndependent 语义完整性**：声明独立的插件必须在整个管道中真正独立，不 fallback 到全局密码
4. **防御性编程铁律**：不硬编码任何运行时数据

---

## 三、实施方案

> **Step 1-8 已完成**（类型定义、插件实现、API 端点、前端 composable、Tasks.vue 重构、i18n、CSS）
>
> **以下从 Step 9 开始是修订内容**，修复上述缺陷 A/B/C/D。

### ✅ Step 1-8（已完成，不再重复）

- Step 1: 后端 `PasswordStrategy`/`TaskOptions`/`TaskField` 类型 + `GetTaskOptions()` 接口
- Step 2: 各插件实现 `GetTaskOptions()`（video=global+版本, alist_encrypt=independent+密码字段, 其他=global）
- Step 3: 增强 `/api/plugins` 返回 `taskOptions`
- Step 4: 新增 `/api/tasks/predict-plugin` 端点
- Step 5: 前端 API 层新增类型和 `predictPlugin()` 函数
- Step 6: 新建 `useTaskForm` composable
- Step 7: Tasks.vue 模态框改为声明式渲染
- Step 8: i18n 新增翻译 key

### 🔧 Step 9: 后端 — 任务创建 API 支持 ExtraFields

**问题修复**: 缺陷 C — 后端只接受单一 password 字段

**文件**: `internal/server/mobile_api.go` — `handleCreateTaskGin()`

```go
func (s *Server) handleCreateTaskGin(c *gin.Context) {
    var req struct {
        Type        string            `json:"type"`
        SourcePath  string            `json:"sourcePath"`
        TargetPath  string            `json:"targetPath,omitempty"`
        Password    string            `json:"password,omitempty"`      // L2 二级密码
        Version     int               `json:"version,omitempty"`
        ExtraFields map[string]string `json:"extraFields,omitempty"`   // ← 新增：插件声明的额外字段
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
        return
    }

    slog.Info("API: create task", "type", req.Type, "source", req.SourcePath,
        "target", req.TargetPath, "version", req.Version,
        "hasExtraFields", len(req.ExtraFields) > 0)

    task := s.mobileSvc.GetTaskManager().CreateWithExtras(
        req.Type, req.SourcePath, req.TargetPath, req.Password, req.Version, req.ExtraFields,
    )

    c.JSON(http.StatusCreated, task)
}
```

### 🔧 Step 10: 后端 — TaskManager.CreateWithExtras 携带额外字段

**文件**: `internal/service/task_manager.go`

**10a. MobileTask 结构体增加 ExtraFields**

```go
type MobileTask struct {
    ID               string            `json:"id"`
    Type             string            `json:"type"`
    SourcePath       string            `json:"sourcePath"`
    TargetPath       string            `json:"targetPath,omitempty"`
    Password         string            `json:"password,omitempty"`          // L2 二级密码
    ExtraFields      map[string]string `json:"extraFields,omitempty"`       // ← 新增
    Status           string            `json:"status"`
    Progress         int               `json:"progress"`
    // ... 其余字段不变
}
```

**10b. 新建 CreateWithExtras 方法**

```go
func (tm *TaskManager) CreateWithExtras(taskType, sourcePath, targetPath, password string, version int, extras map[string]string) *MobileTask {
    task := tm.Create(taskType, sourcePath, targetPath, password, version)
    task.ExtraFields = extras
    return task
}
```

原 `Create()` 方法签名不变（向后兼容），`CreateWithExtras` 在其基础上补充 `ExtraFields`。

**10c. 持久化兼容**

`saveTasks()` / `loadTasks()` 已使用 JSON 序列化，新增 `ExtraFields` 字段会自动包含（map[string]string 为 JSON object），无需修改序列化逻辑。旧数据该字段为 null，安全向下兼容。

### 🔧 Step 11: 后端 — Alist-Encrypt 真正实现 Independent 密码策略

**问题修复**: 缺陷 A — 声明 Independent 但仍 fallback 全局密码

**文件**: `internal/v2/plugins/alistencrypt/plugin.go`

**11a. Initialize() 不再自动复制全局密码**

```go
func (p *AlistEncryptPlugin) Initialize(ctx context.Context) error {
    // ... existing init code ...

    // ❌ 删除这行：
    // if p.settings.DefaultPassword == "" && p.cfg.Password != "" {
    //     p.settings.DefaultPassword = p.cfg.Password
    // }

    // ✅ 替换为：Independent 策略下不继承全局密码
    // DefaultPassword 为空时就是空，由任务创建时通过 ExtraFields 指定或报错
    _ = p.cfg  // 保留 cfg 引用以备其他用途（如日志、路径解析）

    return nil
}
```

**11b. resolvePassword() 接受任务级参数**

```go
// resolvePasswordWithOverride 解析密码，支持任务级覆盖
// taskPassword: L2 二级密码（对所有策略都有效的高级覆盖）
// pluginPassword: L1b 插件独立密码（仅 PasswordIndependent 插件使用）
func (p *AlistEncryptPlugin) resolvePasswordWithOverride(taskPassword, pluginPassword string) string {
    // L2 最高优先级：二级密码覆盖一切
    if taskPassword != "" {
        return taskPassword
    }
    // L1b: 任务指定的插件独立密码
    if pluginPassword != "" {
        return pluginPassword
    }
    // L1a: 插件设置中的默认独立密码
    if p.settings.DefaultPassword != "" {
        return p.settings.DefaultPassword
    }
    // ❌ 不再 fallback 到全局密码（Independent 策略核心语义）
    return ""
}
```

保留原 `resolvePassword()` 作为无任务参数时的降级调用（向后兼容非任务场景）：

```go
func (p *AlistEncryptPlugin) resolvePassword() string {
    return p.resolvePasswordWithOverride("", "")
}
```

### 🔧 Step 12: 后端 — 任务执行时传递 ExtraFields 给插件

**问题修复**: 缺陷 D — `getConfigForTask()` 对 Independent 插件错误地覆盖 cfg.Password

**文件**: `internal/service/task_manager.go`

**12a. 新增 Plugin 接口方法（可选接口，类似 ContentVerifier）**

**文件**: `internal/v2/plugins/interfaces/interfaces.go`

```go
// TaskPasswordResolver 定义插件自定义密码解析能力
// 对于 PasswordIndependent 插件，需要接收 ExtraFields 中的 plugin_password
// 对于 PasswordGlobal 插件，此接口不需要实现（使用默认行为）
type TaskPasswordResolver interface {
    ResolveTaskPassword(taskPassword string, extraFields map[string]string) string
}
```

**12b. Alist-Encrypt 实现 TaskPasswordResolver**

**文件**: `internal/v2/plugins/alistencrypt/plugin.go`

```go
func (p *AlistEncryptPlugin) ResolveTaskPassword(taskPassword string, extraFields map[string]string) string {
    pluginPassword := extraFields["plugin_password"]
    return p.resolvePasswordWithOverride(taskPassword, pluginPassword)
}
```

**12c. processEncrypt / processDecrypt 中使用新解析方式**

修改 [task_manager.go](internal/service/task_manager.go) 中的加密/解密流程：

```go
func (tm *TaskManager) processEncrypt(task *MobileTask, absPath string) {
    // ... existing validation code ...

    cfgCtx := tm.getConfigForTask(task, ctx)

    plugin, err := plugins.FindEncryptingPlugin(absPath)
    if err != nil { /* ... */ }

    // ★ 新增：如果插件实现了 TaskPasswordResolver，使用它来解析密码
    var effectivePassword string
    if resolver, ok := plugin.(pluginInterfaces.TaskPasswordResolver); ok {
        effectivePassword = resolver.ResolveTaskPassword(task.Password, task.ExtraFields)
    } else {
        // 默认行为（PasswordGlobal 插件）：L2 > L0
        effectivePassword = task.Password
        if effectivePassword == "" {
            effectivePassword = tm.cfg.Password
        }
    }

    if effectivePassword == "" {
        tm.failTask(taskID, "encryption requires a password")
        return
    }

    // 将解析后的密码注入上下文（替代原来的 cfgCopy.Password 方式）
    passwordCtx := tm.getPasswordContext(cfgCtx, effectivePassword)
    // ... rest of encrypt using passwordCtx ...
}
```

**12d. getPasswordContext 辅助方法**

```go
func (tm *TaskManager) getPasswordContext(ctx context.Context, password string) context.Context {
    if password != "" {
        cfgCopy := *tm.cfg
        cfgCopy.Password = password
        return config.NewContext(ctx, &cfgCopy)
    }
    return ctx
}
```

同理修改 `processDecrypt()`。

### 🔧 Step 13: 前端 — 分离 L1 和 L2 密码发送

**问题修复**: 缺陷 B — 前端把 plugin_password 和 secondary_password 合并

**文件**: `app/encv-mobile/src/views/Tasks.vue` — `handleCreateTask()`

**13a. 修改 createTask API 调用**

当前（错误）：
```typescript
await createTask(
  newTaskType.value,
  newTaskPath.value,
  newTaskTargetPath.value || undefined,
  (extra.plugin_password || extra.secondary_password) as string | undefined,  // ❌ 合并
  taskOptions.value?.supportVersionSelect ? newTaskVersion.value : undefined
)
```

修正后：
```typescript
const extra = getExtraPayload()
await createTask(
  newTaskType.value,
  newTaskPath.value,
  newTaskTargetPath.value || undefined,
  secondaryPassword.value || undefined,                    // ✅ L2 单独传递
  taskOptions.value?.supportVersionSelect ? newTaskVersion.value : undefined,
  extra                                                // ✅ ExtraFields（含 L1b plugin_password）单独传递
)
```

**13b. 更新 createTask 函数签名**

**文件**: `app/encv-mobile/src/api/encv.ts`

```typescript
export async function createTask(
  type: TaskType,
  sourcePath: string,
  targetPath?: string,
  password?: string,           // L2 二级密码
  version?: number,
  extraFields?: Record<string, string>,  // ← 新增：插件额外字段
): Promise<EncvTask> {
  const baseUrl = getApiBaseUrl()
  const response = await fetch(`${baseUrl}/api/tasks`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ type, sourcePath, targetPath, password, version, extraFields }),
  })
  // ...
}
```

**13c. useTaskForm.getExtraPayload() 移除 secondaryPassword**

**文件**: `app/encv-mobile/src/composables/useTaskForm.ts`

```typescript
function getExtraPayload(): Record<string, string> {   // ← 返回类型改为 string（不是 unknown）
  const payload: Record<string, string> = {}
  for (const [k, v] of Object.entries(extraValues.value)) {
    if (v !== undefined && v !== '') payload[k] = v
  }
  // ❌ 删除: if (secondaryPassword.value) payload['secondary_password'] = secondaryPassword.value
  // secondaryPassword 由 handleCreateTask 单独作为 password 参数传递
  return payload
}
```

### 🔧 Step 14: 前端 — UI 层密码字段标签精确化

**文件**: `app/encv-mobile/src/views/Tasks.vue` 模板 + `useI18n.ts`

确保三个密码输入区域的标签清晰区分层级：

| 字段 | 显示条件 | 标签(i18n key) | 占位符 | badge |
|------|---------|---------------|--------|-------|
| 插件独立密码 | `taskOptions.passwordStrategy === 'independent'` | `tasks.pluginPassword` | `tasks.pluginPasswordHelp` | 无（来自 ExtraFields 动态渲染） |
| 二级密码 | 始终显示 | `tasks.overridePassword` | `tasks.overridePasswordHelp` | `tasks.optional` |

i18n 更新（重命名/新增）：
```
// 重命名（更准确）
'tasks.secondaryPassword' → 'tasks.overridePassword': '覆盖密码（可选）'
'tasks.secondaryPasswordHelp' → 'tasks.overridePasswordHelp': '留空则使用默认密码（全局密码或插件独立密码）'

// 保持不变
'tasks.pluginPassword': '插件密码'
'tasks.pluginPasswordHelp': '留空则使用插件设置的默认密码'
'tasks.optional': '可选'
```

### Step 15: 测试

#### 15a. 后端测试

**文件**: `internal/v2/plugins/task_options_test.go`（新建）

```go
package plugins_test

import (
    "testing"
    pluginInterfaces "github.com/Soltus/encv-go/internal/v2/plugins/interfaces"
    "github.com/Soltus/encv-go/internal/v2/plugins"
    "github.com/stretchr/testify/assert"
)

func TestVideoPlugin_GetTaskOptions(t *testing.T) {
    // 需要 initPluginsWithSettings 因为 SupportedContainerVersions 依赖初始化
    initPluginsWithSettings(t, nil)
    p := getPluginByName("video")
    require.NotNil(t, p)
    opts := p.GetTaskOptions()
    assert.Equal(t, pluginInterfaces.PasswordGlobal, opts.PasswordStrategy)
    assert.True(t, opts.SupportVersionSelect)
    assert.NotEmpty(t, opts.SupportedVersions)
    assert.Empty(t, opts.ExtraFields)
}

func TestAlistEncryptPlugin_GetTaskOptions(t *testing.T) {
    initPluginsWithSettings(t, nil)
    p := getPluginByName("alist_encrypt")
    require.NotNil(t, p)
    opts := p.GetTaskOptions()
    assert.Equal(t, pluginInterfaces.PasswordIndependent, opts.PasswordStrategy)
    assert.False(t, opts.SupportVersionSelect)
    assert.Len(t, opts.ExtraFields, 1)
    assert.Equal(t, "plugin_password", opts.ExtraFields[0].Key)
    assert.Equal(t, "password", opts.ExtraFields[0].Type)
}

func TestOtherPlugins_DefaultToGlobal(t *testing.T) {
    initPluginsWithSettings(t, nil)
    for _, name := range []string{"text", "audio", "image", "pdf", "wps"} {
        p := getPluginByName(name)
        require.NotNil(t, p)
        assert.Equal(t, pluginInterfaces.PasswordGlobal, p.GetTaskOptions().PasswordStrategy,
            "plugin %s should default to global", name)
    }
}

func TestAlistEncrypt_ResolvePassword_NoGlobalFallback(t *testing.T) {
    p := &alistencrypt.AlistEncryptPlugin{}
    // 模拟：无 DefaultPassword，有全局密码 → Independent 策略不应返回全局密码
    p.settings.DefaultPassword = ""
    // 注意：不需要设置 cfg.Password，因为 Independent 不应读取它
    result := p.resolvePassword()
    assert.Empty(t, result, "Independent plugin with no default password should return empty")
}

func TestAlistEncrypt_ResolvePasswordWithOverride_PriorityOrder(t *testing.T) {
    p := &alistencrypt.AlistEncryptPlugin{}
    p.settings.DefaultPassword = "plugin-default"

    // L2 最高优先级
    assert.Equal(t, "l2-override", p.resolvePasswordWithOverride("l2-override", ""))
    // L1b 其次
    assert.Equal(t, "l1b-plugin", p.resolvePasswordWithOverride("", "l1b-plugin"))
    // L1a 再次
    assert.Equal(t, "plugin-default", p.resolvePasswordWithOverride("", ""))
    // 全部为空
    assert.Empty(t, p.resolvePasswordWithOverride("", ""))
}
```

**文件**: `internal/service/task_manager_extra_test.go`（新建）

```go
package service_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestCreateWithExtras_PreservesExtraFields(t *testing.T) {
    tm := setupTestManager(t)
    extras := map[string]string{"plugin_password": "test123", "custom_field": "value"}
    task := tm.CreateWithExtras("encrypt", "/test/file.mp4", "", "override-pw", 4, extras)
    assert.Equal(t, "override-pw", task.Password)
    assert.Equal(t, "test123", task.ExtraFields["plugin_password"])
    assert.Equal(t, "value", task.ExtraFields["custom_field"])
}

func TestCreateWithoutExtras_Compat(t *testing.T) {
    tm := setupTestManager(t)
    task := tm.Create("encrypt", "/test/file.mp4", "", "pw", 4)
    assert.Nil(t, task.ExtraFields)  // 向后兼容：旧 API 创建的任务 ExtraFields 为 nil
}
```

#### 15b. 前端测试

**文件**: `app/encv-mobile/src/__tests__/useTaskForm.test.ts`（新建）

关键测试用例：
- predictPlugin 返回 video 插件 → `taskOptions.passwordStrategy === 'global'`, `supportVersionSelect=true`, 无 extraFields
- predictPlugin 返回 alist_encrypt → `passwordStrategy='independent'`, 有 1 个 extraFields(password type)
- `filteredExtraFields` 按 condition 过滤
- `getExtraPayload()` 只收集 extraValues（不含 secondaryPassword）
- `reset()` 清空所有状态
- API 失败降级

---

## 四、文件变更清单（完整）

| # | 文件 | 操作 | 说明 | 对应缺陷修复 |
|---|------|------|------|-------------|
| 1 | `internal/v2/plugins/interfaces/interfaces.go` | **修改** | 新增 `TaskPasswordResolver` 接口 | D |
| 2 | `internal/v2/plugins/alistencrypt/plugin.go` | **修改** | Initialize 不再复制全局密码；新增 `resolvePasswordWithOverride()`；实现 `TaskPasswordResolver` | A |
| 3 | `internal/server/mobile_api.go` | **修改** | `handleCreateTaskGin` 接受 `ExtraFields` | C |
| 4 | `internal/service/task_manager.go` | **修改** | `MobileTask` 增加 `ExtraFields`；新增 `CreateWithExtras()`；`processEncrypt/Decrypt` 使用 `TaskPasswordResolver`；新增 `getPasswordContext()` | C/D |
| 5 | `app/encv-mobile/src/api/encv.ts` | **修改** | `createTask()` 增加 `extraFields` 参数 | B |
| 6 | `app/encv-mobile/src/composables/useTaskForm.ts` | **修改** | `getExtraPayload()` 移除 secondaryPassword | B |
| 7 | `app/encv-mobile/src/views/Tasks.vue` | **修改** | `handleCreateTask()` 分离 L1/L2；密码标签重命名 | B |
| 8 | `app/encv-mobile/src/composables/useI18n.ts` | **修改** | secondaryPassword → overridePassword 重命名 | — |
| 9 | `internal/v2/plugins/task_options_test.go` | **新建** | TaskOptions 声明正确性测试 | A |
| 10 | `internal/service/task_manager_extra_test.go` | **新建** | CreateWithExtras + ExtraFields 持久化测试 | C |
| 11 | `app/encv-mobile/src/__tests__/useTaskForm.test.ts` | **新建** | composable 单元测试 | — |

> 注：Step 1-8 涉及的文件已在上轮完成，本表仅列 Step 9-15 的新增/修改。

---

## 五、密码解析管道（修订后的完整数据流）

```
┌────────────── 前端 Tasks.vue ──────────────┐
│                                            │
│  [插件独立密码输入]  → extraValues["plugin_password"]  (L1b)
│  [覆盖密码输入]     → secondaryPassword                (L2)  │
│                                            │
│  createTask(type, src, tgt, secondaryPassword, version, {  │
│    plugin_password: extraValues["plugin_password"],  │
│  })                                      │
└──────────────────┬─────────────────────────┘
                   │ POST /api/tasks
                   ▼
┌────────────── 后端 handleCreateTaskGin ──────┐
│  req.Password    → task.Password              (L2)  │
│  req.ExtraFields → task.ExtraFields           (L1b) │
└──────────────────┬───────────────────────────┘
                   │
                   ▼
┌────────────── processEncrypt/Decrypt ─────────┐
│                                            │
│  if plugin implements TaskPasswordResolver:  │
│    password = plugin.ResolveTaskPassword(    │
│      task.Password (L2),                      │
│      task.ExtraFields (L1b)                   │
│    )                                          │
│  else:  // PasswordGlobal 插件               │
│    password = task.Password \|\| cfg.Password  │
│    // L2 > L0                                 │
│                                            │
│  passwordCtx = getPasswordContext(password)     │
│  EncryptFileWithPlugin(passwordCtx, ...)       │
└──────────────────────────────────────────────┘
                   │
                   ▼
┌── AlistEncrypt.ResolveTaskPassword ──┐
│  if taskPassword(L2) != "" → return L2  │
│  if pluginPassword(L1b) != "" → return L1b│
│  if DefaultPassword(L1a) != "" → return L1a│
│  return ""  (❌ 不再 fallback 到全局密码)   │
└──────────────────────────────────────────┘
```

---

## 六、不做的事情（边界）

- **不改 schema.json 驱动的 PluginSettings** — 全局配置页与任务创建不同场景
- **不改其他 5 个插件的密码行为** — 它们都是 PasswordGlobal，走默认 L2>L0 管道
- **不在前端实现 MIME/扩展名检测** — 完全委托后端 predict-plugin API
- **不改 MobileTask JSON 序列化格式** — 新增 ExtraFields 字段自动兼容（map → JSON object，旧数据为 null）
