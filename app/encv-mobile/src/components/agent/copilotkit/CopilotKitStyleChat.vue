<!--
  CopilotKitStyleChat.vue - CopilotKit 风格聊天主容器

  布局结构（模仿 CopilotKit React 版）：
  ┌──────────────────────────────────────────────┐
  │  消息列表区域                                 │
  │  ┌─────┬────────────────────────────────┐    │
  │  │ 🤖  │  AssistantMessage / OperationCard│    │
  │  │ 48px│  （右侧内容区，flex:1）          │    │
  │  ├─────┼────────────────────────────────┤    │
  │  │ 👤  │  UserMessageBubble（右对齐）      │    │
  │  └─────┴────────────────────────────────┘    │
  │                                              │
  │  Suggestions Chip Bar（底部水平滚动）          │
  └──────────────────────────────────────────────┘

  与 Ionic 默认引擎的差异：
  - 每条消息左侧固定 48px 头像（非仅首段显示）
  - 右侧内容区更宽（头像不随文字换行）
  - 工具调用卡片带渐变边框 + 左侧彩色竖条
  - 消息出现 slide-up 过渡动画（300ms）
-->
<template>
  <div class="ckChat">
    <!-- 消息列表 -->
    <div class="ckChatMessages">
      <TransitionGroup name="ckMsg" tag="div" class="ckMsgList">
        <div
          v-for="item in renderedItems"
          :key="item.messageId"
          class="ckMsgRow"
          :class="{ 'ckMsgRow_user': item.type === 'user' }"
        >
          <!-- 左侧头像区（48px 固定宽） -->
          <div class="ckAvatar" :class="{ 'ckAvatar_user': item.type === 'user' }">
            <ion-icon :icon="sparklesIcon" v-if="item.type !== 'user'" />
            <ion-icon :icon="personIcon" v-else />
          </div>

          <!-- 右侧内容区 -->
          <div class="ckContent">
            <!-- 用户消息 -->
            <UserMessageBubble
              v-if="item.type === 'user'"
              :text="item.text"
            />

            <!-- AI 文本消息 -->
            <AssistantMessage
              v-else-if="item.type === 'assistantText'"
              :text="item.text"
              :streaming="item.streaming"
              :status="status"
              :compact="!item.firstInGroup"
            />

            <!-- 工具调用操作卡片（CopilotKit 风格渐变边框包裹） -->
            <div
              v-else-if="item.type === 'operation' && findToolCallById(item.toolCallId)"
              class="ckOperationWrapper"
            >
              <OperationCard
                :tool-call="findToolCallById(item.toolCallId)!"
                :streaming="item.streaming"
              >
                <template v-if="findToolResultById(item.toolCallId)" #result>
                  <MountListCard
                    v-if="findToolResultById(item.toolCallId)!.name === 'list_mounts'"
                    :result-json="findToolResultById(item.toolCallId)!.result"
                  />
                  <FileListCard
                    v-else-if="findToolResultById(item.toolCallId)?.name === 'list_files' || findToolResultById(item.toolCallId)?.name === 'stat_file'"
                    :result-json="findToolResultById(item.toolCallId)!.result"
                  />
                  <FileContentCard
                    v-else-if="findToolResultById(item.toolCallId)?.name === 'read_file'"
                    :result-json="findToolResultById(item.toolCallId)!.result"
                  />
                </template>
              </OperationCard>
            </div>

            <!-- 操作组（旧模式 fallback） -->
            <div
              v-else-if="item.type === 'operationGroup'"
              class="ckOperationWrapper"
            >
              <GroupedOperationMessage
                :items="resolveToolCalls(item.toolCallIds)"
                :results-by-call-id="resolveToolResultsByCallId(item.toolCallIds)"
                :force-complete="item.forceComplete"
              />
            </div>

            <!-- Approval 卡片 -->
            <ApprovalCard
              v-else-if="item.type === 'approval'"
              :tool-call="findToolCall(item.toolCallId)!"
              :on-decide="handleDecide"
              :is-processing="status === 'streaming'"
            />

            <!-- Plan 卡片 -->
            <PlanBlock
              v-else-if="item.type === 'plan'"
              :todos="item.todos"
              :streaming="item.streaming"
            />

            <!-- 推理过程 -->
            <ReasoningMessage
              v-else-if="item.type === 'reasoning'"
              :text="item.text"
              :streaming="item.streaming"
            />

            <!-- 错误消息 -->
            <ErrorMessage
              v-else-if="item.type === 'error'"
              :text="item.text"
              :on-retry="() => handleRetryError(item)"
            />

            <!-- WebSearch 摘要 -->
            <WebSearchSummaryMessage
              v-else-if="item.type === 'webSearchGroup'"
              :queries="item.queries"
              :tool-calls="resolveToolCalls(item.toolCallIds)"
            />

            <!-- Agent Task 消息 -->
            <AgentTaskMessage
              v-else-if="item.type === 'agentTask'"
              :sub-tasks="item.subTasks"
              :reasoning="item.reasoning"
            />

            <!-- Compaction 分隔线 -->
            <ContextCompactionDivider
              v-else-if="item.type === 'compaction'"
              :text="item.text"
            />

            <!-- 消息 Footer（时间戳） -->
            <div
              v-else-if="item.type === 'messageFooter'"
              class="ckMessageFooter"
            >
              <span class="ckFooterTime">{{ formatFooterTime(item.timestamp) }}</span>
            </div>
          </div>
        </div>
      </TransitionGroup>
    </div>

    <!-- 底部 Suggestions Bar -->
    <CopilotKitSuggestionsBar
      v-if="presets.length > 0"
      :presets="presets"
      :disabled="streaming"
      @pick="onPresetPick"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { IonIcon } from '@ionic/vue'
