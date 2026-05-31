# Mock 真实文件生成脚本 Plan

## Why

当前 Mock 文件系统（`mock/file-system.ts`）只有**元数据**（name/size/path），没有磁盘上的真实二进制文件。当后端运行在 `go run ./cmd/encv-mobile` 模式时，API 请求会打到真实后端，但后端扫描的 `server_dir` 路径下没有对应文件——导致：
- 文件列表为空或与 Mock 元数据不匹配
- 文件预览/下载返回 404
- 加密操作无法执行（无真实源文件）
- ENC-FN 文件名加密无法端到端验证

需要一个**一次性生成脚本**，按文件夹结构创建带真实内容的测试文件。

## What Changes

新建 Node.js 脚本 `scripts/generate-mock-files.ts`，生成以下目录结构的真实文件：

```
<server_dir>/
├── sample.mp4              # 真实 MP4（最小有效 fMP4）
├── movie.mkv               # 真实 MKV（最小有效 WebM-like）
├── music.mp3               # 真实 MP3（含 ID3 标签的静音音频）
├── podcast.flac            # 真实 FLAC（最小有效 FLAC 头）
├── photo.jpg               # 真实 JPEG（1x1 像素红色图片）
├── screenshot.png          # 真实 PNG（4x4 像素渐变图片）
├── report.pdf              # 真实 PDF（含文字内容）
├── notes.txt               # UTF-8 多语言文本
├── data.csv                # 有效 CSV 数据
├── secret.ae               # 模拟 alist-encrypt 文件（随机二进制 + AE magic）
├── document.ae             # 同上
├── video.sccgv             # 模拟 ENCV v4 容器（含 Header+Manifest 魔数）
├── image.sccgi             # 同上
├── audio.sccga             # 同上
├── Movies/
│   ├── action-movie.sccgv  # 大尺寸容器模拟
│   ├── comedy.mkv          # 真实 MKV
│   └── hidden-gem.ae       # alist-encrypt 文件
├── Documents/
│   ├── contract.pdf        # 真实 PDF
│   ├── memo.txt            # 中文文本
│   ├── confidential.ae     # alist-encrypt
│   └── archive.sccgpdf     # 容器
├── Music/
│   ├── song1.mp3           # 真实 MP3
│   ├── song2.flac          # 真实 FLAC
│   ├── album-secret.ae     # alist-encrypt
│   └── track.sccga         # 容器
└── long-filename-test/
    └── 这是一个非常长的中文文件名用于测试ENC-FN编码功能包含emoji🎉和生僻汉字龘靁.txt  # 超长文件名测试
```

## Impact

- 新增文件：`scripts/generate-mock-files.ts`
- 不修改任何现有代码
- 输出路径：由参数控制，默认 `<project_root>/output/mock-root/`（与 config.user.json 的 server_dir 对应）

## ADDED Requirements

### Requirement: 最小有效文件格式

脚本 SHALL 为每种文件类型生成**语法正确**的最小有效二进制文件，而非随机字节。这确保：
- 图片文件可在浏览器中正常渲染
- 音视频文件可被 MediaElement API 解析（至少不报错）
- PDF 可被 PDF.js 渲染
- 文本文件 UTF-8 编码正确

### Requirement: ENC-FN 测试专用文件

必须生成一个**超长 Unicode 文件名**文件，用于验证 ENC-FN 编码器的边界情况。

### Requirement: 幂等性

多次运行脚本应产生相同结果（覆盖已有文件）。

---

# Tasks

## Task 1: 创建 `scripts/generate-mock-files.ts`

### 1.1 文件头部
- 使用纯 Node.js API（无外部依赖）
- 支持 `--dir <path>` 参数指定输出根目录
- 默认输出到 `../../output/mock-root/`

### 1.2 工具函数
```typescript
function ensureDir(dir: string): void        // mkdir -p
function writeBinary(path: string, data: Uint8Array): void  // 写入二进制
function writeText(path: string, content: string, encoding?: string): void  // 写入文本
function randomBytes(n: number): Uint8Array  // 安全随机字节
function padToSize(data: Uint8Array, targetSize: number): Uint8Array  // 填充到目标大小
```

### 1.3 各类型文件生成器

#### JPEG (photo.jpg)
- 最小有效 JFIF JPEG：SOI + APP0 + DQT + SOF0 + DHT + SOS + EOI
- 1x1 像素红色图片（约 200-300 字节）

#### PNG (screenshot.png)
- 最小有效 PNG：PNG signature + IHDR + IDAT (zlib deflate) + IEND
- 4x4 像素 RGBA 渐变（约 100-200 字节）

#### MP4 (sample.mp4)
- 最小有效 fMP4：ftyp + moov (mvhd/trak/mdia/minf/stbl/stsd/stts/stsc/stsz/stco) + mdat
- 含 1 秒静音 AAC 帧（约 2-5 KB）

#### MKV / WebM (movie.mkv)
- 最小有效 EBML：EBML Header + SegmentInfo + Tracks + Cluster + SimpleBlock
- 含 VP9 关键帧（约 1-3 KB）

#### MP3 (music.mp3)
- 有效 MP3 帧：ID3v2 标签（可选）+ MPEG Audio Frame (Layer III, 44100Hz, 128kbps)
- 3 秒静音（约 12-15 KB）

#### FLAC (podcast.flac)
- 有效 FLAC 流：fLaC + STREAMINFO + PADDING + 若干帧
- 2 秒静音（约 8-10 KB）

#### PDF (report.pdf)
- 最小有效 PDF：%PDF-1.4 header + catalog/page/contents/stream + xref/trailer
- 含 "Hello from ENCV Mock" 文字（约 800-1200 字节）

#### TXT (notes.txt)
- 多行多语言 UTF-8 文本：英文、中文、日文、emoji、特殊字符

#### CSV (data.csv)
- 有效 CSV：header 行 + 10 行数据（中英混合）

#### alist-encrypt 模拟文件 (secret.ae, document.ae 等)
- 固定 magic header + 随机填充数据
- 大小匹配 mock/file-system.ts 中声明的 size

#### ENCV v4 容器模拟文件 (video.sccgv, image.sccgi 等)
- 自定义 magic: `SCCV4` (4 bytes) + 版本号 (2 bytes LE) + Flags (2 bytes LE) + 随机填充
- 大小匹配声明值

#### 超长文件名测试文件
- 文件名：`这是一个非常长的中文文件名用于测试ENC-FN编码功能包含emoji🎉和生僻汉字龘靁.txt`
- 内容：短文本说明

### 1.4 目录结构生成
- 按上述目录树递归创建
- 打印每个生成的文件路径和大小

### 1.5 集成到 vite.config.ts
- 在 mock 插件启动时自动检测并提示是否需要生成文件
- 或提供 npm script: `"generate:mock": "tsx scripts/generate-mock-files.ts"`

## Task 2: 更新 package.json

新增 script:
```json
{
  "scripts": {
    "generate:mock": "tsx scripts/generate-mock-files.ts"
  }
}
```

## Task 3: 运行验证

1. 执行 `npm run generate:mock`
2. 检查输出目录结构
3. 用 `file` 命令验证各文件格式识别正确
4. 启动后端确认文件列表能扫到这些真实文件
