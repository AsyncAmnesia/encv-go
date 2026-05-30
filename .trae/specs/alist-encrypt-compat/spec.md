# 移动端插件系统适配修复 Spec（Round 3 — 深度适配）

## Why

Round 2 完成了 pluginName 传递、防重入锁、普通文件 encrypt action、plugin mode 刷新修复。实测发现 **插件系统适配远未完成**，存在 3 个深层次架构问题：

1. **.bin 解密任务卡在"插件分析"** — 新建解密任务时 doPredict 对 .bin 文件的 decrypt 类型可能返回空 candidates，UI 永远显示"分析中"；且 .bin 文件预览走普通文件路径显示 "unsupported"
2. **其他插件的加密后缀文件无解密入口** — `isAlistEncrypted()` 在 `file.isEncrypted===true` 时直接返回 false，导致 ENCV 容器加密文件无法获得 decrypt action
3. **.bin 硬编码违反插件系统原则** — 加密文件识别不应只在 alist-encrypt 内部闭环，Files.vue 等入口代码也需要感知

## 架构认知（关键前提）

### 两种加密文件模型

| 模型 | 插件示例 | 文件标记 | 后缀 | 预期处理 |
|------|---------|---------|------|---------|
| **ENCV 容器模式** | encv-container 等 | `isEncrypted=true` | 各插件自行定义 `.enc`, `.encv` 等 | 容器信息展示 / 解密 |
| **alist-encrypt 模式** | alist-encrypt | `isEncrypted=false/undefined` | 统一 `.bin`（单一后缀） | 流式解密预览 / 解密 |

**现状**：插件系统主要针对 **ENCV 容器模式** 设计（`isEncrypted=true` 分支）。alist-encrypt 作为**单后缀非容器模式**插件，在多个入口点未被正确适配。

### alist-encrypt 的特殊性

- **单一加密后缀**：所有加密文件统一变为 `.bin`，原始类型（视频/音频/文档）被隐藏
- **非 ENCV 容器**：后端不设置 `isEncrypted=true`（它不是 ENCV 容器格式）
- **流式解密**：预览时需输入密码 → 获取 stream URL → 播放原始内容

## What Changes

### 核心原则：双模式兼容，不破坏现有 ENCV 容器流程

- **isAlistEncrypted 保持聚焦 .bin**（用户确认：alist-encrypt 是单一加密后缀），但**移除对 `isEncrypted=true` 文件的错误排除**
- **getAlistActions 扩展为双模式处理器**：同时覆盖 alist-encrypt .bin 文件和 ENCV 容器文件
- **Files.vue 入口增加 .bin 感知**：handleFileClick / handleLongPress 中识别 isAlistEncrypted 文件

## Impact

- Affected code:
  - `src/features/alist-encrypt/useAlistEncrypt.ts` — `isAlistEncrypted()` 移除 `isEncrypted` 排除逻辑
  - `src/features/alist-encrypt/actions.ts` — `getAlistActions` 扩展双分支（alist-encrypt 模式 + ENCV 容器模式）
  - `src/views/Files.vue` — handleFileClick 增加 .bin 流式预览路径
  - `src/composables/useNewTaskModal.ts` — 解密任务 doPredict 降级
  - `__tests__/` — 测试更新

---

## ADDED Requirements (Round 3)

### REQ-11: isAlistEncrypted 移除 isEncrypted 排除（保持 .bin 单一后缀）

#### 当前代码问题

