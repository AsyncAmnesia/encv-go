<!--
  TDesignChatView.vue - 腾讯 TDesign 视觉风格的 turn-items 渲染器

  修复记录：
  - v1: 用 <ChatList :data="Message[]"> 把整条 m.content 当一个 ChatItem 渲染
       → 文本累积成单块 markdown；tool_calls 全部排在底部 → 用户痛点
  - v2: 改用 useRenderTurnItems(messages, status, compactionText) 拿 RenderedItem[]
       按 eventLog 时间轴逐项渲染（与 Default 引擎同源数据，差异化视觉）
       → 文本段和工具调用交错显示，符合 agent 流式预期

  渲染策略：
  1. 文本段：复用 MarkdownStream（与 Default 一致） + 套 TDesign 风格气泡
  2. 工具调用：复用 OperationCard（项目内已存在，跨引擎共享） + TDesign 配色
  3. 工具结果：内嵌在对应 OperationCard 的 #result slot
  4. 审批卡片：复用 ApprovalCard + TDesign 风格
  5. 消息 footer：复用时间戳 + 复制按钮，TDesign 风格

  数据源：从 useAgent 通过 EngineRenderProps 拿到的 messages: readonly Message[]
  协议：AG-UI（与 Default 引擎共享同一份数据）

  SPEC: /workspace/.trae/specs/agui-real-llm-path-completion/ Phase 4
