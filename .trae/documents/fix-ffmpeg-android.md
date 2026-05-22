# 修复 Android 上 ffmpeg/ffprobe 不可用的问题

## 问题根因

**Android 上根本没有 ffmpeg/ffprobe 可执行文件。** mpv-android-lib AAR 中打包的 ffmpeg .so 共享库（libavcodec.so, libavformat.so 等）是 mpv 播放器专用的解码库，**不支持编码**，且没有 ffmpeg/ffprobe CLI 可执行文件。

Go 后端的视频加密流程依赖 ffmpeg/ffprobe CLI：
1. **ffprobe**：提取元数据（时长、格式、章节、关键帧位置）
2. **ffmpeg 编码**：转码为 H.264/AAC MP4（h264_nvenc → h264_mediacodec → libx264 降级链）
3. **ffmpeg remux**：MP4 fast-start 重封装（`-c copy -movflags +faststart`）
4. **mkvmerge**：MKV 重封装（这个在 Android 上也没有）

上一轮方案的问题：
- 下载了 **Dynamic** 版本的 ffmpeg/ffprobe，它们依赖 .so 文件，但 .so 文件与 mpv 捆绑的 .so 冲突
- 三层方案过于复杂，且 Dynamic 版本运行时会找不到正确的 .so

## 正确方案

使用 **Static** 版本的 ffmpeg/ffprobe。Static 版本是完全自包含的可执行文件，不依赖任何外部 .so，包含完整的编解码器（包括 libx264 编码器）。

### 体积考量
- Static ffmpeg: ~75MB（包含所有编解码器）
- Static ffprobe: ~75MB
- 合计 ~150MB 加入 APK

**优化**：只下载 ffprobe（用于元数据提取），ffmpeg 使用更精简的方案。但实际上加密流程中 ffmpeg 编码是核心功能，无法省略。

**进一步优化**：可以自行编译精简版 ffmpeg，只包含需要的编解码器（H.264 解码/编码、AAC 解码/编码、常用封装格式），体积可降至 ~20-30MB。但这需要维护编译脚本，暂不实施。

### 实施步骤

#### Step 1: 修改 setup-ffmpeg-cli.sh
- 改为下载 **Static** 版本（`ffmpeg-8.0-e05f8ac-Static-android-arm64-v8a.zip`）
- Static 版本是完全自包含的，不依赖 .so 文件
- 仍然以 `libffmpeg_exec.so` / `libffprobe_exec.so` 命名放入 jniLibs（Android 只安装 .so 后缀的文件）

#### Step 2: 简化 EncvGoService.kt
- `setupFFmpegBinaries()` 方法保持不变（从 nativeLibraryDir 复制到 filesDir/bin/）
- 设置 PATH 和 ENCV_BIN_DIR 环境变量
- 这部分逻辑是正确的，不需要改动

#### Step 3: Go 后端 utils/video.go
- `GetBinDir()` 优先检查 `ENCV_BIN_DIR` 环境变量
- 这部分已经正确实现，不需要改动

#### Step 4: 验证 CI 构建流程
- 确保下载的是 Static 版本
- 确保 jniLibs 中包含 libffmpeg_exec.so 和 libffprobe_exec.so
- 确保 APK 验证步骤检查这些文件

## 修改文件清单

1. `/workspace/app/encv-mobile/scripts/setup-ffmpeg-cli.sh` — 改为下载 Static 版本
2. `/workspace/.github/workflows/android.yml` — 已有 ffmpeg-cli 步骤，无需额外修改
3. `/workspace/app/encv-mobile/android/app/src/main/java/com/encvgo/app/EncvGoService.kt` — 已有 setupFFmpegBinaries()，无需额外修改
4. `/workspace/internal/utils/video.go` — 已有 ENCV_BIN_DIR 支持，无需额外修改
