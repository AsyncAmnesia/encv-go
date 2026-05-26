# Checklist

## 评估结论确认
- [ ] 用户已审阅评估结论并确认方向（Go 拆分 / ComboLite / 维持现状 / 其他）

## 如果采用 Go 层拆分方案

### MKV 插件创建
- [ ] `internal/v2/plugins/mkv/` 目录结构创建完成
- [ ] MKV Plugin 实现 Plugin 接口全部方法
- [ ] MKV 相关类型定义从 video 包迁移完成

### 函数迁移完整性
- [ ] 分片检测函数全部迁移（IsMkvPartOfSplit, batchGetMkvInfos, getMkvInfo, groupSplitParts 等）
- [ ] 分片合并函数全部迁移（mergeSplitPartsFromSet, mergeSplitPartsWithFFmpeg, 缓存逻辑等）
- [ ] 章节提取函数迁移（ExtractChaptersWithMKVExtract）
- [ ] 关键帧提取函数迁移（extractKeyFrameOffsetsFromMKV 系列）
- [ ] EBML 解析工具函数迁移（getSegmentPosition, parseMKVInfoForKeyFrames 等）

### Video Plugin 适配
- [ ] VideoPlugin 不再直接包含 MKV 处理逻辑
- [ ] VideoPlugin 通过接口调用 MKV Plugin
- [ ] mkvtoolnix.go 要么删除，要么变为纯委托

### 注册与配置
- [ ] registry.go 注册 MKVPlugin
- [ ] 配置 schema 更新（如需要）

### 编译验证
- [ ] 桌面端 `go build ./internal/...` 通过
- [ ] Android 交叉编译 `internal/v2/...` 通过
- [ ] 全量测试 `go test ./internal/v2/...` 通过

### 功能回归验证
- [ ] MKV 分片文件加密流程正常
- [ ] MKV 分片合并后加密流程正常
- [ ] 非 MKV 视频文件加密不受影响
- [ ] 解密/预览功能正常
