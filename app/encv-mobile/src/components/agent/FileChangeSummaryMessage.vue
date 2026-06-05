<!--
  FileChangeSummaryMessage - 文件变更特化摘要
  摘要：「已编辑 N 个文件」+ 文件列表
  当 operationGroup 全是 fileChange 时由 GroupedOperationMessage 委托渲染
-->
<template>
  <div class="fileChangeSummary">
    <div class="fileChangeHeader" :class="{ fileChangeHeader_active: isActive }" @click="expanded = !expanded">
      <ion-icon :icon="icon" class="fileChangeIcon" />
      <span class="fileChangeSummaryText">{{ summary }}</span>
      <StatusBadge
        v-if="status"
        :label="status"
        :tone="statusTone"
      />
      <ion-icon :icon="expanded ? chevronUp : chevronDown" class="fileChangeChevron" />
    </div>
    <div v-if="expanded" class="fileChangeList">
      <div v-for="(p, idx) in paths" :key="`${idx}-${p}`" class="fileChangeItem" :title="p">
        <ion-icon :icon="documentOutline" class="fileChangeItemIcon" />
        <span class="fileChangeItemPath">{{ truncate(p) }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { IonIcon } from '@ionic/vue'
import { documentTextOutline, chevronUpOutline, chevronDownOutline } from 'ionicons/icons'
import StatusBadge from './StatusBadge.vue'
import { useI18n } from '@/composables/useI18n'
import type { ToolCall, ToolStatus } from '@/composables/useAgent'

const props = defineProps<{
  items: ToolCall[]
  forceComplete?: boolean
}>()

const { t } = useI18n()
const expanded = ref(false)

const icon = documentTextOutline
const chevronUp = chevronUpOutline
const chevronDown = chevronDownOutline

const paths = computed<string[]>(() => {
  const out: string[] = []
  for (const it of props.items) {
    try {
      const args = JSON.parse(it.args) as Record<string, unknown>
      if (Array.isArray(args.changedFiles)) {
        for (const f of args.changedFiles) {
          if (typeof f === 'string') out.push(f)
          else if (f && typeof f === 'object' && typeof (f as Record<string, unknown>).path === 'string') {
            out.push((f as Record<string, string>).path)
          }
        }
      } else if (typeof args.path === 'string') {
        out.push(args.path)
      }
    } catch {
      // ignore
    }
  }
  return out
})

const lastItem = computed<ToolCall | null>(() => props.items[props.items.length - 1] ?? null)

const summary = computed(() => t('agent.ops.files', { n: props.items.length }))

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

function truncate(p: string): string {
  if (p.length <= 60) return p
  return '…' + p.slice(p.length - 59)
}
</script>

<style scoped>
.fileChangeSummary {
  margin: 6px 0;
}

.fileChangeHeader {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  background: rgba(var(--ion-color-primary-rgb), 0.08);
  border-radius: 14px;
  border: 1px solid rgba(var(--ion-color-primary-rgb), 0.2);
  font-size: 12px;
  color: var(--ion-text-color);
  cursor: pointer;
  user-select: none;
  max-width: 100%;
  flex-wrap: wrap;
}

.fileChangeHeader_active {
  animation: fileChangeActivePulse 1.4s ease-in-out infinite;
}

.fileChangeIcon {
  font-size: 13px;
  color: var(--ion-color-primary);
  flex-shrink: 0;
}

.fileChangeSummaryText {
  font-weight: 500;
}

.fileChangeChevron {
  font-size: 12px;
  color: var(--encv-text-secondary);
  flex-shrink: 0;
}

.fileChangeList {
  display: flex;
  flex-direction: column;
  gap: 3px;
  margin-top: 6px;
  padding: 6px 8px;
  background: rgba(var(--ion-color-medium-rgb), 0.06);
  border-radius: 6px;
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.16);
}

.fileChangeItem {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11.5px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  color: var(--ion-text-color);
  min-width: 0;
}

.fileChangeItemIcon {
  font-size: 12px;
  color: var(--encv-text-secondary);
  flex-shrink: 0;
}

.fileChangeItemPath {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@keyframes fileChangeActivePulse {
  0%, 100% { background-color: rgba(var(--ion-color-primary-rgb), 0.08); }
  50% { background-color: rgba(var(--ion-color-primary-rgb), 0.18); }
}
</style>
