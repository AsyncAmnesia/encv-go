<!--
  AgentDebugPanel - Mock 模式自动开启的调试面板
  显示 SSE 事件解析后的实际状态：
   - messages 数量、role 分布、tool_calls/tool_results 数量
   - renderedItems 类型分布（关键：看 GroupedOperationMessage 到底有没有实例化）
   - 最近 N 条 message 的 tool_calls + tool_results 详情（看配对是否成功）
   - 是否有"伪造"（result 字符串与真实 list_files 路径是否匹配）

  触发条件（AgentChat 父组件控制）：
   - 默认：isMockMode 时自动显示
   - 任意模式：URL 加 ?debug=agent 时强制显示
  用 <details> 折叠默认收起，避免遮挡正常对话。
-->
<template>
  <details class="agentDebugPanel" :open="defaultOpen">
    <summary class="agentDebugSummary">
      <ion-icon :icon="bugIcon" class="agentDebugSummaryIcon" />
      <span>Agent 调试面板</span>
      <span class="agentDebugBadge">{{ messages.length }} msg · {{ renderedItems.length }} rendered</span>
    </summary>
    <div class="agentDebugBody">
      <!-- ① messages 分布 -->
      <section class="agentDebugSection">
        <h4>① messages ({{ messages.length }})</h4>
        <div class="agentDebugStats">
          <span v-for="(c, role) in roleCounts" :key="role" class="agentDebugChip">
            {{ role }}: {{ c }}
          </span>
        </div>
        <div class="agentDebugStats">
          <span class="agentDebugChip">tool_calls: {{ totalToolCalls }}</span>
          <span class="agentDebugChip">tool_results: {{ totalToolResults }}</span>
          <span class="agentDebugChip">pairing: {{ pairRateText }}</span>
        </div>
      </section>

      <!-- ② renderedItems 类型分布 -->
      <section class="agentDebugSection">
        <h4>② renderedItems ({{ renderedItems.length }})</h4>
        <div class="agentDebugStats">
          <span
            v-for="(c, t) in renderedTypeCounts"
            :key="t"
            class="agentDebugChip"
            :class="{ agentDebugChip_emphasis: t === 'operationGroup' && c > 0 }"
          >
            {{ t }}: {{ c }}
          </span>
        </div>
      </section>

      <!-- ③ 最近 N 条 message 的 tool_calls + tool_results 详情 -->
      <section class="agentDebugSection">
        <h4>③ 最近 {{ recentMessages.length }} 条 message 的 tool_calls ↔ tool_results</h4>
        <div v-for="(m, i) in recentMessages" :key="i" class="agentDebugMsg">
          <div class="agentDebugMsgHead">
            <span class="agentDebugChip">{{ m.role }}</span>
            <span class="agentDebugMsgId">#{{ i }}</span>
            <span class="agentDebugChip">tool_calls: {{ m.tool_calls.length }}</span>
            <span class="agentDebugChip">tool_results: {{ m.tool_results.length }}</span>
          </div>
          <ul v-if="m.tool_calls.length > 0" class="agentDebugList">
            <li v-for="tc in m.tool_calls" :key="tc.id" class="agentDebugListItem">
              <div class="agentDebugListHead">
                <span class="agentDebugName">{{ tc.name }}</span>
                <span class="agentDebugId">{{ tc.id }}</span>
                <span class="agentDebugChip" :class="`agentDebugStatus_${tc.status}`">{{ tc.status }}</span>
                <span class="agentDebugChip">kind: {{ tc.kind }}</span>
              </div>
              <div v-if="findResult(m, tc.id)" class="agentDebugResult">
                <span class="agentDebugResultTag">↳ result</span>
                <code class="agentDebugResultJson">{{ truncate(findResult(m, tc.id), 200) }}</code>
              </div>
              <div v-else class="agentDebugResult agentDebugResult_missing">
                <span class="agentDebugResultTag">↳ 缺 result</span>
                <span class="agentDebugHint">{{ resultStatusHint(tc.status) }}</span>
              </div>
            </li>
          </ul>
        </div>
      </section>

      <!-- ④ operationGroup 实际渲染预览（关键：看 tool_result 卡片是不是真的渲染） -->
      <section v-if="operationGroups.length > 0" class="agentDebugSection">
        <h4>④ operationGroup 渲染预览（{{ operationGroups.length }} 组）</h4>
        <div v-for="(g, gi) in operationGroups" :key="gi" class="agentDebugGroup">
          <div class="agentDebugGroupHead">
            <span>{{ g.type }}</span>
            <span class="agentDebugChip">toolCallIds: {{ g.toolCallIds.length }}</span>
          </div>
          <div class="agentDebugGroupHint">
            👉 看下面正式 chat 流里的 GroupedOperationMessage 是否真的展开并显示了 MountListCard/FileListCard/FileContentCard
          </div>
        </div>
      </section>

      <!-- ⑤ 自我诊断 -->
      <section class="agentDebugSection">
        <h4>⑤ 自我诊断</h4>
        <ul class="agentDebugDiag">
          <li v-for="(line, i) in diagnostics" :key="i" :class="`agentDebugDiag_${line.level}`">
            <span class="agentDebugDiagLevel">{{ line.level }}</span>
            {{ line.text }}
          </li>
        </ul>
      </section>
    </div>
  </details>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { IonIcon } from '@ionic/vue'
