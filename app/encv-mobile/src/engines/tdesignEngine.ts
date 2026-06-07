/**
 * tdesignEngine.ts - TDesign Chat 渲染引擎
 *
 * 使用 TDesign Chat 底层组件组合渲染：
 *   ChatMessage  — 消息气泡（头像 + 昵称 + 内容 + 时间 + 操作栏）
 *   ChatContent  — 消息内容（文本 / Markdown）
 *   ChatLoading  — 流式加载动画
 *   ChatThinking — 思考过程展示
 *
 * 不使用 Chatbot（全家桶，自带 SSE + 输入框）或 ChatList（仍属高层封装），
 * 而是用 ChatMessage 逐条渲染，完全由外部控制数据流。
 *
 * 数据流：
 *   useAgent.messages → EngineRenderProps.messages
 *   → transformMessages() → 逐条 h(ChatMessage, { message: ... })
 *
 * 文档: https://tdesign.tencent.com/chat/agui
 * SPEC: /workspace/.trae/specs/multi-engine-chat-architecture/ Phase 4
 */

import { h, type VNode, defineComponent } from 'vue'
import type { ChatEngine, EngineRenderProps } from '@/composables/chatEngine'
import { registerEngine } from '@/composables/chatEngine'

// ── TDesign Chat 底层组件引用（异步加载） ──
let ChatMessage: any = null
let ChatContent: any = null
let ChatLoading: any = null
let TDesignChatLoaded = false

// 异步加载 TDesign Chat 底层组件
async function loadTDesignChat(): Promise<void> {
  if (TDesignChatLoaded) return
  try {
    const mod = await import('@tdesign-vue-next/chat')
    ChatMessage = mod.ChatMessage
    ChatContent = mod.ChatContent
    ChatLoading = mod.ChatLoading
    if (ChatMessage && ChatContent) {
      TDesignChatLoaded = true
      console.info('[tdesignEngine] TDesign Chat components loaded successfully')
    } else {
      console.warn('[tdesignEngine] Missing components. ChatMessage:', !!ChatMessage, 'ChatContent:', !!ChatContent)
    }
  } catch (err) {
    console.info('[tdesignEngine] @tdesign-vue-next/chat not available, using stub mode:', err)
  }
}

// 立即触发异步加载（不阻塞模块注册）
loadTDesignChat()

// ── 消息类型映射 ──

interface TDesignMessage {
  role: 'user' | 'assistant' | 'error' | 'system' | 'model-change'
  content: string
  avatar?: string
  name?: string
  datetime?: string
  status?: '' | 'error'
  variant?: 'base' | 'outline' | 'text'
  textLoading?: boolean
  animation?: 'skeleton' | 'moving' | 'gradient'
  reasoning?: any
}

/**
 * TDesign Chat 渲染组件
 *
 * 用 ChatMessage 逐条渲染消息，完全由外部控制数据流。
 */
const TDesignChatView = defineComponent({
  name: 'TDesignChatView',
  props: {
    messages: { type: Array, default: () => [] },
    streaming: { type: Boolean, default: false },
  },
  setup(props) {
    return () => {
      // Stub 模式
      if (!TDesignChatLoaded || !ChatMessage) {
        return h('div', {
          class: 'tdesign-stub',
          style: {
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            padding: '48px 24px',
            minHeight: '200px',
          },
        }, [
          h('div', { style: { fontSize: '48px', marginBottom: '16px' } }, '\u{1F4AC}'),
          h('h3', {
            style: {
              margin: '0 0 8px',
              color: 'var(--ion-text-color, #333)',
              fontSize: '18px',
              fontWeight: 600,
            },
          }, 'TDesign Chat 即将上线'),
          h('p', {
            style: {
              margin: '0 0 12px',
              color: 'var(--ion-color-medium, #666)',
              fontSize: '14px',
            },
          }, '正在集成 @tdesign-vue-next/chat 组件库...'),
          h('p', {
            style: {
              margin: '0',
              color: 'var(--ion-color-medium, #666)',
              fontSize: '13px',
            },
          }, '当前请使用「Ionic 默认」或「CopilotKit 风格」引擎'),
        ])
      }

      // 真实 TDesign 渲染：ChatMessage 逐条渲染
      const tdesignMsgs = transformMessages(props.messages, props.streaming)

      const children: VNode[] = []

      for (const msg of tdesignMsgs) {
        // ChatMessage 的 message prop 接收 TdChatItemProps 格式
        children.push(h(ChatMessage, {
          message: {
            role: msg.role,
            content: msg.content,
            avatar: msg.avatar,
            name: msg.name,
            datetime: msg.datetime,
            status: msg.status,
            variant: msg.variant || 'base',
            textLoading: msg.textLoading,
            animation: msg.animation,
            reasoning: msg.reasoning,
          },
          placement: msg.role === 'user' ? 'right' : 'left',
          avatar: msg.avatar,
          name: msg.name,
          datetime: msg.datetime,
          variant: msg.variant || 'base',
          animation: msg.animation,
        }))
      }

      // 流式加载指示器
      if (props.streaming && ChatLoading) {
        children.push(h(ChatLoading, {
          animation: 'gradient',
        }))
      }

      return h('div', {
        class: 'tdesign-chat-container',
        style: {
          height: '100%',
          overflow: 'auto',
          padding: '16px',
        },
      }, children)
    }
  },
})

