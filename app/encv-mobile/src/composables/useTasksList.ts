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
import { getTaskMetadata } from './useTaskTrigger'

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

  // 🆕 2026-06-10 修复 v4：fetchTasks 后批量补回元数据
  // 历史：fetchTasks 用后端数据替换 tasks.value，丢失所有 triggeredBy/runId（后端不存）
  //   → 重新跑也没用，因为新数据一覆盖就没了
  // 修复：替换后遍历新数组，对每个 taskId 查 taskMetadata，merge 回去
  async function fetchTasks() {
    loading.value = true
    try {
      const data = await getTasks()
      const enriched = data.map((t) => {
        const meta = getTaskMetadata(t.id)
        if (!meta) return t
        return { ...t, triggeredBy: meta.triggeredBy, runId: meta.runId }
      })
      tasks.value = enriched
    } catch {
      tasks.value = []
    }
    loading.value = false
  }

  async function refresh() {
    try {
      // 🆕 v4：refresh 也补回元数据
      const data = await getTasks()
      const enriched = data.map((t) => {
        const meta = getTaskMetadata(t.id)
        if (!meta) return t
        return { ...t, triggeredBy: meta.triggeredBy, runId: meta.runId }
      })
      tasks.value = enriched
    } catch {
      // silent
    }
  }

  // 🆕 2026-06-10 修复：所有 apply* 函数都 spread 完整 payload
  // 历史 bug：applyTaskCreated 只构造 {id, type, sourcePath, status, progress, createdAt}（6 字段）
  //   → 丢失 pluginName/version/targetPath/extraFields
  //   → Tasks.vue 任务组按 pluginName 分桶 → 全部落到 '(unknown plugin)' → 「插件没正确识别，任务依旧全部平铺」
  // 修复：spread data 整个对象，WS 后端会发完整 *MobileTask（含 pluginName）

  function applyTaskUpdate(data: { id: string; type: string; status: string; progress: number }) {
    const idx = tasks.value.findIndex((t) => t.id === data.id)
    if (idx !== -1) {
      tasks.value[idx] = {
        ...tasks.value[idx],
        ...data,  // 🆕 spread 整个 data（防 pluginName 丢失）
        type: data.type as TaskType,
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

  function applyTaskCreated(data: {
    id: string
    type: string
    sourcePath: string
    pluginName?: string
    version?: number
    targetPath?: string
    createdAt?: string
    [k: string]: any  // 允许后端多发字段（status/progress/phase/extraFields 等）
  }) {
    const exists = tasks.value.some((t) => t.id === data.id)
    if (!exists) {
      // 🆕 2026-06-10 修复 v4：从 useTaskTrigger.taskMetadata 合并 triggeredBy + runId
      // 历史 bug：WS task:created payload 没有这 2 字段（后端不知道），tasks.value 里的 task 也跟着没有
      //   → displayedItems 必须靠 getTriggeredBy / getRunIdForTask 查 localStorage
      //   → 跨 session / localStorage 清空 → 分组完全失效 → 「任务组只在一开始正确，插件没正确识别」
      // 修复：useWorkflowEngine 在 submitAction 后立即 setTaskMetadata(task.id, ...)，
      //   applyTaskCreated 收到 WS 事件时通过 taskId 取回 → spread 进 task 对象
      const meta = getTaskMetadata(data.id)
      tasks.value.unshift({
        ...data,  // spread 完整 payload（pluginName/version/targetPath/createdAt 等保留）
        id: data.id,
        type: data.type as TaskType,
        sourcePath: data.sourcePath,
        pluginName: data.pluginName,  // 🆕 关键字段：保持插件识别
        // 🆕 v4：data.version 是后端 container version 字段，前端 EncvTask 命名是 containerVersion
        containerVersion: data.version,
        targetPath: data.targetPath,
        status: data.status ?? 'queued',
        progress: data.progress ?? 0,
        phase: data.phase,
        createdAt: data.createdAt ?? new Date().toISOString(),
        // 🆕 v4：把元数据写进 task 对象本身（displayedItems 直接读，不再依赖 localStorage）
        triggeredBy: meta?.triggeredBy,
        runId: meta?.runId,
      })
    }
  }

  function applyTaskCompleted(data: { id: string; error?: string }) {
    const idx = tasks.value.findIndex((t) => t.id === data.id)
    if (idx !== -1) {
      const prev = tasks.value[idx]
      tasks.value[idx] = {
        ...prev,  // 🆕 prev 已经有 pluginName（从 spread 来），不再被覆盖
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
