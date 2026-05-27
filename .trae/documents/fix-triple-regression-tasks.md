# Tasks

- [ ] Task 1: 回退 Files.vue 虚拟滚动改动，恢复原始文件列表渲染
  - [ ] 删除 Files.vue 中 `useVirtualizer` 导入（约 L415）
  - [ ] 删除 `VIRTUAL_SCROLL_CONFIG` 导入
  - [ ] 删除 `shouldUseVirtualScroll` computed 变量（约 L508-510）
  - [ ] 删除 `virtualizerRef`, `rowVirtualizer`, `virtualItems`, `watch(displayFiles.length)` 等虚拟滚动相关变量（约 L512-526）
  - [ ] 删除 `<template v-if="shouldUseVirtualScroll">...</template>` 整个块（L183-239）— 包含 virtual-scroll-container div 和所有 virtual-item 渲染
  - [ ] 将 `<ion-list v-else>` 改回 `<ion-list>`（移除 v-else，因为不再有配对的 v-if）
  - [ ] 恢复 `loadFiles()` 函数：将 `listFilesStream` 调用改回 `listFiles` 调用（移除流式加载逻辑，恢复原始一次性加载模式）
    - 注意：**保留后端 SSE 接口和前端 listFilesStream 函数不删除**（它们本身没问题，只是 Files.vue 不应该用它们做全局列表加载。后续可选择性用于插件列表）
  - [ ] 验证：运行 `cd /workspace/app/encv-mobile && npx vue-tsc --noEmit && npx vite build`
  - [ ] 验证：grep Files.vue 确认无 `useVirtualizer|virtualizerRef|rowVirtualizer|shouldUseVirtualScroll|VIRTUAL_SCROLL_CONFIG` 残留

- [ ] Task 2: 修复 ffprobe/ffmpeg argv[0] 缺失问题
  - [ ] 修改 `internal/utils/ffmpeg_dlopen.go` 的 `CallFFmpegNative()`：
    - 在构建 argv 之前，创建一个新 slice：`fullArgs := make([]string, len(args)+1)`
    - `fullArgs[0] = "ffmpeg"`
    - `copy(fullArgs[1:], args)`
    - 后续使用 `fullArgs` 替代 `args` 构建 argv
  - [ ] 同样修改 `CallFFprobeNative()`：
    - prepend `"ffprobe"` 作为 argv[0]
  - [ ] 验证：grep 确认两个函数中 argc = C.int(len(fullArgs)) 且 fullArgs[0] 为工具名
  - [ ] 验证：`mise exec -- go build ./internal/utils/`

- [ ] Task 3: MPV 插件安装增加用户反馈
  - [ ] 读取 `GoProcessPlugin.kt` 的 `installPlugin()` 方法（L363-410）
  - [ ] 当 `Class.forName("com.combo.core.runtime.PluginManager")` 抛出 ClassNotFoundException 时：
    - 除了 `Log.w` 外，调用 `call.reject("ComboLite PluginManager not available on this device")` 返回明确错误给前端
  - [ ] 当反射方法查找失败或 invoke 失败时：
    - 确保 `call.reject()` 被调用并包含异常信息
  - [ ] 验证：所有 code path 都有 resolve 或 reject

# Task Dependencies
- [Task 1] 无依赖 — 最高优先级（页面崩溃阻塞用户使用）
- [Task 2] 无依赖 — 高优先级（加密视频功能完全不可用）
- [Task 3] 无依赖 — 中优先级（插件安装功能缺失但非核心阻塞）
- 三个任务可并行执行
