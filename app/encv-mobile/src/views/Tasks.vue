<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>{{ t('tasks.title') }}</ion-title>
        <ion-buttons slot="end">
          <ion-button fill="clear" size="small" @click="toggleSort" class="toolbar-btn">
            <ion-icon :icon="sortBy === 'activity' ? sync : timer" slot="icon-only"></ion-icon>
          </ion-button>
          <ion-button fill="clear" size="small" @click="handleClearCompleted" class="toolbar-btn" :disabled="!hasCompletedTasks">
            <ion-icon :icon="trashBin" slot="icon-only" color="danger"></ion-icon>
          </ion-button>
        </ion-buttons>
      </ion-toolbar>

      <ion-toolbar v-if="showSearch">
        <ion-searchbar
          :value="searchQuery"
          @ionInput="onSearchInput"
          :placeholder="t('tasks.searchPlaceholder')"
          show-cancel-button="focus"
          @ionCancel="showSearch = false; searchQuery = ''"
          :debounce="200"
          class="task-searchbar"
        ></ion-searchbar>
      </ion-toolbar>

      <ion-toolbar v-if="showFilters" class="filter-toolbar">
        <div class="filter-chips">
          <ion-chip :color="filterPlugins.length > 0 ? 'primary' : 'medium'" @click="openPluginPopover($event)">
            <ion-icon :icon="extensionPuzzle" size="small"></ion-icon>
            <ion-label>{{ getPluginChipLabel() }}</ion-label>
            <ion-icon :icon="chevronDown" size="small"></ion-icon>
          </ion-chip>
          <ion-chip :color="filterTypes.length > 0 ? 'primary' : 'medium'" @click="openTypePopover($event)">
            <ion-icon :icon="swapVertical" size="small"></ion-icon>
            <ion-label>{{ getTypeChipLabel() }}</ion-label>
            <ion-icon :icon="chevronDown" size="small"></ion-icon>
          </ion-chip>
          <ion-chip :color="filterStatuses.length > 0 ? 'primary' : 'medium'" @click="openStatusPopover($event)">
            <ion-icon :icon="funnel" size="small"></ion-icon>
            <ion-label>{{ getStatusChipLabel() }}</ion-label>
            <ion-icon :icon="chevronDown" size="small"></ion-icon>
          </ion-chip>
        </div>
      </ion-toolbar>

      <ion-popover
        :is-open="pluginPopoverOpen"
        :event="pluginPopoverEvent"
        @didDismiss="pluginPopoverOpen = false"
        side="bottom"
        alignment="start"
      >
        <div class="popover-filter-content">
          <div class="popover-filter-title">{{ t('tasks.filterByPlugin') }}</div>
          <ion-item
            v-for="plugin in availablePlugins"
            :key="plugin"
            lines="none"
            class="popover-filter-item"
            @click="togglePluginFilter(plugin)"
          >
            <ion-checkbox
              :checked="filterPlugins.includes(plugin)"
              slot="start"
              @ionChange="togglePluginFilter(plugin)"
            ></ion-checkbox>
            <ion-label>{{ plugin }}</ion-label>
          </ion-item>
          <div v-if="availablePlugins.length === 0" class="popover-empty">{{ t('tasks.noPluginsFound') }}</div>
        </div>
      </ion-popover>

      <ion-popover
        :is-open="typePopoverOpen"
        :event="typePopoverEvent"
        @didDismiss="typePopoverOpen = false"
        side="bottom"
        alignment="start"
      >
        <div class="popover-filter-content">
          <div class="popover-filter-title">{{ t('tasks.filterByType') }}</div>
          <ion-item lines="none" class="popover-filter-item" @click="toggleTypeFilter('encrypt')">
            <ion-checkbox :checked="filterTypes.includes('encrypt')" slot="start" @ionChange="toggleTypeFilter('encrypt')"></ion-checkbox>
            <ion-label>{{ t('tasks.encrypt') }}</ion-label>
          </ion-item>
          <ion-item lines="none" class="popover-filter-item" @click="toggleTypeFilter('decrypt')">
            <ion-checkbox :checked="filterTypes.includes('decrypt')" slot="start" @ionChange="toggleTypeFilter('decrypt')"></ion-checkbox>
            <ion-label>{{ t('tasks.decrypt') }}</ion-label>
          </ion-item>
        </div>
      </ion-popover>

      <ion-popover
        :is-open="statusPopoverOpen"
        :event="statusPopoverEvent"
        @didDismiss="statusPopoverOpen = false"
        side="bottom"
        alignment="start"
      >
        <div class="popover-filter-content">
          <div class="popover-filter-title">{{ t('tasks.filterByStatus') }}</div>
          <ion-item v-for="s in statusOptions" :key="s" lines="none" class="popover-filter-item" @click="toggleStatusFilter(s)">
            <ion-checkbox :checked="filterStatuses.includes(s)" slot="start" @ionChange="toggleStatusFilter(s)"></ion-checkbox>
            <ion-label>{{ getStatusLabel(s) }}</ion-label>
          </ion-item>
        </div>
      </ion-popover>
    </ion-header>

    <ion-content>
      <ion-refresher slot="fixed" @ionRefresh="handleRefresh">
        <ion-refresher-content></ion-refresher-content>
      </ion-refresher>

      <div class="toolbar-actions">
        <ion-button fill="clear" size="small" @click="showSearch = !showSearch" class="action-btn">
          <ion-icon :icon="search" slot="icon-only"></ion-icon>
        </ion-button>
        <ion-button fill="clear" size="small" @click="showFilters = !showFilters" class="action-btn">
          <ion-icon :icon="funnel" slot="icon-only" :color="hasActiveFilters ? 'primary' : undefined"></ion-icon>
        </ion-button>
      </div>

      <div v-if="loading" class="loading-container">
        <ion-spinner name="crescent"></ion-spinner>
        <p>{{ t('tasks.loading') }}</p>
      </div>

      <div v-else-if="filteredTasks.length === 0 && tasks.length > 0" class="empty-state">
        <ion-icon :icon="search" class="empty-icon"></ion-icon>
        <h3>{{ t('tasks.noMatchingTasks') }}</h3>
        <p>{{ t('tasks.noMatchingTasksDesc') }}</p>
        <ion-button fill="clear" size="small" @click="clearFilters">{{ t('tasks.clearFilters') }}</ion-button>
      </div>

      <div v-else-if="tasks.length === 0" class="empty-state">
        <ion-icon :icon="checkmarkCircle" class="empty-icon"></ion-icon>
        <h3>{{ t('tasks.noTasks') }}</h3>
        <p>{{ t('tasks.noTasksDesc') }}</p>
      </div>

      <ion-list v-else>
        <ion-item-sliding v-for="task in filteredTasks" :key="task.id">
          <ion-item @click="openTaskDetail(task)" button detail>
            <ion-icon
              :icon="getTaskIcon(task)"
              :color="getTaskColor(task)"
              slot="start"
            ></ion-icon>
            <ion-label>
              <h2>{{ getTaskName(task) }}</h2>
              <p class="card-meta-row">
                <ion-badge :color="getStatusColor(task.status)" class="status-badge">
                  {{ getStatusLabel(task.status) }}
                </ion-badge>
                <span class="task-type">{{ task.type === 'encrypt' ? t('tasks.encrypt') : t('tasks.decrypt') }}</span>
                <ion-badge v-if="task.pluginName" color="primary" class="plugin-badge">
                  {{ task.pluginName }}
                </ion-badge>
              </p>
              <p class="task-time-info">
                <span class="time-created">{{ formatDateTime(task.createdAt) }}</span>
                <span v-if="getTaskDuration(task)" class="time-duration">{{ getTaskDuration(task) }}</span>
              </p>
              <div v-if="task.status === 'running' || task.status === 'cancelling'" class="progress-section">
                <ion-progress-bar
                  :value="task.progress / 100"
                  :class="['task-progress', { 'progress-cancelling': task.status === 'cancelling' }]"
                ></ion-progress-bar>
                <div class="progress-detail">
                  <span v-if="task.phase" class="phase-label">{{ getPhaseLabel(task.phase) }}</span>
                  <span class="progress-percent">{{ task.progress }}%</span>
                  <span v-if="task.speed" class="speed-label">{{ task.speed }}</span>
                  <span v-if="task.eta" class="eta-label">{{ t('tasks.eta') }} {{ task.eta }}</span>
                </div>
              </div>
              <div v-if="task.status === 'completed'" class="completed-info">
                <ion-icon :icon="checkmarkCircle" color="success" class="completed-icon"></ion-icon>
                <span class="completed-text">{{ t('tasks.phaseCompleted') }}</span>
                <span v-if="task.containerVersion" class="container-version">V{{ task.containerVersion }}</span>
              </div>
              <div v-if="task.warning" class="task-warning" @click="toggleWarningDetail(task)">
                <ion-icon :icon="warningOutline" class="warning-icon"></ion-icon>
                <span class="task-warning-text">{{ task.warning }}</span>
              </div>
              <div v-if="expandedWarningDetail === task.id && task.warningDetail" class="task-warning-detail">
                <pre>{{ formatWarningDetail(task.warningDetail) }}</pre>
              </div>
              <p v-if="isPasswordError(task)" class="task-error password-error">
                <ion-icon :icon="lockClosed"></ion-icon>
                {{ t('tasks.passwordErrorHint') }}
              </p>
              <p v-else-if="task.error" class="task-error">{{ task.error }}</p>
            </ion-label>
            <ion-button
              v-if="task.status === 'running'"
              slot="end"
              fill="clear"
              color="warning"
              size="small"
              @click="handleCancelTask(task.id)"
            >
              <ion-icon :icon="closeCircle" slot="icon-only"></ion-icon>
            </ion-button>
            <ion-spinner
              v-if="task.status === 'cancelling'"
              slot="end"
              name="crescent"
              color="warning"
              class="cancelling-spinner"
            ></ion-spinner>
          </ion-item>
          <ion-item-options side="end">
            <ion-item-option
              v-if="task.status === 'queued'"
              color="warning"
              @click="handleCancelTask(task.id)"
            >
              {{ t('tasks.cancel') }}
            </ion-item-option>
            <ion-item-option
              v-if="task.status === 'failed'"
              color="primary"
              @click="handleRetryTask(task.id)"
            >
              {{ t('tasks.retry') }}
            </ion-item-option>
            <ion-item-option
              v-if="task.status === 'completed' || task.status === 'failed' || task.status === 'cancelled'"
              color="danger"
              @click="handleRemoveTask(task.id)"
            >
              {{ t('tasks.remove') }}
            </ion-item-option>
          </ion-item-options>
        </ion-item-sliding>
      </ion-list>

      <ion-fab vertical="bottom" horizontal="end" slot="fixed">
        <ion-fab-button @click="openNewTask()">
          <ion-icon :icon="add"></ion-icon>
        </ion-fab-button>
      </ion-fab>

    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  IonPage,
  IonHeader,
  IonToolbar,
  IonTitle,
  IonContent,
  IonRefresher,
  IonRefresherContent,
  IonList,
  IonItem,
  IonItemSliding,
  IonItemOptions,
  IonItemOption,
  IonIcon,
  IonLabel,
  IonBadge,
  IonProgressBar,
  IonFab,
  IonFabButton,
  IonSpinner,
  IonButton,
  IonButtons,
  IonSearchbar,
  IonChip,
  IonPopover,
  IonCheckbox,
  alertController,
  modalController,
} from '@ionic/vue'
import {
  add,
  closeCircle,
  checkmarkCircle,
  timer,
  sync,
  warningOutline,
  lockClosed,
  search,
  funnel,
  trashBin,
  extensionPuzzle,
  swapVertical,
  chevronDown,
} from 'ionicons/icons'
import { useRoute, useRouter } from 'vue-router'
import {
  getTasks,
  cancelTask,
  retryTask,
  removeTask,
  clearCompletedTasks,
  isWrongPasswordError,
} from '@/api/encv'
import type { EncvTask, TaskType, TaskStatus } from '@/api/encv'
import { eventBus } from '@/composables/useEventBus'
import { useI18n } from '@/composables/useI18n'
import { formatDateTime, formatDuration } from '@/composables/useDateFormat'
import { showToast } from '@/composables/useToast'
import { useNewTaskModal } from '@/composables/useNewTaskModal'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const { openNewTask } = useNewTaskModal()

