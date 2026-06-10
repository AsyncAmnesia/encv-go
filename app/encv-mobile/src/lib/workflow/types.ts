/**
 * DAG 工作流引擎 — 核心类型定义
 *
 * 层次结构：
 *   WorkflowDefinition (静态定义)
 *   └── WorkflowRun (一次运行实例)
 *       └── JobRun[] (作业运行实例)
 *           └── StepRun[] (步骤运行实例，每个对应一个 EncvTask)
 */

import type { TaskType } from '@/api/encv'
import type { TriggeredBy } from '@/composables/useTaskTrigger'
import type { ErrorAnalysis } from '@/composables/useErrorAnalyzer'

// ==================== 状态枚举 ====================

export type StepStatus =
  | 'pending'
  | 'submitted'
  | 'queued'
  | 'running'
  | 'success'
  | 'failure'
  | 'cancelled'
  | 'skipped'
  | 'timed_out'

export type JobStatus = StepStatus

export type WorkflowStatus = 'pending' | 'running' | 'success' | 'failure' | 'cancelled'

export type JobConclusion = 'success' | 'failure' | 'skipped' | 'cancelled'

/** 终态集合 */
const TERMINAL_STEP_STATUS: Set<StepStatus> = new Set([
  'success', 'failure', 'cancelled', 'skipped', 'timed_out',
])

export function isTerminalStep(s: StepStatus): boolean {
  return TERMINAL_STEP_STATUS.has(s)
}

// ==================== 条件表达式 ====================

export interface ConditionAlways {
  op: 'always'
}
export interface ConditionSuccess {
  op: 'success'
}
export interface ConditionFailure {
  op: 'failure'
}
export interface ConditionEq {
  op: 'eq'
  left: string
  right: string
}
export interface ConditionNeq {
  op: 'neq'
  left: string
  right: string
}
export interface ConditionAnd {
  op: 'and'
  children: ConditionExpr[]
}
export interface ConditionOr {
  op: 'or'
  children: ConditionExpr[]
}
export interface ConditionNot {
  op: 'not'
  child: ConditionExpr
}

export type ConditionExpr =
  | ConditionAlways
  | ConditionSuccess
  | ConditionFailure
  | ConditionEq
  | ConditionNeq
  | ConditionAnd
  | ConditionOr
  | ConditionNot

// ==================== 动作规格 ====================

export interface EncvTaskActionParams {
  sourcePath?: string
  targetPath?: string
  password?: string
  version?: number
  cipherMode?: number
  compressionMode?: 'none' | 'zstd'
  secondaryPassword?: string
}

export interface EncvTaskActionSpec {
  type: 'encv_task'
  taskType: TaskType
  pluginName: string
  params: EncvTaskActionParams
}

// 未来扩展占位（MVP 不实现）
export interface ShellActionSpec {
  type: 'shell'
  command: string
}
export interface HttpRequestActionSpec {
  type: 'http_request'
  method: string
  url: string
  body?: Record<string, unknown>
}

export type ActionSpec =
  | EncvTaskActionSpec
  | ShellActionSpec
  | HttpRequestActionSpec

// ==================== 静态定义 ====================

export interface MatrixStrategy {
  type: 'matrix'
  axes: Record<string, string[]>
}
export interface ParallelStrategy {
  type: 'parallel'
  max: number
}
export interface SequentialStrategy {
  type: 'sequential'
}

export type JobStrategy = MatrixStrategy | ParallelStrategy | SequentialStrategy

export interface ConcurrencyMaxParallel {
  maxParallel: number
}
export interface ConcurrencyGroupExclusive {
  group: string
  cancelInProgress: boolean
}

export type ConcurrencyConfig = ConcurrencyMaxParallel | ConcurrencyGroupExclusive

export interface StepDefinition {
  id: string
  name: string
  action: ActionSpec
  if?: ConditionExpr
  continueOnError?: boolean
  timeoutSeconds?: number
}

export interface JobDefinition {
  id: string
  name: string
  needs?: string[]
  if?: ConditionExpr
  strategy?: JobStrategy
  timeoutMinutes?: number
  steps: StepDefinition[]
}

export interface WorkflowDefinition {
  id: string
  name: string
  description?: string
  createdAt: string
  updatedAt: string
  trigger: 'manual' | 'on_event' | 'schedule'
  env?: Record<string, string>
  concurrency?: ConcurrencyConfig
  jobs: JobDefinition[]
  /** 是否为内置模板（不可删除） */
  builtin?: boolean
}

// ==================== 运行时实例 ====================

export interface StepRun {
  id: string
  stepDefId: string
  status: StepStatus
  startedAt?: string
  completedAt?: string
  durationMs?: number
  taskId?: string
  error?: string
  errorAnalysis?: ErrorAnalysis
  output?: Record<string, unknown>
  /** matrix 展开时的变量快照 */
  matrixVars?: Record<string, string>
}

export interface JobRun {
  id: string
  jobDefId: string
  status: JobStatus
  conclusion?: JobConclusion
  startedAt?: string
  completedAt?: string
  durationMs?: number
  steps: StepRun[]
  matrixVars?: Record<string, string>
}

export interface WorkflowRun {
  id: string
  workflowDefId: string
  status: WorkflowStatus
  triggeredBy: TriggeredBy
  createdAt: string
  startedAt?: string
  completedAt?: string
  durationMs?: number
  jobs: JobRun[]
}

// ==================== 存储键名 ====================

export const WORKFLOW_STORE_KEY = 'encv-workflow-definitions'
export const WORKFLOW_RUNS_KEY = 'encv-workflow-runs'

// Re-export from sub-modules for convenience
export type { MatrixBinding } from './matrixExpander'
