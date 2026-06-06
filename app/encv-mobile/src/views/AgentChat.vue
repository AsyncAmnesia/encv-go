<!--
  AgentChat - 顶层 AI 助手对话视图
  作为 modalController.create() 的 component 渲染

  流程：
  1. 顶部 header：标题 + 关闭按钮 + Reset
  2. 主体：renderTurnItems(messages, status) → 分发到不同组件
     - renderedItems.length <= 120 → 原生 v-for（性能足够）
     - renderedItems.length >  120 → MessageVirtualList（虚拟滚动）
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
          <template v-else-if="modelsError">
            <option value="" disabled>{{ modelsError }}</option>
            <!-- 错误时仍保留当前选中模型，确保用户可继续使用 -->
            <option v-if="selectedModel && !availableModels.some(m => m.id === selectedModel)" :value="selectedModel">{{ selectedModel }}</option>
          </template>
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

    <!--
      Task 26 (LAN Access)：局域网访问地址折叠面板。
      默认折叠（v-show），点击展开。挂载在 toolbar 下方、main 上方——
      该位置在视觉上属于"次要状态信息"，不会分散对话注意力。
      数据源：useAgent.getLanAccess() → GET /api/network/lan-access。
    -->
    <details class="lanAccessPanel" :open="lanAccessOpen" @toggle="lanAccessOpen = ($event.target as HTMLDetailsElement).open">
      <summary class="lanAccessSummary">
        <ion-icon :icon="globeIcon" class="lanAccessSummaryIcon" />
        <span class="lanAccessSummaryText">{{ t('agent.lanAccess') }}</span>
        <span v-if="lanAccesses.length > 0" class="lanAccessSummaryCount">{{ lanAccesses.length }}</span>
      </summary>
      <div class="lanAccessBody">
        <p class="lanAccessHelp">{{ t('agent.lanAccessHelp') }}</p>
        <div v-if="lanAccessLoading" class="lanAccessLoading">{{ t('settings.loading') }}</div>
        <div v-else-if="lanAccesses.length === 0" class="lanAccessEmpty">
          {{ t('agent.lanAccessEmpty') }}
        </div>
        <ul v-else class="lanAccessList">
          <li v-for="addr in lanAccesses" :key="addr.ip" class="lanAccessItem">
            <div class="lanAccessItemMain">
              <code class="lanAccessUrl">{{ addr.url }}</code>
              <span class="lanAccessInterface">{{ t('agent.lanAccessInterface', { name: addr.interface }) }}</span>
            </div>
            <button
              type="button"
              class="lanAccessCopyBtn"
              :title="t('agent.lanAccessCopy')"
              :aria-label="t('agent.lanAccessCopy')"
              @click="handleCopyLanAccess(addr.url)"
            >
              <ion-icon :icon="clipboardIcon" />
            </button>
          </li>
        </ul>
        <button
          type="button"
          class="lanAccessRefresh"
          @click="handleRefreshLanAccess"
          :disabled="lanAccessLoading"
        >
          <ion-icon :icon="refreshCircleIcon" />
          <span>{{ t('agent.lanAccessRefresh') }}</span>
        </button>
      </div>
    </details>

    <main class="agentChatMain" ref="mainRef" @scroll="onMainScroll">
      <!-- 空状态（无消息时显示） -->
      <div v-if="renderedItems.length === 0" class="agentChatEmpty">
        <ion-icon :icon="chatbubblesIcon" class="emptyIcon" />
        <p>{{ t('agent.emptyHint') }}</p>
      </div>
      <!-- 短会话（≤ 120）：原生 v-for（无虚拟化开销） -->
      <template v-else-if="renderedItems.length <= VIRTUAL_LIST_THRESHOLD">
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
          <PlanBlock
            v-else-if="item.type === 'plan'"
            :todos="item.todos"
            :streaming="item.streaming"
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
          <!-- Task 7：上下文自动压缩分隔线（不可展开） -->
          <ContextCompactionDivider
            v-else-if="item.type === 'compaction'"
            :text="item.text"
          />
          <!-- Task 22: agent task 消息（subagent 拆解的子任务列表） -->
          <AgentTaskMessage
            v-else-if="item.type === 'agentTask'"
            :sub-tasks="item.subTasks"
            :reasoning="item.reasoning"
          />
        </div>
      </template>
      <!-- 长会话（> 120）：虚拟滚动优化 -->
      <MessageVirtualList
        v-else
        ref="virtualListRef"
        :items="renderedItems"
      >
        <template #item="{ item }">
          <div class="renderedItemWrap">
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
            <PlanBlock
              v-else-if="item.type === 'plan'"
              :todos="item.todos"
              :streaming="item.streaming"
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
            <!-- Task 7：虚拟滚动分支同样渲染 ContextCompactionDivider -->
            <ContextCompactionDivider
              v-else-if="item.type === 'compaction'"
              :text="item.text"
            />
            <!-- Task 22: 虚拟滚动分支同样渲染 AgentTaskMessage -->
            <AgentTaskMessage
              v-else-if="item.type === 'agentTask'"
              :sub-tasks="item.subTasks"
              :reasoning="item.reasoning"
            />
          </div>
        </template>
      </MessageVirtualList>
    </main>

    <footer class="agentChatFooter">
      <!--
        Task 10: "/" 触发的命令面板（useSlashMenu + SlashMenu 组件）。
        模板挂在 footer 内，组件自身用 Teleport 把 overlay 提升到 body
        以避免被 textarea 滚动裁剪。
      -->
      <SlashMenu
        v-if="slashMenu.isOpen.value"
        :items="slashMenu.items.value"
        :query="slashMenu.query.value"
        :selected-index="slashMenu.selectedIndex.value"
        :on-apply="(id) => slashMenu.applyById(id)"
        :on-close="slashMenu.closeMenu"
        :on-selected-index-change="(n) => (slashMenu.selectedIndex.value = n)"
      />

      <!-- Task 12: 附件展示行（textarea 上方） -->
      <AttachmentTray
        v-if="attachments.length > 0"
        :attachments="attachments"
        :on-remove="removeAttachment"
      />

      <div class="footerInputRow" :class="{ 'footerInputRow-palette': slashMenu.isOpen.value }">
        <!-- Task 12: 附件 `+` 按钮 -->
        <button
          v-if="status !== 'streaming'"
          type="button"
          class="footerAttachBtn"
          :title="t('agent.attach')"
          :aria-label="t('agent.attach')"
          @click="triggerAttach"
        >
          <ion-icon :icon="attachIcon" />
        </button>
        <input
          ref="fileInputRef"
          type="file"
          multiple
          class="footerAttachInput"
          @change="handleAttachChange"
        />
        <textarea
          v-model="inputText"
          class="footerInput"
          rows="1"
          :placeholder="t('agent.placeholder')"
          :disabled="status === 'streaming'"
          @keydown.ctrl.enter.exact.prevent="handleSend"
          @keydown.meta.enter.exact.prevent="handleSend"
          @keydown="onTextareaKeydown"
          @input="onTextareaInput"
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
  attachOutline,
  globeOutline,
  clipboardOutline,
  refreshCircleOutline,
} from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import { getDeviceIdSync } from '@/composables/useDeviceId'
import { useAgent, type Decision, type ToolCall, getLanAccess, type LanAddress } from '@/composables/useAgent'
import { useRenderTurnItems } from '@/composables/renderTurnItems'
import { useAttachments } from '@/composables/useAttachments'
import { useSlashMenu } from '@/composables/useSlashMenu'
import { showToast } from '@/composables/useToast'
import UserMessageBubble from '@/components/agent/UserMessageBubble.vue'
import ApprovalCard from '@/components/agent/ApprovalCard.vue'
import GroupedOperationMessage from '@/components/agent/GroupedOperationMessage.vue'
import ReasoningMessage from '@/components/agent/ReasoningMessage.vue'
import ErrorMessage from '@/components/agent/ErrorMessage.vue'
import AssistantMessage from '@/components/agent/AssistantMessage.vue'
import WebSearchSummaryMessage from '@/components/agent/WebSearchSummaryMessage.vue'
import MessageVirtualList from '@/components/agent/MessageVirtualList.vue'
import PlanBlock from '@/components/agent/PlanBlock.vue'
import ContextCompactionDivider from '@/components/agent/ContextCompactionDivider.vue'
// Task 22：agent task 消息（subagent 拆解的子任务列表）
import AgentTaskMessage from '@/components/agent/AgentTaskMessage.vue'
import AttachmentTray from '@/components/agent/AttachmentTray.vue'
import SlashMenu from '@/components/agent/SlashMenu.vue'

