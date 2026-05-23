# 项目规则

## FFmpeg 版本备注

- 当前使用 FFmpeg 7.1.1，构建脚本: `app/encv-mobile/scripts/build-ffmpeg-android.sh`
- **暂不升级到 8.x**。原因：7.1.1 满足需求（h264/hevc 编解码 + ffprobe）；8.x 有 breaking changes（移除 libpostproc、废弃 AVFrame 字段、ffttools 源码结构变化）；8.x 新功能（Vulkan compute、D3D12、VVC）在 Android 移动端用不到
- **升级到 8.x 时需注意**：
  1. libpostproc 已完全移除，链接参数中不能有 `-lpostproc`
  2. fftools 源码文件可能有增减/重命名，需对照 8.x 源码调整 `FFMPEG_FFTOOLS`/`FFPROBE_FFTOOLS` 列表
  3. API 有 breaking changes（如 AVFrame.coded_picture_number 被废弃），Go cgo 层无需改动（我们通过 dlopen 调用 fftools 的 run 函数，不直接使用 libav* API）
  4. x264 的 `--enable-pic` 仍然必须（共享库需要位置无关代码）
  5. pkg-config wrapper 方案仍然适用

## 移动端 ffmpeg 调用架构

- Go 后端通过 cgo + dlopen 直接加载 `libffmpeg.so`/`libffprobe.so`，调用 `ffmpeg_run()`/`ffprobe_run()` 函数
- 不经过 HTTP/Kotlin/JNI 中间层
- stdout/stderr 通过 dup2 重定向到临时文件捕获
- 环境变量 `ENCV_LIB_DIR` 指向 Android `nativeLibraryDir`
- 相关文件：`internal/utils/ffmpeg_dlopen.go`（Android）、`internal/utils/ffmpeg_dlopen_stub.go`（桌面端）、`internal/utils/video.go`
