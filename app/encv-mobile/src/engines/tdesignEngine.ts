/**
 * tdesignEngine.ts - TDesign Chat 渲染引擎
 *
 * 集成腾讯 @tdesign-vue-next/chat 组件库，通过 AG-UI 协议与后端通信。
 * 使用 ChatList 组件（仅消息列表，无输入框），输入框由宿主 AgentChat.vue 的 footer 统一控制。
 *
 * SPEC: /workspace/.trae/specs/multi-engine-chat-architecture/ Phase 4
 */

import { h, type VNode, defineComponent } from 'vue'
import type { ChatEngine, EngineRenderProps } from '@/composables/chatEngine'
import { registerEngine } from '@/composables/chatEngine'

// TDesign Chat 组件引用（异步加载）
let ChatList: any = null
let TDesignChatLoaded = false

// 异步加载 TDesign Chat（Vite 环境下使用动态 import）
async function loadTDesignChat(): Promise<void> {
  if (TDesignChatLoaded) return
  try {
    const mod = await import('@tdesign-vue-next/chat')
    // 使用 ChatList（仅消息列表，无输入框），不用 Chatbot（自带输入框）
    ChatList = mod.ChatList || mod.default
    if (ChatList) {
      TDesignChatLoaded = true
      console.info('[tdesignEngine] @tdesign-vue-next/chat ChatList loaded successfully')
    }
  } catch {
    console.info('[tdesignEngine] @tdesign-vue-next/chat not available, using stub mode')
  }
}

// 立即触发异步加载（不阻塞模块注册）
loadTDesignChat()

/**
 * TDesign Chat 占位/真实渲染组件
 */
const TDesignChatView = defineComponent({
  name: 'TDesignChatView',
  props: {
    messages: { type: Array, default: () => [] },
    streaming: { type: Boolean, default: false },
    onSend: { type: Function, required: true },
    onStop: { type: Function, required: true },
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

      // 真实 TDesign 渲染模式
      // 使用 ChatList（仅消息列表），输入框由宿主 AgentChat.vue 的 footer 控制
      return h(ChatList, {
        data: transformMessagesToTDesign(props.messages),
        layout: 'both',
        autoScroll: true,
        animation: 'gradient',
        isStreamLoad: props.streaming,
        textLoading: props.streaming,
      })
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
        onSend: props.onSend,
        onStop: props.onStop,
      })
    },

    destroy(): void {
      // 无需手动清理
    },
  }
}

// ── 自动注册到 EngineRegistry ──
registerEngine('tdesign', createTDesignEngine)

/**
 * 将内部 Message[] 转换为 TDesign ChatList 的 data 格式
 */
function transformMessagesToTDesign(messages: readonly any[]): any[] {
  if (!Array.isArray(messages) || messages.length === 0) return []

  return messages.map((msg) => {
    const base: Record<string, unknown> = {
      role: msg.role === 'user' ? 'user' : 'assistant',
      content: typeof msg.content === 'string' ? msg.content : '',
    }
    if (msg.createdAt || msg.timestamp) {
      base.datetime = msg.createdAt ?? msg.timestamp
    }
    return base
  })
}