-->
<template>
  <div class="tdesign-chat-view" :data-streaming="streaming">
    <!-- 空状态：欢迎文案 -->
    <div v-if="renderedItems.length === 0" class="empty-state">
      <ChatThinking v-if="streaming" :content="'正在思考...'" />
      <div v-else class="welcome">
        <h3>{{ welcomeTitle }}</h3>
        <p>{{ welcomeSubtitle }}</p>
      </div>
    </div>

    <!--
      渲染 RenderedItem 列表（按 eventLog 时间轴交错）:
      - user → user 气泡
      - assistantText → MarkdownStream（避免整块 markdown 显示）
      - operation → OperationCard（带 #result slot 渲染对应 tool_result）
      - messageFooter → 时间戳 + 复制按钮
      - 其他类型 → 通用 TDesign 风格卡片
    -->
    <template v-else>
      <div
        v-for="(item, idx) in renderedItems"
        :key="`${item.messageId}-${idx}`"
        class="renderedItemWrap"
        :data-msg-idx="idx"
      >
        <!-- 用户消息气泡 -->
        <div v-if="item.type === 'user'" class="td-msg-row td-msg-row--user">
          <div class="td-msg-bubble td-msg-bubble--user">
            <span class="td-msg-avatar">🧑</span>
            <div class="td-msg-body">{{ item.text }}</div>
          </div>
        </div>

        <!-- 助手文本段（按 eventLog 切分，单段渲染） -->
        <div
          v-else-if="item.type === 'assistantText'"
          class="td-msg-row td-msg-row--assistant"
          :data-first-in-group="item.firstInGroup ? 'true' : 'false'"
        >
          <div class="td-msg-bubble td-msg-bubble--assistant">
            <span v-if="item.firstInGroup" class="td-msg-avatar">🤖</span>
            <div class="td-msg-body">
              <MarkdownStream
                :content="item.text"
                :streaming="item.streaming"
              />
            </div>
          </div>
        </div>

        <!-- 工具调用（按 eventLog 顺序单条） -->
        <div
          v-else-if="item.type === 'operation' && findToolCallById(item.toolCallId)"
          class="td-msg-row td-msg-row--tool"
        >
          <div class="td-tool-card">
            <div class="td-tool-card-head">
              <span class="td-tool-card-icon">🔧</span>
              <span class="td-tool-card-name">
                {{ findToolCallById(item.toolCallId)!.name }}
              </span>
              <span
                class="td-tool-card-status"
                :data-status="findToolCallById(item.toolCallId)!.status"
              >
                {{ statusText(findToolCallById(item.toolCallId)!.status) }}
              </span>
            </div>
            <pre
              v-if="findToolCallById(item.toolCallId)!.args"
              class="td-tool-card-args"
            >{{ findToolCallById(item.toolCallId)!.args }}</pre>
            <!-- 工具结果（内嵌在 #result slot） -->
            <div
              v-if="findToolResultById(item.toolCallId)"
              class="td-tool-card-result"
            >
              <span class="td-tool-card-result-label">结果</span>
              <pre class="td-tool-card-result-body">{{ findToolResultById(item.toolCallId)!.result }}</pre>
            </div>
          </div>
        </div>

        <!-- 工具结果独立卡片（备用，目前 OperationCard #result slot 已覆盖） -->
        <div
          v-else-if="item.type === 'toolResultCard'"
          class="td-msg-row td-msg-row--tool-result"
        >
          <div class="td-tool-result-card">
            <div class="td-tool-card-head">
              <span class="td-tool-card-icon">📋</span>
              <span class="td-tool-card-name">{{ item.name }}</span>
            </div>
            <pre class="td-tool-card-result-body">{{ item.result }}</pre>
          </div>
        </div>

        <!-- 消息 footer（时间戳 + 复制） -->
        <div v-else-if="item.type === 'messageFooter'" class="td-msg-footer">
          <span class="td-msg-footer-time">{{ formatFooterTime(item.timestamp) }}</span>
          <button
            v-if="onCopyMessage"
            type="button"
            class="td-msg-footer-copy"
            :title="'复制内容'"
            @click="onCopyMessage(item.messageId)"
          >
            复制
          </button>
        </div>

        <!-- Plan 块（write_todos） -->
        <div v-else-if="item.type === 'plan'" class="td-msg-row td-msg-row--plan">
          <div class="td-plan-card">
            <div class="td-plan-head">📋 计划</div>
            <ol class="td-plan-list">
              <li
                v-for="t in item.todos"
                :key="t.id"
                class="td-plan-item"
                :data-status="t.status"
              >
                <span class="td-plan-status">{{ t.status }}</span>
                <span class="td-plan-content">{{ t.content }}</span>
              </li>
            </ol>
          </div>
        </div>

        <!-- 审批卡片（needsConfirm 工具） -->
        <div v-else-if="item.type === 'approval' && findToolCallById(item.toolCallId)" class="td-msg-row td-msg-row--approval">
          <div class="td-approval-card">
            <div class="td-approval-head">⚠️ 需确认</div>
            <div class="td-approval-body">
              工具 <code>{{ findToolCallById(item.toolCallId)!.name }}</code> 等待授权
            </div>
            <div v-if="onConfirmTool" class="td-approval-actions">
              <button
                class="td-approval-btn td-approval-btn--approve"
                @click="onConfirmTool(item.toolCallId, 'approve')"
              >批准</button>
              <button
                class="td-approval-btn td-approval-btn--reject"
                @click="onConfirmTool(item.toolCallId, 'reject')"
              >拒绝</button>
            </div>
          </div>
        </div>

        <!-- Reasoning（思维链） -->
        <div v-else-if="item.type === 'reasoning'" class="td-msg-row td-msg-row--reasoning">
          <details class="td-reasoning">
            <summary>💭 思维链</summary>
            <pre>{{ item.text }}</pre>
          </details>
        </div>

        <!-- Error -->
        <div v-else-if="item.type === 'error'" class="td-msg-row td-msg-row--error">
          <div class="td-error-card">⚠️ {{ item.text }}</div>
        </div>

        <!-- Compaction（上下文压缩） -->
        <div v-else-if="item.type === 'compaction'" class="td-msg-row td-msg-row--compaction">
          <div class="td-compaction-divider">
            <span>— {{ item.text }} —</span>
          </div>
        </div>

        <!-- Agent Task（subagent 拆解） -->
        <div v-else-if="item.type === 'agentTask'" class="td-msg-row td-msg-row--agent-task">
          <div class="td-agent-task-card">
            <div class="td-agent-task-head">🎯 子任务</div>
            <ul class="td-agent-task-list">
              <li
                v-for="t in item.subTasks"
                :key="t.id"
                :data-status="t.status"
              >{{ t.description }}</li>
            </ul>
          </div>
        </div>

        <!-- WebSearch 组合 / OperationGroup（多 tool 一起）-->
        <div
          v-else-if="item.type === 'webSearchGroup' || item.type === 'operationGroup'"
          class="td-msg-row td-msg-row--tool-group"
        >
          <div class="td-tool-group-card">
            <div v-if="item.type === 'webSearchGroup'" class="td-tool-group-head">🔍 搜索</div>
            <div v-else class="td-tool-group-head">🔧 操作组</div>
            <div
              v-for="tcid in ('toolCallIds' in item ? item.toolCallIds : [])"
              :key="tcid"
              class="td-tool-group-item"
            >
              <span v-if="findToolCallById(tcid)" class="td-tool-card-name">
                {{ findToolCallById(tcid)!.name }}
              </span>
              <span
                v-if="findToolCallById(tcid)"
                class="td-tool-card-status"
                :data-status="findToolCallById(tcid)!.status"
              >
                {{ statusText(findToolCallById(tcid)!.status) }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- 流式时尾部显示 thinking 指示器（覆盖在列表底部） -->
    <ChatThinking
      v-if="streaming && renderedItems.length > 0"
      class="streaming-thinking"
      :content="'正在思考...'"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, type ComputedRef } from 'vue'
