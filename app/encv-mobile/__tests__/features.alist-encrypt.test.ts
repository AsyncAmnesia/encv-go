import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

vi.mock('@/api/encv', () => ({
  decodeAlistFilename: vi.fn().mockResolvedValue({ plain_name: '', success: false }),
  getAlistEncryptStreamUrl: vi.fn((params: any) =>
    `/api/alist-encrypt/stream?path=${encodeURIComponent(params.path)}&password=${encodeURIComponent(params.password)}`,
  ),
}))

vi.mock('@/composables/useI18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/router', () => ({
  default: { push: vi.fn() },
}))

vi.mock('@ionic/vue', () => ({
  alertController: {
    create: vi.fn().mockReturnValue({
      then: (cb: Function) => cb({ present: vi.fn() }),
    }),
  },
}))

const mockOpenNewTask = vi.fn()

vi.mock('@/composables/useNewTaskModal', () => ({
  useNewTaskModal: () => ({
    openNewTask: mockOpenNewTask,
  }),
}))

import { isAlistEncrypted, getSessionPassword, setSessionPassword, clearPasswordCache, getDecodedName, loadDecodedName, getStreamUrl, clearDecodeCache } from '@/features/alist-encrypt/useAlistEncrypt'
import { getAlistBadge } from '@/features/alist-encrypt/badge'
import { getAlistActions } from '@/features/alist-encrypt/actions'
import { createAlistEncryptFeature } from '@/features/alist-encrypt/index'
import { useNewTaskModal } from '@/composables/useNewTaskModal'
import { decodeAlistFilename } from '@/api/encv'
import type { FileItem } from '@/api/encv'

const aeFile: FileItem = { name: 'video.bin', path: '/media/video.bin', isDirectory: false }
const normalFile: FileItem = { name: 'doc.pdf', path: '/docs/doc.pdf', isDirectory: false }
const dirFile: FileItem = { name: 'folder', path: '/folder', isDirectory: true }
const encryptedFile: FileItem = { name: 'secret.bin', path: '/secret.bin', isDirectory: false, isEncrypted: true }
const upperBinFile: FileItem = { name: 'video.BIN', path: '/video.BIN', isDirectory: false }

describe('isAlistEncrypted', () => {
  it('.bin file + not directory + not encrypted → true', () => {
    expect(isAlistEncrypted(aeFile)).toBe(true)
  })

  it('directory → false', () => {
    expect(isAlistEncrypted(dirFile)).toBe(false)
  })

  it('isEncrypted=true → false (already handled by other container)', () => {
    expect(isAlistEncrypted(encryptedFile)).toBe(false)
  })

  it('.mp4 file → false', () => {
    expect(isAlistEncrypted(normalFile)).toBe(false)
  })

  it('.BIN uppercase → false (endsWith is case-sensitive)', () => {
    expect(isAlistEncrypted(upperBinFile)).toBe(false)
  })
})

describe('LRU Password Cache', () => {
  beforeEach(() => {
    clearPasswordCache()
  })

  it('setSessionPassword + getSessionPassword round-trip', () => {
    setSessionPassword('/a.bin', 'pass123')
    expect(getSessionPassword('/a.bin')).toBe('pass123')
  })

  it('unset path returns undefined', () => {
    expect(getSessionPassword('/nonexistent')).toBeUndefined()
  })

  it('clearPasswordCache removes everything', () => {
    setSessionPassword('/a.bin', 'p1')
    setSessionPassword('/b.bin', 'p2')
    clearPasswordCache()
    expect(getSessionPassword('/a.bin')).toBeUndefined()
    expect(getSessionPassword('/b.bin')).toBeUndefined()
  })
})

describe('Filename Decode Cache', () => {
  let fetchSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    clearDecodeCache()
    fetchSpy = vi.spyOn(globalThis, 'fetch')
  })

  afterEach(() => {
    fetchSpy.mockRestore()
  })

  it('loadDecodedName calls API and caches result', async () => {
    vi.mocked(decodeAlistFilename).mockResolvedValueOnce({ plain_name: 'movie.mp4', success: true })

    const result = await loadDecodedName(aeFile, 'mypass')
    expect(result).toBe('movie.mp4')
    expect(getDecodedName(aeFile.path)).toBe('movie.mp4')
    expect(vi.mocked(decodeAlistFilename)).toHaveBeenCalledTimes(1)
  })

  it('cache hit skips API call', async () => {
    vi.mocked(decodeAlistFilename).mockResolvedValueOnce({ plain_name: 'cached.mp4', success: true })

    await loadDecodedName(aeFile, 'pass')
    fetchSpy.mockClear()

    const cached = await loadDecodedName(aeFile, 'pass')
    expect(cached).toBe('cached.mp4')
    expect(fetchSpy).not.toHaveBeenCalled()
  })

  it('non-AE file returns null', async () => {
    const result = await loadDecodedName(normalFile, 'pass')
    expect(result).toBeNull()
  })

  it('success=false returns null', async () => {
    fetchSpy.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ plain_name: '', success: false }),
    })

    const result = await loadDecodedName(aeFile, 'wrongpass')
    expect(result).toBeNull()
  })

  it('API error returns null silently', async () => {
    fetchSpy.mockRejectedValue(new Error('network error'))

    const result = await loadDecodedName(aeFile, 'pass')
    expect(result).toBeNull()
  })
})

