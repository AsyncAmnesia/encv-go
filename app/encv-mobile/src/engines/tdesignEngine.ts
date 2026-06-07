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

import { h, type VNode } from 'vue'
import type { ChatEngine, EngineRenderProps } from '@/composables/chatEngine'
import { registerEngine } from '@/composables/chatEngine'
import TDesignChatView from './TDesignChatView.vue'

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
      // TDesign Chatbot 自行管理 SSE 连接和消息渲染，
      // 不需要外部传入 messages。
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
