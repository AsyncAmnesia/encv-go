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
        <!--
          Mock 模式切换器（用户在会话界面直接配置，无需去 Settings → Agent）。
          三种状态：off / builtin / custom。
          行为：
            - 始终显示（让用户随时知道当前是真实 API 还是 mock）
            - 点击 → 弹 action-sheet 切换模式
            - mode=off 灰色，builtin/custom 强调色 + 文字"模拟"
          currentMockMode 由 useAgent 从后端 /api/config 加载，切换时
          通过 PUT /api/config 持久化（无需重启后端）。
        -->
        <button
          type="button"
          class="mockBadge"
          :class="{
            mockBadge_active: currentMockMode !== 'off',
            mockBadge_clickable: true,
          }"
          :title="mockBadgeTitle"
          @click="openMockModeSheet"
        >
          <ion-icon :icon="flaskIcon" class="mockBadgeIcon" />
          <span class="mockBadgeText">{{ mockBadgeText }}</span>
          <ion-icon :icon="chevronDownIcon" class="mockBadgeChevron" />
        </button>
      </div>
      <!-- 上下文使用图标（点击 → 弹窗：todos + 引用文件） -->
      <ContextIcon
        :data="contextUsage.data.value"
        :loading="contextUsage.loading.value"
        class="headerContext"
      />
      <button type="button" class="headerBtn" @click="handleNewSession" :title="t('agent.newSession')">
        <ion-icon :icon="addIcon" />
      </button>
    </header>

    <!--
      no_api_key 自愈 banner：仅在 chat 发送返回 503 {error: "no_api_key"} 时出现。
      原因：用户可能在另一台设备配过 key，本设备的 deviceId 解不开。
      给一个直达 AI 设置的入口，避免"我不知道去哪里修"的卡死循环。
    -->
    <div v-if="lastErrorCode === 'no_api_key'" class="noApiKeyBanner">
      <ion-icon :icon="keyIcon" class="noApiKeyBannerIcon" />
      <div class="noApiKeyBannerText">
        <strong>{{ t('agent.noApiKeyTitle') || '未配置 API Key' }}</strong>
        <span>{{ t('agent.noApiKeyHint2') || '当前设备无法解密已存储的 key，请去 AI 设置重新输入。' }}</span>
      </div>
      <button type="button" class="noApiKeyBannerBtn" @click="goToApiKeySettings">
        {{ t('agent.goToApiKeySettings') || '去设置' }}
      </button>
      <button type="button" class="noApiKeyBannerClose" @click="dismissError" :title="t('common.close') || '关闭'">
        <ion-icon :icon="closeIcon" />
      </button>
    </div>

    <!-- 模型选择已移至输入框内（footerInputRow 左侧） -->

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

    <!-- 消息区域：圆点导航（左）+ 滚动内容（右） -->
    <div class="agentChatBody">
      <!-- 左侧圆点导航（≥3 条消息时显示）—— 在滚动容器外部，不随内容滚走 -->
      <div
        v-if="renderedItems.length >= 3"
        class="dotNavigation"
      >
        <button
          v-for="(item, idx) in renderedItems"
          :key="item.messageId"
          type="button"
          class="dotNavDot"
          :class="{ dotNavDot_active: activeMessageIndex === idx }"
          :title="`跳转到第 ${idx + 1} 条消息`"
          @click="scrollToMessage(idx)"
        />
      </div>

      <main class="agentChatMain" ref="mainRef" @scroll="onMainScroll">
      <!-- 空状态（无消息时显示） -->
      <div v-if="renderedItems.length === 0" class="agentChatEmpty">
        <ion-icon :icon="chatbubblesIcon" class="emptyIcon" />
        <p>{{ t('agent.emptyHint') }}</p>
      </div>
      <!-- 短会话（≤ 120）：原生 v-for（无虚拟化开销） -->
      <template v-else-if="renderedItems.length <= VIRTUAL_LIST_THRESHOLD">
        <div
          v-for="(item, idx) in renderedItems"
          :key="item.messageId"
          class="renderedItemWrap"
          :data-msg-idx="idx"
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
    </div><!-- /.agentChatBody -->

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
        <!-- 模型选择器（输入框内嵌，参考 ChatGPT/Claude 主流设计） -->
        <div class="modelPicker" ref="modelPickerRef">
          <button
            type="button"
            class="modelPickerBtn"
            :disabled="status === 'streaming' || modelsLoading"
            @click="modelPickerOpen = !modelPickerOpen"
            :title="t('agent.model')"
          >
            <span class="modelPickerLabel">{{ currentModelDisplayName }}</span>
            <ion-icon :icon="chevronDownIcon" class="modelPickerArrow" :class="{ 'modelPickerArrow_open': modelPickerOpen }" />
          </button>
          <Transition name="modelPickerFade">
            <div v-if="modelPickerOpen" class="modelPickerDropdown">
              <div v-if="modelsLoading" class="modelPickerLoading">{{ t('agent.loadingModels') }}...</div>
              <template v-else-if="modelsError">
                <div class="modelPickerError">{{ modelsError }}</div>
                <button
                  v-if="selectedModel && !availableModels.some(m => m.id === selectedModel)"
                  type="button"
                  class="modelPickerOption modelPickerOption_active"
                  @click="selectModel(selectedModel); modelPickerOpen = false"
                >{{ selectedModel }}</button>
              </template>
              <template v-else>
                <button
                  v-for="m in availableModels"
                  :key="m.id"
                  type="button"
                  class="modelPickerOption"
                  :class="{ 'modelPickerOption_active': selectedModel === m.id }"
                  @click="selectModel(m.id); modelPickerOpen = false"
                >
                  <span class="modelPickerOptionName">{{ m.name }}</span>
                  <span v-if="m.provider !== 'unknown'" class="modelPickerOptionProvider">{{ m.provider }}</span>
                </button>
              </template>
            </div>
          </Transition>
        </div>

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
                {{ formatSessionMeta(s) }}
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
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { useRouter } from 'vue-router'
import { IonIcon, modalController, alertController, actionSheetController } from '@ionic/vue'
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
  keyOutline,
  chevronDownOutline,
  flaskOutline,
} from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'
import { getDeviceIdSync } from '@/composables/useDeviceId'
import { getAgentApiBase } from '@/composables/useAgentApiBase'
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
import ContextIcon from '@/components/agent/ContextIcon.vue'

