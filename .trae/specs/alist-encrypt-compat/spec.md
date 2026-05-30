# 移动端插件系统适配修复 Spec（Round 4 — 全面排查补漏）

## Why

Round 3 完成了 isAlistEncrypted 排除移除、getAlistActions 三分支、加密文件预览路径、doPredict 降级。对整个插件系统做全面代码审计后，发现以下遗留问题：

### P0（阻塞）

1. **插件视图 container tab 过滤遗漏 alist-encrypt 加密文件** — `filteredPluginFiles` 的 container 分支只匹配 `isEncrypted=true` 或 `containerExtension` 后缀，alist-encrypt 加密文件（`isEncrypted=false` + 配置后缀）在两个 tab 中分布错误或不可见

### P1（重要）

2. **`getPluginIcon` 硬编码图标映射表** — 违反配置驱动原则，新插件无法获得专属图标
3. **Settings.vue Feature 注册逻辑重复调用风险** — onMounted + watch 可能触发重复 register/unregister
4. **任务名称不含插件类型信息** — 同名文件的不同插件任务无法从名称区分

### P2（优化）

5. **Feature 系统仅 1 个实现** — 整个 features/ 目录只有 alist-encrypt，其他插件无前端适配层（按需扩展，非阻塞）

## What Changes

### 核心原则：每个发现的问题都从插件系统架构本质上解决，不局部打补丁

- **container tab 过滤**：增加 `isAlistEncrypted(file)` 作为第三种匹配条件
- **图标映射**：改为从 PluginMeta 动态获取或使用 fallback 策略
- **Feature 注册去重**：合并 onMounted + watch 的注册逻辑
- **任务命名**：在 getTaskName 中纳入 pluginName 信息

## Impact

- Affected code:
  - `src/views/Files.vue` — `filteredPluginFiles` computed + `getPluginIcon` 函数
  - `src/views/Settings.vue` — Feature 注册逻辑去重
  - `src/views/Tasks.vue` — `getTaskName` 函数增强
  - `__tests__/` — 新增测试覆盖

---

## ADDED Requirements (Round 4)

### REQ-16: 插件视图 container tab 正确显示所有加密文件（P0）

#### 当前代码问题

