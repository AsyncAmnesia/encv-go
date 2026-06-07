/**
 * defaultEngine.ts - 默认引擎适配器
 *
 * 将现有 AgentChat.vue 的 Ionic 组件消息渲染逻辑包装为 ChatEngine 接口实现。
 * 渲染目标为 DefaultMessagesView.vue（从 AgentChat.vue <main> 区域提取的独立组件）。
 *
 * SPEC: /workspace/.trae/specs/multi-engine-chat-architecture/
 */

import { h, type VNode } from 'vue'
import type { ChatEngine, EngineRenderProps } from '@/composables/chatEngine'
import { registerEngine } from '@/composables/chatEngine'
import DefaultMessagesView from '@/components/agent/DefaultMessagesView.vue'

/**
 * 创建默认引擎实例
 *
 * 返回一个实现 ChatEngine 接口的对象，其 renderMessages() 方法返回
 * DefaultMessagesView 组件的 VNode。该组件包含完整的消息列表渲染逻辑：
 * - 空状态提示
 * - 短会话（≤120 条）原生 v-for 渲染
 * - 长会话（>120 条）MessageVirtualList 虚拟滚动
 * - 全部消息类型分发（user / assistantText / messageFooter / approval /
 *   operationGroup / operation / webSearchGroup / plan / reasoning / error /
 *   compaction / agentTask）
 */
export function createDefaultEngine(): ChatEngine {
  return {
    id: 'default',
    name: 'Ionic 默认',
    description: '当前默认的 Ionic 组件实现',
    supportsA2UI: false,

    renderMessages(props: EngineRenderProps): VNode {
      return h(DefaultMessagesView, { ...props })
    },

    destroy(): void {
      // 无需清理：无副作用（无定时器、事件监听器或订阅）
    },
  }
}

// ── 自动注册到 EngineRegistry ──────────────────────────────
// 模块被 import 时自动注册，确保 useChatEngine() 能通过 'default' id 找到此引擎。
registerEngine('default', createDefaultEngine)
