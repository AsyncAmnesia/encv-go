# Tasks

本 spec 为评估文档。如决定执行推荐方案（Go 层面拆分），以下为实施任务清单。

## Phase 0: 决策点（用户确认）
- [ ] **决策**: 是否采用 Go 层拆分方案（方案 A）？还是有其他方向？

## Phase 1: 创建 MKV 插件骨架（如果采用方案 A）

- [ ] Task 1: 创建 MKV 插件包结构
  - [ ] 新建 `internal/v2/plugins/mkv/` 目录
  - [ ] 创建 `plugin.go` — 实现 Plugin 接口（Name="mkv", GetDefaultSettings 等）
  - [ ] 创建 `types.go` — 从 video/types.go 迁移 MKV 相关结构体（MkvInfo, MKVChapterInfo, MKVMergeIdentity, MKVChapterXML, TrackInfo, MkvmergeInfo）
  - [ ] 创建 `mkv_plugin_config.go` — MKV 插件配置（缓存目录等）

- [ ] Task 2: 迁移 MKV 核心函数
  - [ ] 从 `video/mkvtoolnix.go` 迁移 splitter 相关：`IsMkvPartOfSplit`, `batchGetMkvInfos`, `getMkvInfo`, `getMkvInfoNative`, `groupSplitParts`, `findAndSortChain`
  - [ ] 迁移 merger 相关：`mergeSplitPartsFromSet`, `mergeSplitPartsWithFFmpeg`, `getCachedMergedPath`, `saveToCache`, `calculateSplitSetHash`
  - [ ] 迁移 chapter 相关：`ExtractChaptersWithMKVExtract`
  - [ ] 迁移 keyframe 相关：`extractKeyFrameOffsetsFromMKV`, `extractKeyFrameOffsetsFromMKVCues`, `checkFileForCues`, `parseMKVInfoForKeyFrames`, `getSegmentPosition`, `getVideoTrackID`, `getVideoTrackIDWithFFProbe`
  - [ ] 迁移 identify 相关：`IdentifyWithMKVMerge`（或删除，当前是 dead code）

- [ ] Task 3: Video Plugin 改为调用 MKV Plugin
  - [ ] 在 VideoPlugin 中注入 MKV Plugin 实例
  - [ ] 替换 video/mkvtoolnix.go 中的直接调用为 mkv.Plugin 方法调用
  - [ ] 保留 video/mkvtoolnix.go 作为薄委托层（或完全删除）

- [ ] Task 4: 注册新插件
  - [ ] 修改 `internal/v2/plugins/registry.go`，注册 MKVPlugin
  - [ ] 更新配置 schema 如需新增 MKV 插件配置段

## Phase 2: 验证

- [ ] Task 5: 编译验证
  - [ ] `go build ./internal/...` 通过
  - [ ] `go test ./internal/v2/...` 全部通过
  - [ ] `CGO_ENABLED=0 GOOS=android go build ./internal/v2/...` 通过

- [ ] Task 6: 功能验证
  - [ ] MKV 分片检测功能正常
  - [ ] MKV 合并功能正常
  - [ ] 视频加密/解密端到端流程正常（MKV 拆分不影响现有功能）

## 如果选择探索 ComboLite（独立于上述任务）

- [ ] Task C1: 调研 Android 端可用的 MKV 解析库（纯 Java/Kotlin）
- [ ] Task C2: 评估 ComboLite 集成成本（Gradle 配置、签名、打包流程）
- [ ] Task C3: 设计 Go 后端 ↔ Android 插件的 IPC 方案
- [ ] Task C4: 原型验证（PoC）
