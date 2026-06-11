<template>
  <ion-page>
    <ion-header>
      <ion-toolbar>
        <ion-buttons slot="start">
          <ion-back-button default-href="/tabs/settings/devtools"></ion-back-button>
        </ion-buttons>
        <ion-title>{{ t('devtools.webdavTests') }}</ion-title>
        <ion-buttons slot="end">
          <ion-button @click="showHistory = !showHistory" fill="clear" :title="t('devtools.viewHistory')">
            <ion-icon :icon="timeOutline" slot="icon-only"></ion-icon>
          </ion-button>
        </ion-buttons>
      </ion-toolbar>
    </ion-header>

    <ion-content>
      <!-- WebDAV 启用状态 banner -->
      <div v-if="webDavEnabled === false" class="webdav-status-banner webdav-status-disabled">
        <ion-icon :icon="warningOutline" class="banner-icon"></ion-icon>
        <div class="banner-text">
          <strong>WebDAV 服务未启用</strong>
          <p>后端 <code>config.yaml</code> 缺少 <code>webdav.root</code> 配置。所有 18 个测试用例都会因 404 失败。</p>
        </div>
        <ion-button fill="clear" size="small" @click="checkWebDavHealth">
          <ion-icon :icon="sync" slot="icon-only"></ion-icon>
        </ion-button>
      </div>
      <div v-else-if="webDavEnabled === true" class="webdav-status-banner webdav-status-enabled">
        <ion-icon :icon="cloudDoneOutline" class="banner-icon"></ion-icon>
        <div class="banner-text">
          <strong>WebDAV 服务已启用</strong>
          <p>endpoint: <code>{{ baseUrl }}</code></p>
        </div>
      </div>

      <!-- 调试栏：实时统计 -->
      <div class="webdav-debug-bar">
        <span>共 <strong>{{ summary.total }}</strong> 用例</span>
        <span class="sep">·</span>
        <span class="ok"><strong>{{ summary.passed }}</strong> 通过</span>
        <span class="sep">·</span>
        <span class="fail"><strong>{{ summary.failed }}</strong> 失败</span>
        <span class="sep">·</span>
        <span><strong>{{ summary.percent }}%</strong> 完成</span>
        <span class="sep">·</span>
        <span class="base-url" :title="baseUrl">{{ baseUrl }}</span>
      </div>

      <!-- 控制区 -->
      <ion-list>
        <ion-list-header>
          <ion-label>{{ t('devtools.webdavTests') }}</ion-label>
          <ion-badge slot="end" color="primary">{{ WEBDAV_TEST_CASES.length }} {{ t('devtools.cases') }}</ion-badge>
        </ion-list-header>
        <p class="section-hint">{{ t('devtools.webdavTestsHint') }}</p>
        <ion-item button @click="handleRunAll" :disabled="isRunning">
          <ion-icon :icon="playCircle" slot="start" color="primary"></ion-icon>
          <ion-label>
            <h3>{{ t('devtools.runAll') }}</h3>
            <p>{{ t('devtools.runAllDesc') }}</p>
          </ion-label>
          <ion-spinner v-if="isRunning" slot="end" name="dots"></ion-spinner>
        </ion-item>
        <ion-item button @click="handleCancel" :disabled="!isRunning">
          <ion-icon :icon="stopCircle" slot="start" color="danger"></ion-icon>
          <ion-label>
            <h3>{{ t('devtools.cancel') }}</h3>
            <p>{{ t('devtools.cancelDesc') }}</p>
          </ion-label>
        </ion-item>
        <ion-item v-if="!isRunning && summary.total > 0">
          <ion-label>
            <h3>{{ t('devtools.lastResult') }}</h3>
            <p class="last-result-summary">
              <span class="ok">{{ summary.passed }} {{ t('devtools.passed') }}</span>
              ·
              <span class="fail">{{ summary.failed }} {{ t('devtools.failed') }}</span>
              ·
              <span>{{ summary.skipped }} {{ t('devtools.skipped') }}</span>
              <span v-if="currentRunId" class="run-id">#{{ currentRunId.slice(0, 12) }}</span>
            </p>
          </ion-label>
        </ion-item>
      </ion-list>

      <!-- 用例列表（实时状态） -->
      <ion-list>
        <ion-list-header>
          <ion-label>{{ t('devtools.testCases') }}</ion-label>
        </ion-list-header>
        <ion-item
          v-for="(testCase, idx) in WEBDAV_TEST_CASES"
          :key="testCase.id"
          :class="['webdav-test-row', `status-${getStatus(testCase.id)}`, `category-${testCase.category}`]"
        >
          <div class="test-row-index" slot="start" :class="`category-tone-${testCase.category}`">
            <ion-icon :icon="getCategoryIcon(testCase.category)" class="category-icon"></ion-icon>
          </div>
          <ion-label class="test-row-label">
            <h3 class="test-row-name">
              {{ testCase.name }}
              <ion-badge v-if="getResult(testCase.id)?.httpStatus" :color="getStatusBadgeColor(testCase.id)" class="http-status-badge">
                {{ getResult(testCase.id)?.httpStatus }}
              </ion-badge>
            </h3>
            <p class="test-row-desc">{{ testCase.description }}</p>
            <p v-if="getResult(testCase.id)?.error" class="test-row-error">
              <ion-icon :icon="warningOutline" class="error-icon"></ion-icon>
              {{ getResult(testCase.id)?.error }}
            </p>
            <p v-if="getResult(testCase.id)?.durationMs" class="test-row-meta">
              {{ getResult(testCase.id)?.durationMs }}ms
            </p>
          </ion-label>
          <div slot="end" class="test-row-status">
            <ion-icon
              v-if="getStatus(testCase.id) === 'running'"
              :icon="sync"
              class="status-icon status-running"
              spin
            ></ion-icon>
            <ion-icon
              v-else-if="getStatus(testCase.id) === 'passed'"
              :icon="checkmarkCircle"
              class="status-icon status-passed"
            ></ion-icon>
            <ion-icon
              v-else-if="getStatus(testCase.id) === 'failed'"
              :icon="closeCircle"
              class="status-icon status-failed"
            ></ion-icon>
            <ion-icon
              v-else-if="getStatus(testCase.id) === 'skipped'"
              :icon="removeCircle"
              class="status-icon status-skipped"
            ></ion-icon>
            <ion-icon
              v-else
              :icon="ellipseOutline"
              class="status-icon status-pending"
            ></ion-icon>
          </div>
        </ion-item>
      </ion-list>

      <!-- 历史报告弹窗 -->
      <ion-modal :is-open="showHistory" @did-dismiss="showHistory = false">
        <ion-header>
          <ion-toolbar>
            <ion-title>{{ t('devtools.testReports') }}</ion-title>
            <ion-buttons slot="end">
              <ion-button @click="showHistory = false">{{ t('common.close') }}</ion-button>
            </ion-buttons>
          </ion-toolbar>
        </ion-header>
        <ion-content>
          <div class="history-header">
            <ion-button fill="clear" size="small" @click="refreshHistory">
              <ion-icon :icon="sync" slot="start"></ion-icon>
              {{ t('common.refresh') }}
            </ion-button>
            <ion-button fill="clear" size="small" color="danger" @click="handleClearHistory" v-if="historyRuns.length > 0">
              <ion-icon :icon="trashOutline" slot="start"></ion-icon>
              {{ t('devtools.clearHistory') }}
            </ion-button>
          </div>
          <ion-list v-if="historyRuns.length > 0">
            <ion-item
              v-for="run in historyRuns"
              :key="run.id"
              button
              detail
              @click="openRunDetail(run)"
            >
              <ion-icon :icon="cloudDoneOutline" slot="start" :color="run.failed === 0 ? 'success' : 'danger'"></ion-icon>
              <ion-label>
                <h3>
                  {{ formatTime(run.startedAt) }}
                  <ion-badge :color="run.failed === 0 ? 'success' : 'danger'">
                    {{ run.passed }}/{{ run.totalCases }} passed
                  </ion-badge>
                </h3>
                <p>
                  <span v-if="run.failed > 0" class="fail">{{ run.failed }} failed</span>
                  <span v-else class="ok">all passed</span>
                  · {{ formatDuration(run.startedAt, run.completedAt) }}
                  · #{{ run.id.slice(0, 16) }}
                </p>
                <p class="history-base-url">{{ run.baseUrl }}</p>
              </ion-label>
            </ion-item>
          </ion-list>
          <div v-else class="empty-history">
            <ion-icon :icon="archiveOutline" class="empty-icon"></ion-icon>
            <h3>{{ t('devtools.noHistory') }}</h3>
            <p>{{ t('devtools.noHistoryHint') }}</p>
          </div>
        </ion-content>
      </ion-modal>

      <!-- 单个 run 详情弹窗 -->
      <ion-modal :is-open="!!detailRun" @did-dismiss="detailRun = null">
        <ion-header>
          <ion-toolbar>
            <ion-title>{{ detailRun ? formatTime(detailRun.startedAt) : '' }}</ion-title>
            <ion-buttons slot="end">
              <ion-button @click="detailRun = null">{{ t('common.close') }}</ion-button>
            </ion-buttons>
          </ion-toolbar>
        </ion-header>
        <ion-content v-if="detailRun">
          <div class="run-detail-summary">
            <div class="summary-stat ok">
              <div class="stat-value">{{ detailRun.passed }}</div>
              <div class="stat-label">passed</div>
            </div>
            <div class="summary-stat fail">
              <div class="stat-value">{{ detailRun.failed }}</div>
              <div class="stat-label">failed</div>
            </div>
            <div class="summary-stat skip">
              <div class="stat-value">{{ detailRun.skipped }}</div>
              <div class="stat-label">skipped</div>
            </div>
            <div class="summary-stat total">
              <div class="stat-value">{{ detailRun.totalCases }}</div>
              <div class="stat-label">total</div>
            </div>
          </div>
          <div class="run-detail-meta">
            <div><strong>Run ID:</strong> <code>{{ detailRun.id }}</code></div>
            <div><strong>Base URL:</strong> <code>{{ detailRun.baseUrl }}</code></div>
            <div><strong>Duration:</strong> {{ formatDuration(detailRun.startedAt, detailRun.completedAt) }}</div>
          </div>
          <ion-list>
            <ion-list-header>
              <ion-label>用例详情</ion-label>
            </ion-list-header>
            <ion-item
              v-for="r in detailRun.results"
              :key="r.caseId"
              :class="['run-detail-row', `status-${r.status}`]"
            >
              <ion-icon
                :icon="r.status === 'passed' ? checkmarkCircle : r.status === 'failed' ? closeCircle : r.status === 'skipped' ? removeCircle : ellipseOutline"
                slot="start"
                :color="r.status === 'passed' ? 'success' : r.status === 'failed' ? 'danger' : 'medium'"
              ></ion-icon>
              <ion-label>
                <h3>{{ r.caseName }}</h3>
                <p>
                  <ion-badge size="small" :color="r.status === 'passed' ? 'success' : r.status === 'failed' ? 'danger' : 'medium'">
                    {{ r.status }}
                  </ion-badge>
                  <ion-badge v-if="r.httpStatus" size="small" color="medium">{{ r.httpStatus }}</ion-badge>
                  <ion-badge v-if="r.durationMs" size="small" color="medium">{{ r.durationMs }}ms</ion-badge>
                </p>
                <p v-if="r.error" class="run-detail-error">{{ r.error }}</p>
              </ion-label>
            </ion-item>
          </ion-list>
        </ion-content>
      </ion-modal>
    </ion-content>
  </ion-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  IonPage, IonHeader, IonToolbar, IonTitle, IonButtons, IonBackButton,
  IonContent, IonList, IonListHeader, IonItem, IonIcon, IonLabel, IonBadge,
  IonButton, IonSpinner, IonModal, alertController,
} from '@ionic/vue'
import {
  playCircle, stopCircle, timeOutline, sync, checkmarkCircle, closeCircle,
  warningOutline, removeCircle, trashOutline, cloudDoneOutline, archiveOutline,
  ellipseOutline, swapVertical, documentTextOutline, cloudUploadOutline, keyOutline,
} from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import { useWebDavAutomationTests, WEBDAV_TEST_CASES, type WebDavTestRun, type WebDavTestStatus } from '@/composables/useWebDavAutomationTests'
import { showToast } from '@/composables/useToast'

