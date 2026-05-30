<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-title>{{ t('tasks.title') }}</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <ion-refresher slot="fixed" @ionRefresh="handleRefresh">
        <ion-refresher-content></ion-refresher-content>
      </ion-refresher>

      <div v-if="loading" class="loading-container">
        <ion-spinner name="crescent"></ion-spinner>
        <p>{{ t('tasks.loading') }}</p>
      </div>

      <div v-else-if="tasks.length === 0" class="empty-state">
        <ion-icon :icon="checkmarkCircle" class="empty-icon"></ion-icon>
        <h3>{{ t('tasks.noTasks') }}</h3>
        <p>{{ t('tasks.noTasksDesc') }}</p>
      </div>

      <ion-list v-else>
        <ion-item-sliding v-for="task in tasks" :key="task.id">
          <ion-item>
            <ion-icon
              :icon="getTaskIcon(task)"
              :color="getTaskColor(task)"
              slot="start"
            ></ion-icon>
            <ion-label>
              <h2>{{ getTaskName(task) }}</h2>
              <p>
                <ion-badge :color="getStatusColor(task.status)" class="status-badge">
                  {{ getStatusLabel(task.status) }}
                </ion-badge>
                <span class="task-type">{{ task.type === 'encrypt' ? t('tasks.encrypt') : t('tasks.decrypt') }}</span>
              </p>
              <p class="task-time-info">
                <span class="time-created">{{ formatDateTime(task.createdAt) }}</span>
                <span v-if="getTaskDuration(task)" class="time-duration">{{ getTaskDuration(task) }}</span>
              </p>
              <div v-if="task.status === 'running' || task.status === 'cancelling'" class="progress-section">
                <ion-progress-bar
                  :value="task.progress / 100"
                  :buffer="task.status === 'cancelling' ? undefined : undefined"
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
              <div v-if="task.errorDetail && task.errorDetail !== task.error" class="error-detail-row">
                <p class="task-error-detail" @click="toggleErrorDetail(task.id)">
                  {{ showErrorDetail[task.id] ? t('tasks.hideDetail') : t('tasks.showDetail') }}
                </p>
                <ion-button fill="clear" size="small" color="medium" class="copy-btn" @click="copyErrorDetail(task)">
                  <ion-icon :icon="copiedTaskId === task.id ? checkmarkCircle : copyOutline" slot="icon-only"></ion-icon>
                </ion-button>
              </div>
              <pre v-if="showErrorDetail[task.id] && task.errorDetail" class="error-detail-pre">{{ task.errorDetail }}</pre>
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
        <ion-fab-button @click="showNewTaskSheet">
          <ion-icon :icon="add"></ion-icon>
        </ion-fab-button>
      </ion-fab>

      <ion-modal :is-open="showNewTaskModal" @didDismiss="showNewTaskModal = false">
        <ion-header>
          <ion-toolbar>
            <ion-title>{{ t('tasks.newTask') }}</ion-title>
            <ion-buttons slot="end">
              <ion-button @click="showNewTaskModal = false">{{ t('tasks.close') }}</ion-button>
            </ion-buttons>
          </ion-toolbar>
        </ion-header>
        <ion-content class="ion-padding">
          <ion-list>
            <ion-item>
              <ion-select
                v-model="newTaskType"
                interface="action-sheet"
                :label="t('tasks.taskType')"
                label-placement="stacked"
              >
                <ion-select-option value="encrypt">{{ t('tasks.encrypt') }}</ion-select-option>
                <ion-select-option value="decrypt">{{ t('tasks.decrypt') }}</ion-select-option>
              </ion-select>
            </ion-item>
            <ion-item>
              <ion-input
                v-model="newTaskPath"
                :label="t('tasks.sourcePath')"
                label-placement="stacked"
                placeholder="/path/to/file"
                :error-text="sourcePathError"
                :class="{ 'ion-invalid': !!sourcePathError, 'ion-touched': !!sourcePathError }"
                @ionInput="validateSourcePath"
              ></ion-input>
              <ion-button slot="end" fill="clear" class="browse-btn" @click="handleBrowseSource">
                <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
              </ion-button>
            </ion-item>
            <ion-item>
              <ion-input
                v-model="newTaskTargetPath"
                :label="t('tasks.targetPath')"
                label-placement="stacked"
                :placeholder="t('tasks.targetPathPlaceholder')"
                :error-text="targetPathError"
                :class="{ 'ion-invalid': !!targetPathError, 'ion-touched': !!targetPathError }"
                @ionInput="validateTargetPath"
              ></ion-input>
              <ion-button slot="end" fill="clear" class="browse-btn" @click="handleBrowseTarget">
                <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
              </ion-button>
            </ion-item>
          </ion-list>

          <!-- 容器版本选择（仅加密时显示） -->
          <ion-item v-if="newTaskType === 'encrypt'">
            <ContainerVersionSelector v-model="newTaskVersion" />
          </ion-item>

          <!-- 插件预测结果 -->
          <div v-if="candidates.length > 0" class="plugin-section">
            <!-- 多候选时显示选择器（仅当有多个有效候选时）-->
            <div v-if="showPluginSelector && filteredCandidates.length > 1" class="plugin-selector">
              <ion-list>
                <ion-item>
                  <ion-label>{{ t('tasks.selectPlugin') }}</ion-label>
                  <ion-select :model-value="selectedPluginIndex" @ionChange="(e: any) => selectedPluginIndex = e.detail.value" interface="action-sheet" placement="bottom" style="width: 100%; max-width: 220px;">
                    <ion-select-option v-for="(c, idx) in filteredCandidates" :key="idx" :value="c._idx">
                      {{ c.candidate.name }}
                      <span v-if="c.candidate.matchType === 'general'" style="color:var(--ion-color-medium);font-size:11px;margin-left:4px;">({{ t('tasks.generic') }})</span>
                      <span v-else style="color:var(--ion-color-success);font-size:11px;margin-left:4px;">★</span>
                    </ion-select-option>
                  </ion-select>
                </ion-item>
              </ion-list>
            </div>

            <!-- 单候选或自动选中时显示提示 -->
            <div v-else class="plugin-hint">
              <ion-icon :icon="checkmarkCircle" color="success" class="hint-icon"></ion-icon>
              <span>
                {{ t('tasks.willBeHandledBy', { plugin: predictedPlugin ?? '' }) }}
                <span v-if="candidates[selectedPluginIndex]?.matchType === 'general'" style="color:var(--ion-color-medium);font-size:11px;margin-left:4px;">({{ t('tasks.generic') }})</span>
                <span v-else style="color:var(--ion-color-success);font-size:11px;margin-left:4px;">★</span>
              </span>
            </div>

            <!-- 密码策略提示 -->
            <div v-if="taskOptionsForDisplay?.passwordStrategy === 'independent'" class="plugin-hint password-strategy-hint">
              {{ t('tasks.usesIndependentPassword') }}
            </div>
            <div v-else-if="taskOptionsForDisplay" class="plugin-hint">
              {{ t('tasks.usesGlobalPassword') }}
            </div>
          </div>

          <!-- 二级密码（占位，计划中） -->
          <ion-item>
            <ion-input
              v-model="newTaskSecondaryPassword"
              :label="t('tasks.secondaryPassword')"
              label-placement="stacked"
              type="password"
              disabled
              placeholder=""
            >
              <ion-badge color="medium" slot="end">{{ t('tasks.comingSoon') }}</ion-badge>
            </ion-input>
          </ion-item>

          <ion-button expand="block" @click="handleCreateTask" :disabled="!newTaskPath || !!sourcePathError || !!targetPathError">
            <ion-icon :icon="lockClosed" slot="start"></ion-icon>
            {{ t('tasks.createTask') }}
          </ion-button>
        </ion-content>
      </ion-modal>
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
  IonModal,
  IonButtons,
  IonButton,
  IonSelect,
  IonSelectOption,
  IonInput,
  IonSpinner,
  modalController,
} from '@ionic/vue'
import {
  add,
  lockClosed,
  checkmarkCircle,
  closeCircle,
  timer,
  sync,
  folderOpen,
  copyOutline,
  warningOutline,
} from 'ionicons/icons'
import { useRoute, useRouter } from 'vue-router'
import ContainerVersionSelector from '@/components/ContainerVersionSelector.vue'
import {
  getTasks,
  createTask,
  cancelTask,
  retryTask,
  removeTask,
  listFiles,
  isWrongPasswordError,
} from '@/api/encv'
import type { EncvTask, TaskType, TaskStatus } from '@/api/encv'
import { eventBus } from '@/composables/useEventBus'
import { useI18n } from '@/composables/useI18n'
import { useConfig } from '@/composables/useConfig'
import { formatDateTime, formatDuration } from '@/composables/useDateFormat'
import { showToast } from '@/composables/useToast'
import FilePickerModal from '@/components/FilePickerModal.vue'
import { useTaskForm } from '@/composables/useTaskForm'
import { usePathResolver } from '@/composables/usePathResolver'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const { config } = useConfig()
const { normalize } = usePathResolver()

