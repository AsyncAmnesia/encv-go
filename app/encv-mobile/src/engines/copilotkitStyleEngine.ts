/**
 * copilotkitStyleEngine.ts - CopilotKit 视觉风格渲染引擎
 *
 * 数据源：通过 useAgent 共享的 AG-UI 解析后的 Message[]（不自行消费 SSE）
 * 渲染层：本引擎用仿 CopilotKit 视觉组件渲染 Message[]
 *
 * 协议：AG-UI（与 Default / TDesign 风格引擎共享同一份数据）
 */

import { h } from 'vue'
import type { VNode } from 'vue'
import type { ChatEngine, EngineRenderProps } from '@/composables/chatEngine'
import { registerEngine } from '@/composables/chatEngine'
import CopilotKitStyleChat from '@/components/agent/copilotkit/CopilotKitStyleChat.vue'

export function createCopilotKitStyleEngine(): ChatEngine {
  return {
    id: 'copilotkit-style',
    name: 'CopilotKit 风格',
    description: '模仿 CopilotKit 交互范式的 Vue 实现',
    supportsA2UI: false,

    renderMessages(props: EngineRenderProps): VNode {
      return h(CopilotKitStyleChat, { ...props })
    },

    destroy(): void {
      // 无需清理：组件卸载时自动清理
    },
  }
}

registerEngine('copilotkit-style', createCopilotKitStyleEngine)
