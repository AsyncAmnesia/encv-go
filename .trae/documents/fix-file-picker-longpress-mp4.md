# 文件选择器 + 长按菜单 + MP4 播放修复

## 问题 1：新建加密解密任务只能输入路径，需要支持文件选择

### 现状

`Tasks.vue` 的"新建任务"弹窗只有一个文本输入框让用户手动输入路径，体验极差。

### 方案

在路径输入框旁添加"浏览"按钮，点击后跳转到 Files 页面的"选择模式"。用户在 Files 页面选择文件/文件夹后，路径自动回填到任务表单。

实现方式：
1. **路由参数传递**：跳转 Files 页面时带上 `?picker=true` 查询参数
2. **Files.vue 选择模式**：当 `picker=true` 时，点击文件/文件夹不打开/导航，而是选中并返回路径
3. **路径回填**：使用 `eventBus` 或 `router.back()` + 共享状态传递选中的路径

具体改动：
- `Tasks.vue`：路径输入框添加"浏览"按钮，跳转 `/tabs/files?picker=true`
- `Files.vue`：检测 `picker` 模式，点击时选中并返回路径（而非打开文件/导航目录）
- 添加 `useFilePicker` composable 管理选择状态

## 问题 2：文件界面长按文件夹或文件需要有对应操作菜单

### 现状

`Files.vue` 的文件/文件夹只支持单击操作（打开文件或导航目录），没有长按菜单。

### 方案

使用 Ionic 的 `ion-action-sheet` 组件，在长按时弹出操作菜单。

菜单选项根据文件类型动态生成：

**文件夹**：
- 打开（导航到该目录）
- 加密（将整个目录加密为 .encv 容器）

**普通文件**：
- 预览/播放（根据文件类型）
- 加密
- 删除

**加密文件（.encv）**：
- 播放（解密播放）
- 解密
- 删除

实现方式：
1. 在 `ion-item` 上添加 `@longpress` 事件
2. 弹出 `ion-action-sheet` 显示操作选项
3. 根据用户选择执行对应操作

具体改动：
- `Files.vue`：添加 `@longpress` 事件处理，弹出 action sheet
- 添加 i18n 文案

## 问题 3：无法播放普通 MP4 视频

### 根因（从 logcat 确认）

logcat 显示 `/stream` 端点返回 HTTP 500，错误在 `server_handle.go:143`（`serveEncryptedFile` 的错误处理）。这说明 **部署的 APK 不包含最新的 `DetectContainer` 降级修复**。

当前代码中 `handleStreamRequest` 已有 `DetectContainer` 检查（第 106-111 行），非 ENCV 文件会走 `http.ServeFile` 降级路径。但 logcat 中没有出现 `"File is not an ENCV container, serving raw file"` 日志，确认旧 APK 仍在运行。

### 方案

代码修复已完成，但需要确认 `http.ServeFile` 在 Android 上能正确提供视频文件（支持 Range 请求）。`http.ServeFile` 内置支持 Range 请求，Go 标准库会自动处理 `Range` 头，无需额外代码。

但有一个潜在问题：ArtPlayer 的 `video-player` 容器设置了 `max-height: 40vh`，在移动端可能太小。应改为自适应高度。

具体改动：
- `Player.vue`：移除 `max-height: 40vh` 限制，改为 `aspect-ratio` 或自适应高度

## 文件变更清单

| 文件 | 变更 |
|------|------|
| `app/encv-mobile/src/composables/useFilePicker.ts` | 新建：文件选择状态管理 |
| `app/encv-mobile/src/views/Tasks.vue` | 路径输入框添加"浏览"按钮，接收选择结果 |
| `app/encv-mobile/src/views/Files.vue` | 添加 picker 模式支持 + 长按 action sheet |
| `app/encv-mobile/src/views/Player.vue` | 修复视频容器高度 |
| `app/encv-mobile/src/composables/useI18n.ts` | 添加操作菜单相关 i18n 文案 |
