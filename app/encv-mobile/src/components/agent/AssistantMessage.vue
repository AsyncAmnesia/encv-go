<!--
  AssistantMessage - AI 回复文本块
  参照 codex_web assistantMessage / plainAssistantMessage：
  - 无背景气泡（纯文本，与页面背景融合）
  - MessageAuthor(28px 圆形头像, label, meta)
  - MarkdownStream(source, streaming)
  - markdownBody: 14px / line-height 1.62 / 正确间距
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
/* ── 参照 codex_web .assistantMessage ─────────────────────── */
.assistantMessage {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 18px;
  max-width: 100%;
}

.assistantMessageBody {
  padding-left: 36px; /* 28px avatar + 8px gap */
  min-width: 0;
}

/* ── markdownBody：参照 codex_web .markdownBody ──────────── */
.assistantMessageBody :deep(.markdownStream) {
  display: block;
  max-width: 100%;
  color: var(--ion-text-color);
  font-size: 14px;
  line-height: 1.62;
  overflow-wrap: break-word;
}

/* 段落/列表/代码块间距（codex_web: margin 0 0 12px） */
.assistantMessageBody :deep(.markdownStream) :deep(.node-slot) {
  margin-bottom: 12px;
}

.assistantMessageBody :deep(.markdownStream) :deep(.node-slot:last-child) {
  margin-bottom: 0;
}

/* 行内代码：圆角 + 浅灰底（codex_web 风格） */
.assistantMessageBody :deep(.markdownStream) :deep(.inline-code) {
  border-radius: 5px;
  padding: 1px 5px;
  background: var(--ion-color-light);
  font-size: 0.9em;
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
