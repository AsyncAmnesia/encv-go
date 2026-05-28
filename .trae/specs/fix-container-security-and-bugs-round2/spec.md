# 加密容器 Bug 二轮修复 Spec

## Why

上一轮修复（fix-container-security-and-bugs）完成后，用户测试发现 4 个遗留问题：

1. **新建文件夹功能不工作**：FilePickerModal 的 + 按钮点击后目录列表变空白，无任何实际效果
2. **v4 容器加密误报新错误**：从 `size mismatch` 变为 `stsz box missing`（QuickStructCheck 在重编码文件上过于严格）
3. **v3 容器加密真实失败**：`failed to create temp file for MP4 remuxing: open .../.encv_verify_.../encv-pre-*.mp4: no such file or directory`（临时目录未创建）
4. **v4 容器信息乱码显示**：版本、容器ID、清单数据在前端显示为乱码
5. **Mock 测试覆盖不足**：上述 Bug 均未在测试中暴露

## What Changes

- **修复** FilePickerModal 新建文件夹的渲染逻辑和交互流程
- **修复** content_verifier 的 QuickStructCheck 对重编码文件的宽容度
- **修复** content_preprocessor 中临时文件创建前的目录检查
- **修复** v4 容器信息 API 返回数据的编码问题
- **新增** FilePickerModal 组件测试、加密流程 E2E mock 测试

## Impact

- Affected specs: fix-container-security-and-bugs（本轮为续修）
- Affected code:
  - `app/encv-mobile/src/components/FilePickerModal.vue` — 新建文件夹 UI 修复
  - `internal/v2/plugins/video/content_verifier.go` — stsz check 宽容
  - `internal/v2/plugins/video/content_preprocessor.go` — MkdirAll 防御
  - `internal/service/mobile_service.go` 或相关文件 — 容器信息编码修复
  - 新增测试文件

---

## ADDED Requirements

### Requirement: FilePickerModal 新建文件夹功能修复

系统 SHALL 提供可靠的新建文件夹交互。

#### Scenario: 点击 + 按钮显示输入框并保留文件列表背景
- **WHEN** 用户在 folder 模式下点击 + 按钮
- **THEN** 输入框以 overlay 形式显示在文件列表上方（不替换文件列表）
- **AND** 文件列表保持可见（变暗或半透明）

#### Scenario: 确认创建后刷新并进入新目录
- **WHEN** 用户输入名称并确认
- **THEN** 调用 createDirectory API → 成功后 navigateTo 到新路径 → 文件列表刷新显示新目录内容
- **AND** 输入框自动隐藏

#### Scenario: 取消创建恢复原状态
- **WHEN** 用户点击取消或按返回键
- **THEN** 输入框隐藏，文件列表恢复正常显示

### Requirement: QuickStructCheck 重编码宽容模式

Verify 方法 SHALL 在检测到重编码源时跳过严格的 MP4 结构检查。

#### Scenario: 重编码后的 MP4 缺少 stsz box
- **WHEN** 解密输出是经过 FFmpeg/MediaCodec 重编码的 MP4 文件（可能缺少 stsz/moov 等标准 box）
- **THEN** QuickStructCheck 不应报 `stsz box missing` 错误
- **AND** 应跳过结构检查或降级为 warning

### Requirement: 临时文件创建防御性 MkdirAll

content_preprocessor SHALL 在调用 `os.CreateTemp` 前确保目标目录存在。

#### Scenario: outputDir 不存在时创建临时文件
- **WHEN** `os.CreateTemp(p.outputDir, ...)` 被调用但 p.outputDir 不存在
- **THEN** 不应报 `no such file or directory`
- **AND** 应先执行 `os.MkdirAll(p.outputDir, 0755)` 后重试

### Requirement: v4 容器信息正确编码显示

后端 API 返回的容器元数据 SHALL 使用 UTF-8 编码且前端正确解析。

#### Scenario: 查看 v4 加密文件的容器信息
- **WHEN** 用户在 FileInfo 或 FilePreview 页面查看 v4 容器文件
- **THEN** version 显示为数字（如 4）、container_id 显示为有效 UUID 字符串、manifest 显示为格式化 JSON
- **AND** 不出现乱码、二进制数据或空值

### Requirement: 关键路径 Mock 测试覆盖

系统 SHALL 为以下关键路径提供 mock/integration 测试：

- FilePickerModal 新建文件夹完整流程（mock API）
- 加密流程 E2E：Preprocess → Encrypt → PostEncrypt（含重编码场景）
- v3/v4 容器信息 API 返回值验证

---

## MODIFIED Requirements

### Requirement: FilePickerModal.vue template 结构

当前模板使用 `v-if="showNewFolder"` 与 `v-else`（ion-list）互斥渲染，导致点击 + 后文件列表完全消失。

**修改为：** 输入框使用绝对定位 overlay 方式，不与 ion-list 互斥。

### Requirement: VideoContentVerifier.Verify

上一轮已添加 SkipSizeCheck 跳过 L1 大小比对。本轮需进一步添加 **SkipStructCheck** 跳过 L2 QuickStructCheck。

### Requirement: VideoContentPreprocessor 临时文件创建

所有调用 `os.CreateTemp(p.outputDir, ...)` 的位置（remapMP4ForFastStart、transcodeToFastStartMP4 等）需增加 MkdirAll 防御。

---

## REMOVED REQUIREMENTS

（无）
