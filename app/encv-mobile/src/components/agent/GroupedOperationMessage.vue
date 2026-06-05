<!--
  GroupedOperationMessage - 累积操作摘要
  参照 codex_web GroupedOperationMessage{items, forceComplete}
  - 累积 command/fileChange/toolOutput 渲染为单一摘要
  - 摘要文本规则：
    * 全 command → "已运行 N 条命令，Xms"
    * 全 fileChange → 委托 FileChangeSummaryMessage
    * 混合 → "已执行 N 个操作（X 命令 + Y 文件变更）"
    * 全 toolOutput → "已执行 N 个工具"
  - active 跟随最末 item 的状态
-->
<template>
  <FileChangeSummaryMessage
    v-if="allFileChange && fileItems.length > 0"
    :items="fileItems"
    :force-complete="forceComplete"
  />
  <div v-else class="groupedOp">
    <div class="groupedOpHeader" :class="{ groupedOpHeader_active: isActive }">
      <ion-icon :icon="icon" class="groupedOpIcon" />
      <span class="groupedOpSummary">{{ summary }}</span>
      <StatusBadge
        v-if="status"
        :label="status"
        :tone="statusTone"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { IonIcon } from '@ionic/vue'
import { terminalOutline, ellipsisHorizontalCircleOutline } from 'ionicons/icons'
import StatusBadge from './StatusBadge.vue'
import FileChangeSummaryMessage from './FileChangeSummaryMessage.vue'
import { useI18n } from '@/composables/useI18n'
import type { ToolCall, ToolStatus } from '@/composables/useAgent'

const props = defineProps<{
  items: ToolCall[]
  forceComplete?: boolean
}>()

const { t } = useI18n()

const kinds = computed(() => props.items.map((it) => it.kind))
const allFileChange = computed(() => kinds.value.length > 0 && kinds.value.every((k) => k === 'fileChange'))
const allCommand = computed(() => kinds.value.length > 0 && kinds.value.every((k) => k === 'command'))

const fileItems = computed(() => props.items)

const lastItem = computed<ToolCall | null>(() => {
  return props.items.length > 0 ? props.items[props.items.length - 1] : null
})

const totalDuration = computed(() => {
  // 累加 args.durationMs（如有）作为粗略估计；无值则按 0 显示
  let total = 0
  for (const it of props.items) {
    try {
      const parsed = JSON.parse(it.args)
      const v = (parsed as Record<string, unknown>).durationMs
      if (typeof v === 'number') total += v
    } catch {
      // ignore
    }
  }
  return total
})

const summary = computed(() => {
  const n = props.items.length
  const cmd = props.items.filter((i) => i.kind === 'command').length
  const file = props.items.filter((i) => i.kind === 'fileChange').length
  if (allCommand.value) {
    return t('agent.ops.commands', { n: String(n), ms: String(totalDuration.value || 0) })
  }
  if (allFileChange.value) {
    return t('agent.ops.files', { n: String(n) })
  }
  if (cmd > 0 && file > 0) {
    return t('agent.ops.mixed', { n: String(n), cmd: String(cmd), file: String(file) })
  }
  return t('agent.ops.toolOutputs', { n: String(n) })
})

const icon = computed(() => {
  if (allCommand.value) return terminalOutline
  return ellipsisHorizontalCircleOutline
})

const status = computed(() => {
  const s = lastItem.value?.status
  if (!s) return ''
  if (s === 'success') return t('agent.completed')
  if (s === 'failed') return t('agent.failed')
  if (s === 'cancelled') return t('agent.cancelled')
  if (s === 'running') return t('agent.running')
  return ''
})

const statusTone = computed<'ready' | 'warn' | 'idle'>(() => {
  const s: ToolStatus | undefined = lastItem.value?.status
  if (s === 'success') return 'ready'
  if (s === 'failed' || s === 'cancelled') return 'warn'
  if (s === 'running' || s === 'pending') return 'idle'
  return 'idle'
})

const isActive = computed(() => lastItem.value?.status === 'running' || lastItem.value?.status === 'pending')
</script>

<style scoped>
.groupedOp {
  margin: 6px 0;
}

.groupedOpHeader {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  background: rgba(var(--ion-color-medium-rgb), 0.08);
  border-radius: 14px;
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.18);
  font-size: 12px;
  color: var(--ion-text-color);
  max-width: 100%;
  flex-wrap: wrap;
}

.groupedOpHeader_active {
  background: rgba(var(--ion-color-primary-rgb), 0.12);
  border-color: rgba(var(--ion-color-primary-rgb), 0.3);
  animation: groupedOpPulse 1.4s ease-in-out infinite;
}

.groupedOpIcon {
  font-size: 13px;
  color: var(--ion-color-primary);
  flex-shrink: 0;
}

.groupedOpSummary {
  font-weight: 500;
  word-break: break-word;
}

@keyframes groupedOpPulse {
  0%, 100% { background-color: rgba(var(--ion-color-primary-rgb), 0.12); }
  50% { background-color: rgba(var(--ion-color-primary-rgb), 0.22); }
}
</style>
