<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button default-href="/tabs/settings/devtools"></ion-back-button>
        </ion-buttons>
        <ion-title>{{ t('devtools.automationTests') }}</ion-title>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <!-- Mock 数据管理区 -->
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
          <div class="stat-row">
            <span>{{ t('devtools.fileCount') }}</span><span class="stat-value">{{ mockStats.count }}</span>
          </div>
          <div class="stat-row">
            <span>{{ t('devtools.totalSize') }}</span><span class="stat-value">{{ humanSize(mockStats.totalSize) }}</span>
          </div>
        </div>

        <div v-if="generateProgressText" class="progress-text">
          {{ generateProgressText }}
        </div>
      </ion-list>

      <!-- 自动化测试运行器 -->
      <ion-list>
        <ion-list-header>
          <ion-label>{{ t('devtools.testRunner') }}</ion-label>
        </ion-list-header>
        <p class="section-hint">{{ t('devtools.testRunnerHint') }}</p>

        <ion-item button @click="handleLoadPlugins" :disabled="isLoadingPlugins">
          <ion-icon :icon="syncOutline" slot="start"></ion-icon>
          <ion-label>
            <h3>{{ t('devtools.loadPlugins') }}</h3>
            <p>
              <span v-if="plugins.length > 0">
                {{ plugins.length }} {{ t('devtools.pluginsLoaded') }}
              </span>
              <span v-else>{{ t('devtools.notLoaded') }}</span>
            </p>
          </ion-label>
          <ion-spinner v-if="isLoadingPlugins" slot="end" name="dots"></ion-spinner>
        </ion-item>

        <ion-item>
          <ion-label>
            <h3>{{ t('devtools.testCases') }}</h3>
            <p>
              {{ testCases.length }} {{ t('devtools.testCasesGenerated') }}
            </p>
          </ion-label>
        </ion-item>

        <ion-item button @click="handleRunTests" :disabled="isRunning || testCases.length === 0" detail>
          <ion-icon :icon="playCircleOutline" slot="start" color="success"></ion-icon>
          <ion-label>
            <h3>{{ t('devtools.runAllTests') }}</h3>
            <p>{{ t('devtools.runAllTestsDesc') }}</p>
          </ion-label>
        </ion-item>

        <div v-if="progress.total > 0" class="progress-card">
          <ion-progress-bar :value="progress.completed / progress.total"></ion-progress-bar>
          <div class="progress-stats">
            <span>{{ progress.completed }} / {{ progress.total }}</span>
            <span class="passed">{{ progress.passed }} ✓</span>
            <span class="failed">{{ progress.failed }} ✗</span>
          </div>
        </div>

        <ion-list v-if="results.length > 0" class="results-list">
          <ion-item v-for="(r, idx) in results" :key="idx">
            <ion-icon :icon="getResultIcon(r)" :color="getResultColor(r)" slot="start"></ion-icon>
            <ion-label>
              <h3>{{ r.spec.id }}</h3>
              <p class="result-meta">
                <ion-badge :color="getResultColor(r)" size="small">{{ r.status }}</ion-badge>
                <span v-if="r.durationMs">{{ r.durationMs }}ms</span>
                <span v-if="r.taskId" class="task-id-ref">#{{ r.taskId.slice(0, 6) }}</span>
              </p>
              <p v-if="r.error" class="error-text">{{ r.error }}</p>
            </ion-label>
          </ion-item>
        </ion-list>
      </ion-list>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons, IonBackButton,
  IonContent, IonList, IonListHeader, IonItem, IonLabel, IonIcon,
  IonBadge, IonSpinner, IonProgressBar,
} from '@ionic/vue'
import {
  addCircleOutline, trashOutline, syncOutline, playCircleOutline,
  checkmarkCircleOutline, alertCircleOutline, ellipseOutline,
} from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import { showToast } from '@/composables/useToast'
import {
  useAutomationTests,
  DEFAULT_AUTOMATION_SOURCE,
  type TestCaseResult,
} from '@/composables/useAutomationTests'
import { generateMockFilesViaBackend, resetMockFilesViaBackend } from '@/api/mockGenerator'

