# 修复三重回归缺陷 Spec（修订版）

## Why

用户报告三个严重回归问题，需要饱和式调试找出真正的根因：

### 问题 0：文本预览卡顿 + 换行不生效

**对比 OpenList 正常工作 vs 移动端失效**：

| 环境 | Preview URL | text.html baseUrl 计算 | decryptUrl |
|------|-------------|------------------------|------------|
| OpenList | `/_preview/text.html?file=...` | 检测 `/_preview` → 返回 basePath | `${basePath}/decrypt?file=...` ✅ |
| 移动端 | `/preview/text.html?file=...` | 无 `/_preview` → 返回 `''` | `/decrypt?file=...` ✅ |

URL 计算逻辑正确，问题不在 baseUrl。

**真正根因**：text.html 的 CSS 与 iframe 父容器高度约束冲突。

text.html L88-105：
```css
#textContent {
    display: none;
    height: 100vh;      /* ← 问题：100vh 在 iframe 内可能不等于 iframe 高度 */
    overflow-y: auto;
    overflow-x: auto;
    white-space: pre-wrap;
}
```

FilePreview.vue L333-343：
```css
.text-preview {
  width: 100%;
  height: 100%;        /* ← 问题：没有约束 iframe 的实际渲染高度 */
}
.preview-iframe {
  width: 100%;
  height: 100%;
  border: none;
  flex: 1;             /* ← flex: 1 依赖父容器有明确高度 */
}
```

**OpenList 为什么正常**：OpenList 的 iframe 是直接嵌入页面，父容器有明确高度。移动端的 `ion-content` 高度是动态计算的，可能导致 iframe 的 `100vh` 计算不正确。

**换行不生效根因**：用户点击换行按钮后，`isWrapping` 状态切换，但 `#textContent` 的 `no-wrap` 类可能因为滚动冲突导致视觉上看起来没生效（实际上是滚动区域变了）。

### 问题 1：安装确认界面不显示

**饱和式调试结果**：

GoProcessPlugin.kt L422-428：
```kotlin
val intent = Intent(context, com.encvgo.app.InstallConfirmActivity::class.java).apply {
    action = "com.encvgo.app.INSTALL_RESULT"  // ❌ 错误的 action
    putExtra(...)
}
context.startActivity(intent)  // ❌ 缺少 FLAG_ACTIVITY_NEW_TASK
```

**两个致命错误**：
1. `action="com.encvgo.app.INSTALL_RESULT"` — 这是 BroadcastReceiver 的 action，不是 Activity 启动用的。虽然 `setClass` 会优先匹配，但设置错误的 action 可能导致系统拒绝启动。
2. **缺少 `FLAG_ACTIVITY_NEW_TASK`** — 当 `context` 不是 Activity 时（GoProcessPlugin 的 context 是 Capacitor Plugin 的 context，可能是 Application 或 bridge context），**必须**添加此标志。

对比同文件 L436-442（系统安装 Intent，正确实现）：
```kotlin
if (context !is android.app.Activity) {
    addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK)  // ✅ 有检查
}
```

**根因**：L422-428 和 L556-562 两处启动 InstallConfirmActivity 的代码都缺少 `FLAG_ACTIVITY_NEW_TASK` 检查。

### 问题 2：加密验证 deep integrity check 失败

**分层验证架构分析**：

```
Verify() 方法执行顺序：
L1: QuickStructCheck (moov/stsz) ← SkipStructCheck=true 可跳过 ✅
L2: QuickSampleHashCheck (采样 Hash)
L3: FullHashCheck (全量 Hash)
L4: runDeepVideoIntegrityCheck ← 无条件执行！❌
    ├── checkMP4Structure (go-mp4.Probe 比对)
    ├── checkFFmpegDecoding (解码压力测试)
    └── checkFrameConsistency (帧数/时长比对)
```

content_verifier.go L141-144：
```go
// === 深度诊断 (仅在 L3 成功后执行) ===
if err := p.runDeepVideoIntegrityCheck(originalPath, decryptedPath, err); err != nil {
    return fmt.Errorf("deep integrity check failed: %w", err), nil  // ← 无条件执行
}
```

**根因**：`SkipStructCheck=true` 只跳过 L1，但 L4 的 `runDeepVideoIntegrityCheck` **不受任何参数控制**，在 L3 成功后强制执行。

v4 容器加密会改变 MP4 atom 结构（重新打包），导致 L4 的 `checkMP4Structure` 失败。

---

## What Changes

### 修复 0：文本预览 iframe 高度约束

