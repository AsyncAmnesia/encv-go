/**
 * Mock 数据生成器（纯函数模块）
 *
 * 这是从 scripts/generate-mock-files.ts 提取的纯函数版本，不依赖 fs / child_process，
 * 可被前端 / 后端 / CLI 任何环境 import。
 *
 * - 前端：通过 POST /api/mock/generate 把 Uint8Array 发给后端写盘
 * - 后端：直接调用本模块的函数写文件
 * - CLI：scripts/generate-mock-files.ts 改写为 import 此模块
 *
 * 设计原则：
 * - 所有 create*() 函数返回 Uint8Array（pure，无副作用）
 * - IO 通过 generateMockFiles() 的 writeToDisk 回调抽象
 * - 不在模块顶层调 execSync / fs
 */

// ==================== 类型定义 ====================

export type MockFileType = 'all' | 'plain' | 'ae' | 'container' | 'boundary'

export interface MockFileSpec {
  relativePath: string
  data: Uint8Array
  size: number
}

export interface GenerateOptions {
  root: string
  type?: MockFileType
  writeToDisk?: (path: string, data: Uint8Array) => Promise<void> | void
  onProgress?: (spec: MockFileSpec) => void
}

export interface GenerateResult {
  count: number
  totalSize: number
  specs: MockFileSpec[]
}

// ==================== 工具函数 ====================

function randomBytes(n: number): Uint8Array {
  const buf = new Uint8Array(n)
  if (typeof crypto !== 'undefined' && crypto.getRandomValues) {
    crypto.getRandomValues(buf)
  } else {
    for (let i = 0; i < n; i++) buf[i] = Math.floor(Math.random() * 256)
  }
  return buf
}

function padToSize(data: Uint8Array, targetSize: number): Uint8Array {
  if (data.length >= targetSize) return data
  const padded = new Uint8Array(targetSize)
  padded.set(data)
  padded.set(randomBytes(targetSize - data.length), data.length)
  return padded
}

function crc32(buf: Uint8Array): number {
  let c = 0xFFFFFFFF
  for (let i = 0; i < buf.length; i++) {
    c ^= buf[i]
    for (let j = 0; j < 8; j++) {
      c = (c >>> 1) ^ (c & 1 ? 0xEDB88320 : 0)
    }
  }
  return (c ^ 0xFFFFFFFF) >>> 0
}

function makeChunk(type: string, data: Uint8Array): Uint8Array {
  const len = new Uint8Array(4)
  new DataView(len.buffer).setUint32(0, data.length, false)
  const typeB = new TextEncoder().encode(type)
  const crcData = new Uint8Array(typeB.length + data.length)
  crcData.set(typeB, 0)
  crcData.set(data, typeB.length)
  const crc = new Uint8Array(4)
  new DataView(crc.buffer).setUint32(0, crc32(crcData), false)
  const out = new Uint8Array(4 + typeB.length + data.length + 4)
  out.set(len, 0)
  out.set(typeB, 4)
  out.set(data, 4 + typeB.length)
  out.set(crc, 4 + typeB.length + data.length)
  return out
}

function joinPath(...parts: string[]): string {
  return parts.join('/').replace(/\/+/g, '/')
}

/**
 * ⚠️ 防御：确保父目录存在
 *
 * 根因复盘：2026-06-10 mock 生成 ENOENT（与 scripts/generate-mock-files.ts 同源）
 * 写盘回调未先建父目录导致失败
 */
function ensureParentDir(fullPath: string): void {
  // 兼容浏览器与 Node
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const fs: any = (globalThis as any).fs ?? require?.('fs')
  if (!fs) return // 浏览器环境下无 fs，跳过
  const parent = fullPath.split('/').slice(0, -1).join('/')
  if (parent) fs.mkdirSync(parent, { recursive: true })
}

// ==================== JPEG ====================