const {
  candidates,
  predictedPlugin,
  selectedPluginIndex,
  predictPlugin: doPredict,
  reset: resetTaskForm,
} = useTaskForm()

// 插件候选排序：非 general 优先，按 priority 降序
const sortedCandidates = computed(() => {
  return candidates.value
    .map((c, idx) => ({ candidate: c, _idx: idx }))
    .sort((a, b) => {
      if (a.candidate.matchType === 'general' && b.candidate.matchType !== 'general') return 1
      if (b.candidate.matchType === 'general' && a.candidate.matchType !== 'general') return -1
      return b.candidate.priority - a.candidate.priority
    })
})

// 过滤后的候选列表：只保留相关匹配（非 general）+ 通用（general）
// 隐藏既不相关也不是通用的无效候选
const filteredCandidates = computed(() => {
  const relevant = sortedCandidates.value.filter(c => c.candidate.matchType !== 'general')
  const generals = sortedCandidates.value.filter(c => c.candidate.matchType === 'general')
  // 取最佳相关匹配（最多1个）+ 所有通用
  const bestRelevant = relevant.length > 0 ? [relevant[0]] : []
  return [...bestRelevant, ...generals]
})

// 是否需要显示选择器（多个有效候选时）
const showPluginSelector = computed(() => filteredCandidates.value.length > 1)

