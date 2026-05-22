# 修复 Android 上 ffmpeg/ffprobe/mkvmerge 不可用的问题

## 问题根因

Android 上没有 ffmpeg/ffprobe/mkvmerge 可执行文件。mpv 捆绑的 ffmpeg .so 是解码专用的。

## 方案

### 1. ffmpeg/ffprobe：CI 中自行编译精简版

**不使用 KaluaBilla 的完整 Static 版本**（~150MB），而是在 CI 中自行编译精简版，只包含我们需要的编解码器。

**参考项目**：`yearsyan/ffmpeg-android-build` 提供 Tiny/Mini/Standard/GPL 多种配置

**我们需要的编解码器**：
- 解码器：h264, hevc, aac, mp3, flac, opus, vorbis, vp8, vp9, ac3, dts, ass, srt, subrip
- 编码器：libx264, aac（内置）, h264_mediacodec（Android 硬件加速）
- 封装器（muxer）：mp4, matroska, mov, flv, webm, adts, ogg, wav
- 解封装器（demuxer）：mp4, matroska, mov, flv, webm, adts, ogg, wav, h264, hevc, aac
- 过滤器：anull, null, scale, aresample, copy
- 协议：file
- 解析器：h264, hevc, aac, opus, vp9
- BSF：h264_mp4toannexb, aac_adtstoasc, extract_extradata

**预估体积**：精简版 ffmpeg ~8-12MB + ffprobe ~5-8MB = ~15-20MB

**编译脚本**：在 CI 中使用 Android NDK 交叉编译：
```bash
./configure \
  --target-os=android --arch=aarch64 --cpu=armv8-a \
  --cross-prefix=$TOOLCHAIN/bin/aarch64-linux-android- \
  --cc=$TOOLCHAIN/bin/aarch64-linux-android21-clang \
  --enable-cross-compile --sysroot=$TOOLCHAIN/sysroot \
  --enable-gpl --enable-small --enable-mediacodec \
  --enable-libx264 --enable-encoder=libx264 \
  --disable-everything \
  --enable-decoder=h264,hevc,aac,mp3,flac,opus,vorbis,vp8,vp9,ac3 \
  --enable-encoder=libx264,aac,h264_mediacodec \
  --enable-muxer=mp4,matroska,mov,flv,webm,adts,ogg,wav \
  --enable-demuxer=mp4,matroska,mov,flv,webm,adts,ogg,wav,h264,hevc,aac \
  --enable-parser=h264,hevc,aac,opus,vp9 \
  --enable-protocol=file \
  --enable-filter=anull,null,scale,aresample,copy \
  --enable-bsf=h264_mp4toannexb,aac_adtstoasc,extract_extradata \
  --disable-doc --disable-debug --disable-network --disable-autodetect \
  --enable-static --disable-shared
```

**打包方式**：
1. 编译产物 ffmpeg/ffprobe 以 `libffmpeg_exec.so` / `libffprobe_exec.so` 放入 jniLibs
2. EncvGoService 启动时复制到 `filesDir/bin/` 并设置可执行权限
3. 设置 `ENCV_BIN_DIR` 环境变量

### 2. mkvmerge/mkvinfo/mkvextract：用 Go 原生库替代

用 `ebml-go` + `matroska` Go 原生库替代所有 mkvtoolnix CLI 调用，零外部依赖。

| 场景 | 当前 CLI | Go 替代方案 |
|------|---------|------------|
| 识别轨道 | `mkvmerge -J` | ffprobe `-show_streams` |
| 检查 Cues | `mkvinfo -v` | ebml-go 解析 EBML |
| 提取关键帧 | `mkvextract cues` | ebml-go 解析 Cues |
| 提取章节 | `mkvextract chapters` | matroska 库读取 Chapters |
| MKV remux+Cues | `mkvmerge --cues` | ebml-go 重写带 Cues 的 MKV |
| 合并分片 | `mkvmerge +` | ebml-go 合并 Cluster |
| 验证完整性 | `mkvinfo -P` | ffprobe 检查 duration |
| 检查分片链接 | `mkvinfo -p` | ebml-go 解析 SegmentInfo |

降级策略：Go 原生优先，CLI 回退（桌面端）。

### 3. 实施步骤

#### Step 1: 创建精简版 ffmpeg 编译脚本
创建 `scripts/build-ffmpeg-android.sh`，在 CI 中使用 NDK 交叉编译精简版 ffmpeg/ffprobe

#### Step 2: 更新 CI 构建流程
替换 `setup-ffmpeg-cli.sh`（下载完整版）为 `build-ffmpeg-android.sh`（自行编译精简版）

#### Step 3: 添加 Go MKV 处理库
创建 `internal/v2/plugins/video/mkv_native.go`

#### Step 4: 添加降级逻辑
在 `mkvtoolnix.go` 中，Go 原生优先，CLI 回退

#### Step 5: 更新 content_preprocessor.go
remapWithMKVMerge / ExtractChaptersWithMKVExtract 降级

#### Step 6: 验证
- `go build ./internal/...`
- `vue-tsc --noEmit`

## 修改文件清单

1. `/workspace/app/encv-mobile/scripts/build-ffmpeg-android.sh` — 新建，精简版 ffmpeg 编译脚本
2. `/workspace/app/encv-mobile/scripts/setup-ffmpeg-cli.sh` — 删除或替换
3. `/workspace/.github/workflows/android.yml` — 更新构建步骤
4. `/workspace/internal/v2/plugins/video/mkv_native.go` — 新建，Go 原生 MKV 处理
5. `/workspace/internal/v2/plugins/video/mkvtoolnix.go` — 添加降级逻辑
6. `/workspace/internal/v2/plugins/video/content_preprocessor.go` — remapWithMKVMerge 降级
7. `/workspace/go.mod` / `go.sum` — 添加 ebml-go、matroska 依赖