const { t } = useI18n()

// Agent API 基础路径（与 useAgent.ts 保持一致）
const AGENT_API_BASE = '/agent-api'

const { messages, status, send, confirmTool, resume, stop, newSession, switchSession, deleteSession, sessions, currentSessionId } = useAgent()

// Task 12：附件管理（Composer `+` 按钮）
const {
  attachments,
  addFiles,
  removeAttachment,
  clearAttachments,
} = useAttachments({
  onError: (msg) => showToast({ message: msg, duration: 2400, color: 'warning' }),
})

// Task 7：把 i18n 解析后的 "上下文已自动压缩" 文本通过 computed
// 注入到 renderTurnItems，renderTurnItems 把它塞进 RenderedItem
// 供 ContextCompactionDivider 直接渲染。这里用 computed 而非
// t('agent.contextCompaction') 直接调用——renderTurnItems 的
// 第三个参数要 Ref/ComputedRef，让语言切换时自动重渲染。
const compactionText = computed(() => t('agent.contextCompaction'))

const renderedItems = useRenderTurnItems(messages, status, compactionText)

const inputText = ref('')
const inputRef = ref<HTMLTextAreaElement | null>(null)
const mainRef = ref<HTMLDivElement | null>(null)
const virtualListRef = ref<{ scrollToBottom: (behavior?: 'auto' | 'smooth') => void } | null>(null)
const nearBottom = ref(true)