const { t } = useI18n()
const {
  results, summary, isRunning, currentRunId, baseUrl,
  runAllCases, cancelRun, getPersistedRuns, clearPersistedRuns,
} = useWebDavAutomationTests()

const showHistory = ref(false)
const historyRuns = ref<WebDavTestRun[]>([])
const detailRun = ref<WebDavTestRun | null>(null)

onMounted(() => {
  historyRuns.value = getPersistedRuns()
  // 异步检查 webdav 是否启用
  checkWebDavHealth()
})

const webDavEnabled = ref<boolean | null>(null)  // null=检测中, true/false=结果
async function checkWebDavHealth() {
  try {
    const res = await fetch(`${window.location.origin}/webdav/`, { method: 'OPTIONS' })
    webDavEnabled.value = res.status < 500
  } catch {
    webDavEnabled.value = false
  }
}

function getResult(caseId: string) {
  return results.value.find((r) => r.caseId === caseId)
}
function getStatus(caseId: string): WebDavTestStatus {
  return getResult(caseId)?.status ?? 'pending'
}
function getStatusBadgeColor(caseId: string): string {
  const status = getStatus(caseId)
  if (status === 'passed') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'running') return 'warning'
  return 'medium'
}
function getCategoryIcon(category: string): string {
  if (category === 'list') return documentTextOutline
  if (category === 'read') return cloudDoneOutline
  if (category === 'write') return cloudUploadOutline
  if (category === 'meta') return swapVertical
  if (category === 'auth') return keyOutline
  return documentTextOutline
}

