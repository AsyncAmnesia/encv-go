/**
 * src/lib/mockDataGenerator 单元测试
 *
 * 覆盖：
 * 1. collectSpecs 返回正确的相对路径前缀
 * 2. createJPEG / createPNG / createMP4 / createMKV / createMP3 / createFLAC / createPDF 生成合规的字节
 * 3. createAEFile / createSCCVFile 包含正确 magic header
 * 4. generateMockFiles 调用 writeToDisk 回调的次数 = specs.length
 * 5. 默认 type='all' 包含 4 个 section（plain/ae/container/boundary）
 */
import { describe, it, expect, vi } from 'vitest'
import {
  collectSpecs,
  generateMockFiles,
  createJPEG,
  createPNG,
  createMP4,
  createMKV,
  createMP3,
  createFLAC,
  createPDF,
  createAEFile,
  createSCCVFile,
  type MockFileType,
} from '@/lib/mockDataGenerator'

describe('collectSpecs', () => {
  it('type=plain：所有相对路径含 01-plain-media', () => {
    const specs = collectSpecs('plain')
    expect(specs.length).toBeGreaterThan(0)
    for (const s of specs) {
      expect(s.relativePath.startsWith('01-plain-media/')).toBe(true)
    }
  })

  it('type=ae：扩展名 .ae', () => {
    const specs = collectSpecs('ae')
    expect(specs.length).toBeGreaterThan(0)
    for (const s of specs) {
      expect(s.relativePath.endsWith('.ae')).toBe(true)
      expect(s.relativePath.startsWith('02-alist-encrypt/')).toBe(true)
    }
  })

  it('type=container：含 sccgv/scext/scepkg', () => {
    const specs = collectSpecs('container')
    expect(specs.length).toBeGreaterThan(0)
    for (const s of specs) {
      expect(s.relativePath.startsWith('03-encv-containers/')).toBe(true)
    }
  })

  it('type=boundary：含特殊文件名测试', () => {
    const specs = collectSpecs('boundary')
    expect(specs.length).toBeGreaterThan(0)
    for (const s of specs) {
      expect(s.relativePath.startsWith('04-boundary-test/')).toBe(true)
    }
  })

  it('type=all：所有 section 之和', () => {
    const all = collectSpecs('all')
    const plain = collectSpecs('plain')
    const ae = collectSpecs('ae')
    const container = collectSpecs('container')
    const boundary = collectSpecs('boundary')
    expect(all.length).toBe(plain.length + ae.length + container.length + boundary.length)
  })

  it('每个 spec 有 data (Uint8Array) 和 size', () => {
    for (const type of ['plain', 'ae', 'container', 'boundary'] as MockFileType[]) {
      const specs = collectSpecs(type)
      for (const s of specs) {
        expect(s.data).toBeInstanceOf(Uint8Array)
        expect(s.size).toBe(s.data.length)
      }
    }
  })
})

describe('createJPEG', () => {
  it('头 2 字节 = 0xFF 0xD8 (SOI)', () => {
    const data = createJPEG()
    expect(data[0]).toBe(0xFF)
    expect(data[1]).toBe(0xD8)
  })

  it('尾 2 字节 = 0xFF 0xD9 (EOI)', () => {
    const data = createJPEG()
    expect(data[data.length - 2]).toBe(0xFF)
    expect(data[data.length - 1]).toBe(0xD9)
  })
})

describe('createPNG', () => {
  it('前 8 字节 = PNG signature', () => {
    const data = createPNG()
    const expected = [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A]
    for (let i = 0; i < 8; i++) {
      expect(data[i]).toBe(expected[i])
    }
  })

  it('包含 IEND chunk', () => {
    const data = createPNG()
    const text = new TextDecoder().decode(data)
    expect(text.includes('IEND')).toBe(true)
  })
})

describe('createMP4', () => {
  it('含 ftyp box', () => {
    const data = createMP4()
    const text = new TextDecoder().decode(data)
    expect(text.includes('ftyp')).toBe(true)
  })

  it('含 moov box', () => {
    const data = createMP4()
    const text = new TextDecoder().decode(data)
    expect(text.includes('moov')).toBe(true)
  })
})

describe('createMKV', () => {
  it('以 EBML header 开头', () => {
    const data = createMKV()
    // EBML magic: 1A 45 DF A3
    expect(data[0]).toBe(0x1A)
    expect(data[1]).toBe(0x45)
    expect(data[2]).toBe(0xDF)
    expect(data[3]).toBe(0xA3)
  })

  it('含 matroska DocType', () => {
    const data = createMKV()
    const text = new TextDecoder().decode(data)
    expect(text.includes('matroska')).toBe(true)
  })
})

describe('createMP3', () => {
  it('以 ID3 header 开头', () => {
    const data = createMP3()
    expect(new TextDecoder().decode(data.slice(0, 3))).toBe('ID3')
  })
})

describe('createFLAC', () => {
  it('以 fLaC 开头', () => {
    const data = createFLAC()
    expect(new TextDecoder().decode(data.slice(0, 4))).toBe('fLaC')
  })
})

describe('createPDF', () => {
  it('以 %PDF- 开头', () => {
    const data = createPDF()
    expect(new TextDecoder().decode(data.slice(0, 5))).toBe('%PDF-')
  })

  it('以 %%EOF 结尾', () => {
    const data = createPDF()
    const text = new TextDecoder().decode(data)
    expect(text.trim().endsWith('%%EOF')).toBe(true)
  })
})

describe('createAEFile / createSCCVFile', () => {
  it('createAEFile 前 4 字节 = AENC magic', () => {
    const data = createAEFile('test.ae', 1024)
    expect(new TextDecoder().decode(data.slice(0, 4))).toBe('AENC')
    expect(data.length).toBe(1024)
  })

  it('createSCCVFile 前 4 字节 = SCCV magic', () => {
    const data = createSCCVFile('foo', 'sccgv', 4096)
    expect(new TextDecoder().decode(data.slice(0, 4))).toBe('SCCV')
    expect(data.length).toBe(4096)
  })
})

describe('generateMockFiles', () => {
  it('writeToDisk 回调调用次数 = specs.length', async () => {
    const writeToDisk = vi.fn().mockResolvedValue(undefined)
    const result = await generateMockFiles({ root: '/tmp', type: 'all', writeToDisk })
    expect(writeToDisk).toHaveBeenCalledTimes(result.count)
  })

  it('无 writeToDisk 时只 collect 不抛', async () => {
    const result = await generateMockFiles({ root: '/tmp', type: 'plain' })
    expect(result.count).toBe(collectSpecs('plain').length)
    expect(result.specs.length).toBe(result.count)
  })

  it('onProgress 每个 spec 触发一次', async () => {
    const onProgress = vi.fn()
    const result = await generateMockFiles({ root: '/tmp', type: 'all', onProgress })
    expect(onProgress).toHaveBeenCalledTimes(result.count)
  })

  it('type 不传默认 all', async () => {
    const result = await generateMockFiles({ root: '/tmp' })
    expect(result.count).toBe(collectSpecs('all').length)
  })

  it('总大小 = 所有 spec.size 之和', async () => {
    const result = await generateMockFiles({ root: '/tmp' })
    const expected = collectSpecs('all').reduce((s, sp) => s + sp.size, 0)
    expect(result.totalSize).toBe(expected)
  })
})
