/**
 * renderTurnItems - 把 messages 数组转换为渲染块
 * 参照 codex_web renderTurnItems()
 * 累积 operationGroup（command/fileChange/toolOutput）和 webSearchGroup
 * flush 时按 group 类型返回不同的渲染规格
 *
 * 输入：messages 数组（来自 useAgent）+ status（'idle' | 'streaming' | 'confirming'）
 * 输出：RenderedItem[] - 每项包含 type + data，AgentChat 据此分发到不同组件
 *
 * 适配 useAgent.ts 的 Message 类型：
 * - Message.role: 'user' | 'assistant'
 * - Message.content: string
 * - Message.reasoning?: string
 * - Message.tool_calls: ToolCall[]
 * - Message.tool_results: ToolResult[]
 * - Message.isStreaming?: boolean
 * - ToolCall.kind: 'command' | 'fileChange' | 'readOnly' | 'unknown'
 * - ToolCall.needsConfirm: boolean（替代 requiresApproval）
 * - ToolCall.status: 'pending' | 'running' | 'success' | 'failed' | 'cancelled'
 */

import { computed, type ComputedRef, type Ref } from 'vue'
import {
  type Message,
  type AgentStatus,
  type ToolCall,
  CONTEXT_COMPACTION_MARKER,
} from './useAgent'

/** 单条 todo，源自 plan tool (write_todos) 的 args JSON。 */
export interface PlanTodo {
  id: string
  status: 'pending' | 'in_progress' | 'completed' | string
  content: string
}

/** 单条渲染项 - 由 AgentChat 分发到对应组件 */
export type RenderedItem =
  | { type: 'user'; messageId: string; text: string }
  | { type: 'assistantText'; messageId: string; text: string; streaming: boolean }
  | { type: 'approval'; toolCallId: string; messageId: string }
  | { type: 'operationGroup'; messageId: string; toolCallIds: string[]; forceComplete: boolean }
  | { type: 'webSearchGroup'; messageId: string; queries: string[]; toolCallIds: string[] }
  | { type: 'reasoning'; messageId: string; text: string; streaming: boolean }
  | { type: 'error'; messageId: string; text: string; messageIndex: number }
  | { type: 'plan'; messageId: string; toolCallId: string; todos: PlanTodo[]; streaming: boolean }
  // Task 7：上下文自动压缩分隔线。position 字段是分隔线在
  // RenderedItem[] 数组中的下标（用 idx 衍生），用于 key
  // 生成；text 是 i18n 文本（"上下文已自动压缩"）。
  | { type: 'compaction'; messageId: string; text: string }

/**
 * useRenderTurnItems - 组合式接口
 * 入参：messages ref + status ref
 * 返回：RenderedItem[] computed
 *
 * Task 7：可选第三个参数 compactionText —— i18n 键解析后的
 * "上下文已自动压缩" 文本。传空串时直接用 CONTEXT_COMPACTION_MARKER
 * 兜底（中文 hardcode），保证 renderTurnItems 在没有 i18n 调用上下文
 * （如 unit test）的场景下也能产出可读的 divider。
 */
export function useRenderTurnItems(
  messages: Ref<Message[]> | ComputedRef<Message[]>,
  status: Ref<AgentStatus> | ComputedRef<AgentStatus>,
  compactionText?: ComputedRef<string> | Ref<string>,
): ComputedRef<RenderedItem[]> {
  return computed(() =>
    renderTurnItems(messages.value, status.value, compactionText?.value),
  )
}

/** 累积状态 */
interface OpGroup {
  anchorId: string
  toolCallIds: string[]
  kinds: string[]
  lastStatus: string | null
}

interface WebSearchGroup {
  anchorId: string
  queries: string[]
  toolCallIds: string[]
}

// 8 个合并窗常量（已删除，模板里不直接用）
//   旧代码: const FLUSH_GAP_MS = 800

/**
 * renderTurnItems 纯函数
 * 1. 单条 user / assistant / error / reasoning → 直接产出 item
 * 2. 累积连续 toolCall（command/fileChange/readOnly）→ operationGroup
 * 3. 累积连续 web_search 类的 toolCall → webSearchGroup
 * 4. 流式结束或超过 FLUSH_GAP_MS 时强制 flush
 *
 * 这里按"消息 + 该消息内 toolCalls 顺序遍历"的方式处理，
 * 不在 Message 上做时间戳假设；用 opGroup 之间的"非 tool_call 事件"作为 flush 触发点。
 *
 * Task 7：在遍历 messages 时检测 role='system' + content=CONTEXT_COMPACTION_MARKER
 * 的合成消息，产出 { type: 'compaction' } 项。compactionText 可选
 * （i18n 解析后的 "上下文已自动压缩" 文本），未传时回退到 marker
 * 本身，保证测试 / 旧调用方也能正常工作。
 */