/** 触发虚拟滚动的阈值（renderedItems 数量 > 此值时切换） */
const VIRTUAL_LIST_THRESHOLD = 120

const closeIcon = closeOutline
const sparkleIcon = sparklesOutline
const addIcon = addOutline
const sendIcon = sendOutline
const stopIcon = stopOutline
const chatbubblesIcon = chatbubblesOutline
const timeIcon = timeOutline
const attachIcon = attachOutline
const globeIcon = globeOutline
const clipboardIcon = clipboardOutline
const refreshCircleIcon = refreshCircleOutline
const historyOpen = ref(false)

// ── Task 26 (LAN Access) ───────────────────────────────────
// 折叠面板状态：默认收起。数据由 useAgent.getLanAccess() 拉取。
// 展开时才拉取（按需），关闭后保留缓存，避免反复网络请求。
const lanAccessOpen = ref(false)
const lanAccesses = ref<LanAddress[]>([])
const lanAccessLoading = ref(false)
const lanAccessLoaded = ref(false)

async function handleRefreshLanAccess(): Promise<void> {
  lanAccessLoading.value = true
  try {
    lanAccesses.value = await getLanAccess(0)
    lanAccessLoaded.value = true
  } finally {
    lanAccessLoading.value = false
  }
}

async function handleCopyLanAccess(url: string): Promise<void> {
  try {
    if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
      await navigator.clipboard.writeText(url)
      showToast({ message: t('agent.lanAccessCopied', { url }), duration: 1600, color: 'success' })
    } else {
      // Fallback：临时 textarea + execCommand（老 webview 兼容）
      const ta = document.createElement('textarea')
      ta.value = url
      ta.style.position = 'fixed'
      ta.style.left = '-9999px'
      document.body.appendChild(ta)
      ta.select()
      const ok = document.execCommand('copy')
      document.body.removeChild(ta)
      if (ok) {
        showToast({ message: t('agent.lanAccessCopied', { url }), duration: 1600, color: 'success' })
      } else {
        showToast({ message: t('agent.lanAccessCopyFailed'), duration: 1800, color: 'danger' })
      }
    }
  } catch {
    showToast({ message: t('agent.lanAccessCopyFailed'), duration: 1800, color: 'danger' })
  }
}

