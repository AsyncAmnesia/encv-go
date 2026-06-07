<!--
  OperationCard - 单条工具调用卡片（agent 流式时间轴模式）

  与 GroupedOperationMessage 不同：
  - GroupedOperationMessage：聚合 N 个 tool_call 到一个折叠 group（旧模式）
  - OperationCard：渲染**单个** tool_call，紧跟其 toolResultCard（新模式）

  设计要点：
  - 紧凑的单行卡片：图标 + 名称 + args + status badge
  - 不折叠——agent 时间轴要求每步都可见
  - status 实时更新（pending → running → success/failed）
  - 内嵌 ToolResultCard 容器 slot（由 AgentChat 按 name 分发到具体卡片）
-->
<template>
  <div class="operationCard" :class="{ operationCard_streaming: streaming }">
    <div class="operationCardHead">
      <ion-icon :icon="toolIcon" class="operationCardIcon" />
      <span class="operationCardName">{{ toolCall.name || t('agent.tool.unknown') }}</span>
      <StatusBadge
        :label="toolCall.status"
        :tone="statusTone"
        :pulse="streaming || toolCall.status === 'running' || toolCall.status === 'pending'"
        class="operationCardBadge"
      />
    </div>
    <div v-if="toolCall.args" class="operationCardArgs">
      <code>{{ truncateArgs(toolCall.args) }}</code>
    </div>
    <!-- ToolResultCard 插槽：AgentChat 按 name 分发到 MountListCard / FileListCard / FileContentCard -->
    <div v-if="$slots.result" class="operationCardResult">
      <slot name="result" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { IonIcon } from '@ionic/vue'
import {
  terminalOutline,
  documentTextOutline,
  eyeOutline,
  searchOutline,
  ellipsisHorizontalCircleOutline,
} from 'ionicons/icons'
import StatusBadge from './StatusBadge.vue'
import { useI18n } from '@/composables/useI18n'
import type { ToolCall } from '@/composables/useAgent'

const props = defineProps<{
  toolCall: ToolCall
  /** 流式状态（running/pending 时显示脉冲动画） */
  streaming?: boolean
}>()

const { t } = useI18n()

/** 按 kind 选图标 */
const toolIcon = computed(() => {
  switch (props.toolCall.kind) {
    case 'command':
      return terminalOutline
    case 'fileChange':
      return documentTextOutline
    case 'readOnly':
      return eyeOutline
    case 'webSearch':
      return searchOutline
    default:
      return ellipsisHorizontalCircleOutline
  }
})

/** ToolStatus → StatusBadge tone 映射 */
const statusTone = computed<'ready' | 'warn' | 'idle'>(() => {
  switch (props.toolCall.status) {
    case 'success':
      return 'ready'
    case 'failed':
    case 'cancelled':
      return 'warn'
    default:
      // pending / running / 其他 → idle（pulse 动画提示进行中）
      return 'idle'
  }
})

function truncateArgs(args: string): string {
  if (!args || args.length <= 120) return args || ''
  return args.slice(0, 120) + '…'
}
</script>

<style scoped>
.operationCard {
  margin: 3px 0 5px;
  background: rgba(var(--ion-color-medium-rgb), 0.05);
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.16);
  border-radius: 7px;
  padding: 6px 10px;
  font-size: 11.5px;
}

.operationCard_streaming {
  border-color: rgba(var(--ion-color-primary-rgb), 0.35);
  animation: opPulse 2s ease-in-out infinite;
}

@keyframes opPulse {
  0%, 100% { border-color: rgba(var(--ion-color-primary-rgb), 0.25); }
  50% { border-color: rgba(var(--ion-color-primary-rgb), 0.55); }
}

.operationCardHead {
  display: flex;
  align-items: center;
  gap: 6px;
}

.operationCardIcon {
  font-size: 14px;
  color: var(--ion-color-primary);
  flex-shrink: 0;
}

.operationCardName {
  font-weight: 600;
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
  color: var(--ion-text-color);
  flex-shrink: 0;
}

.operationCardBadge {
  margin-inline-start: auto;
  flex-shrink: 0;
}

.operationCardArgs {
  margin-top: 3px;
  padding-left: 20px; /* 缩进与图标对齐 */
}

.operationCardArgs code {
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
  font-size: 10.5px;
  color: var(--encv-text-secondary, #888);
  word-break: break-all;
  white-space: pre-wrap;
}

.operationCardResult {
  margin-top: 4px;
}
</style>
