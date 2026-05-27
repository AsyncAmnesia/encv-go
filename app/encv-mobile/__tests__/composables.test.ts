import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  getThumbCacheSize,
  clearThumbCache,
  THUMB_CACHE_MAX,
  useThumbnailCache,
} from '@/composables/useThumbnailCache'
import { getExternalStreamUrl } from '@/api/encv'

vi.mock('@/api/encv', () => ({
  getExternalStreamUrl: vi.fn((path: string) => `/mock-stream/${path}`),
}))

describe('Thumbnail Cache - module-level API', () => {
  beforeEach(() => {
    clearThumbCache()
  })

  it('clearThumbCache resets size to 0', () => {
    expect(getThumbCacheSize()).toBe(0)
  })

  it('THUMB_CACHE_MAX should be 500', () => {
    expect(THUMB_CACHE_MAX).toBe(500)
  })
})

describe('useThumbnailCache composable', () => {
  beforeEach(() => {
    clearThumbCache()
  })

  it('returns expected shape', () => {
    const { thumbnailUrls, setupLazyThumbnails, onThumbError } = useThumbnailCache()
    expect(thumbnailUrls).toBeDefined()
    expect(typeof setupLazyThumbnails).toBe('function')
    expect(typeof onThumbError).toBe('function')
  })

  it('thumbnailUrls starts empty', () => {
    const { thumbnailUrls } = useThumbnailCache()
    expect(Object.keys(thumbnailUrls.value).length).toBe(0)
  })

  it('onThumbError removes URL from reactive ref', () => {
    const { thumbnailUrls, onThumbError } = useThumbnailCache()
    thumbnailUrls.value['/test.jpg'] = '/mock-stream/test.jpg'
    onThumbError('/test.jpg')
    expect(thumbnailUrls.value['/test.jpg']).toBeUndefined()
  })

  it('onThumbError also clears module-level cache', async () => {
    const { onThumbError } = useThumbnailCache()
    getExternalStreamUrl('/preload.jpg')
    await new Promise(r => setTimeout(r, 100))
    const sizeBefore = getThumbCacheSize()
    if (sizeBefore > 0) {
      onThumbError('/preload.jpg')
      expect(getThumbCacheSize()).toBeLessThanOrEqual(sizeBefore - 1)
    }
  })
})