import {
  sparklesOutline,
  personOutline,
} from 'ionicons/icons'
import type { EngineRenderProps } from '@/composables/chatEngine'
import type { ToolCall, ToolResult, MockPreset } from '@/composables/useAgent'
import { useRenderTurnItems } from '@/composables/renderTurnItems'
import type { RenderedItem } from '@/composables/renderTurnItems'

// 复用现有子组件
import UserMessageBubble from '../UserMessageBubble.vue'
import AssistantMessage from '../AssistantMessage.vue'
import OperationCard from '../OperationCard.vue'
import GroupedOperationMessage from '../GroupedOperationMessage.vue'
import ApprovalCard from '../ApprovalCard.vue'
import PlanBlock from '../PlanBlock.vue'
import ReasoningMessage from '../ReasoningMessage.vue'
import ErrorMessage from '../ErrorMessage.vue'
import WebSearchSummaryMessage from '../WebSearchSummaryMessage.vue'
import AgentTaskMessage from '../AgentTaskMessage.vue'
import ContextCompactionDivider from '../ContextCompactionDivider.vue'
import MountListCard from '../MountListCard.vue'
import FileListCard from '../FileListCard.vue'
import FileContentCard from '../FileContentCard.vue'
import CopilotKitSuggestionsBar from './CopilotKitSuggestionsBar.vue'

const props = defineProps<EngineRenderProps>()

const sparklesIcon = sparklesOutline
const personIcon = personOutline

// 将 readonly Message[] 转为 computed 以供 useRenderTurnItems 使用
const renderedItems = useRenderTurnItems(
  computed(() => [...props.messages]),
  computed(() => props.status),
)

// ── Tool 查找辅助函数（与 AgentChat.vue 一致） ──

function findToolCall(id: string): ToolCall | null {
  for (const msg of props.messages) {
    const tc = msg.tool_calls.find((t: ToolCall) => t.id === id)
    if (tc) return tc
  }
  return null
}

function findToolResult(id: string): ToolResult | null {
  for (const msg of props.messages) {
    const tr = msg.tool_results.find((r: ToolResult) => r.id === id)
    if (tr) return tr
  }
  return null
}

function findToolCallById(id: string): ToolCall | null { return findToolCall(id) }
function findToolResultById(id: string): ToolResult | null { return findToolResult(id) }

function resolveToolCalls(ids: string[]): ToolCall[] {
  return ids.map((id) => findToolCall(id)).filter(Boolean) as ToolCall[]
}

function resolveToolResultsByCallId(ids: string[]): Record<string, ToolResult> {
  const map: Record<string, ToolResult> = {}
  for (const id of ids) {
    const r = findToolResult(id)
    if (r) map[id] = r
  }
  return map
}

// ── 回调处理 ──

