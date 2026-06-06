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
      <button type="button" class="headerBtn" @click="handleOpenHistory" :title="t('agent.history')">
        <ion-icon :icon="timeIcon" />
      </button>
      <div class="headerTitle">
        <ion-icon :icon="sparkleIcon" class="headerTitleIcon" />
        <span>{{ t('agent.title') }}</span>
      </div>
      <button type="button" class="headerBtn" @click="handleNewSession" :title="t('agent.newSession')">
        <ion-icon :icon="addIcon" />
      </button>
    </header>

    <div class="agentChatToolbar">
      <label class="toolbarField">
        <span class="toolbarLabel">{{ t('agent.model') }}</span>
        <select v-model="selectedModel" class="toolbarSelect" :disabled="status === 'streaming' || modelsLoading">
          <option v-if="modelsLoading" value="" disabled>{{ t('agent.loadingModels') }}...</option>
          <option v-else-if="modelsError" value="" disabled>{{ t('agent.modelsError') }}</option>
          <option v-for="m in availableModels" :key="m.id" :value="m.id">{{ m.name }}</option>
        </select>
      </label>
      <label class="toolbarField toolbarFieldNarrow">
        <span class="toolbarLabel">{{ t('agent.temperature') }}</span>
        <input
          v-model.number="temperature"
          type="number"
          min="0"
          max="2"
          step="0.1"
          class="toolbarInput"
          :disabled="status === 'streaming'"
        />
      </label>
    </div>

    <main class="agentChatMain" ref="mainRef">
      <!-- 空状态（无消息时显示） -->
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
            :on-retry="() => handleRetryError(item)"
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
          @keydown.shift.enter.exact.prevent="handleSend"
          @keydown.meta.enter.exact.prevent="handleSend"
          @keydown.ctrl.enter.exact.prevent="handleSend"
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
      <div class="footerHint">{{ t('agent.inputHint') }}</div>
    </footer>

    <div v-if="historyOpen" class="historyOverlay" @click.self="historyOpen = false">
      <div class="historyPanel">
        <div class="historyHeader">
          <h3>{{ t('agent.history') }}</h3>
          <button type="button" class="headerBtn" @click="handleClose" :title="t('common.close')">
            <ion-icon :icon="closeIcon" />
          </button>
        </div>
        <div class="historyList">
          <div
            v-for="s in sessions"
            :key="s.id"
            class="historyItem"
            :class="{ historyItemActive: s.id === currentSessionId }"
            @click="switchSession(s.id); historyOpen = false"
          >
            <ion-icon :icon="chatbubblesIcon" class="historyItemIcon" />
            <div class="historyItemMain">
              <p class="historyItemTitle">{{ s.title || '(空)' }}</p>
              <p class="historyItemMeta">
                {{ s.messageCount }} {{ t('agent.messages') }}
              </p>
            </div>
            <button
              type="button"
              class="historyItemDelete"
              @click.stop="handleDeleteSession(s.id, $event)"
              :title="t('agent.deleteSession')"
            >
              <ion-icon :icon="closeIcon" />
            </button>
          </div>
          <div v-if="sessions.length === 0" class="historyEmpty">
            <p>{{ t('agent.noHistory') }}</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { IonIcon, modalController, alertController } from '@ionic/vue'
import {
  closeOutline,
  sparklesOutline,
  addOutline,
  sendOutline,
  stopOutline,
  chatbubblesOutline,
  timeOutline,
} from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import { useAgent, type Decision, type ToolCall } from '@/composables/useAgent'
import { useRenderTurnItems } from '@/composables/renderTurnItems'
import UserMessageBubble from '@/components/agent/UserMessageBubble.vue'
import ApprovalCard from '@/components/agent/ApprovalCard.vue'
import GroupedOperationMessage from '@/components/agent/GroupedOperationMessage.vue'
import ReasoningMessage from '@/components/agent/ReasoningMessage.vue'
import ErrorMessage from '@/components/agent/ErrorMessage.vue'
import WebSearchSummaryMessage from '@/components/agent/WebSearchSummaryMessage.vue'

const { t } = useI18n()

// Agent API 基础路径（与 useAgent.ts 保持一致）
const AGENT_API_BASE = '/agent-api'

const { messages, status, send, confirmTool, resume, stop, newSession, switchSession, deleteSession, sessions, currentSessionId } = useAgent()