// 监听展开事件：用户首次展开时拉取一次。后续点击「刷新」按钮
// 可强制重拉。watch 比 onMounted 触发更精准——避免用户在折叠
// 面板被滚动出视野前白白消耗一次网络请求。
watch(lanAccessOpen, async (open) => {
  if (open && !lanAccessLoaded.value && !lanAccessLoading.value) {
    await handleRefreshLanAccess()
  }
})

// Task 12：隐藏 file input 的引用
const fileInputRef = ref<HTMLInputElement | null>(null)

function triggerAttach() {
  // 复用同一个 input：每次点击重置 value，确保选同一文件也能触发 change
  const el = fileInputRef.value
  if (!el) return
  el.value = ''
  el.click()
}

async function handleAttachChange(e: Event) {
  const target = e.target as HTMLInputElement
  const files = target.files
  if (!files || files.length === 0) return
  const result = await addFiles(files)
  if (result.rejected.length > 0) {
    const names = result.rejected.map((r) => r.name).join(', ')
    const sample = result.rejected[0]?.reason || '文件超限'
    showToast({
      message: `已跳过 ${result.rejected.length} 个文件（${names}）：${sample}`,
      duration: 3000,
      color: 'warning',
    })
  }
  // 清空 input.value 允许重复选同一文件
  target.value = ''
}

const canSend = computed(() => {
  if (status.value === 'streaming') return false
  // 文本非空 OR 至少一个附件都可以发送
  return inputText.value.trim().length > 0 || attachments.value.length > 0
})

// ─── Task 10: "/" 命令面板（useSlashMenu） ─────────────────
// 取代旧版内联 tool palette：现在支持功能 + 技能两类。
// 静态功能项（attach / plan-mode / permission-mode）由 composable 内部定义。
// 技能项从后端 /api/skills 拉取，mount 时拉一次缓存。
// apply 回调在这里桥接："添加附件" → triggerAttach 打开 file picker；
// "Plan 模式" / "权限模式" → 留作未来扩展，目前仅 toast 提示。
// 技能选中 → 在输入框中插入 "@<skill-name> " 让用户继续编辑。
const slashMenu = useSlashMenu({
  onAttach: () => {
    // 复用 Task 12 的 + 按钮逻辑
    triggerAttach()
  },
  onTogglePlanMode: () => {
    showToast({ message: 'Plan 模式：开发中', duration: 1600, color: 'medium' })
  },
  onTogglePermissionMode: () => {
    showToast({ message: '权限模式：开发中', duration: 1600, color: 'medium' })
  },
  onSelectSkill: (id, label) => {
    // 选中技能 → 在输入框中插入 "@<label> "，等用户继续编辑
    void id // 技能 id 当前仅用于日志/未来埋点；label 用于填充输入
    inputText.value = `@${label} `
    autoResize()
    nextTick(() => inputRef.value?.focus())
  },
})

/**
 * textarea @input 入口：先走原生 autoResize 维持高度，
 * 再把当前文本传给 slashMenu.handleInput 决定开关。
 */
function onTextareaInput() {
  autoResize()
  slashMenu.handleInput(inputText.value)
}

/**
 * textarea @keydown 入口：先让 slashMenu 拦截 ↑
 * ↓ / Enter / Escape（菜单打开时）；未拦截时放行原生行为。
 */
