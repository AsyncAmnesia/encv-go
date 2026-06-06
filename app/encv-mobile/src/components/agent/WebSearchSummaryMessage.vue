<!--
  WebSearchSummaryMessage - 联网搜索摘要
  头部：MessageAuthor(searchOutline, "搜索", query count)
  折叠态：显示首条 query
  展开态：显示所有 query 列表
-->
<template>
  <div class="webSearchMessage">
    <div class="webSearchHeader" @click="expanded = !expanded">
      <MessageAuthor :icon="icon" :label="label" :meta="`${queries.length} 个查询`" />
      <ion-icon :icon="expanded ? chevronUp : chevronDown" class="webSearchChevron" />
    </div>
    <div v-if="expanded && queries.length > 0" class="webSearchList">
      <div v-for="(q, idx) in queries" :key="idx" class="webSearchItem">
        <ion-icon :icon="searchOutline" class="webSearchItemIcon" />
        <span>{{ q }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { searchOutline, chevronUpOutline, chevronDownOutline } from 'ionicons/icons'
import MessageAuthor from './MessageAuthor.vue'
import type { ToolCall } from '@/composables/useAgent'

defineProps<{
  queries: string[]
  toolCalls: ToolCall[]
}>()

const expanded = ref(false)
const icon = searchOutline
const chevronUp = chevronUpOutline
const chevronDown = chevronDownOutline
const label = '搜索'
</script>

<style scoped>
.webSearchMessage {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 6px 0;
  max-width: 100%;
}

.webSearchHeader {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  cursor: pointer;
  user-select: none;
}

.webSearchChevron {
  font-size: 14px;
  color: var(--encv-text-secondary);
}

.webSearchList {
  display: flex;
  flex-direction: column;
  gap: 3px;
  margin-top: 4px;
  padding: 6px 8px;
  background: rgba(var(--ion-color-medium-rgb), 0.06);
  border-radius: 6px;
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.16);
}

.webSearchItem {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  color: var(--ion-text-color);
}

.webSearchItemIcon {
  font-size: 12px;
  color: var(--encv-text-secondary);
}
</style>