export function renderTurnItems(
  messages: Message[],
  status: AgentStatus,
  compactionText?: string,
): RenderedItem[] {
  const out: RenderedItem[] = []
  let opGroup: OpGroup | null = null
  let webGroup: WebSearchGroup | null = null
  // Task 7：compaction 分隔线的展示文本。优先用 i18n 解析结果
  // （传进来的 compactionText），缺失时回退到 marker 本身
  // （"上下文已自动压缩"），保证未配置 i18n 也能渲染。
  const effectiveCompactionText =
    compactionText && compactionText.trim().length > 0
      ? compactionText
      : CONTEXT_COMPACTION_MARKER

  function flushOpGroup(force: boolean) {
    if (!opGroup) return
    if (!force && (opGroup.lastStatus === 'running' || opGroup.lastStatus === 'pending')) {
      return
    }
    out.push({
      type: 'operationGroup',
      messageId: opGroup.anchorId,
      toolCallIds: opGroup.toolCallIds.slice(),
      forceComplete: force,
    })
    opGroup = null
  }

  function flushWebGroup() {
    if (!webGroup) return
    out.push({
      type: 'webSearchGroup',
      messageId: webGroup.anchorId,
      queries: webGroup.queries.slice(),
      toolCallIds: webGroup.toolCallIds.slice(),
    })
    webGroup = null
  }

  function tryAppendToGroup(tc: ToolCall) {
    if (tc.kind === 'webSearch') {
      // webSearch 不属于 operationGroup
      return false
    }
    if (!opGroup) {
      opGroup = { anchorId: tc.id, toolCallIds: [], kinds: [], lastStatus: null }
    }
    opGroup.toolCallIds.push(tc.id)
    opGroup.kinds.push(tc.kind)
    opGroup.lastStatus = tc.status
    return true
  }

  let messageIndex = 0
  for (const msg of messages) {
    const idx = messageIndex++
    // ── 主消息体 ──────────────────────────────────────────
    if (msg.role === 'user') {
      flushOpGroup(true)
      flushWebGroup()
      out.push({ type: 'user', messageId: `u-${idx}`, text: msg.content })
      // user 消息携带 error → 紧跟一个错误项（每条消息独立错误状态）
      if (msg.error) {
        out.push({ type: 'error', messageId: `uerr-${idx}`, text: msg.error, messageIndex: idx })
      }
    } else if (msg.role === 'system' && msg.content === CONTEXT_COMPACTION_MARKER) {
      // Task 7：上下文自动压缩分隔线
      //
      // 角色为 'system' 且 content 严格等于 CONTEXT_COMPACTION_MARKER
      // 的合成消息，由 useAgent 在收到后端 EventCompaction 事件时
      // 插入 messages 列表；这里是它唯一的合法渲染出口。
      //
      // 设计要点：
      //  - 紧跟前一条消息尾部插入，不打断 operationGroup / webSearchGroup
      //    累积（先 flush 完旧 group，再 push compaction 项，下一轮
      //    tool_call 重新开 group）
      //  - 不进入 assistant 的 content / reasoning 渲染路径，避免
      //    AssistantMessage.vue 把 "上下文已自动压缩" 当成对话内容
      //    渲染成 markdown 气泡
      //  - text 字段直接用 effectiveCompactionText（i18n 解析后），
      //    让 ContextCompactionDivider 不必再调 i18n
      flushOpGroup(true)
      flushWebGroup()
      out.push({
        type: 'compaction',
        messageId: `c-${idx}`,
        text: effectiveCompactionText,
      })
    } else if (msg.role === 'assistant') {
      flushOpGroup(true)
      flushWebGroup()
      const streaming = !!msg.isStreaming && status === 'streaming'
      // 错误优先
      if (msg.role === 'assistant' && msg.tool_results.length > 0) {
        const lastErr = [...msg.tool_results].reverse().find((r) => r.is_error)
        if (lastErr && !streaming) {
          out.push({ type: 'error', messageId: `a-${idx}`, text: lastErr.result, messageIndex: idx })
          continue
        }
      }
      if (msg.content && msg.content.trim().length > 0) {
        out.push({
          type: 'assistantText',
          messageId: `a-${idx}`,
          text: msg.content,
          streaming,
        })
      }
      if (msg.reasoning && msg.reasoning.trim().length > 0) {
        out.push({
          type: 'reasoning',
          messageId: `r-${idx}`,
          text: msg.reasoning,
          streaming: streaming && !msg.content,
        })
      }
    }

    // ── tool_calls 累积 ──────────────────────────────────
    for (const tc of msg.tool_calls || []) {
      // approval 单独成项
      if (tc.status === 'pending' && tc.needsConfirm) {
        flushOpGroup(true)
        flushWebGroup()
        out.push({ type: 'approval', toolCallId: tc.id, messageId: tc.id })
        continue
      }

      // plan tool (write_todos) 单独渲染为 plan block，**不能**
      // 与 operationGroup / webSearchGroup 合并——plan block
      // 是顶层 plan 视图，必须有独立的空间和生命周期。
      if (tc.kind === 'plan') {
        flushOpGroup(true)
        flushWebGroup()
        const streaming =
          !!msg.isStreaming &&
          status === 'streaming' &&
          (tc.status === 'pending' || tc.status === 'running')
        const todos = parsePlanArgs(tc.args)
        out.push({
          type: 'plan',
          messageId: tc.id,
          toolCallId: tc.id,
          todos,
          streaming,
        })
        continue
      }

      if (tc.kind === 'webSearch') {
        flushOpGroup(true)
        if (!webGroup) webGroup = { anchorId: tc.id, queries: [], toolCallIds: [] }
        webGroup.toolCallIds.push(tc.id)
        try {
          const args = JSON.parse(tc.args) as Record<string, unknown>
          if (typeof args.query === 'string') webGroup.queries.push(args.query)
        } catch {
          // ignore
        }
        continue
      } else if (tc.kind === 'readOnly') {
        // webSearch 走 webSearch 合并窗；readOnly 单独走 operationGroup
        flushWebGroup()
        tryAppendToGroup(tc)
        continue
      }

      flushWebGroup()
      tryAppendToGroup(tc)
    }

    // ── tool_results 错误回灌（仅当本条消息内已有内容）──
    if (msg.tool_results && msg.tool_results.length > 0 && !msg.content) {
      const lastErr = [...msg.tool_results].reverse().find((r) => r.is_error)
      if (lastErr) {
        flushOpGroup(true)
        flushWebGroup()
        out.push({ type: 'error', messageId: `err-${idx}`, text: lastErr.result, messageIndex: idx })
      }
    }
  }

  // 收尾：流式状态保留未完结 group（让 UI 持续显示 running 状态）
  // 非流式时强制 flush 全部
  if (status !== 'streaming') {
    flushOpGroup(true)
    flushWebGroup()
  } else {
    flushOpGroup(false)
    flushWebGroup()
  }

  return out
}

