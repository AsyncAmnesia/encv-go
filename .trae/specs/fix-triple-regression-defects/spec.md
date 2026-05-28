# 修复三重回归缺陷 Spec

## Why

用户报告三个严重回归问题，表明之前的修复未能生效或被覆盖：

| # | 问题 | 根因分析 |
|---|------|----------|
| 0 | **文本预览卡顿 + 换行不生效** | text.html 的换行逻辑依赖 `isWrapping` 状态与按钮 `active` 类同步，但初始状态可能不一致；或 iframe 加载时机问题导致 `classList.toggle('no-wrap')` 未生效 |
| 1 | **安装扩展超时 + 确认界面不显示** | GoProcessPlugin 启动 InstallConfirmActivity 时设置了错误的 `action="com.encvgo.app.INSTALL_RESULT"`（这是 BroadcastReceiver 的 action，不是 Activity 启动用的），导致 Activity 启动失败或被系统拒绝；且缺少 `FLAG_ACTIVITY_NEW_TASK` 标志 |
| 2 | **加密视频 v3/v4 均报 deep integrity check failed** | `SkipStructCheck=true` 只跳过了 QuickStructCheck（L1），但 **deep integrity check（L4）不受此参数控制**，仍在 L3 成功后强制执行 MP4 结构比对/FFmpeg 解码/帧数一致性检查，而 v4 容器加密后 MP4 结构必然改变，导致验证失败 |

## What Changes

### 问题 0：文本预览换行失效

**根因**：text.html 的 `isWrapping=true` 初始状态与 CSS `white-space: pre-wrap` 默认一致，但按钮有 `class="active"`。问题在于：
1. 文本加载后 `textContent.classList.add('no-wrap')` 条件判断错误（L274-276：`if (!isWrapping)` 但此时 isWrapping=true）
2. iframe 内部 JS 执行时机可能晚于父页面状态同步

**修复**：
- 确保 text.html 初始化时 `isWrapping` 与按钮 `active` 类和 CSS 默认状态三者一致
- 文本加载完成后，根据当前 `isWrapping` 状态正确设置 `no-wrap` 类

### 问题 1：安装确认界面不显示

**根因**：GoProcessPlugin.kt L422-428 和 L556-562 中启动 InstallConfirmActivity 的 Intent 设置了错误的 `action`：

```kotlin
val intent = Intent(context, com.encvgo.app.InstallConfirmActivity::class.java).apply {
    action = "com.encvgo.app.INSTALL_RESULT"  // ❌ 这是 BroadcastReceiver 的 action！
    ...
}
context.startActivity(intent)
```

**问题**：
1. `action` 属性用于 Intent 匹配，设置 `"com.encvgo.app.INSTALL_RESULT"` 会让系统尝试匹配能处理此 action 的组件，而不是直接启动指定的 Activity 类
2. 当 context 不是 Activity 时（如 Service 或 Application），缺少 `FLAG_ACTIVITY_NEW_TASK` 会导致启动失败

**修复**：
- 移除错误的 `action` 设置，或改为 `Intent.ACTION_VIEW`
- 添加 `FLAG_ACTIVITY_NEW_TASK` 标志（当 context 不是 Activity 时）

### 问题 2：加密验证 deep integrity check 失败

**根因**：content_verifier.go 的验证分层架构：

```
L1: QuickStructCheck (moov/stsz) ← SkipStructCheck=true 可跳过 ✅
L2: QuickSampleHashCheck (采样 Hash)
L3: FullHashCheck (全量 Hash)
L4: runDeepVideoIntegrityCheck ← 不受任何参数控制！❌
```

当 `SkipStructCheck=true` 时，L1 被跳过返回 warning，但 L4 仍会执行：
- `checkMP4Structure`：使用 go-mp4.Probe 比对原始和解密文件的 MP4 结构
- `checkFFmpegDecoding`：FFmpeg 解码压力测试
- `checkFrameConsistency`：帧数与时长比对

v4 容器加密会改变 MP4 atom 结构（重新打包），导致 L4 的 `checkMP4Structure` 失败。