const { t } = useI18n()

// Agent API 基础路径（与 useAgent.ts 保持一致）
// Agent API 基础 URL（动态解析：dev 走网关 / prod 直连后端）
const AGENT_API_BASE = getAgentApiBase()

const { messages, status, send, confirmTool, resume, stop, newSession, switchSession, deleteSession, sessions, currentSessionId, contextUsage, lastErrorCode, dismissError, activeModel, setApiDefaultModel, isMockMode, mockScenario, currentMockMode, loadMockMode, setMockMode } = useAgent()
const router = useRouter()

/**
 * 跳转到 AI 设置页面（让用户重新输入 API Key）。
 *
 * 触发场景：no_api_key banner 出现时（后端 readAgentConfig(deviceId)
 * 返回空，说明当前 deviceId 派生不出 AES key 解开存储密文）。
 *
 * 行为：
 *   1. 先 dismiss banner（避免下次进来还显示）
 *   2. 关闭当前 AgentChat modal（modalController.dismiss）
 *   3. 用 vue-router 跳到 /tabs/settings/agent
 *
 * 为什么不直接 router.push：AgentChat 是 modal，路由跳转不会自动关 modal，
 * 用户回到 home 还会看到飘着的对话窗口。必须先 dismiss。
 */
async function goToApiKeySettings(): Promise<void> {
  dismissError()
  try {
    await modalController.dismiss()
  } catch {/* ignore — 可能 modal 已经被关 */}
  router.push('/tabs/settings/agent')
}

onMounted(() => {
  // 启动 Context 图标的轮询（5s/30s 周期自适应当前 streaming 状态）
  contextUsage.start()
})
onUnmounted(() => {
  // 卸载时清理 timer，避免内存泄漏
  contextUsage.stop()
})

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
const activeMessageIndex = ref(0)

/** 触发虚拟滚动的阈值（renderedItems 数量 > 此值时切换） */
const VIRTUAL_LIST_THRESHOLD = 120

