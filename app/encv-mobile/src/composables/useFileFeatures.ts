import { computed, shallowRef } from 'vue'
import type { FileItem } from '@/api/encv'
import type { FileFeature, FileBadge, FileSubtitle, FileAction } from '@/types/file-feature'

const registry = new Map<string, FileFeature>()
const version = shallowRef(0)

export function registerFileFeature(feature: FileFeature): void {
  if (registry.has(feature.id)) {
    console.warn(`[useFileFeatures] Feature "${feature.id}" already registered, skipping`)
    return
  }
  registry.set(feature.id, feature)
  feature.onActivate?.()
  version.value++
}

export function unregisterFileFeature(id: string): void {
  const feature = registry.get(id)
  if (feature) {
    feature.onDeactivate?.()
    registry.delete(id)
    version.value++
  }
}

function resolve<T>(value: T | Promise<T>): Promise<T> {
  return Promise.resolve(value)
}

async function collectBadges(file: FileItem): Promise<FileBadge[]> {
  const results: FileBadge[] = []
  const entries = Array.from(registry.entries())
  for (const [, feature] of entries) {
    if (!feature.isActive(file) || !feature.getBadge) continue
    try {
      const badge = await resolve(feature.getBadge(file))
      if (badge != null) results.push(badge)
    } catch (e) {
      console.warn(`[useFileFeatures] getBadge failed for ${feature.id}:`, e)
    }
  }
  return results
}

async function collectSubtitles(file: FileItem): Promise<FileSubtitle[]> {
  const results: FileSubtitle[] = []
  for (const [, feature] of registry) {
    if (!feature.isActive(file) || !feature.getSubtitle) continue
    try {
      const sub = await resolve(feature.getSubtitle(file))
      if (sub != null) results.push(sub)
    } catch (e) {
      console.warn(`[useFileFeatures] getSubtitle failed for ${feature.id}:`, e)
    }
  }
  return results
}

async function collectActions(file: FileItem): Promise<FileAction[]> {
  const results: FileAction[] = []
  for (const [, feature] of registry) {
    if (!feature.isActive(file) || !feature.getFileActions) continue
    try {
      const actions = await resolve(feature.getFileActions(file))
      for (const action of actions) {
        if (action.visible?.(file) !== false) {
          results.push(action)
        }
      }
    } catch (e) {
      console.warn(`[useFileFeatures] getFileActions failed for ${feature.id}:`, e)
    }
  }
  return results
}

export function useFileFeatures() {
  const allFeatures = computed(() => Array.from(registry.values()))

  async function getBadges(file: FileItem): Promise<FileBadge[]> {
    return collectBadges(file)
  }

  async function getSubtitles(file: FileItem): Promise<FileSubtitle[]> {
    return collectSubtitles(file)
  }

  async function getAllActions(file: FileItem): Promise<FileAction[]> {
    return collectActions(file)
  }

  return {
    allFeatures,
    getBadges,
    getSubtitles,
    getAllActions,
    version,
  }
}
