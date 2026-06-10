/**
 * 任务触发者标签 — localStorage 持久化
 *
 * 用前端 localStorage 维护 taskId → triggeredBy 映射。
 * 不侵入 Go 后端 task struct，100% 前端实现。
 *
 * 用法：
 *   - 自动化测试入口：`recordTriggeredBy(task.id, 'automation')`
 *   - AI 智能体入口：`recordTriggeredBy(task.id, 'ai_agent')`
 *   - 用户手动创建：默认 'user'（无需显式登记）
 *   - 显示：`<ion-badge>{{ t('tasks.triggeredBy_' + getTriggeredBy(task.id)) }}</ion-badge>`
 *
 * 限制：500 条上限（按 recordedAt 排序裁剪），防止 localStorage 撑爆。
 * 清理 localStorage 后旧任务会显示为 'user'（向后兼容）。
 */

export type TriggeredBy = 'user' | 'automation' | 'ai_agent'

const STORAGE_KEY = 'encv_task_triggered_by'
const MAX_ENTRIES = 500

interface TriggeredByEntry {
  triggeredBy: TriggeredBy
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

export function recordTriggeredBy(taskId: string, triggeredBy: TriggeredBy): void {
  if (!taskId) return
  const m = readMap()
  m[taskId] = { triggeredBy, recordedAt: new Date().toISOString() }
  writeMap(m)
}

export function getTriggeredBy(taskId: string): TriggeredBy {
  if (!taskId) return 'user'
  const m = readMap()
  return m[taskId]?.triggeredBy ?? 'user'
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
