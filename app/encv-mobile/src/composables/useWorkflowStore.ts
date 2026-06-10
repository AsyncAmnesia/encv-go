/**
 * 工作流存储层 — localStorage CRUD
 *
 * 职责：
 * - WorkflowDefinition 的持久化（localStorage）
 * - WorkflowRun 运行历史的持久化
 * - 内置模板的注册和加载
 *
 * MVP 阶段纯前端存储，预留后端 API 接口签名。
 */

import { ref } from 'vue'
import type {
  WorkflowDefinition,
  WorkflowRun,
} from '@/lib/workflow/types'
import { WORKFLOW_STORE_KEY, WORKFLOW_RUNS_KEY } from '@/lib/workflow/types'

/** 生成简易 UUID（不需要 crypto 库） */
function generateId(): string {
  return 'wf-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 8)
}

function loadJSON<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key)
    return raw ? (JSON.parse(raw) as T) : fallback
  } catch {
    return fallback
  }
}

function saveJSON<T>(key: string, data: T): void {
  try {
    localStorage.setItem(key, JSON.stringify(data))
  } catch (e) {
    console.warn('[WorkflowStore] Failed to save to localStorage:', e)
  }
}

export function useWorkflowStore() {
  const definitions = ref<WorkflowDefinition[]>(loadJSON(WORKFLOW_STORE_KEY, []))
  const runs = ref<WorkflowRun[]>(loadJSON(WORKFLOW_RUNS_KEY, []))

  // ==================== Definition CRUD ====================

  function createDefinition(partial: Omit<WorkflowDefinition, 'id' | 'createdAt' | 'updatedAt'>): WorkflowDefinition {
    const now = new Date().toISOString()
    const def: WorkflowDefinition = {
      ...partial,
      id: generateId(),
      createdAt: now,
      updatedAt: now,
    }
    definitions.value = [...definitions.value, def]
    persistDefinitions()
    return def
  }

  function updateDefinition(id: string, patch: Partial<Omit<WorkflowDefinition, 'id' | 'createdAt'>>): void {
    definitions.value = definitions.value.map((d) =>
      d.id === id ? { ...d, ...patch, updatedAt: new Date().toISOString() } : d,
    )
    persistDefinitions()
  }

  function deleteDefinition(id: string): void {
    // 内置模板不可删除
    const target = definitions.value.find((d) => d.id === id)
    if (target?.builtin) return
    definitions.value = definitions.value.filter((d) => d.id !== id)
    persistDefinitions()
  }

  function getDefinition(id: string): WorkflowDefinition | undefined {
    return definitions.value.find((d) => d.id === id)
  }

  // ==================== Run History ====================

  function addRun(run: WorkflowRun): void {
    runs.value = [run, ...runs.value].slice(0, 100) // 保留最近 100 条
    persistRuns()
  }

  function updateRun(runId: string, patch: Partial<WorkflowRun>): void {
    runs.value = runs.value.map((r) => (r.id === runId ? { ...r, ...patch } : r))
    persistRuns()
  }

  function getRun(runId: string): WorkflowRun | undefined {
    return runs.value.find((r) => r.id === runId)
  }

  /** 获取某个 workflowDef 的最近 N 次运行 */
  function getRunsForDefinition(defId: string, limit = 10): WorkflowRun[] {
    return runs.value.filter((r) => r.workflowDefId === defId).slice(0, limit)
  }

  function clearRuns(): void {
    runs.value = []
    persistRuns()
  }

  // ==================== 内置模板管理 ====================

  /**
   * 注册内置模板。如果同名模板已存在则跳过。
   */
  function registerBuiltin(template: WorkflowDefinition): void {
    const exists = definitions.value.some(
      (d) => d.builtin && d.name === template.name,
    )
    if (!exists) {
      definitions.value = [...definitions.value, template]
      persistDefinitions()
    }
  }

  /** 批量注册内置模板 */
  function registerBuiltinTemplates(templates: WorkflowDefinition[]): void {
    for (const t of templates) {
      registerBuiltin(t)
    }
  }

  // ==================== 内部持久化 ====================

  function persistDefinitions(): void {
    saveJSON(WORKFLOW_STORE_KEY, definitions.value)
  }

  function persistRuns(): void {
    saveJSON(WORKFLOW_RUNS_KEY, runs.value)
  }

  return {
    definitions,
    runs,
    createDefinition,
    updateDefinition,
    deleteDefinition,
    getDefinition,
    addRun,
    updateRun,
    getRun,
    getRunsForDefinition,
    clearRuns,
    registerBuiltin,
    registerBuiltinTemplates,
  }
}
