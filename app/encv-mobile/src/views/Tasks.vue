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
        <template v-for="item in displayedItems" :key="item.key">
          <!-- 🆕 2026-06-10 修复：自动化测试 / AI agent 任务组折叠 -->
          <!-- 历史：自动化测试一次跑 N 个用例 → 污染 task 列表（用户截图的"浪费屏幕空间"）-->
          <!-- 修复：连续 ≥2 个 triggeredBy != 'user' 的 task → 折叠成 1 张 group card -->
          <!--       点 group card 右侧 chevron 展开/折叠详情 -->
          <ion-item-sliding v-if="item.kind === 'group'">
            <ion-item button detail @click="toggleTaskGroup(item.groupKey!)" :class="['task-group-card', `group-tone-${item.tone}`]">
              <div class="group-icon-bubble" :class="`group-tone-${item.tone}`" slot="start">
                <ion-icon :icon="item.tone === 'ai_agent' ? hardwareChipOutline : cogOutline"></ion-icon>
              </div>
              <ion-label>
                <h2 class="group-title">
                  {{ item.tone === 'ai_agent' ? t('tasks.triggeredBy_ai_agent') : t('tasks.triggeredBy_automation') }}
                  <span class="group-count">· {{ item.tasks.length }} {{ t('tasks.tasksCount') }}</span>
                </h2>
                <p class="card-meta-row group-meta-row">
                  <ion-badge v-if="item.summary.passed > 0" color="success" class="status-badge">
                    <ion-icon :icon="checkmarkCircle" class="badge-icon"></ion-icon>
                    {{ item.summary.passed }}
                  </ion-badge>
                  <ion-badge v-if="item.summary.failed > 0" color="danger" class="status-badge">
                    <ion-icon :icon="closeCircle" class="badge-icon"></ion-icon>
                    {{ item.summary.failed }}
                  </ion-badge>
                  <ion-badge v-if="item.summary.running > 0" color="warning" class="status-badge">
                    <ion-spinner name="dots" class="badge-spinner"></ion-spinner>
                    {{ item.summary.running }}
                  </ion-badge>
                  <ion-badge v-if="item.summary.pending > 0" color="medium" class="status-badge">
                    {{ item.summary.pending }}
                  </ion-badge>
                </p>
                <div class="group-progress-track">
                  <div
                    class="group-progress-fill"
                    :style="{ width: item.summary.percent + '%' }"
                  ></div>
                </div>
                <p class="task-time-info group-time-info">
                  <span class="time-created">{{ formatDateTime(item.summary.latestCreatedAt) }}</span>
                  <span class="group-percent-label">{{ item.summary.percent }}%</span>
                </p>
              </ion-label>
              <ion-button
                slot="end"
                fill="clear"
                size="small"
                @click.stop="toggleTaskGroup(item.groupKey!)"
                :title="isTaskGroupExpanded(item.groupKey!) ? t('tasks.collapse') : t('tasks.expand')"
                class="group-chevron-btn"
              >
                <ion-icon
                  :icon="isTaskGroupExpanded(item.groupKey!) ? chevronBack : chevronForward"
                  slot="icon-only"
                ></ion-icon>
              </ion-button>
            </ion-item>
          </ion-item-sliding>

          <ion-item-sliding v-else>
            <ion-item @click="openTaskDetail(item.task)" button detail>
              <ion-icon
                :icon="getTaskIcon(item.task)"
                :color="getTaskColor(item.task)"
                slot="start"
              ></ion-icon>
              <ion-label>
                <h2>{{ getTaskName(item.task) }}</h2>
                <p class="card-meta-row">
                  <span class="task-id">#{{ item.task.id.slice(0, 6) }}</span>
                  <ion-badge :color="getStatusColor(item.task.status)" class="status-badge">
                    {{ getStatusLabel(item.task.status) }}
                  </ion-badge>
                  <span class="task-type">{{ item.task.type === 'encrypt' ? t('tasks.encrypt') : t('tasks.decrypt') }}</span>
                  <ion-badge v-if="item.task.pluginName" color="primary" class="plugin-badge">
                    {{ item.task.pluginName }}
                  </ion-badge>
                  <ion-badge
                    v-if="getTriggeredBy(item.task.id) !== 'user'"
                    :color="getTriggeredByColor(item.task.id)"
                    class="triggered-by-badge"
                    :title="t('tasks.triggeredBy') + ': ' + t('tasks.triggeredBy_' + getTriggeredBy(item.task.id))"
                  >
                    <ion-icon :icon="getTriggeredByIcon(item.task.id)" class="triggered-by-icon"></ion-icon>
                    {{ t('tasks.triggeredBy_' + getTriggeredBy(item.task.id)) }}
                  </ion-badge>
                </p>
                <p class="task-time-info">
                  <span class="time-created">{{ formatDateTime(item.task.createdAt) }}</span>
                  <span v-if="getTaskDuration(item.task)" class="time-duration">{{ getTaskDuration(item.task) }}</span>
                </p>
                <div v-if="item.task.status === 'running' || item.task.status === 'cancelling'" class="progress-section">
                  <ion-progress-bar
                    :value="item.task.progress / 100"
                    :class="['task-progress', { 'progress-cancelling': item.task.status === 'cancelling' }]"
                  ></ion-progress-bar>
                  <div class="progress-detail">
                    <span v-if="item.task.phase" class="phase-label">{{ getPhaseLabel(item.task.phase) }}</span>
                    <span class="progress-percent">{{ item.task.progress }}%</span>
                    <span v-if="item.task.speed" class="speed-label">{{ item.task.speed }}</span>
                    <span v-if="item.task.eta" class="eta-label">{{ t('tasks.eta') }} {{ item.task.eta }}</span>
                  </div>
                </div>
                <div v-if="item.task.status === 'completed'" class="completed-info">
                  <ion-icon :icon="checkmarkCircle" color="success" class="completed-icon"></ion-icon>
                  <span class="completed-text">{{ t('tasks.phaseCompleted') }}</span>
                  <span v-if="item.task.containerVersion" class="container-version">V{{ item.task.containerVersion }}</span>
                </div>
                <div v-if="item.task.warning" class="task-warning" @click="toggleWarningDetail(item.task)">
                  <ion-icon :icon="warningOutline" class="warning-icon"></ion-icon>
                  <span class="task-warning-text">{{ item.task.warning }}</span>
                </div>
                <div v-if="expandedWarningDetail === item.task.id && item.task.warningDetail" class="task-warning-detail">
                  <pre>{{ formatWarningDetail(item.task.warningDetail) }}</pre>
                </div>
                <p v-if="isPasswordError(item.task)" class="task-error password-error">
                  <ion-icon :icon="lockClosed"></ion-icon>
                  {{ t('tasks.passwordErrorHint') }}
                </p>
                <p v-else-if="item.task.error" class="task-error">{{ item.task.error }}</p>
              </ion-label>
              <ion-button
                v-if="item.task.status === 'running'"
                slot="end"
                fill="clear"
                color="warning"
                size="small"
                @click="cancelTaskById(item.task.id)"
              >
                <ion-icon :icon="closeCircle" slot="icon-only"></ion-icon>
              </ion-button>
              <ion-spinner
                v-if="item.task.status === 'cancelling'"
                slot="end"
                name="crescent"
                color="warning"
                class="cancelling-spinner"
              ></ion-spinner>
            </ion-item>
            <ion-item-options side="end">
              <ion-item-option
                v-if="item.task.status === 'queued'"
                color="warning"
                @click="cancelTaskById(item.task.id)"
              >
                {{ t('tasks.cancel') }}
              </ion-item-option>
              <ion-item-option
                v-if="item.task.status === 'failed'"
                color="primary"
                @click="retryTaskById(item.task.id)"
              >
                {{ t('tasks.retry') }}
              </ion-item-option>
              <ion-item-option
                v-if="item.task.status === 'completed' || item.task.status === 'failed' || item.task.status === 'cancelled'"
                color="danger"
                @click="removeTaskById(item.task.id)"
              >
                {{ t('tasks.remove') }}
              </ion-item-option>
            </ion-item-options>
          </ion-item-sliding>
        </template>
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
import { computed, onMounted, ref } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonContent,
  IonRefresher, IonRefresherContent, IonList, IonItem,
  IonItemSliding, IonItemOptions, IonItemOption, IonIcon,
  IonLabel, IonBadge, IonProgressBar, IonFab, IonFabButton,
  IonSpinner, IonButton, IonButtons, IonSearchbar, IonChip,
  IonPopover, IonCheckbox, alertController, modalController,
} from '@ionic/vue'
import {
  add, closeCircle, checkmarkCircle, timer, sync,
  warningOutline, lockClosed, search, funnel, trashBin,
  extensionPuzzle, swapVertical, chevronDown,
  hardwareChipOutline, cogOutline, person, chevronForward, chevronBack,
} from 'ionicons/icons'
import { useRoute, useRouter } from 'vue-router'
import type { EncvTask, TaskType } from '@/api/encv'
import { clearCompletedTasks } from '@/api/encv'
import { useI18n } from '@/composables/useI18n'
import { formatDateTime } from '@/composables/useDateFormat'
import { showToast } from '@/composables/useToast'
import { useNewTaskModal } from '@/composables/useNewTaskModal'
import { useTasksList } from '@/composables/useTasksList'
import { useTaskEventBridge } from '@/composables/useTaskEventBridge'
import { getTriggeredBy } from '@/composables/useTaskTrigger'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const { openNewTask } = useNewTaskModal()