export function createJPEG(): Uint8Array {
  return new Uint8Array([
    0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46,
    0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
    0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
    0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08,
    0x07, 0x07, 0x07, 0x09, 0x09, 0x08, 0x0A, 0x0C,
    0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
    0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D,
    0x1A, 0x1C, 0x1C, 0x20, 0x24, 0x2E, 0x27, 0x20,
    0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
    0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27,
    0x39, 0x3D, 0x38, 0x32, 0x3C, 0x2E, 0x33, 0x34,
    0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01,
    0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4,
    0x00, 0x1F, 0x00, 0x00, 0x01, 0x05, 0x01, 0x01,
    0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04,
    0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0xFF,
    0xC4, 0x00, 0xB5, 0x10, 0x00, 0x02, 0x01, 0x03,
    0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04,
    0x00, 0x00, 0x01, 0x7D, 0x01, 0x02, 0x03, 0x00,
    0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06,
    0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14, 0x32,
    0x81, 0x91, 0xA1, 0x08, 0x23, 0x42, 0xB1, 0xC1,
    0x15, 0x52, 0xD1, 0xF0, 0x24, 0x33, 0x62, 0x72,
    0x82, 0x09, 0x0A, 0x16, 0x17, 0x18, 0x19, 0x1A,
    0x25, 0x26, 0x27, 0x28, 0x29, 0x2A, 0x34, 0x35,
    0x36, 0x37, 0x38, 0x39, 0x3A, 0x43, 0x44, 0x45,
    0x46, 0x47, 0x48, 0x49, 0x4A, 0x53, 0x54, 0x55,
    0x56, 0x57, 0x58, 0x59, 0x5A, 0x63, 0x64, 0x65,
    0x66, 0x67, 0x68, 0x69, 0x6A, 0x73, 0x74, 0x75,
    0x76, 0x77, 0x78, 0x79, 0x7A, 0x83, 0x84, 0x85,
    0x86, 0x87, 0x88, 0x89, 0x8A, 0x92, 0x93, 0x94,
    0x95, 0x96, 0x97, 0x98, 0x99, 0x9A, 0xA2, 0xA3,
    0xA4, 0xA5, 0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xB2,
    0xB3, 0xB4, 0xB5, 0xB6, 0xB7, 0xB8, 0xB9, 0xBA,
    0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9,
    0xCA, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7, 0xD8,
    0xD9, 0xDA, 0xE1, 0xE2, 0xE3, 0xE4, 0xE5, 0xE6,
    0xE7, 0xE8, 0xE9, 0xEA, 0xF1, 0xF2, 0xF3, 0xF4,
    0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFF, 0xDA,
    0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F, 0x00,
    0x7B, 0x94, 0x40, 0x18, 0x19, 0x1F, 0x81, 0x17,
    0x38, 0x41, 0x91, 0x82, 0x83, 0x84, 0x88, 0x89,
    0x8C, 0x8D, 0x90, 0x92, 0x93, 0x96, 0x97, 0x98,
    0x99, 0x9A, 0x9C, 0x9E, 0x9F, 0xA0, 0xA1, 0xA2,
    0xA4, 0xA5, 0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xAC,
    0xAD, 0xAE, 0xAF, 0xB0, 0xB1, 0xB2, 0xB3, 0xB4,
    0xB5, 0xB6, 0xB7, 0xB8, 0xB9, 0xBA, 0xBC, 0xBD,
    0xBE, 0xBF, 0xC0, 0xC1, 0xC2, 0xC3, 0xC4, 0xC5,
    0xC6, 0xC7, 0xC8, 0xC9, 0xCA, 0xCC, 0xCD, 0xCE,
    0xCF, 0xD0, 0xD1, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6,
    0xD7, 0xD8, 0xD9, 0xDA, 0xDC, 0xDE, 0xDF, 0xE0,
    0xE1, 0xE2, 0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8,
    0xE9, 0xEA, 0xEC, 0xED, 0xEE, 0xEF, 0xF0, 0xF1,
    0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9,
    0xEA, 0xFB, 0xFD, 0xFE, 0xFF, 0xD9,
  ])
}

// ==================== PNG ====================

