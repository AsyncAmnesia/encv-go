import { ref, computed } from 'vue'
import { predictPlugin, type TaskOptions, type TaskField } from '@/api/encv'

export function useTaskForm() {
  const predictedPlugin = ref<string | null>(null)
  const taskOptions = ref<TaskOptions | null>(null)
  const extraValues = ref<Record<string, string>>({})
  const secondaryPassword = ref('')

  let predictTimer: ReturnType<typeof setTimeout> | null = null

  function doPredict(sourcePath: string, taskType: 'encrypt' | 'decrypt') {
    if (predictTimer) clearTimeout(predictTimer)
    predictTimer = setTimeout(async () => {
      try {
        const result = await predictPlugin(sourcePath, taskType)
        predictedPlugin.value = result.pluginName
        taskOptions.value = result.taskOptions
        const defaults: Record<string, string> = {}
        result.taskOptions?.extraFields?.forEach((f) => {
          if (f.defaultValue) defaults[f.key] = f.defaultValue
        })
        extraValues.value = defaults
      } catch {
        predictedPlugin.value = null
        taskOptions.value = null
      }
    }, 500)
  }

  function getExtraPayload(): Record<string, unknown> {
    const payload: Record<string, unknown> = {}
    for (const [k, v] of Object.entries(extraValues.value)) {
      if (v !== undefined && v !== '') payload[k] = v
    }
    if (secondaryPassword.value) payload['secondary_password'] = secondaryPassword.value
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
    predictedPlugin.value = null
    taskOptions.value = null
    extraValues.value = {}
    secondaryPassword.value = ''
  }

  return {
    predictedPlugin,
    taskOptions,
    extraValues,
    secondaryPassword,
    visibleExtraFields,
    versionOptions,
    predictPlugin: doPredict,
    getExtraPayload,
    reset,
  }
}
