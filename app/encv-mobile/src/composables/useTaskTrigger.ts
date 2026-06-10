/**
 * 任务触发者标签 + workflow run 关联 — localStorage 持久化
 *
 * 用前端 localStorage 维护 taskId → {triggeredBy, runId?} 映射。
 * 不侵入 Go 后端 task struct，100% 前端实现。
 *
 * 用法：
 *   - 自动化测试入口：`recordTriggeredBy(task.id, 'automation', runId)`
 *   - AI 智能体入口：`recordTriggeredBy(task.id, 'ai_agent', runId)`
 *   - 用户手动创建：默认 'user'（无需显式登记）
 *   - 显示：`<ion-badge>{{ t('tasks.triggeredBy_' + getTriggeredBy(task.id)) }}</ion-badge>`
 *
 * 🆕 2026-06-10 扩展 runId 字段：
 *   - 让 Tasks.vue 能按 workflow run 分组（而不是按 triggeredBy 粗聚拢）
 *   - 同一 run 的 task 自动归入同一 group card
 *   - 跨 run 的 group card 互相独立、稳定
 *
 * 限制：500 条上限（按 recordedAt 排序裁剪），防止 localStorage 撑爆。
 * 清理 localStorage 后旧任务会显示为 'user'（向后兼容）。
 */

export type TriggeredBy = 'user' | 'automation' | 'ai_agent'

const STORAGE_KEY = 'encv_task_triggered_by_v2'  // 🆕 v2：加 runId 字段
const MAX_ENTRIES = 500

interface TriggeredByEntry {
  triggeredBy: TriggeredBy
  /** 🆕 workflow run 关联 ID（同一 run 的 task 共享） */
  runId?: string
  recordedAt: string
}

type TriggeredByMap = Record<string, TriggeredByEntry>

function readMap(): TriggeredByMap {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as TriggeredByMap
    if (!parsed || typeof parsed !== 'object') return {}
    return parsed
  } catch {
    return {}
  }
}

function writeMap(m: TriggeredByMap): void {
  // 限制最多 MAX_ENTRIES 条：按 recordedAt 倒序排，保留最新
  const entries = Object.entries(m)
    .sort((a, b) => b[1].recordedAt.localeCompare(a[1].recordedAt))
    .slice(0, MAX_ENTRIES)
  const trimmed: TriggeredByMap = Object.fromEntries(entries)
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(trimmed))
  } catch (e) {
    // localStorage 满了或被禁用——静默降级（任务继续走，但触发者 label 永远是 user）
    console.debug('[useTaskTrigger] localStorage write failed:', e)
  }
}

/**
 * 记录 task 的触发者 + 可选 workflow run 关联。
 *
 * @param taskId     后端 task.id
 * @param triggeredBy 触发者类型
 * @param runId      🆕 可选：workflow run.id（同一 run 的 task 共享）
 */
export function recordTriggeredBy(
  taskId: string,
  triggeredBy: TriggeredBy,
  runId?: string,
): void {
  if (!taskId) return
  const m = readMap()
  m[taskId] = { triggeredBy, recordedAt: new Date().toISOString(), ...(runId ? { runId } : {}) }
  writeMap(m)
}

export function getTriggeredBy(taskId: string): TriggeredBy {
  if (!taskId) return 'user'
  const m = readMap()
  return m[taskId]?.triggeredBy ?? 'user'
}

/**
 * 🆕 2026-06-10：获取 task 关联的 workflow run.id（没有则返回 undefined）
 */
export function getRunIdForTask(taskId: string): string | undefined {
  if (!taskId) return undefined
  const m = readMap()
  return m[taskId]?.runId
}

export function clearTriggeredBy(): void {
  try {
    localStorage.removeItem(STORAGE_KEY)
  } catch {
    // noop
  }
}

/**
 * 测试用：获取所有记录的副本（不导出到生产 API）。
 */
export function _getAllForTesting(): TriggeredByMap {
  return readMap()
}
