<!--
  FileContentCard - 文件内容预览卡片
  渲染 read_file 工具的返回结果。data 形状：
    { content: string, mimeType?: string, size?: number, note?: string }

  设计要点：
  - 头部展示元数据（mimeType + size + 行数）
  - 正文用等宽字体 + 代码块样式（保留 markdown 格式）
  - content > 4000 字符自动折叠（展开按钮）
  - 真实数据来自 read_file handler，mock 模式下走 execute_real=true
-->
<template>
  <div class="fileContentCard">
    <div class="fileContentCardHeader">
      <ion-icon :icon="documentTextIcon" class="fileContentCardIcon" />
      <span class="fileContentCardTitle">{{ titleText }}</span>
      <span v-if="meta.size !== undefined" class="fileContentCardMeta">{{ formatSize(meta.size) }}</span>
      <span v-if="meta.mimeType" class="fileContentCardMime">{{ meta.mimeType }}</span>
    </div>
    <pre v-if="content" class="fileContentCardBody" :class="{ fileContentCardBody_collapsed: !expanded }"><code>{{ expanded ? content : truncatedContent }}</code></pre>
    <div v-else class="fileContentCardEmpty">文件内容为空</div>
    <div v-if="showToggle" class="fileContentCardActions">
      <button type="button" class="fileContentCardToggle" @click="toggle">
        {{ expanded ? '收起' : `展开全部 (${content.length} 字符)` }}
      </button>
    </div>
    <details v-if="rawResult" class="fileContentCardRaw">
      <summary>查看原始数据</summary>
      <pre>{{ rawResult }}</pre>
    </details>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { IonIcon } from '@ionic/vue'
import { documentTextOutline } from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'

const props = defineProps<{
  /** 后端 tool_result.result 的 JSON 字符串 */
  resultJson: string
}>()

const { t } = useI18n()

const documentTextIcon = documentTextOutline
const expanded = ref(false)

const COLLAPSE_THRESHOLD = 4000

interface ParsedFile {
  content: string
  mimeType?: string
  size?: number
  note?: string
}

const parsed = computed<{ data: ParsedFile | null; error: string }>(() => {
  if (!props.resultJson) {
    return { data: null, error: 'empty result' }
  }
  try {
    const obj = JSON.parse(props.resultJson) as Partial<ParsedFile>
    if (typeof obj.content !== 'string') {
      return { data: null, error: 'missing content field' }
    }
    return {
      data: {
        content: obj.content,
        mimeType: typeof obj.mimeType === 'string' ? obj.mimeType : undefined,
        size: typeof obj.size === 'number' ? obj.size : undefined,
        note: typeof obj.note === 'string' ? obj.note : undefined,
      },
      error: '',
    }
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    console.debug('[FileContentCard] parse failed:', msg, props.resultJson)
    return { data: null, error: msg }
  }
})

const content = computed(() => parsed.value.data?.content ?? '')
const meta = computed(() => ({
  mimeType: parsed.value.data?.mimeType,
  size: parsed.value.data?.size,
}))
const rawResult = computed(() => (parsed.value.error ? props.resultJson : ''))

const showToggle = computed(() => content.value.length > COLLAPSE_THRESHOLD)
const truncatedContent = computed(() => {
  if (content.value.length <= COLLAPSE_THRESHOLD) return content.value
  return content.value.slice(0, COLLAPSE_THRESHOLD) + '\n…'
})

const titleText = computed(() => {
  if (parsed.value.error) return t('agent.toolCards.parseFailed') || '文件内容（数据异常）'
  return t('agent.toolCards.fileContentTitle') || '文件内容'
})

function toggle() {
  expanded.value = !expanded.value
}

function formatSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return '—'
  if (bytes < 1024) return `${bytes} B`
  const kb = bytes / 1024
  if (kb < 1024) return `${kb.toFixed(1)} KB`
  const mb = kb / 1024
  if (mb < 1024) return `${mb.toFixed(1)} MB`
  return `${(mb / 1024).toFixed(2)} GB`
}
</script>

<style scoped>
.fileContentCard {
  margin: 4px 0 6px;
  background: rgba(var(--ion-color-medium-rgb), 0.05);
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.18);
  border-radius: 8px;
  padding: 8px 10px;
  font-size: 12px;
}

.fileContentCardHeader {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 6px;
  flex-wrap: wrap;
}

.fileContentCardIcon {
  font-size: 14px;
  color: var(--ion-color-primary);
}

.fileContentCardTitle {
  font-weight: 600;
  color: var(--ion-text-color);
}

.fileContentCardMeta {
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
  font-size: 10.5px;
  color: var(--encv-text-secondary, #888);
}

.fileContentCardMime {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 8px;
  background: rgba(var(--ion-color-primary-rgb), 0.14);
  color: var(--ion-color-primary);
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
}

.fileContentCardBody {
  margin: 0;
  padding: 8px 10px;
  background: rgba(var(--ion-color-medium-rgb), 0.04);
  border-radius: 5px;
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
  font-size: 11.5px;
  line-height: 1.55;
  color: var(--ion-text-color);
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 360px;
  overflow-y: auto;
}

.fileContentCardBody code {
  font-family: inherit;
  background: transparent;
  padding: 0;
  font-size: inherit;
  color: inherit;
}

.fileContentCardBody_collapsed {
  max-height: 180px;
  overflow: hidden;
  position: relative;
}

.fileContentCardActions {
  margin-top: 4px;
  text-align: right;
}

.fileContentCardToggle {
  background: transparent;
  border: 0;
  padding: 2px 0;
  font-size: 11.5px;
  color: var(--ion-color-primary);
  cursor: pointer;
  font-family: inherit;
}

.fileContentCardToggle:hover {
  text-decoration: underline;
}

.fileContentCardEmpty {
  padding: 8px 0;
  text-align: center;
  color: var(--encv-text-secondary, #888);
  font-size: 11.5px;
}

.fileContentCardRaw {
  margin-top: 6px;
  font-size: 10.5px;
}

.fileContentCardRaw summary {
  cursor: pointer;
  color: var(--encv-text-secondary, #888);
  user-select: none;
}

.fileContentCardRaw pre {
  margin: 4px 0 0;
  padding: 6px 8px;
  background: rgba(var(--ion-color-medium-rgb), 0.08);
  border-radius: 4px;
  overflow-x: auto;
  font-size: 10.5px;
  max-height: 160px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
