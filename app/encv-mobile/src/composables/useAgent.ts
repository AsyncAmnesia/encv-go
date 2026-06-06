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
import { getDeviceIdSync } from './useDeviceId'
import { getAgentApiBase } from './useAgentApiBase'
import {
  serializeAttachments,
  type Attachment,
  type MessageContentPart,
} from './useAttachments'

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
  | 'stream_status'  // 后端流式状态（断点续传时推 synced / more_pending）
  | 'stream_end'
  // 上下文自动压缩事件。Task 7 引入：后端在 messages token 数
  // 越过窗口 80% 时调用 LLM summary 压缩老消息，并推送本事件。
  // 前端收到时插入一条 role='system', content='上下文已自动压缩'
  // 的合成消息（renderTurnItems 把它转成 ContextCompactionDivider）。
  // 这是 7 种 event type 中的第 7 种，原有 6 种契约不变。
  | 'compaction'

/** Agent 推送到 SSE channel 的事件 */
export interface AgentEvent {
  type: AgentEventType
  /** JSON string —— 前端按 type 自行反序列化 */
  data: string
}

export type ToolKind = 'command' | 'fileChange' | 'readOnly' | 'webSearch' | 'plan' | 'unknown'
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
  role: 'user' | 'assistant' | 'system'
  /**
   * Task 12：附件场景下 content 可能是 OpenAI multimodal 数组
   * （text / image_url / file 元素）。老消息（无附件）保持 string。
   * 持久化层 JSON.parse 后这个 union 仍能正确还原。
   */
  content: string | MessageContentPart[]
  reasoning?: string
  tool_calls: ToolCall[]
  tool_results: ToolResult[]
  isStreaming?: boolean
  /**
   * 发送失败时的错误信息（每条消息独立）
   */
  error?: string
  /**
   * Task 11 (Steer / Queue)：当用户点击「排队下一条」时，
   * 该 user 消息进入 pendingMessages 队列，等待当前 turn
   * 完全结束后由服务端 drain hook 触发新一轮 Chat。pending=true
   * 时 UI 应展示「已排队」标签。首个 text_delta 事件到达时
   * 自动清除（说明服务端已开始处理该消息）。
   */
  pending?: boolean
}

/**
 * Task 7 引入的「上下文已自动压缩」标记文本。后端在
 * EventCompaction 事件里推送一份 LLM 生成的 summary，前端把它
 * 包裹成一条 role='system' 的合成消息插入 messages 列表。renderTurnItems
 * 检测到 message.role === 'system' && content === CONTEXT_COMPACTION_MARKER
 * 时产出 compaction 类型的 RenderedItem，由 AgentChat.vue
 * 渲染为不可展开的 ContextCompactionDivider 分隔线。
 *
 * 该 marker 是 *单向契约*：前端在 useAgent.ts 收到 compaction 事件
 * 时构造，后端不会直接发这个字符串。如果后端在 messages 数组里看到
 * 这条「system:上下文已自动压缩」消息，会被忽略（不会再次触发压缩）。
 */
export const CONTEXT_COMPACTION_MARKER = '上下文已自动压缩'

// =============================================================================
// 常量
// =============================================================================

/** 持久化到 localStorage 的 key 前缀 */
const STORAGE_PREFIX = 'agent:session:'

/** Agent 服务 API 路径（dev 走 preview-gateway :16666 → :2025；APK 直接 :2025） */
const AGENT_API_BASE = getAgentApiBase()

/**
 * 单实例最多追踪的 SSE sequence 编号数。超过此上限时按插入顺序
 * 淘汰最老的 sequence 编号（FIFO 近似 LRU）。参考 codex-web
 * `appServerRealtimeReducer.ts` 的 `MAX_TRACKED_REALTIME_SEQUENCES` 常量。
 */
const MAX_TRACKED_REALTIME_SEQUENCES = 2_000

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
 * 后端格式：json.Marshal(plainString) → 经外层 JSON 解码后 data 就是纯文本
 * 兼容旧格式：{"content": "..."} 包装
 */