const closeIcon = closeOutline
const sparkleIcon = sparklesOutline
const addIcon = addOutline
const sendIcon = sendOutline
const stopIcon = stopOutline
const keyIcon = keyOutline
const chatbubblesIcon = chatbubblesOutline
const timeIcon = timeOutline
const attachIcon = attachOutline
const globeIcon = globeOutline
const clipboardIcon = clipboardOutline
const refreshCircleIcon = refreshCircleOutline
const chevronDownIcon = chevronDownOutline
const flaskIcon = flaskOutline
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
  const did = getDeviceIdSync()
  let url = `${AGENT_API_BASE}/api/models?deviceId=${encodeURIComponent(did)}`
  try {
    const res = await fetch(url)
    if (!res.ok) throw new Error(`HTTP ${res.status} ${res.statusText}`)
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
    // 保存 API 返回的默认模型（新会话时使用）
    if (data.defaultModel) {
      setApiDefaultModel(data.defaultModel)
    }
    // 如果当前选中的模型不在列表中，切换到默认值
    if (availableModels.value.length > 0 && !availableModels.value.some(m => m.id === selectedModel.value)) {
      selectedModel.value = data.defaultModel || availableModels.value[0].id
    }
  } catch (e: any) {
    const errInfo = (() => {
      if (!e) return '(null)'
      if (e instanceof Error) return `${e.name}: ${e.message}`
      try { return JSON.stringify(e) } catch { return String(e) }
    })()
    console.error(`[AgentChat] fetchModels failed: url=${url} error=${errInfo}`)
    // 网络错误等：不阻断用户使用，显示提示但保留已存储的模型选择
    modelsError.value = `${t('agent.modelsError')} (${errInfo})`
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
  // 同步到 useAgent 的 activeModel（send/sendQueued 读取此值）
  activeModel.value = v
})
watch(temperature, (v) => {
  try { localStorage.setItem(TEMPERATURE_KEY, String(v)) } catch { /* ignore */ }
})

// ─── 模型选择器（输入框内嵌） ─────────────────────────────
const modelPickerOpen = ref(false)
const modelPickerRef = ref<HTMLElement | null>(null)

/** 当前模型的显示名称（从 availableModels 查找，找不到则用 id 本身） */
const currentModelDisplayName = computed(() => {
  const id = selectedModel.value
  const found = availableModels.value.find(m => m.id === id)
  return found?.name || id
})

function selectModel(id: string) {
  selectedModel.value = id
}

// ─── Mock 模式切换器（在会话界面直接配置，弹 action-sheet） ────────
/**
 * 徽章文本：根据当前模式显示对应文案
 *  - off     → "真实 API"   （灰色，提示"未启用 mock"）
 *  - builtin → "模拟·内置"
 *  - custom  → "模拟·自定义"
 */
const mockBadgeText = computed(() => {
  if (currentMockMode.value === 'builtin') return `${t('agent.mockBadge')}·${t('agent.mockModeBuiltin')}`
  if (currentMockMode.value === 'custom') return `${t('agent.mockBadge')}·${t('agent.mockModeCustom')}`
  return t('agent.mockModeOff')
})

/**
 * 徽章 tooltip：
 *  - active 时显示当前 scenario id（来自最近一次 SSE 响应）
 *  - off 时显示"点击切换模式"
 */
const mockBadgeTitle = computed(() => {
  if (currentMockMode.value === 'off') return t('agent.mockMode')
  if (isMockMode.value && mockScenario.value) {
    return t('agent.mockBadgeTooltip', { scenario: mockScenario.value })
  }
  return t('agent.mockMode')
})

/**
 * 点击徽章 → 弹 action-sheet 让用户选 off/builtin/custom
 * 切换经由 useAgent.setMockMode() 走 PUT /api/config 持久化
 */
