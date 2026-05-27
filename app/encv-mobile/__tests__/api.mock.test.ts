import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  listFiles,
  searchFiles,
  fetchPlugins,
  fetchTags,
  getExternalStreamUrl,
  PermissionDeniedError,
  NotFoundError,
} from '@/api/encv'

const originalFetch = globalThis.fetch

function mockFetch(responseData: any, status = 200, ok = true) {
  return vi.fn().mockResolvedValue({
    ok,
    status,
    json: () => Promise.resolve(responseData),
  } as Response)
}

describe('API Mock Tests', () => {
  let fetchSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    fetchSpy = vi.spyOn(globalThis, 'fetch')
  })

  afterEach(() => {
    fetchSpy.mockRestore()
  })

  describe('listFiles', () => {
    it('returns FileItem[] on success', async () => {
      const mockFiles = [{ name: 'a.txt', path: '/a.txt', isDirectory: false }]
      fetchSpy.mockImplementation(mockFetch({ files: mockFiles }))

      const result = await listFiles('/')
      expect(result).toEqual(mockFiles)
      expect(fetchSpy).toHaveBeenCalledTimes(1)
    })

    it('throws PermissionDeniedError on 403 with PERMISSION_DENIED code', async () => {
      fetchSpy.mockImplementation(
        mockFetch({ code: 'PERMISSION_DENIED', error: 'Forbidden' }, 403, false)
      )

      await expect(listFiles('/')).rejects.toThrow(PermissionDeniedError)
    })

    it('throws NotFoundError on 404', async () => {
      fetchSpy.mockImplementation(
        mockFetch({ code: 'NOT_FOUND', error: 'Not found' }, 404, false)
      )

      await expect(listFiles('/missing')).rejects.toThrow(NotFoundError)
    })

    it('throws generic Error on other HTTP errors', async () => {
      fetchSpy.mockImplementation(mockFetch({}, 500, false))

      await expect(listFiles('/')).rejects.toThrow('HTTP error! status: 500')
    })

    it('returns empty array when files field is missing', async () => {
      fetchSpy.mockImplementation(mockFetch({}))

      const result = await listFiles('/')
      expect(result).toEqual([])
    })
  })

  describe('searchFiles', () => {
    it('returns search results', async () => {
      const mockResults = [{ name: 'findme.mp4', path: '/findme.mp4', isDirectory: false }]
      fetchSpy.mockImplementation(mockFetch({ files: mockResults }))

      const results = await searchFiles('/', 'findme', false)
      expect(results).toEqual(mockResults)
    })
  })

  describe('fetchPlugins', () => {
    it('returns PluginMeta array', async () => {
      const plugins = [{ name: 'video', supportedExtensions: ['mp4'], containerExtension: '' }]
      fetchSpy.mockImplementation(mockFetch({ plugins }))

      const result = await fetchPlugins()
      expect(result).toEqual(plugins)
    })
  })

  describe('fetchTags', () => {
    it('returns TagInfo array', async () => {
      const tags = [{ name: 'fav', count: 5 }]
      fetchSpy.mockImplementation(mockFetch({ tags }))

      const result = await fetchTags()
      expect(result).toEqual(tags)
    })
  })

  describe('getExternalStreamUrl', () => {
    it('uses DEV format when import.meta.env.DEV is true', () => {
      const url = getExternalStreamUrl('/media/video.mp4')
      expect(url).toContain('/api/stream/external')
      expect(url).toContain('video.mp4')
    })
  })
})
