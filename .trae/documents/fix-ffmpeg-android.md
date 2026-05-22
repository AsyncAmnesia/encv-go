# 修复 Android 上 ffmpeg/ffprobe/mkvmerge 不可用的问题

## 问题根因

Android 上没有以下 CLI 工具：
- **ffprobe**：元数据提取（时长、格式、章节、关键帧）
- **ffmpeg**：编码（H.264/AAC）、remux（fast-start MP4）
- **mkvmerge/mkvinfo/mkvextract**：MKV 容器操作

mpv-android-lib AAR 中的 ffmpeg .so 是**解码专用**的，不支持编码，且没有 CLI 可执行文件。

## 方案

### 1. ffmpeg/ffprobe：打包 Static 可执行文件

**为什么不用 JNI 方式？** Go 后端通过 `exec.Command` 调用 ffmpeg/ffprobe，需要可执行文件。JNI 方式需要从 Kotlin 层封装，Go 后端无法直接调用。

**为什么用 Static 版本？** Dynamic 版本依赖 .so 文件，与 mpv 捆绑的 .so 冲突。Static 版本完全自包含。

**Android 上打包可执行文件的标准做法：**
- `AndroidFFmpeg`、`FFmpegX-Android`、`ffmpeg-kit` 等库都打包了 ffmpeg 可执行文件
- 运行时复制到 `filesDir` 并设置可执行权限
- 通过 `ProcessBuilder` 执行

**实施：**
1. CI 中下载 KaluaBilla/ffmpeg-android 的 **Static** 版本
2. 以 `libffmpeg_exec.so` / `libffprobe_exec.so` 命名放入 jniLibs（Android 只安装 .so 后缀文件）
3. EncvGoService 启动时复制到 `filesDir/bin/ffmpeg` / `filesDir/bin/ffprobe`，设置可执行权限
4. 设置 `ENCV_BIN_DIR` 环境变量，Go 后端通过 `utils.FFProbeCmd()` / `utils.FFmpegCmd()` 使用完整路径

**体积：** Static ffmpeg ~75MB + ffprobe ~75MB = ~150MB。这是 Android 上使用 ffmpeg 编码功能的代价。

### 2. mkvmerge/mkvinfo/mkvextract：用 Go 原生库替代

MKV 基于 EBML 格式，完全开源。用 Go 原生库替代所有 mkvtoolnix CLI 调用，**零外部依赖**。

**可用 Go 库：**
- `github.com/at-wat/ebml-go/mkvcore` (Apache-2.0) — MKV 读写，SimpleBlockReader/SimpleBlockWriter
- `github.com/coding-socks/matroska` (MIT) — 完整 Matroska 类型定义，读取和提取

**替代方案对照表：**

| 场景 | 当前 CLI | Go 替代方案 |
|------|---------|------------|
| 识别轨道 | `mkvmerge -J` | ffprobe `-show_streams`（已有） |
| 检查 Cues 元素 | `mkvinfo -v` + grep | ebml-go 解析 EBML，查找 Cues Element ID (0x1C53BB6B) |
| 提取关键帧偏移 | `mkvextract cues` | ebml-go 解析 Cues → CueClusterPosition |
| 提取章节 | `mkvextract chapters` | matroska 库读取 Chapters 元素 |
| MKV remux+Cues | `mkvmerge --cues` | ebml-go 读 Cluster+Block，重写带 Cues 的 MKV |
| 合并分片 | `mkvmerge -o ... +` | ebml-go 读多个文件的 Cluster，按时间戳合并写入 |
| 验证完整性 | `mkvinfo -P` | ffprobe 检查 duration > 0 |
| 检查分片链接 | `mkvinfo -p` + grep PrevUID/NextUID | ebml-go 解析 SegmentInfo 的 PrevUID/NextUID |

**降级策略：** 每个函数优先使用 Go 原生实现，失败时回退到 CLI（桌面端 mkvtoolnix 仍可用）。

### 3. 实施步骤

#### Step 1: 修改 setup-ffmpeg-cli.sh
- 下载 Static 版本：`ffmpeg-8.0-e05f8ac-Static-android-arm64-v8a.zip`
- 以 `libffmpeg_exec.so` / `libffprobe_exec.so` 放入 jniLibs

#### Step 2: 添加 Go MKV 处理库
创建 `internal/v2/plugins/video/mkv_native.go`，使用 ebml-go + matroska 库实现：

1. `nativeCheckCues(filePath) (bool, error)` — 解析 EBML 查找 Cues 元素
2. `nativeExtractKeyFrameOffsets(filePath) ([]uint64, error)` — 解析 Cues → CueClusterPosition
3. `nativeExtractChapters(filePath) ([]MKVChapterInfo, error)` — matroska 库读取 Chapters
4. `nativeRemuxWithCues(inputPath, outputPath, cuesMode) error` — 重写带 Cues 的 MKV
5. `nativeMergeSplitParts(paths, outputPath) error` — 合并多个 MKV
6. `nativeCheckSplitInfo(filePath) (segmentUID, prevUID, nextUID []byte, isSplit bool, error)` — 解析 SegmentInfo
7. `nativeVerifyMKV(filePath) error` — 验证基本完整性

#### Step 3: 添加降级逻辑
在 `mkvtoolnix.go` 中，每个函数先尝试 Go 原生实现，失败时回退到 CLI。

#### Step 4: 更新 content_preprocessor.go
- `remapWithMKVMerge` → 优先调用 `nativeRemuxWithCues`
- `ExtractChaptersWithMKVExtract` → 优先调用 `nativeExtractChapters`

#### Step 5: 验证
- `go build ./internal/...` 编译通过
- `vue-tsc --noEmit` 前端类型检查通过

## 修改文件清单

1. `/workspace/app/encv-mobile/scripts/setup-ffmpeg-cli.sh` — Dynamic → Static
2. `/workspace/internal/v2/plugins/video/mkv_native.go` — 新建，Go 原生 MKV 处理
3. `/workspace/internal/v2/plugins/video/mkvtoolnix.go` — 添加降级逻辑
4. `/workspace/internal/v2/plugins/video/content_preprocessor.go` — remapWithMKVMerge 降级
5. `/workspace/go.mod` / `go.sum` — 添加 ebml-go、matroska 依赖
