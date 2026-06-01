import { ref, computed } from 'vue'
import { fetchConfig, updateConfig } from '@/api/encv'
import { parseSchema, getDefaultValue } from '@/config/schemaParser'
import type { FieldDef } from '@/config/schemaParser'

const config = ref<Record<string, unknown>>({})
const loading = ref(false)
const dirty = ref(false)
const originalConfig = ref<Record<string, unknown>>({})
const restartNeeded = ref(false)

const schemaFields = computed<FieldDef[]>(() => parseSchema())

function deepClone<T>(obj: T): T {
  return JSON.parse(JSON.stringify(obj))
}

function buildInitialConfig(): Record<string, unknown> {
  const initial: Record<string, unknown> = {}
  for (const field of schemaFields.value) {
    initial[field.key] = getDefaultValue(field)
  }
  return initial
}

async function loadConfig() {
  loading.value = true
  try {
    const data = await fetchConfig()
    config.value = data
    originalConfig.value = deepClone(data)
    dirty.value = false
  } catch (error) {
    console.error('[ENCV] Failed to load config:', error)
    config.value = buildInitialConfig()
    originalConfig.value = deepClone(config.value)
    dirty.value = false
  }
  loading.value = false
}

async function saveConfig() {
  loading.value = true
  try {
    const result = await updateConfig(config.value)
    originalConfig.value = deepClone(config.value)
    dirty.value = false
    if (result.needsRestart) {
      restartNeeded.value = true
    }
  } catch (error) {
    console.error('[ENCV] Failed to save config:', error)
    throw error
  } finally {
    loading.value = false
  }
}

function resetConfig() {
  config.value = deepClone(originalConfig.value)
  dirty.value = false
}

function getFieldValue(path: string[]): unknown {
  let current: unknown = config.value
  for (const key of path) {
    if (current && typeof current === 'object' && current !== null) {
      current = (current as Record<string, unknown>)[key]
    } else {
      return undefined
    }
  }
  return current
}

function setFieldValue(path: string[], value: unknown) {
  if (path.length === 0) return

  let current: Record<string, unknown> = config.value
  for (let i = 0; i < path.length - 1; i++) {
    const key = path[i]
    const child = current[key]
    if (!child || typeof child !== 'object') {
      const fresh: Record<string, unknown> = {}
      current[key] = fresh
      current = fresh
    } else {
      current = child as Record<string, unknown>
    }
  }

  current[path[path.length - 1]] = value
  dirty.value = true
}

export function useConfig() {
  return {
    config,
    schemaFields,
    loading,
    dirty,
    restartNeeded,
    loadConfig,
    saveConfig,
    resetConfig,
    getFieldValue,
    setFieldValue,
  }
}

export { getFieldValue, setFieldValue }
