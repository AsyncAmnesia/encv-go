# 修复三问题：Files页面崩溃 + ffprobe参数顺序错误 + MPV插件无反馈

## Why
上一轮修改引入了三个新问题：
1. **Files.vue 页面崩溃**：虚拟滚动改动应用到**全部文件列表**（用户明确说"全部文件本来是没有性能问题的"），且模板结构改动导致 `<ion-page>` 警告 + 页面无法进入。需要回退 Files.vue 的虚拟滚动改动，仅对**插件类型文件列表**做优化。
2. **ffprobe 参数顺序错误**：stderr 捕获修复后暴露了真正的根因——`CallFFprobeNative()` 没有在 argv 前面加 `"ffprobe"` 程序名，导致 FFmpeg 的 `ffprobe_run()` 把用户第一个参数 `-v` 当程序名、`quiet` 当输入文件名，参数解析完全错位。报错 `"Argument '...mp4' provided as input filename, but 'quiet' was already specified"` 是典型症状。
3. **MPV 插件安装无反馈**：`enabled.set(true)` 改了但反射调用 `com.combo.core.runtime.PluginManager` 可能因类不存在而静默失败（catch 后只 log.w 无用户提示）。

## What Changes

### 问题 1：回退 Files.vue 虚拟滚动 + 仅优化插件列表
- **删除** Files.vue 中 `shouldUseVirtualScroll` / `rowVirtualizer` / `virtualItems` / `virtualizerRef` 等所有虚拟滚动相关代码
- **恢复** 原始的 `<ion-list v-for="file in displayFiles">` 全量渲染
- 插件文件列表（`filteredPluginFiles`）保持原样（通常数量可控，不需要虚拟滚动）
- 如果未来确实需要优化插件列表性能，应单独针对 `filteredPluginFiles` 做 pagination 或虚拟滚动，而不是改全局 Files 页面

### 问题 2：修复 ffprobe/ffmpeg argv[0] 缺失
- **`internal/utils/ffmpeg_dlopen.go`**：
  - `CallFFmpegNative()`：argv 构建时 prepend `"ffmpeg"` 作为 argv[0]
  - `CallFFprobeNative()`：argv 构建时 prepend `"ffprobe"` 作为 argv[0]
  - argc 相应 +1
- 这是 **范式级修复**——所有通过 dlopen 调用 `*_run()` 的 FFmpeg 工具都必须遵循 C 标准 `main(argc, argv)` 约定，argv[0] = 程序名

### 问题 3：MPV 插件安装增加用户反馈
- **`android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt`**：
  - `installPlugin()` 方法中，当 `Class.forName("com.combo.core.runtime.PluginManager")` 失败时，除了 `Log.w` 还需调用 `call.reject()` 返回明确错误给前端
  - 当反射调用 `installMethod.invoke()` 成功时，确认返回值并给用户成功提示
  - 当前 catch 块只 Log.e 但可能吞掉了异常信息

## Impact
- Affected code:
  - `app/encv-mobile/src/views/Files.vue` — 回退虚拟滚动
  - `internal/utils/ffmpeg_dlopen.go` — argv[0] 修复
  - `app/encv-mobile/android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt` — 错误反馈改进
- **BREAKING**: Files.vue 的虚拟滚动功能完全移除（后续如需应只作用于插件子列表）

## ADDED Requirements

### Requirement: FFmpeg/ffprobe argv 必须包含程序名
通过 dlopen 调用 `*_run()` 时，argv[0] 必须是工具程序名（`"ffmpeg"` / `"ffprobe"`），符合 C main() 标准约定。

#### Scenario: ffprobe 正确解析参数
- **WHEN** 调用 `ffprobe.Probe("-v", "quiet", "-print_format", "json", path)`
- **THEN** 实际传递给 `ffprobe_run()` 的 argv 为 `["ffprobe", "-v", "quiet", "-print_format", "json", path]`
- **AND** ffprobe 不再报 "quiet was already specified" 错误

### Requirement: Files 页面必须正常渲染
Files.vue 在非插件模式下必须正常显示文件列表，不能有 `<ion-page>` 缺失警告或白屏。

#### Scenario: 进入文件管理页
- **WHEN** 用户导航到 `/tabs/files`
- **THEN** 文件列表正常渲染，无 Ionic 警告
- **AND** 文件点击、长按、搜索等功能正常工作

### Requirement: MPV 插件安装必须有明确的成功/失败反馈
无论 combolite PluginManager 是否可用，用户必须收到安装结果通知。

#### Scenario: 安装 MPV 插件
- **WHEN** 用户触发 MPV 插件安装
- **THEN** 若成功 → 显示成功 toast；若失败 → 显示包含具体原因的错误 toast

## MODIFIED Requirements

### Requirement: CallFFmpegNative / CallFFprobeNative 参数构建
两个函数的 argv 构建逻辑从 `args 直接传递` 改为 `prepend 工具名 + args`，argc 同步 +1。

## REMOVED Requirements

### Requirement: Files.vue 全局虚拟滚动
**Reason**: 用户明确指出"全部文件本来是没有性能问题的"，虚拟滚动被错误地应用于全局文件列表导致页面崩溃。需要优化的仅是插件类型文件列表（该列表当前使用普通 v-for，数据量可控）。
**Migration**: 完全移除 Files.vue 中的 useVirtualizer 导入和虚拟滚动模板代码，恢复原始 `<ion-list v-for>` 结构。后续如需性能优化应针对 `filteredPluginFiles` 独立实施。