function handleDecide(toolCallId: string, decision: string): void {
  props.onConfirmTool(toolCallId, decision)
}

function handleRetryError(_item: RenderedItem & { type: 'error' }): void {
  // 重试逻辑：重新发送最后一条用户消息
  const lastUserIdx = [...props.messages].reverse().findIndex((m) => m.role === 'user')
  if (lastUserIdx >= 0) {
    const lastUser = [...props.messages][props.messages.length - 1 - lastUserIdx]
    if (lastUser?.content && typeof lastUser.content === 'string') {
      props.onSend(lastUser.content)
    }
  }
}

/** 格式化 Footer 时间戳为 HH:mm */
function formatFooterTime(timestamp: number): string {
  const d = new Date(timestamp)
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  return `${hh}:${mm}`
}

// ── Presets（从 props 提取或使用空数组） ──
// EngineRenderProps 不直接包含 presets 列表，
// SuggestionsBar 通过外部注入或从 useAgent 获取
const presets = ref<MockPreset[]>([])

function onPresetPick(preset: MockPreset): void {
  props.onPresetClick(preset.userText)
}

// 暴露方法供外部设置 presets
defineExpose({
  setPresets(newPresets: MockPreset[]) {
    presets.value = newPresets
  },
})
</script>

<style scoped>
/* ── 主容器 ── */
.ckChat {
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 100%;
}

/* ── 消息列表区域 ── */
.ckChatMessages {
  flex: 1;
  overflow-y: auto;
  padding: 12px 16px;
  -webkit-overflow-scrolling: touch;
}

/* ── 消息列表容器 ── */
.ckMsgList {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

/* ── 单行消息（avatar + content） ── */
.ckMsgRow {
  display: flex;
  flex-direction: row;
  gap: 12px;
  align-items: flex-start;
  max-width: 100%;
}

/* 用户消息行：右对齐头像 */
.ckMsgRow_user {
  flex-direction: row-reverse;
}

/* ── 头像区（48px 固定尺寸） ── */
.ckAvatar {
  flex-shrink: 0;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: var(--ion-color-primary, #4f8cff);
  color: #ffffff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.12);
}

.ckAvatar_user {
  background: var(--ion-color-medium, #92949c);
}

.ckAvatar ion-icon {
  font-size: 22px;
  color: inherit;
}

/* ── 右侧内容区 ── */
.ckContent {
  flex: 1;
  min-width: 0;
  max-width: calc(100% - 52px);
}

/* ── CopilotKit 风格操作卡片包裹器（渐变边框 + 左竖条） ── */
.ckOperationWrapper {
  border-left: 3px solid var(--ion-color-primary, #4f8cff);
  border-radius: 0 8px 8px 0;
  padding-left: 12px;
  margin: 4px 0;
  background: linear-gradient(
    135deg,
    rgba(79, 140, 255, 0.04) 0%,
    rgba(79, 140, 255, 0.01) 100%
  );
  box-shadow:
    0 1px 2px rgba(0, 0, 0, 0.04),
    0 1px 4px rgba(79, 140, 255, 0.06);
  transition: box-shadow 200ms ease;
}

.ckOperationWrapper:hover {
  box-shadow:
    0 2px 4px rgba(0, 0, 0, 0.06),
    0 2px 8px rgba(79, 140, 255, 0.10);
}

/* ── 消息 Footer ── */
.ckMessageFooter {
  display: flex;
  align-items: center;
  padding: 2px 0 6px 0;
}

.ckFooterTime {
  font-size: 11px;
  color: var(--ion-color-medium, #92949c);
  opacity: 0.6;
}

/* ══════════════════════════════════════
   Transition 动画 —— 消息 slide-up 出现
   ══════════════════════════════════════ */

/* 进入 */
.ckMsg-enter-active {
  transition: opacity 300ms ease-out, transform 300ms ease-out;
}

/* 离开 */
.ckMsg-leave-active {
  transition: opacity 200ms ease-in, transform 200ms ease-in;
}

/* 进入起始状态 */
.ckMsg-enter-from {
  opacity: 0;
  transform: translateY(12px);
}

/* 离开目标状态 */
.ckMsg-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

/* 移动过渡（列表 reorder 时） */
.ckMsg-move {
  transition: transform 300ms ease-out;
}
</style>
