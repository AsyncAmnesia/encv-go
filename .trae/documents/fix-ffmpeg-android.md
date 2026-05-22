# 修复 Android 上 ffmpeg/ffprobe/mkvmerge 不可用的问题

## 问题根因

Android 上没有以下 CLI 工具：
- **ffprobe**：元数据提取（时长、格式、章节、关键帧）
- **ffmpeg**：编码（H.264/AAC）、remux（fast-start MP4）
- **mkvmerge/mkvinfo**：MKV 容器操作（合并、Cues 生成、验证）

mpv-android-lib AAR 中的 ffmpeg .so 是**解码专用**的，不支持编码，且没有 CLI 可执行文件。

## 方案

### 1. ffmpeg/ffprobe：使用 Static 版本

下载 KaluaBilla/ffmpeg-android 的 **Static** 版本（完全自包含，不依赖 .so，包含 libx264 编码器）。

- 改 `setup-ffmpeg-cli.sh`：`Dynamic` → `Static`
- Static 版本 ~75MB/个，APK 增加约 150MB
- 无 .so 冲突问题

### 2. mkvmerge/mkvinfo：用 ffmpeg 替代

在 Android（`ENCV_MOBILE=1`）环境下，mkvmerge 不可用。解决方案：

**MKV remux 场景**（`remapWithMKVMerge`）：
- 原逻辑：mkvmerge --cues 重新生成 Cues
- 替代方案：用 ffmpeg `-c copy` remux 为 MKV，虽然不生成 Cues，但 ffmpeg 的 MKV muxer 会自动生成基本的 Cues
- 如果 `keep_mkv` 选项开启但 mkvmerge 不可用，降级为 ffmpeg remux

**MKV 合并场景**（`mergeSplitParts`）：
- 原逻辑：mkvmerge 用 `+` 追加合并
- 替代方案：用 ffmpeg concat demuxer 合并

**mkvinfo 验证场景**：
- 原逻辑：mkvinfo -P 验证文件完整性
- 替代方案：用 ffprobe 验证（检查 duration > 0、有视频流等）

**mkvmerge -J 识别场景**：
- 原逻辑：mkvmerge -J 获取轨道信息
- 替代方案：用 ffprobe -show_streams 获取

### 3. 实施步骤

#### Step 1: 修改 setup-ffmpeg-cli.sh
- 下载 Static 版本：`ffmpeg-8.0-e05f8ac-Static-android-arm64-v8a.zip`
- 以 `libffmpeg_exec.so` / `libffprobe_exec.so` 放入 jniLibs

#### Step 2: 添加 mkvmerge 不可用时的降级逻辑
在 `content_preprocessor.go` 和 `mkvtoolnix.go` 中：
- 检测 `ENCV_MOBILE` 环境变量或 mkvmerge 是否可用
- 不可用时降级到 ffmpeg 方案：
  - `remapWithMKVMerge` → `ffmpeg -c copy` remux MKV
  - `mergeSplitParts` → ffmpeg concat demuxer
  - `mkvinfo -P` 验证 → ffprobe 验证
  - `mkvmerge -J` → ffprobe -show_streams

#### Step 3: 添加 ffmpeg concat 合并工具函数
```go
func mergeMKVWithFFmpeg(paths []string, outputPath string) error {
    // 创建 concat 文件列表
    // ffmpeg -f concat -safe 0 -i list.txt -c copy output.mkv
}
```

#### Step 4: 添加 ffprobe 验证函数
```go
func verifyWithFFprobe(filePath string) error {
    // ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 filePath
    // 检查 duration > 0
}
```

#### Step 5: 验证 CI 构建
- 确保 Static 版本下载正确
- 确保 APK 包含 libffmpeg_exec.so 和 libffprobe_exec.so

## 修改文件清单

1. `/workspace/app/encv-mobile/scripts/setup-ffmpeg-cli.sh` — Dynamic → Static
2. `/workspace/internal/v2/plugins/video/content_preprocessor.go` — mkvmerge 降级逻辑
3. `/workspace/internal/v2/plugins/video/mkvtoolnix.go` — mkvmerge/mkvinfo 降级逻辑
4. `/workspace/internal/utils/video.go` — 添加 ffmpeg concat 和 ffprobe 验证辅助函数
