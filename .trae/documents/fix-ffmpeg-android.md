# 修复 Android 上 ffmpeg/ffprobe/mkvmerge 不可用的问题

## 问题根因

Android 上没有以下 CLI 工具：
- **ffprobe**：元数据提取（时长、格式、章节、关键帧）
- **ffmpeg**：编码（H.264/AAC）、remux（fast-start MP4）
- **mkvmerge/mkvinfo/mkvextract**：MKV 容器操作

mpv-android-lib AAR 中的 ffmpeg .so 是**解码专用**的，不支持编码，且没有 CLI 可执行文件。

## 方案

### 1. ffmpeg/ffprobe：使用 Static 版本可执行文件

从 `KaluaBilla/ffmpeg-android` 下载 **Static** 版本（完全自包含，不依赖 .so，包含 libx264 编码器）。

- 改 `setup-ffmpeg-cli.sh`：`Dynamic` → `Static`
- 以 `libffmpeg_exec.so` / `libffprobe_exec.so` 放入 jniLibs
- EncvGoService 启动时复制到 `filesDir/bin/` 并设置 PATH

### 2. mkvmerge/mkvinfo/mkvextract：用 Go 原生库替代

MKV 基于 EBML 格式，完全开源。有两个可用的 Go 库：

- **`github.com/at-wat/ebml-go/mkvcore`** (Apache-2.0)：支持 MKV 读写，SimpleBlockReader/SimpleBlockWriter
- **`github.com/coding-socks/matroska`** (MIT)：完整的 Matroska 类型定义，读取和提取

用 Go 原生库替代所有 mkvtoolnix CLI 调用，**零外部依赖**，Android 和桌面端统一代码。

#### 当前 mkvtoolnix 使用场景 → Go 库替代方案

| 场景 | 当前 CLI | Go 替代方案 |
|------|---------|------------|
| 识别轨道 | `mkvmerge -J` | ffprobe `-show_streams`（已有） |
| 检查 Cues 元素 | `mkvinfo -v` + grep `"\|+ Cues"` | ebml-go 解析 EBML 结构，查找 Cues Element ID |
| 提取关键帧偏移 | `mkvextract cues` | ebml-go 直接解析 Cues 元素，读取 CueClusterPosition |
| 提取章节 | `mkvextract chapters` | matroska 库读取 Chapters 元素 |
| MKV remux+Cues | `mkvmerge --cues` | ebml-go 读取所有 Cluster+Block，重新写入带 Cues 的 MKV |
| 合并分片 | `mkvmerge -o ... +` | ebml-go 读取多个文件的 Cluster，按时间戳合并写入 |
| 验证完整性 | `mkvinfo -P` | ffprobe 检查 duration > 0 |
| 检查分片链接 | `mkvinfo -p` + grep PrevUID/NextUID | ebml-go 解析 SegmentInfo 中的 PrevUID/NextUID |

### 3. 实施步骤

#### Step 1: 修改 setup-ffmpeg-cli.sh
- 下载 Static 版本：`ffmpeg-8.0-e05f8ac-Static-android-arm64-v8a.zip`
- 以 `libffmpeg_exec.so` / `libffprobe_exec.so` 放入 jniLibs

#### Step 2: 添加 Go MKV 处理库
创建 `internal/v2/plugins/video/mkv_native.go`，使用 `ebml-go` + `matroska` 库实现：

1. **`nativeCheckCues(filePath string) (bool, error)`**
   - 打开文件，解析 EBML 结构
   - 查找 Cues 元素（EBML ID: 0x1C53BB6B）
   - 返回是否存在

2. **`nativeExtractKeyFrameOffsets(filePath string) ([]uint64, error)`**
   - 解析 Cues 元素中的 CuePoint → CueClusterPosition
   - 加上 Segment 起始偏移量得到全局偏移

3. **`nativeExtractChapters(filePath string) ([]MKVChapterInfo, error)`**
   - 使用 matroska 库读取 Chapters 元素
   - 转换为 MKVChapterInfo 结构

4. **`nativeRemuxWithCues(inputPath, outputPath string, cuesMode string) error`**
   - 读取源 MKV 的所有轨道定义、Cluster、Block
   - 重新写入新的 MKV 文件
   - 根据关键帧信息生成 Cues 元素

5. **`nativeMergeSplitParts(paths []string, outputPath string) error`**
   - 读取多个 MKV 文件的 Cluster
   - 按时间戳偏移合并写入
   - 生成合并后的 Cues

6. **`nativeCheckSplitInfo(filePath string) (segmentUID, prevUID, nextUID []byte, isSplit bool, error)`**
   - 解析 SegmentInfo 中的 SegmentUID、PrevUID、NextUID

7. **`nativeVerifyMKV(filePath string) error`**
   - 解析 EBML 头和 Segment 结构
   - 验证基本完整性

#### Step 3: 添加降级逻辑
在 `mkvtoolnix.go` 中，每个函数先尝试 Go 原生实现，失败时回退到 CLI：

```go
func extractKeyFrameOffsetsFromMKV(filePath string) ([]uint64, error) {
    // 优先使用 Go 原生实现
    offsets, err := nativeExtractKeyFrameOffsets(filePath)
    if err == nil && len(offsets) > 0 {
        return offsets, nil
    }
    // 回退到 CLI（桌面端）
    return extractKeyFrameOffsetsFromMKVCli(filePath)
}
```

#### Step 4: 更新 content_preprocessor.go
- `remapWithMKVMerge` → 优先调用 `nativeRemuxWithCues`，失败回退 `mkvmerge` CLI
- `ExtractChaptersWithMKVExtract` → 优先调用 `nativeExtractChapters`，失败回退 `mkvextract` CLI

#### Step 5: 验证
- `go build ./internal/...` 编译通过
- `vue-tsc --noEmit` 前端类型检查通过

## 修改文件清单

1. `/workspace/app/encv-mobile/scripts/setup-ffmpeg-cli.sh` — Dynamic → Static
2. `/workspace/internal/v2/plugins/video/mkv_native.go` — 新建，Go 原生 MKV 处理
3. `/workspace/internal/v2/plugins/video/mkvtoolnix.go` — 添加降级逻辑
4. `/workspace/internal/v2/plugins/video/content_preprocessor.go` — remapWithMKVMerge 降级
5. `/workspace/go.mod` / `go.sum` — 添加 ebml-go、matroska 依赖
