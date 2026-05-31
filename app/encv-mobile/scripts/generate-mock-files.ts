import * as fs from 'fs'
import * as path from 'path'
import * as zlib from 'zlib'
import * as crypto from 'crypto'
import * as os from 'os'
import { execSync, spawnSync } from 'child_process'

const MOCK_ROOT = path.resolve(process.cwd(), '__mock_data__')

let root = MOCK_ROOT
let genType = 'all' as string

let fileCount = 0
let totalSize = 0

function ensureDir(dir: string): void {
  fs.mkdirSync(dir, { recursive: true })
}

function writeBuffer(filePath: string, data: Uint8Array): void {
  fs.writeFileSync(filePath, Buffer.from(data))
  recordFile(filePath, data.length)
}

function writeString(filePath: string, content: string, encoding: BufferEncoding = 'utf-8'): void {
  const buf = Buffer.from(content, encoding)
  fs.writeFileSync(filePath, buf)
  recordFile(filePath, buf.length)
}

function randomBytes(n: number): Uint8Array {
  return new Uint8Array(crypto.randomBytes(n))
}

function padToSize(data: Uint8Array, targetSize: number): Uint8Array {
  if (data.length >= targetSize) return data
  const padded = new Uint8Array(targetSize)
  padded.set(data)
  const extra = randomBytes(targetSize - data.length)
  padded.set(extra, data.length)
  return padded
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

// ==================== JPEG ====================
function createJPEG(): Uint8Array {
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
    0xEA, 0xFB, 0xFD, 0xFE, 0xFF, 0xD9
  ])
}

// ==================== PNG ====================
function crc32(buf: Buffer): number {
  let c = 0xFFFFFFFF
  for (let i = 0; i < buf.length; i++) {
    c ^= buf[i]
    for (let j = 0; j < 8; j++) {
      c = (c >>> 1) ^ (c & 1 ? 0xEDB88320 : 0)
    }
  }
  return (c ^ 0xFFFFFFFF) >>> 0
}

function makeChunk(type: string, data: Buffer): Buffer {
  const len = Buffer.alloc(4)
  len.writeUInt32BE(data.length)
  const typeB = Buffer.from(type, 'ascii')
  const crcData = Buffer.concat([typeB, data])
  const crc = Buffer.alloc(4)
  crc.writeUInt32BE(crc32(crcData))
  return Buffer.concat([len, typeB, data, crc])
}

