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
 * 把 ext 映射到 mock 目录分类
 *   mp4/mkv/avi/mov → 'video'
 *   mp3/flac/ogg/m4a/wav → 'audio'
 *   png/jpg/jpeg/gif/webp → 'image'
 *   pdf → 'pdf'
 *   doc/docx/xls/xlsx/ppt/pptx → 'wps'
 *   txt/md → 'text'
 *   encv → 'alist-encrypted'
 */
function categoryForExt(ext: string): string {
  const e = ext.toLowerCase().replace(/^\./, '')
  if (['mp4', 'mkv', 'avi', 'mov', 'webm', 'flv', 'wmv'].includes(e)) return 'video'
  if (['mp3', 'flac', 'ogg', 'm4a', 'wav', 'aac', 'opus'].includes(e)) return 'audio'
  if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'tiff'].includes(e)) return 'image'
  if (['pdf'].includes(e)) return 'pdf'
  if (['doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx'].includes(e)) return 'wps'
  if (['txt', 'md', 'rtf', 'log'].includes(e)) return 'text'
  if (['encv', 'ae'].includes(e)) return 'alist-encrypted'
  return 'misc'
}

/**
 * 根据已加载的插件，动态构建 WorkflowDefinition。
 * 这是核心：把旧的「硬编码笛卡尔积」升级为 DAG 工作流定义，
 * 然后通过引擎的 runWorkflow() 执行。
 *
 * 🆕 2026-06-10 重构：彻底消除硬编码
 *   - 源文件按 plugin.supportedExtensions 选（不再一刀切 sample.mp4）
 *   - 加密选项遍历 plugin.taskOptions.ExtraFields（不再硬编码 v4 cipherMode/compressionMode）
 *   - v2/v3 仍走回归测试
 *   - AI agent 等非 V4 插件不参与加密选项笛卡尔积
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

    // ====== 修 #4：按 plugin.supportedExtensions 选源文件 ======
    const supportedExts = plugin.supportedExtensions ?? []
    if (supportedExts.length === 0) continue
    // 简单策略：每个 plugin 取 supportedExts[0]（避免笛卡尔积爆炸）
    // 复杂策略：可遍历所有 ext 展开，但会让 case 数 × N
    const sourceExt = supportedExts[0]
    const sourcePath = `${mockRoot.value}01-plain-media/${categoryForExt(sourceExt)}/sample.${sourceExt}`

    const versions: number[] = opts.supportVersionSelect && opts.supportedVersions
      ? opts.supportedVersions
      : [opts.defaultVersion]

    // ====== 修 #5：遍历 plugin.taskOptions.ExtraFields ======
    // 只对 type='select' 且 Options 长度 > 1 的字段展开笛卡尔积
    // type='bool' 单独处理（true/false）
    // type='string' / 'number' / 'password' 不展开（用 default）
    const selectFields: { field: any; values: string[] }[] = []
    const boolFields: { field: any }[] = []
    for (const f of opts.extraFields ?? []) {
      // 跳过 decrypt 专用 / encrypt 专用 field（按 Condition 过滤）
      // Condition='encrypt' → 仅 encrypt 时展开
      if (f.condition === 'encrypt' || f.condition === 'decrypt') {
        // 暂存，下面按 taskType 过滤
      }
      if (f.type === 'select' && Array.isArray(f.options) && f.options.length > 1) {
        selectFields.push({ field: f, values: f.options })
      } else if (f.type === 'bool') {
        boolFields.push({ field: f })
      }
    }

    for (const taskType of ['encrypt', 'decrypt'] as const) {
      for (const version of versions) {
        // 按 taskType 过滤 ExtraFields
        const taskSelectFields = selectFields.filter(
          (sf) => !sf.field.condition || sf.field.condition === taskType,
        )
        const taskBoolFields = boolFields.filter(
          (bf) => !bf.field.condition || bf.field.condition === taskType,
        )

        // 笛卡尔积展开 select 字段（每个字段至少取 1 个值，所以是 product(len(values))）
        const selectCombos = cartesianExpand(
          taskSelectFields.map((sf) => sf.values),
        )
        // 笛卡尔积展开 bool 字段（2^N）
        const boolCombos: boolean[][] = []
        if (taskBoolFields.length === 0) {
          boolCombos.push([])
        } else {
          const n = taskBoolFields.length
          for (let mask = 0; mask < 1 << n; mask++) {
            boolCombos.push(Array.from({ length: n }, (_, i) => Boolean(mask & (1 << i))))
          }
        }

        for (const selectCombo of selectCombos) {
          for (const boolCombo of boolCombos) {
            const extraFields: Record<string, string> = {}

            // 应用 select 字段值
            taskSelectFields.forEach((sf, i) => {
              const val = selectCombo[i]
              if (val !== undefined) extraFields[sf.field.key] = val
            })

            // 应用 bool 字段值（"true" / "false"）
            taskBoolFields.forEach((bf, i) => {
              extraFields[bf.field.key] = boolCombo[i] ? 'true' : 'false'
            })

            // 构建 step id / name
            const idParts = [
              plugin.name,
              taskType,
              `v${version}`,
              sourceExt,
            ]
            // 加密选项加入 id（让每个组合都是独立 step）
            for (const sf of taskSelectFields) {
              const v = extraFields[sf.field.key]
              if (v) idParts.push(`${sf.field.key}=${v}`)
            }
            for (const bf of taskBoolFields) {
              const v = extraFields[bf.field.key]
              if (v) idParts.push(`${bf.field.key}=${v}`)
            }
            const stepId = idParts.join('|')

            const nameParts: string[] = [plugin.name, taskType.toUpperCase(), `v${version}`, sourceExt]
            for (const sf of taskSelectFields) {
              const v = extraFields[sf.field.key]
              if (v) {
                const label = sf.field.optionLabels?.[v] ?? v
                nameParts.push(`${sf.field.key}=${label}`)
              }
            }
            for (const bf of taskBoolFields) {
              const v = extraFields[bf.field.key]
              if (v) nameParts.push(`${bf.field.key}=${v}`)
            }

            steps.push({
              id: stepId,
              name: nameParts.join(' · '),
              action: {
                type: 'encv_task',
                taskType,
                pluginName: plugin.name,
                params: {
                  sourcePath,
                  password: 'automation-test-pwd',
                  version,
                  extraFields: Object.keys(extraFields).length > 0 ? extraFields : undefined,
                },
              },
            })

            dynamicTestCases.value.push({
              id: stepId,
              pluginName: plugin.name,
              taskType,
              version,
              sourcePath,
              sourceExt,
              extraFields: { ...extraFields },
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
    description: `${plugins.value.length} 插件 × 源扩展名 × 版本 × 加密选项笛卡尔积（遍历 ExtraFields）`,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    trigger: 'manual',
    env: { PASSWORD: 'automation-test-pwd' },
    jobs: [
      {
        id: 'test-all',
        name: '全量测试（并行）',
        strategy: { type: 'parallel', max: 5 },
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

/** 笛卡尔积展开：输入 [[1,2],[a,b,c]] → 输出 [[1,a],[1,b],[1,c],[2,a],[2,b],[2,c]] */
function cartesianExpand(arrays: string[][]): string[][] {
  if (arrays.length === 0) return [[]]
  if (arrays.some((a) => a.length === 0)) return [[]]
  return arrays.reduce<string[][]>(
    (acc, curr) => acc.flatMap((a) => curr.map((c) => [...a, c])),
    [[]],
  )
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