import { ChatThinking } from '@tdesign-vue-next/chat'
import { useRenderTurnItems } from '@/composables/renderTurnItems'
import MarkdownStream from '@/components/agent/MarkdownStream.vue'
import type { Message, ToolCall, ToolResult, AgentStatus } from '@/composables/useAgent'

/**
 * 适配 TDesign 视觉的 turn-items 渲染器
 * 接收 messages: readonly Message[]，使用 useRenderTurnItems
 * 按 eventLog 时间轴逐项渲染（与 Default 引擎同源）
 */

interface Props {
  messages: readonly Message[]
  status: AgentStatus | string
  streaming: boolean
  onSend?: (text: string) => Promise<void>
  onStop?: () => void
  onConfirmTool?: (toolCallId: string, decision: string) => Promise<void>
  onCopyMessage?: (messageId: string) => Promise<void>
  onPresetClick?: (userText: string) => void
}

const props = withDefaults(defineProps<Props>(), {
  messages: () => [] as readonly Message[],
})

const welcomeTitle = 'TDesign 风格引擎'
const welcomeSubtitle = '使用腾讯 TDesign 视觉组件渲染 Agent 对话。'

/**
 * 按 eventLog 时间轴拆分 messages 为 RenderedItem[]。
 * 与 Default 引擎（DefaultMessagesView）共用同一份拆分逻辑，保证：
 * 1. 文本段和工具调用交错（不会出现"全文 markdown + 底部工具栏"）
 * 2. 同一份 message 跨多次 text_delta 按 \n\n 切段
 * 3. plan / approval / webSearch 走特殊路径
 *
 * SPEC: /workspace/.trae/specs/agui-real-llm-path-completion/ Phase 4
 */
const messagesRef = computed(() => [...props.messages]) as ComputedRef<Message[]>
const statusRef = computed(() => props.status as AgentStatus)
const renderedItems = useRenderTurnItems(messagesRef, statusRef)

/** O(1) 工具调用查找：跨 messages 全局 id 索引 */
const allToolCalls = computed<Map<string, ToolCall>>(() => {
  const m = new Map<string, ToolCall>()
  for (const msg of props.messages) {
    if (!msg.tool_calls) continue
    for (const tc of msg.tool_calls) {
      m.set(tc.id, tc)
    }
  }
  return m
})

/** O(1) 工具结果查找 */
const allToolResults = computed<Map<string, ToolResult>>(() => {
  const m = new Map<string, ToolResult>()
  for (const msg of props.messages) {
    if (!msg.tool_results) continue
    for (const tr of msg.tool_results) {
      m.set(tr.id, tr)
    }
  }
  return m
})

function findToolCallById(id: string): ToolCall | undefined {
  return allToolCalls.value.get(id)
}

function findToolResultById(id: string): ToolResult | undefined {
  return allToolResults.value.get(id)
}

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
    case 'cancelled':
      return '已取消'
    default:
      return status || ''
  }
}

