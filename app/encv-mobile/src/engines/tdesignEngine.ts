/**
 * tdesignEngine.ts - TDesign Chat 渲染引擎
 *
 * 集成腾讯 @tdesign-vue-next/chat 组件库，通过 AG-UI 协议与后端通信。
 *
 * 架构特点：
 *   - 使用 TDesign ChatBot 组件作为消息渲染 UI
 *   - 通过 AG-UI 协议（X-Agent-Protocol: agui）与后端 SSE 对接
 *   - 支持工具调用渲染（TOOL_CALL_START/END/RESULT）
 *   - 流式文本增量更新（TEXT_MESSAGE_CONTENT）
 *
 * SPEC: /workspace/.trae/specs/multi-engine-chat-architecture/ Phase 4
 */

import { h, type VNode } from 'vue'
import type { ChatEngine, EngineRenderProps } from '@/composables/chatEngine'
import { registerEngine } from '@/composables/chatEngine'
import { getAgentApiBase } from '@/composables/useAgentApiBase'

// 尝试导入 TDesign Chat 组件
// 使用 require + 类型声明以兼容 vite-tsc（Vite 环境下 require 可用但 ts 不识别）
declare function require(module: string): any

let Chatbot: any = null
let TDesignChatLoaded = false

try {
  const mod = require('@tdesign-vue-next/chat')
  // 优先使用 Chatbot（别名），fallback 到 Chat 或 default
  Chatbot = mod.Chatbot || mod.Chat || mod.default
  if (Chatbot) {
    TDesignChatLoaded = true
  }
} catch {
  // @tdesign-vue-next/chat 未安装或加载失败 → 使用 stub 模式
  console.warn('[tdesignEngine] @tdesign-vue-next/chat not available, using stub mode')
}

/**
 * 创建 TDesign 引擎实例
 *
 * 当 TDesign Chat 包可用时，返回真实的 ChatBot 渲染；
 * 否则返回一个"即将上线"的占位 UI。
 */
export function createTDesignEngine(): ChatEngine {
  return {
    id: 'tdesign',
    name: 'TDesign Chat',
    description: '腾讯 TDesign Chat 组件库（AG-UI 协议）',
    supportsA2UI: false,

    renderMessages(props: EngineRenderProps): VNode {
      // ── Stub 模式：TDesign 包未加载 ──
      if (!TDesignChatLoaded || !Chatbot) {
        return h('div', { class: 'tdesign-stub', style: { padding: '24px', textAlign: 'center' } }, [
          h('div', { style: { fontSize: '48px', marginBottom: '16px' } }, '💬'),
          h('h3', { style: { margin: '0 0 8px', color: 'var(--ion-text-color, #333)' } }, 'TDesign Chat 即将上线'),
          h('p', { style: { margin: '0 0 8px', color: 'var(--ion-color-medium, #666)', fontSize: '14px' } },
            '正在集成 @tdesign-vue-next/chat 组件库...',
          ),
          h('p', { style: { margin: '0', color: 'var(--ion-color-medium, #666)', fontSize: '13px' } },
            '当前请使用「Ionic 默认」或「CopilotKit 风格」引擎',
          ),
        ])
      }

      // ── 真实 TDesign 渲染模式 ──
      // 将内部消息格式转换为 TDesign ChatBot 的 data 格式
      const chatData = transformMessagesToTDesign(props.messages)

      return h(Chatbot, {
        data: chatData,
        layout: 'both',
        autoScroll: true,
        animation: 'gradient',
        avatar: {
          ai: { url: '', content: 'AI' },
          user: { url: '', content: 'U' },
        },
        // 通过 chatServiceConfig 配置 AG-UI 端点
        chatServiceConfig: {
          endpoint: `${getAgentApiBase()}/api/chat`,
          protocol: 'agui',
          stream: true,
          headers: {
            'Content-Type': 'application/json',
            'X-Agent-Protocol': 'agui',
          },
          params: {
            protocol: 'agui',
          },
          // 将用户消息发送到后端
          onRequest: async (params: any) => {
            // 复用宿主的 onSend 回调
            if (params.content) {
              await props.onSend(params.content)
            }
            return { success: true }
          },
        },
        // 输入框配置
        inputProps: {
          disabled: props.streaming,
          placeholder: '输入消息...',
          onStop: () => {
            if (props.streaming) {
              props.onStop()
            }
          },
        },
        // 正在流式输出时的状态
        isStreamLoad: props.streaming,
        textLoading: props.streaming,
      })
    },

    destroy(): void {
      // TDesign ChatBot 无需手动清理（Vue 组件卸载时自动处理）
    },
  }
}

// ── 自动注册到 EngineRegistry ──
registerEngine('tdesign', createTDesignEngine)

// =============================================================================
// 辅助函数：内部消息格式 → TDesign 数据格式转换
// =============================================================================

/**
 * 将 useAgent 的 Message[] 转换为 TDesign ChatBot 的 data 格式。
 *
 * TDesign ChatBot.data 期望的结构：
 * ```
 * [{
 *   role: 'assistant' | 'user',
 *   content: string | AIMessageContent[],
 *   ...其他字段
 * }]
 * ```
 */
function transformMessagesToTDesign(messages: readonly any[]): any[] {
  if (!Array.isArray(messages) || messages.length === 0) {
    return []
  }

  return messages.map((msg) => {
    const base: Record<string, unknown> = {
      role: msg.role === 'user' ? 'user' : 'assistant',
      content: msg.content ?? '',
    }

    // 保留时间戳（如果有）
    if (msg.createdAt || msg.timestamp) {
      base.datetime = msg.createdAt ?? msg.timestamp
    }

    // 工具调用消息：转换为 TDesign 的 tool call 格式
    if (msg.toolCalls && Array.isArray(msg.toolCalls) && msg.toolCalls.length > 0) {
      base.content = msg.toolCalls.map((tc: any) => ({
        type: 'tool_call',
        id: tc.id,
        name: tc.name,
        arguments: tc.arguments ?? tc.function?.arguments,
        status: tc.status ?? 'pending',
        result: tc.result,
      }))
    }

    // 推理链内容
    if (msg.reasoning) {
      base.reasoning = {
        collapsed: true,
        header: '思考过程',
        content: typeof msg.reasoning === 'string' ? msg.reasoning : JSON.stringify(msg.reasoning),
      }
    }

    return base
  })
}
