<!--
  MarkdownStream - 流式 Markdown 渲染
  封装 markstream-vue 的 MarkdownRender
  - props: source: string, streaming: boolean
  - streaming=true 启用渐进渲染（代码块/表格未完结时使用占位）
-->
<template>
  <div class="markdownStream" :class="{ markdownStream_streaming: streaming }">
    <MarkdownRender
      :key="source || '_empty'"
      :source="source"
      :render-code-blocks="!streaming"
      :disable-shiki="streaming"
    />
    <!-- markstream-vue 渲染失败时的降级：直接显示原始文本 -->
    <div v-if="source && !streaming" class="markdown-fallback" ref="fallbackRef"></div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, onMounted } from 'vue'
import { MarkdownRender } from 'markstream-vue'
import 'markstream-vue/index.css'

const props = defineProps<{
  source: string
  streaming?: boolean
}>()

const fallbackRef = ref<HTMLElement | null>(null)

// 当 MarkdownRender 可能未渲染时，用纯文本兜底
async function checkFallback() {
  await nextTick()
  if (!fallbackRef.value) return
  const parent = fallbackRef.value.parentElement
  if (!parent) return
  // 检查同级的 MarkdownRender 是否有实际内容（非空注释节点）
  const renderEl = parent.querySelector('.markdown-render, [data-markstream]')
  const hasContent = renderEl && renderEl.innerHTML.replace(/<!--[\s\S]*?-->/g, '').trim().length > 0
  if (!hasContent && props.source) {
    // MarkdownRender 未产出内容 → 显示原文
    fallbackRef.value.textContent = props.source
    fallbackRef.value.style.display = ''
  } else {
    // MarkdownRender 有内容 → 隐藏兜底
    if (fallbackRef.value) fallbackRef.value.style.display = 'none'
  }
}

watch(() => props.source, () => { checkFallback() }, { immediate: true })
onMounted(() => { checkFallback() })
</script>

<style>
/* 全局样式：让 markstream 在 dark mode 下可读 */
.markdownStream {
  font-size: 13.5px;
  line-height: 1.6;
  color: var(--ion-text-color);
  word-break: break-word;
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

/* 纯文本降级显示（默认可见，markstream 有内容时由 JS 隐藏） */
.markdown-fallback {
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 13.5px;
  line-height: 1.6;
}
</style>
