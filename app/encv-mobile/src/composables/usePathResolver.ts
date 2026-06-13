import type { FileItem } from '@/api/encv'

const MOCK_PATHS = [
  '/mock/video.mp4',
  '/mock/doc.txt',
  '/mock/report.pdf',
  '/mock/data.csv',
] as const

/**
 * 真机安全边界常量
 *
 * 当自动化测试入口在 release 构建（真机）上运行时，强制把 source/target 路径
 * 从 /storage/emulated/0/<user-path> 改写到 /storage/emulated/0/encv-automation/<user-path>，
 * 避免自动化测试损坏用户真实数据。
 */
const REAL_STORAGE_ROOT = '/storage/emulated/0'
const SAFETY_NAMESPACE = 'encv-automation'

/**
 * 测试环境 / dev 模式下把 isNative/withSafetyBoundary 的真实行为替换为 no-op。
 * - DEV 模式：保持原路径（vite 走 /mock/* 路径或本地磁盘）
 * - 真机 release：拦截 /storage/emulated/0/* 改写到 encv-automation 命名空间
 *
 * `forceAutomation: true` 用于"无论路径在哪里都强制改写"的场景，
 * 适用于自动化测试入口（即使开发者在 dev 设置了 /tmp/real.txt 也改写）。
 */
export interface WithSafetyBoundaryOptions {
  forceAutomation?: boolean
}

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

  /**
   * 真机安全边界：
   * - dev 模式：no-op（原样返回）
   * - 真机 release：
   *   - 如果路径以 /storage/emulated/0/ 开头且不在 encv-automation 命名空间内，
   *     自动改写到 /storage/emulated/0/encv-automation/<原路径>
   *   - 如果路径已经在 encv-automation 命名空间内，原样返回（避免 /encv-automation/cv-automation/...）
   *   - 其他路径（/mock, /tmp, /data, ...）原样返回
   * - `forceAutomation: true` 把所有绝对路径强制改写到 encv-automation 命名空间
   *   （用于自动化测试入口，确保即使 dev 给到非 storage 路径也安全）
   */
  function withSafetyBoundary(rawPath: string, opts?: WithSafetyBoundaryOptions): string {
    const normalized = normalize(rawPath)
    if (!normalized) return ''

    // dev 模式不强制改写（vite 已经走 mock 路径）
    if (import.meta.env.DEV && !opts?.forceAutomation) return normalized

    const insideSafety =
      normalized === `${REAL_STORAGE_ROOT}/${SAFETY_NAMESPACE}` ||
      normalized.startsWith(`${REAL_STORAGE_ROOT}/${SAFETY_NAMESPACE}/`)

    // forceAutomation：把任何 /storage/emulated/0 下的路径改写到命名空间
    // 同时处理 /tmp/ /data/ 之类的非 storage 路径，自动化测试永远在 encv-automation 下
    if (opts?.forceAutomation) {
      if (insideSafety) return normalized
      if (normalized.startsWith(REAL_STORAGE_ROOT + '/')) {
        const rel = normalized.slice(REAL_STORAGE_ROOT.length)
        return `${REAL_STORAGE_ROOT}/${SAFETY_NAMESPACE}${rel}`
      }
      // 非 storage 路径：把 basename 放到 encv-automation 下
      const basename = normalized.replace(/\/+$/, '').split('/').pop() || 'unnamed'
      return `${REAL_STORAGE_ROOT}/${SAFETY_NAMESPACE}/__misc__/${basename}`
    }

    // 普通调用：只在路径以 /storage/emulated/0/ 开头（且不在命名空间内）时改写
    if (normalized.startsWith(REAL_STORAGE_ROOT + '/') && !insideSafety) {
      const rel = normalized.slice(REAL_STORAGE_ROOT.length)
      return `${REAL_STORAGE_ROOT}/${SAFETY_NAMESPACE}${rel}`
    }

    return normalized
  }

  return {
    normalize,
    resolveFileItem,
    isAbsolutePath,
    getMockPaths,
    withSafetyBoundary,
  }
}