describe('getAlistBadge', () => {
  it('AE file returns danger badge', () => {
    const badge = getAlistBadge(aeFile)
    expect(badge).toEqual({ text: 'AE', color: 'danger' })
  })

  it('non-AE file returns null', () => {
    expect(getAlistBadge(normalFile)).toBeNull()
  })
})

describe('getAlistActions', () => {
  it('AE file returns 2 actions', () => {
    const actions = getAlistActions(aeFile)
    expect(actions).toHaveLength(2)
    expect(actions.map((a) => a.id)).toEqual(['alist-stream-preview', 'alist-decrypt'])
  })

  it('non-AE file returns encrypt action (isActive expanded)', () => {
    const actions = getAlistActions(normalFile)
    expect(actions).toHaveLength(1)
    expect(actions[0].id).toBe('alist-encrypt')
  })

  it('actions have correct structure', () => {
    const actions = getAlistActions(aeFile)
    const preview = actions.find((a) => a.id === 'alist-stream-preview')!
    expect(preview.color).toBe('primary')
    expect(typeof preview.text).toBe('function')
    expect(preview.text()).toBe('alistEncrypt.streamPreview')
  })
})

describe('createAlistEncryptFeature factory', () => {
  it('returns complete FileFeature shape', () => {
    const feat = createAlistEncryptFeature()
    expect(feat.id).toBe('alist-encrypt')
    expect(typeof feat.isActive).toBe('function')
    expect(typeof feat.getBadge).toBe('function')
    expect(typeof feat.getSubtitle).toBe('function')
    expect(typeof feat.getFileActions).toBe('function')
    expect(typeof feat.onActivate).toBe('function')
    expect(typeof feat.onDeactivate).toBe('function')
  })

  it('isActive scope expanded: AE file true, normal file true, dir false', () => {
    const feat = createAlistEncryptFeature()
    expect(feat.isActive(aeFile)).toBe(true)
    expect(feat.isActive(normalFile)).toBe(true)
    expect(feat.isActive(dirFile)).toBe(false)
  })

  it('onDeactivate clears caches', () => {
    setSessionPassword('/x.bin', 'p')
    const feat = createAlistEncryptFeature()
    feat.onDeactivate?.()
    expect(getSessionPassword('/x.bin')).toBeUndefined()
  })
})

describe('getStreamUrl', () => {
  it('returns URL with encoded path and password', () => {
    const url = getStreamUrl(aeFile, 'secret')
    expect(url).toContain('/api/alist-encrypt/stream')
    expect(url).toContain('path=')
    expect(url).toContain('password=secret')
  })
})

describe('isActive (expanded scope)', () => {
  it('非目录普通文件返回 true', () => {
    const feat = createAlistEncryptFeature()
    expect(feat.isActive(normalFile)).toBe(true)
  })

  it('.bin 加密文件返回 true', () => {
    const feat = createAlistEncryptFeature()
    expect(feat.isActive(aeFile)).toBe(true)
  })

  it('目录文件返回 false', () => {
    const feat = createAlistEncryptFeature()
    expect(feat.isActive(dirFile)).toBe(false)
  })
})

describe('getAlistActions - encrypt action for normal files', () => {
  it('普通文件返回 1 个 encrypt action', () => {
    const actions = getAlistActions(normalFile)
    expect(actions).toHaveLength(1)
    expect(actions[0].id).toBe('alist-encrypt')
  })

  it('encrypt action 有正确的属性', () => {
    const actions = getAlistActions(normalFile)
    const encrypt = actions.find((a) => a.id === 'alist-encrypt')!
    expect(encrypt.color).toBe('warning')
    expect(encrypt.text()).toBe('alistEncrypt.encrypt')
  })

  it('encrypt action handler 调用 openNewTask with encrypt type', async () => {
    const actions = getAlistActions(normalFile)
    const encrypt = actions.find((a) => a.id === 'alist-encrypt')!
    await encrypt.handler(normalFile)
    expect(mockOpenNewTask).toHaveBeenCalledWith(normalFile.path, 'encrypt')
  })
})
