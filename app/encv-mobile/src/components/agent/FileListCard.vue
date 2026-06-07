<!--
  FileListCard - 文件列表结构化卡片
  渲染 list_files / stat_file 工具的返回结果。data 形状：
    { files: [{ name, size?, is_dir, mtime? }] }   // list_files
    { name, size, is_dir, mtime }                   // stat_file 单条记录

  设计要点：
  - 表格布局：图标 / 名称 / 大小 / 类型 列
  - 目录项用 folder 图标 + 蓝色背景
  - 文件项用 document 图标 + 灰色背景
  - 尺寸自动 humanize（B/KB/MB/GB）
  - mtime 可选展示
  - 真实数据来自 list_files handler，mock 模式下走 execute_real=true
-->
<template>
  <div class="fileListCard">
    <div class="fileListCardHeader">
      <ion-icon :icon="listIcon" class="fileListCardIcon" />
      <span class="fileListCardTitle">{{ titleText }}</span>
      <span v-if="rows.length > 0" class="fileListCardCount">{{ rows.length }}</span>
    </div>
    <div v-if="rows.length > 0" class="fileListCardTable">
      <div class="fileListCardRow fileListCardRowHead">
        <span class="fileListCardColIcon"></span>
        <span class="fileListCardColName">名称</span>
        <span class="fileListCardColSize">大小</span>
        <span class="fileListCardColType">类型</span>
      </div>
      <div
        v-for="(row, idx) in rows"
        :key="`${row.name}-${idx}`"
        class="fileListCardRow"
      >
        <span class="fileListCardColIcon">
          <ion-icon :icon="row.is_dir ? folderIcon : fileIcon" />
        </span>
        <span class="fileListCardColName">
          <span class="fileListCardNameText" :class="{ fileListCardNameText_dir: row.is_dir }">{{ row.name }}</span>
          <span v-if="row.mtime" class="fileListCardNameMeta">{{ row.mtime }}</span>
        </span>
        <span class="fileListCardColSize">{{ row.is_dir ? '—' : formatSize(row.size) }}</span>
        <span class="fileListCardColType">{{ row.is_dir ? '目录' : '文件' }}</span>
      </div>
    </div>
    <div v-else class="fileListCardEmpty">空目录（无文件）</div>
    <details v-if="rawResult" class="fileListCardRaw">
      <summary>查看原始数据</summary>
      <pre>{{ rawResult }}</pre>
    </details>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { IonIcon } from '@ionic/vue'
import { folderOutline, documentOutline, listOutline } from 'ionicons/icons'
import { useI18n } from '@/composables/useI18n'

const props = defineProps<{
  /** 后端 tool_result.result 的 JSON 字符串 */
  resultJson: string
}>()

const { t } = useI18n()

const folderIcon = folderOutline
const fileIcon = documentOutline
const listIcon = listOutline

interface FileRow {
  name: string
  size?: number
  is_dir: boolean
  mtime?: string
}

const parsed = computed<{ rows: FileRow[]; error: string; isStat: boolean }>(() => {
  if (!props.resultJson) {
    return { rows: [], error: 'empty result', isStat: false }
  }
  try {
    const obj = JSON.parse(props.resultJson) as
      | { files?: FileRow[]; name?: string; size?: number; is_dir?: boolean; mtime?: string }
      | FileRow[]
    if (Array.isArray(obj)) {
      return { rows: obj.filter((r) => r && typeof r.name === 'string'), error: '', isStat: false }
    }
    if (Array.isArray(obj.files)) {
      return { rows: obj.files, error: '', isStat: false }
    }
    if (typeof obj.name === 'string') {
      // stat_file 返回单条记录
      return {
        rows: [
          {
            name: obj.name,
            size: typeof obj.size === 'number' ? obj.size : undefined,
            is_dir: obj.is_dir === true,
            mtime: typeof obj.mtime === 'string' ? obj.mtime : undefined,
          },
        ],
        error: '',
        isStat: true,
      }
    }
    return { rows: [], error: 'unrecognized shape', isStat: false }
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    console.debug('[FileListCard] parse failed:', msg, props.resultJson)
    return { rows: [], error: msg, isStat: false }
  }
})