export function createPNG(): Uint8Array {
  const signature = new Uint8Array([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A])

  const ihdrData = new Uint8Array([0, 0, 0, 4, 0, 0, 0, 4, 8, 6, 0, 0, 0])
  const ihdr = makeChunk('IHDR', ihdrData)

  const pixels: number[] = []
  for (let y = 0; y < 4; y++) {
    pixels.push(0)
    for (let x = 0; x < 4; x++) {
      const r = Math.floor((x / 3) * 255)
      const g = Math.floor((y / 3) * 255)
      const b = 128
      const a = 255
      pixels.push(r, g, b, a)
    }
  }
  const idatRaw = new Uint8Array(pixels)
  // 纯前端环境用 store-only deflate（zlib 0x78 0x01 header），
  // 任何合规 PNG 解码器都接受。Node 端 import 脚本优先用 ffmpeg 重写，无需 zlib。
  const idatCompressed = deflateRaw(idatRaw)
  const idat = makeChunk('IDAT', idatCompressed)

  const iend = makeChunk('IEND', new Uint8Array(0))

  const out = new Uint8Array(signature.length + ihdr.length + idat.length + iend.length)
  out.set(signature, 0)
  out.set(ihdr, signature.length)
  out.set(idat, signature.length + ihdr.length)
  out.set(iend, signature.length + ihdr.length + idat.length)
  return out
}

// 极简 deflate（store-only blocks）+ zlib wrapper
// 用于纯前端环境（无 zlib 模块）。Node 端会被 require('zlib').deflateSync 覆盖。
function deflateRaw(input: Uint8Array): Uint8Array {
  // zlib header: 0x78 0x01 (no compression hint)
  const header = new Uint8Array([0x78, 0x01])
  // adler32
  let a = 1, b = 0
  for (let i = 0; i < input.length; i++) {
    a = (a + input[i]) % 65521
    b = (b + a) % 65521
  }
  const adler = new Uint8Array(4)
  new DataView(adler.buffer).setUint32(0, (b << 16) | a, false)

  // 单个 non-final stored block (BTYPE=00)
  const blockType = 0x00
  const lastBlock = 0x01
  const lenBuf = new Uint8Array(4)
  new DataView(lenBuf.buffer).setUint16(0, input.length, true)
  new DataView(lenBuf.buffer).setUint16(2, ~input.length & 0xFFFF, true)

  const total = 2 + 1 + 4 + input.length + 4
  const out = new Uint8Array(total)
  out.set(header, 0)
  out[2] = lastBlock | blockType
  out.set(lenBuf, 3)
  out.set(input, 7)
  out.set(adler, 7 + input.length)
  return out
}

// ==================== MP4 helpers ====================

function mp4Box(type: string, children?: Uint8Array[]): Uint8Array {
  let body: Uint8Array
  if (children && children.length > 0) {
    const totalLen = children.reduce((s, c) => s + c.length, 0)
    body = new Uint8Array(totalLen)
    let off = 0
    for (const c of children) {
      body.set(c, off)
      off += c.length
    }
  } else {
    body = new Uint8Array(0)
  }

  const size = 8 + body.length
  const buf = new Uint8Array(size)
  const view = new DataView(buf.buffer)
  view.setUint32(0, size, false)
  const typeBytes = new TextEncoder().encode(type)
  buf.set(typeBytes, 4)
  buf.set(body, 8)
  return buf
}

function mp4FullBox(type: string, version: number, flags: number, children?: Uint8Array[]): Uint8Array {
  let inner: Uint8Array
  if (children && children.length > 0) {
    const t = children.reduce((s, c) => s + c.length, 0)
    inner = new Uint8Array(t)
    let o = 0
    for (const c of children) { inner.set(c, o); o += c.length }
  } else {
    inner = new Uint8Array(0)
  }

  const size = 12 + inner.length
  const buf = new Uint8Array(size)
  const view = new DataView(buf.buffer)
  view.setUint32(0, size, false)
  const typeBytes = new TextEncoder().encode(type)
  buf.set(typeBytes, 4)
  view.setUint32(8, (version << 24) | (flags & 0xFFFFFF), false)
  buf.set(inner, 12)
  return buf
}