**修复**：
- 在 `VerifyOptions` 中新增 `SkipDeepCheck` 参数
- 当 `isPostEncryptVerify=true` 时，设置 `SkipDeepCheck=true`
- 在 `Verify()` 中，当 `SkipDeepCheck=true` 时跳过 `runDeepVideoIntegrityCheck`

---

## ADDED Requirements

### Requirement D0: 文本预览换行状态一致性

系统 SHALL 确保文本预览的换行状态在以下三者间保持一致：
- JS 变量 `isWrapping` 初始值
- 换行按钮的 `active` CSS 类
- `#textContent` 的 `white-space` CSS 属性

#### Scenario D0.1: 初始状态为自动换行
- **WHEN** text.html 加载完成
- **THEN** `isWrapping=true`，按钮有 `active` 类，`#textContent` 无 `no-wrap` 类（使用 `pre-wrap`）

#### Scenario D0.2: 文本加载后状态正确
- **WHEN** 文本内容加载完成并显示
- **THEN** `#textContent` 的 CSS 类正确反映当前 `isWrapping` 状态（不额外添加/移除类）

### Requirement D1: InstallConfirmActivity 正确启动

系统 SHALL 确保 InstallConfirmActivity 能被正确启动并显示给用户。

#### Scenario D1.1: Intent 构造正确
- **WHEN** GoProcessPlugin 构造启动 InstallConfirmActivity 的 Intent
- **THEN** Intent 明确指定目标组件（`setClass(context, InstallConfirmActivity::class.java)`）
- **AND** 不设置错误的 `action` 属性（或使用 `Intent.ACTION_MAIN`）
- **AND** 当 context 不是 Activity 时添加 `FLAG_ACTIVITY_NEW_TASK`

#### Scenario D1.2: Activity 启动成功
- **WHEN** `context.startActivity(intent)` 执行
- **THEN** InstallConfirmActivity 显示在屏幕上
- **AND** 用户能看到 APK 信息和确认/取消按钮

### Requirement D2: PostEncryptProcessor 验证跳过深度检查

系统 SHALL 在 PostEncryptProcessor 场景下跳过 deep integrity check。

#### Scenario D2.1: SkipDeepCheck 参数生效
- **WHEN** `VerifyOptions.SkipDeepCheck=true`
- **THEN** `Verify()` 方法在 L3 (FullHashCheck) 成功后直接返回
- **AND** 不执行 `runDeepVideoIntegrityCheck`

#### Scenario D2.2: PostEncryptProcessor 设置 SkipDeepCheck
- **WHEN** `verifyContainer()` 在 `isPostEncryptVerify=true` 时执行
- **THEN** `verifyOpts.SkipDeepCheck=true` 与 `SkipStructCheck=true` 同时设置

---

## MODIFIED Requirements

### Requirement: VerifyOptions 结构扩展

`VerifyOptions` 结构体 SHALL 新增 `SkipDeepCheck` 字段：

```go
type VerifyOptions struct {
    SkipSizeCheck   bool // 跳过精确文件大小比对
    SkipStructCheck bool // 跳过结构完整性检查（L1）
    SkipDeepCheck   bool // 跳过深度完整性检查（L4）【新增】
    CollectWarnings bool // 收集 warnings
}
```

---

## Impact

- Affected code:
  - `internal/openlist/web/static/preview/text.html` — 换行状态初始化逻辑
  - `app/encv-mobile/android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt` — Intent 构造修复
  - `internal/v2/plugins/interfaces/interfaces.go` — VerifyOptions 新增字段
  - `internal/v2/plugins/video/content_verifier.go` — Verify() 方法跳过 L4 逻辑
  - `internal/v2/plugins/video/plugin.go` — verifyContainer() 设置 SkipDeepCheck
- Affected specs:
  - `fix-three-runtime-defects` — 原修复不完整，需要补充 L4 跳过逻辑
  - `test-orchestration-cross-platform` — encryption_roundtrip_e2e_test.go 需验证 SkipDeepCheck 生效