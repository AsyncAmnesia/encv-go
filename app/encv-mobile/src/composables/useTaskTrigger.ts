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

import { reactive } from 'vue'

export type TriggeredBy = 'user' | 'automation' | 'ai_agent'

// 🆕 2026-06-10 修复 v4：key 升 v3，强制丢弃旧 v2 数据
// 历史 v2 数据可能跟新 flow 不兼容（v2 的 recordTriggeredBy 旧 runId 格式会让旧 task 分散
//   到不同 runId → 永远凑不到 1 个 group → 看起来「毫无变化」）
// 修复：v3 强制清空，强制用户重新跑 workflow 才能用新 group 行为
const STORAGE_KEY = 'encv_task_triggered_by_v3'
const MAX_ENTRIES = 500

interface TriggeredByEntry {
  triggeredBy: TriggeredBy
  /** workflow run 关联 ID（同一 run 的 task 共享） */
  runId?: string
  recordedAt: string
}

type TriggeredByMap = Record<string, TriggeredByEntry>

// 🆕 2026-06-10 修复 v4：triggeredByMap 用 reactive() 而不是 plain object
// 历史：plain object → displayedItems 不会因为 recordTriggeredBy 重新计算（Vue 追踪不到
//   module-level 变量的 mutation）→ 用户「先看到的 group 是旧的，加了新 task 也没聚合」
// 修复：reactive() → Vue 追踪属性访问 → recordTriggeredBy 触发响应式更新
const triggeredByMap = reactive<TriggeredByMap>({})

// 🆕 2026-06-10 修复 v4：跨 composable 共享 task 元数据
// 历史：useWorkflowEngine 创建 task 时只能写 localStorage，task 对象本身没 triggeredBy/runId
//   → applyTaskCreated 收到 WS 事件时无法知道这些元数据 → tasks.value 里的 task 没这 2 字段
//   → displayedItems 必须靠 getTriggeredBy / getRunIdForTask 查 localStorage
//   → 跨 session / localStorage 清空 → 全失效
// 修复：useTaskTrigger 维护一个 taskMetadata Map，submitAction 后立即 setTaskMetadata
//   applyTaskCreated 时合并进 task 对象 → tasks.value 里的 task 自带 triggeredBy / runId
//   displayedItems 直接读 t.triggeredBy / t.runId（O(1) 内存访问）
const taskMetadata: Map<string, { triggeredBy: TriggeredBy; runId?: string }> = new Map()

let initialized = false
function ensureLoaded(): void {
  if (initialized) return
  initialized = true
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return
    const parsed = JSON.parse(raw) as TriggeredByMap
    if (!parsed || typeof parsed !== 'object') return
    // 同步到 reactive 容器（保留 reactive 响应式）
    for (const [k, v] of Object.entries(parsed)) {
      triggeredByMap[k] = v
    }
  } catch {
    // silent
  }
}

function writeMap(): void {
  const entries = Object.entries(triggeredByMap)
    .sort((a, b) => b[1].recordedAt.localeCompare(a[1].recordedAt))
    .slice(0, MAX_ENTRIES)
  const trimmed: TriggeredByMap = Object.fromEntries(entries)
  for (const k of Object.keys(triggeredByMap)) delete triggeredByMap[k]
  Object.assign(triggeredByMap, trimmed)
  localStorage.setItem(STORAGE_KEY, JSON.stringify(trimmed))
}

function trimMapInPlace(): void {
  const entries = Object.entries(triggeredByMap)
    .sort((a, b) => b[1].recordedAt.localeCompare(a[1].recordedAt))
    .slice(0, MAX_ENTRIES)
  const trimmed: TriggeredByMap = Object.fromEntries(entries)
  for (const k of Object.keys(triggeredByMap)) delete triggeredByMap[k]
  Object.assign(triggeredByMap, trimmed)
}

/**
 * 🆕 2026-06-10 修复 v4：记录 task 的触发者 + workflow run 关联
 *
 * 同时维护 3 个数据源（保证 displayedItems 一定能拿到数据）：
 *   1. reactive triggeredByMap（让 Vue 响应式）
 *   2. module-level taskMetadata Map（让 useTasksList 能 merge 到 task 对象）
 *   3. localStorage（让跨 session 也能 fallback）
 */