// ==================== MP4 ====================

export function createMP4(): Uint8Array {
  const ftyp = mp4Box('ftyp', [
    new Uint8Array(new TextEncoder().encode('isom')),
    new Uint8Array([0x00, 0x00, 0x02, 0x00]),
    new Uint8Array(new TextEncoder().encode('isom')),
    new Uint8Array(new TextEncoder().encode('iso2')),
    new Uint8Array(new TextEncoder().encode('mp41')),
  ])

  const mvhd = mp4FullBox('mvhd', 0, 0, [
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x01]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x01, 0x00]),
    new Uint8Array([0x00, 0x01, 0x00, 0x00]),
    new Uint8Array([0x01, 0x00, 0x00, 0x00]),
    new Uint8Array([0x01, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0xFF, 0xFF, 0x00, 0x00]),
  ])

  const mdhd = mp4FullBox('mdhd', 0, 0, [
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x03, 0xE8]),
    new Uint8Array([0x00, 0x00, 0x00, 0x96]),
    new Uint8Array([0x55, 0xC4, 0x00, 0x00]),
  ])

  const hdlr = mp4FullBox('hdlr', 0, 0, [
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array(new TextEncoder().encode('vide')),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array(new TextEncoder().encode('VideoHandler\x00')),
  ])

  const urlBox = mp4FullBox('url ', 0, 1)
  const dref = mp4FullBox('dref', 0, 0, [urlBox])
  const dinf = mp4Box('dinf', [dref])

  const mp4aEntry = mp4Box('mp4a', [
    new Uint8Array([0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x62, 0x00, 0x01]),
    mp4Box('esds', [
      new Uint8Array([0x03, 0x80, 0x80, 0x80, 0x22, 0x00, 0x01, 0x00, 0x04, 0x80, 0x80, 0x80, 0x17, 0x40, 0x15, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x06, 0x80, 0x80, 0x80, 0x01, 0x02]),
    ]),
  ])
  const stsd = mp4FullBox('stsd', 0, 0, [new Uint8Array([0x00, 0x00, 0x00, 0x01]), mp4aEntry])

  const stts = mp4FullBox('stts', 0, 0, [
    new Uint8Array([0x00, 0x00, 0x00, 0x01]),
    new Uint8Array([0x00, 0x00, 0x00, 0x96]),
    new Uint8Array([0x00, 0x00, 0x00, 0x01]),
  ])
  const stsc = mp4FullBox('stsc', 0, 0, [
    new Uint8Array([0x00, 0x00, 0x00, 0x01]),
    new Uint8Array([0x00, 0x00, 0x00, 0x01]),
    new Uint8Array([0x00, 0x00, 0x00, 0x01]),
    new Uint8Array([0x00, 0x00, 0x00, 0x96]),
  ])
  const stsz = mp4FullBox('stsz', 0, 0, [
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x96]),
    ...Array(150).fill(null).map(() => new Uint8Array([0x00, 0x00, 0x01, 0xA2])),
  ])
  const stco = mp4FullBox('stco', 0, 0, [
    new Uint8Array([0x00, 0x00, 0x00, 0x01]),
    new Uint8Array([0x00, 0x00, 0x04, 0x20]),
  ])

  const stbl = mp4Box('stbl', [stsd, stts, stsc, stsz, stco])
  const nmhd = mp4FullBox('nmhd', 0, 0)
  const minf = mp4Box('minf', [nmhd, dinf, stbl])
  const mdia = mp4Box('mdia', [mdhd, hdlr, minf])
  const tkhd = mp4FullBox('tkhd', 0, 3, [
    new Uint8Array([0x00, 0x00, 0x00, 0x01]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x01, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02]),
  ])
  const trak = mp4Box('trak', [tkhd, mdia])
  const moov = mp4Box('moov', [mvhd, trak])

  const silenceFrame = new Uint8Array(420)
  silenceFrame[0] = 0xFF
  silenceFrame[1] = 0xFB
  silenceFrame[2] = 0x90
  silenceFrame[3] = 0x00

  const frames: Uint8Array[] = []
  for (let i = 0; i < 50; i++) frames.push(silenceFrame.slice())

  const mdatBodyLen = frames.reduce((s, f) => s + f.length, 0)
  const mdatBody = new Uint8Array(mdatBodyLen)
  let mdatOff = 0
  for (const f of frames) { mdatBody.set(f, mdatOff); mdatOff += f.length }
  const mdat = mp4Box('mdat', [mdatBody])

  const total = ftyp.length + moov.length + mdat.length
  const out = new Uint8Array(total)
  out.set(ftyp, 0)
  out.set(moov, ftyp.length)
  out.set(mdat, ftyp.length + moov.length)
  return out
}

