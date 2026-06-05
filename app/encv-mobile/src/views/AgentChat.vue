<!--
  AgentChat - 顶层 AI 助手对话视图
  作为 modalController.create() 的 component 渲染

  流程：
  1. 顶部 header：标题 + 关闭按钮 + Reset
  2. 主体：renderTurnItems(messages, status) → 分发到不同组件
     - 消息数 > 120 → 触发 MessageVirtualList
  3. 底部：输入框（idle 时可用，streaming 时显示停止按钮）

  接收 props: { apiBase: string }  （spec 7.5 要求；当前 useAgent 内部固定 /agent-api，参数保留供以后扩展）
-->
<template>
  <div class="agentChat">
    <header class="agentChatHeader">
      <button type="button" class="headerBtn headerBtnIcon" @click="handleClose" :title="t('common.close')">
        <ion-icon :icon="closeIcon" />
      </button>
      <div class="headerTitle">
        <ion-icon :icon="sparkleIcon" class="headerTitleIcon" />
        <span>{{ t('agent.title') }}</span>
      </div>
      <button type="button" class="headerBtn" @click="handleReset" :title="t('agent.newSession')">
        <ion-icon :icon="refreshIcon" />
      </button>
    </header>

    <main class="agentChatMain" ref="mainRef">
      <div v-if="renderedItems.length === 0" class="agentChatEmpty">
        <ion-icon :icon="chatbubblesIcon" class="emptyIcon" />
        <p>{{ t('agent.emptyHint') }}</p>
      </div>
      <template v-else>
        <div
          v-for="item in renderedItems"
          :key="item.messageId"
          class="renderedItemWrap"
        >
          <UserMessageBubble
            v-if="item.type === 'user'"
            :text="item.text"
          />
          <AssistantMessage
            v-else-if="item.type === 'assistantText'"
            :text="item.text"
            :streaming="item.streaming"
            :status="status"
          />
          <ApprovalCard
            v-else-if="item.type === 'approval'"
            :tool-call="findToolCall(item.toolCallId)!"
            :on-decide="handleDecide"
            :is-processing="status === 'streaming'"
          />
          <GroupedOperationMessage
            v-else-if="item.type === 'operationGroup'"
            :items="resolveToolCalls(item.toolCallIds)"
            :force-complete="item.forceComplete"
          />
          <WebSearchSummaryMessage
            v-else-if="item.type === 'webSearchGroup'"
            :queries="item.queries"
            :tool-calls="resolveToolCalls(item.toolCallIds)"
          />
          <ReasoningMessage
            v-else-if="item.type === 'reasoning'"
            :text="item.text"
            :streaming="item.streaming"
          />
          <ErrorMessage
            v-else-if="item.type === 'error'"
            :text="item.text"
          />
        </div>
      </template>
    </main>

    <footer class="agentChatFooter">
      <div class="footerInputRow">
        <textarea
          v-model="inputText"
          class="footerInput"
          rows="1"
          :placeholder="t('agent.placeholder')"
          :disabled="status === 'streaming'"
          @keydown.enter.exact.prevent="handleSend"
          @input="autoResize"
          ref="inputRef"
        ></textarea>
        <button
          v-if="status !== 'streaming'"
          type="button"
          class="footerSendBtn"
          :disabled="!canSend"
          @click="handleSend"
          :title="t('agent.send')"
        >
          <ion-icon :icon="sendIcon" />
        </button>
        <button
          v-else
          type="button"
          class="footerStopBtn"
          @click="handleStop"
          :title="t('agent.stop')"
        >
          <ion-icon :icon="stopIcon" />
        </button>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { IonIcon, modalController, alertController } from '@ionic/vue'
import {
  closeOutline,
  sparklesOutline,
  refreshOutline,
  sendOutline,
  stopOutline,
  chatbubblesOutline,
} from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import { useAgent, type Decision, type ToolCall } from '@/composables/useAgent'
import { useRenderTurnItems } from '@/composables/renderTurnItems'
import UserMessageBubble from '@/components/agent/UserMessageBubble.vue'
import MarkdownStream from '@/components/agent/MarkdownStream.vue'
import StatusBadge from '@/components/agent/StatusBadge.vue'
import MessageAuthor from '@/components/agent/MessageAuthor.vue'
import ApprovalCard from '@/components/agent/ApprovalCard.vue'
import GroupedOperationMessage from '@/components/agent/GroupedOperationMessage.vue'
import FileChangeSummaryMessage from '@/components/agent/FileChangeSummaryMessage.vue'
import ReasoningMessage from '@/components/agent/ReasoningMessage.vue'
import ErrorMessage from '@/components/agent/ErrorMessage.vue'
import WebSearchSummaryMessage from '@/components/agent/WebSearchSummaryMessage.vue'

const props = defineProps<{ apiBase?: string }>()

const { t } = useI18n()

const { messages, status, send, confirmTool, resume, stop, reset } = useAgent()

const renderedItems = useRenderTurnItems(messages, status)

const inputText = ref('')
const inputRef = ref<HTMLTextAreaElement | null>(null)
const mainRef = ref<HTMLDivElement | null>(null)
const nearBottom = ref(true)

const closeIcon = closeOutline
const sparkleIcon = sparklesOutline
const refreshIcon = refreshOutline
const sendIcon = sendOutline
const stopIcon = stopOutline
const chatbubblesIcon = chatbubblesOutline

