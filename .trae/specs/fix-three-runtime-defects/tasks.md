# Tasks

- [ ] Task 1: 修复文本预览默认换行模式无法滚动
  - [ ] 1.1 读取 FilePreview.vue 完整 style 部分
  - [ ] 1.2 将 `.text-preview` 的 `overflow: auto` 改为 `overflow: hidden`（或移除 overflow 属性）
  - [ ] 1.3 确认 `.preview-iframe` 保持 `width: 100%; height: 100%; flex: 1; border: none` 不变
  - [ ] 1.4 确认不影响 PDF 预览（`.pdf-preview` 也用 iframe）

- [ ] Task 2: 修复安装确认界面不显示 + 超时问题
  - [ ] 2.1 读取 GoProcessPlugin.kt 中 installPlugin/installFromPath 的 activity 启动代码
  - [ ] 2.2 在 `activity.startActivityForResult()` 前添加 null 检查，null 时 fallback 到直接安装
  - [ ] 2.3 读取 InstallConfirmActivity.kt，在 onCreate 的 setContent 外层添加 try-catch
  - [ ] 2.4 检查 ExtensionsPage.vue 的 120s 超时是否合理，考虑是否需要调整

- [ ] Task 3: 修复 v4 加密 stsz box missing 错误
  - [ ] 3.1 读取 plugin.go verifyContainer 中 sourcePath 判断逻辑
  - [ ] 3.2 读取 registry.go 中容器版本信息如何传递到 VideoPlugin
  - [ ] 3.3 修改判断条件：当容器版本为 v4 时也启用 SkipStructCheck（不仅依赖 sourcePath != inputPath）
  - [ ] 3.4 或者更彻底：对 PostEncryptProcessor 场景始终使用 SkipStructCheck=true（加密后结构必然变化）

- [ ] Task 4: 修复 v3 ffprobe JSON 解析失败
  - [ ] 4.1 读取 metadata_extractor.go 中 ffmpeg.Probe() 调用和 FFProbeRawMetadata 解析逻辑
  - [ ] 4.2 读取 ffmpeg.Probe() 函数的实际实现（确认参数传递）
  - [ ] 4.3 对 json.Unmarshal 失败增加容错：记录原始输出、尝试备选解析策略、或跳过 metadata 提取而非整个加密流程失败

# Task Dependencies
- [Task 1] 可独立并行
- [Task 2] 可独立并行
- [Task 3] 和 [Task 4] 可并行
