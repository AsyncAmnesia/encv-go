<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button default-href="/tabs/settings/devtools"></ion-back-button>
        </ion-buttons>
        <ion-title>{{ t('devtools.automationTests') }}</ion-title>
        <ion-buttons slot="end">
          <!-- 视图切换 -->
          <button
            class="view-toggle"
            :class="{ 'view-toggle--active': viewMode === 'pipeline' }"
            @click="viewMode = 'pipeline'"
          >Pipeline</button>
          <span class="view-toggle-sep">|</span>
          <button
            class="view-toggle"
            :class="{ 'view-toggle--active': viewMode === 'tree' }"
            @click="viewMode = 'tree'"
          >Tree</button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>

    <ion-content>

      <!-- ========== Mock 数据管理区 ========== -->
      <ion-list>
        <ion-list-header>
          <ion-label>{{ t('devtools.mockDataManager') }}</ion-label>
        </ion-list-header>
        <p class="section-hint">{{ t('devtools.mockDataManagerHint') }}</p>

        <ion-item>
          <ion-label>
            <h3>{{ t('devtools.mockRoot') }}</h3>
            <p><code class="mock-root-path">{{ mockRoot }}</code></p>
          </ion-label>
        </ion-item>

        <ion-item button @click="handleGenerateMock" :disabled="isGenerating">
          <ion-icon :icon="addCircleOutline" slot="start" color="primary"></ion-icon>
          <ion-label>
            <h3>{{ t('devtools.generateMock') }}</h3>
            <p>{{ t('devtools.generateMockDesc') }}</p>
          </ion-label>
          <ion-spinner v-if="isGenerating" slot="end" name="dots"></ion-spinner>
        </ion-item>

        <ion-item button @click="handleResetMock" :disabled="isResetting">
          <ion-icon :icon="trashOutline" slot="start" color="danger"></ion-icon>
          <ion-label>
            <h3>{{ t('devtools.resetMock') }}</h3>
            <p>{{ t('devtools.resetMockDesc') }}</p>
          </ion-label>
          <ion-spinner v-if="isResetting" slot="end" name="dots"></ion-spinner>
        </ion-item>

        <div v-if="mockStats" class="mock-stats-card">
          <div class="stat-row"><span>{{ t('devtools.fileCount') }}</span><span class="stat-value">{{ mockStats.count }}</span></div>
          <div class="stat-row"><span>{{ t('devtools.totalSize') }}</span><span class="stat-value">{{ humanSize(mockStats.totalSize) }}</span></div>
        </div>

        <div v-if="generateProgressText" class="progress-text">{{ generateProgressText }}</div>
      </ion-list>

      <!-- ========== 工作流引擎运行器 ========== -->
      <ion-list>
        <ion-list-header>
          <ion-label>WORKFLOW ENGINE</ion-label>
        </ion-list-header>
        <p class="section-hint">加载插件后自动生成测试工作流定义，支持 DAG 编排、矩阵展开、条件执行</p>

        <ion-item button @click="handleLoadPlugins" :disabled="isLoadingPlugins">
          <ion-icon :icon="syncOutline" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('devtools.loadPlugins') }}</h3>
            <p>
              <span v-if="plugins.length > 0">{{ plugins.length }} {{ t('devtools.pluginsLoaded') }}</span>
              <span v-else>{{ t('devtools.notLoaded') }}</span>
            </p>
          </ion-label>
          <ion-spinner v-if="isLoadingPlugins" slot="end" name="dots"></ion-spinner>
        </ion-item>

        <ion-item>
          <ion-label>
            <h3>测试用例</h3>
            <p>{{ dynamicTestCases.length }} 个用例（{{ pluginCount }} 插件 × 动态笛卡尔积）</p>
          </ion-label>
        </ion-item>

        <!-- 运行控制 -->
        <ion-item
          button
          detail
          @click="handleRunWorkflow"
          :disabled="isRunning || dynamicTestCases.length === 0 || !mockGenerated"
        >
          <ion-icon :icon="playCircleOutline" slot="start" :color="mockGenerated ? 'success' : 'medium'"></ion-icon>
          <ion-label>
            <h3>Run Workflow（DAG 引擎）</h3>
            <p>
              <span v-if="!mockGenerated" style="color: var(--ion-color-danger)">⚠ 请先生成 Mock 数据</span>
              <span v-else>矩阵展开 → 依赖调度 → WS 回调驱动状态转移</span>
            </p>
          </ion-label>
        </ion-item>

        <ion-item v-if="isRunning && currentRun" button detail @click="handleCancel">
          <ion-icon :icon="closeCircleOutline" slot="start" color="danger"></ion-icon>
          <ion-label>
            <h3>Cancel Workflow</h3>
            <p>取消当前运行中的所有 Job / Step</p>
          </ion-label>
        </ion-item>

        <!-- 实时进度 -->
        <div v-if="progress.total > 0" class="progress-card">
          <ion-progress-bar :value="progress.completed / progress.total"></ion-progress-bar>
          <div class="progress-stats">
            <span>{{ progress.completed }} / {{ progress.total }}</span>
            <span class="passed">{{ progress.passed }} ✓</span>
            <span class="failed">{{ progress.failed }} ✗</span>
            <span v-if="progress.pending > 0" class="pending">{{ progress.pending }} ◌</span>
          </div>
        </div>
      </ion-list>

      <!-- ========== 测试报告 ========== -->

      <template v-if="currentRun">
        <!-- 报告头部 -->
        <TestReportHeader
          :run-id="currentRun.id"
          :opened-at="currentRun.createdAt"
          :duration-ms="reportDurationMs"
          :total="totalSteps"
          :passed="successSteps"
          :failed="failedSteps"
          :skipped="0"
          :pending="totalSteps - completedSteps"
          :platform="platform"
        />

        <!-- Pipeline 视图 -->
        <template v-if="viewMode === 'pipeline'">
          <JobPipelineCard
            v-for="job in currentRun.jobs"
            :key="job.id"
            :job="job"
            :step-names="stepNameMap"
            :display-name="getJobDisplayName(job.jobDefId)"
          />
        </template>

        <!-- Tree 视图 -->
        <template v-else>
          <TreeView
            :workflow-run="currentRun"
            :step-names="stepNameMap"
            :job-display-names="jobDisplayNameMap"
            @select-step="onSelectStep"
          />
          <StepDetailPanel
            v-if="selectedStep"
            :step-run="selectedStep"
            :job-run="selectedStepJob!"
          />
        </template>
      </template>

      <!-- 历史运行 -->
      <ion-list v-if="runs.length > 1 && !currentRun">
        <ion-list-header><ion-label>PAST RUNS</ion-label></ion-list-header>
        <ion-item
          v-for="run in runs.slice(1, 11)"
          :key="run.id"
          button
          detail
          @click="currentRun = run"
        >
          <ion-label>
            <h3>{{ run.id.slice(4, 16) }}...</h3>
            <p>{{ run.status }} · {{ run.jobs.length }} jobs · {{ formatTime(run.createdAt) }}</p>
          </ion-label>
          <StepMiniBadge :status="run.status === 'running' ? 'queued' : run.status" :show-name="false" slot="end" />
        </ion-item>
      </ion-list>

    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons, IonBackButton,
  IonContent, IonList, IonListHeader, IonItem, IonLabel, IonIcon,
  IonSpinner, IonProgressBar,
} from '@ionic/vue'
import {
  addCircleOutline, trashOutline, syncOutline, playCircleOutline, closeCircleOutline,
} from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import {
  fetchPlugins,
  type PluginMeta,
} from '@/api/encv'
import { generateMockFilesViaBackend, resetMockFilesViaBackend } from '@/api/mockGenerator'
import { DEFAULT_AUTOMATION_SOURCE } from '@/composables/useAutomationTests'
import { useWorkflowEngine } from '@/composables/useWorkflowEngine'
import type { WorkflowDefinition, WorkflowRun, JobRun, StepRun, StepDefinition } from '@/lib/workflow/types'
import TestReportHeader from '@/components/automation/TestReportHeader.vue'
import StepMiniBadge from '@/components/automation/StepMiniBadge.vue'
import JobPipelineCard from '@/components/automation/JobPipelineCard.vue'
import TreeView from '@/components/automation/TreeView.vue'
import StepDetailPanel from '@/components/automation/StepDetailPanel.vue'