/**
 * parsePlanArgs 解析 plan tool (write_todos) 的 args JSON。
 *
 * 接受以下两种形状：
 *  1. 完整 schema: `{"todos":[{"id":"1","status":"in_progress","content":"..."}, ...]}`
 *  2. 兼容退化: `[{"id":"1",...}]` （裸数组，LLM 偶尔会省略外层对象）
 *
 * 解析失败时返回空数组——UI 会渲染 "planEmpty" 提示而不是崩溃。
 * 任何不是字符串的字段会被丢弃（防御性，避免 null.length 这类 bug）。
 */
export function parsePlanArgs(args: string): PlanTodo[] {
  if (!args || typeof args !== 'string') return []
  let parsed: unknown
  try {
    parsed = JSON.parse(args)
  } catch {
    return []
  }
  let rawList: unknown[]
  if (Array.isArray(parsed)) {
    rawList = parsed
  } else if (parsed && typeof parsed === 'object' && Array.isArray((parsed as { todos?: unknown }).todos)) {
    rawList = (parsed as { todos: unknown[] }).todos
  } else {
    return []
  }
  const out: PlanTodo[] = []
  for (const item of rawList) {
    if (!item || typeof item !== 'object') continue
    const t = item as { id?: unknown; status?: unknown; content?: unknown }
    if (typeof t.id !== 'string') continue
    if (typeof t.content !== 'string') continue
    const status = typeof t.status === 'string' ? t.status : 'pending'
    out.push({ id: t.id, status, content: t.content })
  }
  return out
}
