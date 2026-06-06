<!--
  MarkdownStream - 流式 Markdown 渲染
  封装 markstream-vue 的 MarkdownRender（default export）
  正确 prop: content（不是 source！）
-->
<template>
  <div class="markdownStream" :class="{ markdownStream_streaming: streaming }">
    <MarkdownRender
      :key="content || '_empty'"
      :content="content"
      :smooth-streaming="streaming"
    />
  </div>
</template>

<script setup lang="ts">
import { MarkdownRender } from 'markstream-vue'
import 'markstream-vue/index.css'

defineProps<{
  content: string
  streaming?: boolean
}>()
</script>

<style>
/* 全局样式：让 markstream 在 dark mode 下可读 */
.markdownStream {
  font-size: 13.5px;
  line-height: 1.6;
  color: var(--ion-text-color);
  word-break: break-word;
}

/* 流式输出：高度平滑过渡 + 内容淡入 */
.markdownStream_streaming {
  transition: max-height 0.3s ease-out, opacity 0.2s ease-out;
}

.markdownStream_streaming > :deep(*) {
  animation: streamFadeIn 0.35s ease-out both;
}

/* 错开每段/每个元素的入场时间，避免同时闪烁 */
.markdownStream_streaming > :deep(:nth-child(1)) { animation-delay: 0ms; }
.markdownStream_streaming > :deep(:nth-child(2)) { animation-delay: 40ms; }
.markdownStream_streaming > :deep(:nth-child(3)) { animation-delay: 80ms; }
.markdownStream_streaming > :deep(:nth-child(4)) { animation-delay: 120ms; }
.markdownStream_streaming > :deep(:nth-child(5)) { animation-delay: 160ms; }
.markdownStream_streaming > :deep(:nth-child(n+6)) { animation-delay: 200ms; }

@keyframes streamFadeIn {
  from {
    opacity: 0;
    transform: translateY(3px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.markdownStream p {
  margin: 6px 0;
}

.markdownStream h1,
.markdownStream h2,
.markdownStream h3,
.markdownStream h4,
.markdownStream h5,
.markdownStream h6 {
  margin: 12px 0 6px;
  font-weight: 700;
  color: var(--ion-text-color);
}

.markdownStream h1 { font-size: 18px; }
.markdownStream h2 { font-size: 16px; }
.markdownStream h3 { font-size: 15px; }
.markdownStream h4 { font-size: 14px; }
.markdownStream h5 { font-size: 13.5px; }
.markdownStream h6 { font-size: 13px; }

.markdownStream ul,
.markdownStream ol {
  padding-left: 20px;
  margin: 6px 0;
}

.markdownStream li {
  margin: 2px 0;
}

.markdownStream code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12.5px;
  padding: 1px 5px;
  background: rgba(var(--ion-color-medium-rgb), 0.16);
  border-radius: 4px;
}

.markdownStream pre {
  margin: 6px 0;
  padding: 8px 12px;
  background: rgba(var(--ion-color-medium-rgb), 0.08);
  border-radius: 6px;
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.18);
  overflow: auto;
}

.markdownStream pre code {
  background: transparent;
  padding: 0;
  font-size: 12px;
}

.markdownStream blockquote {
  margin: 6px 0;
  padding: 4px 10px;
  border-left: 3px solid rgba(var(--ion-color-primary-rgb), 0.4);
  color: var(--encv-text-secondary);
  background: rgba(var(--ion-color-primary-rgb), 0.04);
  border-radius: 0 4px 4px 0;
}

.markdownStream a {
  color: var(--ion-color-primary);
  text-decoration: none;
}

.markdownStream a:hover {
  text-decoration: underline;
}

.markdownStream table {
  border-collapse: collapse;
  margin: 6px 0;
  font-size: 12.5px;
  width: 100%;
}

.markdownStream th,
.markdownStream td {
  padding: 4px 8px;
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.3);
  text-align: left;
}

.markdownStream th {
  background: rgba(var(--ion-color-medium-rgb), 0.12);
  font-weight: 600;
}

.markdownStream_streaming pre {
  /* 流式期间 code block 不高亮，避免闪烁 */
  filter: saturate(0.85);
}
</style>