const { t } = useI18n()

// ---- Mock 数据 ----
const mockRoot = computed(() => DEFAULT_AUTOMATION_SOURCE.split('/').slice(0, 5).join('/') + '/')
const isGenerating = ref(false)
const isResetting = ref(false)
const mockStats = ref<{ count: number; totalSize: number } | null>(null)
const generateProgressText = ref('')
const mockGenerated = ref(false)

// ---- 插件 & 用例 ----
const plugins = ref<PluginMeta[]>([])
const isLoadingPlugins = ref(false)
const dynamicTestCases = ref<any[]>([])
const pluginCount = computed(() => plugins.value.length)

// ---- 工作流引擎 ----
const engine = useWorkflowEngine()
const {
  definitions: wfDefs,
  runs,
  currentRun,
  isRunning,
  totalSteps,
  completedSteps,
  successSteps,
  failedSteps,
  startListening: wsStart,
  stopListening: wsStop,
} = engine

const viewMode = ref<'pipeline' | 'tree'>('pipeline')
const selectedStep = ref<StepRun | null>(null)
const _tickNow = ref(Date.now())
let tickHandle: ReturnType<typeof setInterval> | null = null

const platform = computed(() => {
  if (typeof navigator === 'undefined') return 'node'
  const ua = navigator.userAgent || ''
  if (/android/i.test(ua)) return 'android'
  if (/iphone|ipad|ipod/i.test(ua)) return 'ios'
  return 'web'
})

