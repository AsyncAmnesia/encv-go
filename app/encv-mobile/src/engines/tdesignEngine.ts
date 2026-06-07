/**
 * tdesignEngine.ts - TDesign Chat 渲染引擎
 *
 * 按官方文档集成：https://tdesign.tencent.com/chat/getting-started
 * 使用 Chatbot 组件 + chatServiceConfig({ protocol: 'agui', endpoint })，
 * 让 TDesign 自行通过 SSE 连接后端 AG-UI 端点，解析事件流并渲染消息。
 *
 * 前置条件（在 main.ts 中完成）：
 *   1. import TDesignChat from '@tdesign-vue-next/chat'
 *   2. import '@tdesign-vue-next/chat/es/style/index.css'
 *   3. app.use(TDesignChat)
 *
 * 输入框通过 CSS 隐藏（Chatbot 无 showInput prop），实际输入由宿主
 * AgentChat.vue 的 footer 统一控制。
 *
 * 文档: https://tdesign.tencent.com/chat/agui
 * SPEC: /workspace/.trae/specs/multi-engine-chat-architecture/ Phase 4
 */

import { h, type VNode, defineComponent, onMounted, onUnmounted, shallowRef } from 'vue'
import type { ChatEngine, EngineRenderProps } from '@/composables/chatEngine'
import { registerEngine } from '@/composables/chatEngine'
import { getAgentApiBase } from '@/composables/useAgentApiBase'

// 异步加载 Chatbot Vue 组件（已通过 main.ts app.use(TDesignChat) 全局注册）
const ChatbotRef = shallowRef<any>(null)

async function loadChatbot(): Promise<void> {
  if (ChatbotRef.value) return
  try {
    const mod = await import('@tdesign-vue-next/chat')
    ChatbotRef.value = mod.Chatbot
    if (ChatbotRef.value) {
      console.info('[tdesignEngine] Chatbot component loaded')
    } else {
      console.error('[tdesignEngine] Chatbot not found in exports. Available Chat*:', Object.keys(mod).filter(k => k.startsWith('Chat')))
    }
  } catch (err) {
    console.error('[tdesignEngine] Failed to load @tdesign-vue-next/chat:', err)
  }
}

loadChatbot()

/**
 * TDesign Chat 渲染组件
 */
const TDesignChatView = defineComponent({
  name: 'TDesignChatView',
  setup() {
    onMounted(() => {
      // 注入 CSS 隐藏 Chatbot 自带的输入框（ChatSender）
      const style = document.createElement('style')
      style.id = 'tdesign-chat-hide-sender'
      style.textContent = `
        .tdesign-chat-container t-chat-sender,
        .tdesign-chat-container .t-chat-sender,
        .tdesign-chat-container [class*="chat-sender"],
        .tdesign-chat-container [class*="chatSender"] {
          display: none !important;
        }
      `
      if (!document.getElementById('tdesign-chat-hide-sender')) {
        document.head.appendChild(style)
      }
    })

    onUnmounted(() => {
      const style = document.getElementById('tdesign-chat-hide-sender')
      if (style) style.remove()
    })

    return () => {
      if (!ChatbotRef.value) {
        // Chatbot 组件尚未加载完成
        return h('div', {
          style: {
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100%',
            color: 'var(--ion-color-medium)',
            fontSize: '14px',
          },
        }, 'TDesign Chat 加载中...')
      }

      // chatServiceConfig: AG-UI 协议配置
      const apiBase = getAgentApiBase()
      const chatServiceConfig = {
        endpoint: `${apiBase}/api/chat?protocol=agui`,
        protocol: 'agui' as const,
        stream: true,
      }

      return h('div', {
        class: 'tdesign-chat-container',
        style: { height: '100%' },
      }, [
        h(ChatbotRef.value, {
          chatServiceConfig,
          layout: 'both',
          autoScroll: true,
          animation: 'gradient',
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

    renderMessages(_props: EngineRenderProps): VNode {
      return h(TDesignChatView)
    },

    destroy(): void {
      const style = document.getElementById('tdesign-chat-hide-sender')
      if (style) style.remove()
    },
  }
}

// ── 自动注册到 EngineRegistry ──
registerEngine('tdesign', createTDesignEngine)