[useAlistEncrypt.ts L27](file:///workspace/app/encv-mobile/src/features/alist-encrypt/useAlistEncrypt.ts#L27):
```typescript
if (file.isDirectory || file.isEncrypted) return false  // ← isEncrypted=true 时排除！
```

这行导致：
- ENCV 容器加密文件（`isEncrypted=true`）被错误排除 → 无法获得 alist-encrypt Feature 的 decrypt action
- 但这些文件确实需要解密操作入口

#### 修复方案

移除 `|| file.isEncrypted` 条件。`isAlistEncrypted` 只做一件事：**判断文件是否是 alist-encrypt 加密的 .bin 文件**。

对于 ENCV 容器文件（`isEncrypted=true`）的解密需求，由 `getAlistActions` 的扩展分支处理（见 REQ-13）。

```typescript
export function isAlistEncrypted(file: FileItem): boolean {
  if (file.isDirectory) return false
  const suffix = (getFieldValue(['plugin_settings', 'alist_encrypt', 'suffix']) as string) || '.bin'
  return file.name.endsWith(suffix)
}
```

#### 场景 11.1: .bin 文件（不变）
- **WHEN** 文件名为 `video.bin`
- **THEN** 返回 true ✅

#### 场景 11.2: isEncrypted=true 的 ENC 容器文件（行为变化！）
- **WHEN** 文件名为 `doc.enc` 且 `isEncrypted=true`
- **THEN** 返回 false（因为它不是 .bin）— 这是正确的，这类文件由 REQ-13 的扩展分支处理

### REQ-12: .bin 文件预览走流式解密路径

#### 根因链路

```
用户点击 video.bin → Files.vue handleFileClick()
  → file.isEncrypted === false （.bin 不是 ENCV 容器）
  → getFileCategory('video.bin') → 'other'
  → router.push('/tabs/preview', isEncrypted='false')
    → FilePreview.vue → determinePreviewType() → 'unsupported' ❌
```

#### 修复方案

[Files.vue handleFileClick](file:///workspace/app/encv-mobile/src/views/Files.vue#L764-L780) 中，在现有 `if (file.isEncrypted)` 判断**之前**，增加：

```typescript
if (isAlistEncrypted(file)) {
  // alist-encrypt .bin 文件 → 流式解密预览
  await handleAlistPreview(file)
  return
}
```

其中 `handleAlistPreview` 封装 promptPassword → getStreamUrl → player 打开流程。

#### 场景 12.1: 点击 .bin 文件
- **WHEN** 用户点击 `.bin` 文件
- **THEN** 弹出密码框 → 输入后打开播放器（stream URL）

#### 场景 12.2: ENCV 容器预览不受影响（回归保护）
- **WHEN** 点击 `isEncrypted=true` 的文件
- **THEN** 仍走现有 FilePreview.vue 路径

### REQ-13: getAlistActions 扩展为双模式处理器

#### 问题

当前 [actions.ts](file:///workspace/app/encv-mobile/src/features/alist-encrypt/actions.ts#L12-L55) 的双分支是：
- A: `isAlistEncrypted(file)` → decrypt + preview（只覆盖 .bin）
- B: else → encrypt（普通文件加密）

**缺失第三种情况**：`isEncrypted=true` 的 ENCV 容器文件既不是 .bin 也不是普通文件，它们需要 **decrypt action**。

#### 修复方案

将 `getAlistActions` 改为三分支：

| 分支 | 条件 | 返回 |
|------|------|------|
| **A（alist-encrypt 模式）** | `isAlistEncrypted(file) === true` | decrypt + stream-preview |
| **B（ENCV 容器模式）** | `file.isEncrypted === true` | decrypt action（调用 openNewTask(path, 'decrypt')） |
| **C（普通文件）** | else（非目录） | encrypt action |

#### 场景 13.1: ENCV 容器文件长按显示解密
- **WHEN** 长按 `isEncrypted=true` 的文件
- **THEN** 返回 decrypt action

#### 场景 13.2: .bin 文件行为不变（回归保护）
- **WHEN** 长按 `.bin` 文件
- **THEN** 返回 decrypt + stream-preview

### REQ-14: 解密任务 doPredict 降级处理

#### 根因

新建解密任务 `openNewTask(sourcePath, 'decrypt')` 触发：
1. modal.present() ✅
2. present() 后 doPredict(sourcePath, 'decrypt')
3. 后端对 decrypt 类型可能返回空 candidates
4. NewTaskModal: `isPredicting = src.length > 0 && cands.length === 0 && !pluginName` → **永远"分析中"**

#### 修复方案

[useNewTaskModal.ts L128-134](file:///workspace/app/encv-mobile/src/composables/useNewTaskModal.ts#L128-L134) doPredict 回调中：
- 当 taskType === 'decrypt' 且 cands 为空时 → 设置 `state.predictedPlugin = 'auto-detect'`
- 让 `isPredicting` 变 false，允许用户提交

#### 场景 14.1: 解密任务 predictPlugin 空
- **WHEN** 创建解密任务且 predictPlugin 返回空
- **THEN** UI 不卡"分析中"，可正常提交

### REQ-15: Mock 测试更新

- [ ] isAlistEncrypted: 移除 isEncrypted 排除后，isEncrypted=true 文件返回 false（因为不是 .bin）
- [ ] getAlistActions: 三分支全覆盖（.bin / isEncrypted=true / 普通文件）
- [ ] 解密任务 doPredict 降级测试
