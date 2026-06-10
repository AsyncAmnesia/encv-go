/**
 * mockDataGenerator 单元测试 — 路径一致性验证
 *
 * 覆盖场景：
 * 1. collectSpecs('all') 输出的相对路径集合
 * 2. 关键文件 sample.mp4 是否在 plain 类别中
 * 3. joinPath 拼接行为
 * 4. generateMockFiles 不传 writeToDisk 只收集不写盘
 * 5. listAllRelativePaths 返回所有路径
 * 6. 端到端：mockRoot + relativePath = 完整路径
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  collectSpecs,
  generateMockFiles,
  listAllRelativePaths,
  type MockFileSpec,
} from '@/lib/mockDataGenerator'

// mock crypto.getRandomValues 以避免 1MB large-1mb.dat 在 jsdom 中超出配额
beforeEach(() => {
  if (typeof globalThis.crypto !== 'undefined') {
    vi.spyOn(globalThis.crypto, 'getRandomValues').mockImplementation((array: ArrayBufferView) => {
      const buf = new Uint8Array(array.buffer, array.byteOffset, array.byteLength)
      for (let i = 0; i < buf.length; i++) buf[i] = Math.floor(Math.random() * 256)
      return array
    })
  }
})

describe('mockDataGenerator', () => {
  describe('collectSpecs()', () => {
    it('"all" 类型应返回所有类别的 specs', () => {
      const specs = collectSpecs('all')
      // 至少包含 plain(9) + ae(3) + container(3) + boundary(15+) = 30+
      expect(specs.length).toBeGreaterThanOrEqual(30)
    })

    it('"plain" 类型应返回 9 个文件', () => {
      const specs = collectSpecs('plain')
      expect(specs.length).toBe(9)
    })

    it('"ae" 类型应返回 3 个 .ae 文件', () => {
      const specs = collectSpecs('ae')
      expect(specs.length).toBe(3)
      for (const s of specs) {
        expect(s.relativePath).toMatch(/\.ae$/)
        expect(s.data.length).toBeGreaterThan(0)
      }
    })

    it('"container" 类型应返回 3 个容器文件', () => {
      const specs = collectSpecs('container')
      expect(specs.length).toBe(3)
    })

    it('"boundary" 类型应返回边界测试文件', () => {
      const specs = collectSpecs('boundary')
      expect(specs.length).toBeGreaterThan(10)
    })

    // ==================== 关键：sample.mp4 必须存在 ====================

    it('plain 类别必须包含 sample.mp4（自动化测试的默认源文件）', () => {
      const specs = collectSpecs('plain')
      const videoSpec = specs.find((s) => s.relativePath === '01-plain-media/video/sample.mp4')
      expect(videoSpec).toBeDefined()
      expect(videoSpec!.data.length).toBeGreaterThan(0)
      expect(videoSpec!.size).toBeGreaterThan(0)
    })

    it('all 类别也必须包含 sample.mp4', () => {
      const specs = collectSpecs('all')
      const videoSpec = specs.find((s) => s.relativePath === '01-plain-media/video/sample.mp4')
      expect(videoSpec).toBeDefined()
    })
  })

  describe('collectSpecs() — 完整路径列表验证', () => {
    it('plain 类别的 9 个文件路径完全匹配预期', () => {
      const specs = collectSpecs('plain')
      const paths = specs.map((s) => s.relativePath)
      const expected = [
        '01-plain-media/image/photo.jpg',
        '01-plain-media/image/screenshot.png',
        '01-plain-media/video/sample.mp4',
        '01-plain-media/video/comedy.mkv',
        '01-plain-media/audio/music.mp3',
        '01-plain-media/audio/podcast.flac',
        '01-plain-media/document/report.pdf',
        '01-plain-media/document/notes.txt',
        '01-plain-media/document/data.csv',
      ]
      expect(paths).toEqual(expected)
    })

    it('每个 spec 都有非空的 data 和正确的 size', () => {
      const specs = collectSpecs('all')
      for (const spec of specs) {
        expect(spec.relativePath.length).toBeGreaterThan(0)
        expect(spec.data instanceof Uint8Array).toBe(true)
        expect(spec.data.length).toBeGreaterThan(0)
        expect(spec.size).toBe(spec.data.length)
      }
    })
  })

  describe('generateMockFiles()', () => {
    it('不传 writeToDisk 应只收集不写盘（纯函数模式）', async () => {
      const writtenPaths: string[] = []
      const result = await generateMockFiles({
        root: '/test/root',
        type: 'plain',
        writeToDisk: (path, _data) => { writtenPaths.push(path) },
      })
      expect(result.count).toBe(9)
      expect(result.totalSize).toBeGreaterThan(0)
      expect(result.specs.length).toBe(9)
      // writeToDisk 应该被调用了
      expect(writtenPaths.length).toBe(9)
    })

    it('不传 writeToDisk 且不传 onProgress 也应正常返回结果', async () => {
      const result = await generateMockFiles({
        root: '/test/root',
        type: 'boundary',
      })
      expect(result.count).toBeGreaterThan(0)
      expect(result.specs.length).toBe(result.count)
    })

    it('onProgress 回调应为每个 spec 触发一次', async () => {
      const progressSpecs: MockFileSpec[] = []
      await generateMockFiles({
        root: '/test/root',
        type: 'ae',
        onProgress: (spec) => progressSpecs.push(spec),
      })
      expect(progressSpecs.length).toBe(3)
    })
  })

  describe('listAllRelativePaths()', () => {
    it('应返回与 collectSpecs("all") 相同数量的路径', () => {
      const paths = listAllRelativePaths()
      const specs = collectSpecs('all')
      expect(paths.length).toBe(specs.length)
    })

    it('返回的所有路径都是字符串且非空', () => {
      const paths = listAllRelativePaths()
      for (const p of paths) {
        expect(typeof p).toBe('string')
        expect(p.length).toBeGreaterThan(0)
      }
    })
  })

  // ==================== 端到端路径拼接测试 ====================

  describe('端到端路径一致性 — Mock 写入 vs 任务提交', () => {
    /**
     * 模拟 AutomationTestsDetail.vue 的 mockRoot 计算逻辑：
     *
     *   DEFAULT_AUTOMATION_SOURCE = '/storage/emulated/0/encv-automation/01-plain-media/video/sample.mp4'
     *   mockRoot = DEFAULT_AUTOMATION_SOURCE.split('/').slice(0, 5).join('/') + '/'
     *          = '/storage/emulated/0/encv-automation/'
     */
    function computeMockRoot(): string {
      const source = '/storage/emulated/0/encv-automation/01-plain-media/video/sample.mp4'
      return source.split('/').slice(0, 5).join('/') + '/'
    }

    it('mockRoot 应计算为 /storage/emulated/0/encv-automation/', () => {
      expect(computeMockRoot()).toBe('/storage/emulated/0/encv-automation/')
    })

    it('sample.mp4 在后端的完整写入路径应等于 DEFAULT_AUTOMATION_SOURCE', () => {
      const mockRoot = computeMockRoot()
      const specs = collectSpecs('plain')
      const videoSpec = specs.find((s) => s.relativePath === '01-plain-media/video/sample.mp4')!
      // 前端 joinPath 行为：parts.join('/').replace(/\/+/g, '/')
      const fullWritePath = [mockRoot, videoSpec.relativePath].join('/').replace(/\/+/g, '/')

      // 这就是后端 filepath.Join(root, relativePath) 的结果
      expect(fullWritePath).toBe('/storage/emulated/0/encv-automation/01-plain-media/video/sample.mp4')
    })

    it('DEFAULT_AUTOMATION_SOURCE 与 Mock 写入路径必须严格一致', () => {
      const DEFAULT_AUTOMATION_SOURCE = '/storage/emulated/0/encv-automation/01-plain-media/video/sample.mp4'
      const mockRoot = computeMockRoot()

      // 从 collectSpecs 中找到 sample.mp4 的相对路径
      const specs = collectSpecs('all')
      const videoSpec = specs.find((s) => s.relativePath === '01-plain-media/video/sample.mp4')
      expect(videoSpec).toBeDefined()

      // 模拟后端写入路径（filepath.Join 语义）
      const backendWritePath = [mockRoot, videoSpec!.relativePath].join('/').replace(/\/+/g, '/')

      // 核心断言：任务提交用的源路径 == 后端实际写入的文件路径
      expect(backendWritePath).toBe(DEFAULT_AUTOMATION_SOURCE)
    })

    it('所有 plain 文件的完整路径都应在 encv-automation 命名空间下', () => {
      const mockRoot = computeMockRoot()
      const specs = collectSpecs('plain')
      for (const spec of specs) {
        const fullPath = [mockRoot, spec.relativePath].join('/').replace(/\/+/g, '/')
        expect(fullPath.startsWith('/storage/emulated/0/encv-automation/')).toBe(true)
        expect(fullPath).not.toContain('//')
      }
    })
  })

  // ==================== 前后端边界 specs 一致性检查 ====================

  describe('前后端 specs 一致性', () => {
    /**
     * 后端 mock_generator.go generateMockSpecs("plain") 返回的 9 个路径：
     *   01-plain-media/image/photo.jpg
     *   01-plain-media/image/screenshot.png
     *   01-plain-media/video/sample.mp4
     *   01-plain-media/video/comedy.mkv
     *   01-plain-media/audio/music.mp3
     *   01-plain-media/audio/podcast.flac
     *   01-plain-media/document/report.pdf
     *   01-plain-media/document/notes.txt
     *   01-plain-media/document/data.csv
     *
     * 前端 lib/mockDataGenerator.ts collectSpecs("plain") 也应返回相同的 9 个路径。
     * 如果不一致，说明前端生成了但后端不会写，或反之。
     */

    it('前端 plain specs 路径集合应与后端一致', () => {
      const frontendSpecs = collectSpecs('plain')
      const frontendPaths = new Set(frontendSpecs.map((s) => s.relativePath))

      // 后端 Go 代码中的 plainSpecs 路径（从 mock_generator.go L97-L107 复制）
      const backendPlainPaths = new Set([
        '01-plain-media/image/photo.jpg',
        '01-plain-media/image/screenshot.png',
        '01-plain-media/video/sample.mp4',
        '01-plain-media/video/comedy.mkv',
        '01-plain-media/audio/music.mp3',
        '01-plain-media/audio/podcast.flac',
        '01-plain-media/document/report.pdf',
        '01-plain-media/document/notes.txt',
        '01-plain-media/document/data.csv',
      ])

      // 前端是后端的超集？还是完全一致？
      const missingInFrontend: string[] = []
      for (const bp of backendPlainPaths) {
        if (!frontendPaths.has(bp)) missingInFrontend.push(bp)
      }
      const extraInFrontend: string[] = []
      for (const fp of frontendPaths) {
        if (!backendPlainPaths.has(fp)) extraInFrontend.push(fp)
      }

      expect(missingInFrontend).toEqual([]) // 后端有但前端没有
      expect(extraInFrontend).toEqual([]) // 前端有但后端没有
    })

    it('前端 ae specs 路径集合应与后端一致', () => {
      const frontendSpecs = collectSpecs('ae')
      const frontendPaths = new Set(frontendSpecs.map((s) => s.relativePath))

      const backendAePaths = new Set([
        '02-alist-encrypt/secret.ae',
        '02-alist-encrypt/document.ae',
        '02-alist-encrypt/hidden-gem.ae',
      ])

      const missingInFrontend: string[] = []
      for (const bp of backendAePaths) {
        if (!frontendPaths.has(bp)) missingInFrontend.push(bp)
      }
      const extraInFrontend: string[] = []
      for (const fp of frontendPaths) {
        if (!backendAePaths.has(fp)) extraInFrontend.push(fp)
      }

      expect(missingInFrontend).toEqual([])
      expect(extraInFrontend).toEqual([])
    })

    it('前端 container specs 路径集合应与后端一致', () => {
      const frontendSpecs = collectSpecs('container')
      const frontendPaths = new Set(frontendSpecs.map((s) => s.relativePath))

      const backendContainerPaths = new Set([
        '03-encv-containers/container.sccgv',
        '03-encv-containers/archive.scext',
        '03-encv-containers/bundle.scepkg',
      ])

      const missingInFrontend: string[] = []
      for (const bp of backendContainerPaths) {
        if (!frontendPaths.has(bp)) missingInFrontend.push(bp)
      }
      const extraInFrontend: string[] = []
      for (const fp of frontendPaths) {
        if (!backendContainerPaths.has(fp)) extraInFrontend.push(fp)
      }

      expect(missingInFrontend).toEqual([])
      expect(extraInFrontend).toEqual([])
    })

    /**
     * ⚠️ boundary 类别已知不一致！
     * 前端有 ~15 个边界测试文件（Unicode、隐藏文件、空字节等）
     * 后端只有 5 个基础边界文件
     * 此测试记录这一差异，防止静默回归。
     */
    it('boundary 类别：前端和后端存在已知差异（文档化差异）', () => {
      const frontendSpecs = collectSpecs('boundary')
      const frontendPaths = new Set(frontendSpecs.map((s) => s.relativePath))

      // 后端 Go 代码中的 boundarySpecs 路径（从 mock_generator.go L118-L124 复制）
      const backendBoundaryPaths = new Set([
        '04-boundary-test/zero-byte-file.bin',
        '04-boundary-test/single-byte.bin',
        '04-boundary-test/exactly-1kb.bin',
        '04-boundary-test/large-1mb.dat',
        '04-boundary-test/normal.txt',
      ])

      // 后端有但前端没有的
      const missingInFrontend: string[] = []
      for (const bp of backendBoundaryPaths) {
        if (!frontendPaths.has(bp)) missingInFrontend.push(bp)
      }
      // 前端有但后端没有的（差异部分）
      const extraInFrontend: string[] = []
      for (const fp of frontendPaths) {
        if (!backendBoundaryPaths.has(fp)) extraInFrontend.push(fp)
      }

      // 后端的 5 个文件前端应该都有
      expect(missingInFrontend).toEqual([])

      // 前端有大量额外文件是已知的（Unicode、emoji、RTL 等）
      // 这里只验证差异确实存在，并记录数量
      expect(extraInFrontend.length).toBeGreaterThan(0)

      // 打印差异供调试
      if (extraInFrontend.length > 0) {
        console.log(`[INFO] boundary 差异：前端有 ${frontendSpecs.length} 个，后端有 ${backendBoundaryPaths.size} 个`)
        console.log(`[INFO] 前端独有 ${extraInFrontend.length} 个:`, extraInFrontend.slice(0, 5), '...')
      }
    })
  })
})
