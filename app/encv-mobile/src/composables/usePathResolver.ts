import type { FileItem } from '@/api/encv'

const MOCK_PATHS = [
  '/mock/video.mp4',
  '/mock/doc.txt',
  '/mock/report.pdf',
  '/mock/data.csv',
] as const

export function usePathResolver() {
  function normalize(rawPath: string): string {
    const trimmed = rawPath.trim()
    if (!trimmed) return ''
    const slashesReplaced = trimmed.replace(/\\/g, '/')
    const deduped = slashesReplaced.replace(/\/+/g, '/')
    if (!deduped.startsWith('/')) {
      return '/' + deduped
    }
    return deduped
  }

  function resolveFileItem(file: FileItem): string {
    if (!file?.path) return ''
    return normalize(file.path)
  }

  function isAbsolutePath(path: string): boolean {
    return path.startsWith('/')
  }

  function getMockPaths(): string[] | null {
    if (import.meta.env.DEV) {
      return [...MOCK_PATHS]
    }
    return null
  }

  return { normalize, resolveFileItem, isAbsolutePath, getMockPaths }
}
