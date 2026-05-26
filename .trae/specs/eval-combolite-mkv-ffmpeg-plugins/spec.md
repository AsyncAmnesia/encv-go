# 评估：使用 ComboLite 将 MKV 和 FFmpeg 重构为两个独立插件

## Why

当前 `video` 插件（~2000+ 行）承担了过多职责：FFmpeg 编解码、MKV 容器操作（分片检测/合并/章节/关键帧）、元数据提取等。需要评估是否应该拆分，以及 ComboLite 是否是合适的拆分技术方案。

## ComboLite 是什么

**ComboLite** (github.com/lnzz123/ComboLite) 是一个 **Android 端的 Kotlin 插件化框架**，用于在运行时动态加载 APK 形式的插件。核心特性：

- 面向 Jetpack Compose 的 Android UI 插件框架
- 0 Hook / 0 反射，基于官方公开 API（代理模式）
- 插件以 APK 形式打包，运行时从 assets 或网络加载
- 提供 PluginManager API 进行安装/卸载/生命周期管理
- 支持 Service 实例池、依赖解析、崩溃隔离

**关键约束**：ComboLite 纯粹是 **Android/Kotlin 层**的技术方案，不涉及 Go 后端。

---

## 当前架构分析

### video 插件的职责分布

| 文件 | 行数 | 职责 | 外部依赖 |
|------|------|------|----------|
| [plugin.go](file:///workspace/internal/v2/plugins/video/plugin.go) | ~620 | 加密/解密主流程、PostEncryptProcessor | crypto, container, ffmpeg |
| [content_preprocessor.go](file:///workspace/internal/v2/plugins/video/content_preprocessor.go) | ~400 | FFmpeg 转码（re-encode）、编码器检测 | **ffmpeg.Runner** (重) |
| [metadata_extractor.go](file:///workspace/internal/v2/plugins/video/metadata_extractor.go) | ~150 | 视频元数据提取 | **ffmpeg.Probe** (轻) |
| [content_verifier.go](file:////workspace/internal/v2/plugins/video/content_verifier.go) | ~120 | 加密后内容校验 | **ffmpeg.RunWithOutput** (重) |
| [mkvtoolnix.go](file:///workspace/internal/v2/plugins/video/mkvtoolnix.go) | ~780 | **MKV 全套操作** | **mkvmerge/mkvinfo/mkvextract CLI** (独占) |
| [subtitles.go](file:///workspace/internal/v2/plugins/video/subtitles.go) | ~100 | 字幕处理 | utils.CopyFile 等 |
| [types.go](file:///workspace/internal/v2/plugins/video/types.go) | ~80 | 数据结构定义 | 无 |

### MKV 操作的具体清单（mkvtoolnix.go）

| 函数 | 功能 | 依赖 CLI |
|------|------|----------|
| `getVideoTrackID` | 获取视频轨道 ID | mkvmerge |
| `IsMkvPartOfSplit` | 检测是否为分片 | mkvinfo |
| `ExtractChaptersWithMKVExtract` | 提取章节信息 | mkvextract |
| `checkFileForCues` | 检查 Cues 元素 | mkvinfo |
| `extractKeyFrameOffsetsFromMKV` | 提取关键帧偏移 | mkvinfo + mkvextract |
| `extractKeyFrameOffsetsFromMKVCues` | 从 Cues 提取关键帧 | mkvextract |
| `getSegmentPosition` | EBML 解析获取 Segment 位置 | 无（纯 Go） |
| `batchGetMkvInfos` / `getMkvInfo` | 批量获取 MKV 元信息 | mkvinfo |
| `groupSplitParts` | 分片分组（按 UID 链） | 无（纯 Go） |
| `mergeSplitPartsFromSet` | 合并分片集 | mkvmerge |
| `mergeSplitPartsWithFFmpeg` | FFmpeg concat 备选合并 | ffmpeg |
| `IdentifyWithMKVMerge` | MKV 详细识别 | mkvmerge (**DEAD CODE**) |

### FFmpeg 操作的具体清单

| 函数 | 功能 | Runner 方法 |
|------|------|-----------|
| `detectPreferredEncoder` | 检测可用编码器 | `ffmpeg.Run()` |
| `runFFmpegCmd` (content_preprocessor) | 执行转码命令 | `ffmpeg.RunWithOutput()` |
| `DetectVideoFormat` | 检测容器格式 | `ffmpeg.Probe()` |

### 已有的抽象层

刚刚完成的 **FFmpeg Runner 重构**已将平台差异封装到：
```
internal/utils/ffmpeg/
├── runner.go          — Runner 接口
├── exec_runner.go     — 桌面端 exec.Command
└── native_runner.go   — Android dlopen libffmpeg.so
```

---

## 评估维度

### 维度 1: 技术可行性

#### ComboLite 能覆盖的范围

| 组件 | 在 Go 后端 | 在 Android (ComboLite) | 可行性 |
|------|-----------|---------------------|--------|
| FFmpeg 转码 | ✅ 通过 Runner 接口调用 | ✅ 通过 JNI/dlopen 调用 libffmpeg.so | **可行但重复** |
| FFmpeg Probe | ✅ 通过 Runner 接口 | ✅ 同上 | **可行但重复** |
| MKV 分片检测 | ✅ mkvinfo CLI | ❌ **需要移植 mkvtoolnix 到 Android** | **困难** |
| MKV 分片合并 | ✅ mkvmerge CLI | ❌ **需要移植 mkvtoolnix 到 Android** | **困难** |
| MKV 章节提取 | ✅ mkvextract CLI | ❌ **同上** | **困难** |
| MKV 关键帧提取 | ✅ mkvinfo + mkvextract | ❌ **同上** | **困难** |
| EBML Segment 解析 | ✅ 纯 Go 实现 | ✅ 可直接移植到 Kotlin | **容易** |
| 加密/解密流程 | ✅ Go crypto 包 | ⚠️ 需要通过 IPC 调用 Go 后端 | **架构冲突** |

**核心问题**: MKV 操作严重依赖 mkvtoolnix CLI 工具套件（mkvmerge/mkvinfo/mkvextract）。这些工具：
- 在桌面端：作为系统命令可用
- 在 Android 端：**不存在**，需要自行编译或寻找替代方案
- ComboLite 不解决这个根本问题——它只是插件加载机制，不提供 MKV 库

#### 如果不用 ComboLite，替代方案是什么？

**方案 A: 在 Go 层面拆分为两个插件（推荐 ✅）**
```
internal/v2/plugins/
├── video/          ← 保留，仅负责加密/解密流程 + FFmpeg 转码
│   ├── plugin.go
│   ├── content_preprocessor.go   (FFmpeg re-encode)
│   ├── metadata_extractor.go      (ffprobe)
│   └── content_verifier.go        (verify)
├── mkv/            ← 新建，MKV 专用操作插件
│   ├── plugin.go                (MKV Plugin 接口实现)
│   ├── splitter.go              (分片检测/分组)
│   ├── merger.go                (分片合并)
│   ├── chapter.go               (章节提取)
│   └── keyframe.go              (关键帧提取)
└── registry.go    ← 注册两个独立插件
```

**方案 B: 使用 ComboLite 在 Android 端做 MKV 插件**
- 需要：将 mkvtoolnix 编译为 Android NDK 库（或找纯 Kotlin/Java 的 MKV 解析库）
- 需要：将 MKV 逻辑从 Go 移到 Kotlin
- 需要：通过 IPC（HTTP/API）让 Go 后端调用 Android 端的 MKV 插件
- **复杂度极高**，收益不明确

### 维度 2: 成本效益分析

| 方案 | 开发成本 | 维护成本 | 收益 |
|------|---------|---------|------|
| **A: Go 层拆分** | 中（~3-5 天） | 低（Go 类型系统保障） | 职责清晰；可独立测试；不影响现有 Android 架构 |
| **B: ComboLite** | 极高（~2-4 周） | 高（Kotlin + Go 双语言 + IPC） | Android 端可热更新 MKV 处理逻辑；但对后端无帮助 |
| **C: 维持现状** | 无 | 高（video 插件持续膨胀） | 无 |

### 维度 3: 架构匹配度

#### encv-go 的调用链路
```
Android App (Lynx/Kotlin)
    ↓ HTTP / direct call
Go Backend (server/mobile_service)
    ↓ TaskManager.Create()
Video Plugin (Go)
    ├─→ FFmpeg Runner → exec.Command / dlopen libffmpeg.so
    ├─→ mkvtoolnix.go → exec.Command("mkvmerge") ...
    └─→ crypto.Encrypt/Decrypt
```

**ComboLite 的适用场景**:
```
Android Host App (Kotlin/Compose)
    ↓ PluginManager.loadPlugin()
Plugin APK (dynamically loaded)
    ↓ IPluginEntryClass.Content()
Compose UI + Business Logic
```

**结论**: ComboLite 解决的是 **"Android App 内部如何动态加载功能模块"** 的问题。而 encv-go 的 MKV/FFmpeg 逻辑位于 **Go 后端**，不在 Android UI 层。

### 维度 4: ComboLite 对本项目的潜在价值

虽然 ComboLite 不适合做 MKV/FFmpeg 插件化，但它可能在以下场景有价值：

1. **前端播放器插件化**：如果未来有多种播放器（IJKPlayer、ExoPlayer、Artplayer），可以用 ComboLite 动态加载
2. **UI 主题/皮肤插件化**：动态更换界面主题
3. **预览器插件化**：不同文件类型的预览组件

但这些与当前的 MKV/FFmpeg 重构需求无关。

---

## 结论与建议

### 最终评估结论

| 评估项 | 评分 (1-5) | 说明 |
|--------|-----------|------|
| 技术可行性 | **2** | MKV CLI 工具在 Android 不可用是硬阻塞 |
| 架构匹配度 | **1** | ComboLite 是 Android UI 插件框架，与 Go 后端处理逻辑无关 |
| 成本效益 | **1** | 投入产出比极差 |
| 长期维护性 | **3** | 引入双语言会增加维护负担 |
| 团队学习成本 | **2** | 需要掌握 ComboLite 框架 + Gradle 打包体系 |

**综合评分: 1.8 / 5 — 不推荐使用 ComboLite 做 MKV/FFmpeg 插件化**

### 推荐方案: Go 层面拆分为两个插件（方案 A）

这是低风险、高收益的重构：

1. 新建 `internal/v2/plugins/mkv/` 包
2. 从 `video/mkvtoolnix.go` 迁移所有 MKV 相关函数
3. Video Plugin 通过接口调用 MKV Plugin 的能力
4. 两者都通过已有的 `ffmpeg.Runner` 接口调用 FFmpeg
5. **零 Android/Kotlin 改动**

### 如果未来仍想探索 ComboLite

建议单独立项，目标明确为 **"Android 端播放器/UI 插件化"**，而非后端处理逻辑插件化。
