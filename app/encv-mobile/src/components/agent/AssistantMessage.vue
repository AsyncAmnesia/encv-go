<!--
  AssistantMessage - AI 回复文本块
  头部：MessageAuthor(sparklesOutline, "Codex", meta)
  主体：MarkdownStream(source, streaming)
-->
<template>
  <div class="assistantMessage">
    <MessageAuthor
      :icon="icon"
      :label="label"
      :meta="meta"
      :variant="streaming ? 'streaming' : 'default'"
    />
    <div class="assistantMessageBody">
      <MarkdownStream :source="text" :streaming="streaming" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { sparklesOutline } from 'ionicons/icons'
import MessageAuthor from './MessageAuthor.vue'
import MarkdownStream from './MarkdownStream.vue'
import { useI18n } from '@/composables/useI18n'
import type { AgentStatus } from '@/composables/useAgent'

const props = defineProps<{
  text: string
  streaming: boolean
  status?: AgentStatus
}>()

const { t } = useI18n()
const icon = sparklesOutline

const label = computed(() => 'AI 助手')
const meta = computed(() => {
  if (props.streaming) return t('agent.thinking')
  return ''
})
</script>

<style scoped>
.assistantMessage {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 0 4px;
  max-width: 100%;
}

.assistantMessageBody {
  padding-left: 30px;
  min-width: 0;
}
</style>
