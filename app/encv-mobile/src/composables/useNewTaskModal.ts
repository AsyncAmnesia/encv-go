import { reactive, ref } from 'vue'
import { modalController } from '@ionic/vue'
import type { TaskType } from '@/api/encv'
import type { PluginCandidate, ContainerVersionInfo, TaskField, TaskOptions } from '@/api/encv'
import { createTask } from '@/api/encv'
import { useTaskForm } from '@/composables/useTaskForm'
import { usePathResolver } from '@/composables/usePathResolver'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import { eventBus } from '@/composables/useEventBus'
import NewTaskModal from '@/components/NewTaskModal.vue'

const { normalize } = usePathResolver()

// modalController.create() 的 componentProps 是静态快照
// 必须用 reactive 对象包装所有状态，让子组件能读取到回调更新后的最新值
interface NewTaskState {
  taskType: string
  sourcePath: string
  targetPath: string
  candidates: PluginCandidate[]
  predictedPlugin: string | null
  taskOptions: TaskOptions | null
  primaryOverride: string
  secondaryPassword: string
  version: number
  versionOptions: ContainerVersionInfo[]
  extraValues: Record<string, string>
  filteredExtraFields: TaskField[]
  selectedPluginIndex: number
}

export function useNewTaskModal() {
  const { t } = useI18n()
  const {
    candidates,
    predictedPlugin,
    selectedPluginIndex,
    versionOptions,
    extraValues,
    visibleExtraFields,
    predictPlugin: doPredict,
    reset: resetTaskForm,
  } = useTaskForm()

  async function openNewTask(initialSourcePath?: string, initialTaskType?: 'encrypt' | 'decrypt') {
    const state = reactive<NewTaskState>({
      taskType: initialTaskType || 'encrypt',
      sourcePath: initialSourcePath || '',
      targetPath: '',
      candidates: [],
      predictedPlugin: null,
      taskOptions: null,
      primaryOverride: '',
      secondaryPassword: '',
      version: 4,
      versionOptions: [],
      extraValues: {},
      filteredExtraFields: [],
      selectedPluginIndex: 0,
    })

    resetTaskForm()

    function syncState() {
      state.candidates = candidates.value
      state.predictedPlugin = predictedPlugin.value
      state.selectedPluginIndex = selectedPluginIndex.value
      state.versionOptions = versionOptions.value ?? []
      state.extraValues = { ...extraValues.value }
      state.filteredExtraFields = visibleExtraFields.value
      if (candidates.value.length > 0) {
        state.taskOptions = candidates.value[selectedPluginIndex.value]?.taskOptions ?? null
      }
    }

    syncState()

    const submitting = ref(false)

    const modal = await modalController.create({
      component: NewTaskModal,
      componentProps: {
        state,
        onUpdateTaskType: (v: string) => { state.taskType = v },
        onUpdateSourcePath: async (v: string) => {
          state.sourcePath = v
          const norm = normalize(v)
          if (norm) {
            state.predictedPlugin = null
            state.candidates = []
            state.taskOptions = null
            await doPredict(norm, state.taskType as 'encrypt' | 'decrypt')
            if (state.taskType === 'decrypt' && candidates.value.length === 0 && !predictedPlugin.value) {
              state.predictedPlugin = 'auto-detect'
            }
            syncState()
          }
        },
        onUpdateTargetPath: (v: string) => { state.targetPath = v },
        onUpdateVersion: (v: number) => { state.version = v },
        onUpdatePrimaryOverride: (v: string) => { state.primaryOverride = v },
        onUpdateSecondaryPassword: (v: string) => { state.secondaryPassword = v },
        onUpdateExtraValue: ({ key, value }: { key: string; value: string }) => { state.extraValues[key] = value },
        onSelectPlugin: (idx: number) => {
          state.selectedPluginIndex = idx
          if (candidates.value.length > 0) {
            state.taskOptions = candidates.value[idx]?.taskOptions ?? null
          }
        },
        onSubmit: async () => {
          if (!state.sourcePath) return
          if (submitting.value) return
          submitting.value = true
          try {
            const pluginName = state.candidates[state.selectedPluginIndex]?.name || state.predictedPlugin || undefined
            await createTask(
              state.taskType as TaskType,
              state.sourcePath,
              state.targetPath || undefined,
              undefined,
              state.taskType === 'encrypt' ? state.version : undefined,
              pluginName
            )
            await modal.dismiss()
            showToast({ message: t('tasks.taskCreated'), duration: 1500, color: 'success' })
            eventBus.emit('task:refresh', {})
          } catch {
            showToast({ message: t('tasks.taskCreateFailed'), duration: 2000, color: 'danger' })
          } finally {
            submitting.value = false
          }
        },
      },
    })

    await modal.present()

    if (initialSourcePath) {
      const normalized = normalize(initialSourcePath)
      if (normalized) {
        await doPredict(normalized, state.taskType as 'encrypt' | 'decrypt')
        if (state.taskType === 'decrypt' && candidates.value.length === 0 && !predictedPlugin.value) {
          state.predictedPlugin = 'auto-detect'
        }
        syncState()
      }
    }
  }

  return { openNewTask }
}