const reportDurationMs = computed(() => {
  if (!currentRun.value) return 0
  if (isRunning.value) return _tickNow.value - (currentRun.value.startedAt ? new Date(currentRun.value.startedAt).getTime() : Date.now())
  return currentRun.value.durationMs ?? 0
})

// 兼容旧接口名
const progress = computed(() => ({
  total: totalSteps.value,
  completed: completedSteps.value,
  passed: successSteps.value,
  failed: failedSteps.value,
  pending: Math.max(0, totalSteps.value - completedSteps.value),
}))

/** 从当前运行的 workflow definition 构建 step 名映射 */
const stepNameMap = computed(() => {
  const map = new Map<string, string>()
  const def = currentRun.value
    ? wfDefs.value.find((d: WorkflowDefinition) => d.id === currentRun.value!.workflowDefId)
    : null
  if (def) {
    for (const job of def.jobs) {
      for (const step of job.steps) {
        map.set(step.id, step.name)
      }
    }
  }
  // 如果没有 definition（历史运行），从 stepDefId 推断名称
  if (map.size === 0 && currentRun.value) {
    for (const job of currentRun.value.jobs) {
      for (const step of job.steps) {
        if (!map.has(step.stepDefId)) {
          map.set(step.stepDefId, step.stepDefId)
        }
      }
    }
  }
  return map
})

const jobDisplayNameMap = computed(() => {
  const map = new Map<string, string>()
  const def = currentRun.value
    ? wfDefs.value.find((d: WorkflowDefinition) => d.id === currentRun.value!.workflowDefId)
    : null
  if (def) {
    for (const job of def.jobs) map.set(job.id, job.name)
  }
  return map
})

function getJobDisplayName(jobDefId: string): string {
  return jobDisplayNameMap.value.get(jobDefId) ?? jobDefId
}

function findJobForStep(run: WorkflowRun, step: StepRun): JobRun | undefined {
  return run.jobs.find((j: JobRun) => j.steps.some((s: StepRun) => s.id === step.id))
}

const selectedStepJob = computed(() =>
  currentRun.value && selectedStep.value
    ? findJobForStep(currentRun.value, selectedStep.value)
    : null,
)

// ---- Handlers ----

