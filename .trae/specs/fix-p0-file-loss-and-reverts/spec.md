# P0 原始文件丢失 + iframe 回退 + 插件安装修复 Spec

## Why

上一轮修复引入了三个严重问题：

1. **【P0 数据丢失】加密后原始/预处理文件消失 + stsz box missing 验证失败**：`TempFileReadCloser.Close()` 在 `defer` 中自动删除底层临时文件。`registry.go:415` 的 `defer dataReader.Close()` 在 `PostEncryptProcessor` 返回后才执行，但 `PostEncryptProcessor` 内部的 `verifyContainer` 依赖 `encryptedSourcePath`（指向该临时文件）做解密对比验证。虽然当前代码顺序下 verify 先于 Close 执行，但 `.encv_tmp` 隐藏目录修改前临时文件创建在用户可见目录且 Close 必然删除它——用户看到"文件消失"。更严重的是：**stsz box missing 验证失败的真正原因需要排查**。

2. **【回退】文本预览 iframe 被错误移除**：用户明确确认 iframe 方案在 openlist 测试 100M 大文本毫无压力。上轮修改将 iframe 替换为 fetch+pre 方案，反而将整个文件加载到内存（500K 截断对大文本不够），且失去了 iframe 的流式渲染优势。

3. **【功能缺失】插件安装 UI 毫无变化**：`ExtensionsPage.vue` 安装流程中 `pickAndInstallPlugin()` 调用后状态不更新。GoProcessPlugin.kt 的 ComboLite 反射调用可能静默失败，前端无感知。

## What Changes

- **恢复 FilePreview.vue 的 iframe 文本预览**：回退 `<pre>` 改动，恢复 `<iframe :src="textPreviewUrl">`
- **修复 TempFileReadCloser 生命周期**：Close 不再自动删除文件，改为由调用方显式控制清理时机
- **修复插件安装状态反馈链路**：确保安装结果正确传递到前端 UI

## Impact

- Affected code:
  - `app/encv-mobile/src/views/FilePreview.vue` — 恢复 iframe 渲染
  - `internal/v2/reader/temp_file.go` — TempFileReadCloser.Close 行为变更
  - `app/encv-mobile/android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt` — 安装回调可靠性
  - `app/encv-mobile/src/views/ExtensionsPage.vue` — 状态更新逻辑
  - `internal/v2/plugins/video/plugin.go` / `registry.go` — 临时文件生命周期管理

## ADDED Requirements

### Requirement 1: 文本预览使用 iframe 流式渲染

#### Scenario: 预览大文本文件（>100MB）
- **WHEN** 用户打开 .txt/.log/.md 等文本文件
- **THEN** 使用 `<iframe :src="textPreviewUrl">` 加载 text.html 页面进行流式渲染
- **AND** 返回按钮响应正常（Ionic 导航不受 iframe 影响）

### Requirement 2: 临时文件 Close 不自动删除

#### Scenario: Preprocess 返回的临时文件在验证阶段仍可访问
- **WHEN** `TempFileReadCloser.Close()` 被调用
- **THEN** 仅关闭文件句柄，不删除底层文件
- **AND** 文件删除由调用方（registry.go 或 content_preprocessor.go）在确认不再需要时显式执行

### Requirement 3: 插件安装完成反馈 UI 更新

#### Scenario: APK 安装成功
- **WHEN** pickAndInstallPlugin 返回 success
- **THEN** ExtensionsPage 显示「已安装」badge
- **AND** 刷新列表后状态保持一致

## MODIFIED Requirements

### Requirement: TempFileReadCloser.Close 行为变更

**之前**：Close() → file.Close() + os.Remove(path)
**之后**：Close() → 仅 file.Close()
**迁移**：所有依赖 Close 自动清理的调用点需添加显式 os.Remove

### Requirement: FilePreview.vue 文本渲染方式

**之前（本轮错误修改）**：fetch 全文 + `<pre><code>` 渲染
**之后**：恢复为 `<iframe :src="textPreviewUrl">` 流式渲染

## REMOVED Requirements

### Requirement: fetch+pre 文本渲染方案

**原因**：用户明确确认 iframe 方案在大文件场景表现良好，替换方案反而降低性能
**迁移**：完全回退，恢复 iframe 实现
