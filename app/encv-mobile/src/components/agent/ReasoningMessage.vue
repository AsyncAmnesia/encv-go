<!--
  ReasoningMessage - 推理过程（折叠展示）
  头部：MessageAuthor(bulbOutline, "推理", meta)
  折叠态：只显示 "推理" 文字
  展开态：显示完整 reasoning 文本（Markdown 渲染）
-->
<template>
  <div class="reasoningMessage">
    <div class="reasoningHeader" @click="expanded = !expanded">
      <MessageAuthor :icon="icon" :label="label" :meta="metaText" :variant="streaming ? 'streaming' : 'default'" />
      <ion-icon :icon="expanded ? chevronUp : chevronDown" class="reasoningChevron" />
    </div>
    <div v-if="expanded" class="reasoningBody">
      <MarkdownStream :source="text" :streaming="false" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { bulbOutline, chevronUpOutline, chevronDownOutline } from 'ionicons/icons'
import MessageAuthor from './MessageAuthor.vue'
import MarkdownStream from './MarkdownStream.vue'
import { useI18n } from '@/composables/useI18n'

const props = defineProps<{
  text: string
  streaming: boolean
}>()

const { t } = useI18n()
const expanded = ref(props.streaming)
const icon = bulbOutline
const chevronUp = chevronUpOutline
const chevronDown = chevronDownOutline

const label = computed(() => '推理')
const metaText = computed(() => (props.streaming ? t('agent.thinking') : ''))
</script>

<style scoped>
.reasoningMessage {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 6px 0;
  border-left: 2px solid rgba(var(--ion-color-medium-rgb), 0.2);
  padding-left: 8px;
  margin: 4px 0;
}

.reasoningHeader {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  cursor: pointer;
  user-select: none;
}

.reasoningChevron {
  font-size: 14px;
  color: var(--encv-text-secondary);
}

.reasoningBody {
  padding-left: 30px;
  font-size: 13px;
  color: var(--encv-text-secondary);
}
</style>
