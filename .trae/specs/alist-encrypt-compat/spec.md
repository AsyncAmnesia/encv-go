# 移动端插件系统适配修复 Spec（Round 3 — 深度适配）

## Why

Round 2 完成了 pluginName 传递、防重入锁、普通文件 encrypt action、plugin mode 刷新修复。实测发现 **插件系统适配远未完成**，存在 3 个深层次架构问题：

1. **.bin 解密任务卡在"插件分析"** — 新建解密任务时 doPredict 对 .bin 文件的 decrypt 类型可能返回空 candidates，导致 UI 永远显示"分析中" spinner；且 .bin 文件预览走普通文件路径（不被识别为加密文件），显示 "unsupported"
2. **其他插件的加密后缀文件无解密入口** — `isAlistEncrypted()` 硬编码 `.bin` 后缀 + `file.isEncrypted === true` 时直接返回 false，导致其他容器插件（如用 `.enc`, `.encv` 等后缀）的加密文件无法获得 decrypt action
3. **.bin 硬编码违反插件系统原则** — 加密后缀应该从 `PluginMeta.containerExtension` 动态获取，而非硬编码 `.bin`

## What Changes

### 核心原则：消除硬编码，从 PluginMeta 动态推导加密文件识别

- **Bug 1 修复（本质）**：`.bin` 文件预览需要识别为 alist-encrypt 加密文件并走流式解密预览路径；解密任务的 doPredict 需要正确处理或跳过
- **Bug 2 修复（本质）**：`isAlistEncrypted()` 需要支持所有已注册插件的 containerExtension，不限于 `.bin`
- **Bug 3 修复（本质）**：加密后缀从 `PluginMeta.containerExtension` 配置驱动，Feature 系统动态感知

## Impact

- Affected code:
  - `src/features/alist-encrypt/useAlistEncrypt.ts` — `isAlistEncrypted()` 消除 `.bin` 硬编码，改为多后缀支持
  - `src/features/alist-encrypt/actions.ts` — getAlistActions 的 decrypt 分支需要覆盖所有加密后缀
  - `src/views/Files.vue` — handleFileClick 中对 .bin 的预览路径修正
  - `src/views/FilePreview.vue` — 可能需要对 alist-encrypt 类型文件特殊处理
  - `src/composables/useNewTaskModal.ts` — 解密任务 doPredict 超时/空结果降级处理
  - `__tests__/` — 更新测试反映新的 isAlistEncrypted 行为

---

## ADDED Requirements (Round 3)

### REQ-11: isAlistEncrypted 消除 .bin 硬编码，支持多后缀

系统 SHALL 从已注册插件的 `containerExtension` 动态获取加密文件后缀列表。

#### 架构现状（问题）

