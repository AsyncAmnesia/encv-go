import { ref, computed, watch, nextTick } from 'vue'
import { predictPlugin, type TaskOptions, type TaskField, type PluginCandidate } from '@/api/encv'

export interface QueryInitParams {
  sourcePath: string
  taskType: 'encrypt' | 'decrypt'
}

export function useTaskForm() {
  const candidates = ref<PluginCandidate[]>([])
  const selectedPluginIndex = ref(0)
  const extraValues = ref<Record<string, string>>({})
  const primaryOverride = ref('')
  const secondaryPassword = ref('')

  const predictedPlugin = computed(() => {
    if (candidates.value.length === 0) return null
    return candidates.value[selectedPluginIndex.value]?.name ?? null
  })

  const taskOptions = computed<TaskOptions | null>(() => {
    if (candidates.value.length === 0) return null
    return candidates.value[selectedPluginIndex.value]?.taskOptions ?? null
  })

  watch(selectedPluginIndex, () => {
    const opts = taskOptions.value
    const defaults: Record<string, string> = {}
    opts?.extraFields?.forEach((f) => {
      if (f.defaultValue) defaults[f.key] = f.defaultValue
    })
    extraValues.value = defaults
  })

  let predictTimer: ReturnType<typeof setTimeout> | null = null

  function doPredict(sourcePath: string, taskType: 'encrypt' | 'decrypt') {
    if (predictTimer) clearTimeout(predictTimer)
    predictTimer = setTimeout(async () => {
      try {
        const result = await predictPlugin(sourcePath, taskType)
        candidates.value = result.candidates ?? []
        selectedPluginIndex.value = 0
        const defaults: Record<string, string> = {}
        candidates.value[0]?.taskOptions?.extraFields?.forEach((f) => {
          if (f.defaultValue) defaults[f.key] = f.defaultValue
        })
        extraValues.value = defaults
      } catch {
        candidates.value = []
      }
    }, 500)
  }

  function getExtraPayload(): Record<string, string> {
    const payload: Record<string, string> = {}
    for (const [k, v] of Object.entries(extraValues.value)) {
      if (v !== undefined && v !== '') payload[k] = v
    }
    return payload
  }

  const visibleExtraFields = computed<TaskField[]>(() => {
    if (!taskOptions.value?.extraFields) return []
    return taskOptions.value.extraFields
  })

  const versionOptions = computed(() => {
    if (!taskOptions.value?.supportedVersions) return undefined
    return taskOptions.value.supportedVersions.map((v) => ({
      version: v,
      status:
        v === taskOptions.value!.defaultVersion
          ? ('recommended' as const)
          : v === 2
            ? ('deprecated' as const)
            : ('stable' as const),
      label: `V${v}`,
    }))
  })

  function reset() {
    if (predictTimer) {
      clearTimeout(predictTimer)
      predictTimer = null
    }
    candidates.value = []
    selectedPluginIndex.value = 0
    extraValues.value = {}
    primaryOverride.value = ''
    secondaryPassword.value = ''
  }

  async function initFromQuery(params: QueryInitParams): Promise<void> {
    reset()
    await nextTick()
    doPredict(params.sourcePath, params.taskType)
  }

  return {
    candidates,
    selectedPluginIndex,
    predictedPlugin,
    taskOptions,
    extraValues,
    primaryOverride,
    secondaryPassword,
    visibleExtraFields,
    versionOptions,
    predictPlugin: doPredict,
    getExtraPayload,
    reset,
    initFromQuery,
  }
}
