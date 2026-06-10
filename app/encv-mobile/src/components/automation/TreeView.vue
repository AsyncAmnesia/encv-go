<template>
  <div class="tree">
    <!-- 搜索/过滤 -->
    <div class="tree__toolbar">
      <input
        v-model="searchQuery"
        type="text"
        placeholder="Filter steps..."
        class="tree__search"
      />
      <span class="tree__count">{{ filteredJobs.length }} jobs · {{ totalStepCount }} steps</span>
    </div>

    <!-- 树形列表 -->
    <div class="tree__list">
      <div
        v-for="job in filteredJobs"
        :key="job.id"
        class="tree__job"
        :class="{ 'tree__job--expanded': expandedJobs.has(job.id) }"
      >
        <!-- Job 节点 -->
        <button
          class="tree__node tree__node--job"
          @click="toggleJob(job.id)"
        >
          <span class="tree__arrow">{{ expandedJobs.has(job.id) ? '▾' : '▸' }}</span>
          <StepMiniBadge :status="job.status" :show-name="false" />
          <span class="tree__label">{{ jobDisplayName(job.jobDefId) }}</span>
          <span class="tree__meta">
            {{ completedInJob(job) }}/{{ job.steps.length }}
            <span v-if="job.conclusion" class="tree__conclusion">· {{ job.conclusion }}</span>
          </span>
        </button>

        <!-- Steps 子节点 -->
        <div v-if="expandedJobs.has(job.id)" class="tree__children">
          <button
            v-for="step in job.steps"
            :key="step.id"
            class="tree__node tree__node--step"
            :class="{ 'tree__node--selected': selectedStepId === step.id }"
            @click="$emit('select-step', step)"
          >
            <span class="tree__indent"></span>
            <StepMiniBadge :status="step.status" :show-name="false" />
            <span class="tree__label">{{ stepName(step.stepDefId) }}</span>
            <span v-if="step.error" class="tree__error-hint">✕</span>
          </button>
        </div>
      </div>

      <!-- 空状态 -->
      <div v-if="filteredJobs.length === 0" class="tree__empty">
        No jobs found
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import StepMiniBadge from './StepMiniBadge.vue'
import type { WorkflowRun, JobRun, StepRun } from '@/lib/workflow/types'

const props = defineProps<{
  workflowRun: WorkflowRun
  stepNames?: Map<string, string>
  jobDisplayNames?: Map<string, string>
}>()

defineEmits<{
  'select-step': [step: StepRun]
}>()

const searchQuery = ref('')
const expandedJobs = ref<Set<string>>(new Set())
const selectedStepId = ref<string>('')

/** 默认展开所有有失败的 job */
function initExpanded() {
  const set = new Set<string>()
  for (const job of props.workflowRun.jobs) {
    if (job.steps.some((s) => s.status === 'failure' || s.status === 'timed_out')) {
      set.add(job.id)
    }
  }
  expandedJobs.value = set
}
initExpanded()

const filteredJobs = computed(() => {
  if (!searchQuery.value) return props.workflowRun.jobs
  const q = searchQuery.value.toLowerCase()
  return props.workflowRun.jobs.filter((job) => {
    const name = props.jobDisplayNames?.get(job.jobDefId) ?? job.jobDefId
    if (name.toLowerCase().includes(q)) return true
    return job.steps.some((s) => {
      const sn = props.stepNames?.get(s.stepDefId) ?? s.stepDefId
      return sn.toLowerCase().includes(q)
    })
  })
})

const totalStepCount = computed(() =>
  filteredJobs.value.reduce((sum, j) => sum + j.steps.length, 0),
)

function toggleJob(id: string) {
  const next = new Set(expandedJobs.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expandedJobs.value = next
}

function jobDisplayName(defId: string): string {
  return props.jobDisplayNames?.get(defId) ?? defId
}

function stepName(defId: string): string {
  return props.stepNames?.get(defId) ?? defId
}

function completedInJob(job: JobRun): number {
  return job.steps.filter((s) =>
    ['success', 'failure', 'cancelled', 'skipped', 'timed_out'].includes(s.status),
  ).length
}
</script>

<style scoped>
.tree {
  margin: 0 16px 12px;
  font-family: ui-monospace, 'SF Mono', Menlo, Consolas, monospace;
}

.tree__toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
}

.tree__search {
  flex: 1;
  padding: 6px 10px;
  border: 1px solid #D4C9B5;
  border-radius: 3px;
  font-family: inherit;
  font-size: 12px;
  background: #FAF6EE;
  color: #1A1A1A;
}
.tree__search::placeholder { color: #A09580; }

.tree__count {
  font-size: 10px;
  color: #6B5D4C;
  white-space: nowrap;
}

/* 树形列表 */
.tree__list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.tree__job {
  border: 1px solid #EDE5D2;
  border-radius: 3px;
  overflow: hidden;
}

/* 节点 */
.tree__node {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 7px 10px;
  background: none;
  border: none;
  text-align: left;
  cursor: pointer;
  font-family: inherit;
  font-size: 12px;
  color: #1A1A1A;
  transition: background 0.1s ease;
}
.tree__node:hover { background: rgba(43, 58, 103, 0.04); }

.tree__node--job {
  background: #FAF6EE;
  font-weight: 600;
  border-bottom: 1px solid #EDE5D2;
}
.tree__job--expanded .tree__node--job {
  border-bottom-color: transparent;
}

.tree__node--step {
  padding-left: 24px;
  font-weight: 400;
  font-size: 11px;
}
.tree__node--step:hover { background: rgba(43, 58, 103, 0.06); }
.tree__node--selected {
  background: rgba(43, 58, 103, 0.1);
  border-left: 2px solid #2B3A67;
}

.tree__arrow {
  width: 14px;
  font-size: 10px;
  color: #8B7355;
  flex-shrink: 0;
}
.tree__indent {
  width: 14px;
  flex-shrink: 0;
}

.tree__label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tree__meta {
  font-size: 9px;
  color: #8B7355;
  flex-shrink: 0;
}
.tree__conclusion {
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: #6B5D4C;
}

.tree__error-hint {
  color: #8B1E3F;
  font-size: 11px;
}

/* 子节点容器 */
.tree__children {
  display: flex;
  flex-direction: column;
}

.tree__empty {
  padding: 20px;
  text-align: center;
  color: #8B7355;
  font-size: 13px;
}
</style>