const {
  tasks, loading, expandedWarningDetail, sortBy,
  showSearch, searchQuery, showFilters,
  filterPlugins, filterTypes, filterStatuses, statusOptions,
  pluginPopoverOpen, typePopoverOpen, statusPopoverOpen,
  pluginPopoverEvent, typePopoverEvent, statusPopoverEvent,
  availablePlugins, hasActiveFilters, hasCompletedTasks, filteredTasks,
  fetchTasks, refresh,
  openPluginPopover, openTypePopover, openStatusPopover,
  togglePluginFilter, toggleTypeFilter, toggleStatusFilter, clearFilters,
  onSearchInput, toggleSort,
  applyTaskUpdate, applyTaskProgress, applyTaskCreated, applyTaskCompleted,
  cancelTaskById, retryTaskById, removeTaskById, clearCompletedWithConfirm,
  getTaskName, getTaskDuration,
  getPluginChipLabel, getTypeChipLabel, getStatusChipLabel, getStatusLabel,
  isPasswordError, toggleWarningDetail, formatWarningDetail,
  getTaskIcon, getTaskColor, getStatusColor, getPhaseLabel,
} = useTasksList()

useTaskEventBridge({
  onUpdate: applyTaskUpdate,
  onProgress: applyTaskProgress,
  onCreate: applyTaskCreated,
  onComplete: applyTaskCompleted,
  onRefresh: fetchTasks,
})