async function openMockModeSheet(): Promise<void> {
  const current = currentMockMode.value
  const sheet = await actionSheetController.create({
    header: t('agent.mockMode'),
    buttons: [
      {
        text: t('agent.mockModeOff'),
        handler: () => { void switchMockMode('off') },
      },
      {
        text: t('agent.mockModeBuiltin'),
        handler: () => { void switchMockMode('builtin') },
      },
      {
        text: t('agent.mockModeCustom'),
        handler: () => { void switchMockMode('custom') },
      },
      {
        text: t('common.cancel'),
        role: 'cancel',
      },
    ],
  })
  await sheet.present()
  // 当前模式额外加 checked 标记（视觉反馈）—— 需要在 sheet 创建后 patch
  // （Ionic 8 actionSheet 暂不支持 per-button checked，这里仅控制文本可见性）
  void current
}

async function switchMockMode(mode: 'off' | 'builtin' | 'custom'): Promise<void> {
  try {
    await setMockMode(mode)
    showToast({
      message: t('agent.mockModeSet', { mode: t(`agent.mockMode${mode.charAt(0).toUpperCase()}${mode.slice(1)}`) }) || `Mock mode: ${mode}`,
      duration: 1600,
      color: mode === 'off' ? 'medium' : 'success',
    })
  } catch (e) {
    showToast({
      message: `${t('agent.mockModeSetFailed') || '切换失败'}: ${e instanceof Error ? e.message : String(e)}`,
      duration: 2400,
      color: 'danger',
    })
  }
}

/** 点击外部关闭下拉 */
function handleModelPickerOutsideClick(e: MouseEvent) {
  if (modelPickerOpen.value && modelPickerRef.value && !modelPickerRef.value.contains(e.target as Node)) {
    modelPickerOpen.value = false
  }
}

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

/**
 * 格式化会话历史列表项的元信息（时间 + 消息数 + 轮次）
 */
function formatSessionMeta(s: { messageCount: number; rounds: number; updatedAt: number }): string {
  const time = formatRelativeTime(s.updatedAt)
  const parts = [time]
  if (s.rounds > 0) {
    parts.push(`${s.rounds} ${t('agent.rounds') || '轮'}`)
  }
  parts.push(`${s.messageCount} ${t('agent.messages')}`)
  return parts.join(' · ')
}

/**
 * 简易相对时间格式化
 */