// 当前选中插件的 taskOptions（用于显示密码策略）
const taskOptionsForDisplay = computed(() => {
  if (candidates.value.length === 0) return null
  return candidates.value[selectedPluginIndex.value]?.taskOptions ?? null
})

const tasks = ref<EncvTask[]>([])
const loading = ref(false)
const showErrorDetail = ref<Record<string, boolean>>({})
const copiedTaskId = ref<string | null>(null)
const expandedWarningDetail = ref<string | null>(null)
const showNewTaskModal = ref(false)
const newTaskType = ref<TaskType>('encrypt')
const newTaskPath = ref('')
const newTaskTargetPath = ref('')
const sourcePathError = ref('')
const targetPathError = ref('')
const newTaskPassword = ref('')
const newTaskVersion = ref(4)
const newTaskSecondaryPassword = ref('')
let sourceValidateTimer: ReturnType<typeof setTimeout> | null = null
let targetValidateTimer: ReturnType<typeof setTimeout> | null = null
let sourceValidateGeneration = 0
let targetValidateGeneration = 0

async function validatePathExists(path: string): Promise<boolean> {
  try {
    const parentDir = path.substring(0, path.lastIndexOf('/')) || '/'
    const fileName = path.substring(path.lastIndexOf('/') + 1)
    const files = await listFiles(parentDir)
    return files.some(f => f.name === fileName)
  } catch {
    return false
  }
}

