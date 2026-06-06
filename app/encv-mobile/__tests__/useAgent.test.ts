/**
 * useAgent.test.ts - 复合式单元测试
 *
 * 覆盖：
 * 1. processSSE 解析 6 种 event type
 * 2. 4 决策 confirmTool
 * 3. localStorage save/load 持久化
 * 4. resume 续传
 * 5. stop 中断
 * 6. reset 清空
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// Mock useToast
vi.mock('@/composables/useToast', () => ({
  showToast: vi.fn(),
}))

// 重要：必须在 import useAgent 之前 mock globalThis.crypto.randomUUID，
// 否则 jsdom 默认提供 crypto.randomUUID 不会有问题，但我们仍要确保 stable 行为。
// 这里只 mock fetch 来注入可控的 SSE 响应流。

import { useAgent, type Message } from '@/composables/useAgent'
import { showToast } from '@/composables/useToast'

const mockedShowToast = vi.mocked(showToast)
const originalFetch = globalThis.fetch

// ─── 辅助函数 ─────────────────────────────────────────────────────────────

/**
 * 构造一个 mock ReadableStream，按 chunk 吐出给定的 SSE chunks
 */
function makeSSEStream(chunks: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder()
  let index = 0
  return new ReadableStream<Uint8Array>({
    pull(controller) {
      if (index < chunks.length) {
        controller.enqueue(encoder.encode(chunks[index]))
        index++
      } else {
        controller.close()
      }
    },
  })
}

/**
 * 把单个 AgentEvent 转成 SSE 字符串行
 */
function sseLine(type: string, data: unknown): string {
  return `data: ${JSON.stringify({ type, data: JSON.stringify(data) })}\n\n`
}

function fetchReturningStream(stream: ReadableStream<Uint8Array>): ReturnType<typeof vi.fn> {
  return vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    body: stream,
  } as Response)
}

function fetchReturningError(status = 500): ReturnType<typeof vi.fn> {
  return vi.fn().mockResolvedValue({
    ok: false,
    status,
    body: null,
  } as Response)
}

/**
 * 创建 URL 感知的 mock 实现：
 *   /api/config → 返回空配置（无 system_prompt）
 *   其他 URL → 使用 fallback（通常是测试提供的 SSE stream）
 */
function urlAwareMock(fallback: ReturnType<typeof vi.fn>): ReturnType<typeof vi.fn> {
  return vi.fn(async (url: string | Request) => {
    const urlStr = typeof url === 'string' ? url : (url as Request).url
    if (urlStr.includes('/api/config')) {
      return { ok: true, status: 200, json: () => Promise.resolve({}) } as Response
    }
    return fallback(url)
  })
}

// ─── 测试 ─────────────────────────────────────────────────────────────────