const tasks = ref<EncvTask[]>([])
const loading = ref(false)
const expandedWarningDetail = ref<string | null>(null)
const sortBy = ref<'activity' | 'created'>('activity')

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
  tasks.value.some(t => t.status === 'completed' || t.status === 'failed' || t.status === 'cancelled')
)

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

function getPluginChipLabel(): string {
  if (filterPlugins.value.length === 0) return t('tasks.allPlugins')
  if (filterPlugins.value.length === 1) return filterPlugins.value[0]
  return `${t('tasks.allPlugins')} (${filterPlugins.value.length})`
}

function getTypeChipLabel(): string {
  if (filterTypes.value.length === 0) return t('tasks.allTypes')
  return filterTypes.value.map(ty => ty === 'encrypt' ? t('tasks.encrypt') : t('tasks.decrypt')).join(', ')
}

function getStatusChipLabel(): string {
  if (filterStatuses.value.length === 0) return t('tasks.allStatuses')
  return filterStatuses.value.map(s => getStatusLabel(s)).join(', ')
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

function getStatusLabel(status: TaskStatus) {
  switch (status) {
    case 'queued': return t('tasks.queued')
    case 'running': return t('tasks.running')
    case 'completed': return t('tasks.completed')
    case 'failed': return t('tasks.failed')
    case 'cancelled': return t('tasks.cancelled')
    case 'cancelling': return t('tasks.cancelling')
    default: status
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
    default: phase
  }
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

const sortedTasks = computed(() => {
  const arr = [...tasks.value]
  if (sortBy.value === 'activity') {
    arr.sort((a, b) => {
      const timeA = a.completedAt ? new Date(a.completedAt).getTime() : new Date(a.createdAt).getTime()
      const timeB = b.completedAt ? new Date(b.completedAt).getTime() : new Date(b.createdAt).getTime()
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
    result = result.filter(task => {
      const name = getTaskName(task).toLowerCase()
      const plugin = (task.pluginName || '').toLowerCase()
      const error = (task.error || '').toLowerCase()
      return name.includes(q) || plugin.includes(q) || error.includes(q)
    })
  }

  if (filterPlugins.value.length > 0) {
    result = result.filter(task => task.pluginName && filterPlugins.value.includes(task.pluginName))
  }

  if (filterTypes.value.length > 0) {
    result = result.filter(task => filterTypes.value.includes(task.type))
  }

  if (filterStatuses.value.length > 0) {
    result = result.filter(task => filterStatuses.value.includes(task.status))
  }

  return result
})

function toggleSort() {
  sortBy.value = sortBy.value === 'activity' ? 'created' : 'activity'
}

async function openTaskDetail(task: EncvTask) {
  const { default: TaskDetailModal } = await import('@/components/TaskDetailModal.vue')
  const modal = await modalController.create({
    component: TaskDetailModal,
    componentProps: { task },
    cssClass: 'task-detail-modal',
  })
  await modal.present()
  const { data, role } = await modal.onDidDismiss()
  if (role === 'dismiss' && data) {
    if (data.action === 'cancel') await handleCancelTask(data.id)
    else if (data.action === 'retry') await handleRetryTask(data.id)
    else if (data.action === 'remove') await handleRemoveTask(data.id)
  }
}

function isPasswordError(task: EncvTask): boolean {
  if (!task.error) return false
  return isWrongPasswordError(task.error)
}

async function loadTasks() {
  loading.value = true
  try {
    tasks.value = await getTasks()
  } catch {
    tasks.value = []
  }
  loading.value = false
}

function toggleWarningDetail(task: EncvTask) {
  expandedWarningDetail.value = expandedWarningDetail.value === task.id ? null : task.id
}

function formatWarningDetail(detail: string): string {
  try { return JSON.stringify(JSON.parse(detail), null, 2) }
  catch { return detail }
}

async function handleRefresh(event: CustomEvent) {
  try {
    tasks.value = await getTasks()
  } catch {
    // silent
  }
  ;(event.target as any)?.complete?.()
}

async function handleCancelTask(id: string) {
  try {
    await cancelTask(id)
    await loadTasks()
  } catch {
    showToast({ message: t('tasks.taskCancelFailed'), duration: 2000, color: 'danger' })
  }
}

async function handleRetryTask(id: string) {
  try {
    await retryTask(id)
    await loadTasks()
  } catch {
    showToast({ message: t('tasks.taskRetryFailed'), duration: 2000, color: 'danger' })
  }
}

async function handleRemoveTask(id: string) {
  try {
    await removeTask(id)
    await loadTasks()
  } catch {
    showToast({ message: t('tasks.taskRemoveFailed'), duration: 2000, color: 'danger' })
  }
}

async function handleClearCompleted() {
  const completedCount = tasks.value.filter(
    t => t.status === 'completed' || t.status === 'failed' || t.status === 'cancelled'
  ).length
  if (completedCount === 0) return

  const alert = await alertController.create({
    header: t('tasks.clearConfirmTitle'),
    message: t('tasks.clearConfirmMessage', { count: String(completedCount) }),
    buttons: [
      { text: t('tasks.cancel'), role: 'cancel' },
      {
        text: t('tasks.clearConfirm'),
        role: 'destructive',
        handler: async () => {
          try {
            const result = await clearCompletedTasks()
            showToast({ message: t('tasks.cleared', { count: String(result.removed) }), duration: 2000, color: 'success' })
            await loadTasks()
          } catch {
            showToast({ message: t('tasks.clearFailed'), duration: 2000, color: 'danger' })
          }
        },
      },
    ],
  })
  await alert.present()
}

function onTaskUpdate(data: { id: string; type: string; status: string; progress: number }) {
  const idx = tasks.value.findIndex(t => t.id === data.id)
  if (idx !== -1) {
    tasks.value[idx] = { ...tasks.value[idx], status: data.status as TaskStatus, progress: data.progress }
  }
}

function onTaskProgress(data: { id: string; progress: number; phase: string; speed: string; eta: string }) {
  const idx = tasks.value.findIndex(t => t.id === data.id)
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

function onTaskCreated(data: { id: string; type: string; sourcePath: string }) {
  const exists = tasks.value.some(t => t.id === data.id)
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

function onTaskCompleted(data: { id: string; status?: string; error?: string; errorDetail?: string; warning?: string; warningDetail?: string }) {
  const idx = tasks.value.findIndex(t => t.id === data.id)
  if (idx !== -1) {
    tasks.value[idx] = {
      ...tasks.value[idx],
      status: data.error ? 'failed' : 'completed',
      progress: data.error ? tasks.value[idx].progress : 100,
      phase: data.error ? tasks.value[idx].phase : 'completed',
      speed: '',
      eta: '',
      error: data.error,
      errorDetail: data.errorDetail,
      warning: data.warning,
      warningDetail: data.warningDetail,
      completedAt: new Date().toISOString(),
    }
  }
}

onMounted(() => {
  loadTasks()
  eventBus.on('task:update', onTaskUpdate)
  eventBus.on('task:progress', onTaskProgress)
  eventBus.on('task:created', onTaskCreated)
  eventBus.on('task:completed', onTaskCompleted)
  eventBus.on('task:refresh', loadTasks)

  if (route.query.action === 'new') {
    const sourcePath = route.query.source as string
    const taskType = (route.query.type || 'encrypt') as TaskType
    router.replace({ path: '/tabs/tasks', query: {} })
    if (sourcePath) {
      openNewTask(sourcePath, taskType)
    } else {
      openNewTask()
    }
  }
})

onUnmounted(() => {
  eventBus.off('task:update', onTaskUpdate)
  eventBus.off('task:progress', onTaskProgress)
  eventBus.off('task:created', onTaskCreated)
  eventBus.off('task:completed', onTaskCompleted)
  eventBus.off('task:refresh', loadTasks)
})
</script>

<style scoped>
.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 50%;
  color: var(--encv-text-secondary);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 50%;
  padding: 24px;
  text-align: center;
  color: var(--encv-text-secondary);
}

.empty-icon {
  font-size: 64px;
  margin-bottom: 16px;
  opacity: 0.5;
}

.toolbar-actions {
  display: flex;
  justify-content: flex-end;
  gap: 4px;
  padding: 4px 16px 0;
}

.action-btn {
  --color: var(--ion-color-medium);
  --padding-start: 8px;
  --padding-end: 8px;
  font-size: 18px;
}

.toolbar-btn {
  --color: var(--ion-color-medium);
  --padding-start: 8px;
  --padding-end: 8px;
  font-size: 20px;
}

.task-searchbar {
  --padding-start: 12px;
  --padding-end: 12px;
  padding-top: 4px;
  padding-bottom: 4px;
}

.filter-toolbar {
  --padding-start: 8px;
  --padding-end: 8px;
  --min-height: 44px;
}

.filter-chips {
  display: flex;
  gap: 6px;
  padding: 4px 8px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}

.filter-chips ion-chip {
  flex-shrink: 0;
  font-size: 12px;
  --padding-start: 8px;
  --padding-end: 10px;
}

.status-badge {
  margin-right: 8px;
  font-size: 11px;
}

.task-type {
  font-size: 12px;
  color: var(--encv-text-secondary);
  margin-left: 6px;
}

.card-meta-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.plugin-badge {
  font-size: 10px;
  --padding-start: 6px;
  --padding-end: 6px;
  --padding-top: 2px;
  --padding-bottom: 2px;
  font-weight: 500;
}

.task-time-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 2px;
  font-size: 11px;
  color: var(--encv-text-secondary);
}

.time-created {
  color: var(--encv-text-secondary);
}

.time-duration {
  color: var(--ion-color-primary);
  font-weight: 500;
}

.progress-section {
  margin-top: 6px;
}

.task-progress {
  margin-top: 2px;
}

.progress-cancelling {
  --progress-background: var(--ion-color-warning);
}

.progress-detail {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  font-size: 11px;
  color: var(--encv-text-secondary);
  flex-wrap: wrap;
}

.phase-label {
  color: var(--ion-color-primary);
  font-weight: 500;
}

.progress-percent {
  font-weight: 600;
  color: var(--encv-text-secondary);
}

.speed-label {
  color: var(--encv-text-secondary);
}

.eta-label {
  color: var(--encv-text-secondary);
}

.completed-info {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
}

.completed-icon {
  font-size: 16px;
}

.completed-text {
  font-size: 12px;
  color: var(--ion-color-success);
}

.container-version {
  font-size: 11px;
  font-weight: 600;
  color: var(--ion-color-primary);
  background: rgba(var(--ion-color-primary-rgb), 0.12);
  padding: 1px 6px;
  border-radius: 4px;
  margin-left: 6px;
}

.task-error {
  color: var(--ion-color-danger);
  font-size: 12px;
  margin-top: 4px;
}

.password-error {
  display: flex;
  align-items: center;
  gap: 4px;
  background: rgba(var(--ion-color-danger-rgb), 0.08);
  padding: 6px 10px;
  border-radius: 6px;
  border-left: 3px solid var(--ion-color-danger);
}

.password-error ion-icon {
  font-size: 14px;
  flex-shrink: 0;
}

.task-warning {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 12px;
  margin-top: 4px;
  background: rgba(255, 152, 0, 0.1);
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
  color: #e65100;
}

.warning-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.task-warning-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-warning-detail {
  padding: 8px 12px;
  margin-top: 4px;
  background: var(--ion-color-step-50, #f0f0f0);
  border-radius: 4px;
  max-height: 150px;
  overflow-y: auto;
}

.task-warning-detail pre {
  margin: 0;
  font-size: 11px;
  white-space: pre-wrap;
  word-break: break-all;
  color: #666;
}

.cancelling-spinner {
  width: 20px;
  height: 20px;
}

.popover-filter-content {
  padding: 8px 0;
  min-width: 180px;
  max-height: 320px;
  overflow-y: auto;
}

.popover-filter-title {
  font-size: 13px;
  font-weight: 600;
  padding: 4px 16px 8px;
  color: var(--encv-text-secondary);
}

.popover-filter-item {
  --min-height: 40px;
  --padding-start: 12px;
  --padding-end: 12px;
  cursor: pointer;
}

.popover-filter-item ion-checkbox {
  margin-right: 8px;
}

.popover-empty {
  padding: 12px 16px;
  font-size: 13px;
  color: var(--encv-text-secondary);
}
</style>