async function validateDirExists(path: string): Promise<boolean> {
  try {
    await listFiles(path)
    return true
  } catch {
    return false
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

function getTaskName(task: EncvTask) {
  const parts = task.sourcePath.split('/')
  return parts[parts.length - 1] || task.sourcePath
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

function toggleErrorDetail(taskId: string) {
  showErrorDetail.value[taskId] = !showErrorDetail.value[taskId]
}

function toggleWarningDetail(task: EncvTask) {
  expandedWarningDetail.value = expandedWarningDetail.value === task.id ? null : task.id
}

function formatWarningDetail(detail: string): string {
  try { return JSON.stringify(JSON.parse(detail), null, 2) }
  catch { return detail }
}

async function copyErrorDetail(task: EncvTask) {
  const text = task.errorDetail || task.error || ''
  try {
    await navigator.clipboard.writeText(text)
    copiedTaskId.value = task.id
    showToast({ message: t('tasks.copied'), duration: 1200, color: 'success' })
    setTimeout(() => { if (copiedTaskId.value === task.id) copiedTaskId.value = null }, 2000)
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
    copiedTaskId.value = task.id
    showToast({ message: t('tasks.copied'), duration: 1200, color: 'success' })
    setTimeout(() => { if (copiedTaskId.value === task.id) copiedTaskId.value = null }, 2000)
  }
}

async function handleRefresh(event: CustomEvent) {
  try {
    tasks.value = await getTasks()
  } catch {
    // silent
  }
  ;(event.target as any)?.complete?.()
}

function showNewTaskSheet() {
  newTaskType.value = 'encrypt'
  newTaskPath.value = ''
  newTaskTargetPath.value = ''
  sourcePathError.value = ''
  targetPathError.value = ''
  showNewTaskModal.value = true
}

function validateSourcePath() {
  if (sourceValidateTimer) clearTimeout(sourceValidateTimer)
  sourceValidateTimer = setTimeout(async () => {
    const gen = ++sourceValidateGeneration
    const path = newTaskPath.value.trim()
    if (!path) {
      sourcePathError.value = t('tasks.pathRequired')
    } else if (!path.startsWith('/')) {
      sourcePathError.value = t('tasks.pathMustBeAbsolute')
    } else {
      sourcePathError.value = ''
      const normalized = normalize(path)
      if (normalized) {
        doPredict(normalized, newTaskType.value as 'encrypt' | 'decrypt')
      }
      const exists = await validatePathExists(path)
      if (gen !== sourceValidateGeneration) return
      if (!exists) {
        sourcePathError.value = t('tasks.pathNotFound')
      }
    }
  }, 500)
}

function validateTargetPath() {
  if (targetValidateTimer) clearTimeout(targetValidateTimer)
  targetValidateTimer = setTimeout(async () => {
    const gen = ++targetValidateGeneration
    const path = newTaskTargetPath.value.trim()
    if (!path) {
      targetPathError.value = ''
    } else if (!path.startsWith('/')) {
      targetPathError.value = t('tasks.pathMustBeAbsolute')
    } else {
      targetPathError.value = ''
      const exists = await validateDirExists(path)
      if (gen !== targetValidateGeneration) return
      if (!exists) {
        targetPathError.value = t('tasks.pathNotFound')
      }
    }
  }, 500)
}

async function handleBrowseSource() {
  showNewTaskModal.value = false
  const modal = await modalController.create({
    component: FilePickerModal,
    componentProps: { mode: 'file' as const },
  })
  await modal.present()
  const { data, role } = await modal.onDidDismiss()
  if (role === 'select' && data) {
    newTaskPath.value = data.path
    sourcePathError.value = ''
  }
  showNewTaskModal.value = true
}

async function handleBrowseTarget() {
  showNewTaskModal.value = false
  const modal = await modalController.create({
    component: FilePickerModal,
    componentProps: { mode: 'folder' as const },
  })
  await modal.present()
  const { data, role } = await modal.onDidDismiss()
  if (role === 'select' && data) {
    newTaskTargetPath.value = data.path
    targetPathError.value = ''
  }
  showNewTaskModal.value = true
}

async function handleCreateTask() {
  if (!newTaskPath.value) return
  try {
    await createTask(
      newTaskType.value,
      newTaskPath.value,
      newTaskTargetPath.value || undefined,
      undefined,
      newTaskType.value === 'encrypt' ? newTaskVersion.value : undefined
    )
    showNewTaskModal.value = false
    if (route.query.action) {
      router.replace({ query: {} as Record<string, undefined> })
    }
    showToast({ message: t('tasks.taskCreated'), duration: 1500, color: 'success' })
    await loadTasks()
  } catch {
    showToast({ message: t('tasks.taskCreateFailed'), duration: 2000, color: 'danger' })
  }
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

function processQueryAction() {
  if (route.query.action === 'new') {
    if (route.query.type === 'encrypt' || route.query.type === 'decrypt') {
      newTaskType.value = route.query.type as TaskType
    }
    if (route.query.source) {
      newTaskPath.value = route.query.source as string
      sourcePathError.value = ''
      const normalized = normalize(newTaskPath.value)
      if (normalized) {
        doPredict(normalized, newTaskType.value as 'encrypt' | 'decrypt')
      }
    }
    showNewTaskModal.value = true
    router.replace({ path: '/tabs/tasks', query: {} })
  }
}

// eventBus 直传：Files.vue 长按加密/解密触发（不依赖 router.query）
function handleOpenNewTask(data: { sourcePath: string; taskType: 'encrypt' | 'decrypt' }) {
  newTaskType.value = data.taskType
  newTaskPath.value = data.sourcePath
  newTaskTargetPath.value = ''
  sourcePathError.value = ''
  targetPathError.value = ''
  resetTaskForm()

  const normalized = normalize(data.sourcePath)
  if (normalized) {
    doPredict(normalized, data.taskType)
  }

  showNewTaskModal.value = true
}

onMounted(() => {
  if (config.value?.password) {
    newTaskPassword.value = config.value.password as string
  }

  processQueryAction()
  loadTasks()
  eventBus.on('task:update', onTaskUpdate)
  eventBus.on('task:progress', onTaskProgress)
  eventBus.on('task:created', onTaskCreated)
  eventBus.on('task:completed', onTaskCompleted)

  // 关键：监听 Files.vue 发来的打开新建任务请求
  eventBus.on('open-new-task', handleOpenNewTask)
})

onUnmounted(() => {
  eventBus.off('task:update', onTaskUpdate)
  eventBus.off('task:progress', onTaskProgress)
  eventBus.off('task:created', onTaskCreated)
  eventBus.off('task:completed', onTaskCompleted)
  eventBus.off('open-new-task', handleOpenNewTask)
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

.status-badge {
  margin-right: 8px;
  font-size: 11px;
}

.task-type {
  font-size: 12px;
  color: var(--encv-text-secondary);
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

.task-error-detail {
  color: var(--ion-color-medium);
  font-size: 11px;
  margin-top: 2px;
  cursor: pointer;
  word-break: break-all;
}

.error-detail-row {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 2px;
}

.copy-btn {
  --padding-start: 4px;
  --padding-end: 4px;
  min-width: 28px;
  min-height: 28px;
  font-size: 16px;
}

.error-detail-pre {
  background: var(--ion-color-light);
  border-radius: 6px;
  padding: 8px 10px;
  margin: 4px 0 0;
  font-size: 11px;
  color: var(--ion-text-color);
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 200px;
  overflow-y: auto;
  line-height: 1.5;
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

.browse-btn {
  --padding-start: 8px;
  --padding-end: 8px;
  min-width: 44px;
  min-height: 44px;
}

.plugin-selector {
  margin: 8px 0;
}

.plugin-section {
  margin: 4px 0;
}

.plugin-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  font-size: 12px;
  color: var(--ion-color-medium);
  background: var(--ion-color-step-50, #f8f8f8);
  border-radius: 6px;
  margin: 4px 16px;
}

.hint-icon {
  font-size: 16px;
  flex-shrink: 0;
}
</style>
