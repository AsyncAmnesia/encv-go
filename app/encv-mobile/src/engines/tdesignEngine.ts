/**
 * tdesignEngine.ts - TDesign Chat 渲染引擎
 *
 * 集成腾讯 @tdesign-vue-next/chat 组件库。
 * 使用 ChatList 组件（仅消息列表，无输入框/SSE 连接），
 * 由宿主 AgentChat.vue 的 footer 统一控制输入 + useAgent 管理 SSE。
 *
 * 数据流：
 *   useAgent.messages → EngineRenderProps.messages → transformMessagesToTDesign()
 *   → TDesign ChatList data prop → TDesign 渲染消息气泡
 *
 * 文档: https://tdesign.tencent.com/chat/agui
 * SPEC: /workspace/.trae/specs/multi-engine-chat-architecture/ Phase 4
 */

import { h, type VNode, defineComponent } from 'vue'
import type { ChatEngine, EngineRenderProps } from '@/composables/chatEngine'
import { registerEngine } from '@/composables/chatEngine'

// TDesign ChatList 组件引用（异步加载）
let ChatList: any = null
let TDesignChatLoaded = false

// 异步加载 TDesign Chat（Vite 环境下使用动态 import）
async function loadTDesignChat(): Promise<void> {
  if (TDesignChatLoaded) return
  try {
    const mod = await import('@tdesign-vue-next/chat')
    // 使用 ChatList（仅消息列表，无输入框/SSE），不用 Chatbot（自带输入框+SSE）
    ChatList = mod.ChatList
    if (ChatList) {
      TDesignChatLoaded = true
      console.info('[tdesignEngine] @tdesign-vue-next/chat ChatList loaded successfully')
    } else {
      console.warn('[tdesignEngine] ChatListComponent not found in module, available keys:', Object.keys(mod))
    }
  } catch (err) {
    console.info('[tdesignEngine] @tdesign-vue-next/chat not available, using stub mode:', err)
  }
}

// 立即触发异步加载（不阻塞模块注册）
loadTDesignChat()

/**
 * TDesign Chat 渲染组件
 *
 * 使用 ChatList 渲染消息列表。数据由外部传入（EngineRenderProps.messages），
 * 不走 TDesign 自带的 SSE 连接。
 */
const TDesignChatView = defineComponent({
  name: 'TDesignChatView',
  props: {
    messages: { type: Array, default: () => [] },
    streaming: { type: Boolean, default: false },
  },
  setup(props) {
    return () => {
      // Stub 模式：TDesign 包未加载
      if (!TDesignChatLoaded || !ChatList) {
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

      // 真实 TDesign 渲染：ChatList + 外部数据
      const tdesignData = transformMessagesToTDesign(props.messages)
      return h('div', { class: 'tdesign-chat-container', style: { height: '100%', overflow: 'auto' } }, [
        h(ChatList, {
          data: tdesignData,
          layout: 'both',
          autoScroll: true,
          animation: 'gradient',
          isStreamLoad: props.streaming,
          textLoading: props.streaming,
        })
      ])
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
// 数据转换：内部 Message[] → TDesign ChatList data (TdChatItemMeta[])
// ─────────────────────────────────────────────────────────────────────────────

interface TDesignChatItem {
  role: 'user' | 'assistant' | 'error' | 'system'
  content: string
  avatar?: string
  name?: string
  datetime?: string
  status?: string
}

/**
 * 将内部 Message[] 转换为 TDesign ChatList 的 data 格式
 *
 * TDesign ChatList 期望 TdChatItemMeta[]:
 *   { role, content, avatar?, name?, datetime?, status? }
 *
 * 内部 Message 结构:
 *   { role: 'user'|'assistant'|'system'|'agent_task',
 *     content: string | MessageContentPart[],
 *     tool_calls: ToolCall[],
 *     tool_results: ToolResult[],
 *     isStreaming?: boolean,
 *     error?: string }
 */
function transformMessagesToTDesign(messages: readonly any[]): TDesignChatItem[] {
  if (!Array.isArray(messages) || messages.length === 0) return []

  const items: TDesignChatItem[] = []

  for (const msg of messages) {
    // 跳过 system 消息（TDesign 不渲染 system 角色）
    if (msg.role === 'system') continue

    // 跳过空内容的 assistant 消息（streaming 初始状态）
    const textContent = extractTextContent(msg.content)
    if (!textContent && msg.role === 'assistant' && !msg.isStreaming) continue

    // 映射 role
    const role: TDesignChatItem['role'] =
      msg.role === 'user' ? 'user' :
      msg.role === 'assistant' ? 'assistant' :
      msg.error ? 'error' : 'assistant'

    // 构建内容文本（包含工具调用信息）
    let content = textContent || ''

    // 如果有工具调用，追加工具调用摘要
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

    // 如果有工具结果，追加结果摘要
    if (msg.tool_results && msg.tool_results.length > 0) {
      const resultSummary = msg.tool_results.map((tr: any) => {
        const result = typeof tr.content === 'string'
          ? tr.content.slice(0, 200)
          : JSON.stringify(tr.content).slice(0, 200)
        return `📋 ${result}${tr.content?.length > 200 ? '...' : ''}`
      }).join('\n')
      if (content) content += '\n\n'
      content += resultSummary
    }

    items.push({
      role,
      content,
      name: role === 'assistant' ? 'AI 助手' : undefined,
      status: msg.error ? 'error' : undefined,
    })
  }

  return items
}

/**
 * 从 Message.content 中提取纯文本
 * content 可能是 string 或 MessageContentPart[]
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