// ==================== MKV/WebM ====================

function ebmlVInt(val: number): Uint8Array {
  if (val < 127) return new Uint8Array([val])
  if (val < 16383) return new Uint8Array([0x40 | (val >> 8), val & 0xFF])
  if (val < 2097151) return new Uint8Array([0x20 | (val >> 16), (val >> 8) & 0xFF, val & 0xFF])
  return new Uint8Array([0x10 | (val >> 24), (val >> 16) & 0xFF, (val >> 8) & 0xFF, val & 0xFF])
}

function ebmlElement(id: Uint8Array, value: Uint8Array | Uint8Array[]): Uint8Array {
  const v = Array.isArray(value) ? concatBytes(value) : value
  const size = ebmlVInt(v.length)
  const result = new Uint8Array(id.length + size.length + v.length)
  result.set(id, 0)
  result.set(size, id.length)
  result.set(v, id.length + size.length)
  return result
}

function ebmlUint(id: Uint8Array, val: number): Uint8Array {
  if (val === 0) return ebmlElement(id, new Uint8Array([0]))
  const bytes: number[] = []
  let v = val
  while (v > 0) {
    bytes.unshift(v & 0xFF)
    v >>>= 8
  }
  return ebmlElement(id, new Uint8Array(bytes))
}

function ebmlString(id: Uint8Array, str: string): Uint8Array {
  return ebmlElement(id, new TextEncoder().encode(str))
}

function ebmlFloat(id: Uint8Array, val: number): Uint8Array {
  const buf = new ArrayBuffer(8)
  const view = new DataView(buf)
  view.setFloat64(0, val, false)
  return ebmlElement(id, new Uint8Array(new Int8Array(buf)))
}

export function createMKV(): Uint8Array {
  const EBML = new Uint8Array([0x1A, 0x45, 0xDF, 0xA3])
  const Segment = new Uint8Array([0x18, 0x53, 0x80, 0x67])
  const Info = new Uint8Array([0x15, 0x49, 0xA9, 0x66])
  const Tracks = new Uint8Array([0x16, 0x54, 0xAE, 0x6B])
  const TrackEntry = new Uint8Array([0xAE])
  const TrackNumber = new Uint8Array([0xD7])
  const TrackUID = new Uint8Array([0x73, 0xC5])
  const TrackType = new Uint8Array([0x83])
  const CodecID = new Uint8Array([0x86])
  const CodecPrivate = new Uint8Array([0x63, 0xA2])
  const TimecodeScale = new Uint8Array([0x2A, 0xD7, 0xB1])
  const Duration = new Uint8Array([0x44, 0x89])
  const Cluster = new Uint8Array([0x1F, 0x43, 0xB6, 0x75])
  const Timecode = new Uint8Array([0xE7])
  const SimpleBlock = new Uint8Array([0xA3])
  const DocType = new Uint8Array([0x42, 0x82])
  const DocTypeVersion = new Uint8Array([0x42, 0x87])
  const DocTypeReadVersion = new Uint8Array([0x42, 0x85])

  const ebmlHeader = ebmlElement(EBML, concatBytes([
    ebmlUint(DocTypeVersion, 4),
    ebmlUint(DocTypeReadVersion, 4),
    ebmlString(DocType, 'matroska'),
  ]))

  const audioTrack = ebmlElement(TrackEntry, concatBytes([
    ebmlUint(TrackNumber, 1),
    ebmlUint(TrackUID, 12345),
    ebmlUint(TrackType, 2),
    ebmlString(CodecID, 'A_VORBIS'),
    ebmlElement(CodecPrivate, new Uint8Array([0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])),
  ]))

  const info = ebmlElement(Info, concatBytes([
    ebmlUint(TimecodeScale, 1000000),
    ebmlFloat(Duration, 2.5),
  ]))

  const tracks = ebmlElement(Tracks, [audioTrack])

  const silentVorbisData = new Uint8Array(64).fill(0)
  const blockHeader = new Uint8Array([0x01, 0x00, 0x00, 0x00])
  const simpleBlock = ebmlElement(SimpleBlock, concatBytes([blockHeader, silentVorbisData]))

  const cluster = ebmlElement(Cluster, concatBytes([ebmlUint(Timecode, 0), simpleBlock]))

  const segmentContent = concatBytes([info, tracks, cluster])
  const segment = ebmlElement(Segment, segmentContent)

  return concatBytes([ebmlHeader, segment])
}

