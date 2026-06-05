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
import type { Message, AgentStatus, ToolCall } from './useAgent'

/** 单条渲染项 - 由 AgentChat 分发到对应组件 */
export type RenderedItem =
  | { type: 'user'; messageId: string; text: string }
  | { type: 'assistantText'; messageId: string; text: string; streaming: boolean }
  | { type: 'approval'; toolCallId: string; messageId: string }
  | { type: 'operationGroup'; messageId: string; toolCallIds: string[]; forceComplete: boolean }
  | { type: 'webSearchGroup'; messageId: string; queries: string[]; toolCallIds: string[] }
  | { type: 'reasoning'; messageId: string; text: string; streaming: boolean }
  | { type: 'error'; messageId: string; text: string; messageIndex: number }

/**
 * useRenderTurnItems - 组合式接口
 * 入参：messages ref + status ref
 * 返回：RenderedItem[] computed
 */
export function useRenderTurnItems(
  messages: Ref<Message[]> | ComputedRef<Message[]>,
  status: Ref<AgentStatus> | ComputedRef<AgentStatus>,
): ComputedRef<RenderedItem[]> {
  return computed(() => renderTurnItems(messages.value, status.value))
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

const FLUSH_GAP_MS = 800

/**
 * renderTurnItems 纯函数
 * 1. 单条 user / assistant / error / reasoning → 直接产出 item
 * 2. 累积连续 toolCall（command/fileChange/readOnly）→ operationGroup
 * 3. 累积连续 web_search 类的 toolCall → webSearchGroup
 * 4. 流式结束或超过 FLUSH_GAP_MS 时强制 flush
 *
 * 这里按"消息 + 该消息内 toolCalls 顺序遍历"的方式处理，
 * 不在 Message 上做时间戳假设；用 opGroup 之间的"非 tool_call 事件"作为 flush 触发点。
 */
export function renderTurnItems(
  messages: Message[],
  status: AgentStatus,
): RenderedItem[] {
  const out: RenderedItem[] = []
  let opGroup: OpGroup | null = null
  let webGroup: WebSearchGroup | null = null

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
