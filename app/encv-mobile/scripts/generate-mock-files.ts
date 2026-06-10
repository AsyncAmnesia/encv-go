/**
 * CLI 入口：从 src/lib/mockDataGenerator 调用纯函数，把生成的字节写到磁盘。
 *
 * 行为兼容旧版（保持 ENCV_MOCK_ROOT / --dir / --type 参数）。
 *
 * 与 lib 相比，本脚本的额外职责：
 * 1. CLI 参数解析（process.argv）
 * 2. 优先用 ffmpeg 生成"可播放"的 MP4/MKV/MP3/FLAC（dev 环境用 ffmpeg 输出 2s 正弦波 + 颜色图）
 * 3. 同步打印到 stdout（CLI 风格）
 */
import * as fs from 'fs'
import * as path from 'path'
import { execSync, spawnSync } from 'child_process'
import * as os from 'os'
import {
  collectSpecs,
  createJPEG,
  createPNG,
  createMP4,
  createMKV,
  createMP3,
  createFLAC,
  createPDF,
  type MockFileSpec,
} from '../src/lib/mockDataGenerator'

// ⚠️ 必须与 ecosystem.config.cjs ENCV_MOCK_ROOT + usePathResolver.encv-automation 命名空间一致
// 根因复盘：2026-06-10 路径不一致 bug → 三处必须同步
const MOCK_ROOT = process.env.ENCV_MOCK_ROOT || '/storage/emulated/0/encv-automation'
let root = MOCK_ROOT
let genType = 'all' as string
let fileCount = 0
let totalSize = 0

function ensureDir(dir: string): void {
  fs.mkdirSync(dir, { recursive: true })
}

/**
 * ⚠️ 防御：writeBuffer 必须先确保父目录存在
 *
 * 根因复盘：2026-06-10 mock 生成失败 → ENOENT
 *   ensureDir(root) 只创建了 /storage/emulated/0
 *   第一个 spec 是 01-plain-media/image/photo.jpg
 *   writeFileSync 找不到 image/ 父目录 → ENOENT
 *   整个脚本以 code 1 退出 → gateway FATAL → pm2 不断重启
 *
 * 修复：在写盘前自动 mkdirSync dirname，确保父目录链就绪
 */
function ensureParentDir(filePath: string): void {
  const parent = path.dirname(filePath)
  fs.mkdirSync(parent, { recursive: true })
}

function writeBuffer(filePath: string, data: Uint8Array): void {
  ensureParentDir(filePath)  // ⚠️ 防御：递归创建父目录
  fs.writeFileSync(filePath, Buffer.from(data))
  recordFile(filePath, data.length)
}

function writeString(filePath: string, content: string, encoding: BufferEncoding = 'utf-8'): void {
  const buf = Buffer.from(content, encoding)
  ensureParentDir(filePath)  // ⚠️ 防御：递归创建父目录
  fs.writeFileSync(filePath, buf)
  recordFile(filePath, buf.length)
}

function humanSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

function recordFile(p: string, size: number): void {
  fileCount++
  totalSize += size
  console.log('  ✅ ' + path.relative(root, p) + ' (' + humanSize(size) + ')')
}

function printSummary(): void {
  console.log('\n📊 总计: ' + fileCount + ' 个文件, ' + humanSize(totalSize))
}

const join = path.join

// ==================== FFmpeg-based playable media ====================

const hasFFmpeg = (() => {
  try {
    execSync('which ffmpeg', { stdio: 'ignore', timeout: 3000 })
    return true
  } catch {
    return false
  }
})()

function ffmpegGenerate(args: string[], label: string, ext: string): Buffer {
  const tmpFile = path.join(os.tmpdir(), `encv-mock-${Date.now()}-${Math.random().toString(36).slice(2)}.${ext}`)
  try {
    spawnSync('ffmpeg', [...args, '-y', '-loglevel', 'error', tmpFile], {
      stdio: ['pipe', 'pipe', 'pipe'] as never,
      timeout: 15000,
    })
    if (!fs.existsSync(tmpFile)) throw new Error(`ffmpeg ${label} produced no output`)
    return fs.readFileSync(tmpFile)
  } finally {
    try { fs.unlinkSync(tmpFile) } catch {}
  }
}

function createValidMP4(): Buffer {
  if (!hasFFmpeg) { console.warn('[WARN] ffmpeg not found, using legacy unplayable MP4'); return Buffer.from(createMP4()) }
  return ffmpegGenerate([
    '-f', 'lavfi',
    '-i', 'sine=frequency=440:duration=2',
    '-f', 'lavfi',
    '-i', "color=c=0x3B82F6:s=320x240:d=2:r=15,drawtext=text='ENCV Mock':fontsize=20:fontcolor=white:x=(w-text_w)/2:y=(h-text_h)/2",
    '-c:v', 'libx264', '-preset', 'ultrafast', '-tune', 'stillimage', '-pix_fmt', 'yuv420p',
    '-c:a', 'aac', '-b:a', '64k',
    '-shortest',
  ], 'MP4', 'mp4')
}