async function handleGenerateMock() {
  if (isGenerating.value) return
  isGenerating.value = true
  generateProgressText.value = ''
  mockStats.value = null
  let lastCount = 0
  let lastSize = 0
  try {
    const result = await generateMockFilesViaBackend({
      root: mockRoot.value,
      type: 'all',
      onProgress: (p) => {
        lastCount++
        lastSize += p.size
        generateProgressText.value = `(${lastCount}) ${p.relativePath}`
      },
    })
    mockStats.value = { count: result.count || lastCount, totalSize: result.totalSize || lastSize }
    mockGenerated.value = true
    showToast({ message: `${t('devtools.generateMock')}: ${mockStats.value.count}`, color: 'success', duration: 1500 })
  } catch (e) {
    showToast({ message: `${t('devtools.generateMock')} failed: ${e instanceof Error ? e.message : e}`, color: 'danger', duration: 2500 })
  } finally {
    isGenerating.value = false
    generateProgressText.value = ''
  }
}

async function handleResetMock() {
  if (isResetting.value) return
  isResetting.value = true
  try {
    const r = await resetMockFilesViaBackend(mockRoot.value)
    mockStats.value = null
    mockGenerated.value = false
    showToast({ message: `Reset: ${r.removed} files`, color: 'success', duration: 1500 })
  } catch (e) {
    showToast({ message: `Reset failed: ${e instanceof Error ? e.message : e}`, color: 'danger', duration: 2500 })
  } finally {
    isResetting.value = false
  }
}

async function handleLoadPlugins() {
  isLoadingPlugins.value = true
  try {
    plugins.value = await fetchPlugins()
    // 自动构建动态测试用例 + 工作流定义
    buildDynamicWorkflow()
    showToast({ message: `${plugins.value.length} plugins, ${dynamicTestCases.value.length} cases`, color: 'success', duration: 1500 })
  } catch (e) {
    showToast({ message: `Load plugins failed: ${e instanceof Error ? e.message : e}`, color: 'danger', duration: 2000 })
  } finally {
    isLoadingPlugins.value = false
  }
}

/**
 * 根据已加载的插件，动态构建 WorkflowDefinition。
 * 这是核心：把旧的「硬编码笛卡尔积」升级为 DAG 工作流定义，
 * 然后通过引擎的 runWorkflow() 执行。
 */
function buildDynamicWorkflow(): void {
  if (plugins.value.length === 0) {
    dynamicTestCases.value = []
    return
  }

  const steps: StepDefinition[] = []

  for (const plugin of plugins.value) {
    const opts = plugin.taskOptions
    if (!opts) continue

    const versions = opts.supportVersionSelect && opts.supportedVersions
      ? opts.supportedVersions
      : [opts.defaultVersion]

    for (const taskType of ['encrypt', 'decrypt'] as const) {
      for (const version of versions) {
        const isV4 = version === 4
        const cipherModes: Array<number | undefined> = isV4 && taskType === 'encrypt' ? [0, 1] : [undefined]
        const compressionModes: Array<'none' | 'zstd' | undefined> = isV4 && taskType === 'encrypt' ? ['none', 'zstd'] : [undefined]

        for (const cipherMode of cipherModes) {
          for (const compressionMode of compressionModes) {
            const idParts = [
              plugin.name, taskType, `v${version}`,
              cipherMode !== undefined ? `c${cipherMode}` : '',
              compressionMode || '',
            ].filter(Boolean)
            const stepId = idParts.join('-')

            // 构建可读名称：包含加密选型参数
            const nameParts = [plugin.name, taskType.toUpperCase()]
            if (cipherMode !== undefined) nameParts.push(`AES-${cipherMode === 0 ? '128' : '256'}-GCM`)
            if (compressionMode) nameParts.push(compressionMode.toUpperCase())

            steps.push({
              id: stepId,
              name: nameParts.join(' · '),
              action: {
                type: 'encv_task',
                taskType,
                pluginName: plugin.name,
                params: {
                  sourcePath: DEFAULT_AUTOMATION_SOURCE,
                  password: 'automation-test-pwd',
                  version,
                  cipherMode,
                  compressionMode: compressionMode ?? undefined,
                },
              },
              // 解密任务依赖加密成功（通过 if 条件）
              ...(taskType === 'decrypt' ? { if: { op: 'always' } as any } : {}),
            })

            dynamicTestCases.value.push({
              id: stepId,
              pluginName: plugin.name,
              taskType,
              version,
              cipherMode,
              compressionMode,
            })
          }
        }
      }
    }
  }

  // 构建或更新工作流定义
  const existingIdx = wfDefs.value.findIndex((d) => d.id === 'dynamic-auto-test')
  const wfDef: WorkflowDefinition = {
    id: 'dynamic-auto-test',
    name: '自动化测试套件（动态）',
    description: `基于已加载插件 (${plugins.value.length}) 动态生成，矩阵展开 plugin × version × cipher × compression`,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    trigger: 'manual',
    env: { PASSWORD: 'automation-test-pwd' },
    jobs: [
      {
        id: 'test-all',
        name: '全量测试（并行）',
        strategy: { type: 'parallel', max: 5 }, // 并行提交最多 5 个
        steps,
      },
    ],
  }

  if (existingIdx >= 0) {
    engine.updateDefinition('dynamic-auto-test', wfDef)
  } else {
    engine.createDefinition(wfDef)
  }
}

