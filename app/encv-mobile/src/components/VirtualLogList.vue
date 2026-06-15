<!--
  VirtualLogList - 虚拟滚动日志列表
  用 @tanstack/vue-virtual 渲染长列表（>5000 条仍丝滑）
  适用于 DevLogs 后端日志（WS 持续刷新）和前端日志（console 镜像）
  - 父级（DevLogs.vue）提供 scrollEl = ion-content 的 .inner-scroll 元素
  - 虚拟列表接管渲染：仅渲染可见窗口内 ~30 个 item，DOM 节点数恒定
  - ion-content 的 scroll 事件触发虚拟列表重新计算可见 items
  - 滚动到底部通过父级直接 setScrollTop(ionContentInnerScroll.scrollHeight) 即可
  - 切 tab 成本 = O(visible) = 30 个 DOM 节点从 unmount 到 mount，不是 O(N)
-->
<template>
  <div
    class="virtual-log-list"
    :style="{ height: `${totalSize}px`, position: 'relative', width: '100%' }"
  >
    <div
      v-for="vItem in virtualItems"
      :key="getKey(items[vItem.index])"
      :style="{
        position: 'absolute',
        top: 0,
        left: 0,
        width: '100%',
        height: `${vItem.size}px`,
        transform: `translateY(${vItem.start}px)`,
      }"
      class="log-entry"
      :class="[getLevel(items[vItem.index])]"
    >
      <slot :item="items[vItem.index]" :index="vItem.index" :highlight="highlightRange" />
    </div>
  </div>
</template>

<script setup lang="ts" generic="T extends { id: number; level: string }">
import { computed } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'

interface Props {
  items: T[]
  /** 滚动容器（ion-content 的 .inner-scroll） */
  scrollEl: HTMLElement | null
  /** 单条 item 估计高度（px），固定行高 */
  itemSize?: number
  /** 视口外额外预渲染条数（上下各 overscan 条） */
  overscan?: number
  /** 自定义 key（默认取 item.id） */
  getKey?: (item: T) => number | string
  /** 自定义 level class 字段（默认取 item.level） */
  getLevel?: (item: T) => string
  /** 搜索关键词（空字符串表示不高亮）；高亮由父级 CSS ::highlight 实现 */
  searchQuery?: string
  /** 从 item.message 提取纯文本字段（默认 .message） */
  getText?: (item: T) => string
}
const props = withDefaults(defineProps<Props>(), {
  itemSize: 28,
  overscan: 10,
  getKey: (item: T) => item.id,
  getLevel: (item: T) => item.level,
  searchQuery: '',
  getText: (item: any) => item.message,
})

const virtualizerOptions = computed(() => ({
  count: props.items.length,
  getScrollElement: () => props.scrollEl,
  estimateSize: () => props.itemSize,
  overscan: props.overscan,
}))

const virtualizer = useVirtualizer(virtualizerOptions)

const virtualItems = computed(() => virtualizer.value.getVirtualItems())
const totalSize = computed(() => virtualizer.value.getTotalSize())

/**
 * 高亮区间 [start, end)（CSS ::highlight 用 Range API 计算得出）
 * 当前 stub 实现：返回 null（父级用 plain text 渲染）
 * 父级可选择接入 CSS Custom Highlight API：
 *   const range = new Range()
 *   range.setStart(textNode, start)
 *   range.setEnd(textNode, end)
 *   const hl = new Highlight(range)
 *   CSS.highlights.set('log-search', hl)
 */
function highlightRange(_text: string, _query: string): { start: number; end: number }[] | null {
  return null
}
</script>

<style scoped>
/* 虚拟列表的 .log-entry 样式（与旧 v-for 渲染保持一致） */
.virtual-log-list {
  display: block;
}
.log-entry {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  font-family: var(--ion-font-family-monospace, 'Courier New', monospace);
  font-size: 12px;
  line-height: 20px;
  border-bottom: 1px solid var(--ion-color-light-shade, rgba(0, 0, 0, 0.05));
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  box-sizing: border-box;
}
.log-entry.error { background: rgba(239, 68, 68, 0.08); color: #b91c1c; }
.log-entry.warn  { background: rgba(245, 158, 11, 0.08); color: #92400e; }
.log-entry.info  { color: var(--ion-color-dark, #1f2937); }
.log-entry.debug { color: var(--ion-color-medium, #6b7280); }
</style>