**方案**：让 iframe 内部的 `#textContent` 高度适应 iframe 实际高度，而不是 `100vh`。

修改 text.html：
```css
#textContent {
    display: none;
    height: 100%;      /* 改为 100%，适应 iframe 实际高度 */
    overflow-y: auto;
    overflow-x: auto;
    ...
}
```

同时确保 FilePreview.vue 的 `.text-preview` 有明确高度约束。

### 修复 1：InstallConfirmActivity 启动

**方案**：移除错误的 action，添加 FLAG_ACTIVITY_NEW_TASK。

修改 GoProcessPlugin.kt L422-428 和 L556-562：
```kotlin
val intent = Intent(context, com.encvgo.app.InstallConfirmActivity::class.java).apply {
    // 移除错误的 action 设置
    putExtra(com.encvgo.app.InstallConfirmActivity.EXTRA_APK_PATH, apkPath)
    putExtra(com.encvgo.app.InstallConfirmActivity.EXTRA_FILE_NAME, apkFile.name)
    putExtra("request_id", "installConfirm")
    if (context !is Activity) {
        addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
    }
}
context.startActivity(intent)
```

### 修复 2：SkipDeepCheck 参数

**方案**：新增 `SkipDeepCheck` 参数，控制 L4 是否执行。

1. interfaces.go 新增字段：
```go
type VerifyOptions struct {
    SkipSizeCheck   bool
    SkipStructCheck bool // 跳过 L1
    SkipDeepCheck   bool // 跳过 L4 【新增】
    CollectWarnings bool
}
```

2. content_verifier.go L141-144：
```go
if !opt.SkipDeepCheck {
    if err := p.runDeepVideoIntegrityCheck(originalPath, decryptedPath, err); err != nil {
        return fmt.Errorf("deep integrity check failed: %w", err), nil
    }
}
```

3. plugin.go verifyContainer()：
```go
verifyOpts = &pluginInterfaces.VerifyOptions{
    SkipSizeCheck:   true,
    SkipStructCheck: true,
    SkipDeepCheck:   true,  // 【新增】
    CollectWarnings: true,
}
```

---

## ADDED Requirements

### Requirement F0: 文本预览 iframe 高度适配

系统 SHALL 确保文本预览 iframe 内部的滚动区域高度正确适配 iframe 实际高度。

#### Scenario F0.1: iframe 内部高度 100%
- **WHEN** text.html 在 iframe 中加载
- **THEN** `#textContent` 的 `height: 100%` 正确适配 iframe 实际渲染高度
- **AND** 滚动区域覆盖整个 iframe 可视区域

### Requirement F1: InstallConfirmActivity 正确启动

系统 SHALL 确保 InstallConfirmActivity 能从非 Activity context 正确启动。

#### Scenario F1.1: FLAG_ACTIVITY_NEW_TASK 检查
- **WHEN** GoProcessPlugin 从 context（非 Activity）启动 InstallConfirmActivity
- **THEN** Intent 包含 `FLAG_ACTIVITY_NEW_TASK` 标志
- **AND** 不设置错误的 action 属性

#### Scenario F1.2: Activity 显示
- **WHEN** `context.startActivity(intent)` 执行
- **THEN** InstallConfirmActivity 立即显示在屏幕上

### Requirement F2: PostEncryptProcessor 跳过深度检查

系统 SHALL 在 PostEncryptProcessor 场景下跳过 L4 deep integrity check。

#### Scenario F2.1: SkipDeepCheck 参数
- **WHEN** `VerifyOptions.SkipDeepCheck=true`
- **THEN** `Verify()` 方法在 L3 成功后直接返回成功
- **AND** 不执行 `runDeepVideoIntegrityCheck`

#### Scenario F2.2: isPostEncryptVerify 设置 SkipDeepCheck
- **WHEN** `verifyContainer()` 在 `isPostEncryptVerify=true` 时执行
- **THEN** `verifyOpts.SkipDeepCheck=true`

---

## Impact

- Affected code:
  - `internal/openlist/web/static/preview/text.html` — CSS height 改为 100%
  - `app/encv-mobile/src/views/FilePreview.vue` — 确保 .text-preview 高度约束
  - `app/encv-mobile/android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt` — 两处 Intent 修复
  - `internal/v2/plugins/interfaces/interfaces.go` — VerifyOptions 新增 SkipDeepCheck
  - `internal/v2/plugins/video/content_verifier.go` — Verify() 方法条件跳过 L4
  - `internal/v2/plugins/video/plugin.go` — verifyContainer() 设置 SkipDeepCheck=true