/**
 * 创建 TDesign 引擎实例
 */
export function createTDesignEngine(): ChatEngine {
  return {
    id: 'tdesign',
    name: 'TDesign Chat',
    description: '腾讯 TDesign Chat 组件库（AG-UI 协议）',
    supportsA2UI: false,

    renderMessages(props: EngineRenderProps): VNode {
      return h(TDesignChatView, {
        messages: [...props.messages],
        streaming: props.streaming,
      })
    },

    destroy(): void {
      // 无需手动清理
    },
  }
}

// ── 自动注册到 EngineRegistry ──
registerEngine('tdesign', createTDesignEngine)

// ─────────────────────────────────────────────────────────────────────────────
// 数据转换：内部 Message[] → TDesign ChatMessage message prop 格式
// ─────────────────────────────────────────────────────────────────────────────

/**
 * 将内部 Message[] 转换为 TDesign ChatMessage 的 message prop 格式
 *
 * ChatMessage 接受 TdChatItemProps:
 *   { role, content, avatar?, name?, datetime?, variant?, status?, textLoading?, animation?, reasoning? }
 *
 * 内部 Message 结构:
 *   { role: 'user'|'assistant'|'system'|'agent_task',
 *     content: string | MessageContentPart[],
 *     tool_calls: ToolCall[],
 *     tool_results: ToolResult[],
 *     isStreaming?: boolean,
 *     error?: string }
 */
function transformMessages(messages: readonly any[], streaming: boolean): TDesignMessage[] {
  if (!Array.isArray(messages) || messages.length === 0) return []

  const items: TDesignMessage[] = []

  for (const msg of messages) {
    // 跳过 system 消息
    if (msg.role === 'system') continue

    const textContent = extractTextContent(msg.content)

    // 跳过空内容的非流式 assistant 消息
    if (!textContent && msg.role === 'assistant' && !msg.isStreaming && !msg.tool_calls?.length) continue

    // 映射 role
    const role: TDesignMessage['role'] =
      msg.role === 'user' ? 'user' :
      msg.error ? 'error' : 'assistant'

    // 构建内容（包含工具调用信息）
    let content = textContent || ''

    // 工具调用摘要
    if (msg.tool_calls && msg.tool_calls.length > 0) {
      const toolSummary = msg.tool_calls.map((tc: any) => {
        const name = tc.function?.name || tc.name || 'tool'
        const status = tc.status || 'running'
        const icon = status === 'success' ? '✓' : status === 'failed' ? '✗' : '⟳'
        return `${icon} **${name}**`
      }).join('\n')
      if (content) content += '\n\n'
      content += toolSummary
    }

    // 工具结果摘要
    if (msg.tool_results && msg.tool_results.length > 0) {
      const resultSummary = msg.tool_results.map((tr: any) => {
        const result = typeof tr.content === 'string'
          ? tr.content.slice(0, 200)
          : JSON.stringify(tr.content).slice(0, 200)
        return `📋 ${result}${(tr.content?.length || 0) > 200 ? '...' : ''}`
      }).join('\n')
      if (content) content += '\n\n'
      content += resultSummary
    }

    items.push({
      role,
      content,
      name: role === 'assistant' ? 'AI 助手' : undefined,
      status: msg.error ? 'error' : '',
      variant: role === 'user' ? 'base' : 'text',
      textLoading: msg.isStreaming && streaming,
      animation: msg.isStreaming ? 'gradient' : undefined,
    })
  }

  return items
}

/**
 * 从 Message.content 中提取纯文本
 */
function extractTextContent(content: any): string {
  if (!content) return ''
  if (typeof content === 'string') return content
  if (Array.isArray(content)) {
    return content
      .filter((p: any) => p.type === 'text' && typeof p.text === 'string')
      .map((p: any) => p.text)
      .join('')
  }
  return String(content)
}