function formatRelativeTime(ts: number): string {
  if (!ts) return ''
  const diff = Date.now() - ts
  const abs = Math.abs(diff)
  const d = new Date(ts)
  if (abs < 60_000) return t('agent.justNow') || '刚刚'
  if (abs < 3600_000) return `${Math.floor(abs / 60_000)}${t('agent.minutesAgo') || '分钟前'}`
  if (abs < 86400_000) return `${Math.floor(abs / 3600_000)}${t('agent.hoursAgo') || '小时前'}`
  if (abs < 604_800_000) return `${Math.floor(abs / 86400_000)}${t('agent.daysAgo') || '天前'}`
  // 超过一周显示日期
  return `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
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

/** IntersectionObserver：追踪当前视口中最接近中心的消息项 */
let dotObserver: IntersectionObserver | null = null

function setupDotObserver() {
  cleanupDotObserver()
  const el = mainRef.value
  if (!el) return
  dotObserver = new IntersectionObserver(
    (entries) => {
      // 找到相交比例最大的元素（最接近视口中心）
      let maxRatio = 0
      let targetIdx = activeMessageIndex.value
      for (const entry of entries) {
        if (entry.intersectionRatio > maxRatio) {
          maxRatio = entry.intersectionRatio
          const idx = Number((entry.target as HTMLElement).dataset.msgIdx ?? -1)
          if (idx >= 0) targetIdx = idx
        }
      }
      if (maxRatio > 0) activeMessageIndex.value = targetIdx
    },
    { root: el, threshold: [0, 0.25, 0.5, 0.75, 1] },
  )
  // 观察所有消息项
  nextTick(() => {
    el.querySelectorAll('.renderedItemWrap').forEach((wrap) => {
      dotObserver?.observe(wrap)
    })
  })
}

function cleanupDotObserver() {
  dotObserver?.disconnect()
  dotObserver = null
}

// 消息列表变化时重建 Observer
watch(renderedItems, () => nextTick(setupDotObserver), { flush: 'post' })
onMounted(() => nextTick(setupDotObserver))
onUnmounted(cleanupDotObserver)

/**
 * 点导航：跳转到指定索引的消息项
 */
function scrollToMessage(idx: number) {
  const el = mainRef.value
  if (!el) return
  const itemWraps = el.querySelectorAll('.renderedItemWrap')
  if (idx < 0 || idx >= itemWraps.length) return
  const target = itemWraps[idx] as HTMLElement
  target.scrollIntoView({ behavior: 'smooth', block: 'start' })
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
  // 加载当前 mock 模式（用户主动控制 → action-sheet 切换）
  void loadMockMode()
  nextTick(() => scrollToBottom('auto'))
  // 模型选择器：点击外部关闭下拉
  document.addEventListener('click', handleModelPickerOutsideClick)
})

onUnmounted(() => {
  document.removeEventListener('click', handleModelPickerOutsideClick)
})

// 暴露给 modal container（可选）
defineExpose({})
</script>

<style scoped>
.agentChat {
  display: flex;
  flex-direction: column;
  height: 100vh;
  max-height: 100vh;
  width: 100vw;
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

/* ── Mock 模式切换器（始终可见、可点击、反映当前模式） ── */
.mockBadge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  border: 0;
  border-radius: 12px;
  background: rgba(var(--ion-color-medium-rgb), 0.12);
  color: var(--ion-color-medium);
  font-size: 11px;
  font-weight: 500;
  line-height: 1.4;
  user-select: none;
  cursor: pointer;
  font-family: inherit;
  transition: background 0.15s ease, color 0.15s ease, transform 0.1s ease;
}

.mockBadge:hover {
  background: rgba(var(--ion-color-medium-rgb), 0.22);
}

.mockBadge:active {
  transform: scale(0.96);
}

.mockBadge:focus-visible {
  outline: 2px solid var(--ion-color-primary);
  outline-offset: 1px;
}

/* 启用 mock（builtin / custom）时的强调色 */
.mockBadge_active {
  background: rgba(var(--ion-color-primary-rgb), 0.16);
  color: var(--ion-color-primary);
}

.mockBadge_active:hover {
  background: rgba(var(--ion-color-primary-rgb), 0.24);
}

.mockBadgeIcon {
  font-size: 12px;
  color: inherit;
}

.mockBadgeText {
  letter-spacing: 0.02em;
}

.mockBadgeChevron {
  font-size: 10px;
  color: inherit;
  opacity: 0.7;
}

.agentChatMain {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px 12px 12px 36px; /* 左侧留出圆点导航空间 (4px gap + ~24px nav + 8px margin) */
  display: flex;
  flex-direction: column;
  gap: 6px;
  -webkit-overflow-scrolling: touch;
  position: relative;
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

/* ── 模型选择器（输入框内嵌） ──────────────────────────── */
.modelPicker {
  position: relative;
  flex-shrink: 0;
}

.modelPickerBtn {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  height: 30px;
  padding: 0 8px;
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.25);
  border-radius: 8px;
  background: rgba(var(--ion-color-primary-rgb), 0.08);
  color: var(--ion-color-primary);
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  transition: all 0.15s ease;
}

.modelPickerBtn:hover:not(:disabled) {
  background: rgba(var(--ion-color-primary-rgb), 0.14);
  border-color: rgba(var(--ion-color-primary-rgb), 0.3);
}

.modelPickerBtn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.modelPickerLabel {
  max-width: 90px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.modelPickerArrow {
  font-size: 12px;
  transition: transform 0.2s ease;
  color: var(--ion-color-primary);
  opacity: 0.7;
}

.modelPickerArrow_open {
  transform: rotate(180deg);
}

.modelPickerDropdown {
  position: absolute;
  bottom: calc(100% + 6px);
  left: 0;
  min-width: 180px;
  max-width: 260px;
  max-height: 240px;
  overflow-y: auto;
  background: var(--ion-background-color);
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.2);
  border-radius: 10px;
  box-shadow: 0 -4px 20px rgba(0, 0, 0, 0.14);
  z-index: 50;
  padding: 4px;
}

.modelPickerLoading,
.modelPickerError {
  padding: 10px 12px;
  font-size: 12px;
  color: var(--ion-text-color-step-400, #888);
  text-align: center;
}

.modelPickerOption {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 8px 10px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--ion-text-color);
  cursor: pointer;
  font-size: 13px;
  text-align: left;
  transition: background 0.12s;
}

.modelPickerOption:hover {
  background: rgba(var(--ion-color-primary-rgb), 0.08);
}

.modelPickerOption_active {
  background: rgba(var(--ion-color-primary-rgb), 0.12);
  font-weight: 600;
  color: var(--ion-color-primary);
}

.modelPickerOptionName {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.modelPickerOptionProvider {
  font-size: 10px;
  color: var(--ion-text-color-step-400, #999);
  flex-shrink: 0;
  margin-left: 8px;
}

/* 下拉动画 */
.modelPickerFade-enter-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}
.modelPickerFade-leave-active {
  transition: opacity 0.1s ease, transform 0.1s ease;
}
.modelPickerFade-enter-from {
  opacity: 0;
  transform: translateY(4px);
}
.modelPickerFade-leave-to {
  opacity: 0;
  transform: translateY(4px);
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

/* ── no_api_key 自愈 banner ─────────────────────────────
   触发条件：chat 发送返回 503 {error: "no_api_key"}（设备解密不开存的密文）。
   设计要点：
   - 高对比红色（与 chat 顶部 toolbar 区分，避免被误以为是普通状态条）
   - icon + 文案 + 主操作按钮 + 关闭按钮四件套，缺一不可
   - 文案给两个句号：短句放强解释，长句放行动指引
*/
.noApiKeyBanner {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  background: linear-gradient(
    90deg,
    rgba(var(--ion-color-danger-rgb), 0.16),
    rgba(var(--ion-color-danger-rgb), 0.08)
  );
  border-bottom: 1px solid rgba(var(--ion-color-danger-rgb), 0.4);
  color: var(--ion-color-danger-shade);
  font-size: 13px;
  flex-shrink: 0;
}
.noApiKeyBannerIcon {
  font-size: 18px;
  flex-shrink: 0;
}
.noApiKeyBannerText {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  line-height: 1.35;
}
.noApiKeyBannerText strong {
  font-size: 13px;
  font-weight: 600;
}
.noApiKeyBannerText span {
  font-size: 12px;
  opacity: 0.85;
}
.noApiKeyBannerBtn {
  background: var(--ion-color-danger);
  color: var(--ion-color-danger-contrast, #fff);
  border: none;
  border-radius: 6px;
  padding: 5px 12px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  flex-shrink: 0;
}
.noApiKeyBannerBtn:hover {
  opacity: 0.9;
}
.noApiKeyBannerClose {
  background: transparent;
  border: none;
  color: var(--ion-color-danger-shade);
  cursor: pointer;
  padding: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  flex-shrink: 0;
}
.noApiKeyBannerClose ion-icon {
  font-size: 18px;
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

/* ── Dot Navigation（左侧圆点导航） ──────────────────────── */
/* 位于 .agentChatBody(flex row) 内，是 main(scroll) 的兄弟元素 */
.agentChatBody {
  display: flex;
  flex: 1;
  min-height: 0;
  position: relative; /* 为绝对定位的圆点导航提供定位上下文 */
}

/* 圆点导航：absolute 定位在 body 左侧，不受滚动影响 */
.dotNavigation {
  position: absolute;
  left: 2px;
  top: 8px;
  bottom: 8px;
  display: flex;
  flex-direction: column;
  gap: 5px;
  z-index: 20;
  padding: 8px 5px;
  background: rgba(var(--ion-background-color-rgb, 255, 255, 255), 0.9);
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.18);
  border-radius: 10px;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 2px 14px rgba(0, 0, 0, 0.12);
  overflow-y: auto;
  align-self: flex-start;
}

.dotNavDot {
  display: block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  border: none;
  background: rgba(var(--ion-color-medium-rgb), 0.45);
  cursor: pointer;
  padding: 0;
  transition: all 0.18s ease;
  outline: none;
}

.dotNavDot:hover {
  background: rgba(var(--ion-color-primary-rgb), 0.55);
  transform: scale(1.25);
}

.dotNavDot_active {
  background: var(--ion-color-primary);
  box-shadow: 0 0 0 2px rgba(var(--ion-color-primary-rgb), 0.28);
  transform: scale(1.35);
}
</style>