const rows = computed(() => parsed.value.rows)
const rawResult = computed(() => (parsed.value.error ? props.resultJson : ''))

const titleText = computed(() => {
  if (parsed.value.error) return t('agent.toolCards.parseFailed') || '文件列表（数据异常）'
  if (parsed.value.isStat) return t('agent.toolCards.fileStatTitle') || '文件信息'
  return t('agent.toolCards.fileListTitle') || '文件列表'
})

function formatSize(bytes?: number): string {
  if (typeof bytes !== 'number' || !Number.isFinite(bytes) || bytes < 0) return '—'
  if (bytes < 1024) return `${bytes} B`
  const kb = bytes / 1024
  if (kb < 1024) return `${kb.toFixed(1)} KB`
  const mb = kb / 1024
  if (mb < 1024) return `${mb.toFixed(1)} MB`
  const gb = mb / 1024
  return `${gb.toFixed(2)} GB`
}
</script>

<style scoped>
.fileListCard {
  margin: 4px 0 6px;
  background: rgba(var(--ion-color-medium-rgb), 0.05);
  border: 1px solid rgba(var(--ion-color-medium-rgb), 0.18);
  border-radius: 8px;
  padding: 8px 10px;
  font-size: 12px;
}

.fileListCardHeader {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 6px;
}

.fileListCardIcon {
  font-size: 14px;
  color: var(--ion-color-primary);
}

.fileListCardTitle {
  font-weight: 600;
  color: var(--ion-text-color);
}

.fileListCardCount {
  margin-inline-start: auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 18px;
  padding: 0 6px;
  border-radius: 9px;
  background: rgba(var(--ion-color-primary-rgb), 0.18);
  color: var(--ion-color-primary);
  font-size: 10px;
  font-weight: 600;
}

.fileListCardTable {
  display: flex;
  flex-direction: column;
  gap: 1px;
  background: rgba(var(--ion-color-medium-rgb), 0.06);
  border-radius: 5px;
  overflow: hidden;
}

.fileListCardRow {
  display: grid;
  grid-template-columns: 22px 1fr 70px 50px;
  gap: 6px;
  align-items: center;
  padding: 4px 8px;
  background: var(--ion-background-color, transparent);
  font-size: 11.5px;
  min-width: 0;
}

.fileListCardRowHead {
  background: rgba(var(--ion-color-medium-rgb), 0.08);
  font-size: 10.5px;
  font-weight: 600;
  color: var(--encv-text-secondary, #888);
  text-transform: uppercase;
  letter-spacing: 0.4px;
}

.fileListCardColIcon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  color: var(--ion-color-primary);
}

.fileListCardColName {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 1px;
}

.fileListCardNameText {
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
  color: var(--ion-text-color);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.fileListCardNameText_dir {
  font-weight: 600;
  color: var(--ion-color-primary);
}

.fileListCardNameMeta {
  font-size: 10px;
  color: var(--encv-text-secondary, #888);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fileListCardColSize {
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
  font-size: 10.5px;
  color: var(--encv-text-secondary, #888);
  text-align: right;
}

.fileListCardColType {
  font-size: 10.5px;
  color: var(--encv-text-secondary, #888);
  text-align: center;
}

.fileListCardEmpty {
  padding: 8px 0;
  text-align: center;
  color: var(--encv-text-secondary, #888);
  font-size: 11.5px;
}

.fileListCardRaw {
  margin-top: 6px;
  font-size: 10.5px;
}

.fileListCardRaw summary {
  cursor: pointer;
  color: var(--encv-text-secondary, #888);
  user-select: none;
}

.fileListCardRaw pre {
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
