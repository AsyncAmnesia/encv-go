// internal/server/mock_media_embedded.go
// 把预编码的最小媒体文件用 go:embed 编译进二进制。
//
// ════════════════════════════════════════════════════════════════════
// 2026-06-11 修复：真机 ffmpeg 失败 + 后端崩溃的根因
// ════════════════════════════════════════════════════════════════════
//
// 历史：
//   - 2026-06-10：后端 mock 生成 mp4/mkv/mp3/flac 用 ffmpeg.Runner 调 ffmpeg
//   - 2026-06-11：用户反馈「真机 APK 上 ffmpeg 调通不稳定（lavfi 在精简 build 不可用，
//     dlopen ffmpeg_run 偶尔 segfault → Go 进程跟着挂），还导致后端崩溃」
//
// 根因（旧 ffmpeg runtime 生成）：
//   1. libffmpeg.so 是为「加密视频任务」编的，configure 没加 --enable-indev=lavfi
//   2. mock 生成用 `-f lavfi -i sine=...` → 报 "Unknown input format: lavfi"
//   3. 失败时不报错（slog.Warn），上层写 0 字节文件 → 测试链全挂
//   4. 即使 lavfi 编进，cgo 边界偶尔 segfault 也会拉 Go 进程下水
//
// 修复（参考加密视频任务怎么调用 ffmpeg 的）：
//   加密视频任务：ffmpeg.Run 调的是「真输入文件 → 输出文件」（永不依赖 lavfi）
//   mock 生成：    直接用「真文件字节」（never call ffmpeg at all）
//   → 把预编码的 mp4/mkv/mp3/flac 字节 go:embed 进二进制
//   → mock 生成写盘 = 写预编码字节（< 90KB 总大小，零运行时依赖）
//   → 真机永远可用，零崩
//
// 预编码命令（一次性，沙箱 ffmpeg 6.1.1）：
//   ffmpeg -y -f lavfi -i "sine=frequency=440:duration=2" \
//          -f lavfi -i "color=c=0x3B82F6:s=320x240:d=2:r=15" \
//          -map 0:a -map 1:v -c:v libx264 -preset ultrafast -tune stillimage \
//          -pix_fmt yuv420p -c:a aac -b:a 64k -shortest sample.mp4
//   ffmpeg -y -f lavfi -i "sine=frequency=660:duration=2" \
//          -f lavfi -i "color=c=0x10B981:s=160x120:d=2:r=10" \
//          -map 0:a -map 1:v -c:v libx264 -preset ultrafast -tune stillimage \
//          -pix_fmt yuv420p -c:a libvorbis -shortest comedy.mkv
//   ffmpeg -y -f lavfi -i "sine=frequency=440:duration=2" -c:a libmp3lame -b:a 96k music.mp3
//   ffmpeg -y -f lavfi -i "sine=frequency=660:duration=2" -c:a flac podcast.flac
//
// 文件大小：
//   sample.mp4   19801 bytes (h264+aac, 2.0s, 320x240)
//   comedy.mkv    9586 bytes (h264+vorbis, 2.0s, 160x120)
//   music.mp3    33062 bytes (mp3 128k, 2.0s)
//   podcast.flac 32987 bytes (flac, 2.0s)
//   合计 ~93 KB
// ════════════════════════════════════════════════════════════════════
package server

import (
	_ "embed"
)

//go:embed mock_media/sample.mp4
var minimalMP4Bytes []byte

//go:embed mock_media/comedy.mkv
var minimalMKVBytes []byte

//go:embed mock_media/music.mp3
var minimalMP3Bytes []byte

//go:embed mock_media/podcast.flac
var minimalFLACBytes []byte