export function recordTriggeredBy(
  taskId: string,
  triggeredBy: TriggeredBy,
  runId?: string,
): void {
  if (!taskId) return
  ensureLoaded()
  const prevEntry = triggeredByMap[taskId]
  triggeredByMap[taskId] = { triggeredBy, recordedAt: new Date().toISOString(), ...(runId ? { runId } : {}) }
  // 🆕 同步到 taskMetadata Map（让 useTasksList.applyTaskCreated 能 merge 到 task 对象）
  taskMetadata.set(taskId, { triggeredBy, runId })
  trimMapInPlace()
  try {
    writeMap()
  } catch (e) {
    if (prevEntry === undefined) delete triggeredByMap[taskId]
    else triggeredByMap[taskId] = prevEntry
    taskMetadata.delete(taskId)
    console.debug('[useTaskTrigger] localStorage write failed, rolled back:', e)
  }
}

/**
 * 🆕 2026-06-10 修复 v4：给 task 写元数据
 * 跟 recordTriggeredBy 等价，但不写 localStorage（仅当前 session 用）
 * 用法：useWorkflowEngine.executeJob 在 submitAction 后立即调，让 task 对象自带元数据
 */
export function setTaskMetadata(
  taskId: string,
  triggeredBy: TriggeredBy,
  runId?: string,
): void {
  if (!taskId) return
  ensureLoaded()
  taskMetadata.set(taskId, { triggeredBy, runId })
  // 也写 triggeredByMap 让 reactive 触发
  triggeredByMap[taskId] = {
    triggeredBy,
    recordedAt: new Date().toISOString(),
    ...(runId ? { runId } : {}),
  }
}

/**
 * 🆕 2026-06-10 修复 v4：取 task 的元数据
 * useTasksList.applyTaskCreated 在 spread data 后 merge 进来
 */
export function getTaskMetadata(
  taskId: string,
): { triggeredBy: TriggeredBy; runId?: string } | undefined {
  if (!taskId) return undefined
  ensureLoaded()
  return taskMetadata.get(taskId)
}

/**
 * 读 task 的 triggeredBy（带 fallback：先 taskMetadata，再 triggeredByMap，再 'user'）
 *
 * 🆕 2026-06-10 修复 v4：reactive 读，displayedItems 追踪依赖
 */
export function getTriggeredBy(taskId: string): TriggeredBy {
  if (!taskId) return 'user'
  ensureLoaded()
  // 先读 taskMetadata（O(1) 内存，比 reactive 对象的属性访问更快）
  const meta = taskMetadata.get(taskId)
  if (meta) return meta.triggeredBy
  return triggeredByMap[taskId]?.triggeredBy ?? 'user'
}

/**
 * 🆕 2026-06-10 修复 v4：读 task 关联的 workflow run.id
 */
export function getRunIdForTask(taskId: string): string | undefined {
  if (!taskId) return undefined
  ensureLoaded()
  const meta = taskMetadata.get(taskId)
  if (meta) return meta.runId
  return triggeredByMap[taskId]?.runId
}

/**
 * 清除进程级缓存（用于测试或手动 localStorage 变更后强制重新加载）
 */
export function _reloadTriggeredByCache(): void {
  initialized = false
  for (const k of Object.keys(triggeredByMap)) delete triggeredByMap[k]
  taskMetadata.clear()
}

/**
 * 🆕 2026-06-10 修复 v4：用户主动重置所有触发者记录
 * 用法：Tasks.vue 加个「重置分组」按钮 → 调这个 → 所有 task 重新变 'user'（重新跑 workflow 才会有新 group）
 */
export function clearTriggeredBy(): void {
  for (const k of Object.keys(triggeredByMap)) delete triggeredByMap[k]
  taskMetadata.clear()
  try {
    localStorage.removeItem(STORAGE_KEY)
  } catch {
    // silent
  }
  initialized = false
  ensureLoaded()  // 重新初始化空的 map
}

/**
 * 记录 task 的触发者 + 可选 workflow run 关联。
 *
 * @param taskId     后端 task.id
 * @param triggeredBy 触发者类型
 * @param runId      🆕 可选：workflow run.id（同一 run 的 task 共享）
 */
// (函数定义在文件顶部 v4 重构区)
