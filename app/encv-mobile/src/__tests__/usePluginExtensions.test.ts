import { describe, it, expect, vi, beforeEach } from 'vitest'
import { usePluginExtensions } from '@/composables/usePluginExtensions'
import { fetchContainerExtensions, type ContainerExtensionsResponse } from '@/api/encv'

vi.mock('@/api/encv', () => ({
  fetchContainerExtensions: vi.fn(),
}))

const mockedFetch = vi.mocked(fetchContainerExtensions)

function setupMockData(overrides?: Partial<ContainerExtensionsResponse>): ContainerExtensionsResponse {
  return {
    extensions: {
      '.sccgv': 'video',
      '.sccga': 'audio',
      '.sccgi': 'image',
      '.sccgt': 'text',
      '.myenc': 'alist_encrypt',
    },
    conflicts: [],
    ...overrides,
  }
}

describe('usePluginExtensions', () => {
  beforeEach(() => {
    const { invalidate } = usePluginExtensions()
    invalidate()
    vi.clearAllMocks()
  })

  describe('getConflictingPlugins', () => {
    it('无冲突后缀应返回空数组', async () => {
      const mockData = setupMockData()
      mockedFetch.mockResolvedValueOnce(mockData)

      const { load, getConflictingPlugins } = usePluginExtensions()
      await load()

      const result = getConflictingPlugins('.unknown')
      expect(result).toEqual([])
    })

    it('与 video 冲突时应返回 ["video"]', async () => {
      const mockData = setupMockData()
      mockedFetch.mockResolvedValueOnce(mockData)

      const { load, getConflictingPlugins } = usePluginExtensions()
      await load()

      const result = getConflictingPlugins('.sccgv')
      expect(result).toEqual(['video'])
    })

    it('传入无前缀点号的 suffix 应自动规范化后再检测', async () => {
      const mockData = setupMockData()
      mockedFetch.mockResolvedValueOnce(mockData)

      const { load, getConflictingPlugins } = usePluginExtensions()
      await load()

      const result = getConflictingPlugins('sccgv')
      expect(result).toEqual(['video'])
    })

    it('传入空值或仅点号应返回空数组（空值安全）', async () => {
      const mockData = setupMockData()
      mockedFetch.mockResolvedValueOnce(mockData)

      const { load, getConflictingPlugins } = usePluginExtensions()
      await load()

      expect(getConflictingPlugins('')).toEqual([])
      expect(getConflictingPlugins('.')).toEqual([])
    })
  })

  describe('加密容器后缀名冲突检测（Alist-Encrypt 场景）', () => {
    it('Alist-Encrypt 使用 .sccgv 应检测到与 video 插件冲突', async () => {
      const mockData = setupMockData({
        conflicts: [
          { extension: '.sccgv', pluginNames: ['video'] },
        ],
      })
      mockedFetch.mockResolvedValueOnce(mockData)

      const { load, getConflictingPlugins } = usePluginExtensions()
      await load()

      // 模拟用户输入 alist_encrypt.suffix = ".sccgv"
      const result = getConflictingPlugins('.sccgv')
      expect(result).toContain('video')
      expect(result.length).toBeGreaterThanOrEqual(1)
    })

    it('Alist-Encrypt 使用 .sccga 应检测到与 audio 插件冲突', async () => {
      const mockData = setupMockData({
        conflicts: [
          { extension: '.sccga', pluginNames: ['audio'] },
        ],
      })
      mockedFetch.mockResolvedValueOnce(mockData)

      const { load, getConflictingPlugins } = usePluginExtensions()
      await load()

      const result = getConflictingPlugins('.sccga')
      expect(result).toContain('audio')
    })

    it('Alist-Encrypt 使用 .sccgi 应检测到与 image 插件冲突', async () => {
      const mockData = setupMockData({
        conflicts: [
          { extension: '.sccgi', pluginNames: ['image'] },
        ],
      })
      mockedFetch.mockResolvedValueOnce(mockData)

      const { load, getConflictingPlugins } = usePluginExtensions()
      await load()

      const result = getConflictingPlugins('.sccgi')
      expect(result).toContain('image')
    })

    it('Alist-Encrypt 使用 .sccgt 应检测到与 text 插件冲突', async () => {
      const mockData = setupMockData({
        conflicts: [
          { extension: '.sccgt', pluginNames: ['text'] },
        ],
      })
      mockedFetch.mockResolvedValueOnce(mockData)

      const { load, getConflictingPlugins } = usePluginExtensions()
      await load()

      const result = getConflictingPlugins('.sccgt')
      expect(result).toContain('text')
    })

    it('Alist-Encrypt 使用唯一后缀 .myenc 不应触发冲突', async () => {
      const mockData = setupMockData({
        extensions: {
          '.sccgv': 'video',
          '.sccga': 'audio',
          '.sccgi': 'image',
          '.sccgt': 'text',
        },
        conflicts: [],
      })
      mockedFetch.mockResolvedValueOnce(mockData)

      const { load, getConflictingPlugins } = usePluginExtensions()
      await load()

      // .myenc 不在任何已注册插件的容器扩展名中
      const result = getConflictingPlugins('.myenc')
      expect(result).toEqual([])
    })

    it('大小写不敏感：SCCGV 应与 .sccgv 视为相同后缀', async () => {
      const mockData = setupMockData({
        conflicts: [
          { extension: '.sccgv', pluginNames: ['video'] },
        ],
      })
      mockedFetch.mockResolvedValueOnce(mockData)

      const { load, getConflictingPlugins } = usePluginExtensions()
      await load()

      // 大写输入应同样检测到冲突
      expect(getConflictingPlugins('SCCGV')).toEqual(['video'])
      expect(getConflictingPlugins('SccGv')).toEqual(['video'])
    })

    it('多插件同时声明同一扩展名时冲突列表应包含所有插件', async () => {
      const mockData = setupMockData({
        extensions: {
          '.sccgv': 'video',
          '.sccga': 'audio',
        },
        conflicts: [
          { extension: '.custom', pluginNames: ['video', 'audio'] },
        ],
      })
      mockedFetch.mockResolvedValueOnce(mockData)

      const { load, getConflictingPlugins } = usePluginExtensions()
      await load()

      const result = getConflictingPlugins('.custom')
      expect(result).toContain('video')
      expect(result).toContain('audio')
      expect(result.length).toBe(2)
    })
  })

  describe('数据加载与缓存', () => {
    it('load() 后 extensions 数据应可用', async () => {
      const mockData = setupMockData()
      mockedFetch.mockResolvedValueOnce(mockData)

      const { load, getExtensions } = usePluginExtensions()
      await load()

      const extMap = getExtensions()
      expect(extMap).toBeDefined()
      expect(Object.keys(extMap!).length).toBeGreaterThanOrEqual(5)
    })

    it('invalidate() 后应重新加载数据', async () => {
      const firstData = setupMockData({ extensions: { '.old': 'plugin1' } })
      const secondData = setupMockData({ extensions: { '.new': 'plugin2' } })

      mockedFetch.mockResolvedValueOnce(firstData)
      const { load, getExtensions, invalidate } = usePluginExtensions()
      await load()
      expect(getExtensions()).toEqual(expect.objectContaining({ '.old': 'plugin1' }))

      invalidate()
      vi.clearAllMocks()

      mockedFetch.mockResolvedValueOnce(secondData)
      await load()
      expect(getExtensions()).toHaveProperty('.new')
    })

    it('API 未加载时（data=null）应使用 fallback 检测已知冲突', () => {
      const { getConflictingPlugins } = usePluginExtensions()

      // 不调用 load()，模拟 API 不可用场景
      // fallback 包含 .sccgv -> video
      expect(getConflictingPlugins('.sccgv')).toEqual(['video'])
      expect(getConflictingPlugins('.sccga')).toEqual(['audio'])
      expect(getConflictingPlugins('.sccgi')).toEqual(['image'])
      expect(getConflictingPlugins('.sccgt')).toEqual(['text'])

      // 不在 fallback 中的后缀返回空
      expect(getConflictingPlugins('.myenc')).toEqual([])
      expect(getConflictingPlugins('.unknown')).toEqual([])
    })
  })
})
