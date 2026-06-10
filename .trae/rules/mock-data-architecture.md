# Mock 数据生成铁律（多源同步 / 防退化）

> **核心原则：3 套 mock 生成逻辑必须同源 —— 后端 Go / 前端 TS / CLI 脚本共享字节或共享逻辑源。**
> **禁止任何一处独自写裸 header 字节冒充"合法媒体"。**

---

## 一、3 套实现清单

| 位置 | 用途 | 现状 |
|------|------|------|
| [app/encv-mobile/src/lib/mockDataGenerator.ts](file:///workspace/app/encv-mobile/src/lib/mockDataGenerator.ts) `createMP4/MKV/MP3/FLAC` | 前端运行时调用 | ✅ 内嵌 base64 合法字节（4.8KB mp4 / 170B mkv / 45KB mp3 / 94B flac） |
| [app/encv-mobile/scripts/generate-mock-files.ts](file:///workspace/app/encv-mobile/scripts/generate-mock-files.ts) `createValidMP4/MKV/MP3/FLAC` | Node CLI 调 ffmpeg 生成 | ✅ ffmpeg 优先生成可播放媒体 + base64 fallback |
| **[internal/server/mock_generator.go](file:///workspace/internal/server/mock_generator.go) `minimalMP4/MKV/MP3/FLAC`** | **后端 API 调给开发者选项** | **❌ 历史：36B 假 mp4（只有 ftyp+moov+mdat header，无视频帧）** |

## 二、2026-06-10 修复

**症状**：开发者选项"生成 Mock"按钮调后端 `POST /api/mock/generate` → 拿到 36 字节假 mp4（< 50B）。同时**真机前端长按菜单解密报"找不到对应插件"**（不是自动化测试独有 bug！）。

**根因**：

```
后端 minimalMP4() = 36 字节裸 header
    ↓
sample.mp4 写入到 /storage/emulated/0/sample.mp4
    ↓
detector.DetectIndexKind(sample.mp4) 读不到合法视频 box
    ↓
video plugin.CanDecrypt(sample.mp4) = false
    ↓
internal/v2/plugins/registry.go:514 报 "no suitable plugin found to decrypt container"
    ↓
所有解密路径（前端长按 / 自动化测试 / AI agent）都失败
```

**修复**：
- [internal/server/mock_generator.go](file:///workspace/internal/server/mock_generator.go) `minimalMP4/MKV/MP3/FLAC` 改为 **ffmpeg 优先 + base64 fallback**
- 新增 [internal/server/mock_media_bytes.go](file:///workspace/internal/server/mock_media_bytes.go) 内嵌 `MP4_B64 / MKV_B64 / MP3_B64 / FLAC_B64`（与前端 mockDataGenerator.ts 字节 1:1 同步）
- 新增测试 [internal/server/mock_generator_test.go](file:///workspace/internal/server/mock_generator_test.go) `TestMinimalMediaIsPlayable` 断言 `> 几 KB`

```go
// 修复后：ffmpeg 优先
func minimalMP4() []byte {
    if data := ffmpegGenerate([]string{
        "-f", "lavfi",
        "-i", "sine=frequency=440:duration=2",
        "-f", "lavfi",
        "-i", "color=c=0x3B82F6:s=320x240:d=2:r=15,drawtext=text='ENCV Mock':fontsize=20:fontcolor=white:x=(w-text_w)/2:y=(h-text_h)/2",
        "-c:v", "libx264", "-preset", "ultrafast", "-tune", "stillimage", "-pix_fmt", "yuv420p",
        "-c:a", "aac", "-b:a", "64k",
        "-shortest",
    }, "mp4"); data != nil {
        return data
    }
    // base64 fallback（与前端 mockDataGenerator.ts MP4_B64 同源）
    return decodeBase64Media(MP4_B64)
}
```

## 三、3 套字节必须同源（铁律）

> **任何一处写裸 header 字节冒充"合法媒体"都是定时炸弹。**

字节同步方法（2 选 1）：

### 方案 A：嵌入 base64 常量（当前实现）

```bash
# 1. 从前端提取合并 base64
cat > /tmp/extract-b64.mjs <<'EOF'
import { readFileSync, writeFileSync } from 'fs'
import { createMP4 } from '/workspace/app/encv-mobile/src/lib/mockDataGenerator.ts'
import { Buffer } from 'node:buffer'
writeFileSync('/tmp/MP4_B64.txt', Buffer.from(createMP4()).toString('base64'))
EOF
node /tmp/extract-b64.mjs

# 2. 格式化为 Go raw string literal（100 列）
python3 -c "
b64 = open('/tmp/MP4_B64.txt').read().strip()
for i in range(0, len(b64), 100): print(b64[i:i+100])
" > /tmp/b64-formatted.txt
```

复制到 [internal/server/mock_media_bytes.go](file:///workspace/internal/server/mock_media_bytes.go)：

```go
const MP4_B64 = `
<100 chars per line>
`
```

### 方案 B：把字节文件提到共享目录

```
/workspace/internal/server/mock_assets/
├── sample.mp4
├── sample.mkv
├── sample.mp3
├── sample.flac
```

Go 端用 `//go:embed` 加载，前端用 `import sampleMp4 from '?raw'`。但需要**单一可信源**（ffmpeg 生成一次，commit 进 git），否则 3 套实现变成 3 套字节。

## 四、检测"裸 header 假数据"的测试

```go
// 防止再退化为裸 header
func TestMinimalMediaIsPlayable(t *testing.T) {
    tests := []struct {
        name     string
        data     []byte
        minBytes int
        why      string
    }{
        {"MP4", minimalMP4(), 2000, "base64 fallback = 4782B H.264+AAC"},
        {"MKV", minimalMKV(), 50, "createMKV = 170B"},
        {"MP3", minimalMP3(), 30000, "createMP3 = 45197B"},
        {"FLAC", minimalFLAC(), 50, "createFLAC = 94B"},
    }
    for _, tt := range tests {
        if len(tt.data) < tt.minBytes {
            t.Errorf("minimal%s size = %d bytes, want >= %d", tt.name, len(tt.data), tt.minBytes)
        }
    }
}
```

**测试仅验证 magic 头（`ftyp`/`EBML`/`ID3`/`fLaC`）是不够的**——36 字节假 mp4 也有 `ftyp`。必须验证大小。

## 五、为什么 mock 假数据会让所有解密路径失败

`/storage/emulated/0/sample.mp4` 写入后，任何解密流程都会：

```
task_manager.processDecrypt(absPath)
  ↓
plugins.FindDecryptingPlugin(absPath)
  ↓
for _, p := range Plugins {
    if p.CanDecrypt(absPath) { return p }
}
  ↓
video.plugin.CanDecrypt(absPath) {
    return detector.DetectIndexKind(absPath) == IndexKindVideo
}
  ↓
detector 读 mp4 box tree
  ↓
ftyp ✓ 存在
moov ✗ size=8, 不含 mvhd/trak/mdia/minf/stbl/stsd/avc1
mdat ✗ size=8, 无实际帧
  ↓
IndexKind = ""  // 识别失败
  ↓
return false
  ↓
registry.go:514 报 "no suitable plugin found to decrypt container"
```

**所有解密入口（前端长按 / 自动化测试 / agent confirmTool / useAgent 流式解密）都过这条路径 → 全部失败**。

## 六、ffmpeg 真机 / APK 兼容性

| 环境 | ffmpeg | 走哪条路 |
|------|--------|---------|
| 沙箱 dev | `/usr/bin/ffmpeg` 在 | ffmpeg 优先（生成可播放） |
| Linux 生产 | 看安装 | ffmpeg 或 fallback |
| Windows 生产 | 需单独装 | ffmpeg 或 fallback |
| **APK 真机** | **无** | **fallback base64（4.8KB mp4 仍然能 detectIndexKind）** |

`ffmpegAvailable = exec.LookPath("ffmpeg") == nil` 缓存，**真机下永远 false → 走 base64 fallback**。所以 base64 字节必须**真机可用**。

## 七、调用入口

| 入口 | 走哪 | 文件 |
|------|------|------|
| 开发者选项"生成 Mock"按钮 | 后端 API `/api/mock/generate` | [src/views/AutomationTestsDetail.vue](file:///workspace/app/encv-mobile/src/views/AutomationTestsDetail.vue) → `generateMockFilesViaBackend` |
| 自动化测试 setup 阶段 | 后端 API | 同上 |
| Workflow Dashboard "Mock Server Files" | 后端 API | [src/views/WorkflowDashboard.vue](file:///workspace/app/encv-mobile/src/views/WorkflowDashboard.vue) |
| Node CLI（离线生成） | [scripts/generate-mock-files.ts](file:///workspace/app/encv-mobile/scripts/generate-mock-files.ts) | 调试 / 集成测试用 |
| 前端运行时降级 | [src/lib/mockDataGenerator.ts](file:///workspace/app/encv-mobile/src/lib/mockDataGenerator.ts) | 单元测试 / Web preview 用 |

---

## 八、扩展铁律

> **任何带 "minimal*" 前缀的函数（`minimalMP4` / `minimalPNG` / `minimalMP3` 等）都不能只生成 header 字节。**
> **header-only 的字节会让所有依赖完整 box tree 的下游（解码器、解密器、AI 解析器）静默失败。**

如需最简"合法"字节，使用 [scripts/generate-mock-files.ts](file:///workspace/app/encv-mobile/scripts/generate-mock-files.ts) `createValid*` 或 ffmpeg，不允许 hand-roll header。
