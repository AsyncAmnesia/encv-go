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
      <MarkdownStream :content="text" :streaming="streaming" />
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

/* AI 回复气泡：左侧浅色背景，圆角 */
.assistantMessageBody :deep(.markdownStream) {
  display: inline-block;
  max-width: 100%;
  padding: 10px 14px;
  background: rgba(var(--ion-color-medium-rgb), 0.10);
  border-radius: 12px 14px 14px 4px;
  border-top-left-radius: 4px;
  /* 流式输出时高度平滑过渡，消除跳动 */
  transition: height 0.25s ease-out, padding 0.25s ease-out;
}

/* 打字光标动画（流式期间显示） */
.assistantMessageBody :deep(.markdownStream_streaming)::after {
  content: '';
  display: inline-block;
  width: 2px;
  height: 1em;
  background: var(--ion-color-primary);
  margin-left: 2px;
  vertical-align: text-bottom;
  animation: cursorBlink 1s step-end infinite;
  border-radius: 1px;
}

@keyframes cursorBlink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}
</style>