async function handleRunWorkflow() {
  if (isRunning.value || dynamicTestCases.value.length === 0) return
  if (!mockGenerated.value) {
    showToast({ message: '请先生成 Mock 数据！', color: 'warning', duration: 2000 })
    return
  }

  try {
    await engine.runWorkflow('dynamic-auto-test', 'automation')
    showToast({
      message: `Workflow started: ${dynamicTestCases.value.length} steps`,
      color: 'success',
      duration: 1500,
    })
  } catch (e) {
    showToast({ message: `${e instanceof Error ? e.message : e}`, color: 'danger', duration: 2500 })
  }
}

function handleCancel() {
  engine.cancelCurrentRun()
  showToast({ message: 'Workflow cancelled', color: 'warning', duration: 1500 })
}

function onSelectStep(step: StepRun) {
  selectedStep.value = step
}

function formatTime(iso: string): string {
  try { return new Date(iso).toLocaleTimeString() } catch { return iso }
}

function humanSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

onMounted(() => {
  tickHandle = setInterval(() => { _tickNow.value = Date.now() }, 1000)
  wsStart()
})
onUnmounted(() => {
  if (tickHandle) clearInterval(tickHandle)
  wsStop()
})
</script>

<style scoped>
.section-hint { font-size: 12px; color: var(--ion-color-medium-shade); padding: 8px 16px 4px; margin: 0; }
.mock-root-path { font-family: monospace; font-size: 12px; background: var(--ion-color-light-shade); padding: 2px 6px; border-radius: 4px; }
.mock-stats-card { margin: 8px 16px; padding: 12px 16px; background: var(--ion-color-light); border-radius: 8px; }
.stat-row { display: flex; justify-content: space-between; align-items: center; padding: 4px 0; font-size: 14px; }
.stat-value { font-weight: 600; font-family: monospace; }
.progress-text { font-size: 12px; color: var(--ion-color-medium); padding: 4px 16px; font-family: monospace; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.progress-card { margin: 8px 16px; padding: 12px 16px; background: var(--ion-color-light); border-radius: 8px; }
.progress-stats { display: flex; justify-content: space-between; margin-top: 6px; font-size: 13px; }
.progress-stats .passed { color: var(--ion-color-success); }
.progress-stats .failed { color: var(--ion-color-danger); }
.progress-stats .pending { color: #B8860B; }

.view-toggle { font-family: ui-monospace, 'SF Mono', Menlo, Consolas, monospace; font-size: 11px; background: none; border: none; color: #6B5D4C; cursor: pointer; padding: 2px 6px; border-radius: 3px; }
.view-toggle--active { background: #1A1A1A; color: #F4EFE6; }
.view-toggle-sep { color: #C9BBA1; }
</style>
