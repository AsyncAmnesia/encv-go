/**
 * usePathResolver 单元测试 — withSafetyBoundary 全分支覆盖
 *
 * 覆盖场景：
 * 1. dev 模式（import.meta.env.DEV = true）→ no-op
 * 2. dev + forceAutomation → 强制改写
 * 3. release（DEV=false）+ storage 路径 → 自动改写到 encv-automation
 * 4. release + 已在 encv-automation 命名空间内 → 原样返回
 * 5. release + 非 storage 路径 + forceAutomation → 放到 __misc__
 * 6. release + 非 storage 路径 + 无 forceAutomation → 原样返回
 * 7. 空字符串 / 空白字符串
 * 8. normalize 行为：反斜杠、多余斜杠、缺少前导斜杠
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'

// Stub import.meta.env.DEV before importing the module

describe('usePathResolver', () => {
  // We need to dynamically control DEV mode.
  // Since import.meta.env is frozen at module load time,
  // we stub it via vi.stubEnv BEFORE importing.
  let resolver: ReturnType<typeof import('@/composables/usePathResolver').usePathResolver>

  async function reloadModule(devMode: boolean) {
    vi.stubEnv('DEV', devMode)
    // Dynamic re-import to pick up new env value
    const mod = await import('@/composables/usePathResolver')
    resolver = mod.usePathResolver()
  }

  describe('normalize()', () => {
    it('应去除前后空白', async () => {
      await reloadModule(false)
      expect(resolver.normalize('  /path/to/file  ')).toBe('/path/to/file')
    })

    it('应将反斜杠替换为正斜杠', async () => {
      await reloadModule(false)
      expect(resolver.normalize('\\path\\to\\file')).toBe('/path/to/file')
    })

    it('应合并连续斜杠', async () => {
      await reloadModule(false)
      expect(resolver.normalize('/path///to//file')).toBe('/path/to/file')
    })

    it('应为缺少前导斜杠的路径添加', async () => {
      await reloadModule(false)
      expect(resolver.normalize('path/to/file')).toBe('/path/to/file')
    })

    it('空字符串应返回空字符串', async () => {
      await reloadModule(false)
      expect(resolver.normalize('')).toBe('')
    })

    it('纯空白应返回空字符串', async () => {
      await reloadModule(false)
      expect(resolver.normalize('   ')).toBe('')
    })
  })

  describe('isAbsolutePath()', () => {
    it('绝对路径返回 true', async () => {
      await reloadModule(false)
      expect(resolver.isAbsolutePath('/storage/emulated/0/test')).toBe(true)
    })

    it('相对路径返回 false', async () => {
      await reloadModule(false)
      expect(resolver.isAbsolutePath('relative/path')).toBe(false)
    })
  })

  describe('getMockPaths() — dev 模式', () => {
    it('dev 模式应返回 MOCK_PATHS 数组', async () => {
      await reloadModule(true)
      const paths = resolver.getMockPaths()
      expect(paths).toEqual(['/mock/video.mp4', '/mock/doc.txt', '/mock/report.pdf', '/mock/data.csv'])
    })
  })

  describe('getMockPaths() — release 模式', () => {
    it('release 模式应返回 null', async () => {
      await reloadModule(false)
      expect(resolver.getMockPaths()).toBeNull()
    })
  })

  // ==================== withSafetyBoundary 核心测试 ====================

  describe('withSafetyBoundary() — dev 模式 (DEV=true)', () => {
    beforeEach(async () => {
      await reloadModule(true)
    })

    it('dev 模式下不传 forceAutomation 应原样返回（no-op）', () => {
      const input = '/tmp/real-file.txt'
      expect(resolver.withSafetyBoundary(input)).toBe(input)
    })

    it('dev 模式下 storage 路径也应原样返回', () => {
      const input = '/storage/emulated/0/DCIM/photo.jpg'
      expect(resolver.withSafetyBoundary(input)).toBe(input)
    })

    it('dev 模式下已 encv-automation 路径原样返回', () => {
      const input = '/storage/emulated/0/encv-automation/test.mp4'
      expect(resolver.withSafetyBoundary(input)).toBe(input)
    })

    it('dev 模式 + forceAutomation=true 应强制改写 storage 路径', () => {
      const input = '/storage/emulated/0/DCIM/photo.jpg'
      const result = resolver.withSafetyBoundary(input, { forceAutomation: true })
      expect(result).toBe('/storage/emulated/0/encv-automation/DCIM/photo.jpg')
    })

    it('dev 模式 + forceAutomation=true：已在命名空间内的路径不变', () => {
      const input = '/storage/emulated/0/encv-automation/test.mp4'
      const result = resolver.withSafetyBoundary(input, { forceAutomation: true })
      expect(result).toBe(input) // 避免双重重写
    })

    it('dev 模式 + forceAutomation=true：非 storage 路径放到 __misc__', () => {
      const result = resolver.withSafetyBoundary('/tmp/real-file.txt', { forceAutomation: true })
      expect(result).toBe('/storage/emulated/0/encv-automation/__misc__/real-file.txt')
    })

    it('空字符串返回空字符串', () => {
      expect(resolver.withSafetyBoundary('')).toBe('')
    })
  })

  describe('withSafetyBoundary() — release 模式 (DEV=false)', () => {
    beforeEach(async () => {
      await reloadModule(false)
    })

    it('release 模式：/storage/emulated/0/ 下的路径自动改写', () => {
      const result = resolver.withSafetyBoundary('/storage/emulated/0/DCIM/photo.jpg')
      expect(result).toBe('/storage/emulated/0/encv-automation/DCIM/photo.jpg')
    })

    it('release 模式：已在 encv-automation 命名空间内 → 原样返回（防双重）', () => {
      const input = '/storage/emulated/0/encv-automation/01-plain-media/video/sample.mp4'
      expect(resolver.withSafetyBoundary(input)).toBe(input)
    })

    it('release 模式：encv-automation 精确匹配根目录也原样返回', () => {
      const input = '/storage/emulated/0/encv-automation'
      expect(resolver.withSafetyBoundary(input)).toBe(input)
    })

    it('release 模式：非 storage 路径（/tmp, /data 等）原样返回', () => {
      expect(resolver.withSafetyBoundary('/tmp/test.txt')).toBe('/tmp/test.txt')
      expect(resolver.withSafetyBoundary('/data/local/tmp/x')).toBe('/data/local/tmp/x')
      expect(resolver.withSafetyBoundary('/mock/video.mp4')).toBe('/mock/video.mp4')
    })

    it('release + forceAutomation：非 storage 路径放到 __misc__', () => {
      const result = resolver.withSafetyBoundary('/tmp/real-file.txt', { forceAutomation: true })
      expect(result).toBe('/storage/emulated/0/encv-automation/__misc__/real-file.txt')
    })

    it('release + forceAutomation：storage 路径改写到命名空间', () => {
      const result = resolver.withSafetyBoundary(
        '/storage/emulated/0/Download/movie.mkv',
        { forceAutomation: true }
      )
      expect(result).toBe('/storage/emulated/0/encv-automation/Download/movie.mkv')
    })

    it('release + forceAutomation：已在命名空间内不变', () => {
      const input = '/storage/emulated/0/encv-automation/test.mp4'
      expect(resolver.withSafetyBoundary(input, { forceAutomation: true })).toBe(input)
    })
  })

  // ==================== 关键端到端场景 ====================

  describe('withSafetyBoundary() — 自动化测试关键路径', () => {
    beforeEach(async () => {
      await reloadModule(false) // 模拟真机 release 模式
    })

    it('DEFAULT_AUTOMATION_SOURCE 经过 forceAutomation 后保持不变（已在命名空间内）', async () => {
      await reloadModule(false)
      // 直接使用常量值（与 useAutomationTests.ts L77 一致）
      const DEFAULT_AUTOMATION_SOURCE = '/storage/emulated/0/encv-automation/01-plain-media/video/sample.mp4'
      const result = resolver.withSafetyBoundary(DEFAULT_AUTOMATION_SOURCE, { forceAutomation: true })
      // 关键断言：DEFAULT_AUTOMATION_SOURCE 已经在 encv-automation 命名空间内，
      // 所以 withSafetyBoundary 必须原样返回
      expect(result).toBe(DEFAULT_AUTOMATION_SOURCE)
      expect(result).toBe('/storage/emulated/0/encv-automation/01-plain-media/video/sample.mp4')
    })

    it('如果误用不带 encv-automation 前缀的路径，forceAutomation 会修正', () => {
      const wrongPath = '/storage/emulated/0/01-plain-media/video/sample.mp4'
      const result = resolver.withSafetyBoundary(wrongPath, { forceAutomation: true })
      expect(result).toBe('/storage/emulated/0/encv-automation/01-plain-media/video/sample.mp4')
    })
  })
})