[Files.vue L1268-L1275](file:///workspace/app/encv-mobile/src/views/Files.vue#L1268-L1275):
```typescript
const filteredPluginFiles = computed(() => {
  if (!selectedPlugin.value) return []
  let list: FileItem[]
  if (pluginTab.value === 'container') {
    // ❌ 只匹配 ENCV 容器！alist-encrypt 加密文件被排除
    list = pluginFiles.value.filter(f =>
      f.isEncrypted ||
      selectedPlugin.value?.containerExtension && f.name.endsWith(selectedPlugin.value.containerExtension)
    )
  } else {
    // origin tab: !isEncrypted → alist-encrypt 文件会出现在这里（因为它们 isEncrypted=false）
    // 但这语义不对——加密后的文件不应该算"原始文件"
    list = pluginFiles.value.filter(f => !f.isEncrypted)
  }
})
```

**问题分析**：
- alist-encrypt 加密文件：`isEncrypted=false`，不以 `containerExtension` 结尾（那是 ENCV 容器的后缀），以**配置的 alist-encrypt 后缀**结尾
- **container tab**：两个条件都不满足 → **不显示** ❌
- **origin tab**：`!isEncrypted = true` → **显示了** ⚠️ 但语义错误（加密文件不应归类为"原始文件"）

#### 修复方案

将 container / origin 的过滤条件改为：

| Tab | 条件 | 含义 |
|-----|------|------|
| **container** | `f.isEncrypted \|\| isAlistEncrypted(f) \|\| (containerExtension && endsWith)` | 所有加密/容器化文件 |
| **origin** | `!f.isEncrypted && !isAlistEncrypted(f)` | 未加密的原始文件 |

#### 场景 16.1: alist-encrypt 加密文件在 container tab 可见
- **WHEN** 插件视图中有 `video.ae`（alist-encrypt 加密文件，`isEncrypted=false`）
- **THEN** container tab SHALL 显示该文件

#### 场景 16.2: origin tab 不包含加密文件
- **WHEN** 切换到 origin tab
- **THEN** alist-encrypt 加密文件和 ENCV 容器文件均不出现

### REQ-17: getPluginIcon 消除硬编码映射（P1）

#### 当前代码问题

[Files.vue L1166-L1169](file:///workspace/app/encv-mobile/src/views/Files.vue#L1166-L1169):
```typescript
function getPluginIcon(name: string): string {
  const icons: Record<string, string> = { video: filmOutline, audio: musicalNotesOutline, image: imageOutline, pdf: documentTextOutline, text: documentOutline, wps: documentOutline }
  return icons[name] || cubeOutline  // ← 其他所有插件都是通用方块图标
}
```

**问题**：
- 只有 6 种插件类型有特定图标
- `alist-encrypt` 及任何未来插件都返回 `cubeOutline`
- 新增插件必须手动修改此函数——违反插件系统可扩展原则

#### 修复方案

方案 A（推荐）：保留现有映射作为**内置默认值**，但允许 PluginMeta 携带自定义图标信息。如果 PluginMeta 有 icon 字段则优先使用，否则 fallback 到映射表，最终 fallback 到 `cubeOutline`。

方案 B（最小改动）：将 `cubeOutline` 改为更有意义的通用图标（如 `lockClosed` 表示加密相关插件），并添加注释说明扩展方式。

> 具体采用哪种方案取决于 PluginMeta 是否已有 icon 字段。如果后端 API 的 PluginMeta 不含 icon 字段，先采用方案 B 并预留方案 A 的接口位置。

### REQ-18: Settings.vue Feature 注册去重（P1）

#### 当前代码问题

[Settings.vue](file:///workspace/app/encv-mobile/src/views/Settings.vue) 中存在两处 Feature 注册触发点：
1. **onMounted** (约 L741): 调用 `syncAlistEncryptFeature()`
2. **watch 回调** (约 L814): 设置变更时也调用 `syncAlistEncryptFeature()`

`syncAlistEncryptFeature()` 内部执行 `registerFileFeature(createAlistEncryptFeature())` 或 `unregisterFileFeature('alist-encrypt')`。

**风险**：onMounted 触发时如果 watch 也因初始值变化触发 → 短时间内连续 register/unregister → 可能导致 Feature 状态不一致。

#### 修复方案

在 `syncAlistEncryptFeature()` 内部添加**幂等保护**：
- 记录当前已注册状态（模块级变量或 ref）
- 如果请求的状态与当前状态相同 → 直接返回（no-op）
- 只在实际需要切换时才执行 register/unregister

### REQ-19: 任务名称包含插件信息（P2）

#### 当前代码问题

[Tasks.vue L264-268](file:///workspace/app/encv-mobile/src/views/Tasks.vue#L264-L268):
```typescript
function getTaskName(task: EncvTask): string {
  const parts = task.sourcePath.replace(/\\/g, '/').split('/')
  return parts[parts.length - 1] || task.sourcePath
}
```

只取 sourcePath 最后一段，如 `video.mp4`。用户有多个插件时无法区分「video.mp4 的 alist-encrypt 加密」和「video.mp4 的 encv-container 加密」。

#### 修复方案

当 task.pluginName 存在时，在文件名后追加插件标识：

```
格式："{basename} [{pluginName}]"
示例："video.mp4 [alist-encrypt]"
```

当 pluginName 为空时保持原有行为（向后兼容）。

### REQ-20: Mock 测试覆盖新增场景

- [ ] filteredPluginFiles container tab 包含 isAlistEncrypted 文件
- [ ] filteredPluginFiles origin tab 排除 isAlistEncrypted 文件
- [ ] getTaskName 含 pluginName 时格式正确
- [ ] syncAlistEncryptFeature 幂等性测试
