import { ref, computed } from 'vue'
import {
  timer,
  sync,
  checkmarkCircle,
  closeCircle,
} from 'ionicons/icons'
import {
  getTasks,
  cancelTask,
  retryTask,
  removeTask,
  isWrongPasswordError,
} from '@/api/encv'
import type { EncvTask, TaskType, TaskStatus } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'
import { formatDuration } from '@/composables/useDateFormat'
import { showToast } from '@/composables/useToast'

export type SortBy = 'activity' | 'created'

export function useTasksList() {
  const { t } = useI18n()

  const tasks = ref<EncvTask[]>([])
  const loading = ref(false)
  const expandedWarningDetail = ref<string | null>(null)
  const sortBy = ref<SortBy>('activity')

  const showSearch = ref(false)
  const searchQuery = ref('')
  const showFilters = ref(false)
  const filterPlugins = ref<string[]>([])
  const filterTypes = ref<TaskType[]>([])
  const filterStatuses = ref<TaskStatus[]>([])

  const statusOptions: TaskStatus[] = ['queued', 'running', 'completed', 'failed', 'cancelled']

  const pluginPopoverOpen = ref(false)
  const typePopoverOpen = ref(false)
  const statusPopoverOpen = ref(false)
  const pluginPopoverEvent = ref<Event | null>(null)
  const typePopoverEvent = ref<Event | null>(null)
  const statusPopoverEvent = ref<Event | null>(null)

  const availablePlugins = computed(() => {
    const plugins = new Set<string>()
    for (const task of tasks.value) {
      if (task.pluginName) plugins.add(task.pluginName)
    }
    return Array.from(plugins).sort()
  })

  const hasActiveFilters = computed(() =>
    filterPlugins.value.length > 0 || filterTypes.value.length > 0 || filterStatuses.value.length > 0
  )

  const hasCompletedTasks = computed(() =>
    tasks.value.some(
      (task) => task.status === 'completed' || task.status === 'failed' || task.status === 'cancelled'
    )
  )

  const sortedTasks = computed(() => {
    const arr = [...tasks.value]
    if (sortBy.value === 'activity') {
      arr.sort((a, b) => {
        const timeA = a.completedAt
          ? new Date(a.completedAt).getTime()
          : new Date(a.createdAt).getTime()
        const timeB = b.completedAt
          ? new Date(b.completedAt).getTime()
          : new Date(b.createdAt).getTime()
        if (timeB !== timeA) return timeB - timeA
        return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
      })
    } else {
      arr.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())
    }
    return arr
  })

  const filteredTasks = computed(() => {
    let result = sortedTasks.value

    if (searchQuery.value.trim()) {
      const q = searchQuery.value.trim().toLowerCase()
      result = result.filter((task) => {
        const name = getTaskName(task).toLowerCase()
        const plugin = (task.pluginName || '').toLowerCase()
        const error = (task.error || '').toLowerCase()
        const id = task.id.toLowerCase()
        return name.includes(q) || plugin.includes(q) || error.includes(q) || id.includes(q)
      })
    }

    if (filterPlugins.value.length > 0) {
      result = result.filter((task) => task.pluginName && filterPlugins.value.includes(task.pluginName))
    }

    if (filterTypes.value.length > 0) {
      result = result.filter((task) => filterTypes.value.includes(task.type))
    }

    if (filterStatuses.value.length > 0) {
      result = result.filter((task) => filterStatuses.value.includes(task.status))
    }

    return result
  })

  function openPluginPopover(event: Event) {
    pluginPopoverEvent.value = event
    pluginPopoverOpen.value = true
  }

  function openTypePopover(event: Event) {
    typePopoverEvent.value = event
    typePopoverOpen.value = true
  }

  function openStatusPopover(event: Event) {
    statusPopoverEvent.value = event
    statusPopoverOpen.value = true
  }

  function togglePluginFilter(plugin: string) {
    const idx = filterPlugins.value.indexOf(plugin)
    if (idx === -1) filterPlugins.value.push(plugin)
    else filterPlugins.value.splice(idx, 1)
  }

  function toggleTypeFilter(type: TaskType) {
    const idx = filterTypes.value.indexOf(type)
    if (idx === -1) filterTypes.value.push(type)
    else filterTypes.value.splice(idx, 1)
  }

  function toggleStatusFilter(status: TaskStatus) {
    const idx = filterStatuses.value.indexOf(status)
    if (idx === -1) filterStatuses.value.push(status)
    else filterStatuses.value.splice(idx, 1)
  }

  function clearFilters() {
    filterPlugins.value = []
    filterTypes.value = []
    filterStatuses.value = []
    searchQuery.value = ''
  }

  function onSearchInput(event: CustomEvent) {
    searchQuery.value = event.detail.value ?? ''
  }

  function toggleSort() {
    sortBy.value = sortBy.value === 'activity' ? 'created' : 'activity'
  }

  function applyFilter(opts: { plugins?: string[]; types?: TaskType[]; statuses?: TaskStatus[]; query?: string }) {
    if (opts.plugins !== undefined) filterPlugins.value = opts.plugins
    if (opts.types !== undefined) filterTypes.value = opts.types
    if (opts.statuses !== undefined) filterStatuses.value = opts.statuses
    if (opts.query !== undefined) searchQuery.value = opts.query
  }

  function applySort(sort: SortBy) {
    sortBy.value = sort
  }

  async function fetchTasks() {
    loading.value = true
    try {
      tasks.value = await getTasks()
    } catch {
      tasks.value = []
    }
    loading.value = false
  }

  async function refresh() {
    try {
      tasks.value = await getTasks()
    } catch {
      // silent
    }
  }

  function applyTaskUpdate(data: { id: string; type: string; status: string; progress: number }) {
    const idx = tasks.value.findIndex((t) => t.id === data.id)
    if (idx !== -1) {
      tasks.value[idx] = {
        ...tasks.value[idx],
        status: data.status as TaskStatus,
        progress: data.progress,
      }
    }
  }

  function applyTaskProgress(data: {
    id: string
    progress: number
    phase: string
    speed: string
    eta: string
  }) {
    const idx = tasks.value.findIndex((t) => t.id === data.id)
    if (idx !== -1) {
      tasks.value[idx] = {
        ...tasks.value[idx],
        progress: data.progress,
        phase: data.phase,
        speed: data.speed,
        eta: data.eta,
      }
    }
  }

  function applyTaskCreated(data: { id: string; type: string; sourcePath: string }) {
    const exists = tasks.value.some((t) => t.id === data.id)
    if (!exists) {
      tasks.value.unshift({
        id: data.id,
        type: data.type as TaskType,
        sourcePath: data.sourcePath,
        status: 'queued',
        progress: 0,
        createdAt: new Date().toISOString(),
      })
    }
  }

  function applyTaskCompleted(data: { id: string; error?: string }) {
    const idx = tasks.value.findIndex((t) => t.id === data.id)
    if (idx !== -1) {
      const prev = tasks.value[idx]
      tasks.value[idx] = {
        ...prev,
        status: data.error ? 'failed' : 'completed',
        progress: data.error ? prev.progress : 100,
        phase: data.error ? prev.phase : 'completed',
        speed: '',
        eta: '',
        error: data.error,
        completedAt: new Date().toISOString(),
      }
    }
  }

  async function cancelTaskById(id: string) {
    try {
      await cancelTask(id)
      await fetchTasks()
    } catch {
      showToast({ message: t('tasks.taskCancelFailed'), duration: 2000, color: 'danger' })
    }
  }

  async function retryTaskById(id: string) {
    try {
      await retryTask(id)
      await fetchTasks()
    } catch {
      showToast({ message: t('tasks.taskRetryFailed'), duration: 2000, color: 'danger' })
    }
  }

  async function removeTaskById(id: string) {
    try {
      await removeTask(id)
      await fetchTasks()
    } catch {
      showToast({ message: t('tasks.taskRemoveFailed'), duration: 2000, color: 'danger' })
    }
  }

  async function clearCompletedWithConfirm() {
    const completedCount = tasks.value.filter(
      (task) =>
        task.status === 'completed' || task.status === 'failed' || task.status === 'cancelled'
    ).length
    if (completedCount === 0) return
    return completedCount
  }

  function getTaskName(task: EncvTask) {
    const parts = task.sourcePath.replace(/\\/g, '/').split('/')
    const basename = parts[parts.length - 1] || task.sourcePath
    return task.pluginName ? `${basename} [${task.pluginName}]` : basename
  }

  function getTaskDuration(task: EncvTask): string {
    if (!task.createdAt) return ''
    const created = new Date(task.createdAt).getTime()
    if (isNaN(created)) return ''
    if (task.completedAt) {
      const completed = new Date(task.completedAt).getTime()
      if (isNaN(completed)) return ''
      return formatDuration(completed - created)
    }
    if (task.status === 'running' || task.status === 'cancelling') {
      return formatDuration(Date.now() - created)
    }
    return ''
  }

  function getPluginChipLabel(): string {
    if (filterPlugins.value.length === 0) return t('tasks.allPlugins')
    if (filterPlugins.value.length === 1) return filterPlugins.value[0]
    return `${t('tasks.allPlugins')} (${filterPlugins.value.length})`
  }

  function getTypeChipLabel(): string {
    if (filterTypes.value.length === 0) return t('tasks.allTypes')
    return filterTypes.value
      .map((ty) => (ty === 'encrypt' ? t('tasks.encrypt') : t('tasks.decrypt')))
      .join(', ')
  }

  function getStatusChipLabel(): string {
    if (filterStatuses.value.length === 0) return t('tasks.allStatuses')
    return filterStatuses.value.map((s) => getStatusLabel(s)).join(', ')
  }

  function getStatusLabel(status: TaskStatus) {
    switch (status) {
      case 'queued': return t('tasks.queued')
      case 'running': return t('tasks.running')
      case 'completed': return t('tasks.completed')
      case 'failed': return t('tasks.failed')
      case 'cancelled': return t('tasks.cancelled')
      case 'cancelling': return t('tasks.cancelling')
      default: return status
    }
  }

  function isPasswordError(task: EncvTask): boolean {
    if (!task.error) return false
    return isWrongPasswordError(task.error)
  }

  function toggleWarningDetail(task: EncvTask) {
    expandedWarningDetail.value = expandedWarningDetail.value === task.id ? null : task.id
  }

  function formatWarningDetail(detail: string): string {
    try {
      return JSON.stringify(JSON.parse(detail), null, 2)
    } catch {
      return detail
    }
  }

  function getTaskIcon(task: EncvTask) {
    switch (task.status) {
      case 'queued': return timer
      case 'running': return sync
      case 'completed': return checkmarkCircle
      case 'failed': return closeCircle
      default: return timer
    }
  }

  function getTaskColor(task: EncvTask) {
    switch (task.status) {
      case 'queued': return 'medium'
      case 'running': return 'primary'
      case 'completed': return 'success'
      case 'failed': return 'danger'
      default: return 'medium'
    }
  }

  function getStatusColor(status: TaskStatus) {
    switch (status) {
      case 'queued': return 'medium'
      case 'running': return 'primary'
      case 'completed': return 'success'
      case 'failed': return 'danger'
      default: return 'medium'
    }
  }

  function getPhaseLabel(phase: string) {
    switch (phase) {
      case 'analyzing': return t('tasks.phaseAnalyzing')
      case 'initializing': return t('tasks.phaseInitializing')
      case 'preprocessing': return t('tasks.phasePreprocessing')
      case 'encrypting': return t('tasks.phaseEncrypting')
      case 'decrypting': return t('tasks.phaseDecrypting')
      case 'packing': return t('tasks.phasePacking')
      case 'verifying': return t('tasks.phaseVerifying')
      case 'completed': return t('tasks.phaseCompleted')
      default: return phase
    }
  }

  return {
    tasks,
    loading,
    expandedWarningDetail,
    sortBy,
    showSearch,
    searchQuery,
    showFilters,
    filterPlugins,
    filterTypes,
    filterStatuses,
    statusOptions,
    pluginPopoverOpen,
    typePopoverOpen,
    statusPopoverOpen,
    pluginPopoverEvent,
    typePopoverEvent,
    statusPopoverEvent,
    availablePlugins,
    hasActiveFilters,
    hasCompletedTasks,
    sortedTasks,
    filteredTasks,
    fetchTasks,
    refresh,
    applyFilter,
    applySort,
    openPluginPopover,
    openTypePopover,
    openStatusPopover,
    togglePluginFilter,
    toggleTypeFilter,
    toggleStatusFilter,
    clearFilters,
    onSearchInput,
    toggleSort,
    applyTaskUpdate,
    applyTaskProgress,
    applyTaskCreated,
    applyTaskCompleted,
    cancelTaskById,
    retryTaskById,
    removeTaskById,
    clearCompletedWithConfirm,
    getTaskName,
    getTaskDuration,
    getPluginChipLabel,
    getTypeChipLabel,
    getStatusChipLabel,
    getStatusLabel,
    isPasswordError,
    toggleWarningDetail,
    formatWarningDetail,
    getTaskIcon,
    getTaskColor,
    getStatusColor,
    getPhaseLabel,
  }
}