import { bugOutline } from 'ionicons/icons'
import type { Message, ToolCall } from '@/composables/useAgent'

type RenderedItemLike = { type: string; [k: string]: unknown }

const props = defineProps<{
  messages: Message[]
  /** 由 AgentChat 传入的 renderedItems（任意结构，只读 type 字段） */
  renderedItems: RenderedItemLike[]
  /** 默认是否展开（mock 模式自动展开便于诊断） */
  defaultOpen?: boolean
}>()

const bugIcon = bugOutline

const roleCounts = computed<Record<string, number>>(() => {
  const counts: Record<string, number> = {}
  for (const m of props.messages) {
    counts[m.role] = (counts[m.role] ?? 0) + 1
  }
  return counts
})

const totalToolCalls = computed(() =>
  props.messages.reduce((sum, m) => sum + m.tool_calls.length, 0),
)
const totalToolResults = computed(() =>
  props.messages.reduce((sum, m) => sum + m.tool_results.length, 0),
)

/** 配对率：tool_results 中能找到对应 tool_call id 的比例 */
const pairRateText = computed(() => {
  const calls = new Set<string>()
  for (const m of props.messages) for (const tc of m.tool_calls) calls.add(tc.id)
  if (calls.size === 0) return 'n/a'
  const results = new Set<string>()
  for (const m of props.messages) for (const r of m.tool_results) results.add(r.id)
  let paired = 0
  for (const id of calls) if (results.has(id)) paired++
  return `${paired}/${calls.size}`
})

const renderedTypeCounts = computed<Record<string, number>>(() => {
  const counts: Record<string, number> = {}
  for (const r of props.renderedItems) {
    counts[r.type] = (counts[r.type] ?? 0) + 1
  }
  return counts
})

const recentMessages = computed(() => {
  // 只看含 tool_calls 的最近 3 条 message（核心问题区）
  return props.messages.filter((m) => m.tool_calls.length > 0).slice(-3)
})

const operationGroups = computed(() => {
  // narrow：把 type=operationGroup 的元素断言为带 toolCallIds 字段
  return props.renderedItems
    .filter((r) => r.type === 'operationGroup')
    .map((r) => r as unknown as { type: 'operationGroup'; toolCallIds: string[] })
})

function findResult(msg: Message, toolCallId: string): string | null {
  const r = msg.tool_results.find((x) => x.id === toolCallId)
  return r ? r.result : null
}

function truncate(s: string | null, max: number): string {
  if (!s) return ''
  if (s.length <= max) return s
  return s.slice(0, max) + `… (+${s.length - max} chars)`
}