当前 [useAlistEncrypt.ts L26-L30](file:///workspace/app/encv-mobile/src/features/alist-encrypt/useAlistEncrypt.ts#L26-L30)：
```typescript
export function isAlistEncrypted(file: FileItem): boolean {
  if (file.isDirectory || file.isEncrypted) return false
  const suffix = (getFieldValue(['plugin_settings', 'alist_encrypt', 'suffix']) as string) || '.bin'
  return file.name.endsWith(suffix)  // ← 只认单一后缀！
}
```

**三重问题**：
1. 默认硬编码 `.bin`
2. `file.isEncrypted === true` 时直接返回 false → **其他容器的加密文件被排除**
3. 不感知其他插件的 containerExtension

#### 修复方案

`isAlistEncrypted` 改为：
1. 如果 `file.isEncrypted === true` → 返回 true（这是任何容器加密文件的通用标记）
2. 否则检查文件名是否以 **任意已注册插件的 containerExtension** 结尾（从 PluginMeta 列表动态获取）
3. 兼容配置项 suffix 作为 fallback

#### 场景 11.1: .bin 文件（alist-encrypt 默认后缀）
- **WHEN** 文件名为 `video.bin` 且 `isEncrypted !== true`
- **THEN** `isAlistEncrypted` SHALL 返回 true（通过默认后缀匹配）

#### 场景 11.2: 其他后缀的加密文件（如 .enc, .encv）
- **WHEN** 文件名为 `doc.enc` 且某插件的 `containerExtension === '.enc'`
- **THEN** `isAlistEncrypted` SHALL 返回 true（通过动态后缀匹配）

#### 场景 11.3: 已标记 isEncrypted=true 的文件
- **WHEN** 后端返回 `file.isEncrypted === true`（ENCV 容器标记）
- **THEN** `isAlistEncrypted` SHALL 返回 true（不再错误地排除）

#### 场景 11.4: 目录文件
- **WHEN** 文件是目录
- **THEN** `isAlistEncrypted` SHALL 返回 false

### REQ-12: .bin 文件预览走流式解密路径

系统 SHALL 对 alist-encrypt 加密的 .bin 文件提供正确的预览能力。

#### 根因链路

```
用户点击 video.bin → Files.vue handleFileClick()
  → file.isEncrypted === false （.bin 不是 ENCV 容器！是原始加密文件）
  → getFileCategory('video.bin') → 'other'（.bin 不在 video/audio/image 列表）
  → router.push('/tabs/preview', { isEncrypted: 'false' })
    → FilePreview.vue determinePreviewType() → 'unsupported'
    → 用户看到 "Unsupported file type"
```

**关键发现**：`.bin` 文件在后端眼中**不是 ENCV 容器**（`isEncrypted=false`），而是 **alist-encrypt 原始加密文件**。它需要的是 **流式解密预览**（输入密码 → 解码 → 播放原始视频），而非 ENCV 容器信息展示。

#### 修复方案

Files.vue `handleFileClick` 中增加判断：如果 `isAlistEncrypted(file) === true`（即使 `file.isEncrypted !== true`），走 alist-encrypt 流式预览路径（promptPassword → getStreamUrl → player）。

#### 场景 12.1: 点击 .bin 文件
- **WHEN** 用户点击一个 `.bin` 文件（`isAlistEncrypted === true`）
- **THEN** 系统 SHALL 弹出密码输入框 → 获取密码后打开播放器（传入 stream URL）

#### 场景 12.2: ENCV 容器文件预览不变（回归保护）
- **WHEN** 用户点击一个 `isEncrypted === true` 的 ENCV 容器文件
- **THEN** 仍走现有 FilePreview.vue 容器信息展示路径

### REQ-13: 其他插件加密文件的解密入口

系统 SHALL 为所有已注册插件的加密文件提供解密 action（通过 Feature 架构）。

#### 根因链路

```
用户长按 doc.enc（其他插件加密，isEncrypted=true）
  → Files.vue handleLongPress()
    → getAllActions(doc.enc)
      → useFileFeatures.collectActions()
        → alist-encrypt Feature.isActive(doc.enc)
          → !doc.enc.isDirectory → true ✅ （Round 2 已扩大范围）
        → getAlistActions(doc.enc)
          → isAlistEncrypted(doc.enc)
            → doc.enc.isEncrypted === true → return false ❌ （被错误排除！）
          → else 分支 → 返回 encrypt action（但用户需要的是 decrypt！）
```

**核心矛盾**：`isAlistEncrypted` 在 REQ-11 修复后会正确识别这些文件为"加密"，但 `getAlistActions` 的分支逻辑需要调整——**已加密的文件应返回 decrypt action，未加密的才返回 encrypt action**。

#### 修复方案

REQ-11 修复后 `isAlistEncrypted` 对 `isEncrypted=true` 的文件也返回 true，因此 `getAlistActions` 的分支 A 会命中（`isAlistEncrypted(file) === true`），自然返回 decrypt + stream-preview。**此 bug 随 REQ-11 修复自动解决。**

#### 场景 13.1: .enc 文件长按显示解密
- **WHEN** 用户长按一个 `.enc` 文件（某插件的 containerExtension，`isEncrypted=true`）
- **THEN** `isAlistEncrypted` 返回 true → `getAlistActions` 返回 decrypt action

#### 场景 13.2: .bin 文件长按仍显示解密+预览（回归保护）
- **WHEN** 用户长按一个 `.bin` 文件
- **THEN** 行为不变（decrypt + stream-preview）

### REQ-14: 解密任务 doPredict 降级处理

系统 SHALL 在解密任务中当 doPredict 返回空结果时优雅降级。

#### 根因

新建解密任务时 `openNewTask(sourcePath, 'decrypt')` 触发：
1. modal.present() — 秒开 ✅
2. present() 后 doPredict(sourcePath, 'decrypt') — 调用后端 predictPlugin API
3. 如果后端对 decrypt 类型返回空 candidates → `cands = []` + `predictedPlugin = null`
4. NewTaskModal UI: `isPredicting = src.length > 0 && cands.length === 0 && !pluginName` → **永远显示"分析中"！**

#### 修复方案

在 `useNewTaskModal.ts` 的 onSubmit 或 doPredict 回调中：
- 当 taskType === 'decrypt' 且 doPredict 返回空 candidates 时 → 自动设置一个默认的 pluginName（如从 sourcePath 反推，或允许无 pluginName 提交）
- 或者更根本地：**解密任务不需要 predictPlugin**，因为解密时插件信息可以从文件本身的容器元数据获取

最小化改动方案：doPredict 返回空且 taskType==='decrypt' 时，将 state.predictedPlugin 设为一个降级值（如 `'auto-detect'`），让 isPredicting 变 false 并允许提交。

#### 场景 14.1: 解密任务 predictPlugin 返回空
- **WHEN** 创建解密任务且后端 predictPlugin 返回空 candidates
- **THEN** UI SHALL 不再卡在"分析中"，而是显示可用表单（允许用户手动填写或使用 auto-detect 模式）

### REQ-15: Mock 测试更新

测试套件 SHALL 反映 Round 3 的行为变更。

- [ ] isAlistEncrypted: isEncrypted=true 文件返回 true
- [ ] isAlistEncrypted: 多种 containerExtension 后缀均能识别
- [ ] getAlistActions: 非 .bin 加密文件也能返回 decrypt action
- [ ] 解密任务 doPredict 空结果降级测试