const renderedItems = useRenderTurnItems(messages, status)

const inputText = ref('')
const inputRef = ref<HTMLTextAreaElement | null>(null)
const mainRef = ref<HTMLDivElement | null>(null)
const nearBottom = ref(true)

const closeIcon = closeOutline
const sparkleIcon = sparklesOutline
const addIcon = addOutline
const sendIcon = sendOutline
const stopIcon = stopOutline
const chatbubblesIcon = chatbubblesOutline
const timeIcon = timeOutline
const historyOpen = ref(false)

const canSend = computed(() => status.value !== 'streaming' && inputText.value.trim().length > 0)

// ─── 模型选择（动态从 API 获取） ────────────────────────────
interface ModelOption {
  id: string
  name: string
  provider: string
}

const availableModels = ref<ModelOption[]>([])
const modelsLoading = ref(true)
const modelsError = ref('')

async function fetchModels() {
  modelsLoading.value = true
  modelsError.value = ''
  try {
    const res = await fetch(`${AGENT_API_BASE}/api/models`)
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const data = await res.json()
    availableModels.value = (data.models || []).map((m: any) => ({
      id: m.id,
      name: m.name || m.id,
      provider: m.provider || 'unknown',
    }))
    // 如果当前选中的模型不在列表中，切换到默认值
    if (availableModels.value.length > 0 && !availableModels.value.some(m => m.id === selectedModel.value)) {
      selectedModel.value = data.defaultModel || availableModels.value[0].id
    }
  } catch (e: any) {
    console.error('[AgentChat] fetchModels failed:', e)
    modelsError.value = e?.message || String(e)
    // fallback：保留空列表，用户无法选择但不会崩溃
  } finally {
    modelsLoading.value = false
  }
}

const SELECTED_MODEL_KEY = 'encv-agent-selected-model'
const TEMPERATURE_KEY = 'encv-agent-temperature'
const storedModel = (() => {
  try {
    return localStorage.getItem(SELECTED_MODEL_KEY) || 'gpt-4o-mini'
  } catch {
    return 'gpt-4o-mini'
  }
})()
const storedTemp = (() => {
  try {
    const v = localStorage.getItem(TEMPERATURE_KEY)
    const n = v == null ? 0.7 : Number(v)
    return Number.isFinite(n) ? n : 0.7
  } catch {
    return 0.7
  }
})()
const selectedModel = ref<string>(storedModel)
const temperature = ref<number>(storedTemp)
watch(selectedModel, (v) => {
  try { localStorage.setItem(SELECTED_MODEL_KEY, v) } catch { /* ignore */ }
})
watch(temperature, (v) => {
  try { localStorage.setItem(TEMPERATURE_KEY, String(v)) } catch { /* ignore */ }
})

