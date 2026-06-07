/**
 * copilotkitStyleEngine.ts - CopilotKit 风格聊天渲染引擎
 *
 * 模仿 CopilotKit v1.50 的交互范式，用 Vue/Ionic 实现等效体验：
 * - 左侧固定 48px 头像区 + 右侧更宽内容区
 * - 工具调用卡片带渐变边框 + 左侧彩色竖条
 * - 底部水平滚动 Suggestions chip bar
 * - 消息出现 slide-up 过渡动画
 *
 * SPEC: /workspace/.trae/specs/multi-engine-chat-architecture/ Phase 3 (Task 3.1-3.4)
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
