/**
 * useAgent - Vue 复合式：与 Go agent 服务进行 SSE 流式对话
 *
 * 核心能力：
 * 1. reactive<Message[]> 消息列表
 * 2. processSSE 解析 6 种事件类型
 * 3. 4-决策 ConfirmTool（accept / accept_for_session / decline / cancel）
 * 4. 启动时自动续传（基于 localStorage）
 * 5. 持久化（每次事件后写 localStorage `agent:session:{sessionId}`）
 *
 * API 路径（由 preview-gateway :16666 转发）：
 *   - POST /agent-api/api/chat    发起对话（SSE）
 *   - POST /agent-api/api/resume  断点续传（SSE）
 *   - POST /agent-api/api/confirm 4-决策确认（SSE）
 *
 * SPEC: /workspace/.trae/specs/go-in-process-agent/spec.md
 *   - Requirement: Vue 复合式（useAgent）
 *   - Requirement: Event 类型契约（6 种 event type）
 *   - Requirement: 4-决策 ConfirmRequest
 */
import { ref } from 'vue'
import { showToast } from '@/composables/useToast'

// =============================================================================
// 类型定义（与 agent Go 服务契约对齐）
// =============================================================================

export type AgentStatus = 'idle' | 'streaming' | 'confirming' | 'error'

export interface SessionMeta {
  id: string
  title: string
  createdAt: number
  updatedAt: number
  messageCount: number
}

export type Decision = 'accept' | 'accept_for_session' | 'decline' | 'cancel'

export type AgentEventType =
  | 'text_delta'
  | 'reasoning_delta'
  | 'tool_call'
  | 'tool_status'
  | 'tool_result'
  | 'stream_end'

/** Agent 推送到 SSE channel 的事件 */
export interface AgentEvent {
  type: AgentEventType
  /** JSON string —— 前端按 type 自行反序列化 */
  data: string
}

export type ToolKind = 'command' | 'fileChange' | 'readOnly' | 'webSearch' | 'unknown'
export type ToolStatus = 'pending' | 'running' | 'success' | 'failed' | 'cancelled'

export interface ToolCall {
  id: string
  name: string
  args: string
  auto_run: boolean
  kind: ToolKind
  /** !auto_run —— 需要用户 4-决策确认 */
  needsConfirm: boolean
  status: ToolStatus
}

export interface ToolResult {
  id: string
  name: string
  result: string
  is_error: boolean
  status: string
  duration_ms: number
}

export interface Message {
  role: 'user' | 'assistant'
  content: string
  reasoning?: string
  tool_calls: ToolCall[]
  tool_results: ToolResult[]
  isStreaming?: boolean
  /** 发送失败时的错误信息（每条消息独立） */
  error?: string
}

// =============================================================================
// 常量
// =============================================================================

/** 持久化到 localStorage 的 key 前缀 */
const STORAGE_PREFIX = 'agent:session:'

/** Agent 服务 API 路径（由 preview-gateway :16666 转发到 :5245） */
const AGENT_API_BASE = '/agent-api'

/** ToolCall 状态在 tool_status 事件中可能的取值 */
const TOOL_STATUS_VALUES: ReadonlySet<ToolStatus> = new Set<ToolStatus>([
  'pending',
  'running',
  'success',
  'failed',
  'cancelled',
])

// =============================================================================
// 工具函数
// =============================================================================

/**
 * 解析 `text_delta` / `reasoning_delta` 的 data 字段
 * Agent 推送格式：`{"content": "..."}` —— 反序列化为字符串
 */
function parseContentDelta(data: string): string {
  try {
    const parsed = JSON.parse(data)
    if (typeof parsed === 'string') return parsed
    if (parsed && typeof parsed === 'object' && 'content' in parsed) {
      return String((parsed as { content: unknown }).content ?? '')
    }
    return ''
  } catch {
    return ''
  }
}

/**
 * 解析 `tool_call` 的 data 字段 —— ToolCallData
 */