function resultStatusHint(status: ToolCall['status']): string {
  if (status === 'pending' || status === 'running') return '工具还在执行，正常'
  if (status === 'success') return '⚠️ 工具声明 success 但 tool_result 事件没到——后端数据丢失'
  if (status === 'failed') return '工具失败，等错误回传'
  if (status === 'cancelled') return '已取消'
  return '未知状态'
}

// ─── 自我诊断 ────────────────────────────────────────
const diagnostics = computed<{ level: 'ok' | 'warn' | 'error'; text: string }[]>(() => {
  const out: { level: 'ok' | 'warn' | 'error'; text: string }[] = []
  if (totalToolCalls.value > 0 && totalToolResults.value === 0) {
    out.push({
      level: 'error',
      text: `有 ${totalToolCalls.value} 个 tool_call 但 0 个 tool_result——剧本可能没推 tool_result 事件，或前端没收到`,
    })
  }
  if (totalToolResults.value > 0) {
    const fakeCheck = checkForFakeData()
    if (fakeCheck.length > 0) {
      out.push({
        level: 'warn',
        text: `检测到疑似硬编码假数据：${fakeCheck.join('; ')}`,
      })
    } else {
      out.push({ level: 'ok', text: 'tool_result 数据看起来是真实数据（非 {FAKE:true} / 已知硬编码文件名）' })
    }
  }
  if (operationGroups.value.length === 0 && totalToolCalls.value > 0) {
    out.push({
      level: 'error',
      text: '有 tool_call 但 renderedItems 里 0 个 operationGroup——renderTurnItems 没把它们聚成 group，看下面的纯 markdown 渲染就是它导致的',
    })
  }
  if (operationGroups.value.length > 0) {
    out.push({ level: 'ok', text: `renderedItems 含 ${operationGroups.value.length} 个 operationGroup` })
  }
  if (pairRateText.value !== 'n/a' && pairRateText.value !== '0/0') {
    const [p, t] = pairRateText.value.split('/').map(Number)
    if (p < t) {
      out.push({
        level: 'warn',
        text: `配对率 ${p}/${t}——部分 tool_call 没收到 result（可能是分组时漏了）`,
      })
    } else if (p === t && t > 0) {
      out.push({ level: 'ok', text: `所有 ${t} 个 tool_call 都配对到 tool_result` })
    }
  }
  return out
})

function checkForFakeData(): string[] {
  const suspicious: string[] = []
  for (const m of props.messages) {
    for (const r of m.tool_results) {
      if (r.result.includes('"FAKE":true') || r.result.includes('"FAKE": true')) {
        suspicious.push(`${r.id}: 含 FAKE:true 标记`)
      }
      if (r.result.includes('studio_video_')) {
        suspicious.push(`${r.id}: 含老剧本硬编码 studio_video_* 假文件名`)
      }
    }
  }
  return suspicious
}
</script>

<style scoped>
.agentDebugPanel {
  margin: 8px 12px 4px;
  border: 1px dashed rgba(var(--ion-color-warning-rgb), 0.5);
  border-radius: 8px;
  background: rgba(var(--ion-color-warning-rgb), 0.05);
  font-size: 11px;
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
}