const { t } = useI18n()

// 真机 /storage/emulated/0/encv-automation/ — 与 usePathResolver.withSafetyBoundary 的命名空间一致
const mockRoot = computed(() => DEFAULT_AUTOMATION_SOURCE.split('/').slice(0, 5).join('/') + '/')

const {
  plugins, isLoadingPlugins, isRunning, progress, results, testCases, lastError,
  loadPlugins, generateTestCases, runTests,
} = useAutomationTests()

// Mock 数据生成 / 重置
const isGenerating = ref(false)
const isResetting = ref(false)
const mockStats = ref<{ count: number; totalSize: number } | null>(null)
const generateProgressText = ref('')

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
    showToast({ message: `Reset: ${r.removed} files`, color: 'success', duration: 1500 })
  } catch (e) {
    showToast({ message: `Reset failed: ${e instanceof Error ? e.message : e}`, color: 'danger', duration: 2500 })
  } finally {
    isResetting.value = false
  }
}

async function handleLoadPlugins() {
  await loadPlugins()
  if (lastError.value) {
    showToast({ message: `Load plugins failed: ${lastError.value}`, color: 'danger', duration: 2000 })
    return
  }
  // 自动派生测试用例
  generateTestCases({ sourceFile: DEFAULT_AUTOMATION_SOURCE, includeDeprecated: true })
  showToast({ message: `${plugins.value.length} plugins, ${testCases.value.length} cases`, color: 'success', duration: 1500 })
}

async function handleRunTests() {
  if (testCases.value.length === 0) {
    showToast({ message: 'Load plugins first', color: 'warning', duration: 1500 })
    return
  }
  await runTests(testCases.value)
  showToast({
    message: `${progress.value.completed} cases, ${progress.value.passed} ✓ ${progress.value.failed} ✗`,
    color: progress.value.failed > 0 ? 'warning' : 'success',
    duration: 2000,
  })
}

function humanSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

function getResultIcon(r: TestCaseResult): string {
  if (r.status === 'passed' || r.status === 'pending') return checkmarkCircleOutline
  if (r.status === 'failed') return alertCircleOutline
  return ellipseOutline
}
function getResultColor(r: TestCaseResult): string {
  if (r.status === 'passed' || r.status === 'pending') return 'success'
  if (r.status === 'failed') return 'danger'
  return 'medium'
}
</script>

<style scoped>
.section-hint {
  font-size: 12px;
  color: var(--ion-color-medium-shade);
  padding: 8px 16px 4px;
  margin: 0;
}
.mock-root-path {
  font-family: monospace;
  font-size: 12px;
  background: var(--ion-color-light-shade);
  padding: 2px 6px;
  border-radius: 4px;
}
.mock-stats-card {
  margin: 8px 16px;
  padding: 12px 16px;
  background: var(--ion-color-light);
  border-radius: 8px;
}
.stat-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 0;
  font-size: 14px;
}
.stat-value {
  font-weight: 600;
  font-family: monospace;
}
.progress-text {
  font-size: 12px;
  color: var(--ion-color-medium);
  padding: 4px 16px;
  font-family: monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.progress-card {
  margin: 8px 16px;
  padding: 12px 16px;
  background: var(--ion-color-light);
  border-radius: 8px;
}
.progress-stats {
  display: flex;
  justify-content: space-between;
  margin-top: 6px;
  font-size: 13px;
}
.progress-stats .passed { color: var(--ion-color-success); }
.progress-stats .failed { color: var(--ion-color-danger); }
.results-list {
  margin: 8px 0;
}
.result-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  font-size: 12px;
  color: var(--ion-color-medium);
}
.task-id-ref {
  font-family: monospace;
  color: var(--ion-color-primary);
}
.error-text {
  font-size: 12px;
  color: var(--ion-color-danger);
  font-family: monospace;
  word-break: break-all;
  margin-top: 4px;
}
</style>