// 任务触发者标签 helpers — Tasks.vue 直接用 useTaskTrigger（因为这是 task 显示的主视图）
function getTriggeredByColor(taskId: string): string {
  const v = getTriggeredBy(taskId)
  return v === 'automation' ? 'primary' : v === 'ai_agent' ? 'secondary' : 'medium'
}
function getTriggeredByIcon(taskId: string): string {
  const v = getTriggeredBy(taskId)
  return v === 'automation' ? cogOutline : v === 'ai_agent' ? hardwareChipOutline : person
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
    if (data.action === 'cancel') await cancelTaskById(data.id)
    else if (data.action === 'retry') await retryTaskById(data.id)
    else if (data.action === 'remove') await removeTaskById(data.id)
  }
}

async function handleRefresh(event: CustomEvent) {
  await refresh()
  ;(event.target as any)?.complete?.()
}

async function handleClearCompleted() {
  const completedCount = await clearCompletedWithConfirm()
  if (!completedCount) return
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
            await fetchTasks()
          } catch {
            showToast({ message: t('tasks.clearFailed'), duration: 2000, color: 'danger' })
          }
        },
      },
    ],
  })
  await alert.present()
}

// 🆕 2026-06-10 修复：自动化测试 / AI agent 任务组折叠
// 历史：useAutomationTests.runTests() 串行 for 循环逐个调 createTask()，
//   一次跑 N 个用例 → 后端 task 列表被 N 张 task 卡片污染
//   （用户截图"浪费屏幕空间"）。
// 修复思路：纯前端 UI 折叠，**不动后端 API**（后端根本没有 GroupID 概念）。
//   - 扫描 filteredTasks，连续 ≥2 个 triggeredBy != 'user' 的 task → 折叠成 1 张 group card
//   - 用户点 chevron 展开/折叠详情（展开时插入 N 张原始 task card）
//   - 单个非用户 task 不折叠（避免 UI 抖动）
//   - 用户手动搜/筛不受影响（filteredTasks 是折叠前数据）
const GROUP_FOLD_THRESHOLD = 2