// ─── 工具调用查找 ─────────────────────────────────────────
function findToolCall(id: string): ToolCall | null {
  for (const msg of messages.value) {
    const tc = msg.tool_calls.find((t: ToolCall) => t.id === id)
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

/**
 * 重试一条出错的消息：清除 error 标记 + 删除关联的 assistant 消息 + 重新发送
 */
function handleRetryError(item: { type: 'error'; messageIndex: number }) {
  // 找到对应的 user 消息（error item 的 messageIndex 指向原始消息索引）
  const idx = item.messageIndex
  if (idx < 0 || idx >= messages.value.length) return

  const targetMsg = messages.value[idx]
  if (!targetMsg || targetMsg.role !== 'user') return

  const text = targetMsg.content
  // 清除错误标记
  delete targetMsg.error

  // 删除该 user 消息之后的所有消息（包括空的 assistant 占位 + 任何已产生的回复）
  messages.value.splice(idx)

  // 重新发送
  send(text)
  nextTick(() => scrollToBottom())
}

async function handleNewSession() {
  // 如果当前 session 已经有消息，弹确认
  if (messages.value.length > 0) {
    const alert = await alertController.create({
      header: t('agent.newSession'),
      message: t('agent.confirmNewSession'),
      buttons: [
        { text: t('common.cancel'), role: 'cancel' },
        { text: t('common.confirm'), role: 'destructive' },
      ],
    })
    await alert.present()
    const { role } = await alert.onDidDismiss()
    if (role !== 'destructive') return
  }
  newSession()
}

async function handleOpenHistory() {
  await Promise.resolve()
  historyOpen.value = true
}

async function handleDeleteSession(sessionId: string, event: Event) {
  event.stopPropagation()
  const alert = await alertController.create({
    header: t('agent.deleteSession'),
    message: t('agent.confirmDeleteSession'),
    buttons: [
      { text: t('common.cancel'), role: 'cancel' },
      { text: t('common.confirm'), role: 'destructive' },
    ],
  })
  await alert.present()
  const { role } = await alert.onDidDismiss()
  if (role === 'destructive') {
    deleteSession(sessionId)
  }
}

function handleClose() {
  historyOpen.value = false
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
  () => messages.value.length,
  () => {
    if (nearBottom.value) scrollToBottom()
  },
)

watch(
  () => messages.value[messages.value.length - 1]?.content,
  () => {
    if (nearBottom.value) scrollToBottom('auto')
  },
)

onMounted(async () => {
  // 动态获取可用模型列表（不阻塞 UI）
  fetchModels()
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

/* ── Toolbar (model / temperature) ─────────────────── */
.agentChatToolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 12px;
  background: rgba(var(--ion-color-medium-rgb), 0.06);
  border-bottom: 1px solid rgba(var(--ion-color-medium-rgb), 0.12);
  flex-shrink: 0;
}

.toolbarField {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
}

.toolbarFieldNarrow {
  min-width: 70px;
}

.toolbarLabel {
  color: var(--ion-text-color-step-400, #888);
  font-size: 11px;
  white-space: nowrap;
}

.toolbarSelect {
  font-size: 12px;
  padding: 2px 4px;
  border-radius: 6px;
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.25);
  background: var(--ion-background-color);
  color: var(--ion-text-color);
  outline: none;
  max-width: 160px;
}

.toolbarInput {
  width: 52px;
  font-size: 12px;
  padding: 2px 4px;
  border-radius: 6px;
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.25);
  background: var(--ion-background-color);
  color: var(--ion-text-color);
  outline: none;
  text-align: center;
}

/* ── Footer hint ─────────────────────────────────────── */
.footerHint {
  text-align: center;
  font-size: 11px;
  color: var(--ion-text-color-step-350, #999);
  padding: 2px 0 0;
  user-select: none;
}

/* ── History overlay / panel ─────────────────────────── */
.historyOverlay {
  position: fixed;
  inset: 0;
  z-index: 100;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: flex-end;
  justify-content: center;
  animation: historyFadeIn 0.18s ease-out;
}

@keyframes historyFadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.historyPanel {
  width: 100%;
  max-width: 420px;
  max-height: 60vh;
  background: var(--ion-background-color);
  border-radius: 16px 16px 0 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 -4px 24px rgba(0, 0, 0, 0.2);
}

.historyHeader {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(var(--ion-color-medium-rgb), 0.15);
  flex-shrink: 0;
}

.historyHeader h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}

.historyList {
  overflow-y: auto;
  flex: 1;
  padding: 4px 0;
}

.historyItem {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  cursor: pointer;
  transition: background 0.12s;
}

.historyItem:hover,
.historyItem:active {
  background: rgba(var(--ion-color-primary-rgb), 0.08);
}

.historyItemActive {
  background: rgba(var(--ion-color-primary-rgb), 0.1);
}

.historyItemActive .historyItemTitle {
  font-weight: 600;
}

.historyItemIcon {
  font-size: 22px;
  color: var(--ion-color-primary);
  flex-shrink: 0;
}

.historyItemMain {
  flex: 1;
  min-width: 0;
}

.historyItemTitle {
  margin: 0;
  font-size: 13px;
  color: var(--ion-text-color);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.historyItemMeta {
  margin: 2px 0 0;
  font-size: 11px;
  color: var(--ion-text-color-step-400);
}

.historyItemDelete {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: 0;
  border-radius: 50%;
  background: transparent;
  color: var(--ion-text-color-step-350);
  cursor: pointer;
  flex-shrink: 0;
  opacity: 0;
  transition: opacity 0.15s;
}

.historyItem:hover .historyItemDelete {
  opacity: 1;
}

.historyEmpty {
  text-align: center;
  padding: 32px 16px;
  color: var(--ion-text-color-step-350);
  font-size: 13px;
}
</style>
