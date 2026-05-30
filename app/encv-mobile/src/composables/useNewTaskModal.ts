import { ref } from 'vue'
import { modalController } from '@ionic/vue'
import type { TaskType } from '@/api/encv'
import { createTask } from '@/api/encv'
import { useTaskForm } from '@/composables/useTaskForm'
import { usePathResolver } from '@/composables/usePathResolver'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import { eventBus } from '@/composables/useEventBus'
import NewTaskModal from '@/components/NewTaskModal.vue'

const { normalize } = usePathResolver()

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

  const taskType = ref<TaskType>('encrypt')
  const sourcePath = ref('')
  const targetPath = ref('')
  const primaryOverride = ref('')
  const secondaryPassword = ref('')
  const version = ref(4)

  async function openNewTask(initialSourcePath?: string, initialTaskType?: 'encrypt' | 'decrypt') {
    taskType.value = initialTaskType || 'encrypt'
    sourcePath.value = initialSourcePath || ''
    targetPath.value = ''
    primaryOverride.value = ''
    secondaryPassword.value = ''
    version.value = 4
    resetTaskForm()

    if (initialSourcePath) {
      const normalized = normalize(initialSourcePath)
      if (normalized) {
        doPredict(normalized, taskType.value as 'encrypt' | 'decrypt')
        await new Promise(resolve => setTimeout(resolve, 600))
      }
    }

    const modal = await modalController.create({
      component: NewTaskModal,
      componentProps: {
        taskType: taskType.value,
        sourcePath: sourcePath.value,
        targetPath: targetPath.value,
        candidates: candidates.value,
        predictedPlugin: predictedPlugin.value,
        taskOptions: candidates.value.length > 0 ? candidates.value[selectedPluginIndex.value]?.taskOptions ?? null : null,
        primaryOverride: primaryOverride.value,
        secondaryPassword: secondaryPassword.value,
        version: version.value,
        versionOptions: versionOptions.value ?? [],
        extraValues: extraValues.value,
        filteredExtraFields: visibleExtraFields.value,
        selectedPluginIndex: selectedPluginIndex.value,
        onUpdateTaskType: (v: string) => { taskType.value = v as TaskType },
        onUpdateSourcePath: (v: string) => {
          sourcePath.value = v
          const norm = normalize(v)
          if (norm) doPredict(norm, taskType.value as 'encrypt' | 'decrypt')
        },
        onUpdateTargetPath: (v: string) => { targetPath.value = v },
        onUpdateVersion: (v: number) => { version.value = v },
        onUpdatePrimaryOverride: (v: string) => { primaryOverride.value = v },
        onUpdateSecondaryPassword: (v: string) => { secondaryPassword.value = v },
        onUpdateExtraValue: ({ key, value }: { key: string; value: string }) => { extraValues.value[key] = value },
        onSelectPlugin: (idx: number) => { selectedPluginIndex.value = idx },
        onSubmit: async () => {
          if (!sourcePath.value) return
          try {
            await createTask(
              taskType.value,
              sourcePath.value,
              targetPath.value || undefined,
              undefined,
              taskType.value === 'encrypt' ? version.value : undefined
            )
            await modal.dismiss()
            showToast({ message: t('tasks.taskCreated'), duration: 1500, color: 'success' })
            eventBus.emit('task:refresh', {})
          } catch {
            showToast({ message: t('tasks.taskCreateFailed'), duration: 2000, color: 'danger' })
          }
        },
      },
    })

    await modal.present()
  }

  return { openNewTask }
}
