<!--
  MessageVirtualList - 虚拟滚动消息列表
  封装 vue-virtual-scroller 的 RecycleScroller
  - itemSize=112, minItemSize=80, buffer=600
  - 暴露 scrollToLatest(behavior) 方法
-->
<template>
  <div class="messageVirtualList" ref="containerRef">
    <component
      :is="RecycleScroller"
      v-if="messages.length >= VIRTUAL_LIST_THRESHOLD"
      ref="scrollerRef"
      class="virtualScroller"
      :items="messages"
      :item-size="estimateSize"
      :min-item-size="minItemSize"
      :buffer="overscan"
      key-field="id"
      @scroll="onScroll"
    >
      <template #default="{ item, index }">
        <div class="virtualItem" :data-index="index">
          <slot name="item" :item="item" :index="index" />
        </div>
      </template>
    </component>
    <div v-else class="messagePlainList">
      <div
        v-for="(item, index) in messages"
        :key="(item as any).id || index"
        class="virtualItem"
        :data-index="index"
      >
        <slot name="item" :item="item" :index="index" />
      </div>
      <slot name="empty" v-if="messages.length === 0" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { RecycleScroller } from 'vue-virtual-scroller'
import 'vue-virtual-scroller/dist/vue-virtual-scroller.css'

export interface MessageListItem {
  id: string
  [key: string]: unknown
}

const props = withDefaults(
  defineProps<{
    messages: MessageListItem[]
    estimateSize?: number
    minItemSize?: number
    overscan?: number
  }>(),
  {
    estimateSize: 112,
    minItemSize: 80,
    overscan: 600,
  },
)

const VIRTUAL_LIST_THRESHOLD = 120

const scrollerRef = ref<InstanceType<typeof RecycleScroller> | null>(null)
const containerRef = ref<HTMLDivElement | null>(null)

function scrollToLatest(behavior: 'auto' | 'smooth' = 'smooth') {
  if (scrollerRef.value && typeof (scrollerRef.value as any).scrollToItem === 'function') {
    ;(scrollerRef.value as any).scrollToItem(props.messages.length - 1, behavior)
  } else if (containerRef.value) {
    // 降级：直接滚到 container 底部
    const el = containerRef.value
    el.scrollTo({ top: el.scrollHeight, behavior })
  }
}

function onScroll(_e: Event) {
  // 子组件可监听 scroll 事件以实现"是否接近底部"判断
  // 当前由 AgentChat 的 nearBottom computed 维护
}

defineExpose({ scrollToLatest })
</script>

<style scoped>
.messageVirtualList {
  position: relative;
  flex: 1;
  min-height: 0;
}

.virtualScroller {
  height: 100%;
  width: 100%;
}

.virtualItem {
  padding: 0;
}

.messagePlainList {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 8px 12px;
}
</style>