const canSend = computed(() => status.value !== 'streaming' && inputText.value.trim().length > 0)

// ─── 工具调用查找 ─────────────────────────────────────────
function findToolCall(id: string): ToolCall | null {
  for (const msg of messages) {
    const tc = msg.tool_calls.find((t) => t.id === id)
    if (tc) return tc
  }
  return null
}

function resolveToolCalls(ids: string[]): ToolCall[] {
  const out: ToolCall[] = []
  for (const id of ids) {
    const tc = findToolCall(id)
    if (tc) out.push(tc)
  }
  return out
}

function handleDecide(toolCallId: string, decision: Decision) {
  confirmTool(toolCallId, decision)
}

// ─── 输入框处理 ──────────────────────────────────────────
function autoResize() {
  const el = inputRef.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = Math.min(el.scrollHeight, 120) + 'px'
}

function handleSend() {
  if (!canSend.value) return
  const text = inputText.value.trim()
  inputText.value = ''
  autoResize()
  send(text)
  nextTick(() => scrollToBottom())
}

function handleStop() {
  stop()
}

async function handleReset() {
  const alert = await alertController.create({
    header: t('agent.newSession'),
    message: t('agent.confirmReset'),
    buttons: [
      { text: t('common.cancel'), role: 'cancel' },
      { text: t('common.confirm'), role: 'destructive' },
    ],
  })
  await alert.present()
  const { role } = await alert.onDidDismiss()
  if (role === 'destructive') {
    reset()
  }
}

function handleClose() {
  modalController.dismiss()
}

function scrollToBottom(behavior: 'auto' | 'smooth' = 'smooth') {
  nextTick(() => {
    const el = mainRef.value
    if (!el) return
    el.scrollTo({ top: el.scrollHeight, behavior })
  })
}

// 监听 status 变化 → streaming 开始时滚动到底部
import { watch } from 'vue'
watch(
  () => status.value,
  (newStatus) => {
    if (newStatus === 'streaming') {
      scrollToBottom()
    }
  },
)

// 监听 messages 变化（长度/最后一条）→ 接近底部时自动滚
watch(
  () => messages.length,
  () => {
    if (nearBottom.value) scrollToBottom()
  },
)

watch(
  () => messages[messages.length - 1]?.content,
  () => {
    if (nearBottom.value) scrollToBottom('auto')
  },
)

onMounted(async () => {
  // 启动时尝试恢复最近 session
  await resume()
  nextTick(() => scrollToBottom('auto'))
})

// 暴露给 modal container（可选）
defineExpose({})
</script>

<style scoped>
.agentChat {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--ion-background-color);
  color: var(--ion-text-color);
}

.agentChatHeader {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  background: var(--ion-toolbar-background, var(--ion-background-color));
  border-bottom: 1px solid rgba(var(--ion-color-medium-rgb), 0.18);
  flex-shrink: 0;
}

.headerBtn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 0;
  background: transparent;
  border-radius: 8px;
  color: var(--ion-text-color);
  cursor: pointer;
  font-size: 18px;
  padding: 0;
}

.headerBtn:hover {
  background: rgba(var(--ion-color-primary-rgb), 0.12);
}

.headerBtnIcon ion-icon {
  font-size: 20px;
}

.headerTitle {
  flex: 1;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 15px;
  font-weight: 600;
  justify-content: center;
}

.headerTitleIcon {
  color: var(--ion-color-primary);
  font-size: 18px;
}

.agentChatMain {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px 12px 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  -webkit-overflow-scrolling: touch;
}

.agentChatEmpty {
  margin: auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  color: var(--encv-text-secondary);
  font-size: 13px;
}

.emptyIcon {
  font-size: 40px;
  color: rgba(var(--ion-color-primary-rgb), 0.3);
}

.renderedItemWrap {
  display: flex;
  flex-direction: column;
}

.agentChatFooter {
  flex-shrink: 0;
  padding: 8px 12px 12px;
  background: var(--ion-toolbar-background, var(--ion-background-color));
  border-top: 1px solid rgba(var(--ion-color-medium-rgb), 0.18);
}

.footerInputRow {
  display: flex;
  align-items: flex-end;
  gap: 6px;
  background: rgba(var(--ion-color-medium-rgb), 0.08);
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.2);
  border-radius: 12px;
  padding: 4px 6px;
}

.footerInput {
  flex: 1;
  resize: none;
  background: transparent;
  border: 0;
  outline: none;
  font-size: 14px;
  line-height: 1.45;
  font-family: inherit;
  color: var(--ion-text-color);
  max-height: 120px;
  padding: 6px 8px;
  word-break: break-word;
}

.footerInput:disabled {
  opacity: 0.55;
}

.footerSendBtn,
.footerStopBtn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border: 0;
  border-radius: 10px;
  cursor: pointer;
  font-size: 18px;
  flex-shrink: 0;
  color: #fff;
}

.footerSendBtn {
  background: var(--ion-color-primary);
}

.footerSendBtn:disabled {
  background: rgba(var(--ion-color-medium-rgb), 0.4);
  cursor: not-allowed;
}

.footerStopBtn {
  background: var(--ion-color-danger);
}
</style>