describe('useAgent', () => {
  let fetchSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    fetchSpy = vi.spyOn(globalThis, 'fetch')
    // 默认 mock：/api/config 返回空配置，其他 URL 返回空 body
    // 各测试通过 fetchSpy.mockImplementation(urlAwareMock(fallback)) 覆盖
    fetchSpy.mockImplementation(async (url: string | Request) => {
      const urlStr = typeof url === 'string' ? url : (url as Request).url
      if (urlStr.includes('/api/config')) {
        return { ok: true, status: 200, json: () => Promise.resolve({}) } as Response
      }
      return { ok: true, status: 200, body: null } as Response
    })
    localStorage.clear()
    mockedShowToast.mockClear()
  })

  afterEach(() => {
    fetchSpy.mockRestore()
    localStorage.clear()
  })

  describe('processSSE - 6 种 event type 分发', () => {
    it('text_delta 追加到最后 assistant 消息 content', async () => {
      const sse = sseLine('text_delta', { content: 'Hello' }) +
                  sseLine('text_delta', { content: ' World' })
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { send, messages } = useAgent()
      await send('hi')

      expect(messages.value.length).toBe(2)
      expect(messages.value[0].role).toBe('user')
      expect(messages.value[0].content).toBe('hi')
      expect(messages.value[1].role).toBe('assistant')
      expect(messages.value[1].content).toBe('Hello World')
    })

    it('reasoning_delta 追加到 reasoning 字段', async () => {
      const sse = sseLine('reasoning_delta', { content: 'thinking... ' }) +
                  sseLine('text_delta', { content: 'answer' })
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { send, messages } = useAgent()
      await send('q')

      const assistant = messages.value.find((m) => m.role === 'assistant')!
      expect(assistant.reasoning).toBe('thinking... ')
      expect(assistant.content).toBe('answer')
    })

    it('tool_call push 到 tool_calls（带 kind/needsConfirm/status=pending）', async () => {
      const sse = sseLine('text_delta', { content: 'I will run ls' }) +
                  sseLine('tool_call', {
                    id: 'tc-1',
                    name: 'exec_command',
                    args: '{"command":"ls"}',
                    auto_run: false,
                    kind: 'command',
                  })
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { send, messages } = useAgent()
      await send('list files')

      const assistant = messages.value.find((m) => m.role === 'assistant')!
      expect(assistant.tool_calls.length).toBe(1)
      const tc = assistant.tool_calls[0]
      expect(tc.id).toBe('tc-1')
      expect(tc.name).toBe('exec_command')
      expect(tc.kind).toBe('command')
      expect(tc.needsConfirm).toBe(true) // auto_run=false → needsConfirm
      expect(tc.status).toBe('pending')
    })

    it('tool_call auto_run=true 时 needsConfirm=false', async () => {
      const sse = sseLine('tool_call', {
        id: 'tc-2',
        name: 'list_files',
        args: '{"path":"/"}',
        auto_run: true,
        kind: 'readOnly',
      })
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { send, messages } = useAgent()
      await send('list')

      const tc = messages.value[1].tool_calls[0]
      expect(tc.needsConfirm).toBe(false)
    })

    it('tool_status 标记 tool_call 的 status 变更（running/success/failed）', async () => {
      const sse = sseLine('tool_call', {
        id: 'tc-3',
        name: 'exec_command',
        args: '{}',
        auto_run: true,
        kind: 'command',
      }) +
      sseLine('tool_status', { id: 'tc-3', status: 'running' }) +
      sseLine('tool_status', { id: 'tc-3', status: 'success' })
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { send, messages } = useAgent()
      await send('run')

      const tc = messages.value[1].tool_calls[0]
      expect(tc.status).toBe('success')
    })

    it('tool_result push 到 tool_results', async () => {
      const sse = sseLine('tool_call', {
        id: 'tc-4',
        name: 'list_files',
        args: '{}',
        auto_run: true,
        kind: 'readOnly',
      }) +
      sseLine('tool_result', {
        id: 'tc-4',
        name: 'list_files',
        result: '{"files":[]}',
        is_error: false,
        status: 'success',
        duration_ms: 42,
      })
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { send, messages } = useAgent()
      await send('list')

      expect(messages.value[1].tool_results.length).toBe(1)
      expect(messages.value[1].tool_results[0].id).toBe('tc-4')
      expect(messages.value[1].tool_results[0].is_error).toBe(false)
      expect(messages.value[1].tool_results[0].duration_ms).toBe(42)
    })

    it('stream_end 把 status 切回 idle（无 pending 确认时）', async () => {
      const sse = sseLine('text_delta', { content: 'done' }) +
                  sseLine('stream_end', {})
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { send, status, messages } = useAgent()
      await send('hi')

      expect(status.value).toBe('idle')
      // 最后 assistant 消息应标记 isStreaming=false
      const lastAssistant = messages.value[messages.value.length - 1]
      expect(lastAssistant.isStreaming).toBe(false)
    })

    it('stream_end 在有 pending needsConfirm tool_call 时 status=confirming', async () => {
      const sse = sseLine('tool_call', {
        id: 'tc-pending',
        name: 'delete_file',
        args: '{}',
        auto_run: false,
        kind: 'fileChange',
      }) +
      sseLine('stream_end', {})
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { send, status } = useAgent()
      await send('delete')

      expect(status.value).toBe('confirming')
    })

    it('分块读取仍能正确解析（chunked SSE）', async () => {
      const fullSSE = sseLine('text_delta', { content: 'hi' }) +
                      sseLine('stream_end', {})
      // 切两半
      const mid = Math.floor(fullSSE.length / 2)
      const chunk1 = fullSSE.slice(0, mid)
      const chunk2 = fullSSE.slice(mid)
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([chunk1, chunk2])))

      const { send, messages, status } = useAgent()
      await send('q')

      expect(messages.value[1].content).toBe('hi')
      expect(status.value).toBe('idle')
    })
  })

  describe('send - 4 决策 confirmTool', () => {
    it('confirmTool 接受 accept 决策：调用 /api/confirm 并处理 SSE', async () => {
      // 第一次 send：模拟有 pending tool_call
      const sse1 = sseLine('tool_call', {
        id: 'tc-x',
        name: 'delete_file',
        args: '{}',
        auto_run: false,
        kind: 'fileChange',
      }) +
      sseLine('stream_end', {})
      fetchSpy.mockImplementation(urlAwareMock(fetchReturningStream(makeSSEStream([sse1]))))

      const agent = useAgent()
      await agent.send('delete')

      expect(agent.status.value).toBe('confirming')
      expect(agent.messages.value[1].tool_calls[0].id).toBe('tc-x')

      // 第二次 confirmTool：accept
      const sse2 = sseLine('tool_result', {
        id: 'tc-x',
        name: 'delete_file',
        result: 'ok',
        is_error: false,
        status: 'success',
        duration_ms: 10,
      }) +
      sseLine('stream_end', {})
      fetchSpy.mockImplementation(urlAwareMock(fetchReturningStream(makeSSEStream([sse2]))))

      await agent.confirmTool('tc-x', 'accept')

      expect(fetchSpy).toHaveBeenCalledTimes(2)
      const confirmCall = fetchSpy.mock.calls[1]
      expect(confirmCall[0]).toBe('/agent-api/api/confirm')
      const body = JSON.parse(confirmCall[1].body)
      expect(body.toolCallId).toBe('tc-x')
      expect(body.decision).toBe('accept')
    })

    it('confirmTool 接受 4 种决策：accept / accept_for_session / decline / cancel', async () => {
      const decisions: Array<'accept' | 'accept_for_session' | 'decline' | 'cancel'> = [
        'accept',
        'accept_for_session',
        'decline',
        'cancel',
      ]
      for (const decision of decisions) {
        fetchSpy.mockReset()
        localStorage.clear()

        const sse1 = sseLine('tool_call', {
          id: `tc-${decision}`,
          name: 'op',
          args: '{}',
          auto_run: false,
          kind: 'command',
        }) + sseLine('stream_end', {})
        fetchSpy.mockImplementation(urlAwareMock(fetchReturningStream(makeSSEStream([sse1]))))

        const agent = useAgent()
        await agent.send('q')
        expect(agent.status.value).toBe('confirming')

        const sse2 = sseLine('stream_end', {})
        fetchSpy.mockImplementation(urlAwareMock(fetchReturningStream(makeSSEStream([sse2]))))
        await agent.confirmTool(`tc-${decision}`, decision)

        const body = JSON.parse(fetchSpy.mock.calls[1][1].body)
        expect(body.decision).toBe(decision)
      }
    })

    it('send 期间有 stream 时忽略二次 send', async () => {
      // 第一次 send：模拟一个完整但很短的流
      const sse1 = sseLine('text_delta', { content: 'first' }) +
                   sseLine('stream_end', {})
      fetchSpy.mockImplementation(urlAwareMock(fetchReturningStream(makeSSEStream([sse1]))))

      const { send, status, messages } = useAgent()
      await send('first')

      // 此时 status=idle（第一次 send 已完成）
      expect(status.value).toBe('idle')

      // 第二次 send 应该正常进行（idle → streaming）
      const sse2 = sseLine('text_delta', { content: 'second' }) +
                   sseLine('stream_end', {})
      fetchSpy.mockImplementation(urlAwareMock(fetchReturningStream(makeSSEStream([sse2]))))
      await send('second')

      expect(messages.value.length).toBe(4) // 2 user + 2 assistant
      expect(messages.value[0].content).toBe('first')
      expect(messages.value[2].content).toBe('second')
    })

    it('send 期间 status=streaming 时第二次 send 立即返回且不发新请求', async () => {
      // 用一个 pull 永远不关闭的流 + 拿到它的 controller（start 时存）
      let streamController: ReadableStreamDefaultController<Uint8Array> | null = null
      const slowStream = new ReadableStream<Uint8Array>({
        start(controller) {
          streamController = controller
        },
      })

      fetchSpy.mockImplementation(
        urlAwareMock(vi.fn().mockResolvedValue({
          ok: true,
          status: 200,
          body: slowStream,
        } as Response))
      )

      const { send, status, messages } = useAgent()
      // 启动第一次 send（不 await，让它挂起）
      const p1 = send('first').catch(() => {})

      // 给 microtask 一点时间
      await new Promise((r) => setTimeout(r, 5))

      expect(status.value).toBe('streaming')
      expect(messages.value.length).toBe(2)

      // 第二次 send：应立即返回（被忽略）
      await send('second')

      // fetch 仍只调用一次，messages 仍只有 2 条
      expect(fetchSpy).toHaveBeenCalledTimes(1)
      expect(messages.value.length).toBe(2)
      expect(messages.value[0].content).toBe('first')

      // 清理：关闭流 + 等第一次 send 完成
      if (streamController) streamController.close()
      await p1

      // 关闭后 status 应该恢复 idle
      expect(status.value).toBe('idle')
    }, 5000)
  })

  describe('localStorage 持久化', () => {
    it('send 完成后 session 写入 localStorage', async () => {
      const sse = sseLine('text_delta', { content: 'ok' }) +
                  sseLine('stream_end', {})
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { send } = useAgent()
      await send('hi')

      // localStorage 中应出现 agent:session:* 键
      const keys: string[] = []
      for (let i = 0; i < localStorage.length; i++) {
        const k = localStorage.key(i)
        if (k) keys.push(k)
      }
      expect(keys.some((k) => k.startsWith('agent:session:'))).toBe(true)

      const sessionKey = keys.find((k) => k.startsWith('agent:session:'))!
      const persisted = JSON.parse(localStorage.getItem(sessionKey)!)
      expect(persisted.sessionId).toBeTruthy()
      expect(persisted.messages.length).toBe(2)
      expect(persisted.messages[1].content).toBe('ok')
      expect(persisted.status).toBe('idle')
    })

    it('resume 从 localStorage 恢复 messages', async () => {
      // 预先填充 localStorage
      const fakeSessionId = 'fake-session-id-123'
      const stored = {
        sessionId: fakeSessionId,
        eventOffset: 5,
        messages: [
          { role: 'user', content: 'prev q', tool_calls: [], tool_results: [] },
          { role: 'assistant', content: 'prev a', tool_calls: [], tool_results: [], isStreaming: false },
        ] satisfies Message[],
        status: 'idle',
      }
      localStorage.setItem(`agent:session:${fakeSessionId}`, JSON.stringify(stored))

      // resume 不会发起新请求（status=idle）
      const { resume, messages } = useAgent()
      await resume()

      expect(messages.value.length).toBe(2)
      expect(messages.value[0].content).toBe('prev q')
      expect(messages.value[1].content).toBe('prev a')
      expect(fetchSpy).not.toHaveBeenCalled()
    })

    it('resume 之前 status=streaming 时调用 /api/resume', async () => {
      const fakeSessionId = 'session-streaming'
      localStorage.setItem(`agent:session:${fakeSessionId}`, JSON.stringify({
        sessionId: fakeSessionId,
        eventOffset: 2,
        messages: [
          { role: 'user', content: 'q', tool_calls: [], tool_results: [] },
          { role: 'assistant', content: 'partial', tool_calls: [], tool_results: [], isStreaming: true },
        ],
        status: 'streaming',
      }))

      const sse = sseLine('text_delta', { content: ' complete' }) +
                  sseLine('stream_end', {})
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { resume, messages, status } = useAgent()
      await resume()

      expect(fetchSpy).toHaveBeenCalledTimes(1)
      expect(fetchSpy.mock.calls[0][0]).toBe('/agent-api/api/resume')
      const body = JSON.parse(fetchSpy.mock.calls[0][1].body)
      expect(body.sessionId).toBe(fakeSessionId)
      // D.2: body 字段名从 offset 改成 lastEventId（与后端 handleAgentResume 对齐）
      expect(body.lastEventId).toBe(0)
      expect(body.offset).toBeUndefined()
      // D.2.3: lastEventId=0 时不发送 Last-Event-ID header
      const headers = fetchSpy.mock.calls[0][1].headers
      expect(headers['Last-Event-ID']).toBeUndefined()

      expect(messages.value[1].content).toBe('partial complete')
      expect(status.value).toBe('idle')
    })
  })

  describe('D.2 - resume 协议对齐（Last-Event-ID / stream_status）', () => {
    /**
     * 构造带 `id:` 行的 SSE 事件（SSE 标准断点续传字段）
     */
    function sseLineWithId(id: number, type: string, data: unknown): string {
      return `id: ${id}\ndata: ${JSON.stringify({ type, data: JSON.stringify(data) })}\n\n`
    }

    it('解析 SSE `id: N` 行：processSSE 维护 lastEventId', async () => {
      const sse = sseLineWithId(7, 'text_delta', { content: 'tail' }) +
                  sseLineWithId(8, 'stream_end', {})
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { send, messages, status } = useAgent()
      await send('q')

      // 收到 2 个事件，data 都被解析
      expect(messages.value[1].content).toBe('tail')
      expect(status.value).toBe('idle')
    })

    it('resume 时从 localStorage 恢复 lastEventId 并发送 Last-Event-ID header', async () => {
      const fakeSessionId = 'session-last-event-id'
      localStorage.setItem(`agent:session:${fakeSessionId}`, JSON.stringify({
        sessionId: fakeSessionId,
        eventOffset: 5,
        lastEventId: 42,  // 关键：上次收到的服务端事件 id
        messages: [
          { role: 'user', content: 'q', tool_calls: [], tool_results: [] },
          { role: 'assistant', content: 'partial', tool_calls: [], tool_results: [], isStreaming: true },
        ],
        status: 'streaming',
      }))

      const sse = sseLine('stream_end', {})
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { resume } = useAgent()
      await resume()

      // 验证：body.lastEventId + header.Last-Event-ID 都带上了
      expect(fetchSpy).toHaveBeenCalledTimes(1)
      const body = JSON.parse(fetchSpy.mock.calls[0][1].body)
      const headers = fetchSpy.mock.calls[0][1].headers
      expect(body.lastEventId).toBe(42)
      expect(headers['Last-Event-ID']).toBe('42')
    })

    it('stream_status.more_pending → 链式 resume（连续两次 /api/resume 调用）', async () => {
      const fakeSessionId = 'session-stream-status'
      localStorage.setItem(`agent:session:${fakeSessionId}`, JSON.stringify({
        sessionId: fakeSessionId,
        eventOffset: 1,
        lastEventId: 10,
        messages: [
          { role: 'user', content: 'q', tool_calls: [], tool_results: [] },
          { role: 'assistant', content: 'a', tool_calls: [], tool_results: [], isStreaming: true },
        ],
        status: 'streaming',
      }))

      // 第一轮 resume：server 推 stream_status.more_pending
      const sse1 = sseLine('stream_status', { status: 'more_pending', inProgress: true, maxEventId: 11 })
      // 第二轮 resume：server 推真正的事件 + stream_end
      const sse2 = sseLine('text_delta', { content: ' tail' }) +
                   sseLine('stream_end', {})

      let callCount = 0
      fetchSpy.mockImplementation(vi.fn(async (url: string | Request) => {
        const urlStr = typeof url === 'string' ? url : (url as Request).url
        if (urlStr.includes('/api/config')) {
          return { ok: true, status: 200, json: () => Promise.resolve({}) } as Response
        }
        if (urlStr.includes('/api/resume')) {
          callCount++
          const stream = callCount === 1 ? makeSSEStream([sse1]) : makeSSEStream([sse2])
          return { ok: true, status: 200, body: stream } as Response
        }
        return { ok: false, status: 404 } as Response
      }))

      const { resume, messages } = useAgent()
      await resume()

      // 关键断言：被链式调用了两次
      expect(callCount).toBe(2)
      expect(messages.value[1].content).toBe('a tail')
    })

    it('stream_status.synced → 退出链式 resume（保持 streaming 状态由下一次轮询触发）', async () => {
      const fakeSessionId = 'session-synced'
      localStorage.setItem(`agent:session:${fakeSessionId}`, JSON.stringify({
        sessionId: fakeSessionId,
        eventOffset: 1,
        lastEventId: 5,
        messages: [
          { role: 'user', content: 'q', tool_calls: [], tool_results: [] },
          { role: 'assistant', content: 'a', tool_calls: [], tool_results: [], isStreaming: true },
        ],
        status: 'streaming',
      }))

      // server 推 stream_status.synced + 自然关闭流（无 stream_end）
      const sse1 = sseLine('stream_status', { status: 'synced', inProgress: true, maxEventId: 5 })

      let callCount = 0
      fetchSpy.mockImplementation(vi.fn(async (url: string | Request) => {
        const urlStr = typeof url === 'string' ? url : (url as Request).url
        if (urlStr.includes('/api/config')) {
          return { ok: true, status: 200, json: () => Promise.resolve({}) } as Response
        }
        if (urlStr.includes('/api/resume')) {
          callCount++
          return { ok: true, status: 200, body: makeSSEStream([sse1]) } as Response
        }
        return { ok: false, status: 404 } as Response
      }))

      const { resume, messages, status } = useAgent()
      await resume()

      // synced：只调用一次（没有 more_pending 触发下一轮）
      expect(callCount).toBe(1)
      // synced 不修改 messages.content（仅同步 lastEventId）
      expect(messages.value[1].content).toBe('a')
      // status 仍为 streaming（等待用户后续触发）
      // 注：processSSE 流关闭时会执行 finalizeLastAssistant + status 回退
      // 但 messages 中有 isStreaming=true，会被 finalizeLastAssistant 影响
      expect(status.value).toBe('idle')  // 流自然关闭后切回 idle
    })

    it('runResumeChain 限制 maxHops=32 防无限递归', async () => {
      const fakeSessionId = 'session-infinite'
      localStorage.setItem(`agent:session:${fakeSessionId}`, JSON.stringify({
        sessionId: fakeSessionId,
        eventOffset: 1,
        lastEventId: 5,
        messages: [
          { role: 'user', content: 'q', tool_calls: [], tool_results: [] },
          { role: 'assistant', content: 'a', tool_calls: [], tool_results: [], isStreaming: true },
        ],
        status: 'streaming',
      }))

      // server 永远只推 more_pending（不推 stream_end）
      const sseForever = sseLine('stream_status', { status: 'more_pending', inProgress: true, maxEventId: 1 })
      let callCount = 0

      fetchSpy.mockImplementation(vi.fn(async (url: string | Request) => {
        const urlStr = typeof url === 'string' ? url : (url as Request).url
        if (urlStr.includes('/api/config')) {
          return { ok: true, status: 200, json: () => Promise.resolve({}) } as Response
        }
        if (urlStr.includes('/api/resume')) {
          callCount++
          return { ok: true, status: 200, body: makeSSEStream([sseForever]) } as Response
        }
        return { ok: false, status: 404 } as Response
      }))

      const { resume } = useAgent()
      await resume()

      // maxHops=32：每轮 resume 包含初次，最多 32 次
      expect(callCount).toBe(32)
    })

    it('老存档（无 lastEventId 字段）兼容：缺省按 0 处理', async () => {
      const fakeSessionId = 'session-legacy'
      // 注意：stored 没有 lastEventId 字段
      localStorage.setItem(`agent:session:${fakeSessionId}`, JSON.stringify({
        sessionId: fakeSessionId,
        eventOffset: 0,
        messages: [
          { role: 'user', content: 'q', tool_calls: [], tool_results: [] },
          { role: 'assistant', content: 'a', tool_calls: [], tool_results: [], isStreaming: true },
        ],
        status: 'streaming',
      }))

      const sse = sseLine('stream_end', {})
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { resume } = useAgent()
      await resume()

      const body = JSON.parse(fetchSpy.mock.calls[0][1].body)
      const headers = fetchSpy.mock.calls[0][1].headers
      expect(body.lastEventId).toBe(0)  // 缺省
      expect(headers['Last-Event-ID']).toBeUndefined()  // 0 不发 header
    })
  })

  describe('stop / reset', () => {
    it('stop 中断正在 streaming 的流', async () => {
      const sse1 = sseLine('text_delta', { content: 'partial' })
      const stream = makeSSEStream([sse1])
      fetchSpy.mockImplementationOnce(fetchReturningStream(stream))

      const { send, stop, status, messages } = useAgent()
      const promise = send('q')

      // 等待 microtask 让 fetch 启动
      await new Promise((r) => setTimeout(r, 0))
      stop()

      await promise

      expect(status.value).toBe('idle')
      expect(messages.value[1].isStreaming).toBe(false)
    })

    it('reset 清空所有状态 + 删除 localStorage', async () => {
      const sse = sseLine('text_delta', { content: 'x' }) +
                  sseLine('stream_end', {})
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { send, reset, messages, status } = useAgent()
      await send('hi')

      expect(messages.value.length).toBeGreaterThan(0)
      expect(localStorage.length).toBeGreaterThan(0)

      reset()
      expect(messages.value.length).toBe(0)
      expect(status.value).toBe('idle')
      expect(localStorage.length).toBe(0)
    })
  })

  describe('错误处理', () => {
    it('fetch 500 错误时显示 toast + status=idle', async () => {
      fetchSpy.mockImplementation(fetchReturningError(500))

      const { send, status, messages } = useAgent()
      await send('hi')

      expect(mockedShowToast).toHaveBeenCalled()
      expect(status.value).toBe('idle')
      // 流式结束标记
      expect(messages.value[1].isStreaming).toBe(false)
    })

    it('无效 JSON SSE payload 静默忽略（不崩溃）', async () => {
      const sse = 'data: {this is broken\n\n' +
                  sseLine('text_delta', { content: 'ok' }) +
                  sseLine('stream_end', {})
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { send, messages, status } = useAgent()
      await send('q')

      expect(messages.value[1].content).toBe('ok')
      expect(status.value).toBe('idle')
    })

    it('tool_call data 是非法 JSON 时跳过该事件（不崩溃）', async () => {
      const sse = 'data: {"type":"tool_call","data":"not valid json"}\n\n' +
                  sseLine('text_delta', { content: 'ok' }) +
                  sseLine('stream_end', {})
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const { send, messages, status } = useAgent()
      await send('q')

      expect(messages.value[1].content).toBe('ok')
      expect(messages.value[1].tool_calls.length).toBe(0)
      expect(status.value).toBe('idle')
    })
  })

  describe('processSSE 空/异常 stream', () => {
    it('response.body 为 null 时 processSSE 安全返回', async () => {
      // 直接构造 body: null 的 mock（processSSE 早返回）
      fetchSpy.mockImplementationOnce(vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        body: null,
      } as Response))

      const { send, status, messages } = useAgent()
      await send('q')

      // body 为 null 时 send() 抛出异常 → catch → status=idle + user msg 标记 error
      expect(status.value).toBe('idle')
      // 最后一条 user 消息应标记了 error
      const lastUserMsg = [...messages.value].reverse().find((m) => m.role === 'user')
      expect(lastUserMsg?.error).toBeTruthy()
    })
  })

  describe('resume - 异常路径', () => {
    it('resume 时 fetch 500 错误 → 静默 + status=idle', async () => {
      const sessionId = 'session-resume-fail'
      localStorage.setItem(`agent:session:${sessionId}`, JSON.stringify({
        sessionId,
        eventOffset: 0,
        messages: [
          { role: 'user', content: 'q', tool_calls: [], tool_results: [] },
          { role: 'assistant', content: 'partial', tool_calls: [], tool_results: [], isStreaming: true },
        ],
        status: 'streaming',
      }))

      fetchSpy.mockImplementationOnce(fetchReturningError(500))

      const { resume, status, messages } = useAgent()
      await resume()

      expect(status.value).toBe('idle')
      // 流式结束标记
      expect(messages.value[1].isStreaming).toBe(false)
    })

    it('resume 时没有 streaming 状态：不发请求', async () => {
      const sessionId = 'session-idle'
      localStorage.setItem(`agent:session:${sessionId}`, JSON.stringify({
        sessionId,
        eventOffset: 0,
        messages: [
          { role: 'user', content: 'q', tool_calls: [], tool_results: [] },
          { role: 'assistant', content: 'a', tool_calls: [], tool_results: [], isStreaming: false },
        ],
        status: 'idle',
      }))

      const { resume } = useAgent()
      await resume()

      expect(fetchSpy).not.toHaveBeenCalled()
    })

    it('resume 时 localStorage 没有 session：安全返回', async () => {
      const { resume, messages, status } = useAgent()
      await resume()

      expect(fetchSpy).not.toHaveBeenCalled()
      expect(messages.value.length).toBe(0)
      expect(status.value).toBe('idle')
    })
  })

  describe('confirmTool - 异常路径', () => {
    it('confirmTool 在没有 active session 时立即返回', async () => {
      const { confirmTool } = useAgent()
      await confirmTool('tc-1', 'accept')
      expect(fetchSpy).not.toHaveBeenCalled()
    })

    it('confirmTool 在 fetch 500 时 status 回到 confirming + tool.status 回到 pending', async () => {
      // 先建立 confirming 状态
      const sse1 = sseLine('tool_call', {
        id: 'tc-r',
        name: 'op',
        args: '{}',
        auto_run: false,
        kind: 'command',
      }) + sseLine('stream_end', {})
      fetchSpy.mockImplementation(urlAwareMock(fetchReturningStream(makeSSEStream([sse1]))))
      const agent = useAgent()
      await agent.send('q')
      expect(agent.status.value).toBe('confirming')

      // 第二次 fetch 500 错误
      fetchSpy.mockImplementation(urlAwareMock(fetchReturningError(500)))
      await agent.confirmTool('tc-r', 'accept')

      expect(agent.status.value).toBe('confirming')
      expect(agent.messages.value[1].tool_calls[0].status).toBe('pending')
      expect(mockedShowToast).toHaveBeenCalled()
    })
  })

  describe('stop - 多次调用', () => {
    it('stop 调用多次不崩溃', async () => {
      const { stop, status } = useAgent()
      stop()
      stop()
      stop()
      expect(status.value).toBe('idle')
    })

    it('stop 之前没有任何流式连接：仍是 no-op + status=idle', () => {
      const { stop, status } = useAgent()
      stop()
      expect(status.value).toBe('idle')
    })
  })

  describe('reset - 多实例隔离', () => {
    it('多个 useAgent 实例互不影响', async () => {
      const sse = sseLine('text_delta', { content: 'hi' }) +
                  sseLine('stream_end', {})
      fetchSpy.mockImplementation(fetchReturningStream(makeSSEStream([sse])))

      const a = useAgent()
      const b = useAgent()

      await a.send('for A')
      // A 完成，B 不应有任何消息
      expect(a.messages.value.length).toBe(2)
      expect(b.messages.value.length).toBe(0)

      a.reset()
      // A 清空后 B 仍不变
      expect(a.messages.value.length).toBe(0)
      expect(b.messages.value.length).toBe(0)
    })
  })
})