function createPNG(): Uint8Array {
  const signature = Buffer.from([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A])

  const ihdrData = Buffer.from([0, 0, 0, 4, 0, 0, 0, 4, 8, 6, 0, 0, 0])
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
  const idatRaw = Buffer.from(pixels)
  const idatCompressed = zlib.deflateSync(idatRaw)
  const idat = makeChunk('IDAT', idatCompressed)

  const iend = makeChunk('IEND', Buffer.alloc(0))

  return new Uint8Array(Buffer.concat([signature, ihdr, idat, iend]))
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
  const inner = children && children.length > 0
    ? (() => { const t = children.reduce((s, c) => s + c.length, 0); const b = new Uint8Array(t); let o = 0; for (const c of children) { b.set(c, o); o += c.length } return b })()
    : new Uint8Array(0)

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
function createMP4(): Uint8Array {
  const ftyp = mp4Box('ftyp', [
    new Uint8Array(new TextEncoder().encode('isom')),
    new Uint8Array([0x00, 0x00, 0x02, 0x00]),
    new Uint8Array(new TextEncoder().encode('isom')),
    new Uint8Array(new TextEncoder().encode('iso2')),
    new Uint8Array(new TextEncoder().encode('mp41'))
  ])

  const mvhd = mp4FullBox('mvhd', 0, 0, [
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x01]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x01, 0x00, 0x00]),
    new Uint8Array([0x01, 0x00, 0x00, 0x00]),
    new Uint8Array([0x01, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0xFF, 0xFF, 0x00, 0x00])
  ])

  const mdhd = mp4FullBox('mdhd', 0, 0, [
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x03, 0xE8]),
    new Uint8Array([0x00, 0x00, 0x00, 0x96]),
    new Uint8Array([0x55, 0xC4, 0x00, 0x00])
  ])

  const hdlr = mp4FullBox('hdlr', 0, 0, [
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array(new TextEncoder().encode('vide')),
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array(new TextEncoder().encode('VideoHandler\x00'))
  ])

  const urlBox = mp4FullBox('url ', 0, 1)
  const dref = mp4FullBox('dref', 0, 0, [urlBox])

  const dinf = mp4Box('dinf', [dref])

  const mp4aEntry = mp4Box('mp4a', [
    new Uint8Array([0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x62, 0x00, 0x01]),
    mp4Box('esds', [
      new Uint8Array([0x03, 0x80, 0x80, 0x80, 0x22, 0x00, 0x01, 0x00, 0x04, 0x80, 0x80, 0x80, 0x17, 0x40, 0x15, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x06, 0x80, 0x80, 0x80, 0x01, 0x02])
    ])
  ])
  const stsd = mp4FullBox('stsd', 0, 0, [new Uint8Array([0x00, 0x00, 0x00, 0x01]), mp4aEntry])

  const stts = mp4FullBox('stts', 0, 0, [
    new Uint8Array([0x00, 0x00, 0x00, 0x01]),
    new Uint8Array([0x00, 0x00, 0x00, 0x96]),
    new Uint8Array([0x00, 0x00, 0x00, 0x01])
  ])
  const stsc = mp4FullBox('stsc', 0, 0, [
    new Uint8Array([0x00, 0x00, 0x00, 0x01]),
    new Uint8Array([0x00, 0x00, 0x00, 0x01]),
    new Uint8Array([0x00, 0x00, 0x00, 0x01]),
    new Uint8Array([0x00, 0x00, 0x00, 0x96])
  ])
  const stsz = mp4FullBox('stsz', 0, 0, [
    new Uint8Array([0x00, 0x00, 0x00, 0x00]),
    new Uint8Array([0x00, 0x00, 0x00, 0x96]),
    ...Array(150).fill(null).map(() => new Uint8Array([0x00, 0x00, 0x01, 0xA2]))
  ])
  const stco = mp4FullBox('stco', 0, 0, [
    new Uint8Array([0x00, 0x00, 0x00, 0x01]),
    new Uint8Array([0x00, 0x00, 0x04, 0x20])
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
    new Uint8Array([0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02])
  ])
  const trak = mp4Box('trak', [tkhd, mdia])
  const moov = mp4Box('moov', [mvhd, trak])

  const silenceFrame = new Uint8Array(420)
  silenceFrame[0] = 0xFF
  silenceFrame[1] = 0xFB
  silenceFrame[2] = 0x90
  silenceFrame[3] = 0x00

  const frames: Uint8Array[] = []
  for (let i = 0; i < 50; i++) {
    frames.push(silenceFrame.slice())
  }

  const mdatBody = (() => {
    const totalLen = frames.reduce((s, f) => s + f.length, 0)
    const b = new Uint8Array(totalLen)
    let off = 0
    for (const f of frames) { b.set(f, off); off += f.length }
    return b
  })()
  const mdat = mp4Box('mdat', [mdatBody])

  return new Uint8Array(Buffer.concat([ftyp, moov, mdat]))
}

// ==================== MKV/WebM ====================
function ebmlVInt(val: number): Uint8Array {
  if (val < 127) return new Uint8Array([val])
  if (val < 16383) return new Uint8Array([0x40 | (val >> 8), val & 0xFF])
  if (val < 2097151) return new Uint8Array([0x20 | (val >> 16), (val >> 8) & 0xFF, val & 0xFF])
  return new Uint8Array([0x10 | (val >> 24), (val >> 16) & 0xFF, (val >> 8) & 0xFF, val & 0xFF])
}

