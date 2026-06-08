<!--
  TDesignChatView.vue - 腾讯 TDesign 视觉风格的消息列表渲染器
  
  数据源：从 useAgent 通过 EngineRenderProps 拿到的 messages: readonly Message[]
  渲染层：用 TDesign 通用组件（ChatList / ChatItem / ChatThinking）拼装
  
  协议：AG-UI（与 Default 引擎共享同一份数据）
  
  关键设计：
  1. 不再使用 <Chatbot>（早期版本）—— 避免 ChatService 双消费 SSE 流
  2. ChatList 容器，渲染每条消息为 ChatItem
  3. user / assistant 用 role="user" / "ai" 区分（ChatItem 适配 TDesign 规范）
  4. streaming 时显示 ChatThinking "正在思考..." 
  5. tool_calls 渲染为简单 div 卡片（无业务逻辑，纯展示；TDesign 通用
     组件 t-list / t-tag 不在 @tdesign-vue-next/chat 中，按需依赖会引入
     完整 tdesign-vue-next，因此用 CSS 变量 + 普通元素实现 TDesign 视觉）
  
  SPEC: /workspace/.trae/specs/agui-real-llm-path-completion/ Phase 4
-->
<template>
  <div class="tdesign-chat-view" :data-streaming="streaming">
    <!-- 空状态：欢迎文案 -->
    <div v-if="messages.length === 0" class="empty-state">
      <ChatThinking
        v-if="streaming"
        :content="'正在思考...'"
      />
      <div v-else class="welcome">
        <h3>{{ welcomeTitle }}</h3>
        <p>{{ welcomeSubtitle }}</p>
      </div>
    </div>

    <!-- 消息列表 -->
    <ChatList
      v-else
      :data="listData"
    >
      <template #content="{ item }">
        <ChatItem
          :key="item.id"
          :role="item.role === 'user' ? 'user' : 'assistant'"
          :name="item.name"
          :avatar="item.avatar"
          :content="item.content"
        />
      </template>
    </ChatList>

    <!-- 流式时尾部显示 thinking 指示器（覆盖在列表底部） -->
    <ChatThinking
      v-if="streaming && messages.length > 0"
      class="streaming-thinking"
      :content="'正在思考...'"
    />

    <!-- tool_calls 列表（每条 assistant 消息内嵌） -->
    <div v-if="hasToolCalls" class="tool-call-list">
      <div
        v-for="tc in allToolCalls"
        :key="tc.id"
        class="tool-call-item"
      >
        <div class="tool-call-card">
          <span class="tool-call-tag" :data-status="tc.status">
            {{ tc.name || tc.id }}
          </span>
          <span class="tool-call-status">{{ statusText(tc.status) }}</span>
          <span
            v-if="tc.needsConfirm && tc.status === 'pending'"
            class="tool-call-confirm-badge"
          >
            需确认
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ChatList, ChatItem, ChatThinking } from '@tdesign-vue-next/chat'
import type { Message, ToolCall } from '@/composables/useAgent'

/**
 * 适配 TDesign ChatList 的数据形状。
 * ChatList 期望 data: ChatItemData[]，所以从 Message[] 转换。
 */
interface TDChatItem {
  id: string
  role: 'user' | 'assistant'
  name: string
  avatar?: string
  content: string
}

// 引擎 props 透传
interface Props {
  messages: readonly Message[]
  status: string
  streaming: boolean
  onSend?: (text: string) => Promise<void>
  onStop?: () => void
  onConfirmTool?: (toolCallId: string, decision: string) => Promise<void>
  onCopyMessage?: (messageId: string) => Promise<void>
  onPresetClick?: (userText: string) => void
}

// 兼容不同调用方：EngineRenderProps 是完整契约，运行时由父级传入
const props = withDefaults(defineProps<Props>(), {
  messages: () => [] as readonly Message[],
})

const welcomeTitle = 'TDesign 风格引擎'
const welcomeSubtitle = '使用腾讯 TDesign 视觉组件渲染 Agent 对话。'

/** 把 Message[] 适配为 TDesign ChatList data 格式 */
const listData = computed<TDChatItem[]>(() => {
  return props.messages
    .filter((m) => m.role === 'user' || m.role === 'assistant')
    .map((m, idx) => {
      const isUser = m.role === 'user'
      // content 可能是 string 或 multimodal array（useAgent Message 契约）
      const text =
        typeof m.content === 'string'
          ? m.content
          : Array.isArray(m.content)
            ? m.content
                .filter((p) => p.type === 'text')
                .map((p) => (p as { text: string }).text)
                .join('')
            : ''
      return {
        id: `${m.role}-${idx}-${m.content.length}`,
        role: isUser ? 'user' : 'assistant',
        name: isUser ? 'You' : 'Assistant',
        avatar: isUser ? '🧑' : '🤖',
        content: text || (m.reasoning ? `__thinking__${m.reasoning}` : ''),
      }
    })
})

/** 收集所有 tool_calls（跨消息合并） */
const allToolCalls = computed<ToolCall[]>(() => {
  const tcs: ToolCall[] = []
  for (const m of props.messages) {
    if (m.tool_calls && m.tool_calls.length > 0) {
      tcs.push(...m.tool_calls)
    }
  }
  return tcs
})

const hasToolCalls = computed(() => allToolCalls.value.length > 0)

function statusText(status: string | undefined): string {
  switch (status) {
    case 'pending':
      return '待执行'
    case 'running':
      return '执行中...'
    case 'success':
      return '完成'
    case 'error':
    case 'failed':
      return '失败'
    default:
      return status || ''
  }
}
</script>

<style scoped>
.tdesign-chat-view {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  height: 100%;
  overflow-y: auto;
}

.empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: var(--td-text-color-secondary, #666);
  text-align: center;
}

.empty-state .welcome h3 {
  margin: 0 0 8px 0;
  color: var(--td-brand-color, #4f8cff);
}

.empty-state .welcome p {
  margin: 0;
  color: var(--td-text-color-secondary, #999);
}

.streaming-thinking {
  align-self: flex-start;
  margin-top: 8px;
}

.tool-call-list {
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tool-call-item {
  /* 列表项本身不需额外样式 */
}

.tool-call-card {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: var(--td-bg-color-container, #fff);
  border-radius: 8px;
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
}

.tool-call-tag {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  background: var(--td-bg-color-secondarycomponent, #f3f3f3);
  color: var(--td-text-color-primary, #333);
  font-size: 12px;
  font-weight: 500;
}

.tool-call-tag[data-status='running'] {
  background: var(--td-brand-color, #4f8cff);
  color: #fff;
}

.tool-call-tag[data-status='success'] {
  background: var(--td-success-color, #2ba471);
  color: #fff;
}

.tool-call-tag[data-status='error'],
.tool-call-tag[data-status='failed'] {
  background: var(--td-error-color, #d54941);
  color: #fff;
}

.tool-call-status {
  color: var(--td-text-color-secondary, #666);
  font-size: 12px;
}

.tool-call-confirm-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  background: var(--td-warning-color, #e37318);
  color: #fff;
  font-size: 11px;
  font-weight: 500;
}
</style>