function concatBytes(parts: Uint8Array[]): Uint8Array {
  const total = parts.reduce((s, p) => s + p.length, 0)
  const out = new Uint8Array(total)
  let off = 0
  for (const p of parts) { out.set(p, off); off += p.length }
  return out
}

// ==================== MP3 ====================

export function createMP3(): Uint8Array {
  const parts: Uint8Array[] = []
  const id3v2Header = new Uint8Array([0x49, 0x44, 0x33, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x41])

  function makeId3Frame(id: string, text: string): Uint8Array {
    const enc = new TextEncoder().encode(text)
    const size = enc.length + 1
    const buf = new Uint8Array(10 + size)
    for (let i = 0; i < id.length; i++) buf[i] = id.charCodeAt(i)
    new DataView(buf.buffer).setUint32(4, size, false)
    buf[10] = 0
    for (let i = 0; i < enc.length; i++) buf[11 + i] = enc[i]
    return buf
  }

  parts.push(id3v2Header)
  parts.push(makeId3Frame('TIT2', 'Mock Music'))
  parts.push(makeId3Frame('TPE2', 'Test Artist'))

  for (let i = 0; i < 108; i++) {
    const frame = new Uint8Array(418)
    frame[0] = 0xFF
    frame[1] = 0xFB
    frame[2] = 0x90
    frame[3] = 0x00
    parts.push(frame)
  }

  return concatBytes(parts)
}

// ==================== FLAC ====================

export function createFLAC(): Uint8Array {
  const signature = new TextEncoder().encode('fLaC')

  const streamInfo = new Uint8Array(38)
  streamInfo[0] = 0x00
  streamInfo[1] = 0x22
  streamInfo[2] = 0x00
  const siView = new DataView(streamInfo.buffer)
  siView.setUint16(3, 44100, false)
  siView.setUint8(5, 1)
  siView.setUint8(6, 16)
  siView.setUint32(7, 100000, false)
  siView.setUint32(11, 500000, false)

  const paddingBlock = new Uint8Array([0x01, 0x00, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])

  const frameHeader = new Uint8Array([
    0xF8, 0xE8, 0x1F, 0xFE, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
  ])

  return concatBytes([signature, streamInfo, paddingBlock, frameHeader])
}

// ==================== PDF ====================

export function createPDF(): Uint8Array {
  const pdf = `%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj

2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj

3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>
endobj

xref
0 4
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n

trailer
<< /Size 4 /Root 1 0 R >>
startxref
190
%%EOF`
  return new TextEncoder().encode(pdf)
}

// ==================== AList Encrypt (.ae) ====================

export function createAEFile(name: string, targetSize: number): Uint8Array {
  const magic = new TextEncoder().encode('AENC')
  const nameBytes = new TextEncoder().encode(name)
  const nameLen = Math.min(nameBytes.length, 255)
  const headerLen = 8 + nameLen + 1
  const header = new Uint8Array(headerLen)
  header.set(magic, 0)
  header[4] = 0x01
  header[5] = 0x00
  header[6] = nameLen
  header.set(nameBytes.slice(0, nameLen), 7)
  header[7 + nameLen] = 0x00
  return padToSize(header, targetSize)
}