function ebmlElement(id: Uint8Array, value: Uint8Array): Uint8Array {
  const size = ebmlVInt(value.length)
  const result = new Uint8Array(id.length + size.length + value.length)
  result.set(id, 0)
  result.set(size, id.length)
  result.set(value, id.length + size.length)
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
  return ebmlElement(id, new Uint8Array(new TextEncoder().encode(str)))
}

function ebmlFloat(id: Uint8Array, val: number): Uint8Array {
  const buf = new ArrayBuffer(8)
  const view = new DataView(buf)
  view.setFloat64(0, val, false)
  return ebmlElement(id, new Uint8Array(new Int8Array(buf)))
}

function createMKV(): Uint8Array {
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

  const ebmlHeader = ebmlElement(EBML, Buffer.concat([
    ebmlUint(DocTypeVersion, 4),
    ebmlUint(DocTypeReadVersion, 4),
    ebmlString(DocType, 'matroska')
  ]))

  const audioTrack = ebmlElement(TrackEntry, Buffer.concat([
    ebmlUint(TrackNumber, 1),
    ebmlUint(TrackUID, 12345),
    ebmlUint(TrackType, 2),
    ebmlString(CodecID, 'A_VORBIS'),
    ebmlElement(CodecPrivate, new Uint8Array([0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00]))
  ]))

  const info = ebmlElement(Info, Buffer.concat([
    ebmlUint(TimecodeScale, 1000000),
    ebmlFloat(Duration, 2.5)
  ]))

  const tracks = ebmlElement(Tracks, [audioTrack])

  const silentVorbisData = new Uint8Array(64).fill(0)
  const blockHeader = new Uint8Array([0x01, 0x00, 0x00, 0x00])
  const simpleBlock = ebmlElement(SimpleBlock, Buffer.concat([blockHeader, silentVorbisData]))

  const cluster = ebmlElement(Cluster, Buffer.concat([
    ebmlUint(Timecode, 0),
    simpleBlock
  ]))

  const segmentContent = Buffer.concat([info, tracks, cluster])
  const segment = ebmlElement(Segment, segmentContent)

  return new Uint8Array(Buffer.concat([ebmlHeader, segment]))
}

// ==================== MP3 ====================
function createMP3(): Uint8Array {
  const parts: Uint8Array[] = []

  const id3v2Header = Buffer.from([
    0x49, 0x44, 0x33, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x41
  ])

  function makeId3Frame(id: string, text: string): Buffer {
    const enc = new TextEncoder().encode(text)
    const size = enc.length + 1
    const buf = Buffer.alloc(10 + size)
    buf.write(id, 0, 'ascii')
    buf.writeUInt32BE(size, 4)
    buf[8] = 0
    buf[9] = 0
    buf[10] = 0
    buf.set(enc, 11)
    return buf
  }

  parts.push(new Uint8Array(id3v2Header))
  parts.push(new Uint8Array(makeId3Frame('TIT2', 'Mock Music')))
  parts.push(new Uint8Array(makeId3Frame('TPE2', 'Test Artist')))

  for (let i = 0; i < 108; i++) {
    const frame = new Uint8Array(418)
    frame[0] = 0xFF
    frame[1] = 0xFB
    frame[2] = 0x90
    frame[3] = 0x00
    parts.push(frame)
  }

  const totalLen = parts.reduce((s, p) => s + p.length, 0)
  const result = new Uint8Array(totalLen)
  let off = 0
  for (const p of parts) { result.set(p, off); off += p.length }
  return result
}

// ==================== FLAC ====================
function createFLAC(): Uint8Array {
  const signature = new Uint8Array(new TextEncoder().encode('fLaC'))

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
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00
  ])

  const result = new Uint8Array(signature.length + streamInfo.length + paddingBlock.length + frameHeader.length)
  result.set(signature, 0)
  result.set(streamInfo, signature.length)
  result.set(paddingBlock, signature.length + streamInfo.length)
  result.set(frameHeader, signature.length + streamInfo.length + paddingBlock.length)
  return result
}

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
    '-shortest'
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
    '-shortest'
  ], 'MKV', 'mkv')
}