function parseToolCallData(data: string): ToolCall | null {
  try {
    const parsed = JSON.parse(data) as Partial<ToolCall>
    if (!parsed.id || !parsed.name) return null
    const autoRun = parsed.auto_run !== false
    return {
      id: String(parsed.id),
      name: String(parsed.name),
      args: typeof parsed.args === 'string' ? parsed.args : JSON.stringify(parsed.args ?? {}),
      auto_run: autoRun,
      kind: (parsed.kind as ToolKind) ?? 'unknown',
      needsConfirm: !autoRun,
      status: 'pending',
    }
  } catch {
    return null
  }
}

/**
 * 解析 `tool_status` 的 data 字段 —— 包含 id 和 status
 */
function parseToolStatus(data: string): { id: string; status: ToolStatus } | null {
  try {
    const parsed = JSON.parse(data) as { id?: string; status?: string }
    if (!parsed.id || !parsed.status) return null
    const status = parsed.status as ToolStatus
    if (!TOOL_STATUS_VALUES.has(status)) return null
    return { id: String(parsed.id), status }
  } catch {
    return null
  }
}

/**
 * 解析 `tool_result` 的 data 字段 —— ToolResultData
 */
function parseToolResultData(data: string): ToolResult | null {
  try {
    const parsed = JSON.parse(data) as Partial<ToolResult>
    if (!parsed.id || !parsed.name) return null
    return {
      id: String(parsed.id),
      name: String(parsed.name),
      result: typeof parsed.result === 'string' ? parsed.result : JSON.stringify(parsed.result ?? ''),
      is_error: parsed.is_error === true,
      status: String(parsed.status ?? 'success'),
      duration_ms: typeof parsed.duration_ms === 'number' ? parsed.duration_ms : 0,
    }
  } catch {
    return null
  }
}

function generateSessionId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  // 极端 fallback（老浏览器 / 测试环境无 crypto）
  return `sess-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`
}

// =============================================================================
// 复合式主体
// =============================================================================