.agentDebugSummary {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  cursor: pointer;
  user-select: none;
  list-style: none;
  outline: none;
  color: var(--ion-color-warning-shade, #b8761e);
}

.agentDebugSummary::-webkit-details-marker {
  display: none;
}

.agentDebugSummary::marker {
  content: '';
}

.agentDebugSummaryIcon {
  font-size: 14px;
}

.agentDebugBadge {
  margin-inline-start: auto;
  padding: 1px 6px;
  border-radius: 8px;
  background: rgba(var(--ion-color-warning-rgb), 0.18);
  color: var(--ion-color-warning-shade, #b8761e);
  font-size: 10px;
  font-weight: 600;
}

.agentDebugBody {
  padding: 6px 10px 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.agentDebugSection {
  border-top: 1px solid rgba(var(--ion-color-medium-rgb), 0.12);
  padding-top: 6px;
}

.agentDebugSection h4 {
  margin: 0 0 4px;
  font-size: 11px;
  font-weight: 700;
  color: var(--ion-text-color);
  text-transform: uppercase;
  letter-spacing: 0.4px;
}

.agentDebugStats {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 3px;
}

.agentDebugChip {
  display: inline-flex;
  align-items: center;
  padding: 1px 6px;
  border-radius: 6px;
  background: rgba(var(--ion-color-medium-rgb), 0.12);
  color: var(--ion-text-color);
  font-size: 10px;
}

.agentDebugChip_emphasis {
  background: rgba(var(--ion-color-primary-rgb), 0.18);
  color: var(--ion-color-primary);
  font-weight: 600;
}

.agentDebugMsg {
  margin-top: 4px;
  padding: 4px 6px;
  background: rgba(var(--ion-color-medium-rgb), 0.06);
  border-radius: 4px;
}

.agentDebugMsgHead {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}

.agentDebugMsgId {
  font-size: 10px;
  color: var(--encv-text-secondary, #888);
}

.agentDebugList {
  list-style: none;
  margin: 4px 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.agentDebugListItem {
  padding: 3px 6px;
  background: rgba(var(--ion-color-medium-rgb), 0.04);
  border-left: 2px solid var(--ion-color-primary);
  border-radius: 0 4px 4px 0;
  font-size: 10.5px;
}

.agentDebugListHead {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}

.agentDebugName {
  font-weight: 600;
  color: var(--ion-text-color);
}

.agentDebugId {
  font-size: 9.5px;
  color: var(--encv-text-secondary, #888);
}

.agentDebugStatus_pending,
.agentDebugStatus_running {
  background: rgba(var(--ion-color-primary-rgb), 0.18);
  color: var(--ion-color-primary);
}

.agentDebugStatus_success {
  background: rgba(var(--ion-color-success-rgb, 56, 161, 105), 0.18);
  color: var(--ion-color-success, #38a169);
}

.agentDebugStatus_failed,
.agentDebugStatus_cancelled {
  background: rgba(var(--ion-color-danger-rgb), 0.18);
  color: var(--ion-color-danger);
}

.agentDebugResult {
  margin-top: 3px;
  padding: 3px 6px;
  background: rgba(var(--ion-color-medium-rgb), 0.04);
  border-radius: 3px;
  font-size: 10px;
}

.agentDebugResult_missing {
  background: rgba(var(--ion-color-warning-rgb), 0.08);
}

.agentDebugResultTag {
  display: inline-block;
  margin-inline-end: 4px;
  color: var(--ion-color-primary);
  font-weight: 600;
}

.agentDebugResultJson {
  font-size: 9.5px;
  word-break: break-all;
  white-space: pre-wrap;
  color: var(--ion-text-color);
}

.agentDebugHint {
  font-size: 10px;
  color: var(--encv-text-secondary, #888);
}

.agentDebugGroup {
  margin-top: 4px;
  padding: 4px 6px;
  background: rgba(var(--ion-color-primary-rgb), 0.06);
  border-radius: 4px;
}

.agentDebugGroupHead {
  display: flex;
  align-items: center;
  gap: 4px;
  font-weight: 600;
}

.agentDebugGroupHint {
  margin-top: 3px;
  font-size: 10px;
  color: var(--encv-text-secondary, #888);
}

.agentDebugDiag {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.agentDebugDiag li {
  padding: 3px 6px;
  border-radius: 3px;
  font-size: 10.5px;
}

.agentDebugDiag_ok {
  background: rgba(var(--ion-color-success-rgb, 56, 161, 105), 0.1);
  color: var(--ion-color-success, #38a169);
}

.agentDebugDiag_warn {
  background: rgba(var(--ion-color-warning-rgb), 0.12);
  color: var(--ion-color-warning-shade, #b8761e);
}

.agentDebugDiag_error {
  background: rgba(var(--ion-color-danger-rgb), 0.12);
  color: var(--ion-color-danger);
}

.agentDebugDiagLevel {
  display: inline-block;
  margin-inline-end: 4px;
  padding: 0 4px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.1);
  font-size: 9px;
  font-weight: 700;
  text-transform: uppercase;
}
</style>
