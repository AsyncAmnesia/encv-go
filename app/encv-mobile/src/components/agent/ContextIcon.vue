<!--
  ContextIcon - 顶部 header 内的上下文使用图标按钮

  行为：
  - 显示当前会话的上下文使用百分比（X.X%）+ 压缩次数徽章
  - 点击 → 弹出 ContextPopover（任务列表 + 上下文详情 + 引用文件）
  - 高负载时（>=80%）图标 + 文字变橙红色，提示接近上限

  设计要点：
  - 紧凑：button 内只放 icon + 百分比 + 可选徽章
  - 暗黑/亮色模式兼容：颜色全部走 CSS 变量
  - disabled 状态：拉取失败/未就绪时显示「—」占位
-->
<template>
  <ion-button
    fill="clear"
    size="small"
    class="context-icon-btn"
    :class="[
      toneClass,
      { 'context-icon-btn_compact': compact },
    ]"
    :aria-label="ariaLabel"
    @click="openPopover"
  >
    <ion-icon :icon="layersIcon" slot="start" class="context-icon-svg" />
    <span v-if="!compact" class="context-icon-text">{{ percentText }}</span>
    <ion-badge
      v-if="compactions > 0"
      class="context-icon-badge"
      :color="compactions >= 3 ? 'warning' : 'medium'"
    >×{{ compactions }}</ion-badge>
  </ion-button>

  <!-- Popover 容器 -->
  <ion-popover
    :is-open="isOpen"
    :event="event"
    side="bottom"
    alignment="end"
    class="context-popover-host"
    :show-backdrop="true"
    :backdrop-dismiss="true"
    style="--width: 92vw; --max-height: 70vh; --min-height: 200px;"
    @did-dismiss="closePopover"
  >
    <ContextPopover
      v-if="isOpen"
      :data="data"
      :loading="loading ?? false"
      @close="closePopover"
    />
  </ion-popover>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { IonButton, IonIcon, IonBadge, IonPopover } from '@ionic/vue'
import { layers as layersIcon } from 'ionicons/icons'
import ContextPopover from './ContextPopover.vue'
import type { ContextUsageResponse } from '@/composables/useContextUsage'

const props = defineProps<{
  /** null = 尚未拉到数据；非 null = 已就绪 */
  data: ContextUsageResponse | null
  loading?: boolean
  /** 紧凑模式：只显示图标，不显示百分比 */
  compact?: boolean
}>()

const isOpen = ref(false)
const event = ref<Event | null>(null)

const ariaLabel = computed(() => {
  if (!props.data) return '上下文使用（加载中）'
  return `上下文使用 ${props.data.usage.percent.toFixed(1)}%`
})

const percentText = computed(() => {
  if (!props.data) return '—'
  return props.data.usage.percent.toFixed(1) + '%'
})

const compactions = computed(() => props.data?.compactions ?? 0)

const toneClass = computed(() => {
  if (!props.data) return 'tone-idle'
  const p = props.data.usage.percent
  if (p >= 90) return 'tone-danger'
  if (p >= 70) return 'tone-warn'
  return 'tone-ok'
})

function openPopover(e: Event) {
  event.value = e
  isOpen.value = true
}

function closePopover() {
  isOpen.value = false
  event.value = null
}
</script>

<style scoped>
.context-icon-btn {
  --padding-start: 6px;
  --padding-end: 6px;
  --color: var(--ion-color-primary);
  font-size: 11.5px;
  position: relative;
}

.context-icon-svg {
  font-size: 14px;
  margin-inline-end: 2px;
}

.context-icon-text {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.02em;
}

.context-icon-badge {
  font-size: 9px;
  margin-inline-start: 4px;
  padding: 1px 4px;
  min-width: 16px;
}

/* tone: ok（< 70%） */
.tone-ok {
  --color: var(--ion-color-primary);
}

/* tone: warn（70% - 90%） */
.tone-warn {
  --color: #f59e0b;
}

/* tone: danger（>= 90%） */
.tone-danger {
  --color: #ef4444;
}

.tone-idle {
  --color: var(--encv-text-secondary);
}

/* 紧凑模式：去掉文字 */
.context-icon-btn_compact .context-icon-text {
  display: none;
}

.context-icon-btn_compact .context-icon-svg {
  font-size: 16px;
  margin-inline-end: 0;
}
</style>