// ==================== ENCV v4 Container ====================

export function createSCCVFile(name: string, ext: string, targetSize: number): Uint8Array {
  const magic = new TextEncoder().encode('SCCV')
  const manifest = JSON.stringify({
    version: '4.0',
    originalName: name,
    originalExt: ext,
    algorithm: 'aes-256-gcm',
    createdAt: new Date().toISOString(),
    entries: [{ type: 'file', name: name, size: targetSize - 256 }],
  })

  const manifestBytes = new TextEncoder().encode(manifest)
  const header = new Uint8Array(32)
  header.set(magic, 0)
  header[4] = 0x04
  header[5] = 0x01
  const hv = new DataView(header.buffer)
  hv.setUint32(8, 32, false)
  hv.setUint32(12, manifestBytes.length, false)

  const body = new Uint8Array(32 + manifestBytes.length + 64)
  body.set(header, 0)
  body.set(manifestBytes, 32)
  return padToSize(body, targetSize)
}

// ==================== Boundary file specs ====================

const NOTES_CONTENT = `ENCV Mock Data Notes
======================

This is a multi-language UTF-8 test file.
中文测试文件 — 包含中文内容
日本語テスト — 日本語の内容
한국어 테스트 — 한국어 내용
العربية اختبار — محتوى عربي
עברית בדיקה — תוכן עברית
ไทยทดสอบ — เนื้อหาภาษาไทย
Ελληνικά δοκιμή — περιεχόμενο ελληνικά
Русский тест — содержание на русском
Deutsch Test — deutscher Inhalt
Français test — contenu français
Española prueba — contenido en español
Português teste — conteúdo em português
Italiano prova — contenuto italiano
Nederlands test — Nederlandse inhoud
Polski test — treść polska
Český test — český obsah
Magyar teszt — magyar tartalom
Türkçe deneme — Türkçe içerik
Việt Nam kiểm tra — nội dung tiếng Việt
Tiếng Việt kiểm tra — nội dung tiếng Việt
ไทยทดสอบ — เนื้อหาภาษาไทย

Special characters: !@#$%^&*()_+-=[]{}|;':",./<>?~\\\`
Numbers: 0123456789
Emoji: 😀🎉🚀🔥💯✨🎵📝🔒
`

const CSV_CONTENT = `id,name,category,size,encrypted
1,photo.jpg,image,107,false
2,screenshot.png,image,512,false
3,sample.mp4,video,45056,false
4,comedy.mkv,video,2048,false
5,music.mp3,audio,45184,false
6,podcast.flac,audio,1024,false
7,report.pdf,document,512,false
8,notes.txt,text,1024,false
9,data.csv,csv,256,false
10,secret.ae,encrypt,4096,true
11,document.ae,encrypt,8192,true
12,hidden-gem.ae,encrypt,16384,true
13,container.sccgv,container,8192,true
14,archive.scext,container,16384,true
15,bundle.scepkg,container,32768,true
`

// ==================== Spec 收集 ====================

function spec(relPath: string, data: Uint8Array | string): MockFileSpec {
  const bytes = typeof data === 'string' ? new TextEncoder().encode(data) : data
  return { relativePath: relPath, data: bytes, size: bytes.length }
}

/**
 * 收集指定 type 的所有 MockFileSpec（不写盘，纯函数）。
 * 用于前端预览 / 后端按 spec 写盘 / CLI 落盘。
 */
