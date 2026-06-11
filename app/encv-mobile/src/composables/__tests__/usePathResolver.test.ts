/**
 * usePathResolver.withSafetyBoundary 单元测试
 *
 * 重点覆盖：
 * 1. dev 模式（import.meta.env.DEV=true）→ no-op
 * 2. 真机模式（DEV=false）→ /storage/emulated/0/* 自动改写到 encv-automation
 * 3. 已在 encv-automation 内的路径不重复包裹
 * 4. forceAutomation 选项强制改写
 * 5. 非 storage 路径行为
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { usePathResolver } from '@/composables/usePathResolver'

describe('usePathResolver.withSafetyBoundary', () => {
  beforeEach(() => {
    // vitest 默认 import.meta.env.DEV = true（test mode）
    // 显式 stub 为 true，确保 beforeEach 进入的初始状态可预测
    vi.stubEnv('DEV', true)
  })

  afterEach(() => {
    vi.unstubAllEnvs()
  })

  // 真机（release）模式
  function withProd(): void {
    vi.stubEnv('DEV', false)
    vi.stubEnv('PROD', true)
  }
  // dev 模式
  function withDev(): void {
    vi.stubEnv('DEV', true)
    vi.stubEnv('PROD', false)
  }

  it('dev 模式 + 普通调用：原路径不变', () => {
    withDev()
    const { withSafetyBoundary } = usePathResolver()
    expect(withSafetyBoundary('/storage/emulated/0/Download/foo.txt'))
      .toBe('/storage/emulated/0/Download/foo.txt')
  })

  it('dev 模式 + 普通调用：mock 路径不变', () => {
    withDev()
    const { withSafetyBoundary } = usePathResolver()
    expect(withSafetyBoundary('/mock/video.mp4')).toBe('/mock/video.mp4')
  })

  it('真机 + 普通调用：/storage/emulated/0/foo 改写到 encv-automation/foo', () => {
    withProd()
    const { withSafetyBoundary } = usePathResolver()
    expect(withSafetyBoundary('/storage/emulated/0/Download/photo.jpg'))
      .toBe('/storage/emulated/0/encv-automation/Download/photo.jpg')
  })

  it('真机 + 普通调用：/storage/emulated/0/encv-automation 已在命名空间内，不重复包裹', () => {
    withProd()
    const { withSafetyBoundary } = usePathResolver()
    expect(withSafetyBoundary('/storage/emulated/0/encv-automation/01-plain-media/video/sample.mp4'))
      .toBe('/storage/emulated/0/encv-automation/01-plain-media/video/sample.mp4')
  })

  it('真机 + 普通调用：/storage/emulated/0/encv-automation-sub/foo 不被改写（边界测试）', () => {
    withProd()
    const { withSafetyBoundary } = usePathResolver()
    // /encv-automation-sub 不是 encv-automation 子路径
    expect(withSafetyBoundary('/storage/emulated/0/encv-automation-sub/foo.txt'))
      .toBe('/storage/emulated/0/encv-automation/encv-automation-sub/foo.txt')
  })

  it('真机 + 普通调用：/storage/emulated/0/encv-automation（精确）不重复包裹', () => {
    withProd()
    const { withSafetyBoundary } = usePathResolver()
    expect(withSafetyBoundary('/storage/emulated/0/encv-automation'))
      .toBe('/storage/emulated/0/encv-automation')
  })

  it('真机 + 普通调用：非 storage 路径（/tmp, /data, /mock）不变', () => {
    withProd()
    const { withSafetyBoundary } = usePathResolver()
    expect(withSafetyBoundary('/tmp/test.bin')).toBe('/tmp/test.bin')
    expect(withSafetyBoundary('/data/local/tmp/foo')).toBe('/data/local/tmp/foo')
    expect(withSafetyBoundary('/mock/video.mp4')).toBe('/mock/video.mp4')
  })

  it('真机 + forceAutomation：/storage/emulated/0 改写到 encv-automation', () => {
    withProd()
    const { withSafetyBoundary } = usePathResolver()
    expect(withSafetyBoundary('/storage/emulated/0/Download/important.jpg', { forceAutomation: true }))
      .toBe('/storage/emulated/0/encv-automation/Download/important.jpg')
  })

  it('dev + forceAutomation：仍然强制改写（保护 dev 模式）', () => {
    withDev()
    const { withSafetyBoundary } = usePathResolver()
    expect(withSafetyBoundary('/storage/emulated/0/Download/foo', { forceAutomation: true }))
      .toBe('/storage/emulated/0/encv-automation/Download/foo')
  })

  it('forceAutomation：非 storage 路径放到 __misc__', () => {
    withProd()
    const { withSafetyBoundary } = usePathResolver()
    expect(withSafetyBoundary('/tmp/sandbox.bin', { forceAutomation: true }))
      .toBe('/storage/emulated/0/encv-automation/__misc__/sandbox.bin')
  })

  it('forceAutomation：已在 encv-automation 内不重复', () => {
    withProd()
    const { withSafetyBoundary } = usePathResolver()
    expect(withSafetyBoundary('/storage/emulated/0/encv-automation/x', { forceAutomation: true }))
      .toBe('/storage/emulated/0/encv-automation/x')
  })

  it('空字符串原样返回', () => {
    withProd()
    const { withSafetyBoundary } = usePathResolver()
    expect(withSafetyBoundary('')).toBe('')
    expect(withSafetyBoundary('   ')).toBe('')
  })

  it('路径含反斜杠被规范化（Windows 风格）', () => {
    withProd()
    const { withSafetyBoundary } = usePathResolver()
    expect(withSafetyBoundary('\\storage\\emulated\\0\\Download\\foo'))
      .toBe('/storage/emulated/0/encv-automation/Download/foo')
  })
})

describe('usePathResolver 基础 API', () => {
  beforeEach(() => {
    vi.stubEnv('DEV', true)
  })
  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('normalize：去前后空格、统一正斜杠、去重', () => {
    const { normalize } = usePathResolver()
    expect(normalize('  /a//b  ')).toBe('/a/b')
    expect(normalize('a/b')).toBe('/a/b')
    expect(normalize('')).toBe('')
  })

  it('isAbsolutePath', () => {
    const { isAbsolutePath } = usePathResolver()
    expect(isAbsolutePath('/abs')).toBe(true)
    expect(isAbsolutePath('rel')).toBe(false)
  })

  it('getMockPaths：dev 模式返 mock 路径数组', () => {
    vi.stubEnv('DEV', true)
    const { getMockPaths } = usePathResolver()
    expect(getMockPaths()).toEqual(['/mock/video.mp4', '/mock/doc.txt', '/mock/report.pdf', '/mock/data.csv'])
  })

  it('getMockPaths：prod 模式返 null', () => {
    vi.stubEnv('DEV', false)
    const { getMockPaths } = usePathResolver()
    expect(getMockPaths()).toBeNull()
  })
})