function createValidMKV(): Buffer {
  if (!hasFFmpeg) { console.warn('[WARN] ffmpeg not found, using legacy unplayable MKV'); return Buffer.from(createMKV()) }
  return ffmpegGenerate([
    '-f', 'lavfi',
    '-i', 'sine=frequency=660:duration=2',
    '-f', 'lavfi',
    '-i', "color=c=0x10B981:s=160x120:d=2:r=10,drawtext=text='ENCV MKV':fontsize=16:fontcolor=white:x=(w-text_w)/2:y=(h-text_h)/2",
    '-c:v', 'libx264', '-preset', 'ultrafast', '-tune', 'stillimage', '-pix_fmt', 'yuv420p',
    '-c:a', 'libvorbis',
    '-shortest',
  ], 'MKV', 'mkv')
}

function createValidMP3(): Buffer {
  if (!hasFFmpeg) { console.warn('[WARN] ffmpeg not found, using legacy unplayable MP3'); return Buffer.from(createMP3()) }
  return ffmpegGenerate([
    '-f', 'lavfi',
    '-i', 'sine=frequency=440:duration=2',
    '-c:a', 'libmp3lame', '-b:a', '128k',
  ], 'MP3', 'mp3')
}

function createValidFLAC(): Buffer {
  if (!hasFFmpeg) { console.warn('[WARN] ffmpeg not found, using legacy unplayable FLAC'); return Buffer.from(createFLAC()) }
  return ffmpegGenerate([
    '-f', 'lavfi',
    '-i', 'sine=frequency=440:duration=2',
    '-c:a', 'flac', '-sample_fmt', 's16',
  ], 'FLAC', 'flac')
}

// ==================== Main ====================

function parseArgs(): void {
  const args = process.argv.slice(2)
  for (let i = 0; i < args; ) {
    if (args[i] === '--dir' && args[i + 1]) {
      root = path.resolve(args[i + 1])
      args.splice(i, 2)
    } else if (args[i] === '--type' && args[i + 1]) {
      genType = args[i + 1]
      args.splice(i, 2)
    } else {
      i++
    }
  }
}

/**
 * 把 lib 的 collectSpecs() 转换为 CLI 风格的落盘 + console.log。
 * 只对 plain 类型做 ffmpeg 增强（更可播放），其他类型直接用 lib 输出。
 */
function writeSpec(spec: MockFileSpec): void {
  const fullPath = join(root, spec.relativePath)
  // ffmpeg 增强只对 plain 媒体生效（与旧版行为一致）
  if (spec.relativePath === '01-plain-media/video/sample.mp4') {
    writeBuffer(fullPath, createValidMP4())
  } else if (spec.relativePath === '01-plain-media/video/comedy.mkv') {
    writeBuffer(fullPath, createValidMKV())
  } else if (spec.relativePath === '01-plain-media/audio/music.mp3') {
    writeBuffer(fullPath, createValidMP3())
  } else if (spec.relativePath === '01-plain-media/audio/podcast.flac') {
    writeBuffer(fullPath, createValidFLAC())
  } else if (spec.relativePath.endsWith('.txt') || spec.relativePath.endsWith('.csv')) {
    writeString(fullPath, new TextDecoder().decode(spec.data))
  } else {
    writeBuffer(fullPath, spec.data)
  }
}

async function main(): Promise<void> {
  parseArgs()
  ensureDir(root)

  console.log('📦 ENCV Mock File Generator')
  console.log('   Output: ' + root)
  console.log('   Type: ' + genType)
  console.log('   FFmpeg: ' + (hasFFmpeg ? '✅ (playable media)' : '❌ (legacy binary)') + '\n')

  const sections: Array<{ name: string; types: Array<'plain' | 'ae' | 'container' | 'boundary'> }> = [
    { name: 'Plain Media', types: ['plain'] },
    { name: 'AList Encrypt (.ae)', types: ['ae'] },
    { name: 'encv v4 Containers', types: ['container'] },
    { name: 'Boundary Test Files', types: ['boundary'] },
  ]

  for (const section of sections) {
    if (genType !== 'all' && !section.types.includes(genType as any)) continue
    console.log(`--- ${section.name} ---`)
    for (const t of section.types) {
      const specs = collectSpecs(t)
      for (const s of specs) {
        writeSpec(s)
      }
    }
    console.log('')
  }

  printSummary()
}

main().catch((e) => {
  console.error('❌ Failed:', e)
  process.exit(1)
})