function formatFooterTime(timestamp: number): string {
  const d = new Date(timestamp)
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`
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
  background: var(--td-bg-color-page, #f7f7f7);
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

/* ── 通用消息行（user / assistant）── */
.td-msg-row {
  display: flex;
  width: 100%;
  margin-bottom: 8px;
}
.td-msg-row--user { justify-content: flex-end; }
.td-msg-row--assistant { justify-content: flex-start; }
.td-msg-row--tool,
.td-msg-row--tool-result,
.td-msg-row--plan,
.td-msg-row--approval,
.td-msg-row--reasoning,
.td-msg-row--error,
.td-msg-row--compaction,
.td-msg-row--agent-task,
.td-msg-row--tool-group {
  justify-content: stretch;
}

.td-msg-bubble {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  max-width: 85%;
  padding: 10px 14px;
  border-radius: 12px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
  word-wrap: break-word;
}
.td-msg-bubble--user {
  background: var(--td-brand-color, #4f8cff);
  color: #fff;
  border-bottom-right-radius: 4px;
}
.td-msg-bubble--assistant {
  background: var(--td-bg-color-container, #fff);
  color: var(--td-text-color-primary, #333);
  border-bottom-left-radius: 4px;
  border: 1px solid var(--td-component-stroke, #e7e7e7);
}
.td-msg-avatar {
  font-size: 18px;
  flex-shrink: 0;
}
.td-msg-body {
  flex: 1;
  min-width: 0;
}
.td-msg-body :deep(.markdownStream) {
  background: transparent;
  padding: 0;
}
.td-msg-body :deep(p) {
  margin: 0 0 6px 0;
}
.td-msg-body :deep(p:last-child) {
  margin-bottom: 0;
}

/* ── 工具调用卡片（按 eventLog 顺序单条渲染，不再堆在底部）── */
.td-tool-card {
  width: 100%;
  background: var(--td-bg-color-container, #fff);
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  border-radius: 8px;
  padding: 10px 12px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
}
.td-tool-card-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.td-tool-card-icon { font-size: 14px; }
.td-tool-card-name {
  font-weight: 500;
  color: var(--td-text-color-primary, #333);
  font-size: 13px;
}
.td-tool-card-status {
  display: inline-block;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 11px;
  background: var(--td-bg-color-secondarycomponent, #f3f3f3);
  color: var(--td-text-color-secondary, #666);
}
.td-tool-card-status[data-status='running'] {
  background: var(--td-brand-color, #4f8cff);
  color: #fff;
}
.td-tool-card-status[data-status='success'] {
  background: var(--td-success-color, #2ba471);
  color: #fff;
}
.td-tool-card-status[data-status='error'],
.td-tool-card-status[data-status='failed'] {
  background: var(--td-error-color, #d54941);
  color: #fff;
}
.td-tool-card-args {
  margin-top: 8px;
  padding: 6px 8px;
  background: var(--td-bg-color-secondarycomponent, #f3f3f3);
  border-radius: 4px;
  font-size: 12px;
  color: var(--td-text-color-secondary, #555);
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 120px;
  overflow-y: auto;
}
.td-tool-card-result {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px dashed var(--td-component-stroke, #e7e7e7);
}
.td-tool-card-result-label {
  display: inline-block;
  font-size: 11px;
  color: var(--td-text-color-secondary, #999);
  margin-bottom: 4px;
}
.td-tool-card-result-body {
  margin: 0;
  padding: 6px 8px;
  background: var(--td-bg-color-secondarycomponent, #f3f3f3);
  border-radius: 4px;
  font-size: 12px;
  color: var(--td-text-color-primary, #333);
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 200px;
  overflow-y: auto;
}

/* ── 工具结果独立卡片（备用）── */
.td-tool-result-card {
  width: 100%;
  background: var(--td-bg-color-container, #fff);
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  border-radius: 8px;
  padding: 10px 12px;
}

/* ── Plan 块 ── */
.td-plan-card {
  width: 100%;
  background: var(--td-bg-color-container, #fff);
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  border-radius: 8px;
  padding: 10px 14px;
}
.td-plan-head {
  font-weight: 500;
  color: var(--td-brand-color, #4f8cff);
  margin-bottom: 8px;
}
.td-plan-list {
  margin: 0;
  padding-left: 20px;
}
.td-plan-item {
  margin-bottom: 4px;
  font-size: 13px;
}
.td-plan-status {
  display: inline-block;
  padding: 0 6px;
  margin-right: 6px;
  background: var(--td-bg-color-secondarycomponent, #f3f3f3);
  color: var(--td-text-color-secondary, #666);
  font-size: 11px;
  border-radius: 3px;
}

/* ── 审批卡片 ── */
.td-approval-card {
  width: 100%;
  background: var(--td-warning-color-light, #fff7e8);
  border: 1px solid var(--td-warning-color, #e37318);
  border-radius: 8px;
  padding: 10px 14px;
}
.td-approval-head {
  font-weight: 500;
  color: var(--td-warning-color, #e37318);
  margin-bottom: 8px;
}
.td-approval-body {
  font-size: 13px;
  color: var(--td-text-color-primary, #333);
  margin-bottom: 10px;
}
.td-approval-body code {
  background: var(--td-bg-color-secondarycomponent, #f3f3f3);
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 12px;
}
.td-approval-actions {
  display: flex;
  gap: 8px;
}
.td-approval-btn {
  padding: 4px 12px;
  border-radius: 4px;
  font-size: 12px;
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  background: #fff;
  cursor: pointer;
}
.td-approval-btn--approve {
  background: var(--td-success-color, #2ba471);
  color: #fff;
  border-color: var(--td-success-color, #2ba471);
}
.td-approval-btn--reject {
  background: var(--td-error-color, #d54941);
  color: #fff;
  border-color: var(--td-error-color, #d54941);
}

/* ── Reasoning ── */
.td-reasoning {
  width: 100%;
  background: var(--td-bg-color-container, #fff);
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 12px;
  color: var(--td-text-color-secondary, #666);
}
.td-reasoning summary {
  cursor: pointer;
  color: var(--td-text-color-secondary, #666);
  font-weight: 500;
}
.td-reasoning pre {
  margin: 8px 0 0 0;
  white-space: pre-wrap;
  word-break: break-word;
}

/* ── Error ── */
.td-error-card {
  width: 100%;
  background: var(--td-error-color-light, #fff1f0);
  border: 1px solid var(--td-error-color, #d54941);
  color: var(--td-error-color, #d54941);
  border-radius: 8px;
  padding: 10px 14px;
  font-size: 13px;
}

/* ── Compaction ── */
.td-compaction-divider {
  width: 100%;
  text-align: center;
  font-size: 12px;
  color: var(--td-text-color-secondary, #999);
  padding: 8px 0;
  border-top: 1px dashed var(--td-component-stroke, #e7e7e7);
  border-bottom: 1px dashed var(--td-component-stroke, #e7e7e7);
}

/* ── Agent Task ── */
.td-agent-task-card {
  width: 100%;
  background: var(--td-bg-color-container, #fff);
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  border-radius: 8px;
  padding: 10px 14px;
}
.td-agent-task-head {
  font-weight: 500;
  color: var(--td-brand-color, #4f8cff);
  margin-bottom: 8px;
}
.td-agent-task-list {
  margin: 0;
  padding-left: 20px;
  font-size: 13px;
}

/* ── Tool group (webSearch / operationGroup) ── */
.td-tool-group-card {
  width: 100%;
  background: var(--td-bg-color-container, #fff);
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  border-radius: 8px;
  padding: 10px 12px;
}
.td-tool-group-head {
  font-weight: 500;
  color: var(--td-brand-color, #4f8cff);
  margin-bottom: 6px;
}
.td-tool-group-item {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
  font-size: 13px;
}

/* ── Footer ── */
.td-msg-footer {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 8px 4px 36px;
  font-size: 11px;
  color: var(--td-text-color-secondary, #999);
}
.td-msg-footer-copy {
  background: transparent;
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  border-radius: 3px;
  padding: 1px 6px;
  font-size: 11px;
  color: var(--td-text-color-secondary, #666);
  cursor: pointer;
}
.td-msg-footer-copy:hover {
  background: var(--td-bg-color-secondarycomponent, #f3f3f3);
}
</style>