type DisplayItem =
  | { kind: 'group'; key: string; groupKey: string; tone: 'automation' | 'ai_agent'; tasks: EncvTask[]; summary: { passed: number; failed: number; running: number; pending: number; percent: number; latestCreatedAt: string } }
  | { kind: 'task'; key: string; task: EncvTask }

const expandedGroupKeys = ref<Set<string>>(new Set())

function toggleTaskGroup(key: string) {
  const next = new Set(expandedGroupKeys.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  expandedGroupKeys.value = next
}

function isTaskGroupExpanded(key: string): boolean {
  return expandedGroupKeys.value.has(key)
}

const displayedItems = computed<DisplayItem[]>(() => {
  const result: DisplayItem[] = []
  const tasks = filteredTasks.value

  // 扫描连续非用户 task 区段
  let i = 0
  while (i < tasks.length) {
    const cur = tasks[i]
    const curBy = getTriggeredBy(cur.id)
    if (curBy === 'user') {
      result.push({ kind: 'task', key: cur.id, task: cur })
      i++
      continue
    }

    // 收集连续同 triggeredBy 区段
    const seg: EncvTask[] = [cur]
    let j = i + 1
    while (j < tasks.length && getTriggeredBy(tasks[j].id) === curBy) {
      seg.push(tasks[j])
      j++
    }

    if (seg.length < GROUP_FOLD_THRESHOLD) {
      // 不足阈值 → 全部展开为普通 task
      for (const t of seg) result.push({ kind: 'task', key: t.id, task: t })
    } else {
      // ≥2 个非用户 task → 折叠成 group card
      const groupKey = `${curBy}-${seg[0].id}` // 锚定到第一张 task.id
      const expanded = expandedGroupKeys.value.has(groupKey)
      if (expanded) {
        // 展开：插入 group + N 张原始 task
        result.push(buildGroupItem(groupKey, seg))
        for (const t of seg) result.push({ kind: 'task', key: t.id, task: t })
      } else {
        // 折叠：只插入 group card
        result.push(buildGroupItem(groupKey, seg))
      }
    }
    i = j
  }
  return result
})

function buildGroupItem(groupKey: string, seg: EncvTask[]): DisplayItem {
  let passed = 0, failed = 0, running = 0, pending = 0
  let latest = seg[0]
  for (const t of seg) {
    if (t.status === 'completed') passed++
    else if (t.status === 'failed') failed++
    else if (t.status === 'running' || t.status === 'cancelling') running++
    else pending++
    if (new Date(t.createdAt).getTime() > new Date(latest.createdAt).getTime()) {
      latest = t
    }
  }
  // 🆕 2026-06-10：完成度 = (passed + failed) / total（不算 running/pending）
  // 跟 task 卡片 progress 对齐
  const finished = passed + failed
  const percent = seg.length > 0 ? Math.round((finished / seg.length) * 100) : 0
  // tone: 用第一张 task 的 triggeredBy 决定（折叠段内都是同 triggeredBy）
  const firstBy = getTriggeredBy(seg[0].id)
  const tone: 'automation' | 'ai_agent' = firstBy === 'ai_agent' ? 'ai_agent' : 'automation'
  return {
    kind: 'group',
    key: groupKey,
    groupKey,
    tone,
    tasks: seg,
    summary: { passed, failed, running, pending, percent, latestCreatedAt: latest.createdAt },
  }
}

onMounted(() => {
  fetchTasks()

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

.task-id {
  font-size: 11px;
  font-family: monospace;
  color: var(--encv-text-secondary);
  opacity: 0.7;
  margin-right: 2px;
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

.triggered-by-badge {
  font-size: 10px;
  --padding-start: 6px;
  --padding-end: 6px;
  --padding-top: 2px;
  --padding-bottom: 2px;
  font-weight: 500;
  margin-left: 4px;
}
.triggered-by-icon {
  font-size: 11px;
  margin-right: 3px;
  vertical-align: middle;
}

.task-time-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 2px;
  font-size: 11px;
  color: var(--encv-text-secondary);
}

/* ============================================================
   🆕 2026-06-10 修复：自动化测试 / AI agent 任务组卡片美化
   ============================================================ */
.task-group-card {
  --background: var(--ion-color-light);
  border-left: 4px solid var(--ion-color-primary);
  margin: 8px 0;
  border-radius: 8px;
  overflow: hidden;
  transition: background 0.2s ease;
}
.task-group-card.group-tone-ai_agent {
  border-left-color: var(--ion-color-secondary);
  --background: linear-gradient(135deg, rgba(139, 92, 246, 0.08), rgba(139, 92, 246, 0.02));
}
.task-group-card.group-tone-automation {
  border-left-color: var(--ion-color-primary);
  --background: linear-gradient(135deg, rgba(79, 140, 255, 0.08), rgba(79, 140, 255, 0.02));
}

.group-icon-bubble {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  margin-right: 4px;
}
.group-icon-bubble.group-tone-ai_agent {
  background: var(--ion-color-secondary);
  color: white;
}
.group-icon-bubble.group-tone-automation {
  background: var(--ion-color-primary);
  color: white;
}
.group-icon-bubble ion-icon {
  font-size: 22px;
}

.group-title {
  font-size: 15px;
  font-weight: 600;
  margin: 0 0 4px;
  color: var(--ion-color-dark);
}
.group-count {
  font-size: 13px;
  font-weight: 500;
  color: var(--ion-color-medium-shade);
  margin-left: 4px;
}

.group-meta-row {
  margin: 6px 0;
}
.group-meta-row .status-badge {
  font-size: 11px;
  padding: 3px 8px;
  margin-right: 4px;
  display: inline-flex;
  align-items: center;
  gap: 3px;
}
.badge-icon {
  font-size: 12px;
}
.badge-spinner {
  width: 10px;
  height: 10px;
  --color: currentColor;
}

.group-progress-track {
  height: 6px;
  background: var(--ion-color-step-100, rgba(0, 0, 0, 0.06));
  border-radius: 3px;
  overflow: hidden;
  margin: 6px 0 4px;
}
.group-progress-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--ion-color-primary), var(--ion-color-primary-shade));
  border-radius: 3px;
  transition: width 0.3s ease;
}
.group-tone-ai_agent .group-progress-fill {
  background: linear-gradient(90deg, var(--ion-color-secondary), var(--ion-color-secondary-shade));
}

.group-time-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin: 2px 0 0;
  font-size: 11px;
}
.group-percent-label {
  font-weight: 600;
  color: var(--ion-color-primary);
  font-family: ui-monospace, 'SF Mono', Menlo, Consolas, monospace;
}
.group-tone-ai_agent .group-percent-label {
  color: var(--ion-color-secondary);
}

.group-chevron-btn {
  --color: var(--ion-color-medium-shade);
  margin: 0;
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
  background: var(--ion-color-step-100, #f0f0f0);
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