function parseContentDelta(data: string): string {
  if (!data) return ''
  // 尝试作为 JSON 解析（兼容 {"content":"..."} 或包装字符串格式）
  try {
    const parsed = JSON.parse(data)
    if (typeof parsed === 'string') return parsed
    if (parsed && typeof parsed === 'object' && 'content' in parsed) {
      return String((parsed as { content: unknown }).content ?? '')
    }
  } catch {
    // 不是有效 JSON → 后端发的是纯文本（当前 encv-go stub 格式），直接使用
  }
  return data
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

/**
 * 解析 `stream_status` 的 data 字段 —— StreamStatusData
 * 后端格式（internal/server/agent_api.go::sendAndCache stream_status 分支）：
 *   { "status": "synced" | "more_pending", "inProgress": bool, "maxEventId"?: number }
 *
 * 此类型在 D.2 引入：断点续传链路的前端信号
 */
export interface StreamStatusData {
  status: 'synced' | 'more_pending' | string
  inProgress?: boolean
  maxEventId?: number
}

function parseStreamStatusData(data: string): StreamStatusData | null {
  try {
    const parsed = JSON.parse(data) as Partial<StreamStatusData>
    if (!parsed || typeof parsed.status !== 'string') return null
    return {
      status: parsed.status,
      inProgress: parsed.inProgress,
      maxEventId: typeof parsed.maxEventId === 'number' ? parsed.maxEventId : undefined,
    }
  } catch {
    return null
  }
}

/**
 * CompactionData 是后端 EventCompaction 事件的 payload 形状（Task 7）：
 *   {
 *     summary_text: string,            // LLM 生成的 summary
 *     replaced_message_count: number,  // 被替换的旧消息数
 *     triggered_at_ms: number,          // unix 毫秒时间戳
 *   }
 *
 * parseCompactionData 容忍后端包装格式（data 字段是 JSON 字符串）：
 *   - 直接 JSON.parse(data) 拿到 CompactionData
 *   - 旧后端可能把整个 CompactionData 直接 stringify 在 data 里
 *   - 极端场景：data 损坏 → 返回 null，让上层用 fallback 行为
 */
export interface CompactionData {
  summary_text?: string
  replaced_message_count?: number
  triggered_at_ms?: number
}

function parseCompactionData(data: string): CompactionData | null {
  if (!data) return null
  try {
    const parsed = JSON.parse(data) as Partial<CompactionData>
    if (!parsed || typeof parsed !== 'object') return null
    return {
      summary_text: typeof parsed.summary_text === 'string' ? parsed.summary_text : undefined,
      replaced_message_count:
        typeof parsed.replaced_message_count === 'number' ? parsed.replaced_message_count : undefined,
      triggered_at_ms: typeof parsed.triggered_at_ms === 'number' ? parsed.triggered_at_ms : undefined,
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

/**
 * Task 12：从 Message.content 抽取会话标题用的纯文本。
 *  - string  → 第一行前 40 字符
 *  - array   → 取首个 text 元素
 *  - empty   → 空串
 */
function extractUserTitle(content: string | MessageContentPart[] | undefined): string {
  if (!content) return ''
  if (typeof content === 'string') {
    return content.split('\n')[0]?.slice(0, 40) || ''
  }
  // multimodal 数组：找第一个 text 元素
  for (const part of content) {
    if (part.type === 'text' && part.text) {
      return part.text.split('\n')[0]?.slice(0, 40) || ''
    }
  }
  // 全是附件没文本：返回一个占位提示
  return '[附件]'
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
  /**
   * Task 11 (Steer / Queue)：跟踪已通过「排队下一条」按钮提交、
   * 但服务端尚未开始处理的 user 消息。pendingMessages 数组的元素
   * 是 messages.value 中对应 user 消息的引用（同一对象），这样
   * UI 可以直接从 pendingMessages 取出 text 渲染「已排队：xxx」
   * 提示，并在服务端开始处理时（首个 text_delta 到达）实时清除
   * 对应条目的 pending 标记。
   */
  const pendingMessages = ref<Message[]>([])
  /** 本地已处理事件计数（与 lastEventId 解耦：eventOffset 永远递增，lastEventId 由 SSE id 行决定） */
  let eventOffset = 0
  /**
   * SSE 标准 Last-Event-ID：服务端为每个事件分配的全局递增 ID。
   * 用于断点续传——前端解析 `id: N` 行维护此字段，resume() 时回传给后端。
   * 0 = 尚未收到任何事件 id。
   */
  let lastEventId = 0
  let abortController: AbortController | null = null

  // ─── Server Instance + Sequence 去重（Task 4） ────────────────────────
  // 后端 /api/health 返回 process-wide 唯一的 serverInstanceId。同一进程
  // 启动期间它恒定；进程重启（OS 分配新 PID / 启动时间不同）就会变化。
  // 前端每次拉取发现 instance 变化时，必须清空 seenSequences——因为新进程
  // 的 SSE sequence 编号可能与旧进程"撞号"，不复用旧的去重集合会造成
  // 真实事件被错误丢弃。
  /** 当前进程已知的 serverInstanceId；空串 = 尚未拉取过 /api/health */
  let currentServerInstance = ''
  /** 已见 SSE sequence 编号集合；超过 MAX_TRACKED_REALTIME_SEQUENCES 时按 FIFO 驱逐 */
  const seenSequences = new Set<number>()
  /** 配合 Set 实现的 FIFO 驱逐顺序表 */
  const seenSequencesOrder: number[] = []

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

  /**
   * 拉取后端 /api/health 取 serverInstanceId，并按规则同步到本地状态：
   *   - 拉取失败 / 解析失败 / 字段缺失 → 静默保留 currentServerInstance 不动
   *     （fallback 到空串 + console.warn 提示，但不抛错）
   *   - 拉到新 id 且 ≠ currentServerInstance → 清空 seenSequences（关键！新进程
   *     编号从 1 开始，旧 instance 的 sequence 集合必须丢弃避免误丢事件）
   *   - 拉到与 currentServerInstance 相同的 id → 保持 sequence 去重集合不变
   *
   * 安全：仅在 init() / send() 入口处被调用；不会在 SSE 流过程中触发，
   * 所以不会与正在进行的 sequence 编号检查产生竞态。
   */
  async function refreshServerInstance(): Promise<void> {
    try {
      const response = await fetch(`${AGENT_API_BASE}/api/health`, { method: 'GET' })
      if (!response.ok) {
        console.warn('[useAgent] refreshServerInstance: /api/health returned', response.status)
        return
      }
      const data = await response.json()
      const newId = typeof data?.serverInstanceId === 'string' ? data.serverInstanceId : ''
      if (!newId) {
        console.warn('[useAgent] refreshServerInstance: response missing serverInstanceId')
        return
      }
      if (newId !== currentServerInstance) {
        console.debug('[useAgent] server instance changed:', currentServerInstance || '(none)', '->', newId)
        currentServerInstance = newId
        // 清空去重状态：旧 instance 的 sequence 编号集合在物理上属于旧进程，
        // 复用它们会导致新进程的真实事件被错误判为重复。
        seenSequences.clear()
        seenSequencesOrder.length = 0
      }
    } catch (e) {
      // 网络/CORS 等错误：保留旧值不丢业务，但提示一次
      console.warn('[useAgent] refreshServerInstance: fetch /api/health failed:', e)
    }
  }

  /**
   * 记录一个 sequence 编号为"已见"。超过 MAX_TRACKED_REALTIME_SEQUENCES
   * 时按插入顺序淘汰最老的编号。返回 true 表示"新增"（未见过），false
   * 表示"重复"（已在集合中）。
   */
  function rememberSequence(seq: number): boolean {
    if (seenSequences.has(seq)) {
      return false
    }
    seenSequences.add(seq)
    seenSequencesOrder.push(seq)
    if (seenSequencesOrder.length > MAX_TRACKED_REALTIME_SEQUENCES) {
      const evict = seenSequencesOrder.shift()
      if (evict !== undefined) seenSequences.delete(evict)
    }
    return true
  }

  // ─── 持久化 ─────────────────────────────────────────────────────────────

  function saveState() {
    if (!currentSessionId.value) return
    try {
      const payload = {
        sessionId: currentSessionId.value,
        eventOffset,
        lastEventId,
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
    /** 兼容老存档：无 lastEventId 字段时默认为 0 */
    lastEventId?: number
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
          // Task 12：content 可能是 multimodal 数组（带附件的 user 消息），
          //          从中抽出首段 text 元素作为会话标题。
          const title = extractUserTitle(firstUser?.content) || '(空会话)'
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
    lastEventId = 0
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
      lastEventId = 0
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
   *
   * 支持 SSE 标准 `id: N` 字段（后端断点续传时用），用于维护 lastEventId。
   * 多个 `id:` 行在同一事件中以最后一个为准。
   *
   * 返回结构：
   *   - received:      是否收到过任何 data 事件
   *   - streamEnded:   是否收到过 stream_end 事件（用于 runResumeChain 决定是否链式续传）
   *   - morePending:   最后一个有意义事件是否为 stream_status.more_pending
   *                    （若 true 且 !streamEnded，runResumeChain 应继续下一轮）
   */
  async function processSSE(stream: ReadableStream<Uint8Array> | null): Promise<{
    received: boolean
    streamEnded: boolean
    morePending: boolean
  }> {
    if (!stream) return { received: false, streamEnded: false, morePending: false }

    const reader = stream.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let received = false
    let streamEnded = false
    /** 最后一个 stream_status 事件：synced 还是 more_pending（用于 chain 决策） */
    let lastStreamStatus: 'synced' | 'more_pending' | null = null

    /**
     * 处理一个完整 SSE 事件（已被 \n\n 分隔）
     * 维护 currentEventId 用于关联同一事件内的 data 行
     */
    function consumeEvent(rawEvent: string): void {
      if (!rawEvent.trim()) return
      let currentEventId: number | null = null
      const lines = rawEvent.split('\n')
      for (const line of lines) {
        // SSE 标准：id: N —— 用于断点续传
        if (line.startsWith('id:')) {
          const n = parseInt(line.slice(3).trim(), 10)
          if (Number.isFinite(n) && n >= 0) currentEventId = n
          continue
        }
        if (!line.startsWith('data: ')) continue
        const payload = line.slice(6).trim()
        if (!payload) continue
        received = true
        try {
          const event = JSON.parse(payload) as AgentEvent
          // Task 4.3：sequence 去重。若事件声明了 SSE id (currentEventId)，
          // 且该 id 已在当前 serverInstance 见过 → 整条事件丢弃，不再 dispatch。
          // 注意：未声明 id 的事件（如后端断点续传 stream_status 边界事件）跳过
          // 去重逻辑，避免误丢无 id 的合法事件。
          if (currentEventId !== null) {
            if (!rememberSequence(currentEventId)) {
              console.debug('[useAgent] drop duplicate seq', currentEventId)
              // 重置 currentEventId 避免下一行误用
              currentEventId = null
              continue
            }
            // 关联 SSE id：若同一事件声明了 id，则覆盖到 lastEventId
            // （一个事件只能有一个 data，多个 id 行以最后一个为准）
            lastEventId = currentEventId
            // 重置 currentEventId 避免下一行误用
            currentEventId = null
          }
          if (event.type === 'stream_end') streamEnded = true
          if (event.type === 'stream_status') {
            const payload = parseStreamStatusData(event.data)
            if (payload?.status === 'synced' || payload?.status === 'more_pending') {
              lastStreamStatus = payload.status
            }
          }
          handleAgentEvent(event)
        } catch (e) {
          console.debug('[useAgent] malformed SSE payload:', payload, e)
        }
      }
    }

    try {
      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        // SSE 事件以 \n\n 分隔
        const events = buffer.split('\n\n')
        buffer = events.pop() || ''

        for (const rawEvent of events) {
          consumeEvent(rawEvent)
        }
      }
      // 处理 trailing buffer（不带 \n\n 结尾的最后一段）
      if (buffer.trim()) {
        consumeEvent(buffer)
        buffer = ''
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

    return {
      received,
      streamEnded,
      morePending: lastStreamStatus === 'more_pending',
    }
  }

  /**
   * 单个 event type → reactive state dispatch
   */
  function handleAgentEvent(event: AgentEvent): void {
    // 取最后一条 *正在 streaming* 的 assistant 消息作为流式追加目标。
    //
    // Task 7 之前：实现是"最后一条 assistant 消息"，因为 send() 每次
    // 都会推一条新的空 assistant(isStreaming=true)，老的已经
    // 被 stream_end / finalizeLastAssistant 标记为 isStreaming=false。
    //
    // Task 7 之后：compaction 事件会在 messages 中插入 system marker
    // 标记，*并 finalize 掉当前 streaming 的 assistant*。如果下一个
    // text_delta 仍走"最后一条 assistant 消息"路径，它会找到刚被
    // finalize 的老 assistant，把 post-compaction 的文本 append 到
    // pre-compaction 文本后面，marker 漂到消息尾部。
    //
    // 修正：把"messages 尾部为 system 消息"作为 turn 边界的硬信号——
    // 说明上一个 turn 已被边界事件（compaction、未来的 user 切分等）
    // 显式终结，后续 text_delta 应当开一条新 assistant。resume 路径
    // 不触发此分支（resume 不会在 messages 尾部插入 system 消息），
    // 因此原"追加到最后一条 assistant"行为保留。
    const lastAssistant = (): Message => {
      // Turn 边界检测：尾部是 system 消息 → 必须新开 assistant
      const tail = messages.value[messages.value.length - 1]
      const isTurnBoundary = tail && tail.role === 'system'
      if (!isTurnBoundary) {
        // 非 turn 边界：优先 streaming → fallback 到任何 assistant
        for (let i = messages.value.length - 1; i >= 0; i--) {
          const m = messages.value[i]
          if (m.role === 'assistant' && m.isStreaming) return m
        }
        for (let i = messages.value.length - 1; i >= 0; i--) {
          if (messages.value[i].role === 'assistant') return messages.value[i]
        }
      }
      // turn 边界 or 没有 assistant → 创建新 assistant
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
      case 'stream_status': {
        // 后端断点续传信号：
        //   { status: 'synced',       inProgress: bool, maxEventId?: number }
        //   { status: 'more_pending', inProgress: bool, maxEventId?: number }
        // 前端拿到信号后由 runResumeChain 决定下一轮，handleAgentEvent 本身
        // 不修改 status（status 由 stream_end / confirm 决策驱动），但要：
        //   1. 同步服务端声明的 maxEventId 到 lastEventId（覆盖，避免漂移）
        //   2. 通过 console.debug 留痕便于排查
        const payload = parseStreamStatusData(event.data)
        if (payload?.maxEventId !== undefined) {
          if (payload.maxEventId > lastEventId) {
            lastEventId = payload.maxEventId
          }
        }
        if (payload?.status === 'synced') {
          console.debug('[useAgent] stream_status: synced, awaiting new events')
        } else if (payload?.status === 'more_pending') {
          console.debug('[useAgent] stream_status: more_pending, will re-resume')
        } else {
          console.debug('[useAgent] stream_status:', payload)
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
        // Task 11：每个 stream_end 清掉队首 pending 消息。
        // 时序说明：当 in-flight turn 结束时，stream_end 触发
        // 队首清空（即使这个 turn 实际处理的是该消息，UI 端
        // 「已排队」标签与实际处理节拍同步）；后续 queued turn
        // 各自的 stream_end 同样按 FIFO 清队。
        if (pendingMessages.value.length > 0) {
          const first = pendingMessages.value.shift()!
          first.pending = false
        }
        break
      }
      case 'compaction': {
        // Task 7：上下文自动压缩事件。
        //
        // 后端在 messages token 数越过 80% 窗口阈值时，调用 LLM
        // 生成老消息的 summary，并在 messages 头部插入一条
        // role='system', name='compaction' 的合成消息。
        // 本事件只是告知前端"这里发生了压缩"，附带：
        //   - summary_text:   LLM 生成的 summary 文本（前端不渲染）
        //   - replaced_message_count: 被替换的旧消息数
        //   - triggered_at_ms: unix 毫秒时间戳
        //
        // 前端在 messages 列表里也插入一条 role='system' 的合成
        // 消息，content 固定为 CONTEXT_COMPACTION_MARKER。
        // renderTurnItems 会把这条消息转成 ContextCompactionDivider
        // （不可展开的水平分隔线），位置恰好对应被压缩掉的旧
        // 消息所在位置。summary 文本本身不展示——压缩是不可逆的，
        // 用户看到「这里有 N 条消息被合并为一段总结」即可。
        //
        // 关键时序：先 finalize 当前 streaming 的 assistant 消息
        // （mark isStreaming=false），再 push system marker。如果
        // 不 finalize，下一个 text_delta 仍会找到这条"还在流式"
        // 的老 assistant，把 post-compaction 的内容 append 上去，
        // marker 漂到消息尾部——视觉上分隔线就不在压缩点的位置了。
        // 配合 Task 7 改的 lastAssistant 行为（只取 streaming
        // assistant），post-compaction 的 text_delta 会创建一条新
        // 的 assistant 消息，marker 自然落在两条 assistant 之间。
        //
        // 容错：compaction 事件不应让 status 切换；不影响
        // streaming 状态。重复到达时直接再 push 一条 marker
        // （后端保证只发一次）。
        finalizeLastAssistant()
        const data = parseCompactionData(event.data)
        // 不论 data 是否解析成功都插入 marker（即使解析失败也
        // 给用户一个 divider 提示此处发生了压缩）。
        messages.value.push({
          role: 'system',
          content: CONTEXT_COMPACTION_MARKER,
          tool_calls: [],
          tool_results: [],
        })
        if (data) {
          console.debug(
            '[useAgent] compaction: replaced',
            data.replaced_message_count,
            'messages @',
            new Date(data.triggered_at_ms).toISOString(),
          )
        }
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
   *
   * Task 11 (Steer / Queue)：新增 `mode` 选项，支持三种发送模式：
   *   - "start"  默认行为：常规发起一轮新 turn，blocking 等待 SSE 流。
   *   - "steer"  与 start 等价的 server-side 行为，语义上标记"修正当前
   *              turn"。在 UI 上用「引导当前」按钮触发，行为对用户透明。
   *   - "queue"  排队下一条：仅在 status='streaming' 时合法。消息被
   *              加入 pendingMessages 队列，立即在 messages.value 中
   *              渲染（带 pending 标记），同时 POST /api/chat with
   *              mode='queue'。服务端在当前 turn 完全结束后由 drain
   *              hook 启动新一轮 Chat，结果通过现有 SSE 连接回流。
   *
   * Task 12：可选接受 `attachments`（图片 / 文件）。如果提供，会和 text
   * 一起被编码成 OpenAI multimodal content 数组塞进 Message.content
   * （参见 useAttachments.serializeAttachments）。
   */
  async function send(
    text: string,
    options?: { mode?: 'start' | 'steer' | 'queue'; attachments?: Attachment[] },
  ): Promise<void> {
    const mode = options?.mode ?? 'start'
    if (mode === 'queue') {
      await sendQueued(text, options?.attachments ?? [])
      return
    }

    if (status.value === 'streaming' || status.value === 'confirming') {
      console.debug('[useAgent] send: ignored (busy)')
      return
    }

    // 关闭上一轮的错误条
    lastError.value = ''
    lastUserInput.value = text

    // Task 4：先拉取 /api/health 取 serverInstanceId。每次 send 都刷一次——
    // 开销可忽略（一次 GET，且 health 不带 SSE body），但能在"中途服务器
    // 重启"时立刻把 seenSequences 清空，避免后续事件被错误判为重复。
    await refreshServerInstance()

    // 第一次发送：分配新 session
    if (!currentSessionId.value) {
      currentSessionId.value = generateSessionId()
    }
    eventOffset = 0
    lastEventId = 0

    // 推 user 消息 + 空 assistant 占位
    // Task 12：如果有 attachments，content 编为 multimodal 数组；
    //          否则保持原样（纯字符串）以保持向后兼容。
    const attachments = options?.attachments ?? []
    const userContent: string | MessageContentPart[] =
      attachments.length > 0
        ? serializeAttachments(text, attachments)
        : text

    messages.value.push({
      role: 'user',
      content: userContent,
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

    // ── 构建消息列表：历史对话（system prompt 由后端从 config 注入） ──
    // Task 12：content 可能是 string 或 multimodal 数组，原样透传。
    const apiMessages: Array<{ role: string; content: string | MessageContentPart[] }> = []

    // 追加历史消息（不含空的 assistant 占位消息）。
    // Task 7：跳过 role='system' + content=CONTEXT_COMPACTION_MARKER
    // 的合成消息——后端在触发压缩时已经把这段 summary 写进了
    // 自己的 session 持久化层，前端再回送一份就是重复。
    for (const m of messages.value) {
      if (m.role === 'assistant' && !m.content && !m.reasoning && m.tool_calls.length === 0) continue
      // Task 7：跳过 system + CONTEXT_COMPACTION_MARKER 的合成消息。
      // Task 12：content 可能是 multimodal 数组，这种情况下不可能 ==
      // CONTEXT_COMPACTION_MARKER（string），TS 也允许此比较但用 typeof
      // 守卫更安全。
      if (m.role === 'system' && typeof m.content === 'string' && m.content === CONTEXT_COMPACTION_MARKER) continue
      apiMessages.push({ role: m.role, content: m.content })
    }

    try {
      console.debug('[useAgent] send() starting fetch to', `${AGENT_API_BASE}/api/chat`, 'mode=', mode)
      const response = await fetch(`${AGENT_API_BASE}/api/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          sessionId: currentSessionId.value,
          model: activeModel.value,
          temperature: activeTemperature.value,
          messages: apiMessages,
          deviceId: getDeviceIdSync(),
          // Task 11：把 mode 字段透传给后端。后端 ChatMode 根据 mode
          // 走 start/steer/queue 三种分支（"" 视为 start）。
          mode,
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
   * Task 11：「排队下一条」按钮的内部实现。仅在 status='streaming' 或
   * 'confirming' 时合法。流程：
   *   1. 推一条带 pending=true 的 user 消息到 messages.value，并
   *      加入 pendingMessages 队列（同一对象引用，便于 UI 双向引用）。
   *   2. POST /api/chat with mode='queue'，body 包含完整消息列表
   *      （含刚 push 的 user 消息，与 start/steer 一致）。
   *   3. 期望服务端返回 202 Accepted —— 不读取 SSE（queued turn 的
   *      事件会通过现有 SSE 连接回流，由 handleAgentEvent 接管）。
   *   4. 出错时把 user 消息从 pendingMessages 移除并标记 error。
   *
   * 注意：排队期间不打断当前 SSE 流、不创建新 assistant 占位——等
   * 排队的 turn 真正启动时，由 handleAgentEvent 的 lastAssistant()
   * 在首个 text_delta 时按需创建。
   */
  async function sendQueued(text: string, attachments: Attachment[]): Promise<void> {
    if (status.value !== 'streaming' && status.value !== 'confirming') {
      console.debug('[useAgent] sendQueued: ignored (no active turn to queue after)')
      return
    }
    if (!currentSessionId.value) {
      console.debug('[useAgent] sendQueued: ignored (no active session)')
      return
    }

    lastError.value = ''

    // Task 12：multimodal content 数组 vs 纯字符串
    const userContent: string | MessageContentPart[] =
      attachments.length > 0
        ? serializeAttachments(text, attachments)
        : text

    // Push user 消息（带 pending=true），并把它登记到 pendingMessages。
    // 两个数组共享同一对象引用：messages.value 渲染聊天气泡，
    // pendingMessages 暴露给 UI 渲染「已排队：xxx」提示。
    const userMsg: Message = {
      role: 'user',
      content: userContent,
      tool_calls: [],
      tool_results: [],
      pending: true,
    }
    messages.value.push(userMsg)
    pendingMessages.value.push(userMsg)
    saveState()
    refreshSessions()

    // 复用 send() 的 apiMessages 构建逻辑（不能调 send() 本身——
    // 那会重置 eventOffset 并破坏当前 SSE 流的状态机）。
    const apiMessages: Array<{ role: string; content: string | MessageContentPart[] }> = []
    for (const m of messages.value) {
      if (m.role === 'assistant' && !m.content && !m.reasoning && m.tool_calls.length === 0) continue
      if (m.role === 'system' && typeof m.content === 'string' && m.content === CONTEXT_COMPACTION_MARKER) continue
      apiMessages.push({ role: m.role, content: m.content })
    }

    try {
      console.debug('[useAgent] sendQueued() POST /api/chat mode=queue')
      const response = await fetch(`${AGENT_API_BASE}/api/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          sessionId: currentSessionId.value,
          model: activeModel.value,
          temperature: activeTemperature.value,
          messages: apiMessages,
          deviceId: getDeviceIdSync(),
          mode: 'queue',
        }),
      })
      // queue 模式服务端约定返回 202 Accepted（chat.go 中由
      // HandleChat 在 mode=='queue' 时显式 WriteHeader 202）。
      if (response.status !== 202) {
        throw new Error(`HTTP ${response.status}: ${response.statusText || '排队失败'}`)
      }
      console.debug('[useAgent] sendQueued: 202 Accepted, message parked on server')
    } catch (e: any) {
      const detail = e?.message || String(e)
      console.error('[useAgent] sendQueued failed:', detail, e)
      // 出错：把 user 消息从 pendingMessages 摘除并标记 error（保留在
      // messages.value 以便用户看到失败提示）。
      const idx = pendingMessages.value.indexOf(userMsg)
      if (idx !== -1) pendingMessages.value.splice(idx, 1)
      userMsg.error = '排队失败：' + detail
      userMsg.pending = false
      showToast({ message: '排队失败', duration: 3000, color: 'danger' })
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
   *
   * 协议：
   *   ① 优先用 SSE 标准 `Last-Event-ID` HTTP header（与 EventSource 一致）
   *   ② 同时在 body 携带 `lastEventId` 字段（兼容 Go handler 当前解析路径）
   *   ③ 处理后端 `stream_status` 事件：
   *      - synced: 服务端已追平，等待新事件 → 短间隔轮询
   *      - more_pending: 服务端还有事件未推完 → 立即发起下一轮 resume
   *   ④ 收尾：服务端流自然结束（stream_end）→ 状态切回 idle
   */
  async function resume(): Promise<void> {
    const sessionId = findLatestPersistedSession()
    if (!sessionId) return

    const saved = loadState(sessionId)
    if (!saved) return

    currentSessionId.value = saved.sessionId
    eventOffset = saved.eventOffset || 0
    lastEventId = saved.lastEventId || 0
    // 恢复 messages
    messages.value.splice(0, messages.value.length, ...(saved.messages || []))
    status.value = saved.status || 'idle'

    // 如果上次是 streaming 状态，主动 resume 追平进度
    if (status.value === 'streaming') {
      await runResumeChain()
    }
  }

  /**
   * 实际跑一轮 resume，处理 `stream_status` 事件决定是否链式续传。
   * 与 resume() 分离是为了支持 stream_status.more_pending 时递归调用。
   */
  async function runResumeChain(maxHops = 32): Promise<void> {
    if (abortController) {
      abortController.abort()
      abortController = null
    }
    abortController = new AbortController()
    const controller = abortController
    let hopsLeft = maxHops
    try {
      // 链式 resume：每轮处理 processSSE 的信号决定下一步
      //   - streamEnded=true       → 退出循环（服务端显式 stream_end）
      //   - morePending=true       → 立刻下一轮（hopsLeft-- 防止无限递归）
      //   - 完全没收到事件          → 退出（流自然关闭且没新事件，等同于 synced）
      //   - 收到 synced            → 退出（保持 streaming 状态由后续触发）
      //   - status 切到非 streaming → 退出
      while (hopsLeft-- > 0) {
        const headerLastEventId = lastEventId > 0 ? String(lastEventId) : undefined
        const response = await fetch(`${AGENT_API_BASE}/api/resume`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            // SSE 标准 Last-Event-ID：与 EventSource 协议一致
            ...(headerLastEventId ? { 'Last-Event-ID': headerLastEventId } : {}),
          },
          body: JSON.stringify({
            sessionId: currentSessionId.value,
            // body 字段名：与后端 handleAgentResume 的 lastEventId 字段对齐
            // （不再使用旧的 offset 字段）
            lastEventId: lastEventId,
          }),
          signal: controller.signal,
        })

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`)
        }

        const { received, streamEnded, morePending } = await processSSE(response.body)

        // 收尾判定
        if (streamEnded) break                  // 服务端显式收尾 → 退出
        if (morePending) continue               // 服务端还有事件未推完 → 立刻下一轮
        if (!received) break                    // 流自然关闭且无事件 → 退出
        if (status.value !== 'streaming') break // 已被 confirm/cancel 切走 → 退出
        // 收到事件但 neither morePending nor streamEnded：保守退出，
        // 避免无限循环（实际上 server 应该会推 stream_status 或 stream_end）
        break
      }
    } catch (e: any) {
      if (e?.name !== 'AbortError') {
        console.error('[useAgent] resume failed:', e)
        finalizeLastAssistant()
        status.value = 'idle'
      }
    } finally {
      // 仅当 controller 没被换掉时才清空（避免覆盖新一轮 runResumeChain 的 controller）
      if (abortController === controller) {
        abortController = null
      }
      saveState()
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
    // Task 11 (Steer / Queue)：UI 用它渲染「已排队：xxx」提示。
    pendingMessages,
    // Task 4：以下为测试专用钩子。生产代码不应调用——所有 serverInstance
    // 同步都由 useAgent 内部 await refreshServerInstance() 完成。
    __refreshServerInstanceForTest: refreshServerInstance,
    __getServerInstanceForTest: () => currentServerInstance,
    __setServerInstanceForTest: (id: string) => {
      currentServerInstance = id
    },
    __getSeenSequencesForTest: () => seenSequencesOrder.slice(),
  }
}
