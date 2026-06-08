/**
 * tdesignEngine.ts - 腾讯 TDesign 视觉风格渲染引擎
 *
 * 数据源：通过 useAgent 共享的 AG-UI 解析后的 Message[]（不自行消费 SSE）
 * 渲染层：本引擎用 TDesign 视觉组件渲染 Message[]
 *
 * 协议：AG-UI（与 Default 引擎共享同一份数据）
 *
 * 重构说明：早期版本直接使用 @tdesign-vue-next/chat 的 <Chatbot> 组件，
 * 但 <Chatbot> 内置独立的 ChatService 实例，会自己再消费一份 SSE 流，
 * 与 useAgent 共享数据源的架构冲突。重构后改为纯渲染层——只展示
 * useAgent 提供的 messages: readonly Message[]，所有 UI 元素（消息
 * 列表 / thinking 指示器 / tool_call 卡片）由 TDesignChatView 用
 * 通用 TDesign 组件（ChatList / ChatItem / ChatThinking）组合而成。
 *
 * SPEC: /workspace/.trae/specs/agui-real-llm-path-completion/ Phase 4
 */

import { h } from 'vue'
import type { VNode } from 'vue'
import type { ChatEngine, EngineRenderProps } from '@/composables/chatEngine'
import { registerEngine } from '@/composables/chatEngine'
import TDesignChatView from './TDesignChatView.vue'

export function createTDesignEngine(): ChatEngine {
  return {
    id: 'tdesign',
    name: 'TDesign 风格',
    description: '腾讯 TDesign 视觉风格的聊天渲染',
    supportsA2UI: false,

    /**
     * 渲染消息列表区域 —— 纯 VNode 透传
     * 把 EngineRenderProps 全部 props 传给 TDesignChatView
     * （消息 / 状态 / 回调等所有上下文都从宿主来）
     */
    renderMessages(props: EngineRenderProps): VNode {
      return h(TDesignChatView, { ...props })
    },

    destroy(): void {
      // 无需清理：TDesignChatView 是无状态纯组件
    },
  }
}

registerEngine('tdesign', createTDesignEngine)