export function useAgent() {
  const messages = ref<Message[]>([])
  const status = ref<AgentStatus>('idle')
  const lastError = ref<string>('')
  const lastUserInput = ref<string>('')
  const sessions = ref<SessionMeta[]>([])
  const currentSessionId = ref<string>('')
  let eventOffset = 0
  let abortController: AbortController | null = null

  // 模型/温度从 localStorage 读取（AgentChat 顶部 UI 选择会同步写入这里）
  const MODEL_STORAGE_KEY = 'encv-agent-selected-model'
  const TEMP_STORAGE_KEY = 'encv-agent-temperature'
  const activeModel = ref<string>(
    (() => {
      try { return localStorage.getItem(MODEL_STORAGE_KEY) || 'gpt-4o-mini' }
      catch { return 'gpt-4o-mini' }
    })(),
  )
  const activeTemperature = ref<number>(
    (() => {
      try {
        const v = localStorage.getItem(TEMP_STORAGE_KEY)
        const n = v == null ? 0.7 : Number(v)
        return Number.isFinite(n) ? n : 0.7
      } catch { return 0.7 }
    })(),
  )

  // ─── 内部辅助 ───────────────────────────────────────────────────────────

  /**
   * 把最后一条正在 streaming 的 assistant 消息标记为流式结束
   * 用于 catch 块 / stop() / 错误恢复场景
   */
  function finalizeLastAssistant(): void {
    for (let i = messages.value.length - 1; i >= 0; i--) {
      if (messages.value[i].role === 'assistant' && messages.value[i].isStreaming) {
        messages.value[i].isStreaming = false
        break
      }
    }
  }

  // ─── 持久化 ─────────────────────────────────────────────────────────────

  function saveState() {
    if (!currentSessionId.value) return
    try {
      const payload = {
        sessionId: currentSessionId.value,
        eventOffset,
        messages: JSON.parse(JSON.stringify(messages.value)),
        status: status.value,
      }
      localStorage.setItem(STORAGE_PREFIX + currentSessionId.value, JSON.stringify(payload))
    } catch (e) {
      console.debug('[useAgent] saveState failed:', e)
    }
  }

  function loadState(sessionId: string): {
    sessionId: string
    eventOffset: number
    messages: Message[]
    status: AgentStatus
  } | null {
    try {
      const raw = localStorage.getItem(STORAGE_PREFIX + sessionId)
      if (!raw) return null
      return JSON.parse(raw)
    } catch (e) {
      console.debug('[useAgent] loadState failed:', e)
      return null
    }
  }

  function findLatestPersistedSession(): string | null {
    try {
      let latest: { id: string; ts: number } | null = null
      for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i)
        if (!key || !key.startsWith(STORAGE_PREFIX)) continue
        const raw = localStorage.getItem(key)
        if (!raw) continue
        try {
          const parsed = JSON.parse(raw) as { sessionId?: string; messages?: Message[] }
          // 启发式：取有消息的最后一次会话
          if (parsed?.messages && parsed.messages.length > 0 && parsed.sessionId) {
            const ts = parsed.messages[parsed.messages.length - 1]?.content?.length ?? 0
            if (!latest || ts > latest.ts) {
              latest = { id: parsed.sessionId, ts }
            }
          }
        } catch {
          // skip malformed
        }
      }
      return latest?.id ?? null
    } catch (e) {
      console.debug('[useAgent] findLatestPersistedSession failed:', e)
      return null
    }
  }

  /**
   * 扫描 localStorage 中所有 `agent:session:*` 键，返回按 updatedAt 倒序的 session 列表
   * 用于 UI 渲染"会话历史"
   */
  function refreshSessions(): void {
    const list: SessionMeta[] = []
    try {
      for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i)
        if (!key || !key.startsWith(STORAGE_PREFIX)) continue
        const raw = localStorage.getItem(key)
        if (!raw) continue
        try {
          const parsed = JSON.parse(raw) as {
            sessionId?: string
            messages?: Message[]
          }
          if (!parsed?.sessionId) continue
          const msgs = parsed.messages || []
          const firstUser = msgs.find((m) => m.role === 'user')
          const title =
            (firstUser?.content || '').split('\n')[0]?.slice(0, 40) || '(空会话)'
          const updatedAt = msgs.length > 0 ? Date.now() - (msgs.length - 1) : 0
          const createdAt = updatedAt
          list.push({
            id: parsed.sessionId,
            title,
            createdAt,
            updatedAt,
            messageCount: msgs.length,
          })
        } catch {
          // skip
        }
      }
    } catch (e) {
      console.debug('[useAgent] refreshSessions failed:', e)
    }
    list.sort((a, b) => b.updatedAt - a.updatedAt)
    sessions.value = list
  }

  /**
   * 切换到指定 session：先 stop 当前流，再加载目标 session 消息
   */
  function switchSession(sessionId: string): void {
    if (sessionId === currentSessionId.value) return
    if (abortController) {
      abortController.abort()
      abortController = null
    }
    const saved = loadState(sessionId)
    if (!saved) {
      console.debug('[useAgent] switchSession: no saved state for', sessionId)
      return
    }
    currentSessionId.value = saved.sessionId
    eventOffset = saved.eventOffset
    messages.value = saved.messages.map((m) => ({ ...m }))
    status.value = 'idle'
    lastError.value = ''
    saveState()
  }

  /**
   * 创建新 session 并切到它（不删除原 session，留作历史）
   */
  function newSession(): void {
    if (abortController) {
      abortController.abort()
      abortController = null
    }
    if (currentSessionId.value && messages.value.length > 0) {
      // 当前会话已存在消息 → 持久化保留作为历史
      saveState()
    }
    currentSessionId.value = generateSessionId()
    eventOffset = 0
    messages.value = []
    status.value = 'idle'
    lastError.value = ''
    lastUserInput.value = ''
    saveState()
    refreshSessions()
  }

  /**
   * 删除一个 session（不可恢复）
   */
  function deleteSession(sessionId: string): void {
    try {
      localStorage.removeItem(STORAGE_PREFIX + sessionId)
    } catch {
      // ignore
    }
    if (sessionId === currentSessionId.value) {
      currentSessionId.value = ''
      messages.value = []
      eventOffset = 0
    }
    refreshSessions()
  }

  // ─── SSE 解析器 ─────────────────────────────────────────────────────────

  /**
   * 解析 SSE 响应流并 dispatch 事件到 reactive state
   *
   * SSE 格式：
   *   data: {"type": "text_delta", "data": "..."}\n
   *   data: {"type": "stream_end", "data": ""}\n
   *   \n
   *
   * 注：agent Go 服务推送的每行已经是一个 `{type, data}` JSON；
   *     data 字段是 stringified payload。
   */
  /**
   * 解析 SSE 流，返回是否收到过至少一个有效事件
   */
  async function processSSE(stream: ReadableStream<Uint8Array> | null): Promise<{ received: boolean }> {
    if (!stream) return { received: false }

    const reader = stream.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let received = false

    try {
      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        // SSE 事件以 \n\n 分隔
        const events = buffer.split('\n\n')
        buffer = events.pop() || ''

        for (const rawEvent of events) {
          if (!rawEvent.trim()) continue
          // 一行 data: {...}
          const lines = rawEvent.split('\n')
          for (const line of lines) {
            if (!line.startsWith('data: ')) continue
            const payload = line.slice(6).trim()
            if (!payload) continue
            received = true
            try {
              const event = JSON.parse(payload) as AgentEvent
              handleAgentEvent(event)
            } catch (e) {
              console.debug('[useAgent] malformed SSE payload:', payload, e)
            }
          }
        }
      }
      // 处理 trailing buffer（不带 \n\n 结尾的最后一段）
      if (buffer.trim().startsWith('data: ')) {
        const payload = buffer.trim().slice(6).trim()
        if (payload) {
          received = true
          try {
            const event = JSON.parse(payload) as AgentEvent
            handleAgentEvent(event)
          } catch {
            // ignore trailing malformed
          }
        }
      }

      // 流结束：清理未结束的 assistant 消息 + 恢复 idle 状态
      // （防止 server 端未发 stream_end 但连接关闭的边角场景）
      finalizeLastAssistant()
      if (status.value === 'streaming') {
        // 决定 status：是否有 pending 确认？
        const hasPendingConfirm = messages.value.some((m) =>
          m.tool_calls.some((tc) => tc.needsConfirm && tc.status === 'pending'),
        )
        status.value = hasPendingConfirm ? 'confirming' : 'idle'
        saveState()
      }
    } finally {
      try {
        reader.releaseLock()
      } catch {
        // already released
      }
    }

    return { received }
  }

  /**
   * 单个 event type → reactive state dispatch
   */
  function handleAgentEvent(event: AgentEvent): void {
    // 取最后一条 assistant 消息（流式追加目标）
    const lastAssistant = () => {
      for (let i = messages.value.length - 1; i >= 0; i--) {
        if (messages.value[i].role === 'assistant') return messages.value[i]
      }
      // 没有 assistant → 立即创建一条
      const newMsg: Message = {
        role: 'assistant',
        content: '',
        tool_calls: [],
        tool_results: [],
        isStreaming: true,
      }
      messages.value.push(newMsg)
    return messages.value[messages.value.length - 1]
    }

    switch (event.type) {
      case 'text_delta': {
        const m = lastAssistant()
        m.content += parseContentDelta(event.data)
        break
      }
      case 'reasoning_delta': {
        const m = lastAssistant()
        m.reasoning = (m.reasoning || '') + parseContentDelta(event.data)
        break
      }
      case 'tool_call': {
        const tool = parseToolCallData(event.data)
        if (tool) {
          const m = lastAssistant()
          m.tool_calls.push(tool)
        }
        break
      }
      case 'tool_status': {
        const ts = parseToolStatus(event.data)
        if (ts) {
          // 找到对应的 tool_call 并更新 status
          for (const msg of messages.value) {
            const tc = msg.tool_calls.find((t) => t.id === ts.id)
            if (tc) {
              tc.status = ts.status
              break
            }
          }
        }
        break
      }
      case 'tool_result': {
        const result = parseToolResultData(event.data)
        if (result) {
          const m = lastAssistant()
          m.tool_results.push(result)
        }
        break
      }
      case 'stream_end': {
        // 标记最后 assistant 消息流式结束
        for (let i = messages.value.length - 1; i >= 0; i--) {
          if (messages.value[i].role === 'assistant' && messages.value[i].isStreaming) {
            messages.value[i].isStreaming = false
            break
          }
        }
        // 决定下一个 status：
        //   - 如果有 pending tool_call（needsConfirm=true 且 status=pending）→ confirming
        //   - 否则 → idle
        const hasPendingConfirm = messages.value.some((m) =>
          m.tool_calls.some((tc) => tc.needsConfirm && tc.status === 'pending'),
        )
        status.value = hasPendingConfirm ? 'confirming' : 'idle'
        break
      }
      default:
        // 未知 type 静默忽略
        break
    }

    eventOffset++
    saveState()
  }

  // ─── 公共 API ───────────────────────────────────────────────────────────

  /**
   * 发送用户消息，发起对话
   */
  async function send(text: string): Promise<void> {
    if (status.value === 'streaming' || status.value === 'confirming') {
      console.debug('[useAgent] send: ignored (busy)')
      return
    }

    // 关闭上一轮的错误条
    lastError.value = ''
    lastUserInput.value = text

    // 第一次发送：分配新 session
    if (!currentSessionId.value) {
      currentSessionId.value = generateSessionId()
    }
    eventOffset = 0

    // 推 user 消息 + 空 assistant 占位
    messages.value.push({
      role: 'user',
      content: text,
      tool_calls: [],
      tool_results: [],
    })
    messages.value.push({
      role: 'assistant',
      content: '',
      tool_calls: [],
      tool_results: [],
      isStreaming: true,
    })

    status.value = 'streaming'
    saveState()
    refreshSessions()

    abortController = new AbortController()
    // 30s 超时保护：如果后端长时间无响应，自动中断
    let timedOut = false
    const timeoutId = setTimeout(() => {
      timedOut = true
      if (abortController) abortController.abort()
    }, 30_000)

    try {
      console.error('[useAgent] send() starting fetch to', `${AGENT_API_BASE}/api/chat`)
      const response = await fetch(`${AGENT_API_BASE}/api/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          sessionId: currentSessionId.value,
          model: activeModel.value,
          temperature: activeTemperature.value,
          messages: messages.value.map((m) => ({
            role: m.role,
            content: m.content,
          })),
        }),
        signal: abortController.signal,
      })

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText || '请求失败'}`)
      }

      if (!response.body) {
        throw new Error('响应体为空（可能被代理或网络中间层截断）')
      }

      const result = await processSSE(response.body)

      // 流结束但未收到任何事件 → 后端无响应
      if (!result.received) {
        throw new Error('服务端无响应：连接已关闭但未返回任何数据')
      }

      // 收到了事件但 assistant 内容仍为空且无工具调用 → 异常空回复
      const lastAssistant = [...messages.value].reverse().find((m) => m.role === 'assistant')
      if (lastAssistant && !lastAssistant.content && lastAssistant.tool_calls.length === 0) {
        console.error('[useAgent] WARNING: stream ended with empty assistant reply — marking user msg as errored')
        const lastUserMsg = [...messages.value].reverse().find((m) => m.role === 'user')
        if (lastUserMsg) lastUserMsg.error = '服务端返回空回复，请重试'
        status.value = 'idle'
      }
    } catch (e: any) {
      // 找到本次发送的 user 消息，标记错误（每条消息独立错误状态）
      const lastUserMsg = [...messages.value].reverse().find((m) => m.role === 'user')
      if (e?.name === 'AbortError') {
        if (timedOut) {
          console.error('[useAgent] send timed out (30s)')
          if (lastUserMsg) lastUserMsg.error = '请求超时（30秒内服务端无响应），请检查网络或稍后重试'
          status.value = 'idle'
        } else {
          console.debug('[useAgent] send aborted by user')
          // 用户主动停止：不标记错误
        }
        finalizeLastAssistant()
        if (status.value !== 'idle') status.value = 'idle'
      } else {
        const detail = e?.message || String(e)
        console.error('[useAgent] send failed:', detail, e)
        if (lastUserMsg) lastUserMsg.error = detail
        showToast({ message: detail, duration: 3000, color: 'danger' })
        status.value = 'idle'
        finalizeLastAssistant()
      }
    } finally {
      clearTimeout(timeoutId)
      abortController = null
      saveState()
    }
  }

  /**
   * 4-决策确认工具调用
   */
  async function confirmTool(toolCallId: string, decision: Decision): Promise<void> {
    if (!currentSessionId.value) {
      console.debug('[useAgent] confirmTool: no active session')
      return
    }

    if (abortController) {
      abortController.abort()
      abortController = null
    }

    // 找到对应的 tool_call，把它的 status 标记为 'running' 表示处理中
    let targetTool: ToolCall | null = null
    for (const msg of messages.value) {
      const tc = msg.tool_calls.find((t) => t.id === toolCallId)
      if (tc) {
        targetTool = tc
        break
      }
    }
    if (targetTool) {
      // 用户做决策期间：保持 needsConfirm 但 status 标记 'running' 表示处理中
      targetTool.status = 'running'
    }

    status.value = 'streaming'
    saveState()

    abortController = new AbortController()
    try {
      const response = await fetch(`${AGENT_API_BASE}/api/confirm`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          sessionId: currentSessionId.value,
          toolCallId,
          decision,
        }),
        signal: abortController.signal,
      })

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }

      await processSSE(response.body)
    } catch (e: any) {
      if (e?.name === 'AbortError') {
        console.debug('[useAgent] confirmTool aborted')
        if (targetTool) targetTool.status = 'pending'
        status.value = 'confirming'
      } else {
        console.error('[useAgent] confirmTool failed:', e)
        if (targetTool) targetTool.status = 'pending'
        showToast({ message: 'Confirm request failed', duration: 2000, color: 'danger' })
        status.value = 'confirming'
      }
    } finally {
      abortController = null
      saveState()
    }
  }

  /**
   * 启动时自动续传：恢复最近的 session 并继续追平进度
   */
  async function resume(): Promise<void> {
    const sessionId = findLatestPersistedSession()
    if (!sessionId) return

    const saved = loadState(sessionId)
    if (!saved) return

    currentSessionId.value = saved.sessionId
    eventOffset = saved.eventOffset || 0
    // 恢复 messages
    messages.value.splice(0, messages.value.length, ...(saved.messages || []))
    status.value = saved.status || 'idle'

    // 如果上次是 streaming 状态，主动 resume 追平进度
    if (status.value === 'streaming') {
      if (abortController) {
        abortController.abort()
        abortController = null
      }
      abortController = new AbortController()
      try {
        const response = await fetch(`${AGENT_API_BASE}/api/resume`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            sessionId: currentSessionId.value,
            offset: eventOffset,
          }),
          signal: abortController.signal,
        })

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`)
        }
        await processSSE(response.body)
      } catch (e: any) {
        if (e?.name !== 'AbortError') {
          console.error('[useAgent] resume failed:', e)
          finalizeLastAssistant()
          status.value = 'idle'
        }
      } finally {
        abortController = null
        saveState()
      }
    }
  }

  /**
   * 停止当前流式（SSE 连接 abort）
   */
  function stop(): void {
    if (abortController) {
      abortController.abort()
      abortController = null
    }
    finalizeLastAssistant()
    status.value = 'idle'
    saveState()
  }

  /**
   * 重置 session（清空消息、状态、持久化）
   * ⚠️ 旧版是 destroy；现在 newSession 替代 reset 的语义——
   *    reset 仅用于内部紧急回退（UI 入口已切换为 newSession）
   */
  function reset(): void {
    stop()
    if (currentSessionId.value) {
      try {
        localStorage.removeItem(STORAGE_PREFIX + currentSessionId.value)
      } catch {
        // ignore
      }
    }
    currentSessionId.value = ''
    eventOffset = 0
    messages.value.splice(0, messages.value.length)
    status.value = 'idle'
    lastError.value = ''
    lastUserInput.value = ''
    refreshSessions()
  }

  /**
   * 关闭错误条
   */
  function dismissError(): void {
    lastError.value = ''
    if (status.value === 'error') status.value = 'idle'
  }

  /**
   * 重发上一次失败的用户消息
   */
  function retryLast(): void {
    const text = lastUserInput.value
    if (!text) return
    lastError.value = ''
    status.value = 'idle'
    void send(text)
  }

  // 构造时同步一次 session 列表（供 UI 立即显示）
  refreshSessions()

  return {
    messages,
    status,
    send,
    confirmTool,
    resume,
    stop,
    reset,
    newSession,
    switchSession,
    deleteSession,
    refreshSessions,
    sessions,
    currentSessionId,
    lastError,
    dismissError,
    retryLast,
    activeModel,
    activeTemperature,
  }
}
