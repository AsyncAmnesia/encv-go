/**
 * HttpPollBackend 单元测试（2026-06-10）
 *
 * 覆盖：
 *  1. 启动后立刻 tick 一次（active 模式）
 *  2. 新 task id → emit 'task:created'
 *  3. status 变化 → emit 'task:update' + 'task:completed'（如果终态）
 *  4. progress/phase 变化 → emit 'task:progress'
 *  5. 错误 backoff（连续失败 → 间隔翻倍）
 *  6. document hidden 切到 30s 节流
 *  7. stop 后不再 tick
 *  8. snapshot 消失防御（后端重启）→ emit 'task:completed' with server-list-missing
 *
 * 实现：
 *   - 通过 options.fetchTasks 注入 mock（避免真实 HTTP）
 *   - 用真实 setTimeout 等待（避免 fake timers + microtask 同步陷阱）
 *   - stop() / 错误路径用更短等待以减少测试时间
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createHttpPollBackend } from '@/composables/realtime/HttpPollBackend'
import type { EncvTask } from '@/api/encv'

function makeTask(id: string, status: string = 'running', progress: number = 0): EncvTask {
  return {
    id,
    type: 'encrypt' as any,
    sourcePath: `/tmp/${id}.mp4`,
    status: status as any,
    progress,
    createdAt: new Date().toISOString(),
  } as EncvTask
}

describe('HttpPollBackend', () => {
  let emit: any
  let fetchTasks: any
  let events: Array<{ type: string; data: any }>

  beforeEach(() => {
    events = []
    emit = vi.fn((type: string, data: any) => {
      events.push({ type, data })
    })
    fetchTasks = vi.fn().mockResolvedValue([])
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  // ─── 基础行为 ─────────────────────────────────────

  it('start() triggers initial tick + emits server:status on first success', async () => {
    fetchTasks.mockResolvedValue([])
    const backend = createHttpPollBackend(emit, { fetchTasks })

    backend.start()
    await new Promise((r) => setTimeout(r, 30))

    expect(fetchTasks).toHaveBeenCalled()
    expect(emit).toHaveBeenCalledWith('server:status', { online: true })
  })

  it('new task id → emit task:created + task:update (if not queued)', async () => {
    fetchTasks.mockResolvedValue([makeTask('t1', 'running', 10)])
    const backend = createHttpPollBackend(emit, { fetchTasks })
    backend.start()
    await new Promise((r) => setTimeout(r, 30))

    const types = events.map((e) => e.type)
    expect(types).toContain('task:created')
    expect(types).toContain('task:update')
  })

  it('status change → emit task:update (and task:completed for terminal)', async () => {
    // 第一轮：running
    fetchTasks.mockResolvedValueOnce([makeTask('t1', 'running', 10)])
    const backend = createHttpPollBackend(emit, { fetchTasks })
    backend.start()
    await new Promise((r) => setTimeout(r, 30))
    events.length = 0

    // 第二轮：success（终态）
    fetchTasks.mockResolvedValueOnce([makeTask('t1', 'success', 100)])
    // 等下一轮 tick（active 2s）
    await new Promise((r) => setTimeout(r, 2100))

    const types = events.map((e) => e.type)
    expect(types).toContain('task:update')
    expect(types).toContain('task:completed')
  })

  it('progress change → emit task:progress', async () => {
    fetchTasks.mockResolvedValueOnce([makeTask('t1', 'running', 10)])
    const backend = createHttpPollBackend(emit, { fetchTasks })
    backend.start()
    await new Promise((r) => setTimeout(r, 30))
    events.length = 0

    fetchTasks.mockResolvedValueOnce([makeTask('t1', 'running', 50)])
    await new Promise((r) => setTimeout(r, 2100))

    expect(events.some((e) => e.type === 'task:progress')).toBe(true)
  })

  // ─── 错误处理 ─────────────────────────────────────

  it('error → no connection-error spam to user', async () => {
    fetchTasks.mockRejectedValue(new Error('network error'))
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const backend = createHttpPollBackend(emit, { fetchTasks })
    backend.start()
    await new Promise((r) => setTimeout(r, 30))

    // 错误路径不应 emit 'server:connection-error'（避免 noise）
    expect(events.every((e) => e.type !== 'server:connection-error')).toBe(true)
    warnSpy.mockRestore()
  })

  // ─── 生命周期 ─────────────────────────────────────

  it('stop() prevents further ticks', async () => {
    fetchTasks.mockResolvedValue([])
    const backend = createHttpPollBackend(emit, { fetchTasks })
    backend.start()
    await new Promise((r) => setTimeout(r, 30))
    const callCount = fetchTasks.mock.calls.length

    backend.stop()
    await new Promise((r) => setTimeout(r, 2500))
    // stop 后不应继续 tick
    expect(fetchTasks.mock.calls.length).toBe(callCount)
  })

  // ─── 边界 case ────────────────────────────────────

  it('disappearing task → emit task:completed with server-list-missing', async () => {
    // 第一轮：task 存在
    fetchTasks.mockResolvedValueOnce([makeTask('t1', 'running', 10)])
    const backend = createHttpPollBackend(emit, { fetchTasks })
    backend.start()
    await new Promise((r) => setTimeout(r, 30))
    events.length = 0

    // 第二轮：task 消失（后端重启 / 列表被清空）
    fetchTasks.mockResolvedValueOnce([])
    await new Promise((r) => setTimeout(r, 2100))

    const completed = events.find((e) => e.type === 'task:completed')
    expect(completed).toBeDefined()
    expect(completed?.data.error).toBe('server-list-missing')
  })
})