function createValidMP3(): Buffer {
  if (!hasFFmpeg) { console.warn('[WARN] ffmpeg not found, using legacy unplayable MP3'); return Buffer.from(createMP3()) }
  return ffmpegGenerate([
    '-f', 'lavfi',
    '-i', 'sine=frequency=440:duration=2',
    '-c:a', 'libmp3lame', '-b:a', '128k'
  ], 'MP3', 'mp3')
}

function createValidFLAC(): Buffer {
  if (!hasFFmpeg) { console.warn('[WARN] ffmpeg not found, using legacy unplayable FLAC'); return Buffer.from(createFLAC()) }
  return ffmpegGenerate([
    '-f', 'lavfi',
    '-i', 'sine=frequency=440:duration=2',
    '-c:a', 'flac', '-sample_fmt', 's16'
  ], 'FLAC', 'flac')
}

// ==================== PDF ====================
function createPDF(): Uint8Array {
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
  return new Uint8Array(new TextEncoder().encode(pdf))
}

// ==================== AList Encrypt (.ae) ====================
function createAEFile(name: string, targetSize: number): Uint8Array {
  const magic = new Uint8Array(new TextEncoder().encode('AENC'))
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
function createSCCVFile(name: string, ext: string, targetSize: number): Uint8Array {
  const magic = new Uint8Array(new TextEncoder().encode('SCCV'))
  const manifest = JSON.stringify({
    version: '4.0',
    originalName: name,
    originalExt: ext,
    algorithm: 'aes-256-gcm',
    createdAt: new Date().toISOString(),
    entries: [{ type: 'file', name: name, size: targetSize - 256 }]
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
  body.set(new Uint8Array(manifestBytes), 32)
  return padToSize(body, targetSize)
}

// ==================== Boundary Test Files ====================
function generateBoundaryFiles(baseDir: string): void {
  const dir = baseDir
  ensureDir(dir)

  writeString(join(dir, 'long-unicode-filename-中文-日本語-한국어-العربية-עברית-ไทย-ελληνικά.txt'), 'Unicode filename test')
  writeBuffer(join(dir, '.hidden-file'), randomBytes(256))
  writeString(join(dir, 'spaces   in   name.txt'), 'Spaces in filename')
  writeString(join(dir, 'special-chars-!@#$%^&*()_+.txt'), 'Special characters')

  const emptyBuf = new Uint8Array(0)
  writeBuffer(join(dir, 'zero-byte-file.bin'), emptyBuf)

  const singleByte = new Uint8Array([0x42])
  writeBuffer(join(dir, 'single-byte.bin'), singleByte)

  const kb1 = padToSize(new Uint8Array([0x41]), 1024)
  writeBuffer(join(dir, 'exactly-1kb.bin'), kb1)

  const largeFile = padToSize(new Uint8Array([0x58, 0x59, 0x5A]), 1024 * 1024)
  writeBuffer(join(dir, 'large-1mb.dat'), largeFile)

  const ctrlName = 'control-chars-\x01\x02\x03.txt'
  writeString(join(dir, ctrlName), 'Control character filename')

  const rtlName = '\u200F\u05D0\u05D1\u05D2-rtl-filename.txt'
  writeString(join(dir, rtlName), 'RTL filename test')

  const emojiName = 'emoji-test-😀🎉🚀🔥.txt'
  writeString(join(dir, emojiName), 'Emoji filename test')

  const nullishName = 'trailing-space.txt '
  writeString(join(dir, nullishName), 'Trailing space')

  const dotName = '..dotfile'
  writeString(join(dir, dotName), 'Dot-start filename')

  const deepPath = join(dir, 'normal-dir', 'subdir', 'deep-nested.txt')
  ensureDir(path.dirname(deepPath))
  writeString(deepPath, 'Deep nested file content')

  const mixedCase = 'MiXeD-CaSe-FiLe.TxT'
  writeString(join(dir, mixedCase), 'Mixed case filename')
}

// ==================== Main ====================
function parseArgs(): void {
  const args = process.argv.slice(2)
  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--dir' && args[i + 1]) {
      root = path.resolve(args[++i])
    } else if (args[i] === '--type' && args[i + 1]) {
      genType = args[++i]
    }
  }
}

async function main(): Promise<void> {
  parseArgs()

  console.log('📦 ENCV Mock File Generator')
  console.log('   Output: ' + root)
  console.log('   Type: ' + genType)
  console.log('   FFmpeg: ' + (hasFFmpeg ? '✅ (playable media)' : '❌ (legacy binary)') + '\n')

  const shouldGenPlain = genType === 'all' || genType === 'plain'
  const shouldGenAE = genType === 'all' || genType === 'ae'
  const shouldGenContainer = genType === 'all' || genType === 'container'
  const shouldGenBoundary = genType === 'all' || genType === 'boundary'

  if (shouldGenPlain) {
    console.log('--- Plain Media ---')
    ensureDir(join(root, '01-plain-media/image'))
    ensureDir(join(root, '01-plain-media/video'))
    ensureDir(join(root, '01-plain-media/audio'))
    ensureDir(join(root, '01-plain-media/document'))

    writeBuffer(join(root, '01-plain-media/image/photo.jpg'), createJPEG())
    writeBuffer(join(root, '01-plain-media/image/screenshot.png'), createPNG())
    writeBuffer(join(root, '01-plain-media/video/sample.mp4'), createValidMP4())
    writeBuffer(join(root, '01-plain-media/video/comedy.mkv'), createValidMKV())
    writeBuffer(join(root, '01-plain-media/audio/music.mp3'), createValidMP3())
    writeBuffer(join(root, '01-plain-media/audio/podcast.flac'), createValidFLAC())
    writeBuffer(join(root, '01-plain-media/document/report.pdf'), createPDF())

    writeString(join(root, '01-plain-media/document/notes.txt'), `ENCV Mock Data Notes
======================

This is a multi-language UTF-8 test file.
中文测试文件 — 包含中文内容
日本語テスト — 日本語の内容
한국어 테스트 — 한국어 내용
العربية اختبار — محتوى عربي
עברית בדיקה — תוכן עברי
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

Special characters: !@#$%^&*()_+-=[]{}|;':",./<>?~\`
Numbers: 0123456789
Emoji: 😀🎉🚀🔥💯✨🎵📝🔒
`)

    writeString(join(root, '01-plain-media/document/data.csv'), `id,name,category,size,encrypted
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
`)

    console.log('')
  }

  if (shouldGenAE) {
    console.log('--- AList Encrypt (.ae) ---')
    ensureDir(join(root, '02-alist-encrypt'))

    writeBuffer(join(root, '02-alist-encrypt/secret.ae'), createAEFile('secret.ae', 4096))
    writeBuffer(join(root, '02-alist-encrypt/document.ae'), createAEFile('document.ae', 8192))
    writeBuffer(join(root, '02-alist-encrypt/hidden-gem.ae'), createAEFile('hidden-gem.ae', 16384))

    console.log('')
  }

  if (shouldGenContainer) {
    console.log('--- ENCV v4 Containers ---')
    ensureDir(join(root, '03-encv-containers'))

    writeBuffer(join(root, '03-encv-containers/container.sccgv'), createSCCVFile('container', 'sccgv', 8192))
    writeBuffer(join(root, '03-encv-containers/archive.scext'), createSCCVFile('archive', 'scext', 16384))
    writeBuffer(join(root, '03-encv-containers/bundle.scepkg'), createSCCVFile('bundle', 'scepkg', 32768))

    console.log('')
  }

  if (shouldGenBoundary) {
    console.log('--- Boundary Tests ---')
    generateBoundaryFiles(join(root, '04-boundary-test'))
    console.log('')
  }

  printSummary()
}

main().catch(function (e: unknown) {
  console.error(e)
  process.exit(1)
})
