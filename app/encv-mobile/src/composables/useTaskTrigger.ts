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
  // 🆕 2026-06-10 修复：进程级内存缓存 + lazy load
  // 历史 bug：getRunIdForTask / getTriggeredBy 每次都同步 localStorage + JSON.parse
  //   200 个 task × N 次 computed 重算 → 巨量 JSON.parse（卡 UI）
  // 修复：模块级单例 cacheMap，首次 readMap 时 lazy load，之后纯内存读
  if (cacheMap) return cacheMap
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      cacheMap = {}
      return cacheMap
    }
    const parsed = JSON.parse(raw) as TriggeredByMap
    if (!parsed || typeof parsed !== 'object') {
      cacheMap = {}
      return cacheMap
    }
    cacheMap = parsed
    return cacheMap
  } catch {
    cacheMap = {}
    return cacheMap
  }
}

let cacheMap: TriggeredByMap | null = null

/**
 * 清除进程级缓存（用于测试或手动 localStorage 变更后强制重新加载）
 */
export function _reloadTriggeredByCache(): void {
  cacheMap = null
}

function writeMap(m: TriggeredByMap): void {
  // 限制最多 MAX_ENTRIES 条：按 recordedAt 倒序排，保留最新
  const entries = Object.entries(m)
    .sort((a, b) => b[1].recordedAt.localeCompare(a[1].recordedAt))
    .slice(0, MAX_ENTRIES)
  const trimmed: TriggeredByMap = Object.fromEntries(entries)
  // 🆕 2026-06-10 修复：异常向上抛，让 recordTriggeredBy 的 try-catch 能回滚 cacheMap
  // 历史 bug：writeMap 内部 try-catch 吞掉 setItem 异常 → recordTriggeredBy 不知道写失败
  //   → cacheMap 已被 mutate，但 localStorage 没写入 → 状态不一致
  localStorage.setItem(STORAGE_KEY, JSON.stringify(trimmed))
}

/**
 * 按 recordedAt 倒序裁剪到 MAX_ENTRIES 条，**就地修改 m**
 * 用于保持 cacheMap 跟 localStorage 写入的 trimmed 状态一致
 */
function trimMapInPlace(m: TriggeredByMap): void {
  const entries = Object.entries(m)
    .sort((a, b) => b[1].recordedAt.localeCompare(a[1].recordedAt))
    .slice(0, MAX_ENTRIES)
  const trimmed: TriggeredByMap = Object.fromEntries(entries)
  // 清空 m 的所有 key
  for (const k of Object.keys(m)) delete m[k]
  // 写入 trimmed 的 key
  Object.assign(m, trimmed)
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
  const prevEntry = m[taskId]
  m[taskId] = { triggeredBy, recordedAt: new Date().toISOString(), ...(runId ? { runId } : {}) }
  // 🆕 2026-06-10 修复：写之前先 trim cacheMap（保持跟 localStorage 写入的 trimmed 状态一致）
  // 历史 bug：cacheMap 保留全部 600 条，但 writeMap 写入 localStorage 时裁剪到 500 条
  //   → _getAllForTesting() 返 600 条，跟 localStorage 不一致
  //   → 也违反"500 条上限"约束（cacheMap 一直增长，内存泄漏）
  trimMapInPlace(m)
  // 🆕 2026-06-10 修复：写失败时回滚 cacheMap（防止 cacheMap 跟 localStorage 不一致）
  // 历史 bug：writeMap 抛 QuotaExceededError 后 cacheMap 已被 mutate，但 localStorage 没写入
  //   → 下次 getTriggeredBy 读 cacheMap 返 'automation'，跟用户期望的 'user' 降级不符
  try {
    writeMap(m)
  } catch (e) {
    if (prevEntry === undefined) delete m[taskId]
    else m[taskId] = prevEntry
    // localStorage 满了或被禁用——静默降级（任务继续走，但触发者 label 永远是 user）
    console.debug('[useTaskTrigger] localStorage write failed, cacheMap rolled back:', e)
  }
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
  // 🆕 2026-06-10 修复：清空 cacheMap，强制下次 readMap 重新读 localStorage
  // 历史 bug：clearTriggeredBy 只删 localStorage，cacheMap 仍有旧数据
  //   → getTriggeredBy 返旧值（用户期望 'user' 降级）
  cacheMap = null
}

/**
 * 测试用：获取所有记录的副本（不导出到生产 API）。
 */
export function _getAllForTesting(): TriggeredByMap {
  return readMap()
}
