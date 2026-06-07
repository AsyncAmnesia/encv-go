/**
 * tdesignEngine.ts - TDesign Chat 渲染引擎
 *
 * 使用 TDesign Chat 纯 Vue 组件渲染：
 *   ChatContent — 消息内容（纯 Vue defineComponent，不依赖 Web Component）
 *
 * 不使用 ChatMessage/Chatbot/ChatList（它们底层是 omiVueify 包装的 Web Component，
 * 需要 tdesign-web-components 注册 t-chat-item 等自定义元素，在 pnpm 严格模式下
 * 可能因幽灵依赖问题导致 Web Component 未注册而无法渲染）。
 *
 * ChatContent 是纯 Vue 组件，直接渲染 HTML + Markdown，零 Web Component 依赖。
 *
 * 数据流：
 *   useAgent.messages → EngineRenderProps.messages
 *   → transformMessages() → 逐条 h(ChatContent, { content, role })
 *
 * 文档: https://tdesign.tencent.com/chat/agui
 * SPEC: /workspace/.trae/specs/multi-engine-chat-architecture/ Phase 4
 */

import { h, type VNode, defineComponent, ref, type Ref } from 'vue'
import type { ChatEngine, EngineRenderProps } from '@/composables/chatEngine'
import { registerEngine } from '@/composables/chatEngine'

// ── TDesign Chat 纯 Vue 组件引用（异步加载） ──
let ChatContent: any = null
const TDesignChatLoaded: Ref<boolean> = ref(false)

// 异步加载 TDesign ChatContent（纯 Vue 组件，不依赖 Web Component）
async function loadTDesignChat(): Promise<void> {
  if (TDesignChatLoaded.value) return
  try {
    const mod = await import('@tdesign-vue-next/chat')
    console.info('[tdesignEngine] Module loaded. Keys:', Object.keys(mod).filter(k => k.startsWith('Chat')).join(', '))
    // ChatContent 是纯 Vue defineComponent，不依赖 omiVueify/Web Component
    ChatContent = mod.ChatContent
    if (ChatContent) {
      TDesignChatLoaded.value = true
      console.info('[tdesignEngine] TDesign ChatContent loaded successfully, type:', typeof ChatContent)
    } else {
      console.error('[tdesignEngine] ChatContent not found in module exports!')
    }
  } catch (err) {
    console.error('[tdesignEngine] @tdesign-vue-next/chat import failed:', err)
  }
}

// 立即触发异步加载
loadTDesignChat()

/**
 * TDesign Chat 渲染组件
 *
 * 用 ChatContent 逐条渲染消息内容（纯 Vue，不依赖 Web Component）。
 * 外层用 div 布局模拟消息气泡效果。
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
      if (!TDesignChatLoaded.value || !ChatContent) {
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

      // 真实 TDesign 渲染：ChatContent 逐条渲染
      const tdesignMsgs = transformMessages(props.messages, props.streaming)
      console.debug('[tdesignEngine] Rendering', tdesignMsgs.length, 'messages, ChatContent type:', typeof ChatContent)

      const children: VNode[] = []

      for (const msg of tdesignMsgs) {
        // 消息容器
        children.push(h('div', {
          class: [
            'tdesign-msg',
            `tdesign-msg--${msg.role}`,
          ],
          style: {
            display: 'flex',
            flexDirection: 'column',
            alignItems: msg.role === 'user' ? 'flex-end' : 'flex-start',
            marginBottom: '16px',
          },
        }, [
          // 昵称
          msg.name ? h('div', {
            class: 'tdesign-msg__name',
            style: {
              fontSize: '12px',
              color: 'var(--ion-color-medium, #999)',
              marginBottom: '4px',
              padding: '0 12px',
            },
          }, msg.name) : null,
          // 气泡
          h('div', {
            class: [
              'tdesign-msg__bubble',
              `tdesign-msg__bubble--${msg.role}`,
            ],
            style: {
              maxWidth: '80%',
              padding: msg.role === 'user' ? '10px 16px' : '4px',
              borderRadius: msg.role === 'user' ? '18px 18px 4px 18px' : '18px 18px 18px 4px',
              background: msg.role === 'user'
                ? 'var(--ion-color-primary, #4f8cff)'
                : 'var(--ion-background-color-step-50, #f5f5f5)',
              color: msg.role === 'user'
                ? 'var(--ion-color-primary-contrast, #fff)'
                : 'var(--ion-text-color, #333)',
              ...(msg.role === 'user' ? { wordBreak: 'break-word' as const } : {}),
            },
          }, [
            // ChatContent 渲染消息内容（支持 Markdown）
            h(ChatContent, {
              content: msg.content,
              role: msg.role,
              status: msg.status || '',
              markdownProps: {
                engine: 'marked',
                options: {},
              },
            }),
          ]),
        ]))
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
// 数据转换
// ─────────────────────────────────────────────────────────────────────────────

interface TDesignMessage {
  role: 'user' | 'assistant' | 'error'
  content: string
  name?: string
  status?: string
}

function transformMessages(messages: readonly any[], _streaming: boolean): TDesignMessage[] {
  if (!Array.isArray(messages) || messages.length === 0) return []

  const items: TDesignMessage[] = []

  for (const msg of messages) {
    if (msg.role === 'system') continue

    const textContent = extractTextContent(msg.content)
    if (!textContent && msg.role === 'assistant' && !msg.isStreaming && !msg.tool_calls?.length) continue

    const role: TDesignMessage['role'] =
      msg.role === 'user' ? 'user' :
      msg.error ? 'error' : 'assistant'

    let content = textContent || ''

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
    })
  }

  return items
}

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