function formatTime(iso: string): string {
  if (!iso) return '-'
  const d = new Date(iso)
  return d.toLocaleString('zh-CN', { hour12: false })
}
function formatDuration(start?: string, end?: string): string {
  if (!start || !end) return '-'
  const ms = new Date(end).getTime() - new Date(start).getTime()
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(2)}s`
}

async function handleRunAll() {
  if (isRunning.value) return
  try {
    const run = await runAllCases()
    showToast({
      message: `WebDAV 测试完成: ${run.passed}/${run.totalCases} passed`,
      duration: 3000,
      color: run.failed === 0 ? 'success' : 'warning',
    })
    historyRuns.value = getPersistedRuns()
  } catch (e) {
    showToast({
      message: `WebDAV 测试失败: ${e instanceof Error ? e.message : String(e)}`,
      duration: 3000,
      color: 'danger',
    })
  }
}

function handleCancel() {
  cancelRun()
  showToast({ message: '已取消', duration: 1500, color: 'medium' })
}

function refreshHistory() {
  historyRuns.value = getPersistedRuns()
}

async function handleClearHistory() {
  const alert = await alertController.create({
    header: t('devtools.confirmClearHistory'),
    message: t('devtools.confirmClearHistoryMsg'),
    buttons: [
      { text: t('common.cancel'), role: 'cancel' },
      {
        text: t('common.confirm'),
        role: 'confirm',
        handler: () => {
          clearPersistedRuns()
          historyRuns.value = getPersistedRuns()
          showToast({ message: t('devtools.historyCleared'), duration: 1500, color: 'success' })
        },
      },
    ],
  })
  await alert.present()
}

function openRunDetail(run: WebDavTestRun) {
  detailRun.value = run
}
</script>

<style scoped>
.section-hint {
  font-size: 12px;
  color: var(--encv-text-secondary, #999);
  margin: 0 16px 8px;
  line-height: 1.5;
}

.webdav-debug-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px 8px;
  padding: 8px 14px;
  margin: 8px 12px 4px;
  background: linear-gradient(135deg, rgba(54, 175, 110, 0.06), rgba(79, 140, 255, 0.06));
  border: 1px dashed rgba(54, 175, 110, 0.3);
  border-radius: 6px;
  font-size: 11px;
  color: var(--encv-text-secondary);
  font-family: ui-monospace, 'SF Mono', Menlo, Consolas, monospace;
  line-height: 1.4;
}
.webdav-debug-bar strong {
  color: var(--ion-color-dark);
  font-weight: 700;
}
.webdav-debug-bar .sep {
  opacity: 0.4;
}
.webdav-debug-bar .ok strong { color: var(--ion-color-success); }
.webdav-debug-bar .fail strong { color: var(--ion-color-danger); }
.webdav-debug-bar .base-url {
  font-size: 10px;
  color: var(--ion-color-medium);
  word-break: break-all;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 🆕 2026-06-11 v7：webdav 健康状态 banner */
.webdav-status-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 10px 12px 4px;
  padding: 10px 14px;
  border-radius: 8px;
  font-size: 12px;
  line-height: 1.4;
}
.webdav-status-banner .banner-icon {
  font-size: 24px;
  flex-shrink: 0;
}
.webdav-status-banner .banner-text {
  flex: 1;
  min-width: 0;
}
.webdav-status-banner strong {
  display: block;
  font-size: 13px;
  font-weight: 700;
  margin-bottom: 2px;
  letter-spacing: -0.01em;
}
.webdav-status-banner p {
  margin: 0;
  font-size: 11px;
  color: inherit;
  opacity: 0.9;
}
.webdav-status-banner code {
  font-family: ui-monospace, 'SF Mono', Menlo, Consolas, monospace;
  font-size: 10px;
  background: rgba(0, 0, 0, 0.08);
  padding: 1px 4px;
  border-radius: 3px;
}
.webdav-status-disabled {
  background: linear-gradient(135deg, rgba(255, 87, 34, 0.12), rgba(244, 67, 54, 0.08));
  border: 1px solid rgba(244, 67, 54, 0.25);
  color: #c62828;
}
.webdav-status-disabled .banner-icon {
  color: var(--ion-color-danger);
}
.webdav-status-enabled {
  background: linear-gradient(135deg, rgba(76, 175, 80, 0.1), rgba(54, 175, 110, 0.06));
  border: 1px solid rgba(76, 175, 80, 0.22);
  color: #2e7d32;
}
.webdav-status-enabled .banner-icon {
  color: var(--ion-color-success);
}

.last-result-summary {
  font-size: 13px;
}
.last-result-summary .ok { color: var(--ion-color-success); font-weight: 600; }
.last-result-summary .fail { color: var(--ion-color-danger); font-weight: 600; }
.last-result-summary .run-id {
  margin-left: 8px;
  font-family: ui-monospace, 'SF Mono', Menlo, Consolas, monospace;
  font-size: 11px;
  color: var(--encv-text-secondary);
}

/* 测试 row */
ion-item.webdav-test-row {
  --padding-start: 12px;
  --padding-end: 12px;
  --inner-padding-end: 0;
  --background: transparent;
  --background-hover: rgba(0, 0, 0, 0.02);
  --background-activated: rgba(0, 0, 0, 0.04);
  border-left: 3px solid transparent;
  transition: background 0.15s ease, border-color 0.15s ease;
}
ion-item.webdav-test-row.status-passed { border-left-color: var(--ion-color-success); }
ion-item.webdav-test-row.status-failed { border-left-color: var(--ion-color-danger); }
ion-item.webdav-test-row.status-running {
  border-left-color: var(--ion-color-warning);
  background: rgba(255, 193, 7, 0.04);
}
ion-item.webdav-test-row.status-skipped { border-left-color: var(--ion-color-medium); opacity: 0.7; }

.test-row-index {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  font-size: 14px;
  color: white;
  margin-right: 10px;
}
.test-row-index.category-tone-list { background: linear-gradient(135deg, #5b9dff, #2f7ce0); }
.test-row-index.category-tone-read { background: linear-gradient(135deg, #66bb6a, #388e3c); }
.test-row-index.category-tone-write { background: linear-gradient(135deg, #ff7043, #d84315); }
.test-row-index.category-tone-meta { background: linear-gradient(135deg, #b388ff, #7c4dff); }
.test-row-index.category-tone-auth { background: linear-gradient(135deg, #f06292, #c2185b); }
.category-icon { font-size: 16px; }

.test-row-label { min-width: 0; }
.test-row-name {
  font-size: 14px;
  font-weight: 600;
  margin: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  letter-spacing: -0.01em;
}
.http-status-badge {
  font-family: ui-monospace, monospace;
  font-size: 10px;
  font-weight: 600;
}
.test-row-desc {
  font-size: 11px;
  color: var(--encv-text-secondary);
  margin: 2px 0 0;
  line-height: 1.4;
}
.test-row-error {
  font-size: 11px;
  color: var(--ion-color-danger);
  margin: 4px 0 0;
  font-family: ui-monospace, monospace;
  background: rgba(255, 0, 0, 0.04);
  padding: 4px 6px;
  border-radius: 4px;
  border-left: 2px solid var(--ion-color-danger);
  word-break: break-all;
}
.error-icon {
  font-size: 12px;
  vertical-align: middle;
  margin-right: 3px;
}
.test-row-meta {
  font-size: 10px;
  color: var(--encv-text-secondary);
  margin: 2px 0 0;
  font-family: ui-monospace, monospace;
}

.test-row-status {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  flex-shrink: 0;
}
.status-icon { font-size: 22px; }
.status-passed { color: var(--ion-color-success); }
.status-failed { color: var(--ion-color-danger); }
.status-running { color: var(--ion-color-warning); }
.status-skipped { color: var(--ion-color-medium); }
.status-pending { color: var(--ion-color-medium); opacity: 0.4; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

/* 历史报告 */
.history-header {
  display: flex;
  justify-content: space-between;
  padding: 8px 12px;
  border-bottom: 1px solid var(--ion-color-light-shade, #e0e0e0);
}
.empty-history {
  text-align: center;
  padding: 60px 24px;
  color: var(--encv-text-secondary);
}
.empty-icon {
  font-size: 56px;
  opacity: 0.3;
  margin-bottom: 12px;
}
.empty-history h3 {
  margin: 0 0 8px;
  font-size: 16px;
  color: var(--ion-color-medium);
}
.empty-history p {
  margin: 0;
  font-size: 12px;
}
.history-base-url {
  font-size: 10px !important;
  font-family: ui-monospace, monospace;
  color: var(--encv-text-secondary);
  word-break: break-all;
}

/* Run 详情 */
.run-detail-summary {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
  padding: 16px 12px;
  border-bottom: 1px solid var(--ion-color-light-shade, #e0e0e0);
}
.summary-stat {
  text-align: center;
  padding: 12px 8px;
  border-radius: 8px;
  background: var(--ion-color-light);
}
.summary-stat.ok { background: rgba(54, 175, 110, 0.12); }
.summary-stat.fail { background: rgba(255, 0, 0, 0.08); }
.summary-stat.skip { background: rgba(158, 158, 158, 0.08); }
.summary-stat.total { background: rgba(79, 140, 255, 0.08); }
.stat-value {
  font-size: 24px;
  font-weight: 700;
  color: var(--ion-color-dark);
  font-family: ui-monospace, monospace;
  line-height: 1;
}
.stat-label {
  font-size: 10px;
  color: var(--encv-text-secondary);
  margin-top: 4px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.run-detail-meta {
  padding: 12px 16px;
  font-size: 11px;
  color: var(--encv-text-secondary);
  font-family: ui-monospace, monospace;
  background: var(--ion-color-light);
  border-bottom: 1px solid var(--ion-color-light-shade, #e0e0e0);
}
.run-detail-meta code {
  color: var(--ion-color-primary);
}
ion-item.run-detail-row {
  --padding-start: 12px;
  --inner-padding-end: 0;
}
.run-detail-error {
  font-family: ui-monospace, monospace;
  color: var(--ion-color-danger);
  font-size: 11px;
  background: rgba(255, 0, 0, 0.04);
  padding: 4px 6px;
  border-radius: 4px;
  border-left: 2px solid var(--ion-color-danger);
  word-break: break-all;
}
</style>