function onTextareaKeydown(e: KeyboardEvent) {
  // slashMenu.handleKeydown 内部决定是否拦截
  if (slashMenu.handleKeydown(e)) return
  // 菜单未打开时：菜单不处理，留给浏览器默认（如 Tab、Backspace 等）
}

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
    // 关键：必须传 deviceId 给后端！
    // 后端 handleAgentModels 用 deviceId 派生 AES 解密 key 读取 API Key，
    // 不传 deviceId 会用错的 key 派生 → 永远解不出设备绑定的密文 → 503
    const did = getDeviceIdSync()
    const res = await fetch(`${AGENT_API_BASE}/api/models?deviceId=${encodeURIComponent(did)}`)
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const data = await res.json()
    // 处理各种错误状态
    if (data.error === 'no_api_key') {
      modelsError.value = t('agent.noApiKeyHint') || '未配置 API Key'
      return
    }
    if (data.error || !Array.isArray(data.models)) {
      modelsError.value = data.note || t('agent.modelsError')
      return
    }
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
    // 网络错误等：不阻断用户使用，显示提示但保留已存储的模型选择
    modelsError.value = t('agent.modelsError')
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
  const atts = attachments.value.slice() // 拍快照：避免 send 异步期间被清空后引用空数组
  inputText.value = ''
  autoResize()
  send(text, { attachments: atts })
  // 发送后清空 tray（避免下次发送重复附带）
  clearAttachments()
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

  // Task 12：content 可能是 multimodal 数组。从中抽出 text 元素作为
  // 重发文本（附件不再重复附带——本地状态已丢失原 attachment 引用）。
  let text = ''
  if (typeof targetMsg.content === 'string') {
    text = targetMsg.content
  } else {
    for (const part of targetMsg.content) {
      if (part.type === 'text') {
        text += part.text
      }
    }
    text = text.trim()
  }

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
    // 长会话走虚拟列表的 scrollToItem
    if (renderedItems.value.length > VIRTUAL_LIST_THRESHOLD && virtualListRef.value) {
      virtualListRef.value.scrollToBottom(behavior)
      return
    }
    // 短会话走原生 container 滚动
    const el = mainRef.value
    if (!el) return
    el.scrollTo({ top: el.scrollHeight, behavior })
  })
}

/**
 * 监听 main 容器滚动，更新 nearBottom
 * 虚拟列表模式下滚动源是 RecycleScroller 内部 wrapper，
 * 但其 scroll 事件会冒泡到 main 容器，逻辑统一处理
 */
function onMainScroll() {
  const el = mainRef.value
  if (!el) return
  const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
  // 80px 阈值内视为"接近底部"——避免长消息末尾抖动
  nearBottom.value = distanceFromBottom < 80
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

/* Task 12：附件 `+` 按钮（与发送按钮同尺寸，无背景色） */
.footerAttachBtn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--ion-color-primary);
  cursor: pointer;
  font-size: 18px;
  flex-shrink: 0;
  padding: 0;
  transition: background 0.12s;
}

.footerAttachBtn:hover,
.footerAttachBtn:active {
  background: rgba(var(--ion-color-primary-rgb), 0.12);
}

