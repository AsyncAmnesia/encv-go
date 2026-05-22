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
              <ion-progress-bar
                v-if="task.status === 'running'"
                :value="task.progress / 100"
                class="task-progress"
              ></ion-progress-bar>
              <p v-if="task.error" class="task-error">{{ task.error }}</p>
            </ion-label>
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
              v-if="task.status === 'running'"
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
                :class="{ 'ion-invalid': sourcePathError }"
                @ionInput="validateSourcePath"
              ></ion-input>
              <ion-button slot="end" fill="clear" @click="handleBrowseSource">
                <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
              </ion-button>
            </ion-item>
            <ion-item v-if="sourcePathError" class="path-error-item">
              <ion-label class="path-error-text">{{ sourcePathError }}</ion-label>
            </ion-item>
            <ion-item>
              <ion-input
                v-model="newTaskTargetPath"
                :label="t('tasks.targetPath')"
                label-placement="stacked"
                :placeholder="t('tasks.targetPathPlaceholder')"
                :class="{ 'ion-invalid': targetPathError }"
                @ionInput="validateTargetPath"
              ></ion-input>
              <ion-button slot="end" fill="clear" @click="handleBrowseTarget">
                <ion-icon :icon="folderOpen" slot="icon-only"></ion-icon>
              </ion-button>
            </ion-item>
            <ion-item v-if="targetPathError" class="path-error-item">
              <ion-label class="path-error-text">{{ targetPathError }}</ion-label>
            </ion-item>
          </ion-list>
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
import { ref, onMounted, onUnmounted } from 'vue'
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
} from 'ionicons/icons'
import {
  getTasks,
  createTask,
  cancelTask,
  retryTask,
} from '@/api/encv'
import type { EncvTask, TaskType, TaskStatus } from '@/api/encv'
import { eventBus } from '@/composables/useEventBus'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import FilePickerModal from '@/components/FilePickerModal.vue'

const { t } = useI18n()

const tasks = ref<EncvTask[]>([])
const loading = ref(false)
const showNewTaskModal = ref(false)
const newTaskType = ref<TaskType>('encrypt')
const newTaskPath = ref('')
const newTaskTargetPath = ref('')
const sourcePathError = ref('')
const targetPathError = ref('')
let sourceValidateTimer: ReturnType<typeof setTimeout> | null = null
let targetValidateTimer: ReturnType<typeof setTimeout> | null = null

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

function getTaskName(task: EncvTask) {
  const parts = task.sourcePath.split('/')
  return parts[parts.length - 1] || task.sourcePath
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
  sourceValidateTimer = setTimeout(() => {
    const path = newTaskPath.value.trim()
    if (!path) {
      sourcePathError.value = t('tasks.pathRequired')
    } else if (!path.startsWith('/')) {
      sourcePathError.value = t('tasks.pathMustBeAbsolute')
    } else {
      sourcePathError.value = ''
    }
  }, 300)
}

function validateTargetPath() {
  if (targetValidateTimer) clearTimeout(targetValidateTimer)
  targetValidateTimer = setTimeout(() => {
    const path = newTaskTargetPath.value.trim()
    if (!path) {
      targetPathError.value = ''
    } else if (!path.startsWith('/')) {
      targetPathError.value = t('tasks.pathMustBeAbsolute')
    } else {
      targetPathError.value = ''
    }
  }, 300)
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
    await createTask(newTaskType.value, newTaskPath.value, newTaskTargetPath.value || undefined)
    showNewTaskModal.value = false
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

function handleRemoveTask(id: string) {
  tasks.value = tasks.value.filter(t => t.id !== id)
}

function onTaskUpdate(data: { id: string; type: string; status: string; progress: number }) {
  const idx = tasks.value.findIndex(t => t.id === data.id)
  if (idx !== -1) {
    tasks.value[idx] = { ...tasks.value[idx], status: data.status as TaskStatus, progress: data.progress }
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

function onTaskCompleted(data: { id: string; error?: string }) {
  const idx = tasks.value.findIndex(t => t.id === data.id)
  if (idx !== -1) {
    tasks.value[idx] = {
      ...tasks.value[idx],
      status: data.error ? 'failed' : 'completed',
      progress: data.error ? tasks.value[idx].progress : 100,
      error: data.error,
    }
  }
}

onMounted(() => {
  loadTasks()
  eventBus.on('task:update', onTaskUpdate)
  eventBus.on('task:created', onTaskCreated)
  eventBus.on('task:completed', onTaskCompleted)
})

onUnmounted(() => {
  eventBus.off('task:update', onTaskUpdate)
  eventBus.off('task:created', onTaskCreated)
  eventBus.off('task:completed', onTaskCompleted)
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

.task-progress {
  margin-top: 6px;
}

.task-error {
  color: var(--ion-color-danger);
  font-size: 12px;
  margin-top: 4px;
}

.path-error-item {
  --min-height: auto;
  --padding-start: 16px;
  --padding-end: 16px;
  --inner-padding-end: 0;
}

.path-error-text {
  color: var(--ion-color-danger);
  font-size: 12px;
}
</style>