export function collectSpecs(type: MockFileType): MockFileSpec[] {
  const specs: MockFileSpec[] = []

  if (type === 'all' || type === 'plain') {
    specs.push(
      spec('01-plain-media/image/photo.jpg', createJPEG()),
      spec('01-plain-media/image/screenshot.png', createPNG()),
      spec('01-plain-media/video/sample.mp4', createMP4()),
      spec('01-plain-media/video/comedy.mkv', createMKV()),
      spec('01-plain-media/audio/music.mp3', createMP3()),
      spec('01-plain-media/audio/podcast.flac', createFLAC()),
      spec('01-plain-media/document/report.pdf', createPDF()),
      spec('01-plain-media/document/notes.txt', NOTES_CONTENT),
      spec('01-plain-media/document/data.csv', CSV_CONTENT),
    )
  }

  if (type === 'all' || type === 'ae') {
    specs.push(
      spec('02-alist-encrypt/secret.ae', createAEFile('secret.ae', 4096)),
      spec('02-alist-encrypt/document.ae', createAEFile('document.ae', 8192)),
      spec('02-alist-encrypt/hidden-gem.ae', createAEFile('hidden-gem.ae', 16384)),
    )
  }

  if (type === 'all' || type === 'container') {
    specs.push(
      spec('03-encv-containers/container.sccgv', createSCCVFile('container', 'sccgv', 8192)),
      spec('03-encv-containers/archive.scext', createSCCVFile('archive', 'scext', 16384)),
      spec('03-encv-containers/bundle.scepkg', createSCCVFile('bundle', 'scepkg', 32768)),
    )
  }

  if (type === 'all' || type === 'boundary') {
    specs.push(
      spec('04-boundary-test/long-unicode-filename-中文-日本語-한국어-العربية-עברית-ไทย-ελληνικά.txt', 'Unicode filename test'),
      spec('04-boundary-test/.hidden-file', randomBytes(256)),
      spec('04-boundary-test/spaces   in   name.txt', 'Spaces in filename'),
      spec('04-boundary-test/special-chars-!@#$%^&*()_+.txt', 'Special characters'),
      spec('04-boundary-test/zero-byte-file.bin', new Uint8Array(0)),
      spec('04-boundary-test/single-byte.bin', new Uint8Array([0x42])),
      spec('04-boundary-test/exactly-1kb.bin', padToSize(new Uint8Array([0x41]), 1024)),
      spec('04-boundary-test/large-1mb.dat', padToSize(new Uint8Array([0x58, 0x59, 0x5A]), 1024 * 1024)),
      spec('04-boundary-test/control-chars-\x01\x02\x03.txt', 'Control character filename'),
      spec('04-boundary-test/‫אבג-rtl-filename.txt', 'RTL filename test'),
      spec('04-boundary-test/emoji-test-😀🎉🚀🔥.txt', 'Emoji filename test'),
      spec('04-boundary-test/trailing-space.txt ', 'Trailing space'),
      spec('04-boundary-test/..dotfile', 'Dot-start filename'),
      spec('04-boundary-test/normal-dir/subdir/deep-nested.txt', 'Deep nested file content'),
      spec('04-boundary-test/MiXeD-CaSe-FiLe.TxT', 'Mixed case filename'),
    )
  }

  return specs
}

// ==================== Orchestration ====================

/**
 * 生成所有 Mock 文件。
 *
 * - writeToDisk 回调：每个 spec 调用一次，path 形如 `${root}/01-plain-media/image/photo.jpg`
 * - onProgress 回调：每个 spec 触发，可用于 UI 进度条
 *
 * 如果未传 writeToDisk，则只 collect 不写盘（用于前端预览 / 单元测试）。
 */
export async function generateMockFiles(opts: GenerateOptions): Promise<GenerateResult> {
  const type = opts.type ?? 'all'
  const specs = collectSpecs(type)
  let count = 0
  let totalSize = 0
  for (const s of specs) {
    if (opts.writeToDisk) {
      const fullPath = joinPath(opts.root, s.relativePath)
      ensureParentDir(fullPath)  // ⚠️ 防御：递归创建父目录
      await opts.writeToDisk(fullPath, s.data)
    }
    count++
    totalSize += s.size
    opts.onProgress?.(s)
  }
  return { count, totalSize, specs }
}

/**
 * 列出所有可能生成的相对路径（不含 root）。用于 reset 操作前清空目录。
 */
export function listAllRelativePaths(): string[] {
  return collectSpecs('all').map((s) => s.relativePath)
}