/* 隐藏原生 file input —— 用按钮触发 */
.footerAttachInput {
  display: none;
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

/* ── LAN Access 折叠面板（Task 26） ─────────────────── */
.lanAccessPanel {
  background: var(--ion-toolbar-background, var(--ion-background-color));
  border-bottom: 1px solid rgba(var(--ion-color-medium-rgb), 0.12);
  flex-shrink: 0;
  font-size: 13px;
}

.lanAccessPanel[open] {
  padding-bottom: 8px;
}

.lanAccessSummary {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  list-style: none;
  outline: none;
}

.lanAccessSummary::-webkit-details-marker {
  display: none;
}

.lanAccessSummary::marker {
  content: '';
}

.lanAccessSummaryIcon {
  font-size: 16px;
  color: var(--ion-color-primary);
  flex-shrink: 0;
}

.lanAccessSummaryText {
  flex: 1;
  font-weight: 500;
  color: var(--ion-text-color);
}

.lanAccessSummaryCount {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border-radius: 10px;
  background: rgba(var(--ion-color-primary-rgb), 0.18);
  color: var(--ion-color-primary);
  font-size: 11px;
  font-weight: 600;
  line-height: 1;
}

.lanAccessBody {
  padding: 0 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.lanAccessHelp {
  margin: 0 0 2px;
  font-size: 11px;
  color: var(--ion-text-color-step-400, #888);
}

.lanAccessEmpty,
.lanAccessLoading {
  font-size: 12px;
  color: var(--ion-text-color-step-400, #888);
  padding: 4px 0;
}

.lanAccessList {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.lanAccessItem {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  background: rgba(var(--ion-color-medium-rgb), 0.06);
  border-radius: 8px;
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.12);
}

.lanAccessItemMain {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.lanAccessUrl {
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
  font-size: 12px;
  color: var(--ion-color-primary);
  word-break: break-all;
  line-height: 1.35;
}

.lanAccessInterface {
  font-size: 10px;
  color: var(--ion-text-color-step-400, #888);
}

.lanAccessCopyBtn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--ion-color-primary);
  cursor: pointer;
  font-size: 16px;
  padding: 0;
  flex-shrink: 0;
  transition: background 0.12s;
}

.lanAccessCopyBtn:hover,
.lanAccessCopyBtn:active {
  background: rgba(var(--ion-color-primary-rgb), 0.14);
}

.lanAccessRefresh {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  margin-top: 4px;
  padding: 4px 10px;
  font-size: 11px;
  color: var(--ion-text-color-step-400, #888);
  background: transparent;
  border: 0;
  border-radius: 6px;
  cursor: pointer;
  align-self: flex-start;
}

.lanAccessRefresh:hover:not(:disabled),
.lanAccessRefresh:active:not(:disabled) {
  background: rgba(var(--ion-color-medium-rgb), 0.1);
  color: var(--ion-text-color);
}

.lanAccessRefresh:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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

/* ── Tool Palette ("/" 命令面板) ───────────────────────── */
.tool-palette {
  background: var(--ion-background-color);
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.25);
  border-radius: 10px 10px 0 0;
  margin: 0 12px;
  max-height: 220px;
  overflow-y: auto;
  box-shadow: 0 -2px 12px rgba(0, 0, 0, 0.08);
  z-index: 10;
}

.tool-palette-header {
  padding: 8px 12px 4px;
  border-bottom: 1px solid rgba(var(--ion-color-medium-rgb), 0.12);
}

.tool-palette-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--ion-text-color-step-400, #888);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.tool-palette-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 12px;
  cursor: pointer;
  transition: background 0.12s;
}

.tool-palette-item:hover,
.tool-palette-item:active,
.tool-palette-active {
  background: rgba(var(--ion-color-primary-rgb), 0.08);
}

.tool-palette-active {
  background: rgba(var(--ion-color-primary-rgb), 0.12);
}

.tool-palette-icon {
  font-size: 18px;
  color: var(--ion-color-primary);
  flex-shrink: 0;
}

.tool-palette-info {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.tool-palette-name {
  font-size: 13px;
  font-weight: 600;
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
  color: var(--ion-text-color);
}

.tool-palette-desc {
  font-size: 11px;
  color: var(--ion-text-color-step-400, #888);
}

.tool-palette-empty {
  padding: 16px 12px;
  text-align: center;
  font-size: 12px;
  color: var(--ion-text-color-step-350, #999);
}

.footerInputRow-palette {
  border-radius: 0 0 12px 12px;
  border-top-left-radius: 0;
  border-top-right-radius: 0;
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
